package schema

import "github.com/mio-cc/anywhere-door/gen"

// V08 必填输入已连，或绑定这个端口的参数（ui.bind）有值。
var ruleV08 = rule{code: gen.DiagV08, local: true, check: func(c *Context) []gen.Diag {
	var out []gen.Diag
	for _, na := range c.nodes() {
		if na.def == nil {
			continue
		}
		np := c.nodePortsOf(na.n, na.def)
		if np.open {
			continue
		}
		id := string(na.n.ID)
		for _, name := range sortedKeys(np.in) {
			pd := np.in[name]
			if pd.Required == nil || !*pd.Required || len(na.s.In(id, name)) > 0 || boundSet(na, name) {
				continue
			}
			lbl := name
			if pd.Label != nil && *pd.Label != "" {
				lbl = *pd.Label
			}
			out = append(out, c.diag(gen.DiagV08, gen.DiagLevelError, Loc{Scope: na.s.ID(), Node: id, Port: name}, map[string]any{"port": lbl}))
		}
	}
	return out
}}

// boundSet：有参数绑定这个端口（ui.bind，C.03.05-R2），而且节点上写了值。
func boundSet(na nodeAt, port string) bool {
	if na.n.Params == nil {
		return false
	}
	for name, p := range na.def.Params {
		if p.UI == nil || p.UI.Bind == nil || *p.UI.Bind != port {
			continue
		}
		if _, ok := (*na.n.Params)[name]; ok && hasValue(na.n, na.def, name) {
			return true
		}
	}
	return false
}
