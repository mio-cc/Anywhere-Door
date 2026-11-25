package schema

import (
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/mio-cc/anywhere-door/engine/graph"
	"github.com/mio-cc/anywhere-door/gen"
)

// Options 是注册表的选项。
type Options struct{ Logger *slog.Logger }

// Package 是扩展包里已经解析好的内容（E.04 负责读文件）。
type Package struct {
	Name, Version string
	NodeDefs      []*gen.NodeDef
	Types         []*gen.TypeDef
	Recipes       []*gen.Recipe
	RoleCards     []*gen.Rolecard
}

// Registry 是节点定义、类型、配方、角色卡的注册表（E.03.01）。每次修改都在副本上做，成功后整体替换：
// 已经发出去的 Snapshot 永远不变。
type Registry struct {
	mu  sync.RWMutex
	log *slog.Logger
	cur *state
}

// NewRegistry 建一个空的注册表；先 LoadBuiltin 再用。
func NewRegistry(o Options) *Registry {
	log := o.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	st := newState()
	st.hash = st.computeHash()
	return &Registry{log: log, cur: st}
}

func (r *Registry) state() *state {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cur
}

// Snapshot 是此刻注册表的只读视图；之后安装、卸载都不影响它（冻结运行时用）。
func (r *Registry) Snapshot() View { return r.state() }

// Live 是始终反映最新内容的视图（编辑态用）。
func (r *Registry) Live() View { return live{r} }

// Install 安装扩展包（E.04 调用）。不能覆盖内置项和别的包的项（C.13-R3）；失败时注册表不变，返回 Errors。
func (r *Registry) Install(p *Package) error {
	if p == nil || p.Name == "" {
		return Errors{{Msg: "扩展包没有名字"}}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cur.packages[p.Name] {
		return Errors{{Item: p.Name, Msg: "扩展包已经安装"}}
	}
	st := r.cur.clone()
	st.packages[p.Name] = true
	var errs Errors
	add := func(item string, err error) {
		if err != nil {
			errs = append(errs, &RegistryError{Item: item, Msg: err.Error()})
		}
	}
	for _, t := range p.Types {
		add(t.Name, st.addType(p.Name, t))
	}
	for _, c := range p.Recipes {
		add(c.Name, st.addRecipe(p.Name, c))
	}
	for _, c := range p.RoleCards {
		add(c.Name, st.addRoleCard(p.Name, c))
	}
	for _, d := range p.NodeDefs {
		add(d.Name, st.addNodeDef(p.Name, d))
	}
	errs = append(errs, st.link()...)
	if len(errs) > 0 {
		return errs
	}
	r.cur = st
	r.log.Info("扩展包已安装", "package", p.Name, "version", p.Version, "hash", st.hash)
	return nil
}

// Uninstall 卸载扩展包；内置项不能卸载。
func (r *Registry) Uninstall(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if name == builtinOwner {
		return Errors{{Msg: "内置项不能卸载"}}
	}
	if !r.cur.packages[name] {
		return Errors{{Item: name, Msg: "没有安装这个扩展包"}}
	}
	st := r.cur.clone()
	st.remove(name)
	if errs := st.link(); len(errs) > 0 {
		return errs
	}
	r.cur = st
	r.log.Info("扩展包已卸载", "package", name, "hash", st.hash)
	return nil
}

// RegistryError 是一条注册错误：哪个文件第几行、哪一项、什么问题。
type RegistryError struct {
	File string
	Line int
	Item string
	Msg  string
}

func (e *RegistryError) Error() string {
	var b strings.Builder
	b.WriteString("schema: ")
	if e.File != "" {
		b.WriteString(e.File)
		if e.Line > 0 {
			fmt.Fprintf(&b, " 第 %d 行", e.Line)
		}
		b.WriteString("：")
	}
	if e.Item != "" {
		b.WriteString("「" + e.Item + "」")
	}
	b.WriteString(e.Msg)
	return b.String()
}

// Errors 是一组注册错误（LoadBuiltin、Install 返回全部，不止第一条）。
type Errors []*RegistryError

func (es Errors) Error() string {
	parts := make([]string, len(es))
	for i, e := range es {
		parts[i] = e.Error()
	}
	return strings.Join(parts, "\n")
}

const builtinOwner = ""

// state 是注册表的一版内容；发布后不再修改。
type state struct {
	nodedefs    map[string][]*gen.NodeDef // 查找键 → 按次版本升序
	types       map[string][]*gen.TypeDef // 名称@主版本 → 按次版本升序
	recipes     map[string]*gen.Recipe    // 名称@版本
	rolecards   map[string]*gen.Rolecard  // 名称@版本
	owner       map[string]string         // 种类:名称 → 包名（内置为 ""）
	packages    map[string]bool
	docs        map[string]*jsonschema.Schema
	typeSchemas map[string]*jsonschema.Schema // 名称@主版本
	msgs        *Messages
	hash        string
}

func newState() *state {
	return &state{
		nodedefs: map[string][]*gen.NodeDef{}, types: map[string][]*gen.TypeDef{},
		recipes: map[string]*gen.Recipe{}, rolecards: map[string]*gen.Rolecard{},
		owner: map[string]string{}, packages: map[string]bool{},
		docs: map[string]*jsonschema.Schema{}, typeSchemas: map[string]*jsonschema.Schema{},
		msgs: &Messages{log: slog.New(slog.DiscardHandler)},
	}
}

func cloneMap[V any](m map[string]V) map[string]V {
	out := make(map[string]V, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func cloneLists[V any](m map[string][]V) map[string][]V {
	out := make(map[string][]V, len(m))
	for k, v := range m {
		out[k] = append([]V(nil), v...)
	}
	return out
}

func (st *state) clone() *state {
	return &state{
		nodedefs: cloneLists(st.nodedefs), types: cloneLists(st.types),
		recipes: cloneMap(st.recipes), rolecards: cloneMap(st.rolecards),
		owner: cloneMap(st.owner), packages: cloneMap(st.packages),
		docs: st.docs, typeSchemas: cloneMap(st.typeSchemas), msgs: st.msgs,
	}
}

// remove 删掉某个包的全部项。
func (st *state) remove(pkg string) {
	delete(st.packages, pkg)
	for k, o := range st.owner {
		if o != pkg {
			continue
		}
		delete(st.owner, k)
		kind, name, _ := strings.Cut(k, ":")
		switch kind {
		case "node":
			for key, l := range st.nodedefs {
				if len(l) > 0 && l[0].Name == name && strings.HasPrefix(key, pkg+"/") {
					delete(st.nodedefs, key)
				}
			}
		case "type":
			for key, l := range st.types {
				if len(l) > 0 && l[0].Name == name {
					delete(st.types, key)
					delete(st.typeSchemas, key)
				}
			}
		case "recipe":
			for key, c := range st.recipes {
				if c.Name == name {
					delete(st.recipes, key)
				}
			}
		case "role":
			for key, c := range st.rolecards {
				if c.Name == name {
					delete(st.rolecards, key)
				}
			}
		}
	}
}

// claim 记下某个名字归谁；已经归别人时返回错误。
func (st *state) claim(kind, name, owner string) error {
	k := kind + ":" + name
	if o, ok := st.owner[k]; ok && o != owner {
		who := "内置"
		if o != builtinOwner {
			who = "扩展包「" + o + "」"
		}
		return fmt.Errorf("已经由%s声明，不能覆盖", who)
	}
	st.owner[k] = owner
	return nil
}

func majorOf(version string) string {
	m, _, _ := strings.Cut(version, ".")
	return m
}

func nodeKey(owner string, d *gen.NodeDef) string {
	if owner == builtinOwner {
		return d.Name
	}
	return owner + "/" + d.Name + "@" + majorOf(d.Version)
}

// lookupKey 把 "Agent"、"pack/节点@1"、"pack/节点@1.2" 换成查找键（名称@主版本）。
func lookupKey(ref string) string {
	name, ver, ok := strings.Cut(ref, "@")
	if !ok {
		return ref
	}
	return name + "@" + majorOf(ver)
}

func minorOf(version string) int {
	_, m, _ := strings.Cut(version, ".")
	n, _ := strconv.Atoi(m)
	return n
}

func (st *state) addNodeDef(owner string, d *gen.NodeDef) error {
	if d == nil {
		return fmt.Errorf("节点定义为空")
	}
	if owner != builtinOwner && d.Kind == "atomic" {
		return fmt.Errorf("扩展包不能声明原子节点（kind=atomic）")
	}
	if err := checkNodeDef(d); err != nil {
		return err
	}
	if err := st.claim("node", d.Name, owner); err != nil {
		return err
	}
	key := nodeKey(owner, d)
	l := st.nodedefs[key]
	for _, x := range l {
		if x.Version == d.Version {
			return fmt.Errorf("版本 %s 重复", d.Version)
		}
	}
	l = append(l, d)
	sort.Slice(l, func(i, j int) bool { return minorOf(l[i].Version) < minorOf(l[j].Version) })
	st.nodedefs[key] = l
	return nil
}

func typeKey(name string, major int64) string { return name + "@" + strconv.FormatInt(major, 10) }

func (st *state) addType(owner string, t *gen.TypeDef) error {
	if t == nil {
		return fmt.Errorf("类型为空")
	}
	if owner != builtinOwner && t.System != nil && *t.System {
		return fmt.Errorf("扩展包不能声明系统类型（C.13.03-R2）")
	}
	if err := st.claim("type", t.Name, owner); err != nil {
		return err
	}
	key := typeKey(t.Name, t.Major)
	l := st.types[key]
	var prev, next *gen.TypeDef
	for _, x := range l {
		switch {
		case x.Minor == t.Minor:
			return fmt.Errorf("版本 %d.%d 重复", t.Major, t.Minor)
		case x.Minor < t.Minor:
			prev = x
		case next == nil:
			next = x
		}
	}
	if prev != nil {
		if err := checkMinor(prev, t); err != nil {
			return err
		}
	}
	if next != nil {
		if err := checkMinor(t, next); err != nil {
			return err
		}
	}
	l = append(l, t)
	sort.Slice(l, func(i, j int) bool { return l[i].Minor < l[j].Minor })
	st.types[key] = l
	return nil
}

// checkMinor：新次版本只能新增可选字段，旧字段原样保留（C.13.03-R1）。
func checkMinor(old, cur *gen.TypeDef) error {
	have := map[string]string{}
	if cur.Fields != nil {
		for _, f := range *cur.Fields {
			b, _ := graph.Canonical(f)
			have[f.Name] = string(b)
		}
	}
	oldNames := map[string]bool{}
	if old.Fields != nil {
		for _, f := range *old.Fields {
			oldNames[f.Name] = true
			b, _ := graph.Canonical(f)
			if have[f.Name] != string(b) {
				return fmt.Errorf("%d.%d 改了或删了 %d.%d 的字段 %s：只能新增可选字段，否则升主版本（C.13.03-R1）", cur.Major, cur.Minor, old.Major, old.Minor, f.Name)
			}
		}
	}
	if cur.Fields != nil {
		for _, f := range *cur.Fields {
			if !oldNames[f.Name] && f.Required != nil && *f.Required {
				return fmt.Errorf("%d.%d 新增了必填字段 %s：次版本只能新增可选字段（C.13.03-R1）", cur.Major, cur.Minor, f.Name)
			}
		}
	}
	return nil
}

func (st *state) addRecipe(owner string, c *gen.Recipe) error {
	if c == nil {
		return fmt.Errorf("配方为空")
	}
	if err := st.claim("recipe", c.Name, owner); err != nil {
		return err
	}
	key := c.Name + "@" + strconv.FormatInt(c.Version, 10)
	if _, dup := st.recipes[key]; dup {
		return fmt.Errorf("版本 %d 重复", c.Version)
	}
	st.recipes[key] = c
	return nil
}

func (st *state) addRoleCard(owner string, c *gen.Rolecard) error {
	if c == nil {
		return fmt.Errorf("角色卡为空")
	}
	if err := st.claim("role", c.Name, owner); err != nil {
		return err
	}
	key := c.Name + "@" + strconv.FormatInt(c.Version, 10)
	if _, dup := st.rolecards[key]; dup {
		return fmt.Errorf("版本 %d 重复", c.Version)
	}
	st.rolecards[key] = c
	return nil
}

// link 做跨项的检查（引用的类型已注册、角色卡继承），生成类型的 JSON Schema，算哈希。
func (st *state) link() Errors {
	var errs Errors
	keys := sortedKeys(st.nodedefs)
	for _, k := range keys {
		for _, d := range st.nodedefs[k] {
			for _, msg := range st.checkNodeDefTypes(d) {
				errs = append(errs, &RegistryError{Item: d.Name, Msg: msg})
			}
		}
	}
	for _, k := range sortedKeys(st.rolecards) {
		if msg := st.checkExtends(st.rolecards[k]); msg != "" {
			errs = append(errs, &RegistryError{Item: k, Msg: msg})
		}
	}
	errs = append(errs, st.compileTypes()...)
	st.hash = st.computeHash()
	return errs
}

func (st *state) checkExtends(c *gen.Rolecard) string {
	seen := map[string]bool{c.Name: true}
	for cur := c; cur.Extends != nil && *cur.Extends != ""; {
		parent, ok := st.RoleCard(*cur.Extends)
		if !ok {
			return "继承的父角色卡「" + *cur.Extends + "」没有注册"
		}
		if seen[parent.Name] {
			return "角色卡的继承链成环：" + *cur.Extends
		}
		seen[parent.Name] = true
		cur = parent
	}
	return ""
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// computeHash：所有项按"种类 + 名称 + 版本"排序，逐个规范化序列化后拼接，算 sha256（E.03.01 §5）。
func (st *state) computeHash() string {
	type ent struct {
		k string
		v any
	}
	var es []ent
	for _, l := range st.nodedefs {
		for _, d := range l {
			es = append(es, ent{"1node:" + nodeOwnerKey(st, d), d})
		}
	}
	for _, l := range st.types {
		for _, t := range l {
			es = append(es, ent{fmt.Sprintf("2type:%s@%d.%d", t.Name, t.Major, t.Minor), t})
		}
	}
	for k, c := range st.recipes {
		es = append(es, ent{"3recipe:" + k, c})
	}
	for k, c := range st.rolecards {
		es = append(es, ent{"4role:" + k, c})
	}
	sort.Slice(es, func(i, j int) bool { return es[i].k < es[j].k })
	var buf []byte
	for _, e := range es {
		b, err := graph.Canonical(e.v)
		if err != nil {
			continue
		}
		buf = append(buf, e.k...)
		buf = append(buf, 0)
		buf = append(buf, b...)
		buf = append(buf, 0)
	}
	return graph.HashBytes(buf)
}

func nodeOwnerKey(st *state, d *gen.NodeDef) string {
	return st.owner["node:"+d.Name] + "/" + d.Name + "@" + d.Version
}
