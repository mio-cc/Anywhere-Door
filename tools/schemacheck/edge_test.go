package main

import "testing"

const edgeRef = "graph.schema.json#/$defs/Edge"

func edgeJSON(from, to string) string {
	return `{"id": "01JDE0000000000000000000X1", "from": {"node": "01JDN0000000000000000000X1", "port": "` + from +
		`"}, "to": {"node": "01JDN0000000000000000000X2", "port": "` + to + `"}}`
}

func TestEdge(t *testing.T) {
	t.Run("C0203-T1 端口名", func(t *testing.T) {
		s := repoSet(t)
		for f, at := range map[string]string{
			"invalid/edge_port_uppercase.json":  "/root/edges/0/from/port",
			"invalid/edge_port_index_1000.json": "/root/edges/0/to/port",
		} {
			if e := manifestEntry(t, s, f); e.ErrorAt == nil || *e.ErrorAt != at {
				t.Errorf("清单里 %s 的 error_at 应为 %q：%+v", f, at, e)
			}
		}
		for _, p := range []string{"out", "in[0]", "in[999]", "_x", "out_2"} {
			mustValid(t, s, edgeRef, edgeJSON(p, "in"))
		}
		for _, p := range []string{"Result", "in[1000]", "in[]", "in[-1]", "2out", "a-b", ""} {
			mustInvalid(t, s, edgeRef, edgeJSON("out", p), "/to/port")
		}
		mustInvalid(t, s, edgeRef, `{"id": "01JDE0000000000000000000X1", "from": {"node": "01JDN0000000000000000000X1", "port": "out"}}`, "")
		mustInvalid(t, s, edgeRef, `{"id": "01JDE0000000000000000000X1", "from": {"node": "01JDN0000000000000000000X1", "port": "out", "type": "方案@1"}, "to": {"node": "01JDN0000000000000000000X2", "port": "in"}}`, "/from")
		mustValid(t, s, "common.schema.json#/$defs/PortRef", `{"node": "01JDN0000000000000000000X1", "port": "in[3]"}`)
	})
	t.Run("C0203-T2 跨子图示例", func(t *testing.T) {
		s := repoSet(t)
		e := manifestEntry(t, s, "semantic/edge_cross_scope.json")
		if e.Valid == nil || !*e.Valid || e.Diag != "V05" {
			t.Errorf("清单里 edge_cross_scope.json 应当 valid: true、diag: V05：%+v", e)
		}
	})
}
