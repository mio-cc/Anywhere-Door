package sched

import (
	"context"
	"maps"
	"sync"
	"sync/atomic"
)

// sem 是计数信号量：channel 的容量就是上限。
type sem chan struct{}

func newSem(n int) sem { return make(sem, n) }

func (s sem) tryAcquire() bool {
	select {
	case s <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s sem) acquire(ctx context.Context) error {
	select {
	case s <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s sem) release() { <-s }

// DefaultGlobal 是全局并发的默认值（G.11 pools.global_concurrency）。
const DefaultGlobal = 8

// PoolConfig 是一个资源池的配额。
type PoolConfig struct {
	Concurrency int // ≤ 0：不限
	PerMinute   int // ≤ 0：不限速
	Burst       int
}

// Config 是全部资源池的配置（E.15.02 §4）。
type Config struct {
	Global  int                   // 全局并发；≤ 0 时取 DefaultGlobal
	Default PoolConfig            // 没单独配的资源池用它
	Pools   map[string]PoolConfig // 键：llm:<provider>/<model> | tool:<id>
}

// Pools 管资源池：Reconfigure 之后取的快照用新配置，已取的快照不变（下一次运行生效）。
type Pools struct {
	mu    sync.Mutex
	clock Clock
	gen   *generation
}

type generation struct {
	cfg    Config
	clock  Clock
	global *Gate
	mu     sync.Mutex
	pools  map[string]*Pool
}

// NewPools 用 cfg 建资源池。
func NewPools(cfg Config, clock Clock) *Pools {
	return &Pools{clock: clock, gen: newGeneration(cfg, clock)}
}

func newGeneration(cfg Config, clock Clock) *generation {
	g := cfg.Global
	if g <= 0 {
		g = DefaultGlobal
	}
	cfg.Pools = maps.Clone(cfg.Pools)
	return &generation{cfg: cfg, clock: clock, global: &Gate{sem: newSem(g)}, pools: map[string]*Pool{}}
}

// Reconfigure 换配置：之后取的快照用新配置。
func (ps *Pools) Reconfigure(cfg Config) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.gen = newGeneration(cfg, ps.clock)
}

// Snapshot 是一次运行看到的资源池（运行开始时取一份）。
func (ps *Pools) Snapshot() *PoolSet {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return &PoolSet{gen: ps.gen}
}

// PoolSet 是同一份配置下的资源池。
type PoolSet struct{ gen *generation }

// Global 是全局并发：同一份配置的所有快照共享。
func (s *PoolSet) Global() *Gate { return s.gen.global }

// Get 是这个键的资源池；同一份配置里同一个键总是同一个池。
func (s *PoolSet) Get(key string) *Pool {
	g := s.gen
	g.mu.Lock()
	defer g.mu.Unlock()
	if p := g.pools[key]; p != nil {
		return p
	}
	c, ok := g.cfg.Pools[key]
	if !ok {
		c = g.cfg.Default
	}
	p := &Pool{key: key}
	if c.Concurrency > 0 {
		p.sem = newSem(c.Concurrency)
	}
	if c.PerMinute > 0 {
		p.rate = NewRateLimiter(c.PerMinute, c.Burst, g.clock)
	}
	g.pools[key] = p
	return p
}

// Gate 是全局并发的信号量。
type Gate struct{ sem sem }

// TryAcquire 不等待：满了返回 false。release 可以多次调用，只释放一次。
func (g *Gate) TryAcquire() (release func(), ok bool) {
	if !g.sem.tryAcquire() {
		return nil, false
	}
	return onceRelease(g.sem), true
}

func onceRelease(s sem) func() {
	var once sync.Once
	return func() { once.Do(s.release) }
}

// Pool 是一个模型或工具的资源池：并发上限加令牌桶。
type Pool struct {
	key     string
	sem     sem          // nil：不限并发
	rate    *RateLimiter // nil：不限速
	waiting atomic.Int64
}

// Key 是资源池的键。
func (p *Pool) Key() string { return p.key }

// Waiting 是正在等的调用数。
func (p *Pool) Waiting() int64 { return p.waiting.Load() }

// Acquire 等到能调用：先等并发、再等令牌。要等时调一次 onWait(等待数)（由调用方发 node.throttled）。
// ctx 取消时立刻返回 ctx 的错误，不占名额。
func (p *Pool) Acquire(ctx context.Context, onWait func(waiting int64)) (release func(), err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	waited := false
	wait := func() {
		if !waited {
			waited = true
			n := p.waiting.Add(1)
			if onWait != nil {
				onWait(n)
			}
		}
	}
	defer func() {
		if waited {
			p.waiting.Add(-1)
		}
	}()
	release = func() {}
	if p.sem != nil {
		if !p.sem.tryAcquire() {
			wait()
			if err := p.sem.acquire(ctx); err != nil {
				return nil, err
			}
		}
		release = onceRelease(p.sem)
	}
	if p.rate != nil && !p.rate.TryTake() {
		wait()
		if err := p.rate.Wait(ctx); err != nil {
			release()
			return nil, err
		}
	}
	return release, nil
}

// Ready 是派发时的粗过滤：并发和令牌都有余量（不占名额）。
func (p *Pool) Ready() bool {
	if p.sem != nil {
		if len(p.sem) >= cap(p.sem) {
			return false
		}
	}
	return p.rate == nil || p.rate.Ready()
}
