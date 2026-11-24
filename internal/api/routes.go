package api

import (
	"net/http"
	"strings"
)

// routes 注册 /api/ 下的全部路由（B.03.05 §4）；B.04 的叶子各自往这里加。
func (s *Server) routes() {
	s.handlePublic("GET /api/v1/health", s.handleHealth)      // B.04.14：唯一不鉴权的接口（B-R3）
	s.handle("POST /api/v1/admin/shutdown", s.handleShutdown) // B.04.14
}

// dispatch：/api/ 交给路由表；带 t 参数的 GET / 是引导（B.03.05 §5 第 2 条）；其余交给 Static。
func (s *Server) dispatch(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasPrefix(r.URL.Path, "/api/"):
		s.serveAPI(w, r)
	case r.URL.Path == "/" && r.Method == http.MethodGet && r.URL.Query().Has("t"):
		s.handleBootstrap(w, r)
	case s.static != nil:
		s.static.ServeHTTP(w, r)
	default:
		writeError(w, r, &notFoundError{resource: "页面", id: r.URL.Path})
	}
}

// serveAPI：路由命中，或路径存在但方法不对（ServeMux 自己写 405 和 Allow），交给 ServeMux；路径不存在写 404 not_found。
func (s *Server) serveAPI(w http.ResponseWriter, r *http.Request) {
	if _, pattern := s.mux.Handler(r); pattern != "" || s.otherMethodMatches(r) {
		s.mux.ServeHTTP(w, r)
		return
	}
	writeError(w, r, &notFoundError{resource: "接口", id: r.URL.Path})
}

var apiMethods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

// otherMethodMatches：换一个方法能不能命中路由（用来区分 404 和 405）。
func (s *Server) otherMethodMatches(r *http.Request) bool {
	for _, m := range apiMethods {
		if m == r.Method {
			continue
		}
		r2 := r.Clone(r.Context())
		r2.Method = m
		if _, pattern := s.mux.Handler(r2); pattern != "" {
			return true
		}
	}
	return false
}
