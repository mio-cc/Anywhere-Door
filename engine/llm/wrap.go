package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/mio-cc/anywhere-door/gen"
)

// Limiter 是资源池：并发 + 速率（E.15.02）；*sched.Pool 满足它。llm 不 import sched（E.01 §1）。
type Limiter interface {
	Acquire(ctx context.Context, onWait func(waiting int64)) (release func(), err error)
	Penalize(factor float64, d time.Duration)
}

// Source 给出可调用的 Client；*Registry 满足它。
type Source interface {
	Client(provider, model string) (Client, error)
	ClientWithKey(provider, model, key string) (Client, error)
	Keys(provider string) []string
	Fallbacks(provider string) []string
}

// Target 是一个提供方上的一个模型。
type Target struct{ Provider, Model string }

// Pool 是它的资源池名（E.15.02 的键）。
func (t Target) Pool() string { return "llm:" + t.Provider + "/" + t.Model }

// CallRecord 是一次真实调用（请求已发出）的记录，调用方据此写 llm.call（E.14.02）。
type CallRecord struct {
	Target  Target
	Key     string // 密钥名（不是原文）；没有多密钥时为空
	Attempt int    // 这个提供方上的第几次真实调用（从 1 起）
	Req     Req
	Resp    *Resp // 失败时为 nil
	Err     *Error
	Latency time.Duration
}

// Hooks 把调用过程交给调用方记录；都可以为 nil。
type Hooks struct {
	OnCall     func(ctx context.Context, r CallRecord)                   // 每次真实调用恰好一次
	OnWait     func(pool string, waiting int64)                          // 在资源池排队（node.throttled）
	OnFallback func(ctx context.Context, from, to Target, reason *Error) // 换到备用提供方（llm.fallback）
}

// WrapOpts 是调用外层的依赖。
type WrapOpts struct {
	Source   Source
	Pools    func(t Target) Limiter // nil 或返回 nil：不限
	Breakers *Breakers              // nil：不熔断
	Keys     *KeyRing               // nil：不换密钥
	Retry    RetryCfg               // Max ≤ 0 时用 DefaultRetry
	Now      func() time.Time
	Sleep    func(ctx context.Context, d time.Duration) error
	Rand     func() float64 // [0, 1)；nil：不抖动
	Hooks    Hooks
}

// CodeCircuitOpen：熔断打开，没有发请求。
const CodeCircuitOpen = "circuit_open"

// 429 降速（E.10.09 §5）。
const (
	penaltyFactor = 0.8
	penaltyFor    = 5 * time.Minute
)

type wrapped struct {
	t    Target
	o    WrapOpts
	caps Caps
}

// Wrap 给一个模型包上资源池、熔断、重试、换密钥和故障转移（E.10.09）；由 RunCtx.LLM 调用。Caps 是主提供方的。
func Wrap(t Target, o WrapOpts) (Client, error) {
	if o.Source == nil || o.Now == nil || o.Sleep == nil {
		return nil, errors.New("llm.Wrap：Source、Now、Sleep 必填")
	}
	c, err := o.Source.Client(t.Provider, t.Model)
	if err != nil {
		return nil, err
	}
	if o.Retry.Max <= 0 {
		o.Retry = DefaultRetry
	}
	return &wrapped{t: t, o: o, caps: c.Caps()}, nil
}

func (w *wrapped) Caps() Caps { return w.caps }

func failover(e *Error) bool {
	switch e.Class {
	case gen.FailureClassTransient, gen.FailureClassRateLimited, gen.FailureClassAuthDenied:
		return true
	}
	return false
}

// Complete 先试主提供方，失败且可以转移时按顺序试备用提供方（只转一层）。
func (w *wrapped) Complete(ctx context.Context, req Req) (Stream, error) {
	s, e := w.try(ctx, w.t, req)
	if e == nil {
		return s, nil
	}
	from := w.t
	for _, name := range w.o.Source.Fallbacks(w.t.Provider) {
		if !failover(e) || ctx.Err() != nil {
			break
		}
		to := Target{Provider: name, Model: w.t.Model}
		if _, err := w.o.Source.Client(to.Provider, to.Model); err != nil {
			continue // 备用提供方没有这个模型
		}
		if w.o.Hooks.OnFallback != nil {
			w.o.Hooks.OnFallback(ctx, from, to, e)
		}
		var next *Error
		if s, next = w.try(ctx, to, req); next == nil {
			return s, nil
		}
		e, from = next, to
	}
	return nil, e
}

func (w *wrapped) record(ctx context.Context, r CallRecord) {
	if w.o.Hooks.OnCall != nil {
		w.o.Hooks.OnCall(ctx, r)
	}
}

func (w *wrapped) onWait(t Target) func(int64) {
	if w.o.Hooks.OnWait == nil {
		return nil
	}
	pool := t.Pool()
	return func(n int64) { w.o.Hooks.OnWait(pool, n) }
}

// try 在一个提供方上调用：选密钥 → 资源池 → 熔断 → 发请求；第一个片段之前失败时换密钥或退避重试。
func (w *wrapped) try(ctx context.Context, t Target, req Req) (Stream, *Error) {
	o := w.o
	br := o.Breakers.For(t)
	var lim Limiter
	if o.Pools != nil {
		lim = o.Pools(t)
	}
	keys := o.Source.Keys(t.Provider)
	attempt, retries, switches := 0, 0, 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, cancelled(err)
		}
		key := ""
		if o.Keys != nil && len(keys) > 0 {
			k, wait, ok := o.Keys.Pick(t.Provider, keys, o.Now())
			if !ok {
				return nil, &Error{Class: gen.FailureClassRateLimited, Provider: t.Provider, Code: CodeKeysCooling, RetryAfter: wait,
					Message: fmt.Sprintf("提供方 %s 的密钥都在冷却，%s 后恢复", t.Provider, wait)}
			}
			key = k
		}
		var c Client
		var err error
		if key != "" {
			c, err = o.Source.ClientWithKey(t.Provider, t.Model, key)
		} else {
			c, err = o.Source.Client(t.Provider, t.Model)
		}
		if err != nil {
			return nil, asError(err)
		}
		release := func() {}
		if lim != nil {
			rel, err := lim.Acquire(ctx, w.onWait(t))
			if err != nil {
				return nil, cancelled(err)
			}
			release = rel
		}
		if !br.Allow(o.Now()) {
			release()
			return nil, &Error{Class: gen.FailureClassTransient, Provider: t.Provider, Code: CodeCircuitOpen,
				Message: fmt.Sprintf("提供方 %s 的模型 %s 熔断中，稍后再试", t.Provider, t.Model)}
		}
		attempt++
		start := o.Now()
		s, first, err := open(ctx, c, req)
		if err == nil {
			return &wstream{w: w, ctx: ctx, inner: s, first: &first, release: release, br: br, start: start,
				rec: CallRecord{Target: t, Key: key, Attempt: attempt, Req: req}}, nil
		}
		release()
		e := asError(err)
		br.Record(o.Now(), outcome(e))
		w.record(ctx, CallRecord{Target: t, Key: key, Attempt: attempt, Req: req, Err: e, Latency: o.Now().Sub(start)})
		if err := ctx.Err(); err != nil {
			return nil, cancelled(err)
		}
		if d := CoolFor(e); key != "" && d > 0 {
			o.Keys.Cool(t.Provider, key, o.Now().Add(d))
			if switches < len(keys)-1 {
				switches++
				continue // 马上换下一个密钥，不等待、不算重试次数
			}
		}
		if !retryable(e) || retries+1 >= o.Retry.Max {
			return nil, e
		}
		d := o.Retry.delay(retries, e, o.Rand)
		if d < 0 {
			return nil, e
		}
		if e.Class == gen.FailureClassRateLimited && lim != nil {
			lim.Penalize(penaltyFactor, penaltyFor)
		}
		retries++
		if err := o.Sleep(ctx, d); err != nil {
			return nil, cancelled(err)
		}
	}
}

// open 发请求并读第一个片段：第一个片段之前的错误都算这次调用失败（可以重试）。
func open(ctx context.Context, c Client, req Req) (Stream, Delta, error) {
	s, err := c.Complete(ctx, req)
	if err != nil {
		return nil, Delta{}, err
	}
	d, err := s.Next()
	if err != nil {
		_ = s.Close()
		return nil, Delta{}, err
	}
	return s, d, nil
}

func cancelled(err error) *Error {
	return newError(gen.FailureClassCancelled, "cancelled", "调用已取消："+err.Error())
}

func asError(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return cancelled(err)
	case errors.Is(err, io.EOF):
		return newError(gen.FailureClassTransient, CodeStreamTruncated, "流在结束前断开（没有收到结束标志）")
	}
	return newError(gen.FailureClassInternal, "internal", err.Error())
}

// wstream 先交出已经读到的第一个片段；流结束、出错或被提前关闭时释放资源池、记熔断、回调 OnCall（恰好一次）。
// 交出第一个片段之后的错误原样交给调用方，由它整次重来（E.10.09 §5）。
type wstream struct {
	w       *wrapped
	ctx     context.Context
	inner   Stream
	first   *Delta
	release func()
	br      *Breaker
	rec     CallRecord
	start   time.Time
	once    sync.Once
	done    bool
}

func (s *wstream) Next() (Delta, error) {
	if s.done {
		return Delta{}, io.EOF
	}
	var d Delta
	if s.first != nil {
		d, s.first = *s.first, nil
	} else {
		var err error
		if d, err = s.inner.Next(); err != nil {
			e := asError(err)
			s.finish(nil, e)
			return Delta{}, e
		}
	}
	if d.Done {
		s.finish(d.Resp, nil)
	}
	return d, nil
}

func (s *wstream) Close() error {
	s.finish(nil, newError(gen.FailureClassCancelled, "closed", "调用方提前关闭了流"))
	return nil
}

func (s *wstream) finish(resp *Resp, e *Error) {
	s.once.Do(func() {
		s.done = true
		_ = s.inner.Close()
		s.release()
		now := s.w.o.Now()
		s.br.Record(now, outcome(e))
		r := s.rec
		r.Resp, r.Err, r.Latency = resp, e, now.Sub(s.start)
		s.w.record(s.ctx, r)
	})
}
