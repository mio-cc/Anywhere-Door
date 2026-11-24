package api

import "net/http"

// handleShutdown：POST /api/v1/admin/shutdown，只有用户或 anydoorctl（IsUser）。
// 先写出 202，再调用 Deps.Shutdown（同一个 Server 上只调用一次，重复请求也返回 202）；关闭顺序归 B.02。
func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if c, _ := CallerFrom(r.Context()); !IsUser(c) {
		writeError(w, r, &forbiddenError{action: "POST /api/v1/admin/shutdown", target: "anydoor", reasons: []string{"只有本机用户或 anydoorctl 能关闭服务"}})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
	if fl, ok := w.(http.Flusher); ok {
		fl.Flush()
	}
	s.shutdown()
}
