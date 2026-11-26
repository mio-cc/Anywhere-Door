package op_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/mio-cc/anywhere-door/engine/authz"
	"github.com/mio-cc/anywhere-door/engine/op"
	"github.com/mio-cc/anywhere-door/engine/schema/migrate"
	"github.com/mio-cc/anywhere-door/gen"
)

// recAuthz 一律放行，记下收到的请求。
type recAuthz struct {
	mu   sync.Mutex
	reqs []authz.Request
}

func (a *recAuthz) Check(_ context.Context, r authz.Request) authz.Result {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.reqs = append(a.reqs, r)
	return authz.Result{Effect: authz.EffectAllow}
}

func (a *recAuthz) last() authz.Request {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.reqs[len(a.reqs)-1]
}

// state 是图的内容（不含版本号），比较撤销、重做前后用。
func state(h *harness) string {
	d := h.doc()
	return asJSON(d.Root) + "|" + asJSON(d.Title) + "|" + asJSON(d.Meta) + "|" + d.SchemaVersion
}

func nodeIn(d *gen.Graph, n int) *gen.Node {
	for i := range d.Root.Nodes {
		if d.Root.Nodes[i].ID == nid(n) {
			return &d.Root.Nodes[i]
		}
	}
	return nil
}

func (h *harness) undo(a gen.Actor) {
	h.t.Helper()
	if _, err := h.p.Undo(ctx, h.gid, "main", a); err != nil {
		h.t.Fatalf("Undo：%v", err)
	}
}

func (h *harness) redo(a gen.Actor) {
	h.t.Helper()
	if _, err := h.p.Redo(ctx, h.gid, "main", a); err != nil {
		h.t.Fatalf("Redo：%v", err)
	}
}

func label(n *gen.Node) string {
	if n == nil || n.Label == nil {
		return ""
	}
	return *n.Label
}

func TestUndoRedo(t *testing.T) {
	t.Run("E0502-T1 按 actor 分栈", func(t *testing.T) {
		h := setup(t, "mem")
		h.must(user, addNode(1, "资源", "root"))
		h.must(model, withReason(addNode(2, "资源", "root"), "加一个资源"))
		h.must(user, setLabel(1, "用户的"))
		h.must(model, withReason(setLabel(2, "模型的"), "改名"))
		h.undo(user)
		d := h.doc()
		if label(nodeIn(d, 1)) != "" || label(nodeIn(d, 2)) != "模型的" {
			t.Fatalf("用户的 Undo 只撤销用户的修改：%s", asJSON(d.Root))
		}
		h.undo(model)
		if d := h.doc(); label(nodeIn(d, 2)) != "" {
			t.Fatalf("模型的 Undo 撤销模型的修改：%s", asJSON(d.Root))
		}
		h.undo(user) // 撤销用户加的节点 1（中间夹着已经互相抵消的修改，E.05.02 §5.6）
		d = h.doc()
		if nodeIn(d, 1) != nil || nodeIn(d, 2) == nil {
			t.Errorf("用户第二次 Undo 应当只删掉节点 1：%s", asJSON(d.Root))
		}
		recs := h.records()
		last := recs[len(recs)-1]
		if *last.Op.Actor != user || !strings.HasPrefix(*last.Op.Reason, "撤销 ") {
			t.Errorf("撤销 Op 的 actor 是发起者、reason 是\"撤销 <组>\"：%v %v", last.Op.Actor, last.Op.Reason)
		}
	})

	t.Run("E0502-T2 撤销重做往返", func(t *testing.T) {
		h := setup(t, "mem")
		h.must(user, addNode(1, "资源", "root"), addNode(2, "Agent", "root"))
		h.must(user, connect(1, 1, "ref", 2, "model"))
		before := state(h)
		h.must(user, setLabel(1, "甲"), setParam(1, "kind", "列表"), moveNode(2, 5, 5), mkop(gen.OpDisconnect, gen.PayloadDisconnect{IDs: []gen.Ulid{eid(1)}}))
		after := state(h)
		h.undo(user)
		if state(h) != before {
			t.Fatalf("撤销后应当回到撤销前：\n%s\n%s", state(h), before)
		}
		h.redo(user)
		if state(h) != after {
			t.Fatalf("重做后应当回到重做前：\n%s\n%s", state(h), after)
		}

		// 模型加节点后用户撤回：撤回 Op 的 actor 是用户，授权按用户判定（C0503-T2）
		rec := &recAuthz{}
		h2 := setup(t, "mem", func(o *op.Options) { o.Authz = rec })
		res := h2.must(model, withReason(addNode(1, "资源", "root"), "加一个资源"))
		if _, err := h2.p.RevertOp(ctx, h2.gid, "main", res.Applied[0], user, ""); err != nil {
			t.Fatal(err)
		}
		recs := h2.records()
		last := recs[len(recs)-1]
		if *last.Op.Actor != user || !strings.HasPrefix(*last.Op.Reason, "撤回 "+res.Applied[0]) {
			t.Errorf("撤回 Op 的 actor 应当是用户、reason 是\"撤回 <op_id>\"：%v %v", last.Op.Actor, *last.Op.Reason)
		}
		if a := rec.last(); a.Actor != user || a.Action != "op.remove_nodes" {
			t.Errorf("授权应当按用户判定：%+v", a)
		}
		if len(h2.doc().Root.Nodes) != 0 {
			t.Errorf("撤回后节点应当没了")
		}
		// 用户的 Undo 撤销不到模型的 Op
		h3 := setup(t, "mem")
		h3.must(model, withReason(addNode(1, "资源", "root"), "加一个资源"))
		if _, err := h3.p.Undo(ctx, h3.gid, "main", user); !errors.Is(err, op.ErrNothingToUndo) {
			t.Errorf("用户的 Undo 不应当撤销模型的 Op：%v", err)
		}
	})

	t.Run("E0502-T3 revert_op 冲突", func(t *testing.T) {
		h := setup(t, "mem")
		h.must(user, addNode(1, "资源", "root"))
		res := h.must(user, setLabel(1, "甲"))
		h.must(user, removeNodes(1))
		_, err := h.p.RevertOp(ctx, h.gid, "main", res.Applied[0], user, "")
		var ce *op.ConflictError
		if !errors.As(err, &ce) || !strings.Contains(fmt.Sprint(ce.Conflicts), string(nid(1))) {
			t.Errorf("撤回已被删除的节点上的修改应当 409 并指出节点：%v", err)
		}
		r2 := h.must(user, addNode(2, "资源", "root"))
		h.undo(user)
		if _, err := h.p.RevertOp(ctx, h.gid, "main", r2.Applied[0], user, ""); !errors.As(err, &ce) {
			t.Errorf("撤回已撤销的 Op 应当 409：%v", err)
		}
		if _, err := h.p.RevertOp(ctx, h.gid, "main", "01JCQZ00000000000000000099", user, ""); !errors.Is(err, op.ErrNotFound) {
			t.Errorf("撤回不存在的 Op 应当 404：%v", err)
		}
		if _, err := h.p.RevertOp(ctx, h.gid, "main", res.Applied[0], model, ""); !errors.Is(err, op.ErrBadRequest) {
			t.Errorf("模型撤回不写 reason 应当 400：%v", err)
		}
	})

	t.Run("E0502-T4 revert_to 差分正确", func(t *testing.T) {
		h := setup(t, "mem")
		saved := map[int64]string{0: state(h)}
		for _, o := range []gen.Op{
			addNode(1, "资源", "root"), addNode(2, "Agent", "root"), connect(1, 1, "ref", 2, "model"),
			setLabel(1, "甲"), setParam(1, "kind", "列表"), removeNodes(2),
			mkop(gen.OpSetGraphTitle, gen.PayloadSetGraphTitle{To: "标题"}),
		} {
			res := h.must(user, o)
			saved[res.Version] = state(h)
		}
		for _, v := range []int64{5, 2, 0} { // 后面的回到版本会跨过前面的 revert_to
			if _, err := h.p.RevertTo(ctx, h.gid, "main", v, user, ""); err != nil {
				t.Fatalf("RevertTo(%d)：%v", v, err)
			}
			if got := state(h); got != saved[v] {
				t.Errorf("回到版本 %d 后的图不对：\n%s\n%s", v, got, saved[v])
			}
		}
		head := h.head()
		if res, err := h.p.RevertTo(ctx, h.gid, "main", head, user, ""); err != nil || res.Version != head || len(res.Applied) != 0 {
			t.Errorf("回到 head 应当什么都不做：%v %+v", err, res)
		}
		if _, err := h.p.RevertTo(ctx, h.gid, "main", head+1, user, ""); !errors.Is(err, op.ErrBadRequest) {
			t.Errorf("回到不存在的版本应当 400：%v", err)
		}

		chain := []migrate.Step{{From: "1.0", To: "1.1", Fn: func(d map[string]any) (map[string]any, error) { return d, nil }}}
		hm := setup(t, "mem", func(o *op.Options) { o.Migrations = chain })
		hm.must(user, addNode(1, "资源", "root")) // 第 1 版是 migrate
		var ce *op.ConflictError
		if _, err := hm.p.RevertTo(ctx, hm.gid, "main", 0, user, ""); !errors.As(err, &ce) {
			t.Errorf("跨过 migrate 应当 409：%v", err)
		}
		if _, err := hm.p.RevertTo(ctx, hm.gid, "main", 1, user, ""); err != nil || len(hm.doc().Root.Nodes) != 0 {
			t.Errorf("回到 migrate 之后的版本可以：%v", err)
		}

		// Submit 提交 revert_op、revert_to 与调用方法的结果相同
		a, b := setup(t, "mem"), setup(t, "mem")
		var ra, rb *op.Result
		for _, x := range []*harness{a, b} {
			x.must(user, addNode(1, "资源", "root"))
			r := x.must(user, setLabel(1, "甲"))
			x.must(user, addNode(2, "资源", "root"))
			if x == a {
				ra = r
			} else {
				rb = r
			}
		}
		if _, err := a.p.RevertOp(ctx, a.gid, "main", ra.Applied[0], user, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := b.p.Submit(ctx, b.batch(user, b.head(), mkop(gen.OpRevertOp, gen.PayloadRevertOp{OpID: gen.Ulid(rb.Applied[0])}))); err != nil {
			t.Fatal(err)
		}
		if _, err := a.p.RevertTo(ctx, a.gid, "main", 1, user, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := b.p.Submit(ctx, b.batch(user, b.head(), mkop(gen.OpRevertTo, gen.PayloadRevertTo{Version: 1}))); err != nil {
			t.Fatal(err)
		}
		if state(a) != state(b) {
			t.Errorf("经 Submit 和调用方法的结果应当相同：\n%s\n%s", state(a), state(b))
		}
		if g, _, err := a.reopen().Read(ctx, a.gid, "main", op.Head); err != nil || asJSON(g.Doc()) != asJSON(a.doc()) {
			t.Errorf("重放带 revert_to 的日志应当得到同样的图：%v", err)
		}
	})

	t.Run("E0502-T5 多步重做", func(t *testing.T) {
		h := setup(t, "mem")
		s := []string{state(h)}
		for i := 1; i <= 3; i++ {
			h.must(user, addNode(i, "资源", "root"), setLabel(i, fmt.Sprintf("节点 %d", i)))
			s = append(s, state(h))
		}
		for i := 2; i >= 0; i-- {
			h.undo(user)
			if state(h) != s[i] {
				t.Fatalf("撤销到第 %d 步不对：%s", i, state(h))
			}
		}
		for i := 1; i <= 3; i++ {
			h.redo(user)
			if state(h) != s[i] {
				t.Fatalf("重做到第 %d 步不对：%s", i, state(h))
			}
		}
		if _, err := h.p.Redo(ctx, h.gid, "main", user); !errors.Is(err, op.ErrNothingToUndo) {
			t.Errorf("没有可重做的了：%v", err)
		}
		h.undo(user)
		h.must(user, addNode(9, "资源", "root"))
		if _, err := h.p.Redo(ctx, h.gid, "main", user); !errors.Is(err, op.ErrNothingToUndo) {
			t.Errorf("新的普通操作之后重做栈应当清空：%v", err)
		}
	})

	t.Run("E0502-T6 抵消", func(t *testing.T) {
		h := setup(t, "mem")
		h.must(user, addNode(1, "资源", "root"))
		h.must(user, moveNode(1, 50, 50))
		h.undo(user) // 撤销移动
		h.undo(user) // 再撤销添加：中间的移动和撤销移动互相抵消，不算冲突
		if n := len(h.doc().Root.Nodes); n != 0 {
			t.Errorf("两次撤销后应当没有节点：%d", n)
		}

		h2 := setup(t, "mem")
		h2.must(user, addNode(1, "资源", "root"))
		h2.must(user, setLabel(1, "甲"))
		before := state(h2)
		h2.undo(user)
		h2.undo(user)
		h2.redo(user)
		h2.redo(user) // 撤销添加和重做添加互相抵消（P.09 K264）
		if state(h2) != before {
			t.Errorf("撤销两次、重做两次应当回到原来：\n%s\n%s", state(h2), before)
		}

		fresh := setup(t, "mem")
		if _, err := fresh.p.Undo(ctx, fresh.gid, "main", user); !errors.Is(err, op.ErrNothingToUndo) {
			t.Errorf("没有可撤销的组：%v", err)
		}
	})
}
