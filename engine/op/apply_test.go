package op

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mio-cc/anywhere-door/engine/schema"
	"github.com/mio-cc/anywhere-door/gen"
)

// promoView 在内置注册表上加一个有可提升参数的测试节点类型（内置节点没有可提升的参数）。
type promoView struct{ schema.View }

const promoType = "test/提升@1"

var promoDef = &gen.NodeDef{
	Name: "test/提升", Version: "1.0", Kind: "composite", Category: "扩展",
	Impl:   gen.Impl{Type: "builtin", Raw: json.RawMessage(`{"type":"builtin"}`)},
	Params: map[string]gen.ParamDef{"k": {Type: "string", Label: "k", Promotable: ptr(true)}},
	Ports:  gen.NodeDefPorts{Inputs: map[string]gen.PortDef{}, Outputs: map[string]gen.PortDef{"out": {Type: "任意"}}},
}

func (v promoView) NodeDef(ref string) (*gen.NodeDef, bool) {
	if ref == promoType {
		return promoDef, true
	}
	return v.View.NodeDef(ref)
}

var envIDs atomic.Int64

func testEnv(v schema.View, internal bool) *env {
	return &env{view: v, internal: internal, newID: func() string { return fmt.Sprintf("01JCMA%020d", envIDs.Add(1)) }}
}

func fixtureM(v schema.View) *mutable {
	d := fixtureDoc()
	normalizeDoc(d, v)
	return newMutable(d)
}

func apply1(m *mutable, e *env, o gen.Op) ([]gen.Op, error) {
	a, ok := appliers[o.Kind]
	if !ok {
		return nil, fmt.Errorf("没有 %s 的应用函数", o.Kind)
	}
	inv, _, err := a(m, &o, e)
	return inv, err
}

func docJSON(m *mutable) string { return string(mustRaw(m.doc)) }

func TestApply(t *testing.T) {
	v := promoView{testView(t)}
	str := func(s string) *string { return &s }
	loop := tnode(30, "循环")
	withKids := tnode(31, "资源")
	withKids.Children = &gen.Subgraph{Nodes: []gen.Node{}, Edges: []gen.Edge{}}
	long := strings.Repeat("字", 81)

	t.Run("E0501-T1 每种 Op 正反例（应用函数）", func(t *testing.T) {
		pos := []struct {
			name string
			ops  []gen.Op
		}{
			{"add_node", []gen.Op{mkOp(t, 1, gen.OpAddNode, gen.PayloadAddNode{Scope: "root", Node: tnode(20, "资源", "kind", "列表")})}},
			{"add_node 容器自动加端口", []gen.Op{mkOp(t, 2, gen.OpAddNode, gen.PayloadAddNode{Scope: "root", Node: loop})}},
			{"add_node 进容器", []gen.Op{mkOp(t, 3, gen.OpAddNode, gen.PayloadAddNode{Scope: string(tid(10)), Node: tnode(21, "Agent")})}},
			{"remove_nodes 连同连线", []gen.Op{mkOp(t, 4, gen.OpRemoveNodes, gen.PayloadRemoveNodes{IDs: []gen.Ulid{tid(3)}})}},
			{"remove_nodes 先子节点后容器", []gen.Op{mkOp(t, 5, gen.OpRemoveNodes, gen.PayloadRemoveNodes{IDs: []gen.Ulid{tid(11), tid(10)}})}},
			{"connect 替换非多连端口的旧线", []gen.Op{mkOp(t, 6, gen.OpConnect, gen.PayloadConnect{Edge: tedge(40, 5, "ref", 3, "model")})}},
			{"connect 多连端口", []gen.Op{mkOp(t, 7, gen.OpConnect, gen.PayloadConnect{Edge: tedge(41, 4, "ref", 3, "tools")})}},
			{"disconnect", []gen.Op{mkOp(t, 8, gen.OpDisconnect, gen.PayloadDisconnect{IDs: []gen.Ulid{teid(1), teid(4)}})}},
			{"set_param 让动态端口消失", []gen.Op{mkOp(t, 9, gen.OpSetParam, gen.PayloadSetParam{Node: tid(1), Key: "template", To: raw("")})}},
			{"set_param 普通参数", []gen.Op{mkOp(t, 10, gen.OpSetParam, gen.PayloadSetParam{Node: tid(4), Key: "kind", To: raw("列表")})}},
			{"set_param 设成默认值", []gen.Op{mkOp(t, 11, gen.OpSetParam, gen.PayloadSetParam{Node: tid(1), Key: "kind", To: raw("模型")})}},
			{"promote / demote", []gen.Op{
				mkOp(t, 12, gen.OpAddNode, gen.PayloadAddNode{Scope: "root", Node: tnode(22, promoType)}),
				mkOp(t, 13, gen.OpPromoteParam, gen.PayloadPromoteParam{Node: tid(22), Key: "k"}),
				mkOp(t, 14, gen.OpConnect, gen.PayloadConnect{Edge: tedge(42, 4, "ref", 22, "k")}),
				mkOp(t, 15, gen.OpDemoteParam, gen.PayloadDemoteParam{Node: tid(22), Key: "k"}),
			}},
			{"set_label / set_tags / set_lock", []gen.Op{
				mkOp(t, 16, gen.OpSetLabel, gen.PayloadSetLabel{Node: tid(3), To: "甲"}),
				mkOp(t, 17, gen.OpSetTags, gen.PayloadSetTags{Node: tid(3), To: []string{"a", "b"}}),
				mkOp(t, 18, gen.OpSetLock, gen.PayloadSetLock{Node: tid(3), To: "frozen"}),
			}},
			{"move_nodes / set_node_ui", []gen.Op{
				mkOp(t, 19, gen.OpMoveNodes, gen.PayloadMoveNodes{Moves: []gen.PayloadMoveNodesMovesItem{{ID: tid(3), To: gen.Pos{X: 5}}, {ID: tid(3), To: gen.Pos{X: 9}}}}),
				mkOp(t, 20, gen.OpSetNodeUI, gen.PayloadSetNodeUi{Node: tid(3), To: gen.NodeUi{"collapsed": json.RawMessage("true")}}),
			}},
			{"便签", []gen.Op{
				mkOp(t, 21, gen.OpAddNote, gen.PayloadAddNote{Scope: string(tid(10)), Note: gen.Note{ID: tnid(5), Text: "新"}}),
				mkOp(t, 22, gen.OpUpdateNote, gen.PayloadUpdateNote{ID: tnid(1), To: gen.PayloadUpdateNoteTo{Size: &gen.NoteSize{W: 100, H: 50}, Color: str("yellow")}}),
				mkOp(t, 23, gen.OpRemoveNotes, gen.PayloadRemoveNotes{IDs: []gen.Ulid{tnid(2)}}),
			}},
			{"标题与说明", []gen.Op{
				mkOp(t, 24, gen.OpSetGraphTitle, gen.PayloadSetGraphTitle{To: "标题"}),
				mkOp(t, 25, gen.OpSetGraphMeta, gen.PayloadSetGraphMeta{To: gen.PayloadSetGraphMetaTo{Description: str("说明")}}),
			}},
		}
		for _, c := range pos {
			m := fixtureM(v)
			before := docJSON(m)
			var invs [][]gen.Op
			for _, o := range c.ops {
				inv, err := apply1(m, testEnv(v, false), o)
				if err != nil {
					t.Fatalf("%s：%s 应当成功：%v", c.name, o.Kind, err)
				}
				invs = append(invs, inv)
			}
			for i := len(invs) - 1; i >= 0; i-- {
				for _, io := range invs[i] {
					if _, err := apply1(m, testEnv(v, true), io); err != nil {
						t.Fatalf("%s：逆操作 %s 失败：%v（%s）", c.name, io.Kind, err, io.Payload)
					}
				}
			}
			if got := docJSON(m); got != before {
				t.Errorf("%s：逆序应用逆操作后应当回到原图：\n%s\n%s", c.name, got, before)
			}
		}

		// 几个应用效果
		m := fixtureM(v)
		o := mkOp(t, 30, gen.OpAddNode, gen.PayloadAddNode{Scope: "root", Node: loop})
		if _, _, err := appliers[o.Kind](m, &o, testEnv(v, false)); err != nil { // 直接调用：apply1 传值，补全的 payload 会丢
			t.Fatal(err)
		}
		var p gen.PayloadAddNode
		if err := json.Unmarshal(o.Payload, &p); err != nil || p.Node.Children == nil || len(p.Node.Children.Nodes) != 2 {
			t.Errorf("容器的 payload 应当补上一对子图端口：%s", o.Payload)
		}
		if _, err := apply1(m, testEnv(v, false), mkOp(t, 31, gen.OpSetParam, gen.PayloadSetParam{Node: tid(1), Key: "template", To: raw("")})); err != nil {
			t.Fatal(err)
		}
		if e, _ := m.edge(string(teid(3))); e != nil {
			t.Error("动态端口消失时连到它的线应当一并删除")
		}
		if n, _ := m.node(string(tid(4))); n.Params != nil {
			t.Errorf("等于默认值的参数不应当写进文档：%s", mustRaw(n.Params))
		}

		neg := []struct {
			name string
			op   gen.Op
			want error
		}{
			{"add_node 层不存在", mkOp(t, 101, gen.OpAddNode, gen.PayloadAddNode{Scope: string(tid(99)), Node: tnode(20, "资源")}), ErrInvalid},
			{"add_node ID 已占用", mkOp(t, 102, gen.OpAddNode, gen.PayloadAddNode{Scope: "root", Node: tnode(3, "资源")}), ErrInvalid},
			{"add_node ID 不是 ULID", mkOp(t, 103, gen.OpAddNode, gen.PayloadAddNode{Scope: "root", Node: gen.Node{ID: "x", Type: "资源"}}), ErrInvalid},
			{"add_node 类型没注册", mkOp(t, 104, gen.OpAddNode, gen.PayloadAddNode{Scope: "root", Node: tnode(20, "没有这个@1")}), ErrInvalid},
			{"add_node 不是容器却带子图", mkOp(t, 105, gen.OpAddNode, gen.PayloadAddNode{Scope: "root", Node: withKids}), ErrInvalid},
			{"remove_nodes 不存在", mkOp(t, 106, gen.OpRemoveNodes, gen.PayloadRemoveNodes{IDs: []gen.Ulid{tid(3), tid(99)}}), ErrInvalid},
			{"remove_nodes 空", mkOp(t, 107, gen.OpRemoveNodes, gen.PayloadRemoveNodes{IDs: []gen.Ulid{}}), ErrInvalid},
			{"restore_nodes ID 已占用", mkOp(t, 108, gen.OpRestoreNodes, gen.PayloadRestoreNodes{Removed: gen.Removed{
				Nodes: []gen.RemovedNodesItem{{Scope: "root", Node: tnode(3, "Agent")}}, Edges: []gen.RemovedEdgesItem{}}}), ErrInvalid},
			{"connect 连线 ID 已占用", mkOp(t, 109, gen.OpConnect, gen.PayloadConnect{Edge: tedge(1, 5, "ref", 3, "tools")}), ErrInvalid},
			{"connect 节点不存在", mkOp(t, 110, gen.OpConnect, gen.PayloadConnect{Edge: tedge(40, 99, "ref", 3, "tools")}), ErrInvalid},
			{"connect 两端不在同一层", mkOp(t, 111, gen.OpConnect, gen.PayloadConnect{Edge: tedge(40, 11, "out", 3, "tools")}), ErrInvalid},
			{"disconnect 不存在", mkOp(t, 112, gen.OpDisconnect, gen.PayloadDisconnect{IDs: []gen.Ulid{teid(1), teid(99)}}), ErrInvalid},
			{"set_param 节点不存在", mkOp(t, 113, gen.OpSetParam, gen.PayloadSetParam{Node: tid(99), Key: "kind", To: raw("列表")}), ErrInvalid},
			{"set_param 没有这个参数", mkOp(t, 114, gen.OpSetParam, gen.PayloadSetParam{Node: tid(4), Key: "nope", To: raw(1)}), ErrInvalid},
			{"set_param 内核锁定的参数", mkOp(t, 115, gen.OpSetParam, gen.PayloadSetParam{Node: tid(3), Key: "context_src", To: raw("x")}), ErrInvalid},
			{"set_param path 用在不是对象的参数上", mkOp(t, 116, gen.OpSetParam, gen.PayloadSetParam{Node: tid(4), Key: "kind", Path: &[]string{"a"}, To: raw(1)}), ErrInvalid},
			{"promote_param 不能提升", mkOp(t, 117, gen.OpPromoteParam, gen.PayloadPromoteParam{Node: tid(4), Key: "kind"}), ErrInvalid},
			{"demote_param 没有提升", mkOp(t, 118, gen.OpDemoteParam, gen.PayloadDemoteParam{Node: tid(4), Key: "kind"}), ErrInvalid},
			{"set_label 太长", mkOp(t, 119, gen.OpSetLabel, gen.PayloadSetLabel{Node: tid(3), To: long}), ErrInvalid},
			{"set_label 空", mkOp(t, 120, gen.OpSetLabel, gen.PayloadSetLabel{Node: tid(3), To: ""}), ErrInvalid},
			{"set_tags 太多", mkOp(t, 121, gen.OpSetTags, gen.PayloadSetTags{Node: tid(3), To: strings.Split("a,b,c,d,e,f,g,h,i,j,k,l,m,n,o,p,q", ",")}), ErrInvalid},
			{"set_lock 取值不对", mkOp(t, 122, gen.OpSetLock, gen.PayloadSetLock{Node: tid(3), To: "all"}), ErrInvalid},
			{"move_nodes 不存在", mkOp(t, 123, gen.OpMoveNodes, gen.PayloadMoveNodes{Moves: []gen.PayloadMoveNodesMovesItem{{ID: tid(3)}, {ID: tid(99)}}}), ErrInvalid},
			{"move_nodes 空", mkOp(t, 124, gen.OpMoveNodes, gen.PayloadMoveNodes{Moves: []gen.PayloadMoveNodesMovesItem{}}), ErrInvalid},
			{"set_node_ui 节点不存在", mkOp(t, 125, gen.OpSetNodeUI, gen.PayloadSetNodeUi{Node: tid(99), To: gen.NodeUi{}}), ErrInvalid},
			{"add_note ID 已占用", mkOp(t, 126, gen.OpAddNote, gen.PayloadAddNote{Scope: "root", Note: gen.Note{ID: tnid(1)}}), ErrInvalid},
			{"add_note 层不存在", mkOp(t, 127, gen.OpAddNote, gen.PayloadAddNote{Scope: string(tid(3)), Note: gen.Note{ID: tnid(9)}}), ErrInvalid},
			{"update_note 不存在", mkOp(t, 128, gen.OpUpdateNote, gen.PayloadUpdateNote{ID: tnid(9), To: gen.PayloadUpdateNoteTo{Text: str("x")}}), ErrInvalid},
			{"update_note 什么都没改", mkOp(t, 129, gen.OpUpdateNote, gen.PayloadUpdateNote{ID: tnid(1)}), ErrInvalid},
			{"remove_notes 不存在", mkOp(t, 130, gen.OpRemoveNotes, gen.PayloadRemoveNotes{IDs: []gen.Ulid{tnid(1), tnid(9)}}), ErrInvalid},
			{"set_graph_title 太长", mkOp(t, 131, gen.OpSetGraphTitle, gen.PayloadSetGraphTitle{To: strings.Repeat("题", 121)}), ErrInvalid},
			{"set_graph_title 空", mkOp(t, 132, gen.OpSetGraphTitle, gen.PayloadSetGraphTitle{To: ""}), ErrInvalid},
			{"payload 多出字段", gen.Op{Kind: gen.OpSetLabel, Payload: json.RawMessage(`{"node":"` + string(tid(3)) + `","to":"a","x":1}`)}, ErrBadRequest},
		}
		for _, c := range neg {
			m := fixtureM(v)
			before := docJSON(m)
			_, err := apply1(m, testEnv(v, false), c.op)
			if !errors.Is(err, c.want) {
				t.Errorf("%s：应当是 %v，实际 %v", c.name, c.want, err)
			}
			if docJSON(m) != before {
				t.Errorf("%s：失败了不应当改图", c.name)
			}
		}
	})
}
