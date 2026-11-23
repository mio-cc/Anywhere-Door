package main

import (
	"strings"
	"testing"
)

const (
	apiID1 = "01JD8X3Q5V7W9Y2Z4A6B8C0D1E"
	apiID2 = "01JD8X3Q5V7W9Y2Z4A6B8C0D2F"
	apiTS  = "2026-11-24T08:00:00.000Z"
)

// apiValid 是 api.schema.json 每个结构的一个合法例（E0701-T4）。
var apiValid = map[string]string{
	"GraphMeta": `{"id": "` + apiID1 + `", "title": "写作流水线", "version": 3, "stats": {"layers": 2, "subgraphs": 1, "nodes": 7}` +
		`, "created_at": "` + apiTS + `", "updated_at": "` + apiTS + `"}`,
	"BranchMeta": `{"name": "draft/a", "base_version": 2, "head": 4, "behind": 1, "ahead": 2` +
		`, "created_by": {"kind": "user", "id": "local"}, "created_at": "` + apiTS + `"}`,
	"OpRecord": `{"graph_id": "` + apiID1 + `", "branch": "main", "version": 3, "undone": false` +
		`, "op": {"kind": "set_graph_title", "payload": {"to": "写作"}, "id": "` + apiID2 + `", "actor": {"kind": "user", "id": "local"}, "ts": "` + apiTS + `"}}`,
	"RunSteps": `{"done": 2, "total": 5}`,
	"RunMeta": `{"id": "` + apiID2 + `", "kind": "run", "graph_id": "` + apiID1 + `", "branch": "main", "graph_version": 3, "status": "running", "no": 1` +
		`, "headline": {"kind": "running_at", "node_label": "写作"}, "steps": {"done": 2, "total": 5}, "usage": {"in": 10, "out": 20, "cached": 0}` +
		`, "cost_micros": 1200, "last_seq": 17, "started_by": {"kind": "user", "id": "local"}, "created_at": "` + apiTS + `"}`,
	"NodeStateRow": `{"exec_key": "x1", "node_id": "` + apiID1 + `", "loc": {"node_id": "` + apiID1 + `", "path": "根图/写作"}` +
		`, "state": "running", "attempt": 1, "progress": 0.5, "started_at": "` + apiTS + `", "last_seq": 3}`,
}

// apiWith 把合法例最后的 } 换成 extra + }。
func apiWith(name, extra string) string {
	v := apiValid[name]
	return strings.TrimSuffix(v, "}") + extra + "}"
}

// apiReplace 把合法例里第一处 old 换成 repl。
func apiReplace(name, old, repl string) string {
	return strings.Replace(apiValid[name], old, repl, 1)
}

func TestAPISchema(t *testing.T) {
	t.Run("E0701-T4 api.schema.json", func(t *testing.T) {
		s := repoSet(t)
		ref := func(name string) string { return "api.schema.json#/$defs/" + name }
		for name, v := range apiValid {
			mustValid(t, s, ref(name), v)
		}
		mustValid(t, s, ref("BranchMeta"), `{"name": "main", "base_version": 0, "head": 0, "behind": 0, "ahead": 0, "created_at": "`+apiTS+`", "deleted_at": "`+apiTS+`"}`)
		mustValid(t, s, ref("RunMeta"), apiWith("RunMeta", `, "session_id": "`+apiID1+`", "replay_of": "`+apiID1+`", "fork_of": "`+apiID1+`", "fork_seq": 3, "ended_at": "`+apiTS+`"`))

		mustInvalid(t, s, ref("GraphMeta"), apiReplace("GraphMeta", `"title": "写作流水线", `, ""), "")
		mustInvalid(t, s, ref("GraphMeta"), apiWith("GraphMeta", `, "owner": "local"`), "")
		mustInvalid(t, s, ref("GraphMeta"), apiReplace("GraphMeta", `"created_at": "`+apiTS, `"created_at": "2026-11-24T08:00:00+08:00`), "/created_at")
		mustInvalid(t, s, ref("GraphMeta"), apiReplace("GraphMeta", `"nodes": 7`, `"nodes": 7, "edges": 3`), "/stats")

		mustInvalid(t, s, ref("BranchMeta"), apiReplace("BranchMeta", `"behind": 1`, `"behind": -1`), "/behind")
		mustInvalid(t, s, ref("BranchMeta"), apiReplace("BranchMeta", `"draft/a"`, `"feature/x"`), "/name")
		mustInvalid(t, s, ref("BranchMeta"), apiReplace("BranchMeta", `"head": 4, `, ""), "")

		mustInvalid(t, s, ref("OpRecord"), apiReplace("OpRecord", `, "undone": false`, ""), "")
		mustInvalid(t, s, ref("OpRecord"), apiReplace("OpRecord", `"version": 3`, `"version": 0`), "/version")
		mustInvalid(t, s, ref("OpRecord"), apiReplace("OpRecord", `"kind": "set_graph_title"`, `"kind": "rename"`), "/op")

		mustInvalid(t, s, ref("RunSteps"), `{"done": 1}`, "")
		mustInvalid(t, s, ref("RunSteps"), `{"done": 1, "total": 2, "failed": 0}`, "")

		mustInvalid(t, s, ref("RunMeta"), apiReplace("RunMeta", `"no": 1`, `"no": 0`), "/no")
		mustInvalid(t, s, ref("RunMeta"), apiReplace("RunMeta", `"kind": "run"`, `"kind": "batch"`), "/kind")
		mustInvalid(t, s, ref("RunMeta"), apiReplace("RunMeta", `"status": "running"`, `"status": "succeeded"`), "/status")
		mustInvalid(t, s, ref("RunMeta"), apiReplace("RunMeta", `"kind": "running_at"`, `"kind": "idle"`), "/headline/kind")
		mustInvalid(t, s, ref("RunMeta"), apiReplace("RunMeta", `, "last_seq": 17`, ""), "")

		mustInvalid(t, s, ref("NodeStateRow"), apiReplace("NodeStateRow", `"progress": 0.5`, `"progress": 1.5`), "/progress")
		mustInvalid(t, s, ref("NodeStateRow"), apiReplace("NodeStateRow", `"attempt": 1`, `"attempt": 0`), "/attempt")
		mustInvalid(t, s, ref("NodeStateRow"), apiReplace("NodeStateRow", `"exec_key": "x1", `, ""), "")
		mustInvalid(t, s, ref("NodeStateRow"), apiWith("NodeStateRow", `, "error": {"code": "timeout"}`), "/error")
	})
}
