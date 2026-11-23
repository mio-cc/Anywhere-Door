package storetest

import (
	"testing"

	"github.com/mio-cc/anywhere-door/engine/store"
	"github.com/mio-cc/anywhere-door/gen"
)

func graphIDs(p store.Page[gen.GraphMeta]) []string {
	out := []string{}
	for _, m := range p.Items {
		out = append(out, string(m.ID))
	}
	return out
}

func graphCases() []tcase {
	return []tcase{
		{"Create、Meta、List 排序和翻页", func(t *testing.T, s store.Store) {
			g1, g2, g3 := newGraph(t, s, 1, 1), newGraph(t, s, 2, 3), newGraph(t, s, 3, 2)
			m, err := s.Graphs().Meta(ctx, g1)
			must(t, err)
			same(t, "Meta", m, graphMeta(g1, 1))
			p, err := s.Graphs().List(ctx, store.GraphQuery{Limit: 2})
			must(t, err)
			same(t, "第一页", graphIDs(p), []string{g2, g3})
			if p.NextCursor == "" {
				t.Fatal("第一页之后应当还有下一页")
			}
			p, err = s.Graphs().List(ctx, store.GraphQuery{Limit: 2, Cursor: p.NextCursor})
			must(t, err)
			same(t, "第二页", graphIDs(p), []string{g1})
			if p.NextCursor != "" {
				t.Errorf("最后一页的 NextCursor 应当为空，是 %q", p.NextCursor)
			}
			p, err = s.Graphs().List(ctx, store.GraphQuery{})
			must(t, err)
			same(t, "Limit 为 0", graphIDs(p), []string{g2, g3, g1})
		}},
		{"同时间按 id 倒序", func(t *testing.T, s store.Store) {
			g1, g2, g3 := newGraph(t, s, 1, 5), newGraph(t, s, 2, 5), newGraph(t, s, 3, 5)
			var got []string
			cursor := ""
			for {
				p, err := s.Graphs().List(ctx, store.GraphQuery{Limit: 1, Cursor: cursor})
				must(t, err)
				got = append(got, graphIDs(p)...)
				if p.NextCursor == "" {
					break
				}
				cursor = p.NextCursor
			}
			same(t, "逐页读出", got, []string{g3, g2, g1})
		}},
		{"Create 重复、Meta 不存在、软删除", func(t *testing.T, s store.Store) {
			g1, g2 := newGraph(t, s, 1, 1), newGraph(t, s, 2, 2)
			isErr(t, "重复 Create", s.Graphs().Create(ctx, graphMeta(g1, 1), doc(g1, "main", 0), user), store.ErrConflict)
			_, err := s.Graphs().Meta(ctx, id(99))
			isErr(t, "Meta 不存在", err, store.ErrNotFound)
			must(t, s.Graphs().SoftDelete(ctx, g1, at(5)))
			p, err := s.Graphs().List(ctx, store.GraphQuery{})
			must(t, err)
			same(t, "List 不含已删除", graphIDs(p), []string{g2})
			m, err := s.Graphs().Meta(ctx, g1)
			must(t, err)
			if m.DeletedAt == nil || !m.DeletedAt.Equal(at(5)) {
				t.Errorf("已删除的图 deleted_at 应当是 %v，是 %v", at(5), m.DeletedAt)
			}
			isErr(t, "删除不存在的图", s.Graphs().SoftDelete(ctx, id(99), at(6)), store.ErrNotFound)
		}},
		{"Commit 推进版本、更新元信息", func(t *testing.T, s store.Store) {
			gid := newGraph(t, s, 1, 1)
			c := commitOf(gid, "main", opRecs(gid, "main", 1, 2), nil, 10)
			must(t, s.Graphs().Commit(ctx, c))
			m, err := s.Graphs().Meta(ctx, gid)
			must(t, err)
			if m.Version != 2 || m.Title != c.Title || m.Stats != c.Stats || !m.UpdatedAt.Equal(at(10)) || !m.CreatedAt.Equal(at(1)) {
				t.Errorf("提交后的元信息不对：%+v", m)
			}
			recs, err := s.Ops().Range(ctx, gid, "main", 0, 10)
			must(t, err)
			same(t, "Op 记录", recs, c.Ops)
		}},
		{"Commit 版本不连续、Op id 重复时不写入", func(t *testing.T, s store.Store) {
			gid := newGraph(t, s, 1, 1)
			commit(t, s, gid, "main", 1, 1, false, 2)
			isErr(t, "跳过版本 2", s.Graphs().Commit(ctx, commitOf(gid, "main", opRecs(gid, "main", 3, 3), nil, 3)), store.ErrConflict)
			isErr(t, "版本 1 重复", s.Graphs().Commit(ctx, commitOf(gid, "main", opRecs(gid, "main", 1, 1), nil, 3)), store.ErrConflict)
			dup := opRecs(gid, "main", 2, 3)
			dup[1].Op.ID = dup[0].Op.ID
			isErr(t, "同一批里 Op id 重复", s.Graphs().Commit(ctx, commitOf(gid, "main", dup, nil, 3)), store.ErrConflict)
			old := opRec(gid, "main", 2, "")
			old.Op.ID = opRec(gid, "main", 1, "").Op.ID
			isErr(t, "Op id 和已有的重复", s.Graphs().Commit(ctx, commitOf(gid, "main", []gen.OpRecord{old}, nil, 3)), store.ErrConflict)
			isErr(t, "空提交", s.Graphs().Commit(ctx, commitOf(gid, "main", nil, nil, 3)), store.ErrConflict)
			isErr(t, "分支不存在", s.Graphs().Commit(ctx, commitOf(gid, "draft/x", opRecs(gid, "draft/x", 2, 2), nil, 3)), store.ErrNotFound)
			isErr(t, "图不存在", s.Graphs().Commit(ctx, commitOf(id(99), "main", opRecs(id(99), "main", 1, 1), nil, 3)), store.ErrNotFound)
			m, err := s.Graphs().Meta(ctx, gid)
			must(t, err)
			if m.Version != 1 || !m.UpdatedAt.Equal(at(2)) {
				t.Errorf("失败的提交改了元信息：%+v", m)
			}
			recs, err := s.Ops().Range(ctx, gid, "main", 0, 10)
			must(t, err)
			same(t, "失败的提交不留 Op", versions(recs), []int64{1})
		}},
		{"Base：最近的全文加之后的 Op", func(t *testing.T, s store.Store) {
			gid := newGraph(t, s, 1, 1)
			commit(t, s, gid, "main", 1, 1, false, 2)
			commit(t, s, gid, "main", 2, 2, true, 3)
			commit(t, s, gid, "main", 3, 3, false, 4)
			for _, c := range []struct {
				v, docV int64
				ops     []int64
			}{{3, 2, []int64{3}}, {2, 2, []int64{}}, {1, 0, []int64{1}}, {0, 0, []int64{}}} {
				snap, err := s.Graphs().Base(ctx, gid, "main", c.v)
				must(t, err)
				if snap.Version != c.docV || snap.Doc == nil || *snap.Doc.Title != *title(c.docV) {
					t.Errorf("Base(%d) 的全文应当是版本 %d，得到 %d", c.v, c.docV, snap.Version)
				}
				same(t, "Base 之后的 Op", versions(snap.Ops), c.ops)
			}
			_, err := s.Graphs().Base(ctx, gid, "main", 4)
			isErr(t, "超过 head", err, store.ErrNotFound)
		}},
		{"Base：草稿分支接 main", func(t *testing.T, s store.Store) {
			gid := newGraph(t, s, 1, 1)
			commit(t, s, gid, "main", 1, 1, false, 2)
			commit(t, s, gid, "main", 2, 2, true, 3)
			commit(t, s, gid, "main", 3, 3, false, 4)
			must(t, s.Graphs().CreateBranch(ctx, gid, gen.BranchMeta{Name: "draft/a", BaseVersion: 2, CreatedAt: ts(5)}))
			must(t, s.Graphs().CreateBranch(ctx, gid, gen.BranchMeta{Name: "draft/b", BaseVersion: 3, CreatedAt: ts(5)}))
			commit(t, s, gid, "draft/a", 3, 4, false, 6)
			snap, err := s.Graphs().Base(ctx, gid, "draft/a", 4)
			must(t, err)
			if snap.Version != 2 || *snap.Doc.Title != "v2" {
				t.Errorf("draft/a 应当接 main 版本 2 的全文，得到 %d", snap.Version)
			}
			same(t, "draft/a 的 Op", snap.Ops, opRecs(gid, "draft/a", 3, 4))
			snap, err = s.Graphs().Base(ctx, gid, "draft/b", 3)
			must(t, err)
			if snap.Version != 2 {
				t.Errorf("draft/b 应当接 main 版本 2 的全文，得到 %d", snap.Version)
			}
			same(t, "draft/b 接上 main 到分出点的 Op", snap.Ops, opRecs(gid, "main", 3, 3))
			commit(t, s, gid, "draft/a", 5, 5, true, 7)
			snap, err = s.Graphs().Base(ctx, gid, "draft/a", 5)
			must(t, err)
			if snap.Version != 5 || len(snap.Ops) != 0 {
				t.Errorf("draft/a 有自己的全文时应当直接用它：%d、%d 条 Op", snap.Version, len(snap.Ops))
			}
			m, err := s.Graphs().Meta(ctx, gid)
			must(t, err)
			if m.Version != 3 || !m.UpdatedAt.Equal(at(7)) {
				t.Errorf("草稿分支的提交只更新 updated_at：%+v", m)
			}
		}},
		{"分支：behind、ahead、名字和删除", func(t *testing.T, s store.Store) {
			gid := newGraph(t, s, 1, 1)
			commit(t, s, gid, "main", 1, 2, false, 2)
			must(t, s.Graphs().CreateBranch(ctx, gid, gen.BranchMeta{Name: "draft/a", BaseVersion: 1, CreatedBy: &user, CreatedAt: ts(5)}))
			commit(t, s, gid, "main", 3, 3, false, 6)
			commit(t, s, gid, "draft/a", 2, 2, false, 7)
			bs, err := s.Graphs().Branches(ctx, gid)
			must(t, err)
			if len(bs) != 2 {
				t.Fatalf("应当有 2 个分支，得到 %d", len(bs))
			}
			d, mn := bs[0], bs[1]
			if d.Name != "draft/a" || d.BaseVersion != 1 || d.Head != 2 || d.Behind != 2 || d.Ahead != 1 || !d.CreatedAt.Equal(at(5)) || d.CreatedBy == nil {
				t.Errorf("draft/a 不对：%+v", d)
			}
			if mn.Name != "main" || mn.BaseVersion != 0 || mn.Head != 3 || mn.Behind != 0 || mn.Ahead != 0 || !mn.CreatedAt.Equal(at(1)) {
				t.Errorf("main 不对：%+v", mn)
			}
			isErr(t, "名字不是 draft/", s.Graphs().CreateBranch(ctx, gid, gen.BranchMeta{Name: "feature/x", BaseVersion: 1, CreatedAt: ts(8)}), store.ErrConflict)
			isErr(t, "已存在", s.Graphs().CreateBranch(ctx, gid, gen.BranchMeta{Name: "draft/a", BaseVersion: 1, CreatedAt: ts(8)}), store.ErrConflict)
			isErr(t, "分出点超过 main 的 head", s.Graphs().CreateBranch(ctx, gid, gen.BranchMeta{Name: "draft/c", BaseVersion: 9, CreatedAt: ts(8)}), store.ErrConflict)
			isErr(t, "图不存在", s.Graphs().CreateBranch(ctx, id(99), gen.BranchMeta{Name: "draft/c", BaseVersion: 0, CreatedAt: ts(8)}), store.ErrNotFound)
			isErr(t, "删 main", s.Graphs().DeleteBranch(ctx, gid, "main", at(9)), store.ErrConflict)
			isErr(t, "删不存在的分支", s.Graphs().DeleteBranch(ctx, gid, "draft/zz", at(9)), store.ErrNotFound)
			must(t, s.Graphs().DeleteBranch(ctx, gid, "draft/a", at(9)))
			bs, err = s.Graphs().Branches(ctx, gid)
			must(t, err)
			if len(bs) != 1 || bs[0].Name != "main" {
				t.Errorf("删除后应当只剩 main：%+v", bs)
			}
			isErr(t, "已删除的名字不能再建", s.Graphs().CreateBranch(ctx, gid, gen.BranchMeta{Name: "draft/a", BaseVersion: 1, CreatedAt: ts(10)}), store.ErrConflict)
			isErr(t, "向已删除的分支提交", s.Graphs().Commit(ctx, commitOf(gid, "draft/a", opRecs(gid, "draft/a", 3, 3), nil, 10)), store.ErrNotFound)
			isErr(t, "再删一次", s.Graphs().DeleteBranch(ctx, gid, "draft/a", at(11)), store.ErrNotFound)
		}},
	}
}
