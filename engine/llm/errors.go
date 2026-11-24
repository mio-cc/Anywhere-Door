package llm

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mio-cc/anywhere-door/gen"
)

// Error 是调用模型时的错误（E.10.01 §4.5）。调用时的错误一律是 *Error。
type Error struct {
	Class      gen.FailureClass // C.11 §4
	Status     int              // HTTP 状态；网络错误为 0
	Provider   string
	Code       string // 上游错误码
	Message    string // 上游说明（截到 500 字，不含密钥）
	RetryAfter time.Duration
}

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString("llm: ")
	if e.Provider != "" {
		b.WriteString("提供方 " + e.Provider + "：")
	}
	b.WriteString(e.Message)
	b.WriteString("（" + string(e.Class))
	if e.Status != 0 {
		b.WriteString("，HTTP " + strconv.Itoa(e.Status))
	}
	if e.Code != "" {
		b.WriteString("，" + e.Code)
	}
	b.WriteString("）")
	return b.String()
}

// ErrSecretLocked：SecretResolver 在密钥库锁定时返回包着它的错误（组装处把 secret.ErrLocked 包成它，P.09 K251 ⑦）。
var ErrSecretLocked = errors.New("llm: 密钥库已锁定")

// 本包自己产生的错误码。
const (
	CodeSecretLocked    = "secret_locked"    // 上层对外报 provider_unavailable（B.07.02 §5）
	CodeSecretMissing   = "secret_missing"   // 密钥不存在
	CodeTestGuard       = "test_guard"       // 测试中不允许调用真实模型（E.10-R5）
	CodeStreamTruncated = "stream_truncated" // 流在结束标志之前断开
	CodeTimeout         = "timeout"
	CodeNetwork         = "network"
)

const maxMessage = 500

func newError(class gen.FailureClass, code, msg string) *Error {
	return &Error{Class: class, Code: code, Message: msg}
}

// Errorf 是协议包用的：分类留给本包（Class 为空时按 §5.5 分类）。
func Errorf(class gen.FailureClass, code, format string, args ...any) *Error {
	return &Error{Class: class, Code: code, Message: fmt.Sprintf(format, args...)}
}

// classify 按 E.10.01 §5.5：先查 quirks 表的错误规则，没有命中再按通用规则。
func classify(caps CapsTable, protocol string, status int, code, msg string) gen.FailureClass {
	if caps != nil {
		if c, ok := caps.ErrorClass(protocol, status, code, msg); ok {
			return gen.FailureClass(c)
		}
	}
	switch {
	case status == 0 || status == http.StatusRequestTimeout || status >= 500:
		return gen.FailureClassTransient
	case status == http.StatusTooManyRequests:
		return gen.FailureClassRateLimited
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return gen.FailureClassAuthDenied
	case status == http.StatusBadRequest || status == http.StatusNotFound || status == http.StatusRequestEntityTooLarge || status == http.StatusUnprocessableEntity:
		return gen.FailureClassUserInput
	}
	return gen.FailureClassInternal
}

// parseRetryAfter 认秒数和 HTTP 日期两种写法；不认识或已经过去时为 0。
func parseRetryAfter(v string, now time.Time) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if n, err := strconv.Atoi(v); err == nil {
		if n < 0 {
			return 0
		}
		return time.Duration(n) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := t.Sub(now); d > 0 {
			return d.Round(time.Second)
		}
	}
	return 0
}

var keyLikeRe = regexp.MustCompile(`(?i)\bsk-[a-z0-9_\-]{8,}|\bbearer\s+[a-z0-9._~+/\-]{8,}=*`)

// Redact 把 s 里出现的密钥原文和像密钥的串（sk-…、Bearer …）换成 [REDACTED]（INV-22，录放的脱敏也用它，E.10.08 §5）。
func Redact(s string, secrets ...string) string {
	for _, k := range secrets {
		if len(k) >= 4 {
			s = strings.ReplaceAll(s, k, "[REDACTED]")
		}
	}
	return keyLikeRe.ReplaceAllString(s, "[REDACTED]")
}

// truncate 截到 n 个字符。
func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}
