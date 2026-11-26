package op_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"pgregory.net/rapid"

	"github.com/mio-cc/anywhere-door/engine/authz"
	"github.com/mio-cc/anywhere-door/engine/op"
	"github.com/mio-cc/anywhere-door/engine/schema/migrate"
	"github.com/mio-cc/anywhere-door/gen"
)

var backends = []string{"mem", "sqlite"}

func TestPipelineOps(t *testing.T) {
	t.Run("E0501-T1 每种 Op 正反例（流水线）", func(t *testing.T) {
		h := setup(t, "mem")
		h.must(user, addNode(1, "资源", "root"), addNode(2, "资源", "root"), addNode(3, "Agent", "root"))
		note := gen.Note{ID: tid(1), Text: "便签"}
		desc := "说明"
		h.must(user,
			setLabel(3, "执行"),
			mkop(gen.OpSetTags, gen.PayloadSetTags{Node: nid(3), To: []string{"a"}}),
			setParam(1, "kind", "列表"),
			moveNode(2, 40, 60),
			mkop(gen.OpSetNodeUI, gen.PayloadSetNodeUi{Node: nid(3), To: gen.NodeUi{"collapsed": json.RawMessage("true")}}),
			mkop(gen.OpAddNote, gen.PayloadAddNote{Scope: "root", Note: note}),
			mkop(gen.OpSetGraphTitle, gen.PayloadSetGraphTitle{To: "标题"}),
			mkop(gen.OpSetGraphMeta, gen.PayloadSetGraphMeta{To: gen.PayloadSetGraphMetaTo{Description: &desc}}),
		)
		text := "改过"
		h.must(user, mkop(gen.OpUpdateNote, gen.PayloadUpdateNote{ID: tid(1), To: gen.PayloadUpdateNoteTo{Text: &text}}))
		h.must(user, mkop(gen.OpSetLock, gen.PayloadSetLock{Node: nid(3), To: "user"}))
		d := h.doc()
		n3 := d.Root.Nodes[2]
		if *d.Title != "标题" || *n3.Label != "执行" || *n3.Lock != "user" || (*d.Root.Notes)[0].Text != "改过" || d.Root.Nodes[1].Pos.Y != 60 {
			t.Errorf("应用后的图不对：%s", asJSON(d))
		}
		h.must(user, mkop(gen.OpRemoveNotes, gen.PayloadRemoveNotes{IDs: []gen.Ulid{tid(1)}}))

		// 模型提交 set_lock 返回 403（C0502-T2）
		_, err := h.submit(model, withReason(mkop(gen.OpSetLock, gen.PayloadSetLock{Node: nid(1), To: "user"}), "锁住"))
		if !errors.Is(err, op.ErrForbidden) {
			t.Errorf("模型的 set_lock 应当 403：%v", err)
		}
		// 前置条件
		if _, err := h.submit(user, setLabel(99, "x")); !errors.Is(err, op.ErrInvalid) {
			t.Errorf("改不存在的节点应当 ErrInvalid：%v", err)
		}
		if _, err := h.submit(user, connect(1, 1, "ref", 99, "model")); !errors.Is(err, op.ErrInvalid) {
			t.Errorf("连到不存在的节点应当 ErrInvalid：%v", err)
		}
		// 类型不兼容：资源引用接到 Agent 的 context（V14 / V06，新出现的阻止提交）
		var ig *op.InvalidGraphError
		if _, err := h.submit(user, connect(9, 1, "ref", 3, "context")); !errors.As(err, &ig) || len(ig.Diags) == 0 {
			t.Errorf("类型不兼容应当 422：%v", err)
		}

		// connect 到非多连端口：替换旧连线，提交逆操作后恢复（C0502-T3）
		h.must(user, connect(1, 1, "ref", 3, "model"))
		after1 := asJSON(h.doc().Root)
		res := h.must(user, connect(2, 2, "ref", 3, "model"))
		d = h.doc()
		if len(d.Root.Edges) != 1 || d.Root.Edges[0].ID != eid(2) {
			t.Fatalf("旧连线应当被替换：%s", asJSON(d.Root.Edges))
		}
		recs := h.records()
		last := recs[len(recs)-1]
		if string(*last.Op.ID) != res.Applied[0] || last.Op.Inverse == nil || len(*last.Op.Inverse) != 2 {
			t.Fatalf("逆操作应当是断开新线、接回旧线：%s", asJSON(last.Op.Inverse))
		}
		h.must(user, *last.Op.Inverse...)
		if got := asJSON(h.doc().Root); got != after1 {
			t.Errorf("提交逆操作后应当回到替换前：\n%s\n%s", got, after1)
		}
		// 删节点连带删相连的连线
		h.must(user, removeNodes(3))
		if d := h.doc(); len(d.Root.Edges) != 0 || len(d.Root.Nodes) != 2 {
			t.Errorf("删节点应当连带删连线：%s", asJSON(d.Root))
		}
	})

	t.Run("E0501-T3 并发 100", func(t *testing.T) {
		for _, be := range backends {
			h := setup(t, be)
			var wg sync.WaitGroup
			errs := make(chan error, 100)
			for i := 0; i < 100; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					if _, err := h.p.Submit(ctx, h.batch(user, 0, addNode(100+i, "资源", "root"))); err != nil {
						errs <- err
					}
				}(i)
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				t.Errorf("%s：并发提交失败：%v", be, err)
			}
			recs := h.records()
			if len(recs) != 100 {
				t.Fatalf("%s：应当有 100 条 Op，实际 %d 条", be, len(recs))
			}
			for i, r := range recs {
				if r.Version != int64(i+1) {
					t.Fatalf("%s：版本应当连续，第 %d 条是 %d", be, i+1, r.Version)
				}
			}
			if n := len(h.doc().Root.Nodes); n != 100 {
				t.Errorf("%s：应当有 100 个节点，实际 %d 个", be, n)
			}
		}
	})

	t.Run("E0501-T4 批次原子性", func(t *testing.T) {
		h := setup(t, "mem")
		h.must(user, addNode(1, "资源", "root"))
		before, evs := len(h.records()), len(h.bus.all())
		_, err := h.submit(user, setLabel(1, "甲"), addNode(2, "资源", "root"), setLabel(99, "乙"))
		if !errors.Is(err, op.ErrInvalid) || !strings.Contains(err.Error(), "第 3 条") {
			t.Fatalf("第 3 条失败应当整批失败并写明第几条：%v", err)
		}
		if len(h.records()) != before || len(h.bus.all()) != evs {
			t.Errorf("失败的批次不应当落库、发事件")
		}
		if d := h.doc(); len(d.Root.Nodes) != 1 || d.Root.Nodes[0].Label != nil {
			t.Errorf("失败的批次不应当改图：%s", asJSON(d.Root))
		}
	})

	t.Run("E0501-T5 actor 与 reason", func(t *testing.T) {
		h := setup(t, "mem")
		forged := withReason(addNode(1, "资源", "root"), "加一个资源")
		forged.Actor = &user
		h.must(model, forged)
		if r := h.records()[0]; r.Op.Actor == nil || *r.Op.Actor != model {
			t.Errorf("落库的 actor 应当是模型：%v", r.Op.Actor)
		}
		if _, err := h.submit(model, addNode(2, "资源", "root")); !errors.Is(err, op.ErrBadRequest) {
			t.Errorf("模型不带 reason 应当 400：%v", err)
		}
	})

	t.Run("E0501-T6 不收的 kind", func(t *testing.T) {
		h := setup(t, "mem")
		want := map[gen.OpKind]string{
			gen.OpGroup: "M2", gen.OpUngroup: "M2", gen.OpSetSubgraphRef: "M2", gen.OpImportApply: "M2",
			gen.OpSetMandate: "M3", gen.OpApplyPatch: "M4", gen.OpCreateBranch: "M4", gen.OpDeleteBranch: "M4", gen.OpMergeBranch: "M4",
		}
		for k, ms := range want {
			_, err := h.submit(user, gen.Op{Kind: k, Payload: json.RawMessage(`{}`)})
			if !errors.Is(err, op.ErrBadRequest) || !strings.Contains(err.Error(), ms) {
				t.Errorf("%s 应当 400 并写明 %s：%v", k, ms, err)
			}
		}
		rm := mkop(gen.OpRestoreNodes, gen.PayloadRestoreNodes{Removed: gen.Removed{Nodes: []gen.RemovedNodesItem{}, Edges: []gen.RemovedEdgesItem{}}})
		if _, err := h.submit(user, rm); !errors.Is(err, op.ErrBadRequest) {
			t.Errorf("客户端提交 restore_nodes 应当 400：%v", err)
		}
		if _, err := h.submit(user, gen.Op{Kind: "no_such_kind", Payload: json.RawMessage(`{}`)}); !errors.Is(err, op.ErrBadRequest) {
			t.Errorf("不认识的 kind 应当 400：%v", err)
		}
		if _, err := h.submit(user, gen.Op{Kind: gen.OpSetLabel, Payload: json.RawMessage(`{"node":"x","to":"a","extra":1}`)}); !errors.Is(err, op.ErrBadRequest) {
			t.Errorf("payload 多出字段应当 400：%v", err)
		}
	})

	t.Run("E0501-T7 migrate Op", func(t *testing.T) {
		chain := []migrate.Step{{From: "1.0", To: "1.1", Fn: func(d map[string]any) (map[string]any, error) {
			d["meta"] = map[string]any{"migrated": true}
			return d, nil
		}}}
		h := setup(t, "mem", func(o *op.Options) { o.Migrations = chain })
		res := h.must(user, mkop(gen.OpSetGraphTitle, gen.PayloadSetGraphTitle{To: "甲"}))
		recs := h.records()
		if len(recs) != 2 || recs[0].Op.Kind != gen.OpMigrate || recs[0].Version != 1 || *recs[0].Op.Actor != (gen.Actor{Kind: gen.ActorKindRuntime, ID: "migrate"}) {
			t.Fatalf("应当先写一条 runtime/migrate 的 migrate Op：%s", asJSON(recs))
		}
		if recs[0].Op.Inverse != nil {
			t.Errorf("migrate 不可逆，不应当有 inverse")
		}
		if res.Version != 2 || recs[1].Op.Kind != gen.OpSetGraphTitle {
			t.Errorf("客户端的批次应当接在升级后面：%+v", res)
		}
		if d := h.doc(); d.SchemaVersion != "1.1" || d.Meta == nil {
			t.Errorf("head 应当是升级后的格式：%s", asJSON(d))
		}
		g, _, err := h.p.Read(ctx, h.gid, "main", 0)
		if err != nil || g.SchemaVersion() != "1.0" {
			t.Errorf("Read 旧版本应当还是 1.0：%v %v", g, err)
		}
		h.must(user, mkop(gen.OpSetGraphTitle, gen.PayloadSetGraphTitle{To: "乙"}))
		if n := len(h.records()); n != 3 {
			t.Errorf("升级只做一次，应当共 3 条，实际 %d 条", n)
		}
		g2, _, err := h.reopen().Read(ctx, h.gid, "main", op.Head)
		if err != nil || asJSON(g2.Doc()) != asJSON(h.doc()) {
			t.Errorf("重放带 migrate 的日志应当得到同样的图：%v", err)
		}
	})

	t.Run("E0501-T9 Base 加重放", func(t *testing.T) {
		for _, be := range backends {
			h := setup(t, be)
			saved := map[int64]string{}
			for i := 1; i <= 120; i++ {
				var o gen.Op
				switch i % 3 {
				case 1:
					o = addNode(i, "资源", "root")
				case 2:
					o = setLabel(i-1, fmt.Sprintf("节点 %d", i))
				default:
					o = moveNode(i-2, int64(i), int64(i))
				}
				res := h.must(user, o)
				saved[res.Version] = asJSON(h.doc())
			}
			p2 := h.reopen()
			for _, v := range []int64{1, 49, 50, 51, 99, 100, 101, 119, 120} {
				g, _, err := p2.Read(ctx, h.gid, "main", v)
				if err != nil {
					t.Fatalf("%s：Read(%d)：%v", be, v, err)
				}
				if got := asJSON(g.Doc()); got != saved[v] {
					t.Errorf("%s：重放得到的第 %d 版和提交时不同", be, v)
				}
			}
			for v, want := range map[int64]int64{49: 0, 50: 50, 99: 50, 100: 100, 120: 100} {
				s, err := h.s.Graphs().Base(ctx, h.gid, "main", v)
				if err != nil || s.Version != want {
					t.Errorf("%s：Base(%d) 的全文应当在第 %d 版，实际 %d（%v）", be, v, want, s.Version, err)
				}
			}
		}
	})

	t.Run("E0501-T10 服务器补全写进日志", func(t *testing.T) {
		h := setup(t, "mem")
		h.must(user, addNode(1, "循环", "root"))
		var p gen.PayloadAddNode
		if err := json.Unmarshal(h.records()[0].Op.Payload, &p); err != nil {
			t.Fatal(err)
		}
		if p.Node.Children == nil || len(p.Node.Children.Nodes) != 2 {
			t.Fatalf("日志里的 add_node 应当带一对子图端口：%s", h.records()[0].Op.Payload)
		}
		for _, n := range p.Node.Children.Nodes {
			if n.Type != gen.NodeTypeRef(gen.NodeTypeSubPort) {
				t.Errorf("自动加的应当是子图端口：%s", n.Type)
			}
		}
		g, _, err := h.reopen().Read(ctx, h.gid, "main", op.Head)
		if err != nil || asJSON(g.Doc()) != asJSON(h.doc()) {
			t.Errorf("清掉缓存重放应当得到同样的图：%v", err)
		}
	})

	t.Run("E0501-T11 广播与通知", func(t *testing.T) {
		h := setup(t, "mem")
		res := h.must(user, addNode(1, "资源", "root"), addNode(2, "Agent", "root"))
		evs := h.bus.all()
		if len(evs) != 3 || evs[0].Kind != gen.EvOpApplied || evs[0].Seq != 1 || evs[1].Seq != 2 || evs[2].Kind != gen.EvValidate || evs[2].Seq != 0 {
			t.Fatalf("应当是两条 op.applied（seq 为版本号）加一条 validate：%s", asJSON(evs))
		}
		var dv gen.DataValidate
		if err := json.Unmarshal(evs[2].Data, &dv); err != nil || dv.Version != 2 {
			t.Errorf("validate 的 version 应当是 2：%s", evs[2].Data)
		}
		if len(res.Diags) == 0 {
			t.Errorf("Agent 的必填输入没连，Result.Diags 应当有诊断（不阻止提交）")
		}
		if _, err := h.submit(user, setLabel(99, "x")); err == nil {
			t.Fatal("应当失败")
		}
		if len(h.bus.all()) != 3 {
			t.Errorf("失败的批次不应当发事件")
		}
		h.runs.mu.Lock()
		defer h.runs.mu.Unlock()
		if !slices.Equal(h.runs.versions, []int64{2}) {
			t.Errorf("RunNotifier 应当收到最后的版本 2：%v", h.runs.versions)
		}
	})

	t.Run("E0501-T12 rebase 等价性", func(t *testing.T) {
		rapid.Check(t, func(rt *rapid.T) {
			a, b := setup(t, "mem"), setup(t, "mem")
			k := rapid.IntRange(2, 6).Draw(rt, "k")
			var init []gen.Op
			for i := 1; i <= k; i++ {
				init = append(init, addNode(i, "资源", "root"))
			}
			a.must(user, init...)
			b.must(user, init...)
			group := func(label string, parity, extra int) []gen.Op {
				var ops []gen.Op
				for j := rapid.IntRange(1, 5).Draw(rt, label+"n"); j > 0; j-- {
					node := 2*rapid.IntRange(0, (k-1)/2).Draw(rt, label+"node") + 1 + parity
					if node > k {
						node = 1 + parity
					}
					switch rapid.IntRange(0, 4).Draw(rt, label+"kind") {
					case 0:
						ops = append(ops, setLabel(node, rapid.SampledFrom([]string{"甲", "乙", "丙"}).Draw(rt, label+"label")))
					case 1:
						ops = append(ops, moveNode(node, int64(rapid.IntRange(0, 9).Draw(rt, label+"x")), 0))
					case 2:
						ops = append(ops, setParam(node, "kind", rapid.SampledFrom([]string{"模型", "列表", "常量"}).Draw(rt, label+"v")))
					case 3:
						ops = append(ops, mkop(gen.OpSetTags, gen.PayloadSetTags{Node: nid(node), To: []string{label}}))
					default:
						ops = append(ops, addNode(extra+j, "资源", "root"))
					}
				}
				return ops
			}
			first, second := group("a", 0, 100), group("b", 1, 200)
			if k < 2 {
				return
			}
			base := a.head()
			if _, err := a.p.Submit(ctx, a.batch(user, base, first...)); err != nil {
				rt.Fatalf("第一组：%v", err)
			}
			res, err := a.p.Submit(ctx, a.batch(user, base, second...))
			if err != nil || !res.Rebased {
				rt.Fatalf("第二组以旧版本提交应当自动 rebase：%v %+v", err, res)
			}
			b.must(user, first...)
			b.must(user, second...)
			if x, y := asJSON(a.doc().Root), asJSON(b.doc().Root); x != y {
				rt.Fatalf("自动 rebase 和按最新版本提交的结果不同：\n%s\n%s", x, y)
			}
		})
	})

	t.Run("E0501-T13 提议与急停接线", func(t *testing.T) {
		h := setup(t, "mem", func(o *op.Options) { o.Authz = modelPropose{} })
		h.must(user, addNode(1, "资源", "root"))
		if _, err := h.submit(model, withReason(setLabel(1, "甲"), "改名")); !errors.Is(err, op.ErrForbidden) {
			t.Errorf("没有 Proposer 时转提议应当 403：%v", err)
		}
		prop := &fakeProposer{id: "01JCQP00000000000000000001"}
		h2 := setup(t, "mem", func(o *op.Options) { o.Authz = modelPropose{}; o.Proposer = prop })
		h2.must(user, addNode(1, "资源", "root"))
		res, err := h2.submit(model, withReason(setLabel(1, "甲"), "改名"))
		if err != nil || res.Proposal != prop.id || len(res.Applied) != 0 || len(h2.records()) != 1 || prop.n != 1 {
			t.Errorf("有 Proposer 时应当返回提议、不落 Op：%v %+v", err, res)
		}
		es := &fakeEstop{on: true}
		h3 := setup(t, "mem", func(o *op.Options) { o.Estop = es })
		if _, err := h3.submit(model, withReason(addNode(1, "资源", "root"), "加")); !errors.Is(err, op.ErrEstopped) {
			t.Errorf("急停中模型的批次应当 423：%v", err)
		}
		h3.must(user, addNode(1, "资源", "root"))
	})
}

type modelPropose struct{}

func (modelPropose) Check(_ context.Context, r authz.Request) authz.Result {
	if r.Actor.Kind == gen.ActorKindModel {
		return authz.Result{Effect: authz.EffectPropose}
	}
	return authz.Result{Effect: authz.EffectAllow}
}

type fakeProposer struct {
	id string
	n  int
}

func (f *fakeProposer) Propose(context.Context, op.Repo, op.Batch, []authz.Result) (string, error) {
	f.n++
	return f.id, nil
}

type fakeEstop struct{ on bool }

func (e *fakeEstop) Active(string) bool { return e.on }
