package main

import (
	"strings"
	"testing"
)

const nodeRef = "graph.schema.json#/$defs/Node"

// nodeJSON 拼一个节点，extra 是追加在 pos 后面的字段（以逗号开头）。
func nodeJSON(typ, extra string) string {
	return `{"id": "01JDN0000000000000000000X1", "type": "` + typ + `", "pos": {"x": 0, "y": 0}` + extra + `}`
}

func TestNode(t *testing.T) {
	t.Run("C0202-T1 互斥", func(t *testing.T) {
		s := repoSet(t)
		if e := manifestEntry(t, s, "invalid/node_children_and_ref.json"); e.ErrorAt == nil || *e.ErrorAt != "/root/nodes/0" {
			t.Errorf("清单里 node_children_and_ref.json 的 error_at：%+v", e)
		}
		sub := `"children": {"nodes": [], "edges": []}`
		ref := `"ref": {"name": "资料采集", "version": "1.2.0", "pin": "link"}`
		mustValid(t, s, nodeRef, nodeJSON("子图引用", ", "+sub))
		mustValid(t, s, nodeRef, nodeJSON("子图引用", ", "+ref))
		mustInvalid(t, s, nodeRef, nodeJSON("子图引用", ", "+sub+", "+ref), "")
	})
	t.Run("C0202-T2 类型名", func(t *testing.T) {
		s := repoSet(t)
		for _, f := range []string{"invalid/node_type_unknown.json", "invalid/node_type_no_major.json"} {
			if e := manifestEntry(t, s, f); e.ErrorAt == nil || *e.ErrorAt != "/root/nodes/0/type" {
				t.Errorf("清单里 %s 的 error_at：%+v", f, e)
			}
		}
		atomic := []string{"资源", "Agent", "上下文装配", "工具", "扇出", "汇合", "聚合", "分支", "循环", "账本", "人工", "子图端口", "子图引用"}
		for _, typ := range append(atomic, "pricing-pack/价格标准化@1", "a.b-c_d/x@12") {
			mustValid(t, s, nodeRef, nodeJSON(typ, ""))
		}
		for _, typ := range []string{"调度器", "agent", "pack/x", "Pack/x@1", "pack/x@1.2", "pack/a/b@1", "pack/@1", "pack/" + strings.Repeat("名", 61) + "@1", ""} {
			mustInvalid(t, s, nodeRef, nodeJSON(typ, ""), "/type")
		}
	})
	t.Run("C0202-T3 开放字段", func(t *testing.T) {
		s := repoSet(t)
		mustValid(t, s, nodeRef, nodeJSON("Agent", `, "params": {"output_schema": "方案@1", "x_extra": {"any": [1, 2]}}, "ui": {"collapsed": true, "x_extra": 1}`))
		mustInvalid(t, s, nodeRef, nodeJSON("Agent", `, "status": "running"`), "")
		mustInvalid(t, s, nodeRef, nodeJSON("Agent", `, "lock": "kernel"`), "/lock")
		mustInvalid(t, s, nodeRef, nodeJSON("Agent", `, "promoted": ["Model"]`), "/promoted/0")
		mustInvalid(t, s, nodeRef, nodeJSON("Agent", `, "label": ""`), "/label")
		mustInvalid(t, s, nodeRef, `{"id": "01JDN0000000000000000000X1", "type": "Agent"}`, "")
	})
}
