package op

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/mio-cc/anywhere-door/engine/authz"
	"github.com/mio-cc/anywhere-door/engine/graph"
	"github.com/mio-cc/anywhere-door/gen"
)

// Decision 是授权钩子的结论（E.05.05 §4）。
type Decision int

// 授权钩子的结论。
const (
	Apply   Decision = iota // 直接应用
	Propose                 // 整批转为提议
	Deny                    // 拒绝
)

// Estop 报告急停（E.05.01 §4；E.06.04 实现，M3）。为 nil 时视为没有急停。
type Estop interface {
	Active(runID string) bool // 全局或这个运行在急停中
}

// 授权拒绝的三种错误（E.05.01 §7），由 *DeniedError 包着。
var (
	ErrForbidden  = errors.New("op: 授权拒绝")
	ErrNodeLocked = errors.New("op: 节点已锁定")
	ErrEstopped   = errors.New("op: 急停中，模型不能改图")
)

// DeniedError 是被授权钩子拒绝（E.05.05 §7）：Err 是上面三种之一；Reasons 是命中的授权书、规则；
// 节点锁时 Node、Lock 是锁住的那个节点和锁（C.11 §3 node_locked 的 details）。
type DeniedError struct {
	Err     error
	Reasons []string
	Node    string
	Lock    string
}

func (e *DeniedError) Error() string {
	msg := e.Err.Error()
	if len(e.Reasons) > 0 {
		msg += "：" + strings.Join(e.Reasons, "；")
	}
	return msg
}

func (e *DeniedError) Unwrap() error { return e.Err }

// authInput 是授权钩子看到的一批 Op（target 已由 E.05.03 算好）。
type authInput struct {
	Actor    gen.Actor
	Approver *gen.Actor
	Taint    bool
	Mandate  string
	RunID    string
	Ops      []gen.Op
	G        *graph.Graph // 批次开始时的图（head）
}

// ActionOf 是判定用的动作（C.09 §4.3）：gen.AuthzOpActions 里列出的按表，其余是 "op." + kind。
func ActionOf(kind gen.OpKind) gen.Action {
	if a, ok := gen.AuthzOpActions[kind]; ok && a != "per_sub_op" {
		return gen.Action(a)
	}
	return gen.Action("op." + string(kind))
}

// authorize 在 Op 落地前做授权判定（E.05.05 §5）：急停 → 只看操作者的 Op → 实例锁 → 逐条 Check → 汇总。
// Deny 时一定带 *DeniedError；Propose 时返回每条的判定结果，由流水线交给 Proposer。
func authorize(ctx context.Context, chk authz.Checker, es Estop, in authInput) (Decision, []authz.Result, error) {
	model := in.Actor.Kind == gen.ActorKindModel
	if es != nil && model && es.Active(in.RunID) {
		return Deny, nil, &DeniedError{Err: ErrEstopped, Reasons: []string{"急停中"}}
	}
	for i := range in.Ops {
		switch op := &in.Ops[i]; op.Kind {
		case gen.OpSetLock:
			if in.Actor.Kind != gen.ActorKindUser {
				return Deny, nil, &DeniedError{Err: ErrForbidden, Reasons: []string{"只有用户能设置或解除节点锁（C.03.03-R3）"}}
			}
		case gen.OpMigrate:
			if in.Actor.Kind != gen.ActorKindRuntime {
				return Deny, nil, &DeniedError{Err: ErrForbidden, Reasons: []string{"格式升级只能由内核提交（C.12-R2）"}}
			}
		}
	}
	for i := range in.Ops {
		if err := checkLocks(&in.Ops[i], in.Actor, in.G); err != nil {
			return Deny, nil, err
		}
	}
	who := in.Actor
	if in.Approver != nil {
		who = *in.Approver // 被采纳的提议按用户判定（C.08-R7）
	}
	graphID := ""
	if in.G != nil {
		graphID = in.G.ID()
	}
	var results []authz.Result
	for i := range in.Ops {
		op := &in.Ops[i]
		if op.Kind == gen.OpSetLock || op.Kind == gen.OpMigrate {
			continue
		}
		tg := targetOf(op, in.G)
		results = append(results, chk.Check(ctx, authz.Request{
			Actor:   who,
			Action:  ActionOf(op.Kind),
			Target:  authz.Target{Graph: graphID, Scope: tg.Scope, Nodes: idsOf(tg.Nodes)},
			Taint:   in.Taint || (op.Taint != nil && *op.Taint),
			Mandate: in.Mandate,
		}))
	}
	var denied []string
	allAllow := true
	for _, r := range results {
		switch r.Effect {
		case authz.EffectDeny:
			denied = append(denied, r.Reasons...)
		case authz.EffectAllow:
		default:
			allAllow = false
		}
	}
	switch {
	case denied != nil || hasDeny(results):
		return Deny, results, &DeniedError{Err: ErrForbidden, Reasons: sortedIDs(denied)}
	case allAllow:
		return Apply, results, nil
	default:
		return Propose, results, nil // 有 propose、ask 的整批转为提议（C.08-R4）
	}
}

func hasDeny(rs []authz.Result) bool {
	for _, r := range rs {
		if r.Effect == authz.EffectDeny {
			return true
		}
	}
	return false
}

// targetOf 是 Op 的作用对象：流水线已经算好时用它，否则现算（算不出来时只有 root）。
func targetOf(op *gen.Op, g *graph.Graph) gen.OpTarget {
	if op.Target != nil {
		return *op.Target
	}
	if t, err := Target(op, g); err == nil {
		return t
	}
	return gen.OpTarget{Scope: graph.RootScope}
}

func idsOf(p *[]gen.Ulid) []string {
	if p == nil {
		return nil
	}
	return strs(*p)
}

// checkLocks 查实例锁（C.03.03、E.05.05 §5 第 3 步）。作用对象 = target.nodes、target.edges 两端的节点、
// target.scope 的容器链；加东西的 Op（add_node、add_note、restore_nodes）只看容器链。每个对象沿容器往上找锁（C.03.03-R4）：
// 有 frozen → 所有人都不行（set_lock 例外，用户要能解锁）；有 user 而操作者不是用户 → 不行（move_nodes、set_node_ui 例外，
// C.03.03-R5）。migrate 不查锁。
func checkLocks(op *gen.Op, actor gen.Actor, g *graph.Graph) error {
	if g == nil || op.Kind == gen.OpMigrate {
		return nil
	}
	tg := targetOf(op, g)
	objs := map[string]bool{}
	if tg.Scope != "" && tg.Scope != graph.RootScope {
		objs[tg.Scope] = true // 容器节点本身；再往上由 lockOf 沿容器链找
	}
	switch op.Kind {
	case gen.OpAddNode, gen.OpAddNote, gen.OpRestoreNodes:
	default:
		for _, id := range idsOf(tg.Nodes) {
			objs[id] = true
		}
		for _, id := range idsOf(tg.Edges) {
			if e, ok := g.Edge(id); ok {
				objs[string(e.From.Node)] = true
				objs[string(e.To.Node)] = true
			}
		}
	}
	ids := make([]string, 0, len(objs))
	for id := range objs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	view := op.Kind == gen.OpMoveNodes || op.Kind == gen.OpSetNodeUI
	for _, id := range ids {
		node, lock := lockOf(g, id)
		switch {
		case lock == "frozen" && op.Kind != gen.OpSetLock:
		case lock == "user" && actor.Kind != gen.ActorKindUser && !view:
		default:
			continue
		}
		return &DeniedError{Err: ErrNodeLocked, Node: node, Lock: lock,
			Reasons: []string{"节点 " + node + " 已锁定（" + lock + "）"}}
	}
	return nil
}

// lockOf 是节点和它的容器链上最严的锁：frozen 优先于 user（C.03.03-R4）。没有锁时 lock 为空。节点不在图里时也为空
// （新加的节点由容器链判断）。
func lockOf(g *graph.Graph, id string) (node, lock string) {
	for cur := id; cur != ""; {
		n, ok := g.Node(cur)
		if !ok {
			break
		}
		if n.Lock != nil {
			switch *n.Lock {
			case "frozen":
				return cur, "frozen"
			case "user":
				if lock == "" {
					node, lock = cur, "user"
				}
			}
		}
		s := g.ScopeOf(cur)
		if s == nil || s.ID() == graph.RootScope {
			break
		}
		cur = s.ID()
	}
	return node, lock
}
