package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mio-cc/anywhere-door/gen"
)

func TestTables(t *testing.T) {
	t.Run("C14-T5 状态机", func(t *testing.T) {
		// C.07 §2、§3.1 的表（期望照文档写，不读 state_machine.yaml）
		run := []string{"pending", "running", "paused", "done", "failed", "cancelled"}
		runNext := map[string][]string{
			"pending": {"running", "cancelled"},
			"running": {"paused", "done", "failed", "cancelled"},
			"paused":  {"running", "cancelled"},
		}
		runTerm := map[string]bool{"done": true, "failed": true, "cancelled": true}
		for _, a := range run {
			for _, b := range run {
				want := contains(runNext[a], b)
				if got := gen.RunCanTransition(gen.RunStatus(a), gen.RunStatus(b)); got != want {
					t.Errorf("RunCanTransition(%s, %s) = %v，C.07 §2 是 %v", a, b, got, want)
				}
			}
			if got := gen.RunIsTerminal(gen.RunStatus(a)); got != runTerm[a] {
				t.Errorf("RunIsTerminal(%s) = %v", a, got)
			}
		}
		node := []string{"idle", "queued", "running", "waiting", "done", "failed", "cancelled", "skipped", "cached"}
		nodeNext := map[string][]string{
			"idle":    {"queued", "cancelled", "skipped", "cached"},
			"queued":  {"running", "cancelled"},
			"running": {"queued", "waiting", "done", "failed", "cancelled"},
			"waiting": {"running", "failed", "cancelled"},
			"failed":  {"queued"},
		}
		nodeTerm := map[string]bool{"done": true, "failed": true, "cancelled": true, "skipped": true, "cached": true}
		for _, a := range node {
			for _, b := range node {
				want := contains(nodeNext[a], b)
				if got := gen.NodeCanTransition(gen.NodeState(a), gen.NodeState(b)); got != want {
					t.Errorf("NodeCanTransition(%s, %s) = %v，C.07 §3.1 是 %v", a, b, got, want)
				}
			}
			if got := gen.NodeIsTerminal(gen.NodeState(a)); got != nodeTerm[a] {
				t.Errorf("NodeIsTerminal(%s) = %v", a, got)
			}
		}
	})
	t.Run("C14-T7 数据表与文案", func(t *testing.T) {
		dir := filepath.Join(repoRoot, "schema", "builtin")
		var cr []containerRuleYAML
		if _, err := readYAMLStrict(filepath.Join(dir, "container_rules.yaml"), &cr); err != nil {
			t.Fatal(err)
		}
		var wantCR []gen.ContainerRule
		for _, r := range cr {
			wantCR = append(wantCR, gen.ContainerRule{Type: r.Type, Children: r.Children, Ref: r.Ref})
		}
		if !reflect.DeepEqual(gen.ContainerRules, wantCR) {
			t.Errorf("ContainerRules 和 container_rules.yaml 不一致：%v / %v", gen.ContainerRules, wantCR)
		}
		var ops map[string]string
		if _, err := readYAMLStrict(filepath.Join(dir, "aggregate_ops.yaml"), &ops); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(gen.AggregateOps, ops) {
			t.Errorf("AggregateOps 和 aggregate_ops.yaml 不一致")
		}
		var az authzYAML
		if _, err := readYAMLStrict(filepath.Join(dir, "authz_matrix.yaml"), &az); err != nil {
			t.Fatal(err)
		}
		if len(gen.AuthzRows) != len(az.Rows) || len(gen.AuthzFixed) != len(az.Fixed) || len(gen.AuthzOpActions) != len(az.OpActions) {
			t.Errorf("Authz 表的行数和 authz_matrix.yaml 不一致")
		}
		if gen.AuthzFixed["op.set_lock"] != "user" || gen.AuthzOpActions["apply_patch"] != "per_sub_op" {
			t.Errorf("AuthzFixed / AuthzOpActions 内容不对：%v %v", gen.AuthzFixed, gen.AuthzOpActions)
		}
		for i, r := range az.Rows {
			if i < len(gen.AuthzRows) && len(gen.AuthzRows[i].Actions) != len(r.Actions) {
				t.Errorf("AuthzRows[%d] 的动作数和 YAML 不一致", i)
			}
		}
		var wm struct {
			Rules []widgetRuleYAML `yaml:"rules"`
		}
		if _, err := readYAMLStrict(filepath.Join(dir, "widget_map.yaml"), &wm); err != nil {
			t.Fatal(err)
		}
		if len(gen.WidgetRules) != len(wm.Rules) || gen.WidgetRules[0].When.SpanSteps != wm.Rules[0].When.SpanSteps {
			t.Errorf("WidgetRules 和 widget_map.yaml 不一致")
		}

		// 反例：写坏的表和文案生成失败
		m := repoModel(t)
		bad := t.TempDir()
		for _, f := range []string{"container_rules.yaml", "widget_map.yaml", "category_map.yaml", "aggregate_ops.yaml", "authz_matrix.yaml", "state_machine.yaml"} {
			b, err := os.ReadFile(filepath.Join(dir, f))
			if err != nil {
				t.Fatal(err)
			}
			if f == "authz_matrix.yaml" {
				s := string(b)
				if !strings.Contains(s, "actions: [op.view]") {
					t.Fatal("authz_matrix.yaml 里找不到 actions: [op.view]，测试要跟着改")
				}
				b = []byte(strings.Replace(s, "actions: [op.view]", "actions: [op.view, op.drop_all]", 1))
			}
			if err := os.WriteFile(filepath.Join(bad, f), b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if _, _, err := emitTables(m, bad); err == nil || !strings.Contains(err.Error(), "op.drop_all") {
			t.Errorf("authz_matrix.yaml 写了不存在的动作应当生成失败，实际：%v", err)
		}
		msgDir := filepath.Join(dir, "messages")
		badMsg := t.TempDir()
		eb, err := os.ReadFile(filepath.Join(msgDir, "errors.zh.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		var kept []string
		for _, line := range strings.Split(string(eb), "\n") {
			if !strings.HasPrefix(line, "bad_request:") {
				kept = append(kept, line)
			}
		}
		ab, err := os.ReadFile(filepath.Join(msgDir, "assign.zh.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(badMsg, "errors.zh.yaml"), []byte(strings.Join(kept, "\n")), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(badMsg, "assign.zh.yaml"), append(ab, []byte("C3: 多出来的一条\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err = emitMessages(m, badMsg)
		if err == nil || !strings.Contains(err.Error(), "bad_request") || !strings.Contains(err.Error(), "C3") {
			t.Errorf("ERRORS 少一个错误码、ASSIGN 多一条时应当生成失败，实际：%v", err)
		}
	})
}

// wireLookup 把 wireTypes 变成 类型名 → 字段名 → 节点写法。
func wireLookup(m *model) (map[string]map[string]string, map[string]*union) {
	fields, unions := map[string]map[string]string{}, map[string]*union{}
	for _, wt := range m.wireTypes() {
		fields[wt.name] = map[string]string{}
		for _, f := range wt.fields {
			fields[wt.name][f[0]] = f[1]
		}
		if wt.union != nil {
			unions[wt.name] = wt.union
		}
	}
	return fields, unions
}

func TestWire(t *testing.T) {
	t.Run("C14-T6 字段映射表", func(t *testing.T) {
		m := repoModel(t)
		fields, unions := wireLookup(m)
		for _, c := range []struct{ typ, field, want string }{
			// 同名字段在不同类型里不同（P.09 K235 修的就是这几处）
			{"Node", "params", wireRaw},
			{"NodeDef", "params", `{ map: { t: "ParamDef" } }`},
			{"Node", "ui", wireRaw}, // NodeUi 是开放对象（C.02.05）
			{"ParamDef", "ui", `{ t: "ParamUI" }`},
			{"Mandate", "overrides", `{ t: "Overrides" }`},
			{"SubgraphRef", "overrides", wireRaw},
			{"Mandate", "expires", `{ t: "Expiry" }`},
			{"Overrides", "budget", `{ t: "Budget" }`},
			{"DataRunCreated", "budget", wireRaw},
			{"Message", "usage", `{ t: "MessageUsage" }`},
			{"DataLlmCall", "usage", wireRaw},
			{"PayloadAddNote", "note", `{ t: "Note" }`},
			{"DataNodeProgress", "note", wireRaw},
			{"Message", "blocks", `{ list: { t: "MessageBlock" } }`},
			{"Subgraph", "nodes", `{ list: { t: "Node" } }`},
			{"Op", "inverse", `{ list: { t: "Op" } }`},
			// 映射：键是数据
			{"Session", "orchestrator_effort", wireRaw},
			{"Proposal", "decisions", wireRaw},
			{"OpsApplied", "id_map", wireRaw},
			// 联合字段由 union 决定
			{"Op", "payload", wireRaw},
			{"RunEvent", "data", wireRaw},
		} {
			got, ok := fields[c.typ][c.field]
			if !ok {
				t.Errorf("%s.%s 不在映射表里", c.typ, c.field)
				continue
			}
			if got != c.want {
				t.Errorf("%s.%s 是 %s，应为 %s", c.typ, c.field, got, c.want)
			}
		}
		for _, c := range []struct {
			typ, disc, field string
			branches         int
		}{
			{"Op", "kind", "payload", 30},
			{"RunEvent", "kind", "data", 39},
			{"GraphEvent", "kind", "data", 10},
			{"SessionEvent", "kind", "data", 9},
			{"MessageBlock", "kind", "", 7},
			{"MessageRef", "kind", "", 4},
			{"Impl", "type", "", 5},
		} {
			u := unions[c.typ]
			if u == nil || u.disc != c.disc || u.field != c.field || len(u.branches) != c.branches {
				t.Errorf("%s 的 union 应为 disc=%s field=%q、%d 个分支，实际 %+v", c.typ, c.disc, c.field, c.branches, u)
			}
		}
		if b := unions["Op"]; b != nil && b.branches[0].typ != "PayloadAddNode" {
			t.Errorf("Op 的第一个分支应为 add_node → PayloadAddNode，实际 %+v", b.branches[0])
		}

		set, err := loadSchemas(filepath.Join("testdata", "schema"))
		if err != nil {
			t.Fatal(err)
		}
		al, err := readAliases(filepath.Join("testdata", "aliases.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		tm, err := buildModel(set, al.Roots)
		if err != nil {
			t.Fatal(err)
		}
		tf, tu := wireLookup(tm)
		for _, c := range []struct{ typ, field, want string }{
			{"Thing", "box", `{ t: "ThingBox" }`},
			{"Thing", "rows", `{ list: { t: "ThingRowsItem" } }`},
			{"Thing", "open", wireRaw},
			{"Thing", "scores", wireRaw},
			{"Thing", "tags", wireRaw},
			{"Thing", "child", `{ t: "Thing" }`},
			{"Thing", "shape", `{ t: "Shape" }`},
			{"CmdSay", "mood", `{ t: "ThingBox" }`},
		} {
			if got := tf[c.typ][c.field]; got != c.want {
				t.Errorf("testdata：%s.%s 是 %q，应为 %s", c.typ, c.field, got, c.want)
			}
		}
		if u := tu["Cmd"]; u == nil || u.field != "args" || len(u.branches) != 2 {
			t.Errorf("testdata：Cmd 应当是字段联合（args，2 个分支）：%+v", u)
		}
		if _, ok := tf["ShapeCircle"]; !ok {
			t.Errorf("testdata：整体联合的分支 ShapeCircle 应当在映射表里")
		}
	})
}
