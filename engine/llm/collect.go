package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"

	"github.com/mio-cc/anywhere-door/gen"
)

// Collect 读完整个流，返回最后的 Resp，并关闭流。流在没有 Done 时结束 → transient 错误。
func Collect(s Stream) (*Resp, error) {
	defer func() { _ = s.Close() }()
	var a accumulator
	for {
		d, err := s.Next()
		if errors.Is(err, io.EOF) {
			return nil, newError(gen.FailureClassTransient, CodeStreamTruncated, "流在结束前断开（没有收到结束标志）")
		}
		if err != nil {
			return nil, err
		}
		a.add(d)
		if d.Done {
			if d.Resp != nil {
				return d.Resp, nil
			}
			return a.finish(nil)
		}
	}
}

// accumulator 拼接流里的文本、思考和工具调用（工具调用按 Index 对上）。
type accumulator struct {
	text, thinking strings.Builder
	calls          map[int]*callAcc
}

type callAcc struct {
	id, name string
	args     strings.Builder
}

func (a *accumulator) add(d Delta) {
	a.text.WriteString(d.Text)
	a.thinking.WriteString(d.Thinking)
	if tc := d.ToolCall; tc != nil {
		if a.calls == nil {
			a.calls = map[int]*callAcc{}
		}
		c := a.calls[tc.Index]
		if c == nil {
			c = &callAcc{}
			a.calls[tc.Index] = c
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

// finish 用拼好的内容补齐 r（r 为 nil 时新建）：r 里没有的文本、思考、工具调用用拼好的。
func (a *accumulator) finish(r *Resp) (*Resp, error) {
	out := &Resp{}
	if r != nil {
		*out = *r
	}
	if out.Text == "" {
		out.Text = a.text.String()
	}
	if out.Thinking == "" {
		out.Thinking = a.thinking.String()
	}
	if len(out.ToolCalls) == 0 && len(a.calls) > 0 {
		idx := make([]int, 0, len(a.calls))
		for i := range a.calls {
			idx = append(idx, i)
		}
		sort.Ints(idx)
		for _, i := range idx {
			c := a.calls[i]
			args := strings.TrimSpace(c.args.String())
			if args == "" {
				args = "{}"
			}
			var buf bytes.Buffer
			if err := json.Compact(&buf, []byte(args)); err != nil {
				return nil, newError(gen.FailureClassFormatInvalid, "bad_tool_args", "工具 "+c.name+" 的参数不是合法的 JSON："+err.Error())
			}
			out.ToolCalls = append(out.ToolCalls, ToolCall{ID: c.id, Name: c.name, Args: json.RawMessage(buf.Bytes())})
		}
	}
	if out.Finish == "" {
		out.Finish = FinishStop
		if len(out.ToolCalls) > 0 {
			out.Finish = FinishToolCalls
		}
	}
	return out, nil
}

// stream 包在协议的流外面（E.10.01 §5.1 第 4 步）：保证最后一个 Delta 是 Done=true 并带完整 Resp，
// 收集 Raw，取出结构化输出，把错误变成 *Error。
type stream struct {
	c       *client
	ctx     context.Context
	cancel  context.CancelFunc
	body    io.Closer
	tap     *tap
	inner   Stream
	out     *OutputSpec
	secrets []string

	acc      accumulator
	finished bool
	released bool
}

func (s *stream) Next() (Delta, error) {
	if s.finished {
		return Delta{}, io.EOF
	}
	d, err := s.inner.Next()
	if err != nil {
		s.finished = true
		e := s.streamError(err)
		s.release()
		return Delta{}, e
	}
	s.acc.add(d)
	if !d.Done {
		return d, nil
	}
	s.finished = true
	resp, err := s.acc.finish(d.Resp)
	if err != nil {
		s.release()
		return Delta{}, s.c.fill(err.(*Error), s.secrets)
	}
	if resp.Model == "" {
		resp.Model = s.c.upstream
	}
	if s.out != nil {
		extractStructured(resp, s.out)
	}
	resp.Raw = redactRaw(s.tap.raw(), s.secrets)
	if resp.Usage == (Usage{}) {
		s.c.r.log.Warn("上游没有返回用量，按 0 记", "provider", s.c.p.cfg.Name, "model", s.c.upstream)
	}
	s.release()
	d.Resp = resp
	return d, nil
}

func (s *stream) Close() error {
	s.release()
	return nil
}

func (s *stream) release() {
	if s.released {
		return
	}
	s.released = true
	_ = s.inner.Close()
	_ = s.body.Close()
	s.cancel()
}

func (s *stream) streamError(err error) *Error {
	if errors.Is(err, io.EOF) {
		return s.c.fill(newError(gen.FailureClassTransient, CodeStreamTruncated, "流在结束前断开（没有收到结束标志）"), s.secrets)
	}
	var le *Error
	if errors.As(err, &le) {
		cp := *le
		return s.c.fill(&cp, s.secrets)
	}
	return s.c.transportError(s.ctx, err, s.secrets)
}

// extractStructured 把结构化输出放进 Structured（E.10.01 §5.1 第 5 步）：强制工具 emit_<名字> 的参数，
// 或原生写法返回的文本。强制工具不出现在 ToolCalls 里。修复和文本解析在 E.10.06（M1）。
func extractStructured(r *Resp, o *OutputSpec) {
	name := EmitPrefix + o.Name
	for i, tc := range r.ToolCalls {
		if tc.Name != name {
			continue
		}
		r.Structured = tc.Args
		r.ToolCalls = append(r.ToolCalls[:i:i], r.ToolCalls[i+1:]...)
		if len(r.ToolCalls) == 0 {
			r.ToolCalls = nil
			if r.Finish == FinishToolCalls {
				r.Finish = FinishStop
			}
		}
		return
	}
	if t := strings.TrimSpace(r.Text); t != "" && json.Valid([]byte(t)) {
		var buf bytes.Buffer
		if json.Compact(&buf, []byte(t)) == nil {
			r.Structured = json.RawMessage(buf.Bytes())
		}
	}
}

// redactRaw 按 §5.5 的规则脱敏 Raw（INV-22）：这次请求用到的密钥原文和像密钥的串换成 [REDACTED]。
// 替换后不再是合法 JSON 时（密钥跨了 JSON 的结构字符）不保留 Raw。
func redactRaw(raw json.RawMessage, secrets []string) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	r := Redact(string(raw), secrets...)
	if !json.Valid([]byte(r)) {
		return nil
	}
	return json.RawMessage(r)
}

// maxRaw 是 Resp.Raw 收集的上限（E.10.01 §4.1）。
const maxRaw = 1 << 20

// tap 在协议读响应体的同时留一份（最多 maxRaw 字节），结束时整理成 Raw。
type tap struct {
	r   io.Reader
	buf bytes.Buffer
}

func newTap(r io.Reader) *tap { return &tap{r: r} }

func (t *tap) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	if n > 0 && t.buf.Len() < maxRaw {
		room := maxRaw - t.buf.Len()
		t.buf.Write(p[:min(n, room)])
	}
	return n, err
}

// raw 是 SSE 的 data 逐条组成的 JSON 数组：合法的 JSON 原样放，其他的（如 [DONE]）作为字符串。
func (t *tap) raw() json.RawMessage {
	items := []json.RawMessage{}
	sr := NewSSEReader(bytes.NewReader(t.buf.Bytes()))
	for {
		ev, err := sr.Next()
		if err != nil {
			break
		}
		d := strings.TrimSpace(ev.Data)
		if json.Valid([]byte(d)) {
			items = append(items, json.RawMessage(d))
			continue
		}
		b, _ := json.Marshal(d)
		items = append(items, b)
	}
	b, err := json.Marshal(items)
	if err != nil {
		return nil
	}
	return b
}
