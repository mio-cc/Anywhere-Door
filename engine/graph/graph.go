// Package graph 把图文档加载成带索引的内存结构（设计文档 E.02）：按 ID、按端口、按所在子图查找，
// 子图内的拓扑分层与环检测，深拷贝、冻结视图和规范化哈希。
//
// 只依赖 gen，不含任何业务规则（E.02-R1）。本包只有读方法；图的修改只经过 engine/op（E.02-R2、INV-08）。
package graph

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mio-cc/anywhere-door/gen"
)

// RootScope 是根图的子图 ID。
const RootScope = "root"

// ErrDuplicateID：节点、连线、便签的 ID 在整棵树里重复（C.02-R1）。
var ErrDuplicateID = errors.New("graph: 图里有重复的 ID")

// Node 是图里的一个节点（只读）。
type Node struct{ gen.Node }

// Edge 是图里的一条连线（只读）。
type Edge struct{ gen.Edge }

// Note 是图里的一张便签（只读）。
type Note struct{ gen.Note }

// Graph 是带索引的内存图（E.02.01）。所有方法都只读；同一个 Graph 可以被多个 goroutine 同时读。
type Graph struct {
	raw     []byte // 规范化之前的文档（本包自己序列化的），Doc、Clone 从它重建
	doc     gen.Graph
	nodes   map[string]*Node
	edges   map[string]*Edge
	notes   map[string]*Note
	scopes  map[string]*Scope
	scopeOf map[string]string // 节点 ID → 所在子图 ID
	parent  map[string]string // 子图 ID → 容器节点 ID（根图为 ""）
	depth   map[string]int
	order   []*Scope
}

// FromDoc 深拷贝 d 后建立索引（E.02.01 §5）。只有 ID 重复时失败；悬空连线、跨子图连线照常加载，由校验报出（V05）。
func FromDoc(d *gen.Graph) (*Graph, error) {
	if d == nil {
		return nil, errors.New("graph: 图文档为空")
	}
	raw, err := encode(d)
	if err != nil {
		return nil, fmt.Errorf("graph: 图文档不能序列化：%w", err)
	}
	return build(raw)
}

func build(raw []byte) (*Graph, error) {
	g := &Graph{
		raw:     raw,
		nodes:   map[string]*Node{},
		edges:   map[string]*Edge{},
		notes:   map[string]*Note{},
		scopes:  map[string]*Scope{},
		scopeOf: map[string]string{},
		parent:  map[string]string{},
		depth:   map[string]int{},
	}
	if err := json.Unmarshal(raw, &g.doc); err != nil {
		return nil, fmt.Errorf("graph: 图文档不能解析：%w", err)
	}
	seen := map[string]string{}
	if err := g.addScope(RootScope, &g.doc.Root, "", 0, seen); err != nil {
		return nil, err
	}
	g.order = sortedScopes(g.scopes, g.depth)
	return g, nil
}

// Doc 导出为文档（深拷贝；数组保持原来的顺序）。
func (g *Graph) Doc() *gen.Graph {
	var d gen.Graph
	// raw 是本包用同一个类型序列化出来的，一定能解回去
	_ = json.Unmarshal(g.raw, &d)
	return &d
}

// ID 是图 ID。
func (g *Graph) ID() string { return string(g.doc.ID) }

// Version 是图的版本号。
func (g *Graph) Version() int64 { return g.doc.Version }

// SchemaVersion 是图文档格式的版本（C.12）。
func (g *Graph) SchemaVersion() string { return g.doc.SchemaVersion }

// Node 按 ID 在整棵树里找节点。
func (g *Graph) Node(id string) (*Node, bool) {
	n, ok := g.nodes[id]
	return n, ok
}

// Edge 按 ID 在整棵树里找连线。
func (g *Graph) Edge(id string) (*Edge, bool) {
	e, ok := g.edges[id]
	return e, ok
}

// Note 按 ID 在整棵树里找便签。
func (g *Graph) Note(id string) (*Note, bool) {
	n, ok := g.notes[id]
	return n, ok
}

// Scope 按 ID 找子图：RootScope 或容器节点的 ID。
func (g *Graph) Scope(id string) (*Scope, bool) {
	s, ok := g.scopes[id]
	return s, ok
}

// ScopeOf 是节点所在的子图；节点不存在时为 nil。
func (g *Graph) ScopeOf(nodeID string) *Scope {
	id, ok := g.scopeOf[nodeID]
	if !ok {
		return nil
	}
	return g.scopes[id]
}

// Parent 是子图的容器节点；根图、不存在的子图为 nil。
func (g *Graph) Parent(scopeID string) *Node {
	p := g.parent[scopeID]
	if p == "" {
		return nil
	}
	return g.nodes[p]
}

// Depth 是子图的嵌套深度：根图为 0（C.02.04 §6）；不存在的子图为 -1。
func (g *Graph) Depth(scopeID string) int {
	d, ok := g.depth[scopeID]
	if !ok {
		return -1
	}
	return d
}

// Scopes 是所有子图：根图在前，其余按深度、再按 ID 排序。
func (g *Graph) Scopes() []*Scope {
	return append([]*Scope(nil), g.order...)
}
