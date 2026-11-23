package main

import "testing"

// graphEvents 是 C.06.03 §2 的 10 个图协同事件（名单和级别照表抄）。
var graphEvents = []eventSpec{
	{"op.applied", "P", false},
	{"op.coalesced", "P", false},
	{"branch.updated", "P", false},
	{"proposal.created", "P", false},
	{"proposal.updated", "P", false},
	{"proposal.decided", "P", false},
	{"mandate.changed", "P", false},
	{"validate", "E", false},
	{"presence", "E", false},
	{"model.activity", "E", false},
}

// graphSeq：图协同流只有 op.applied、op.coalesced 带序号（图版本号，≥ 1），其余为 0（C.06.03-R1）。
func graphSeq(e eventSpec) string {
	if e.kind == "op.applied" || e.kind == "op.coalesced" {
		return "≥1"
	}
	return "0"
}

func TestGraphEvent(t *testing.T) {
	t.Run("C0603-T1 事件名单", func(t *testing.T) {
		s := repoSet(t)
		if len(graphEvents) != 10 {
			t.Fatalf("测试里的名单有 %d 个，C.06.03 §2 是 10 个", len(graphEvents))
		}
		checkEventNames(t, s, "graph-event.schema.json", graphEvents, graphSeq)
	})
	t.Run("C0603-T3 事件向量", func(t *testing.T) {
		s := repoSet(t)
		checkEventVectors(t, s, "graph-event.schema.json", "schema/examples/graph_event_vectors.json", graphEvents)
	})
}
