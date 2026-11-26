package op

import (
	"context"
	"fmt"
	"strings"

	"github.com/mio-cc/anywhere-door/engine/schema"
	"github.com/mio-cc/anywhere-door/gen"
)

// 撤回一条 Op、回到某个版本（E.05.02 §5.5）。两种都可以经 Submit 提交，由这里的展开换成具体的 Op（E.05-R7）。
func init() {
	expanders[gen.OpRevertOp] = expandRevertOp
	expanders[gen.OpRevertTo] = expandRevertTo
	appliers[gen.OpRevertTo] = applyRevertTo
}

// RevertOp 撤回任意一条 Op（可以是别人的、模型的）。reason 只在 actor 是模型时必填；落库的 reason 是"撤回 <op_id>：<reason>"。
func (p *Pipeline) RevertOp(ctx context.Context, graphID, branch, opID string, actor gen.Actor, reason string) (*Result, error) {
	o := mk(gen.OpRevertOp, gen.PayloadRevertOp{OpID: gen.Ulid(opID)})
	if reason != "" {
		o.Reason = ptr(reason)
	}
	return p.Submit(ctx, Batch{GraphID: graphID, Branch: branch, Actor: actor, Ops: []gen.Op{o}})
}

// RevertTo 回到某个版本：已经是这个版本时什么都不做，返回 head。
func (p *Pipeline) RevertTo(ctx context.Context, graphID, branch string, version int64, actor gen.Actor, reason string) (*Result, error) {
	head, err := p.o.Repo.Head(ctx, graphID, branch)
	if err != nil {
		return nil, repoErr(err, fmt.Sprintf("图 %s 分支 %s", graphID, branch))
	}
	if version == head {
		_, diags, err := p.Read(ctx, graphID, branch, Head)
		if err != nil {
			return nil, err
		}
		return &Result{Version: head, Applied: []string{}, IDMap: map[string]string{}, Diags: diags}, nil
	}
	o := mk(gen.OpRevertTo, gen.PayloadRevertTo{Version: version})
	if reason != "" {
		o.Reason = ptr(reason)
	}
	return p.Submit(ctx, Batch{GraphID: graphID, Branch: branch, BaseVersion: head, Actor: actor, Ops: []gen.Op{o}})
}

func withNote(base string, o *gen.Op) string {
	if o.Reason != nil && strings.TrimSpace(*o.Reason) != "" {
		return base + "：" + *o.Reason
	}
	return base
}

// expandRevertOp：目标 Op 的 inverse 作为新批次，base 是目标 Op 的版本；不能交换时流水线返回 409（C.05.03-R4）。
func expandRevertOp(ctx context.Context, _ *Pipeline, r Repo, c *cached, b *Batch) error {
	o := b.Ops[0]
	pl, err := payloadOf[gen.PayloadRevertOp](&o)
	if err != nil {
		return err
	}
	id := string(pl.OpID)
	rec, err := r.Op(ctx, id)
	if err != nil {
		return repoErr(err, "Op "+id)
	}
	if string(rec.GraphID) != b.GraphID || string(rec.Branch) != b.Branch {
		return notFoundf("图 %s 分支 %s 里没有 Op %s", b.GraphID, b.Branch, id)
	}
	conflict := func(why string) error {
		return &ConflictError{Head: c.version, Conflicts: []Conflict{{OpID: id, Object: keyDoc}}, Reason: why}
	}
	if rec.Undone {
		return conflict("这条 Op 所在的组已经撤销")
	}
	if rec.Op.Inverse == nil {
		return conflict(string(rec.Op.Kind) + " 不可逆")
	}
	if len(*rec.Op.Inverse) == 0 {
		return conflict("这条 Op 没有可撤回的改动")
	}
	ops := append([]gen.Op(nil), *rec.Op.Inverse...)
	reason := withNote("撤回 "+id, &o)
	for i := range ops {
		ops[i].Reason = ptr(reason)
	}
	b.Ops, b.BaseVersion, b.internal = ops, rec.Version, true
	return nil
}

// expandRevertTo：Range(v, head) 的记录倒序、每条的 inverse 按原顺序拼成一条 revert_to（不用 Diff，P.09 K261 ⑧）。
func expandRevertTo(ctx context.Context, _ *Pipeline, r Repo, c *cached, b *Batch) error {
	o := b.Ops[0]
	pl, err := payloadOf[gen.PayloadRevertTo](&o)
	if err != nil {
		return err
	}
	v, head := pl.Version, c.version
	if v < 0 || v > head {
		return badf("版本 %d 不在 0–%d 之间", v, head)
	}
	if v == head {
		return badf("图已经是版本 %d", v)
	}
	recs, err := r.Range(ctx, b.GraphID, b.Branch, v, head)
	if err != nil {
		return repoErr(err, "Op 日志")
	}
	sub := []gen.Op{}
	for i := len(recs) - 1; i >= 0; i-- {
		if recs[i].Op.Inverse == nil {
			return &ConflictError{Head: head, Conflicts: []Conflict{{OpID: opIDOf(recs[i]), Object: keyDoc}},
				Reason: fmt.Sprintf("第 %d 版的 %s 不可逆，回不到版本 %d", recs[i].Version, recs[i].Op.Kind, v)}
		}
		sub = append(sub, *recs[i].Op.Inverse...)
	}
	op := mk(gen.OpRevertTo, gen.PayloadRevertTo{Version: v, Ops: &sub})
	op.Reason = ptr(withNote(fmt.Sprintf("回到版本 %d", v), &o))
	b.Ops, b.BaseVersion, b.internal = []gen.Op{op}, head, true
	return nil
}

// applyRevertTo 按顺序应用 ops 里的每一条；逆操作是 revert_to{应用前的版本, 子 Op 的逆操作倒序}。
// 失败时 m 可能已改了一部分：调用方（流水线）丢掉整个副本。
func applyRevertTo(m *mutable, op *gen.Op, e *env) ([]gen.Op, schema.Touched, error) {
	p, err := payloadOf[gen.PayloadRevertTo](op)
	if err != nil {
		return nil, noTouch, err
	}
	if p.Ops == nil {
		return nil, noTouch, invalidf("revert_to 没有展开的 Op（只能由服务器展开）")
	}
	before := m.doc.Version
	ie := *e
	ie.internal = true
	var invs [][]gen.Op
	var t schema.Touched
	for i := range *p.Ops {
		sub := (*p.Ops)[i]
		a, ok := appliers[sub.Kind]
		if !ok {
			return nil, noTouch, invalidf("revert_to 里的第 %d 条（%s）不能应用", i+1, sub.Kind)
		}
		inv, st, err := a(m, &sub, &ie)
		if err != nil {
			return nil, noTouch, fmt.Errorf("revert_to 里的第 %d 条（%s）：%w", i+1, sub.Kind, err)
		}
		invs = append(invs, inv)
		t.Nodes = append(t.Nodes, st.Nodes...)
		t.Edges = append(t.Edges, st.Edges...)
	}
	back := []gen.Op{}
	for i := len(invs) - 1; i >= 0; i-- {
		back = append(back, invs[i]...)
	}
	return []gen.Op{mk(gen.OpRevertTo, gen.PayloadRevertTo{Version: before, Ops: &back})}, t, nil
}
