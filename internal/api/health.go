package api

import "net/http"

// healthBody 是 health 的全部内容，不能扩展（B-R3）。
type healthBody struct {
	OK      bool   `json:"ok"`
	Version string `json:"version"`
}

// handleHealth：GET /api/v1/health，不鉴权，只返回 {ok, version}（B.04.14），供单实例检测（lock.Probe）和安装程序使用。
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthBody{OK: true, Version: s.version})
}
