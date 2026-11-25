package schema

import "github.com/mio-cc/anywhere-door/gen"

// V12 嵌套深度（C.02.04 §6，根图为 0）：第一次超过 4 层的子图报 warn，第一次超过 8 层的报 error，报在容器节点上。
// 子图引用成环要等子图库（M2，E.14.13）。
var ruleV12 = rule{code: gen.DiagV12, check: func(c *Context) []gen.Diag {
	var out []gen.Diag
	for _, s := range c.scopes() {
		d := c.G.Depth(s.ID())
		parent := c.G.Parent(s.ID())
		if parent == nil || (d != 5 && d != 9) {
			continue
		}
		at := Loc{Scope: c.G.ScopeOf(string(parent.ID)).ID(), Node: string(parent.ID)}
		if d == 5 {
			out = append(out, c.diag(gen.DiagV12, gen.DiagLevelWarn, at, map[string]any{"depth": d}))
		} else {
			out = append(out, c.diag(gen.DiagV12, gen.DiagLevelError, at, map[string]any{"depth": d, "chain": "无"}))
		}
	}
	return out
}}
