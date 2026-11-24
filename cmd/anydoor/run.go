package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mio-cc/anywhere-door/engine/store/sqlite"
	"github.com/mio-cc/anywhere-door/internal/api"
	"github.com/mio-cc/anywhere-door/internal/platform/auth"
	"github.com/mio-cc/anywhere-door/internal/platform/config"
	"github.com/mio-cc/anywhere-door/internal/platform/datadir"
	"github.com/mio-cc/anywhere-door/internal/platform/lock"
	"github.com/mio-cc/anywhere-door/internal/platform/secret"
)

// schemaVersions 是这个程序支持的 schema 版本范围（G.09-R1）：schema/graph.schema.json 的 schema_version。
const schemaVersions = "图文档 1.0"

// shutdownLimit 是关闭的总时限（B.01 §4）。
const shutdownLimit = 15 * time.Second

// 测试里替换（run_test.go 的 TestMain）。
var (
	secretForceFile = false           // true：密钥用加密文件后端，不碰真实系统密钥库（B.07.02）
	probeWait       = 3 * time.Second // 已有实例第一次探活不通时，等多久再探一次（B.02 §5）
)

// run 是可测试的入口：ctx 取消即关闭；返回退出码（B.02 §7）。
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	a, err := config.ParseArgs(args)
	if err != nil {
		say(stderr, "错误：%v", err)
		return 2
	}
	if a.Version {
		say(stdout, "anydoor %s", version)
		say(stdout, "提交：%s", commit)
		say(stdout, "构建时间：%s", date)
		say(stdout, "支持的 schema 版本：%s", schemaVersions)
		return 0
	}
	return serve(ctx, a, stdout, stderr)
}

func say(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format+"\n", args...)
}

// addresses 是给人看的地址和带 token 的引导地址（B.03.05 §5 第 2 条）。
func addresses(port int, token string) (base, withToken string) {
	base = "http://127.0.0.1:" + strconv.Itoa(port) + "/"
	return base, base + "?t=" + token
}

// cleanups 按相反顺序执行（B.01 §3：任何一步失败都按相反顺序清理已初始化的部分）。
type cleanups []func()

func (c *cleanups) add(f func()) { *c = append(*c, f) }

func (c cleanups) run() {
	for i := len(c) - 1; i >= 0; i-- {
		c[i]()
	}
}

// serve 按 B.02 §5 的 M0 组装顺序启动，直到 ctx 取消或收到 shutdown 请求（P.09 K242）。
func serve(ctx context.Context, a config.Args, stdout, stderr io.Writer) int {
	closeLog := func() {}
	defer func() { closeLog() }() // 日志最后关（B.02 §5"关闭"）
	var done cleanups
	defer func() { done.run() }() // 闭包：退出时才读 done（defer done.run() 会在这里就取走空列表）
	fail := func(code int, format string, args ...any) int {
		say(stderr, "错误："+format, args...)
		return code
	}

	root, err := datadir.Resolve(a.DataDir, a.Dev)
	if err != nil {
		return fail(1, "%v", err)
	}
	d, err := datadir.Open(root)
	if err != nil {
		return fail(1, "%v", err)
	}
	cfg, err := config.Load(a, os.Environ(), d, config.Options{Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		return fail(1, "%v", err)
	}
	open := cfg.Bool("server.open_browser")
	l, inst, err := lock.Acquire(d)
	if err != nil {
		return fail(1, "%v", err)
	}
	if inst != nil {
		return handoff(ctx, d, inst, open, stdout, stderr)
	}
	done.add(func() { _ = l.Release() })

	log, cl, err := openLog(d, cfg.String("log.level"), stderr, time.Now())
	if err != nil {
		return fail(1, "%v", err)
	}
	closeLog = cl
	for _, w := range cfg.Warnings() {
		log.Warn("config: " + w)
	}
	if a.Dev {
		log.Warn("开发模式，不要用于日常使用（S.02-R9）", "vite", viteURL)
	}
	if a.FakeLLM {
		log.Info("--fake-llm 在 M1 起生效（E.10.08）")
	}

	st, err := sqlite.Open(ctx, d.DB(), sqlite.Options{Logger: log})
	switch {
	case errors.Is(err, sqlite.ErrSchemaTooNew):
		return fail(3, "数据库需要更新版本的 Anywhere Door（%s）", d.DB())
	case err != nil:
		return fail(1, "%v", err)
	}
	done.add(func() {
		if err := st.Close(); err != nil {
			log.Warn("关闭数据库时出错", "err", err)
		}
	})
	au, err := auth.Open(ctx, d, st.Settings(), auth.Options{})
	if err != nil {
		return fail(1, "%v", err)
	}
	if _, err := secret.Open(ctx, d, secret.Options{Dev: a.Dev, ForceFile: secretForceFile, Logger: log}); err != nil {
		return fail(1, "%v", err)
	}

	lns, err := listen(int(cfg.Int("server.port")), log)
	if err != nil {
		return fail(2, "%v", err)
	}
	done.add(func() {
		for _, ln := range lns {
			_ = ln.Close()
		}
	})
	port := lns[0].Addr().(*net.TCPAddr).Port
	static, err := frontend(a.Dev)
	if err != nil {
		return fail(1, "%v", err)
	}
	stopc := make(chan struct{})
	srv, err := api.NewServer(api.Deps{
		Auth:     au,
		Port:     port,
		Version:  version,
		Dev:      a.Dev,
		Static:   static,
		Shutdown: sync.OnceFunc(func() { close(stopc) }),
		Logger:   log,
	})
	if err != nil {
		return fail(1, "%v", err)
	}
	hs := &http.Server{
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}
	if err := l.WritePort(port); err != nil {
		return fail(1, "%v", err)
	}
	errc := make(chan error, len(lns))
	for _, ln := range lns {
		go func() { errc <- hs.Serve(ln) }()
	}

	base, withToken := addresses(port, au.Token())
	shown := base
	if !open {
		shown = withToken
	}
	say(stdout, "Anywhere Door 已启动：%s", shown)
	log.Info("已启动", "port", port, "data_dir", d.Root, "version", version)
	if open {
		openBrowser(withToken, stdout, log)
	}

	code := 0
	select {
	case <-ctx.Done():
	case <-stopc:
	case err := <-errc:
		log.Error("HTTP 服务异常退出", "err", err)
		say(stderr, "错误：HTTP 服务异常退出：%v", err)
		code = 1
	}
	log.Info("正在关闭")
	sctx, cancel := context.WithTimeout(context.Background(), shutdownLimit)
	defer cancel()
	if err := hs.Shutdown(sctx); err != nil {
		return fail(1, "没能在 15 秒内关闭：%v", err)
	}
	if code == 0 {
		say(stdout, "Anywhere Door 已退出")
	}
	return code
}

// handoff 把这次启动转交给已在运行的实例（B.02 §5"已有实例"，P.09 K240）。
func handoff(ctx context.Context, d datadir.Dir, inst *lock.Instance, open bool, stdout, stderr io.Writer) int {
	alive := func(port int) bool {
		c, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		return lock.Probe(c, port)
	}
	port := inst.Port
	if !alive(port) {
		select {
		case <-time.After(probeWait):
		case <-ctx.Done():
			return 1
		}
		port = readPort(d)
		if !alive(port) {
			say(stderr, "错误：另一个 Anywhere Door 进程没有响应（PID %d），请结束它后重试", inst.PID)
			return 1
		}
	}
	tok, err := auth.ReadToken(d)
	if err != nil {
		say(stderr, "错误：%v", err)
		return 1
	}
	base, withToken := addresses(port, tok)
	shown := base
	if !open {
		shown = withToken
	}
	say(stdout, "Anywhere Door 已在运行：%s", shown)
	if open {
		openBrowser(withToken, stdout, nil)
	}
	return 0
}

// readPort 重新读 <DataDir>/port（对方可能刚刚写好）；读不到为 0。
func readPort(d datadir.Dir) int {
	b, err := os.ReadFile(d.Port())
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0
	}
	return n
}
