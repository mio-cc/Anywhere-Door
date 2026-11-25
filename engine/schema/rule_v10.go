package schema

import (
	"encoding/json"

	"github.com/mio-cc/anywhere-door/gen"
)

// V10 循环至少有一个有限出口：定义里有 max_rounds 参数的节点，按轮次（trigger 不是 event）时 max_rounds 是正整数；
// 按事件时 idle_timeout_ms、exits 至少有一个（按定义的参数判断，不比较类型名）。
var ruleV10 = rule{code: gen.DiagV10, local: true, check: func(c *Context) []gen.Diag {
	var out []gen.Diag
	for _, na := range c.nodes() {
		if na.def == nil {
			continue
		}
		if _, ok := na.def.Params["max_rounds"]; !ok {
			continue
		}
		ok := false
		if trig, _ := paramString(na.n, na.def, "trigger"); trig == "event" {
			ok = hasValue(na.n, na.def, "idle_timeout_ms") || hasValue(na.n, na.def, "exits")
		} else {
			var n float64
			ok = json.Unmarshal(paramRaw(na.n, na.def, "max_rounds"), &n) == nil && n >= 1
		}
		if !ok {
			out = append(out, c.diag(gen.DiagV10, gen.DiagLevelError, Loc{Scope: na.s.ID(), Node: string(na.n.ID)}, map[string]any{"label": label(na.n)}))
		}
	}
	return out
}}
