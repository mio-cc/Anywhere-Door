package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/mio-cc/anywhere-door/engine/graph"
	"github.com/mio-cc/anywhere-door/gen"
)

// Touched 是被改动的节点、连线 ID（含已删除的）；Op 流水线负责把 Op 转成它（第 ③ 片，E.03.03 §4）。
type Touched struct{ Nodes, Edges []string }

// Opts 是校验选项。
type Opts struct {
	ForRun bool // 冻结前校验：warn 也返回，但只有 error 阻止运行；不设时只返回 error
}

// Rule 是一条图校验规则（E.03.03 §5）。每条规则一个文件，在 rules.go 里按编号登记。
type Rule interface {
	Code() gen.DiagCode
	Check(c *Context) []gen.Diag
}

// Context 是规则看到的东西：图、视图、推断结果，以及这一次要检查的范围。
type Context struct {
	G     *graph.Graph
	V     View
	Types Types

	inferDiags []gen.Diag
	msgs       *Messages
	limit      map[string]bool // 只检查这些子图（增量时的局部规则）；nil 为全部
	cports     map[string]*cPorts
	ports      map[string]*nodePorts
	ends       map[string]*edgeEnds
	assigns    map[string]*assignRes2
}

// Validate 全量校验（E.03.03 §5）。结果排序、去重；不设 ForRun 时只返回 error。
func Validate(g *graph.Graph, v View, opt Opts) []gen.Diag {
	return run(g, v, opt, rules, nil, nil)
}

// ValidateIncremental 增量校验：prev 是上次（同样的 opt）的完整结果，返回新的完整结果。
// 局部规则只对被触及的节点、连线所在的子图（和它们的父子图）重新检查，其余沿用 prev；
// 全局规则（V01、V06、V09、V12、V14、V16：要跨子图看类型或整张图）每次全量。
// touched 里有找不到的 ID（已删除）时退回全量：删掉的东西不知道原来在哪个子图（P.09 K259）。
func ValidateIncremental(g *graph.Graph, prev []gen.Diag, touched Touched, v View, opt Opts) []gen.Diag {
	affected := map[string]bool{}
	add := func(sid string) {
		affected[sid] = true
		if p := g.Parent(sid); p != nil {
			if ps := g.ScopeOf(string(p.ID)); ps != nil {
				affected[ps.ID()] = true
			}
		}
	}
	for _, id := range touched.Nodes {
		if _, ok := g.Node(id); !ok {
			return Validate(g, v, opt)
		}
		add(g.ScopeOf(id).ID())
		if _, ok := g.Scope(id); ok {
			add(id)
		}
	}
	for _, id := range touched.Edges {
		e, ok := g.Edge(id)
		if !ok {
			return Validate(g, v, opt)
		}
		for _, n := range []gen.Ulid{e.From.Node, e.To.Node} {
			if s := g.ScopeOf(string(n)); s != nil {
				add(s.ID())
			}
		}
		for _, s := range g.Scopes() {
			for _, se := range s.Edges() {
				if string(se.ID) == id {
					add(s.ID())
				}
			}
		}
	}
	local := map[gen.DiagCode]bool{}
	for _, r := range rules {
		if isLocal(r) {
			local[r.Code()] = true
		}
	}
	var keep []gen.Diag
	for _, d := range prev {
		if local[d.Code] && !affected[str(d.Scope)] {
			keep = append(keep, d)
		}
	}
	return run(g, v, opt, rules, affected, keep)
}

func run(g *graph.Graph, v View, opt Opts, rs []Rule, limit map[string]bool, keep []gen.Diag) []gen.Diag {
	types, idiags := Infer(g, v)
	c := &Context{G: g, V: v, Types: types, inferDiags: idiags, msgs: v.Messages(),
		cports: map[string]*cPorts{}, ports: map[string]*nodePorts{}, ends: map[string]*edgeEnds{}, assigns: map[string]*assignRes2{}}
	out := append([]gen.Diag(nil), keep...)
	for _, r := range rs {
		c.limit = nil
		if limit != nil && isLocal(r) {
			c.limit = limit
		}
		out = append(out, c.safe(r)...)
	}
	if !opt.ForRun {
		errs := out[:0]
		for _, d := range out {
			if d.Level == gen.DiagLevelError {
				errs = append(errs, d)
			}
		}
		out = errs
	}
	Sort(out)
	return Dedup(out)
}

// safe 跑一条规则；规则 panic 时转成一条 V01 诊断（E.03.03 §7），不影响别的规则。
func (c *Context) safe(r Rule) (ds []gen.Diag) {
	defer func() {
		if p := recover(); p != nil {
			slog.Default().Error("校验规则内部错误", "rule", string(r.Code()), "panic", p)
			ds = []gen.Diag{{Code: gen.DiagV01, Level: gen.DiagLevelError, Msg: "校验器内部错误：" + string(r.Code())}}
		}
	}()
	return r.Check(c)
}

// diag 生成一条诊断。
func (c *Context) diag(code gen.DiagCode, level gen.DiagLevel, at Loc, vars map[string]any) gen.Diag {
	if c.msgs != nil {
		return c.msgs.Diag(code, level, at, vars)
	}
	d := gen.Diag{Code: code, Level: level, Msg: string(code)}
	if at.Scope != "" {
		d.Scope = &at.Scope
	}
	if at.Node != "" {
		n := gen.Ulid(at.Node)
		d.Node = &n
	}
	if at.Port != "" {
		d.Port = &at.Port
	}
	if at.Param != "" {
		d.Param = &at.Param
	}
	return d
}

// scopes 是这一次要检查的子图。
func (c *Context) scopes() []*graph.Scope {
	var out []*graph.Scope
	for _, s := range c.G.Scopes() {
		if c.limit == nil || c.limit[s.ID()] {
			out = append(out, s)
		}
	}
	return out
}

type nodeAt struct {
	n   *graph.Node
	s   *graph.Scope
	def *gen.NodeDef // 节点类型没有注册时为 nil
}

// nodes 是这一次要检查的节点（按子图、再按文档顺序）。
func (c *Context) nodes() []nodeAt {
	var out []nodeAt
	for _, s := range c.scopes() {
		for _, n := range s.Nodes() {
			def, _ := c.V.NodeDef(string(n.Type))
			out = append(out, nodeAt{n: n, s: s, def: def})
		}
	}
	return out
}

func label(n *graph.Node) string {
	if n.Label != nil && *n.Label != "" {
		return *n.Label
	}
	return string(n.Type)
}

func containerOf(def *gen.NodeDef) string {
	if def == nil || def.Container == nil || *def.Container == "" {
		return "none"
	}
	return *def.Container
}

// derived：节点定义没有声明端口、又能带子图（子图引用）：外部端口由子图里的端口节点决定（C.02.04-R3）。
func derived(def *gen.NodeDef) bool {
	return def != nil && len(def.Ports.Inputs) == 0 && len(def.Ports.Outputs) == 0 && containerOf(def) != "none"
}

// onlyPort 是只有一个端口时的端口名（子图端口节点的 in / out）；不止一个时取排序后的第一个。
func onlyPort[V any](m map[string]V) string {
	ks := sortedKeys(m)
	if len(ks) == 0 {
		return ""
	}
	return ks[0]
}

// cPorts 是由子图端口节点生成的外部端口（C.02.04-R3，P.09 K258 ⑤）。
type cPorts struct {
	in, out map[string]*gen.PortDef
	entry   map[string]string // 入口端口节点 ID → 外部输入端口名
	exit    map[string]string // 外部输出端口名 → 端口节点 ID
	open    bool              // 引用子图库、没有内联子图：端口要到 M2 才知道，不查
}

func uniqName(m map[string]*gen.PortDef, base string) string {
	if _, ok := m[base]; !ok {
		return base
	}
	for i := 2; ; i++ {
		if n := base + "_" + strconv.Itoa(i); m[n] == nil {
			return n
		}
	}
}

func portFromType(t *Type, lbl string) *gen.PortDef {
	ref := gen.TypeRef("任意")
	if t != nil {
		ref = gen.TypeRef(t.String())
	}
	l := lbl
	return &gen.PortDef{Type: ref, Label: &l}
}

// ContainerPorts 是子图引用这类节点由子图端口节点生成的外部端口（C.02.04-R3）：entry → 输入端口，exit → 输出端口，
// summary、escalation → 同名输出端口；名字取 port_name，缺省 in / out，重名依次 in_2、out_2…；类型取那个端口节点推断出的类型。
// 节点定义声明了端口的（循环、扇出）不用它，用 Ports。
func ContainerPorts(g *graph.Graph, n *graph.Node, v View, types Types) (in, out map[string]*gen.PortDef) {
	cp := containerPorts(g, n, v, types)
	return cp.in, cp.out
}

func containerPorts(g *graph.Graph, n *graph.Node, v View, types Types) *cPorts {
	cp := &cPorts{in: map[string]*gen.PortDef{}, out: map[string]*gen.PortDef{}, entry: map[string]string{}, exit: map[string]string{}}
	s, ok := g.Scope(string(n.ID))
	if !ok {
		cp.open = n.Ref != nil
		return cp
	}
	for _, b := range s.Nodes() {
		def, ok := v.NodeDef(string(b.Type))
		if !ok || def.Category != "边界" {
			continue
		}
		role, ok := paramString(b, def, "role")
		if !ok {
			continue
		}
		pn, _ := paramString(b, def, "port_name")
		id := string(b.ID)
		var t *Type
		switch role {
		case "entry":
			name := pn
			if name == "" {
				name = "in"
			}
			name = uniqName(cp.in, name)
			if types != nil {
				t = types.Port(id, onlyPort(def.Ports.Outputs), DirOut)
			}
			cp.in[name] = portFromType(t, label(b))
			cp.entry[id] = name
		case "exit", "summary", "escalation":
			name := role
			if role == "exit" {
				name = pn
				if name == "" {
					name = "out"
				}
			}
			name = uniqName(cp.out, name)
			if types != nil {
				t = types.Port(id, onlyPort(def.Ports.Inputs), DirIn)
			}
			cp.out[name] = portFromType(t, label(b))
			cp.exit[name] = id
		}
	}
	return cp
}

func (c *Context) cport(n *graph.Node) *cPorts {
	id := string(n.ID)
	if cp, ok := c.cports[id]; ok {
		return cp
	}
	cp := containerPorts(c.G, n, c.V, c.Types)
	c.cports[id] = cp
	return cp
}

type nodePorts struct {
	in, out map[string]*gen.PortDef
	open    bool
}

// nodePortsOf 是节点的全部端口：普通节点用 Ports（含动态端口），子图引用这类用 ContainerPorts。
func (c *Context) nodePortsOf(n *graph.Node, def *gen.NodeDef) *nodePorts {
	id := string(n.ID)
	if np, ok := c.ports[id]; ok {
		return np
	}
	np := &nodePorts{}
	if derived(def) {
		cp := c.cport(n)
		np.in, np.out, np.open = cp.in, cp.out, cp.open
	} else {
		np.in, np.out = Ports(n, def, c.V)
	}
	c.ports[id] = np
	return np
}

// entryPortName 是入口端口节点对应的容器外部输入端口名：子图引用按 ContainerPorts；声明了端口的容器写了 port_name 取它，
// 否则取容器第一个来自 input: 的类型变量的端口（同 E.03.02 §5 入口端口的取法），都没有时为 in。
func (c *Context) entryPortName(parent *graph.Node, pdef *gen.NodeDef, entry *graph.Node, edef *gen.NodeDef) string {
	if derived(pdef) {
		return c.cport(parent).entry[string(entry.ID)]
	}
	if pn, _ := paramString(entry, edef, "port_name"); pn != "" {
		return pn
	}
	if pdef.TypeVars != nil {
		for _, v := range sortedKeys(*pdef.TypeVars) {
			for _, src := range splitSources((*pdef.TypeVars)[v]) {
				if p, ok := strings.CutPrefix(src, "input:"); ok {
					return strings.TrimSuffix(p, ".elem")
				}
			}
		}
	}
	return "in"
}

// srcNodes 是连线源头真正产出值的节点（P.09 K258 ④）：源是入口端口节点时沿容器外部那个输入端口的上游找；
// 源是子图引用这类节点、端口来自出口端口节点时，沿子图里连到那个出口端口节点的线往回找；一直找到不是这两种的节点。
// 找不到上游时为空。
func (c *Context) srcNodes(nodeID, port string, depth int) []*graph.Node {
	n, ok := c.G.Node(nodeID)
	if !ok || depth > 64 {
		return nil
	}
	def, ok := c.V.NodeDef(string(n.Type))
	if !ok {
		return []*graph.Node{n}
	}
	if def.Category == "边界" {
		if role, _ := paramString(n, def, "role"); role == "entry" {
			s := c.G.ScopeOf(nodeID)
			parent := c.G.Parent(s.ID())
			if parent == nil {
				return nil
			}
			pdef, ok := c.V.NodeDef(string(parent.Type))
			if !ok {
				return nil
			}
			pport := c.entryPortName(parent, pdef, n, def)
			ps := c.G.ScopeOf(string(parent.ID))
			var out []*graph.Node
			for _, e := range ps.In(string(parent.ID), pport) {
				out = append(out, c.srcNodes(string(e.From.Node), e.From.Port, depth+1)...)
			}
			return out
		}
	}
	if derived(def) {
		xid, ok := c.cport(n).exit[port]
		if !ok {
			return nil
		}
		cs, _ := c.G.Scope(nodeID)
		xn, _ := c.G.Node(xid)
		xdef, ok := c.V.NodeDef(string(xn.Type))
		if !ok {
			return nil
		}
		var out []*graph.Node
		for _, e := range cs.In(xid, onlyPort(xdef.Ports.Inputs)) {
			out = append(out, c.srcNodes(string(e.From.Node), e.From.Port, depth+1)...)
		}
		return out
	}
	return []*graph.Node{n}
}

var slotRe = regexp.MustCompile(`^([a-z_][a-z0-9_]*)\[(\d{1,3})\]$`)

// edgeEnds 是一条连线两端核对后的结果（V05 和用到端口的规则共用）。
type edgeEnds struct {
	ok               bool   // 两端节点、端口都对
	reason           string // 不对时给 V05 的原因；空表示不报 V05（例如节点类型没注册，交给 V02）
	at               Loc
	dstDef           *gen.NodeDef
	dstPD            *gen.PortDef
	srcType, dstType *Type
}

func (c *Context) edgeEnds(s *graph.Scope, e *graph.Edge) *edgeEnds {
	if ee, ok := c.ends[string(e.ID)]; ok {
		return ee
	}
	ee := c.checkEnds(s, e)
	c.ends[string(e.ID)] = ee
	return ee
}

func (c *Context) checkEnds(s *graph.Scope, e *graph.Edge) *edgeEnds {
	from, to := string(e.From.Node), string(e.To.Node)
	ee := &edgeEnds{at: Loc{Scope: s.ID(), Node: to, Port: e.To.Port}}
	src, okS := c.G.Node(from)
	dst, okD := c.G.Node(to)
	switch {
	case !okS:
		ee.at = Loc{Scope: s.ID(), Node: to, Port: e.To.Port}
		ee.reason = "源节点 " + from + " 不存在"
		return ee
	case !okD:
		ee.at = Loc{Scope: s.ID(), Node: from, Port: e.From.Port}
		ee.reason = "目标节点 " + to + " 不存在"
		return ee
	case !s.Has(from) || !s.Has(to):
		ee.reason = "两端不在同一个子图里"
		return ee
	}
	sdef, ok1 := c.V.NodeDef(string(src.Type))
	ddef, ok2 := c.V.NodeDef(string(dst.Type))
	if !ok1 || !ok2 {
		return ee
	}
	sp, dp := c.nodePortsOf(src, sdef), c.nodePortsOf(dst, ddef)
	if !sp.open {
		if _, ok := sp.out[e.From.Port]; !ok {
			ee.at = Loc{Scope: s.ID(), Node: from, Port: e.From.Port}
			if _, isIn := sp.in[e.From.Port]; isIn {
				ee.reason = "「" + label(src) + "」的「" + e.From.Port + "」是输入端口，不能从它连出"
			} else {
				ee.reason = "「" + label(src) + "」没有输出端口「" + e.From.Port + "」"
			}
			return ee
		}
	}
	port := e.To.Port
	var pd *gen.PortDef
	if m := slotRe.FindStringSubmatch(port); m != nil {
		port = m[1]
		pd = dp.in[port]
		if !dp.open && (pd == nil || pd.Dynamic == nil || !*pd.Dynamic) {
			ee.reason = "「" + label(dst) + "」没有动态端口「" + port + "」"
			return ee
		}
	} else {
		pd = dp.in[port]
		if !dp.open && pd == nil {
			if _, isOut := dp.out[port]; isOut {
				ee.reason = "「" + label(dst) + "」的「" + port + "」是输出端口，不能连进去"
			} else {
				ee.reason = "「" + label(dst) + "」没有输入端口「" + port + "」"
			}
			return ee
		}
	}
	ee.ok, ee.dstDef, ee.dstPD = true, ddef, pd
	ee.srcType = c.portType(src, sdef, sp.out[e.From.Port], e.From.Port, DirOut)
	ee.dstType = c.portType(dst, ddef, pd, port, DirIn)
	return ee
}

// portType 是端口推断后的类型：子图引用这类节点取 ContainerPorts 的类型，其余取推断结果。
func (c *Context) portType(n *graph.Node, def *gen.NodeDef, pd *gen.PortDef, port, dir string) *Type {
	if derived(def) {
		if pd == nil {
			return nil
		}
		t, err := Parse(string(pd.Type))
		if err != nil {
			return nil
		}
		return t
	}
	if c.Types == nil {
		return nil
	}
	return c.Types.Port(string(n.ID), port, dir)
}

func kernelLocked(pd *gen.PortDef) bool { return pd != nil && pd.Lock != nil && *pd.Lock == "kernel" }

type assignRes2 struct {
	a  gen.Assign
	ok bool // 做了判定（两端都对、类型都有）
}

// assign 判一条连线的类型（C.04.02，V06 / V14 共用）：对每个源头节点各判一次（C2 看源头节点的类型，P.09 K258 ④），
// 有一次不通过就返回那一次；源头找不到时不做 C2。
func (c *Context) assign(s *graph.Scope, e *graph.Edge) *assignRes2 {
	if r, ok := c.assigns[string(e.ID)]; ok {
		return r
	}
	r := &assignRes2{}
	c.assigns[string(e.ID)] = r
	ee := c.edgeEnds(s, e)
	if !ee.ok || ee.srcType == nil || ee.dstType == nil {
		return r
	}
	at := AssignAt{DstContainer: containerOf(ee.dstDef) != "none"}
	srcs := c.srcNodes(string(e.From.Node), e.From.Port, 0)
	if len(srcs) == 0 {
		if ee.dstType.Kind == Named {
			if d, ok := c.V.Type(ee.dstType.key()); ok && d.Producer != nil {
				at.SrcNode = string(*d.Producer) // 没有源头：C2 不判，只比类型
			}
		}
		r.a, r.ok = Assignable(ee.srcType, ee.dstType, at, c.V), true
		return r
	}
	for i, n := range srcs {
		at.SrcNode = string(n.Type)
		a := Assignable(ee.srcType, ee.dstType, at, c.V)
		if i == 0 || !a.Ok {
			r.a, r.ok = a, true
		}
		if !a.Ok {
			break
		}
	}
	return r
}

// ---- 参数（V03、V04 共用）----

type paramKey struct {
	def  *gen.NodeDef
	name string
}

// paramSchemas 缓存编译好的参数 schema：注册表里的定义只读，按定义的指针和参数名做键。
var (
	paramMu      sync.Mutex
	paramSchemas = map[paramKey]*jsonschema.Schema{}
)

// paramSchema 是按 ParamDef 生成的 JSON Schema（V03）。object 没写 schema 时接受任意 JSON（P.09 K258 ⑥）。
func paramSchema(def *gen.NodeDef, name string, pd gen.ParamDef) (*jsonschema.Schema, error) {
	paramMu.Lock()
	defer paramMu.Unlock()
	k := paramKey{def, name}
	if s, ok := paramSchemas[k]; ok {
		return s, nil
	}
	doc := map[string]any{}
	switch pd.Type {
	case "integer", "number":
		doc["type"] = pd.Type
		if pd.Min != nil {
			doc["minimum"] = *pd.Min
		}
		if pd.Max != nil {
			doc["maximum"] = *pd.Max
		}
	case "boolean":
		doc["type"] = "boolean"
	case "object":
		if pd.Schema != nil {
			b, err := json.Marshal(*pd.Schema)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(b, &doc); err != nil {
				return nil, err
			}
		}
	case "enum":
	default: // string、text、expr、各种 *_ref
		doc["type"] = "string"
		if pd.MaxLength != nil {
			doc["maxLength"] = *pd.MaxLength
		}
	}
	if pd.Enum != nil {
		vals := make([]any, 0, len(*pd.Enum))
		for _, raw := range *pd.Enum {
			var x any
			if err := json.Unmarshal(raw, &x); err != nil {
				return nil, err
			}
			vals = append(vals, x)
		}
		doc["enum"] = vals
	}
	b, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	const url = "https://anydoor.local/param.schema.json"
	comp := jsonschema.NewCompiler()
	if err := comp.AddResource(url, inst); err != nil {
		return nil, err
	}
	s, err := comp.Compile(url)
	if err != nil {
		return nil, err
	}
	paramSchemas[k] = s
	return s, nil
}

// validateJSON 用 schema 校验一段 JSON；不通过时返回第一处原因。
func validateJSON(sch *jsonschema.Schema, raw []byte) string {
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return "不是合法的 JSON"
	}
	if err := sch.Validate(inst); err != nil {
		var ve *jsonschema.ValidationError
		if errors.As(err, &ve) {
			loc, msg := firstLeaf(ve)
			if p := pointer(loc); p != "" {
				return p + "：" + msg
			}
			return msg
		}
		return err.Error()
	}
	return ""
}

func sameJSON(a, b json.RawMessage) bool {
	if len(a) == 0 || len(b) == 0 {
		return len(a) == len(b)
	}
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}

// hasValue：参数（节点上写了的，或缺省值）不是空：没有、null、"" 都算空。
func hasValue(n *graph.Node, def *gen.NodeDef, name string) bool {
	raw := paramRaw(n, def, name)
	if len(raw) == 0 {
		return false
	}
	var x any
	if json.Unmarshal(raw, &x) != nil || x == nil {
		return false
	}
	s, isStr := x.(string)
	return !isStr || s != ""
}
