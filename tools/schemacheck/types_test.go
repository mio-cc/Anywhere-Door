package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestTypeRef(t *testing.T) {
	t.Run("C0401-T1 TypeRef 外形", func(t *testing.T) {
		s := repoSet(t)
		const ref = "types.schema.json#/$defs/TypeRef"
		for _, v := range []string{"任意", "$T", "$U", "列表[执行结果@1]", "列表[$T]", "列表[列表[方案@1]]", "方案@1", "方案@1.2", "上下文包@1", strings.Repeat("类", 30) + "@1"} {
			mustValid(t, s, ref, `"`+v+`"`)
		}
		for _, v := range []string{"$t", "$TT", "列表[]", "方案", "方案@x", "方案@1.2.3", "方案@", "@1", "列表[方案@1", "pack/方案@1", "方 案@1", strings.Repeat("类", 31) + "@1", ""} {
			mustInvalid(t, s, ref, `"`+v+`"`, "")
		}
		mustInvalid(t, s, ref, `1`, "")
		// "任意@1" 这类用保留名当类型名的写法，外形上符合 Named，schema 不拦（C.04.01 §2 的 pattern 只查外形），由解析器拒绝（E.03.02）
	})
}

func TestCategoryMap(t *testing.T) {
	t.Run("C0401-T2 类别表", func(t *testing.T) {
		s := repoSet(t)
		want := []string{"any", "ctx", "ref", "list", "tool", "gen", "ctrl", "state"}
		if got := enumOf(t, s, "types.schema.json", "Category"); !reflect.DeepEqual(got, want) {
			t.Errorf("Category：%v，应为 %v（C.04.01 §3）", got, want)
		}
		var cm struct {
			Categories []string `yaml:"categories"`
			Rules      []struct {
				Type     string `yaml:"type"`
				List     bool   `yaml:"list"`
				Default  bool   `yaml:"default"`
				Category string `yaml:"category"`
			} `yaml:"rules"`
		}
		readYAML(t, "schema/builtin/category_map.yaml", &cm)
		if !reflect.DeepEqual(cm.Categories, want) {
			t.Errorf("categories：%v，应与 Category 同序", cm.Categories)
		}
		// §3 表除 ctrl 以外的每一行：怎么匹配 → 类别
		rows := map[string]string{"type:任意": "any", "list": "list", "type:上下文包": "ctx", "type:资源引用": "ref", "type:工具端点": "tool", "type:状态": "state", "default": "gen"}
		seen := map[string]bool{}
		for i, r := range cm.Rules {
			n := 0
			key := ""
			if r.Type != "" {
				n++
				key = "type:" + r.Type
				if strings.ContainsAny(r.Type, "@[]/ ") {
					t.Errorf("第 %d 条：type 只写名称，不带版本：%q", i, r.Type)
				}
			}
			if r.List {
				n++
				key = "list"
			}
			if r.Default {
				n++
				key = "default"
			}
			if n != 1 {
				t.Errorf("第 %d 条：type、list、default 恰好写一个：%+v", i, r)
				continue
			}
			if r.Category == "ctrl" {
				t.Errorf("第 %d 条：ctrl 不由类型推出，只能由 PortDef.category 指定（C.04.01 §3）", i)
			}
			if want, ok := rows[key]; !ok || want != r.Category {
				t.Errorf("第 %d 条 %s → %s，§3 表里是 %q", i, key, r.Category, want)
			}
			if seen[key] {
				t.Errorf("第 %d 条：%s 重复", i, key)
			}
			seen[key] = true
		}
		for key := range rows {
			if !seen[key] {
				t.Errorf("没有覆盖 §3 表的 %s", key)
			}
		}
		if n := len(cm.Rules); n == 0 || !cm.Rules[n-1].Default {
			t.Error("最后一条应当是 default（其他命名类型和类型变量都是 gen）")
		}
	})
}
