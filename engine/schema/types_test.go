package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mio-cc/anywhere-door/engine/graph"
	"github.com/mio-cc/anywhere-door/gen"
)

func examplePath(name string) string { return filepath.Join("..", "..", "schema", "examples", name) }

func loadGraph(t testing.TB, name string) *gen.Graph {
	t.Helper()
	b, err := os.ReadFile(examplePath(name))
	if err != nil {
		t.Fatal(err)
	}
	var d gen.Graph
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	return &d
}

func mustFromDoc(t testing.TB, d *gen.Graph) *graph.Graph {
	t.Helper()
	g, err := graph.FromDoc(d)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestParse(t *testing.T) {
	t.Run("E0302-T1 TypeRef 解析", func(t *testing.T) {
		long := strings.Repeat("名", 30)
		for _, ok := range []string{"任意", "$T", "$U", "列表[方案@1]", "列表[$T]", "方案@1", "方案@1.2", "列表[列表[任意]]", "上下文包@1", long + "@0"} {
			ty, err := Parse(ok)
			if err != nil {
				t.Errorf("%s 应当合法：%v", ok, err)
				continue
			}
			if ty.String() != ok {
				t.Errorf("%s 往返成了 %s", ok, ty.String())
			}
		}
		for _, bad := range []string{"$t", "列表[]", "方案", "方案@x", "", "列表[方案@1", "方@1@2", "a b@1", long + "名@1", "$TT", "方案@1.", "方案@-1", "a/b@1"} {
			if _, err := Parse(bad); !errors.Is(err, ErrBadTypeRef) {
				t.Errorf("%q 应当是 ErrBadTypeRef：%v", bad, err)
			}
		}
		ty, _ := Parse("方案@1.2")
		if ty.Kind != Named || ty.Name != "方案" || ty.Major != 1 || ty.Minor != 2 {
			t.Errorf("%+v", ty)
		}
		l, _ := Parse("列表[$T]")
		if l.Kind != List || l.Elem.Kind != Var || l.Elem.Var != "T" || !l.HasVar() {
			t.Errorf("%+v", l)
		}
		if s := l.Subst(map[string]*Type{"T": ty}).String(); s != "列表[方案@1.2]" || l.String() != "列表[$T]" {
			t.Errorf("Subst：%s；原值不变：%s", s, l.String())
		}
		for ref, want := range map[string]string{"任意": "any", "列表[方案@1]": "list", "上下文包@1": "ctx", "资源引用@1": "ref", "工具端点@1": "tool", "状态@1": "state", "方案@1": "gen", "$T": "gen"} {
			ty, _ := Parse(ref)
			if got := Category(ty, "", nil); got != want {
				t.Errorf("Category(%s) = %s，想要 %s", ref, got, want)
			}
		}
		if got := Category(ty, "ctrl", nil); got != "ctrl" {
			t.Errorf("写了 PortDef.category 时用它：%s", got)
		}
	})
}

// vecTypes 是 assign_vectors.json 里的类型表。
type vecTypes struct {
	m    map[string]*gen.TypeDef
	msgs *Messages
}

func (v vecTypes) Type(ref string) (*gen.TypeDef, bool) {
	name, major, ok := splitNamed(ref)
	if !ok {
		return nil, false
	}
	d, ok := v.m[typeKey(name, major)]
	return d, ok
}

func (v vecTypes) Messages() *Messages { return v.msgs }

func TestAssignable(t *testing.T) {
	t.Run("E0302-T2 assign_vectors 全集", func(t *testing.T) {
		b, err := os.ReadFile(examplePath("assign_vectors.json"))
		if err != nil {
			t.Fatal(err)
		}
		var f struct {
			Types []struct {
				Name        string   `json:"name"`
				Major       int64    `json:"major"`
				Minor       int64    `json:"minor"`
				Extends     string   `json:"extends"`
				MigrateFrom []string `json:"migrate_from"`
				Producer    string   `json:"producer"`
			} `json:"types"`
			Vectors []struct {
				Name         string `json:"name"`
				Src          string `json:"src"`
				Dst          string `json:"dst"`
				SrcNode      string `json:"src_node"`
				DstContainer bool   `json:"dst_container"`
				Want         struct {
					OK      bool   `json:"ok"`
					Expand  bool   `json:"expand"`
					Migrate string `json:"migrate"`
					Rule    string `json:"rule"`
					Reason  string `json:"reason"`
				} `json:"want"`
			} `json:"vectors"`
		}
		if err := json.Unmarshal(b, &f); err != nil {
			t.Fatal(err)
		}
		ts := vecTypes{m: map[string]*gen.TypeDef{}, msgs: loadMessages(t, nil)}
		for _, ty := range f.Types {
			d := &gen.TypeDef{Name: ty.Name, Major: ty.Major, Minor: ty.Minor}
			if ty.Extends != "" {
				e := gen.TypeRef(ty.Extends)
				d.Extends = &e
			}
			if len(ty.MigrateFrom) > 0 {
				m := map[string]string{}
				for _, k := range ty.MigrateFrom {
					m[k] = "v"
				}
				d.MigrateFrom = &m
			}
			if ty.Producer != "" {
				p := gen.NodeTypeRef(ty.Producer)
				d.Producer = &p
			}
			ts.m[typeKey(ty.Name, ty.Major)] = d
		}
		if len(f.Vectors) < 40 {
			t.Fatalf("向量太少：%d", len(f.Vectors))
		}
		deref := func(p *string) string {
			if p == nil {
				return ""
			}
			return *p
		}
		for _, v := range f.Vectors {
			src, err1 := Parse(v.Src)
			dst, err2 := Parse(v.Dst)
			if err1 != nil || err2 != nil {
				t.Errorf("%s：%v %v", v.Name, err1, err2)
				continue
			}
			got := Assignable(src, dst, AssignAt{SrcNode: v.SrcNode, DstContainer: v.DstContainer}, ts)
			expand := got.Expand != nil && *got.Expand
			if got.Ok != v.Want.OK || got.Rule != v.Want.Rule || expand != v.Want.Expand || deref(got.Migrate) != v.Want.Migrate || deref(got.Reason) != v.Want.Reason {
				t.Errorf("%s（%s → %s）：想要 %+v，实际 ok=%v rule=%s expand=%v migrate=%q reason=%q", v.Name, v.Src, v.Dst, v.Want, got.Ok, got.Rule, expand, deref(got.Migrate), deref(got.Reason))
			}
		}
	})
}

// varPortsWithEdges 是图里有连线、声明类型带变量的端口。
func varPortsWithEdges(t *testing.T, g *graph.Graph, v View) []pkey {
	t.Helper()
	var out []pkey
	graph.Walk(g, func(n *graph.Node, s *graph.Scope, _ []*graph.Node) bool {
		def, ok := v.NodeDef(string(n.Type))
		if !ok {
			t.Errorf("节点类型 %s 没有注册", n.Type)
			return true
		}
		ins, outs := Ports(n, def, v)
		for name, p := range ins {
			if parseOrAny(string(p.Type)).HasVar() && len(s.In(string(n.ID), name)) > 0 {
				out = append(out, pkey{string(n.ID), name, DirIn})
			}
		}
		for name, p := range outs {
			if parseOrAny(string(p.Type)).HasVar() && len(s.Out(string(n.ID), name)) > 0 {
				out = append(out, pkey{string(n.ID), name, DirOut})
			}
		}
		return true
	})
	return out
}

// firstParamTyped 找图里第一个类型变量来自参数、带变量的输出端口有连线的节点（按 NodeDef 判断，不比较类型名）。
func firstParamTyped(t *testing.T, g *graph.Graph, v View) (*graph.Node, string, string) {
	t.Helper()
	var node *graph.Node
	var param, port string
	graph.Walk(g, func(n *graph.Node, s *graph.Scope, _ []*graph.Node) bool {
		def, ok := v.NodeDef(string(n.Type))
		if !ok || def.TypeVars == nil {
			return true
		}
		for _, src := range *def.TypeVars {
			p, ok := strings.CutPrefix(src, "param:")
			if !ok {
				continue
			}
			for name := range def.Ports.Outputs {
				if len(s.Out(string(n.ID), name)) > 0 && parseOrAny(string(def.Ports.Outputs[name].Type)).HasVar() {
					node, param, port = n, p, name
					return false
				}
			}
		}
		return true
	})
	if node == nil {
		t.Fatal("graph_loop.json 里没有类型来自参数、输出有连线的节点")
	}
	return node, param, port
}

func setParam(d *gen.Graph, id, name string, raw json.RawMessage) {
	var visit func(sg *gen.Subgraph)
	visit = func(sg *gen.Subgraph) {
		for i := range sg.Nodes {
			n := &sg.Nodes[i]
			if string(n.ID) == id {
				if n.Params == nil {
					n.Params = &map[string]json.RawMessage{}
				}
				(*n.Params)[name] = raw
			}
			if n.Children != nil {
				visit(n.Children)
			}
		}
	}
	visit(&d.Root)
}

func TestInfer(t *testing.T) {
	r := builtinRegistry(t)
	v := r.Live()

	t.Run("E0302-T3 推断模板", func(t *testing.T) {
		g := mustFromDoc(t, loadGraph(t, "graph_loop.json"))
		types, diags := Infer(g, v)
		for _, d := range diags {
			if d.Code == gen.DiagV16 {
				t.Errorf("不应当有 V16：%s（节点 %s 端口 %s）", d.Msg, ulid(d.Node), str(d.Port))
			}
		}
		res := types.(*inferResult)
		ports := varPortsWithEdges(t, g, v)
		if len(ports) == 0 {
			t.Fatal("graph_loop.json 里应当有带类型变量、有连线的端口")
		}
		for _, k := range ports {
			if res.unresolved[k] || types.Port(k.node, k.port, k.dir) == nil {
				t.Errorf("端口 %s.%s（%s）没有推断出类型", k.node, k.port, k.dir)
			}
		}
		n, param, port := firstParamTyped(t, g, v)
		def, _ := v.NodeDef(string(n.Type))
		want, _ := paramString(n, def, param)
		if got := types.Port(string(n.ID), port, DirOut); got == nil || got.String() != want {
			t.Errorf("%s 的 %s 应当是参数 %s 的值 %s：%v", n.ID, port, param, want, got)
		}

		// 把类型参数清空：下游有连线，报 V16 并给出建议（C.04.03 §5 验收 2）
		d := g.Doc()
		setParam(d, string(n.ID), param, json.RawMessage(`""`))
		_, diags = Infer(mustFromDoc(t, d), v)
		found := false
		for _, dg := range diags {
			if dg.Code == gen.DiagV16 && ulid(dg.Node) == string(n.ID) && str(dg.Param) == param && strings.Contains(dg.Msg, paramLabel(def, param)) {
				found = true
			}
		}
		if !found {
			t.Errorf("清空 %s 后应当在 %s 上报 V16：%+v", param, n.ID, diags)
		}
	})

	t.Run("E0302-T4 推断缓存失效正确", func(t *testing.T) {
		d := loadGraph(t, "graph_loop.json")
		g := mustFromDoc(t, d)
		n, param, port := firstParamTyped(t, g, v)
		var c InferCache
		c.Infer(g, v)
		c.Infer(mustFromDoc(t, g.Doc()), v)
		if c.Misses() != 1 {
			t.Fatalf("同样的图应当命中缓存：重新计算了 %d 次", c.Misses())
		}
		d2 := g.Doc()
		setParam(d2, string(n.ID), param, json.RawMessage(`"判定@1"`))
		types, _ := c.Infer(mustFromDoc(t, d2), v)
		if c.Misses() != 2 {
			t.Errorf("改了参数要重新计算：%d", c.Misses())
		}
		if got := types.Port(string(n.ID), port, DirOut); got == nil || got.String() != "判定@1" {
			t.Errorf("结果要反映改动：%v", got)
		}
	})

	t.Run("E0302-T5 推断性能", func(t *testing.T) {
		g := mustFromDoc(t, chain500())
		types, diags := Infer(g, v)
		if len(diags) != 0 {
			t.Fatalf("链式图不应当有诊断：%+v", diags)
		}
		if got := types.Port(fmt.Sprintf("P%04d", 499), "out", DirOut); got == nil || got.String() != "资源引用@1" {
			t.Fatalf("类型要沿 499 条连线传到最后：%v", got)
		}
		if os.Getenv("ANYDOOR_PERF") != "1" {
			t.Log("没有设 ANYDOOR_PERF=1：只检查能推断，不按 10ms 判（P.09 K238 的做法）")
			return
		}
		res := testing.Benchmark(func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				Infer(g, v)
			}
		})
		if per := time.Duration(res.NsPerOp()); per > 10*time.Millisecond {
			t.Errorf("500 节点推断用了 %v，超过 10ms", per)
		}
	})

	t.Run("E0302-T6 动态端口", func(t *testing.T) {
		def, ok := v.NodeDef("资源")
		if !ok || def.DynamicPorts == nil {
			t.Fatal("资源节点应当有动态端口")
		}
		n := &graph.Node{Node: gen.Node{ID: "R", Type: "资源", Params: &map[string]json.RawMessage{
			def.DynamicPorts.FromParam: json.RawMessage(`"你好{{name}}，{{ topic }}和{{name}}{{Bad}}{{ 中文 }}"`),
		}}}
		in, _ := Ports(n, def, v)
		var dyn []string
		for k := range in {
			if _, static := def.Ports.Inputs[k]; !static {
				dyn = append(dyn, k)
			}
		}
		if len(dyn) != 2 || in["name"] == nil || in["topic"] == nil || string(in["name"].Type) != "任意" {
			t.Errorf("提示词变量去重后每个一个端口，不合法的名字不生成：%v", dyn)
		}
		if got := TemplateVars("{{a}}{{ b }}{{a}}"); !reflect.DeepEqual(got, []string{"a", "b"}) {
			t.Errorf("TemplateVars：%v", got)
		}

		b, err := os.ReadFile(examplePath("schema_expand_vectors.json"))
		if err != nil {
			t.Fatal(err)
		}
		var vs []struct {
			Name       string          `json:"name"`
			Reserved   []string        `json:"reserved"`
			Schema     json.RawMessage `json:"schema"`
			Expandable bool            `json:"expandable"`
			Params     []struct {
				Key   string          `json:"key"`
				Param json.RawMessage `json:"param"`
				Group string          `json:"group"`
				Port  string          `json:"port"`
			} `json:"params"`
		}
		if err := json.Unmarshal(b, &vs); err != nil {
			t.Fatal(err)
		}
		norm := func(x any) string {
			b, _ := json.Marshal(x)
			dec := json.NewDecoder(bytes.NewReader(b))
			var y any
			_ = dec.Decode(&y)
			out, _ := json.Marshal(y)
			return string(out)
		}
		var first json.RawMessage
		var firstPorts []string
		for _, vec := range vs {
			ok, got := ExpandSchema(vec.Schema, vec.Reserved)
			if ok != vec.Expandable || len(got) != len(vec.Params) {
				t.Errorf("%s：expandable=%v（想要 %v），%d 个子参数（想要 %d）", vec.Name, ok, vec.Expandable, len(got), len(vec.Params))
				continue
			}
			for i, want := range vec.Params {
				g := got[i]
				if g.Key != want.Key || g.Group != want.Group || g.Port != want.Port || norm(g.Param) != norm(want.Param) {
					t.Errorf("%s 第 %d 个：想要 %s/%s/%s %s，实际 %s/%s/%s %s", vec.Name, i, want.Key, want.Group, want.Port, want.Param, g.Key, g.Group, g.Port, norm(g.Param))
				}
			}
			if ok && first == nil {
				first = vec.Schema
				for _, p := range vec.Params {
					firstPorts = append(firstPorts, p.Port)
				}
			}
		}

		// schema_from：视图提供所选对象的入参 schema 时，每个子参数一个 任意 端口
		var tool *gen.NodeDef
		for _, d := range v.NodeDefs() {
			if d.DynamicPorts != nil {
				if p := d.Params[d.DynamicPorts.FromParam]; p.SchemaFrom != nil {
					tool = d
				}
			}
		}
		if tool == nil || first == nil {
			t.Fatal("没有 schema_from 动态端口的节点定义，或者没有能展开的向量")
		}
		tn := &graph.Node{Node: gen.Node{ID: "T", Type: gen.NodeTypeRef(tool.Name)}}
		in, _ = Ports(tn, tool, v)
		if len(in) != len(tool.Ports.Inputs) {
			t.Errorf("视图不提供 schema 时不生成端口：%d", len(in))
		}
		in, _ = Ports(tn, tool, withSchema{View: v, schema: first})
		for _, p := range firstPorts {
			if in[p] == nil || string(in[p].Type) != "任意" {
				t.Errorf("schema_from 展开的端口 %s 应当是 任意", p)
			}
		}
		if len(in) != len(tool.Ports.Inputs)+len(firstPorts) {
			t.Errorf("端口数：%d", len(in))
		}
	})
}

// extraTypes 在内置视图上多加几个类型（E0302-T8 的嵌套字段）。
type extraTypes struct {
	View
	types map[string]*gen.TypeDef
}

func (e extraTypes) Type(ref string) (*gen.TypeDef, bool) {
	if d, ok := e.types[ref]; ok {
		return d, true
	}
	return e.View.Type(ref)
}

// aggDoc：Agent（output_schema = schema）→ 聚合的 port 端口（op、list_field 按参数）→ 子图端口，让 result 有连线。
func aggDoc(schema, port, op, listField string) *gen.Graph {
	raw := func(s string) json.RawMessage { b, _ := json.Marshal(s); return b }
	ap := map[string]json.RawMessage{"output_schema": raw(schema)}
	gp := map[string]json.RawMessage{"op": raw(op)}
	if listField != "" {
		gp["list_field"] = raw(listField)
	}
	d := &gen.Graph{ID: "01JDAGG0000000000000000001", Branch: "main", SchemaVersion: "1.0"}
	d.Root.Nodes = []gen.Node{
		{ID: "A", Type: "Agent", Params: &ap},
		{ID: "G", Type: "聚合", Params: &gp},
		{ID: "P", Type: "子图端口"},
	}
	d.Root.Edges = []gen.Edge{
		{ID: "E1", From: gen.PortRef{Node: "A", Port: "result"}, To: gen.PortRef{Node: "G", Port: port}},
		{ID: "E2", From: gen.PortRef{Node: "G", Port: "result"}, To: gen.PortRef{Node: "P", Port: "in"}},
	}
	return d
}

func TestInferField(t *testing.T) {
	r := builtinRegistry(t)
	base := r.Live()
	strp := func(s string) *string { return &s }
	nested := []gen.Field{{Name: "a", Type: "object", Fields: &[]gen.Field{{Name: "b", Type: "列表[方案@1]"}, {Name: "c", Type: "string"}}}}
	v := extraTypes{View: base, types: map[string]*gen.TypeDef{"嵌套@1": {Name: "嵌套", Major: 1, Fields: &nested, Description: strp("测试用")}}}

	t.Run("E0302-T8 字段来源与备选来源", func(t *testing.T) {
		cases := []struct {
			name, schema, port, op, field string
			items, result                 string
		}{
			{"聚类 → 按比例分级", "簇标注@1", "value", "group_ratio", "labels", "列表[任意]", "聚合结果@1"},
			{"采集 → 按来源去重", "执行结果@1", "value", "dedupe", "sources", "列表[任意]", "列表[任意]"},
			{"嵌套字段", "嵌套@1", "value", "vote", "a.b", "列表[方案@1]", "方案@1"},
			{"接 items 照旧取元素", "列表[方案@1]", "items", "vote", "", "列表[方案@1]", "方案@1"},
			{"接 items 时 list_field 不起作用", "列表[方案@1]", "items", "dedupe", "labels", "列表[方案@1]", "列表[方案@1]"},
		}
		for _, c := range cases {
			types, diags := Infer(mustFromDoc(t, aggDoc(c.schema, c.port, c.op, c.field)), v)
			for _, d := range diags {
				if d.Code == gen.DiagV16 {
					t.Errorf("%s：不应当有 V16：%s", c.name, d.Msg)
				}
			}
			if got := types.Port("G", "items", DirIn); got == nil || got.String() != c.items {
				t.Errorf("%s：items 应当是 %s：%v", c.name, c.items, got)
			}
			if got := types.Port("G", "result", DirOut); got == nil || got.String() != c.result {
				t.Errorf("%s：result 应当是 %s：%v", c.name, c.result, got)
			}
		}
		// 字段写错、不是列表、没写：value 接了线，在 list_field 上报 V16
		def, _ := v.NodeDef("聚合")
		for _, field := range []string{"nope", "understanding_consistent", "", "a.c", "a.nope"} {
			schema := "簇标注@1"
			if strings.HasPrefix(field, "a.") {
				schema = "嵌套@1"
			}
			_, diags := Infer(mustFromDoc(t, aggDoc(schema, "value", "dedupe", field)), v)
			found := false
			for _, d := range diags {
				if d.Code == gen.DiagV16 && ulid(d.Node) == "G" && str(d.Param) == "list_field" && str(d.Port) == "value" && strings.Contains(d.Msg, paramLabel(def, "list_field")) {
					found = true
				}
			}
			if !found {
				t.Errorf("list_field = %q：应当在 G 的 value / list_field 上报 V16：%+v", field, diags)
			}
		}
		// 上游是 任意：取 任意，不报 V16
		types, diags := Infer(mustFromDoc(t, aggDoc("任意", "value", "dedupe", "labels")), v)
		if len(diags) != 0 {
			t.Errorf("上游是任意时不应当有诊断：%+v", diags)
		}
		if got := types.Port("G", "result", DirOut); got == nil || got.String() != "列表[任意]" {
			t.Errorf("上游是任意时 result 应当是 列表[任意]：%v", got)
		}
	})
}

type withSchema struct {
	View
	schema json.RawMessage
}

func (w withSchema) ObjectSchema(string) (json.RawMessage, bool) { return w.schema, true }

// chain500 是 1 个资源节点加 499 个子图端口节点连成的链（类型沿 input:in 一路传下去）。
func chain500() *gen.Graph {
	d := &gen.Graph{ID: "01JDPERF000000000000000002", Branch: "main", SchemaVersion: "1.0"}
	d.Root.Nodes = append(d.Root.Nodes, gen.Node{ID: "P0000", Type: "资源"})
	for i := 1; i < 500; i++ {
		id := fmt.Sprintf("P%04d", i)
		d.Root.Nodes = append(d.Root.Nodes, gen.Node{ID: gen.Ulid(id), Type: "子图端口"})
		from := gen.PortRef{Node: gen.Ulid(fmt.Sprintf("P%04d", i-1)), Port: "out"}
		if i == 1 {
			from.Port = "ref"
		}
		d.Root.Edges = append(d.Root.Edges, gen.Edge{ID: gen.Ulid("E" + strconv.Itoa(i)), From: from, To: gen.PortRef{Node: gen.Ulid(id), Port: "in"}})
	}
	return d
}

func BenchmarkInfer500(b *testing.B) {
	r := NewRegistry(Options{})
	if err := r.LoadBuiltin(repoFS()); err != nil {
		b.Fatal(err)
	}
	v := r.Live()
	g, err := graph.FromDoc(chain500())
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Infer(g, v)
	}
}
