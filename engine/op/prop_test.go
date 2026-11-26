package op

import (
	"encoding/json"
	"sort"
	"testing"

	"pgregory.net/rapid"

	"github.com/mio-cc/anywhere-door/engine/graph"
	"github.com/mio-cc/anywhere-door/engine/schema"
	"github.com/mio-cc/anywhere-door/gen"
)

func keysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

var genKinds = []gen.OpKind{
	gen.OpAddNode, gen.OpAddNode, gen.OpRemoveNodes, gen.OpConnect, gen.OpConnect, gen.OpDisconnect,
	gen.OpSetParam, gen.OpSetParam, gen.OpPromoteParam, gen.OpDemoteParam, gen.OpSetLabel, gen.OpSetTags,
	gen.OpSetLock, gen.OpMoveNodes, gen.OpSetNodeUI, gen.OpAddNote, gen.OpUpdateNote, gen.OpRemoveNotes,
	gen.OpSetGraphTitle, gen.OpSetGraphMeta,
}

var genTypes = []string{"资源", "Agent", "上下文装配", "循环", "汇合", promoType}

// genOp 按图现在的样子随机生成一条 M1 Op（可能违反前置条件，应用函数会拒绝）。
func genOp(rt *rapid.T, m *mutable, e *env, v schema.View) gen.Op {
	nodes, edges, notes, scopes := keysOf(m.nodeAt), keysOf(m.edgeAt), keysOf(m.noteAt), keysOf(m.scopes)
	pick := func(l []string, label string) string { return rapid.SampledFrom(l).Draw(rt, label) }
	newNode := func() gen.Op {
		n := gen.Node{ID: gen.Ulid(e.newID()), Type: gen.NodeTypeRef(pick(genTypes, "type")),
			Pos: gen.Pos{X: int64(rapid.IntRange(0, 9).Draw(rt, "x"))}}
		return mk(gen.OpAddNode, gen.PayloadAddNode{Scope: pick(scopes, "scope"), Node: n})
	}
	kind := rapid.SampledFrom(genKinds).Draw(rt, "kind")
	if len(nodes) == 0 && kind != gen.OpAddNote && kind != gen.OpSetGraphTitle && kind != gen.OpSetGraphMeta {
		return newNode()
	}
	switch kind {
	case gen.OpAddNode:
		return newNode()
	case gen.OpRemoveNodes:
		ids := []gen.Ulid{gen.Ulid(pick(nodes, "rm"))}
		if rapid.Bool().Draw(rt, "rm2") {
			ids = append(ids, gen.Ulid(pick(nodes, "rm")))
		}
		return mk(gen.OpRemoveNodes, gen.PayloadRemoveNodes{IDs: ids})
	case gen.OpConnect:
		from := pick(nodes, "from")
		scope := m.nodeAt[from]
		var same []string
		for _, n := range nodes {
			if m.nodeAt[n] == scope {
				same = append(same, n)
			}
		}
		to := pick(same, "to")
		fn, _ := m.node(from)
		tn, _ := m.node(to)
		def, ok := v.NodeDef(string(fn.Type))
		in := keysOf(ports(tn, v))
		if !ok || len(def.Ports.Outputs) == 0 || len(in) == 0 {
			return newNode()
		}
		return mk(gen.OpConnect, gen.PayloadConnect{Edge: gen.Edge{ID: gen.Ulid(e.newID()),
			From: gen.PortRef{Node: gen.Ulid(from), Port: pick(keysOf(def.Ports.Outputs), "fport")},
			To:   gen.PortRef{Node: gen.Ulid(to), Port: pick(in, "tport")}}})
	case gen.OpDisconnect:
		if len(edges) == 0 {
			return newNode()
		}
		return mk(gen.OpDisconnect, gen.PayloadDisconnect{IDs: []gen.Ulid{gen.Ulid(pick(edges, "edge"))}})
	case gen.OpSetParam:
		node := pick(nodes, "node")
		n, _ := m.node(node)
		def, ok := v.NodeDef(string(n.Type))
		if !ok || len(def.Params) == 0 {
			return newNode()
		}
		key := pick(keysOf(def.Params), "key")
		pd := def.Params[key]
		var val json.RawMessage
		switch {
		case pd.Enum != nil && len(*pd.Enum) > 0:
			val = rapid.SampledFrom(*pd.Enum).Draw(rt, "enum")
		case pd.Type == "string" || pd.Type == "text":
			val = mustRaw(pick([]string{"", "a", "{{x}}", "{{x}}{{y}}"}, "text"))
		case pd.Type == "integer" || pd.Type == "number":
			val = mustRaw(rapid.IntRange(0, 3).Draw(rt, "num"))
		case pd.Type == "boolean":
			val = mustRaw(rapid.Bool().Draw(rt, "bool"))
		default:
			val = json.RawMessage("null")
		}
		return mk(gen.OpSetParam, gen.PayloadSetParam{Node: gen.Ulid(node), Key: key, To: val})
	case gen.OpPromoteParam, gen.OpDemoteParam:
		return mk(kind, gen.PayloadPromoteParam{Node: gen.Ulid(pick(nodes, "node")), Key: "k"})
	case gen.OpSetLabel:
		return mk(kind, gen.PayloadSetLabel{Node: gen.Ulid(pick(nodes, "node")), To: pick([]string{"甲", "乙", ""}, "label")})
	case gen.OpSetTags:
		return mk(kind, gen.PayloadSetTags{Node: gen.Ulid(pick(nodes, "node")), To: [][]string{{}, {"a"}, {"a", "b"}}[rapid.IntRange(0, 2).Draw(rt, "tags")]})
	case gen.OpSetLock:
		return mk(kind, gen.PayloadSetLock{Node: gen.Ulid(pick(nodes, "node")), To: pick([]string{"none", "user", "frozen"}, "lock")})
	case gen.OpMoveNodes:
		return mk(kind, gen.PayloadMoveNodes{Moves: []gen.PayloadMoveNodesMovesItem{{ID: gen.Ulid(pick(nodes, "node")),
			To: gen.Pos{X: int64(rapid.IntRange(0, 9).Draw(rt, "x")), Y: int64(rapid.IntRange(0, 9).Draw(rt, "y"))}}}})
	case gen.OpSetNodeUI:
		ui := gen.NodeUi{}
		if rapid.Bool().Draw(rt, "ui") {
			ui["collapsed"] = json.RawMessage("true")
		}
		return mk(kind, gen.PayloadSetNodeUi{Node: gen.Ulid(pick(nodes, "node")), To: ui})
	case gen.OpAddNote:
		return mk(kind, gen.PayloadAddNote{Scope: pick(scopes, "scope"), Note: gen.Note{ID: gen.Ulid(e.newID()), Text: "便签"}})
	case gen.OpUpdateNote, gen.OpRemoveNotes:
		if len(notes) == 0 {
			return newNode()
		}
		id := gen.Ulid(pick(notes, "note"))
		if kind == gen.OpRemoveNotes {
			return mk(kind, gen.PayloadRemoveNotes{IDs: []gen.Ulid{id}})
		}
		to := gen.PayloadUpdateNoteTo{}
		switch rapid.IntRange(0, 2).Draw(rt, "field") {
		case 0:
			to.Text = ptr("改过")
		case 1:
			to.Size = &gen.NoteSize{W: 10, H: 10}
		default:
			to.Color = ptr(pick([]string{"none", "yellow"}, "color"))
		}
		return mk(kind, gen.PayloadUpdateNote{ID: id, To: to})
	case gen.OpSetGraphTitle:
		return mk(kind, gen.PayloadSetGraphTitle{To: pick([]string{"甲", "乙"}, "title")})
	default:
		to := gen.PayloadSetGraphMetaTo{}
		if rapid.Bool().Draw(rt, "desc") {
			to.Description = ptr("说明")
		}
		return mk(gen.OpSetGraphMeta, gen.PayloadSetGraphMeta{To: to})
	}
}

func TestOpProperties(t *testing.T) {
	v := promoView{testView(t)}
	base := fixtureDoc()
	normalizeDoc(base, v)

	t.Run("E0501-T2 逆操作属性测试", func(t *testing.T) {
		for round := 0; round < 10; round++ { // rapid 每轮默认 100 组，共 1000 组（C0503-T1）
			rapid.Check(t, func(rt *rapid.T) {
				m := newMutable(cloneDoc(base))
				e, ie := testEnv(v, false), testEnv(v, true)
				start := graph.Hash(m.doc)
				var invs [][]gen.Op
				for i, n := 0, rapid.IntRange(1, 50).Draw(rt, "n"); i < n; i++ {
					o := genOp(rt, m, e, v)
					before := docJSON(m)
					inv, err := apply1(m, e, o)
					if err != nil {
						if docJSON(m) != before {
							rt.Fatalf("%s 失败了却改了图：%v", o.Kind, err)
						}
						continue
					}
					invs = append(invs, inv)
				}
				for i := len(invs) - 1; i >= 0; i-- {
					for j := range invs[i] {
						if _, err := apply1(m, ie, invs[i][j]); err != nil {
							rt.Fatalf("逆操作 %s 失败：%v（%s）", invs[i][j].Kind, err, invs[i][j].Payload)
						}
					}
				}
				if got := graph.Hash(m.doc); got != start {
					rt.Fatalf("逆序应用逆操作后图变了：\n%s", docJSON(m))
				}
			})
		}
	})

	t.Run("E0501-T8 增量与全量一致", func(t *testing.T) {
		opts := schema.Opts{ForRun: true}
		rapid.Check(t, func(rt *rapid.T) { // E0303-T19
			m := newMutable(cloneDoc(base))
			e := testEnv(v, false)
			g, err := graph.FromDoc(m.doc)
			if err != nil {
				rt.Fatal(err)
			}
			prev := nonNil(schema.Validate(g, v, opts))
			for i, n := 0, rapid.IntRange(1, 20).Draw(rt, "n"); i < n; i++ {
				o := genOp(rt, m, e, v)
				a, ok := appliers[o.Kind]
				if !ok {
					rt.Fatalf("没有 %s 的应用函数", o.Kind)
				}
				_, touched, err := a(m, &o, e)
				if err != nil {
					continue
				}
				ng, err := graph.FromDoc(m.doc)
				if err != nil {
					rt.Fatal(err)
				}
				inc := prev
				if !viewOnly[o.Kind] {
					inc = nonNil(schema.ValidateIncremental(ng, prev, touched, v, opts))
				}
				full := nonNil(schema.Validate(ng, v, opts))
				if x, y := string(mustRaw(inc)), string(mustRaw(full)); x != y {
					rt.Fatalf("%s（%s）之后增量和全量不同：\n增量 %s\n全量 %s", o.Kind, o.Payload, x, y)
				}
				prev = inc
			}
		})
	})
}
