package schema

import (
	"encoding/json"
	"errors"
	"io/fs"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/mio-cc/anywhere-door/gen"
)

func ptr[T any](v T) *T { return &v }

func builtinRegistry(t testing.TB) *Registry {
	t.Helper()
	r := NewRegistry(Options{})
	if err := r.LoadBuiltin(repoFS()); err != nil {
		t.Fatalf("内置注册表加载失败：\n%v", err)
	}
	return r
}

func extNode(name string) *gen.NodeDef {
	return &gen.NodeDef{
		Name: name, Version: "1.0", Kind: "composite", Category: "扩展",
		Impl:   gen.Impl{Type: "builtin", Raw: json.RawMessage(`{"type":"builtin"}`)},
		Params: map[string]gen.ParamDef{"n": {Type: "integer", Label: "数量"}},
		Ports: gen.NodeDefPorts{
			Inputs:  map[string]gen.PortDef{"in": {Type: "任意"}},
			Outputs: map[string]gen.PortDef{"out": {Type: "方案@1"}},
		},
	}
}

func field(name, typ string, required bool) gen.Field {
	f := gen.Field{Name: name, Type: typ}
	if required {
		f.Required = ptr(true)
	}
	return f
}

func extType(name string, minor int64, fields ...gen.Field) *gen.TypeDef {
	return &gen.TypeDef{Name: name, Major: 1, Minor: minor, Fields: &fields}
}

// installErr 安装一个包，返回错误文字（成功时为 ""）。
func installErr(t *testing.T, r *Registry, p *Package) string {
	t.Helper()
	err := r.Install(p)
	if err == nil {
		return ""
	}
	var es Errors
	if !errors.As(err, &es) || len(es) == 0 {
		t.Fatalf("Install 应当返回 Errors：%T %v", err, err)
	}
	return err.Error()
}

func validJSON(t *testing.T, s *jsonschema.Schema, doc string) error {
	t.Helper()
	v, err := jsonschema.UnmarshalJSON(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	return s.Validate(v)
}

func TestRegistry(t *testing.T) {
	t.Run("E0301-T1 内置全量加载", func(t *testing.T) {
		r := builtinRegistry(t)
		v := r.Live()
		nodes := []string{"Agent", "上下文装配", "人工", "分支", "子图引用", "子图端口", "工具", "循环", "扇出", "汇合", "聚合", "账本", "资源"}
		for _, n := range nodes {
			if d, ok := v.NodeDef(n); !ok || d.Name != n {
				t.Errorf("没有节点定义 %s", n)
			}
		}
		defs := v.NodeDefs()
		if len(defs) != len(nodes) || !sort.SliceIsSorted(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name }) {
			t.Errorf("NodeDefs：%d 个，要按名字排序", len(defs))
		}
		types := []string{"资源引用", "上下文包", "工具端点", "状态", "简报", "升级", "整数", "方案", "簇标注", "任务卡", "执行结果", "判定", "目标", "差距报告", "聚合结果", "统计", "分歧报告"}
		for _, n := range types {
			if d, ok := v.Type(n + "@1.0"); !ok || d.Name != n {
				t.Errorf("没有类型 %s@1", n)
			}
		}
		if len(r.state().types) != len(types) {
			t.Errorf("类型有 %d 个，想要 %d 个", len(r.state().types), len(types))
		}
		for _, n := range []string{"主脑配方", "决策者配方", "执行者配方", "智囊配方", "聚类员配方", "验收者配方"} {
			if _, ok := v.Recipe(n + "@1"); !ok {
				t.Errorf("没有配方 %s@1", n)
			}
		}
		if len(r.state().recipes) != 6 {
			t.Errorf("配方有 %d 个", len(r.state().recipes))
		}
		for _, n := range []string{"主脑", "决策者", "执行者", "智囊", "编排者", "聚类员", "验收者"} {
			if _, ok := v.RoleCard(n); !ok {
				t.Errorf("没有角色卡 %s", n)
			}
		}
		if len(r.state().rolecards) != 7 {
			t.Errorf("角色卡有 %d 张", len(r.state().rolecards))
		}
		if _, ok := v.DocSchema("graph.schema.json"); !ok {
			t.Error("没有编译好的 graph.schema.json")
		}
		if _, ok := v.NodeDef("没有"); ok {
			t.Error("不存在的节点定义")
		}
		if !strings.HasPrefix(v.Hash(), "sha256:") || v.Messages() == nil {
			t.Error("Hash / Messages")
		}
	})

	t.Run("E0301-T2 覆盖内置被拒", func(t *testing.T) {
		r := builtinRegistry(t)
		for name, p := range map[string]*Package{
			"同名节点": {Name: "p", NodeDefs: []*gen.NodeDef{extNode("Agent")}},
			"同名类型": {Name: "p", Types: []*gen.TypeDef{extType("方案", 0)}},
			"同名配方": {Name: "p", Recipes: []*gen.Recipe{{Name: "执行者配方", Version: 2, OnOverflow: "fail"}}},
		} {
			if msg := installErr(t, r, p); !strings.Contains(msg, "内置") {
				t.Errorf("%s 应当被拒绝：%q", name, msg)
			}
		}
		if err := r.Uninstall(""); err == nil {
			t.Error("内置项不能卸载")
		}
		if err := r.Uninstall("没装"); err == nil {
			t.Error("没装的包不能卸载")
		}
		p := &Package{Name: "p1", Version: "1.0.0", NodeDefs: []*gen.NodeDef{extNode("采集")}, Types: []*gen.TypeDef{extType("线索", 0, field("url", "string", true))}}
		if msg := installErr(t, r, p); msg != "" {
			t.Fatal(msg)
		}
		if _, ok := r.Live().NodeDef("p1/采集@1"); !ok {
			t.Error("扩展节点要能按 包名/名称@主版本 查到")
		}
		if msg := installErr(t, r, p); !strings.Contains(msg, "已经安装") {
			t.Errorf("重复安装：%q", msg)
		}
		if msg := installErr(t, r, &Package{Name: "p2", Types: []*gen.TypeDef{extType("线索", 1)}}); !strings.Contains(msg, "p1") {
			t.Errorf("别的包的类型也不能覆盖：%q", msg)
		}
		if err := r.Uninstall("p1"); err != nil {
			t.Fatal(err)
		}
		if _, ok := r.Live().NodeDef("p1/采集@1"); ok {
			t.Error("卸载后应当查不到")
		}
		if _, ok := r.Live().Type("线索@1"); ok {
			t.Error("卸载后类型应当查不到")
		}
	})

	t.Run("E0301-T3 扩展包不能声明原子节点和系统类型", func(t *testing.T) {
		r := builtinRegistry(t)
		atomic := extNode("新原子")
		atomic.Kind = "atomic"
		if msg := installErr(t, r, &Package{Name: "p", NodeDefs: []*gen.NodeDef{atomic}}); !strings.Contains(msg, "原子") {
			t.Errorf("原子节点：%q", msg)
		}
		sys := extType("新系统类型", 0)
		sys.System = ptr(true)
		if msg := installErr(t, r, &Package{Name: "p", Types: []*gen.TypeDef{sys}}); !strings.Contains(msg, "系统类型") {
			t.Errorf("系统类型：%q", msg)
		}
		if msg := installErr(t, r, &Package{Name: "p", Types: []*gen.TypeDef{extType("上下文包", 0)}}); msg == "" {
			t.Error("同名系统类型要被拒绝（C.04.01 §5 验收 3）")
		}
	})

	t.Run("E0301-T4 Snapshot 隔离", func(t *testing.T) {
		r := builtinRegistry(t)
		snap := r.Snapshot()
		h := snap.Hash()
		if msg := installErr(t, r, &Package{Name: "p1", NodeDefs: []*gen.NodeDef{extNode("采集")}}); msg != "" {
			t.Fatal(msg)
		}
		if _, ok := snap.NodeDef("p1/采集@1"); ok || snap.Hash() != h || len(snap.NodeDefs()) != 13 {
			t.Error("Snapshot 之后安装的包不应当出现在快照里")
		}
		if _, ok := r.Live().NodeDef("p1/采集@1"); !ok || r.Live().Hash() == h {
			t.Error("Live 要看得到新包")
		}
		mid := r.Snapshot()
		if err := r.Uninstall("p1"); err != nil {
			t.Fatal(err)
		}
		if _, ok := mid.NodeDef("p1/采集@1"); !ok {
			t.Error("卸载不影响之前的快照")
		}
		if r.Live().Hash() != h {
			t.Error("卸载后哈希回到原来的值")
		}
	})

	t.Run("E0301-T5 哈希稳定", func(t *testing.T) {
		a, b := builtinRegistry(t), builtinRegistry(t)
		if a.Live().Hash() != b.Live().Hash() {
			t.Fatal("同样内容的内置注册表哈希不同")
		}
		p1 := &Package{Name: "p1", NodeDefs: []*gen.NodeDef{extNode("采集")}}
		p2 := &Package{Name: "p2", Types: []*gen.TypeDef{extType("线索", 0, field("url", "string", false))}}
		for _, p := range []*Package{p1, p2} {
			if msg := installErr(t, a, p); msg != "" {
				t.Fatal(msg)
			}
		}
		for _, p := range []*Package{p2, p1} {
			if msg := installErr(t, b, p); msg != "" {
				t.Fatal(msg)
			}
		}
		if a.Live().Hash() != b.Live().Hash() {
			t.Error("安装顺序不同，哈希不应当不同")
		}
		c := builtinRegistry(t)
		changed := extType("线索", 0, field("url", "string", false))
		changed.Description = ptr("改了一个字段")
		for _, p := range []*Package{p1, {Name: "p2", Types: []*gen.TypeDef{changed}}} {
			if msg := installErr(t, c, p); msg != "" {
				t.Fatal(msg)
			}
		}
		if c.Live().Hash() == a.Live().Hash() {
			t.Error("内容变了，哈希要变")
		}
	})

	t.Run("E0301-T6 YAML 行号报错", func(t *testing.T) {
		mfs := fstest.MapFS{}
		err := fs.WalkDir(repoFS(), ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			if strings.HasSuffix(p, ".schema.json") && !strings.Contains(p, "/") || strings.HasPrefix(p, "builtin/messages/") {
				b, err := fs.ReadFile(repoFS(), p)
				if err != nil {
					return err
				}
				mfs[p] = &fstest.MapFile{Data: b}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		src, err := fs.ReadFile(repoFS(), "builtin/nodedefs/资源.yaml")
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(src), "\n")
		implLine := 0
		for i, l := range lines {
			if strings.HasPrefix(l, "impl:") {
				implLine = i + 1
				lines[i] = "impl: { type: 不存在的执行方式 }"
			}
		}
		if implLine == 0 {
			t.Fatal("资源.yaml 里没有 impl 行")
		}
		mfs["builtin/nodedefs/不合法.yaml"] = &fstest.MapFile{Data: []byte(strings.Join(lines, "\n"))}
		mfs["builtin/nodedefs/坏格式.yaml"] = &fstest.MapFile{Data: []byte("name: 坏\nversion: \"1.0\"\nports: [a, b\nkind: atomic\n")}
		r := NewRegistry(Options{})
		err = r.LoadBuiltin(mfs)
		var es Errors
		if !errors.As(err, &es) || len(es) != 2 {
			t.Fatalf("应当恰好两条错误：%v", err)
		}
		byFile := map[string]*RegistryError{}
		for _, e := range es {
			byFile[e.File] = e
		}
		if e := byFile["builtin/nodedefs/不合法.yaml"]; e == nil || e.Line != implLine || !strings.Contains(e.Error(), "第 ") {
			t.Errorf("schema 不通过的文件应当报在第 %d 行：%+v", implLine, e)
		}
		if e := byFile["builtin/nodedefs/坏格式.yaml"]; e == nil || e.Line <= 0 || !strings.Contains(e.Msg, "YAML") {
			t.Errorf("格式错的文件要带行号：%+v", e)
		}
		if len(r.Live().NodeDefs()) != 0 {
			t.Error("有错误时注册表不变")
		}
	})

	t.Run("E0301-T7 参数与端口声明", func(t *testing.T) {
		r := builtinRegistry(t)
		dup := extNode("重名")
		dup.Params["in"] = gen.ParamDef{Type: "string", Label: "输入"}
		many := extNode("端口多")
		for _, n := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
			many.Ports.Inputs[n] = gen.PortDef{Type: "任意"}
		}
		widget := extNode("控件")
		widget.Params["n"] = gen.ParamDef{Type: "integer", Label: "数量", UI: &gen.ParamUI{Widget: ptr("switch")}}
		secret := extNode("密钥")
		secret.Params["key"] = gen.ParamDef{Type: "string", Label: "密钥", Secret: ptr(true), Default: json.RawMessage(`"sk-` + strings.Repeat("x", 10) + `"`)}
		unreg := extNode("没注册")
		unreg.Ports.Outputs["out"] = gen.PortDef{Type: "列表[没有的类型@1]"}
		for _, c := range []struct {
			d    *gen.NodeDef
			want string
		}{{dup, "重名"}, {many, "最多 8 个"}, {widget, "switch"}, {secret, "密钥值"}, {unreg, "没有注册"}} {
			if msg := installErr(t, r, &Package{Name: "p", NodeDefs: []*gen.NodeDef{c.d}}); !strings.Contains(msg, c.want) {
				t.Errorf("%s 应当因为「%s」被拒绝：%q", c.d.Name, c.want, msg)
			}
		}
		ok := extNode("合格")
		ok.Params["key"] = gen.ParamDef{Type: "string", Label: "密钥", Secret: ptr(true), Default: json.RawMessage(`"openai_main"`), UI: &gen.ParamUI{Widget: ptr("connection")}}
		if msg := installErr(t, r, &Package{Name: "p", NodeDefs: []*gen.NodeDef{ok}}); msg != "" {
			t.Errorf("合格的节点定义：%s", msg)
		}
		if !LooksLikeSecret("AIza123") || !LooksLikeSecret(strings.Repeat("a", 41)) || LooksLikeSecret("openai_main") || LooksLikeSecret(strings.Repeat("a b", 20)) {
			t.Error("LooksLikeSecret")
		}
	})

	t.Run("E0301-T8 类型声明", func(t *testing.T) {
		r := builtinRegistry(t)
		v := r.Live()
		for k := range r.state().types {
			if _, ok := v.TypeSchema(k); !ok {
				t.Errorf("%s 没有 JSON Schema", k)
			}
		}
		for _, c := range []struct {
			ref, doc string
			ok       bool
		}{
			{"整数@1", `3`, true},
			{"整数@1", `"三"`, false},
			{"任务卡@1", `{"id":"t1","instruction":"查资料","purpose":"写报告","good_result":"三条来源","output_type":"执行结果@1"}`, true},
			{"任务卡@1", `{"instruction":"查资料","purpose":"写报告","good_result":"三条来源","output_type":"执行结果@1"}`, false},
			{"执行结果@1", `{"task_id":"t1","status":"完成","conclusion":"找到了"}`, true},
			{"执行结果@1", `{"task_id":"t1","status":"差不多","conclusion":"找到了"}`, false},
			{"执行结果@1", `{"task_id":"t1","status":"完成","conclusion":"` + strings.Repeat("长", 301) + `"}`, false},
		} {
			s, ok := v.TypeSchema(c.ref)
			if !ok {
				t.Fatalf("没有 %s 的 JSON Schema", c.ref)
			}
			if err := validJSON(t, s, c.doc); (err == nil) != c.ok {
				t.Errorf("%s 校验 %.40s：想要 ok=%v，实际 %v", c.ref, c.doc, c.ok, err)
			}
		}
		bad := &Package{Name: "p", Types: []*gen.TypeDef{
			extType("线索", 0, field("url", "string", true)),
			extType("线索", 1, field("url", "string", true), field("title", "string", true)),
		}}
		if msg := installErr(t, r, bad); !strings.Contains(msg, "必填") {
			t.Errorf("次版本新增必填字段要失败：%q", msg)
		}
		good := &Package{Name: "p", Types: []*gen.TypeDef{
			extType("线索", 1, field("url", "string", true), field("title", "string", false)),
			extType("线索", 0, field("url", "string", true)),
		}}
		if msg := installErr(t, r, good); msg != "" {
			t.Fatal(msg)
		}
		if d, ok := v.Type("线索@1"); !ok || d.Minor != 1 {
			t.Error("主版本 1 取最高次版本")
		}
		s, _ := v.TypeSchema("线索@1.0")
		if err := validJSON(t, s, `{"url":"https://example.com","title":"标题"}`); err != nil {
			t.Error(err)
		}
	})

	t.Run("E0301-T10 角色卡继承", func(t *testing.T) {
		r := builtinRegistry(t)
		card := func(name, extends string) *gen.Rolecard {
			c := &gen.Rolecard{Name: name, Version: 1, Identity: "测试", Mission: "测试"}
			if extends != "" {
				c.Extends = ptr(extends)
			}
			return c
		}
		if msg := installErr(t, r, &Package{Name: "p", RoleCards: []*gen.Rolecard{card("孤儿", "不存在的卡")}}); !strings.Contains(msg, "没有注册") {
			t.Errorf("父卡不存在：%q", msg)
		}
		if msg := installErr(t, r, &Package{Name: "p", RoleCards: []*gen.Rolecard{card("甲", "乙@1"), card("乙", "甲@1")}}); !strings.Contains(msg, "成环") {
			t.Errorf("继承成环：%q", msg)
		}
		if msg := installErr(t, r, &Package{Name: "p", RoleCards: []*gen.Rolecard{card("子卡", "智囊")}}); msg != "" {
			t.Fatal(msg)
		}
		c, ok := r.Live().RoleCard("子卡@1")
		if !ok || c.Extends == nil || *c.Extends != "智囊" || c.Identity != "测试" {
			t.Errorf("原样返回，不合并继承：%+v", c)
		}
	})
}
