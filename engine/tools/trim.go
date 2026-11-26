package tools

import (
	"context"
	"errors"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/mio-cc/anywhere-door/engine/blob"
	"github.com/mio-cc/anywhere-door/gen"
)

// TrimMode 是裁剪方式（工具节点的 trim 参数，E.14.04）。
type TrimMode string

// 裁剪方式的取值。
const (
	TrimRelevant TrimMode = "relevant"
	TrimHead     TrimMode = "head"
	TrimNone     TrimMode = "none"
)

// TrimOpts 是裁剪选项；MaxTokens 为 0 时用 2000。
type TrimOpts struct {
	Mode      TrimMode
	MaxTokens int
	Query     string
}

// Counter 计 token；tokens.Counter（E.10.07）满足它，由组装处传入。
type Counter interface {
	CountText(s string) int
}

const (
	defaultMaxTokens = 2000
	chunkTokens      = 500
	summaryRunes     = 300
	inlineLimit      = 8 << 10
	defaultMediaType = "text/plain; charset=utf-8"
)

// Trim 把原始结果裁剪成摘要和片段，原文写进工作区（E.11.02 §5）。
func Trim(ctx context.Context, raw Raw, o TrimOpts, blobs blob.Store, cnt Counter) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, &Error{Class: gen.FailureClassCancelled, Code: "cancelled", Msg: "调用已取消", Err: err}
	}
	mode := o.Mode
	if mode == "" {
		mode = TrimRelevant
	}
	if mode != TrimRelevant && mode != TrimHead && mode != TrimNone {
		return Result{}, Errorf(gen.FailureClassUserInput, "bad_trim", "不认识的裁剪方式 %q", string(mode))
	}
	limit := o.MaxTokens
	if limit <= 0 {
		limit = defaultMaxTokens
	}
	text := raw.Text
	if text == "" && len(raw.Data) > 0 {
		text = string(raw.Data)
	}
	mt := raw.MediaType
	if mt == "" {
		mt = defaultMediaType
	}
	ref, err := blobs.Put(ctx, strings.NewReader(text), mt)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return Result{}, &Error{Class: gen.FailureClassCancelled, Code: "cancelled", Msg: "调用已取消", Err: err}
		}
		return Result{}, &Error{Class: gen.FailureClassTransient, Code: "blob_put", Msg: "工具结果写入工作区失败", Err: err}
	}
	res := Result{Ref: ref, Bytes: len(text), Summary: summary(text)}
	if len(raw.Data) > 0 && len(raw.Data) <= inlineLimit {
		res.Data = raw.Data
	}
	switch mode {
	case TrimNone:
		if text != "" && len(text) <= inlineLimit {
			res.Snippets = []Snippet{{Text: text}}
		}
	case TrimHead:
		res.Snippets = pick(chunks(text, chunkTokens, cnt), nil, limit, cnt)
	default:
		cs := chunks(text, chunkTokens, cnt)
		var scores []float64
		if strings.TrimSpace(o.Query) != "" {
			scores = bm25(o.Query, cs)
		}
		if !anyPositive(scores) {
			scores = nil
		}
		res.Snippets = pick(cs, scores, limit, cnt)
	}
	res.Tokens = cnt.CountText(res.Summary)
	for _, s := range res.Snippets {
		res.Tokens += cnt.CountText(s.Text)
	}
	return res, nil
}

// summary 取原文开头 300 个字。
func summary(text string) string {
	s := strings.TrimSpace(text)
	if utf8.RuneCountInString(s) <= summaryRunes {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:summaryRunes]))
}

func anyPositive(xs []float64) bool {
	for _, x := range xs {
		if x > 0 {
			return true
		}
	}
	return false
}

// pick 按顺序（有分数时按分数从高到低，并列按位置）取块，直到加上下一块会超过 limit；至少取一块。
// 取到的块按在原文里的位置输出。
func pick(cs []chunk, scores []float64, limit int, cnt Counter) []Snippet {
	if len(cs) == 0 {
		return nil
	}
	order := make([]int, len(cs))
	for i := range order {
		order[i] = i
	}
	if scores != nil {
		sort.SliceStable(order, func(a, b int) bool { return scores[order[a]] > scores[order[b]] })
	}
	var chosen []int
	used := 0
	for _, i := range order {
		c := cs[i]
		if used+c.tokens > limit {
			if len(chosen) == 0 {
				c.text = cut(c.text, limit, cnt)
				c.tokens = cnt.CountText(c.text)
				cs[i] = c
				chosen = append(chosen, i)
			}
			break
		}
		chosen = append(chosen, i)
		used += c.tokens
	}
	sort.Ints(chosen)
	out := make([]Snippet, 0, len(chosen))
	for _, i := range chosen {
		s := Snippet{Text: cs[i].text, Offset: cs[i].offset}
		if scores != nil {
			s.Score = scores[i]
		}
		out = append(out, s)
	}
	return out
}

// cut 取 s 的最长前缀，使它的 token 数不超过 limit（按 rune 二分）。
func cut(s string, limit int, cnt Counter) string {
	if limit <= 0 || s == "" {
		return ""
	}
	if cnt.CountText(s) <= limit {
		return s
	}
	rs := []rune(s)
	// 先找一个超出的上界，避免对很长的文本整段二分
	hi := limit * 8
	for hi < len(rs) && cnt.CountText(string(rs[:hi])) <= limit {
		hi *= 2
	}
	if hi > len(rs) {
		hi = len(rs)
	}
	lo := 0 // rs[:lo] 不超出
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
