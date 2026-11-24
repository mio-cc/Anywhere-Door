// Package fake 是脚本化的 llm.Client（设计文档 E.10.08 §4）：给执行器、编排者、对话 Agent 的单元测试用（Q.06 §3.2）。
// 按调用次序取 Step，记下收到的请求，不联网。
package fake

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/mio-cc/anywhere-door/engine/llm"
	"github.com/mio-cc/anywhere-door/gen"
)

// Step 是脚本的一步。
type Step struct {
	Deltas []llm.Delta // 按顺序吐出；最后会补一个 Done
	Resp   *llm.Resp   // 只给结果时用；和 Deltas 一起给时作为 Done 的 Resp
	Err    error       // 模拟错误（*llm.Error）：没有 Deltas 时 Complete 直接返回它；有 Deltas 时吐完再返回它
}

// Client 实现 llm.Client。
type Client struct {
	caps llm.Caps

	mu    sync.Mutex
	steps []Step
	n     int
	reqs  []llm.Req
}

// New 建一个按脚本返回的 Client；脚本用完后再调用 → 错误。
func New(caps llm.Caps, steps ...Step) *Client {
	return &Client{caps: caps, steps: steps}
}

// Caps 是建的时候给的能力。
func (c *Client) Caps() llm.Caps { return c.caps }

// Requests 是收到过的请求，供断言"模型收到了什么"。
func (c *Client) Requests() []llm.Req {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.reqs)
}

// Complete 按调用次序取下一步。
func (c *Client) Complete(_ context.Context, r llm.Req) (llm.Stream, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reqs = append(c.reqs, r)
	if c.n >= len(c.steps) {
		return nil, &llm.Error{Class: gen.FailureClassInternal, Code: "fake_exhausted",
			Message: fmt.Sprintf("fake：脚本只有 %d 步，第 %d 次调用没有对应的步骤", len(c.steps), c.n+1)}
	}
	s := c.steps[c.n]
	c.n++
	if s.Err != nil && len(s.Deltas) == 0 {
		return nil, s.Err
	}
	ds := slices.Clone(s.Deltas)
	if s.Err != nil {
		return &stream{ds: ds, err: s.Err}, nil
	}
	if len(ds) == 0 || !ds[len(ds)-1].Done {
		ds = append(ds, llm.Delta{Done: true})
	}
	last := &ds[len(ds)-1]
	if last.Resp == nil {
		if s.Resp != nil {
			cp := *s.Resp
			last.Resp = &cp
		} else {
			resp, err := build(ds)
			if err != nil {
				return nil, err
			}
			last.Resp = resp
		}
	}
	return &stream{ds: ds}, nil
}

// build 用 Deltas 拼出 Resp（文本、思考、按 Index 的工具调用）。
func build(ds []llm.Delta) (*llm.Resp, error) {
	var text, thinking strings.Builder
	type call struct {
		id, name string
		args     strings.Builder
	}
	calls := map[int]*call{}
	for _, d := range ds {
		text.WriteString(d.Text)
		thinking.WriteString(d.Thinking)
		if tc := d.ToolCall; tc != nil {
			c := calls[tc.Index]
			if c == nil {
				c = &call{}
				calls[tc.Index] = c
			}
			if c.id == "" {
				c.id = tc.ID
			}
			if c.name == "" {
				c.name = tc.Name
			}
			c.args.WriteString(tc.Args)
		}
	}
	r := &llm.Resp{Text: text.String(), Thinking: thinking.String(), Finish: llm.FinishStop, Model: "fake"}
	idx := make([]int, 0, len(calls))
	for i := range calls {
		idx = append(idx, i)
	}
	sort.Ints(idx)
	for _, i := range idx {
		c := calls[i]
		a := strings.TrimSpace(c.args.String())
		if a == "" {
			a = "{}"
		}
		var buf bytes.Buffer
		if err := json.Compact(&buf, []byte(a)); err != nil {
			return nil, fmt.Errorf("fake：脚本里工具 %s 的参数不是合法的 JSON：%w", c.name, err)
		}
		r.ToolCalls = append(r.ToolCalls, llm.ToolCall{ID: c.id, Name: c.name, Args: buf.Bytes()})
	}
	if len(r.ToolCalls) > 0 {
		r.Finish = llm.FinishToolCalls
	}
	return r, nil
}

type stream struct {
	ds  []llm.Delta
	err error
}

func (s *stream) Next() (llm.Delta, error) {
	if len(s.ds) == 0 {
		if s.err != nil {
			err := s.err
			s.err = nil
			return llm.Delta{}, err
		}
		return llm.Delta{}, io.EOF
	}
	d := s.ds[0]
	s.ds = s.ds[1:]
	return d, nil
}

func (s *stream) Close() error { return nil }
