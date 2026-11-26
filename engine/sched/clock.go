// Package sched 是调度的基础件（设计文档 E.15）：时钟、令牌桶限流、并发与资源池、就绪判定与优先队列。
// 运行的调度循环（E.16.01）用它们；本包不知道运行和执行器的细节。
package sched

import "time"

// Clock 是内核取时间的唯一方式（E.15-R3）：生产用 RealClock，测试用 NewVirtualClock。
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
	NewTimer(d time.Duration) Timer
	NewTicker(d time.Duration) Ticker
}

// Timer 同 time.Timer。
type Timer interface {
	C() <-chan time.Time
	Stop() bool
	Reset(d time.Duration) bool
}

// Ticker 同 time.Ticker。
type Ticker interface {
	C() <-chan time.Time
	Stop()
}

// RealClock 包装 time 包（本文件是唯一可以直接调用 time.Now 等的地方，E.15.04 §5）。
var RealClock Clock = realClock{}

type realClock struct{}

func (realClock) Now() time.Time                         { return time.Now() }
func (realClock) After(d time.Duration) <-chan time.Time { return time.After(d) }
func (realClock) NewTimer(d time.Duration) Timer         { return realTimer{time.NewTimer(d)} }
func (realClock) NewTicker(d time.Duration) Ticker       { return realTicker{time.NewTicker(d)} }

type realTimer struct{ t *time.Timer }

func (r realTimer) C() <-chan time.Time        { return r.t.C }
func (r realTimer) Stop() bool                 { return r.t.Stop() }
func (r realTimer) Reset(d time.Duration) bool { return r.t.Reset(d) }

type realTicker struct{ t *time.Ticker }

func (r realTicker) C() <-chan time.Time { return r.t.C }
func (r realTicker) Stop()               { r.t.Stop() }
