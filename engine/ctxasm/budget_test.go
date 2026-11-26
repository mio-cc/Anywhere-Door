package ctxasm_test

import (
	"testing"

	"github.com/mio-cc/anywhere-door/engine/ctxasm"
	"github.com/mio-cc/anywhere-door/engine/llm"
)

func f64(x float64) *float64 { return &x }

func TestBudget(t *testing.T) {
	t.Run("E1303-T1 公式", func(t *testing.T) {
		big := llm.Caps{Window: 128000, MaxOutput: 8192}
		cases := []struct {
			name string
			in   ctxasm.BudgetIn
			want int
		}{
			{"配方比例", ctxasm.BudgetIn{Ratio: 0.1, MaxRatio: 0.25, Caps: big}, 12800},
			{"节点覆盖但不超过上限", ctxasm.BudgetIn{Ratio: 0.1, MaxRatio: 0.25, Node: f64(0.3), Caps: big}, 32000},
			{"节点在上限内", ctxasm.BudgetIn{Ratio: 0.1, MaxRatio: 0.25, Node: f64(0.05), Caps: big}, 6400},
			{"没有上限", ctxasm.BudgetIn{Ratio: 0.4, Caps: big}, 51200},
		}
		for _, c := range cases {
			out := ctxasm.Budget(c.in)
			if out.Tokens != c.want || out.Clamped || out.Window != 128000 || out.Warn != "" {
				t.Errorf("%s：%+v，应为 %d", c.name, out, c.want)
			}
		}
		// Overhead 为 0 按 1500：窗口 10000、输出 2000、比例 0.9 → 可用 6500
		out := ctxasm.Budget(ctxasm.BudgetIn{Ratio: 0.9, Caps: llm.Caps{Window: 10000, MaxOutput: 2000}})
		if out.Tokens != 6500 || !out.Clamped {
			t.Errorf("Overhead 默认 1500：%+v", out)
		}
		out = ctxasm.Budget(ctxasm.BudgetIn{Ratio: 0.9, Overhead: 500, Caps: llm.Caps{Window: 10000, MaxOutput: 2000}})
		if out.Tokens != 7500 {
			t.Errorf("Overhead 500：%+v", out)
		}
	})

	t.Run("E1303-T2 夹紧", func(t *testing.T) {
		out := ctxasm.Budget(ctxasm.BudgetIn{Ratio: 0.5, Caps: llm.Caps{Window: 8000, MaxOutput: 4000}})
		if out.Tokens != 2500 || !out.Clamped {
			t.Errorf("应夹紧到 2500：%+v", out)
		}
		out = ctxasm.Budget(ctxasm.BudgetIn{Ratio: 0.5, Overhead: 5000, Caps: llm.Caps{Window: 8000, MaxOutput: 4000}})
		if out.Tokens != 0 || !out.Clamped {
			t.Errorf("实际可用为负时为 0：%+v", out)
		}
	})

	t.Run("E1303-T3 窗口未知", func(t *testing.T) {
		out := ctxasm.Budget(ctxasm.BudgetIn{Ratio: 0.1})
		if out.Window != 8000 || out.Tokens != 800 || out.Warn == "" {
			t.Errorf("窗口未知按 8000：%+v", out)
		}
		// 预留 min(4096, 窗口/4)：窗口 8000 → 2000；比例 0.9 → 装配 7200，可用 8000-2000-1500=4500
		out = ctxasm.Budget(ctxasm.BudgetIn{Ratio: 0.9})
		if out.Tokens != 4500 || !out.Clamped {
			t.Errorf("预留 2000：%+v", out)
		}
		// 窗口 100000、MaxOutput 0 → 预留 4096；比例 1 → 可用 100000-4096-1500=94404
		out = ctxasm.Budget(ctxasm.BudgetIn{Ratio: 1, Caps: llm.Caps{Window: 100000}})
		if out.Tokens != 94404 {
			t.Errorf("预留 4096：%+v", out)
		}
	})
}
