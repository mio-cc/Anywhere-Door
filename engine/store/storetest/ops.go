package storetest

import (
	"encoding/json"
	"testing"

	"github.com/mio-cc/anywhere-door/engine/store"
	"github.com/mio-cc/anywhere-door/gen"
)

func undoneOf(t *testing.T, s store.Store, gid string) []bool {
	t.Helper()
	recs, err := s.Ops().Range(ctx, gid, "main", 0, 100)
	must(t, err)
	out := []bool{}
	for _, r := range recs {
		out = append(out, r.Undone)
	}
	return out
}

func opCases() []tcase {
	return []tcase{
		{"Range 和 Get", func(t *testing.T, s store.Store) {
			gid := newGraph(t, s, 1, 1)
			commit(t, s, gid, "main", 1, 4, false, 2)
			recs, err := s.Ops().Range(ctx, gid, "main", 1, 3)
			must(t, err)
			same(t, "Range(1, 3]", recs, opRecs(gid, "main", 2, 3))
			recs, err = s.Ops().Range(ctx, gid, "main", 4, 9)
			must(t, err)
			if len(recs) != 0 {
				t.Errorf("head 之后应当没有 Op，得到 %d 条", len(recs))
			}
			r, err := s.Ops().Get(ctx, opID("main", 3))
			must(t, err)
			same(t, "Get", r, opRec(gid, "main", 3, ""))
			_, err = s.Ops().Get(ctx, id(99))
			isErr(t, "Get 不存在", err, store.ErrNotFound)
		}},
		{"Coalesce 只换 payload", func(t *testing.T, s store.Store) {
			gid := newGraph(t, s, 1, 1)
			commit(t, s, gid, "main", 1, 2, false, 2)
			must(t, s.Ops().Coalesce(ctx, opID("main", 1), json.RawMessage(`{"to": "合并后"}`)))
			want := opRec(gid, "main", 1, "")
			want.Op.Payload = json.RawMessage(`{"to":"合并后"}`)
			r, err := s.Ops().Get(ctx, opID("main", 1))
			must(t, err)
			same(t, "合并后的 Op", r, want)
			recs, err := s.Ops().Range(ctx, gid, "main", 0, 2)
			must(t, err)
			same(t, "Range 读到的也是合并后的", recs[0], want)
			isErr(t, "Coalesce 不存在", s.Ops().Coalesce(ctx, id(99), json.RawMessage(`{}`)), store.ErrNotFound)
		}},
		{"撤销栈：UndoTop、UndoMark 同步 Op 的 undone", func(t *testing.T, s store.Store) {
			gid := newGraph(t, s, 1, 1)
			g1, g2 := id(301), id(302)
			ops := []gen.OpRecord{opRec(gid, "main", 1, g1), opRec(gid, "main", 2, g2), opRec(gid, "main", 3, g2)}
			must(t, s.Graphs().Commit(ctx, commitOf(gid, "main", ops, nil, 2)))
			e1 := store.UndoEntry{GraphID: gid, Branch: "main", ActorID: "local", Seq: 1, GroupID: g1}
			e2 := store.UndoEntry{GraphID: gid, Branch: "main", ActorID: "local", Seq: 2, GroupID: g2}
			must(t, s.Ops().UndoPush(ctx, e1))
			must(t, s.Ops().UndoPush(ctx, e2))
			_, ok, err := s.Ops().UndoTop(ctx, gid, "main", "other", false)
			must(t, err)
			if ok {
				t.Error("别的 actor 的撤销栈应当是空的")
			}
			top, ok, err := s.Ops().UndoTop(ctx, gid, "main", "local", false)
			must(t, err)
			same(t, "栈顶", []any{ok, top}, []any{true, e2})
			must(t, s.Ops().UndoMark(ctx, e2, true))
			same(t, "撤销 g2 后 Op 的 undone", undoneOf(t, s, gid), []bool{false, true, true})
			top, ok, err = s.Ops().UndoTop(ctx, gid, "main", "local", false)
			must(t, err)
			same(t, "没撤销的栈顶", []any{ok, top}, []any{true, e1})
			e2.Undone = true
			top, ok, err = s.Ops().UndoTop(ctx, gid, "main", "local", true)
			must(t, err)
			same(t, "已撤销的栈顶", []any{ok, top}, []any{true, e2})
			must(t, s.Ops().UndoMark(ctx, e2, false))
			same(t, "重做 g2 后 Op 的 undone", undoneOf(t, s, gid), []bool{false, false, false})
			_, ok, err = s.Ops().UndoTop(ctx, gid, "main", "local", true)
			must(t, err)
			if ok {
				t.Error("重做之后应当没有已撤销的项")
			}
		}},
	}
}
