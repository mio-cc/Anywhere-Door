package prompt

import (
	"strconv"
	"strings"

	"github.com/mio-cc/anywhere-door/engine/ctxasm"
	"github.com/mio-cc/anywhere-door/engine/schema"
	"github.com/mio-cc/anywhere-door/gen"
)

// maxExtends 是角色卡沿 extends 最多走的层数（C.13.02-R1）。
const maxExtends = 4

// replaceMark 写在子卡列表字段的第一项时，整体替换父卡的列表（C.13.02-R3）。
const replaceMark = "!replace"

func cardErr(code, msg string) *ctxasm.Error {
	return &ctxasm.Error{Class: gen.FailureClassUserInput, Code: code, Msg: msg}
}

func cardKey(c *gen.Rolecard) string { return c.Name + "@" + strconv.FormatInt(c.Version, 10) }

// ResolveRoleCard 解析角色卡继承（C.13.02-R1–R3）：@latest 去掉后取最高版本；沿 extends 最多 4 层，超过或有环报
// rolecard_chain；文字字段子卡非空时覆盖；列表字段追加在父卡后面，子卡第一项是 "!replace" 时整体替换（去掉这一项）。
// 返回合并后的副本，名字、版本、extends 是子卡自己的。
func ResolveRoleCard(ref string, v schema.View) (*gen.Rolecard, error) {
	var chain []*gen.Rolecard
	seen := map[string]bool{}
	cur := strings.TrimSuffix(ref, "@latest")
	for {
		c, ok := v.RoleCard(cur)
		if !ok {
			if len(chain) == 0 {
				return nil, cardErr("rolecard_missing", "角色卡「"+ref+"」不存在")
			}
			return nil, cardErr("rolecard_missing", "角色卡「"+cardKey(chain[len(chain)-1])+"」继承的「"+cur+"」不存在")
		}
		k := cardKey(c)
		if seen[k] {
			return nil, cardErr("rolecard_chain", "角色卡「"+ref+"」的继承有环：经过「"+k+"」两次")
		}
		seen[k] = true
		chain = append(chain, c)
		if c.Extends == nil || *c.Extends == "" {
			break
		}
		if len(chain) > maxExtends {
			return nil, cardErr("rolecard_chain", "角色卡「"+ref+"」的继承超过 "+strconv.Itoa(maxExtends)+" 层")
		}
		cur = strings.TrimSuffix(*c.Extends, "@latest")
	}
	out := *chain[len(chain)-1]
	for i := len(chain) - 2; i >= 0; i-- {
		out = mergeCard(out, *chain[i])
	}
	out.Name, out.Version, out.Extends = chain[0].Name, chain[0].Version, chain[0].Extends
	return &out, nil
}

func mergeCard(parent, child gen.Rolecard) gen.Rolecard {
	out := parent
	if child.Identity != "" {
		out.Identity = child.Identity
	}
	if child.Mission != "" {
		out.Mission = child.Mission
	}
	out.GoodOutput = textOver(parent.GoodOutput, child.GoodOutput)
	out.WhenStuck = textOver(parent.WhenStuck, child.WhenStuck)
	out.Reason = textOver(parent.Reason, child.Reason)
	out.Status = textOver(parent.Status, child.Status)
	if child.Author != nil {
		out.Author = child.Author
	}
	out.Method = listOver(parent.Method, child.Method)
	out.Boundaries = listOver(parent.Boundaries, child.Boundaries)
	out.Vars = listOver(parent.Vars, child.Vars)
	out.FitModels = listOver(parent.FitModels, child.FitModels)
	if child.Examples != nil {
		ex := append([]gen.WsRef(nil), *child.Examples...)
		if parent.Examples != nil {
			ex = append(append([]gen.WsRef(nil), *parent.Examples...), ex...)
		}
		out.Examples = &ex
	}
	return out
}

func textOver(parent, child *string) *string {
	if child != nil && *child != "" {
		return child
	}
	return parent
}

func listOver(parent, child *[]string) *[]string {
	if child == nil {
		return parent
	}
	c := *child
	if len(c) > 0 && c[0] == replaceMark {
		out := append([]string{}, c[1:]...)
		return &out
	}
	var out []string
	if parent != nil {
		out = append(out, *parent...)
	}
	out = append(out, c...)
	return &out
}
