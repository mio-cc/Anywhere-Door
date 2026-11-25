package graph

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mio-cc/anywhere-door/gen"
)

func examplePath(name string) string { return filepath.Join("..", "..", "schema", "examples", name) }

// loadDoc 读 schema/examples 下的图文档，返回解出的文档和原文。
func loadDoc(t testing.TB, name string) (*gen.Graph, []byte) {
	t.Helper()
	b, err := os.ReadFile(examplePath(name))
	if err != nil {
		t.Fatal(err)
	}
	var d gen.Graph
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatalf("%s：%v", name, err)
	}
	return &d, b
}

// sortedJSON 是按键排序的序列化（比较往返用）。
func sortedJSON(t testing.TB, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var x any
	if err := dec.Decode(&x); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(x)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func node(id, typ string) gen.Node { return gen.Node{ID: gen.Ulid(id), Type: gen.NodeTypeRef(typ)} }

func edge(id, from, fp, to, tp string) gen.Edge {
	return gen.Edge{ID: gen.Ulid(id), From: gen.PortRef{Node: gen.Ulid(from), Port: fp}, To: gen.PortRef{Node: gen.Ulid(to), Port: tp}}
}

func doc(nodes []gen.Node, edges []gen.Edge) *gen.Graph {
	return &gen.Graph{ID: "01JDTEST000000000000000001", Branch: "main", SchemaVersion: "1.0", Root: gen.Subgraph{Nodes: nodes, Edges: edges}}
}

// gen1000 是 1000 个节点、3 层嵌套的图：根图 10 个循环，每个里面 9 个节点加 1 个扇出，扇出里面 89 个节点；每层用链连起来。
func gen1000() *gen.Graph {
	n, e := 0, 0
	nid := func() gen.Ulid { n++; return gen.Ulid(fmt.Sprintf("N%05d", n)) }
	eid := func() gen.Ulid { e++; return gen.Ulid(fmt.Sprintf("E%05d", e)) }
	leaves := func(sg *gen.Subgraph, k int) {
		var prev gen.Ulid
		for i := 0; i < k; i++ {
			id := nid()
			sg.Nodes = append(sg.Nodes, gen.Node{ID: id, Type: "资源"})
			if prev != "" {
				sg.Edges = append(sg.Edges, gen.Edge{ID: eid(), From: gen.PortRef{Node: prev, Port: "ref"}, To: gen.PortRef{Node: id, Port: "in"}})
			}
			prev = id
		}
	}
	d := &gen.Graph{ID: "01JDPERF000000000000000001", Branch: "main", SchemaVersion: "1.0"}
	for i := 0; i < 10; i++ {
		inner := &gen.Subgraph{}
		leaves(inner, 89)
		mid := &gen.Subgraph{}
		leaves(mid, 9)
		mid.Nodes = append(mid.Nodes, gen.Node{ID: nid(), Type: "扇出", Children: inner})
		d.Root.Nodes = append(d.Root.Nodes, gen.Node{ID: nid(), Type: "循环", Children: mid})
	}
	return d
}

func TestFromDoc(t *testing.T) {
	t.Run("E0201-T1 往返无损", func(t *testing.T) {
		for _, name := range []string{"graph_minimal.json", "graph_loop.json", "graph_ui_unknown.json"} {
			d, raw := loadDoc(t, name)
			g, err := FromDoc(d)
			if err != nil {
				t.Fatalf("%s：%v", name, err)
			}
			if got, want := sortedJSON(t, g.Doc()), sortedJSON(t, json.RawMessage(raw)); got != want {
				t.Errorf("%s：Doc() 和原文不一致\n原文：%s\n导出：%s", name, want, got)
			}
		}
		d, _ := loadDoc(t, "graph_ui_unknown.json")
		g, err := FromDoc(d)
		if err != nil {
			t.Fatal(err)
		}
		const id = "01JDNVW0000000000000000001"
		n, ok := g.Node(id)
		if !ok || n.UI == nil || (*n.UI)["x_extra"] == nil || n.Params == nil || (*n.Params)["output_schema"] == nil {
			t.Fatalf("开放对象里的未知键要保留：%+v", n)
		}
		// 改输入不影响 Graph
		d.Root.Nodes[0].Pos.X = 9999
		(*d.Root.Nodes[0].UI)["color"] = json.RawMessage(`"c9"`)
		if n2, _ := g.Node(id); n2.Pos.X == 9999 || string((*n2.UI)["color"]) == `"c9"` {
			t.Error("改 FromDoc 的输入不应当影响 Graph")
		}
		// 改 Doc() 的结果不影响 Graph
		out := g.Doc()
		out.Root.Nodes[0].Pos.X = 1
		(*out.Root.Nodes[0].UI)["color"] = json.RawMessage(`"c8"`)
		if again := g.Doc(); again.Root.Nodes[0].Pos.X == 1 || string((*again.Root.Nodes[0].UI)["color"]) == `"c8"` {
			t.Error("改 Doc() 的结果不应当影响 Graph")
		}
		if g.ID() != "01JDGRAPHVW000000000000001" || g.Version() != 1 || g.SchemaVersion() != "1.0" {
			t.Errorf("ID / Version / SchemaVersion：%s %d %s", g.ID(), g.Version(), g.SchemaVersion())
		}
		if _, ok := g.Note("01JDNTVW000000000000000001"); !ok {
			t.Error("便签要能按 ID 查到")
		}
		if root, _ := g.Scope(RootScope); len(root.Notes()) != 1 || len(root.Nodes()) != 1 {
			t.Error("便签不算在 Nodes() 里")
		}
	})

	t.Run("E0201-T2 子图索引", func(t *testing.T) {
		d, _ := loadDoc(t, "graph_loop.json")
		g, err := FromDoc(d)
		if err != nil {
			t.Fatal(err)
		}
		root, ok := g.Scope(RootScope)
		if !ok || g.Depth(RootScope) != 0 || g.Parent(RootScope) != nil {
			t.Fatal("根图")
		}
		var loop *Node
		for _, n := range root.Nodes() {
			if n.Children != nil {
				loop = n
			}
			if g.ScopeOf(string(n.ID)) != root {
				t.Errorf("%s 应当在根图里", n.ID)
			}
		}
		if loop == nil {
			t.Fatal("graph_loop.json 的根图里应当有带子图的节点")
		}
		s1, ok := g.Scope(string(loop.ID))
		if !ok || g.Depth(s1.ID()) != 1 || g.Parent(s1.ID()) != loop {
			t.Fatalf("第 1 层子图：%v depth=%d", ok, g.Depth(string(loop.ID)))
		}
		inner := 0
		for _, n := range s1.Nodes() {
			if g.ScopeOf(string(n.ID)) != s1 {
				t.Errorf("%s 应当在第 1 层子图里", n.ID)
			}
			if n.Children == nil {
				continue
			}
			inner++
			s2, ok := g.Scope(string(n.ID))
			if !ok || g.Depth(s2.ID()) != 2 || g.Parent(s2.ID()) != n {
				t.Errorf("第 2 层子图 %s", n.ID)
				continue
			}
			for _, c := range s2.Nodes() {
				if g.ScopeOf(string(c.ID)) != s2 {
					t.Errorf("%s 应当在子图 %s 里", c.ID, n.ID)
				}
			}
		}
		scopes := g.Scopes()
		if inner == 0 || len(scopes) != 2+inner || scopes[0] != root || scopes[1] != s1 {
			t.Fatalf("Scopes 的顺序：根图在前，再按深度、ID；共 %d 个", len(scopes))
		}
		for i := 2; i+1 < len(scopes); i++ {
			if scopes[i].ID() > scopes[i+1].ID() {
				t.Error("同一深度按 ID 排序")
			}
		}
		for _, e := range root.Edges() {
			from, to := string(e.From.Node), string(e.To.Node)
			if !containsEdge(root.Out(from, e.From.Port), e) || !containsEdge(root.In(to, e.To.Port), e) ||
				!containsEdge(root.OutAll(from), e) || !containsEdge(root.InAll(to), e) {
				t.Errorf("连线 %s 的端口索引不对", e.ID)
			}
		}
		if g.ScopeOf("nope") != nil || g.Depth("nope") != -1 || g.Parent("nope") != nil {
			t.Error("不存在的 ID")
		}

		// ref 不展开
		ref := node("R1", "子图引用")
		ref.Ref = &gen.SubgraphRef{Name: "资料采集", Pin: "minor", Version: "1.0"}
		g2, err := FromDoc(doc([]gen.Node{ref}, nil))
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := g2.Scope("R1"); ok || len(g2.Scopes()) != 1 {
			t.Error("子图引用不在这里展开")
		}
	})

	t.Run("E0201-T3 重复 ID", func(t *testing.T) {
		nested := node("C", "循环")
		nested.Children = &gen.Subgraph{Nodes: []gen.Node{node("A", "资源")}}
		noted := doc([]gen.Node{node("A", "资源")}, []gen.Edge{edge("E", "A", "ref", "A", "in")})
		noted.Root.Notes = &[]gen.Note{{ID: "E", Text: "x"}}
		cases := []struct {
			name string
			d    *gen.Graph
			id   string
		}{
			{"节点与节点", doc([]gen.Node{node("A", "资源"), node("A", "资源")}, nil), "A"},
			{"连线与节点", doc([]gen.Node{node("X", "资源")}, []gen.Edge{edge("X", "X", "ref", "X", "in")}), "X"},
			{"便签与连线", noted, "E"},
			{"跨子图", doc([]gen.Node{node("A", "资源"), nested}, nil), "A"},
		}
		for _, c := range cases {
			_, err := FromDoc(c.d)
			if !errors.Is(err, ErrDuplicateID) || !strings.Contains(err.Error(), c.id) {
				t.Errorf("%s：应当是包着 ErrDuplicateID、写出 %s 的错误：%v", c.name, c.id, err)
			}
		}
		if _, err := FromDoc(nil); err == nil {
			t.Error("nil 文档应当报错")
		}
	})

	t.Run("E0201-T4 悬空连线", func(t *testing.T) {
		g, err := FromDoc(doc([]gen.Node{node("A", "资源")}, []gen.Edge{edge("E", "A", "ref", "Z", "in")}))
		if err != nil {
			t.Fatalf("悬空连线不应当让加载失败：%v", err)
		}
		root, _ := g.Scope(RootScope)
		if _, ok := g.Edge("E"); !ok || len(root.In("Z", "in")) != 1 || len(root.Out("A", "ref")) != 1 || len(root.InAll("Z")) != 1 {
			t.Error("悬空连线也要建索引")
		}
	})

	t.Run("E0201-T5 加载性能", func(t *testing.T) {
		d := gen1000()
		g, err := FromDoc(d)
		if err != nil {
			t.Fatal(err)
		}
		if len(g.nodes) != 1000 || len(g.Scopes()) != 21 || g.Depth(g.Scopes()[20].ID()) != 2 {
			t.Fatalf("测试图应当是 1000 个节点、3 层：%d 个节点、%d 个子图", len(g.nodes), len(g.Scopes()))
		}
		if os.Getenv("ANYDOOR_PERF") != "1" {
			t.Log("没有设 ANYDOOR_PERF=1：只检查能加载，不按 5ms 判（P.09 K238 的做法）")
			return
		}
		r := testing.Benchmark(func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, err := FromDoc(d); err != nil {
					b.Fatal(err)
				}
			}
		})
		if per := time.Duration(r.NsPerOp()); per > 5*time.Millisecond {
			t.Errorf("1000 节点加载用了 %v，超过 5ms", per)
		}
	})
}

func containsEdge(es []*Edge, e *Edge) bool {
	for _, x := range es {
		if x.ID == e.ID {
			return true
		}
	}
	return false
}

func BenchmarkFromDoc1000(b *testing.B) {
	d := gen1000()
	for i := 0; i < b.N; i++ {
		if _, err := FromDoc(d); err != nil {
			b.Fatal(err)
		}
	}
}
