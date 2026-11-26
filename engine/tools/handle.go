package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"time"
	"unicode/utf8"

	"github.com/mio-cc/anywhere-door/engine/authz"
	"github.com/mio-cc/anywhere-door/engine/blob"
	"github.com/mio-cc/anywhere-door/gen"
)

// Limiter 是资源池（并发 + 速率）；*sched.Pool 满足它，由组装处接上。
type Limiter interface {
	Acquire(ctx context.Context, onWait func(waiting int64)) (release func(), err error)
}

// Intent 是 tool.intent 事件的内容（INV-14）。
type Intent struct {
	IdemKey, Tool string
	Risk          Risk
	ArgsRef       blob.Ref
}

// CallRecord 是 tool.call 事件的内容；每次 Call 恰好一条。
type CallRecord struct {
	IdemKey, Tool      string
	Risk               Risk
	Status             string // succeeded | failed | deduplicated
	ArgsRef, ResultRef blob.Ref
	Taint              bool
	Queued, Latency    time.Duration
	Err                *Error
}

// Hooks 把事件交给调用方写（执行器，第 ⑥ 片）。
type Hooks struct {
	OnIntent func(ctx context.Context, in Intent)
	OnCall   func(ctx context.Context, r CallRecord)
	OnWait   func(pool string, waiting int64)
}

// Env 是 Handle 需要的环境。
type Env struct {
	RunID, ExecKey, Scope string // Scope：直接调用为空，Agent 内调用为 turn/index（只用于幂等键）
	Actor                 gen.Actor
	Target                authz.Target // 授权判定的对象（所在图和作用域）
	Authz                 authz.Checker
	Ledger                Ledger
	Blobs                 blob.Store
	Pool                  func(name string) Limiter // nil 或返回 nil：不限
	Counter               Counter                   // nil：一个字算一个 token
	Trim                  TrimOpts
	Hooks                 Hooks
	Now                   func() time.Time // nil = time.Now
}

// Handle 是执行器拿到的工具句柄（RunCtx.Tool，第 ⑥ 片）。
type Handle interface {
	Def() Def
	Call(ctx context.Context, args json.RawMessage, taint bool) (Result, error)
}

// NewHandle 包装一个工具实现。
func NewHandle(i Impl, env Env) Handle { return &handle{impl: i, def: i.Def(), env: env} }

const defaultTimeout = 60 * time.Second

type handle struct {
	impl Impl
	def  Def
	env  Env
}

func (h *handle) Def() Def { return h.def }

func (h *handle) now() time.Time {
	if h.env.Now != nil {
		return h.env.Now()
	}
	return time.Now()
}

type runeCount struct{}

func (runeCount) CountText(s string) int { return utf8.RuneCountInString(s) }

func (h *handle) counter() Counter {
	if h.env.Counter != nil {
		return h.env.Counter
	}
	return runeCount{}
}

func (h *handle) limiter() (string, Limiter) {
	name := h.def.Pool
	if name == "" {
		name = "tool:" + h.def.ID
	}
	if h.env.Pool == nil {
		return name, nil
	}
	return name, h.env.Pool(name)
}

// Call 按 E.11.01 §5 调用一次工具。
func (h *handle) Call(ctx context.Context, args json.RawMessage, taint bool) (Result, error) {
	d := h.def
	key, err := IdemKey(h.env.RunID, h.env.ExecKey, h.env.Scope, args)
	if err != nil {
		return h.done(ctx, &CallRecord{Tool: d.ID, Risk: d.Risk}, Result{}, Errorf(gen.FailureClassUserInput, "bad_args", "工具「%s」的参数不是合法的 JSON", d.ID))
	}
	rec := &CallRecord{IdemKey: key, Tool: d.ID, Risk: d.Risk}
	argsRef, err := h.env.Blobs.Put(ctx, bytes.NewReader(args), "application/json")
	if err != nil {
		return h.done(ctx, rec, Result{}, h.blobErr(err, key))
	}
	rec.ArgsRef = argsRef
	existing, err := h.env.Ledger.Begin(ctx, Record{
		IdemKey: key, RunID: h.env.RunID, ExecKey: h.env.ExecKey, Tool: d.ID, Risk: d.Risk,
		Status: StatusStarted, ArgsRef: argsRef, StartedAt: h.now(),
	})
	if err != nil {
		return h.done(ctx, rec, Result{}, &Error{Class: gen.FailureClassTransient, Code: "ledger", Msg: h.who(key) + "的调用记录写入失败", Err: err})
	}
	if existing != nil {
		switch existing.Status {
		case StatusSucceeded:
			r, e := h.load(ctx, existing.ResultRef, key)
			if e != nil {
				return h.done(ctx, rec, Result{}, e)
			}
			rec.ResultRef = existing.ResultRef
			rec.Status = StatusDeduplicated
			rec.Taint = r.Taint
			return h.done(ctx, rec, r, nil)
		case StatusStarted:
			if !d.Idempotent {
				return h.done(ctx, rec, Result{}, Errorf(gen.FailureClassUserInput, "tool_maybe_executed",
					"工具「%s」上次调用可能已经执行过，M1 不自动重发；确认后重新运行", d.ID))
			}
		default:
			return h.done(ctx, rec, Result{}, Errorf(gen.FailureClassTransient, "tool_failed_before", "工具「%s」同一次调用上次已失败", d.ID))
		}
	}
	r, e := h.invoke(ctx, rec, args, key, taint)
	// 走到这里工具可能已经执行过：账本不随调用方取消，否则会滞留 started 或记成 failed（P.09 K274）
	post := context.WithoutCancel(ctx)
	if e != nil {
		if ferr := h.env.Ledger.Finish(post, key, StatusFailed, "", h.now()); ferr != nil && e.Err == nil {
			e.Err = ferr
		}
		return h.done(ctx, rec, Result{}, e)
	}
	if err := h.env.Ledger.Finish(post, key, StatusSucceeded, rec.ResultRef, h.now()); err != nil {
		return h.done(ctx, rec, Result{}, &Error{Class: gen.FailureClassTransient, Code: "ledger", Msg: h.who(key) + "的调用记录写入失败", Err: err})
	}
	rec.Status = StatusSucceeded
	return h.done(ctx, rec, r, nil)
}

// invoke 做授权、意图、资源池、超时、调用、裁剪，结果写进工作区（§5 第 3–7 步）。
func (h *handle) invoke(ctx context.Context, rec *CallRecord, args json.RawMessage, key string, taint bool) (Result, *Error) {
	d := h.def
	if e := h.authorize(ctx, taint); e != nil {
		return Result{}, e
	}
	if d.Risk >= RiskL1 && h.env.Hooks.OnIntent != nil {
		h.env.Hooks.OnIntent(ctx, Intent{IdemKey: key, Tool: d.ID, Risk: d.Risk, ArgsRef: rec.ArgsRef})
	}
	name, lim := h.limiter()
	queued := h.now()
	if lim != nil {
		release, err := lim.Acquire(ctx, func(waiting int64) {
			if h.env.Hooks.OnWait != nil {
				h.env.Hooks.OnWait(name, waiting)
			}
		})
		if err != nil {
			return Result{}, h.classify(ctx, ctx, err, key)
		}
		defer release()
	}
	rec.Queued = h.now().Sub(queued)
	if d.Risk >= RiskL2 {
		if e := h.authorize(ctx, taint); e != nil { // S.04-R6：发出前再判定一次
			return Result{}, e
		}
	}
	timeout := d.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := h.now()
	raw, err := h.impl.Invoke(cctx, Call{Args: args, IdemKey: key, Taint: taint})
	rec.Latency = h.now().Sub(start)
	if err != nil {
		return Result{}, h.classify(ctx, cctx, err, key)
	}
	// 工具已经成功：裁剪、落原文、写结果不随调用方取消（P.09 K274）
	post := context.WithoutCancel(ctx)
	r, err := Trim(post, raw, h.env.Trim, h.env.Blobs, h.counter())
	if err != nil {
		var te *Error
		if errors.As(err, &te) {
			return Result{}, te
		}
		return Result{}, &Error{Class: gen.FailureClassTransient, Code: "trim", Msg: h.who(key) + "的结果裁剪失败", Err: err}
	}
	r.Taint = taint || d.TaintsResult
	if d.TaintsResult {
		r.TaintSrc = uniqSorted(raw.Sources)
	}
	b, err := json.Marshal(r)
	if err != nil {
		return Result{}, &Error{Class: gen.FailureClassInternal, Code: "encode", Msg: h.who(key) + "的结果编码失败", Err: err}
	}
	ref, err := h.env.Blobs.Put(post, bytes.NewReader(b), "application/json")
	if err != nil {
		return Result{}, h.blobErr(err, key)
	}
	rec.ResultRef = ref
	rec.Taint = r.Taint
	return r, nil
}

// done 发 OnCall（恰好一次）并返回。OnCall 拿不随取消的 ctx：每次 Call 的记录都要写下（K274）。
func (h *handle) done(ctx context.Context, rec *CallRecord, r Result, e *Error) (Result, error) {
	if e != nil {
		rec.Status = StatusFailed
		rec.Err = e
	}
	if h.env.Hooks.OnCall != nil {
		h.env.Hooks.OnCall(context.WithoutCancel(ctx), *rec)
	}
	if e != nil {
		return Result{}, e
	}
	return r, nil
}

func (h *handle) authorize(ctx context.Context, taint bool) *Error {
	if h.env.Authz == nil {
		return Errorf(gen.FailureClassInternal, "no_authz", "工具「%s」没有授权判定器（INV-09）", h.def.ID)
	}
	res := h.env.Authz.Check(ctx, authz.Request{Actor: h.env.Actor, Action: h.def.Risk.Action(), Target: h.env.Target, Taint: taint})
	if res.Effect == authz.EffectAllow {
		return nil
	}
	return Errorf(gen.FailureClassAuthDenied, "auth_denied", "工具「%s」没有得到授权（%s）", h.def.ID, string(res.Effect))
}

// who 是错误信息里的工具称呼：工具 ID 加参数哈希的前 8 位，不带参数原文（§7）。
func (h *handle) who(key string) string {
	if len(key) > 8 {
		key = key[:8]
	}
	return "工具「" + h.def.ID + "」（参数 " + key + "）"
}

func (h *handle) blobErr(err error, key string) *Error {
	if errors.Is(err, context.Canceled) {
		return &Error{Class: gen.FailureClassCancelled, Code: "cancelled", Msg: "调用已取消", Err: err}
	}
	return &Error{Class: gen.FailureClassTransient, Code: "blob_put", Msg: h.who(key) + "的数据写入工作区失败", Err: err}
}

func (h *handle) classify(parent, cctx context.Context, err error, key string) *Error {
	var te *Error
	if errors.As(err, &te) {
		return te
	}
	if parent.Err() != nil {
		return &Error{Class: gen.FailureClassCancelled, Code: "cancelled", Msg: "调用已取消", Err: err}
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(cctx.Err(), context.DeadlineExceeded) {
		return &Error{Class: gen.FailureClassTimeout, Code: "timeout", Msg: h.who(key) + "超时", Err: err}
	}
	return &Error{Class: gen.FailureClassTransient, Code: "tool_error", Msg: h.who(key) + "调用失败", Err: err}
}

func (h *handle) load(ctx context.Context, ref blob.Ref, key string) (Result, *Error) {
	rc, _, err := h.env.Blobs.Get(ctx, ref)
	if err != nil {
		return Result{}, &Error{Class: gen.FailureClassTransient, Code: "blob_get", Msg: h.who(key) + "上次的结果读不出来", Err: err}
	}
	defer func() { _ = rc.Close() }()
	b, err := io.ReadAll(rc)
	if err != nil {
		return Result{}, &Error{Class: gen.FailureClassTransient, Code: "blob_get", Msg: h.who(key) + "上次的结果读不出来", Err: err}
	}
	var r Result
	if err := json.Unmarshal(b, &r); err != nil {
		return Result{}, &Error{Class: gen.FailureClassInternal, Code: "decode", Msg: h.who(key) + "上次的结果格式不对", Err: err}
	}
	return r, nil
}

func uniqSorted(xs []string) []string {
	if len(xs) == 0 {
		return nil
	}
	m := map[string]bool{}
	var out []string
	for _, x := range xs {
		if x != "" && !m[x] {
			m[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}
