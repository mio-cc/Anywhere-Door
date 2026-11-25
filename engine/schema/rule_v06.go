package schema

import "github.com/mio-cc/anywhere-door/gen"

// V06 类型兼容（Assignable）。目标端口是内核锁定的（Agent 的 context，INV-07）、按 C2 被拒的交给 V14（P.09 K259）。
var ruleV06 = rule{code: gen.DiagV06, check: func(c *Context) []gen.Diag {
	var out []gen.Diag
	for _, s := range c.scopes() {
		for _, e := range s.Edges() {
			r := c.assign(s, e)
			if !r.ok || r.a.Ok {
				continue
			}
			ee := c.edgeEnds(s, e)
			if r.a.Rule == "C2" && kernelLocked(ee.dstPD) {
				continue
			}
			out = append(out, c.diag(gen.DiagV06, gen.DiagLevelError, ee.at,
				map[string]any{"src": ee.srcType.String(), "dst": ee.dstType.String(), "reason": str(r.a.Reason)}))
		}
	}
	return out
}}
