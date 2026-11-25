package graph

// Walk 深度优先遍历整棵树（E.02.02 §5）：按子图的 Nodes() 顺序访问节点，有 children 时先进入子图、再访问下一个兄弟。
// path 是从根到当前节点所在子图的容器节点链（根图里的节点为空）；fn 返回 false 时整个遍历停止。
func Walk(g *Graph, fn func(n *Node, scope *Scope, path []*Node) bool) {
	root, ok := g.Scope(RootScope)
	if !ok {
		return
	}
	walkScope(g, root, nil, fn)
}

func walkScope(g *Graph, s *Scope, path []*Node, fn func(n *Node, scope *Scope, path []*Node) bool) bool {
	for _, n := range s.nodes {
		if !fn(n, s, append([]*Node(nil), path...)) {
			return false
		}
		if n.Children == nil {
			continue
		}
		child, ok := g.Scope(string(n.ID))
		if !ok {
			continue
		}
		if !walkScope(g, child, append(append([]*Node(nil), path...), n), fn) {
			return false
		}
	}
	return true
}
