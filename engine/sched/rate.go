package sched

import (
	"context"
	"math"
	"sync"
	"time"
)

// eps 吸收浮点误差：差一点点到 1 个令牌也算有（否则虚拟时钟下可能要多推一次）。
const eps = 1e-9

// RateLimiter 是令牌桶（E.15.03）：只有基于注入 Clock 的一种实现，生产和测试用同一套代码（P.09 K265）。
type RateLimiter struct {
	mu     sync.Mutex
	clock  Clock
	perSec float64 // ≤ 0：不限速
	burst  float64
	tokens float64
	last   time.Time
	factor float64   // 降速倍数（Penalize）
	until  time.Time // 降速到什么时候；零值表示没有降速
}

// NewRateLimiter：perMinute ≤ 0 时不限速；burst ≤ 0 时为 1；开始时令牌是满的。
func NewRateLimiter(perMinute, burst int, clock Clock) *RateLimiter {
	if burst <= 0 {
		burst = 1
	}
	return &RateLimiter{clock: clock, perSec: float64(perMinute) / 60, burst: float64(burst), tokens: float64(burst),
		last: clock.Now(), factor: 1}
}

func (r *RateLimiter) unlimited() bool { return r.perSec <= 0 }

func (r *RateLimiter) rateAt(t time.Time) float64 {
	if !r.until.IsZero() && t.Before(r.until) {
		return r.perSec * r.factor
	}
	return r.perSec
}

// refill 在锁里调用：降速期间和之后分段补充令牌。
func (r *RateLimiter) refill(now time.Time) {
	for r.last.Before(now) {
		end, rate := now, r.rateAt(r.last)
		if !r.until.IsZero() && r.last.Before(r.until) && r.until.Before(now) {
			end = r.until
		}
		r.tokens = math.Min(r.burst, r.tokens+end.Sub(r.last).Seconds()*rate)
		r.last = end
	}
	if !r.until.IsZero() && !now.Before(r.until) {
		r.factor, r.until = 1, time.Time{}
	}
}

// TryTake 有令牌就拿一个，不等待。
func (r *RateLimiter) TryTake() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.unlimited() {
		return true
	}
	r.refill(r.clock.Now())
	if r.tokens >= 1-eps {
		r.tokens = max(0, r.tokens-1)
		return true
	}
	return false
}

// Ready：现在有没有令牌（不拿），派发时的粗过滤。
func (r *RateLimiter) Ready() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.unlimited() {
		return true
	}
	r.refill(r.clock.Now())
	return r.tokens >= 1-eps
}

// Wait 等到拿到一个令牌；ctx 取消时立刻返回 ctx 的错误。
func (r *RateLimiter) Wait(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		r.mu.Lock()
		if r.unlimited() {
			r.mu.Unlock()
			return nil
		}
		now := r.clock.Now()
		r.refill(now)
		if r.tokens >= 1-eps {
			r.tokens = max(0, r.tokens-1)
			r.mu.Unlock()
			return nil
		}
		d := time.Duration(math.Ceil((1 - r.tokens) / r.rateAt(now) * float64(time.Second)))
		r.mu.Unlock()
		if d <= 0 {
			d = time.Nanosecond
		}
		t := r.clock.NewTimer(d)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C():
		}
	}
}

// Penalize 把速率乘以 factor（0–1），d 之后恢复；多次叠加取最低的速率、最晚的恢复时间（E.15.03 §5）。
func (r *RateLimiter) Penalize(factor float64, d time.Duration) {
	if factor <= 0 {
		factor = 0.01
	}
	if factor > 1 {
		factor = 1
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.clock.Now()
	r.refill(now)
	if r.until.IsZero() {
		r.factor = factor
	} else {
		r.factor = math.Min(r.factor, factor)
	}
	if u := now.Add(d); u.After(r.until) {
		r.until = u
	}
}
