package op

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/mio-cc/anywhere-door/gen"
)

// ErrNothingToUndo：没有可撤销（或可重做）的组；API 层返回 200 + {applied: []}，不当作错误（E.05.02 §7）。
var ErrNothingToUndo = errors.New("op: 没有可撤销（或可重做）的操作")

// 撤销、重做批次的 reason 前缀（C.05.03-R3）；cancelPairs 靠它认出互相抵消的记录。
const (
	undoPrefix = "撤销 "
	redoPrefix = "重做 "
	scanChunk  = 200
)

func init() {
	filterMissed = cancelPairs
}

// Undo 撤销这个操作者撤销栈栈顶的组（E.05.02 §5.3）：组里的记录倒序、每条的 inverse 按原顺序拼成一个新批次，
// 走 Submit 的全部步骤（授权按发起者，C.05.03-R2）；成功后同一个事务里弹出撤销栈栈顶、把组推进重做栈。
func (p *Pipeline) Undo(ctx context.Context, graphID, branch string, actor gen.Actor) (*Result, error) {
	var top UndoEntry
	b := Batch{GraphID: graphID, Branch: branch, Actor: actor, internal: true, mode: modeUndo}
	b.pre = func(ctx context.Context, _ *Pipeline, r Repo, c *cached, bb *Batch) error {
		t, ok, err := r.UndoTop(ctx, graphID, branch, actor.ID, false)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNothingToUndo
		}
		recs, err := groupRecords(ctx, r, graphID, branch, t.GroupID, c.version)
		if err != nil {
			return err
		}
		if len(recs) == 0 {
			return fmt.Errorf("op: 撤销栈里的组 %s 在日志里找不到", t.GroupID)
		}
		var ops []gen.Op
		for i := len(recs) - 1; i >= 0; i-- {
			if recs[i].Op.Inverse == nil {
				return &ConflictError{Head: c.version, Conflicts: []Conflict{{OpID: opIDOf(recs[i]), Object: keyDoc}},
					Reason: fmt.Sprintf("第 %d 版的 %s 不可逆", recs[i].Version, recs[i].Op.Kind)}
			}
			ops = append(ops, *recs[i].Op.Inverse...)
		}
		reason := undoPrefix + t.GroupID
		for i := range ops {
			ops[i].Reason = ptr(reason)
		}
		bb.Ops, bb.BaseVersion, top = ops, recs[len(recs)-1].Version, t
		return nil
	}
	b.onTx = func(ctx context.Context, r Repo, _ *Result) error {
		if err := r.UndoMark(ctx, top, true); err != nil {
			return err
		}
		id := actor.ID + redoSuffix
		seq, err := nextSeq(ctx, r, graphID, branch, id)
		if err != nil {
			return err
		}
		return r.UndoPush(ctx, UndoEntry{GraphID: graphID, Branch: branch, ActorID: id, Seq: seq, GroupID: top.GroupID})
	}
	return p.submit(ctx, b)
}

// Redo 重做重做栈栈顶的组（E.05.02 §5.4）：重新提交组里记录的 Op（服务器补全后的 kind、payload、coalesce_key），
// base 是撤销它的那一批的最后一个版本；成功后弹出重做栈栈顶，新的组由流水线推进撤销栈。
func (p *Pipeline) Redo(ctx context.Context, graphID, branch string, actor gen.Actor) (*Result, error) {
	var top UndoEntry
	rid := actor.ID + redoSuffix
	b := Batch{GraphID: graphID, Branch: branch, Actor: actor, internal: true, mode: modeRedo}
	b.pre = func(ctx context.Context, _ *Pipeline, r Repo, c *cached, bb *Batch) error {
		t, ok, err := r.UndoTop(ctx, graphID, branch, rid, false)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNothingToUndo
		}
		recs, err := groupRecords(ctx, r, graphID, branch, t.GroupID, c.version)
		if err != nil {
			return err
		}
		if len(recs) == 0 {
			return fmt.Errorf("op: 重做栈里的组 %s 在日志里找不到", t.GroupID)
		}
		base, found, err := findBatch(ctx, r, graphID, branch, c.version, undoPrefix+t.GroupID, actor)
		if err != nil {
			return err
		}
		if !found {
			return &ConflictError{Head: c.version, Reason: "找不到撤销组 " + t.GroupID + " 的记录"}
		}
		ops := make([]gen.Op, len(recs))
		for i, rec := range recs {
			ops[i] = gen.Op{Kind: rec.Op.Kind, Payload: append(json.RawMessage{}, rec.Op.Payload...),
				CoalesceKey: rec.Op.CoalesceKey, Reason: ptr(redoPrefix + t.GroupID)}
		}
		bb.Ops, bb.BaseVersion, top = ops, base, t
		return nil
	}
	b.onTx = func(ctx context.Context, r Repo, _ *Result) error { return r.UndoMark(ctx, top, true) }
	return p.submit(ctx, b)
}

func groupOf(rec gen.OpRecord) string {
	if rec.Op.GroupID == nil {
		return ""
	}
	return string(*rec.Op.GroupID)
}

func opIDOf(rec gen.OpRecord) string {
	if rec.Op.ID == nil {
		return ""
	}
	return string(*rec.Op.ID)
}

func reasonOf(rec gen.OpRecord) string {
	if rec.Op.Reason == nil {
		return ""
	}
	return *rec.Op.Reason
}

func sameActor(a *gen.Actor, b gen.Actor) bool { return a != nil && a.Kind == b.Kind && a.ID == b.ID }

func actorKey(a *gen.Actor) string {
	if a == nil {
		return ""
	}
	return string(a.Kind) + ":" + a.ID
}

// scanBack 从 head 往前每次取 scanChunk 条，逐条（从新到旧）交给 fn，fn 返回 true 时停。
func scanBack(ctx context.Context, r Repo, graphID, branch string, head int64, fn func(gen.OpRecord) bool) error {
	hi := head
	for hi > 0 {
		lo := max(hi-scanChunk, 0)
		recs, err := r.Range(ctx, graphID, branch, lo, hi)
		if err != nil {
			return repoErr(err, "Op 日志")
		}
		for i := len(recs) - 1; i >= 0; i-- {
			if fn(recs[i]) {
				return nil
			}
		}
		hi = lo
	}
	return nil
}

// groupRecords 是一个组在分支里的记录（升序）。一批一个组、版本连续（E.05.02 §5.2）：从 head 往前找到它以后，
// 再遇到不属于它的就停（不同时间复用同一个 group_id 时只认最近的那一段）。
func groupRecords(ctx context.Context, r Repo, graphID, branch, group string, head int64) ([]gen.OpRecord, error) {
	var out []gen.OpRecord
	err := scanBack(ctx, r, graphID, branch, head, func(rec gen.OpRecord) bool {
		if groupOf(rec) == group {
			out = append(out, rec)
			return false
		}
		return len(out) > 0
	})
	slices.Reverse(out)
	return out, err
}

// findBatch 是 reason 等于给定值、操作者相同的最近一条记录的版本（撤销某个组的那一批的最后一个版本）。
func findBatch(ctx context.Context, r Repo, graphID, branch string, head int64, reason string, actor gen.Actor) (int64, bool, error) {
	var v int64
	found := false
	err := scanBack(ctx, r, graphID, branch, head, func(rec gen.OpRecord) bool {
		if reasonOf(rec) == reason && sameActor(rec.Op.Actor, actor) {
			v, found = rec.Version, true
		}
		return found
	})
	return v, found, err
}

// cancelPairs 去掉错过的记录里互相抵消的（E.05.02 §5.6，P.09 K264）：
//
//	① 已撤销的组（记录的 undone 为 true）和撤销它的那一批（reason "撤销 <组>"，操作者相同）；
//	② 剩下的"撤销 <组>"批次和重做它的那一批（reason "重做 <组>"，操作者相同）。
//
// 两者都要在 missed 里才去掉；① 用过的撤销批次不再参与 ②（组、撤销、重做三者都在时净效果是重做，重做要留下）。
func cancelPairs(_ *Batch, missed []gen.OpRecord) []gen.OpRecord {
	type info struct {
		actor  string
		reason string
		undone bool
	}
	groups := map[string]*info{}
	var order []string
	for _, rec := range missed {
		g := groupOf(rec)
		in := groups[g]
		if in == nil {
			in = &info{actor: actorKey(rec.Op.Actor), reason: reasonOf(rec)}
			groups[g] = in
			order = append(order, g)
		}
		if rec.Undone {
			in.undone = true
		}
	}
	undoOf, redoOf := map[string]string{}, map[string]string{} // "<目标组>|<操作者>" → 批次的组
	for _, g := range order {
		in := groups[g]
		if t, ok := strings.CutPrefix(in.reason, undoPrefix); ok {
			undoOf[t+"|"+in.actor] = g
		}
		if t, ok := strings.CutPrefix(in.reason, redoPrefix); ok {
			redoOf[t+"|"+in.actor] = g
		}
	}
	drop, used := map[string]bool{}, map[string]bool{}
	for _, g := range order {
		in := groups[g]
		if !in.undone {
			continue
		}
		if u, ok := undoOf[g+"|"+in.actor]; ok {
			drop[g], drop[u], used[u] = true, true, true
		}
	}
	for k, u := range undoOf {
		if used[u] {
			continue
		}
		if rd, ok := redoOf[k]; ok {
			drop[u], drop[rd] = true, true
		}
	}
	if len(drop) == 0 {
		return missed
	}
	out := make([]gen.OpRecord, 0, len(missed))
	for _, rec := range missed {
		if !drop[groupOf(rec)] {
			out = append(out, rec)
		}
	}
	return out
}
