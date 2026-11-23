package main

import (
	"reflect"
	"slices"
	"testing"
)

const paramUI = "nodedef.schema.json#/$defs/ParamUI"

// paramUIDef 取 nodedef.schema.json 里的 $defs/ParamUI 原文。
func paramUIDef(t *testing.T, s *Set) map[string]any {
	t.Helper()
	defs, _ := s.Raw["nodedef.schema.json"]["$defs"].(map[string]any)
	def, ok := defs["ParamUI"].(map[string]any)
	if !ok {
		t.Fatal("nodedef.schema.json 里没有 $defs/ParamUI")
	}
	return def
}

func propEnum(t *testing.T, def map[string]any, field string) []string {
	t.Helper()
	props, _ := def["properties"].(map[string]any)
	f, _ := props[field].(map[string]any)
	raw, ok := f["enum"].([]any)
	if !ok {
		t.Fatalf("ParamUI.%s 没有 enum", field)
	}
	out := make([]string, len(raw))
	for i, v := range raw {
		out[i], _ = v.(string)
	}
	return out
}

func TestParamUI(t *testing.T) {
	t.Run("C0305-T1 ParamUI 定义", func(t *testing.T) {
		s := repoSet(t)
		def := paramUIDef(t, s)
		if def["additionalProperties"] != false {
			t.Error("ParamUI 要写 additionalProperties: false")
		}
		props, _ := def["properties"].(map[string]any)
		want := []string{"bind", "connection_type", "group", "order", "placeholder", "rows", "show_if", "widget"}
		if got := sortedKeys(props); !reflect.DeepEqual(got, want) {
			t.Errorf("ParamUI 的字段：%v，应为 C.03.05 §4 的 %v", got, want)
		}
		if got := propEnum(t, def, "group"); !reflect.DeepEqual(got, []string{"常用", "高级"}) {
			t.Errorf("group：%v，只能是 常用、高级", got)
		}
		// widget 的取值 = widget_map.yaml 推得出的控件 + auto + connection（C.03.02 §4、C.03.05 §4）
		var wm widgetMap
		readYAML(t, "schema/builtin/widget_map.yaml", &wm)
		wantWidgets := []string{"auto", "connection"}
		for _, r := range wm.Rules {
			if r.Widget != "" && !slices.Contains(wantWidgets, r.Widget) {
				wantWidgets = append(wantWidgets, r.Widget)
			}
		}
		got := propEnum(t, def, "widget")
		slices.Sort(got)
		slices.Sort(wantWidgets)
		if !reflect.DeepEqual(got, wantWidgets) {
			t.Errorf("widget：%v，应为 widget_map.yaml 的控件加 auto、connection：%v", got, wantWidgets)
		}
		// ParamDef.ui 引用它
		mustValid(t, s, paramDef, `{"type": "text", "label": "指令", "ui": {"group": "常用", "widget": "textarea", "bind": "prompt", "rows": 6}}`)
	})
	t.Run("C0305-T2 ParamUI 反例", func(t *testing.T) {
		s := repoSet(t)
		mustValid(t, s, paramUI, `{}`)
		mustValid(t, s, paramUI, `{"group": "常用", "order": 1, "widget": "connection", "connection_type": "llm.openai", "placeholder": "选择连接", "show_if": "kind == '模型'"}`)
		mustInvalid(t, s, paramUI, `{"group": "常用", "help": "悬停提示"}`, "")
		mustInvalid(t, s, paramUI, `{"inline": true}`, "")
		mustInvalid(t, s, paramUI, `{"rows": 1}`, "/rows")
		mustInvalid(t, s, paramUI, `{"rows": 13}`, "/rows")
		mustInvalid(t, s, paramUI, `{"group": "节点上"}`, "/group")
		mustInvalid(t, s, paramUI, `{"widget": "area"}`, "/widget")
		mustInvalid(t, s, paramUI, `{"order": 1.5}`, "/order")
		mustInvalid(t, s, paramDef, `{"type": "string", "label": "名称", "ui": {"group": "常用", "x": 1}}`, "/ui")
	})
}
