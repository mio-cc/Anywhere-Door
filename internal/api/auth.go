package api

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mio-cc/anywhere-door/gen"
	"github.com/mio-cc/anywhere-door/internal/platform/auth"
)

// Caller.Via 的取值。
const (
	ViaCookie = "cookie" // 浏览器里的用户
	ViaBearer = "bearer" // anydoorctl（S.02-R4）
)

// Caller 是发出请求的一方（B.03.05 §5 第 3 条）。
type Caller struct {
	Actor gen.Actor
	Via   string
}

// CallerFrom 取鉴权中间件放进 context 的调用方；不鉴权的路由上没有。
func CallerFrom(ctx context.Context) (Caller, bool) {
	c, ok := ctx.Value(keyCaller).(Caller)
	return c, ok
}

// IsUser：只有用户能做的接口用它判断。cookie 的用户，或 Bearer 的 anydoorctl（命令行代表用户本人，S.09-R2）；
// 其他 external 调用方没有这个权利。
func IsUser(c Caller) bool {
	switch {
	case c.Via == ViaCookie && c.Actor.Kind == gen.ActorKindUser:
		return true
	case c.Via == ViaBearer && c.Actor.Kind == gen.ActorKindExternal && c.Actor.ID == "anydoorctl":
		return true
	}
	return false
}

// callerOf：cookie 有效是用户；否则 Bearer 有效是 anydoorctl；两者都有时以 cookie 为准。
func (s *Server) callerOf(r *http.Request) (Caller, bool) {
	if ck, err := r.Cookie(auth.CookieName); err == nil && s.auth.VerifySession(ck.Value) {
		return Caller{Actor: gen.Actor{Kind: gen.ActorKindUser, ID: "local"}, Via: ViaCookie}, true
	}
	if s.bearerOK(r) {
		return Caller{Actor: gen.Actor{Kind: gen.ActorKindExternal, ID: "anydoorctl"}, Via: ViaBearer}, true
	}
	return Caller{}, false
}

// bearerOK：Authorization: Bearer <token>，常量时间比较（auth.VerifyToken）。
func (s *Server) bearerOK(r *http.Request) bool {
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	return ok && s.auth.VerifyToken(strings.TrimSpace(tok))
}

// handleBootstrap：GET /?t=<token> 换会话 cookie，再 302 到 /，URL 里不再有 token（S.02-R3）。
func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	if !s.auth.VerifyToken(r.URL.Query().Get("t")) {
		logFrom(r.Context()).Warn("api: 引导用的 token 不对")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, "访问令牌无效。请重新运行 Anywhere Door。\n")
		return
	}
	v, _, err := s.auth.NewSession(r.Context())
	if err != nil {
		writeError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName,
		Value:    v,
		Path:     "/",
		MaxAge:   int(auth.SessionTTL / time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, "/", http.StatusFound)
}
