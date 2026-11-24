package main

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// frontend 是 /api/ 以外路径的处理：开发模式代理到 Vite，生产模式服务嵌入的前端（B.02 §5"路由"）。
func frontend(dev bool) (http.Handler, error) {
	if dev {
		return devProxy(viteURL)
	}
	sub, err := fs.Sub(webDist, "dist")
	if err != nil {
		return nil, err
	}
	return staticHandler(sub), nil
}

// staticHandler 服务前端产物 dist：带哈希的 assets/ 长缓存，index.html 不缓存；找不到的 assets/ 文件 404，
// 其余找不到的路径回退到 index.html（前端路由）。dist 里没有 index.html（只有占位的 .gitkeep，没先构建前端）时
// 一律返回说明页（P.09 K187）。
func staticHandler(dist fs.FS) http.Handler {
	index, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		return notBuilt()
	}
	files := http.FileServerFS(dist)
	serveIndex := func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if p == "" || p == "index.html" {
			serveIndex(w)
			return
		}
		info, err := fs.Stat(dist, p)
		switch {
		case err == nil && !info.IsDir():
			if strings.HasPrefix(p, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			files.ServeHTTP(w, r)
		case strings.HasPrefix(p, "assets/"):
			http.NotFound(w, r)
		default:
			serveIndex(w)
		}
	})
}

const notBuiltPage = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><title>Anywhere Door</title></head>
<body style="font-family:system-ui,sans-serif;max-width:40em;margin:4em auto;line-height:1.6">
<h1>前端还没有构建</h1>
<p>在仓库根运行 <code>make build</code>（或 <code>task build</code>），再启动 anydoor。</p>
<p>后端接口照常可用：<a href="/api/v1/health">/api/v1/health</a></p>
</body></html>
`

// notBuilt 是嵌入目录里只有占位文件时的说明页（B.02 §5，P.09 K187）。
func notBuilt() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = io.WriteString(w, notBuiltPage)
	})
}
