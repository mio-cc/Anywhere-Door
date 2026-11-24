package llm

import (
	"fmt"
	"slices"
)

// EffortLevels 是上层统一使用的 7 级思考强度，从低到高（U.09.06-R5、E.14.02）。
var EffortLevels = []string{"off", "minimal", "low", "medium", "high", "xhigh", "max"}

func effortIndex(s string) int { return slices.Index(EffortLevels, s) }

// ResolveEffort 按 E.10.05 §5 把请求的强度换成这个模型支持的一级。
// value 为 "" 表示请求里不带这个参数（没有请求、请求 off、模型不支持调节）；warn 不为空时调用方记一条 warn。
// 规则：在 Efforts 里原样用；不在时先查 EffortMap；再取 Efforts 里最接近的一级，一样近取较高的一级。
func ResolveEffort(requested string, c Caps) (value string, warn string) {
	if requested == "" || requested == "off" || len(c.Efforts) == 0 {
		return "", ""
	}
	if slices.Contains(c.Efforts, requested) {
		return requested, ""
	}
	if m, ok := c.EffortMap[requested]; ok && m != "" {
		return m, fmt.Sprintf("这个模型不支持思考强度 %s，按映射改用 %s", requested, m)
	}
	ri := effortIndex(requested)
	if ri < 0 {
		return "", fmt.Sprintf("不认识的思考强度 %s，请求里不带这个参数", requested)
	}
	best, bestIdx, bestDist := "", -1, len(EffortLevels)+1
	for _, e := range c.Efforts {
		ei := effortIndex(e)
		if ei <= 0 { // 不认识的级和 off 不参与
			continue
		}
		d := ei - ri
		if d < 0 {
			d = -d
		}
		if d < bestDist || (d == bestDist && ei > bestIdx) {
			best, bestIdx, bestDist = e, ei, d
		}
	}
	if best == "" {
		return "", fmt.Sprintf("这个模型不支持思考强度 %s，请求里不带这个参数", requested)
	}
	return best, fmt.Sprintf("这个模型不支持思考强度 %s，改用最接近的 %s", requested, best)
}
