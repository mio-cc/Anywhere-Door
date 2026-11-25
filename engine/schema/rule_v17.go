package schema

import "github.com/mio-cc/anywhere-door/gen"

// V17 孤立节点：所在子图里一条连线都没有的节点；类别为 边界 的节点不算。
var ruleV17 = rule{code: gen.DiagV17, local: true, check: func(c *Context) []gen.Diag {
	var out []gen.Diag
	for _, na := range c.nodes() {
		if na.def == nil || na.def.Category == "边界" {
			continue
		}
		id := string(na.n.ID)
		if len(na.s.InAll(id)) == 0 && len(na.s.OutAll(id)) == 0 {
			out = append(out, c.diag(gen.DiagV17, gen.DiagLevelWarn, Loc{Scope: na.s.ID(), Node: id}, map[string]any{"label": label(na.n)}))
		}
	}
	return out
}}
