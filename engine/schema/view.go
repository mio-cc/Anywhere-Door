package schema

import (
	"sort"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/mio-cc/anywhere-door/gen"
)

// TypeSource 是查类型和文案的最小接口（连线判定只需要它，E.03.02）。
type TypeSource interface {
	Type(ref string) (*gen.TypeDef, bool)
	Messages() *Messages
}

// View 是注册表的只读视图。返回的定义只读，调用方不能修改。
type View interface {
	TypeSource
	NodeDef(typeRef string) (*gen.NodeDef, bool)      // "Agent" / "pack/节点@1"（主版本 → 已安装的最高次版本）
	NodeDefs() []*gen.NodeDef                         // 按类型名排序
	TypeSchema(ref string) (*jsonschema.Schema, bool) // 类型对应的 JSON Schema（C.13.03-R3）
	DocSchema(name string) (*jsonschema.Schema, bool) // 编译好的 schema/*.schema.json，如 "graph.schema.json"
	Recipe(ref string) (*gen.Recipe, bool)            // "执行者配方@1"；不写版本时取最高版本
	RoleCard(ref string) (*gen.Rolecard, bool)        // "智囊@1"；不写版本时取最高版本；原样返回，不合并继承
	Hash() string                                     // 注册表哈希（C.12 §2）
}

var _ View = (*state)(nil)

// NodeDef 按类型名找节点定义。
func (st *state) NodeDef(ref string) (*gen.NodeDef, bool) {
	l := st.nodedefs[lookupKey(ref)]
	if len(l) == 0 {
		return nil, false
	}
	return l[len(l)-1], true
}

// NodeDefs 是所有节点定义（每个查找键取最高次版本），按查找键排序。
func (st *state) NodeDefs() []*gen.NodeDef {
	keys := sortedKeys(st.nodedefs)
	out := make([]*gen.NodeDef, 0, len(keys))
	for _, k := range keys {
		l := st.nodedefs[k]
		out = append(out, l[len(l)-1])
	}
	return out
}

// splitNamed 把 "方案@1"、"方案@1.2" 拆成名称和主版本；不是这种写法时 ok 为 false。
func splitNamed(ref string) (name string, major int64, ok bool) {
	name, ver, found := strings.Cut(ref, "@")
	if !found || name == "" {
		return "", 0, false
	}
	m, err := strconv.ParseInt(majorOf(ver), 10, 64)
	if err != nil || m < 0 {
		return "", 0, false
	}
	return name, m, true
}

// Type 按 "名称@主版本[.次版本]" 找类型：取这个主版本已注册的最高次版本。
func (st *state) Type(ref string) (*gen.TypeDef, bool) {
	name, major, ok := splitNamed(ref)
	if !ok {
		return nil, false
	}
	l := st.types[typeKey(name, major)]
	if len(l) == 0 {
		return nil, false
	}
	return l[len(l)-1], true
}

// TypeSchema 是类型对应的 JSON Schema。
func (st *state) TypeSchema(ref string) (*jsonschema.Schema, bool) {
	name, major, ok := splitNamed(ref)
	if !ok {
		return nil, false
	}
	s, ok := st.typeSchemas[typeKey(name, major)]
	return s, ok
}

// DocSchema 是编译好的 schema/*.schema.json。
func (st *state) DocSchema(name string) (*jsonschema.Schema, bool) {
	s, ok := st.docs[name]
	return s, ok
}

// latest 按 "名称@版本" 或 "名称" 找；只写名称时取最高版本。
func latest[V any](m map[string]V, ref string, version func(V) int64) (V, bool) {
	if v, ok := m[ref]; ok {
		return v, true
	}
	var zero V
	if strings.Contains(ref, "@") {
		return zero, false
	}
	var cands []V
	for k, v := range m {
		if name, _, _ := strings.Cut(k, "@"); name == ref {
			cands = append(cands, v)
		}
	}
	if len(cands) == 0 {
		return zero, false
	}
	sort.Slice(cands, func(i, j int) bool { return version(cands[i]) < version(cands[j]) })
	return cands[len(cands)-1], true
}

// Recipe 找配方。
func (st *state) Recipe(ref string) (*gen.Recipe, bool) {
	return latest(st.recipes, ref, func(c *gen.Recipe) int64 { return c.Version })
}

// RoleCard 找角色卡。
func (st *state) RoleCard(ref string) (*gen.Rolecard, bool) {
	return latest(st.rolecards, ref, func(c *gen.Rolecard) int64 { return c.Version })
}

// Messages 是诊断和连线判定的文案。
func (st *state) Messages() *Messages { return st.msgs }

// Hash 是注册表哈希。
func (st *state) Hash() string { return st.hash }

// live 每次调用都读注册表的最新内容。
type live struct{ r *Registry }

func (l live) NodeDef(ref string) (*gen.NodeDef, bool)          { return l.r.state().NodeDef(ref) }
func (l live) NodeDefs() []*gen.NodeDef                         { return l.r.state().NodeDefs() }
func (l live) Type(ref string) (*gen.TypeDef, bool)             { return l.r.state().Type(ref) }
func (l live) TypeSchema(ref string) (*jsonschema.Schema, bool) { return l.r.state().TypeSchema(ref) }
func (l live) DocSchema(name string) (*jsonschema.Schema, bool) { return l.r.state().DocSchema(name) }
func (l live) Recipe(ref string) (*gen.Recipe, bool)            { return l.r.state().Recipe(ref) }
func (l live) RoleCard(ref string) (*gen.Rolecard, bool)        { return l.r.state().RoleCard(ref) }
func (l live) Messages() *Messages                              { return l.r.state().Messages() }
func (l live) Hash() string                                     { return l.r.state().Hash() }
