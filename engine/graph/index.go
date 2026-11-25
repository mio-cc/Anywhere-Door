package graph

import (
	"fmt"
	"sort"

	"github.com/mio-cc/anywhere-door/gen"
)

type portKey struct{ node, port string }

// Scope 是一个子图（根图或某个容器节点的 children），带按端口的连线索引。
type Scope struct {
	id      string
	sg      *gen.Subgraph
	nodes   []*Node
	edges   []*Edge
	notes   []*Note
	nodeSet map[string]bool
	in, out map[portKey][]*Edge
	inAll   map[string][]*Edge
	outAll  map[string][]*Edge
}

// addScope 建立一个子图的索引，再进入它的容器节点。seen 记录已经用过的 ID 和位置（三种对象共用一个命名空间）。
func (g *Graph) addScope(id string, sg *gen.Subgraph, parent string, depth int, seen map[string]string) error {
	s := &Scope{
		id: id, sg: sg, nodeSet: map[string]bool{},
		in: map[portKey][]*Edge{}, out: map[portKey][]*Edge{},
		inAll: map[string][]*Edge{}, outAll: map[string][]*Edge{},
	}
	g.scopes[id], g.parent[id], g.depth[id] = s, parent, depth
	claim := func(oid, what string) error {
		where := what + "（子图 " + id + "）"
		if prev, ok := seen[oid]; ok {
			return fmt.Errorf("%w：%s 同时是%s和%s", ErrDuplicateID, oid, prev, where)
		}
		seen[oid] = where
		return nil
	}
	for i := range sg.Nodes {
		n := &Node{Node: sg.Nodes[i]}
		if err := claim(string(n.ID), "节点"); err != nil {
			return err
		}
		g.nodes[string(n.ID)] = n
		g.scopeOf[string(n.ID)] = id
		s.nodes = append(s.nodes, n)
		s.nodeSet[string(n.ID)] = true
	}
	for i := range sg.Edges {
		e := &Edge{Edge: sg.Edges[i]}
		if err := claim(string(e.ID), "连线"); err != nil {
			return err
		}
		g.edges[string(e.ID)] = e
		s.edges = append(s.edges, e)
	}
	if sg.Notes != nil {
		for i := range *sg.Notes {
			n := &Note{Note: (*sg.Notes)[i]}
			if err := claim(string(n.ID), "便签"); err != nil {
				return err
			}
			g.notes[string(n.ID)] = n
			s.notes = append(s.notes, n)
		}
	}
	sort.Slice(s.nodes, func(i, j int) bool { return s.nodes[i].ID < s.nodes[j].ID })
	sort.Slice(s.edges, func(i, j int) bool { return s.edges[i].ID < s.edges[j].ID })
	sort.Slice(s.notes, func(i, j int) bool { return s.notes[i].ID < s.notes[j].ID })
	for _, e := range s.edges {
		from, to := string(e.From.Node), string(e.To.Node)
		s.out[portKey{from, e.From.Port}] = append(s.out[portKey{from, e.From.Port}], e)
		s.in[portKey{to, e.To.Port}] = append(s.in[portKey{to, e.To.Port}], e)
		s.outAll[from] = append(s.outAll[from], e)
		s.inAll[to] = append(s.inAll[to], e)
	}
	g.scopes[id] = s
	for _, n := range s.nodes {
		if n.Children != nil {
			if err := g.addScope(string(n.ID), n.Children, string(n.ID), depth+1, seen); err != nil {
				return err
			}
		}
	}
	return nil
}

func sortedScopes(scopes map[string]*Scope, depth map[string]int) []*Scope {
	out := make([]*Scope, 0, len(scopes))
	for _, s := range scopes {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		di, dj := depth[out[i].id], depth[out[j].id]
		if di != dj {
			return di < dj
		}
		return out[i].id < out[j].id
	})
	return out
}

// ID 是子图 ID：RootScope 或容器节点的 ID。
func (s *Scope) ID() string { return s.id }

// Nodes 是子图里的节点，按 ID 排序。
func (s *Scope) Nodes() []*Node { return append([]*Node(nil), s.nodes...) }

// Edges 是子图里声明的连线，按 ID 排序。
func (s *Scope) Edges() []*Edge { return append([]*Edge(nil), s.edges...) }

// Notes 是子图里的便签，按 ID 排序。
func (s *Scope) Notes() []*Note { return append([]*Note(nil), s.notes...) }

// Has 判断节点是不是在这个子图里（不含更深的子图）。
func (s *Scope) Has(nodeID string) bool { return s.nodeSet[nodeID] }

// In 是进入某个输入端口的连线（按 ID 排序）。
func (s *Scope) In(nodeID, port string) []*Edge {
	return append([]*Edge(nil), s.in[portKey{nodeID, port}]...)
}

// Out 是从某个输出端口出去的连线（按 ID 排序）。
func (s *Scope) Out(nodeID, port string) []*Edge {
	return append([]*Edge(nil), s.out[portKey{nodeID, port}]...)
}

// InAll 是进入某个节点的所有连线。
func (s *Scope) InAll(nodeID string) []*Edge { return append([]*Edge(nil), s.inAll[nodeID]...) }

// OutAll 是从某个节点出去的所有连线。
func (s *Scope) OutAll(nodeID string) []*Edge { return append([]*Edge(nil), s.outAll[nodeID]...) }
