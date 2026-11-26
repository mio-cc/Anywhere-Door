package ctxasm

import (
	"reflect"
	"strconv"

	"github.com/mio-cc/anywhere-door/engine/schema"
)

// InheritSection 是透传约束锁定段的段名（E.13.06）。
const InheritSection = "约束"

// Carry 算出节点输出要携带的 inherit 字段副本（E.13.06 §5，P.09 K273）：同名字段的值都保留、合并成一个：
// 先是各条输入数据里直接带的 inherit 字段（值的类型的顶层字段里 inherit: true 的；列表取各元素的），按输入顺序；
// 再是各条输入携带的副本，按输入顺序。合并按 mergeInherit：对象按键合并、同键取排在前面的；列表首尾相接、去重；
// 形状不同时合成一个列表。没有时返回 nil。
func Carry(in []Value, v schema.View) map[string]any {
	var vals [][2]any // [键, 值]，按优先顺序
	for _, x := range in {
		d := direct(x, v)
		for _, k := range sortedKeys(d) {
			vals = append(vals, [2]any{k, d[k]})
		}
	}
	for _, x := range in {
		for _, k := range sortedKeys(x.Inherit) {
			if val := x.Inherit[k]; val != nil {
				vals = append(vals, [2]any{k, val})
			}
		}
	}
	if len(vals) == 0 {
		return nil
	}
	out := map[string]any{}
	for _, kv := range vals {
		k := kv[0].(string)
		if cur, ok := out[k]; ok {
			out[k] = mergeInherit(cur, kv[1])
		} else {
			out[k] = kv[1]
		}
	}
	return out
}

// mergeInherit 合并同名的两个 inherit 值，a 排在前面：相同时取 a；都是对象时按键合并，同键取 a 的；
// 都是列表时首尾相接、去掉重复的项；形状不同时合成一个列表（列表展开成它的项）。两边都不会被改动。
func mergeInherit(a, b any) any {
	if reflect.DeepEqual(a, b) {
		return a
	}
	am, aok := a.(map[string]any)
	bm, bok := b.(map[string]any)
	if aok && bok {
		out := make(map[string]any, len(am)+len(bm))
		for k, val := range bm {
			out[k] = val
		}
		for k, val := range am {
			out[k] = val
		}
		return out
	}
	var out []any
	for _, x := range append(items(a), items(b)...) {
		dup := false
		for _, y := range out {
			if reflect.DeepEqual(x, y) {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, x)
		}
	}
	return out
}

func items(x any) []any {
	if l, ok := x.([]any); ok {
		return l
	}
	return []any{x}
}

// direct 是值里直接带的 inherit 字段。
func direct(x Value, v schema.View) map[string]any {
	if x.Type == "" || x.Data == nil {
		return nil
	}
	t, err := schema.Parse(x.Type)
	if err != nil {
		return nil
	}
	items := []any{x.Data}
	if t.Kind == schema.List && t.Elem != nil {
		t = t.Elem
		l, ok := x.Data.([]any)
		if !ok {
			return nil
		}
		items = l
	}
	if t.Kind != schema.Named {
		return nil
	}
	fs, ok := typeFields(t.Name+"@"+strconv.Itoa(t.Major), v, 0)
	if !ok {
		return nil
	}
	out := map[string]any{}
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		for _, f := range fs {
			if f.Inherit == nil || !*f.Inherit {
				continue
			}
			if val, ok := m[f.Name]; ok && val != nil {
				if _, seen := out[f.Name]; !seen {
					out[f.Name] = val
				}
			}
		}
	}
	return out
}

// InheritPart 生成锁定段"约束"：来源 inherit，内容按 yaml 渲染（键按字典序）。m 为空时返回 false。
func InheritPart(m map[string]any, cnt Counter) (Part, bool) {
	if len(m) == 0 {
		return Part{}, false
	}
	if cnt == nil {
		cnt = runeCounter{}
	}
	text := RenderText(m, FormatYAML)
	return Part{Name: InheritSection, Text: text, Source: "inherit", Tokens: cnt.CountText(text), Locked: true}, true
}

// InheritOver：锁定段超过预算的 30%。
func InheritOver(tokens, budget int) bool {
	if budget <= 0 {
		return tokens > 0
	}
	return tokens*10 > budget*3
}
