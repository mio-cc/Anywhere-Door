// Package messages 实现 anthropic_messages 协议（设计文档 E.10.03）：POST {base_url}/messages，SSE 流式。
// Anthropic，以及提供这种兼容接口的服务都用它；包里不知道自己连的是哪一家（E.10-R2）。
package messages

import (
	"encoding/json"

	"github.com/mio-cc/anywhere-door/engine/llm"
)

type protocol struct{}

// New 返回 anthropic_messages 协议。默认鉴权是请求头 x-api-key，默认请求头 anthropic-version: 2023-06-01。
func New() llm.Protocol { return protocol{} }

func (protocol) Name() string { return llm.ProtoMessages }

func (protocol) DefaultPaths() map[string]string {
	return map[string]string{"complete": "/messages", "models": "/models"}
}

func (protocol) DefaultAuth() llm.Auth { return llm.Auth{Type: llm.AuthHeader, Name: "x-api-key"} }

func (protocol) DefaultHeaders() map[string]string {
	return map[string]string{"anthropic-version": "2023-06-01"}
}

// DecodeError 取 {type: error, error: {type, message}}：code 是 error.type。
func (protocol) DecodeError(_ int, body []byte) (string, string) {
	var e struct {
		Error *struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &e) != nil {
		return "", ""
	}
	if e.Error != nil {
		return e.Error.Type, e.Error.Message
	}
	return "", e.Message
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
