package messages

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/mio-cc/anywhere-door/engine/llm"
	"github.com/mio-cc/anywhere-door/gen"
)

// Decode 把流式响应解码成 Delta（E.10.03 §5）：按 data 里的 type 字段。思考块只给上层显示，不回传。
func (protocol) Decode(body io.Reader, _ llm.Caps) llm.Stream {
	return &decoder{sse: llm.NewSSEReader(body)}
}

type usageObj struct {
	InputTokens          int `json:"input_tokens"`
	OutputTokens         int `json:"output_tokens"`
	CacheReadInputTokens int `json:"cache_read_input_tokens"`
}

type event struct {
	Type    string `json:"type"`
	Index   int    `json:"index"`
	Message *struct {
		Model string    `json:"model"`
		Usage *usageObj `json:"usage"`
	} `json:"message"`
	ContentBlock *struct {
		Type     string `json:"type"`
		ID       string `json:"id"`
		Name     string `json:"name"`
		Text     string `json:"text"`
		Thinking string `json:"thinking"`
	} `json:"content_block"`
	Delta *struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
		Thinking    string `json:"thinking"`
		StopReason  string `json:"stop_reason"`
	} `json:"delta"`
	Usage *usageObj `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

type decoder struct {
	sse     *llm.SSEReader
	n       int
	pending []llm.Delta
	done    bool
	finish  string
	model   string
	usage   llm.Usage
}

func (d *decoder) Close() error { return nil }

func (d *decoder) Next() (llm.Delta, error) {
	for {
		if len(d.pending) > 0 {
			x := d.pending[0]
			d.pending = d.pending[1:]
			return x, nil
		}
		if d.done {
			return llm.Delta{}, io.EOF
		}
		ev, err := d.sse.Next()
		if errors.Is(err, io.EOF) {
			return llm.Delta{}, llm.Errorf(gen.FailureClassTransient, llm.CodeStreamTruncated, "流在 message_stop 之前断开（收到 %d 个事件）", d.n)
		}
		if err != nil {
			return llm.Delta{}, err
		}
		d.n++
		data := strings.TrimSpace(ev.Data)
		if data == "" {
			continue
		}
		var e event
		if err := json.Unmarshal([]byte(data), &e); err != nil {
			return llm.Delta{}, llm.Errorf(gen.FailureClassInternal, "bad_event", "第 %d 个事件不是合法的 JSON：%v", d.n, err)
		}
		if err := d.take(&e); err != nil {
			return llm.Delta{}, err
		}
	}
}

func (d *decoder) take(e *event) error {
	switch e.Type {
	case "message_start":
		if m := e.Message; m != nil {
			d.model = m.Model
			if u := m.Usage; u != nil {
				d.usage = llm.Usage{In: u.InputTokens, Cached: u.CacheReadInputTokens, Out: u.OutputTokens}
			}
		}
	case "content_block_start":
		if b := e.ContentBlock; b != nil {
			switch b.Type {
			case "tool_use":
				d.pending = append(d.pending, llm.Delta{ToolCall: &llm.ToolCallDelta{Index: e.Index, ID: b.ID, Name: b.Name}})
			case "text":
				if b.Text != "" {
					d.pending = append(d.pending, llm.Delta{Text: b.Text})
				}
			case "thinking":
				if b.Thinking != "" {
					d.pending = append(d.pending, llm.Delta{Thinking: b.Thinking})
				}
			}
		}
	case "content_block_delta":
		if x := e.Delta; x != nil {
			switch x.Type {
			case "text_delta":
				d.pending = append(d.pending, llm.Delta{Text: x.Text})
			case "input_json_delta":
				d.pending = append(d.pending, llm.Delta{ToolCall: &llm.ToolCallDelta{Index: e.Index, Args: x.PartialJSON}})
			case "thinking_delta":
				d.pending = append(d.pending, llm.Delta{Thinking: x.Thinking})
			}
		}
	case "message_delta":
		if x := e.Delta; x != nil && x.StopReason != "" {
			d.finish = finishOf(x.StopReason)
		}
		if u := e.Usage; u != nil {
			d.usage.Out = u.OutputTokens
			if u.InputTokens > 0 {
				d.usage.In = u.InputTokens
			}
			if u.CacheReadInputTokens > 0 {
				d.usage.Cached = u.CacheReadInputTokens
			}
		}
	case "message_stop":
		d.done = true
		d.pending = append(d.pending, llm.Delta{Done: true, Resp: &llm.Resp{Finish: d.finish, Usage: d.usage, Model: d.model}})
	case "error":
		code, msg := "", "上游返回 error 事件"
		if e.Error != nil {
			code, msg = e.Error.Type, e.Error.Message
		}
		return &llm.Error{Status: 200, Code: code, Message: msg}
	}
	return nil
}

func finishOf(s string) string {
	switch s {
	case "max_tokens", "model_context_window_exceeded":
		return llm.FinishLength
	case "tool_use":
		return llm.FinishToolCalls
	case "refusal":
		return llm.FinishFiltered
	}
	return llm.FinishStop
}
