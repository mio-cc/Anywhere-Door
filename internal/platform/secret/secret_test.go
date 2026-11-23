package secret

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/mio-cc/anywhere-door/internal/platform/datadir"
)

var ctx = context.Background()

// fakeKeyring 是测试用的系统密钥库；mode 控制探测时的故障。
type fakeKeyring struct {
	m    map[string]string
	mode string // "" | "set-fails" | "get-fails" | "get-differs"
}

func newFake(mode string) *fakeKeyring { return &fakeKeyring{m: map[string]string{}, mode: mode} }

func (f *fakeKeyring) Get(service, user string) (string, error) {
	if f.mode == "get-fails" {
		return "", errors.New("没有桌面会话")
	}
	v, ok := f.m[service+"/"+user]
	if !ok {
		return "", keyring.ErrNotFound
	}
	if f.mode == "get-differs" {
		return v + "x", nil
	}
	return v, nil
}

func (f *fakeKeyring) Set(service, user, password string) error {
	if f.mode == "set-fails" {
		return errors.New("没有桌面会话")
	}
	f.m[service+"/"+user] = password
	return nil
}

func (f *fakeKeyring) Delete(service, user string) error {
	if _, ok := f.m[service+"/"+user]; !ok {
		return keyring.ErrNotFound
	}
	delete(f.m, service+"/"+user)
	return nil
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func isErr(t *testing.T, what string, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Errorf("%s：错误是 %v，应当是 %v", what, err, want)
	}
}

func openDir(t *testing.T) datadir.Dir {
	t.Helper()
	d, err := datadir.Open(t.TempDir())
	must(t, err)
	return d
}

func randomValue(t *testing.T) string {
	t.Helper()
	b := make([]byte, 20)
	_, err := rand.Read(b)
	must(t, err)
	return hex.EncodeToString(b) // 40 个字符
}

// roundTrip 在 s 上做一遍 Put / Resolve / List / Delete。
func roundTrip(t *testing.T, s *Store, prefix string) {
	t.Helper()
	a, b := prefix+"openai_main", prefix+"short"
	long := "sk-proj-1234567890abcdef"
	must(t, s.Put(ctx, a, long))
	must(t, s.Put(ctx, b, "abc"))
	if v, err := s.Resolve(ctx, a); err != nil || v != long {
		t.Errorf("Resolve 得到 %q %v", v, err)
	}
	list, err := s.List(ctx)
	must(t, err)
	got := map[string]string{}
	for _, it := range list {
		got[it.Name] = it.Last4
	}
	if got[a] != "cdef" || got[b] != "" {
		t.Errorf("List 的 last4 不对：%v", got)
	}
	must(t, s.Delete(ctx, a))
	must(t, s.Delete(ctx, a))
	_, err = s.Resolve(ctx, a)
	isErr(t, "删除后 Resolve", err, ErrNotFound)
	must(t, s.Delete(ctx, b))
	list, err = s.List(ctx)
	must(t, err)
	for _, it := range list {
		if strings.HasPrefix(it.Name, prefix) {
			t.Errorf("删除后 List 里还有 %s", it.Name)
		}
	}
}

func TestKeyring(t *testing.T) {
	t.Run("B0702-T1 keyring", func(t *testing.T) {
		fk := newFake("")
		s, err := Open(ctx, openDir(t), Options{Keyring: fk})
		must(t, err)
		if s.Backend() != "keyring" || s.Locked() {
			t.Fatalf("后端是 %s，锁定 %v", s.Backend(), s.Locked())
		}
		roundTrip(t, s, "")
		for _, bad := range []string{"", "Upper", "1abc", "has-dash", "a" + strings.Repeat("b", 64)} {
			isErr(t, "名字 "+bad, s.Put(ctx, bad, "v"), ErrBadName)
			_, err := s.Resolve(ctx, bad)
			isErr(t, "Resolve "+bad, err, ErrBadName)
		}
		_, err = s.Resolve(ctx, "missing")
		isErr(t, "不存在的名字", err, ErrNotFound)
		if len(fk.m) != 0 {
			t.Errorf("假密钥库里应当什么都不剩：%v", fk.m)
		}
		if os.Getenv("ANYDOOR_KEYRING_TEST") == "1" {
			sys, err := Open(ctx, openDir(t), Options{})
			must(t, err)
			if sys.Backend() != "keyring" {
				t.Fatalf("真实系统密钥库不可用，后端是 %s", sys.Backend())
			}
			roundTrip(t, sys, "anydoor_test_")
		}
	})
	t.Run("B0702-T2 回退", func(t *testing.T) {
		for _, mode := range []string{"set-fails", "get-fails", "get-differs"} {
			fk := newFake(mode)
			s, err := Open(ctx, openDir(t), Options{Keyring: fk})
			must(t, err)
			if s.Backend() != "file" || !s.Locked() {
				t.Errorf("%s：应当改用文件后端且锁定，是 %s", mode, s.Backend())
			}
			if len(fk.m) != 0 {
				t.Errorf("%s：探测条目留在了密钥库里：%v", mode, fk.m)
			}
		}
	})
}

func TestFile(t *testing.T) {
	t.Run("B0702-T3 加密文件", func(t *testing.T) {
		d := openDir(t)
		s, err := Open(ctx, d, Options{ForceFile: true})
		must(t, err)
		if !s.Locked() || s.Backend() != "file" {
			t.Fatal("新开的文件后端应当锁定")
		}
		_, err = s.Resolve(ctx, "openai_main")
		isErr(t, "未解锁 Resolve", err, ErrLocked)
		isErr(t, "未解锁 Put", s.Put(ctx, "openai_main", "v"), ErrLocked)
		if _, err := s.List(ctx); err != nil {
			t.Errorf("锁定时 List 也应当能列：%v", err)
		}
		if err := s.Unlock(ctx, ""); err == nil {
			t.Error("首次设置空密码应当返回错误")
		}
		must(t, s.Unlock(ctx, "correct horse"))
		if s.Locked() {
			t.Fatal("解锁后不应当锁定")
		}
		roundTrip(t, s, "")
		must(t, s.Put(ctx, "keep", "value-for-reopen"))
		first, err := os.ReadFile(d.Secrets())
		must(t, err)
		must(t, s.Put(ctx, "keep2", "another-value"))
		second, err := os.ReadFile(d.Secrets())
		must(t, err)
		var doc1, doc2 fileDoc
		must(t, json.Unmarshal(first, &doc1))
		must(t, json.Unmarshal(second, &doc2))
		if doc1.Nonce == doc2.Nonce {
			t.Error("两次写入的 nonce 应当不同")
		}
		if k := doc2.KDF; k.Alg != "argon2id" || k.M != 65536 || k.T != 3 || k.P != 1 {
			t.Errorf("kdf 参数是 %+v", k)
		}

		s2, err := Open(ctx, d, Options{ForceFile: true})
		must(t, err)
		if !s2.Locked() {
			t.Error("重新打开后应当锁定")
		}
		isErr(t, "错误密码", s2.Unlock(ctx, "wrong"), ErrBadPassword)
		must(t, s2.Unlock(ctx, "correct horse"))
		if v, err := s2.Resolve(ctx, "keep"); err != nil || v != "value-for-reopen" {
			t.Errorf("重新打开后读到 %q %v", v, err)
		}

		data, err := base64.StdEncoding.DecodeString(doc2.Data)
		must(t, err)
		data[0] ^= 0xff
		doc2.Data = base64.StdEncoding.EncodeToString(data)
		bad, err := json.Marshal(doc2)
		must(t, err)
		must(t, os.WriteFile(d.Secrets(), bad, 0o600))
		s3, err := Open(ctx, d, Options{ForceFile: true})
		must(t, err)
		isErr(t, "损坏的文件", s3.Unlock(ctx, "correct horse"), ErrCorrupt)
	})
	t.Run("B0702-T4 暴力尝试限制", func(t *testing.T) {
		d := openDir(t)
		c := &clock{t: time.Date(2026, 11, 24, 8, 0, 0, 0, time.UTC)}
		s, err := Open(ctx, d, Options{ForceFile: true, Now: c.now})
		must(t, err)
		must(t, s.Unlock(ctx, "pw"))
		s, err = Open(ctx, d, Options{ForceFile: true, Now: c.now})
		must(t, err)
		for i := 0; i < 5; i++ {
			isErr(t, "错误密码", s.Unlock(ctx, "nope"), ErrBadPassword)
		}
		isErr(t, "冷却中", s.Unlock(ctx, "pw"), ErrTooManyAttempts)
		c.t = c.t.Add(29 * time.Second)
		isErr(t, "29 秒时仍在冷却", s.Unlock(ctx, "pw"), ErrTooManyAttempts)
		c.t = c.t.Add(time.Second)
		must(t, s.Unlock(ctx, "pw"))
		for i := 0; i < 4; i++ {
			isErr(t, "错误密码", s.Unlock(ctx, "nope"), ErrBadPassword)
		}
		must(t, s.Unlock(ctx, "pw"))
		isErr(t, "成功后计数清零", s.Unlock(ctx, "nope"), ErrBadPassword)
		must(t, s.Unlock(ctx, "pw"))
	})
	t.Run("B0702-T5 无明文残留", func(t *testing.T) {
		check := func(t *testing.T, d datadir.Dir, v string) {
			t.Helper()
			needles := []string{v, base64.StdEncoding.EncodeToString([]byte(v)), base64.RawURLEncoding.EncodeToString([]byte(v))}
			err := filepath.WalkDir(d.Root, func(p string, e os.DirEntry, err error) error {
				if err != nil || e.IsDir() {
					return err
				}
				b, err := os.ReadFile(p)
				if err != nil {
					return err
				}
				for _, n := range needles {
					if strings.Contains(string(b), n) {
						t.Errorf("%s 里有密钥原文", p)
					}
				}
				return nil
			})
			must(t, err)
		}
		d1 := openDir(t)
		s1, err := Open(ctx, d1, Options{Keyring: newFake("")})
		must(t, err)
		v1 := randomValue(t)
		must(t, s1.Put(ctx, "openai_main", v1))
		check(t, d1, v1)

		d2 := openDir(t)
		s2, err := Open(ctx, d2, Options{ForceFile: true})
		must(t, err)
		must(t, s2.Unlock(ctx, "pw"))
		v2 := randomValue(t)
		must(t, s2.Put(ctx, "openai_main", v2))
		check(t, d2, v2)
	})
	t.Run("B0702-T6 开发模式", func(t *testing.T) {
		env := map[string]string{"ANYDOOR_DEV_SECRET_OPENAI_MAIN": "from-env"}
		getenv := func(k string) string { return env[k] }
		fk := newFake("")
		dev, err := Open(ctx, openDir(t), Options{Keyring: fk, Dev: true, Getenv: getenv})
		must(t, err)
		if v, err := dev.Resolve(ctx, "openai_main"); err != nil || v != "from-env" {
			t.Errorf("开发模式应当用环境变量：%q %v", v, err)
		}
		prod, err := Open(ctx, openDir(t), Options{Keyring: newFake(""), Getenv: getenv})
		must(t, err)
		must(t, prod.Put(ctx, "openai_main", "from-store"))
		for i := 0; i < 2; i++ {
			if v, err := prod.Resolve(ctx, "openai_main"); err != nil || v != "from-store" {
				t.Errorf("正式模式应当忽略环境变量：%q %v", v, err)
			}
		}
	})
}
