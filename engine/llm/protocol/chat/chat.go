// Package chat 实现 chat_completions 协议（设计文档 E.10.02）：POST {base_url}/chat/completions，SSE 流式。
// 所有讲这种协议的服务都用它，区别只在提供方配置里（E.10.01 §4.3）；包里不知道自己连的是哪一家（E.10-R2）。
package chat

import (
	"encoding/json"
	"strings"

	"github.com/mio-cc/anywhere-door/engine/llm"
)

type protocol struct{}

// New 返回 chat_completions 协议。
func New() llm.Protocol { return protocol{} }

func (protocol) Name() string { return llm.ProtoChat }

func (protocol) DefaultPaths() map[string]string {
	return map[string]string{"complete": "/chat/completions", "models": "/models"}
}

func (protocol) DefaultAuth() llm.Auth { return llm.Auth{Type: llm.AuthBearer} }

func (protocol) DefaultHeaders() map[string]string { return nil }

// DecodeError 取 {error: {message, type, code}}；code 为空时用 type。也认顶层的 message / type / code / detail。
func (protocol) DecodeError(_ int, body []byte) (string, string) {
	var e struct {
		Error   json.RawMessage `json:"error"`
		Message json.RawMessage `json:"message"`
		Type    string          `json:"type"`
		Code    json.RawMessage `json:"code"`
		Detail  json.RawMessage `json:"detail"`
	}
	if json.Unmarshal(body, &e) != nil {
		return "", ""
	}
	if len(e.Error) > 0 && e.Error[0] == '{' {
		var in struct {
			Message json.RawMessage `json:"message"`
			Type    string          `json:"type"`
			Code    json.RawMessage `json:"code"`
		}
		_ = json.Unmarshal(e.Error, &in)
		code := text(in.Code)
		if code == "" {
			code = in.Type
		}
		return code, text(in.Message)
	}
	code := text(e.Code)
	if code == "" {
		code = e.Type
	}
	msg := text(e.Message)
	if msg == "" {
		msg = text(e.Error)
	}
	if msg == "" {
		msg = text(e.Detail)
	}
	return code, msg
}

// text 把 JSON 字符串解出来；null 为空；其他 JSON 原样（紧凑）返回。
func text(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return ""
	}
	var str string
	if json.Unmarshal(raw, &str) == nil {
		return str
	}
	return s
}

// DecodeModels 取 GET /models 的 {data: [{id}]}。
func (protocol) DecodeModels(body []byte) ([]string, error) {
	var m struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(m.Data))
	for _, d := range m.Data {
		out = append(out, d.ID)
	}
	return out, nil
}
