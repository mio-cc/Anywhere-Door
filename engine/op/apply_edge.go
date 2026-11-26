package op

import (
	"github.com/mio-cc/anywhere-door/engine/schema"
	"github.com/mio-cc/anywhere-door/gen"
)

// 连线（E.05.01 §5.2）。端口存在、方向、类型兼容、成环交给校验（V05–V07、V09）。
func init() {
	appliers[gen.OpConnect] = applyConnect
	appliers[gen.OpDisconnect] = applyDisconnect
}

func applyConnect(m *mutable, op *gen.Op, e *env) ([]gen.Op, schema.Touched, error) {
	p, err := payloadOf[gen.PayloadConnect](op)
	if err != nil {
		return nil, noTouch, err
	}
	ed := p.Edge
	id := string(ed.ID)
	if !validID(id) {
		return nil, noTouch, invalidf("连线 ID %q 不是 ULID", id)
	}
	if m.used(id) {
		return nil, noTouch, invalidf("连线 ID %s 已被占用", id)
	}
	from, fs := m.node(string(ed.From.Node))
	to, ts := m.node(string(ed.To.Node))
	if from == nil || to == nil {
		return nil, noTouch, invalidf("连线两端的节点不存在")
	}
	if fs != ts {
		return nil, noTouch, invalidf("连线两端不在同一层")
	}
	var replaced []gen.Edge
	if pd, ok := ports(to, e.view)[ed.To.Port]; ok && (pd.Multi == nil || !*pd.Multi) {
		replaced = m.edgesInto(ts, string(ed.To.Node), ed.To.Port)
	}
	t := schema.Touched{Nodes: []string{string(ed.From.Node), string(ed.To.Node)}, Edges: []string{id}}
	inv := []gen.Op{mk(gen.OpDisconnect, gen.PayloadDisconnect{IDs: []gen.Ulid{ed.ID}})}
	for _, old := range replaced {
		m.removeEdge(string(old.ID))
		t.Edges = append(t.Edges, string(old.ID))
		inv = append(inv, mk(gen.OpConnect, gen.PayloadConnect{Edge: old}))
	}
	m.insertEdge(ts, ed)
	return inv, t, nil
}

func applyDisconnect(m *mutable, op *gen.Op, _ *env) ([]gen.Op, schema.Touched, error) {
	p, err := payloadOf[gen.PayloadDisconnect](op)
	if err != nil {
		return nil, noTouch, err
	}
	if len(p.IDs) == 0 {
		return nil, noTouch, invalidf("没有写要断开的连线")
	}
	seen := map[gen.Ulid]bool{}
	for _, id := range p.IDs {
		if ed, _ := m.edge(string(id)); ed == nil {
			return nil, noTouch, invalidf("连线 %s 不存在", id)
		}
		if seen[id] {
			return nil, noTouch, invalidf("连线 %s 写了两次", id)
		}
		seen[id] = true
	}
	var t schema.Touched
	var inv []gen.Op
	for _, id := range p.IDs {
		ed := m.removeEdge(string(id))
		t.Edges = append(t.Edges, string(id))
		t.Nodes = append(t.Nodes, string(ed.From.Node), string(ed.To.Node))
		inv = append(inv, mk(gen.OpConnect, gen.PayloadConnect{Edge: ed}))
	}
	return inv, t, nil
}
