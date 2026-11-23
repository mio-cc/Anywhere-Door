package main

import (
	"reflect"
	"testing"
)

func TestSubgraph(t *testing.T) {
	t.Run("C0204-T1 容器规则表", func(t *testing.T) {
		type rule struct {
			Type     string `yaml:"type"`
			Children string `yaml:"children"`
			Ref      string `yaml:"ref"`
		}
		var rules []rule
		readYAML(t, "schema/builtin/container_rules.yaml", &rules)
		want := []rule{ // C.02.04 §5
			{"循环", "required", "forbidden"},
			{"扇出", "required", "forbidden"},
			{"子图引用", "one_of", "one_of"},
		}
		if !reflect.DeepEqual(rules, want) {
			t.Errorf("container_rules.yaml：%+v\n应为 C.02.04 §5：%+v", rules, want)
		}
		atomic := map[string]bool{"资源": true, "Agent": true, "上下文装配": true, "工具": true, "扇出": true, "汇合": true, "聚合": true,
			"分支": true, "循环": true, "账本": true, "人工": true, "子图端口": true, "子图引用": true}
		for _, r := range rules {
			if !atomic[r.Type] {
				t.Errorf("%s 不是原子节点类型名", r.Type)
			}
		}
	})
	t.Run("C0204-T2 子图引用", func(t *testing.T) {
		s := repoSet(t)
		if e := manifestEntry(t, s, "invalid/subgraph_ref_bad_version.json"); e.ErrorAt == nil || *e.ErrorAt != "/root/nodes/0/ref/version" {
			t.Errorf("清单里 subgraph_ref_bad_version.json 的 error_at：%+v", e)
		}
		const ref = "graph.schema.json#/$defs/SubgraphRef"
		mustValid(t, s, ref, `{"name": "资料采集", "version": "1.2.0", "pin": "snapshot"}`)
		mustValid(t, s, ref, `{"name": "资料采集", "version": "1.2.0", "pin": "link", "overrides": {"01JDN0000000000000000000E2": "small-model"}}`)
		mustInvalid(t, s, ref, `{"name": "资料采集", "version": "1.2.0", "pin": "latest"}`, "/pin")
		mustInvalid(t, s, ref, `{"name": "资料采集@1", "version": "1.2.0", "pin": "link"}`, "/name")
		mustInvalid(t, s, ref, `{"name": "资料采集", "version": "1.2.0"}`, "")
		mustInvalid(t, s, ref, `{"name": "资料采集", "version": "1.2.0", "pin": "link", "overrides": {"x": 1}}`, "/overrides/x")
		const sub = "graph.schema.json#/$defs/Subgraph"
		mustInvalid(t, s, sub, `{"nodes": []}`, "")
		mustInvalid(t, s, sub, `{"nodes": [], "edges": [], "view": {"x": 0}}`, "")
	})
	t.Run("C0204-T3 诊断示例", func(t *testing.T) {
		s := repoSet(t)
		for f, diag := range map[string]string{
			"semantic/loop_without_children.json": "V11",
			"semantic/subgraph_without_exit.json": "V11",
			"semantic/nesting_depth_9.json":       "V12",
			"semantic/nesting_depth_5.json":       "V12",
		} {
			if e := manifestEntry(t, s, f); e.Valid == nil || !*e.Valid || e.Diag != diag {
				t.Errorf("清单里 %s 应当 valid: true、diag: %s：%+v", f, diag, e)
			}
		}
		for f, depth := range map[string]int{"nesting_depth_9.json": 9, "nesting_depth_5.json": 5} {
			g := readGraph(t, "semantic/"+f)
			d := 0
			for sg := &g.Root; ; d++ {
				var next *gSubgraph
				for i := range sg.Nodes {
					if sg.Nodes[i].Children != nil {
						next = sg.Nodes[i].Children
					}
				}
				if next == nil {
					break
				}
				sg = next
			}
			if d != depth {
				t.Errorf("%s：最深的子图应在第 %d 层（根图是第 0 层），实际第 %d 层", f, depth, d)
			}
		}
	})
}
