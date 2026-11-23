package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 四种声明式写法的示例（C.03.04 §3），登记在 schema/examples/manifest.yaml。
var implExamples = map[string]string{
	"nodedefs/翻译.json":    "prompt",
	"nodedefs/汇率查询.json":  "http",
	"nodedefs/查询工单.json":  "mcp",
	"nodedefs/生成与评审.json": "subgraph",
}

const implRef = "nodedef.schema.json#/$defs/Impl"

// readJSONExample 读 schema/examples/ 下的一个 JSON 文件（file 相对 schema/examples/）。
func readJSONExample(t *testing.T, file string, v any) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, "schema", "examples", filepath.FromSlash(file)))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("%s：%v", file, err)
	}
}

func TestImpl(t *testing.T) {
	t.Run("C0304-T1 四种写法", func(t *testing.T) {
		s := repoSet(t)
		for file, typ := range implExamples {
			e := manifestEntry(t, s, file) // 按清单校验，应当通过
			if e.Schema != "nodedef.schema.json" || e.Valid == nil || !*e.Valid {
				t.Errorf("清单里 %s 应按 nodedef.schema.json 校验并通过：%+v", file, e)
			}
			var doc struct {
				Kind string `json:"kind"`
				Impl struct {
					Type string `json:"type"`
				} `json:"impl"`
			}
			readJSONExample(t, file, &doc)
			if doc.Kind != "extension" || doc.Impl.Type != typ {
				t.Errorf("%s：kind=%s、impl.type=%s，应为 extension、%s", file, doc.Kind, doc.Impl.Type, typ)
			}
		}
		// 按 type 判别：每种只认自己的字段（子结构按 §3）
		mustValid(t, s, implRef, `{"type": "builtin"}`)
		mustValid(t, s, implRef, `{"type": "builtin", "entry": "fx.convert"}`)
		mustValid(t, s, implRef, `{"type": "prompt", "output_schema": "方案@1"}`)
		mustValid(t, s, implRef, `{"type": "prompt", "rolecard": "验收者@latest", "output_schema": "判定@1"}`)
		mustValid(t, s, implRef, `{"type": "http", "risk": "L2", "method": "POST", "url": "https://api.example.com/x", "body": {"a": 1}}`)
		mustValid(t, s, implRef, `{"type": "mcp", "server": "jira", "tool": "search_issues"}`)
		mustValid(t, s, implRef, `{"type": "subgraph", "ref": {"name": "生成-评审", "version": "1.0.0"}}`)
		mustInvalid(t, s, implRef, `{"type": "prompt", "output_schema": "方案@1", "method": "GET"}`, "")
		mustInvalid(t, s, implRef, `{"type": "builtin", "server": "jira"}`, "")
		mustInvalid(t, s, implRef, `{"type": "mcp", "server": "jira", "tool": "t", "ref": {"name": "x", "version": "1.0.0"}}`, "")
		mustInvalid(t, s, implRef, `{"type": "script", "entry": "x.py"}`, "/type") // K17：脚本节点取消
		mustInvalid(t, s, implRef, `{"server": "jira"}`, "")
		mustInvalid(t, s, implRef, `"builtin"`, "")
	})
	t.Run("C0304-T2 Impl 反例", func(t *testing.T) {
		s := repoSet(t)
		const http = `"type": "http", "method": "GET", "url": "https://api.example.com/x"`
		mustInvalid(t, s, implRef, `{`+http+`}`, "")                   // http 缺 risk（R5）
		mustValid(t, s, implRef, `{`+http+`, "risk": "L0"}`)           // GET 可以是 L0
		for _, m := range []string{"POST", "PUT", "PATCH", "DELETE"} { // R5：会改动外部数据的方法最低 L2
			h := strings.Replace(http, `"GET"`, `"`+m+`"`, 1)
			mustInvalid(t, s, implRef, `{`+h+`, "risk": "L0"}`, "/risk")
			mustInvalid(t, s, implRef, `{`+h+`, "risk": "L1"}`, "/risk")
			mustValid(t, s, implRef, `{`+h+`, "risk": "L2"}`)
		}
		mustInvalid(t, s, implRef, `{`+http+`, "risk": "L4"}`, "/risk")
		mustInvalid(t, s, implRef, `{`+http+`, "risk": "L0", "timeout_ms": 1.5}`, "/timeout_ms")
		mustInvalid(t, s, implRef, `{"type": "mcp", "tool": "search_issues", "risk": "L0"}`, "") // mcp 缺 server
		mustInvalid(t, s, implRef, `{"type": "subgraph", "expose": {}}`, "")                     // subgraph 缺 ref
		mustInvalid(t, s, implRef, `{"type": "subgraph", "ref": {"name": "生成-评审", "version": "1.0"}}`, "/ref/version")
		mustInvalid(t, s, implRef, `{"type": "subgraph", "ref": {"name": "x", "version": "1.0.0"}, "expose": {"rounds": {"node": "x", "param": "max_rounds"}}}`, "/expose/rounds/node")
		mustInvalid(t, s, implRef, `{"type": "prompt", "rolecard": "验收者"}`, "") // 缺 output_schema，角色卡没有版本
	})
}
