package op

import (
	"maps"

	"github.com/mio-cc/anywhere-door/engine/schema"
	"github.com/mio-cc/anywhere-door/gen"
)

// 视图与便签（E.05.01 §5.2）：不影响运行，也不影响校验。
func init() {
	appliers[gen.OpMoveNodes] = applyMoveNodes
	appliers[gen.OpSetNodeUI] = applySetNodeUI
	appliers[gen.OpAddNote] = applyAddNote
	appliers[gen.OpUpdateNote] = applyUpdateNote
	appliers[gen.OpRemoveNotes] = applyRemoveNotes
}

func applyMoveNodes(m *mutable, op *gen.Op, _ *env) ([]gen.Op, schema.Touched, error) {
	p, err := payloadOf[gen.PayloadMoveNodes](op)
	if err != nil {
		return nil, noTouch, err
	}
	if len(p.Moves) == 0 {
		return nil, noTouch, invalidf("没有写要移动的节点")
	}
	for _, mv := range p.Moves {
		if n, _ := m.node(string(mv.ID)); n == nil {
			return nil, noTouch, invalidf("节点 %s 不存在", mv.ID)
		}
	}
	back := make([]gen.PayloadMoveNodesMovesItem, len(p.Moves))
	var t schema.Touched
	for i, mv := range p.Moves {
		n, _ := m.node(string(mv.ID))
		back[len(p.Moves)-1-i] = gen.PayloadMoveNodesMovesItem{ID: mv.ID, To: n.Pos}
		n.Pos = mv.To
		t.Nodes = append(t.Nodes, string(mv.ID))
	}
	return []gen.Op{mk(gen.OpMoveNodes, gen.PayloadMoveNodes{Moves: back})}, t, nil
}

func applySetNodeUI(m *mutable, op *gen.Op, _ *env) ([]gen.Op, schema.Touched, error) {
	p, err := payloadOf[gen.PayloadSetNodeUi](op)
	if err != nil {
		return nil, noTouch, err
	}
	n, _ := m.node(string(p.Node))
	if n == nil {
		return nil, noTouch, invalidf("节点 %s 不存在", p.Node)
	}
	old := gen.NodeUi{}
	if n.UI != nil {
		old = maps.Clone(*n.UI)
	}
	if len(p.To) == 0 {
		n.UI = nil
	} else {
		u := maps.Clone(p.To)
		n.UI = &u
	}
	return []gen.Op{mk(gen.OpSetNodeUI, gen.PayloadSetNodeUi{Node: p.Node, To: old})}, schema.Touched{Nodes: []string{string(p.Node)}}, nil
}

func applyAddNote(m *mutable, op *gen.Op, _ *env) ([]gen.Op, schema.Touched, error) {
	p, err := payloadOf[gen.PayloadAddNote](op)
	if err != nil {
		return nil, noTouch, err
	}
	if _, ok := m.scopes[p.Scope]; !ok {
		return nil, noTouch, invalidf("层 %s 不存在", p.Scope)
	}
	id := string(p.Note.ID)
	if !validID(id) {
		return nil, noTouch, invalidf("便签 ID %q 不是 ULID", id)
	}
	if m.used(id) {
		return nil, noTouch, invalidf("便签 ID %s 已被占用", id)
	}
	n := p.Note
	normNote(&n)
	m.insertNote(p.Scope, n)
	return []gen.Op{mk(gen.OpRemoveNotes, gen.PayloadRemoveNotes{IDs: []gen.Ulid{n.ID}})}, noTouch, nil
}

// applyUpdateNote 的逆操作是删掉再整份加回来：update_note 写不出"删掉 size"（P.09 K261 ⑦）。
func applyUpdateNote(m *mutable, op *gen.Op, _ *env) ([]gen.Op, schema.Touched, error) {
	p, err := payloadOf[gen.PayloadUpdateNote](op)
	if err != nil {
		return nil, noTouch, err
	}
	n, scope := m.note(string(p.ID))
	if n == nil {
		return nil, noTouch, invalidf("便签 %s 不存在", p.ID)
	}
	to := p.To
	if to.Pos == nil && to.Size == nil && to.Text == nil && to.Color == nil {
		return nil, noTouch, invalidf("update_note 至少要改一项")
	}
	old := *n
	if to.Pos != nil {
		n.Pos = *to.Pos
	}
	if to.Size != nil {
		n.Size = ptr(*to.Size)
	}
	if to.Text != nil {
		n.Text = *to.Text
	}
	if to.Color != nil {
		n.Color = ptr(*to.Color)
		normNote(n)
	}
	return []gen.Op{
		mk(gen.OpRemoveNotes, gen.PayloadRemoveNotes{IDs: []gen.Ulid{p.ID}}),
		mk(gen.OpAddNote, gen.PayloadAddNote{Scope: scope, Note: old}),
	}, noTouch, nil
}

func applyRemoveNotes(m *mutable, op *gen.Op, _ *env) ([]gen.Op, schema.Touched, error) {
	p, err := payloadOf[gen.PayloadRemoveNotes](op)
	if err != nil {
		return nil, noTouch, err
	}
	if len(p.IDs) == 0 {
		return nil, noTouch, invalidf("没有写要删的便签")
	}
	seen := map[gen.Ulid]bool{}
	for _, id := range p.IDs {
		if n, _ := m.note(string(id)); n == nil || seen[id] {
			return nil, noTouch, invalidf("便签 %s 不存在或写了两次", id)
		}
		seen[id] = true
	}
	var inv []gen.Op
	for _, id := range p.IDs {
		scope := m.noteAt[string(id)]
		n := m.removeNote(string(id))
		inv = append(inv, mk(gen.OpAddNote, gen.PayloadAddNote{Scope: scope, Note: n}))
	}
	return inv, noTouch, nil
}
