package chat

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/mio-cc/anywhere-door/engine/llm"
	"github.com/mio-cc/anywhere-door/gen"
)

// Decode 把流式响应解码成 Delta（E.10.02 §5）：每个 data: 是一个 chunk，[DONE] 结束。
func (protocol) Decode(body io.Reader, _ llm.Caps) llm.Stream {
	return &decoder{sse: llm.NewSSEReader(body)}
}

type chunk struct {
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Content          json.RawMessage `json:"content"`
			ReasoningContent json.RawMessage `json:"reasoning_content"`
			Reasoning        json.RawMessage `json:"reasoning"`
			ToolCalls        []struct {
				Index    *int   `json:"index"`
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		PromptTokensDetails *struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
		CompletionTokensDetails *struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
	Error json.RawMessage `json:"error"`
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
			if d.finish != "" { // 有 finish_reason、没有 [DONE]：照常结束
				return d.end(), nil
			}
			return llm.Delta{}, llm.Errorf(gen.FailureClassTransient, llm.CodeStreamTruncated, "流在 [DONE] 和 finish_reason 之前断开（收到 %d 个事件）", d.n)
		}
		if err != nil {
			return llm.Delta{}, err
		}
		d.n++
		data := strings.TrimSpace(ev.Data)
		if data == "[DONE]" {
			return d.end(), nil
		}
		var ch chunk
		if err := json.Unmarshal([]byte(data), &ch); err != nil {
			return llm.Delta{}, llm.Errorf(gen.FailureClassInternal, "bad_chunk", "第 %d 个事件不是合法的 JSON：%v", d.n, err)
		}
		if len(ch.Error) > 0 && string(ch.Error) != "null" {
			code, msg := protocol{}.DecodeError(0, []byte(`{"error":`+string(ch.Error)+`}`))
			return llm.Delta{}, &llm.Error{Status: 200, Code: code, Message: msg}
		}
		d.take(&ch)
	}
}

func (d *decoder) end() llm.Delta {
	d.done = true
	return llm.Delta{Done: true, Resp: &llm.Resp{Finish: d.finish, Usage: d.usage, Model: d.model}}
}

func (d *decoder) take(ch *chunk) {
	if ch.Model != "" {
		d.model = ch.Model
	}
	if u := ch.Usage; u != nil {
		d.usage = llm.Usage{In: u.PromptTokens, Out: u.CompletionTokens}
		if u.PromptTokensDetails != nil {
			d.usage.Cached = u.PromptTokensDetails.CachedTokens
		}
		if u.CompletionTokensDetails != nil {
			d.usage.Reasoning = u.CompletionTokensDetails.ReasoningTokens
		}
	}
	for _, c := range ch.Choices {
		if c.Index != 0 {
			continue
		}
		text, thinking := content(c.Delta.Content)
		thinking += plain(c.Delta.ReasoningContent) + plain(c.Delta.Reasoning)
		if text != "" || thinking != "" {
			d.pending = append(d.pending, llm.Delta{Text: text, Thinking: thinking})
		}
		for i, tc := range c.Delta.ToolCalls {
			idx := i
			if tc.Index != nil {
				idx = *tc.Index
			}
			d.pending = append(d.pending, llm.Delta{ToolCall: &llm.ToolCallDelta{Index: idx, ID: tc.ID, Name: tc.Function.Name, Args: tc.Function.Arguments}})
		}
		if c.FinishReason != nil && *c.FinishReason != "" {
			d.finish = finishOf(*c.FinishReason)
		}
	}
}

func finishOf(s string) string {
	switch s {
	case "length", "model_length":
		return llm.FinishLength
	case "tool_calls", "function_call":
		return llm.FinishToolCalls
	case "content_filter":
		return llm.FinishFiltered
	}
	return llm.FinishStop
}

// content：delta.content 一般是字符串；有的服务给数组（{type: text, text} 和 {type: thinking, thinking: …}）。
func content(raw json.RawMessage) (text, thinking string) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s, ""
	}
	var items []struct {
		Type     string          `json:"type"`
		Text     string          `json:"text"`
		Thinking json.RawMessage `json:"thinking"`
	}
	if json.Unmarshal(raw, &items) != nil {
		return "", ""
	}
	for _, it := range items {
		switch it.Type {
		case "thinking", "reasoning":
			thinking += it.Text + plain(it.Thinking)
		default:
			text += it.Text
		}
	}
	return text, thinking
}

// plain：字符串、或 [{text}] 这样的数组，取出文字。
func plain(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var items []struct {
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &items) != nil {
		return ""
	}
	out := ""
	for _, it := range items {
		out += it.Text
	}
	return out
}
