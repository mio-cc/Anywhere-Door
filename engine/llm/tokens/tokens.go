// Package tokens 估算 token 数（设计文档 E.10.07）：给上下文预算用。M1 只有估算加校准，
// tiktoken 精确计数和校准值的持久化在 M5（P.09 K268）。
package tokens

import (
	"encoding/json"
	"math"
	"sync"
	"unicode"

	"github.com/mio-cc/anywhere-door/engine/llm"
)

// Counter 估算文本和一次请求的 token 数。
type Counter interface {
	CountText(s string) int
	Count(msgs []llm.Message, tools []llm.ToolSpec) int
}

// 估算规则（E.10.07 §5）。
const (
	cjkWeight       = 1.1  // 中日韩字符每个
	otherPerToken   = 4.0  // 其余字符每 4 个
	safety          = 1.1  // 安全系数
	messageOverhead = 4    // 每条消息另加
	attachment      = 1000 // 图片、文件各按
	toolDivisor     = 3.5  // 工具定义序列化后的长度除以它
	minRatio        = 0.7
	maxRatio        = 1.5
	ewmaKeep        = 0.8
)

func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r)
}

func raw(s string) float64 {
	var cjk, other int
	for _, r := range s {
		if isCJK(r) {
			cjk++
		} else {
			other++
		}
	}
	return float64(cjk)*cjkWeight + float64(other)/otherPerToken
}

// Approx 是未校准的估算：向上取整（中日韩字符 × 1.1 + 其余字符 ÷ 4）× 1.1；空串为 0。
func Approx(s string) int {
	if s == "" {
		return 0
	}
	return int(math.Ceil(raw(s) * safety))
}

// count 是未校准的整次请求估算。
func count(msgs []llm.Message, tools []llm.ToolSpec) int {
	n := 0
	for _, m := range msgs {
		n += messageOverhead
		for _, p := range m.Parts {
			if p.Type == llm.PartText {
				n += Approx(p.Text)
			} else {
				n += attachment
			}
		}
		for _, tc := range m.ToolCalls {
			n += Approx(tc.Name) + Approx(string(tc.Args))
		}
	}
	for _, t := range tools {
		b, _ := json.Marshal(struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Schema      json.RawMessage `json:"schema,omitempty"`
		}{t.Name, t.Description, t.Schema})
		n += int(math.Ceil(float64(len(b)) / toolDivisor))
	}
	return n
}

// Stats 记每个模型"实际 ÷ 估算"的比值（指数滑动平均，限制在 0.7–1.5），只在内存里。并发安全。
type Stats struct {
	mu    sync.Mutex
	ratio map[string]float64
}

// NewStats 建空的校准表。
func NewStats() *Stats { return &Stats{ratio: map[string]float64{}} }

// Observe 记一次实际用量；估算或实际 ≤ 0 时忽略。
func (s *Stats) Observe(model string, estimated, actual int) {
	if s == nil || estimated <= 0 || actual <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.ratio[model]
	if !ok {
		old = 1
	}
	r := old*ewmaKeep + float64(actual)/float64(estimated)*(1-ewmaKeep)
	s.ratio[model] = math.Min(maxRatio, math.Max(minRatio, r))
}

// Ratio 是模型当前的比值；没有记录时为 1。
func (s *Stats) Ratio(model string) float64 {
	if s == nil {
		return 1
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.ratio[model]; ok {
		return r
	}
	return 1
}

type counter struct {
	stats *Stats
	model string
}

// For 返回一个模型的计数器：每次计数时取最新的比值。M1 不管 tokenizer 是什么都用估算（P.09 K268）。
func For(tokenizer string, stats *Stats, model string) Counter {
	_ = tokenizer
	return counter{stats: stats, model: model}
}

func (c counter) scale(n int) int {
	if n == 0 {
		return 0
	}
	return int(math.Ceil(float64(n) * c.stats.Ratio(c.model)))
}

func (c counter) CountText(s string) int { return c.scale(Approx(s)) }

func (c counter) Count(msgs []llm.Message, tools []llm.ToolSpec) int {
	return c.scale(count(msgs, tools))
}
