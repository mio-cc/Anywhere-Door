// Command anydoor 是 Anywhere Door 的主程序（B.02）：解析参数、组装平台层与存储、启动本机 HTTP 服务、打开浏览器。
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// 由 tools/build 用 -ldflags -X 注入（O.01 §4.2、G.09-R2）。
var (
	version = "0.0.0-dev"
	commit  = "unknown"
	date    = "1970-01-01T00:00:00Z"
)

func main() {
	restore := utf8Console()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	restore()
	os.Exit(code)
}
