package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestAggregateOps(t *testing.T) {
	t.Run("C0403-T1 算子表", func(t *testing.T) {
		s := repoSet(t)
		var ops map[string]string
		readYAML(t, "schema/builtin/aggregate_ops.yaml", &ops)
		// C.04.03 §2 的表
		want := map[string]string{
			"count": "整数@1", "group": "聚合结果@1", "group_ratio": "聚合结果@1", "vote": "$T",
			"dedupe": "列表[$T]", "sort": "列表[$T]", "filter": "列表[$T]", "top_k": "列表[$T]", "concat": "列表[$T]",
			"stats": "统计@1", "divergence": "分歧报告@1",
		}
		if !reflect.DeepEqual(ops, want) {
			t.Errorf("aggregate_ops.yaml：%v\n应为 C.04.03 §2：%v", ops, want)
		}
		for op, out := range ops {
			mustValid(t, s, "types.schema.json#/$defs/TypeRef", `"`+out+`"`)
			if i := strings.Index(out, "$"); i >= 0 && !strings.HasPrefix(out[i:], "$T") {
				t.Errorf("%s 的输出 %s：类型变量只用 $T（聚合输入的元素类型）", op, out)
			}
		}
	})
}
