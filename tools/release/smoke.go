package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// startTimeout 是等"已启动"那一行的时限（O.03 §6.4 第 2 步）；测试里调短。
var startTimeout = 30 * time.Second

const startedPrefix = "Anywhere Door 已启动："

var (
	scriptRe = regexp.MustCompile(`<script[^>]+type="module"`)
	assetRe  = regexp.MustCompile(`(?:src|href)="(/assets/[^"]+)"`)
)

// syncBuf 收集子进程的输出。
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

// Smoke 是启动冒烟（O.03 §6.4）：--version → 用临时数据目录启动 → health、引导、前端、静态资源 → 关闭。
func Smoke(ctx context.Context, bin string) (err error) {
	out, verr := exec.CommandContext(ctx, bin, "--version").Output()
	if verr != nil {
		return fmt.Errorf("第 1 步：%s --version 失败：%w", bin, verr)
	}
	version, verr := parseVersion(string(out))
	if verr != nil {
		return fmt.Errorf("第 1 步：%w\n--version 的输出：\n%s", verr, out)
	}

	dir, err := os.MkdirTemp("", "anydoor-smoke-")
	if err != nil {
		return err
	}
	defer removeAll(dir) // 最先注册、最后执行：子进程结束之后才删（第 ⑧b 片评审第 8 行）
	cmd := exec.Command(bin, "--no-browser", "--fake-llm", "--port", "0", "--data-dir", dir)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr syncBuf
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("第 2 步：启动失败：%w", err)
	}
	// 子进程退出后 waitDone 关闭、waitErr 是退出结果；关闭的通道可以读多次，谁都能等
	waitDone := make(chan struct{})
	var waitErr error
	lines := make(chan string, 64)
	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			select {
			case lines <- sc.Text():
			default: // 读到"已启动"之后不再有人收，丢掉，但要一直读，免得子进程写满管道
			}
		}
		waitErr = cmd.Wait() // 读完管道才能 Wait
		close(waitDone)
	}()
	defer func() {
		select {
		case <-waitDone:
		default:
			_ = cmd.Process.Kill()
			<-waitDone
		}
		if err != nil && stderr.String() != "" {
			err = fmt.Errorf("%w\n程序的标准错误：\n%s", err, stderr.String())
		}
	}()

	addr, err := waitStarted(ctx, lines, waitDone, &waitErr)
	if err != nil {
		return err
	}
	u, err := url.Parse(addr)
	if err != nil || u.Query().Get("t") == "" {
		return fmt.Errorf("第 2 步：\"已启动\"那一行的地址 %q 不对（--no-browser 时应当带 token）", addr)
	}
	base := u.Scheme + "://" + u.Host
	token := u.Query().Get("t")
	if err := checkServing(ctx, base, addr, version); err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		if err := post(ctx, base+"/api/v1/admin/shutdown", token); err != nil {
			return fmt.Errorf("第 4 步：%w", err)
		}
	} else if err := cmd.Process.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("第 4 步：发送关闭信号：%w", err)
	}
	select {
	case <-waitDone:
		if waitErr != nil {
			return fmt.Errorf("第 4 步：退出码不是 0：%w", waitErr)
		}
	case <-time.After(15 * time.Second):
		return errors.New("第 4 步：15 秒内没有退出")
	}
	return nil
}

func waitStarted(ctx context.Context, lines <-chan string, waitDone <-chan struct{}, waitErr *error) (string, error) {
	timer := time.NewTimer(startTimeout)
	defer timer.Stop()
	for {
		select {
		case l := <-lines:
			if rest, ok := strings.CutPrefix(strings.TrimSpace(l), startedPrefix); ok {
				return strings.TrimSpace(rest), nil
			}
		case <-waitDone:
			return "", fmt.Errorf("第 2 步：程序提前退出：%v", *waitErr)
		case <-timer.C:
			return "", fmt.Errorf("第 2 步：%v 内没有打印 %q", startTimeout, startedPrefix)
		case <-ctx.Done():
			return "", fmt.Errorf("第 2 步：%w", ctx.Err())
		}
	}
}

var (
	versionLineRe = regexp.MustCompile(`(?m)^anydoor (\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+dirty)?)\s*$`)
	commitLineRe  = regexp.MustCompile(`(?m)^提交：([0-9a-f]{40})\s*$`)
	dateLineRe    = regexp.MustCompile(`(?m)^构建时间：(\S+)\s*$`)
	schemaLineRe  = regexp.MustCompile(`(?m)^支持的 schema 版本：\S`)
)

// parseVersion 检查 --version 的输出符合 O.01 §4.2（版本号、40 位提交号、RFC 3339 的提交时间）和 B.02 §5（schema 版本范围），
// 并且都是构建时注入的值（不是程序里的缺省值 0.0.0-dev、unknown、1970-01-01），返回版本号。
func parseVersion(out string) (string, error) {
	out = strings.ReplaceAll(out, "\r\n", "\n")
	m := versionLineRe.FindStringSubmatch(out)
	if m == nil || m[1] == "0.0.0-dev" {
		return "", errors.New("版本号那一行不对（应当是\"anydoor <版本>\"，且是构建时注入的）")
	}
	if commitLineRe.FindStringSubmatch(out) == nil {
		return "", errors.New("提交那一行应当是 40 位的提交号")
	}
	d := dateLineRe.FindStringSubmatch(out)
	if d == nil {
		return "", errors.New("没有构建时间那一行")
	}
	if ts, err := time.Parse(time.RFC3339, d[1]); err != nil || ts.Unix() <= 0 {
		return "", fmt.Errorf("构建时间 %q 不是注入的提交时间", d[1])
	}
	if !schemaLineRe.MatchString(out) {
		return "", errors.New("没有支持的 schema 版本那一行")
	}
	return m[1], nil
}

// checkServing 是 O.03 §6.4 第 3 步；health 报的版本要和 --version 的一致。
func checkServing(ctx context.Context, base, withToken, version string) error {
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar, Timeout: 10 * time.Second, Transport: &http.Transport{Proxy: nil}}
	get := func(u string) (*http.Response, string, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, "", err
		}
		resp, err := c.Do(req)
		if err != nil {
			return nil, "", err
		}
		defer func() { _ = resp.Body.Close() }()
		b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		return resp, string(b), err
	}
	resp, body, err := get(base + "/api/v1/health")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(body, `"ok":true`) {
		return fmt.Errorf("第 3 步：health 不正常：%v %s", err, body)
	}
	if !strings.Contains(body, `"version":"`+version+`"`) {
		return fmt.Errorf("第 3 步：health 报的版本和 --version 的 %s 不一致：%s", version, body)
	}
	resp, body, err = get(withToken)
	if err != nil || resp.StatusCode != http.StatusOK {
		return fmt.Errorf("第 3 步：打开带 token 的地址失败：%v", err)
	}
	if resp.Request.URL.RawQuery != "" {
		return fmt.Errorf("第 3 步：引导后地址里还有 token：%s", resp.Request.URL)
	}
	if !scriptRe.MatchString(body) {
		return errors.New("第 3 步：首页不是前端的 index.html（没有入口脚本）")
	}
	m := assetRe.FindStringSubmatch(body)
	if m == nil {
		return errors.New("第 3 步：首页里没有 /assets/ 下的文件")
	}
	resp, _, err = get(base + m[1])
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(resp.Header.Get("Cache-Control"), "immutable") {
		return fmt.Errorf("第 3 步：%s 应当是 200 且带长缓存头：%v", m[1], err)
	}
	return nil
}

func post(ctx context.Context, u, token string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{Proxy: nil}}).Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("关闭接口返回 %d，应当是 202", resp.StatusCode)
	}
	return nil
}

// removeAll 删临时数据目录；Windows 上进程刚退出时文件可能还占着，重试几次。
func removeAll(dir string) {
	for i := 0; i < 20; i++ {
		if os.RemoveAll(dir) == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	warnf("删不掉临时目录 %s", dir)
}
