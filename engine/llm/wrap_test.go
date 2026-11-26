package llm_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mio-cc/anywhere-door/engine/llm"
	"github.com/mio-cc/anywhere-door/engine/llm/fake"
	"github.com/mio-cc/anywhere-door/engine/sched"
	"github.com/mio-cc/anywhere-door/gen"
)

var t0 = time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)

// clock 是注入的时间：Sleep 只记下时长并把时间往前推。
type clock struct {
	mu    sync.Mutex
	now   time.Time
	slept []time.Duration
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.slept = append(c.slept, d)
	c.now = c.now.Add(d)
	return nil
}

func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// source 是测试用的 Source：clients 的键是提供方名，或 "提供方|密钥名"。
type source struct {
	clients map[string]llm.Client
	keys    map[string][]string
	fb      map[string][]string
}

func (s *source) Client(p, _ string) (llm.Client, error) {
	if c, ok := s.clients[p]; ok {
		return c, nil
	}
	return nil, &llm.Error{Class: gen.FailureClassUserInput, Provider: p, Code: "model_not_found", Message: "没有这个模型"}
}

func (s *source) ClientWithKey(p, _, key string) (llm.Client, error) {
	if c, ok := s.clients[p+"|"+key]; ok {
		return c, nil
	}
	return nil, &llm.Error{Class: gen.FailureClassUserInput, Provider: p, Code: llm.CodeKeyNotFound, Message: "没有这个密钥"}
}

func (s *source) Keys(p string) []string      { return s.keys[p] }
func (s *source) Fallbacks(p string) []string { return s.fb[p] }

// records 收集 OnCall。
type records struct {
	mu sync.Mutex
	rs []llm.CallRecord
}

func (r *records) add(_ context.Context, c llm.CallRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rs = append(r.rs, c)
}

func (r *records) all() []llm.CallRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.rs)
}

func fail(class gen.FailureClass) fake.Step {
	return fake.Step{Err: &llm.Error{Class: class, Code: "x", Message: "失败"}}
}

func failAfter(class gen.FailureClass, retryAfter time.Duration) fake.Step {
	return fake.Step{Err: &llm.Error{Class: class, Code: "x", Message: "失败", RetryAfter: retryAfter}}
}

func ok() fake.Step { return fake.Step{Resp: &llm.Resp{Text: "好", Finish: llm.FinishStop}} }

var req = llm.Req{Model: "m", Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{{Type: llm.PartText, Text: "hi"}}}}}

func call(c llm.Client) (*llm.Resp, *llm.Error) {
	st, err := c.Complete(context.Background(), req)
	if err == nil {
		var resp *llm.Resp
		if resp, err = llm.Collect(st); err == nil {
			return resp, nil
		}
	}
	var le *llm.Error
	if !errors.As(err, &le) {
		le = &llm.Error{Class: gen.FailureClassInternal, Message: err.Error()}
	}
	return nil, le
}

type setup struct {
	clk  *clock
	rec  *records
	src  *source
	opts llm.WrapOpts
}

func newSetup(clients map[string]llm.Client) *setup {
	s := &setup{clk: &clock{now: t0}, rec: &records{}, src: &source{clients: clients, keys: map[string][]string{}, fb: map[string][]string{}}}
	s.opts = llm.WrapOpts{Source: s.src, Now: s.clk.Now, Sleep: s.clk.Sleep, Hooks: llm.Hooks{OnCall: s.rec.add}}
	return s
}

func (s *setup) wrap(t *testing.T, provider string) llm.Client {
	t.Helper()
	c, err := llm.Wrap(llm.Target{Provider: provider, Model: "m"}, s.opts)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestRetry(t *testing.T) {
	t.Run("E1009-T1 重试分类", func(t *testing.T) {
		s := newSetup(map[string]llm.Client{"p": fake.New(llm.Caps{}, fail(gen.FailureClassTransient), fail(gen.FailureClassTransient), fail(gen.FailureClassTransient))})
		if _, e := call(s.wrap(t, "p")); e == nil || e.Class != gen.FailureClassTransient {
			t.Fatalf("重试耗尽返回最后一次的错误：%v", e)
		}
		if !slices.Equal(s.clk.slept, []time.Duration{time.Second, 2 * time.Second}) || len(s.rec.all()) != 3 || s.rec.all()[2].Attempt != 3 {
			t.Errorf("一共 3 次、等 1s、2s：%v %d", s.clk.slept, len(s.rec.all()))
		}

		s = newSetup(map[string]llm.Client{"p": fake.New(llm.Caps{}, fail(gen.FailureClassTransient), ok())})
		if resp, e := call(s.wrap(t, "p")); e != nil || resp.Text != "好" {
			t.Errorf("重试后成功：%v", e)
		}

		for _, class := range []gen.FailureClass{gen.FailureClassUserInput, gen.FailureClassFormatInvalid} {
			f := fake.New(llm.Caps{}, fail(class), ok())
			s = newSetup(map[string]llm.Client{"p": f})
			if _, e := call(s.wrap(t, "p")); e == nil || e.Class != class || len(f.Requests()) != 1 || len(s.clk.slept) != 0 {
				t.Errorf("%s 不重试：%v %d", class, e, len(f.Requests()))
			}
		}

		s = newSetup(map[string]llm.Client{"p": fake.New(llm.Caps{}, fail(gen.FailureClassTransient), ok())})
		s.opts.Rand = func() float64 { return 0 }
		if _, e := call(s.wrap(t, "p")); e != nil || !slices.Equal(s.clk.slept, []time.Duration{800 * time.Millisecond}) {
			t.Errorf("Rand 恒为 0 时等待 × 0.8：%v %v", s.clk.slept, e)
		}

		s = newSetup(map[string]llm.Client{"p": fake.New(llm.Caps{}, fail(gen.FailureClassTransient), fail(gen.FailureClassTransient),
			fail(gen.FailureClassTransient), fail(gen.FailureClassTransient), ok())})
		s.opts.Retry = llm.RetryCfg{Max: 5, Base: 20 * time.Second, MaxDelay: 30 * time.Second, MaxRetryAfter: time.Minute}
		if _, e := call(s.wrap(t, "p")); e != nil || !slices.Equal(s.clk.slept, []time.Duration{20 * time.Second, 30 * time.Second, 30 * time.Second, 30 * time.Second}) {
			t.Errorf("MaxDelay 封顶：%v %v", s.clk.slept, e)
		}

		broken := fake.New(llm.Caps{}, fake.Step{Deltas: []llm.Delta{{Text: "部分"}}, Err: &llm.Error{Class: gen.FailureClassTransient, Code: "network", Message: "断了"}}, ok())
		s = newSetup(map[string]llm.Client{"p": broken})
		if _, e := call(s.wrap(t, "p")); e == nil || len(broken.Requests()) != 1 || len(s.rec.all()) != 1 || s.rec.all()[0].Err == nil {
			t.Errorf("交出第一个片段之后的错误原样返回、不重试：%v %d", e, len(broken.Requests()))
		}
	})
}

// limiter 记下 Penalize 和并发。
type limiter struct {
	mu        sync.Mutex
	penalties []string
}

func (l *limiter) Acquire(context.Context, func(int64)) (func(), error) { return func() {}, nil }
func (l *limiter) Penalize(f float64, d time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.penalties = append(l.penalties, time.Duration(f*1000).String()+"/"+d.String())
}

func TestRateLimited(t *testing.T) {
	t.Run("E1009-T2 429 降速", func(t *testing.T) {
		lim := &limiter{}
		s := newSetup(map[string]llm.Client{"p": fake.New(llm.Caps{}, failAfter(gen.FailureClassRateLimited, 7*time.Second), ok())})
		s.opts.Pools = func(llm.Target) llm.Limiter { return lim }
		if _, e := call(s.wrap(t, "p")); e != nil || !slices.Equal(s.clk.slept, []time.Duration{7 * time.Second}) {
			t.Errorf("按 Retry-After 等 7 秒：%v %v", s.clk.slept, e)
		}
		if !slices.Equal(lim.penalties, []string{"800ns/5m0s"}) {
			t.Errorf("资源池 Penalize(0.8, 5m)：%v", lim.penalties)
		}
		f := fake.New(llm.Caps{}, failAfter(gen.FailureClassRateLimited, 10*time.Minute), ok())
		s = newSetup(map[string]llm.Client{"p": f})
		if _, e := call(s.wrap(t, "p")); e == nil || e.Class != gen.FailureClassRateLimited || len(s.clk.slept) != 0 || len(f.Requests()) != 1 {
			t.Errorf("Retry-After 超过上限时不等、直接失败：%v %v", e, s.clk.slept)
		}
	})
}

func TestBreaker(t *testing.T) {
	t.Run("E1009-T3 熔断", func(t *testing.T) {
		target := llm.Target{Provider: "p", Model: "m"}
		b := llm.NewBreakers(llm.DefaultBreaker).For(target)
		now := t0
		for i := 0; i < 9; i++ {
			b.Record(now, llm.Success)
		}
		for i := 0; i < 10; i++ {
			b.Record(now, llm.Failure)
		}
		if b.State(now) != llm.StateClosed {
			t.Fatal("不到 20 次请求不打开")
		}
		b.Record(now, llm.Failure)
		if b.State(now) != llm.StateOpen || b.Allow(now) {
			t.Fatal("20 次里 11 次失败应当打开")
		}
		now = now.Add(30 * time.Second)
		if !b.Allow(now) || b.Allow(now) {
			t.Fatal("30 秒后只放一个试探请求")
		}
		b.Record(now, llm.Neutral)
		if b.State(now) != llm.StateHalfOpen || !b.Allow(now) {
			t.Fatal("Neutral 不计：还是半开，再放一个试探")
		}
		b.Record(now, llm.Failure)
		if b.State(now) != llm.StateOpen {
			t.Fatal("试探失败再打开")
		}
		now = now.Add(30 * time.Second)
		if !b.Allow(now) {
			t.Fatal("又过 30 秒半开")
		}
		b.Record(now, llm.Success)
		if b.State(now) != llm.StateClosed || !b.Allow(now) {
			t.Fatal("试探成功关闭")
		}

		f := fake.New(llm.Caps{}, fail(gen.FailureClassTransient), fail(gen.FailureClassTransient), ok())
		s := newSetup(map[string]llm.Client{"p": f})
		s.opts.Retry = llm.RetryCfg{Max: 1}
		s.opts.Breakers = llm.NewBreakers(llm.BreakerCfg{Window: time.Minute, MinRequests: 2, ErrorRate: 0.5, Open: 30 * time.Second})
		c := s.wrap(t, "p")
		if _, e := call(c); e == nil {
			t.Fatal("第 1 次瞬时失败应返回错误")
		}
		if _, e := call(c); e == nil {
			t.Fatal("第 2 次瞬时失败应返回错误")
		}
		if _, e := call(c); e == nil || e.Code != llm.CodeCircuitOpen || len(f.Requests()) != 2 || len(s.rec.all()) != 2 {
			t.Errorf("熔断打开时不发请求：%v %d %d", e, len(f.Requests()), len(s.rec.all()))
		}
		s.clk.add(30 * time.Second)
		if _, e := call(c); e != nil {
			t.Errorf("30 秒后试探成功：%v", e)
		}
	})
}

// slowClient 在 Complete 时把注入的时间往前推 d（测 Latency）。
type slowClient struct {
	llm.Client
	clk *clock
	d   time.Duration
}

func (c slowClient) Complete(ctx context.Context, r llm.Req) (llm.Stream, error) {
	c.clk.add(c.d)
	return c.Client.Complete(ctx, r)
}

func TestRecord(t *testing.T) {
	t.Run("E1009-T4 记录", func(t *testing.T) {
		s := newSetup(nil)
		s.src.clients = map[string]llm.Client{"p": slowClient{Client: fake.New(llm.Caps{}, ok(), fail(gen.FailureClassUserInput), ok()), clk: s.clk, d: 3 * time.Second}}
		c := s.wrap(t, "p")
		if _, e := call(c); e != nil {
			t.Fatalf("成功一次：%v", e)
		}
		rs := s.rec.all()
		if len(rs) != 1 || rs[0].Resp == nil || rs[0].Err != nil || rs[0].Attempt != 1 || rs[0].Latency != 3*time.Second || rs[0].Target.Provider != "p" {
			t.Fatalf("成功一次记录：%+v", rs)
		}
		if _, e := call(c); e == nil {
			t.Fatal("失败一次：UserInput 应返回错误")
		}
		if rs = s.rec.all(); len(rs) != 2 || rs[1].Err == nil || rs[1].Resp != nil {
			t.Fatalf("失败一次记录：%+v", rs)
		}
		st, err := c.Complete(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		_ = st.Close()
		_ = st.Close()
		if rs = s.rec.all(); len(rs) != 3 || rs[2].Err == nil || rs[2].Err.Class != gen.FailureClassCancelled {
			t.Fatalf("提前 Close 恰好一次记录：%+v", rs)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := c.Complete(ctx, req); err == nil || len(s.rec.all()) != 3 {
			t.Errorf("ctx 取消时不发请求、不记录：%v %d", err, len(s.rec.all()))
		}
	})
}

// countingClient 记下同时在飞的调用数（从 Complete 到流结束）。
type countingClient struct {
	inflight, peak *atomic.Int64
}

func (countingClient) Caps() llm.Caps { return llm.Caps{} }

func (c countingClient) Complete(context.Context, llm.Req) (llm.Stream, error) {
	n := c.inflight.Add(1)
	for {
		p := c.peak.Load()
		if n <= p || c.peak.CompareAndSwap(p, n) {
			break
		}
	}
	time.Sleep(5 * time.Millisecond)
	return &doneStream{inflight: c.inflight}, nil
}

type doneStream struct {
	inflight *atomic.Int64
	done     bool
}

func (s *doneStream) Next() (llm.Delta, error) {
	if !s.done {
		s.done = true
		s.inflight.Add(-1)
	}
	return llm.Delta{Done: true, Resp: &llm.Resp{Text: "好", Finish: llm.FinishStop}}, nil
}

func (s *doneStream) Close() error { return nil }

func TestPoolLimit(t *testing.T) {
	t.Run("E1009-T5 资源池上限", func(t *testing.T) {
		var inflight, peak atomic.Int64
		s := newSetup(map[string]llm.Client{"p": countingClient{inflight: &inflight, peak: &peak}})
		set := sched.NewPools(sched.Config{Pools: map[string]sched.PoolConfig{"llm:p/m": {Concurrency: 2}}}, sched.RealClock).Snapshot()
		s.opts.Pools = func(t llm.Target) llm.Limiter { return set.Get(t.Pool()) }
		var waits atomic.Int64
		var pool atomic.Value
		s.opts.Hooks.OnWait = func(p string, _ int64) { waits.Add(1); pool.Store(p) }
		c := s.wrap(t, "p")
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, e := call(c); e != nil {
					t.Error(e)
				}
			}()
		}
		wg.Wait()
		if peak.Load() > 2 || peak.Load() == 0 {
			t.Errorf("同时在飞不超过 2：%d", peak.Load())
		}
		if waits.Load() == 0 || pool.Load() != "llm:p/m" {
			t.Errorf("排队时 OnWait 收到池名：%d %v", waits.Load(), pool.Load())
		}
	})
}

func TestKeysAndFailover(t *testing.T) {
	t.Run("E1009-T6 换密钥与故障转移", func(t *testing.T) {
		s := newSetup(map[string]llm.Client{
			"p":   fake.New(llm.Caps{}),
			"p|a": fake.New(llm.Caps{}, failAfter(gen.FailureClassRateLimited, 5*time.Second)),
			"p|b": fake.New(llm.Caps{}, ok()),
		})
		s.src.keys["p"] = []string{"a", "b"}
		s.opts.Keys = llm.NewKeyRing()
		if _, e := call(s.wrap(t, "p")); e != nil || len(s.clk.slept) != 0 {
			t.Fatalf("a 被限流后马上换 b、不等待：%v %v", e, s.clk.slept)
		}
		if rs := s.rec.all(); len(rs) != 2 || rs[0].Key != "a" || rs[0].Err == nil || rs[1].Key != "b" || rs[1].Err != nil || rs[1].Attempt != 2 {
			t.Errorf("两次真实调用、密钥名对：%+v", rs)
		}

		q := fake.New(llm.Caps{}, ok(), ok())
		s = newSetup(map[string]llm.Client{
			"p":   fake.New(llm.Caps{}),
			"p|a": fake.New(llm.Caps{}, fail(gen.FailureClassAuthDenied)),
			"p|b": fake.New(llm.Caps{}, fail(gen.FailureClassAuthDenied)),
			"q":   q,
		})
		s.src.keys["p"] = []string{"a", "b"}
		s.src.fb["p"] = []string{"q"}
		s.opts.Keys = llm.NewKeyRing()
		var reasons []string
		s.opts.Hooks.OnFallback = func(_ context.Context, from, to llm.Target, reason *llm.Error) {
			reasons = append(reasons, from.Provider+"→"+to.Provider+":"+reason.Code)
		}
		c := s.wrap(t, "p")
		if _, e := call(c); e != nil {
			t.Fatalf("两个密钥都鉴权失败后转到 q：%v", e)
		}
		if _, e := call(c); e != nil {
			t.Fatalf("密钥都在冷却时直接转到 q：%v", e)
		}
		if !slices.Equal(reasons, []string{"p→q:x", "p→q:" + llm.CodeKeysCooling}) || len(q.Requests()) != 2 {
			t.Errorf("OnFallback 每次一次、写明原因：%v %d", reasons, len(q.Requests()))
		}

		q2 := fake.New(llm.Caps{}, ok())
		s = newSetup(map[string]llm.Client{"x": fake.New(llm.Caps{}, fail(gen.FailureClassUserInput)), "q2": q2})
		s.src.fb["x"] = []string{"q2"}
		if _, e := call(s.wrap(t, "x")); e == nil || e.Class != gen.FailureClassUserInput || len(q2.Requests()) != 0 {
			t.Errorf("user_input 不转移：%v %d", e, len(q2.Requests()))
		}

		q3 := fake.New(llm.Caps{}, ok())
		s = newSetup(map[string]llm.Client{"y": fake.New(llm.Caps{}, fail(gen.FailureClassTransient), fail(gen.FailureClassTransient), fail(gen.FailureClassTransient)), "q3": q3})
		s.src.fb["y"] = []string{"missing", "q3"}
		if _, e := call(s.wrap(t, "y")); e != nil || len(q3.Requests()) != 1 {
			t.Errorf("备用提供方没有这个模型时跳过：%v %d", e, len(q3.Requests()))
		}
	})
}
