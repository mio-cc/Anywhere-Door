package ctxasm

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/mio-cc/anywhere-door/engine/schema"
	"github.com/mio-cc/anywhere-door/gen"
)

// ExpSource 是保留来源名：经验库条目（C.13.01-R5）。M1 没有经验库，取值为空（P.09 K271）。
const ExpSource = "exp"

// Plan 是编译好的配方（E.13.01 §4）。
type Plan struct {
	Recipe   *gen.Recipe
	Ref      string                  // 名称@版本
	Sources  map[string]*schema.Type // recipe.inputs；exp 不在这里
	Sections []SectionPlan
	Exclude  []Exclusion
}

// SectionPlan 是一段的取值计划。
type SectionPlan struct {
	Spec      gen.Section
	Source    string   // 来源名；"" = in 全部或 in[i]
	Index     int      // in[i] 的 i；-1 = 不是 in[i]
	Path      []string // 来源（或 in[i]）之后的字段路径
	Transform Transform
	Format    string
	Order     int
}

// Exclusion 是一条排除：type:方案 → Type；path:*.reasoning → Path ["*", "reasoning"]。
type Exclusion struct {
	Type string
	Path []string
}

var fromRe = regexp.MustCompile(`^in(?:\[(\d+)\])?((?:\.[^.\[\]\s]+)*)$`)

// Compile 编译配方（E.13.01 §5）：来源、from、transform、exclude；字段路径按来源类型检查。
func Compile(r *gen.Recipe, v schema.View) (*Plan, []gen.Diag) {
	ref := r.Name + "@" + strconv.FormatInt(r.Version, 10)
	p := &Plan{Recipe: r, Ref: ref, Sources: map[string]*schema.Type{}}
	var ds []gen.Diag
	if r.Inputs != nil {
		names := make([]string, 0, len(*r.Inputs))
		for n := range *r.Inputs {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			if n == ExpSource {
				ds = append(ds, diag(gen.DiagLevelError, "配方「%s」的来源名 exp 是保留的，不能在 inputs 里声明", ref))
				continue
			}
			t, err := schema.Parse(string((*r.Inputs)[n]))
			if err != nil {
				ds = append(ds, diag(gen.DiagLevelError, "配方「%s」的来源 %s 的类型写法不对", ref, n))
				continue
			}
			p.Sources[n] = t
		}
	}
	for i, s := range r.Sections {
		sp := SectionPlan{Spec: s, Index: -1, Order: i, Format: FormatText}
		if s.Format != nil && *s.Format != "" {
			sp.Format = *s.Format
		}
		switch sp.Format {
		case FormatText, FormatBullets, FormatYAML, FormatJSON:
		default:
			ds = append(ds, diag(gen.DiagLevelError, "配方「%s」的段「%s」的 format 不认识：%s", ref, s.Name, sp.Format))
		}
		m := fromRe.FindStringSubmatch(s.From)
		if m == nil {
			ds = append(ds, diag(gen.DiagLevelError, "配方「%s」的段「%s」的 from 写法不对：%s", ref, s.Name, s.From))
			continue
		}
		path := strings.Split(strings.TrimPrefix(m[2], "."), ".")
		if m[2] == "" {
			path = nil
		}
		switch {
		case m[1] != "":
			sp.Index, _ = strconv.Atoi(m[1])
			sp.Path = path
		case len(path) > 0:
			sp.Source, sp.Path = path[0], path[1:]
			if sp.Source != ExpSource {
				t, ok := p.Sources[sp.Source]
				if !ok {
					ds = append(ds, diag(gen.DiagLevelError, "配方「%s」的段「%s」用的来源 %s 没有在 inputs 里声明", ref, s.Name, sp.Source))
				} else if !pathExists(t, sp.Path, v) {
					ds = append(ds, diag(gen.DiagLevelWarn, "配方「%s」读取的字段 %s 不存在", ref, strings.Join(path, ".")))
				}
			}
		}
		tr := ""
		if s.Transform != nil {
			tr = *s.Transform
		}
		t, err := ParseTransform(tr)
		if err != nil {
			ds = append(ds, diag(gen.DiagLevelError, "配方「%s」的段「%s」：%s", ref, s.Name, err.Error()))
		}
		sp.Transform = t
		p.Sections = append(p.Sections, sp)
	}
	if r.Exclude != nil {
		for _, e := range *r.Exclude {
			switch {
			case strings.HasPrefix(e, "type:") && len(e) > len("type:"):
				p.Exclude = append(p.Exclude, Exclusion{Type: strings.TrimPrefix(e, "type:")})
			case strings.HasPrefix(e, "path:") && len(e) > len("path:"):
				p.Exclude = append(p.Exclude, Exclusion{Path: strings.Split(strings.TrimPrefix(e, "path:"), ".")})
			default:
				ds = append(ds, diag(gen.DiagLevelError, "配方「%s」的 exclude 只认 type: 和 path:：%s", ref, e))
			}
		}
	}
	return p, ds
}

// fieldList 给出一个字段类型下的成员字段；known 为假表示查不了（任意、没写成员的 object、未注册的类型）。
func fieldList(typ string, nested *[]gen.Field, v schema.View) (fs []gen.Field, known bool) {
	if nested != nil && len(*nested) > 0 {
		return *nested, true
	}
	switch typ {
	case "string", "integer", "number", "boolean":
		return nil, true
	case "object", "列表[object]":
		return nil, false
	}
	t, err := schema.Parse(typ)
	if err != nil {
		return nil, false
	}
	for t.Kind == schema.List && t.Elem != nil {
		t = t.Elem
	}
	if t.Kind != schema.Named {
		return nil, false
	}
	return typeFields(t.Name+"@"+strconv.Itoa(t.Major), v, 0)
}

// typeFields 是类型的字段，含 extends 链上父类型的字段。
func typeFields(ref string, v schema.View, depth int) ([]gen.Field, bool) {
	td, ok := v.Type(ref)
	if !ok || depth > 8 {
		return nil, false
	}
	var out []gen.Field
	if td.Extends != nil {
		if parent, ok := typeFields(string(*td.Extends), v, depth+1); ok {
			out = append(out, parent...)
		}
	}
	if td.Fields != nil {
		out = append(out, *td.Fields...)
	}
	return out, true
}

func pathExists(src *schema.Type, path []string, v schema.View) bool {
	typ := src.String()
	var nested *[]gen.Field
	for _, seg := range path {
		fs, known := fieldList(typ, nested, v)
		if !known {
			return true
		}
		var hit *gen.Field
		for i := range fs {
			if fs[i].Name == seg {
				hit = &fs[i]
				break
			}
		}
		if hit == nil {
			return false
		}
		typ, nested = hit.Type, hit.Fields
	}
	return true
}

// Cache 按"名称@版本 + 注册表哈希"缓存编译结果；零值可用。
type Cache struct {
	mu sync.Mutex
	m  map[string]cached
}

type cached struct {
	plan *Plan
	ds   []gen.Diag
}

// Compile 按引用取配方并编译；配方不存在时报 V03 error。
func (c *Cache) Compile(ref string, v schema.View) (*Plan, []gen.Diag) {
	r, ok := v.Recipe(ref)
	if !ok {
		return nil, []gen.Diag{diag(gen.DiagLevelError, "配方「%s」不存在", ref)}
	}
	key := r.Name + "@" + strconv.FormatInt(r.Version, 10) + "|" + v.Hash()
	c.mu.Lock()
	defer c.mu.Unlock()
	if x, ok := c.m[key]; ok {
		return x.plan, x.ds
	}
	p, ds := Compile(r, v)
	if c.m == nil {
		c.m = map[string]cached{}
	}
	c.m[key] = cached{plan: p, ds: ds}
	return p, ds
}

// Bound 是来源分配的结果。In 是全部输入（in[i] 用），All 是去掉被排除类型后的输入（in 用）。
type Bound struct {
	In      []Value
	All     []Value
	Sources map[string]Value
}

// Bind 按类型把输入分给来源（C.13.01-R4）。
func (p *Plan) Bind(in []Value, v schema.View) (Bound, []gen.Diag) {
	b := Bound{In: in, Sources: map[string]Value{}}
	var ds []gen.Diag
	names := make([]string, 0, len(p.Sources))
	for n := range p.Sources {
		names = append(names, n)
	}
	sort.Strings(names)
	lists := map[string][]Value{}
	for i, x := range in {
		if p.excludedType(x.Type) {
			continue
		}
		b.All = append(b.All, x)
		var hits []string
		for _, n := range names {
			if compatible(x.Type, p.Sources[n], v) {
				hits = append(hits, n)
			}
		}
		switch {
		case len(hits) > 1:
			ds = append(ds, diag(gen.DiagLevelError, "配方「%s」的第 %d 条输入同时符合来源 %s", p.Ref, i, strings.Join(hits, "、")))
		case len(hits) == 1:
			n := hits[0]
			if p.Sources[n].Kind == schema.List {
				lists[n] = append(lists[n], x)
			} else if _, dup := b.Sources[n]; dup {
				ds = append(ds, diag(gen.DiagLevelError, "配方「%s」的来源 %s 对应的输入不唯一", p.Ref, n))
			} else {
				b.Sources[n] = x
			}
		}
	}
	for _, n := range names {
		if xs, ok := lists[n]; ok {
			b.Sources[n] = mergeList(p.Sources[n].String(), xs)
		}
	}
	return b, ds
}

func mergeList(typ string, xs []Value) Value {
	out := Value{Type: typ}
	items := []any{}
	for _, x := range xs {
		if l, ok := x.Data.([]any); ok {
			items = append(items, l...)
		} else {
			items = append(items, x.Data)
		}
		out.Taint = out.Taint || x.Taint
		out.TaintSrc = append(out.TaintSrc, x.TaintSrc...)
		for k, val := range x.Inherit {
			if out.Inherit == nil {
				out.Inherit = map[string]any{}
			}
			if _, ok := out.Inherit[k]; !ok {
				out.Inherit[k] = val
			}
		}
	}
	out.Data = items
	out.TaintSrc = uniqSorted(out.TaintSrc)
	return out
}

func (p *Plan) excludedType(typ string) bool {
	if typ == "" {
		return false
	}
	t, err := schema.Parse(typ)
	if err != nil {
		return false
	}
	for t.Kind == schema.List && t.Elem != nil {
		t = t.Elem
	}
	if t.Kind != schema.Named {
		return false
	}
	for _, e := range p.Exclude {
		if e.Type != "" && (e.Type == t.Name || e.Type == t.Name+"@"+strconv.Itoa(t.Major)) {
			return true
		}
	}
	return false
}

// compatible：值的类型和来源类型相同、或是它的子类型（沿 extends，C.04.02 C5）；列表来源也收元素类型的值。
func compatible(vt string, src *schema.Type, v schema.View) bool {
	if vt == "" {
		return false
	}
	t, err := schema.Parse(vt)
	if err != nil || t.Kind == schema.Any {
		return false
	}
	if src.Kind == schema.List && src.Elem != nil {
		if sameOrSub(t, src.Elem, v) {
			return true
		}
		return t.Kind == schema.List && t.Elem != nil && sameOrSub(t.Elem, src.Elem, v)
	}
	return sameOrSub(t, src, v)
}

func sameOrSub(a, b *schema.Type, v schema.View) bool {
	if a.Kind != schema.Named || b.Kind != schema.Named {
		return false
	}
	want := b.Name + "@" + strconv.Itoa(b.Major)
	cur := a.Name + "@" + strconv.Itoa(a.Major)
	for i := 0; i < 8; i++ {
		if cur == want {
			return true
		}
		td, ok := v.Type(cur)
		if !ok || td.Extends == nil {
			return false
		}
		pt, err := schema.Parse(string(*td.Extends))
		if err != nil || pt.Kind != schema.Named {
			return false
		}
		cur = pt.Name + "@" + strconv.Itoa(pt.Major)
	}
	return false
}

func uniqSorted(xs []string) []string {
	if len(xs) == 0 {
		return nil
	}
	m := map[string]bool{}
	var out []string
	for _, x := range xs {
		if x != "" && !m[x] {
			m[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}
