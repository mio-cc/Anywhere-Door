// Package lock 是单实例锁（B.07.04）：同一个数据目录只有一个 anydoor 在运行。
// 转交给已运行的实例（开浏览器、重试、提示）在 cmd/anydoor（B.02 §5）。
package lock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/mio-cc/anywhere-door/internal/platform/datadir"
)

// Instance 是已经在运行的实例；Port 为 0 表示对方还没写端口文件。
type Instance struct {
	PID  int
	Port int
}

// Lock 是持有中的锁。
type Lock struct {
	f    *os.File
	d    datadir.Dir
	done bool
}

// Acquire 取数据目录的锁。拿到：返回 *Lock；已被别的实例持有：返回 nil, *Instance, nil。
func Acquire(d datadir.Dir) (*Lock, *Instance, error) {
	p := d.Lock()
	f, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("lock: 打不开 %s：%w", p, err)
	}
	ok, err := tryLock(f)
	if err != nil {
		_ = f.Close()
		return nil, nil, fmt.Errorf("lock: 锁不了 %s：%w", p, err)
	}
	if !ok {
		_ = f.Close()
		return nil, &Instance{PID: readInt(p), Port: readInt(d.Port())}, nil
	}
	l := &Lock{f: f, d: d}
	if err := l.writePID(); err != nil {
		_ = unlock(f)
		_ = f.Close()
		return nil, nil, fmt.Errorf("lock: 写 %s：%w", p, err)
	}
	return l, nil, nil
}

func (l *Lock) writePID() error {
	if err := datadir.Restrict(l.d.Lock()); err != nil {
		return err
	}
	if err := l.f.Truncate(0); err != nil {
		return err
	}
	if _, err := l.f.WriteAt([]byte(strconv.Itoa(os.Getpid())+"\n"), 0); err != nil {
		return err
	}
	return l.f.Sync()
}

// readInt 读文件里的十进制整数；读不到或格式不对时为 0。
func readInt(p string) int {
	b, err := os.ReadFile(p)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// WritePort 写 <DataDir>/port。
func (l *Lock) WritePort(port int) error {
	if l.done {
		return errors.New("lock: 锁已经释放")
	}
	if err := datadir.WritePrivate(l.d.Port(), []byte(strconv.Itoa(port)+"\n")); err != nil {
		return fmt.Errorf("lock: 写端口文件：%w", err)
	}
	return nil
}

// Release 删 port 文件、解锁、关闭锁文件（锁文件本身留着）；重复调用什么也不做。
func (l *Lock) Release() error {
	if l.done {
		return nil
	}
	l.done = true
	var errs []error
	if err := os.Remove(l.d.Port()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		errs = append(errs, err)
	}
	errs = append(errs, unlock(l.f), l.f.Close())
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("lock: 释放：%w", err)
	}
	return nil
}

// Probe 请求 http://127.0.0.1:<port>/api/v1/health，200 且 {"ok": true} 为存活；超时由 ctx 决定。
func Probe(ctx context.Context, port int) bool {
	if port <= 0 || port > 65535 {
		return false
	}
	c := &http.Client{
		Transport:     &http.Transport{Proxy: nil, DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(port)+"/api/v1/health", nil)
	if err != nil {
		return false
	}
	resp, err := c.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var v struct {
		OK bool `json:"ok"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1024)).Decode(&v); err != nil {
		return false
	}
	return v.OK
}
