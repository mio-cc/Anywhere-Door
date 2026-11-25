package schema

import "github.com/mio-cc/anywhere-door/gen"

// V14 Agent 的 context 只接上下文装配（INV-07，C2 的友好提示）：目标端口内核锁定（lock: kernel）、按 C2 被拒（源头节点不是
// 目标类型的 producer；源头沿子图边界回溯，P.09 K258 ④）。别的端口按 C2 被拒的报 V06（P.09 K259）。
var ruleV14 = rule{code: gen.DiagV14, check: func(c *Context) []gen.Diag {
	var out []gen.Diag
	for _, s := range c.scopes() {
		for _, e := range s.Edges() {
			r := c.assign(s, e)
			if !r.ok || r.a.Ok || r.a.Rule != "C2" {
				continue
			}
			if ee := c.edgeEnds(s, e); kernelLocked(ee.dstPD) {
				out = append(out, c.diag(gen.DiagV14, gen.DiagLevelError, ee.at, nil))
			}
		}
	}
	return out
}}
