package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

const (
	mandateExample = "mandate/mandate.json"
	mandateRef     = "mandate.schema.json#/$defs/Mandate"
	authzMatrix    = "schema/builtin/authz_matrix.yaml"
)

// authzTable 是 C.09 §5 的判定表：一组动作 → 档位 0–4 的判定；条件写成 条件?满足时:不满足时。
var authzTable = []struct{ actions, levels string }{
	{"graph.read run.read trace.read", "allow allow allow allow allow"},
	{"op.view", "deny allow allow allow allow"},
	{"op.add_node op.restore_nodes op.connect op.disconnect op.set_param op.promote_param op.demote_param op.set_label op.set_tags op.group op.set_subgraph_ref op.set_graph_title op.set_graph_meta op.create_branch op.merge_branch op.revert_op",
		"deny propose overrides_ops?allow:propose allow allow"},
	{"op.remove_nodes op.ungroup op.revert_to op.delete_branch", "deny propose ask allow allow"},
	{"res.edit_rolecard res.edit_recipe res.define_type", "deny propose ask allow allow"},
	{"res.switch_model", "deny propose in_overrides_models?allow:ask allow allow"},
	{"authz.set_mandate", "deny deny deny ask allow"},
	{"authz.request_elevation", "deny allow allow allow allow"},
	{"run.start run.pause", "deny ask allow allow allow"},
	{"run.cancel", "deny allow allow allow allow"},
	{"orch.spawn", "deny deny overrides_allow_spawn?allow:deny allow allow"},
	{"orch.self_modify", "deny deny deny ask allow"},
	{"tool.L0", "deny allow allow allow allow"},
	{"tool.L1 tool.L2", "deny ask within_tools_max_risk?allow:ask allow allow"},
	{"tool.L3", "deny ask ask ask allow"},
	{"estop.trigger", "allow allow allow allow allow"},
	{"estop.release", "deny deny deny deny deny"},
}

// authzFixed 是不走档位表的动作（C.09 §4.2），authzOpActions 是 §4.3 的例外。
var (
	authzFixed     = map[string]string{"op.set_lock": "user", "op.migrate": "runtime"}
	authzOpActions = map[string]string{
		"move_nodes": "op.view", "set_node_ui": "op.view", "add_note": "op.view", "update_note": "op.view", "remove_notes": "op.view",
		"set_mandate": "authz.set_mandate", "apply_patch": "per_sub_op", "import_apply": "per_sub_op",
	}
	authzDecisions  = map[string]bool{"allow": true, "propose": true, "ask": true, "deny": true}
	authzConditions = map[string]bool{"overrides_ops": true, "in_overrides_models": true, "overrides_allow_spawn": true, "within_tools_max_risk": true}
)

type authzRow struct {
	Actions []string          `json:"actions"`
	Levels  []json.RawMessage `json:"levels"`
}

type authzDoc struct {
	Rows      []authzRow        `json:"rows"`
	Fixed     map[string]string `json:"fixed"`
	OpActions map[string]string `json:"op_actions"`
}

// strictJSON 把 b 解到 v，不认识的字段报错。
func strictJSON(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// authzCell 把一格判定写成 authzTable 的写法；格式不对时 ok 为 false。
func authzCell(raw json.RawMessage) (string, bool) {
	var d string
	if json.Unmarshal(raw, &d) == nil {
		return d, authzDecisions[d]
	}
	var c struct {
		If   string `json:"if"`
		Then string `json:"then"`
		Else string `json:"else"`
	}
	if strictJSON(raw, &c) != nil || !authzConditions[c.If] || !authzDecisions[c.Then] || !authzDecisions[c.Else] {
		return string(raw), false
	}
	return c.If + "?" + c.Then + ":" + c.Else, true
}

// mandateActions 读 mandate.schema.json 里 Action 的枚举值。
func mandateActions(t *testing.T, s *Set) []string {
	t.Helper()
	defs, _ := s.Raw["mandate.schema.json"]["$defs"].(map[string]any)
	act, _ := defs["Action"].(map[string]any)
	enum, ok := act["enum"].([]any)
	if !ok {
		t.Fatal("mandate.schema.json 里没有 $defs/Action 的 enum")
	}
	var out []string
	for _, v := range enum {
		str, _ := v.(string)
		out = append(out, str)
	}
	return out
}

// mandateWith 读示例 mandate.json，用 edit 改一处后返回 JSON 文本。
func mandateWith(t *testing.T, edit func(m map[string]any)) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, "schema", "examples", filepath.FromSlash(mandateExample)))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	edit(m)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestMandate(t *testing.T) {
	t.Run("C09-T4 授权书 schema", func(t *testing.T) {
		s := repoSet(t)
		manifestEntry(t, s, mandateExample)
		mustValid(t, s, mandateRef, mandateWith(t, func(map[string]any) {}))
		const id = "01JD8X3Q5V7W9Y2Z4A6B8C0D1E"
		set := func(k string, v any) func(map[string]any) { return func(m map[string]any) { m[k] = v } }
		ops := func(k string, v any) func(map[string]any) {
			return func(m map[string]any) { m["overrides"].(map[string]any)["ops"].(map[string]any)[k] = v }
		}
		for _, c := range []struct {
			name string
			edit func(map[string]any)
			at   string
		}{
			{"level 为 5", set("level", 5), "/level"},
			{"grantee 为 user:x", set("grantee", "user:x"), "/grantee"},
			{"until 缺 until_at", set("expires", map[string]any{"kind": "until"}), "/expires"},
			{"forever 带 run_id", set("expires", map[string]any{"kind": "forever", "run_id": id}), "/expires"},
			{"until 带 run_id", set("expires", map[string]any{"kind": "until", "until_at": "2026-12-31T16:00:00.000Z", "run_id": id}), "/expires"},
			{"run 缺 run_id", set("expires", map[string]any{"kind": "run"}), "/expires"},
			{"created_by 是模型", set("created_by", map[string]any{"kind": "model", "id": "large-model", "instance_id": "chat:" + id}), "/created_by/kind"},
			{"deny 里的动作不存在", set("deny", []any{"op.drop_all"}), "/deny/0"},
			// 键名不合法（propertyNames）：校验器把这类错误记在根位置（同 nodedef_test.go），位置写 ""；其余与示例相同，示例能通过
			{"overrides.ops 的键不是动作", ops("op.drop_all", "allow"), ""},
			{"overrides.ops 的值为 ask", ops("op.add_node", "ask"), "/overrides/ops/op.add_node"},
			{"branch 为 feature", set("branch", "feature"), "/branch"},
			{"overrides.scope 写成名称", func(m map[string]any) { m["overrides"].(map[string]any)["scope"] = []any{"主图"} }, "/overrides/scope/0"},
			{"多出字段", set("owner", "local"), ""},
		} {
			t.Run(c.name, func(t *testing.T) { mustInvalid(t, s, mandateRef, mandateWith(t, c.edit), c.at) })
		}
		const expRef = "mandate.schema.json#/$defs/Expiry"
		mustValid(t, s, expRef, `{"kind": "run", "run_id": "`+id+`"}`)
		mustValid(t, s, expRef, `{"kind": "session"}`)
		mustValid(t, s, expRef, `{"kind": "forever"}`)
	})
	t.Run("C09-T5 判定表数据", func(t *testing.T) {
		s := repoSet(t)
		b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(authzMatrix)))
		if err != nil {
			t.Fatal(err)
		}
		j, err := yaml.YAMLToJSON(b)
		if err != nil {
			t.Fatalf("%s 不是合法的 YAML：%v", authzMatrix, err)
		}
		var doc authzDoc
		if err := strictJSON(j, &doc); err != nil {
			t.Fatalf("%s：%v", authzMatrix, err)
		}

		var got []string
		for i, r := range doc.Rows {
			if len(r.Levels) != 5 {
				t.Errorf("rows[%d]：levels 有 %d 格，应为 5 格（档位 0–4）", i, len(r.Levels))
			}
			cells := make([]string, 0, len(r.Levels))
			for k, raw := range r.Levels {
				cell, ok := authzCell(raw)
				if !ok {
					t.Errorf("rows[%d].levels[%d] 是 %s：只能写 allow / propose / ask / deny，或 {if, then, else}（条件只有 §5.1 的 4 种）", i, k, cell)
				}
				cells = append(cells, cell)
			}
			got = append(got, strings.Join(r.Actions, " ")+" = "+strings.Join(cells, " "))
		}
		var want []string
		for _, r := range authzTable {
			want = append(want, r.actions+" = "+r.levels)
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("rows 与 C.09 §5 不一致：\n实际：\n%s\n应为：\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
		}

		actions := mandateActions(t, s)
		if len(actions) != 43 {
			t.Errorf("Action 有 %d 个，C.09 §4.2 是 43 个", len(actions))
		}
		isAction := map[string]bool{}
		for _, a := range actions {
			isAction[a] = true
		}
		seen := map[string]int{}
		for _, r := range doc.Rows {
			for _, a := range r.Actions {
				seen[a]++
			}
		}
		for a := range doc.Fixed {
			seen[a]++
		}
		for _, a := range sortedKeys(seen) {
			if !isAction[a] {
				t.Errorf("判定表里的 %s 不在 Action 里", a)
			}
		}
		for _, a := range actions {
			if seen[a] != 1 {
				t.Errorf("%s 在 rows 和 fixed 里出现 %d 次，应当恰好 1 次", a, seen[a])
			}
		}
		if len(doc.Fixed) != len(authzFixed) {
			t.Errorf("fixed 是 %v，应为 %v", doc.Fixed, authzFixed)
		}
		for a, who := range authzFixed {
			if doc.Fixed[a] != who {
				t.Errorf("fixed.%s 是 %q，应为 %q", a, doc.Fixed[a], who)
			}
		}

		kinds := opKindEnum(t, s)
		isKind := map[string]bool{}
		for _, k := range kinds {
			isKind[k] = true
		}
		var extra []string
		for k := range doc.OpActions {
			if !isKind[k] {
				extra = append(extra, k)
			}
		}
		sort.Strings(extra)
		if len(extra) > 0 {
			t.Errorf("op_actions 里的 %v 不是 OpKind", extra)
		}
		if len(doc.OpActions) != len(authzOpActions) {
			t.Errorf("op_actions 是 %v，应为 C.09 §4.3 的 %v", doc.OpActions, authzOpActions)
		}
		for _, k := range kinds {
			a, listed := doc.OpActions[k]
			if !listed {
				a = "op." + k
			}
			if want, ok := authzOpActions[k]; ok && a != want {
				t.Errorf("op_actions.%s 是 %q，应为 %q", k, a, want)
			}
			if a != "per_sub_op" && !isAction[a] {
				t.Errorf("OpKind %s 对应的动作 %s 不在 Action 里", k, a)
			}
		}
	})
}
