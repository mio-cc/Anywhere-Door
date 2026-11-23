package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const (
	sessionExample   = "session/session.json"
	assistantExample = "session/message_assistant.json"
	userExample      = "session/message_user.json"
	sessionRef       = "session.schema.json#/$defs/Session"
	messageRef       = "session.schema.json#/$defs/Message"
)

// kindsOf 是示例 file 里 list 数组每一项的 kind。
func kindsOf(t *testing.T, file, list string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, "schema", "examples", filepath.FromSlash(file)))
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("%s：%v", file, err)
	}
	var items []struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(raw[list], &items); err != nil {
		t.Fatalf("%s 的 %s：%v", file, list, err)
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Kind)
	}
	return out
}

func TestSession(t *testing.T) {
	t.Run("C10-T1 示例", func(t *testing.T) {
		s := repoSet(t)
		for _, f := range []string{sessionExample, assistantExample, userExample} {
			manifestEntry(t, s, f)
		}
		if got, want := kindsOf(t, assistantExample, "blocks"), []string{"graph", "proposal", "applied", "approval", "run", "focus", "error"}; !sameSet(got, want) || len(got) != len(want) {
			t.Errorf("助手消息的卡片块是 %v，应当 7 种各一个：%v", got, want)
		}
		if got, want := kindsOf(t, userExample, "refs"), []string{"node", "file", "session", "skill"}; !sameSet(got, want) || len(got) != len(want) {
			t.Errorf("用户消息的引用是 %v，应当 4 种各一个：%v", got, want)
		}
		var sess struct {
			GraphIDs           []string          `json:"graph_ids"`
			LastLocation       map[string]string `json:"last_location"`
			OrchestratorEffort map[string]string `json:"orchestrator_effort"`
			Mandates           []struct {
				Anchor string `json:"anchor"`
			} `json:"mandates"`
		}
		b, err := os.ReadFile(filepath.Join(repoRoot, "schema", "examples", filepath.FromSlash(sessionExample)))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, &sess); err != nil {
			t.Fatal(err)
		}
		if len(sess.GraphIDs) != 2 || len(sess.LastLocation) == 0 || len(sess.OrchestratorEffort) == 0 || len(sess.Mandates) != 1 || sess.Mandates[0].Anchor != "session" {
			t.Errorf("会话示例应当挂 2 张图，带 last_location、orchestrator_effort 和一个 session 挂载点：%+v", sess)
		}
	})
	t.Run("C10-T3 反例", func(t *testing.T) {
		s := repoSet(t)
		obj := func(m map[string]any, path ...any) map[string]any { return elem(m, path...).(map[string]any) }
		mustInvalidCases(t, s, messageRef, assistantExample, []exampleCase{
			{"卡片块 kind 为 chart", func(m map[string]any) { obj(m, "blocks", 0)["kind"] = "chart" }, "/blocks/0/kind"},
			{"proposal 块缺 counts", func(m map[string]any) { delete(obj(m, "blocks", 1), "counts") }, "/blocks/1"},
			{"Message.error 多出字段", func(m map[string]any) {
				m["status"] = "failed"
				m["error"] = map[string]any{"code": "budget_exceeded", "message": "本轮预算用完了", "stack": "…"}
			}, "/error"},
		})
		mustInvalidCases(t, s, messageRef, userExample, []exampleCase{
			{"file 引用同时有 ref 和 path", func(m map[string]any) { obj(m, "refs", 1)["path"] = "docs/需求.md" }, "/refs/1"},
			{"file 引用两个都没有", func(m map[string]any) { delete(obj(m, "refs", 1), "ref") }, "/refs/1"},
			{"refs 21 个", func(m map[string]any) {
				one := elem(m, "refs", 0)
				list := make([]any, 21)
				for i := range list {
					list[i] = one
				}
				m["refs"] = list
			}, "/refs"},
			{"selection.node_ids 51 个", func(m map[string]any) {
				one := elem(m, "selection", "node_ids", 0)
				list := make([]any, 51)
				for i := range list {
					list[i] = one
				}
				obj(m, "selection")["node_ids"] = list
			}, "/selection/node_ids"},
		})
		mustInvalidCases(t, s, sessionRef, sessionExample, []exampleCase{
			{"Session 多出字段", func(m map[string]any) { m["owner"] = "local" }, ""},
			{"orchestrator_effort 的值为 extreme", func(m map[string]any) { obj(m, "orchestrator_effort")["large-model"] = "extreme" }, "/orchestrator_effort/large-model"},
			{"last_location.scope 写成名称路径", func(m map[string]any) { obj(m, "last_location")["scope"] = "主图/检索" }, "/last_location/scope"},
			{"会话挂载点的 anchor 为 graph", func(m map[string]any) { obj(m, "mandates", 0)["anchor"] = "graph" }, "/mandates/0/anchor"},
			{"会话挂载点的 anchor 为节点", func(m map[string]any) { obj(m, "mandates", 0)["anchor"] = "node:01JD8X3Q5V7W9Y2Z4A6B8C0D1E" }, "/mandates/0/anchor"},
		})
	})
}
