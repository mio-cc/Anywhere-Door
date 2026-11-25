package schema

import "github.com/mio-cc/anywhere-door/gen"

// V05 连线的端口存在、方向正确、两端在同一子图（C.02.03）；子图引用的端口按 ContainerPorts，动态端口写作 名字[N]。
var ruleV05 = rule{code: gen.DiagV05, local: true, check: func(c *Context) []gen.Diag {
	var out []gen.Diag
	for _, s := range c.scopes() {
		for _, e := range s.Edges() {
			if ee := c.edgeEnds(s, e); !ee.ok && ee.reason != "" {
				out = append(out, c.diag(gen.DiagV05, gen.DiagLevelError, ee.at, map[string]any{"reason": ee.reason}))
			}
		}
	}
	return out
}}
