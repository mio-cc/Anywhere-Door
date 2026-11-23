package storetest

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/mio-cc/anywhere-door/engine/store"
)

func checkpointCases() []tcase {
	cp := func(exec string, seq int64) store.Checkpoint {
		return store.Checkpoint{RunID: id(11), ExecKey: exec, Seq: seq, Executor: "llm", StateRef: "ws://cp/" + exec, TS: at(int(seq))}
	}
	return []tcase{
		{"当前检查点覆盖写", func(t *testing.T, s store.Store) {
			must(t, s.Checkpoints().Put(ctx, cp("x", 5), false))
			must(t, s.Checkpoints().Put(ctx, cp("x", 8), false))
			got, ok, err := s.Checkpoints().Latest(ctx, id(11), "x")
			must(t, err)
			same(t, "Latest", []any{ok, got}, []any{true, cp("x", 8)})
			_, ok, err = s.Checkpoints().Latest(ctx, id(11), "y")
			must(t, err)
			if ok {
				t.Error("没写过的 exec_key 不应当有检查点")
			}
		}},
		{"历史和 At", func(t *testing.T, s store.Store) {
			must(t, s.Checkpoints().Put(ctx, cp("x", 5), false))
			must(t, s.Checkpoints().Put(ctx, cp("x", 8), true))
			must(t, s.Checkpoints().Put(ctx, cp("x", 12), true))
			got, ok, err := s.Checkpoints().Latest(ctx, id(11), "x")
			must(t, err)
			same(t, "Latest", []any{ok, got}, []any{true, cp("x", 12)})
			for _, c := range []struct {
				seq, want int64
				ok        bool
			}{{10, 8, true}, {12, 12, true}, {99, 12, true}, {7, 0, false}} {
				got, ok, err := s.Checkpoints().At(ctx, id(11), "x", c.seq)
				must(t, err)
				if ok != c.ok || (ok && !jsonEq(got, cp("x", c.want))) {
					t.Errorf("At(%d) 得到 %+v %v，应当 seq %d %v（不保留历史的那次不算）", c.seq, got, ok, c.want, c.ok)
				}
			}
		}},
	}
}

func toolCallCases() []tcase {
	call := store.ToolCall{IdemKey: "k1", RunID: id(11), ExecKey: "x", Tool: "fs.write", Risk: 2, Status: store.ToolStarted,
		ArgsRef: "ws://args/1", StartedAt: at(1)}
	return []tcase{
		{"Begin 幂等", func(t *testing.T, s store.Store) {
			ex, err := s.ToolCalls().Begin(ctx, call)
			must(t, err)
			if ex != nil {
				t.Fatalf("第一次 Begin 不应当有已有记录：%+v", ex)
			}
			again := call
			again.Tool, again.StartedAt = "fs.delete", at(2)
			ex, err = s.ToolCalls().Begin(ctx, again)
			must(t, err)
			if ex == nil || !jsonEq(*ex, call) {
				t.Errorf("第二次 Begin 应当返回第一次的记录，得到 %+v", ex)
			}
			got, err := s.ToolCalls().Get(ctx, "k1")
			must(t, err)
			if !jsonEq(got, call) {
				t.Errorf("Get 得到 %+v，应当 %+v（第二次 Begin 不写）", got, call)
			}
			_, err = s.ToolCalls().Get(ctx, "k9")
			isErr(t, "Get 不存在", err, store.ErrNotFound)
		}},
		{"Finish 状态限制", func(t *testing.T, s store.Store) {
			_, err := s.ToolCalls().Begin(ctx, call)
			must(t, err)
			isErr(t, "改回 started", s.ToolCalls().Finish(ctx, "k1", store.ToolStarted, "", at(3)), store.ErrConflict)
			must(t, s.ToolCalls().Finish(ctx, "k1", store.ToolSucceeded, "ws://result/1", at(3)))
			got, err := s.ToolCalls().Get(ctx, "k1")
			must(t, err)
			if got.Status != store.ToolSucceeded || got.ResultRef != "ws://result/1" || got.EndedAt == nil || !got.EndedAt.Equal(at(3)) {
				t.Errorf("Finish 之后不对：%+v", got)
			}
			isErr(t, "再 Finish 一次", s.ToolCalls().Finish(ctx, "k1", store.ToolFailed, "", at(4)), store.ErrConflict)
			isErr(t, "Finish 不存在", s.ToolCalls().Finish(ctx, "k9", store.ToolFailed, "", at(4)), store.ErrNotFound)
		}},
	}
}

func settingsCases() []tcase {
	return []tcase{
		{"键值读写", func(t *testing.T, s store.Store) {
			_, ok, err := s.Settings().Get(ctx, "theme")
			must(t, err)
			if ok {
				t.Error("没写过的键不应当存在")
			}
			all, err := s.Settings().All(ctx)
			must(t, err)
			if len(all) != 0 {
				t.Errorf("空库的 All 应当为空：%v", all)
			}
			must(t, s.Settings().Put(ctx, "theme", json.RawMessage(`"dark"`), at(1)))
			must(t, s.Settings().Put(ctx, "budget", json.RawMessage(`{"month_micros": 5000000}`), at(1)))
			must(t, s.Settings().Put(ctx, "theme", json.RawMessage(`"light"`), at(2)))
			v, ok, err := s.Settings().Get(ctx, "theme")
			must(t, err)
			same(t, "覆盖后的值", []any{ok, v}, []any{true, json.RawMessage(`"light"`)})
			all, err = s.Settings().All(ctx)
			must(t, err)
			same(t, "All", all, map[string]json.RawMessage{"theme": json.RawMessage(`"light"`), "budget": json.RawMessage(`{"month_micros":5000000}`)})
		}},
	}
}

var errBoom = errors.New("boom")

func txCases() []tcase {
	gone := func(t *testing.T, s store.Store, what, gid, rid string) {
		t.Helper()
		_, err := s.Graphs().Meta(ctx, gid)
		isErr(t, what+"：图", err, store.ErrNotFound)
		if rid != "" {
			_, err = s.Runs().Get(ctx, rid)
			isErr(t, what+"：运行", err, store.ErrNotFound)
			evs, err := s.Events().RunRange(ctx, rid, 0, 0)
			must(t, err)
			if len(evs) != 0 {
				t.Errorf("%s：事件应当没有留下，有 %d 条", what, len(evs))
			}
		}
	}
	return []tcase{
		{"fn 返回错误", func(t *testing.T, s store.Store) {
			err := s.Tx(ctx, func(tx store.Store) error {
				gid := newGraph(t, tx, 1, 1)
				commit(t, tx, gid, "main", 1, 1, false, 2)
				rid := newRun(t, tx, 11, 1)
				_, err := tx.Events().AppendRun(ctx, rid, runEvents(rid, "", 3, 2))
				must(t, err)
				if _, err := tx.Graphs().Meta(ctx, gid); err != nil {
					t.Errorf("事务里应当读到自己的写入：%v", err)
				}
				return errBoom
			})
			if !errors.Is(err, errBoom) {
				t.Errorf("Tx 应当原样返回 fn 的错误，得到 %v", err)
			}
			gone(t, s, "回滚后", id(1), id(11))
		}},
		{"fn panic", func(t *testing.T, s store.Store) {
			err := s.Tx(ctx, func(tx store.Store) error {
				newGraph(t, tx, 1, 1)
				newRun(t, tx, 11, 1)
				panic("boom")
			})
			if err == nil {
				t.Error("fn panic 时 Tx 应当返回错误")
			}
			gone(t, s, "panic 后", id(1), id(11))
			newGraph(t, s, 2, 2)
		}},
		{"嵌套 Tx 随外层", func(t *testing.T, s store.Store) {
			err := s.Tx(ctx, func(tx store.Store) error {
				must(t, tx.Tx(ctx, func(in store.Store) error { newGraph(t, in, 1, 1); return nil }))
				return errBoom
			})
			isErr(t, "外层失败", err, errBoom)
			gone(t, s, "外层回滚后", id(1), "")
			must(t, s.Tx(ctx, func(tx store.Store) error {
				return tx.Tx(ctx, func(in store.Store) error { newGraph(t, in, 2, 1); return nil })
			}))
			if _, err := s.Graphs().Meta(ctx, id(2)); err != nil {
				t.Errorf("外层提交后内层的写入应当在：%v", err)
			}
		}},
	}
}
