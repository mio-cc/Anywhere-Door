package api

import "net/http"

// 安全响应头（S.02-R8）；开发模式只给 connect-src 加 Vite 的热更新地址（S.02-R9）。不写任何 CORS 头（S.02-R7）。
const (
	csp    = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: blob:; connect-src 'self'; font-src 'self'; frame-ancestors 'none'"
	cspDev = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: blob:; connect-src 'self' ws://127.0.0.1:5173; font-src 'self'; frame-ancestors 'none'"
)

func setSecurityHeaders(h http.Header, dev bool) {
	if dev {
		h.Set("Content-Security-Policy", cspDev)
	} else {
		h.Set("Content-Security-Policy", csp)
	}
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cross-Origin-Opener-Policy", "same-origin")
}
