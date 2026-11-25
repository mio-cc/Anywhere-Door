package schema

import (
	"sort"
	"strings"
	"sync"

	"github.com/mio-cc/anywhere-door/engine/graph"
	"github.com/mio-cc/anywhere-door/gen"
)

// 端口方向。
const (
	DirIn  = "in"
	DirOut = "out"
)

// Types 是推断后每个端口的类型。
type Types interface {
	Port(nodeID, port, dir string) *Type // dir: in / out；端口不存在、节点类型没有注册时为 nil；推断不出的变量按 任意
}

type pkey struct{ node, port, dir string }

type inferResult struct {
	ports      map[pkey]*Type
	unresolved map[pkey]bool // 还有推断不出的变量（已按 任意 处理）
}

func (r *inferResult) Port(nodeID, port, dir string) *Type { return r.ports[pkey{nodeID, port, dir}] }

type nodeInfo struct {
	n     *graph.Node
	def   *gen.NodeDef
	scope *graph.Scope
	decl  map[pkey]*Type
	vars  []string
	binds map[string]*Type
}

type inferer struct {
	g     *graph.Graph
	v     View
	infos map[string]*nodeInfo
	order []*nodeInfo
}

// Infer 按 C.04.03 §3 推断类型变量。推断不出、而用到这个变量的端口有连线时生成 V16（C.04.03 §3 第 6 步）；
// 冲突不在这里报，交给连线检查（V06，C.04.03-R1）。
func Infer(g *graph.Graph, v View) (Types, []gen.Diag) {
	in := &inferer{g: g, v: v, infos: map[string]*nodeInfo{}}
	graph.Walk(g, func(n *graph.Node, s *graph.Scope, _ []*graph.Node) bool {
		def, ok := v.NodeDef(string(n.Type))
		if !ok {
			return true
		}
		info := &nodeInfo{n: n, def: def, scope: s, decl: map[pkey]*Type{}, binds: map[string]*Type{}}
		ins, outs := Ports(n, def, v)
		for name, p := range ins {
			info.decl[pkey{string(n.ID), name, DirIn}] = parseOrAny(string(p.Type))
		}
		for name, p := range outs {
			info.decl[pkey{string(n.ID), name, DirOut}] = parseOrAny(string(p.Type))
		}
		if def.TypeVars != nil {
			info.vars = sortedKeys(*def.TypeVars)
		}
		in.infos[string(n.ID)] = info
		in.order = append(in.order, info)
		return true
	})
	for round := 0; round <= len(in.order); round++ {
		changed := false
		for _, info := range in.order {
			for _, name := range info.vars {
				if info.binds[name] != nil {
					continue
				}
				if t := in.source(info, name); t != nil && !t.HasVar() {
					info.binds[name] = t
					changed = true
				}
			}
		}
		if !changed {
			break
		}
	}
	return in.result()
}

func parseOrAny(ref string) *Type {
	t, err := Parse(ref)
	if err != nil {
		return &Type{Kind: Any}
	}
	return t
}

// outType 是某个输出端口当前推断出的类型；节点类型没注册时按 任意；还有变量时为 nil。
func (in *inferer) outType(nodeID, port string) *Type {
	info := in.infos[nodeID]
	if info == nil {
		return &Type{Kind: Any}
	}
	d, ok := info.decl[pkey{nodeID, port, DirOut}]
	if !ok {
		return &Type{Kind: Any}
	}
	t := d.Subst(info.binds)
	if t.HasVar() {
		return nil
	}
	return t
}

// splitSources 把 "a | b" 拆成依次尝试的来源（C.04.03 §2）。
func splitSources(src string) []string {
	parts := strings.Split(src, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// source 按 NodeDef.type_vars 的来源写法（C.04.03 §2）取一个变量的类型：几个来源依次尝试，第一个取到的为准；
// 暂时都取不到时为 nil。
func (in *inferer) source(info *nodeInfo, name string) *Type {
	for _, src := range splitSources((*info.def.TypeVars)[name]) {
		if t := in.source1(info, src); t != nil {
			return t
		}
	}
	return nil
}

func (in *inferer) source1(info *nodeInfo, src string) *Type {
	kind, arg, _ := strings.Cut(src, ":")
	id := string(info.n.ID)
	switch kind {
	case "param":
		s, ok := paramString(info.n, info.def, arg)
		if !ok {
			return nil
		}
		t, err := Parse(s)
		if err != nil {
			return nil
		}
		return t
	case "input":
		port, elem := strings.CutSuffix(arg, ".elem")
		edges := info.scope.In(id, port)
		if len(edges) == 0 {
			return in.entryType(info)
		}
		t := in.outType(string(edges[0].From.Node), edges[0].From.Port)
		if t == nil || !elem {
			return t
		}
		if t.Kind == List {
			return t.Elem
		}
		// 上游不是列表：变量取上游本身的类型，端口类型照常确定；"单个值不能接到列表端口"由连线检查按 C8 报 V06，
		// 不在推断里报 V16（C.04.03-R1：冲突不报推断错误）。
		return t
	case "body":
		return in.bodyType(info, arg)
	case "field":
		port, pname, ok := fieldArgs(arg)
		if !ok {
			return nil
		}
		path, ok := paramString(info.n, info.def, pname)
		if !ok || path == "" {
			return nil
		}
		edges := info.scope.In(id, port)
		if len(edges) == 0 {
			return nil
		}
		t := in.outType(string(edges[0].From.Node), edges[0].From.Port)
		if t == nil {
			return nil
		}
		return in.fieldElem(t, path)
	case "op":
		pname, rest, ok := strings.Cut(arg, "(")
		if !ok {
			return nil
		}
		v := strings.TrimSuffix(rest, ")")
		op, ok := paramString(info.n, info.def, pname)
		if !ok {
			return nil
		}
		ref, ok := gen.AggregateOps[op]
		if !ok {
			return nil
		}
		t, err := Parse(ref)
		if err != nil {
			return nil
		}
		return t.Subst(map[string]*Type{"T": info.binds[v]})
	}
	return nil
}

// fieldArgs 拆开 field:<端口>(<参数>) 的参数部分。
func fieldArgs(arg string) (port, param string, ok bool) {
	port, rest, ok := strings.Cut(arg, "(")
	if !ok || port == "" || !strings.HasSuffix(rest, ")") {
		return "", "", false
	}
	return port, strings.TrimSuffix(rest, ")"), true
}

// fieldElem 是类型 t 里字段路径 path（点分）的列表元素类型（C.04.03 §2 的 field:，P.09 K258）：t 是 任意 时为 任意；
// 中间的字段经过 fields 嵌套或命名类型往下找；最后一个字段要是 列表[X]：X 是已注册的命名类型时取 X，
// 是 object、标量这类不是命名类型的写法时为 任意。字段不存在、不是列表时为 nil。
func (in *inferer) fieldElem(t *Type, path string) *Type {
	if t.Kind == Any {
		return &Type{Kind: Any}
	}
	fields, ok := in.typeFields(t)
	if !ok {
		return nil
	}
	segs := strings.Split(path, ".")
	for i, seg := range segs {
		var f *gen.Field
		for j := range fields {
			if fields[j].Name == seg {
				f = &fields[j]
				break
			}
		}
		if f == nil {
			return nil
		}
		if i < len(segs)-1 {
			if f.Fields != nil {
				fields = *f.Fields
				continue
			}
			nt, err := Parse(f.Type)
			if err != nil {
				return nil
			}
			if fields, ok = in.typeFields(nt); !ok {
				return nil
			}
			continue
		}
		inner, isList := strings.CutPrefix(f.Type, "列表[")
		if !isList || !strings.HasSuffix(inner, "]") {
			return nil
		}
		et, err := Parse(strings.TrimSuffix(inner, "]"))
		switch {
		case err != nil, et.HasVar():
			return &Type{Kind: Any}
		case et.Kind == Named:
			if _, ok := in.v.Type(et.key()); !ok {
				return &Type{Kind: Any}
			}
		}
		return et
	}
	return nil
}

// typeFields 是命名类型的字段；不是命名类型、没注册或没有字段时 ok 为 false。
func (in *inferer) typeFields(t *Type) ([]gen.Field, bool) {
	if t == nil || t.Kind != Named {
		return nil, false
	}
	d, ok := in.v.Type(t.key())
	if !ok || d.Fields == nil {
		return nil, false
	}
	return *d.Fields, true
}

// entryType：容器子图里的入口端口节点（role=entry）没有上游连线时，类型取自容器节点（C.04.03 §3 第 4 步的反方向），依次：
//  1. 容器上对应的类型变量：写了 port_name 取来自那个输入端口的变量，否则取容器第一个来自 input: 的变量；
//  2. 容器上那个输入端口声明的类型（不是 任意、变量都已确定时）；
//  3. 连到容器那个输入端口的上游类型（port_name 没写时按 in）：子图引用的端口来自子图里的端口节点、没有声明，
//     声明为 任意 的端口（如扇出的 shared）也靠这一步拿到具体类型；接了几路上游时按 任意；
//  4. 都没有来源（根图的入口、容器那个端口没接线）时按 任意，不报 V16（P.09 K259 ①）。
func (in *inferer) entryType(info *nodeInfo) *Type {
	if role, _ := paramString(info.n, info.def, "role"); role != "entry" {
		return nil
	}
	parent := in.g.Parent(info.scope.ID())
	if parent == nil {
		return &Type{Kind: Any} // 根图的入口：值来自运行的输入，类型不定（P.09 K259 ①）
	}
	c := in.infos[string(parent.ID)]
	if c == nil {
		return &Type{Kind: Any}
	}
	cid := string(c.n.ID)
	pn, _ := paramString(info.n, info.def, "port_name")
	if c.def.TypeVars != nil {
		for _, v := range c.vars {
			src := (*c.def.TypeVars)[v]
			if pn == "" && strings.HasPrefix(src, "input:") || pn != "" && (src == "input:"+pn || src == "input:"+pn+".elem") {
				if b := c.binds[v]; b != nil {
					return b
				}
				if p := strings.TrimSuffix(strings.TrimPrefix(src, "input:"), ".elem"); len(c.scope.In(cid, p)) == 0 {
					return &Type{Kind: Any} // 容器那个端口没有接线：值不存在，类型不定（K259 ①）
				}
				return nil
			}
		}
	}
	port := pn
	if port == "" {
		port = "in"
	}
	if d, ok := c.decl[pkey{cid, port, DirIn}]; ok && d.Kind != Any {
		if t := d.Subst(c.binds); !t.HasVar() {
			return t
		}
		return nil
	}
	switch edges := c.scope.In(cid, port); {
	case len(edges) == 1:
		return in.outType(string(edges[0].From.Node), edges[0].From.Port)
	case len(edges) > 1: // multi 端口接了几路上游：不猜，按 任意
		return &Type{Kind: Any}
	}
	return &Type{Kind: Any} // 找不到来源（容器那个端口没有接线，或没有这个端口）：类型不定（K259 ①）
}

// bodyType：容器子图里 role 为 R 的端口节点推断出的类型（body:R）。
func (in *inferer) bodyType(info *nodeInfo, role string) *Type {
	s, ok := in.g.Scope(string(info.n.ID))
	if !ok {
		return nil
	}
	for _, n := range s.Nodes() {
		ci := in.infos[string(n.ID)]
		if ci == nil || len(ci.vars) == 0 {
			continue
		}
		if r, _ := paramString(ci.n, ci.def, "role"); r == role {
			return ci.binds[ci.vars[0]]
		}
	}
	return nil
}

func mentions(t *Type, v string) bool {
	switch {
	case t == nil:
		return false
	case t.Kind == Var:
		return t.Var == v
	case t.Kind == List:
		return mentions(t.Elem, v)
	}
	return false
}

func (in *inferer) result() (Types, []gen.Diag) {
	res := &inferResult{ports: map[pkey]*Type{}, unresolved: map[pkey]bool{}}
	var diags []gen.Diag
	msgs := in.v.Messages()
	for _, info := range in.order {
		id := string(info.n.ID)
		keys := make([]pkey, 0, len(info.decl))
		for k := range info.decl {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].dir != keys[j].dir {
				return keys[i].dir < keys[j].dir
			}
			return keys[i].port < keys[j].port
		})
		for _, k := range keys {
			t := info.decl[k].Subst(info.binds)
			if t.HasVar() {
				res.unresolved[k] = true
				t = t.anyVars()
			}
			res.ports[k] = t
		}
		for _, name := range info.vars {
			if info.binds[name] != nil {
				continue
			}
			srcs := splitSources((*info.def.TypeVars)[name])
			port, src := "", srcs[0] // 建议按第一个来源写（E.03.02 §5）
			for _, k := range keys {
				if !mentions(info.decl[k], name) {
					continue
				}
				if k.dir == DirIn && len(info.scope.In(id, k.port)) > 0 || k.dir == DirOut && len(info.scope.Out(id, k.port)) > 0 {
					port = k.port
					break
				}
			}
			if port == "" {
				// field: 来源的端口声明里不带这个变量（聚合的 value 是 任意），接了线也算用到（P.09 K258）
				for _, s := range srcs {
					k, a, _ := strings.Cut(s, ":")
					if p, _, ok := fieldArgs(a); k == "field" && ok && len(info.scope.In(id, p)) > 0 {
						port, src = p, s
						break
					}
				}
			}
			if port == "" || msgs == nil {
				continue
			}
			kind, arg, _ := strings.Cut(src, ":")
			at := Loc{Scope: info.scope.ID(), Node: id, Port: port}
			hint := "「" + arg + "」的连线"
			switch kind {
			case "param":
				at.Param = arg
				hint = "参数「" + paramLabel(info.def, arg) + "」"
			case "op":
				p, _, _ := strings.Cut(arg, "(")
				at.Param = p
				hint = "参数「" + paramLabel(info.def, p) + "」"
			case "body":
				hint = "子图里的" + arg + "端口"
			case "field":
				_, p, _ := fieldArgs(arg)
				at.Param = p
				hint = "参数「" + paramLabel(info.def, p) + "」"
			}
			diags = append(diags, msgs.Diag(gen.DiagV16, gen.DiagLevelError, at, map[string]any{"var": "「" + port + "」的类型", "param": hint}))
		}
	}
	Sort(diags)
	return res, diags
}

func paramLabel(def *gen.NodeDef, name string) string {
	if p, ok := def.Params[name]; ok && p.Label != "" {
		return p.Label
	}
	return name
}

// InferCache 缓存推断结果：图文档和注册表都没变时直接用上次的结果（C.04.03-R2）。可以并发使用。
type InferCache struct {
	mu     sync.Mutex
	key    string
	types  Types
	diags  []gen.Diag
	misses int
}

// Infer 同 Infer，命中缓存时不重新计算。
func (c *InferCache) Infer(g *graph.Graph, v View) (Types, []gen.Diag) {
	key := graph.Hash(g.Doc()) + "|" + v.Hash()
	c.mu.Lock()
	defer c.mu.Unlock()
	if key != c.key || c.types == nil {
		c.types, c.diags = Infer(g, v)
		c.key = key
		c.misses++
	}
	return c.types, append([]gen.Diag(nil), c.diags...)
}

// Misses 是重新计算的次数。
func (c *InferCache) Misses() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.misses
}
