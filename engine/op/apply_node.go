package op

import (
	"encoding/json"

	"github.com/mio-cc/anywhere-door/engine/schema"
	"github.com/mio-cc/anywhere-door/gen"
)

// 节点的增删（E.05.01 §5.2）。
func init() {
	appliers[gen.OpAddNode] = applyAddNode
	appliers[gen.OpRemoveNodes] = applyRemoveNodes
	appliers[gen.OpRestoreNodes] = applyRestoreNodes
}

var noTouch schema.Touched

// container 是节点定义的容器规则（required / one_of；空表示不能带子图，C.02.04）。
func container(def *gen.NodeDef) string {
	if def == nil || def.Container == nil {
		return ""
	}
	return *def.Container
}

// checkNewTree：要加进来的节点（连同子图）ID 都是没用过的 ULID、类型已注册、不是容器的不带子图。
func (m *mutable) checkNewTree(n *gen.Node, v schema.View, seen map[string]bool) error {
	claim := func(id, what string) error {
		if !validID(id) {
			return invalidf("%s ID %q 不是 ULID", what, id)
		}
		if m.used(id) || seen[id] {
			return invalidf("%s ID %s 已被占用", what, id)
		}
		seen[id] = true
		return nil
	}
	if err := claim(string(n.ID), "节点"); err != nil {
		return err
	}
	def, ok := v.NodeDef(string(n.Type))
	if !ok {
		return invalidf("节点类型 %s 没有注册", n.Type)
	}
	if container(def) == "" && (n.Children != nil || n.Ref != nil) {
		return invalidf("%s 节点不能带子图", n.Type)
	}
	if n.Children == nil {
		return nil
	}
	for i := range n.Children.Nodes {
		if err := m.checkNewTree(&n.Children.Nodes[i], v, seen); err != nil {
			return err
		}
	}
	for _, e := range n.Children.Edges {
		if err := claim(string(e.ID), "连线"); err != nil {
			return err
		}
	}
	if n.Children.Notes != nil {
		for _, nt := range *n.Children.Notes {
			if err := claim(string(nt.ID), "便签"); err != nil {
				return err
			}
		}
	}
	return nil
}

// autoPorts 是容器节点没带子图时放的一对子图端口节点（C.05.02 §2 add_node）：入口在左、出口在右。
func autoPorts(n gen.Node, e *env) *gen.Subgraph {
	port := func(role string, dx int64) gen.Node {
		params := map[string]json.RawMessage{"role": mustRaw(role)}
		return gen.Node{ID: gen.Ulid(e.newID()), Type: gen.NodeTypeRef(gen.NodeTypeSubPort),
			Pos: gen.Pos{X: n.Pos.X + dx, Y: n.Pos.Y}, Params: &params}
	}
	return &gen.Subgraph{Nodes: []gen.Node{port("entry", -240), port("exit", 240)}, Edges: []gen.Edge{}}
}

func applyAddNode(m *mutable, op *gen.Op, e *env) ([]gen.Op, schema.Touched, error) {
	p, err := payloadOf[gen.PayloadAddNode](op)
	if err != nil {
		return nil, noTouch, err
	}
	if _, ok := m.scopes[p.Scope]; !ok {
		return nil, noTouch, invalidf("层 %s 不存在", p.Scope)
	}
	n := p.Node
	if err := m.checkNewTree(&n, e.view, map[string]bool{}); err != nil {
		return nil, noTouch, err
	}
	def, _ := e.view.NodeDef(string(n.Type))
	if c := container(def); (c == "required" || c == "one_of") && n.Children == nil && n.Ref == nil && !e.replay {
		n.Children = autoPorts(n, e)
		p.Node = n
		op.Payload = mustRaw(p) // 服务器补全的写进日志（E.05-R7）
	}
	normNode(&n, e.view)
	m.insertNode(p.Scope, n)
	return []gen.Op{mk(gen.OpRemoveNodes, gen.PayloadRemoveNodes{IDs: []gen.Ulid{n.ID}})},
		schema.Touched{Nodes: nodeTree(&n), Edges: treeEdges(&n)}, nil
}

func applyRemoveNodes(m *mutable, op *gen.Op, _ *env) ([]gen.Op, schema.Touched, error) {
	p, err := payloadOf[gen.PayloadRemoveNodes](op)
	if err != nil {
		return nil, noTouch, err
	}
	if len(p.IDs) == 0 {
		return nil, noTouch, invalidf("没有写要删的节点")
	}
	for _, id := range p.IDs {
		if n, _ := m.node(string(id)); n == nil {
			return nil, noTouch, invalidf("节点 %s 不存在", id)
		}
	}
	removed := gen.Removed{Nodes: []gen.RemovedNodesItem{}, Edges: []gen.RemovedEdgesItem{}}
	var t schema.Touched
	for _, id := range p.IDs {
		n, scope := m.node(string(id))
		if n == nil {
			continue // 已随前面删掉的容器一起删了
		}
		for _, ed := range m.edgesOf(scope, string(id)) {
			m.removeEdge(string(ed.ID))
			removed.Edges = append(removed.Edges, gen.RemovedEdgesItem{Scope: scope, Edge: ed})
			t.Edges = append(t.Edges, string(ed.ID))
		}
		node := m.removeNode(string(id))
		removed.Nodes = append(removed.Nodes, gen.RemovedNodesItem{Scope: scope, Node: node})
		t.Nodes = append(t.Nodes, nodeTree(&node)...)
		t.Edges = append(t.Edges, treeEdges(&node)...)
	}
	return []gen.Op{mk(gen.OpRestoreNodes, gen.PayloadRestoreNodes{Removed: removed})}, t, nil
}

// applyRestoreNodes 按删除的相反顺序插回节点（先删子节点、后删容器时，容器要先回来），再插回连线。
func applyRestoreNodes(m *mutable, op *gen.Op, e *env) ([]gen.Op, schema.Touched, error) {
	p, err := payloadOf[gen.PayloadRestoreNodes](op)
	if err != nil {
		return nil, noTouch, err
	}
	rm := p.Removed
	if len(rm.Nodes) == 0 {
		return nil, noTouch, invalidf("没有要恢复的节点")
	}
	seen := map[string]bool{}
	avail := map[string]bool{}
	for s := range m.scopes {
		avail[s] = true
	}
	for i := len(rm.Nodes) - 1; i >= 0; i-- {
		it := rm.Nodes[i]
		if !avail[it.Scope] {
			return nil, noTouch, invalidf("层 %s 不存在", it.Scope)
		}
		if err := m.checkNewTree(&it.Node, e.view, seen); err != nil {
			return nil, noTouch, err
		}
		markScopes(&it.Node, avail)
	}
	for _, it := range rm.Edges {
		if !avail[it.Scope] {
			return nil, noTouch, invalidf("层 %s 不存在", it.Scope)
		}
		id := string(it.Edge.ID)
		if !validID(id) || m.used(id) || seen[id] {
			return nil, noTouch, invalidf("连线 ID %s 已被占用", id)
		}
		seen[id] = true
	}
	var t schema.Touched
	ids := make([]gen.Ulid, 0, len(rm.Nodes))
	for i := len(rm.Nodes) - 1; i >= 0; i-- {
		n := rm.Nodes[i].Node
		normNode(&n, e.view)
		m.insertNode(rm.Nodes[i].Scope, n)
		t.Nodes = append(t.Nodes, nodeTree(&n)...)
		t.Edges = append(t.Edges, treeEdges(&n)...)
	}
	for _, it := range rm.Nodes {
		ids = append(ids, it.Node.ID)
	}
	for _, it := range rm.Edges {
		m.insertEdge(it.Scope, it.Edge)
		t.Edges = append(t.Edges, string(it.Edge.ID))
	}
	return []gen.Op{mk(gen.OpRemoveNodes, gen.PayloadRemoveNodes{IDs: ids})}, t, nil
}

func markScopes(n *gen.Node, avail map[string]bool) {
	if n.Children == nil {
		return
	}
	avail[string(n.ID)] = true
	for i := range n.Children.Nodes {
		markScopes(&n.Children.Nodes[i], avail)
	}
}
