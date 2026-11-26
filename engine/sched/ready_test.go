package sched

import (
	"slices"
	"testing"

	"github.com/mio-cc/anywhere-door/gen"
)

func edge(port string, req bool, from gen.NodeState, active bool) InEdge {
	return InEdge{Port: port, Required: req, From: from, Active: active}
}

func idle(in ...InEdge) ReadyInput {
	return ReadyInput{State: gen.NodeStateIdle, ScopeActive: true, In: in}
}

func want(t *testing.T, name string, in ReadyInput, r Readiness, empty ...string) {
	t.Helper()
	got, e := Judge(in)
	if got != r || !slices.Equal(e, empty) {
		t.Errorf("%s：应当是 %v %v，实际 %v %v", name, r, empty, got, e)
	}
}

func TestJudge(t *testing.T) {
	done, run := gen.NodeStateDone, gen.NodeStateRunning
	t.Run("E1501-T1 菱形", func(t *testing.T) {
		want(t, "一个上游还在跑", idle(edge("a", true, done, true), edge("b", true, run, false)), NotReady)
		want(t, "两个上游都完成", idle(edge("a", true, done, true), edge("b", true, gen.NodeStateCached, true)), Ready)
		want(t, "没有入边", idle(), Ready)
		want(t, "不是 idle", ReadyInput{State: gen.NodeStateRunning, ScopeActive: true}, NotReady)
		want(t, "重试中的 queued", ReadyInput{State: gen.NodeStateQueued, Retrying: true, ScopeActive: true}, Ready)
		want(t, "作用域不活动", ReadyInput{State: gen.NodeStateIdle}, NotReady)
		want(t, "上游失败交给运行", idle(edge("a", true, gen.NodeStateFailed, false), edge("b", true, done, true)), NotReady)
	})
	t.Run("E1501-T2 跳过传播", func(t *testing.T) {
		want(t, "分支没选这个出口", idle(edge("in", true, done, false)), Skip)
		want(t, "上游 skipped", idle(edge("in", true, gen.NodeStateSkipped, false)), Skip)
		want(t, "必填未激活、可选激活", idle(edge("a", true, done, false), edge("b", false, done, true)), Skip)
	})
	t.Run("E1501-T3 可选端口", func(t *testing.T) {
		want(t, "可选端口为空", idle(edge("a", true, done, true), edge("b", false, gen.NodeStateSkipped, false)), Ready, "b")
		want(t, "可选端口两条线一条激活", idle(edge("a", true, done, true), edge("b", false, done, false), edge("b", false, done, true)), Ready)
	})
	t.Run("E1501-T4 EarlyReady", func(t *testing.T) {
		kOfN := func(arrived, _ int) bool { return arrived >= 2 }
		in := idle(edge("in", false, done, true), edge("in", false, done, true), edge("in", false, run, false))
		in.Early = kOfN
		want(t, "3 个里 2 个完成", in, Ready)
		in.In[1].From = run
		want(t, "3 个里 1 个完成", in, NotReady)
	})
	t.Run("E1501-T5 优先级", func(t *testing.T) {
		var q Queue[string]
		q.Push(Prio{Escalation: 1, Critical: -1, Round: 1, User: 1}, "普通 1")
		q.Push(Prio{Escalation: 1, Critical: -5, Round: 1, User: 1}, "关键路径")
		q.Push(Prio{Escalation: 0, Critical: 0, Round: 9, User: 2}, "升级")
		q.Push(Prio{Escalation: 1, Critical: -1, Round: 1, User: 1}, "普通 2")
		q.Push(Prio{Escalation: 1, Critical: -1, Round: 0, User: 2}, "轮次小")
		q.Push(Prio{Escalation: 1, Critical: -1, Round: 1, User: 0}, "P0")
		var got []string
		for q.Len() > 0 {
			v, _ := q.Pop()
			got = append(got, v)
		}
		if w := []string{"升级", "关键路径", "轮次小", "P0", "普通 1", "普通 2"}; !slices.Equal(got, w) {
			t.Errorf("出队顺序 %v，应当是 %v", got, w)
		}
		if _, ok := q.Pop(); ok {
			t.Error("空队列 Pop 返回 false")
		}
	})
}
