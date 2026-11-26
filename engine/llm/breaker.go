package llm

import (
	"sync"
	"time"
)

// BreakerCfg 是熔断的设置（E.10.09 §5）。
type BreakerCfg struct {
	Window      time.Duration // 统计窗口
	MinRequests int           // 窗口内至少这么多请求才判断
	ErrorRate   float64       // 失败率超过它就打开
	Open        time.Duration // 打开多久后半开
}

// DefaultBreaker 是缺省的熔断设置。
var DefaultBreaker = BreakerCfg{Window: time.Minute, MinRequests: 20, ErrorRate: 0.5, Open: 30 * time.Second}

// Outcome 是一次调用对熔断的意义。
type Outcome int

// 调用结果：Neutral 不计入失败率（user_input、format_invalid、cancelled 等）。
const (
	Success Outcome = iota
	Failure
	Neutral
)

// 熔断状态。
const (
	StateClosed   = "closed"
	StateOpen     = "open"
	StateHalfOpen = "half_open"
)

// Breakers 是每个 Target 一个熔断器。nil 表示不熔断。
type Breakers struct {
	cfg BreakerCfg
	mu  sync.Mutex
	m   map[Target]*Breaker
}

// NewBreakers 建熔断器表；c.Window ≤ 0 时用 DefaultBreaker。
func NewBreakers(c BreakerCfg) *Breakers {
	if c.Window <= 0 {
		c = DefaultBreaker
	}
	return &Breakers{cfg: c, m: map[Target]*Breaker{}}
}

// For 是这个 Target 的熔断器；b 为 nil 时返回 nil（不熔断）。
func (b *Breakers) For(t Target) *Breaker {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	br := b.m[t]
	if br == nil {
		br = &Breaker{cfg: b.cfg, state: StateClosed}
		b.m[t] = br
	}
	return br
}

// Breaker 是一个 Target 的熔断器。方法对 nil 也能调用（总是放行）。
type Breaker struct {
	cfg      BreakerCfg
	mu       sync.Mutex
	state    string
	events   []bevent
	openedAt time.Time
	probing  bool
}

type bevent struct {
	at     time.Time
	failed bool
}

func (b *Breaker) advance(now time.Time) {
	if b.state == StateOpen && !now.Before(b.openedAt.Add(b.cfg.Open)) {
		b.state, b.probing = StateHalfOpen, false
	}
}

// State 是当前状态：closed、open、half_open。
func (b *Breaker) State(now time.Time) string {
	if b == nil {
		return StateClosed
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.advance(now)
	return b.state
}

// Allow：现在能不能发请求。半开时只放一个试探请求，它有结果（Record）之前不再放。
func (b *Breaker) Allow(now time.Time) bool {
	if b == nil {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.advance(now)
	switch b.state {
	case StateOpen:
		return false
	case StateHalfOpen:
		if b.probing {
			return false
		}
		b.probing = true
	}
	return true
}

// Record 记一次调用的结果。
func (b *Breaker) Record(now time.Time, o Outcome) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.advance(now)
	switch b.state {
	case StateOpen:
		return
	case StateHalfOpen:
		b.probing = false
		switch o {
		case Success:
			b.state, b.events = StateClosed, nil
		case Failure:
			b.state, b.openedAt = StateOpen, now
		}
		return
	}
	if o == Neutral {
		return
	}
	b.events = append(b.events, bevent{at: now, failed: o == Failure})
	cut := now.Add(-b.cfg.Window)
	k := 0
	for k < len(b.events) && !b.events[k].at.After(cut) {
		k++
	}
	b.events = b.events[k:]
	if len(b.events) < b.cfg.MinRequests {
		return
	}
	fails := 0
	for _, e := range b.events {
		if e.failed {
			fails++
		}
	}
	if float64(fails)/float64(len(b.events)) > b.cfg.ErrorRate {
		b.state, b.openedAt, b.events = StateOpen, now, nil
	}
}

func outcome(e *Error) Outcome {
	if e == nil {
		return Success
	}
	if retryable(e) {
		return Failure
	}
	return Neutral
}
