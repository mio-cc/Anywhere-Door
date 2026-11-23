package auth

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mio-cc/anywhere-door/internal/platform/datadir"
)

// memKV 是测试用的键值仓库（不 import engine/store）。
type memKV struct{ m map[string]json.RawMessage }

func newKV() *memKV { return &memKV{m: map[string]json.RawMessage{}} }

func (k *memKV) Get(_ context.Context, key string) (json.RawMessage, bool, error) {
	v, ok := k.m[key]
	return append(json.RawMessage{}, v...), ok, nil
}

func (k *memKV) Put(_ context.Context, key string, value json.RawMessage, _ time.Time) error {
	k.m[key] = append(json.RawMessage{}, value...)
	return nil
}

// clock 是可以拨动的假时钟。
type clock struct{ t time.Time }

func (c *clock) now() time.Time      { return c.t }
func (c *clock) add(d time.Duration) { c.t = c.t.Add(d) }
func newClock() *clock               { return &clock{t: time.Date(2026, 11, 24, 8, 0, 0, 0, time.UTC)} }
func openDir(t *testing.T) datadir.Dir {
	t.Helper()
	d, err := datadir.Open(t.TempDir())
	must(t, err)
	return d
}
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

var ctx = context.Background()

func TestToken(t *testing.T) {
	t.Run("B0701-T1 生成与读取", func(t *testing.T) {
		d := openDir(t)
		a, err := Open(ctx, d, newKV(), Options{})
		must(t, err)
		tok := a.Token()
		if len(tok) != 43 {
			t.Errorf("token 应当是 43 个字符，是 %d", len(tok))
		}
		b, err := os.ReadFile(d.Token())
		must(t, err)
		if string(b) != tok+"\n" {
			t.Errorf("文件内容是 %q", b)
		}
		a2, err := Open(ctx, d, newKV(), Options{})
		must(t, err)
		if a2.Token() != tok {
			t.Error("再次打开应当读出同一个 token")
		}
		if r, err := ReadToken(d); err != nil || r != tok {
			t.Errorf("ReadToken 得到 %q %v", r, err)
		}
		must(t, os.WriteFile(d.Token(), []byte("不是 token\n"), 0o600))
		if _, err := Open(ctx, d, newKV(), Options{}); err == nil || !strings.Contains(err.Error(), "不是有效的 token") {
			t.Errorf("乱码应当返回错误：%v", err)
		}
		if _, err := ReadToken(openDir(t)); err == nil {
			t.Error("没有 token 文件时 ReadToken 应当返回错误")
		}
	})
	t.Run("B0701-T2 轮换", func(t *testing.T) {
		d, kv := openDir(t), newKV()
		a, err := Open(ctx, d, kv, Options{})
		must(t, err)
		old := a.Token()
		cookie, _, err := a.NewSession(ctx)
		must(t, err)
		nt, err := a.Rotate(ctx)
		must(t, err)
		if a.VerifyToken(old) || !a.VerifyToken(nt) || nt == old {
			t.Error("轮换后旧 token 应当失效、新 token 有效")
		}
		if a.VerifySession(cookie) {
			t.Error("轮换前发的 cookie 应当失效")
		}
		a2, err := Open(ctx, d, kv, Options{})
		must(t, err)
		if a2.VerifySession(cookie) || a2.Token() != nt {
			t.Error("重新打开后旧 cookie 仍应失效，token 是新的")
		}
	})
	t.Run("B0701-T3 文件权限", func(t *testing.T) {
		d := openDir(t)
		_, err := Open(ctx, d, newKV(), Options{})
		must(t, err)
		if ok, err := datadir.IsPrivate(d.Token()); err != nil || !ok {
			t.Errorf("token 文件应当只有当前用户能访问：%v %v", ok, err)
		}
	})
}

func TestSession(t *testing.T) {
	t.Run("B0701-T4 会话过期与持久化", func(t *testing.T) {
		d, kv, c := openDir(t), newKV(), newClock()
		a, err := Open(ctx, d, kv, Options{Now: c.now, MaxSessions: 3})
		must(t, err)
		first, exp, err := a.NewSession(ctx)
		must(t, err)
		if !exp.Equal(c.t.Add(30 * 24 * time.Hour)) {
			t.Errorf("过期时间是 %v，应当 30 天后", exp)
		}
		c.add(29 * 24 * time.Hour)
		if !a.VerifySession(first) {
			t.Error("29 天时应当有效")
		}
		second, _, err := a.NewSession(ctx)
		must(t, err)
		c.add(24*time.Hour + time.Millisecond)
		if a.VerifySession(first) {
			t.Error("30 天加 1 毫秒时应当无效")
		}
		if !a.VerifySession(second) {
			t.Error("第二个会话应当仍然有效")
		}
		raw := string(kv.m[SessionsKey])
		if strings.Contains(raw, first) || strings.Contains(raw, second) {
			t.Error("KV 里不应当有 cookie 原值")
		}
		if !strings.Contains(raw, hashOf(first)) {
			t.Error("重新打开之前 KV 里还有第一个会话的哈希")
		}
		a2, err := Open(ctx, d, kv, Options{Now: c.now, MaxSessions: 3})
		must(t, err)
		if !a2.VerifySession(second) || a2.VerifySession(first) {
			t.Error("重新打开后未过期的有效、过期的无效")
		}
		if strings.Contains(string(kv.m[SessionsKey]), hashOf(first)) {
			t.Error("过期的会话应当已从 KV 里删掉")
		}

		var cookies []string
		for i := 0; i < 4; i++ {
			ck, _, err := a2.NewSession(ctx)
			must(t, err)
			cookies = append(cookies, ck)
			c.add(time.Second)
		}
		if a2.VerifySession(second) {
			t.Error("超过 MaxSessions 时最早过期的应当被挤掉")
		}
		for i, ck := range cookies[1:] {
			if !a2.VerifySession(ck) {
				t.Errorf("第 %d 个新会话应当有效", i+2)
			}
		}
	})
	t.Run("B0701-T5 错误输入", func(t *testing.T) {
		a, err := Open(ctx, openDir(t), newKV(), Options{})
		must(t, err)
		tok := a.Token()
		flipped := []byte(tok)
		if flipped[0] == 'A' {
			flipped[0] = 'B'
		} else {
			flipped[0] = 'A'
		}
		for _, bad := range []string{"", tok[:42], tok + "x", string(flipped)} {
			if a.VerifyToken(bad) {
				t.Errorf("VerifyToken(%q) 应当是 false", bad)
			}
		}
		for _, bad := range []string{"", "乱码", tok} {
			if a.VerifySession(bad) {
				t.Errorf("VerifySession(%q) 应当是 false", bad)
			}
		}
	})
}
