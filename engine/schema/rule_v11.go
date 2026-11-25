package schema

import "github.com/mio-cc/anywhere-door/gen"

// V11 容器规则（NodeDef.container，与 container_rules.yaml 一致）；子图里有入口和出口（类别 边界、role 为 entry / exit 的节点）；
// 声明了端口的容器上，子图端口节点写的 port_name 要有同方向的端口（P.09 K258 ⑤）。
var ruleV11 = rule{code: gen.DiagV11, local: true, check: func(c *Context) []gen.Diag {
	var out []gen.Diag
	for _, na := range c.nodes() {
		if na.def == nil {
			continue
		}
		bad := func(reason string) {
			out = append(out, c.diag(gen.DiagV11, gen.DiagLevelError, Loc{Scope: na.s.ID(), Node: string(na.n.ID)}, map[string]any{"label": label(na.n), "reason": reason}))
		}
		hasChildren, hasRef := na.n.Children != nil, na.n.Ref != nil
		switch containerOf(na.def) {
		case "required":
			if !hasChildren {
				bad("必须带子图")
			}
			if hasRef {
				bad("不能引用子图库，要内联子图")
			}
		case "one_of":
			if hasChildren == hasRef {
				bad("内联子图和子图库引用要有且只有一个")
			}
		default:
			if hasChildren || hasRef {
				bad("这种节点不能带子图")
			}
		}
		cs, ok := c.G.Scope(string(na.n.ID))
		if !ok {
			continue
		}
		declared := !derived(na.def)
		roles := map[string]bool{}
		for _, b := range cs.Nodes() {
			bdef, ok := c.V.NodeDef(string(b.Type))
			if !ok || bdef.Category != "边界" {
				continue
			}
			role, ok := paramString(b, bdef, "role")
			if !ok {
				continue
			}
			roles[role] = true
			pn, _ := paramString(b, bdef, "port_name")
			if !declared || pn == "" {
				continue
			}
			ports := na.def.Ports.Outputs
			if role == "entry" {
				ports = na.def.Ports.Inputs
			}
			if _, ok := ports[pn]; !ok {
				bad("子图端口「" + label(b) + "」的端口名「" + pn + "」在「" + label(na.n) + "」上没有对应的端口")
			}
		}
		if !roles["entry"] || !roles["exit"] {
			bad("子图里要有入口和出口端口节点")
		}
	}
	return out
}}
