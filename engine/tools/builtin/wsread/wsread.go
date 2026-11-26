// Package wsread 是内置工具 builtin:ws_read（设计文档 E.11.08）：按引用读工作区原文。
// 只能读本运行里出现过的引用，由运行通过 Seen 提供（第 ⑥ 片）。
package wsread

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/mio-cc/anywhere-door/engine/blob"
	"github.com/mio-cc/anywhere-door/engine/tools"
	"github.com/mio-cc/anywhere-door/gen"
)

// Seen 回答一个引用在本运行里是否出现过。
type Seen interface {
	Seen(ref blob.Ref) bool
}

const (
	maxBytes     = 20 << 20
	defaultLimit = 4000
	maxLimit     = 20000
)

var input = json.RawMessage(`{"type":"object","properties":{` +
	`"ref":{"type":"string","pattern":"^ws://sha256:[0-9a-f]{64}$","description":"工作区引用"},` +
	`"offset":{"type":"integer","minimum":0,"default":0,"description":"从第几个字开始"},` +
	`"limit":{"type":"integer","minimum":1,"maximum":20000,"default":4000,"description":"最多读多少个字"},` +
	`"query":{"type":"string","description":"只返回含这些关键词的段落（空格分开）"}},` +
	`"required":["ref"],"additionalProperties":false}`)

// New 返回 ws_read 工具。
func New(r blob.Reader, seen Seen) tools.Impl { return &impl{r: r, seen: seen} }

type impl struct {
	r    blob.Reader
	seen Seen
}

func (w *impl) Def() tools.Def {
	return tools.Def{
		ID:           "builtin:ws_read",
		Name:         "ws_read",
		Description:  "按引用读工作区原文：可以指定从第几个字开始、读多少字，或者只看含关键词的段落",
		Input:        input,
		Risk:         tools.RiskL0,
		Idempotent:   true,
		Timeout:      10 * time.Second,
		TaintsResult: true,
	}
}

type args struct {
	Ref    string `json:"ref"`
	Offset *int   `json:"offset"`
	Limit  *int   `json:"limit"`
	Query  string `json:"query"`
}

type data struct {
	Ref        string `json:"ref"`
	Offset     int    `json:"offset"`
	Limit      int    `json:"limit"`
	Total      int    `json:"total"`
	NextOffset int    `json:"next_offset"`
}

func userErr(code, msg string) *tools.Error {
	return &tools.Error{Class: gen.FailureClassUserInput, Code: code, Msg: msg}
}

// Invoke 按 E.11.08 §5 读一段原文。
func (w *impl) Invoke(ctx context.Context, c tools.Call) (tools.Raw, error) {
	var a args
	dec := json.NewDecoder(bytes.NewReader(c.Args))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&a); err != nil {
		return tools.Raw{}, userErr("bad_args", "ws_read 的参数不对：要有 ref，offset、limit 是整数")
	}
	off, lim := 0, defaultLimit
	if a.Offset != nil {
		off = *a.Offset
	}
	if a.Limit != nil {
		lim = *a.Limit
	}
	if off < 0 || lim < 1 || lim > maxLimit {
		return tools.Raw{}, userErr("bad_args", "ws_read 的 offset 不能小于 0，limit 要在 1–20000 之间")
	}
	ref, err := blob.ParseRef(a.Ref)
	if err != nil {
		return tools.Raw{}, userErr("bad_ref", "引用格式不对，应为 ws://sha256:<64 位十六进制>")
	}
	if w.seen == nil || !w.seen.Seen(ref) {
		return tools.Raw{}, userErr("ref_not_in_run", "只能读本运行里出现过的引用")
	}
	rc, _, err := w.r.Get(ctx, ref)
	if err != nil {
		return tools.Raw{}, readErr(ctx, err)
	}
	defer func() { _ = rc.Close() }()
	b, err := io.ReadAll(io.LimitReader(rc, maxBytes+1))
	if err != nil {
		return tools.Raw{}, readErr(ctx, err)
	}
	if len(b) > maxBytes {
		return tools.Raw{}, userErr("too_large", "原文超过 20 MB，ws_read 读不了")
	}
	rs := []rune(string(b))
	total := len(rs)
	if off > total {
		off = total
	}
	end := min(off+lim, total)
	text := string(rs[off:end])
	if q := strings.TrimSpace(a.Query); q != "" {
		text = filter(text, q)
	}
	next := end
	if end >= total {
		next = -1
	}
	d, err := json.Marshal(data{Ref: string(ref), Offset: off, Limit: lim, Total: total, NextOffset: next})
	if err != nil {
		return tools.Raw{}, &tools.Error{Class: gen.FailureClassInternal, Code: "encode", Msg: "ws_read 结果编码失败", Err: err}
	}
	return tools.Raw{Text: text, Data: d, Sources: []string{string(ref)}}, nil
}

func readErr(ctx context.Context, err error) *tools.Error {
	switch {
	case ctx.Err() != nil:
		return &tools.Error{Class: gen.FailureClassCancelled, Code: "cancelled", Msg: "调用已取消", Err: err}
	case errors.Is(err, blob.ErrNotFound):
		return userErr("ref_not_found", "工作区里没有这个引用的内容")
	case errors.Is(err, blob.ErrCorrupt):
		return &tools.Error{Class: gen.FailureClassInternal, Code: "corrupt", Msg: "工作区内容和引用的哈希不一致", Err: err}
	}
	return &tools.Error{Class: gen.FailureClassTransient, Code: "blob_get", Msg: "读工作区失败", Err: err}
}

// filter 只留含任一关键词的段落（空行分段，不区分大小写）。
func filter(text, query string) string {
	words := strings.Fields(strings.ToLower(query))
	var keep []string
	for _, p := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n") {
		lp := strings.ToLower(p)
		for _, w := range words {
			if strings.Contains(lp, w) {
				keep = append(keep, strings.Trim(p, "\n"))
				break
			}
		}
	}
	return strings.Join(keep, "\n\n")
}
