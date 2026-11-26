package op

import (
	"slices"
	"unicode/utf8"

	"github.com/mio-cc/anywhere-door/engine/schema"
	"github.com/mio-cc/anywhere-door/gen"
)

// 节点的显示名、标签、实例锁（E.05.01 §5.2）。internal 批次用空值表示删掉这个字段。
func init() {
	appliers[gen.OpSetLabel] = applySetLabel
	appliers[gen.OpSetTags] = applySetTags
	appliers[gen.OpSetLock] = applySetLock
}

func applySetLabel(m *mutable, op *gen.Op, e *env) ([]gen.Op, schema.Touched, error) {
	p, err := payloadOf[gen.PayloadSetLabel](op)
	if err != nil {
		return nil, noTouch, err
	}
	n, _ := m.node(string(p.Node))
	if n == nil {
		return nil, noTouch, invalidf("节点 %s 不存在", p.Node)
	}
	if l := utf8.RuneCountInString(p.To); l > 80 || (l == 0 && !e.internal) {
		return nil, noTouch, invalidf("显示名要 1–80 个字，现在是 %d 个", l)
	}
	old := ""
	if n.Label != nil {
		old = *n.Label
	}
	if p.To == "" {
		n.Label = nil
	} else {
		n.Label = ptr(p.To)
	}
	return []gen.Op{mk(gen.OpSetLabel, gen.PayloadSetLabel{Node: p.Node, To: old})}, schema.Touched{Nodes: []string{string(p.Node)}}, nil
}

func applySetTags(m *mutable, op *gen.Op, _ *env) ([]gen.Op, schema.Touched, error) {
	p, err := payloadOf[gen.PayloadSetTags](op)
	if err != nil {
		return nil, noTouch, err
	}
	n, _ := m.node(string(p.Node))
	if n == nil {
		return nil, noTouch, invalidf("节点 %s 不存在", p.Node)
	}
	if len(p.To) > 16 {
		return nil, noTouch, invalidf("标签最多 16 个，现在是 %d 个", len(p.To))
	}
	old := []string{}
	if n.Tags != nil {
		old = append(old, *n.Tags...)
	}
	if len(p.To) == 0 {
		n.Tags = nil
	} else {
		n.Tags = ptr(slices.Clone(p.To))
	}
	return []gen.Op{mk(gen.OpSetTags, gen.PayloadSetTags{Node: p.Node, To: old})}, schema.Touched{Nodes: []string{string(p.Node)}}, nil
}

func applySetLock(m *mutable, op *gen.Op, _ *env) ([]gen.Op, schema.Touched, error) {
	p, err := payloadOf[gen.PayloadSetLock](op)
	if err != nil {
		return nil, noTouch, err
	}
	n, _ := m.node(string(p.Node))
	if n == nil {
		return nil, noTouch, invalidf("节点 %s 不存在", p.Node)
	}
	if p.To != "none" && p.To != "user" && p.To != "frozen" {
		return nil, noTouch, invalidf("锁只能是 none、user、frozen，不能是 %q", p.To)
	}
	old := "none"
	if n.Lock != nil {
		old = *n.Lock
	}
	if p.To == "none" {
		n.Lock = nil
	} else {
		n.Lock = ptr(p.To)
	}
	return []gen.Op{mk(gen.OpSetLock, gen.PayloadSetLock{Node: p.Node, To: old})}, schema.Touched{Nodes: []string{string(p.Node)}}, nil
}
