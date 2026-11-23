package main

import (
	"strings"
	"testing"
)

// opKinds 是 C.05.02 §2–§6 表里的 30 种 Op（C.05.01 §2）。
var opKinds = []string{
	"add_node", "remove_nodes", "restore_nodes", "connect", "disconnect", "set_param", "promote_param", "demote_param",
	"set_label", "set_tags", "set_lock", "group", "ungroup", "set_subgraph_ref",
	"move_nodes", "set_node_ui", "add_note", "update_note", "remove_notes",
	"set_graph_title", "set_graph_meta", "set_mandate",
	"apply_patch", "import_apply", "revert_op", "revert_to", "migrate",
	"create_branch", "delete_branch", "merge_branch",
}

const opRef = "op.schema.json#/$defs/Op"

// 测试用的 ULID（26 位 Crockford Base32，没有 I、L、O、U）。
const (
	opID1 = "01JD8X3Q5V7W9Y2Z4A6B8C0D1E"
	opID2 = "01JD8X3Q5V7W9Y2Z4A6B8C0D2F"
	opID3 = "01JD8X3Q5V7W9Y2Z4A6B8C0D3G"
	opID4 = "01JD8X3Q5V7W9Y2Z4A6B8C0D4H"
	opID5 = "01JD8X3Q5V7W9Y2Z4A6B8C0D5J"
)

// opKindEnum 读 op.schema.json 里 OpKind 的枚举值。
func opKindEnum(t *testing.T, s *Set) []string {
	t.Helper()
	defs, _ := s.Raw["op.schema.json"]["$defs"].(map[string]any)
	kind, _ := defs["OpKind"].(map[string]any)
	enum, ok := kind["enum"].([]any)
	if !ok {
		t.Fatal("op.schema.json 里没有 $defs/OpKind 的 enum")
	}
	out := make([]string, 0, len(enum))
	for _, v := range enum {
		str, ok := v.(string)
		if !ok {
			t.Fatalf("OpKind 的枚举值 %v 不是字符串", v)
		}
		out = append(out, str)
	}
	return out
}

// opWith 是最小 Op（只有 kind、payload）再加上 extra 里的字段（extra 以逗号开头）。
// payload 用 remove_nodes 的合法写法：C.05.02 按 kind 判别 payload 后它仍然合法。
func opWith(extra string) string {
	return `{"kind": "remove_nodes", "payload": {"ids": ["` + opID5 + `"]}` + extra + `}`
}

func TestOp(t *testing.T) {
	t.Run("C0501-T4 信封", func(t *testing.T) {
		s := repoSet(t)
		if got := opKindEnum(t, s); len(got) != len(opKinds) || !sameSet(got, opKinds) {
			t.Errorf("OpKind 是 %v，应为 C.05.02 §2–§6 的 30 种：%v", got, opKinds)
		}
		mustValid(t, s, opRef, opWith(""))
		full := `, "id": "` + opID1 + `", "graph_id": "` + opID2 + `", "branch": "draft/try-1", "base_version": 42` +
			`, "actor": {"kind": "model", "id": "large-model", "instance_id": "chat:` + opID3 + `"}` +
			`, "approver": {"kind": "user", "id": "local"}, "mandate_id": "` + opID4 + `", "reason": "加一个执行者", "taint": true` +
			`, "target": {"scope": "root", "nodes": ["` + opID5 + `"], "edges": [], "notes": []}` +
			`, "inverse": [{"kind": "remove_nodes", "payload": {"ids": ["` + opID5 + `"]}}]` +
			`, "group_id": "` + opID1 + `", "coalesce_key": "drag:` + opID5 + `", "ts": "2026-11-23T12:01:03.120Z"`
		mustValid(t, s, opRef, opWith(full))
		mustValid(t, s, opRef, `{"kind": "set_param", "payload": {"node": "`+opID5+`", "key": "temperature", "to": 0.7}, "coalesce_key": "slide:`+opID5+`"}`)

		mustInvalid(t, s, opRef, `{"payload": {"ids": ["`+opID5+`"]}}`, "")
		mustInvalid(t, s, opRef, `{"kind": "remove_node", "payload": {"ids": ["`+opID5+`"]}}`, "/kind")
		mustInvalid(t, s, opRef, opWith(`, "author": "local"`), "")
		mustInvalid(t, s, opRef, opWith(`, "coalesce_key": "push:`+opID1+`"`), "/coalesce_key")
		mustInvalid(t, s, opRef, opWith(`, "branch": "feature/x"`), "/branch")
		mustInvalid(t, s, opRef, opWith(`, "id": "01JD8X3Q5V7W9Y2Z4A6B8C0DIE"`), "/id")
		mustInvalid(t, s, opRef, opWith(`, "inverse": [{"kind": "remove_nodes"}]`), "/inverse/0")
		mustInvalid(t, s, opRef, opWith(`, "target": {"scope": "主图"}`), "/target/scope")

		const tmpRef = "op.schema.json#/$defs/TmpId"
		mustValid(t, s, tmpRef, `"tmp:1"`)
		mustValid(t, s, tmpRef, `"tmp:9999"`)
		mustInvalid(t, s, tmpRef, `"tmp:a"`, "")
		mustInvalid(t, s, tmpRef, `"tmp:10000"`, "")
	})
	t.Run("C0501-T5 请求和响应", func(t *testing.T) {
		s := repoSet(t)
		const reqRef = "op.schema.json#/$defs/OpsRequest"
		const appliedRef = "op.schema.json#/$defs/OpsApplied"
		const proposedRef = "op.schema.json#/$defs/OpsProposed"
		one := opWith("")
		ops := func(n int) string { return strings.TrimSuffix(strings.Repeat(one+", ", n), ", ") }

		mustValid(t, s, reqRef, `{"branch": "main", "base_version": 42, "group_id": "`+opID1+`", "ops": [`+ops(2)+`]}`)
		mustValid(t, s, reqRef, `{"branch": "draft/a", "base_version": 0, "ops": [`+ops(500)+`]}`)
		mustValid(t, s, appliedRef, `{"version": 44, "applied": ["`+opID2+`", "`+opID3+`"], "rebased": false, "id_map": {"tmp:1": "`+opID4+`"}, "diags": []}`)
		mustValid(t, s, appliedRef, `{"version": 44, "applied": [], "rebased": true, "id_map": {}, "diags": []}`)
		mustValid(t, s, proposedRef, `{"proposal_id": "`+opID5+`", "status": "open"}`)

		mustInvalid(t, s, reqRef, `{"branch": "main", "base_version": 42, "ops": []}`, "/ops")
		mustInvalid(t, s, reqRef, `{"branch": "main", "base_version": 42, "ops": [`+ops(501)+`]}`, "/ops")
		mustInvalid(t, s, reqRef, `{"branch": "main", "base_version": -1, "ops": [`+one+`]}`, "/base_version")
		mustInvalid(t, s, proposedRef, `{"proposal_id": "`+opID5+`", "status": "applied"}`, "/status")
		// 键名不合法（propertyNames）：校验器把这类错误记在根位置，不在对象下面，所以位置写 ""（同 nodedef_test.go 的做法）；
		// 只断言被拒绝；它是因为键名被拒、不是别的原因，由上面键为 tmp:1、其余相同的正例能通过保证
		mustInvalid(t, s, appliedRef, `{"version": 44, "applied": ["`+opID2+`", "`+opID3+`"], "rebased": false, "id_map": {"`+opID1+`": "`+opID4+`"}, "diags": []}`, "")
	})
}
