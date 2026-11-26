// Package ctxasm 是上下文装配（设计文档 E.13）：决定 Agent 能看到什么。按配方从输入里取内容、变换、分配预算、
// 处理溢出、透传约束字段，产出 上下文包@1。
//
// 只依赖 gen、schema、blob 和 llm 的接口包（E.13-R4）：输入用本包的 Value，token 计数用 Counter，
// 摘要用 Summarizer，事件数据交给执行器写（P.09 K271）。
package ctxasm

import (
	"math"

	"github.com/mio-cc/anywhere-door/engine/llm"
)

// BudgetIn 是预算计算的输入（E.13.03 §4）。
type BudgetIn struct {
	Ratio    float64  // 配方 budget.ratio
	MaxRatio float64  // 配方 budget.max_ratio；0 = 不限
	Node     *float64 // 上下文装配节点的 budget 参数；nil = 用 Ratio
	Caps     llm.Caps // 下游 Agent 所用模型的能力
	Overhead int      // ①–⑤ 段和工具 schema 的估算；≤ 0 时用 1500
}

// BudgetOut 是预算计算的结果。
type BudgetOut struct {
	Tokens  int
	Clamped bool
	Window  int
	Warn    string
}

const (
	defaultWindow   = 8000
	defaultOverhead = 1500
	maxReserve      = 4096
)

// Budget 按 E.13.03 §5 的公式计算装配预算。
func Budget(in BudgetIn) BudgetOut {
	ratio := in.Ratio
	if in.Node != nil {
		ratio = *in.Node
	}
	if in.MaxRatio > 0 && ratio > in.MaxRatio {
		ratio = in.MaxRatio
	}
	out := BudgetOut{Window: in.Caps.Window}
	if out.Window <= 0 {
		out.Window = defaultWindow
		out.Warn = "模型窗口未知，按 8000 计算"
	}
	reserve := in.Caps.MaxOutput
	if reserve <= 0 {
		reserve = min(maxReserve, out.Window/4)
	}
	overhead := in.Overhead
	if overhead <= 0 {
		overhead = defaultOverhead
	}
	asm := int(math.Floor(ratio * float64(out.Window)))
	avail := out.Window - reserve - overhead
	out.Tokens = asm
	if avail < asm {
		out.Tokens = avail
		out.Clamped = true
	}
	if out.Tokens < 0 {
		out.Tokens = 0
	}
	return out
}
