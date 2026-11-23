package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// 图文档的最小结构，只取结构检查要用的字段。
type gNode struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Label    string          `json:"label"`
	Params   map[string]any  `json:"params"`
	Children *gSubgraph      `json:"children"`
	Ref      json.RawMessage `json:"ref"`
}

type gEdge struct {
	ID   string            `json:"id"`
	From map[string]string `json:"from"`
	To   map[string]string `json:"to"`
}

type gSubgraph struct {
	Nodes []gNode `json:"nodes"`
	Edges []gEdge `json:"edges"`
}

type gGraph struct {
	Root gSubgraph `json:"root"`
}

func readGraph(t *testing.T, name string) gGraph {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, "schema", "examples", name))
	if err != nil {
		t.Fatal(err)
	}
	var g gGraph
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatalf("%s：%v", name, err)
	}
	return g
}

func TestGraphExamples(t *testing.T) {
	t.Run("C0201-T1 合法示例", func(t *testing.T) {
		s := repoSet(t)
		for _, f := range []string{"graph_minimal.json", "graph_loop.json"} {
			if e := manifestEntry(t, s, f); e.Schema != "graph.schema.json" || e.Valid == nil || !*e.Valid {
				t.Errorf("清单里 %s 应当按 graph.schema.json 校验、valid: true：%+v", f, e)
			}
		}
	})
	t.Run("C0201-T2 非法示例", func(t *testing.T) {
		s := repoSet(t)
		for f, at := range map[string]string{
			"invalid/graph_missing_root.json":     "",
			"invalid/graph_version_negative.json": "/version",
			"invalid/graph_branch_feature.json":   "/branch",
		} {
			if e := manifestEntry(t, s, f); e.Valid == nil || *e.Valid || e.ErrorAt == nil || *e.ErrorAt != at {
				t.Errorf("清单里 %s 应当 valid: false、error_at: %q：%+v", f, at, e)
			}
		}
		mustValid(t, s, "graph.schema.json", `{"id": "01JDGRAPH00000000000000001", "schema_version": "1.0", "version": 0, "branch": "draft/model-01jd", "root": {"nodes": [], "edges": []}}`)
		mustInvalid(t, s, "graph.schema.json", `{"id": "01JDGRAPH00000000000000001", "schema_version": "0.9", "version": 0, "branch": "main", "root": {"nodes": [], "edges": []}}`, "/schema_version")
		mustInvalid(t, s, "graph.schema.json", `{"id": "01JDGRAPH00000000000000001", "schema_version": "1.0", "version": 0, "branch": "main", "root": {"nodes": [], "edges": []}, "view": {}}`, "")
		mustInvalid(t, s, "graph.schema.json", `{"id": "01JDGRAPH00000000000000001", "schema_version": "1.0", "version": 0, "branch": "main", "root": {"nodes": [], "edges": []}, "packages": ["pack@1"]}`, "/packages/0")
	})
	t.Run("C0201-T3 闭环示例的结构", func(t *testing.T) {
		g := readGraph(t, "graph_loop.json")
		ids := map[string]bool{}
		maxDepth := 0
		found := map[string]int{} // 关键节点 → 所在层
		var walk func(sg *gSubgraph, depth int, path string)
		walk = func(sg *gSubgraph, depth int, path string) {
			maxDepth = max(maxDepth, depth)
			here := map[string]bool{}
			roles := map[string]bool{}
			for _, n := range sg.Nodes {
				if ids[n.ID] {
					t.Errorf("%s：节点 ID %s 重复（C.02-R1：全树唯一）", path, n.ID)
				}
				ids[n.ID] = true
				here[n.ID] = true
				if n.Type == "子图端口" {
					if r, _ := n.Params["role"].(string); r != "" {
						roles[r] = true
					}
				}
				if n.Label == "闭环迭代" || n.Label == "资料采集" {
					found[n.Label] = depth
				}
			}
			if depth > 0 && (!roles["entry"] || !roles["exit"]) {
				t.Errorf("%s：子图要有 entry 和 exit 子图端口节点（C.02.04-R2），实际 %v", path, sortedKeys(roles))
			}
			for _, e := range sg.Edges {
				if ids[e.ID] {
					t.Errorf("%s：连线 ID %s 重复", path, e.ID)
				}
				ids[e.ID] = true
				if !here[e.From["node"]] || !here[e.To["node"]] {
					t.Errorf("%s：连线 %s 的两端不在同一个子图里（C.02-R2）", path, e.ID)
				}
			}
			for i := range sg.Nodes {
				n := &sg.Nodes[i]
				if n.Children != nil {
					walk(n.Children, depth+1, path+"/"+n.Label)
				}
			}
		}
		walk(&g.Root, 0, "根图")
		if maxDepth != 2 {
			t.Errorf("应当有三层（层号 0–2），实际最深第 %d 层", maxDepth)
		}
		if d, ok := found["闭环迭代"]; !ok || d != 0 {
			t.Errorf("闭环迭代应在根图（第 0 层）：%v", found)
		}
		if d, ok := found["资料采集"]; !ok || d != 1 {
			t.Errorf("资料采集应在闭环迭代里（第 1 层的节点）：%v", found)
		}
	})
}
