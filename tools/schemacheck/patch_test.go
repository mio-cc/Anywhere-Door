package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const (
	patchExample    = "patch/graph_patch.json"
	proposalExample = "patch/proposal.json"
	previewExample  = "patch/preview.json"
	patchRef        = "graph_patch.schema.json"
	proposalRef     = "graph_patch.schema.json#/$defs/Proposal"
	previewRef      = "graph_patch.schema.json#/$defs/ProposalPreview"
)

// exampleWith 读 schema/examples/ 下的示例 file（JSON 对象），用 edit 改一处后返回 JSON 文本。
func exampleWith(t *testing.T, file string, edit func(m map[string]any)) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, "schema", "examples", filepath.FromSlash(file)))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%s：%v", file, err)
	}
	edit(m)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// elem 取 m 里路径 path 上的对象或数组元素（测试里改示例用；路径写错时测试 panic，说明测试本身写错了）。
func elem(m map[string]any, path ...any) any {
	var cur any = m
	for _, p := range path {
		switch k := p.(type) {
		case string:
			cur = cur.(map[string]any)[k]
		case int:
			cur = cur.([]any)[k]
		}
	}
	return cur
}

type exampleCase struct {
	name string
	edit func(map[string]any)
	at   string
}

// mustInvalidCases：示例 file 按 cases 各改一处后，按 ref 校验都失败，且报错位置在 at 下。
func mustInvalidCases(t *testing.T, s *Set, ref, file string, cases []exampleCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { mustInvalid(t, s, ref, exampleWith(t, file, c.edit), c.at) })
	}
}

func TestPatch(t *testing.T) {
	t.Run("C08-T5 GraphPatch", func(t *testing.T) {
		s := repoSet(t)
		manifestEntry(t, s, patchExample)
		ops := func(n int) func(map[string]any) {
			return func(m map[string]any) {
				one := elem(m, "ops", 0)
				list := make([]any, n)
				for i := range list {
					list[i] = one
				}
				m["ops"] = list
			}
		}
		mustValid(t, s, patchRef, exampleWith(t, patchExample, ops(200)))
		mustInvalidCases(t, s, patchRef, patchExample, []exampleCase{
			{"ops 为空", ops(0), "/ops"},
			{"ops 超过 200 条", ops(201), "/ops"},
			{"PatchOp 缺 reason", func(m map[string]any) { delete(elem(m, "ops", 0).(map[string]any), "reason") }, "/ops/0"},
			{"kind 不在 OpKind", func(m map[string]any) { elem(m, "ops", 0).(map[string]any)["kind"] = "add" }, "/ops/0/kind"},
			{"多出 op_reasons", func(m map[string]any) { m["op_reasons"] = map[string]any{} }, ""},
			{"origin 不在枚举", func(m map[string]any) { m["origin"] = "chat" }, "/origin"},
		})
	})
	t.Run("C08-T6 提议与预览", func(t *testing.T) {
		s := repoSet(t)
		manifestEntry(t, s, proposalExample)
		manifestEntry(t, s, previewExample)
		mustInvalidCases(t, s, proposalRef, proposalExample, []exampleCase{
			{"status 为 pending", func(m map[string]any) { m["status"] = "pending" }, "/status"},
			{"decisions 的值为 maybe", func(m map[string]any) { m["decisions"] = map[string]any{"01JDPATCH0P00000000000001A": "maybe"} }, "/decisions/01JDPATCH0P00000000000001A"},
			// 键名不合法（propertyNames）：校验器把这类错误记在根位置（同 nodedef_test.go），位置写 ""；其余与示例相同，示例能通过
			{"decisions 的键不是 ULID", func(m map[string]any) { m["decisions"] = map[string]any{"tmp:1": "accepted"} }, ""},
		})
		mustInvalidCases(t, s, previewRef, previewExample, []exampleCase{
			{"ghost 为 move", func(m map[string]any) { elem(m, "marks", 0).(map[string]any)["ghost"] = "move" }, "/marks/0/ghost"},
			{"target 为空对象", func(m map[string]any) { elem(m, "marks", 0).(map[string]any)["target"] = map[string]any{} }, "/marks/0/target"},
		})
	})
}
