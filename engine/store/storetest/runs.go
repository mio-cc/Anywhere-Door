package storetest

import (
	"strconv"
	"testing"

	"github.com/mio-cc/anywhere-door/engine/store"
	"github.com/mio-cc/anywhere-door/gen"
)

func runIDs(ms []gen.RunMeta) []string {
	out := []string{}
	for _, m := range ms {
		out = append(out, string(m.ID))
	}
	return out
}

func listRuns(t *testing.T, s store.Store, q store.RunQuery) []string {
	t.Helper()
	var got []string
	for {
		p, err := s.Runs().List(ctx, q)
		must(t, err)
		got = append(got, runIDs(p.Items)...)
		if p.NextCursor == "" {
			return got
		}
		q.Cursor = p.NextCursor
	}
}

func nodeRow(key string, state gen.NodeState, seq int64) gen.NodeStateRow {
	return gen.NodeStateRow{ExecKey: key, NodeID: gen.Ulid(id(7)), Loc: gen.Loc{NodeID: gen.Ulid(id(7)), Path: "根图/写作"},
		State: state, Attempt: 1, LastSeq: seq}
}

func runCases() []tcase {
	return []tcase{
		{"Create 分配 no、Get", func(t *testing.T, s store.Store) {
			a, b := id(1), id(2)
			m1, err := s.Runs().Create(ctx, runMeta(id(11), a, 1))
			must(t, err)
			m2, err := s.Runs().Create(ctx, runMeta(id(12), a, 2))
			must(t, err)
			m3, err := s.Runs().Create(ctx, runMeta(id(13), b, 3))
			must(t, err)
			same(t, "no", []int64{m1.No, m2.No, m3.No}, []int64{1, 2, 1})
			want := runMeta(id(12), a, 2)
			want.No = 2
			same(t, "Create 返回的 RunMeta", m2, want)
			got, err := s.Runs().Get(ctx, id(12))
			must(t, err)
			same(t, "Get", got, want)
			_, err = s.Runs().Get(ctx, id(99))
			isErr(t, "Get 不存在", err, store.ErrNotFound)
			_, err = s.Runs().Create(ctx, runMeta(id(11), a, 4))
			isErr(t, "重复 Create", err, store.ErrConflict)
		}},
		{"List 过滤、排序、翻页", func(t *testing.T, s store.Store) {
			a, b, sess := id(1), id(2), gen.Ulid(id(50))
			r2 := runMeta(id(12), a, 3)
			r2.Kind, r2.SessionID = "chat", &sess
			for _, m := range []gen.RunMeta{runMeta(id(11), a, 1), r2, runMeta(id(13), b, 2), runMeta(id(14), a, 4), runMeta(id(15), b, 4)} {
				_, err := s.Runs().Create(ctx, m)
				must(t, err)
			}
			must(t, s.Runs().SetStatus(ctx, id(14), "running", nil))
			p, err := s.Runs().List(ctx, store.RunQuery{Limit: 2})
			must(t, err)
			same(t, "第一页", runIDs(p.Items), []string{id(15), id(14)})
			same(t, "逐页读出全部", listRuns(t, s, store.RunQuery{Limit: 2}), []string{id(15), id(14), id(12), id(13), id(11)})
			same(t, "按图", listRuns(t, s, store.RunQuery{GraphID: a}), []string{id(14), id(12), id(11)})
			same(t, "按会话和种类", listRuns(t, s, store.RunQuery{SessionID: string(sess), Kind: "chat"}), []string{id(12)})
			same(t, "按种类", listRuns(t, s, store.RunQuery{Kind: "run", GraphID: b}), []string{id(15), id(13)})
			same(t, "按状态", listRuns(t, s, store.RunQuery{Status: "running"}), []string{id(14)})
		}},
		{"SetStatus、Unfinished", func(t *testing.T, s store.Store) {
			for i := 1; i <= 5; i++ {
				newRun(t, s, 10+i, i)
			}
			must(t, s.Runs().SetStatus(ctx, id(11), "running", nil))
			must(t, s.Runs().SetStatus(ctx, id(12), "paused", nil))
			end := at(30)
			must(t, s.Runs().SetStatus(ctx, id(13), "done", &end))
			must(t, s.Runs().SetStatus(ctx, id(15), "failed", &end))
			u, err := s.Runs().Unfinished(ctx)
			must(t, err)
			same(t, "Unfinished", runIDs(u), []string{id(11), id(12), id(14)})
			m, err := s.Runs().Get(ctx, id(13))
			must(t, err)
			if m.Status != "done" || m.EndedAt == nil || !m.EndedAt.Equal(end) {
				t.Errorf("结束的运行不对：%+v", m)
			}
			isErr(t, "SetStatus 不存在", s.Runs().SetStatus(ctx, id(99), "running", nil), store.ErrNotFound)
		}},
		{"AddUsage、SetProgress", func(t *testing.T, s store.Store) {
			rid := newRun(t, s, 11, 1)
			must(t, s.Runs().AddUsage(ctx, rid, store.Usage{In: 1, Out: 2, Cached: 3, CostMicros: 100}))
			must(t, s.Runs().AddUsage(ctx, rid, store.Usage{In: 1, Out: 2, Cached: 3, CostMicros: 100}))
			label := "写作"
			h := &gen.RunMetaHeadline{Kind: "running_at", NodeLabel: &label}
			must(t, s.Runs().SetProgress(ctx, rid, h, gen.RunSteps{Done: 1, Total: 5}))
			m, err := s.Runs().Get(ctx, rid)
			must(t, err)
			same(t, "用量", []any{m.Usage, m.CostMicros}, []any{gen.RunMetaUsage{In: 2, Out: 4, Cached: 6}, 200})
			same(t, "进度", []any{m.Headline, m.Steps}, []any{h, gen.RunSteps{Done: 1, Total: 5}})
			must(t, s.Runs().SetProgress(ctx, rid, nil, gen.RunSteps{Done: 5, Total: 5}))
			m, err = s.Runs().Get(ctx, rid)
			must(t, err)
			if m.Headline != nil || m.Steps.Done != 5 {
				t.Errorf("headline 为 nil 时应当清掉：%+v", m)
			}
			isErr(t, "AddUsage 不存在", s.Runs().AddUsage(ctx, id(99), store.Usage{In: 1}), store.ErrNotFound)
			isErr(t, "SetProgress 不存在", s.Runs().SetProgress(ctx, id(99), nil, gen.RunSteps{}), store.ErrNotFound)
		}},
		{"租约", func(t *testing.T, s store.Store) {
			rid := newRun(t, s, 11, 1)
			acquire := func(owner string, now, until int) bool {
				t.Helper()
				ok, err := s.Runs().AcquireLease(ctx, rid, owner, at(now), at(until))
				must(t, err)
				return ok
			}
			same(t, "没有持有者", acquire("a", 0, 10), true)
			same(t, "别人持有、没过期", acquire("b", 5, 15), false)
			same(t, "自己持有", acquire("a", 6, 20), true)
			same(t, "到期时间正好等于 now", acquire("b", 20, 30), true)
			isErr(t, "不是持有者续租", s.Runs().RenewLease(ctx, rid, "a", at(40)), store.ErrLeaseHeld)
			must(t, s.Runs().RenewLease(ctx, rid, "b", at(40)))
			same(t, "续租后别人拿不到", acquire("a", 35, 50), false)
			_, err := s.Runs().AcquireLease(ctx, id(99), "a", at(0), at(1))
			isErr(t, "AcquireLease 不存在", err, store.ErrNotFound)
			isErr(t, "RenewLease 不存在", s.Runs().RenewLease(ctx, id(99), "a", at(1)), store.ErrNotFound)
		}},
		{"节点状态和视图", func(t *testing.T, s store.Store) {
			rid := newRun(t, s, 11, 1)
			must(t, s.Runs().PutNodeStates(ctx, rid, []gen.NodeStateRow{nodeRow("b", "running", 1), nodeRow("a", "running", 2)}))
			a2 := nodeRow("a", "done", 5)
			a2.EndedAt = tsp(9)
			must(t, s.Runs().PutNodeStates(ctx, rid, []gen.NodeStateRow{a2}))
			rows, err := s.Runs().NodeStates(ctx, rid)
			must(t, err)
			same(t, "节点状态", rows, []gen.NodeStateRow{a2, nodeRow("b", "running", 1)})
			isErr(t, "PutNodeStates 运行不存在", s.Runs().PutNodeStates(ctx, id(99), []gen.NodeStateRow{nodeRow("a", "running", 1)}), store.ErrNotFound)
			isErr(t, "PutView 运行不存在", s.Runs().PutView(ctx, id(99), 1, "ws://views/1"), store.ErrNotFound)
			must(t, s.Runs().PutView(ctx, rid, 10, "ws://views/10"))
			must(t, s.Runs().PutView(ctx, rid, 20, "ws://views/20"))
			for _, c := range []struct {
				seq, want int64
				ok        bool
			}{{15, 10, true}, {20, 20, true}, {99, 20, true}, {5, 0, false}} {
				v, ref, ok, err := s.Runs().ViewBefore(ctx, rid, c.seq)
				must(t, err)
				if ok != c.ok || (ok && (v != c.want || ref != "ws://views/"+strconv.FormatInt(c.want, 10))) {
					t.Errorf("ViewBefore(%d) 得到 %d %q %v，应当 %d %v", c.seq, v, ref, ok, c.want, c.ok)
				}
			}
		}},
	}
}
