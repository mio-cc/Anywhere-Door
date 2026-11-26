// STUB(E.06.02): M1 授权固定放行（P.09 K255）。E.06.02 实施时整体替换；下面的名字和字段只增不改（E.05.05 §4）。

// Package authz 是授权判定（设计文档 E.06）。M1 只有固定放行的桩 Fixed；五档、授权书、审批在 M3。
package authz

import (
	"context"

	"github.com/mio-cc/anywhere-door/gen"
)

// Effect 是判定结果（C.09 §5）。
type Effect string

// 判定结果的取值。
const (
	EffectAllow   Effect = "allow"
	EffectPropose Effect = "propose"
	EffectAsk     Effect = "ask"
	EffectDeny    Effect = "deny"
)

// Target 是判定的对象。
type Target struct {
	Graph string
	Scope string
	Nodes []string
}

// Request 是一次判定请求。
type Request struct {
	Actor   gen.Actor
	Action  gen.Action
	Target  Target
	Taint   bool
	Mandate string
}

// Result 是一次判定的结果；Reasons 是命中的授权书、规则。
type Result struct {
	Effect  Effect
	Reasons []string
}

// Checker 做授权判定（INV-09）。
type Checker interface {
	Check(ctx context.Context, req Request) Result
}

// FixedReason 是 Fixed 给出的原因。
const FixedReason = "M1 授权固定放行（P.09 K255）"

// Fixed 对任何请求都放行（M1，P.09 K255 ③）。
type Fixed struct{}

// Check 一律 allow。
func (Fixed) Check(context.Context, Request) Result {
	return Result{Effect: EffectAllow, Reasons: []string{FixedReason}}
}
