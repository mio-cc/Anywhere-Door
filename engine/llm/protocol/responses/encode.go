package responses

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mio-cc/anywhere-door/engine/llm"
)

// Encode 把统一请求编码成 responses 的请求体（E.10.04 §5）。总是 store: false，没有 previous_response_id。
func (protocol) Encode(r llm.Req, upstream string, c llm.Caps) (map[string]any, error) {
	body := map[string]any{"model": upstream, "stream": true, "store": false}
	var sys []string
	input := make([]any, 0, len(r.Messages))
	for i, m := range r.Messages {
		switch m.Role {
		case llm.RoleSystem:
			sys = append(sys, joinText(m.Parts))
		case llm.RoleUser:
			content, err := userContent(m.Parts)
			if err != nil {
				return nil, fmt.Errorf("第 %d 条消息：%w", i+1, err)
			}
			input = append(input, map[string]any{"role": "user", "content": content})
		case llm.RoleAssistant:
			var out []any
			for _, p := range m.Parts {
				if p.Type == llm.PartText {
					out = append(out, map[string]any{"type": "output_text", "text": p.Text})
				}
			}
			if len(out) > 0 {
				input = append(input, map[string]any{"role": "assistant", "content": out})
			}
			for _, tc := range m.ToolCalls {
				args := string(tc.Args)
				if args == "" {
					args = "{}"
				}
				input = append(input, map[string]any{"type": "function_call", "call_id": tc.ID, "name": tc.Name, "arguments": args})
			}
		case llm.RoleTool:
			input = append(input, map[string]any{"type": "function_call_output", "call_id": m.ToolCallID, "output": joinText(m.Parts)})
		default:
			return nil, fmt.Errorf("第 %d 条消息：不认识的角色 %q", i+1, m.Role)
		}
	}
	if len(sys) > 0 {
		body["instructions"] = strings.Join(sys, "\n\n")
	}
	body["input"] = input

	tools := make([]any, 0, len(r.Tools)+1)
	for _, t := range r.Tools {
		tools = append(tools, encodeTool(t))
	}
	choice := toolChoice(r.ToolChoice)
	if o := r.Output; o != nil {
		if c.NativeSchema {
			body["text"] = map[string]any{"format": map[string]any{"type": "json_schema", "name": o.Name, "schema": schemaOf(o.Schema), "strict": o.Strict}}
		} else {
			st := llm.StructuredTool(o)
			tools = append(tools, encodeTool(st))
			choice = map[string]any{"type": "function", "name": st.Name}
		}
	}
	if len(tools) > 0 {
		body["tools"] = tools
	}
	if choice != nil {
		body["tool_choice"] = choice
	}
	if r.MaxTokens > 0 {
		body["max_output_tokens"] = r.MaxTokens
	}
	if r.Temperature != nil {
		body["temperature"] = *r.Temperature
	}
	if r.TopP != nil {
		body["top_p"] = *r.TopP
	}
	if v, _ := llm.ResolveEffort(r.Reasoning, c); v != "" {
		body["reasoning"] = map[string]any{"effort": v}
	}
	return body, nil
}

func userContent(parts []llm.Part) ([]any, error) {
	out := make([]any, 0, len(parts))
	for _, p := range parts {
		switch p.Type {
		case llm.PartText:
			out = append(out, map[string]any{"type": "input_text", "text": p.Text})
		case llm.PartImage:
			out = append(out, map[string]any{"type": "input_image", "image_url": "data:" + p.Mime + ";base64," + base64.StdEncoding.EncodeToString(p.Data)})
		default:
			return nil, fmt.Errorf("responses 协议不支持 %s 类型的内容", p.Type)
		}
	}
	return out, nil
}

func joinText(parts []llm.Part) string {
	var b strings.Builder
	for _, p := range parts {
		if p.Type == llm.PartText {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

func encodeTool(t llm.ToolSpec) map[string]any {
	m := map[string]any{"type": "function", "name": t.Name, "parameters": schemaOf(t.Schema), "strict": false}
	if t.Description != "" {
		m["description"] = t.Description
	}
	return m
}

func toolChoice(s string) any {
	switch s {
	case "":
		return nil
	case "auto", "none", "required":
		return s
	}
	return map[string]any{"type": "function", "name": s}
}

func schemaOf(raw json.RawMessage) any {
	if len(raw) == 0 {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	return raw
}
