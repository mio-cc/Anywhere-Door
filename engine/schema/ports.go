package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/mio-cc/anywhere-door/engine/graph"
	"github.com/mio-cc/anywhere-door/gen"
)

// SchemaSource 由能提供"所选对象的入参 schema"的视图实现（如工具注册表，E.11）；Ports 用它展开 schema_from 参数。
// 视图没有实现时，schema_from 参数不生成端口。
type SchemaSource interface {
	ObjectSchema(ref string) (json.RawMessage, bool)
}

var (
	portNameRe = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)
	tmplVarRe  = regexp.MustCompile(`\{\{\s*([^{}]*?)\s*\}\}`)
)

// TemplateVars 是文本里的 {{变量}}，按出现顺序去重（含不合法的名字）。
func TemplateVars(s string) []string {
	var out []string
	for _, m := range tmplVarRe.FindAllStringSubmatch(s, -1) {
		if !slices.Contains(out, m[1]) {
			out = append(out, m[1])
		}
	}
	return out
}

// ValidPortName：端口名只能是 ^[a-z_][a-z0-9_]*$（C.02.03 §4）。
func ValidPortName(s string) bool { return portNameRe.MatchString(s) }

// paramRaw 是节点上参数的值；没写时用 NodeDef 的缺省值。
func paramRaw(n *graph.Node, def *gen.NodeDef, name string) json.RawMessage {
	if n != nil && n.Params != nil {
		if v, ok := (*n.Params)[name]; ok {
			return v
		}
	}
	if p, ok := def.Params[name]; ok {
		return p.Default
	}
	return nil
}

func paramString(n *graph.Node, def *gen.NodeDef, name string) (string, bool) {
	raw := paramRaw(n, def, name)
	var s string
	if len(raw) == 0 || json.Unmarshal(raw, &s) != nil {
		return "", false
	}
	return s, true
}

// Ports 是节点的全部端口：NodeDef 的静态端口加动态端口（C.03.02 §5、C.03.05-R4）。返回的 PortDef 是副本。
func Ports(n *graph.Node, def *gen.NodeDef, v View) (in, out map[string]*gen.PortDef) {
	in, out = map[string]*gen.PortDef{}, map[string]*gen.PortDef{}
	if def == nil {
		return in, out
	}
	for k, p := range def.Ports.Inputs {
		c := p
		in[k] = &c
	}
	for k, p := range def.Ports.Outputs {
		c := p
		out[k] = &c
	}
	dp := def.DynamicPorts
	if dp == nil {
		return in, out
	}
	pd, ok := def.Params[dp.FromParam]
	if !ok {
		return in, out
	}
	typ := gen.TypeRef("任意")
	if dp.Type != nil {
		typ = *dp.Type
	}
	limit := -1
	if dp.Max != nil {
		limit = int(*dp.Max)
	}
	add := func(name, label string) {
		if limit == 0 {
			return
		}
		if _, dup := in[name]; dup {
			return
		}
		l := label
		in[name] = &gen.PortDef{Type: typ, Label: &l}
		if limit > 0 {
			limit--
		}
	}
	switch {
	case pd.Type == "text" || pd.Type == "string":
		s, _ := paramString(n, def, dp.FromParam)
		for _, name := range TemplateVars(s) {
			if ValidPortName(name) {
				add(name, name)
			}
		}
	case pd.Type == "object" && pd.SchemaFrom != nil:
		ss, ok := v.(SchemaSource)
		if !ok {
			return in, out
		}
		src, arg, _ := strings.Cut(*pd.SchemaFrom, ":")
		if src != "param" {
			return in, out
		}
		ref, ok := paramString(n, def, arg)
		if !ok {
			return in, out
		}
		schema, ok := ss.ObjectSchema(ref)
		if !ok {
			return in, out
		}
		var reserved []string
		for k := range def.Ports.Inputs {
			reserved = append(reserved, k)
		}
		for k := range def.Ports.Outputs {
			reserved = append(reserved, k)
		}
		for k := range def.Params {
			reserved = append(reserved, k)
		}
		slices.Sort(reserved)
		if ok, params := ExpandSchema(schema, reserved); ok {
			typ = "任意" // schema_from 展开的端口一律 任意（C.03.05-R4）
			for _, p := range params {
				add(p.Port, p.Param.Label)
			}
		}
	}
	return in, out
}

// ExpandedParam 是 schema_from 对象参数展开出的一个子参数（C.03.02 §5）。
type ExpandedParam struct {
	Key   string       // 属性名，原样
	Param gen.ParamDef // 子参数
	Group string       // 常用 / 高级（R8）
	Port  string       // 动态输入端口名（R8）
}

// ExpandSchema 按 C.03.02 §5（R7、R8、R10）展开一个对象的 JSON Schema。reserved 是节点上已有的端口名和参数名。
// 不能展开时 ok 为 false（R10：退回整块的 object 参数）。
func ExpandSchema(schema json.RawMessage, reserved []string) (bool, []ExpandedParam) {
	_, top, ok := objectKeys(schema)
	if !ok {
		return false, nil
	}
	if raw, has := top["type"]; has {
		var typ string
		if json.Unmarshal(raw, &typ) != nil || typ != "object" {
			return false, nil
		}
	}
	keys, props, ok := objectKeys(top["properties"])
	if !ok || len(keys) > 32 {
		return false, nil
	}
	var required []string
	if raw, has := top["required"]; has {
		_ = json.Unmarshal(raw, &required)
	}
	taken := map[string]bool{}
	for _, r := range reserved {
		taken[r] = true
	}
	var out []ExpandedParam
	common := 0
	for _, k := range keys {
		var sub map[string]any
		_ = json.Unmarshal(props[k], &sub) // 布尔 schema 读不成对象，按没写 type 处理
		group := "高级"
		if slices.Contains(required, k) && common < 4 {
			group = "常用"
			common++
		}
		var pd gen.ParamDef
		b, _ := json.Marshal(expandParamDef(k, sub))
		_ = json.Unmarshal(b, &pd)
		out = append(out, ExpandedParam{Key: k, Group: group, Port: expandPortName(k, taken), Param: pd})
	}
	return true, out
}

// objectKeys 按出现顺序读一个 JSON 对象的键和值；不是对象时 ok 为 false。
func objectKeys(raw json.RawMessage) (keys []string, vals map[string]json.RawMessage, ok bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, nil, false
	}
	vals = map[string]json.RawMessage{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, nil, false
		}
		k, _ := tok.(string)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, nil, false
		}
		if _, dup := vals[k]; !dup {
			keys = append(keys, k)
		}
		vals[k] = v
	}
	return keys, vals, true
}

// expandParamDef 是 R7 的映射：一个属性的 JSON Schema → 子参数。
func expandParamDef(key string, sub map[string]any) map[string]any {
	sub = nullable(sub)
	p := map[string]any{"type": "object"}
	typ, _ := sub["type"].(string)
	for _, k := range []string{"oneOf", "anyOf", "allOf"} {
		if _, has := sub[k]; has {
			typ = ""
		}
	}
	num := func(from, to string) {
		if v, ok := sub[from].(float64); ok {
			p[to] = v
		}
	}
	switch typ {
	case "string":
		if enum, ok := sub["enum"].([]any); ok {
			p["type"], p["enum"] = "enum", enum
			break
		}
		p["type"] = "string"
		if ml, ok := sub["maxLength"].(float64); ok {
			if ml > 200 {
				p["type"] = "text"
			}
			p["max_length"] = ml
		}
	case "integer":
		p["type"], p["step"] = "integer", 1.0
		num("minimum", "min")
		num("maximum", "max")
		num("multipleOf", "step")
	case "number":
		p["type"] = "number"
		num("minimum", "min")
		num("maximum", "max")
		num("multipleOf", "step")
	case "boolean":
		p["type"] = "boolean"
	}
	label := key
	if title, ok := sub["title"].(string); ok && title != "" {
		label = title
	}
	p["label"] = truncRunes(label, 20)
	if d, ok := sub["description"].(string); ok && d != "" {
		p["help"] = truncRunes(d, 200)
	}
	if d, has := sub["default"]; has && d != nil {
		p["default"] = d
	}
	return p
}

// nullable 是 R7 的可空写法：去掉 null 后只剩一种类型时按那一种。
func nullable(sub map[string]any) map[string]any {
	if types, ok := sub["type"].([]any); ok {
		var rest []any
		for _, ty := range types {
			if ty != "null" {
				rest = append(rest, ty)
			}
		}
		if len(rest) != 1 {
			return sub
		}
		out := map[string]any{}
		for k, v := range sub {
			out[k] = v
		}
		out["type"] = rest[0]
		return out
	}
	for _, k := range []string{"anyOf", "oneOf"} {
		alts, ok := sub[k].([]any)
		if !ok || len(alts) != 2 {
			continue
		}
		for i, alt := range alts {
			null, _ := alt.(map[string]any)
			other, ok := alts[1-i].(map[string]any)
			if len(null) != 1 || null["type"] != "null" || !ok {
				continue
			}
			out := map[string]any{}
			for key, val := range other {
				out[key] = val
			}
			for _, f := range []string{"title", "description", "default"} {
				if v, has := sub[f]; has {
					out[f] = v
				}
			}
			return out
		}
	}
	return sub
}

func truncRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// expandPortName 是 R8 的端口名换算：A–Z 前加 _ 再转小写，其余不在 [a-z0-9_] 里的字符换成 _，数字开头加 p_，
// 和已有的名字重名时依次加 _2、_3。
func expandPortName(key string, taken map[string]bool) string {
	var b strings.Builder
	for _, r := range key {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteByte('_')
			b.WriteRune(r + ('a' - 'A'))
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	base := b.String()
	if base == "" || (base[0] >= '0' && base[0] <= '9') {
		base = "p_" + base
	}
	name := base
	for i := 2; taken[name]; i++ {
		name = fmt.Sprintf("%s_%d", base, i)
	}
	taken[name] = true
	return name
}
