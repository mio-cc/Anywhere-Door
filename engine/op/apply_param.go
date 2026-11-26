package op

import (
	"encoding/json"
	"slices"
	"sort"

	"github.com/mio-cc/anywhere-door/engine/schema"
	"github.com/mio-cc/anywhere-door/gen"
)

// 参数（E.05.01 §5.2）。值合不合法交给校验（V03）。
func init() {
	appliers[gen.OpSetParam] = applySetParam
	appliers[gen.OpPromoteParam] = applyPromoteParam
	appliers[gen.OpDemoteParam] = applyDemoteParam
}

func paramOf(n *gen.Node, key string) (json.RawMessage, bool) {
	if n.Params == nil {
		return nil, false
	}
	v, ok := (*n.Params)[key]
	return v, ok
}

// setParam 写参数；等于默认值（没有默认值时为 null）就删键（P.09 K262 ③）。
func setParam(n *gen.Node, key string, val, def json.RawMessage) {
	if isDefault(val, def) {
		if n.Params != nil {
			delete(*n.Params, key)
			if len(*n.Params) == 0 {
				n.Params = nil
			}
		}
		return
	}
	if n.Params == nil {
		n.Params = &map[string]json.RawMessage{}
	}
	(*n.Params)[key] = append(json.RawMessage{}, val...)
}

func applySetParam(m *mutable, op *gen.Op, e *env) ([]gen.Op, schema.Touched, error) {
	p, err := payloadOf[gen.PayloadSetParam](op)
	if err != nil {
		return nil, noTouch, err
	}
	n, scope := m.node(string(p.Node))
	if n == nil {
		return nil, noTouch, invalidf("节点 %s 不存在", p.Node)
	}
	def, ok := e.view.NodeDef(string(n.Type))
	if !ok {
		return nil, noTouch, invalidf("节点类型 %s 没有注册", n.Type)
	}
	pd, ok := def.Params[p.Key]
	if !ok {
		return nil, noTouch, invalidf("%s 没有参数 %s", n.Type, p.Key)
	}
	if pd.Lock != nil && (*pd.Lock == "def" || *pd.Lock == "kernel") {
		return nil, noTouch, invalidf("参数 %s 已锁定（%s，C.03.03）", p.Key, *pd.Lock)
	}
	old, had := paramOf(n, p.Key)
	oldWhole := append(json.RawMessage{}, old...)
	if !had {
		oldWhole = pd.Default
	}
	if len(oldWhole) == 0 {
		oldWhole = json.RawMessage("null")
	}
	newWhole := p.To
	if p.Path != nil {
		if len(*p.Path) != 1 || pd.Type != "object" || pd.SchemaFrom == nil {
			return nil, noTouch, invalidf("只有 schema_from 对象参数能写 path（%s）", p.Key)
		}
		obj := map[string]json.RawMessage{}
		if !isNull(oldWhole) {
			if err := json.Unmarshal(oldWhole, &obj); err != nil {
				return nil, noTouch, invalidf("参数 %s 现在的值不是对象", p.Key)
			}
		}
		if prop := (*p.Path)[0]; isNull(p.To) {
			delete(obj, prop)
		} else {
			obj[prop] = p.To
		}
		newWhole = mustRaw(obj)
	}
	if len(newWhole) == 0 {
		newWhole = json.RawMessage("null")
	}
	before := ports(n, e.view)
	setParam(n, p.Key, newWhole, pd.Default)
	after := ports(n, e.view)
	t := schema.Touched{Nodes: []string{string(p.Node)}}
	inv := []gen.Op{mk(gen.OpSetParam, gen.PayloadSetParam{Node: p.Node, Key: p.Key, To: oldWhole})}
	var gone []string
	for name := range before {
		if _, ok := after[name]; !ok {
			gone = append(gone, name)
		}
	}
	sort.Strings(gone)
	for _, name := range gone {
		for _, ed := range m.edgesInto(scope, string(p.Node), name) {
			m.removeEdge(string(ed.ID))
			t.Edges = append(t.Edges, string(ed.ID))
			inv = append(inv, mk(gen.OpConnect, gen.PayloadConnect{Edge: ed}))
		}
	}
	return inv, t, nil
}

func applyPromoteParam(m *mutable, op *gen.Op, e *env) ([]gen.Op, schema.Touched, error) {
	p, err := payloadOf[gen.PayloadPromoteParam](op)
	if err != nil {
		return nil, noTouch, err
	}
	n, _ := m.node(string(p.Node))
	if n == nil {
		return nil, noTouch, invalidf("节点 %s 不存在", p.Node)
	}
	def, ok := e.view.NodeDef(string(n.Type))
	if !ok {
		return nil, noTouch, invalidf("节点类型 %s 没有注册", n.Type)
	}
	if pd, ok := def.Params[p.Key]; !ok || pd.Promotable == nil || !*pd.Promotable {
		return nil, noTouch, invalidf("参数 %s 不能提升为端口", p.Key)
	}
	if n.Promoted != nil && slices.Contains(*n.Promoted, p.Key) {
		return nil, noTouch, invalidf("参数 %s 已经提升", p.Key)
	}
	l := []string{}
	if n.Promoted != nil {
		l = append(l, *n.Promoted...)
	}
	l = append(l, p.Key)
	slices.Sort(l)
	n.Promoted = &l
	return []gen.Op{mk(gen.OpDemoteParam, gen.PayloadDemoteParam{Node: p.Node, Key: p.Key})},
		schema.Touched{Nodes: []string{string(p.Node)}}, nil
}

func applyDemoteParam(m *mutable, op *gen.Op, _ *env) ([]gen.Op, schema.Touched, error) {
	p, err := payloadOf[gen.PayloadDemoteParam](op)
	if err != nil {
		return nil, noTouch, err
	}
	n, scope := m.node(string(p.Node))
	if n == nil {
		return nil, noTouch, invalidf("节点 %s 不存在", p.Node)
	}
	if n.Promoted == nil || !slices.Contains(*n.Promoted, p.Key) {
		return nil, noTouch, invalidf("参数 %s 没有提升", p.Key)
	}
	l := slices.DeleteFunc(append([]string{}, *n.Promoted...), func(s string) bool { return s == p.Key })
	if len(l) == 0 {
		n.Promoted = nil
	} else {
		n.Promoted = &l
	}
	t := schema.Touched{Nodes: []string{string(p.Node)}}
	inv := []gen.Op{mk(gen.OpPromoteParam, gen.PayloadPromoteParam{Node: p.Node, Key: p.Key})}
	for _, ed := range m.edgesInto(scope, string(p.Node), p.Key) {
		m.removeEdge(string(ed.ID))
		t.Edges = append(t.Edges, string(ed.ID))
		inv = append(inv, mk(gen.OpConnect, gen.PayloadConnect{Edge: ed}))
	}
	return inv, t, nil
}
