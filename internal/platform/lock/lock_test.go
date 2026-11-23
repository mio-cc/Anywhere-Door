package lock

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/mio-cc/anywhere-door/internal/platform/datadir"
)

const (
	helperEnv    = "ANYDOOR_LOCK_HELPER"
	helperDirEnv = "ANYDOOR_LOCK_HELPER_DIR"
)

// TestLockHelper 只在子进程里起作用（B0704-T2）：取锁后打印 locked 并一直等着被杀。
func TestLockHelper(t *testing.T) {
	dir := os.Getenv(helperDirEnv)
	if os.Getenv(helperEnv) != "1" || dir == "" {
		return
	}
	l, inst, err := Acquire(datadir.Dir{Root: dir})
	if err != nil || inst != nil || l == nil {
		fmt.Println("failed", err, inst)
		os.Exit(2)
	}
	fmt.Println("locked")
	time.Sleep(time.Hour)
}

func openDir(t *testing.T) datadir.Dir {
	t.Helper()
	d, err := datadir.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestAcquire(t *testing.T) {
	t.Run("B0704-T1 第二次取锁", func(t *testing.T) {
		d := openDir(t)
		l, inst, err := Acquire(d)
		if err != nil || inst != nil || l == nil {
			t.Fatalf("第一次应当拿到锁：%v %v", inst, err)
		}
		if err := l.WritePort(7300); err != nil {
			t.Fatal(err)
		}
		l2, inst, err := Acquire(d)
		if err != nil || l2 != nil || inst == nil {
			t.Fatalf("第二次应当拿不到：%v %v %v", l2, inst, err)
		}
		if inst.PID != os.Getpid() || inst.Port != 7300 {
			t.Errorf("读出的实例是 %+v，应当 PID %d、端口 7300", inst, os.Getpid())
		}
		if err := l.Release(); err != nil {
			t.Fatal(err)
		}
		if err := l.Release(); err != nil {
			t.Errorf("重复 Release 应当什么也不做：%v", err)
		}
		if _, err := os.Stat(d.Port()); err == nil {
			t.Error("Release 后 port 文件应当删除")
		}
		l3, inst, err := Acquire(d)
		if err != nil || l3 == nil || inst != nil {
			t.Fatalf("释放后应当能再拿到：%v %v", inst, err)
		}
		if err := l3.Release(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("B0704-T2 崩溃释放", func(t *testing.T) {
		d := openDir(t)
		cmd := exec.Command(os.Args[0], "-test.run=^TestLockHelper$", "-test.count=1")
		cmd.Env = append(os.Environ(), helperEnv+"=1", helperDirEnv+"="+d.Root)
		out, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
		ready := make(chan string, 1)
		go func() {
			sc := bufio.NewScanner(out)
			for sc.Scan() {
				if sc.Text() == "locked" {
					ready <- "locked"
					return
				}
			}
			ready <- "子进程退出了"
		}()
		select {
		case s := <-ready:
			if s != "locked" {
				t.Fatal(s)
			}
		case <-time.After(20 * time.Second):
			t.Fatal("等子进程取锁超时")
		}
		_, inst, err := Acquire(d)
		if err != nil || inst == nil {
			t.Fatalf("子进程持有锁时应当拿不到：%v %v", inst, err)
		}
		if inst.PID != cmd.Process.Pid {
			t.Errorf("读出的 PID 是 %d，应当是子进程的 %d", inst.PID, cmd.Process.Pid)
		}
		if err := cmd.Process.Kill(); err != nil {
			t.Fatal(err)
		}
		_ = cmd.Wait()
		var l *Lock
		for i := 0; i < 50 && l == nil; i++ {
			if l, _, err = Acquire(d); err != nil {
				t.Fatal(err)
			}
			if l == nil {
				time.Sleep(100 * time.Millisecond)
			}
		}
		if l == nil {
			t.Fatal("子进程被杀后应当能拿到锁")
		}
		if err := l.Release(); err != nil {
			t.Fatal(err)
		}
	})
}

func portOf(t *testing.T, raw string) int {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	n, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestProbe(t *testing.T) {
	t.Run("B0704-T3 存活探测", func(t *testing.T) {
		ctx := context.Background()
		ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/health" {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write([]byte(`{"ok":true,"version":"0.0.0"}`))
		}))
		defer ok.Close()
		if !Probe(ctx, portOf(t, ok.URL)) {
			t.Error("正常的实例应当探测为存活")
		}
		bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer bad.Close()
		if Probe(ctx, portOf(t, bad.URL)) {
			t.Error("返回 500 应当是 false")
		}

		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = ln.Close() }()
		held := make(chan net.Conn, 8)
		go func() {
			for {
				c, err := ln.Accept()
				if err != nil {
					return
				}
				held <- c
			}
		}()
		defer func() {
			for len(held) > 0 {
				_ = (<-held).Close()
			}
		}()
		tctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
		defer cancel()
		start := time.Now()
		if Probe(tctx, ln.Addr().(*net.TCPAddr).Port) {
			t.Error("不回应的端口应当是 false")
		}
		if d := time.Since(start); d > 3*time.Second {
			t.Errorf("超时应当由 ctx 决定，用了 %v", d)
		}

		closed, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		cport := closed.Addr().(*net.TCPAddr).Port
		_ = closed.Close()
		if Probe(ctx, cport) {
			t.Error("没人听的端口应当是 false")
		}
		if Probe(ctx, 0) {
			t.Error("端口 0 应当是 false")
		}
	})
}
