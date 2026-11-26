package ctxasm_test

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/mio-cc/anywhere-door/engine/ctxasm"
	"github.com/mio-cc/anywhere-door/engine/schema"
	"github.com/mio-cc/anywhere-door/gen"
)

var (
	viewOnce sync.Once
	viewVal  schema.View
	viewErr  error
)

// view 是加载了仓库内置内容的注册表视图（各测试文件共用）。
func view(t testing.TB) schema.View {
	t.Helper()
	viewOnce.Do(func() {
		r := schema.NewRegistry(schema.Options{})
		viewErr = r.LoadBuiltin(os.DirFS(filepath.Join("..", "..", "schema")))
		viewVal = r.Snapshot()
	})
	if viewErr != nil {
		t.Fatal(viewErr)
	}
	return viewVal
}

// runeCounter 一个字算一个 token（本包后面几个叶子的测试共用：fit_test、inherit_test、assemble_test）。
type runeCounter struct{}

// 本叶子的测试还用不到 runeCounter：先断言它满足 ctxasm.Counter，免得在后面的测试文件进来之前被 lint 报 unused。
var _ ctxasm.Counter = runeCounter{}

func (runeCounter) CountText(s string) int { return utf8.RuneCountInString(s) }

func sp(s string) *string { return &s }

func levels(ds []gen.Diag, lv gen.DiagLevel) []string {
	var out []string
	for _, d := range ds {
		if d.Level == lv {
			if d.Code != "V03" {
				out = append(out, "代码不是 V03："+d.Msg)
				continue
			}
			out = append(out, d.Msg)
		}
	}
	return out
}

func TestRecipe(t *testing.T) {
	v := view(t)

	t.Run("E1301-T1 内置配方", func(t *testing.T) {
		var c ctxasm.Cache
		for _, name := range []string{"主脑配方", "智囊配方", "聚类员配方", "决策者配方", "执行者配方", "验收者配方"} {
			p, ds := c.Compile(name+"@1", v)
			if errs := levels(ds, gen.DiagLevelError); len(errs) > 0 || p == nil {
				t.Errorf("%s 编译有错误：%v", name, errs)
				continue
			}
			if len(p.Sections) != len(p.Recipe.Sections) || p.Ref != name+"@1" {
				t.Errorf("%s 段数或引用不对：%d %s", name, len(p.Sections), p.Ref)
			}
			if p2, _ := c.Compile(name+"@1", v); p2 != p {
				t.Errorf("%s 应命中缓存", name)
			}
		}
		if _, ds := c.Compile("没有的配方@1", v); len(levels(ds, gen.DiagLevelError)) != 1 {
			t.Errorf("不存在的配方应报 error：%v", ds)
		}
		p, _ := c.Compile("执行者配方@1", v)
		s := p.Sections[0]
		if s.Source != "task" || strings.Join(s.Path, ".") != "instruction" || s.Index != -1 || s.Order != 0 {
			t.Errorf("执行者配方第一段的计划不对：%+v", s)
		}
		if len(p.Exclude) != 3 || p.Exclude[0].Type != "方案" || strings.Join(p.Exclude[1].Path, ".") != "*.reasoning" {
			t.Errorf("exclude 不对：%+v", p.Exclude)
		}
	})

	t.Run("E1301-T2 路径检查", func(t *testing.T) {
		inputs := map[string]gen.TypeRef{"task": "任务卡@1", "plans": "列表[方案@1]"}
		r := &gen.Recipe{Name: "测试配方", Version: 1, Inputs: &inputs, Sections: []gen.Section{
			{Name: "坏字段", From: "in.task.foo"},
			{Name: "好字段", From: "in.task.instruction"},
			{Name: "列表元素", From: "in.plans.understanding"},
			{Name: "嵌套", From: "in.plans.steps.desc"},
			{Name: "嵌套坏", From: "in.plans.steps.bar"},
			{Name: "未声明", From: "in.plan.x"},
			{Name: "经验", From: "in.exp", Transform: sp("top_k(3)")},
			{Name: "全部", From: "in"},
			{Name: "第二条", From: "in[1].title"},
		}}
		p, ds := ctxasm.Compile(r, v)
		warns, errs := levels(ds, gen.DiagLevelWarn), levels(ds, gen.DiagLevelError)
		if len(warns) != 2 || !strings.Contains(warns[0], "task.foo") || !strings.Contains(warns[1], "plans.steps.bar") {
			t.Errorf("应有 2 条 warn（task.foo、plans.steps.bar）：%v", warns)
		}
		if len(errs) != 1 || !strings.Contains(errs[0], "plan") {
			t.Errorf("未声明的来源应报 1 条 error：%v", errs)
		}
		last := p.Sections[len(p.Sections)-1]
		if last.Index != 1 || strings.Join(last.Path, ".") != "title" || last.Source != "" {
			t.Errorf("in[1].title 的计划不对：%+v", last)
		}
		if all := p.Sections[len(p.Sections)-2]; all.Source != "" || all.Index != -1 || len(all.Path) != 0 {
			t.Errorf("in 的计划不对：%+v", all)
		}
		bad := &gen.Recipe{Name: "坏配方", Version: 1, Sections: []gen.Section{
			{Name: "a", From: "input.x"},
			{Name: "b", From: "in", Transform: sp("top_k(0)")},
			{Name: "c", From: "in", Format: sp("markdown")},
		}, Exclude: &[]string{"kind:方案"}}
		if _, ds := ctxasm.Compile(bad, v); len(levels(ds, gen.DiagLevelError)) != 4 {
			t.Errorf("from、transform、format、exclude 写错各一条 error：%v", ds)
		}
	})

	t.Run("E1301-T3 各 transform", func(t *testing.T) {
		list := []any{
			map[string]any{"title": "甲", "url": "u1", "x": float64(1)},
			map[string]any{"title": "乙", "url": "u2"},
			map[string]any{"title": "丙", "url": "u3"},
		}
		tk, _ := ctxasm.ParseTransform("top_k(2)")
		if got := tk.Apply(list).([]any); len(got) != 2 {
			t.Errorf("top_k(2)：%d 项", len(got))
		}
		fs, _ := ctxasm.ParseTransform("fields(title, url)")
		got := fs.Apply(list).([]any)[0].(map[string]any)
		if len(got) != 2 || got["x"] != nil {
			t.Errorf("fields 只留 title、url：%v", got)
		}
		tp, _ := ctxasm.ParseTransform("template({{title}}：{{url}})")
		if s := tp.Apply(list); s != "甲：u1\n乙：u2\n丙：u3" {
			t.Errorf("template：%q", s)
		}
		for _, bad := range []string{"top_k(0)", "top_k(x)", "fields()", "template()", "summarize", "fields(a b)"} {
			if _, err := ctxasm.ParseTransform(bad); err == nil {
				t.Errorf("%q 应报错", bad)
			}
		}
		for _, ok := range []string{"", "as_is", "pointer", "summary"} {
			if _, err := ctxasm.ParseTransform(ok); err != nil {
				t.Errorf("%q 应合法：%v", ok, err)
			}
		}

		data := map[string]any{
			"b": []any{"x", map[string]any{"k": "v", "n": float64(2)}},
			"a": "文本",
			"c": 1.5,
			"d": true,
			"e": nil,
		}
		wantYAML := "a: 文本\nb:\n  - x\n  - k: v\n    n: 2\nc: 1.5\nd: true\ne:"
		if got := ctxasm.RenderText(data, ctxasm.FormatYAML); got != wantYAML {
			t.Errorf("yaml：\n%s\n应为：\n%s", got, wantYAML)
		}
		if got := ctxasm.RenderText(data, ctxasm.FormatText); got != wantYAML {
			t.Errorf("text 的对象按 yaml：\n%s", got)
		}
		wantJSON := "{\n  \"a\": \"文本\",\n  \"b\": [\n    \"x\",\n    {\n      \"k\": \"v\",\n      \"n\": 2\n    }\n  ],\n  \"c\": 1.5,\n  \"d\": true,\n  \"e\": null\n}"
		if got := ctxasm.RenderText(data, ctxasm.FormatJSON); got != wantJSON {
			t.Errorf("json：\n%s", got)
		}
		bul := []any{"甲", map[string]any{"n": float64(2), "k": "v"}, float64(3)}
		if got := ctxasm.RenderText(bul, ctxasm.FormatBullets); got != "- 甲\n- k：v；n：2\n- 3" {
			t.Errorf("bullets：%q", got)
		}
		if got := ctxasm.RenderText("原样\n两行", ctxasm.FormatText); got != "原样\n两行" {
			t.Errorf("text 字符串原样：%q", got)
		}
		multi := map[string]any{"note": "第一行\n第二行", "empty": []any{}}
		if got := ctxasm.RenderText(multi, ctxasm.FormatYAML); got != "empty: []\nnote: |\n  第一行\n  第二行" {
			t.Errorf("多行与空列表：%q", got)
		}
	})

	t.Run("E1301-T4 来源分配", func(t *testing.T) {
		inputs := map[string]gen.TypeRef{"goal": "目标@1", "task": "任务卡@1", "plans": "列表[方案@1]"}
		r := &gen.Recipe{Name: "分配配方", Version: 1, Inputs: &inputs, Exclude: &[]string{"type:执行结果"},
			Sections: []gen.Section{{Name: "任务", From: "in.task.instruction"}}}
		p, ds := ctxasm.Compile(r, v)
		if len(levels(ds, gen.DiagLevelError)) > 0 {
			t.Fatal(ds)
		}
		in := []ctxasm.Value{
			{Type: "任务卡@1", Data: map[string]any{"instruction": "做事"}},
			{Type: "目标@1", Data: map[string]any{"intent": "目标"}, Taint: true, TaintSrc: []string{"u1"}},
			{Type: "方案@1", Data: map[string]any{"understanding": "一"}, Taint: true, TaintSrc: []string{"u2"}},
			{Type: "列表[方案@1]", Data: []any{map[string]any{"understanding": "二"}, map[string]any{"understanding": "三"}}},
			{Type: "执行结果@1", Data: map[string]any{"status": "ok"}},
			{Data: "没有类型"},
		}
		b, ds := p.Bind(in, v)
		if len(ds) != 0 {
			t.Fatalf("不应有诊断：%v", ds)
		}
		if b.Sources["task"].Data.(map[string]any)["instruction"] != "做事" || b.Sources["goal"].Type != "目标@1" {
			t.Errorf("goal、task 分配不对：%+v", b.Sources)
		}
		plans := b.Sources["plans"]
		if l := plans.Data.([]any); len(l) != 3 || plans.Type != "列表[方案@1]" || !plans.Taint || strings.Join(plans.TaintSrc, ",") != "u2" {
			t.Errorf("三份方案应合成一个列表：%+v", plans)
		}
		if len(b.All) != 5 || len(b.In) != 6 {
			t.Errorf("被排除类型的值不进 All：%d / %d", len(b.All), len(b.In))
		}
		_, ds = p.Bind([]ctxasm.Value{{Type: "目标@1"}, {Type: "目标@1"}}, v)
		if errs := levels(ds, gen.DiagLevelError); len(errs) != 1 || !strings.Contains(errs[0], "goal") || !strings.Contains(errs[0], "不唯一") {
			t.Errorf("两条目标对应一个 goal 应报 V03：%v", ds)
		}
	})
}
