package schema

import (
	"slices"
	"strconv"

	"github.com/mio-cc/anywhere-door/gen"
)

// AssignAt 是连线判定需要的上下文：源节点的类型（C2 的 producer 检查）、目标是不是容器节点（C.04.02-R3：不做 C7 展开）。
type AssignAt struct {
	SrcNode      string
	DstContainer bool
}

type assignRes struct {
	ok      bool
	expand  bool
	migrate string
	rule    string
	reason  string
}

// Assignable 按 C.04.02 §2 的规则表判断 src 端口能不能接到 dst 端口（第一条命中就返回）。类型变量应当已经推断过。
func Assignable(src, dst *Type, at AssignAt, ts TypeSource) gen.Assign {
	r := assigner{ts: ts, at: at}.assign(src, dst, false)
	out := gen.Assign{Ok: r.ok, Rule: r.rule}
	if r.expand {
		out.Expand = ptrTo(true)
	}
	if r.migrate != "" {
		out.Migrate = ptrTo(r.migrate)
	}
	if r.reason != "" {
		out.Reason = ptrTo(r.reason)
	}
	return out
}

func ptrTo[T any](v T) *T { return &v }

type assigner struct {
	ts TypeSource
	at AssignAt
}

func (a assigner) def(t *Type) *gen.TypeDef {
	if t == nil || t.Kind != Named {
		return nil
	}
	d, _ := a.ts.Type(t.key())
	return d
}

func (a assigner) reject(rule string, src, dst *Type, producer string) assignRes {
	name := dst.String()
	if dst.Kind == Named {
		name = dst.Name
	}
	var reason string
	if m := a.ts.Messages(); m != nil {
		reason = m.Assign(rule, map[string]any{"src": src.String(), "dst": dst.String(), "dst_name": name, "producer": producer})
	}
	return assignRes{rule: rule, reason: reason}
}

// ancestors 是沿 extends 向上的类型（名称@主版本），最多 16 层。
func (a assigner) ancestors(t *Type) []string {
	var out []string
	d := a.def(t)
	for d != nil && d.Extends != nil && len(out) < 16 {
		p, err := Parse(string(*d.Extends))
		if err != nil || p.Kind != Named {
			break
		}
		out = append(out, p.key())
		d = a.def(p)
	}
	return out
}

func (a assigner) assign(s, d *Type, inList bool) assignRes {
	if d.Kind == Any {
		return assignRes{ok: true, rule: "C0"}
	}
	if s.Kind == Any {
		return a.reject("C1", s, d, "")
	}
	if dd := a.def(d); dd != nil && dd.Producer != nil && *dd.Producer != "" && a.at.SrcNode != string(*dd.Producer) {
		return a.reject("C2", s, d, string(*dd.Producer))
	}
	if s.Kind == Named && d.Kind == Named {
		if s.Name == d.Name {
			if s.Major == d.Major {
				return assignRes{ok: true, rule: "C3"}
			}
			if dd := a.def(d); dd != nil && dd.MigrateFrom != nil {
				if _, ok := (*dd.MigrateFrom)[strconv.Itoa(s.Major)]; ok {
					return assignRes{ok: true, rule: "C4", migrate: s.Name + "@" + strconv.Itoa(s.Major) + "→" + strconv.Itoa(d.Major)}
				}
			}
			return a.reject("C4", s, d, "")
		}
		if slices.Contains(a.ancestors(s), d.key()) {
			return assignRes{ok: true, rule: "C5"}
		}
		if slices.Contains(a.ancestors(d), s.key()) {
			return a.reject("C6", s, d, "")
		}
	}
	if s.Kind == List && d.Kind != List && !a.at.DstContainer && !inList {
		if e := a.assign(s.Elem, d, true); e.ok {
			return assignRes{ok: true, expand: true, migrate: e.migrate, rule: "C7"}
		}
	}
	if s.Kind != List && d.Kind == List {
		return a.reject("C8", s, d, "")
	}
	if s.Kind == List && d.Kind == List {
		e := a.assign(s.Elem, d.Elem, true)
		return assignRes{ok: e.ok, migrate: e.migrate, rule: "C9", reason: e.reason}
	}
	return a.reject("C10", s, d, "")
}
