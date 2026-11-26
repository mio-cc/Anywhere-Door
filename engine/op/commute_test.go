package op

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/mio-cc/anywhere-door/engine/graph"
	"github.com/mio-cc/anywhere-door/engine/schema"
	"github.com/mio-cc/anywhere-door/gen"
)

// 本包测试共用的辅助函数（E.05.03 先建；后面的叶子只调用、不修改，需要别的写在自己的测试文件里）。

var (
	viewOnce sync.Once
	viewVal  schema.View
	viewErr  error
)

// testView 是仓库 schema/ 的内置注册表（只加载一次）。
func testView(t testing.TB) schema.View {
	t.Helper()
	viewOnce.Do(func() {
		r := schema.NewRegistry(schema.Options{})
		viewErr = r.LoadBuiltin(os.DirFS(filepath.Join("..", "..", "schema")))
		viewVal = r.Snapshot()
	})
	if viewErr != nil {
		t.Fatalf("内置注册表加载失败：\n%v", viewErr)
	}
	return viewVal
}

func tid(n int) gen.Ulid   { return gen.Ulid(fmt.Sprintf("01JCMT%020d", n)) }
func teid(n int) gen.Ulid  { return gen.Ulid(fmt.Sprintf("01JCME%020d", n)) }
func tnid(n int) gen.Ulid  { return gen.Ulid(fmt.Sprintf("01JCMN%020d", n)) }
func topid(n int) gen.Ulid { return gen.Ulid(fmt.Sprintf("01JCMP%020d", n)) }

// tnode 是一个节点；kvs 是参数名、值交替。
func tnode(n int, typ string, kvs ...any) gen.Node {
	nd := gen.Node{ID: tid(n), Type: gen.NodeTypeRef(typ)}
	if len(kvs) > 0 {
		p := map[string]json.RawMessage{}
		for i := 0; i+1 < len(kvs); i += 2 {
			b, _ := json.Marshal(kvs[i+1])
			p[kvs[i].(string)] = b
		}
		nd.Params = &p
	}
	return nd
}

func tedge(n, from int, fport string, to int, tport string) gen.Edge {
	return gen.Edge{ID: teid(n), From: gen.PortRef{Node: tid(from), Port: fport}, To: gen.PortRef{Node: tid(to), Port: tport}}
}

// fixtureDoc 是测试用的图：
//
//	资源 1（提示词模板 {{a}}，动态端口 a）、上下文装配 2、Agent 3、资源 4、资源 5、循环 10（子图：入口 11 → 出口 12，便签 n2）
//	e1 4.ref→3.model  e2 2.context→3.context  e3 5.ref→1.a  e4 1.ref→3.tools（多连）  e5 3.result→10.x  便签 n1
func fixtureDoc() *gen.Graph {
	loop := tnode(10, "循环")
	inner := []gen.Note{{ID: tnid(2), Text: "子图里的便签"}}
	loop.Children = &gen.Subgraph{
		Nodes: []gen.Node{tnode(11, "子图端口", "role", "entry"), tnode(12, "子图端口", "role", "exit")},
		Edges: []gen.Edge{tedge(10, 11, "out", 12, "in")},
		Notes: &inner,
	}
	notes := []gen.Note{{ID: tnid(1), Text: "便签"}}
	return &gen.Graph{
		ID: "01JCMG00000000000000000001", Branch: "main", SchemaVersion: "1.0", Version: 7,
		Root: gen.Subgraph{
			Nodes: []gen.Node{
				tnode(1, "资源", "kind", "提示词", "template", "{{a}}"),
				tnode(2, "上下文装配"),
				tnode(3, "Agent"),
				tnode(4, "资源"),
				tnode(5, "资源"),
				loop,
			},
			Edges: []gen.Edge{
				tedge(1, 4, "ref", 3, "model"),
				tedge(2, 2, "context", 3, "context"),
				tedge(3, 5, "ref", 1, "a"),
				tedge(4, 1, "ref", 3, "tools"),
				tedge(5, 3, "result", 10, "x"),
			},
			Notes: &notes,
		},
	}
}

func fixture(t testing.TB) *graph.Graph {
	t.Helper()
	g, err := graph.FromDoc(fixtureDoc())
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// mkOp 是一条新提交的 Op（没有 inverse）。
func mkOp(t testing.TB, n int, kind gen.OpKind, payload any) gen.Op {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	id := topid(n)
	return gen.Op{ID: &id, Kind: kind, Payload: raw}
}

// mkRec 是一条已落库的 Op 记录（有 inverse，可以为空）。
func mkRec(op gen.Op, inverse ...gen.Op) gen.OpRecord {
	if inverse == nil {
		inverse = []gen.Op{}
	}
	op.Inverse = &inverse
	return gen.OpRecord{Op: op}
}

func raw(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func keys(ks ...Key) []Key {
	out := append([]Key{}, ks...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func kn(n int) Key               { return nodeKey(string(tid(n))) }
func ke(n int) Key               { return edgeKey(string(teid(n))) }
func knt(n int) Key              { return noteKey(string(tnid(n))) }
func kv(n int) Key               { return viewKey(string(tid(n))) }
func kp(n int, port string) Key  { return portKey(string(tid(n)), port) }
func kpar(n int, key string) Key { return paramKey(string(tid(n)), key) }
func kf(n int, field string) Key { return fieldKey(string(tid(n)), field) }

func sameKeys(a, b []Key) bool { return fmt.Sprint(a) == fmt.Sprint(b) }

func TestWriteSets(t *testing.T) {
	v := testView(t)
	g := fixture(t)
	str := func(s string) *string { return &s }
	child := tnode(21, "循环")
	child.Children = &gen.Subgraph{Nodes: []gen.Node{tnode(22, "子图端口", "role", "entry")}, Edges: []gen.Edge{}}
	t.Run("E0503-T1 写集合表全格", func(t *testing.T) {
		cases := []struct {
			name string
			op   gen.Op
			w, r []Key
		}{
			{"add_node 根图", mkOp(t, 1, gen.OpAddNode, gen.PayloadAddNode{Scope: "root", Node: tnode(20, "Agent")}), keys(kn(20)), nil},
			{"add_node 进容器、带子图", mkOp(t, 2, gen.OpAddNode, gen.PayloadAddNode{Scope: string(tid(10)), Node: child}), keys(kn(21), kn(22)), keys(kn(10))},
			{"remove_nodes 连同子图和相连的连线", mkOp(t, 3, gen.OpRemoveNodes, gen.PayloadRemoveNodes{IDs: []gen.Ulid{tid(10)}}), keys(kn(10), kn(11), kn(12), ke(10), ke(5), knt(2)), nil},
			{"restore_nodes", mkOp(t, 4, gen.OpRestoreNodes, gen.PayloadRestoreNodes{Removed: gen.Removed{
				Nodes: []gen.RemovedNodesItem{{Scope: "root", Node: tnode(30, "Agent")}, {Scope: string(tid(10)), Node: tnode(31, "Agent")}},
				Edges: []gen.RemovedEdgesItem{{Scope: "root", Edge: tedge(30, 4, "ref", 30, "model")}},
			}}), keys(kn(30), kn(31), ke(30)), keys(kn(10))},
			{"connect 非多连端口：端口和被替换的连线", mkOp(t, 5, gen.OpConnect, gen.PayloadConnect{Edge: tedge(40, 5, "ref", 3, "context")}), keys(ke(40), kp(3, "context"), ke(2)), keys(kn(5), kn(3))},
			{"connect 多连端口：只有连线", mkOp(t, 6, gen.OpConnect, gen.PayloadConnect{Edge: tedge(41, 4, "ref", 3, "tools")}), keys(ke(41)), keys(kn(4), kn(3))},
			{"disconnect", mkOp(t, 7, gen.OpDisconnect, gen.PayloadDisconnect{IDs: []gen.Ulid{teid(1)}}), keys(ke(1)), nil},
			{"set_param 让动态端口消失", mkOp(t, 8, gen.OpSetParam, gen.PayloadSetParam{Node: tid(1), Key: "template", To: raw("")}), keys(kpar(1, "template"), ke(3)), keys(kn(1))},
			{"set_param 普通参数", mkOp(t, 9, gen.OpSetParam, gen.PayloadSetParam{Node: tid(4), Key: "kind", To: raw("列表")}), keys(kpar(4, "kind")), keys(kn(4))},
			{"set_param 子参数", mkOp(t, 10, gen.OpSetParam, gen.PayloadSetParam{Node: tid(3), Key: "x", Path: &[]string{"p"}, To: raw(1)}), keys(Key(string(kpar(3, "x")) + "/p")), keys(kn(3))},
			{"promote_param", mkOp(t, 11, gen.OpPromoteParam, gen.PayloadPromoteParam{Node: tid(3), Key: "k"}), keys(kpar(3, "k"), kp(3, "k")), keys(kn(3))},
			{"demote_param 连同连到它的线", mkOp(t, 12, gen.OpDemoteParam, gen.PayloadDemoteParam{Node: tid(3), Key: "model"}), keys(kpar(3, "model"), kp(3, "model"), ke(1)), keys(kn(3))},
			{"set_label", mkOp(t, 13, gen.OpSetLabel, gen.PayloadSetLabel{Node: tid(3), To: "甲"}), keys(kf(3, "label")), keys(kn(3))},
			{"set_tags", mkOp(t, 14, gen.OpSetTags, gen.PayloadSetTags{Node: tid(3), To: []string{"a"}}), keys(kf(3, "tags")), keys(kn(3))},
			{"set_lock", mkOp(t, 15, gen.OpSetLock, gen.PayloadSetLock{Node: tid(3), To: "user"}), keys(kf(3, "lock")), keys(kn(3))},
			{"move_nodes", mkOp(t, 16, gen.OpMoveNodes, gen.PayloadMoveNodes{Moves: []gen.PayloadMoveNodesMovesItem{{ID: tid(3)}, {ID: tid(4)}}}), keys(kv(3), kv(4)), keys(kn(3), kn(4))},
			{"set_node_ui", mkOp(t, 17, gen.OpSetNodeUI, gen.PayloadSetNodeUi{Node: tid(3), To: gen.NodeUi{}}), keys(kv(3)), keys(kn(3))},
			{"add_note 进容器", mkOp(t, 18, gen.OpAddNote, gen.PayloadAddNote{Scope: string(tid(10)), Note: gen.Note{ID: tnid(50), Text: "x"}}), keys(knt(50)), keys(kn(10))},
			{"update_note", mkOp(t, 19, gen.OpUpdateNote, gen.PayloadUpdateNote{ID: tnid(1), To: gen.PayloadUpdateNoteTo{Text: str("y")}}), keys(knt(1)), nil},
			{"remove_notes", mkOp(t, 20, gen.OpRemoveNotes, gen.PayloadRemoveNotes{IDs: []gen.Ulid{tnid(1)}}), keys(knt(1)), nil},
			{"set_graph_title", mkOp(t, 21, gen.OpSetGraphTitle, gen.PayloadSetGraphTitle{To: "标题"}), keys("graph:title"), nil},
			{"set_graph_meta", mkOp(t, 22, gen.OpSetGraphMeta, gen.PayloadSetGraphMeta{To: gen.PayloadSetGraphMetaTo{}}), keys("graph:meta"), nil},
			{"migrate", mkOp(t, 23, gen.OpMigrate, gen.PayloadMigrate{FromSchema: "1.0", ToSchema: "1.1"}), keys(keyDoc), nil},
			{"revert_to 是子 Op 的并集", mkOp(t, 24, gen.OpRevertTo, gen.PayloadRevertTo{Version: 3, Ops: &[]gen.Op{
				mkOp(t, 25, gen.OpSetLabel, gen.PayloadSetLabel{Node: tid(3), To: "乙"}),
				mkOp(t, 26, gen.OpMoveNodes, gen.PayloadMoveNodes{Moves: []gen.PayloadMoveNodesMovesItem{{ID: tid(4)}}}),
			}}), keys(kf(3, "label"), kv(4)), keys(kn(3), kn(4))},
			{"已落库的 remove_nodes 按 inverse 算", func() gen.Op {
				r := mkRec(mkOp(t, 27, gen.OpRemoveNodes, gen.PayloadRemoveNodes{IDs: []gen.Ulid{tid(40)}}),
					mkOp(t, 28, gen.OpRestoreNodes, gen.PayloadRestoreNodes{Removed: gen.Removed{
						Nodes: []gen.RemovedNodesItem{{Scope: "root", Node: tnode(40, "资源")}},
						Edges: []gen.RemovedEdgesItem{{Scope: "root", Edge: tedge(40, 40, "ref", 3, "tools")}},
					}}))
				return r.Op
			}(), keys(kn(40), ke(40)), nil},
		}
		for _, c := range cases {
			if got := WriteSet(&c.op, g, v); !sameKeys(got, c.w) {
				t.Errorf("%s：写集合 %v，应当是 %v", c.name, got, c.w)
			}
			if got := RefSet(&c.op, g); !sameKeys(got, c.r) {
				t.Errorf("%s：引用集合 %v，应当是 %v", c.name, got, c.r)
			}
		}

		pairs := []struct {
			name string
			op   gen.Op
			rec  gen.OpRecord
			ok   bool
			want []Key
		}{
			{"两条 set_param 改同一参数（C0504-T2）",
				mkOp(t, 101, gen.OpSetParam, gen.PayloadSetParam{Node: tid(4), Key: "kind", To: raw("提示词")}),
				mkRec(mkOp(t, 102, gen.OpSetParam, gen.PayloadSetParam{Node: tid(4), Key: "kind", To: raw("列表")}),
					mkOp(t, 103, gen.OpSetParam, gen.PayloadSetParam{Node: tid(4), Key: "kind", To: raw("模型")})),
				false, keys(kpar(4, "kind"))},
			{"改参数和改显示名可以交换",
				mkOp(t, 104, gen.OpSetLabel, gen.PayloadSetLabel{Node: tid(4), To: "甲"}),
				mkRec(mkOp(t, 105, gen.OpSetParam, gen.PayloadSetParam{Node: tid(4), Key: "kind", To: raw("列表")}),
					mkOp(t, 106, gen.OpSetParam, gen.PayloadSetParam{Node: tid(4), Key: "kind", To: raw("模型")})),
				true, nil},
			{"删节点和改它的参数",
				mkOp(t, 107, gen.OpRemoveNodes, gen.PayloadRemoveNodes{IDs: []gen.Ulid{tid(4)}}),
				mkRec(mkOp(t, 108, gen.OpSetParam, gen.PayloadSetParam{Node: tid(4), Key: "kind", To: raw("列表")}),
					mkOp(t, 109, gen.OpSetParam, gen.PayloadSetParam{Node: tid(4), Key: "kind", To: raw("模型")})),
				false, keys(kn(4))},
			{"连同一个非多连端口",
				mkOp(t, 110, gen.OpConnect, gen.PayloadConnect{Edge: tedge(40, 5, "ref", 3, "context")}),
				mkRec(mkOp(t, 111, gen.OpConnect, gen.PayloadConnect{Edge: tedge(41, 4, "ref", 3, "context")}),
					mkOp(t, 112, gen.OpDisconnect, gen.PayloadDisconnect{IDs: []gen.Ulid{teid(41)}}),
					mkOp(t, 113, gen.OpConnect, gen.PayloadConnect{Edge: tedge(2, 2, "context", 3, "context")})),
				false, keys(ke(2), kp(3, "context"))},
			{"连同一个多连端口可以交换",
				mkOp(t, 114, gen.OpConnect, gen.PayloadConnect{Edge: tedge(40, 5, "ref", 3, "tools")}),
				mkRec(mkOp(t, 115, gen.OpConnect, gen.PayloadConnect{Edge: tedge(41, 4, "ref", 3, "tools")}),
					mkOp(t, 116, gen.OpDisconnect, gen.PayloadDisconnect{IDs: []gen.Ulid{teid(41)}})),
				true, nil},
			{"往已被删掉的容器里加节点",
				mkOp(t, 117, gen.OpAddNode, gen.PayloadAddNode{Scope: string(tid(10)), Node: tnode(20, "Agent")}),
				mkRec(mkOp(t, 118, gen.OpRemoveNodes, gen.PayloadRemoveNodes{IDs: []gen.Ulid{tid(10)}}),
					mkOp(t, 119, gen.OpRestoreNodes, gen.PayloadRestoreNodes{Removed: gen.Removed{
						Nodes: []gen.RemovedNodesItem{{Scope: "root", Node: tnode(10, "循环")}},
						Edges: []gen.RemovedEdgesItem{{Scope: "root", Edge: tedge(5, 3, "result", 10, "x")}},
					}})),
				false, keys(kn(10))},
			{"migrate 和什么都冲突",
				mkOp(t, 120, gen.OpSetLabel, gen.PayloadSetLabel{Node: tid(3), To: "甲"}),
				mkRec(mkOp(t, 121, gen.OpMigrate, gen.PayloadMigrate{FromSchema: "1.0", ToSchema: "1.1"})),
				false, keys(keyDoc)},
			{"标题和说明互不影响",
				mkOp(t, 122, gen.OpSetGraphTitle, gen.PayloadSetGraphTitle{To: "甲"}),
				mkRec(mkOp(t, 123, gen.OpSetGraphMeta, gen.PayloadSetGraphMeta{To: gen.PayloadSetGraphMetaTo{}}),
					mkOp(t, 124, gen.OpSetGraphMeta, gen.PayloadSetGraphMeta{To: gen.PayloadSetGraphMetaTo{}})),
				true, nil},
			{"改已被删掉的便签",
				mkOp(t, 125, gen.OpUpdateNote, gen.PayloadUpdateNote{ID: tnid(1), To: gen.PayloadUpdateNoteTo{Text: str("y")}}),
				mkRec(mkOp(t, 126, gen.OpRemoveNotes, gen.PayloadRemoveNotes{IDs: []gen.Ulid{tnid(1)}}),
					mkOp(t, 127, gen.OpAddNote, gen.PayloadAddNote{Scope: "root", Note: gen.Note{ID: tnid(1), Text: "便签"}})),
				false, keys(knt(1))},
			{"断开已随动态端口一起删掉的连线",
				mkOp(t, 128, gen.OpDisconnect, gen.PayloadDisconnect{IDs: []gen.Ulid{teid(3)}}),
				mkRec(mkOp(t, 129, gen.OpSetParam, gen.PayloadSetParam{Node: tid(1), Key: "template", To: raw("")}),
					mkOp(t, 130, gen.OpSetParam, gen.PayloadSetParam{Node: tid(1), Key: "template", To: raw("{{a}}")}),
					mkOp(t, 131, gen.OpConnect, gen.PayloadConnect{Edge: tedge(3, 5, "ref", 1, "a")})),
				false, keys(ke(3))},
		}
		for _, c := range pairs {
			ok, cs := Commutes([]gen.Op{c.op}, g, v, []gen.OpRecord{c.rec})
			if ok != c.ok {
				t.Errorf("%s：Commutes = %v，应当是 %v（%v）", c.name, ok, c.ok, cs)
				continue
			}
			var got []Key
			for _, x := range cs {
				if x.OpID != string(*c.op.ID) {
					t.Errorf("%s：冲突的 op_id 是 %s，应当是这一批里的 %s", c.name, x.OpID, *c.op.ID)
				}
				got = append(got, x.Object)
			}
			if !sameKeys(got, c.want) {
				t.Errorf("%s：冲突对象 %v，应当是 %v", c.name, got, c.want)
			}
		}
	})

	t.Run("E0503-T3 例外", func(t *testing.T) {
		move := func(n, node int) gen.Op {
			return mkOp(t, n, gen.OpMoveNodes, gen.PayloadMoveNodes{Moves: []gen.PayloadMoveNodesMovesItem{{ID: tid(node), To: gen.Pos{X: int64(n)}}}})
		}
		removeRec := func(n int, ids ...int) gen.OpRecord {
			var u []gen.Ulid
			var items []gen.RemovedNodesItem
			for _, id := range ids {
				u = append(u, tid(id))
				items = append(items, gen.RemovedNodesItem{Scope: "root", Node: tnode(id, "资源")})
			}
			return mkRec(mkOp(t, n, gen.OpRemoveNodes, gen.PayloadRemoveNodes{IDs: u}),
				mkOp(t, n+1, gen.OpRestoreNodes, gen.PayloadRestoreNodes{Removed: gen.Removed{Nodes: items, Edges: []gen.RemovedEdgesItem{{Scope: "root", Edge: tedge(3, 5, "ref", 1, "a")}}}}))
		}
		if ok, cs := Commutes([]gen.Op{move(201, 3)}, g, v, []gen.OpRecord{mkRec(move(202, 3), move(203, 3))}); !ok {
			t.Errorf("两条 move_nodes 移同一节点应当可以交换：%v", cs)
		}
		ui := mkOp(t, 204, gen.OpSetNodeUI, gen.PayloadSetNodeUi{Node: tid(3), To: gen.NodeUi{}})
		if ok, cs := Commutes([]gen.Op{ui}, g, v, []gen.OpRecord{mkRec(move(205, 3), move(206, 3))}); !ok {
			t.Errorf("set_node_ui 和 move_nodes 同一节点应当可以交换：%v", cs)
		}

		dup := mkOp(t, 207, gen.OpRemoveNodes, gen.PayloadRemoveNodes{IDs: []gen.Ulid{tid(5)}})
		missed := []gen.OpRecord{removeRec(208, 5)}
		if ok, cs := Commutes([]gen.Op{dup}, g, v, missed); !ok {
			t.Errorf("两条 remove_nodes 删同一节点应当可以交换：%v", cs)
		}
		if got := Rebase([]gen.Op{dup}, missed); len(got) != 0 {
			t.Errorf("删空的 remove_nodes 应当整条去掉，还剩 %d 条", len(got))
		}
		two := mkOp(t, 210, gen.OpRemoveNodes, gen.PayloadRemoveNodes{IDs: []gen.Ulid{tid(5), tid(1)}})
		if ok, cs := Commutes([]gen.Op{two}, g, v, missed); !ok {
			t.Errorf("部分重复的 remove_nodes 应当可以交换：%v", cs)
		}
		got := Rebase([]gen.Op{two, ui}, missed)
		if len(got) != 2 {
			t.Fatalf("Rebase 后应当还有 2 条，实际 %d 条", len(got))
		}
		p, err := decode[gen.PayloadRemoveNodes](&got[0])
		if err != nil || len(p.IDs) != 1 || p.IDs[0] != tid(1) {
			t.Errorf("Rebase 后 remove_nodes 应当只剩节点 1：%s（%v）", got[0].Payload, err)
		}
		if string(got[1].Payload) != string(ui.Payload) {
			t.Errorf("Rebase 不应当改别的 Op：%s", got[1].Payload)
		}

		rm := mkOp(t, 211, gen.OpRemoveNodes, gen.PayloadRemoveNodes{IDs: []gen.Ulid{tid(3)}})
		ok, cs := Commutes([]gen.Op{rm}, g, v, []gen.OpRecord{mkRec(move(212, 3), move(213, 3))})
		if ok || len(cs) != 1 || cs[0].Object != kn(3) {
			t.Errorf("remove_nodes 和 move_nodes 同一节点应当冲突在 node 键上：%v %v", ok, cs)
		}
	})

	t.Run("E0503-T4 Target", func(t *testing.T) {
		tg := func(scope string, nodes, edges, notes []gen.Ulid) gen.OpTarget {
			out := gen.OpTarget{Scope: scope}
			if nodes != nil {
				out.Nodes = &nodes
			}
			if edges != nil {
				out.Edges = &edges
			}
			if notes != nil {
				out.Notes = &notes
			}
			return out
		}
		u := func(ids ...gen.Ulid) []gen.Ulid { return ids }
		forged := mkOp(t, 310, gen.OpSetLabel, gen.PayloadSetLabel{Node: tid(3), To: "甲"})
		forged.Target = &gen.OpTarget{Scope: string(tid(10)), Nodes: &[]gen.Ulid{tid(99)}}
		cases := []struct {
			name string
			op   gen.Op
			want gen.OpTarget
		}{
			{"add_node 进容器", mkOp(t, 301, gen.OpAddNode, gen.PayloadAddNode{Scope: string(tid(10)), Node: child}), tg(string(tid(10)), u(tid(21), tid(22)), nil, nil)},
			{"remove_nodes 子图里的节点", mkOp(t, 302, gen.OpRemoveNodes, gen.PayloadRemoveNodes{IDs: []gen.Ulid{tid(11)}}), tg(string(tid(10)), u(tid(11)), nil, nil)},
			{"connect", mkOp(t, 303, gen.OpConnect, gen.PayloadConnect{Edge: tedge(40, 4, "ref", 3, "context")}), tg("root", u(tid(3), tid(4)), u(teid(40)), nil)},
			{"disconnect 子图里的连线", mkOp(t, 304, gen.OpDisconnect, gen.PayloadDisconnect{IDs: []gen.Ulid{teid(10)}}), tg(string(tid(10)), nil, u(teid(10)), nil)},
			{"update_note", mkOp(t, 305, gen.OpUpdateNote, gen.PayloadUpdateNote{ID: tnid(1), To: gen.PayloadUpdateNoteTo{Text: str("y")}}), tg("root", nil, nil, u(tnid(1)))},
			{"remove_notes 子图里的便签", mkOp(t, 306, gen.OpRemoveNotes, gen.PayloadRemoveNotes{IDs: []gen.Ulid{tnid(2)}}), tg(string(tid(10)), nil, nil, u(tnid(2)))},
			{"set_graph_title", mkOp(t, 307, gen.OpSetGraphTitle, gen.PayloadSetGraphTitle{To: "甲"}), tg("root", nil, nil, nil)},
			{"revert_to", mkOp(t, 308, gen.OpRevertTo, gen.PayloadRevertTo{Version: 1, Ops: &[]gen.Op{mkOp(t, 309, gen.OpSetLabel, gen.PayloadSetLabel{Node: tid(11), To: "甲"})}}), tg("root", u(tid(11)), nil, nil)},
			{"客户端填的 target 不用", forged, tg("root", u(tid(3)), nil, nil)},
			{"restore_nodes", mkOp(t, 311, gen.OpRestoreNodes, gen.PayloadRestoreNodes{Removed: gen.Removed{
				Nodes: []gen.RemovedNodesItem{{Scope: string(tid(10)), Node: tnode(30, "Agent")}},
				Edges: []gen.RemovedEdgesItem{{Scope: string(tid(10)), Edge: tedge(30, 11, "out", 30, "context")}},
			}}), tg(string(tid(10)), u(tid(30)), u(teid(30)), nil)},
			{"move_nodes", mkOp(t, 312, gen.OpMoveNodes, gen.PayloadMoveNodes{Moves: []gen.PayloadMoveNodesMovesItem{{ID: tid(12)}, {ID: tid(11)}}}), tg(string(tid(10)), u(tid(11), tid(12)), nil, nil)},
		}
		for _, c := range cases {
			got, err := Target(&c.op, g)
			if err != nil {
				t.Errorf("%s：%v", c.name, err)
				continue
			}
			a, _ := json.Marshal(got)
			b, _ := json.Marshal(c.want)
			if string(a) != string(b) {
				t.Errorf("%s：target %s，应当是 %s", c.name, a, b)
			}
		}
		grp := mkOp(t, 313, gen.OpGroup, gen.PayloadGroup{Scope: "root", NodeIDs: []gen.Ulid{tid(3), tid(4)}, NewID: tid(60), Label: "组"})
		if _, err := Target(&grp, g); err == nil {
			t.Error("M1 还不收的 group 应当返回错误")
		}
	})
}
