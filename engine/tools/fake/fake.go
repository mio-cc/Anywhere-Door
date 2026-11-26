// Package fake 是工具测试用的假实现（设计文档 E.11.01）：脚本化的工具、内存里的工具调用表、记下请求的授权判定器。
// 给 tools、内置工具和执行器的单元测试用，不联网、不落盘。
package fake

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/mio-cc/anywhere-door/engine/authz"
	"github.com/mio-cc/anywhere-door/engine/blob"
	"github.com/mio-cc/anywhere-door/engine/tools"
)

// Step 是一次调用的脚本：Block 为真时一直等到 ctx 结束，返回 ctx 的错误。
type Step struct {
	Raw   tools.Raw
	Err   error
	Block bool
}

// Tool 按调用次序取 Step（取完后重复最后一个），记下每次调用。
type Tool struct {
	D     tools.Def
	mu    sync.Mutex
	steps []Step
	calls []tools.Call
}

// New 返回脚本化的工具。
func New(d tools.Def, steps ...Step) *Tool { return &Tool{D: d, steps: steps} }

// Def 返回定义。
func (t *Tool) Def() tools.Def { return t.D }

// Invoke 取下一个 Step。
func (t *Tool) Invoke(ctx context.Context, c tools.Call) (tools.Raw, error) {
	t.mu.Lock()
	n := len(t.calls)
	t.calls = append(t.calls, c)
	var s Step
	if len(t.steps) > 0 {
		s = t.steps[min(n, len(t.steps)-1)]
	}
	t.mu.Unlock()
	if s.Block {
		<-ctx.Done()
		return tools.Raw{}, ctx.Err()
	}
	return s.Raw, s.Err
}

// Calls 是收到的调用（副本）。
func (t *Tool) Calls() []tools.Call {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]tools.Call(nil), t.calls...)
}

// ErrConflict：Finish 的状态不对。
var ErrConflict = errors.New("fake: 工具调用状态冲突")

// Ledger 是内存里的工具调用表，语义同 store 的工具调用表。
type Ledger struct {
	mu sync.Mutex
	m  map[string]tools.Record
}

// NewLedger 返回空表。
func NewLedger() *Ledger { return &Ledger{m: map[string]tools.Record{}} }

// Begin 已有同一个键时不写入、返回已有的那一行。
func (l *Ledger) Begin(_ context.Context, r tools.Record) (*tools.Record, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if old, ok := l.m[r.IdemKey]; ok {
		return &old, nil
	}
	if r.Status == "" {
		r.Status = tools.StatusStarted
	}
	l.m[r.IdemKey] = r
	return nil, nil
}

// Finish 只能从 started 改成 succeeded 或 failed。
func (l *Ledger) Finish(_ context.Context, key, status string, ref blob.Ref, _ time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	r, ok := l.m[key]
	if !ok {
		return fmt.Errorf("%w：没有 %s", ErrConflict, key)
	}
	if r.Status != tools.StatusStarted || (status != tools.StatusSucceeded && status != tools.StatusFailed) {
		return fmt.Errorf("%w：不能从 %s 改成 %s", ErrConflict, r.Status, status)
	}
	r.Status, r.ResultRef = status, ref
	l.m[key] = r
	return nil
}

// Put 直接写入一行（测试里预置状态用）。
func (l *Ledger) Put(r tools.Record) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.m[r.IdemKey] = r
}

// Get 读一行。
func (l *Ledger) Get(key string) (tools.Record, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	r, ok := l.m[key]
	return r, ok
}

// Checker 按次序给出判定结果（取完后重复最后一个；没有时一律 allow），记下每次请求。
type Checker struct {
	mu      sync.Mutex
	Effects []authz.Effect
	reqs    []authz.Request
}

// Check 记下请求、给出下一个结果。
func (c *Checker) Check(_ context.Context, req authz.Request) authz.Result {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := len(c.reqs)
	c.reqs = append(c.reqs, req)
	e := authz.EffectAllow
	if len(c.Effects) > 0 {
		e = c.Effects[min(n, len(c.Effects)-1)]
	}
	return authz.Result{Effect: e, Reasons: []string{"fake"}}
}

// Requests 是收到的判定请求（副本）。
func (c *Checker) Requests() []authz.Request {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]authz.Request(nil), c.reqs...)
}
