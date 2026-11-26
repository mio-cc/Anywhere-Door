package ctxasm

import (
	"fmt"

	"github.com/mio-cc/anywhere-door/engine/blob"
	"github.com/mio-cc/anywhere-door/gen"
)

// Value 是装配用的一个输入值（E.13.01 §4）。执行器（第 ⑥ 片）把它自己的值转成 Value。
type Value struct {
	Type     string         // 数据类型引用，如 任务卡@1、列表[方案@1]；空 = 任意
	Data     any            // JSON 解码后的值：map[string]any、[]any、string、float64、bool、nil
	Ref      blob.Ref       // 这个值在工作区的引用（有时有）
	Taint    bool           // 含外部内容（E.06.03）
	TaintSrc []string       // 污染来源
	Inherit  map[string]any // 沿数据流携带的 inherit 字段副本（E.13.06）
}

// Counter 计 token；tokens.Counter（E.10.07）满足它。
type Counter interface {
	CountText(s string) int
}

// Error 是装配错误；Class 是失败类别（C.07）。
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

func diag(level gen.DiagLevel, format string, args ...any) gen.Diag {
	return gen.Diag{Code: gen.DiagCode("V03"), Level: level, Msg: fmt.Sprintf(format, args...)}
}
