// Package api 是本机 HTTP 服务（B.03）：服务器骨架、中间件链、本机鉴权与错误响应（B.03.05），以及各个接口（B.04）。
package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"sync"

	"github.com/mio-cc/anywhere-door/internal/platform/auth"
)

// Deps 是 NewServer 需要的东西，由 cmd/anydoor 组装（B.02 §5）。
type Deps struct {
	Auth     *auth.Auth   // B.07.01
	Port     int          // 实际监听端口：Host / Origin 校验用（B.02 先监听再建 Server）
	Version  string       // health 返回（B.04.14）
	Dev      bool         // --dev：CSP 放宽（S.02-R9）
	Static   http.Handler // /api/ 以外的路径：嵌入的前端或 Vite 代理（B.02）；nil 时 404
	Shutdown func()       // POST /api/v1/admin/shutdown 调用（B.04.14）；只会被调用一次
	Logger   *slog.Logger // nil 时丢弃
}

// Server 是本机 HTTP 服务；Handler 可以并发使用。
type Server struct {
	auth     *auth.Auth
	version  string
	dev      bool
	static   http.Handler
	shutdown func()
	log      *slog.Logger
	hosts    map[string]bool // 允许的 Host（小写，S.02-R5）
	origins  map[string]bool // 允许的 Origin（小写，S.02-R6）
	mux      *http.ServeMux  // /api/ 下的路由（routes.go）
	public   map[string]bool // 不鉴权的路由模式（B-R3）
	handler  http.Handler
}

// NewServer 检查依赖、读入错误文案、注册路由、组好中间件链。
func NewServer(d Deps) (*Server, error) {
	if d.Auth == nil {
		return nil, errors.New("api: 缺少 Auth")
	}
	if d.Port < 1 || d.Port > 65535 {
		return nil, errors.New("api: 端口 " + strconv.Itoa(d.Port) + " 不在 1–65535 之间")
	}
	if _, err := loadMessages(); err != nil {
		return nil, err
	}
	if d.Logger == nil {
		d.Logger = slog.New(slog.DiscardHandler)
	}
	stop := d.Shutdown
	if stop == nil {
		stop = func() {}
	}
	s := &Server{
		auth:     d.Auth,
		version:  d.Version,
		dev:      d.Dev,
		static:   d.Static,
		shutdown: sync.OnceFunc(stop),
		log:      d.Logger,
		hosts:    map[string]bool{},
		origins:  map[string]bool{},
		mux:      http.NewServeMux(),
		public:   map[string]bool{},
	}
	p := strconv.Itoa(d.Port)
	for _, h := range []string{"127.0.0.1", "localhost", "[::1]"} {
		s.hosts[h+":"+p] = true
		s.origins["http://"+h+":"+p] = true
	}
	s.routes()
	s.handler = s.chain(http.HandlerFunc(s.dispatch))
	return s, nil
}

// Handler 是带完整中间件链的 http.Handler。
func (s *Server) Handler() http.Handler { return s.handler }

// handle 注册一条 /api/ 下需要鉴权的路由。
func (s *Server) handle(pattern string, h http.HandlerFunc) { s.mux.HandleFunc(pattern, h) }

// handlePublic 注册一条不鉴权的路由；只给 health 用（B-R3）。
func (s *Server) handlePublic(pattern string, h http.HandlerFunc) {
	s.mux.HandleFunc(pattern, h)
	s.public[pattern] = true
}

// isPublic：请求命中的路由是不是不鉴权的。
func (s *Server) isPublic(r *http.Request) bool {
	_, pattern := s.mux.Handler(r)
	return pattern != "" && s.public[pattern]
}
