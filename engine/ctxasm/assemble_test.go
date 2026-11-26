package ctxasm_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mio-cc/anywhere-door/engine/ctxasm"
	"github.com/mio-cc/anywhere-door/engine/llm"
	"github.com/mio-cc/anywhere-door/gen"
)

var update = flag.Bool("update", false, "重新生成 testdata/*.golden（生成后要人读过再提交，P.09 K271）")

// execInputs 读 testdata/inputs_exec.json：一张任务卡、一个目标。
func execInputs(t *testing.T) (task, goal map[string]any) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "inputs_exec.json"))
	if err != nil {
		t.Fatal(err)
	}
	var in struct{ Task, Goal map[string]any }
	if err := json.Unmarshal(b, &in); err != nil {
		t.Fatal(err)
	}
	return in.Task, in.Goal
}

func execValues(task, goal map[string]any) []ctxasm.Value {
	return []ctxasm.Value{{Type: "任务卡@1", Data: task}, {Type: "目标@1", Data: goal}}
}

// recipeCopy 是内置配方的一份副本（测试里改它，不影响注册表）。
func recipeCopy(t *testing.T, ref string) *gen.Recipe {
	t.Helper()
	r, ok := view(t).Recipe(ref)
	if !ok {
		t.Fatalf("没有配方 %s", ref)
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var c gen.Recipe
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	return &c
}

func compile(t *testing.T, r *gen.Recipe) *ctxasm.Plan {
	t.Helper()
	p, ds := ctxasm.Compile(r, view(t))
	if errs := levels(ds, gen.DiagLevelError); len(errs) > 0 {
		t.Fatalf("配方编译有错误：%v", errs)
	}
	return p
}

func asmEnv(t *testing.T) ctxasm.Env {
	return ctxasm.Env{View: view(t), Blobs: newBlobs(t), Counter: runeCounter{}, Caps: llm.Caps{Window: 32000, MaxOutput: 4000}}
}

func valueJSON(t *testing.T, p ctxasm.Packet) []byte {
	t.Helper()
	b, err := json.MarshalIndent(p.Value(), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(b, '\n')
}

func section(p ctxasm.Packet, name string) (ctxasm.PacketSection, bool) {
	for _, s := range p.Sections {
		if s.Name == name {
			return s, true
		}
	}
	return ctxasm.PacketSection{}, false
}

func TestAssemble(t *testing.T) {
	ctx := context.Background()
	exec := func(t *testing.T) *ctxasm.Plan { return compile(t, recipeCopy(t, "执行者配方@1")) }

	t.Run("E1302-T1 黄金文件", func(t *testing.T) {
		task, goal := execInputs(t)
		pkt, err := ctxasm.Assemble(ctx, exec(t), execValues(task, goal), asmEnv(t))
		if err != nil {
			t.Fatal(err)
		}
		got := valueJSON(t, pkt)
		golden := filepath.Join("testdata", "exec_packet.golden")
		if *update {
			if err := os.MkdirAll(filepath.Dir(golden), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(golden, got, 0o644); err != nil {
				t.Fatal(err)
			}
			t.Logf("已写 %s：\n%s", golden, got)
			return
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("读不了黄金文件（先用 go test -run E1302 -update 生成，读过再提交）：%v", err)
		}
		if !bytes.Equal(bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n")), got) {
			t.Errorf("装配结果和黄金文件不同：\n%s", got)
		}
		// 不靠黄金文件也要成立的几点
		if _, ok := section(pkt, "经验"); ok {
			t.Error("M1 没有经验库，经验段不应出现")
		}
		if s, ok := section(pkt, "约束"); !ok || !s.Locked || s.Source != "inherit" {
			t.Errorf("约束段应是透传约束的锁定段：%+v", s)
		}
		if s, ok := section(pkt, "必要输入"); !ok || !strings.Contains(s.Text, "[原文: ") || s.Ref == "" || !pkt.Pointers {
			t.Errorf("必要输入是 transform: pointer，应是指针形式：%+v", s)
		}
		body, _ := json.Marshal(pkt.Value())
		if bytes.Contains(body, []byte("决策者的推理")) || bytes.Contains(body, []byte("第一版任务卡")) {
			t.Error("exclude 的 path:*.reasoning、path:history 没有生效")
		}
		if pkt.Tokens > pkt.Budget {
			t.Errorf("包的 token 数 %d 超过预算 %d", pkt.Tokens, pkt.Budget)
		}
		ev := pkt.Event()
		if ev.BudgetTokens != int64(pkt.Budget) || !bytes.Contains(ev.Sections, []byte(`"locked":true`)) || bytes.Contains(ev.Sections, []byte("content")) {
			t.Errorf("事件数据不对：%s", ev.Sections)
		}
	})

	t.Run("E1302-T2 污染", func(t *testing.T) {
		task, goal := execInputs(t)
		in := execValues(task, goal)
		in[0].Taint, in[0].TaintSrc = true, []string{"web:example.com", "web:a.example"}
		pkt, err := ctxasm.Assemble(ctx, exec(t), in, asmEnv(t))
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"任务", "目的", "好结果", "检查项", "必要输入"} {
			if s, ok := section(pkt, name); !ok || !s.Taint {
				t.Errorf("段 %s 用到了任务卡，应污染：%+v", name, s)
			}
		}
		if s, ok := section(pkt, "约束"); !ok || s.Taint {
			t.Errorf("约束段不应污染：%+v", s)
		}
		if !pkt.Taint || strings.Join(pkt.TaintSrc, ",") != "web:a.example,web:example.com" {
			t.Errorf("包应污染、来源合并排序：%v %v", pkt.Taint, pkt.TaintSrc)
		}
	})

	t.Run("E1302-T3 必需段", func(t *testing.T) {
		task, goal := execInputs(t)
		delete(task, "instruction")
		_, err := ctxasm.Assemble(ctx, exec(t), execValues(task, goal), asmEnv(t))
		var ce *ctxasm.Error
		if !errors.Is(err, ctxasm.ErrMissingRequired) || !errors.As(err, &ce) || ce.Class != gen.FailureClassUserInput || !strings.Contains(ce.Msg, "「任务」") {
			t.Errorf("应报必需段「任务」缺失（user_input）：%v", err)
		}
	})

	t.Run("E1302-T4 确定性", func(t *testing.T) {
		task, goal := execInputs(t)
		a, err := ctxasm.Assemble(ctx, exec(t), execValues(task, goal), asmEnv(t))
		if err != nil {
			t.Fatal(err)
		}
		// 键顺序不同的同一份输入
		var t2, g2 map[string]any
		if err := json.Unmarshal([]byte(`{"good_result":"每家一行：模型、输入单价、输出单价、来源链接","purpose":"给方案比较提供成本数据","instruction":"查出三家厂商大模型 2026 年公开的 API 报价","id":"T1","output_type":"执行结果@1","history":["第一版任务卡"],"reasoning":"决策者的推理过程，不该给执行者看","constraints":["只用公开资料","不登录任何账号"],"inputs":[{"items":["vendor-a","vendor-b","vendor-c"],"name":"厂商清单"},{"note":"按发布当天的汇率换算成人民币","name":"币种说明"}],"checklist":["三家都有","单价统一为每百万 tokens","每个数据附来源链接"]}`), &t2); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(`{"criteria":[{"must":true,"method":"规则","desc":"三家都有报价","id":"C1"}],"constraints":{"region":"中国大陆","currency":"CNY"},"intent":"比较三家大模型的性价比","id":"G1"}`), &g2); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 3; i++ {
			b, err := ctxasm.Assemble(ctx, exec(t), execValues(t2, g2), asmEnv(t))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(valueJSON(t, a), valueJSON(t, b)) {
				t.Fatalf("同样的输入装配结果不同：\n%s\n%s", valueJSON(t, a), valueJSON(t, b))
			}
		}
	})

	t.Run("E1302-T5 变量", func(t *testing.T) {
		task, goal := execInputs(t)
		task["checklist"] = []any{strings.Repeat("很长的检查项", 80)}
		r := &gen.Recipe{Name: "测试变量配方", Version: 1, OnOverflow: "truncate", Budget: gen.RecipeBudget{Ratio: 0.003},
			Inputs: &map[string]gen.TypeRef{"task": "任务卡@1"},
			Sections: []gen.Section{
				{Name: "任务", From: "in.task.instruction", Required: ptr(true), Var: sp("任务")},
				{Name: "参考", From: "in.task.checklist", Priority: ptr(int64(5)), Var: sp("参考")},
			}}
		env := asmEnv(t)
		env.Caps = llm.Caps{Window: 100000, MaxOutput: 4000}
		pkt, err := ctxasm.Assemble(ctx, compile(t, r), execValues(task, goal)[:1], env)
		if err != nil {
			t.Fatal(err)
		}
		if pkt.Vars["任务"] != task["instruction"] {
			t.Errorf("Vars 里应有任务段渲染后的文字：%v", pkt.Vars)
		}
		if _, ok := pkt.Vars["参考"]; ok || len(pkt.Dropped) == 0 {
			t.Errorf("被收缩的段不应出现在 Vars 里：%v %+v", pkt.Vars, pkt.Dropped)
		}
	})

	t.Run("E1302-T6 透传约束", func(t *testing.T) {
		task, goal := execInputs(t)
		delete(task, "constraints") // 约束只来自目标
		goal["constraints"] = map[string]any{"currency": "CNY", "region": "中国大陆", "note": strings.Repeat("只用公开资料，", 25)}
		r := recipeCopy(t, "执行者配方@1")
		ex := []string{"path:constraints", "type:目标"}
		r.Exclude = &ex
		env := asmEnv(t)
		env.Caps = llm.Caps{Window: 100000, MaxOutput: 4000}
		env.Budget = ptr(0.006) // 600 tokens
		pkt, err := ctxasm.Assemble(ctx, compile(t, r), execValues(task, goal), env)
		if err != nil {
			t.Fatal(err)
		}
		s, ok := section(pkt, "约束")
		if !ok || !s.Locked || !strings.Contains(s.Text, "CNY") {
			t.Fatalf("exclude 去不掉约束段：%+v", pkt.Sections)
		}
		for _, d := range pkt.Dropped {
			if d.Section == "约束" {
				t.Errorf("约束段不应被收缩：%+v", d)
			}
		}
		n := 0
		for _, w := range pkt.Warnings {
			if strings.Contains(w, "透传约束占了预算") {
				n++
			}
		}
		if n != 1 {
			t.Errorf("约束超过预算 30%% 时应有一条告警：%v（约束 %d / 预算 %d）", pkt.Warnings, s.Tokens, pkt.Budget)
		}
	})
}

func ptr[T any](v T) *T { return &v }
