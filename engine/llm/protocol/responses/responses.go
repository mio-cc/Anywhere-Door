// Package responses 实现 responses 协议（设计文档 E.10.04）：POST {base_url}/responses，SSE 流式。
// 本项目自己管理对话历史（事件日志，INV-11），只用无状态的用法：每次发完整的输入，store: false，不用 previous_response_id。
package responses

import (
	"encoding/json"
	"strings"

	"github.com/mio-cc/anywhere-door/engine/llm"
)

type protocol struct{}

// New 返回 responses 协议。
func New() llm.Protocol { return protocol{} }

func (protocol) Name() string { return llm.ProtoResponses }

func (protocol) DefaultPaths() map[string]string {
	return map[string]string{"complete": "/responses", "models": "/models"}
}

func (protocol) DefaultAuth() llm.Auth { return llm.Auth{Type: llm.AuthBearer} }

func (protocol) DefaultHeaders() map[string]string { return nil }

// DecodeError 取 {error: {message, type, code}}（同 E.10.02）；code 为空时用 type。
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
		return errorObject(e.Error)
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

// errorObject 取 {message, type, code}。
func errorObject(raw json.RawMessage) (string, string) {
	var in struct {
		Message json.RawMessage `json:"message"`
		Type    string          `json:"type"`
		Code    json.RawMessage `json:"code"`
	}
	_ = json.Unmarshal(raw, &in)
	code := text(in.Code)
	if code == "" {
		code = in.Type
	}
	return code, text(in.Message)
}

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
