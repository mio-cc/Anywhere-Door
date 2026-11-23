package main

import "testing"

func TestEvent(t *testing.T) {
	t.Run("C0601-T3 事件信封", func(t *testing.T) {
		s := repoSet(t)
		const ref = "event.schema.json#/$defs/Event"
		const ts = `"ts": "2026-11-22T10:02:03.120Z"`
		const actor = `"actor": {"kind": "runtime", "id": "sched"}`
		const run = `"run_id": "01JDRVN0000000000000000001"`
		const head = ts + ", " + actor + ", " + run
		const loc = `"loc": {"node_id": "01JD8X3Q5V7W9Y2Z4A6B8C0D1E", "path": "根图/闭环迭代#r2/执行者#3", "iter": {"round": 2, "fan": 3}}`
		mustValid(t, s, ref, `{"seq": 1, "kind": "node.progress", `+head+`, "data": {"pct": 62}}`)
		mustValid(t, s, ref, `{"seq": 0, "kind": "node.chunk", `+head+`, "data": {"port": "out", "delta": "价格"}}`)
		mustValid(t, s, ref, `{"seq": 118, "kind": "approval.resolved", `+head+`, `+loc+`, "cause_seq": 117, "data": {}}`)

		mustInvalid(t, s, ref, `{"seq": 1, "kind": "node.progress", `+ts+`, `+run+`, "data": {}}`, "")
		mustInvalid(t, s, ref, `{"seq": -1, "kind": "node.progress", `+head+`, "data": {}}`, "/seq")
		mustInvalid(t, s, ref, `{"seq": 1, "kind": "node.progress", `+head+`, "data": "62"}`, "/data")
		mustInvalid(t, s, ref, `{"seq": 1, "kind": "node.progress", `+head+`, "data": {}, "level": "P"}`, "")
		mustInvalid(t, s, ref, `{"seq": 1, "kind": "node.progress", "ts": "2026-11-22T10:02:03.120+08:00", `+actor+`, `+run+`, "data": {}}`, "/ts")
	})
}
