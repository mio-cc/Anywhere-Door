package ctxasm

import (
	"context"
	"errors"
	"fmt"

	"github.com/mio-cc/anywhere-door/engine/blob"
	"github.com/mio-cc/anywhere-door/engine/llm"
	"github.com/mio-cc/anywhere-door/engine/schema"
	"github.com/mio-cc/anywhere-door/gen"
)

// Env 是装配需要的环境（E.13.02 §4）。
type Env struct {
	View       schema.View
	Blobs      blob.Store
	Counter    Counter
	Summarizer Summarizer
	Caps       llm.Caps // 决定窗口的模型能力
	Overhead   int      // ①–⑤ 段估算（E.13.03）
	Budget     *float64 // 节点 budget 参数
	OnOverflow string   // 节点 on_overflow 参数；空 = 配方的
}

// ErrMissingRequired：配方的必需段没有内容。返回时包成 *Error{user_input, "missing_required"}，可以用 errors.Is 判断。
var ErrMissingRequired = errors.New("配方的必需段没有内容")

// Assemble 按 E.13.02 §5 装配：分配来源 → 排除 → 取值、变换 → 加入透传约束锁定段 → 预算与溢出处理 → 排序。
func Assemble(ctx context.Context, plan *Plan, in []Value, env Env) (Packet, error) {
	if env.Counter == nil {
		env.Counter = runeCounter{}
	}
	b, ds := plan.Bind(in, env.View)
	for _, d := range ds {
		if d.Level == gen.DiagLevelError {
			return Packet{}, &Error{Class: gen.FailureClassUserInput, Code: "source_ambiguous", Msg: d.Msg}
		}
	}
	// 2. 排除：type: 已在 Bind 里去掉（不进 All、不分给来源）；path: 在副本上删字段
	all := make([]Value, len(b.All))
	for i, x := range b.All {
		all[i] = plan.strip(x)
	}
	sources := make(map[string]Value, len(b.Sources))
	for k, x := range b.Sources {
		sources[k] = plan.strip(x)
	}
	byIndex := make([]*Value, len(b.In))
	for i, x := range b.In {
		if !plan.excludedType(x.Type) {
			s := plan.strip(x)
			byIndex[i] = &s
		}
	}

	inherit := Carry(in, env.View) // 透传约束不受 exclude 影响（E.13.06）
	lock, hasLock := InheritPart(inherit, env.Counter)
	f := &fitter{ctx: ctx, env: FitEnv{Blobs: env.Blobs, Counter: env.Counter, Summarizer: env.Summarizer}}

	var parts []Part
	vars := map[string]string{} // 段名 → var 名
	placed := false
	for _, sp := range plan.Sections {
		order := sp.Order * 2 // 留出奇数位置给锁定段
		if hasLock && sp.Spec.Name == InheritSection {
			lp := lock
			lp.Order = order
			parts = append(parts, lp)
			placed = true
			continue
		}
		data, taint, taintSrc := pick(sp, all, sources, byIndex)
		required := sp.Spec.Required != nil && *sp.Spec.Required
		if empty(data) {
			if required {
				return Packet{}, &Error{Class: gen.FailureClassUserInput, Code: "missing_required",
					Msg: fmt.Sprintf("配方「%s」的必需段「%s」没有内容", plan.Ref, sp.Spec.Name), Err: ErrMissingRequired}
			}
			continue
		}
		p := Part{Name: sp.Spec.Name, Text: RenderText(sp.Transform.Apply(data), sp.Format), Source: sp.Spec.From,
			Required: required, Order: order, Taint: taint, TaintSrc: taintSrc}
		if sp.Spec.Priority != nil {
			p.Priority = int(*sp.Spec.Priority)
		}
		if sp.Spec.Budget != nil {
			p.Budget = *sp.Spec.Budget
		}
		switch sp.Transform.Kind {
		case TransformPointer:
			if err := f.ensureRef(&p); err != nil {
				return Packet{}, err
			}
			p.Text, p.Pointer = PointerText(p.Name, p.Text, p.Ref), true
		case TransformSummary:
			p.Summary = true
		}
		p.Tokens = env.Counter.CountText(p.Text)
		if sp.Spec.Var != nil && *sp.Spec.Var != "" {
			vars[p.Name] = *sp.Spec.Var
		}
		parts = append(parts, p)
	}
	if hasLock && !placed {
		lp := lock
		lp.Order = 1 // 配方里没有"约束"段：放在第一段之后
		parts = append(parts, lp)
	}

	maxRatio := 0.0
	if plan.Recipe.Budget.MaxRatio != nil {
		maxRatio = *plan.Recipe.Budget.MaxRatio
	}
	bo := Budget(BudgetIn{Ratio: plan.Recipe.Budget.Ratio, MaxRatio: maxRatio, Node: env.Budget, Caps: env.Caps, Overhead: env.Overhead})
	mode := env.OnOverflow
	if mode == "" {
		mode = plan.Recipe.OnOverflow
	}
	fitted, dropped, err := Fit(ctx, parts, bo.Tokens, mode, f.env)
	if err != nil {
		return Packet{}, err
	}

	pkt := Packet{Recipe: plan.Ref, Budget: bo.Tokens, Clamped: bo.Clamped, Dropped: dropped, Inherit: inherit}
	if bo.Warn != "" {
		pkt.Warnings = append(pkt.Warnings, bo.Warn)
	}
	shrunk := map[string]bool{}
	for _, d := range dropped {
		shrunk[d.Section] = true
	}
	for _, p := range fitted {
		pkt.Sections = append(pkt.Sections, PacketSection{Name: p.Name, Text: p.Text, Source: p.Source, Ref: p.Ref,
			Tokens: p.Tokens, Taint: p.Taint, Locked: p.Locked, TaintSrc: p.TaintSrc})
		pkt.Tokens += p.Tokens
		pkt.Pointers = pkt.Pointers || p.Pointer
		if p.Taint {
			pkt.Taint = true
			pkt.TaintSrc = append(pkt.TaintSrc, p.TaintSrc...)
		}
		if v, ok := vars[p.Name]; ok && !shrunk[p.Name] {
			if pkt.Vars == nil {
				pkt.Vars = map[string]string{}
			}
			pkt.Vars[v] = p.Text
		}
		if p.Locked && p.Name == InheritSection && InheritOver(p.Tokens, bo.Tokens) {
			pkt.Warnings = append(pkt.Warnings, fmt.Sprintf("透传约束占了预算的 %d%%，请精简约束", p.Tokens*100/max(1, bo.Tokens)))
		}
	}
	pkt.TaintSrc = uniqSorted(pkt.TaintSrc)
	return pkt, nil
}

// pick 按段的 from 取值（C.13.01-R4）：in → 全部输入（按输入顺序的列表）；in[i] → 第 i 条；in.<来源>.<路径> → 来源的值；
// exp 在 M1 为空。返回取到的值和它用到的输入值的污染。
func pick(sp SectionPlan, all []Value, sources map[string]Value, byIndex []*Value) (any, bool, []string) {
	var used []Value
	var data any
	switch {
	case sp.Index >= 0:
		if sp.Index < len(byIndex) && byIndex[sp.Index] != nil {
			used = []Value{*byIndex[sp.Index]}
			data = used[0].Data
		}
	case sp.Source == ExpSource:
		return nil, false, nil
	case sp.Source == "":
		used = all
		l := make([]any, 0, len(all))
		for _, x := range all {
			l = append(l, x.Data)
		}
		data = l
	default:
		if x, ok := sources[sp.Source]; ok {
			used = []Value{x}
			data = x.Data
		}
	}
	data = lookup(data, sp.Path)
	taint := false
	var src []string
	for _, x := range used {
		if x.Taint {
			taint = true
			src = append(src, x.TaintSrc...)
		}
	}
	return data, taint, uniqSorted(src)
}

// empty：nil、空字符串、空列表、空对象。
func empty(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	}
	return isEmptyColl(v)
}

// strip 在值的副本上删掉 exclude 里 path: 写的字段（* 匹配任意一层；列表不算一层）。
func (p *Plan) strip(x Value) Value {
	has := false
	for _, e := range p.Exclude {
		if len(e.Path) > 0 {
			has = true
		}
	}
	if !has {
		return x
	}
	d := deepCopy(x.Data)
	for _, e := range p.Exclude {
		if len(e.Path) > 0 {
			d = removePath(d, e.Path)
		}
	}
	x.Data = d
	return x
}

func removePath(d any, path []string) any {
	switch x := d.(type) {
	case []any:
		for i := range x {
			x[i] = removePath(x[i], path)
		}
	case map[string]any:
		if len(path) == 1 {
			if path[0] == "*" {
				for k := range x {
					delete(x, k)
				}
			} else {
				delete(x, path[0])
			}
			return x
		}
		if path[0] == "*" {
			for k, v := range x {
				x[k] = removePath(v, path[1:])
			}
		} else if v, ok := x[path[0]]; ok {
			x[path[0]] = removePath(v, path[1:])
		}
	}
	return d
}

func deepCopy(v any) any {
	switch x := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, e := range x {
			m[k] = deepCopy(e)
		}
		return m
	case []any:
		l := make([]any, len(x))
		for i, e := range x {
			l[i] = deepCopy(e)
		}
		return l
	}
	return v
}
