package main

import "testing"

// sessionEvents 是 C.06.04 §2 的 9 个会话事件（名单和级别照表抄）。
var sessionEvents = []eventSpec{
	{"session.updated", "P", false},
	{"session.graph_attached", "P", false},
	{"session.graph_detached", "P", false},
	{"message.created", "P", false},
	{"message.delta", "E", false},
	{"message.block_added", "P", false},
	{"message.completed", "P", false},
	{"message.failed", "P", false},
	{"chat.status", "E", false},
}

func TestSessionEvent(t *testing.T) {
	t.Run("C0604-T1 事件名单", func(t *testing.T) {
		s := repoSet(t)
		if len(sessionEvents) != 9 {
			t.Fatalf("测试里的名单有 %d 个，C.06.04 §2 是 9 个", len(sessionEvents))
		}
		checkEventNames(t, s, "session-event.schema.json", sessionEvents, seqByLevel)
	})
	t.Run("C0604-T3 事件向量", func(t *testing.T) {
		s := repoSet(t)
		checkEventVectors(t, s, "session-event.schema.json", "schema/examples/session_event_vectors.json", sessionEvents)
	})
}
