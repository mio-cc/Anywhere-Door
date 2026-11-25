package schema

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mio-cc/anywhere-door/gen"
)

func vid(n int) gen.Ulid { return gen.Ulid(fmt.Sprintf("01JDVT%020d", n)) }
func eid(n int) gen.Ulid { return gen.Ulid(fmt.Sprintf("01JDET%020d", n)) }

// vnode 是一个节点；kv 是参数名、值交替。
func vnode(n int, typ string, kv ...any) gen.Node {
	nd := gen.Node{ID: vid(n), Type: gen.NodeTypeRef(typ)}
	if len(kv) > 0 {
		p := map[string]json.RawMessage{}
		for i := 0; i+1 < len(kv); i += 2 {
			b, _ := json.Marshal(kv[i+1])
			p[kv[i].(string)] = b
		}
		nd.Params = &p
	}
	return nd
}

func vedge(n, from int, fport string, to int, tport string) gen.Edge {
	return gen.Edge{ID: eid(n), From: gen.PortRef{Node: vid(from), Port: fport}, To: gen.PortRef{Node: vid(to), Port: tport}}
}

func vsub(nodes []gen.Node, edges []gen.Edge) *gen.Subgraph {
	if nodes == nil {
		nodes = []gen.Node{}
	}
	if edges == nil {
		edges = []gen.Edge{}
	}
	return &gen.Subgraph{Nodes: nodes, Edges: edges}
}

func withChildren(n gen.Node, sg *gen.Subgraph) gen.Node {
	n.Children = sg
	return n
}

// vdoc 从 graph_minimal.json 改出一张图（E.03.03 实施卡：正反例在测试里从它改出来）。
func vdoc(t testing.TB, nodes []gen.Node, edges []gen.Edge) *gen.Graph {
	t.Helper()
	d := loadGraph(t, "graph_minimal.json")
	d.Root = *vsub(nodes, edges)
	return d
}

func vrun(t *testing.T, v View, d *gen.Graph, forRun bool) []gen.Diag {
	t.Helper()
	return Validate(mustFromDoc(t, d), v, Opts{ForRun: forRun})
}

// onlyCode：有诊断，而且都是这个码。
func onlyCode(t *testing.T, name string, ds []gen.Diag, code gen.DiagCode) {
	t.Helper()
	if len(ds) == 0 {
		t.Errorf("%s：应当报 %s，实际没有诊断", name, code)
	}
	for _, d := range ds {
		if d.Code != code {
			t.Errorf("%s：只应当报 %s，还报了 %s（节点 %s 端口 %s）：%s", name, code, d.Code, ulid(d.Node), str(d.Port), d.Msg)
		}
	}
}

func noDiag(t *testing.T, name string, ds []gen.Diag) {
	t.Helper()
	for _, d := range ds {
		t.Errorf("%s：不应当有诊断：%s %s（节点 %s 端口 %s 参数 %s）：%s", name, d.Level, d.Code, ulid(d.Node), str(d.Port), str(d.Param), d.Msg)
	}
}

// ctxAgent：资源 1 → 上下文装配 2 → Agent 3 的 context，资源 4 → Agent 3 的 model。
func ctxAgent(params ...any) ([]gen.Node, []gen.Edge) {
	return []gen.Node{vnode(1, "资源"), vnode(2, "上下文装配"), vnode(3, "Agent", params...), vnode(4, "资源")},
		[]gen.Edge{vedge(1, 1, "ref", 2, "model"), vedge(2, 2, "context", 3, "context"), vedge(3, 4, "ref", 3, "model")}
}

// loopWith 是一个循环节点（ID 10）带入口 11、出口 12 的子图。
func loopWith(kv ...any) gen.Node {
	return withChildren(vnode(10, "循环", kv...), vsub(
		[]gen.Node{vnode(11, "子图端口", "role", "entry"), vnode(12, "子图端口", "role", "exit")},
		[]gen.Edge{vedge(11, 11, "out", 12, "in")}))
}

// nest 是嵌套 depth 层的子图引用（每层有入口、出口和下一层），照 semantic/nesting_depth_*.json 的写法。
func nest(depth int) gen.Node {
	var inner *gen.Node
	for lvl := depth; lvl >= 1; lvl-- {
		nodes := []gen.Node{vnode(200+lvl, "子图端口", "role", "entry"), vnode(300+lvl, "子图端口", "role", "exit")}
		edges := []gen.Edge{vedge(200+lvl, 200+lvl, "out", 300+lvl, "in")}
		if inner != nil {
			nodes = append(nodes, *inner)
			edges = []gen.Edge{vedge(200+lvl, 200+lvl, "out", 100+lvl+1, "in"), vedge(300+lvl, 100+lvl+1, "out", 300+lvl, "in")}
		}
		n := withChildren(vnode(100+lvl, "子图引用"), vsub(nodes, edges))
		inner = &n
	}
	return *inner
}

func TestValidateRules(t *testing.T) {
	v := builtinRegistry(t).Live()

	t.Run("E0303-T01 V01", func(t *testing.T) {
		noDiag(t, "graph_minimal", vrun(t, v, vdoc(t, nil, nil), false))
		d := vdoc(t, nil, nil)
		d.Branch = "feature/x"
		onlyCode(t, "branch 不合法", vrun(t, v, d, false), gen.DiagV01)
	})
	t.Run("E0303-T02 V02", func(t *testing.T) {
		noDiag(t, "账本", vrun(t, v, vdoc(t, []gen.Node{vnode(1, "账本")}, nil), false))
		// 类型名要符合 graph.schema.json 的写法，否则先报 V01
		onlyCode(t, "没有注册的类型", vrun(t, v, vdoc(t, []gen.Node{vnode(1, "test-pack/没有这种节点@1")}, nil), false), gen.DiagV02)
	})
	t.Run("E0303-T03 V03", func(t *testing.T) {
		noDiag(t, "合法参数", vrun(t, v, vdoc(t, []gen.Node{vnode(1, "账本", "op", "read", "path", "目标树"), vnode(2, "资源", "kind", "列表", "value", []any{1, 2})}, nil), false))
		for name, n := range map[string]gen.Node{
			"枚举外的值":  vnode(1, "账本", "op", "nope"),
			"没有这个参数": vnode(1, "账本", "nope_param", 1),
			"超出范围":   vnode(1, "汇合", "k", 0),
			"表达式写错":  vnode(1, "聚合", "expr", "1 +"),
			"配方不存在":  vnode(1, "上下文装配", "recipe", "没有的配方@1"),
			"类型没有注册": vnode(1, "工具", "result_type", "没有的类型@1"),
			"类型写法不对": vnode(1, "工具", "result_type", "方案@"),
			"值的类型不对": vnode(1, "汇合", "k", "三"),
			"角色卡不存在": vnode(1, "资源", "kind", "角色卡", "rolecard", "没有的卡@1"),
		} {
			ds := vrun(t, v, vdoc(t, []gen.Node{n}, nil), false)
			onlyCode(t, name, ds, gen.DiagV03)
			for _, d := range ds {
				if str(d.Param) == "" {
					t.Errorf("%s：V03 要写参数名：%+v", name, d)
				}
			}
		}
	})
	t.Run("E0303-T04 V04", func(t *testing.T) {
		noDiag(t, "等于固定值", vrun(t, v, vdoc(t, []gen.Node{vnode(1, "聚合", "deterministic", true)}, nil), false))
		ds := vrun(t, v, vdoc(t, []gen.Node{vnode(1, "聚合", "deterministic", false)}, nil), false)
		onlyCode(t, "改了 kernel 锁参数", ds, gen.DiagV04)
		if len(ds) == 1 && str(ds[0].Param) != "deterministic" {
			t.Errorf("V04 要写参数名：%+v", ds[0])
		}
	})
	t.Run("E0303-T05 V05", func(t *testing.T) {
		noDiag(t, "端口都对", vrun(t, v, vdoc(t, []gen.Node{vnode(1, "资源"), vnode(2, "账本"), vnode(3, "汇合")},
			[]gen.Edge{vedge(1, 1, "ref", 2, "value"), vedge(2, 1, "ref", 3, "in[0]"), vedge(3, 2, "value", 3, "in[1]")}), false))
		for name, e := range map[string]gen.Edge{
			"没有的输出端口":   vedge(1, 1, "nope", 2, "value"),
			"没有的输入端口":   vedge(1, 1, "ref", 2, "nope"),
			"连进输出端口":    vedge(1, 1, "ref", 3, "collected"),
			"不是动态端口的插口": vedge(1, 1, "ref", 2, "value[0]"),
			"目标节点不存在":   vedge(1, 1, "ref", 99, "value"),
		} {
			onlyCode(t, name, vrun(t, v, vdoc(t, []gen.Node{vnode(1, "资源"), vnode(2, "账本"), vnode(3, "汇合")}, []gen.Edge{e}), false), gen.DiagV05)
		}
	})
	t.Run("E0303-T06 V06", func(t *testing.T) {
		noDiag(t, "列表接列表", vrun(t, v, vdoc(t, []gen.Node{vnode(1, "资源", "kind", "列表", "value", []any{}), vnode(2, "聚合")}, []gen.Edge{vedge(1, 1, "items", 2, "items")}), false))
		ds := vrun(t, v, vdoc(t, []gen.Node{vnode(1, "资源"), vnode(2, "聚合")}, []gen.Edge{vedge(1, 1, "ref", 2, "items")}), false)
		onlyCode(t, "单个值接列表（C8）", ds, gen.DiagV06)
		// 系统类型的端口接了不是 producer 的节点（不是内核锁定的端口）：报 V06，原因是 C2 的文案
		nodes, edges := ctxAgent()
		nodes = append(nodes, vnode(5, "账本"))
		edges[2] = vedge(3, 5, "value", 3, "model")
		ds = vrun(t, v, vdoc(t, nodes, edges), false)
		onlyCode(t, "账本接 model（C2）", ds, gen.DiagV06)
		if len(ds) == 1 && !strings.Contains(ds[0].Msg, "资源") {
			t.Errorf("C2 的原因要说明只能来自资源节点：%s", ds[0].Msg)
		}
	})
	t.Run("E0303-T07 V07", func(t *testing.T) {
		noDiag(t, "多连端口接两条", vrun(t, v, vdoc(t, []gen.Node{vnode(1, "资源"), vnode(2, "资源"), vnode(3, "上下文装配")},
			[]gen.Edge{vedge(1, 1, "ref", 3, "in"), vedge(2, 2, "ref", 3, "in")}), false))
		onlyCode(t, "单连端口接两条", vrun(t, v, vdoc(t, []gen.Node{vnode(1, "资源"), vnode(2, "资源"), vnode(3, "账本")},
			[]gen.Edge{vedge(1, 1, "ref", 3, "value"), vedge(2, 2, "ref", 3, "value")}), false), gen.DiagV07)
		onlyCode(t, "重复连线", vrun(t, v, vdoc(t, []gen.Node{vnode(1, "资源"), vnode(3, "上下文装配")},
			[]gen.Edge{vedge(1, 1, "ref", 3, "in"), vedge(2, 1, "ref", 3, "in")}), false), gen.DiagV07)
	})
	t.Run("E0303-T08 V08", func(t *testing.T) {
		nodes, edges := ctxAgent()
		noDiag(t, "必填都接了", vrun(t, v, vdoc(t, nodes, edges), false))
		ds := vrun(t, v, vdoc(t, []gen.Node{vnode(3, "Agent")}, nil), false)
		onlyCode(t, "Agent 没接 context、model", ds, gen.DiagV08)
		if len(ds) != 2 {
			t.Errorf("context、model 各一条：%+v", ds)
		}
	})
	t.Run("E0303-T09 V09", func(t *testing.T) {
		noDiag(t, "没有环", vrun(t, v, vdoc(t, []gen.Node{vnode(1, "账本"), vnode(2, "账本")}, []gen.Edge{vedge(1, 1, "value", 2, "value")}), false))
		onlyCode(t, "两个节点成环", vrun(t, v, vdoc(t, []gen.Node{vnode(1, "账本"), vnode(2, "账本")},
			[]gen.Edge{vedge(1, 1, "value", 2, "value"), vedge(2, 2, "value", 1, "value")}), false), gen.DiagV09)
	})
	t.Run("E0303-T10 V10", func(t *testing.T) {
		noDiag(t, "按轮次", vrun(t, v, vdoc(t, []gen.Node{loopWith()}, nil), false))
		noDiag(t, "按事件、有退出条件", vrun(t, v, vdoc(t, []gen.Node{loopWith("trigger", "event")}, nil), false))
		onlyCode(t, "按事件、没有出口", vrun(t, v, vdoc(t, []gen.Node{loopWith("trigger", "event", "exits", "")}, nil), false), gen.DiagV10)
	})
	t.Run("E0303-T11 V11", func(t *testing.T) {
		noDiag(t, "循环带入口出口", vrun(t, v, vdoc(t, []gen.Node{loopWith()}, nil), false))
		onlyCode(t, "循环没有子图", vrun(t, v, vdoc(t, []gen.Node{vnode(10, "循环")}, nil), false), gen.DiagV11)
		onlyCode(t, "子图只有入口", vrun(t, v, vdoc(t, []gen.Node{withChildren(vnode(10, "子图引用"), vsub([]gen.Node{vnode(11, "子图端口", "role", "entry")}, nil))}, nil), false), gen.DiagV11)
		onlyCode(t, "普通节点带子图", vrun(t, v, vdoc(t, []gen.Node{withChildren(vnode(10, "账本"), loopWith().Children)}, nil), false), gen.DiagV11)
	})
	t.Run("E0303-T12 V12", func(t *testing.T) {
		noDiag(t, "4 层", vrun(t, v, vdoc(t, []gen.Node{nest(4)}, nil), true))
		ds := vrun(t, v, vdoc(t, []gen.Node{nest(5)}, nil), true)
		onlyCode(t, "5 层", ds, gen.DiagV12)
		if HasError(ds) {
			t.Errorf("5 层只是 warn：%+v", ds)
		}
		ds = vrun(t, v, vdoc(t, []gen.Node{nest(9)}, nil), true)
		onlyCode(t, "9 层", ds, gen.DiagV12)
		if !HasError(ds) {
			t.Errorf("9 层是 error：%+v", ds)
		}
	})
	t.Run("E0303-T14 V14", func(t *testing.T) {
		nodes, edges := ctxAgent()
		noDiag(t, "context 接上下文装配", vrun(t, v, vdoc(t, nodes, edges), false))
		edges[1] = vedge(2, 1, "ref", 3, "context")
		onlyCode(t, "context 接资源", vrun(t, v, vdoc(t, nodes, edges), false), gen.DiagV14)
	})
	t.Run("E0303-T16 V16", func(t *testing.T) {
		nodes, edges := ctxAgent("output_schema", "方案@1")
		nodes = append(nodes, vnode(5, "账本"))
		edges = append(edges, vedge(4, 3, "result", 5, "value"))
		noDiag(t, "设了输出类型", vrun(t, v, vdoc(t, nodes, edges), false))
		nodes[2] = vnode(3, "Agent", "output_schema", "")
		onlyCode(t, "输出类型清空", vrun(t, v, vdoc(t, nodes, edges), false), gen.DiagV16)
	})
	t.Run("E0303-T17 V17", func(t *testing.T) {
		noDiag(t, "边界节点不算", vrun(t, v, vdoc(t, []gen.Node{vnode(1, "子图端口")}, nil), true))
		ds := vrun(t, v, vdoc(t, []gen.Node{vnode(1, "账本")}, nil), true)
		onlyCode(t, "孤立的账本", ds, gen.DiagV17)
		if HasError(ds) {
			t.Errorf("V17 是 warn：%+v", ds)
		}
	})
}

type panicRule struct{}

func (panicRule) Code() gen.DiagCode        { return gen.DiagV17 }
func (panicRule) Check(*Context) []gen.Diag { panic("测试用的 panic") }

// chainV 是 1 个资源节点加 499 个子图端口节点连成的链（ID 是合法的 ULID，校验没有诊断）。
func chainV() *gen.Graph {
	d := &gen.Graph{ID: "01JDPERF000000000000000003", Branch: "main", SchemaVersion: "1.0"}
	d.Root = *vsub(nil, nil)
	d.Root.Nodes = append(d.Root.Nodes, vnode(0, "资源"))
	for i := 1; i < 500; i++ {
		d.Root.Nodes = append(d.Root.Nodes, vnode(i, "子图端口"))
		port := "out"
		if i == 1 {
			port = "ref"
		}
		d.Root.Edges = append(d.Root.Edges, vedge(i, i-1, port, i, "in"))
	}
	return d
}

func TestValidate(t *testing.T) {
	v := builtinRegistry(t).Live()

	t.Run("E0303-T20 校验性能", func(t *testing.T) {
		g := mustFromDoc(t, chainV())
		prev := Validate(g, v, Opts{})
		noDiag(t, "500 节点的链", prev)
		if os.Getenv("ANYDOOR_PERF") != "1" {
			t.Log("没有设 ANYDOOR_PERF=1：只检查能校验，不按 20ms / 5ms 判（P.09 K238 的做法）")
			return
		}
		touched := Touched{Nodes: []string{string(vid(250))}}
		full := testing.Benchmark(func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				Validate(g, v, Opts{})
			}
		})
		inc := testing.Benchmark(func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ValidateIncremental(g, prev, touched, v, Opts{})
			}
		})
		if per := time.Duration(full.NsPerOp()); per > 20*time.Millisecond {
			t.Errorf("500 节点全量校验用了 %v，超过 20ms", per)
		}
		if per := time.Duration(inc.NsPerOp()); per > 5*time.Millisecond {
			t.Errorf("500 节点增量校验用了 %v，超过 5ms", per)
		}
	})

	t.Run("E0303-T21 规则 panic 隔离", func(t *testing.T) {
		g := mustFromDoc(t, vdoc(t, []gen.Node{vnode(1, "test-pack/没有这种节点@1")}, nil))
		want := Validate(g, v, Opts{})
		got := run(g, v, Opts{}, append(append([]Rule(nil), rules...), panicRule{}), nil, nil)
		var rest []gen.Diag
		internal := 0
		for _, d := range got {
			if d.Code == gen.DiagV01 && d.Msg == "校验器内部错误：V17" {
				internal++
				continue
			}
			rest = append(rest, d)
		}
		if internal != 1 {
			t.Errorf("应当多一条 V01「校验器内部错误：V17」：%+v", got)
		}
		if !reflect.DeepEqual(rest, want) {
			t.Errorf("其他规则的结果要照常：\n%+v\n%+v", rest, want)
		}
	})

	t.Run("E0303-T22 semantic 示例", func(t *testing.T) {
		f, err := os.Open(examplePath("manifest.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = f.Close() }()
		want := map[string]gen.DiagCode{}
		file := ""
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line, _, _ := strings.Cut(sc.Text(), "#")
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "- file:"):
				file = strings.TrimSpace(strings.TrimPrefix(line, "- file:"))
			case strings.HasPrefix(line, "diag:") && strings.HasPrefix(file, "semantic/"):
				want[file] = gen.DiagCode(strings.TrimSpace(strings.TrimPrefix(line, "diag:")))
			}
		}
		if err := sc.Err(); err != nil {
			t.Fatal(err)
		}
		if len(want) < 5 {
			t.Fatalf("清单里写了 diag 的 semantic 文件太少：%v", want)
		}
		for _, file := range sortedKeys(want) {
			code := want[file]
			ds := vrun(t, v, loadGraph(t, file), true)
			found := false
			for _, d := range ds {
				switch {
				case d.Code == code:
					found = true
				case d.Level == gen.DiagLevelError:
					t.Errorf("%s：只应当触发 %s，还有 error %s：%s", file, code, d.Code, d.Msg)
				}
			}
			if !found {
				t.Errorf("%s：应当触发 %s：%+v", file, code, ds)
			}
		}
		for _, name := range []string{"graph_minimal.json", "graph_loop.json"} {
			ds := vrun(t, v, loadGraph(t, name), true)
			for _, d := range ds {
				if d.Level == gen.DiagLevelError {
					t.Errorf("%s 不应当有 error：%s（节点 %s 端口 %s 参数 %s）：%s", name, d.Code, ulid(d.Node), str(d.Port), str(d.Param), d.Msg)
				} else {
					t.Logf("%s 的 warn：%s %s", name, d.Code, d.Msg)
				}
			}
		}
	})

	t.Run("E0303-T23 增量与全量一致", func(t *testing.T) {
		opt := Opts{ForRun: true}
		base := loadGraph(t, "graph_loop.json")
		g0 := mustFromDoc(t, base)
		prev := Validate(g0, v, opt)
		changes := loopChanges(t)
		if len(changes) != 10 {
			t.Fatalf("应当有 10 种改动：%d", len(changes))
		}
		some := 0
		for _, ch := range changes {
			d := g0.Doc()
			touched := ch.apply(d)
			g := mustFromDoc(t, d)
			full := Validate(g, v, opt)
			inc := ValidateIncremental(g, prev, touched, v, opt)
			if len(full) > 0 {
				some++
			}
			if !reflect.DeepEqual(full, inc) {
				t.Errorf("%s：增量结果和全量不同：\n全量 %+v\n增量 %+v", ch.name, full, inc)
			}
		}
		if some < 5 {
			t.Errorf("改动后应当大多有诊断（否则没测到东西）：%d", some)
		}
	})

	t.Run("E0303-T24 运行前校验", func(t *testing.T) {
		d := vdoc(t, []gen.Node{vnode(1, "账本")}, nil)
		noDiag(t, "编辑时不返回 warn", vrun(t, v, d, false))
		ds := vrun(t, v, d, true)
		onlyCode(t, "运行前返回 warn", ds, gen.DiagV17)
		if HasError(ds) {
			t.Error("只有 warn 时 HasError 应当为 false")
		}
		ds = vrun(t, v, vdoc(t, []gen.Node{vnode(1, "账本"), vnode(2, "test-pack/没有这种节点@1")}, nil), true)
		if !HasError(ds) {
			t.Errorf("有 error 时 HasError 应当为 true：%+v", ds)
		}
	})

	t.Run("E0303-T25 边界回溯与子图引用端口", func(t *testing.T) {
		// 扇出子图里入口端口节点接 Agent 的 context：上游是上下文装配不报，是资源报 V14
		fan := func(upstream string, port string) *gen.Graph {
			inner := vsub(
				[]gen.Node{vnode(11, "子图端口", "role", "entry", "port_name", "shared"), vnode(12, "资源"), vnode(13, "Agent", "output_schema", "方案@1"), vnode(14, "子图端口", "role", "exit", "port_name", "results")},
				[]gen.Edge{vedge(11, 11, "out", 13, "context"), vedge(12, 12, "ref", 13, "model"), vedge(13, 13, "result", 14, "in")})
			return vdoc(t, []gen.Node{vnode(1, upstream), withChildren(vnode(10, "扇出"), inner)}, []gen.Edge{vedge(1, 1, port, 10, "shared")})
		}
		noDiag(t, "上游是上下文装配", vrun(t, v, fan("上下文装配", "context"), false))
		onlyCode(t, "上游是资源", vrun(t, v, fan("资源", "ref"), false), gen.DiagV14)

		loop := func(edit func(d *gen.Graph)) []gen.Diag {
			d := loadGraph(t, "graph_loop.json")
			edit(d)
			return vrun(t, v, d, false)
		}
		noDiag(t, "graph_loop（入口接 model、tools，子图引用的 in / out）", loop(func(*gen.Graph) {}))
		// 智囊扇出的列表换成任务卡：入口接 model 的源头不是资源，报 V06（model 不是内核锁定的端口）
		// 另加一份决策者（配方 + Agent），不在智囊团下游：直接改接决策者会成环，另报 V09
		onlyCode(t, "扇出的列表来自 Agent", loop(func(d *gen.Graph) {
			e := findEdge(t, d, func(e *gen.Edge) bool { return e.From.Port == "items" && e.To.Port == "items" })
			box := findNode(t, d, func(n *gen.Node) bool {
				if n.Children == nil {
					return false
				}
				for _, x := range n.Children.Edges {
					if x.ID == e.ID {
						return true
					}
				}
				return false
			})
			sg := box.Children
			var recipe, planner, model gen.Node
			for _, n := range sg.Nodes {
				switch {
				case n.Type == "上下文装配" && param(&n, "recipe") == "决策者配方@1":
					recipe = n
				case n.Type == "Agent" && param(&n, "output_schema") == "列表[任务卡@1]":
					planner = n
				case n.Type == "资源" && param(&n, "kind") == "模型":
					model = n
				}
			}
			if recipe.ID == "" || planner.ID == "" || model.ID == "" {
				t.Fatal("graph_loop.json 的闭环子图里找不到决策者配方、决策者或模型")
			}
			recipe.ID, planner.ID = vid(901), vid(902)
			e.From = gen.PortRef{Node: planner.ID, Port: "result"} // 先改 e，下面 append 可能让 e 失效
			sg.Nodes = append(sg.Nodes, recipe, planner)
			sg.Edges = append(sg.Edges, vedge(901, 901, "context", 902, "context"),
				gen.Edge{ID: eid(902), From: gen.PortRef{Node: model.ID, Port: "ref"}, To: gen.PortRef{Node: planner.ID, Port: "model"}})
		}), gen.DiagV06)
		onlyCode(t, "连到子图引用不存在的端口", loop(func(d *gen.Graph) {
			ref := findNode(t, d, func(n *gen.Node) bool { return n.Type == "子图引用" })
			e := findEdge(t, d, func(e *gen.Edge) bool { return e.To.Node == ref.ID && e.To.Port == "in" })
			e.To.Port = "nope"
		}), gen.DiagV05)
		onlyCode(t, "入口的 port_name 在扇出上没有", loop(func(d *gen.Graph) {
			fanout := findNode(t, d, func(n *gen.Node) bool {
				return n.Type == "扇出" && n.Children != nil && hasNode(n.Children, func(c *gen.Node) bool { return c.Type == "Agent" && param(c, "output_schema") == "方案@1" })
			})
			for i := range fanout.Children.Nodes {
				if c := &fanout.Children.Nodes[i]; param(c, "port_name") == "shared" {
					(*c.Params)["port_name"] = json.RawMessage(`"nope"`)
				}
			}
		}), gen.DiagV11)
	})
}

func param(n *gen.Node, name string) string {
	if n.Params == nil {
		return ""
	}
	var s string
	if json.Unmarshal((*n.Params)[name], &s) != nil {
		return ""
	}
	return s
}

func eachNode(sg *gen.Subgraph, fn func(n *gen.Node)) {
	for i := range sg.Nodes {
		n := &sg.Nodes[i]
		fn(n)
		if n.Children != nil {
			eachNode(n.Children, fn)
		}
	}
}

func eachEdge(sg *gen.Subgraph, fn func(e *gen.Edge)) {
	for i := range sg.Edges {
		fn(&sg.Edges[i])
	}
	for i := range sg.Nodes {
		if c := sg.Nodes[i].Children; c != nil {
			eachEdge(c, fn)
		}
	}
}

func hasNode(sg *gen.Subgraph, pred func(n *gen.Node) bool) bool {
	found := false
	eachNode(sg, func(n *gen.Node) { found = found || pred(n) })
	return found
}

func findNode(t *testing.T, d *gen.Graph, pred func(n *gen.Node) bool) *gen.Node {
	t.Helper()
	var out *gen.Node
	eachNode(&d.Root, func(n *gen.Node) {
		if out == nil && pred(n) {
			out = n
		}
	})
	if out == nil {
		t.Fatal("graph_loop.json 里找不到要改的节点")
	}
	return out
}

func findEdge(t *testing.T, d *gen.Graph, pred func(e *gen.Edge) bool) *gen.Edge {
	t.Helper()
	var out *gen.Edge
	eachEdge(&d.Root, func(e *gen.Edge) {
		if out == nil && pred(e) {
			out = e
		}
	})
	if out == nil {
		t.Fatal("graph_loop.json 里找不到要改的连线")
	}
	return out
}

// scopeOf 是装着这个节点的子图。
func scopeOf(d *gen.Graph, id gen.Ulid) *gen.Subgraph {
	var out *gen.Subgraph
	var walk func(sg *gen.Subgraph)
	walk = func(sg *gen.Subgraph) {
		for i := range sg.Nodes {
			if sg.Nodes[i].ID == id {
				out = sg
			}
			if c := sg.Nodes[i].Children; c != nil {
				walk(c)
			}
		}
	}
	walk(&d.Root)
	return out
}

type loopChange struct {
	name  string
	apply func(d *gen.Graph) Touched
}

// loopChanges 是对 graph_loop.json 的 10 种改动（E0303-T23）：改参数、加删节点、加删连线、改端口名。
func loopChanges(t *testing.T) []loopChange {
	setP := func(n *gen.Node, name, raw string) {
		if n.Params == nil {
			n.Params = &map[string]json.RawMessage{}
		}
		(*n.Params)[name] = json.RawMessage(raw)
	}
	isLoop := func(n *gen.Node) bool { return n.Type == "循环" }
	return []loopChange{
		{"循环最大轮数改成 0", func(d *gen.Graph) Touched {
			n := findNode(t, d, isLoop)
			setP(n, "max_rounds", "0")
			return Touched{Nodes: []string{string(n.ID)}}
		}},
		{"分支条件写错", func(d *gen.Graph) Touched {
			n := findNode(t, d, func(n *gen.Node) bool { return n.Type == "分支" })
			setP(n, "when", `"1 +"`)
			return Touched{Nodes: []string{string(n.ID)}}
		}},
		{"决策者的输出类型改成判定", func(d *gen.Graph) Touched {
			n := findNode(t, d, func(n *gen.Node) bool { return n.Type == "Agent" && param(n, "output_schema") == "列表[任务卡@1]" })
			setP(n, "output_schema", `"判定@1"`)
			return Touched{Nodes: []string{string(n.ID)}}
		}},
		{"循环里加一个孤立的账本", func(d *gen.Graph) Touched {
			sg := findNode(t, d, isLoop).Children
			sg.Nodes = append(sg.Nodes, vnode(900, "账本"))
			return Touched{Nodes: []string{string(vid(900))}}
		}},
		{"加一个没注册的节点", func(d *gen.Graph) Touched {
			d.Root.Nodes = append(d.Root.Nodes, vnode(901, "test-pack/没有这种节点@1"))
			return Touched{Nodes: []string{string(vid(901))}}
		}},
		{"复制一条已有的连线", func(d *gen.Graph) Touched {
			sg := findNode(t, d, isLoop).Children
			e := sg.Edges[0]
			e.ID = eid(900)
			sg.Edges = append(sg.Edges, e)
			return Touched{Edges: []string{string(e.ID)}}
		}},
		{"加一条端口不对的连线", func(d *gen.Graph) Touched {
			sg := findNode(t, d, isLoop).Children
			e := sg.Edges[0]
			e.ID, e.To.Port = eid(901), "nope"
			sg.Edges = append(sg.Edges, e)
			return Touched{Edges: []string{string(e.ID)}}
		}},
		{"删一条连线", func(d *gen.Graph) Touched {
			sg := findNode(t, d, isLoop).Children
			id := sg.Edges[1].ID
			sg.Edges = append(sg.Edges[:1:1], sg.Edges[2:]...)
			return Touched{Edges: []string{string(id)}}
		}},
		{"删按比例分级的聚合", func(d *gen.Graph) Touched {
			n := findNode(t, d, func(n *gen.Node) bool { return n.Type == "聚合" })
			id := n.ID
			sg := scopeOf(d, id)
			var keep []gen.Node
			for _, x := range sg.Nodes {
				if x.ID != id {
					keep = append(keep, x)
				}
			}
			sg.Nodes = keep
			return Touched{Nodes: []string{string(id)}}
		}},
		{"扇出入口的 port_name 改错", func(d *gen.Graph) Touched {
			var id gen.Ulid
			eachNode(&d.Root, func(n *gen.Node) {
				if id == "" && param(n, "port_name") == "shared" {
					setP(n, "port_name", `"nope"`)
					id = n.ID
				}
			})
			return Touched{Nodes: []string{string(id)}}
		}},
	}
}
