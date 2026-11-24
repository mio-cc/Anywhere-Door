package main

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var bg = context.Background()

func TestMain(m *testing.M) {
	secretForceFile = true // 不碰真实系统密钥库（B.07.02）
	probeWait = 300 * time.Millisecond
	openURL = opened.record // 不打开真的浏览器
	os.Exit(m.Run())
}

// recorder 记下 openURL 被调用的地址。
type recorder struct {
	mu   sync.Mutex
	urls []string
}

var opened = &recorder{}

func (r *recorder) record(u string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.urls = append(r.urls, u)
	return nil
}

func (r *recorder) take() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.urls
	r.urls = nil
	return out
}

// syncBuf 是可以一边写一边读的缓冲（run 在另一个 goroutine 里写）。
type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// running 是在 goroutine 里运行的一次 run。
type running struct {
	t         *testing.T
	cancel    context.CancelFunc
	out, errb *syncBuf
	exited    chan struct{}
	code      int
}

func start(t *testing.T, args ...string) *running {
	t.Helper()
	ctx, cancel := context.WithCancel(bg)
	r := &running{t: t, cancel: cancel, out: &syncBuf{}, errb: &syncBuf{}, exited: make(chan struct{})}
	go func() {
		r.code = run(ctx, args, r.out, r.errb)
		close(r.exited)
	}()
	t.Cleanup(func() {
		cancel()
		if _, ok := r.wait(20 * time.Second); !ok {
			t.Error("清理时 20 秒内没有退出")
		}
	})
	return r
}

func (r *running) wait(d time.Duration) (int, bool) {
	select {
	case <-r.exited:
		return r.code, true
	case <-time.After(d):
		return 0, false
	}
}

// stop 取消 ctx（相当于 Ctrl+C），返回退出码。
func (r *running) stop() int {
	r.t.Helper()
	r.cancel()
	code, ok := r.wait(20 * time.Second)
	if !ok {
		r.t.Fatal("取消后 20 秒内没有退出")
	}
	return code
}

// waitLine 等标准输出出现以 prefix 开头的一行，返回这一行 prefix 后面的部分。
func (r *running) waitLine(prefix string) string {
	r.t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if rest, ok := lineAfter(r.out.String(), prefix); ok {
			return rest
		}
		select {
		case <-r.exited:
			r.t.Fatalf("run 提前返回 %d；stdout：%s\nstderr：%s", r.code, r.out.String(), r.errb.String())
		case <-time.After(50 * time.Millisecond):
		}
	}
	r.t.Fatalf("30 秒内没有等到 %q；stdout：%s", prefix, r.out.String())
	return ""
}

func lineAfter(text, prefix string) (string, bool) {
	for _, l := range strings.Split(text, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimRight(l, "\r"), prefix); ok {
			return strings.TrimSpace(rest), true
		}
	}
	return "", false
}

// parseAddr 从"已启动"那一行的地址里取端口和 token。
func parseAddr(t *testing.T, addr string) (port, token string) {
	t.Helper()
	u, err := url.Parse(addr)
	if err != nil || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.Path != "/" {
		t.Fatalf("地址 %q 不对：%v", addr, err)
	}
	return u.Port(), u.Query().Get("t")
}

var client = &http.Client{
	Transport:     &http.Transport{Proxy: nil, DisableKeepAlives: true},
	Timeout:       10 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func call(t *testing.T, method, u string, hdr map[string]string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(method, u, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s：%v", method, u, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, string(b)
}

// lanIPv4 是本机第一个非回环、非链路本地的 IPv4 地址；没有时为空。
func lanIPv4() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok {
			if ip := n.IP.To4(); ip != nil && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() {
				return ip.String()
			}
		}
	}
	return ""
}

func freePort(t *testing.T) (int, net.Listener) {
	t.Helper()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return ln.Addr().(*net.TCPAddr).Port, ln
}

func TestRun(t *testing.T) {
	t.Run("B02-T1 启动与关闭", func(t *testing.T) {
		dir := t.TempDir()
		opened.take()
		r := start(t, "--data-dir", dir, "--no-browser")
		addr := r.waitLine("Anywhere Door 已启动：")
		port, tok := parseAddr(t, addr)
		if tok == "" {
			t.Errorf("--no-browser 时地址应当带 token：%s", addr)
		}
		resp, body := call(t, http.MethodGet, "http://127.0.0.1:"+port+"/api/v1/health", nil)
		if resp.StatusCode != http.StatusOK || !strings.Contains(body, `"ok":true`) {
			t.Errorf("health：%d %s", resp.StatusCode, body)
		}
		b, err := os.ReadFile(filepath.Join(dir, "port"))
		if err != nil || strings.TrimSpace(string(b)) != port {
			t.Errorf("port 文件是 %q（%v），应当是 %s", b, err, port)
		}
		if ip := lanIPv4(); ip != "" {
			if c, err := net.DialTimeout("tcp", net.JoinHostPort(ip, port), time.Second); err == nil {
				_ = c.Close()
				t.Errorf("局域网地址 %s:%s 不应当连得上（S.02-R1）", ip, port)
			}
		} else {
			t.Log("这台机器没有非回环的 IPv4 地址，没有做局域网连接检查")
		}
		began := time.Now()
		if code := r.stop(); code != 0 {
			t.Errorf("退出码 %d；stderr：%s", code, r.errb.String())
		}
		if d := time.Since(began); d > 15*time.Second {
			t.Errorf("关闭用了 %v，超过 15 秒", d)
		}
		if _, err := os.Stat(filepath.Join(dir, "port")); !os.IsNotExist(err) {
			t.Errorf("退出后 port 文件应当已删除：%v", err)
		}
		if logs, _ := filepath.Glob(filepath.Join(dir, "logs", "anydoor-*.log")); len(logs) == 0 {
			t.Error("logs/ 下应当有日志文件")
		}
		if !strings.Contains(r.out.String(), "Anywhere Door 已退出") {
			t.Errorf("stdout：%s", r.out.String())
		}
		if u := opened.take(); len(u) != 0 {
			t.Errorf("--no-browser 时不应当打开浏览器：%v", u)
		}
	})

	t.Run("B02-T2 开发代理", func(t *testing.T) {
		vite := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
				// 假的热更新 WebSocket：回 101，再把收到的一行原样写回
				conn, rw, err := http.NewResponseController(w).Hijack()
				if err != nil {
					return
				}
				defer func() { _ = conn.Close() }()
				_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
				_ = rw.Flush()
				line, _ := rw.ReadString('\n')
				_, _ = rw.WriteString("echo:" + line)
				_ = rw.Flush()
				return
			}
			_, _ = io.WriteString(w, "vite:"+r.URL.Path)
		}))
		defer vite.Close()
		old := viteURL
		viteURL = vite.URL
		defer func() { viteURL = old }()
		r := start(t, "--dev", "--data-dir", t.TempDir(), "--no-browser")
		port, _ := parseAddr(t, r.waitLine("Anywhere Door 已启动："))
		base := "http://127.0.0.1:" + port
		resp, body := call(t, http.MethodGet, base+"/src/main.tsx", nil)
		if body != "vite:/src/main.tsx" {
			t.Errorf("应当代理到 Vite，得到 %d %q", resp.StatusCode, body)
		}
		if c := resp.Header.Get("Content-Security-Policy"); !strings.Contains(c, "ws://127.0.0.1:5173") {
			t.Errorf("开发模式的 CSP：%s", c)
		}
		if resp, body := call(t, http.MethodGet, base+"/api/v1/health", nil); resp.StatusCode != http.StatusOK || strings.HasPrefix(body, "vite:") {
			t.Errorf("/api/ 应当由本程序回答：%d %s", resp.StatusCode, body)
		}
		// 热更新的 WebSocket 也经代理转到 Vite（B.02 §5"开发模式"）
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 5*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = conn.Close() }()
		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
		_, err = io.WriteString(conn, "GET /__hmr HTTP/1.1\r\nHost: 127.0.0.1:"+port+"\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n"+
			"Sec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n")
		if err != nil {
			t.Fatal(err)
		}
		br := bufio.NewReader(conn)
		ws, err := http.ReadResponse(br, nil)
		if err != nil {
			t.Fatalf("WebSocket 握手：%v", err)
		}
		if ws.StatusCode != http.StatusSwitchingProtocols {
			t.Fatalf("WebSocket 握手应当是 101，是 %d", ws.StatusCode)
		}
		_, _ = io.WriteString(conn, "ping\n")
		if line, err := br.ReadString('\n'); err != nil || line != "echo:ping\n" {
			t.Errorf("WebSocket 连接上的数据应当原样转发：%q %v", line, err)
		}
		if code := r.stop(); code != 0 {
			t.Errorf("退出码 %d", code)
		}
	})

	t.Run("B02-T4 单实例转交", func(t *testing.T) {
		dir := t.TempDir()
		first := start(t, "--data-dir", dir, "--no-browser")
		addr := first.waitLine("Anywhere Door 已启动：")
		port, tok := parseAddr(t, addr)
		opened.take()

		out, errb := &syncBuf{}, &syncBuf{}
		if code := run(bg, []string{"--data-dir", dir, "--no-browser"}, out, errb); code != 0 {
			t.Fatalf("第二次启动应当返回 0，是 %d：%s", code, errb.String())
		}
		if got, _ := lineAfter(out.String(), "Anywhere Door 已在运行："); got != addr {
			t.Errorf("应当打印已有实例带 token 的地址 %q，得到：%s", addr, out.String())
		}
		if u := opened.take(); len(u) != 0 {
			t.Errorf("--no-browser 时不应当打开浏览器：%v", u)
		}

		out, errb = &syncBuf{}, &syncBuf{}
		if code := run(bg, []string{"--data-dir", dir}, out, errb); code != 0 {
			t.Fatalf("第三次启动应当返回 0，是 %d：%s", code, errb.String())
		}
		if u := opened.take(); len(u) != 1 || u[0] != "http://127.0.0.1:"+port+"/?t="+tok {
			t.Errorf("应当打开一次带 token 的地址，得到 %v", u)
		}
		if got, _ := lineAfter(out.String(), "Anywhere Door 已在运行："); got != "http://127.0.0.1:"+port+"/" {
			t.Errorf("打开浏览器时终端里的地址不带 token，得到：%s", out.String())
		}

		dead, ln := freePort(t)
		_ = ln.Close()
		if err := os.WriteFile(filepath.Join(dir, "port"), []byte(strconv.Itoa(dead)+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		out, errb = &syncBuf{}, &syncBuf{}
		if code := run(bg, []string{"--data-dir", dir, "--no-browser"}, out, errb); code != 1 {
			t.Errorf("已有实例没有响应时应当返回 1，是 %d", code)
		}
		if !strings.Contains(errb.String(), "没有响应") {
			t.Errorf("stderr：%s", errb.String())
		}
		if code := first.stop(); code != 0 {
			t.Errorf("第一个实例退出码 %d", code)
		}
	})

	t.Run("B02-T5 --version", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "never")
		out, errb := &syncBuf{}, &syncBuf{}
		if code := run(bg, []string{"--version", "--data-dir", dir}, out, errb); code != 0 {
			t.Fatalf("退出码 %d：%s", code, errb.String())
		}
		for _, want := range []string{"anydoor " + version, "提交：" + commit, "构建时间：" + date, "支持的 schema 版本：图文档 1.0"} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("输出里没有 %q：%s", want, out.String())
			}
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Errorf("--version 不应当创建数据目录：%v", err)
		}
	})

	t.Run("B02-T7 启动失败", func(t *testing.T) {
		busy, ln := freePort(t)
		defer func() { _ = ln.Close() }()
		dir := t.TempDir()
		out, errb := &syncBuf{}, &syncBuf{}
		if code := run(bg, []string{"--data-dir", dir, "--no-browser", "--port", strconv.Itoa(busy)}, out, errb); code != 2 {
			t.Errorf("端口被占用时应当返回 2，是 %d：%s", code, errb.String())
		}
		if s := errb.String(); !strings.Contains(s, "错误：") || !strings.Contains(s, "被占用") || strings.Contains(s, "goroutine") {
			t.Errorf("错误输出应当是一句中文、没有堆栈：%s", s)
		}
		if _, err := os.Stat(filepath.Join(dir, "port")); !os.IsNotExist(err) {
			t.Errorf("失败后不应当留下 port 文件：%v", err)
		}
		out, errb = &syncBuf{}, &syncBuf{}
		if code := run(bg, []string{"--bogus"}, out, errb); code != 2 {
			t.Errorf("不认识的参数应当返回 2，是 %d", code)
		}
		r := start(t, "--data-dir", dir, "--no-browser")
		r.waitLine("Anywhere Door 已启动：")
		if code := r.stop(); code != 0 {
			t.Errorf("锁应当已释放、能再次启动；退出码 %d", code)
		}
	})

	t.Run("B02-T8 shutdown 接口", func(t *testing.T) {
		r := start(t, "--data-dir", t.TempDir(), "--no-browser")
		port, tok := parseAddr(t, r.waitLine("Anywhere Door 已启动："))
		resp, body := call(t, http.MethodPost, "http://127.0.0.1:"+port+"/api/v1/admin/shutdown", map[string]string{"Authorization": "Bearer " + tok})
		if resp.StatusCode != http.StatusAccepted {
			t.Fatalf("shutdown：%d %s", resp.StatusCode, body)
		}
		code, ok := r.wait(15 * time.Second)
		if !ok || code != 0 {
			t.Errorf("15 秒内应当以 0 退出：退出了 %v，退出码 %d", ok, code)
		}
	})
}
