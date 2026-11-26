package ctxasm_test

import (
	"reflect"
	"testing"

	"github.com/mio-cc/anywhere-door/engine/ctxasm"
)

func TestInherit(t *testing.T) {
	v := view(t)
	cons := map[string]any{"currency": "CNY", "region": "华东"}

	t.Run("E1306-T1 深层透传", func(t *testing.T) {
		goal := ctxasm.Value{Type: "目标@1", Data: map[string]any{"id": "g1", "intent": "调研", "constraints": cons}}
		c1 := ctxasm.Carry([]ctxasm.Value{goal}, v)
		if !reflect.DeepEqual(c1, map[string]any{"constraints": cons}) {
			t.Fatalf("第一层：%v", c1)
		}
		v2 := ctxasm.Value{Type: "方案@1", Data: map[string]any{"understanding": "x"}, Inherit: c1}
		c2 := ctxasm.Carry([]ctxasm.Value{v2}, v)
		v3 := ctxasm.Value{Type: "执行结果@1", Data: map[string]any{"status": "ok"}, Inherit: c2}
		c3 := ctxasm.Carry([]ctxasm.Value{v3}, v)
		if !reflect.DeepEqual(c3["constraints"], cons) {
			t.Errorf("第三层应有根目标的约束：%v", c3)
		}
		// 同名的值都保留（P.09 K273）：直接带的排在前面，列表首尾相接、去重
		task := ctxasm.Value{Type: "任务卡@1", Data: map[string]any{"constraints": []any{"近", "同"}}, Inherit: map[string]any{"constraints": []any{"同", "远"}}}
		if got := ctxasm.Carry([]ctxasm.Value{task}, v); !reflect.DeepEqual(got["constraints"], []any{"近", "同", "远"}) {
			t.Errorf("列表应首尾相接、去重，直接带的在前：%v", got)
		}
		// 都是对象：按键合并，同键取排在前面的
		a := ctxasm.Value{Type: "目标@1", Data: map[string]any{"constraints": map[string]any{"currency": "A", "region": "华东"}}}
		b := ctxasm.Value{Type: "目标@1", Data: map[string]any{"constraints": map[string]any{"currency": "B", "lang": "zh"}}}
		if got := ctxasm.Carry([]ctxasm.Value{a, b}, v); !reflect.DeepEqual(got["constraints"], map[string]any{"currency": "A", "region": "华东", "lang": "zh"}) {
			t.Errorf("对象按键合并、同键取在前的：%v", got)
		}
		// 形状不同：合成一个列表；后面直接带的排在前面携带的之前
		far := ctxasm.Value{Type: "方案@1", Data: map[string]any{}, Inherit: map[string]any{"constraints": "远"}}
		if got := ctxasm.Carry([]ctxasm.Value{far, a}, v); !reflect.DeepEqual(got["constraints"], []any{map[string]any{"currency": "A", "region": "华东"}, "远"}) {
			t.Errorf("形状不同时合成列表、直接带的在前：%v", got)
		}
		// 实际的一条链：根目标的约束（对象）带到决策者产出的任务卡上，任务卡自己也有约束（列表）：两者都在
		card := ctxasm.Value{Type: "任务卡@1", Data: map[string]any{"constraints": []any{"只用公开资料"}}, Inherit: c1}
		if got := ctxasm.Carry([]ctxasm.Value{card}, v); !reflect.DeepEqual(got["constraints"], []any{"只用公开资料", cons}) {
			t.Errorf("任务卡的约束和根目标的约束都要在：%v", got)
		}
		if !reflect.DeepEqual(a.Data.(map[string]any)["constraints"], map[string]any{"currency": "A", "region": "华东"}) {
			t.Error("合并不能改动输入")
		}
		// 列表取元素的
		list := ctxasm.Value{Type: "列表[任务卡@1]", Data: []any{map[string]any{"id": "t1"}, map[string]any{"constraints": []any{"甲"}}}}
		if got := ctxasm.Carry([]ctxasm.Value{list}, v); !reflect.DeepEqual(got["constraints"], []any{"甲"}) {
			t.Errorf("列表取元素的：%v", got)
		}
		if got := ctxasm.Carry([]ctxasm.Value{{Type: "方案@1", Data: map[string]any{"understanding": "x"}}, {Data: "无类型"}}, v); got != nil {
			t.Errorf("没有 inherit 字段时为 nil：%v", got)
		}
	})

	t.Run("E1306-T2 锁定段", func(t *testing.T) {
		p, ok := ctxasm.InheritPart(map[string]any{"constraints": cons}, runeCounter{})
		want := "constraints:\n  currency: CNY\n  region: 华东"
		if !ok || p.Name != ctxasm.InheritSection || !p.Locked || p.Source != "inherit" || p.Text != want || p.Tokens != len([]rune(want)) {
			t.Errorf("锁定段不对：%+v", p)
		}
		if ctxasm.InheritSection != "约束" { // 段名是契约（E.13.06 §5），常量改了这里要知道
			t.Errorf("锁定段的段名应为「约束」：%q", ctxasm.InheritSection)
		}
		p2, _ := ctxasm.InheritPart(map[string]any{"constraints": cons}, runeCounter{})
		if p2.Text != p.Text {
			t.Error("内容应确定")
		}
		if _, ok := ctxasm.InheritPart(nil, runeCounter{}); ok {
			t.Error("空的副本不生成段")
		}
	})

	t.Run("E1306-T3 告警阈值", func(t *testing.T) {
		if !ctxasm.InheritOver(31, 100) || ctxasm.InheritOver(30, 100) || ctxasm.InheritOver(0, 0) || !ctxasm.InheritOver(1, 0) {
			t.Error("超过 30% 为真、正好 30% 为假")
		}
	})
}
