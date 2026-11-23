package main

import (
	"bytes"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// 测试用的假子进程：打印 ready，等到退出信号就以 0 退出；最多活 60 秒，避免残留。
func TestMain(m *testing.M) {
	if os.Getenv("ANYDOOR_DEV_FAKE") != "" {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		fmt.Println("ready")
		select {
		case <-sig:
		case <-time.After(60 * time.Second):
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type safeBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestStopAll(t *testing.T) {
	t.Run("O01-T6 dev 退出", func(t *testing.T) {
		env := append(os.Environ(), "ANYDOOR_DEV_FAKE=1")
		out := &safeBuf{}
		g, err := startAll([]spec{
			{name: "api", cmd: os.Args[0], env: env},
			{name: "web", cmd: os.Args[0], env: env},
		}, out)
		if err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(20 * time.Second)
		for !strings.Contains(out.String(), "[api] ready") || !strings.Contains(out.String(), "[web] ready") {
			if time.Now().After(deadline) {
				g.stopAll(5 * time.Second)
				t.Fatalf("两个假进程没有都启动：\n%s", out.String())
			}
			time.Sleep(50 * time.Millisecond)
		}
		start := time.Now()
		g.stopAll(5 * time.Second)
		for _, p := range g.procs {
			if !isDone(p) {
				t.Fatalf("%s 没有退出", p.name)
			}
		}
		if time.Since(start) > 15*time.Second {
			t.Fatalf("结束得太慢：%v", time.Since(start))
		}
	})
}

func TestLineWriter(t *testing.T) {
	t.Run("O01-T6/按行加前缀", func(t *testing.T) {
		var buf bytes.Buffer
		w := &lineWriter{prefix: "[x] ", out: &buf, mu: &sync.Mutex{}}
		_, _ = w.Write([]byte("a\nb"))
		_, _ = w.Write([]byte("c\n"))
		_, _ = w.Write([]byte("d"))
		w.flush()
		if got := buf.String(); got != "[x] a\n[x] bc\n[x] d\n" {
			t.Fatalf("得到 %q", got)
		}
	})
}
