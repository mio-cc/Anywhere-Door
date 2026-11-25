package graph

import (
	"math/rand"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/mio-cc/anywhere-door/gen"
)

func mustGraph(t testing.TB, d *gen.Graph) *Graph {
	t.Helper()
	g, err := FromDoc(d)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func rootOf(t testing.TB, g *Graph) *Scope {
	t.Helper()
	s, ok := g.Scope(RootScope)
	if !ok {
		t.Fatal("没有根图")
	}
	return s
}

func nodes(ids ...string) []gen.Node {
	out := make([]gen.Node, len(ids))
	for i, id := range ids {
		out[i] = node(id, "资源")
	}
	return out
}

func TestTopo(t *testing.T) {
	t.Run("E0202-T1 分层", func(t *testing.T) {
		// 菱形 A→B、A→C、B→D、C→D；链 E→F；单独的 G；一条指向不存在节点的连线不算
		d := doc(nodes("A", "B", "C", "D", "E", "F", "G"), []gen.Edge{
			edge("e1", "A", "o", "B", "i"), edge("e2", "A", "o", "C", "i"), edge("e3", "B", "o", "D", "i"),
			edge("e4", "C", "o", "D", "i"), edge("e5", "E", "o", "F", "i"), edge("e6", "F", "o", "Z", "i"),
			edge("e7", "A", "o", "B", "j"),
		})
		levels, err := rootOf(t, mustGraph(t, d)).TopoLevels()
		want := [][]string{{"A", "E", "G"}, {"B", "C", "F"}, {"D"}}
		if err != nil || !reflect.DeepEqual(levels, want) {
			t.Errorf("想要 %v，实际 %v %v", want, levels, err)
		}
		s := rootOf(t, mustGraph(t, d))
		if got := s.Upstream("D"); !reflect.DeepEqual(got, []string{"A", "B", "C"}) {
			t.Errorf("Upstream(D)：%v", got)
		}
		if got := s.Downstream("A"); !reflect.DeepEqual(got, []string{"B", "C", "D"}) {
			t.Errorf("Downstream(A)：%v", got)
		}
	})

	t.Run("E0202-T2 环检测", func(t *testing.T) {
		d := doc(nodes("A", "B", "C", "D", "E", "F", "G"), []gen.Edge{
			edge("e1", "A", "o", "B", "i"), edge("e2", "B", "o", "A", "i"), // 环 1
			edge("e3", "C", "o", "D", "i"), edge("e4", "D", "o", "E", "i"), edge("e5", "E", "o", "C", "i"), // 环 2
			edge("e6", "F", "o", "F", "i"), // 自环
			edge("e7", "E", "o", "G", "i"),
		})
		s := rootOf(t, mustGraph(t, d))
		want := [][]string{{"A", "B"}, {"C", "D", "E"}, {"F"}}
		if got := s.SCCs(); !reflect.DeepEqual(got, want) {
			t.Errorf("SCCs：想要 %v，实际 %v", want, got)
		}
		_, err := s.TopoLevels()
		ec, ok := err.(*ErrCycle)
		if !ok || !reflect.DeepEqual(ec.SCCs, want) || ec.Error() == "" {
			t.Errorf("TopoLevels 应当返回 *ErrCycle：%v", err)
		}
		if got := rootOf(t, mustGraph(t, doc(nodes("A", "B"), []gen.Edge{edge("e", "A", "o", "B", "i")}))).SCCs(); len(got) != 0 {
			t.Errorf("没有环时 SCCs 为空：%v", got)
		}
	})

	t.Run("E0202-T3 Dominated 共享上游", func(t *testing.T) {
		// A→B→N；C→B；S→N、S→M（S 是共享上游）；X→C
		d := doc(nodes("A", "B", "C", "M", "N", "S", "X"), []gen.Edge{
			edge("e1", "A", "o", "B", "i"), edge("e2", "B", "o", "N", "i"), edge("e3", "C", "o", "B", "i"),
			edge("e4", "S", "o", "N", "i"), edge("e5", "S", "o", "M", "i"), edge("e6", "X", "o", "C", "i"),
		})
		s := rootOf(t, mustGraph(t, d))
		if got := s.Dominated("N"); !reflect.DeepEqual(got, []string{"A", "B", "C", "X"}) {
			t.Errorf("Dominated(N)：%v（S 也通向 M，不应当在里面）", got)
		}
		if got := s.Dominated("M"); len(got) != 0 {
			t.Errorf("Dominated(M)：%v", got)
		}
	})

	t.Run("E0202-T4 确定性", func(t *testing.T) {
		base := doc(nodes("A", "B", "C", "D", "E", "F"), []gen.Edge{
			edge("e1", "A", "o", "B", "i"), edge("e2", "A", "o", "C", "i"), edge("e3", "B", "o", "D", "i"),
			edge("e4", "C", "o", "D", "i"), edge("e5", "E", "o", "F", "i"), edge("e6", "F", "o", "E", "i"),
		})
		inner := node("L", "循环")
		inner.Children = &gen.Subgraph{Nodes: nodes("P", "Q"), Edges: []gen.Edge{edge("e7", "P", "o", "Q", "i")}}
		base.Root.Nodes = append(base.Root.Nodes, inner)
		type result struct {
			Levels [][]string
			Err    string
			SCCs   [][]string
			Up     []string
			Walk   []string
		}
		run := func(d *gen.Graph) result {
			g := mustGraph(t, d)
			s := rootOf(t, g)
			var r result
			lv, err := s.TopoLevels()
			r.Levels = lv
			if err != nil {
				r.Err = err.Error()
			}
			r.SCCs, r.Up = s.SCCs(), s.Upstream("D")
			Walk(g, func(n *Node, _ *Scope, path []*Node) bool {
				p := ""
				for _, x := range path {
					p += string(x.ID) + "/"
				}
				r.Walk = append(r.Walk, p+string(n.ID))
				return true
			})
			return r
		}
		want := run(base)
		if !reflect.DeepEqual(want.Walk, []string{"A", "B", "C", "D", "E", "F", "L", "L/P", "L/Q"}) {
			t.Errorf("Walk 的顺序：%v", want.Walk)
		}
		rng := rand.New(rand.NewSource(1))
		for i := 0; i < 100; i++ {
			d := mustGraph(t, base).Doc()
			rng.Shuffle(len(d.Root.Nodes), func(a, b int) { d.Root.Nodes[a], d.Root.Nodes[b] = d.Root.Nodes[b], d.Root.Nodes[a] })
			rng.Shuffle(len(d.Root.Edges), func(a, b int) { d.Root.Edges[a], d.Root.Edges[b] = d.Root.Edges[b], d.Root.Edges[a] })
			if got := run(d); !reflect.DeepEqual(got, want) {
				t.Fatalf("第 %d 次打乱后结果不同：\n%+v\n%+v", i, got, want)
			}
		}
		// fn 返回 false 时停止
		count := 0
		Walk(mustGraph(t, base), func(*Node, *Scope, []*Node) bool { count++; return count < 3 })
		if count != 3 {
			t.Errorf("返回 false 后应当停止：访问了 %d 个", count)
		}
	})

	t.Run("E0202-T5 分层性能", func(t *testing.T) {
		g := mustGraph(t, flat1000())
		levels, err := rootOf(t, g).TopoLevels()
		if err != nil || len(levels) == 0 {
			t.Fatalf("%v", err)
		}
		if os.Getenv("ANYDOOR_PERF") != "1" {
			t.Log("没有设 ANYDOOR_PERF=1：只检查能分层，不按 2ms 判（P.09 K238 的做法）")
			return
		}
		s := rootOf(t, g)
		r := testing.Benchmark(func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, err := s.TopoLevels(); err != nil {
					b.Fatal(err)
				}
			}
		})
		if per := time.Duration(r.NsPerOp()); per > 2*time.Millisecond {
			t.Errorf("1000 节点分层用了 %v，超过 2ms", per)
		}
	})
}

// flat1000 是一个子图里 1000 个节点：每个节点连到后面 1、2 个节点（有分叉和汇合，没有环）。
func flat1000() *gen.Graph {
	var ns []gen.Node
	var es []gen.Edge
	id := func(i int) string { return "N" + pad(i) }
	for i := 0; i < 1000; i++ {
		ns = append(ns, node(id(i), "资源"))
		for _, j := range []int{i + 1, i + 2} {
			if j < 1000 {
				es = append(es, edge("E"+pad(i)+pad(j), id(i), "o", id(j), "i"))
			}
		}
	}
	return doc(ns, es)
}

func pad(i int) string {
	s := "0000" + itoa(i)
	return s[len(s)-4:]
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for ; i > 0; i /= 10 {
		b = append([]byte{byte('0' + i%10)}, b...)
	}
	return string(b)
}

func BenchmarkTopoLevels1000(b *testing.B) {
	g, err := FromDoc(flat1000())
	if err != nil {
		b.Fatal(err)
	}
	s, _ := g.Scope(RootScope)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.TopoLevels(); err != nil {
			b.Fatal(err)
		}
	}
}
