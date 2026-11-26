package sched

import (
	"slices"

	"github.com/mio-cc/anywhere-door/gen"
)

// Readiness 是就绪判定 Judge 的结果（E.15.01）。
type Readiness int

// 就绪判定的结果。
const (
	NotReady Readiness = iota
	Ready
	Skip
)

func (r Readiness) String() string {
	switch r {
	case Ready:
		return "Ready"
	case Skip:
		return "Skip"
	}
	return "NotReady"
}

// InEdge 是一条入边和它上游这次执行的状态。
type InEdge struct {
	Port     string        // 本节点的输入端口
	Required bool          // 这个端口必填
	From     gen.NodeState // 上游的状态
	Active   bool          // 上游到终态时激活了这条线的源端口（分支、人工只激活一个出口）
}

// ReadyInput 是一个节点执行的就绪判定输入。
type ReadyInput struct {
	State       gen.NodeState
	Retrying    bool // 重试中的 queued
	ScopeActive bool // 所在作用域在活动中；根图总是 true
	In          []InEdge
	Early       func(arrived, total int) bool // 执行器声明的 EarlyReady；nil 表示没有
}

func settled(s gen.NodeState) bool {
	return s == gen.NodeStateDone || s == gen.NodeStateCached || s == gen.NodeStateSkipped
}

func (e InEdge) activated() bool {
	return (e.From == gen.NodeStateDone || e.From == gen.NodeStateCached) && e.Active
}

// startable：节点是 idle 或重试中的 queued，且所在作用域在活动中（§5 第 1 步）。
func startable(in ReadyInput) bool {
	idle := in.State == gen.NodeStateIdle
	retry := in.State == gen.NodeStateQueued && in.Retrying
	return (idle || retry) && in.ScopeActive
}

// Judge 判断一个节点执行能不能开始（E.15.01 §5）。empty 是没有激活连线的可选端口（按名字排序、去重）。
// 上游失败或被取消时返回 NotReady：失败的路由由运行处理（E.16.01）。
func Judge(in ReadyInput) (r Readiness, empty []string) {
	if !startable(in) {
		return NotReady, nil
	}
	if len(in.In) == 0 {
		return Ready, nil
	}
	arrived, pending := 0, false
	for _, e := range in.In {
		switch {
		case e.From == gen.NodeStateFailed || e.From == gen.NodeStateCancelled:
			return NotReady, nil
		case !settled(e.From):
			pending = true
		case e.activated():
			arrived++
		}
	}
	if pending {
		if in.Early != nil && in.Early(arrived, len(in.In)) {
			return Ready, optionalEmpty(in.In)
		}
		return NotReady, nil
	}
	if arrived == 0 {
		return Skip, nil
	}
	ports := map[string]bool{} // 端口 → 有没有激活的线
	required := map[string]bool{}
	for _, e := range in.In {
		ports[e.Port] = ports[e.Port] || e.activated()
		if e.Required {
			required[e.Port] = true
		}
	}
	for p := range required {
		if !ports[p] {
			return Skip, nil
		}
	}
	return Ready, optionalEmpty(in.In)
}

func optionalEmpty(in []InEdge) []string {
	ports := map[string]bool{}
	required := map[string]bool{}
	for _, e := range in {
		ports[e.Port] = ports[e.Port] || e.activated()
		if e.Required {
			required[e.Port] = true
		}
	}
	var out []string
	for p, ok := range ports {
		if !ok && !required[p] {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out
}
