// Command dev 在一个终端里同时启动后端和前端（设计文档 O.01 §4.3）：
//
//	go run ./tools/dev [-- anydoor 的参数，例如 --fake-llm]
//
// 后端：go run ./cmd/anydoor --dev --data-dir ./.anydoor-dev；前端：pnpm -C web dev。
// 两边的输出加上 [api]、[web] 前缀合并打印；任一进程退出或按 Ctrl+C 时结束另一个。
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout)) }

func run(args []string, out io.Writer) int {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	root, err := repoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误：", err)
		return 1
	}
	api := spec{name: "api", cmd: "go", args: append([]string{"run", "./cmd/anydoor", "--dev", "--data-dir", "./.anydoor-dev"}, args...), dir: root}
	web := spec{name: "web", cmd: "pnpm", args: []string{"-C", "web", "dev"}, dir: root}
	g, err := startAll([]spec{api, web}, out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误：", err)
		return 1
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sig:
		g.say("收到退出信号，正在结束后端和前端…")
	case name := <-g.exited:
		g.say(fmt.Sprintf("%s 已退出，正在结束另一个…", name))
	}
	g.stopAll(10 * time.Second)
	return 0
}

func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("找不到 go.mod：请在仓库里运行")
		}
		dir = parent
	}
}

type spec struct {
	name, cmd string
	args      []string
	dir       string
	env       []string // 为空时继承当前环境
}

type proc struct {
	spec
	cmd  *exec.Cmd
	out  *lineWriter
	done chan struct{}
}

type group struct {
	procs  []*proc
	exited chan string
	mu     *sync.Mutex
	out    io.Writer
}

// startAll 依次启动；任何一个启动失败就结束已启动的，返回错误。
func startAll(specs []spec, out io.Writer) (*group, error) {
	g := &group{exited: make(chan string, len(specs)), mu: &sync.Mutex{}, out: out}
	for _, s := range specs {
		c := exec.Command(s.cmd, s.args...)
		c.Dir = s.dir
		if s.env != nil {
			c.Env = s.env
		}
		setProcAttr(c)
		lw := &lineWriter{prefix: "[" + s.name + "] ", out: out, mu: g.mu}
		c.Stdout, c.Stderr = lw, lw
		if err := c.Start(); err != nil {
			g.stopAll(5 * time.Second)
			return nil, fmt.Errorf("启动 %s 失败：%w", s.name, err)
		}
		p := &proc{spec: s, cmd: c, out: lw, done: make(chan struct{})}
		g.procs = append(g.procs, p)
		go func() {
			_ = c.Wait()
			lw.flush()
			close(p.done)
			g.exited <- p.name
		}()
	}
	return g, nil
}

// stopAll 先礼貌地请每个进程退出（Windows：CTRL_BREAK；其他平台：SIGINT 给整个进程组），
// timeout 内没退出的强制结束整棵进程树。
func (g *group) stopAll(timeout time.Duration) {
	for _, p := range g.procs {
		if !isDone(p) {
			_ = interrupt(p.cmd)
		}
	}
	if g.waitAll(timeout) {
		return
	}
	for _, p := range g.procs {
		if !isDone(p) {
			g.say(p.name + " 没有按时退出，强制结束")
			_ = kill(p.cmd)
		}
	}
	g.waitAll(5 * time.Second)
}

func (g *group) waitAll(timeout time.Duration) bool {
	deadline := time.After(timeout)
	for _, p := range g.procs {
		select {
		case <-p.done:
		case <-deadline:
			return false
		}
	}
	return true
}

func (g *group) say(msg string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	_, _ = fmt.Fprintln(g.out, "[dev] "+msg)
}

func isDone(p *proc) bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

// lineWriter 按行给输出加前缀；多个进程共用一把锁，行与行不会交错。
type lineWriter struct {
	prefix string
	out    io.Writer
	mu     *sync.Mutex
	buf    bytes.Buffer
}

func (w *lineWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Write(b)
	for {
		i := bytes.IndexByte(w.buf.Bytes(), '\n')
		if i < 0 {
			break
		}
		line := w.buf.Next(i + 1)
		if _, err := fmt.Fprintf(w.out, "%s%s", w.prefix, line); err != nil {
			return len(b), err
		}
	}
	return len(b), nil
}

func (w *lineWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf.Len() > 0 {
		_, _ = fmt.Fprintf(w.out, "%s%s\n", w.prefix, w.buf.Bytes())
		w.buf.Reset()
	}
}
