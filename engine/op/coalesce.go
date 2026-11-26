package op

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/mio-cc/anywhere-door/engine/graph"
	"github.com/mio-cc/anywhere-door/engine/schema"
	"github.com/mio-cc/anywhere-door/gen"
)

// coalesceWindow：同一个操作者、同一个 coalesce_key 的两次提交间隔不到它就原地合并（C.05.04 §4）。
const coalesceWindow = 2 * time.Second

func init() { coalescer = tryCoalesce }

// tryCoalesce 是拖动合并（E.05.04 §5），在 Submit 第 4 步调用。条件不满足时 handled 为 false，按普通 Op 往下走；
// 只合并分支最新一条（存储的 Coalesce 只改 payload，不改那一版的全文和哈希，P.09 K261 ③）。
func tryCoalesce(ctx context.Context, p *Pipeline, r Repo, c *cached, b *Batch) (bool, *Result, func(), error) {
	no := func() (bool, *Result, func(), error) { return false, nil, nil, nil }
	if len(b.Ops) != 1 || c.version == 0 || b.BaseVersion != c.version {
		return no()
	}
	o := b.Ops[0]
	if o.CoalesceKey == nil || (o.Kind != gen.OpMoveNodes && o.Kind != gen.OpSetParam) {
		return no()
	}
	recs, err := r.Range(ctx, b.GraphID, b.Branch, c.version-1, c.version)
	if err != nil {
		return false, nil, nil, repoErr(err, "上一条 Op")
	}
	if len(recs) != 1 {
		return no()
	}
	prev := recs[0]
	po := prev.Op
	if po.ID == nil || po.Kind != o.Kind || po.CoalesceKey == nil || *po.CoalesceKey != *o.CoalesceKey || prev.Undone || !sameActor(po.Actor, b.Actor) {
		return no()
	}
	now := p.o.Now().UTC()
	last := c.lastCoalesce
	if last.IsZero() && po.Ts != nil {
		last = po.Ts.Time
	}
	if now.Sub(last) >= coalesceWindow {
		return no()
	}
	if same, err := sameObject(&po, &o); err != nil || !same {
		return no()
	}

	m := newMutable(cloneDoc(c.doc))
	inv, touched, err := p.applyOne(m, &o, p.env(false, false))
	if err != nil {
		return false, nil, nil, fmt.Errorf("第 1 条（%s）：%w", o.Kind, err)
	}
	if o.Kind == gen.OpSetParam && (len(inv) != 1 || len(touched.Edges) > 0) {
		return no() // 动态端口消失：一并删除的连线要进逆操作，按普通 Op（§5 条件 5）
	}
	t, err := Target(&o, c.g)
	if err != nil {
		return false, nil, nil, badf("第 1 条：%v", err)
	}
	o.Target = &t
	d, _, err := authorize(ctx, p.o.Authz, p.o.Estop, authInput{Actor: b.Actor, Approver: b.Approver, Taint: b.Taint,
		Mandate: b.MandateID, RunID: b.RunID, Ops: []gen.Op{o}, G: c.g})
	if err != nil {
		return false, nil, nil, err
	}
	if d != Apply {
		return no() // 要转提议的按普通 Op 走
	}
	ng, err := graph.FromDoc(m.doc)
	if err != nil {
		return false, nil, nil, invalidf("%v", err)
	}
	diags := c.diags
	if o.Kind != gen.OpMoveNodes {
		diags = nonNil(schema.ValidateIncremental(ng, c.diags, touched, p.o.View, schema.Opts{ForRun: true}))
		if bl := blocking(c.diags, diags); len(bl) > 0 {
			return false, nil, nil, &InvalidGraphError{Diags: bl}
		}
	}
	prevID := string(*po.ID)
	if err := r.Coalesce(ctx, prevID, o.Payload); err != nil {
		return false, nil, nil, repoErr(err, "合并 "+prevID)
	}
	nc := &cached{version: c.version, doc: m.doc, g: ng, diags: diags, lastFull: c.lastFull, lastCoalesce: now}
	res := &Result{Version: c.version, Applied: []string{prevID}, IDMap: map[string]string{}, Diags: append([]gen.Diag{}, diags...)}
	ts := gen.Timestamp{Time: now}
	gid := gen.Ulid(b.GraphID)
	evs := []gen.GraphEvent{{Actor: b.Actor, GraphID: gid, Kind: gen.EvOpCoalesced, Seq: c.version, Ts: ts,
		Data: mustRaw(gen.DataOpCoalesced{OpID: gen.Ulid(prevID), To: o.Payload, Version: c.version})}}
	structural := o.Kind != gen.OpMoveNodes
	if structural {
		evs = append(evs, gen.GraphEvent{Actor: b.Actor, GraphID: gid, Kind: gen.EvValidate, Seq: 0, Ts: ts,
			Data: mustRaw(gen.DataValidate{Diags: mustRaw(diags), Version: c.version})})
	}
	k := cacheKey{b.GraphID, b.Branch}
	post := func() {
		p.cachePut(k, nc)
		if p.o.Bus != nil {
			p.o.Bus.Publish(evs)
		}
		if p.o.Runs != nil && structural {
			p.o.Runs.GraphChanged(b.GraphID, b.Branch, c.version)
		}
	}
	return true, res, post, nil
}

// sameObject：作用对象相同。move_nodes 是节点集合相同；set_param 是 (node, key, path) 相同。
func sameObject(a, b *gen.Op) (bool, error) {
	switch a.Kind {
	case gen.OpMoveNodes:
		pa, err := decode[gen.PayloadMoveNodes](a)
		if err != nil {
			return false, err
		}
		pb, err := decode[gen.PayloadMoveNodes](b)
		if err != nil {
			return false, err
		}
		ids := func(p *gen.PayloadMoveNodes) []string {
			out := make([]string, len(p.Moves))
			for i, m := range p.Moves {
				out[i] = string(m.ID)
			}
			slices.Sort(out)
			return slices.Compact(out)
		}
		return slices.Equal(ids(pa), ids(pb)), nil
	case gen.OpSetParam:
		pa, err := decode[gen.PayloadSetParam](a)
		if err != nil {
			return false, err
		}
		pb, err := decode[gen.PayloadSetParam](b)
		if err != nil {
			return false, err
		}
		path := func(p *gen.PayloadSetParam) []string {
			if p.Path == nil {
				return nil
			}
			return *p.Path
		}
		return pa.Node == pb.Node && pa.Key == pb.Key && slices.Equal(path(pa), path(pb)), nil
	}
	return false, nil
}
