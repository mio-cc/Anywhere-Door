package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// viteURL 是开发模式代理的目标（Vite 开发服务器，O.01 §4.3）；测试里替换。
var viteURL = "http://127.0.0.1:5173"

// devProxy 把请求（包括热更新的 WebSocket）反向代理到 Vite（B.02 §5"开发模式"）。
func devProxy(target string) (http.Handler, error) {
	u, err := url.Parse(target)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("开发模式的代理地址 %q 不对：%v", target, err)
	}
	return &httputil.ReverseProxy{Rewrite: func(pr *httputil.ProxyRequest) { pr.SetURL(u) }}, nil
}
