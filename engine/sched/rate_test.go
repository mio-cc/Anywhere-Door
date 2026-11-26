package sched

import (
	"context"
	"errors"
	"testing"
	"time"
)

// takeFor 每 100ms 尽量多拿令牌，共 d 时长，返回拿到几个。
func takeFor(v *VirtualClock, r *RateLimiter, d time.Duration) int {
	n := 0
	for el := time.Duration(0); el < d; el += 100 * time.Millisecond {
		for r.TryTake() {
			n++
		}
		v.Advance(100 * time.Millisecond)
	}
	for r.TryTake() {
		n++
	}
	return n
}

func TestRateLimiter(t *testing.T) {
	t.Run("E1503-T1 速率", func(t *testing.T) {
		v := NewVirtualClock(t0)
		r := NewRateLimiter(60, 5, v)
		if n := takeFor(v, r, time.Minute); n < 64 || n > 65 {
			t.Errorf("60 rpm、burst 5 一分钟应当拿到 64–65 个，实际 %d", n)
		}
		if r.Ready() {
			t.Error("刚拿完不应当还有令牌")
		}
		done := make(chan error, 1)
		go func() { done <- r.Wait(context.Background()) }()
		v.BlockUntil(1)
		v.Advance(time.Second)
		if err := <-done; err != nil {
			t.Errorf("一秒后应当拿到：%v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		go func() { done <- r.Wait(ctx) }()
		v.BlockUntil(1)
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("ctx 取消时应当立刻返回：%v", err)
		}
		if !NewRateLimiter(0, 0, v).TryTake() {
			t.Error("perMinute 为 0 时不限速")
		}
	})

	t.Run("E1503-T2 降速恢复", func(t *testing.T) {
		v := NewVirtualClock(t0)
		r := NewRateLimiter(60, 1, v)
		r.TryTake()
		r.Penalize(0.5, 5*time.Minute)
		if n := takeFor(v, r, time.Minute); n < 29 || n > 31 {
			t.Errorf("降速一半后一分钟应当约 30 个，实际 %d", n)
		}
		v.Advance(4 * time.Minute)
		for r.TryTake() {
		}
		if n := takeFor(v, r, time.Minute); n < 59 || n > 61 {
			t.Errorf("5 分钟后应当恢复到约 60 个，实际 %d", n)
		}
		r.Penalize(0.8, time.Minute)
		r.Penalize(0.5, time.Minute)
		for r.TryTake() {
		}
		if n := takeFor(v, r, 30*time.Second); n < 14 || n > 16 {
			t.Errorf("两次叠加取最低（0.5），30 秒应当约 15 个，实际 %d", n)
		}
	})
}
