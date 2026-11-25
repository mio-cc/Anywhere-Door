package schema

import "github.com/mio-cc/anywhere-door/gen"

// V07 非多连输入最多一条线（动态插口每个插口一条）；没有完全相同的两条连线。
var ruleV07 = rule{code: gen.DiagV07, local: true, check: func(c *Context) []gen.Diag {
	var out []gen.Diag
	for _, s := range c.scopes() {
		count := map[[2]string]int{}
		same := map[[4]string]int{}
		var order [][2]string
		for _, e := range s.Edges() {
			ee := c.edgeEnds(s, e)
			if !ee.ok {
				continue
			}
			k := [2]string{string(e.To.Node), e.To.Port}
			if count[k] == 0 {
				order = append(order, k)
			}
			same[[4]string{string(e.From.Node), e.From.Port, k[0], k[1]}]++
			if ee.dstPD == nil || ee.dstPD.Multi == nil || !*ee.dstPD.Multi {
				count[k]++
			} else if count[k] == 0 {
				count[k] = -1 // 多连端口：只看重复连线
			}
		}
		dup := map[[2]string]bool{}
		for k, n := range same {
			if n > 1 {
				dup[[2]string{k[2], k[3]}] = true
			}
		}
		for _, k := range order {
			if count[k] > 1 || dup[k] {
				out = append(out, c.diag(gen.DiagV07, gen.DiagLevelError, Loc{Scope: s.ID(), Node: k[0], Port: k[1]}, map[string]any{"port": k[1]}))
			}
		}
	}
	return out
}}
