// Package tools 是工具接口、注册表和调用包装（设计文档 E.11）。
//
// 只依赖 gen、blob、obs、authz（E.11-R4）：资源池、工具调用表、token 计数用本包定义的小接口，
// 事件通过 Hooks 交给调用方写，在组装处接上（P.09 K271）。
package tools

import (
	"encoding/json"
	"fmt"

	"github.com/mio-cc/anywhere-door/engine/blob"
	"github.com/mio-cc/anywhere-door/gen"
)

// Raw 是工具实现返回的原始结果（E.11.02 §4）。
type Raw struct {
	Text      string          // 正文；为空时用 Data 的 JSON 文本
	Data      json.RawMessage // 结构化结果（可选）
	MediaType string          // 默认 text/plain; charset=utf-8
	Sources   []string        // 来源：URL、文件路径、ws:// 引用（污染来源）
}

// Snippet 是放进上下文的一段原文；Offset 是它在原文里的字符（rune）位置。
type Snippet struct {
	Text   string  `json:"text"`
	Offset int     `json:"offset"`
	Score  float64 `json:"score"`
}

// Result 是裁剪后的工具结果。Ref 永远非空（keep_pointer kernel 锁，C.03.03 §5）。
type Result struct {
	Summary  string          `json:"summary"`
	Snippets []Snippet       `json:"snippets,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
	Ref      blob.Ref        `json:"ref"`
	Taint    bool            `json:"taint,omitempty"`
	TaintSrc []string        `json:"taint_src,omitempty"`
	Bytes    int             `json:"bytes"`
	Tokens   int             `json:"tokens"`
}

// Error 是工具错误：Class 是失败类别（C.07），Msg 给人看，不含密钥和完整参数（E.11.01 §7）。
type Error struct {
	Class gen.FailureClass
	Code  string
	Msg   string
	Err   error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Msg + "：" + e.Err.Error()
	}
	return e.Msg
}

// Unwrap 返回底层错误。
func (e *Error) Unwrap() error { return e.Err }

// Errorf 构造一个工具错误。
func Errorf(class gen.FailureClass, code, format string, args ...any) *Error {
	return &Error{Class: class, Code: code, Msg: fmt.Sprintf(format, args...)}
}
