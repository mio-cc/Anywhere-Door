package op

import (
	"fmt"

	"github.com/mio-cc/anywhere-door/engine/graph"
	"github.com/mio-cc/anywhere-door/gen"
)

// Target 按 payload 算这条 Op 的作用对象（E.05.03 §5.1）。客户端填的 target 不用（C.05.01 §3）。
// g 是应用之前的图；节点、连线、便签在 g 里找不到时 scope 取 root（前置条件由应用函数检查）。
func Target(op *gen.Op, g *graph.Graph) (gen.OpTarget, error) {
	var t targetBuilder
	if err := t.add(op, g); err != nil {
		return gen.OpTarget{}, err
	}
	return t.result(), nil
}

type targetBuilder struct {
	scope               string
	nodes, edges, notes []string
}

func (t *targetBuilder) setScope(s string) {
	if t.scope == "" {
		t.scope = s
	}
}

func (t *targetBuilder) result() gen.OpTarget {
	out := gen.OpTarget{Scope: t.scope}
	if out.Scope == "" {
		out.Scope = graph.RootScope
	}
	if l := sortedIDs(t.nodes); len(l) > 0 {
		u := ulids(l)
		out.Nodes = &u
	}
	if l := sortedIDs(t.edges); len(l) > 0 {
		u := ulids(l)
		out.Edges = &u
	}
	if l := sortedIDs(t.notes); len(l) > 0 {
		u := ulids(l)
		out.Notes = &u
	}
	return out
}

func (t *targetBuilder) add(op *gen.Op, g *graph.Graph) error {
	switch op.Kind {
	case gen.OpAddNode:
		p, err := decode[gen.PayloadAddNode](op)
		if err != nil {
			return err
		}
		t.setScope(p.Scope)
		t.nodes = append(t.nodes, nodeTree(&p.Node)...)
	case gen.OpAddNote:
		p, err := decode[gen.PayloadAddNote](op)
		if err != nil {
			return err
		}
		t.setScope(p.Scope)
		t.notes = append(t.notes, string(p.Note.ID))
	case gen.OpRemoveNodes:
		p, err := decode[gen.PayloadRemoveNodes](op)
		if err != nil {
			return err
		}
		t.nodeList(g, strs(p.IDs))
	case gen.OpMoveNodes:
		p, err := decode[gen.PayloadMoveNodes](op)
		if err != nil {
			return err
		}
		ids := make([]string, len(p.Moves))
		for i, m := range p.Moves {
			ids[i] = string(m.ID)
		}
		t.nodeList(g, ids)
	case gen.OpSetParam, gen.OpPromoteParam, gen.OpDemoteParam, gen.OpSetLabel, gen.OpSetTags, gen.OpSetLock, gen.OpSetNodeUI:
		id, err := nodeOf(op)
		if err != nil {
			return err
		}
		t.nodeList(g, []string{id})
	case gen.OpRestoreNodes:
		p, err := decode[gen.PayloadRestoreNodes](op)
		if err != nil {
			return err
		}
		for i := range p.Removed.Nodes {
			t.setScope(p.Removed.Nodes[i].Scope)
			t.nodes = append(t.nodes, nodeTree(&p.Removed.Nodes[i].Node)...)
		}
		for _, e := range p.Removed.Edges {
			t.setScope(e.Scope)
			t.edges = append(t.edges, string(e.Edge.ID))
		}
	case gen.OpConnect:
		p, err := decode[gen.PayloadConnect](op)
		if err != nil {
			return err
		}
		from, to := string(p.Edge.From.Node), string(p.Edge.To.Node)
		if _, ok := nodeScope(g, from); ok {
			t.nodeList(g, []string{from, to})
		} else {
			t.nodeList(g, []string{to, from})
		}
		t.edges = append(t.edges, string(p.Edge.ID))
	case gen.OpDisconnect:
		p, err := decode[gen.PayloadDisconnect](op)
		if err != nil {
			return err
		}
		for _, id := range p.IDs {
			if g != nil {
				if e, ok := g.Edge(string(id)); ok {
					if s, ok := nodeScope(g, string(e.From.Node)); ok {
						t.setScope(s)
					}
				}
			}
			t.edges = append(t.edges, string(id))
		}
	case gen.OpUpdateNote:
		p, err := decode[gen.PayloadUpdateNote](op)
		if err != nil {
			return err
		}
		t.noteList(g, []string{string(p.ID)})
	case gen.OpRemoveNotes:
		p, err := decode[gen.PayloadRemoveNotes](op)
		if err != nil {
			return err
		}
		t.noteList(g, strs(p.IDs))
	case gen.OpSetGraphTitle, gen.OpSetGraphMeta, gen.OpMigrate:
		t.setScope(graph.RootScope)
	case gen.OpRevertTo:
		p, err := decode[gen.PayloadRevertTo](op)
		if err != nil {
			return err
		}
		t.setScope(graph.RootScope)
		if p.Ops != nil {
			for i := range *p.Ops {
				if err := t.add(&(*p.Ops)[i], g); err != nil {
					return err
				}
			}
		}
	default:
		return fmt.Errorf("op: 还没有 %s 的作用对象（E.05-R6）", op.Kind)
	}
	return nil
}

// nodeList 记下节点；scope 取第一个在 g 里找得到的节点所在的层。
func (t *targetBuilder) nodeList(g *graph.Graph, ids []string) {
	for _, id := range ids {
		if s, ok := nodeScope(g, id); ok {
			t.setScope(s)
			break
		}
	}
	t.nodes = append(t.nodes, ids...)
}

func (t *targetBuilder) noteList(g *graph.Graph, ids []string) {
	for _, id := range ids {
		if s, ok := noteScope(g, id); ok {
			t.setScope(s)
			break
		}
	}
	t.notes = append(t.notes, ids...)
}

// nodeScope 是节点所在的层；g 为空或找不到节点时 ok 为 false。
func nodeScope(g *graph.Graph, id string) (string, bool) {
	if g == nil {
		return "", false
	}
	s := g.ScopeOf(id)
	if s == nil {
		return "", false
	}
	return s.ID(), true
}

// noteScope 是便签所在的层（graph 没有按便签查层的索引，逐层找）。
func noteScope(g *graph.Graph, id string) (string, bool) {
	if g == nil {
		return "", false
	}
	if _, ok := g.Note(id); !ok {
		return "", false
	}
	for _, s := range g.Scopes() {
		for _, n := range s.Notes() {
			if string(n.ID) == id {
				return s.ID(), true
			}
		}
	}
	return "", false
}
