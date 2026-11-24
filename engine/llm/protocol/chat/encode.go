package chat

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/mio-cc/anywhere-door/engine/llm"
)

// Encode 把统一请求编码成 chat_completions 的请求体（E.10.02 §5）。
func (protocol) Encode(r llm.Req, upstream string, c llm.Caps) (map[string]any, error) {
	body := map[string]any{
		"model":          upstream,
		"stream":         true,
		"stream_options": map[string]any{"include_usage": true},
	}
	msgs := make([]any, 0, len(r.Messages))
	for i, m := range r.Messages {
		om, err := encodeMessage(m)
		if err != nil {
			return nil, fmt.Errorf("第 %d 条消息：%w", i+1, err)
		}
		msgs = append(msgs, om)
	}
	body["messages"] = msgs

	tools := make([]any, 0, len(r.Tools)+1)
	for _, t := range r.Tools {
		tools = append(tools, encodeTool(t))
	}
	choice := toolChoice(r.ToolChoice)
	if o := r.Output; o != nil {
		if c.NativeSchema {
			body["response_format"] = map[string]any{
				"type":        "json_schema",
				"json_schema": map[string]any{"name": o.Name, "schema": schemaOf(o.Schema), "strict": o.Strict},
			}
		} else {
			st := llm.StructuredTool(o)
			tools = append(tools, encodeTool(st))
			choice = map[string]any{"type": "function", "function": map[string]any{"name": st.Name}}
		}
	}
	if len(tools) > 0 {
		body["tools"] = tools
	}
	if choice != nil {
		body["tool_choice"] = choice
	}
	if r.MaxTokens > 0 {
		body["max_tokens"] = r.MaxTokens
	}
	if r.Temperature != nil {
		body["temperature"] = *r.Temperature
	}
	if r.TopP != nil {
		body["top_p"] = *r.TopP
	}
	if len(r.Stop) > 0 {
		body["stop"] = r.Stop
	}
	if v, _ := llm.ResolveEffort(r.Reasoning, c); v != "" {
		body["reasoning_effort"] = v
	}
	return body, nil
}

func encodeMessage(m llm.Message) (map[string]any, error) {
	switch m.Role {
	case llm.RoleSystem, llm.RoleUser, llm.RoleAssistant, llm.RoleTool:
	default:
		return nil, fmt.Errorf("不认识的角色 %q", m.Role)
	}
	om := map[string]any{"role": string(m.Role)}
	if m.Role == llm.RoleTool {
		om["tool_call_id"] = m.ToolCallID
		om["content"] = joinText(m.Parts)
		return om, nil
	}
	content, err := encodeContent(m.Parts)
	if err != nil {
		return nil, err
	}
	if m.Role == llm.RoleAssistant && len(m.ToolCalls) > 0 {
		calls := make([]any, 0, len(m.ToolCalls))
		for _, tc := range m.ToolCalls {
			args := string(tc.Args)
			if args == "" {
				args = "{}"
			}
			calls = append(calls, map[string]any{"id": tc.ID, "type": "function", "function": map[string]any{"name": tc.Name, "arguments": args}})
		}
		om["tool_calls"] = calls
		if len(m.Parts) > 0 {
			om["content"] = content
		}
		return om, nil
	}
	om["content"] = content
	return om, nil
}

// encodeContent：只有一段文本时写字符串，否则写数组（E.10.02 §5）。
func encodeContent(parts []llm.Part) (any, error) {
	if len(parts) == 0 {
		return "", nil
	}
	if len(parts) == 1 && parts[0].Type == llm.PartText {
		return parts[0].Text, nil
	}
	out := make([]any, 0, len(parts))
	for _, p := range parts {
		switch p.Type {
		case llm.PartText:
			out = append(out, map[string]any{"type": "text", "text": p.Text})
		case llm.PartImage:
			out = append(out, map[string]any{"type": "image_url", "image_url": map[string]any{"url": dataURL(p)}})
		default:
			return nil, fmt.Errorf("chat_completions 协议不支持 %s 类型的内容", p.Type)
		}
	}
	return out, nil
}

func dataURL(p llm.Part) string {
	return "data:" + p.Mime + ";base64," + base64.StdEncoding.EncodeToString(p.Data)
}

func joinText(parts []llm.Part) string {
	s := ""
	for _, p := range parts {
		if p.Type == llm.PartText {
			s += p.Text
		}
	}
	return s
}

func encodeTool(t llm.ToolSpec) map[string]any {
	fn := map[string]any{"name": t.Name, "parameters": schemaOf(t.Schema)}
	if t.Description != "" {
		fn["description"] = t.Description
	}
	return map[string]any{"type": "function", "function": fn}
}

// toolChoice：auto / none / required 原样，工具名写成 {type: function, function: {name}}；空为不写。
func toolChoice(s string) any {
	switch s {
	case "":
		return nil
	case "auto", "none", "required":
		return s
	}
	return map[string]any{"type": "function", "function": map[string]any{"name": s}}
}

func schemaOf(raw json.RawMessage) any {
	if len(raw) == 0 {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	return raw
}
