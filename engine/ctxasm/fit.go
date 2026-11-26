package ctxasm

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/mio-cc/anywhere-door/engine/blob"
	"github.com/mio-cc/anywhere-door/gen"
)

// Part 是装配中的一段（E.13.04 §4）。
type Part struct {
	Name, Text, Source string
	Ref                blob.Ref // 原文引用；收缩时为空就把原文写进工作区
	Tokens, Priority   int
	Budget             float64 // 本段占总预算的比例；0 = 不单独限制
	Required, Locked   bool
	Summary            bool // transform: summary
	Pointer            bool // 已经是指针形式
	Taint              bool
	TaintSrc           []string
	Order              int // 在包里的位置
}

// Dropped 记一次收缩：Mode 是 pointer | summarize | truncate。
type Dropped struct {
	Section string   `json:"section"`
	Ref     blob.Ref `json:"ref,omitempty"`
	Mode    string   `json:"mode"`
	Saved   int      `json:"saved"`
}

// Summarizer 做摘要；M1 由上下文装配执行器（E.14.03）用内部 Agent 执行实现（P.09 K32、K271）。
type Summarizer interface {
	Summarize(ctx context.Context, text string, keep []string, maxTokens int) (string, error)
}

// FitEnv 是溢出处理需要的环境。
type FitEnv struct {
	Blobs      blob.Store
	Counter    Counter
	Summarizer Summarizer
}

// 溢出处理方式（C.13.01 on_overflow）。
const (
	OverflowPointer   = "pointer"
	OverflowSummarize = "summarize"
	OverflowTruncate  = "truncate"
	OverflowReject    = "reject"
)

// ErrContextOverflow：必需内容放不下。返回时包成 *Error{user_input, "context_overflow"}，可以用 errors.Is 判断。
var ErrContextOverflow = errors.New("上下文放不下必需内容，请提高预算或换更大窗口的模型")

const (
	minSummaryTokens = 200
	minSummarizeGoal = 50
	pointerRunes     = 80
)

func overflowErr() *Error {
	return &Error{Class: gen.FailureClassUserInput, Code: "context_overflow", Msg: ErrContextOverflow.Error(), Err: ErrContextOverflow}
}

type runeCounter struct{}

func (runeCounter) CountText(s string) int { return utf8.RuneCountInString(s) }

// Fit 按 E.13.04 §5 让各段放进 total 个 token。
func Fit(ctx context.Context, parts []Part, total int, mode string, env FitEnv) ([]Part, []Dropped, error) {
	if mode == "" {
		mode = OverflowPointer
	}
	switch mode {
	case OverflowPointer, OverflowSummarize, OverflowTruncate, OverflowReject:
	default:
		return nil, nil, userErrf("bad_overflow", "不认识的溢出处理方式 %q", mode)
	}
	f := &fitter{ctx: ctx, env: env, mode: mode}
	if f.env.Counter == nil {
		f.env.Counter = runeCounter{}
	}
	ps := append([]Part(nil), parts...)
	if total < 0 {
		total = 0
	}
	// 1. transform: summary 的段先摘要
	for i := range ps {
		if ps[i].Summary && !ps[i].Locked {
			goal := max(segCap(ps[i], total), minSummaryTokens)
			if err := f.summarize(&ps[i], goal); err != nil {
				return nil, nil, err
			}
		}
	}
	// 2. 每段受自己的预算上限约束
	for i := range ps {
		p := &ps[i]
		if !shrinkable(*p) || p.Budget <= 0 {
			continue
		}
		if c := segCap(*p, total); p.Tokens > c {
			if mode == OverflowReject {
				return nil, nil, overflowErr()
			}
			if err := f.shrink(p, c); err != nil {
				return nil, nil, err
			}
		}
	}
	// 3. 总量仍超出：按优先级从低到高（数字大的先；同优先级位置靠后的先）收缩
	if sum(ps) > total {
		if mode == OverflowReject {
			return nil, nil, overflowErr()
		}
		order := make([]int, 0, len(ps))
		for i := range ps {
			if shrinkable(ps[i]) {
				order = append(order, i)
			}
		}
		sort.SliceStable(order, func(a, b int) bool {
			pa, pb := ps[order[a]], ps[order[b]]
			if pa.Priority != pb.Priority {
				return pa.Priority > pb.Priority
			}
			return pa.Order > pb.Order
		})
		for _, i := range order {
			over := sum(ps) - total
			if over <= 0 {
				break
			}
			if err := f.shrink(&ps[i], max(0, ps[i].Tokens-over)); err != nil {
				return nil, nil, err
			}
		}
		if sum(ps) > total {
			return nil, nil, overflowErr()
		}
	}
	sort.SliceStable(ps, func(a, b int) bool { return ps[a].Order < ps[b].Order })
	return ps, f.dropped, nil
}

func shrinkable(p Part) bool { return !p.Required && !p.Locked }

func segCap(p Part, total int) int {
	if p.Budget <= 0 {
		return total
	}
	return int(p.Budget * float64(total))
}

func sum(ps []Part) int {
	n := 0
	for _, p := range ps {
		n += p.Tokens
	}
	return n
}

type fitter struct {
	ctx     context.Context
	env     FitEnv
	mode    string
	dropped []Dropped
}

func (f *fitter) count(s string) int { return f.env.Counter.CountText(s) }

// ensureRef 把原文写进工作区（还没有引用时）。
func (f *fitter) ensureRef(p *Part) error {
	if p.Ref != "" {
		return nil
	}
	if f.env.Blobs == nil {
		return &Error{Class: gen.FailureClassInternal, Code: "no_blobs", Msg: "溢出处理没有工作区"}
	}
	ref, err := f.env.Blobs.Put(f.ctx, strings.NewReader(p.Text), "text/plain; charset=utf-8")
	if err != nil {
		return &Error{Class: gen.FailureClassTransient, Code: "blob_put", Msg: "段「" + p.Name + "」的原文写入工作区失败", Err: err}
	}
	p.Ref = ref
	return nil
}

func (f *fitter) record(p *Part, mode string, before int) {
	f.dropped = append(f.dropped, Dropped{Section: p.Name, Ref: p.Ref, Mode: mode, Saved: max(0, before-p.Tokens)})
}

// shrink 按模式把一段收缩到 goal 个 token 以内（做不到时尽量小）。
func (f *fitter) shrink(p *Part, goal int) error {
	if p.Tokens <= goal || p.Pointer {
		return nil
	}
	switch f.mode {
	case OverflowSummarize:
		if goal >= minSummarizeGoal {
			return f.summarize(p, goal)
		}
		return f.pointer(p)
	case OverflowTruncate:
		return f.truncate(p, goal)
	}
	return f.pointer(p)
}

func (f *fitter) pointer(p *Part) error {
	if err := f.ensureRef(p); err != nil {
		return err
	}
	before := p.Tokens
	txt := PointerText(p.Name, p.Text, p.Ref)
	n := f.count(txt)
	if n >= before { // 指针形式不比原文短：不动
		return nil
	}
	p.Text, p.Tokens, p.Pointer = txt, n, true
	f.record(p, OverflowPointer, before)
	return nil
}

func (f *fitter) summarize(p *Part, goal int) error {
	if f.env.Summarizer == nil {
		return &Error{Class: gen.FailureClassInternal, Code: "no_summarizer", Msg: "段「" + p.Name + "」要摘要，但没有摘要执行器"}
	}
	if err := f.ensureRef(p); err != nil {
		return err
	}
	before := p.Tokens
	s, err := f.env.Summarizer.Summarize(f.ctx, p.Text, nil, goal)
	if err != nil {
		return err
	}
	p.Text = s
	p.Tokens = f.count(s)
	f.record(p, OverflowSummarize, before)
	return nil
}

func (f *fitter) truncate(p *Part, goal int) error {
	if err := f.ensureRef(p); err != nil {
		return err
	}
	before := p.Tokens
	suffix := "…（已截断，原文: " + string(p.Ref) + "）"
	keep := goal - f.count(suffix)
	head := ""
	if keep > 0 {
		head = cutTokens(p.Text, keep, f.env.Counter)
	}
	if n := f.count(head + suffix); n < before {
		p.Text, p.Tokens = head+suffix, n
		f.record(p, OverflowTruncate, before)
	}
	return nil
}

// cutTokens 取 s 的最长前缀，使 token 数不超过 limit（按 rune 二分）。
func cutTokens(s string, limit int, cnt Counter) string {
	if limit <= 0 {
		return ""
	}
	if cnt.CountText(s) <= limit {
		return s
	}
	rs := []rune(s)
	lo, hi := 0, len(rs)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if cnt.CountText(string(rs[:mid])) <= limit {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return string(rs[:lo])
}

// PointerText 是指针形式："【段名】一句摘要 [原文: ws://…]"。一句摘要取到第一个句号、问号、感叹号或换行，最多 80 字。
func PointerText(name, text string, ref blob.Ref) string {
	s := strings.TrimSpace(text)
	var b strings.Builder
	n := 0
	for _, r := range s {
		if r == '\n' || n >= pointerRunes {
			break
		}
		b.WriteRune(r)
		n++
		if strings.ContainsRune("。？！?!", r) {
			break
		}
	}
	first := strings.TrimSpace(b.String())
	if first == "" {
		return "【" + name + "】[原文: " + string(ref) + "]"
	}
	return "【" + name + "】" + first + " [原文: " + string(ref) + "]"
}

func userErrf(code, format string, args ...any) *Error {
	return &Error{Class: gen.FailureClassUserInput, Code: code, Msg: fmt.Sprintf(format, args...)}
}
