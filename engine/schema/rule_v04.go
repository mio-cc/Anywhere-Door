package schema

import "github.com/mio-cc/anywhere-door/gen"

// V04 锁定参数没有被改：定义里 lock 为 def / kernel 的参数，节点上写了、又和缺省值不同。
var ruleV04 = rule{code: gen.DiagV04, local: true, check: func(c *Context) []gen.Diag {
	var out []gen.Diag
	for _, na := range c.nodes() {
		if na.def == nil || na.n.Params == nil {
			continue
		}
		for _, name := range sortedKeys(na.def.Params) {
			pd := na.def.Params[name]
			if pd.Lock == nil || (*pd.Lock != "def" && *pd.Lock != "kernel") {
				continue
			}
			raw, ok := (*na.n.Params)[name]
			if !ok || sameJSON(raw, pd.Default) {
				continue
			}
			lbl := pd.Label
			if lbl == "" {
				lbl = name
			}
			out = append(out, c.diag(gen.DiagV04, gen.DiagLevelError, Loc{Scope: na.s.ID(), Node: string(na.n.ID), Param: name}, map[string]any{"param": lbl}))
		}
	}
	return out
}}
