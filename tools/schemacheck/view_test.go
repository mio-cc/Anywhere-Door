package main

import "testing"

func TestView(t *testing.T) {
	t.Run("C0205-T1 坐标整数", func(t *testing.T) {
		s := repoSet(t)
		if e := manifestEntry(t, s, "invalid/node_pos_float.json"); e.ErrorAt == nil || *e.ErrorAt != "/root/nodes/0/pos/x" {
			t.Errorf("清单里 node_pos_float.json 的 error_at：%+v", e)
		}
		const pos = "graph.schema.json#/$defs/Pos"
		mustValid(t, s, pos, `{"x": -40, "y": 0}`)
		mustInvalid(t, s, pos, `{"x": 0, "y": 1.5}`, "/y")
		mustInvalid(t, s, pos, `{"x": 0}`, "")
		mustInvalid(t, s, pos, `{"x": 0, "y": 0, "z": 0}`, "")
	})
	t.Run("C0205-T2 开放的 ui", func(t *testing.T) {
		s := repoSet(t)
		manifestEntry(t, s, "graph_ui_unknown.json")
		if e := manifestEntry(t, s, "invalid/note_size_extra.json"); e.ErrorAt == nil || *e.ErrorAt != "/root/notes/0/size" {
			t.Errorf("清单里 note_size_extra.json 的 error_at：%+v", e)
		}
		const ui = "graph.schema.json#/$defs/NodeUi"
		mustValid(t, s, ui, `{"x_extra": {"anything": true}}`)
		mustInvalid(t, s, ui, `{"width": 100}`, "/width")
		mustInvalid(t, s, ui, `{"color": "red"}`, "/color")
		const note = "graph.schema.json#/$defs/Note"
		mustValid(t, s, note, `{"id": "01JDNTVW000000000000000003", "pos": {"x": 0, "y": 0}, "text": "便签"}`)
		mustInvalid(t, s, note, `{"id": "01JDNTVW000000000000000003", "pos": {"x": 0, "y": 0}, "text": "便签", "ui": {}}`, "")
		mustInvalid(t, s, note, `{"id": "01JDNTVW000000000000000003", "pos": {"x": 0, "y": 0}, "text": "便签", "size": {"w": 79, "h": 40}}`, "/size/w")
	})
}
