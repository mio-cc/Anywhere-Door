package schema

import (
	"encoding/json"
	"strings"

	"github.com/expr-lang/expr"

	"github.com/mio-cc/anywhere-door/gen"
)

// V03 参数符合 ParamDef：没有这个参数、值不符合按 ParamDef 生成的 JSON Schema、expr 编译不过、
// type_ref 写法不对或类型没注册、配方 / 角色卡不存在。空字符串当作没写。
var ruleV03 = rule{code: gen.DiagV03, local: true, check: func(c *Context) []gen.Diag {
	var out []gen.Diag
	for _, na := range c.nodes() {
		if na.def == nil || na.n.Params == nil {
			continue
		}
		params := *na.n.Params
		for _, name := range sortedKeys(params) {
			reason := ""
			pd, ok := na.def.Params[name]
			if ok {
				reason = c.paramProblem(na.def, name, pd, params[name])
			} else {
				reason = "节点「" + label(na.n) + "」没有这个参数"
			}
			if reason != "" {
				lbl := name
				if ok && pd.Label != "" {
					lbl = pd.Label
				}
				out = append(out, c.diag(gen.DiagV03, gen.DiagLevelError, Loc{Scope: na.s.ID(), Node: string(na.n.ID), Param: name},
					map[string]any{"param": lbl, "reason": reason}))
			}
		}
	}
	return out
}}

func (c *Context) paramProblem(def *gen.NodeDef, name string, pd gen.ParamDef, raw json.RawMessage) string {
	sch, err := paramSchema(def, name, pd)
	if err != nil {
		return "参数定义编译不了：" + err.Error()
	}
	if r := validateJSON(sch, raw); r != "" {
		return r
	}
	var s string
	if json.Unmarshal(raw, &s) != nil || s == "" {
		return ""
	}
	switch pd.Type {
	case "expr":
		if _, err := expr.Compile(s); err != nil {
			msg, _, _ := strings.Cut(err.Error(), "\n")
			return "表达式写法不对：" + msg
		}
	case "type_ref":
		t, err := Parse(s)
		if err != nil {
			return "类型写法不对：" + s
		}
		if n := c.unregistered(t); n != "" {
			return "类型「" + n + "」没有注册"
		}
	case "recipe_ref":
		if _, ok := c.V.Recipe(s); !ok {
			return "配方「" + s + "」不存在"
		}
	case "rolecard_ref":
		if _, ok := c.V.RoleCard(strings.TrimSuffix(s, "@latest")); !ok {
			return "角色卡「" + s + "」不存在"
		}
	}
	return ""
}

// unregistered 是类型里第一个没有注册的命名类型；都注册了为 ""。
func (c *Context) unregistered(t *Type) string {
	switch t.Kind {
	case List:
		return c.unregistered(t.Elem)
	case Named:
		if _, ok := c.V.Type(t.key()); !ok {
			return t.String()
		}
	}
	return ""
}
