// Package secret 是密钥存储（B.07.02）：优先系统密钥库，不可用时用主密码加密的本地文件。
// Store 按方法签名实现 engine.SecretResolver（B.07-R1），只在调用 Resolve 时读取密钥（S.03-R3）。
package secret

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mio-cc/anywhere-door/internal/platform/datadir"
)

// 哨兵错误。
var (
	ErrLocked          = errors.New("secret: 密钥库已锁定，请先解锁")
	ErrNotFound        = errors.New("secret: 没有这个密钥")
	ErrBadName         = errors.New("secret: 密钥名只能是小写字母开头的小写字母、数字、下划线，最长 64")
	ErrBadPassword     = errors.New("secret: 主密码不对")
	ErrTooManyAttempts = errors.New("secret: 主密码连续错误 5 次，请 30 秒后再试")
	ErrCorrupt         = errors.New("secret: 加密文件已损坏")
)

// Info 是 List 的一项（不含密钥值）。
type Info struct {
	Name, Last4 string
	UpdatedAt   time.Time
}

// Keyring 是系统密钥库的三个操作；缺省是 zalando/go-keyring 的同名函数，测试注入假的。
type Keyring interface {
	Get(service, user string) (string, error)
	Set(service, user, password string) error
	Delete(service, user string) error
}

// Options 是 Open 的选项。
type Options struct {
	Keyring   Keyring             // nil：系统密钥库
	ForceFile bool                // 不探测，直接用加密文件
	Dev       bool                // --dev：允许 ANYDOOR_DEV_SECRET_<名>（S.03-R8）
	Getenv    func(string) string // 缺省 os.Getenv
	Now       func() time.Time
	Logger    *slog.Logger
}

// Service 是系统密钥库里的服务名。
const Service = "AnywhereDoor"

const (
	probeUser   = "__anydoor_probe__"
	maxFailures = 5
	cooldown    = 30 * time.Second
	devPrefix   = "ANYDOOR_DEV_SECRET_"
	tsLayout    = "2006-01-02T15:04:05.000Z"
)

var nameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// Store 是密钥库；并发安全。
type Store struct {
	mu        sync.Mutex
	d         datadir.Dir
	o         Options
	kr        Keyring // 非 nil：系统密钥库后端
	key       []byte  // 文件后端解锁后派生出的密钥；nil 表示锁定
	hdr       kdfParams
	failures  int
	coolUntil time.Time
	warned    map[string]bool
}

// Open 选后端：ForceFile 时用文件；否则探测系统密钥库，失败改用文件。
func Open(ctx context.Context, d datadir.Dir, o Options) (*Store, error) {
	if o.Getenv == nil {
		o.Getenv = os.Getenv
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	s := &Store{d: d, o: o, warned: map[string]bool{}}
	if !o.ForceFile {
		kr := o.Keyring
		if kr == nil {
			kr = systemKeyring{}
		}
		if err := probe(kr); err != nil {
			o.Logger.Info("secret: 系统密钥库不可用，改用加密文件", "err", err)
		} else {
			s.kr = kr
		}
	}
	return s, nil
}

func probe(kr Keyring) error {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return err
	}
	v := hex.EncodeToString(b[:])
	if err := kr.Set(Service, probeUser, v); err != nil {
		return err
	}
	got, err := kr.Get(Service, probeUser)
	derr := kr.Delete(Service, probeUser)
	if err != nil {
		return err
	}
	if got != v {
		return errors.New("读回的值和写入的不同")
	}
	return derr
}

// Backend 是 keyring 或 file。
func (s *Store) Backend() string {
	if s.kr != nil {
		return "keyring"
	}
	return "file"
}

// Locked 是文件后端且没有解锁。
func (s *Store) Locked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.kr == nil && s.key == nil
}

// Resolve 读一个密钥（满足 engine.SecretResolver）。
func (s *Store) Resolve(ctx context.Context, name string) (string, error) {
	if !nameRe.MatchString(name) {
		return "", ErrBadName
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	env := devPrefix + strings.ToUpper(name)
	if v := s.o.Getenv(env); v != "" {
		if s.o.Dev {
			return v, nil
		}
		if !s.warned[env] {
			s.warned[env] = true
			s.o.Logger.Warn("secret: 不是开发模式，忽略环境变量", "name", env)
		}
	}
	if s.kr != nil {
		v, err := s.kr.Get(Service, name)
		if err != nil {
			return "", s.keyringErr(name, err)
		}
		return v, nil
	}
	m, err := s.readSecrets()
	if err != nil {
		return "", err
	}
	v, ok := m[name]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

// Put 写一个密钥，同时更新索引。
func (s *Store) Put(ctx context.Context, name, value string) error {
	if !nameRe.MatchString(name) {
		return ErrBadName
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.kr != nil {
		if err := s.kr.Set(Service, name, value); err != nil {
			return fmt.Errorf("secret: 写不了 %s：%w", name, err)
		}
	} else {
		m, err := s.readSecrets()
		if err != nil {
			return err
		}
		m[name] = value
		if err := s.writeSecrets(m); err != nil {
			return err
		}
	}
	return s.updateIndex(name, &value)
}

// Delete 删一个密钥；不存在也返回 nil。
func (s *Store) Delete(ctx context.Context, name string) error {
	if !nameRe.MatchString(name) {
		return ErrBadName
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.kr != nil {
		if err := s.kr.Delete(Service, name); err != nil && !errors.Is(s.keyringErr(name, err), ErrNotFound) {
			return fmt.Errorf("secret: 删不了 %s：%w", name, err)
		}
	} else {
		m, err := s.readSecrets()
		if err != nil {
			return err
		}
		if _, ok := m[name]; ok {
			delete(m, name)
			if err := s.writeSecrets(m); err != nil {
				return err
			}
		}
	}
	return s.updateIndex(name, nil)
}

// List 读索引，按名字排序；锁定时也能列。
func (s *Store) List(ctx context.Context) ([]Info, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx, err := s.readIndex()
	if err != nil {
		return nil, err
	}
	out := make([]Info, 0, len(idx))
	for name, e := range idx {
		at, _ := time.Parse(time.RFC3339Nano, e.UpdatedAt)
		out = append(out, Info{Name: name, Last4: e.Last4, UpdatedAt: at.UTC()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

type indexEntry struct {
	Last4     string `json:"last4"`
	UpdatedAt string `json:"updated_at"`
}

func (s *Store) readIndex() (map[string]indexEntry, error) {
	b, err := os.ReadFile(s.d.SecretsIndex())
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]indexEntry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("secret: 读不了索引：%w", err)
	}
	m := map[string]indexEntry{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("secret: 索引格式不对：%w", err)
	}
	return m, nil
}

// updateIndex 设置（value 非 nil）或删除一项。
func (s *Store) updateIndex(name string, value *string) error {
	m, err := s.readIndex()
	if err != nil {
		return err
	}
	if value == nil {
		if _, ok := m[name]; !ok {
			return nil
		}
		delete(m, name)
	} else {
		m[name] = indexEntry{Last4: last4(*value), UpdatedAt: s.o.Now().UTC().Format(tsLayout)}
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := datadir.WritePrivate(s.d.SecretsIndex(), append(b, '\n')); err != nil {
		return fmt.Errorf("secret: 写不了索引：%w", err)
	}
	return nil
}

// last4 是值的末 4 个字符；不足 8 个字符时为空串（S.03-R5）。
func last4(v string) string {
	r := []rune(v)
	if len(r) < 8 {
		return ""
	}
	return string(r[len(r)-4:])
}
