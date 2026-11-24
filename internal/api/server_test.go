package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mio-cc/anywhere-door/internal/platform/auth"
	"github.com/mio-cc/anywhere-door/internal/platform/datadir"
)

const (
	testPort    = 7311
	testHost    = "127.0.0.1:7311"
	testVersion = "9.9.9-test"
	localOrigin = "http://127.0.0.1:7311"
	panicText   = "秘密的 panic 原文"
)

var ctx = context.Background()

// memKV 是测试用的会话存放处（不 import engine/store）。
type memKV struct {
	mu sync.Mutex
	m  map[string]json.RawMessage
}

func newKV() *memKV { return &memKV{m: map[string]json.RawMessage{}} }

func (k *memKV) Get(_ context.Context, key string) (json.RawMessage, bool, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	v, ok := k.m[key]
	return append(json.RawMessage{}, v...), ok, nil
}

func (k *memKV) Put(_ context.Context, key string, value json.RawMessage, _ time.Time) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.m[key] = append(json.RawMessage{}, value...)
	return nil
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// fixture 是一个在临时数据目录上、带几条测试路由的 Server。
type fixture struct {
	t     *testing.T
	s     *Server
	a     *auth.Auth
	h     http.Handler
	stops atomic.Int32
}

func newFixture(t *testing.T, dev bool) *fixture {
	t.Helper()
	d, err := datadir.Open(t.TempDir())
	must(t, err)
	a, err := auth.Open(ctx, d, newKV(), auth.Options{})
	must(t, err)
	f := &fixture{t: t, a: a}
	static := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "static") })
	s, err := NewServer(Deps{Auth: a, Port: testPort, Version: testVersion, Dev: dev, Static: static,
		Shutdown: func() { f.stops.Add(1) }})
	must(t, err)
	s.handle("GET /api/v1/test/ping", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	s.handlePublic("GET /api/v1/test/open", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"open": true})
	})
	s.handle("GET /api/v1/test/whoami", func(w http.ResponseWriter, r *http.Request) {
		c, found := CallerFrom(r.Context())
		writeJSON(w, http.StatusOK, map[string]any{"kind": c.Actor.Kind, "id": c.Actor.ID, "via": c.Via, "user": IsUser(c), "found": found})
	})
	s.handle("POST /api/v1/test/echo", func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]int{"len": len(b)})
	})
	s.handle("GET /api/v1/test/bad", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, &badRequestError{errs: []FieldError{{Path: "/name", Message: "缺少 name"}, {Path: "/limit", Message: "limit 太大"}}})
	})
	s.handle("GET /api/v1/test/forbidden", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, &forbiddenError{action: "GET /api/v1/test/forbidden", target: "anydoor", reasons: []string{"只有用户能做", "测试"}})
	})
	s.handle("GET /api/v1/test/panic", func(http.ResponseWriter, *http.Request) { panic(panicText) })
	f.s, f.h = s, s.Handler()
	return f
}

type reqOpt func(*http.Request)

func withCookie(v string) reqOpt {
	return func(r *http.Request) { r.AddCookie(&http.Cookie{Name: auth.CookieName, Value: v}) }
}
func withBearer(tok string) reqOpt {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+tok) }
}
func withOrigin(o string) reqOpt { return func(r *http.Request) { r.Header.Set("Origin", o) } }
func withHost(h string) reqOpt   { return func(r *http.Request) { r.Host = h } }

func (f *fixture) do(method, target string, body []byte, opts ...reqOpt) *httptest.ResponseRecorder {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	r := httptest.NewRequest(method, target, rd)
	r.Host = testHost
	for _, o := range opts {
		o(r)
	}
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, r)
	return rec
}

// login 走一次引导，返回会话 cookie 的值。
func (f *fixture) login() string {
	f.t.Helper()
	rec := f.do(http.MethodGet, "/?t="+f.a.Token(), nil)
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.CookieName {
			return c.Value
		}
	}
	f.t.Fatalf("引导没有发 cookie：%d %s", rec.Code, rec.Body.String())
	return ""
}

// errOf 解析错误响应。
func errOf(t *testing.T, rec *httptest.ResponseRecorder) errorPayload {
	t.Helper()
	var b errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("响应不是错误响应：%v：%s", err, rec.Body.String())
	}
	return b.Error
}

func wantStatus(t *testing.T, rec *httptest.ResponseRecorder, status int, what string) {
	t.Helper()
	if rec.Code != status {
		t.Errorf("%s：状态码 %d，应当是 %d（%s）", what, rec.Code, status, strings.TrimSpace(rec.Body.String()))
	}
}

func TestServer(t *testing.T) {
	t.Run("B0305-T7 安全响应头", func(t *testing.T) {
		f := newFixture(t, false)
		cookie := f.login()
		cases := []struct {
			name   string
			rec    *httptest.ResponseRecorder
			status int
		}{
			{"200", f.do(http.MethodGet, "/api/v1/test/ping", nil, withCookie(cookie)), http.StatusOK},
			{"302", f.do(http.MethodGet, "/?t="+f.a.Token(), nil), http.StatusFound},
			{"401", f.do(http.MethodGet, "/api/v1/test/ping", nil), http.StatusUnauthorized},
			{"403", f.do(http.MethodGet, "/api/v1/test/ping", nil, withCookie(cookie), withHost("evil.example:7311")), http.StatusForbidden},
			{"404", f.do(http.MethodGet, "/api/v1/nope", nil, withCookie(cookie)), http.StatusNotFound},
			{"413", f.do(http.MethodPost, "/api/v1/test/echo", make([]byte, maxBody+1), withCookie(cookie), withOrigin(localOrigin)), http.StatusRequestEntityTooLarge},
			{"500", f.do(http.MethodGet, "/api/v1/test/panic", nil, withCookie(cookie)), http.StatusInternalServerError},
		}
		want := map[string]string{
			"Content-Security-Policy":    csp,
			"X-Content-Type-Options":     "nosniff",
			"Referrer-Policy":            "no-referrer",
			"Cross-Origin-Opener-Policy": "same-origin",
		}
		for _, c := range cases {
			wantStatus(t, c.rec, c.status, c.name)
			for k, v := range want {
				if got := c.rec.Header().Get(k); got != v {
					t.Errorf("%s：%s 是 %q，应当是 %q", c.name, k, got, v)
				}
			}
			if id := c.rec.Header().Get("X-Request-Id"); len(id) != 26 {
				t.Errorf("%s：X-Request-Id 是 %q", c.name, id)
			}
			for k := range c.rec.Header() {
				if strings.HasPrefix(strings.ToLower(k), "access-control-allow") {
					t.Errorf("%s：不应当有 CORS 头 %s", c.name, k)
				}
			}
		}
		if got := f.do(http.MethodGet, "/api/v1/test/open", nil).Header().Get("Content-Security-Policy"); strings.Contains(got, "ws://") {
			t.Errorf("非开发模式的 CSP 不应当放行 ws：%s", got)
		}
		dev := newFixture(t, true)
		if got := dev.do(http.MethodGet, "/api/v1/test/open", nil).Header().Get("Content-Security-Policy"); !strings.Contains(got, "connect-src 'self' ws://127.0.0.1:5173") {
			t.Errorf("开发模式的 CSP 应当放行 Vite 的热更新：%s", got)
		}
	})

	t.Run("B0305-T9 请求体大小", func(t *testing.T) {
		f := newFixture(t, false)
		cookie := f.login()
		rec := f.do(http.MethodPost, "/api/v1/test/echo", make([]byte, maxBody), withCookie(cookie), withOrigin(localOrigin))
		wantStatus(t, rec, http.StatusOK, "正好 1MB")
		if !strings.Contains(rec.Body.String(), `"len":1048576`) {
			t.Errorf("回显长度不对：%s", rec.Body.String())
		}
		rec = f.do(http.MethodPost, "/api/v1/test/echo", make([]byte, maxBody+1), withCookie(cookie), withOrigin(localOrigin))
		wantStatus(t, rec, http.StatusRequestEntityTooLarge, "1MB 加 1 字节（有 Content-Length）")
		if e := errOf(t, rec); e.Code != "too_large" {
			t.Errorf("错误码是 %s", e.Code)
		}
		// 不带 Content-Length（分块传输）时由 MaxBytesReader 拦下
		r := httptest.NewRequest(http.MethodPost, "/api/v1/test/echo", bytes.NewReader(make([]byte, maxBody+1)))
		r.Host, r.ContentLength = testHost, -1
		r.Header.Set("Origin", localOrigin)
		r.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookie})
		rec = httptest.NewRecorder()
		f.h.ServeHTTP(rec, r)
		wantStatus(t, rec, http.StatusRequestEntityTooLarge, "1MB 加 1 字节（没有 Content-Length）")
		if e := errOf(t, rec); e.Code != "too_large" {
			t.Errorf("错误码是 %s", e.Code)
		}
	})
}
