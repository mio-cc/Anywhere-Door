package op

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/mio-cc/anywhere-door/engine/graph"
	"github.com/mio-cc/anywhere-door/engine/schema"
	"github.com/mio-cc/anywhere-door/gen"
)

// Key 是写集合、引用集合里的一项（E.05.03 §4）：
//
//	node:<id>  edge:<id>  note:<id>  port:<node>/<port>  param:<node>/<key>[/<属性名>]
//	field:<node>/<label|tags|lock>  view:<node>  graph:title  graph:meta  graph:doc
type Key string

// keyDoc 是整份文档（migrate）：和任何键都算相交（E.05.03 §5.3）。
const keyDoc Key = "graph:doc"

func nodeKey(id string) Key { return Key("node:" + id) }

func edgeKey(id string) Key { return Key("edge:" + id) }

func noteKey(id string) Key { return Key("note:" + id) }

func viewKey(id string) Key { return Key("view:" + id) }

func portKey(node, port string) Key { return Key("port:" + node + "/" + port) }

func paramKey(node, key string) Key { return Key("param:" + node + "/" + key) }

func fieldKey(node, field string) Key { return Key("field:" + node + "/" + field) }

// Conflict 是一处冲突：这一批里的哪条 Op 和错过的 Op 在哪个键上相交（写进 GraphConflictDetails.conflicts）。
type Conflict struct {
	OpID   string
	Object Key
}

// WriteSet 是这条 Op 的写集合（E.05.03 §5.2），排序、去重。没有 inverse 的是新提交的 Op，一并删除的东西按 g 算；
// 已落库的 Op（有 inverse）按 payload 和 inverse 算。payload 解不开时按整份文档（graph:doc）算，和谁都冲突。
func WriteSet(op *gen.Op, g *graph.Graph, v schema.View) []Key {
	return sortedKeys(collect(op, g, v).w)
}

// RefSet 是这条 Op 的引用集合：它用到、但不改的对象（另一条 Op 删了它就不能交换）。
func RefSet(op *gen.Op, g *graph.Graph) []Key {
	return sortedKeys(collect(op, g, nil).r)
}

// Commutes 判断这一批 Op 能不能和错过的 Op 交换（C.05.04 §2.1、E.05.03 §5.3）。g 是 head 的图（已应用了 missed）。
// 不能交换时 Conflicts 每对一条（同一条 Op、同一个键只写一次），按 op_id、object 排序。
func Commutes(ops []gen.Op, g *graph.Graph, v schema.View, missed []gen.OpRecord) (bool, []Conflict) {
	ms := make([]*keySet, len(missed))
	for i := range missed {
		ms[i] = collect(&missed[i].Op, g, v)
	}
	seen := map[Conflict]bool{}
	var out []Conflict
	for i := range ops {
		a := collect(&ops[i], g, v)
		id := ""
		if ops[i].ID != nil {
			id = string(*ops[i].ID)
		}
		for j := range ms {
			for _, k := range clash(ops[i].Kind, a, missed[j].Op.Kind, ms[j]) {
				c := Conflict{OpID: id, Object: k}
				if !seen[c] {
					seen[c] = true
					out = append(out, c)
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].OpID != out[j].OpID {
			return out[i].OpID < out[j].OpID
		}
		return out[i].Object < out[j].Object
	})
	return len(out) == 0, out
}

// clash 是两条 Op 相交的键：W∩W、W∩R、R∩W（R∩R 不算）。例外（E.05.03 §5.3）：两条都只改视图时 view: 键不算（后写者胜，
// C.02.05-R3）；两条 remove_nodes 删同一个节点幂等，node:、edge: 键不算。
func clash(ka gen.OpKind, a *keySet, kb gen.OpKind, b *keySet) []Key {
	if (a.w[keyDoc] && !b.empty()) || (b.w[keyDoc] && !a.empty()) {
		return []Key{keyDoc}
	}
	views := isView(ka) && isView(kb)
	removes := ka == gen.OpRemoveNodes && kb == gen.OpRemoveNodes
	var out []Key
	for k := range a.w {
		if b.w[k] {
			s := string(k)
			switch {
			case views && strings.HasPrefix(s, "view:"):
			case removes && (strings.HasPrefix(s, "node:") || strings.HasPrefix(s, "edge:")):
			default:
				out = append(out, k)
				continue
			}
		}
		if b.r[k] {
			out = append(out, k)
		}
	}
	for k := range a.r {
		if b.w[k] {
			out = append(out, k)
		}
	}
	return out
}

func isView(k gen.OpKind) bool { return k == gen.OpMoveNodes || k == gen.OpSetNodeUI }

type keySet struct{ w, r map[Key]bool }

func (s *keySet) empty() bool { return len(s.w)+len(s.r) == 0 }

func sortedKeys(m map[Key]bool) []Key {
	out := make([]Key, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// collect 算一条 Op 的写集合和引用集合。没有 inverse 的是新提交的 Op：一并删除的东西（子孙、相连的连线、
// 被替换的连线、消失的动态端口上的连线）按 g 算；已落库的不再按 g 算，改为并进 inverse 的写集合（被删的都在 inverse 里）。
func collect(op *gen.Op, g *graph.Graph, v schema.View) *keySet {
	s := &keySet{w: map[Key]bool{}, r: map[Key]bool{}}
	s.add(op, g, v, op.Inverse == nil, true)
	inv := inverseOps(op)
	for i := range inv {
		s.add(&inv[i], g, v, false, false)
	}
	return s
}

func (s *keySet) add(op *gen.Op, g *graph.Graph, v schema.View, live, refs bool) {
	ref := func(k Key) {
		if refs {
			s.r[k] = true
		}
	}
	container := func(scope string) {
		if scope != "" && scope != graph.RootScope {
			ref(nodeKey(scope))
		}
	}
	bad := func() { s.w[keyDoc] = true }
	switch op.Kind {
	case gen.OpAddNode:
		p, err := decode[gen.PayloadAddNode](op)
		if err != nil {
			bad()
			return
		}
		s.tree(&p.Node)
		container(p.Scope)
	case gen.OpRemoveNodes:
		p, err := decode[gen.PayloadRemoveNodes](op)
		if err != nil {
			bad()
			return
		}
		for _, id := range p.IDs {
			s.w[nodeKey(string(id))] = true
			if live {
				s.removedLive(g, string(id))
			}
		}
	case gen.OpRestoreNodes:
		p, err := decode[gen.PayloadRestoreNodes](op)
		if err != nil {
			bad()
			return
		}
		for i := range p.Removed.Nodes {
			s.tree(&p.Removed.Nodes[i].Node)
			container(p.Removed.Nodes[i].Scope)
		}
		for _, e := range p.Removed.Edges {
			s.w[edgeKey(string(e.Edge.ID))] = true
		}
	case gen.OpConnect:
		p, err := decode[gen.PayloadConnect](op)
		if err != nil {
			bad()
			return
		}
		e := p.Edge
		s.w[edgeKey(string(e.ID))] = true
		if exclusive(g, v, e.To) {
			s.w[portKey(string(e.To.Node), e.To.Port)] = true
			if live {
				for _, old := range inEdges(g, string(e.To.Node), e.To.Port) {
					s.w[edgeKey(old)] = true
				}
			}
		}
		ref(nodeKey(string(e.From.Node)))
		ref(nodeKey(string(e.To.Node)))
	case gen.OpDisconnect:
		p, err := decode[gen.PayloadDisconnect](op)
		if err != nil {
			bad()
			return
		}
		for _, id := range p.IDs {
			s.w[edgeKey(string(id))] = true
		}
	case gen.OpSetParam:
		p, err := decode[gen.PayloadSetParam](op)
		if err != nil {
			bad()
			return
		}
		node := string(p.Node)
		k := paramKey(node, p.Key)
		if p.Path != nil && len(*p.Path) > 0 {
			k = Key(string(k) + "/" + (*p.Path)[0])
		} else if live {
			for _, id := range vanishedEdges(g, v, node, p.Key, p.To) {
				s.w[edgeKey(id)] = true
			}
		}
		s.w[k] = true
		ref(nodeKey(node))
	case gen.OpPromoteParam, gen.OpDemoteParam:
		// 两种 payload 字段相同（{node, key}，C.05.02 §2）。
		p, err := decode[gen.PayloadPromoteParam](op)
		if err != nil {
			bad()
			return
		}
		node := string(p.Node)
		s.w[paramKey(node, p.Key)] = true
		s.w[portKey(node, p.Key)] = true
		if live && op.Kind == gen.OpDemoteParam {
			for _, id := range inEdges(g, node, p.Key) {
				s.w[edgeKey(id)] = true
			}
		}
		ref(nodeKey(node))
	case gen.OpSetLabel, gen.OpSetTags, gen.OpSetLock:
		node, err := nodeOf(op)
		if err != nil {
			bad()
			return
		}
		f := map[gen.OpKind]string{gen.OpSetLabel: "label", gen.OpSetTags: "tags", gen.OpSetLock: "lock"}[op.Kind]
		s.w[fieldKey(node, f)] = true
		ref(nodeKey(node))
	case gen.OpMoveNodes:
		p, err := decode[gen.PayloadMoveNodes](op)
		if err != nil {
			bad()
			return
		}
		for _, m := range p.Moves {
			s.w[viewKey(string(m.ID))] = true
			ref(nodeKey(string(m.ID)))
		}
	case gen.OpSetNodeUI:
		node, err := nodeOf(op)
		if err != nil {
			bad()
			return
		}
		s.w[viewKey(node)] = true
		ref(nodeKey(node))
	case gen.OpAddNote:
		p, err := decode[gen.PayloadAddNote](op)
		if err != nil {
			bad()
			return
		}
		s.w[noteKey(string(p.Note.ID))] = true
		container(p.Scope)
	case gen.OpUpdateNote:
		p, err := decode[gen.PayloadUpdateNote](op)
		if err != nil {
			bad()
			return
		}
		s.w[noteKey(string(p.ID))] = true
	case gen.OpRemoveNotes:
		p, err := decode[gen.PayloadRemoveNotes](op)
		if err != nil {
			bad()
			return
		}
		for _, id := range p.IDs {
			s.w[noteKey(string(id))] = true
		}
	case gen.OpSetGraphTitle:
		s.w["graph:title"] = true
	case gen.OpSetGraphMeta:
		s.w["graph:meta"] = true
	case gen.OpRevertTo:
		p, err := decode[gen.PayloadRevertTo](op)
		if err != nil {
			bad()
			return
		}
		if p.Ops != nil {
			for i := range *p.Ops {
				s.add(&(*p.Ops)[i], g, v, live, refs)
			}
		}
	default:
		// migrate 和 M1 还不收的 kind（E.05-R6）：整份文档。
		s.w[keyDoc] = true
	}
}

// tree 把节点、它子图里的节点、连线、便签都记进写集合。
func (s *keySet) tree(n *gen.Node) {
	s.w[nodeKey(string(n.ID))] = true
	if n.Children == nil {
		return
	}
	for i := range n.Children.Nodes {
		s.tree(&n.Children.Nodes[i])
	}
	for _, e := range n.Children.Edges {
		s.w[edgeKey(string(e.ID))] = true
	}
	if n.Children.Notes != nil {
		for _, nt := range *n.Children.Notes {
			s.w[noteKey(string(nt.ID))] = true
		}
	}
}

// removedLive 是删一个节点时一并删掉的东西（按 g）：子图里的全部内容、所在层里相连的连线。
func (s *keySet) removedLive(g *graph.Graph, id string) {
	if g == nil {
		return
	}
	n, ok := g.Node(id)
	if !ok {
		return
	}
	s.tree(&n.Node)
	if sc := g.ScopeOf(id); sc != nil {
		for _, e := range sc.InAll(id) {
			s.w[edgeKey(string(e.ID))] = true
		}
		for _, e := range sc.OutAll(id) {
			s.w[edgeKey(string(e.ID))] = true
		}
	}
}

// exclusive：这个输入端口只能接一条线（不是多连）。节点在 g 里找不到时为 false（只算 edge 键，E.05.03 §4）；
// 类型或端口不认识时按只能接一条算（宁可多报冲突）。
func exclusive(g *graph.Graph, v schema.View, to gen.PortRef) bool {
	if g == nil {
		return false
	}
	n, ok := g.Node(string(to.Node))
	if !ok {
		return false
	}
	if v == nil {
		return true
	}
	def, ok := v.NodeDef(string(n.Type))
	if !ok {
		return true
	}
	in, _ := schema.Ports(n, def, v)
	pd, ok := in[to.Port]
	return !ok || pd.Multi == nil || !*pd.Multi
}

// inEdges 是进入某个端口的连线 ID。
func inEdges(g *graph.Graph, node, port string) []string {
	if g == nil {
		return nil
	}
	sc := g.ScopeOf(node)
	if sc == nil {
		return nil
	}
	var out []string
	for _, e := range sc.In(node, port) {
		out = append(out, string(e.ID))
	}
	return out
}

// vanishedEdges 是把参数 key 改成 to 之后，消失的动态端口上的连线（C.03.05-R4、C.05.02 §2 set_param），按 ID 排序。
// E.05.01 的 set_param 应用函数也用它。
func vanishedEdges(g *graph.Graph, v schema.View, node, key string, to json.RawMessage) []string {
	if g == nil || v == nil {
		return nil
	}
	n, ok := g.Node(node)
	if !ok {
		return nil
	}
	def, ok := v.NodeDef(string(n.Type))
	if !ok {
		return nil
	}
	before, _ := schema.Ports(n, def, v)
	m := n.Node
	params := map[string]json.RawMessage{}
	if m.Params != nil {
		for k, x := range *m.Params {
			params[k] = x
		}
	}
	params[key] = to
	m.Params = &params
	after, _ := schema.Ports(&graph.Node{Node: m}, def, v)
	var out []string
	for name := range before {
		if _, ok := after[name]; !ok {
			out = append(out, inEdges(g, node, name)...)
		}
	}
	sort.Strings(out)
	return out
}
