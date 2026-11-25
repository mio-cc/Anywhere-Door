package schema

import (
	"sort"

	"github.com/mio-cc/anywhere-door/gen"
)

// V09 环只能在循环节点内部形成（INV-06）：每个子图的强连通分量非空即报，报在分量里 ID 最小的节点上。
var ruleV09 = rule{code: gen.DiagV09, check: func(c *Context) []gen.Diag {
	var out []gen.Diag
	for _, s := range c.scopes() {
		for _, comp := range s.SCCs() {
			ids := append([]string(nil), comp...)
			sort.Strings(ids)
			out = append(out, c.diag(gen.DiagV09, gen.DiagLevelError, Loc{Scope: s.ID(), Node: ids[0]}, nil))
		}
	}
	return out
}}
