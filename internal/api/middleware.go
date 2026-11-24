package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

type ctxKey int

const (
	keyRequestID ctxKey = iota
	keyLogger
	keyCaller
)

// 请求体上限（B.03.02）和 /api/ 普通请求的处理时限（B.03 §3）。
const (
	maxBody      = 1 << 20
	maxBodyText  = "1MB"
	requestLimit = 30 * time.Second
)

// chain 按 B.03 §3 的顺序组装中间件（M0 不含幂等键）：
// recover → request_id → 安全响应头 → Host → Origin → 鉴权 → 请求体大小与时限 → 处理函数。
// recover 和 request_id 写在最外层同一个函数里：先生成 ID 再 defer recover，panic 时的 500 也带 request_id。
func (s *Server) chain(h http.Handler) http.Handler {
	h = s.limits(h)
	h = s.authenticate(h)
	h = s.checkOrigin(h)
	h = s.checkHost(h)
	h = s.securityHeaders(h)
	return s.recoverWithID(h)
}

func (s *Server) recoverWithID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := ulid.Make().String()
		log := s.log.With("request_id", id)
		ctx := context.WithValue(r.Context(), keyRequestID, id)
		ctx = context.WithValue(ctx, keyLogger, log)
		r = r.WithContext(ctx)
		w.Header().Set("X-Request-Id", id)
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if v == http.ErrAbortHandler {
				panic(v)
			}
			log.Error("api: 处理请求时 panic", "method", r.Method, "path", r.URL.Path,
				"panic", fmt.Sprint(v), "stack", string(debug.Stack()))
			setSecurityHeaders(w.Header(), s.dev)
			writeError(w, r, errPanic)
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w.Header(), s.dev)
		next.ServeHTTP(w, r)
	})
}

// checkHost：Host 只能是本机的三种写法加上实际端口（S.02-R5，防 DNS 重绑定）。
func (s *Server) checkHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hosts[strings.ToLower(r.Host)] {
			logFrom(r.Context()).Warn("api: Host 不是本机，已拒绝", "host", r.Host)
			writeError(w, r, errOrigin)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// checkOrigin：非 GET 请求和 SSE 请求的 Origin 必须是本机；缺失时只允许带有效 Bearer 的请求（S.02-R6）。
func (s *Server) checkOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || isSSE(r) {
			o := r.Header.Get("Origin")
			ok := s.origins[strings.ToLower(o)] || (o == "" && s.bearerOK(r))
			if !ok {
				logFrom(r.Context()).Warn("api: Origin 不是本机，已拒绝", "origin", o, "method", r.Method, "path", r.URL.Path)
				writeError(w, r, errOrigin)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// authenticate：/api/ 下除不鉴权的路由外，都要有效的 cookie 或 Bearer（S.02-R4）。
func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") || s.isPublic(r) {
			next.ServeHTTP(w, r)
			return
		}
		c, ok := s.callerOf(r)
		if !ok {
			writeError(w, r, errUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), keyCaller, c)))
	})
}

// limits：请求体不超过 1MB（B.03.02）；/api/ 的非 SSE 请求 30 秒时限（B.03 §3）。
// 时限只对 /api/：开发模式代理的热更新 WebSocket 不能被掐断。
func (s *Server) limits(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > maxBody {
			writeError(w, r, errTooLarge)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		if strings.HasPrefix(r.URL.Path, "/api/") && !isSSE(r) {
			ctx, cancel := context.WithTimeout(r.Context(), requestLimit)
			defer cancel()
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

func isSSE(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/event-stream")
}

func requestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(keyRequestID).(string)
	return id
}

var discard = slog.New(slog.DiscardHandler)

func logFrom(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(keyLogger).(*slog.Logger); ok {
		return l
	}
	return discard
}
