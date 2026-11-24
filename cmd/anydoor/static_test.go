package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mio-cc/anywhere-door/internal/api"
	"github.com/mio-cc/anywhere-door/internal/platform/auth"
	"github.com/mio-cc/anywhere-door/internal/platform/datadir"
)

func get(h http.Handler, target string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

// memKV 是测试用的会话存放处。
type memKV struct {
	mu sync.Mutex
	m  map[string]json.RawMessage
}

func (k *memKV) Get(_ context.Context, key string) (json.RawMessage, bool, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	v, ok := k.m[key]
	return v, ok, nil
}

func (k *memKV) Put(_ context.Context, key string, value json.RawMessage, _ time.Time) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.m[key] = value
	return nil
}

func TestStatic(t *testing.T) {
	t.Run("B02-T3 前端回退", func(t *testing.T) {
		h := staticHandler(fstest.MapFS{
			"index.html":          {Data: []byte("<html>首页</html>")},
			"assets/app-1a2b.js":  {Data: []byte("console.log(1)")},
			"favicon.svg":         {Data: []byte("<svg/>")},
			"assets/sub/deep.css": {Data: []byte("body{}")},
		})
		rec := get(h, "/some/route")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "首页") || rec.Header().Get("Cache-Control") != "no-cache" {
			t.Errorf("前端路由应当回退到 index.html：%d %q %q", rec.Code, rec.Body.String(), rec.Header().Get("Cache-Control"))
		}
		if rec := get(h, "/"); !strings.Contains(rec.Body.String(), "首页") || rec.Header().Get("Cache-Control") != "no-cache" {
			t.Errorf("/ 应当是 index.html 且不缓存：%q", rec.Body.String())
		}
		rec = get(h, "/assets/app-1a2b.js")
		if rec.Body.String() != "console.log(1)" || rec.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
			t.Errorf("assets 应当带长缓存：%q %q", rec.Body.String(), rec.Header().Get("Cache-Control"))
		}
		if rec := get(h, "/favicon.svg"); rec.Body.String() != "<svg/>" || rec.Header().Get("Cache-Control") != "no-cache" {
			t.Errorf("assets 以外的文件不缓存：%q %q", rec.Body.String(), rec.Header().Get("Cache-Control"))
		}
		if rec := get(h, "/assets/x.js"); rec.Code != http.StatusNotFound {
			t.Errorf("不存在的 assets 文件应当 404，是 %d", rec.Code)
		}
		if rec := get(h, "/../index.html"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "首页") {
			t.Errorf("路径里的 .. 应当被清理：%d", rec.Code)
		}
	})

	t.Run("B02-T6 未构建前端", func(t *testing.T) {
		h := staticHandler(fstest.MapFS{".gitkeep": {Data: nil}})
		for _, p := range []string{"/", "/some/route", "/assets/app.js"} {
			if rec := get(h, p); !strings.Contains(rec.Body.String(), "make build") {
				t.Errorf("%s 应当是说明页：%q", p, rec.Body.String())
			}
		}
		d, err := datadir.Open(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		a, err := auth.Open(bg, d, &memKV{m: map[string]json.RawMessage{}}, auth.Options{})
		if err != nil {
			t.Fatal(err)
		}
		s, err := api.NewServer(api.Deps{Auth: a, Port: 7312, Version: "t", Static: h})
		if err != nil {
			t.Fatal(err)
		}
		for p, want := range map[string]string{"/api/v1/health": `"ok":true`, "/": "make build"} {
			r := httptest.NewRequest(http.MethodGet, p, nil)
			r.Host = "127.0.0.1:7312"
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, r)
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), want) {
				t.Errorf("%s：%d %s", p, rec.Code, rec.Body.String())
			}
		}
	})
}
