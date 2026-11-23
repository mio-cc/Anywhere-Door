package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// eventSpec 是一个事件在目录表里的样子：名字、级别（P / S / E）、是不是节点级（要带 loc）。
type eventSpec struct {
	kind, level string
	loc         bool
}

// runEvents 是 C.06.02 §2–§8 的 39 个运行事件（名单和级别照表抄）。
var runEvents = []eventSpec{
	{"run.created", "P", false},
	{"run.status", "P", false},
	{"run.cancel_requested", "P", false},
	{"run.graph_swapped", "P", false},
	{"engine.illegal_transition", "P", false},
	{"node.state", "P", true},
	{"node.progress", "S", true},
	{"node.chunk", "E", true},
	{"node.input", "P", true},
	{"node.output", "P", true},
	{"node.throttled", "S", true},
	{"node.log", "P", true},
	{"loop.round", "P", false},
	{"checkpoint.saved", "P", false},
	{"ledger.changed", "P", false},
	{"llm.call", "P", false},
	{"llm.fallback", "P", false},
	{"tool.intent", "P", false},
	{"tool.call", "P", false},
	{"ctx.assembled", "P", false},
	{"prompt.rendered", "P", false},
	{"orch.started", "P", false},
	{"orch.tool_call", "E", false},
	{"orch.expanded", "P", false},
	{"orch.finished", "P", false},
	{"approval.requested", "P", false},
	{"approval.resolved", "P", false},
	{"escalation.raised", "P", false},
	{"escalation.merged", "P", false},
	{"escalation.routed", "P", false},
	{"escalation.resolved", "P", false},
	{"budget.charged", "P", false},
	{"budget.warn", "P", false},
	{"budget.exceeded", "P", false},
	{"estop.triggered", "P", false},
	{"estop.released", "P", false},
	{"brief.updated", "P", false},
	{"edge.flow", "E", false},
	{"pool.usage", "E", false},
}

// eventDataDef 是事件 data 的定义名：Data 加事件名的驼峰写法（node.state → DataNodeState，C.06.01-R6）。
func eventDataDef(kind string) string {
	var b strings.Builder
	b.WriteString("Data")
	for _, w := range strings.FieldsFunc(kind, func(r rune) bool { return r == '.' || r == '_' }) {
		b.WriteString(strings.ToUpper(w[:1]) + w[1:])
	}
	return b.String()
}

// eventBranch 是流 schema 里一种 kind 的分支：seq 的约束（"≥1" 或 "0"）、data 引用的定义、是否要求 loc。
type eventBranch struct {
	seq, data string
	loc       bool
}

// eventBranches 读流 schema 的顶层：kind 的枚举，和 allOf 里每种 kind 的分支（allOf[0] 是 Event，C.06.01-R6）。
func eventBranches(t *testing.T, s *Set, file string) ([]string, map[string]eventBranch) {
	t.Helper()
	raw, ok := s.Raw[file]
	if !ok {
		t.Fatalf("schema/%s 不存在", file)
	}
	props, _ := raw["properties"].(map[string]any)
	kindSch, _ := props["kind"].(map[string]any)
	enum, ok := kindSch["enum"].([]any)
	if !ok {
		t.Fatalf("%s 顶层没有 kind 的枚举", file)
	}
	var kinds []string
	for _, v := range enum {
		str, _ := v.(string)
		kinds = append(kinds, str)
	}
	all, _ := raw["allOf"].([]any)
	if len(all) == 0 {
		t.Fatalf("%s 顶层没有 allOf", file)
	}
	if first, _ := all[0].(map[string]any); first["$ref"] != "event.schema.json#/$defs/Event" {
		t.Errorf("%s 的 allOf[0] 应当是 event.schema.json#/$defs/Event", file)
	}
	branches := map[string]eventBranch{}
	for i, item := range all[1:] {
		m, _ := item.(map[string]any)
		ifs, _ := m["if"].(map[string]any)
		ifProps, _ := ifs["properties"].(map[string]any)
		k, _ := ifProps["kind"].(map[string]any)
		kind, _ := k["const"].(string)
		if kind == "" {
			t.Errorf("%s 的 allOf[%d] 没有写 kind 的 const", file, i+1)
			continue
		}
		then, _ := m["then"].(map[string]any)
		tp, _ := then["properties"].(map[string]any)
		seq, _ := tp["seq"].(map[string]any)
		data, _ := tp["data"].(map[string]any)
		var b eventBranch
		b.data, _ = data["$ref"].(string)
		switch {
		case isNum(seq["const"], 0):
			b.seq = "0"
		case isNum(seq["minimum"], 1):
			b.seq = "≥1"
		}
		req, _ := then["required"].([]any)
		for _, r := range req {
			if r == "loc" {
				b.loc = true
			}
		}
		if _, dup := branches[kind]; dup {
			t.Errorf("%s 的 allOf 里 %s 出现了两次", file, kind)
		}
		branches[kind] = b
	}
	return kinds, branches
}

// seqByLevel：运行流、会话流按级别定 seq（C.06.01-R8）：P、S 级 ≥ 1，E 级为 0。
func seqByLevel(e eventSpec) string {
	if e.level == "E" {
		return "0"
	}
	return "≥1"
}

// checkEventNames：kind 的枚举恰好是 want；每种 kind 一条分支，seq 的约束、是否要求 loc、data 的引用与表一致；Data… 定义和 kind 一一对应。
func checkEventNames(t *testing.T, s *Set, file string, want []eventSpec, seqFor func(eventSpec) string) {
	t.Helper()
	kinds, branches := eventBranches(t, s, file)
	names := make([]string, 0, len(want))
	defsWant := make([]string, 0, len(want))
	for _, e := range want {
		names = append(names, e.kind)
		defsWant = append(defsWant, eventDataDef(e.kind))
	}
	if len(kinds) != len(names) || !sameSet(kinds, names) {
		t.Errorf("%s 的 kind 是 %v，应为这 %d 个：%v", file, kinds, len(names), names)
	}
	if len(branches) != len(names) {
		t.Errorf("%s 的 allOf 有 %d 种 kind 的分支，应为 %d 种", file, len(branches), len(names))
	}
	for _, e := range want {
		b, ok := branches[e.kind]
		if !ok {
			t.Errorf("%s：没有 %s 的分支", file, e.kind)
			continue
		}
		if b.seq != seqFor(e) {
			t.Errorf("%s 的 seq 约束是 %q，%s 级应为 %q", e.kind, b.seq, e.level, seqFor(e))
		}
		if b.loc != e.loc {
			t.Errorf("%s：分支要求 loc 是 %v，应为 %v（节点级事件要带 loc）", e.kind, b.loc, e.loc)
		}
		if b.data != "#/$defs/"+eventDataDef(e.kind) {
			t.Errorf("%s 的 data 应当引用 #/$defs/%s，实际是 %q", e.kind, eventDataDef(e.kind), b.data)
		}
	}
	defs, _ := s.Raw[file]["$defs"].(map[string]any)
	var got []string
	for name := range defs {
		if strings.HasPrefix(name, "Data") {
			got = append(got, name)
		}
	}
	if !sameSet(got, defsWant) {
		t.Errorf("%s 的 Data… 定义是 %v，应当和 kind 一一对应：%v", file, got, defsWant)
	}
}

// eventVector 是事件向量文件的一项（C.06.02–C.06.04 实施卡的向量格式）。
type eventVector struct {
	Name    string          `json:"name"`
	Event   json.RawMessage `json:"event"`
	Valid   *bool           `json:"valid"`
	ErrorAt *string         `json:"error_at"`
}

// checkEventVectors：向量文件里每一项按 file 校验，结果和 valid、error_at 一致；want 里每种 kind 至少一个合法事件。
func checkEventVectors(t *testing.T, s *Set, file, vectors string, want []eventSpec) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(vectors)))
	if err != nil {
		t.Fatal(err)
	}
	var vs []eventVector
	if err := strictJSON(b, &vs); err != nil {
		t.Fatalf("%s：%v", vectors, err)
	}
	valid, names := map[string]int{}, map[string]bool{}
	for i, v := range vs {
		if v.Name == "" || names[v.Name] {
			t.Errorf("%s 第 %d 项：name 为空或重复：%q", vectors, i, v.Name)
		}
		names[v.Name] = true
		if len(v.Event) == 0 || v.Valid == nil || *v.Valid == (v.ErrorAt != nil) {
			t.Errorf("%s：要写 event 和 valid；valid 为 false 时写 error_at，为 true 时不写", v.Name)
			continue
		}
		if !*v.Valid {
			mustInvalid(t, s, file, string(v.Event), *v.ErrorAt)
			continue
		}
		var head struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(v.Event, &head); err != nil {
			t.Errorf("%s：%v", v.Name, err)
			continue
		}
		valid[head.Kind]++
		mustValid(t, s, file, string(v.Event))
	}
	for _, e := range want {
		if valid[e.kind] == 0 {
			t.Errorf("%s 里没有 %s 的合法事件，每种 kind 至少一个", vectors, e.kind)
		}
	}
}

func TestRunEvent(t *testing.T) {
	t.Run("C0602-T1 事件名单", func(t *testing.T) {
		s := repoSet(t)
		if len(runEvents) != 39 {
			t.Fatalf("测试里的名单有 %d 个，C.06.02 §2–§8 是 39 个", len(runEvents))
		}
		checkEventNames(t, s, "run-event.schema.json", runEvents, seqByLevel)
	})
	t.Run("C0602-T2 事件向量", func(t *testing.T) {
		s := repoSet(t)
		checkEventVectors(t, s, "run-event.schema.json", "schema/examples/run_event_vectors.json", runEvents)
	})
}
