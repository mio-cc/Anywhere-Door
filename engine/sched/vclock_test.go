package sched

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)

func fired(c <-chan time.Time) (time.Time, bool) {
	select {
	case at := <-c:
		return at, true
	default:
		return time.Time{}, false
	}
}

func TestVirtualClock(t *testing.T) {
	t.Run("E1504-T1 定时器顺序", func(t *testing.T) {
		v := NewVirtualClock(t0)
		a := v.After(3 * time.Second)
		b := v.NewTimer(time.Second)
		c := v.After(2 * time.Second)
		stopped := v.NewTimer(time.Second)
		if !stopped.Stop() || stopped.Stop() {
			t.Error("Stop 第一次返回 true、第二次返回 false")
		}
		v.Advance(5 * time.Second)
		for i, ch := range []<-chan time.Time{b.C(), c, a} {
			at, ok := fired(ch)
			if !ok || !at.Equal(t0.Add(time.Duration(i+1)*time.Second)) {
				t.Errorf("第 %d 个定时器应当在 %ds 触发：%v %v", i+1, i+1, at, ok)
			}
		}
		if _, ok := fired(stopped.C()); ok {
			t.Error("Stop 之后不应当触发")
		}
		if !v.Now().Equal(t0.Add(5 * time.Second)) {
			t.Errorf("Advance 后的时间不对：%v", v.Now())
		}
		if b.Reset(time.Second) {
			t.Error("已触发的定时器 Reset 返回 false")
		}
		v.Advance(time.Second)
		if _, ok := fired(b.C()); !ok {
			t.Error("Reset 之后应当再触发一次")
		}
		tk := v.NewTicker(time.Minute)
		n := 0
		for i := 0; i < 3; i++ {
			v.Advance(time.Minute)
			if _, ok := fired(tk.C()); ok {
				n++
			}
		}
		tk.Stop()
		v.Advance(time.Minute)
		if _, ok := fired(tk.C()); ok || n != 3 {
			t.Errorf("Ticker 每个周期一次、Stop 后不再触发：%d", n)
		}
		day := v.After(24 * time.Hour)
		v.Advance(24 * time.Hour)
		if _, ok := fired(day); !ok {
			t.Error("24 小时的 After 应当在 Advance 后立刻触发")
		}
		if _, ok := fired(v.After(0)); !ok {
			t.Error("After(0) 应当立刻触发")
		}
	})

	t.Run("E1504-T2 BlockUntil", func(t *testing.T) {
		v := NewVirtualClock(t0)
		done := make(chan time.Time)
		go func() { done <- <-v.After(time.Hour) }()
		v.BlockUntil(1)
		v.Advance(time.Hour)
		if at := <-done; !at.Equal(t0.Add(time.Hour)) {
			t.Errorf("等待的 goroutine 应当在 1 小时后醒来：%v", at)
		}
	})
}
