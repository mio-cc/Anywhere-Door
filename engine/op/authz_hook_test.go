package op

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/mio-cc/anywhere-door/engine/authz"
	"github.com/mio-cc/anywhere-door/engine/graph"
	"github.com/mio-cc/anywhere-door/gen"
)

var (
	userA    = gen.Actor{Kind: gen.ActorKindUser, ID: "local"}
	modelA   = gen.Actor{Kind: gen.ActorKindModel, ID: "chat:01JCMS00000000000000000001"}
	runtimeA = gen.Actor{Kind: gen.ActorKindRuntime, ID: "migrate"}
)

// lockedGraph 是 fixtureDoc 改出的图：locks 是节点编号 → 锁。
func lockedGraph(t *testing.T, locks map[int]string) *graph.Graph {
	t.Helper()
	d := fixtureDoc()
	var walk func(sg *gen.Subgraph)
	walk = func(sg *gen.Subgraph) {
		for i := range sg.Nodes {
			for n, l := range locks {
				if sg.Nodes[i].ID == tid(n) {
					sg.Nodes[i].Lock = &l
				}
			}
			if sg.Nodes[i].Children != nil {
				walk(sg.Nodes[i].Children)
			}
		}
	}
	walk(&d.Root)
	g, err := graph.FromDoc(d)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// targeted 给 Op 填上作用对象（流水线第 5 步做的事）。
func targeted(t *testing.T, g *graph.Graph, ops ...gen.Op) []gen.Op {
	t.Helper()
	for i := range ops {
		tg, err := Target(&ops[i], g)
		if err != nil {
			t.Fatal(err)
		}
		ops[i].Target = &tg
	}
	return ops
}

// recChecker 记下收到的请求，按动作给结果（没写的是 allow）。
type recChecker struct {
	by   map[gen.Action]authz.Effect
	reqs []authz.Request
}

func (c *recChecker) Check(_ context.Context, r authz.Request) authz.Result {
	c.reqs = append(c.reqs, r)
	e, ok := c.by[r.Action]
	if !ok {
		e = authz.EffectAllow
	}
	return authz.Result{Effect: e, Reasons: []string{"规则 " + string(r.Action)}}
}

type fakeEstop struct {
	on   bool
	runs []string
}

func (e *fakeEstop) Active(runID string) bool {
	e.runs = append(e.runs, runID)
	return e.on
}

func opLabel(t *testing.T, n, node int) gen.Op {
	return mkOp(t, n, gen.OpSetLabel, gen.PayloadSetLabel{Node: tid(node), To: "甲"})
}

func opParam(t *testing.T, n, node int) gen.Op {
	return mkOp(t, n, gen.OpSetParam, gen.PayloadSetParam{Node: tid(node), Key: "kind", To: raw("列表")})
}

func opMove(t *testing.T, n, node int) gen.Op {
	return mkOp(t, n, gen.OpMoveNodes, gen.PayloadMoveNodes{Moves: []gen.PayloadMoveNodesMovesItem{{ID: tid(node), To: gen.Pos{X: 1}}}})
}

func authRun(t *testing.T, chk authz.Checker, es Estop, actor gen.Actor, g *graph.Graph, ops ...gen.Op) (Decision, []authz.Result, error) {
	t.Helper()
	return authorize(context.Background(), chk, es, authInput{Actor: actor, G: g, Ops: targeted(t, g, ops...)})
}

// wantLocked：拒绝，错误是 ErrNodeLocked，锁住的是 node 的 lock。
func wantLocked(t *testing.T, name string, d Decision, err error, node int, lock string) {
	t.Helper()
	var de *DeniedError
	if d != Deny || !errors.Is(err, ErrNodeLocked) || !errors.As(err, &de) {
		t.Errorf("%s：应当是 node_locked，实际 %v %v", name, d, err)
		return
	}
	if de.Node != string(tid(node)) || de.Lock != lock {
		t.Errorf("%s：锁住的应当是 %s（%s），实际 %s（%s）", name, tid(node), lock, de.Node, de.Lock)
	}
}

func wantApply(t *testing.T, name string, d Decision, err error) {
	t.Helper()
	if d != Apply || err != nil {
		t.Errorf("%s：应当直接应用，实际 %v %v", name, d, err)
	}
}

func TestAuthorize(t *testing.T) {
	fixed := authz.Fixed{}

	t.Run("E0505-T1 锁", func(t *testing.T) {
		g := lockedGraph(t, map[int]string{3: "user"})
		chk := &recChecker{by: map[gen.Action]authz.Effect{"op.set_param": authz.EffectPropose}}
		d, _, err := authRun(t, chk, nil, modelA, g, opParam(t, 1, 3))
		wantLocked(t, "模型改 user 锁节点的参数（不转提议）", d, err, 3, "user")
		if len(chk.reqs) != 0 {
			t.Errorf("锁住时不应当再调 Check：%v", chk.reqs)
		}
		d, _, err = authRun(t, fixed, nil, modelA, g, mkOp(t, 2, gen.OpDisconnect, gen.PayloadDisconnect{IDs: []gen.Ulid{teid(1)}}))
		wantLocked(t, "模型断开 user 锁节点的连线", d, err, 3, "user")
		d, _, err = authRun(t, fixed, nil, userA, g, opParam(t, 3, 3))
		wantApply(t, "用户改 user 锁节点", d, err)
		d, _, err = authRun(t, fixed, nil, modelA, g, opMove(t, 4, 3))
		wantApply(t, "模型移动 user 锁节点（C0303-T3）", d, err)

		g = lockedGraph(t, map[int]string{10: "user"})
		d, _, err = authRun(t, fixed, nil, modelA, g, opLabel(t, 5, 11))
		wantLocked(t, "容器 user 锁时模型改子图里的节点", d, err, 10, "user")
		d, _, err = authRun(t, fixed, nil, modelA, g, mkOp(t, 6, gen.OpAddNode, gen.PayloadAddNode{Scope: string(tid(10)), Node: tnode(20, "Agent")}))
		wantLocked(t, "容器 user 锁时模型往里加节点", d, err, 10, "user")
		d, _, err = authRun(t, fixed, nil, modelA, g, opLabel(t, 7, 4))
		wantApply(t, "模型改别处的节点", d, err)

		g = lockedGraph(t, map[int]string{3: "frozen"})
		d, _, err = authRun(t, fixed, nil, userA, g, opParam(t, 8, 3))
		wantLocked(t, "用户改 frozen 节点（C0303-T2）", d, err, 3, "frozen")
		d, _, err = authRun(t, fixed, nil, modelA, g, opMove(t, 9, 3))
		wantLocked(t, "模型移动 frozen 节点", d, err, 3, "frozen")
		d, _, err = authRun(t, fixed, nil, userA, g, mkOp(t, 10, gen.OpSetLock, gen.PayloadSetLock{Node: tid(3), To: "none"}))
		wantApply(t, "用户解除 frozen 锁", d, err)
		g = lockedGraph(t, map[int]string{3: "none"})
		d, _, err = authRun(t, fixed, nil, userA, g, opParam(t, 11, 3))
		wantApply(t, "解锁之后用户可以改", d, err)

		g = lockedGraph(t, map[int]string{10: "user", 11: "frozen"})
		d, _, err = authRun(t, fixed, nil, userA, g, opLabel(t, 12, 11))
		wantLocked(t, "frozen 优先于 user", d, err, 11, "frozen")
	})

	t.Run("E0505-T2 整批提议", func(t *testing.T) {
		g := fixture(t)
		chk := &recChecker{by: map[gen.Action]authz.Effect{"op.set_param": authz.EffectPropose}}
		d, rs, err := authRun(t, chk, nil, modelA, g, opLabel(t, 1, 3), opParam(t, 2, 4))
		if d != Propose || err != nil || len(rs) != 2 {
			t.Errorf("一条 allow、一条 propose 应当整批转为提议（C08-T1）：%v %v %v", d, rs, err)
		}
		chk = &recChecker{by: map[gen.Action]authz.Effect{"op.set_param": authz.EffectAsk}}
		if d, _, err := authRun(t, chk, nil, modelA, g, opParam(t, 3, 4)); d != Propose || err != nil {
			t.Errorf("ask 也转为提议：%v %v", d, err)
		}
		chk = &recChecker{by: map[gen.Action]authz.Effect{"op.set_param": authz.EffectPropose, "op.set_label": authz.EffectDeny}}
		d, _, err = authRun(t, chk, nil, modelA, g, opLabel(t, 4, 3), opParam(t, 5, 4))
		var de *DeniedError
		if d != Deny || !errors.Is(err, ErrForbidden) || !errors.As(err, &de) || !slices.Equal(de.Reasons, []string{"规则 op.set_label"}) {
			t.Errorf("有一条 deny 应当整批拒绝，带原因：%v %v", d, err)
		}
		if len(chk.reqs) != 2 || chk.reqs[0].Target.Graph != g.ID() || chk.reqs[0].Target.Scope != "root" ||
			!slices.Equal(chk.reqs[0].Target.Nodes, []string{string(tid(3))}) {
			t.Errorf("Check 收到的请求不对：%+v", chk.reqs)
		}
	})

	t.Run("E0505-T3 急停", func(t *testing.T) {
		g := fixture(t)
		es := &fakeEstop{on: true}
		chk := &recChecker{}
		d, _, err := authorize(context.Background(), chk, es, authInput{Actor: modelA, G: g, RunID: "01JCMR00000000000000000001", Ops: targeted(t, g, opLabel(t, 1, 3))})
		if d != Deny || !errors.Is(err, ErrEstopped) {
			t.Errorf("急停中模型的 Op 应当 423：%v %v", d, err)
		}
		if !slices.Equal(es.runs, []string{"01JCMR00000000000000000001"}) || len(chk.reqs) != 0 {
			t.Errorf("急停按运行判断、不再调 Check：%v %v", es.runs, chk.reqs)
		}
		d, _, err = authRun(t, chk, es, userA, g, opLabel(t, 2, 3))
		wantApply(t, "急停中用户照常改图（P.09 K82）", d, err)
		d, _, err = authRun(t, chk, nil, modelA, g, opLabel(t, 3, 3))
		wantApply(t, "没有急停时模型照常", d, err)
	})

	t.Run("E0505-T4 M1 桩与动作名", func(t *testing.T) {
		r := fixed.Check(context.Background(), authz.Request{Actor: modelA, Action: "op.remove_nodes"})
		if r.Effect != authz.EffectAllow || !slices.Equal(r.Reasons, []string{authz.FixedReason}) {
			t.Errorf("Fixed 应当一律放行：%+v", r)
		}
		g := fixture(t)
		chk := &recChecker{}
		d, _, err := authRun(t, chk, nil, modelA, g, mkOp(t, 1, gen.OpSetLock, gen.PayloadSetLock{Node: tid(3), To: "user"}))
		if d != Deny || !errors.Is(err, ErrForbidden) {
			t.Errorf("模型的 set_lock 应当 403：%v %v", d, err)
		}
		mig := mkOp(t, 2, gen.OpMigrate, gen.PayloadMigrate{FromSchema: "1.0", ToSchema: "1.1"})
		if d, _, err := authRun(t, chk, nil, userA, g, mig); d != Deny || !errors.Is(err, ErrForbidden) {
			t.Errorf("用户的 migrate 应当 403：%v %v", d, err)
		}
		d, _, err = authRun(t, chk, nil, runtimeA, lockedGraph(t, map[int]string{3: "frozen"}), mig)
		wantApply(t, "内核的 migrate（不查锁、不调 Check）", d, err)
		d, _, err = authRun(t, chk, nil, userA, g, mkOp(t, 3, gen.OpSetLock, gen.PayloadSetLock{Node: tid(3), To: "user"}))
		wantApply(t, "用户的 set_lock", d, err)
		if len(chk.reqs) != 0 {
			t.Errorf("set_lock、migrate 只看操作者，不调 Check：%v", chk.reqs)
		}
		for k, want := range map[gen.OpKind]gen.Action{
			gen.OpMoveNodes: "op.view", gen.OpSetNodeUI: "op.view", gen.OpAddNote: "op.view", gen.OpUpdateNote: "op.view",
			gen.OpRemoveNotes: "op.view", gen.OpSetMandate: "authz.set_mandate", gen.OpAddNode: "op.add_node",
			gen.OpRevertTo: "op.revert_to", gen.OpSetParam: "op.set_param",
		} {
			if got := ActionOf(k); got != want {
				t.Errorf("ActionOf(%s) = %s，应当是 %s", k, got, want)
			}
		}
		_, _, err = authorize(context.Background(), chk, nil, authInput{Actor: modelA, Approver: &userA, G: g, Ops: targeted(t, g, opLabel(t, 4, 3))})
		if err != nil || len(chk.reqs) != 1 || chk.reqs[0].Actor != userA {
			t.Errorf("有 Approver 时 Check 收到的应当是用户（C.08-R7）：%v %+v", err, chk.reqs)
		}
		_, _, _ = authorize(context.Background(), chk, nil, authInput{Actor: modelA, Taint: true, Mandate: "01JCMM00000000000000000001", G: g, Ops: targeted(t, g, opLabel(t, 5, 3))})
		if last := chk.reqs[len(chk.reqs)-1]; !last.Taint || last.Mandate != "01JCMM00000000000000000001" {
			t.Errorf("taint、mandate 应当传给 Check：%+v", last)
		}
	})
}
