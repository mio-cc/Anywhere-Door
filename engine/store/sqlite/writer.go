// Package sqlite 是 store.Store 的 SQLite 实现（E.07.02–E.07.04），驱动 modernc.org/sqlite。
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

var errWriterClosed = errors.New("sqlite: 写队列已关闭")

// 写事务的超时和重试（E.07.04 §5）。
const (
	slowTx      = time.Second
	txTimeout   = 5 * time.Second
	busyRetries = 3
	busyBackoff = 50 * time.Millisecond
)

type writeReq struct {
	ctx  context.Context
	fn   func(*sql.Tx) error
	done chan error
}

// writer 是单写者队列：一个写连接，所有写事务由一个 goroutine 串行执行（E.07.04）。
type writer struct {
	db     *sql.DB
	log    *slog.Logger
	reqs   chan writeReq
	mu     sync.Mutex
	closed bool
	quit   chan struct{} // Close 关闭它：goroutine 处理完已排队的请求后退出
	exited chan struct{}
}

// newWriter 启动写队列；db 的 MaxOpenConns 必须是 1。
func newWriter(db *sql.DB, log *slog.Logger) *writer {
	if log == nil {
		log = slog.Default()
	}
	w := &writer{db: db, log: log, reqs: make(chan writeReq, 256), quit: make(chan struct{}), exited: make(chan struct{})}
	go w.loop()
	return w
}

func (w *writer) loop() {
	defer close(w.exited)
	for {
		select {
		case r := <-w.reqs:
			w.serve(r)
		case <-w.quit:
			for {
				select {
				case r := <-w.reqs:
					w.serve(r)
				default:
					return
				}
			}
		}
	}
}

func (w *writer) serve(r writeReq) {
	if err := r.ctx.Err(); err != nil {
		r.done <- err // 排队时已取消：不执行
		return
	}
	var err error
	for attempt := 0; ; attempt++ {
		err = w.run(r)
		if !isBusy(err) || attempt == busyRetries {
			break
		}
		time.Sleep(busyBackoff)
	}
	if isBusy(err) {
		w.log.Error("sqlite: 写事务重试后仍然忙", "err", err)
	}
	r.done <- err
}

// run 执行一个写事务；fn panic 时回滚并转成错误。
func (w *writer) run(r writeReq) (err error) {
	ctx, cancel := context.WithTimeout(r.ctx, txTimeout)
	defer cancel()
	start := time.Now()
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: 开始写事务：%w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("sqlite: 写事务里 panic：%v", p)
		}
		if err != nil {
			_ = tx.Rollback()
		}
		switch d := time.Since(start); {
		case errors.Is(err, context.DeadlineExceeded) && r.ctx.Err() == nil:
			w.log.Error("sqlite: 写事务超时，已回滚", "elapsed", d)
		case d > slowTx:
			w.log.Warn("sqlite: 写事务太慢", "elapsed", d)
		}
	}()
	if err = r.fn(tx); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: 提交写事务：%w", err)
	}
	return nil
}

func isBusy(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "SQLITE_BUSY") || strings.Contains(s, "database is locked")
}

// Do 把 fn 放进写队列，等它执行完；fn 的错误原样返回。
func (w *writer) Do(ctx context.Context, fn func(*sql.Tx) error) error {
	r := writeReq{ctx: ctx, fn: fn, done: make(chan error, 1)}
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return errWriterClosed
	}
	select {
	case w.reqs <- r:
		w.mu.Unlock()
	case <-ctx.Done():
		w.mu.Unlock()
		return ctx.Err()
	}
	select {
	case err := <-r.done:
		return err
	case <-ctx.Done():
		// 已经排队：还没开始的不会执行，已开始的随 ctx 取消回滚；done 有缓冲，不会卡住写队列
		return ctx.Err()
	}
}

// Close 不再接收新请求，把已排队的执行完；超过 timeout 返回错误。
func (w *writer) Close(timeout time.Duration) error {
	w.mu.Lock()
	if !w.closed {
		w.closed = true
		close(w.quit)
	}
	w.mu.Unlock()
	select {
	case <-w.exited:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("sqlite: 关闭写队列超过 %v", timeout)
	}
}
