package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

func TestHealth(t *testing.T) {
	t.Run("B0414-T1 health 不泄露信息", func(t *testing.T) {
		f := newFixture(t, false)
		rec := f.do(http.MethodGet, "/api/v1/health", nil)
		wantStatus(t, rec, http.StatusOK, "不带凭证")
		var m map[string]any
		must(t, json.Unmarshal(rec.Body.Bytes(), &m))
		if len(m) != 2 || m["ok"] != true || m["version"] != testVersion {
			t.Errorf("health 应当恰好是 {ok: true, version}，得到 %v", m)
		}
		withAuth := f.do(http.MethodGet, "/api/v1/health", nil, withCookie(f.login()))
		if !bytes.Equal(withAuth.Body.Bytes(), rec.Body.Bytes()) {
			t.Errorf("带 cookie 时内容应当相同：%s / %s", withAuth.Body.String(), rec.Body.String())
		}
		wantStatus(t, f.do(http.MethodGet, "/api/v1/health", nil, withHost("evil.example:7311")), http.StatusForbidden, "Host 不对")
		post := f.do(http.MethodPost, "/api/v1/health", nil, withBearer(f.a.Token()))
		wantStatus(t, post, http.StatusMethodNotAllowed, "POST")
		if allow := post.Header().Get("Allow"); allow == "" {
			t.Error("405 应当带 Allow")
		}
	})

	t.Run("B0414-T3 shutdown", func(t *testing.T) {
		f := newFixture(t, false)
		cookie := f.login()
		rec := f.do(http.MethodPost, "/api/v1/admin/shutdown", nil, withCookie(cookie), withOrigin(localOrigin))
		wantStatus(t, rec, http.StatusAccepted, "cookie 用户")
		if b := bytes.TrimSpace(rec.Body.Bytes()); string(b) != `{"ok":true}` {
			t.Errorf("响应体是 %s", b)
		}
		wantStatus(t, f.do(http.MethodPost, "/api/v1/admin/shutdown", nil, withBearer(f.a.Token())), http.StatusAccepted, "Bearer")
		if n := f.stops.Load(); n != 1 {
			t.Errorf("Shutdown 应当一共只被调用一次，调用了 %d 次", n)
		}
		wantStatus(t, f.do(http.MethodPost, "/api/v1/admin/shutdown", nil, withOrigin(localOrigin)), http.StatusUnauthorized, "不带凭证")
		wantStatus(t, f.do(http.MethodGet, "/api/v1/admin/shutdown", nil, withCookie(cookie)), http.StatusMethodNotAllowed, "GET")

		g := newFixture(t, false)
		evil := g.do(http.MethodPost, "/api/v1/admin/shutdown", nil, withCookie(g.login()), withOrigin("http://evil.example"))
		wantStatus(t, evil, http.StatusForbidden, "别的网站的 Origin")
		if e := errOf(t, evil); e.Code != "origin_rejected" {
			t.Errorf("别的网站的 Origin 应当是 origin_rejected，是 %s", e.Code)
		}
		if n := g.stops.Load(); n != 0 {
			t.Errorf("被拒绝的请求不应当调用 Shutdown，调用了 %d 次", n)
		}
	})
}
