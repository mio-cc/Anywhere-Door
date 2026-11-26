package sched

import (
	"sync"
	"time"
)

// VirtualClock 是测试用的时钟（E.15.04）：时间只在 Advance 时往前走，到期的定时器按到期时间依次触发。
type VirtualClock struct {
	mu     sync.Mutex
	cond   *sync.Cond
	now    time.Time
	seq    int64
	timers []*vtimer // 在等的定时器
}

type vtimer struct {
	v      *VirtualClock
	c      chan time.Time
	at     time.Time
	period time.Duration // > 0：Ticker
	seq    int64
	active bool
}

// NewVirtualClock 从 start 开始。
func NewVirtualClock(start time.Time) *VirtualClock {
	v := &VirtualClock{now: start}
	v.cond = sync.NewCond(&v.mu)
	return v
}

// Now 是当前的虚拟时间。
func (v *VirtualClock) Now() time.Time {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.now
}

// After 同 time.After。
func (v *VirtualClock) After(d time.Duration) <-chan time.Time { return v.add(d, 0).c }

// NewTimer 同 time.NewTimer。
func (v *VirtualClock) NewTimer(d time.Duration) Timer { return vTimer{v.add(d, 0)} }

// NewTicker 同 time.NewTicker：d ≤ 0 时 panic。
func (v *VirtualClock) NewTicker(d time.Duration) Ticker {
	if d <= 0 {
		panic("sched: NewTicker 的周期必须大于 0")
	}
	return vTicker{v.add(d, d)}
}

func (v *VirtualClock) add(d, period time.Duration) *vtimer {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.seq++
	t := &vtimer{v: v, c: make(chan time.Time, 1), period: period, seq: v.seq}
	v.schedule(t, d)
	return t
}

// schedule 在锁里调用：到期时间不晚于现在的一次性定时器立刻触发，其余登记。
func (v *VirtualClock) schedule(t *vtimer, d time.Duration) {
	t.at = v.now.Add(d)
	if d <= 0 && t.period == 0 {
		t.active = false
		send(t.c, v.now)
		return
	}
	if !t.active {
		t.active = true
		v.timers = append(v.timers, t)
	}
	v.cond.Broadcast()
}

func send(c chan time.Time, at time.Time) {
	select {
	case c <- at:
	default: // 和 time 包一样：没人取就丢
	}
}

func (v *VirtualClock) remove(t *vtimer) {
	t.active = false
	out := v.timers[:0]
	for _, x := range v.timers {
		if x != t {
			out = append(out, x)
		}
	}
	v.timers = out
}

// Advance 把时间往前推 d，途中到期的定时器按到期时间（同时到期按创建顺序）依次触发。
func (v *VirtualClock) Advance(d time.Duration) {
	v.mu.Lock()
	defer v.mu.Unlock()
	target := v.now.Add(d)
	for {
		var next *vtimer
		for _, t := range v.timers {
			if t.at.After(target) {
				continue
			}
			if next == nil || t.at.Before(next.at) || (t.at.Equal(next.at) && t.seq < next.seq) {
				next = t
			}
		}
		if next == nil {
			break
		}
		if next.at.After(v.now) {
			v.now = next.at
		}
		send(next.c, v.now)
		if next.period > 0 {
			next.at = next.at.Add(next.period)
		} else {
			v.remove(next)
		}
	}
	v.now = target
}

// BlockUntil 等到至少有 n 个定时器在等（避免测试里"还没登记就 Advance"的竞态）。
func (v *VirtualClock) BlockUntil(n int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	for len(v.timers) < n {
		v.cond.Wait()
	}
}

type vTimer struct{ t *vtimer }

func (x vTimer) C() <-chan time.Time { return x.t.c }

func (x vTimer) Stop() bool {
	v := x.t.v
	v.mu.Lock()
	defer v.mu.Unlock()
	was := x.t.active
	if was {
		v.remove(x.t)
	}
	return was
}

func (x vTimer) Reset(d time.Duration) bool {
	v := x.t.v
	v.mu.Lock()
	defer v.mu.Unlock()
	was := x.t.active
	if was && d <= 0 {
		v.remove(x.t)
	}
	v.schedule(x.t, d)
	return was
}

type vTicker struct{ t *vtimer }

func (x vTicker) C() <-chan time.Time { return x.t.c }

func (x vTicker) Stop() {
	v := x.t.v
	v.mu.Lock()
	defer v.mu.Unlock()
	if x.t.active {
		v.remove(x.t)
	}
}
