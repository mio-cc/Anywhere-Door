package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

const paramDef = "nodedef.schema.json#/$defs/ParamDef"

// widgetWhen 是 widget_map.yaml 一条规则的附加条件（C.03.02 §4，含义见文件头注释）。
type widgetWhen struct {
	SpanSteps     float64 `yaml:"span_steps"`
	MaxOptions    int     `yaml:"max_options"`
	MaxLabelChars int     `yaml:"max_label_chars"`
	HasSchema     bool    `yaml:"has_schema"`
}

type widgetRule struct {
	Types   []string    `yaml:"types"`
	When    *widgetWhen `yaml:"when"`
	Widget  string      `yaml:"widget"`
	Subform bool        `yaml:"subform"`
}

type widgetMap struct {
	Rules []widgetRule `yaml:"rules"`
}

// match：def 满足这条规则的附加条件。没写 when 时总是满足。
func (w *widgetWhen) match(def map[string]any) bool {
	if w == nil {
		return true
	}
	if w.SpanSteps > 0 {
		lo, okLo := def["min"].(float64)
		hi, okHi := def["max"].(float64)
		step, ok := def["step"].(float64)
		if !ok {
			step = 1
		}
		if !okLo || !okHi || hi-lo > w.SpanSteps*step {
			return false
		}
	}
	if w.MaxOptions > 0 {
		opts, _ := def["enum"].([]any)
		if len(opts) > w.MaxOptions {
			return false
		}
		labels, _ := def["enum_labels"].(map[string]any)
		for _, o := range opts {
			name := fmt.Sprint(o)
			if l, ok := labels[name].(string); ok {
				name = l
			}
			if utf8.RuneCountInString(name) > w.MaxLabelChars {
				return false
			}
		}
	}
	if w.HasSchema {
		_, hasSchema := def["schema"]
		_, hasFrom := def["schema_from"]
		if !hasSchema && !hasFrom {
			return false
		}
	}
	return true
}

// refWidget 按 widget_map.yaml 给一个 ParamDef 推出控件，子表单返回 "subform"。只用来核对表的含义；
// 正式实现在前端（W.06.03、W.06.10、W.07.03），读同一张表的生成物 WIDGET_MAP。
func refWidget(rules []widgetRule, def map[string]any) string {
	typ, _ := def["type"].(string)
	for _, r := range rules {
		if !slices.Contains(r.Types, typ) || !r.When.match(def) {
			continue
		}
		if r.Subform {
			return "subform"
		}
		return r.Widget
	}
	return ""
}

// paramTypes 是 ParamDef.type 的枚举。
func paramTypes(t *testing.T, s *Set) []string {
	t.Helper()
	defs, _ := s.Raw["nodedef.schema.json"]["$defs"].(map[string]any)
	def, _ := defs["ParamDef"].(map[string]any)
	props, _ := def["properties"].(map[string]any)
	typ, _ := props["type"].(map[string]any)
	raw, ok := typ["enum"].([]any)
	if !ok {
		t.Fatal("nodedef.schema.json 里没有 $defs/ParamDef/properties/type/enum")
	}
	out := make([]string, len(raw))
	for i, v := range raw {
		out[i], _ = v.(string)
	}
	return out
}

func TestWidgetMap(t *testing.T) {
	t.Run("C0302-T1 控件映射表", func(t *testing.T) {
		s := repoSet(t)
		var wm widgetMap
		readYAML(t, "schema/builtin/widget_map.yaml", &wm)
		// C.03.02 §4 的表，一行一条，顺序就是匹配顺序
		want := []widgetRule{
			{Types: []string{"integer", "number"}, When: &widgetWhen{SpanSteps: 100}, Widget: "slider"},
			{Types: []string{"integer", "number"}, Widget: "stepper"},
			{Types: []string{"boolean"}, Widget: "switch"},
			{Types: []string{"enum"}, When: &widgetWhen{MaxOptions: 4, MaxLabelChars: 6}, Widget: "segmented"},
			{Types: []string{"enum"}, Widget: "select"},
			{Types: []string{"string"}, Widget: "input"},
			{Types: []string{"text"}, Widget: "textarea"},
			{Types: []string{"expr"}, Widget: "code"},
			{Types: []string{"model_ref"}, Widget: "model"},
			{Types: []string{"rolecard_ref"}, Widget: "rolecard"},
			{Types: []string{"type_ref", "recipe_ref", "tool_ref"}, Widget: "picker"},
			{Types: []string{"object"}, When: &widgetWhen{HasSchema: true}, Subform: true},
			{Types: []string{"object"}, Widget: "code"},
		}
		if !reflect.DeepEqual(wm.Rules, want) {
			t.Errorf("widget_map.yaml 与 C.03.02 §4 不一致：\n得到 %+v\n应为 %+v", wm.Rules, want)
		}
		// ParamUI.widget 去掉 auto 的取值（C.03.05 §4）；ParamUI 在 C.03.05 建，建好后由 C0305-T1 拿 schema 再对一次
		uiWidgets := []string{"input", "textarea", "stepper", "slider", "segmented", "select", "switch", "picker", "code", "model", "rolecard", "connection"}
		types := paramTypes(t, s)
		fallback := map[string]bool{}
		for i, r := range wm.Rules {
			if (r.Widget != "") == r.Subform {
				t.Errorf("第 %d 条：widget 和 subform 恰好写一个：%+v", i, r)
			}
			if r.Widget != "" && !slices.Contains(uiWidgets, r.Widget) {
				t.Errorf("第 %d 条：控件 %s 不在 ParamUI.widget 的取值里（C.03.05 §4）", i, r.Widget)
			}
			if r.Widget == "connection" {
				t.Errorf("第 %d 条：connection 只能由 ui.widget 显式指定，不由类型推出（C.03.02 §4）", i)
			}
			for _, ty := range r.Types {
				if !slices.Contains(types, ty) {
					t.Errorf("第 %d 条：%s 不是 ParamDef.type 的取值", i, ty)
				}
				if fallback[ty] {
					t.Errorf("第 %d 条：%s 前面已经有不带条件的规则，这一条永远不会命中", i, ty)
				}
				if r.When == nil {
					fallback[ty] = true
				}
			}
		}
		for _, ty := range types {
			if !fallback[ty] {
				t.Errorf("参数类型 %s 没有不带条件的兜底规则，可能推不出控件", ty)
			}
		}
		// 表的含义：§4 每一行和 §7 的两个例子
		for _, c := range []struct{ def, want string }{
			{`{"type": "integer", "label": "并发", "min": 1, "max": 64, "step": 1}`, "slider"},
			{`{"type": "number", "label": "温度", "min": 0, "max": 1}`, "slider"},
			{`{"type": "integer", "label": "上限", "min": 0, "max": 1000}`, "stepper"},
			{`{"type": "number", "label": "比例", "min": 0, "max": 1000, "step": 10}`, "slider"},
			{`{"type": "integer", "label": "轮数", "min": 1}`, "stepper"},
			{`{"type": "boolean", "label": "开关"}`, "switch"},
			{`{"type": "enum", "label": "汇合", "enum": ["all", "k_of_n", "first", "timeout"], "enum_labels": {"all": "全部", "k_of_n": "前 K 份", "first": "第一份", "timeout": "超时截止"}}`, "segmented"},
			{`{"type": "enum", "label": "五项", "enum": ["a", "b", "c", "d", "e"]}`, "select"},
			{`{"type": "enum", "label": "显示名长", "enum": ["a", "b"], "enum_labels": {"a": "一二三四五六七"}}`, "select"},
			{`{"type": "enum", "label": "枚举值长", "enum": ["toolong", "b"]}`, "select"},
			{`{"type": "string", "label": "名称"}`, "input"},
			{`{"type": "text", "label": "指令"}`, "textarea"},
			{`{"type": "expr", "label": "条件"}`, "code"},
			{`{"type": "model_ref", "label": "模型"}`, "model"},
			{`{"type": "rolecard_ref", "label": "角色卡"}`, "rolecard"},
			{`{"type": "tool_ref", "label": "工具"}`, "picker"},
			{`{"type": "object", "label": "参数", "schema_from": "param:tool"}`, "subform"},
			{`{"type": "object", "label": "设置", "schema": {"type": "object"}}`, "subform"},
			{`{"type": "object", "label": "原始"}`, "code"},
		} {
			mustValid(t, s, paramDef, c.def)
			var def map[string]any
			if err := json.Unmarshal([]byte(c.def), &def); err != nil {
				t.Fatal(err)
			}
			if got := refWidget(wm.Rules, def); got != c.want {
				t.Errorf("%s：推出 %q，应为 %q", c.def, got, c.want)
			}
		}
	})
}

// expandVector 是 schema_expand_vectors.json 的一条（C.03.02 §5）。
type expandVector struct {
	Name       string          `json:"name"`
	Reserved   []string        `json:"reserved"` // 节点上已有的端口名和参数名（这里是工具节点的，C.03.01 §6、E.14.04）
	Schema     json.RawMessage `json:"schema"`
	Expandable bool            `json:"expandable"`
	Params     []expandParam   `json:"params"`
}

type expandParam struct {
	Key   string          `json:"key"`   // 属性名，原样
	Param json.RawMessage `json:"param"` // 子参数，是一个 ParamDef
	Group string          `json:"group"` // 常用 / 高级（R8）
	Port  string          `json:"port"`  // 动态输入端口名（R8）
}

type refParam struct {
	key, group, port string
	param            map[string]any
}

// objectKeys 按出现顺序读一个 JSON 对象的键和值；不是对象时 ok 为 false。R7、R8 都按声明顺序。
func objectKeys(raw json.RawMessage) (keys []string, vals map[string]json.RawMessage, ok bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, nil, false
	}
	vals = map[string]json.RawMessage{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, nil, false
		}
		k, _ := tok.(string)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, nil, false
		}
		if _, dup := vals[k]; !dup {
			keys = append(keys, k)
		}
		vals[k] = v
	}
	return keys, vals, true
}

// refExpand 按 C.03.02 §5（R7、R8、R10）展开 schema_from 对象参数。只用来核对向量本身写对了；
// 正式实现在后端（E.03.02）和前端（W.06.10、W.07.03），跑同一组向量。
func refExpand(schema json.RawMessage, reserved []string) (bool, []refParam) {
	_, top, ok := objectKeys(schema)
	if !ok {
		return false, nil
	}
	if raw, has := top["type"]; has {
		var typ string
		if json.Unmarshal(raw, &typ) != nil || typ != "object" {
			return false, nil
		}
	}
	keys, props, ok := objectKeys(top["properties"])
	if !ok || len(keys) > 32 {
		return false, nil
	}
	var required []string
	if raw, has := top["required"]; has {
		_ = json.Unmarshal(raw, &required)
	}
	taken := map[string]bool{}
	for _, r := range reserved {
		taken[r] = true
	}
	var out []refParam
	common := 0
	for _, k := range keys {
		var sub map[string]any
		_ = json.Unmarshal(props[k], &sub) // 布尔 schema（true / false）读不成对象，按没写 type 处理
		group := "高级"
		if slices.Contains(required, k) && common < 4 {
			group = "常用"
			common++
		}
		out = append(out, refParam{key: k, group: group, port: refPortName(k, taken), param: refParamDef(k, sub)})
	}
	return true, out
}

// refParamDef 是 R7 的映射表：一个属性的 JSON Schema → 子参数 ParamDef。
func refParamDef(key string, sub map[string]any) map[string]any {
	sub = refNullable(sub)
	p := map[string]any{"type": "object"}
	typ, _ := sub["type"].(string) // 可空写法已去掉 null；type 仍是数组（如 ["string", "integer"]）时按没写 type 处理
	for _, k := range []string{"oneOf", "anyOf", "allOf"} {
		if _, has := sub[k]; has {
			typ = ""
		}
	}
	num := func(from, to string) {
		if v, ok := sub[from].(float64); ok {
			p[to] = v
		}
	}
	switch typ {
	case "string":
		if enum, ok := sub["enum"].([]any); ok {
			p["type"], p["enum"] = "enum", enum
			break
		}
		p["type"] = "string"
		if ml, ok := sub["maxLength"].(float64); ok {
			if ml > 200 {
				p["type"] = "text"
			}
			p["max_length"] = ml
		}
	case "integer":
		p["type"], p["step"] = "integer", 1.0
		num("minimum", "min")
		num("maximum", "max")
		num("multipleOf", "step")
	case "number":
		p["type"] = "number"
		num("minimum", "min")
		num("maximum", "max")
		num("multipleOf", "step")
	case "boolean":
		p["type"] = "boolean"
	}
	label := key
	if title, ok := sub["title"].(string); ok && title != "" {
		label = title
	}
	p["label"] = truncRunes(label, 20)
	if d, ok := sub["description"].(string); ok && d != "" {
		p["help"] = truncRunes(d, 200)
	}
	if d, has := sub["default"]; has && d != nil { // default: null 不写（R7）
		p["default"] = d
	}
	return p
}

// refNullable 是 R7 的可空写法：type 是数组、去掉 "null" 后只剩一种类型时按那一种；
// anyOf 或 oneOf 恰好两项、其中一项是 {"type": "null"} 时按另一项，title、description、default 外层写了就用外层的。
// 其他情况原样返回。
func refNullable(sub map[string]any) map[string]any {
	if types, ok := sub["type"].([]any); ok {
		var rest []any
		for _, ty := range types {
			if ty != "null" {
				rest = append(rest, ty)
			}
		}
		if len(rest) != 1 {
			return sub
		}
		out := map[string]any{}
		for k, v := range sub {
			out[k] = v
		}
		out["type"] = rest[0]
		return out
	}
	for _, k := range []string{"anyOf", "oneOf"} {
		alts, ok := sub[k].([]any)
		if !ok || len(alts) != 2 {
			continue
		}
		for i, alt := range alts {
			null, _ := alt.(map[string]any)
			other, ok := alts[1-i].(map[string]any)
			if len(null) != 1 || null["type"] != "null" || !ok {
				continue
			}
			out := map[string]any{}
			for key, val := range other {
				out[key] = val
			}
			for _, f := range []string{"title", "description", "default"} {
				if v, has := sub[f]; has {
					out[f] = v
				}
			}
			return out
		}
	}
	return sub
}

func truncRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// refPortName 是 R8 的端口名换算：A–Z 前加 _ 再转小写，其余不在 [a-z0-9_] 里的字符换成 _，
// 数字开头加 p_，和 taken 里的名字重名时依次加 _2、_3。用掉的名字记进 taken。
func refPortName(key string, taken map[string]bool) string {
	var b strings.Builder
	for _, r := range key {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteByte('_')
			b.WriteRune(r + ('a' - 'A'))
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	base := b.String()
	if base == "" || (base[0] >= '0' && base[0] <= '9') {
		base = "p_" + base
	}
	name := base
	for i := 2; taken[name]; i++ {
		name = fmt.Sprintf("%s_%d", base, i)
	}
	taken[name] = true
	return name
}

// normJSON 把值经 JSON 往返一次，便于和向量里读出的值直接比较。
func normJSON(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSchemaExpand(t *testing.T) {
	t.Run("C0302-T2 展开向量", func(t *testing.T) {
		s := repoSet(t)
		b, err := os.ReadFile(filepath.Join(repoRoot, "schema", "examples", "schema_expand_vectors.json"))
		if err != nil {
			t.Fatal(err)
		}
		var vs []expandVector
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&vs); err != nil {
			t.Fatalf("schema_expand_vectors.json：%v", err)
		}
		portRe := regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)
		seenTypes := map[string]bool{}
		unexpandable := 0
		for _, v := range vs {
			t.Run(v.Name, func(t *testing.T) {
				ok, got := refExpand(v.Schema, v.Reserved)
				if ok != v.Expandable {
					t.Fatalf("expandable 应为 %v，按 §5 算出 %v", v.Expandable, ok)
				}
				if !ok {
					unexpandable++
					if len(v.Params) > 0 {
						t.Errorf("不能展开时 params 应为空（R10）")
					}
					return
				}
				if len(got) != len(v.Params) {
					t.Fatalf("展开出 %d 个子参数，向量里写了 %d 个", len(got), len(v.Params))
				}
				used := map[string]bool{}
				for _, r := range v.Reserved {
					used[r] = true
				}
				common := 0
				for i, want := range v.Params {
					mustValid(t, s, paramDef, string(want.Param))
					if !portRe.MatchString(want.Port) {
						t.Errorf("%s：端口名 %q 不符合 ^[a-z_][a-z0-9_]*$", want.Key, want.Port)
					}
					if used[want.Port] {
						t.Errorf("%s：端口名 %q 和已有端口、参数或别的子参数重名", want.Key, want.Port)
					}
					used[want.Port] = true
					switch want.Group {
					case "常用":
						common++
					case "高级":
					default:
						t.Errorf("%s：group 只能是 常用 或 高级：%q", want.Key, want.Group)
					}
					g := got[i]
					if g.key != want.Key || g.group != want.Group || g.port != want.Port {
						t.Errorf("第 %d 个：向量写的是 %s / %s / %s，按 §5 算出 %s / %s / %s", i, want.Key, want.Group, want.Port, g.key, g.group, g.port)
					}
					var wantParam map[string]any
					if err := json.Unmarshal(want.Param, &wantParam); err != nil {
						t.Fatal(err)
					}
					if gp := normJSON(t, g.param); !reflect.DeepEqual(gp, normJSON(t, wantParam)) {
						t.Errorf("%s：向量写的是 %v，按 R7 算出 %v", want.Key, wantParam, gp)
					}
					seenTypes[fmt.Sprint(wantParam["type"])] = true
				}
				if common > 4 {
					t.Errorf("常用子参数 %d 个，最多 4 个（R8）", common)
				}
			})
		}
		for _, ty := range []string{"string", "text", "enum", "integer", "number", "boolean", "object"} {
			if !seenTypes[ty] {
				t.Errorf("向量没有覆盖映射表里产出 %s 的那一行", ty)
			}
		}
		if unexpandable == 0 {
			t.Error("向量没有覆盖无法展开的情况（R10）")
		}
	})
}

func TestParamDef(t *testing.T) {
	t.Run("C0302-T3 ParamDef 反例", func(t *testing.T) {
		s := repoSet(t)
		mustValid(t, s, paramDef, `{"type": "string", "label": "密钥名", "secret": true}`)
		mustValid(t, s, paramDef, `{"type": "integer", "label": "并发", "secret": false}`)
		mustValid(t, s, paramDef, `{"type": "object", "label": "参数", "schema_from": "param:tool"}`)
		mustValid(t, s, paramDef, `{"type": "enum", "label": "汇合", "enum": ["all", 3], "enum_labels": {"all": "全部"}, "default": "all", "lock": "def"}`)
		mustInvalid(t, s, paramDef, `{"type": "integer", "label": "密钥", "secret": true}`, "/type")
		mustInvalid(t, s, paramDef, `{"type": "text", "label": "密钥", "secret": true}`, "/type")
		mustInvalid(t, s, paramDef, `{"type": "string", "label": "参数", "schema_from": "param:tool"}`, "/type")
		mustInvalid(t, s, paramDef, `{"type": "object", "label": "参数", "schema_from": "tool"}`, "/schema_from")
		mustInvalid(t, s, paramDef, `{"type": "enum", "label": "空", "enum": []}`, "/enum")
		mustInvalid(t, s, paramDef, `{"type": "string"}`, "")
		mustInvalid(t, s, paramDef, `{"type": "string", "label": "`+strings.Repeat("长", 21)+`"}`, "/label")
		mustInvalid(t, s, paramDef, `{"type": "date", "label": "日期"}`, "/type")
		mustInvalid(t, s, paramDef, `{"type": "string", "label": "锁", "lock": "user"}`, "/lock")
		// C.03.02 改版前的界面字段（P.09 K212）：已移到 ui
		mustInvalid(t, s, paramDef, `{"type": "string", "label": "旧写法", "inline": true}`, "")
		mustInvalid(t, s, paramDef, `{"type": "string", "label": "旧写法", "widget": "input"}`, "")
	})
}
