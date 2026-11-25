package graph

import (
	"encoding/json"

	"github.com/mio-cc/anywhere-door/gen"
)

// Clone 是完全独立的深拷贝（含索引，E.02.03 §5）。
func (g *Graph) Clone() *Graph {
	c, err := build(g.raw)
	if err != nil {
		// raw 是本包序列化出来、并且已经成功建过一次索引的，不会走到这里
		return &Graph{raw: g.raw, nodes: map[string]*Node{}, edges: map[string]*Edge{}, notes: map[string]*Note{},
			scopes: map[string]*Scope{}, scopeOf: map[string]string{}, parent: map[string]string{}, depth: map[string]int{}}
	}
	return c
}

// Freeze 是冻结视图：只读，可以被多个 goroutine 同时读（运行快照、回放、并发读取用，INV-12）。
func (g *Graph) Freeze() *Frozen { return &Frozen{g: g.Clone()} }

// CloneSubgraph 是这个子图的深拷贝（打包、运行时展开用）。
func (s *Scope) CloneSubgraph() *gen.Subgraph {
	var out gen.Subgraph
	if b, err := encode(s.sg); err == nil {
		_ = json.Unmarshal(b, &out)
	}
	return &out
}

// Frozen 是不可变的图：方法和 Graph 的只读方法相同。
type Frozen struct{ g *Graph }

// Doc 导出为文档（深拷贝）。
func (f *Frozen) Doc() *gen.Graph { return f.g.Doc() }

// ID 是图 ID。
func (f *Frozen) ID() string { return f.g.ID() }

// Version 是图的版本号。
func (f *Frozen) Version() int64 { return f.g.Version() }

// SchemaVersion 是图文档格式的版本。
func (f *Frozen) SchemaVersion() string { return f.g.SchemaVersion() }

// Node 按 ID 找节点。
func (f *Frozen) Node(id string) (*Node, bool) { return f.g.Node(id) }

// Edge 按 ID 找连线。
func (f *Frozen) Edge(id string) (*Edge, bool) { return f.g.Edge(id) }

// Note 按 ID 找便签。
func (f *Frozen) Note(id string) (*Note, bool) { return f.g.Note(id) }

// Scope 按 ID 找子图。
func (f *Frozen) Scope(id string) (*Scope, bool) { return f.g.Scope(id) }

// ScopeOf 是节点所在的子图。
func (f *Frozen) ScopeOf(nodeID string) *Scope { return f.g.ScopeOf(nodeID) }

// Parent 是子图的容器节点。
func (f *Frozen) Parent(scopeID string) *Node { return f.g.Parent(scopeID) }

// Depth 是子图的嵌套深度。
func (f *Frozen) Depth(scopeID string) int { return f.g.Depth(scopeID) }

// Scopes 是所有子图。
func (f *Frozen) Scopes() []*Scope { return f.g.Scopes() }

// Graph 是冻结视图背后的图（只读方法同样可以并发调用）；Walk 等接受 *Graph 的函数用它。
func (f *Frozen) Graph() *Graph { return f.g }
