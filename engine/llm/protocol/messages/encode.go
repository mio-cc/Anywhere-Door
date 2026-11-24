package messages

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mio-cc/anywhere-door/engine/llm"
)

// defaultMaxTokens 是请求和能力表都没有给出输出上限时用的值（这个协议要求 max_tokens）。
const defaultMaxTokens = 4096

type turn struct {
	role   string
	blocks []any
}

// Encode 把统一请求编码成 anthropic_messages 的请求体（E.10.03 §5）。
// system 放顶层；相邻的同角色消息合并（这个协议要求 user / assistant 交替）；工具结果放进 user 消息。
func (protocol) Encode(r llm.Req, upstream string, c llm.Caps) (map[string]any, error) {
	maxTokens := r.MaxTokens
	if maxTokens <= 0 {
		maxTokens = c.MaxOutput
	}
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	body := map[string]any{"model": upstream, "stream": true, "max_tokens": maxTokens}

	var sys []string
	sysCache := false
	var turns []turn
	add := func(role string, blocks []any) {
		if len(blocks) == 0 {
			return
		}
		if n := len(turns); n > 0 && turns[n-1].role == role {
			turns[n-1].blocks = append(turns[n-1].blocks, blocks...)
			return
		}
		turns = append(turns, turn{role: role, blocks: blocks})
	}
	for i, m := range r.Messages {
		var role string
		var blocks []any
		switch m.Role {
		case llm.RoleSystem:
			sys = append(sys, joinText(m.Parts))
			sysCache = sysCache || m.Cache
			continue
		case llm.RoleUser:
			role = "user"
			for _, p := range m.Parts {
				b, err := partBlock(p)
				if err != nil {
					return nil, fmt.Errorf("第 %d 条消息：%w", i+1, err)
				}
				blocks = append(blocks, b)
			}
		case llm.RoleAssistant:
			role = "assistant"
			for _, p := range m.Parts {
				if p.Type == llm.PartText && p.Text != "" {
					blocks = append(blocks, map[string]any{"type": "text", "text": p.Text})
				}
			}
			for _, tc := range m.ToolCalls {
				var input any = map[string]any{}
				if len(tc.Args) > 0 {
					input = tc.Args
				}
				blocks = append(blocks, map[string]any{"type": "tool_use", "id": tc.ID, "name": tc.Name, "input": input})
			}
		case llm.RoleTool:
			role = "user"
			blocks = []any{map[string]any{"type": "tool_result", "tool_use_id": m.ToolCallID, "content": joinText(m.Parts)}}
		default:
			return nil, fmt.Errorf("第 %d 条消息：不认识的角色 %q", i+1, m.Role)
		}
		if m.Cache && c.PromptCache && len(blocks) > 0 {
			if last, ok := blocks[len(blocks)-1].(map[string]any); ok {
				last["cache_control"] = map[string]any{"type": "ephemeral"}
			}
		}
		add(role, blocks)
	}
	if len(sys) > 0 {
		s := strings.Join(sys, "\n\n")
		if sysCache && c.PromptCache {
			body["system"] = []any{map[string]any{"type": "text", "text": s, "cache_control": map[string]any{"type": "ephemeral"}}}
		} else {
			body["system"] = s
		}
	}
	msgs := make([]any, 0, len(turns))
	for _, t := range turns {
		msgs = append(msgs, map[string]any{"role": t.role, "content": t.blocks})
	}
	body["messages"] = msgs

	tools := make([]any, 0, len(r.Tools)+1)
	for _, t := range r.Tools {
		tools = append(tools, encodeTool(t))
	}
	choice := toolChoice(r.ToolChoice)
	if o := r.Output; o != nil {
		// 这个协议的结构化输出 M0 一律用强制工具（P.09 K251 ④），即使能力表标了 native_schema
		if !c.Tools {
			return nil, errors.New("这个模型不支持工具，没法用强制工具做结构化输出")
		}
		st := llm.StructuredTool(o)
		tools = append(tools, encodeTool(st))
		choice = map[string]any{"type": "tool", "name": st.Name}
	}
	if len(tools) > 0 {
		body["tools"] = tools
	}
	if choice != nil {
		body["tool_choice"] = choice
	}
	if r.Temperature != nil {
		body["temperature"] = *r.Temperature
	}
	if r.TopP != nil {
		body["top_p"] = *r.TopP
	}
	if len(r.Stop) > 0 {
		body["stop_sequences"] = r.Stop
	}
	if v, _ := llm.ResolveEffort(r.Reasoning, c); v != "" {
		if b := c.EffortBudget[v]; b > 0 {
			body["thinking"] = map[string]any{"type": "enabled", "budget_tokens": b}
		}
	}
	return body, nil
}

func partBlock(p llm.Part) (map[string]any, error) {
	switch p.Type {
	case llm.PartText:
		return map[string]any{"type": "text", "text": p.Text}, nil
	case llm.PartImage:
		return map[string]any{"type": "image", "source": source(p)}, nil
	case llm.PartFile:
		return map[string]any{"type": "document", "source": source(p)}, nil
	}
	return nil, fmt.Errorf("不认识的内容类型 %q", p.Type)
}

func source(p llm.Part) map[string]any {
	return map[string]any{"type": "base64", "media_type": p.Mime, "data": base64.StdEncoding.EncodeToString(p.Data)}
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
	var schema any = map[string]any{"type": "object", "properties": map[string]any{}}
	if len(t.Schema) > 0 {
		schema = json.RawMessage(t.Schema)
	}
	m := map[string]any{"name": t.Name, "input_schema": schema}
	if t.Description != "" {
		m["description"] = t.Description
	}
	return m
}

// toolChoice：auto → {type: auto}、required → {type: any}、none → {type: none}、工具名 → {type: tool, name}。
func toolChoice(s string) any {
	switch s {
	case "":
		return nil
	case "auto":
		return map[string]any{"type": "auto"}
	case "required":
		return map[string]any{"type": "any"}
	case "none":
		return map[string]any{"type": "none"}
	}
	return map[string]any{"type": "tool", "name": s}
}
