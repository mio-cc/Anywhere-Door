package responses

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/mio-cc/anywhere-door/engine/llm"
	"github.com/mio-cc/anywhere-door/gen"
)

// Decode 把流式响应解码成 Delta（E.10.04 §5）：按 data 里的 type 字段，event: 行只作参考；不认识的事件忽略。
func (protocol) Decode(body io.Reader, _ llm.Caps) llm.Stream {
	return &decoder{sse: llm.NewSSEReader(body), started: map[int]bool{}, gotArgs: map[int]bool{}}
}

type usageObj struct {
	InputTokens        int `json:"input_tokens"`
	OutputTokens       int `json:"output_tokens"`
	InputTokensDetails *struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"input_tokens_details"`
	OutputTokensDetails *struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"output_tokens_details"`
}

type event struct {
	Type        string `json:"type"`
	Delta       string `json:"delta"`
	OutputIndex int    `json:"output_index"`
	Item        *struct {
		Type      string `json:"type"`
		CallID    string `json:"call_id"`
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"item"`
	Response *struct {
		Model             string          `json:"model"`
		Usage             *usageObj       `json:"usage"`
		Error             json.RawMessage `json:"error"`
		IncompleteDetails *struct {
			Reason string `json:"reason"`
		} `json:"incomplete_details"`
	} `json:"response"`
	Code    json.RawMessage `json:"code"`
	Message string          `json:"message"`
	Error   json.RawMessage `json:"error"`
}

type decoder struct {
	sse     *llm.SSEReader
	n       int
	pending []llm.Delta
	done    bool
	model   string
	usage   llm.Usage
	started map[int]bool // 开始了的工具调用（按 output_index）
	gotArgs map[int]bool // 收到过参数片段的工具调用
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
			return llm.Delta{}, llm.Errorf(gen.FailureClassTransient, llm.CodeStreamTruncated, "流在 response.completed / response.incomplete 之前断开（收到 %d 个事件）", d.n)
		}
		if err != nil {
			return llm.Delta{}, err
		}
		d.n++
		data := strings.TrimSpace(ev.Data)
		if data == "" || data == "[DONE]" {
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
	if r := e.Response; r != nil {
		if r.Model != "" {
			d.model = r.Model
		}
		if u := r.Usage; u != nil {
			d.usage = llm.Usage{In: u.InputTokens, Out: u.OutputTokens}
			if u.InputTokensDetails != nil {
				d.usage.Cached = u.InputTokensDetails.CachedTokens
			}
			if u.OutputTokensDetails != nil {
				d.usage.Reasoning = u.OutputTokensDetails.ReasoningTokens
			}
		}
	}
	switch e.Type {
	case "response.output_text.delta":
		if e.Delta != "" {
			d.pending = append(d.pending, llm.Delta{Text: e.Delta})
		}
	case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
		if e.Delta != "" {
			d.pending = append(d.pending, llm.Delta{Thinking: e.Delta})
		}
	case "response.output_item.added":
		if it := e.Item; it != nil && it.Type == "function_call" {
			d.started[e.OutputIndex] = true
			tc := &llm.ToolCallDelta{Index: e.OutputIndex, ID: it.CallID, Name: it.Name}
			if it.Arguments != "" {
				d.gotArgs[e.OutputIndex] = true
				tc.Args = it.Arguments
			}
			d.pending = append(d.pending, llm.Delta{ToolCall: tc})
		}
	case "response.function_call_arguments.delta":
		d.gotArgs[e.OutputIndex] = true
		d.pending = append(d.pending, llm.Delta{ToolCall: &llm.ToolCallDelta{Index: e.OutputIndex, Args: e.Delta}})
	case "response.output_item.done":
		// 有的服务不发参数片段，只在结束时给完整参数
		if it := e.Item; it != nil && it.Type == "function_call" && !d.gotArgs[e.OutputIndex] {
			tc := &llm.ToolCallDelta{Index: e.OutputIndex, Args: it.Arguments}
			if !d.started[e.OutputIndex] {
				d.started[e.OutputIndex] = true
				tc.ID, tc.Name = it.CallID, it.Name
			}
			d.gotArgs[e.OutputIndex] = true
			d.pending = append(d.pending, llm.Delta{ToolCall: tc})
		}
	case "response.completed":
		finish := llm.FinishStop
		if len(d.started) > 0 {
			finish = llm.FinishToolCalls
		}
		d.end(finish)
	case "response.incomplete":
		finish := llm.FinishLength
		if r := e.Response; r != nil && r.IncompleteDetails != nil && r.IncompleteDetails.Reason == "content_filter" {
			finish = llm.FinishFiltered
		}
		d.end(finish)
	case "response.failed":
		code, msg := "", "上游返回 response.failed"
		if r := e.Response; r != nil && len(r.Error) > 0 && string(r.Error) != "null" {
			code, msg = errorObject(r.Error)
		}
		return &llm.Error{Status: 200, Code: code, Message: msg}
	case "error":
		code, msg := text(e.Code), e.Message
		if len(e.Error) > 0 && e.Error[0] == '{' {
			code, msg = errorObject(e.Error)
		}
		return &llm.Error{Status: 200, Code: code, Message: msg}
	}
	return nil
}

func (d *decoder) end(finish string) {
	d.done = true
	d.pending = append(d.pending, llm.Delta{Done: true, Resp: &llm.Resp{Finish: finish, Usage: d.usage, Model: d.model}})
}
