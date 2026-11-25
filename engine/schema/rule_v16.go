package schema

import "github.com/mio-cc/anywhere-door/gen"

// V16 类型变量能推断：来自 Infer（E.03.02）。
var ruleV16 = rule{code: gen.DiagV16, check: func(c *Context) []gen.Diag {
	return append([]gen.Diag(nil), c.inferDiags...)
}}
