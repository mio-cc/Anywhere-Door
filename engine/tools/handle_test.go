package tools_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mio-cc/anywhere-door/engine/authz"
	"github.com/mio-cc/anywhere-door/engine/blob"
	"github.com/mio-cc/anywhere-door/engine/tools"
	"github.com/mio-cc/anywhere-door/engine/tools/fake"
	"github.com/mio-cc/anywhere-door/gen"
)

type recorder struct {
	intents []tools.Intent
	calls   []tools.CallRecord
}

func (r *recorder) hooks() tools.Hooks {
	return tools.Hooks{
		OnIntent: func(_ context.Context, in tools.Intent) { r.intents = append(r.intents, in) },
		OnCall:   func(_ context.Context, c tools.CallRecord) { r.calls = append(r.calls, c) },
	}
}

func setup(t *testing.T, ck authz.Checker) (tools.Env, *fake.Ledger, *recorder) {
	t.Helper()
	led := fake.NewLedger()
	rec := &recorder{}
	if ck == nil {
		ck = authz.Fixed{}
	}
	return tools.Env{
		RunID: "run1", ExecKey: "n1", Actor: gen.Actor{Kind: gen.ActorKindRuntime, ID: "node:n1"},
		Authz: ck, Ledger: led, Blobs: newBlobs(t), Counter: runeCounter{}, Hooks: rec.hooks(),
	}, led, rec
}

func def(id string, risk tools.Risk) tools.Def {
	return tools.Def{ID: id, Name: strings.TrimPrefix(id, "builtin:"), Risk: risk}
}

func okStep(text string) fake.Step { return fake.Step{Raw: tools.Raw{Text: text}} }

// cancelLedger 和真的存储一样：ctx 取消以后写不进去（P.09 K274）。
type cancelLedger struct{ *fake.Ledger }

func (l cancelLedger) Begin(ctx context.Context, r tools.Record) (*tools.Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return l.Ledger.Begin(ctx, r)
}

func (l cancelLedger) Finish(ctx context.Context, key, status string, ref blob.Ref, at time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return l.Ledger.Finish(ctx, key, status, ref, at)
}

// cancelAfter 调用成功后、返回之前取消调用方的 ctx：工具刚做完运行就被取消。
type cancelAfter struct {
	*fake.Tool
	cancel context.CancelFunc
}

func (c cancelAfter) Invoke(ctx context.Context, call tools.Call) (tools.Raw, error) {
	raw, err := c.Tool.Invoke(ctx, call)
	c.cancel()
	return raw, err
}

func TestHandle(t *testing.T) {
	ctx := context.Background()

	t.Run("E1101-T1 幂等去重", func(t *testing.T) {
		env, _, rec := setup(t, nil)
		f := fake.New(def("builtin:echo", tools.RiskL0), okStep("结果一"), okStep("结果二"))
		h := tools.NewHandle(f, env)
		r1, err := h.Call(ctx, json.RawMessage(`{"a":1,"b":2}`), false)
		if err != nil {
			t.Fatal(err)
		}
		r2, err := h.Call(ctx, json.RawMessage(`{ "b": 2, "a": 1 }`), false)
		if err != nil {
			t.Fatal(err)
		}
		if len(f.Calls()) != 1 {
			t.Fatalf("同一幂等键应只调用一次，实际 %d 次", len(f.Calls()))
		}
		if r1.Ref != r2.Ref || r1.Summary != "结果一" || r2.Summary != "结果一" {
			t.Errorf("第二次应返回第一次的结果：%q / %q", r1.Summary, r2.Summary)
		}
		if len(rec.calls) != 2 || rec.calls[0].Status != tools.StatusSucceeded || rec.calls[1].Status != tools.StatusDeduplicated {
			t.Errorf("OnCall 状态不对：%+v", rec.calls)
		}
		k1, _ := tools.IdemKey("run1", "n1", "", json.RawMessage(`{"a":1,"b":2}`))
		k2, _ := tools.IdemKey("run1", "n1", "turn/1", json.RawMessage(`{"a":1,"b":2}`))
		if k1 == k2 || f.Calls()[0].IdemKey != k1 {
			t.Errorf("幂等键应包含 scope，且带给工具：%s %s %s", k1, k2, f.Calls()[0].IdemKey)
		}
	})

	t.Run("E1101-T2 崩溃恢复", func(t *testing.T) {
		args := json.RawMessage(`{"q":"x"}`)
		key, err := tools.IdemKey("run1", "n1", "", args)
		if err != nil {
			t.Fatal(err)
		}
		for _, idem := range []bool{true, false} {
			env, led, _ := setup(t, nil)
			led.Put(tools.Record{IdemKey: key, Status: tools.StatusStarted})
			d := def("builtin:send", tools.RiskL1)
			d.Idempotent = idem
			f := fake.New(d, okStep("发出去了"))
			_, err := tools.NewHandle(f, env).Call(ctx, args, false)
			if idem {
				if err != nil || len(f.Calls()) != 1 || f.Calls()[0].IdemKey != key {
					t.Errorf("幂等工具应带同一键重发：%v %+v", err, f.Calls())
				}
				if r, _ := led.Get(key); r.Status != tools.StatusSucceeded {
					t.Errorf("重发成功后应记为 succeeded：%s", r.Status)
				}
				continue
			}
			var te *tools.Error
			if !errors.As(err, &te) || te.Code != "tool_maybe_executed" || te.Class != gen.FailureClassUserInput {
				t.Errorf("非幂等工具应报 tool_maybe_executed：%v", err)
			}
			if len(f.Calls()) != 0 {
				t.Errorf("非幂等工具不应重发")
			}
		}
		env, led, _ := setup(t, nil)
		led.Put(tools.Record{IdemKey: key, Status: tools.StatusFailed})
		f := fake.New(def("builtin:send", tools.RiskL1), okStep("x"))
		_, err = tools.NewHandle(f, env).Call(ctx, args, false)
		var te *tools.Error
		if !errors.As(err, &te) || te.Code != "tool_failed_before" || len(f.Calls()) != 0 {
			t.Errorf("同一键上次失败应报 tool_failed_before：%v", err)
		}
	})

	t.Run("E1101-T3 风险下限", func(t *testing.T) {
		r := tools.NewRegistry()
		if err := r.Register(fake.New(def("builtin:ws_read", tools.RiskL0))); err != nil {
			t.Fatalf("ws_read L0 应能注册：%v", err)
		}
		if err := r.Register(fake.New(def("builtin:ws_read", tools.RiskL0))); err == nil {
			t.Error("重复注册应被拒")
		}
		tools.MinRisk["builtin:test_write"] = tools.RiskL1
		t.Cleanup(func() { delete(tools.MinRisk, "builtin:test_write") })
		if err := r.Register(fake.New(def("builtin:test_write", tools.RiskL0))); err == nil {
			t.Error("低于下限应被拒")
		}
		if err := r.Register(fake.New(def("builtin:test_write", tools.RiskL2))); err != nil {
			t.Errorf("可以调高：%v", err)
		}
		bad := []tools.Def{
			def("builtin:unlisted", tools.RiskL0),                     // 内置但没登记下限
			{ID: "web:x", Name: "x"},                                  // ID 前缀不对
			{ID: "mcp:svc/tool", Name: "Tool"},                        // 名字有大写
			{ID: "mcp:svc/tool2", Name: "tool2", Risk: tools.Risk(7)}, // 风险不对
		}
		for _, d := range bad {
			if err := r.Register(fake.New(d)); err == nil {
				t.Errorf("应被拒：%+v", d)
			}
		}
		if err := r.Register(fake.New(tools.Def{ID: "mcp:svc/tool", Name: "tool", Risk: tools.RiskL2})); err != nil {
			t.Errorf("MCP 工具应能注册：%v", err)
		}
		ids := []string{}
		for _, d := range r.List() {
			ids = append(ids, d.ID)
		}
		if strings.Join(ids, ",") != "builtin:test_write,builtin:ws_read,mcp:svc/tool" {
			t.Errorf("List 应按 ID 排序：%v", ids)
		}
		if _, ok := r.Get("builtin:ws_read"); !ok {
			t.Error("Get 取不到")
		}
		if _, err := tools.ParseRisk("L4"); err == nil {
			t.Error("L4 不合法")
		}
		if rk, _ := tools.ParseRisk("L2"); rk.String() != "L2" || rk.Action() != gen.ActToolL2 {
			t.Errorf("ParseRisk / Action 不对：%v", rk)
		}
	})

	t.Run("E1101-T5 发出前再判定", func(t *testing.T) {
		for _, c := range []struct {
			risk    tools.Risk
			checks  int
			intents int
		}{{tools.RiskL0, 1, 0}, {tools.RiskL1, 1, 1}, {tools.RiskL2, 2, 1}, {tools.RiskL3, 2, 1}} {
			ck := &fake.Checker{}
			env, _, rec := setup(t, ck)
			f := fake.New(def("builtin:t", c.risk), okStep("好"))
			if _, err := tools.NewHandle(f, env).Call(ctx, json.RawMessage(`{}`), true); err != nil {
				t.Fatal(err)
			}
			reqs := ck.Requests()
			if len(reqs) != c.checks || len(rec.intents) != c.intents {
				t.Errorf("%s：判定 %d 次、意图 %d 次，应为 %d、%d", c.risk, len(reqs), len(rec.intents), c.checks, c.intents)
			}
			if reqs[0].Action != c.risk.Action() || !reqs[0].Taint {
				t.Errorf("%s：判定请求不对：%+v", c.risk, reqs[0])
			}
		}
		ck := &fake.Checker{Effects: []authz.Effect{authz.EffectAllow, authz.EffectDeny}}
		env, _, _ := setup(t, ck)
		f := fake.New(def("builtin:t", tools.RiskL2), okStep("好"))
		_, err := tools.NewHandle(f, env).Call(ctx, json.RawMessage(`{}`), false)
		var te *tools.Error
		if !errors.As(err, &te) || te.Class != gen.FailureClassAuthDenied || len(f.Calls()) != 0 {
			t.Errorf("第二次判定 deny 时不调用、返回 auth_denied：%v，调用 %d 次", err, len(f.Calls()))
		}
		for _, e := range []authz.Effect{authz.EffectAsk, authz.EffectPropose} {
			env, _, _ := setup(t, &fake.Checker{Effects: []authz.Effect{e}})
			_, err := tools.NewHandle(fake.New(def("builtin:t", tools.RiskL0), okStep("好")), env).Call(ctx, json.RawMessage(`{}`), false)
			if !errors.As(err, &te) || te.Class != gen.FailureClassAuthDenied {
				t.Errorf("M1 的 %s 按 auth_denied：%v", e, err)
			}
		}
	})

	t.Run("E1101-T6 失败与记录", func(t *testing.T) {
		env, led, rec := setup(t, nil)
		d := def("builtin:slow", tools.RiskL0)
		d.Timeout = 20 * time.Millisecond
		_, err := tools.NewHandle(fake.New(d, fake.Step{Block: true}), env).Call(ctx, json.RawMessage(`{"secret":"不该出现的参数"}`), false)
		var te *tools.Error
		if !errors.As(err, &te) || te.Class != gen.FailureClassTimeout {
			t.Fatalf("超时应为 timeout：%v", err)
		}
		if strings.Contains(err.Error(), "不该出现的参数") {
			t.Errorf("错误信息不能带参数原文：%v", err)
		}
		key, _ := tools.IdemKey("run1", "n1", "", json.RawMessage(`{"secret":"不该出现的参数"}`))
		if r, _ := led.Get(key); r.Status != tools.StatusFailed {
			t.Errorf("失败应记为 failed：%s", r.Status)
		}
		if len(rec.calls) != 1 || rec.calls[0].Status != tools.StatusFailed || rec.calls[0].Err == nil {
			t.Errorf("OnCall 应恰好一次且带错误：%+v", rec.calls)
		}

		env, _, _ = setup(t, nil)
		f := fake.New(def("builtin:bad", tools.RiskL0), fake.Step{Err: tools.Errorf(gen.FailureClassUserInput, "not_found", "没找到")})
		_, err = tools.NewHandle(f, env).Call(ctx, json.RawMessage(`{}`), false)
		if !errors.As(err, &te) || te.Class != gen.FailureClassUserInput || te.Code != "not_found" {
			t.Errorf("工具给的类别应原样保留：%v", err)
		}
		f = fake.New(def("builtin:bad", tools.RiskL0), fake.Step{Err: errors.New("连接被重置")})
		_, err = tools.NewHandle(f, env).Call(ctx, json.RawMessage(`{"x":1}`), false)
		if !errors.As(err, &te) || te.Class != gen.FailureClassTransient {
			t.Errorf("没写类别的错误按 transient：%v", err)
		}

		env, _, _ = setup(t, nil)
		d = def("builtin:web", tools.RiskL0)
		d.TaintsResult = true
		f = fake.New(d, fake.Step{Raw: tools.Raw{Text: "网页", Sources: []string{"https://b.example", "https://a.example", "https://b.example"}}})
		r, err := tools.NewHandle(f, env).Call(ctx, json.RawMessage(`{}`), false)
		if err != nil {
			t.Fatal(err)
		}
		if !r.Taint || strings.Join(r.TaintSrc, " ") != "https://a.example https://b.example" {
			t.Errorf("外部内容应污染、来源去重排序：%v %v", r.Taint, r.TaintSrc)
		}
		f = fake.New(def("builtin:calc", tools.RiskL0), okStep("42"))
		r, _ = tools.NewHandle(f, env).Call(ctx, json.RawMessage(`{"z":0}`), false)
		r2, _ := tools.NewHandle(f, env).Call(ctx, json.RawMessage(`{"y":2}`), true)
		if r.Taint || !r2.Taint || len(r2.TaintSrc) != 0 {
			t.Errorf("污染跟输入走：%v %v %v", r.Taint, r2.Taint, r2.TaintSrc)
		}
	})

	t.Run("E1101-T7 调用成功后取消", func(t *testing.T) {
		env, led, rec := setup(t, nil)
		env.Ledger = cancelLedger{led}
		var hookErrs []error
		env.Hooks.OnCall = func(c context.Context, cr tools.CallRecord) {
			hookErrs = append(hookErrs, c.Err())
			rec.calls = append(rec.calls, cr)
		}
		cctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		f := fake.New(def("builtin:send", tools.RiskL1), okStep("已发出"))
		args := json.RawMessage(`{"to":"a"}`)
		r, err := tools.NewHandle(cancelAfter{f, cancel}, env).Call(cctx, args, false)
		if cctx.Err() == nil {
			t.Fatal("前提不成立：调用返回时调用方的 ctx 应该已经取消")
		}
		if err != nil {
			t.Fatalf("工具已经成功，调用方取消也应返回结果：%v", err)
		}
		key, _ := tools.IdemKey("run1", "n1", "", args)
		got, _ := led.Get(key)
		if got.Status != tools.StatusSucceeded || got.ResultRef == "" {
			t.Fatalf("账本应记 succeeded 和结果引用，不能滞留 started：%+v", got)
		}
		if len(rec.calls) != 1 || rec.calls[0].Status != tools.StatusSucceeded || rec.calls[0].ResultRef != got.ResultRef {
			t.Errorf("OnCall 应恰好一次、succeeded、结果引用和账本一致：%+v", rec.calls)
		}
		if len(hookErrs) != 1 || hookErrs[0] != nil {
			t.Errorf("OnCall 拿到的 ctx 不应随调用方取消：%v", hookErrs)
		}
		r2, err := tools.NewHandle(f, env).Call(ctx, args, false)
		if err != nil || r2.Ref != r.Ref || len(f.Calls()) != 1 {
			t.Fatalf("同一键再调应直接拿上次的结果、不再执行：%v %d", err, len(f.Calls()))
		}
		if len(rec.calls) != 2 || rec.calls[1].Status != tools.StatusDeduplicated {
			t.Errorf("第二次应为 deduplicated：%+v", rec.calls)
		}
	})
}
