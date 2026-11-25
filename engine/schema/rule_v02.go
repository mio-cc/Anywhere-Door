package schema

import "github.com/mio-cc/anywhere-door/gen"

// V02 节点类型已注册、扩展包已安装。
var ruleV02 = rule{code: gen.DiagV02, local: true, check: func(c *Context) []gen.Diag {
	var out []gen.Diag
	for _, na := range c.nodes() {
		if na.def == nil {
			out = append(out, c.diag(gen.DiagV02, gen.DiagLevelError, Loc{Scope: na.s.ID(), Node: string(na.n.ID)}, map[string]any{"type": string(na.n.Type)}))
		}
	}
	return out
}}
