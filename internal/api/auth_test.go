package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/mio-cc/anywhere-door/gen"
)

func TestAuth(t *testing.T) {
	t.Run("B0305-T1 Origin", func(t *testing.T) {
		f := newFixture(t, false)
		cookie := f.login()
		rec := f.do(http.MethodPost, "/api/v1/test/echo", []byte("x"), withCookie(cookie), withOrigin("http://evil.example"))
		wantStatus(t, rec, http.StatusForbidden, "别的网站的 Origin")
		if e := errOf(t, rec); e.Code != gen.ErrCodeOriginRejected {
			t.Errorf("错误码是 %s", e.Code)
		}
		for _, o := range []string{"http://127.0.0.1:7311", "http://localhost:7311", "http://[::1]:7311"} {
			wantStatus(t, f.do(http.MethodPost, "/api/v1/test/echo", []byte("x"), withCookie(cookie), withOrigin(o)), http.StatusOK, o)
		}
		wantStatus(t, f.do(http.MethodPost, "/api/v1/test/echo", []byte("x"), withCookie(cookie), withOrigin("http://127.0.0.1:9999")),
			http.StatusForbidden, "端口不对的 Origin")
		wantStatus(t, f.do(http.MethodPost, "/api/v1/test/echo", []byte("x"), withCookie(cookie)), http.StatusForbidden, "没有 Origin、只有 cookie")
		wantStatus(t, f.do(http.MethodPost, "/api/v1/test/echo", []byte("x"), withBearer(f.a.Token())), http.StatusOK, "没有 Origin、Bearer 有效")
		wantStatus(t, f.do(http.MethodPost, "/api/v1/test/echo", []byte("x"), withBearer("错的")), http.StatusForbidden, "没有 Origin、Bearer 无效")
		wantStatus(t, f.do(http.MethodGet, "/api/v1/test/ping", nil, withCookie(cookie), withOrigin("http://evil.example")), http.StatusOK, "GET 不查 Origin")
		sse := f.do(http.MethodGet, "/api/v1/test/ping", nil, withCookie(cookie), withOrigin("http://evil.example"),
			func(r *http.Request) { r.Header.Set("Accept", "text/event-stream") })
		wantStatus(t, sse, http.StatusForbidden, "SSE 要查 Origin")
	})

	t.Run("B0305-T2 Host 重绑定", func(t *testing.T) {
		f := newFixture(t, false)
		cookie := f.login()
		for _, h := range []string{"evil.example:7311", "127.0.0.1:9999", "127.0.0.1", "192.168.1.2:7311"} {
			rec := f.do(http.MethodGet, "/api/v1/test/ping", nil, withCookie(cookie), withHost(h))
			wantStatus(t, rec, http.StatusForbidden, h)
			if e := errOf(t, rec); e.Code != gen.ErrCodeOriginRejected {
				t.Errorf("%s：错误码是 %s", h, e.Code)
			}
		}
		wantStatus(t, f.do(http.MethodGet, "/api/v1/test/open", nil, withHost("evil.example:7311")), http.StatusForbidden, "不鉴权的路由也查 Host")
		for _, h := range []string{"127.0.0.1:7311", "localhost:7311", "LOCALHOST:7311", "[::1]:7311"} {
			wantStatus(t, f.do(http.MethodGet, "/api/v1/test/ping", nil, withCookie(cookie), withHost(h)), http.StatusOK, h)
		}
	})

	t.Run("B0305-T3 无 cookie", func(t *testing.T) {
		f := newFixture(t, false)
		recs := map[string][]reqOpt{
			"没有凭证":       nil,
			"cookie 是乱码": {withCookie("not-a-session")},
			"Bearer 错":   {withBearer("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")},
		}
		var first *errorPayload
		for name, opts := range recs {
			rec := f.do(http.MethodGet, "/api/v1/test/ping", nil, opts...)
			wantStatus(t, rec, http.StatusUnauthorized, name)
			e := errOf(t, rec)
			if e.Code != gen.ErrCodeUnauthorized {
				t.Errorf("%s：错误码是 %s", name, e.Code)
			}
			e.RequestID = ""
			if first == nil {
				first = &e
			} else if e.Code != first.Code || e.Message != first.Message || e.Details != nil || first.Details != nil {
				t.Errorf("%s：响应体和别的情况不同（不应当说明原因）：%+v / %+v", name, e, *first)
			}
		}
		wantStatus(t, f.do(http.MethodGet, "/api/v1/nope", nil), http.StatusUnauthorized, "不存在的接口没有凭证时也是 401")
		wantStatus(t, f.do(http.MethodGet, "/api/v1/test/open", nil), http.StatusOK, "不鉴权的路由")
	})

	t.Run("B0305-T4 token 轮换", func(t *testing.T) {
		f := newFixture(t, false)
		old := f.a.Token()
		rec := f.do(http.MethodGet, "/?t="+url.QueryEscape(old), nil)
		wantStatus(t, rec, http.StatusFound, "引导")
		if loc := rec.Header().Get("Location"); loc != "/" {
			t.Errorf("应当跳转到 /，是 %q", loc)
		}
		cookies := rec.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("应当发一个 cookie，发了 %d 个", len(cookies))
		}
		c := cookies[0]
		if c.Name != "hx_session" || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.MaxAge != 30*24*3600 || c.Secure {
			t.Errorf("cookie 的属性不对：%+v", c)
		}
		if strings.Contains(rec.Header().Get("Set-Cookie"), old) {
			t.Error("cookie 里不能有 token")
		}
		wantStatus(t, f.do(http.MethodGet, "/api/v1/test/ping", nil, withCookie(c.Value)), http.StatusOK, "用新 cookie")
		if _, err := f.a.Rotate(ctx); err != nil {
			t.Fatal(err)
		}
		wantStatus(t, f.do(http.MethodGet, "/api/v1/test/ping", nil, withCookie(c.Value)), http.StatusUnauthorized, "轮换后旧 cookie")
		for name, tok := range map[string]string{"轮换前的 token": old, "错的 token": "abc", "空 token": ""} {
			rec := f.do(http.MethodGet, "/?t="+url.QueryEscape(tok), nil)
			wantStatus(t, rec, http.StatusUnauthorized, name)
			if v := rec.Header().Values("Set-Cookie"); len(v) != 0 {
				t.Errorf("%s：不应当发 cookie：%v", name, v)
			}
		}
		wantStatus(t, f.do(http.MethodGet, "/?t="+url.QueryEscape(f.a.Token()), nil), http.StatusFound, "新 token 引导")
		if body := f.do(http.MethodGet, "/", nil).Body.String(); body != "static" {
			t.Errorf("不带 t 的 / 应当交给 Static，得到 %q", body)
		}
	})

	t.Run("B0305-T6 Caller", func(t *testing.T) {
		f := newFixture(t, false)
		cookie := f.login()
		who := func(opts ...reqOpt) map[string]any {
			t.Helper()
			rec := f.do(http.MethodGet, "/api/v1/test/whoami", nil, opts...)
			wantStatus(t, rec, http.StatusOK, "whoami")
			var m map[string]any
			must(t, json.Unmarshal(rec.Body.Bytes(), &m))
			return m
		}
		check := func(name string, m map[string]any, kind, id, via string) {
			t.Helper()
			if m["kind"] != kind || m["id"] != id || m["via"] != via || m["user"] != true || m["found"] != true {
				t.Errorf("%s：得到 %v", name, m)
			}
		}
		check("cookie", who(withCookie(cookie)), "user", "local", ViaCookie)
		check("Bearer", who(withBearer(f.a.Token())), "external", "anydoorctl", ViaBearer)
		check("两者都有", who(withCookie(cookie), withBearer(f.a.Token())), "user", "local", ViaCookie)
		// 两种凭证各自判断：cookie 无效时照常看 Bearer（anydoorctl 不带 cookie；浏览器不带 Bearer）
		check("坏 cookie 加有效 Bearer", who(withCookie("not-a-session"), withBearer(f.a.Token())), "external", "anydoorctl", ViaBearer)
		wantStatus(t, f.do(http.MethodGet, "/api/v1/test/whoami", nil, withCookie(cookie), withBearer("错的")), http.StatusOK, "有效 cookie 加坏 Bearer")
		if IsUser(Caller{Actor: gen.Actor{Kind: gen.ActorKindExternal, ID: "other"}, Via: ViaBearer}) {
			t.Error("其他 external 调用方不是用户")
		}
		if IsUser(Caller{Actor: gen.Actor{Kind: gen.ActorKindModel, ID: "m"}, Via: ViaCookie}) || IsUser(Caller{}) {
			t.Error("模型和空调用方不是用户")
		}
		if _, ok := CallerFrom(ctx); ok {
			t.Error("没有经过鉴权的 context 里不应当有调用方")
		}
	})
}
