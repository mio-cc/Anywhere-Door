// STUB(B.02): 最小启动程序。只做 --version、/api/v1/health、嵌入前端与"前端还没有构建"页、--dev 代理、优雅关闭，
// 让 O.01 的 build / dev / 冒烟能跑通；数据目录、鉴权、单实例、打开浏览器的完整行为由 B.02、B.03.05、B.07 实施时整体替换（G.02.06-R10）。

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// 由 tools/build 用 -ldflags -X 注入（O.01 §4.2）。
var (
	version = "0.0.0-dev"
	commit  = "unknown"
	date    = "1970-01-01T00:00:00Z"
)

const viteAddr = "http://127.0.0.1:5173"

func main() {
	restore := utf8Console()
	code := run(os.Args[1:], os.Stdout)
	restore()
	os.Exit(code)
}

func run(args []string, out io.Writer) int {
	fset := flag.NewFlagSet("anydoor", flag.ContinueOnError)
	showVersion := fset.Bool("version", false, "打印版本并退出")
	port := fset.Int("port", 0, "监听端口；0 = 优先 7300，被占用时由系统分配")
	noBrowser := fset.Bool("no-browser", false, "不自动打开浏览器")
	dev := fset.Bool("dev", false, "开发模式：/api 以外的请求代理到 Vite（"+viteAddr+"）")
	fset.Bool("fake-llm", false, "演示模式（桩：还不起作用）")
	fset.String("data-dir", "", "数据目录（桩：还不使用）")
	fset.String("log-level", "info", "日志级别（桩：还不使用）")
	if err := fset.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		_, _ = fmt.Fprintf(out, "anydoor %s\n提交：%s\n构建时间：%s\n支持的 schema 版本：还没有（桩，B.02）\n", version, commit, date)
		return 0
	}
	ln, err := listen(*port)
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误：端口被占用或无法监听：", err)
		return 2
	}
	h, err := handler(*dev)
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误：", err)
		return 1
	}
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}
	addr := "http://" + ln.Addr().String() + "/"
	mode := ""
	if *dev {
		mode = "（开发模式，前端由 Vite 提供）"
	}
	_, _ = fmt.Fprintf(out, "Anywhere Door 已启动%s：%s\n", mode, addr)
	if !*noBrowser {
		openBrowser(addr)
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sig:
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(os.Stderr, "错误：", err)
			return 1
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "错误：没能在 15 秒内关闭：", err)
		return 1
	}
	_, _ = fmt.Fprintln(out, "Anywhere Door 已退出")
	return 0
}

// listen 只监听 127.0.0.1（INV-21）；端口为 0 时先试 7300。
func listen(port int) (net.Listener, error) {
	if port != 0 {
		return net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	}
	if ln, err := net.Listen("tcp", "127.0.0.1:7300"); err == nil {
		return ln, nil
	}
	return net.Listen("tcp", "127.0.0.1:0")
}

func handler(dev bool) (http.Handler, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "version": version})
	})
	mux.Handle("/api/", http.NotFoundHandler())
	if dev {
		target, err := url.Parse(viteAddr)
		if err != nil {
			return nil, err
		}
		mux.Handle("/", httputil.NewSingleHostReverseProxy(target))
		return mux, nil
	}
	front, err := frontend()
	if err != nil {
		return nil, err
	}
	mux.Handle("/", front)
	return mux, nil
}

// frontend 服务嵌入的前端：带哈希的 assets/ 长缓存，index.html 不缓存，找不到的路径回退到 index.html（前端路由）。
func frontend() (http.Handler, error) {
	sub, err := fs.Sub(webDist, "dist")
	if err != nil {
		return nil, err
	}
	index, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		return notBuilt(), nil
	}
	files := http.FileServerFS(sub)
	serveIndex := func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" || p == "index.html" {
			serveIndex(w)
			return
		}
		if info, err := fs.Stat(sub, p); err == nil && !info.IsDir() {
			if strings.HasPrefix(p, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			files.ServeHTTP(w, r)
			return
		}
		serveIndex(w)
	}), nil
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

func openBrowser(u string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	case "darwin":
		cmd = exec.Command("open", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "提示：没能自动打开浏览器，请手动打开", u)
		return
	}
	go func() { _ = cmd.Wait() }()
}
