package storetest

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/mio-cc/anywhere-door/engine/store"
	"github.com/mio-cc/anywhere-door/gen"
)

func sessionEvent(sid string, sec int) gen.SessionEvent {
	return gen.SessionEvent{Ts: ts(sec), Kind: "session.updated", Actor: user, SessionID: gen.Ulid(sid),
		Data: json.RawMessage(`{"n":` + strconv.Itoa(sec) + `}`)}
}

func eventCases() []tcase {
	return []tcase{
		{"AppendRun 填 seq、更新 last_seq", func(t *testing.T, s store.Store) {
			r1, r2 := newRun(t, s, 11, 1), newRun(t, s, 12, 2)
			evs := runEvents(r1, id(7), 3, 10)
			last, err := s.Events().AppendRun(ctx, r1, evs)
			must(t, err)
			same(t, "第一批", []any{last, seqsOf(evs)}, []any{3, []int64{1, 2, 3}})
			evs = runEvents(r1, id(7), 2, 20)
			last, err = s.Events().AppendRun(ctx, r1, evs)
			must(t, err)
			same(t, "第二批接着编号", []any{last, seqsOf(evs)}, []any{5, []int64{4, 5}})
			m, err := s.Runs().Get(ctx, r1)
			must(t, err)
			if m.LastSeq != 5 {
				t.Errorf("RunMeta.last_seq 是 %d，应当 5", m.LastSeq)
			}
			evs = runEvents(r2, "", 2, 30)
			last, err = s.Events().AppendRun(ctx, r2, evs)
			must(t, err)
			same(t, "另一个运行从 1 起", []any{last, seqsOf(evs)}, []any{2, []int64{1, 2}})
			_, err = s.Events().AppendRun(ctx, id(99), runEvents(id(99), "", 1, 40))
			isErr(t, "运行不存在", err, store.ErrNotFound)
		}},
		{"RunRange、RunByNode、RunUntil", func(t *testing.T, s store.Store) {
			rid := newRun(t, s, 11, 1)
			na, nb := id(7), id(8)
			var all []gen.RunEvent
			for i := 0; i < 10; i++ {
				node := nb
				if i%2 == 0 {
					node = na
				}
				all = append(all, runEvent(rid, node, 10+i))
			}
			_, err := s.Events().AppendRun(ctx, rid, all)
			must(t, err)
			got, err := s.Events().RunRange(ctx, rid, 3, 4)
			must(t, err)
			same(t, "RunRange(3, 4 条)", got, all[3:7])
			got, err = s.Events().RunRange(ctx, rid, 8, 0)
			must(t, err)
			same(t, "RunRange 到末尾", seqsOf(got), []int64{9, 10})
			got, err = s.Events().RunByNode(ctx, rid, na, 4)
			must(t, err)
			same(t, "RunByNode", seqsOf(got), []int64{5, 7, 9})
			got, err = s.Events().RunUntil(ctx, rid, 3)
			must(t, err)
			same(t, "RunUntil", got, all[:3])
			got, err = s.Events().RunRange(ctx, rid, 10, 0)
			must(t, err)
			if len(got) != 0 {
				t.Errorf("最后一条之后应当没有事件，得到 %d 条", len(got))
			}
		}},
		{"会话事件", func(t *testing.T, s store.Store) {
			s1, s2 := id(51), id(52)
			a := []gen.SessionEvent{sessionEvent(s1, 1), sessionEvent(s1, 2)}
			last, err := s.Events().AppendSession(ctx, s1, a)
			must(t, err)
			same(t, "第一批", []any{last, a[0].Seq, a[1].Seq}, []any{2, 1, 2})
			last, err = s.Events().AppendSession(ctx, s2, []gen.SessionEvent{sessionEvent(s2, 3)})
			must(t, err)
			same(t, "另一个会话从 1 起", last, 1)
			b := []gen.SessionEvent{sessionEvent(s1, 4)}
			last, err = s.Events().AppendSession(ctx, s1, b)
			must(t, err)
			same(t, "接着编号", last, 3)
			got, err := s.Events().SessionRange(ctx, s1, 1, 0)
			must(t, err)
			same(t, "SessionRange", got, []gen.SessionEvent{a[1], b[0]})
			got, err = s.Events().SessionRange(ctx, s1, 0, 1)
			must(t, err)
			same(t, "SessionRange limit 1", got, a[:1])
		}},
	}
}
