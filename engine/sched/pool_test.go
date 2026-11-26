package sched

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPools(t *testing.T) {
	t.Run("E1502-T1 并发上限", func(t *testing.T) {
		ps := NewPools(Config{Pools: map[string]PoolConfig{"llm:a/b": {Concurrency: 4}}}, RealClock)
		p := ps.Snapshot().Get("llm:a/b")
		var inflight, peak atomic.Int64
		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				rel, err := p.Acquire(context.Background(), nil)
				if err != nil {
					t.Error(err)
					return
				}
				n := inflight.Add(1)
				for {
					m := peak.Load()
					if n <= m || peak.CompareAndSwap(m, n) {
						break
					}
				}
				time.Sleep(2 * time.Millisecond)
				inflight.Add(-1)
				rel()
			}()
		}
		wg.Wait()
		if peak.Load() > 4 || peak.Load() == 0 {
			t.Errorf("同时在飞最多 4 个，实际峰值 %d", peak.Load())
		}
		var rels []func()
		for i := 0; i < 4; i++ {
			r, _ := p.Acquire(context.Background(), nil)
			rels = append(rels, r)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := p.Acquire(ctx, nil); !errors.Is(err, context.Canceled) {
			t.Errorf("ctx 取消时立刻返回：%v", err)
		}
		if p.Ready() || p.Waiting() != 0 {
			t.Errorf("满了不 Ready，取消的不算等待：%v %d", p.Ready(), p.Waiting())
		}
		for _, r := range rels {
			r()
			r() // 多次调用只释放一次
		}
		if !p.Ready() {
			t.Error("释放后应当 Ready")
		}
	})

	t.Run("E1502-T2 全局上限", func(t *testing.T) {
		ps := NewPools(Config{Global: 2}, RealClock)
		a, b := ps.Snapshot(), ps.Snapshot()
		r1, ok1 := a.Global().TryAcquire()
		_, ok2 := b.Global().TryAcquire()
		_, ok3 := a.Global().TryAcquire()
		if !ok1 || !ok2 || ok3 {
			t.Fatalf("全局上限 2、两个快照共享：%v %v %v", ok1, ok2, ok3)
		}
		r1()
		if _, ok := b.Global().TryAcquire(); !ok {
			t.Error("释放后应当能拿到")
		}
		if cap(NewPools(Config{}, RealClock).Snapshot().Global().sem) != DefaultGlobal {
			t.Error("默认全局并发应当是 8")
		}
	})

	t.Run("E1502-T3 热更新", func(t *testing.T) {
		ps := NewPools(Config{Default: PoolConfig{Concurrency: 1}}, RealClock)
		old := ps.Snapshot()
		hold, _ := old.Get("tool:x").Acquire(context.Background(), nil)
		ps.Reconfigure(Config{Default: PoolConfig{Concurrency: 3}})
		if old.Get("tool:x").Ready() {
			t.Error("旧快照还是旧配置（上限 1）")
		}
		fresh := ps.Snapshot().Get("tool:x")
		var rels []func()
		for i := 0; i < 3; i++ {
			if !fresh.Ready() {
				t.Fatalf("新快照用新配置（上限 3），第 %d 个就满了", i+1)
			}
			r, _ := fresh.Acquire(context.Background(), nil)
			rels = append(rels, r)
		}
		if fresh.Ready() {
			t.Error("第 4 个应当满了")
		}
		hold()
		for _, r := range rels {
			r()
		}
	})

	t.Run("E1502-T4 等待回调", func(t *testing.T) {
		ps := NewPools(Config{Pools: map[string]PoolConfig{"c": {Concurrency: 1}}}, RealClock)
		p := ps.Snapshot().Get("c")
		first, _ := p.Acquire(context.Background(), nil)
		got := make(chan int64, 1)
		done := make(chan struct{})
		go func() {
			rel, err := p.Acquire(context.Background(), func(n int64) { got <- n })
			if err == nil {
				rel()
			}
			close(done)
		}()
		if n := <-got; n != 1 {
			t.Errorf("并发满时 onWait 收到等待数 1，实际 %d", n)
		}
		first()
		<-done
		if p.Waiting() != 0 {
			t.Errorf("拿到后等待数减一：%d", p.Waiting())
		}

		v := NewVirtualClock(t0)
		rp := NewPools(Config{Default: PoolConfig{PerMinute: 60, Burst: 1}}, v).Snapshot().Get("llm:p/m")
		if _, err := rp.Acquire(context.Background(), func(int64) { t.Error("有令牌时不应当回调") }); err != nil {
			t.Fatal(err)
		}
		waited := make(chan int64, 1)
		finished := make(chan error, 1)
		go func() {
			_, err := rp.Acquire(context.Background(), func(n int64) { waited <- n })
			finished <- err
		}()
		if n := <-waited; n != 1 {
			t.Errorf("没有令牌时也回调：%d", n)
		}
		v.BlockUntil(1)
		v.Advance(time.Second)
		if err := <-finished; err != nil {
			t.Errorf("一秒后应当拿到：%v", err)
		}
	})
}
