package schema

import "github.com/mio-cc/anywhere-door/gen"

// rule 是一条规则的实现：local 为 true 的是局部规则（增量时只查被触及的子图），其余每次全量。
type rule struct {
	code  gen.DiagCode
	local bool
	check func(c *Context) []gen.Diag
}

func (r rule) Code() gen.DiagCode          { return r.code }
func (r rule) Check(c *Context) []gen.Diag { return r.check(c) }
func (r rule) Local() bool                 { return r.local }

func isLocal(r Rule) bool {
	l, ok := r.(interface{ Local() bool })
	return ok && l.Local()
}

// rules 是 M1 的规则，按编号（E.03.03 §5）；每条规则的实现在 rule_vNN.go。V13、V15、V18 在 M3 加。
var rules = []Rule{ruleV01, ruleV02, ruleV03, ruleV04, ruleV05, ruleV06, ruleV07, ruleV08, ruleV09, ruleV10, ruleV11, ruleV12, ruleV14, ruleV16, ruleV17}
