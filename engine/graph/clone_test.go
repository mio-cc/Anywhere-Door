package graph

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/mio-cc/anywhere-door/gen"
)

// firstWithParams 找一个有 params 的节点。
func firstWithParams(t testing.TB, g *Graph) string {
	t.Helper()
	var id string
	Walk(g, func(n *Node, _ *Scope, _ []*Node) bool {
		if n.Params != nil && len(*n.Params) > 0 {
			id = string(n.ID)
			return false
		}
		return true
	})
	if id == "" {
		t.Fatal("示例图里没有带 params 的节点")
	}
	return id
}

func TestClone(t *testing.T) {
	t.Run("E0203-T1 副本独立", func(t *testing.T) {
		d, _ := loadDoc(t, "graph_loop.json")
		g := mustGraph(t, d)
		before := Hash(g.Doc())
		if before == "" || !strings.HasPrefix(before, "sha256:") || Hash(d) != before {
			t.Fatalf("哈希：%q；输入文档的哈希应当相同：%q", before, Hash(d))
		}
		id := firstWithParams(t, g)

		c := g.Clone()
		if Hash(c.Doc()) != before || len(c.Scopes()) != len(g.Scopes()) {
			t.Fatal("Clone 的内容应当和原图相同")
		}
		n, _ := c.Node(id)
		(*n.Params)["x_clone"] = json.RawMessage(`1`)
		n.Pos.X = 424242
		if o, _ := g.Node(id); (*o.Params)["x_clone"] != nil || o.Pos.X == 424242 || Hash(g.Doc()) != before {
			t.Error("改 Clone 的节点不应当影响原图")
		}

		f := g.Freeze()
		fn, _ := f.Node(id)
		(*fn.Params)["x_frozen"] = json.RawMessage(`2`)
		if o, _ := g.Node(id); (*o.Params)["x_frozen"] != nil || Hash(g.Doc()) != before || Hash(f.Doc()) != before {
			t.Error("改冻结视图拿到的节点不应当影响原图；冻结视图的文档不变")
		}

		root, _ := g.Scope(RootScope)
		sg := root.CloneSubgraph()
		if got, want := sortedJSON(t, sg), sortedJSON(t, g.Doc().Root); got != want {
			t.Errorf("CloneSubgraph 的内容应当和根图相同")
		}
		sg.Nodes[0].Pos.X = 515151
		if Hash(g.Doc()) != before {
			t.Error("改 CloneSubgraph 的结果不应当影响原图")
		}

		// 改原图的输入文档再重新加载：原来的副本不变
		d2 := g.Doc()
		d2.Root.Nodes[0].Pos.X = 7
		g2 := mustGraph(t, d2)
		if Hash(g2.Doc()) == before || Hash(c.Doc()) != before {
			t.Error("重新加载的图哈希应当变了，之前的副本不变")
		}
	})

	t.Run("E0203-T2 并发读", func(t *testing.T) {
		// 开发机没有 C 编译器，跑不了 -race（要 cgo）：这里同时读并比较结果，-race 在有 cgo 的机器上照样适用
		d, _ := loadDoc(t, "graph_loop.json")
		f := mustGraph(t, d).Freeze()
		want := Hash(f.Doc())
		var ids []string
		Walk(f.Graph(), func(n *Node, _ *Scope, _ []*Node) bool { ids = append(ids, string(n.ID)); return true })
		var wg sync.WaitGroup
		errs := make(chan string, 100)
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				id := ids[i%len(ids)]
				if _, ok := f.Node(id); !ok {
					errs <- "Node " + id
				}
				s := f.ScopeOf(id)
				if s == nil || !s.Has(id) {
					errs <- "ScopeOf " + id
					return
				}
				_ = s.Nodes()
				_ = s.InAll(id)
				_ = s.OutAll(id)
				_, _ = s.TopoLevels()
				_ = f.Depth(s.ID())
				_ = f.Parent(s.ID())
				_ = f.Scopes()
				if Hash(f.Doc()) != want {
					errs <- "Hash"
				}
			}(i)
		}
		wg.Wait()
		close(errs)
		for e := range errs {
			t.Error(e)
		}
	})

	t.Run("E0203-T3 哈希向量", func(t *testing.T) {
		b, err := os.ReadFile(examplePath("hash_vectors.json"))
		if err != nil {
			t.Fatal(err)
		}
		var file struct {
			Vectors []struct {
				Name      string          `json:"name"`
				Doc       json.RawMessage `json:"doc"`
				Canonical string          `json:"canonical"`
				Hash      string          `json:"hash"`
			} `json:"vectors"`
		}
		if err := json.Unmarshal(b, &file); err != nil {
			t.Fatal(err)
		}
		if len(file.Vectors) < 7 {
			t.Fatalf("向量太少：%d", len(file.Vectors))
		}
		for _, v := range file.Vectors {
			got, err := Canonical(v.Doc)
			if err != nil {
				t.Errorf("%s：%v", v.Name, err)
				continue
			}
			if string(got) != v.Canonical {
				t.Errorf("%s：规范化结果不同\n想要 %s\n实际 %s", v.Name, v.Canonical, got)
			}
			if h := HashBytes(got); h != v.Hash {
				t.Errorf("%s：哈希不同：想要 %s，实际 %s", v.Name, v.Hash, h)
			}
			if strings.HasPrefix(v.Name, "graph_") {
				var d gen.Graph
				if err := json.Unmarshal(v.Doc, &d); err != nil {
					t.Fatalf("%s：%v", v.Name, err)
				}
				if h := Hash(&d); h != v.Hash {
					t.Errorf("%s：Hash(gen.Graph) 不同：想要 %s，实际 %s", v.Name, v.Hash, h)
				}
			}
		}
		if _, err := Canonical(json.RawMessage(`{"x":1e400}`)); err == nil {
			t.Error("超出 float64 的数字应当报错")
		}
		for in, want := range map[string]string{"1.0": "1", "-0": "0", "1e21": "1e+21", "1.5e-7": "1.5e-7", "0.000001": "0.000001", "1e-7": "1e-7", "123e-2": "1.23"} {
			if got, err := jsNumber(in); err != nil || got != want {
				t.Errorf("jsNumber(%s) = %q, %v；想要 %q", in, got, err, want)
			}
		}
	})
}
