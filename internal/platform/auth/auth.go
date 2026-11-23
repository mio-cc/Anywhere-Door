// Package auth 是本机鉴权的平台部分（B.07.01）：本机 token 的生成与保存，会话 cookie 值的发放与校验。
// HTTP 层（设 cookie、/?t= 引导、Host / Origin 校验）在 internal/api（B.03.05）。
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mio-cc/anywhere-door/internal/platform/datadir"
)

// KV 是会话的存放处；组装时传 store.Store.Settings()（E.07.01 的 SettingsRepo 满足它）。
type KV interface {
	Get(ctx context.Context, key string) (json.RawMessage, bool, error)
	Put(ctx context.Context, key string, value json.RawMessage, at time.Time) error
}

// Options 是 Open 的选项。
type Options struct {
	Now         func() time.Time // 缺省 time.Now
	MaxSessions int              // ≤ 0 时为 100
}

const (
	CookieName  = "hx_session"
	SessionTTL  = 30 * 24 * time.Hour // 一律 30 天（P.09 K239）
	SessionsKey = "auth.sessions"     // KV 里的键
)

const tsLayout = "2006-01-02T15:04:05.000Z"

// Auth 持有本机 token 和会话表；并发安全。
type Auth struct {
	mu       sync.Mutex
	d        datadir.Dir
	kv       KV
	now      func() time.Time
	max      int
	token    string
	sessions map[string]time.Time // sha256 hex → 过期时间
}

// Open 读出（没有就生成）本机 token，读出会话表并去掉已过期的。
func Open(ctx context.Context, d datadir.Dir, kv KV, o Options) (*Auth, error) {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.MaxSessions <= 0 {
		o.MaxSessions = 100
	}
	a := &Auth{d: d, kv: kv, now: o.Now, max: o.MaxSessions, sessions: map[string]time.Time{}}
	tok, err := ReadToken(d)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if tok, err = newSecret(); err != nil {
			return nil, err
		}
		if err := writeToken(d, tok); err != nil {
			return nil, err
		}
	case err != nil:
		return nil, err
	}
	a.token = tok
	if err := a.load(ctx); err != nil {
		return nil, err
	}
	return a, nil
}

// ReadToken 只读 token 文件（anydoorctl 用，B.08）；文件不存在或格式不对返回错误。
func ReadToken(d datadir.Dir) (string, error) {
	p := d.Token()
	b, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("auth: %s 不存在：%w", p, err)
	}
	if err != nil {
		return "", fmt.Errorf("auth: 读不了 %s：%w，请检查数据目录权限", p, err)
	}
	t := strings.TrimSpace(string(b))
	if raw, err := base64.RawURLEncoding.DecodeString(t); err != nil || len(raw) != 32 {
		return "", fmt.Errorf("auth: %s 内容不是有效的 token，删除它后重启会重新生成", p)
	}
	return t, nil
}

func writeToken(d datadir.Dir, tok string) error {
	if err := datadir.WritePrivate(d.Token(), []byte(tok+"\n")); err != nil {
		return fmt.Errorf("auth: 写不了 token 文件：%w", err)
	}
	return nil
}

// newSecret 是 32 字节随机数的 base64url（43 个字符）。
func newSecret() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("auth: 生成随机数：%w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func hashOf(cookie string) string {
	h := sha256.Sum256([]byte(cookie))
	return hex.EncodeToString(h[:])
}

// Token 是当前的本机 token。
func (a *Auth) Token() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.token
}

// VerifyToken 用常量时间比较。
func (a *Auth) VerifyToken(t string) bool {
	a.mu.Lock()
	cur := a.token
	a.mu.Unlock()
	return t != "" && subtle.ConstantTimeCompare([]byte(t), []byte(cur)) == 1
}

// Rotate 生成新 token、作废全部会话，返回新 token。
func (a *Auth) Rotate(ctx context.Context) (string, error) {
	tok, err := newSecret()
	if err != nil {
		return "", err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := writeToken(a.d, tok); err != nil {
		return "", err
	}
	a.token = tok
	a.sessions = map[string]time.Time{}
	if err := a.save(ctx); err != nil {
		return "", err
	}
	return tok, nil
}

// NewSession 发一个新的会话 cookie 值，30 天后过期。
func (a *Auth) NewSession(ctx context.Context) (string, time.Time, error) {
	cookie, err := newSecret()
	if err != nil {
		return "", time.Time{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now()
	exp := now.Add(SessionTTL).UTC().Truncate(time.Millisecond)
	a.prune(now)
	a.sessions[hashOf(cookie)] = exp
	for len(a.sessions) > a.max {
		a.dropEarliest()
	}
	if err := a.save(ctx); err != nil {
		return "", time.Time{}, err
	}
	return cookie, exp, nil
}

// VerifySession 判断 cookie 值是否有效（未过期）；不写 KV。
func (a *Auth) VerifySession(cookie string) bool {
	if cookie == "" {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	exp, ok := a.sessions[hashOf(cookie)]
	return ok && exp.After(a.now())
}

// prune 去掉已过期的会话，返回去掉了几个。
func (a *Auth) prune(now time.Time) int {
	n := 0
	for h, exp := range a.sessions {
		if !exp.After(now) {
			delete(a.sessions, h)
			n++
		}
	}
	return n
}

func (a *Auth) dropEarliest() {
	keys := make([]string, 0, len(a.sessions))
	for h := range a.sessions {
		keys = append(keys, h)
	}
	sort.Slice(keys, func(i, j int) bool {
		ei, ej := a.sessions[keys[i]], a.sessions[keys[j]]
		if !ei.Equal(ej) {
			return ei.Before(ej)
		}
		return keys[i] < keys[j]
	})
	delete(a.sessions, keys[0])
}

func (a *Auth) load(ctx context.Context) error {
	raw, ok, err := a.kv.Get(ctx, SessionsKey)
	if err != nil {
		return fmt.Errorf("auth: 读会话表：%w", err)
	}
	if !ok {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("auth: 会话表格式不对：%w", err)
	}
	for h, s := range m {
		exp, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			continue
		}
		a.sessions[h] = exp.UTC()
	}
	if a.prune(a.now()) > 0 || len(a.sessions) != len(m) {
		return a.save(ctx)
	}
	return nil
}

func (a *Auth) save(ctx context.Context) error {
	m := make(map[string]string, len(a.sessions))
	for h, exp := range a.sessions {
		m[h] = exp.UTC().Format(tsLayout)
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("auth: 编码会话表：%w", err)
	}
	if err := a.kv.Put(ctx, SessionsKey, raw, a.now()); err != nil {
		return fmt.Errorf("auth: 写会话表：%w", err)
	}
	return nil
}
