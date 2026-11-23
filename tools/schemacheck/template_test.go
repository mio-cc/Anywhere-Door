package main

import (
	"reflect"
	"testing"
)

func TestTemplate(t *testing.T) {
	const ref = "template.schema.json"
	t.Run("C1304-T1 清单 schema", func(t *testing.T) {
		s := repoSet(t)
		e := manifestEntry(t, s, "template_minimal.json") // 按清单校验，应当通过
		if e.Schema != ref || e.Valid == nil || !*e.Valid {
			t.Errorf("清单里 template_minimal.json 应按 %s 校验并通过：%+v", ref, e)
		}
		var doc map[string]any
		readJSONExample(t, "template_minimal.json", &doc)
		if got, want := sortedKeys(doc), []string{"graph", "inputs", "name", "summary", "title", "version"}; !reflect.DeepEqual(got, want) {
			t.Errorf("template_minimal.json 的字段是 %v，应只有必填字段加 inputs：%v", got, want)
		}
		items, _ := doc["inputs"].([]any)
		if len(items) != 1 {
			t.Fatalf("inputs 应恰好 1 项，实际 %d 项", len(items))
		}
		item, _ := items[0].(map[string]any)
		if got, want := sortedKeys(item), []string{"key", "label", "target"}; !reflect.DeepEqual(got, want) {
			t.Errorf("inputs[0] 的字段是 %v，应只有必填的 %v", got, want)
		}
	})
	t.Run("C1304-T2 清单反例", func(t *testing.T) {
		s := repoSet(t)
		const head = `"name": "闭环迭代", "title": "闭环迭代", "summary": "反复生成和评审，直到验收通过"`
		const item = `"key": "topic", "label": "你想调研什么？"`
		mustValid(t, s, ref, `{`+head+`, "version": "1.0.0", "graph": "graph.json", "inputs": [{`+item+`, "target": "inputs.topic"}]}`)
		mustInvalid(t, s, ref, `{`+head+`, "version": "1.0.0", "graph": "main.json"}`, "/graph")
		mustInvalid(t, s, ref, `{`+head+`, "version": "1.0", "graph": "graph.json"}`, "/version")
		mustInvalid(t, s, ref, `{`+head+`, "version": "1.0.0", "graph": "graph.json", "inputs": [{`+item+`}]}`, "/inputs/0")
		mustInvalid(t, s, ref, `{`+head+`, "version": "1.0.0", "graph": "graph.json", "inputs": [{`+item+`, "target": "inputs.topic", "hint": "写一个主题"}]}`, "/inputs/0")
	})
}
