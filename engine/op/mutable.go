package op

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"

	"github.com/mio-cc/anywhere-door/engine/graph"
	"github.com/mio-cc/anywhere-door/engine/schema"
	"github.com/mio-cc/anywhere-door/gen"
)

// mutable 是可以改的图文档（E.05-R5）：graph 只读，op 在 *gen.Graph 的副本上改，自带按 ID 找层的索引。
// 子图的 Children 是指针，节点在切片里移动不影响它，所以 scopes 里的 *gen.Subgraph 一直有效；节点、连线、便签的
// 指针只在下一次修改切片之前有效。
type mutable struct {
	doc    *gen.Graph
	scopes map[string]*gen.Subgraph // root 或容器节点 ID → 子图
	nodeAt map[string]string        // 节点 ID → 所在层
	edgeAt map[string]string
	noteAt map[string]string
}

func newMutable(d *gen.Graph) *mutable {
	m := &mutable{doc: d}
	m.reindex()
	return m
}

func (m *mutable) reindex() {
	m.scopes = map[string]*gen.Subgraph{graph.RootScope: &m.doc.Root}
	m.nodeAt, m.edgeAt, m.noteAt = map[string]string{}, map[string]string{}, map[string]string{}
	m.index(graph.RootScope, &m.doc.Root)
}

func (m *mutable) index(scope string, sg *gen.Subgraph) {
	for i := range sg.Nodes {
		n := &sg.Nodes[i]
		m.nodeAt[string(n.ID)] = scope
		if n.Children != nil {
			m.scopes[string(n.ID)] = n.Children
			m.index(string(n.ID), n.Children)
		}
	}
	for _, e := range sg.Edges {
		m.edgeAt[string(e.ID)] = scope
	}
	if sg.Notes != nil {
		for _, n := range *sg.Notes {
			m.noteAt[string(n.ID)] = scope
		}
	}
}

// used：ID 在整棵树里已经是节点、连线或便签（C.02-R1）。
func (m *mutable) used(id string) bool {
	_, a := m.nodeAt[id]
	_, b := m.edgeAt[id]
	_, c := m.noteAt[id]
	return a || b || c
}

func byID[T any](id func(T) gen.Ulid) func(a, b T) int {
	return func(a, b T) int { return strings.Compare(string(id(a)), string(id(b))) }
}

var (
	nodeCmp = byID(func(n gen.Node) gen.Ulid { return n.ID })
	edgeCmp = byID(func(e gen.Edge) gen.Ulid { return e.ID })
	noteCmp = byID(func(n gen.Note) gen.Ulid { return n.ID })
)

func find[T any](s []T, id string, idOf func(T) gen.Ulid) int {
	for i := range s {
		if string(idOf(s[i])) == id {
			return i
		}
	}
	return -1
}

func nodeID(n gen.Node) gen.Ulid { return n.ID }
func edgeID(e gen.Edge) gen.Ulid { return e.ID }
func noteID(n gen.Note) gen.Ulid { return n.ID }

// node 是节点和它所在的层；不存在时为 nil。
func (m *mutable) node(id string) (*gen.Node, string) {
	scope, ok := m.nodeAt[id]
	if !ok {
		return nil, ""
	}
	sg := m.scopes[scope]
	if i := find(sg.Nodes, id, nodeID); i >= 0 {
		return &sg.Nodes[i], scope
	}
	return nil, ""
}

func (m *mutable) edge(id string) (*gen.Edge, string) {
	scope, ok := m.edgeAt[id]
	if !ok {
		return nil, ""
	}
	sg := m.scopes[scope]
	if i := find(sg.Edges, id, edgeID); i >= 0 {
		return &sg.Edges[i], scope
	}
	return nil, ""
}

func (m *mutable) note(id string) (*gen.Note, string) {
	scope, ok := m.noteAt[id]
	if !ok {
		return nil, ""
	}
	sg := m.scopes[scope]
	if sg.Notes == nil {
		return nil, ""
	}
	if i := find(*sg.Notes, id, noteID); i >= 0 {
		return &(*sg.Notes)[i], scope
	}
	return nil, ""
}

// insertNode 按 ID 排序插入（节点先规范化）。
func (m *mutable) insertNode(scope string, n gen.Node) {
	sg := m.scopes[scope]
	i, _ := slices.BinarySearchFunc(sg.Nodes, n, nodeCmp)
	sg.Nodes = slices.Insert(sg.Nodes, i, n)
	m.reindex()
}

func (m *mutable) removeNode(id string) gen.Node {
	scope := m.nodeAt[id]
	sg := m.scopes[scope]
	i := find(sg.Nodes, id, nodeID)
	n := sg.Nodes[i]
	sg.Nodes = slices.Delete(sg.Nodes, i, i+1)
	m.reindex()
	return n
}

func (m *mutable) insertEdge(scope string, e gen.Edge) {
	sg := m.scopes[scope]
	i, _ := slices.BinarySearchFunc(sg.Edges, e, edgeCmp)
	sg.Edges = slices.Insert(sg.Edges, i, e)
	m.edgeAt[string(e.ID)] = scope
}

func (m *mutable) removeEdge(id string) gen.Edge {
	scope := m.edgeAt[id]
	sg := m.scopes[scope]
	i := find(sg.Edges, id, edgeID)
	e := sg.Edges[i]
	sg.Edges = slices.Delete(sg.Edges, i, i+1)
	delete(m.edgeAt, id)
	return e
}

func (m *mutable) insertNote(scope string, n gen.Note) {
	sg := m.scopes[scope]
	if sg.Notes == nil {
		sg.Notes = &[]gen.Note{}
	}
	i, _ := slices.BinarySearchFunc(*sg.Notes, n, noteCmp)
	*sg.Notes = slices.Insert(*sg.Notes, i, n)
	m.noteAt[string(n.ID)] = scope
}

func (m *mutable) removeNote(id string) gen.Note {
	scope := m.noteAt[id]
	sg := m.scopes[scope]
	i := find(*sg.Notes, id, noteID)
	n := (*sg.Notes)[i]
	*sg.Notes = slices.Delete(*sg.Notes, i, i+1)
	if len(*sg.Notes) == 0 {
		sg.Notes = nil
	}
	delete(m.noteAt, id)
	return n
}

// edgesInto 是同一层里进入某个端口的连线（按 ID 排序）。
func (m *mutable) edgesInto(scope, node, port string) []gen.Edge {
	var out []gen.Edge
	for _, e := range m.scopes[scope].Edges {
		if string(e.To.Node) == node && e.To.Port == port {
			out = append(out, e)
		}
	}
	return out
}

// edgesOf 是同一层里一端是这个节点的连线（按 ID 排序）。
func (m *mutable) edgesOf(scope, node string) []gen.Edge {
	var out []gen.Edge
	for _, e := range m.scopes[scope].Edges {
		if string(e.From.Node) == node || string(e.To.Node) == node {
			out = append(out, e)
		}
	}
	return out
}

// ports 是节点现在的输入端口（含动态端口，E.03.02 Ports）。类型没注册时为空。
func ports(n *gen.Node, v schema.View) map[string]*gen.PortDef {
	def, ok := v.NodeDef(string(n.Type))
	if !ok {
		return map[string]*gen.PortDef{}
	}
	in, _ := schema.Ports(&graph.Node{Node: *n}, def, v)
	return in
}

// ---- 规范化（E.05.01 §5.2、P.09 K262 ③）----

// normalizeDoc：每层的 nodes、edges、notes 按 ID 排序；promoted 排序；等于默认值的可选字段、等于 NodeDef 默认值的参数不写。
func normalizeDoc(d *gen.Graph, v schema.View) {
	if d.Title != nil && *d.Title == "" {
		d.Title = nil
	}
	if d.Meta != nil && len(*d.Meta) == 0 {
		d.Meta = nil
	}
	normSubgraph(&d.Root, v)
}

func normSubgraph(sg *gen.Subgraph, v schema.View) {
	if sg.Nodes == nil {
		sg.Nodes = []gen.Node{}
	}
	if sg.Edges == nil {
		sg.Edges = []gen.Edge{}
	}
	for i := range sg.Nodes {
		normNode(&sg.Nodes[i], v)
	}
	slices.SortStableFunc(sg.Nodes, nodeCmp)
	slices.SortStableFunc(sg.Edges, edgeCmp)
	if sg.Notes != nil {
		if len(*sg.Notes) == 0 {
			sg.Notes = nil
		} else {
			for i := range *sg.Notes {
				normNote(&(*sg.Notes)[i])
			}
			slices.SortStableFunc(*sg.Notes, noteCmp)
		}
	}
}

func normNode(n *gen.Node, v schema.View) {
	if n.Lock != nil && *n.Lock == "none" {
		n.Lock = nil
	}
	if n.Label != nil && *n.Label == "" {
		n.Label = nil
	}
	if n.Tags != nil && len(*n.Tags) == 0 {
		n.Tags = nil
	}
	if n.UI != nil && len(*n.UI) == 0 {
		n.UI = nil
	}
	if n.Promoted != nil {
		if len(*n.Promoted) == 0 {
			n.Promoted = nil
		} else {
			slices.Sort(*n.Promoted)
		}
	}
	stripDefaults(n, v)
	if n.Children != nil {
		normSubgraph(n.Children, v)
	}
}

func normNote(n *gen.Note) {
	if n.Color != nil && *n.Color == "none" {
		n.Color = nil
	}
}

// stripDefaults 删掉等于 NodeDef 默认值的参数（没有默认值时 null 当作没写）；类型没注册、定义里没有的参数原样保留。
func stripDefaults(n *gen.Node, v schema.View) {
	if n.Params == nil {
		return
	}
	def, ok := v.NodeDef(string(n.Type))
	if ok {
		for k, val := range *n.Params {
			if pd, ok := def.Params[k]; ok && isDefault(val, pd.Default) {
				delete(*n.Params, k)
			}
		}
	}
	if len(*n.Params) == 0 {
		n.Params = nil
	}
}

// isDefault：规范化 JSON 相同（def 为空时按 null 比）。
func isDefault(val, def json.RawMessage) bool {
	return canon(val) == canon(def)
}

// canon 是一段 JSON 的规范化写法；空的当作 null，不是合法 JSON 时原样返回。
func canon(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "null"
	}
	b, err := graph.Canonical(raw)
	if err != nil {
		return string(raw)
	}
	return string(b)
}

func isNull(raw json.RawMessage) bool { return canon(raw) == "null" }

// cloneDoc 深拷贝一份图文档。
func cloneDoc(d *gen.Graph) *gen.Graph {
	b, err := json.Marshal(d)
	if err != nil {
		panic("op: 图文档不能序列化：" + err.Error())
	}
	var out gen.Graph
	if err := json.Unmarshal(b, &out); err != nil {
		panic("op: 图文档不能反序列化：" + err.Error())
	}
	return &out
}

var ulidRe = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)

func validID(s string) bool { return ulidRe.MatchString(s) }

// treeEdges 是节点子图里（各层）的连线 ID。
func treeEdges(n *gen.Node) []string {
	if n.Children == nil {
		return nil
	}
	var out []string
	for _, e := range n.Children.Edges {
		out = append(out, string(e.ID))
	}
	for i := range n.Children.Nodes {
		out = append(out, treeEdges(&n.Children.Nodes[i])...)
	}
	return out
}

func mustRaw(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic("op: 序列化失败：" + err.Error())
	}
	return b
}

// mk 是一条服务器生成的 Op（逆操作、展开的子 Op）。
func mk(kind gen.OpKind, payload any) gen.Op { return gen.Op{Kind: kind, Payload: mustRaw(payload)} }

func ptr[T any](v T) *T { return &v }
