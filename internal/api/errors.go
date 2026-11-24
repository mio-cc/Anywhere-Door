package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/goccy/go-yaml"

	"github.com/mio-cc/anywhere-door/gen"
	"github.com/mio-cc/anywhere-door/schema/builtin/messages"
)

// 包内的哨兵错误；处理函数返回它们（或包着它们的错误），writeError 映射成 HTTP 响应（B.03.03）。
var (
	errUnauthorized = errors.New("api: 没有有效的 cookie 或 Bearer token")
	errOrigin       = errors.New("api: Host 或 Origin 不是本机的 Anywhere Door")
	errTooLarge     = errors.New("api: 请求体超过上限")
	errPanic        = errors.New("api: 处理函数 panic")
)

// notFoundError 是找不到的资源；details 是 C.11 §3 的 resource、id。
type notFoundError struct{ resource, id string }

func (e *notFoundError) Error() string { return "api: 找不到" + e.resource + "：" + e.id }

// FieldError 是 bad_request 的一条原因（C.11 §3：errors: [{path, message}]）。
type FieldError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// badRequestError 是请求不合格；errs 至少一条。
type badRequestError struct{ errs []FieldError }

func (e *badRequestError) Error() string {
	parts := make([]string, len(e.errs))
	for i, fe := range e.errs {
		parts[i] = fe.Path + "：" + fe.Message
	}
	return "api: 请求不合格：" + strings.Join(parts, "；")
}

// forbiddenError 是拒绝（C.11 §3：decision, reasons, action, target）。M0 只有"只有用户能做"的接口检查，
// 不经过授权引擎：action 写"方法 路由"、target 写 anydoor；E.06 落地后由授权引擎的判定结果填（P.09 K245）。
type forbiddenError struct {
	action, target string
	reasons        []string
}

func (e *forbiddenError) Error() string {
	return "api: 拒绝 " + e.action + "：" + strings.Join(e.reasons, "；")
}

// statusOf 是 M0 用到的错误码和 HTTP 状态（B.03.03 §5）；表里没有的错误码一律按 internal 处理。
var statusOf = map[gen.ApiErrorCode]int{
	gen.ErrCodeBadRequest:     http.StatusBadRequest,
	gen.ErrCodeUnauthorized:   http.StatusUnauthorized,
	gen.ErrCodeForbidden:      http.StatusForbidden,
	gen.ErrCodeOriginRejected: http.StatusForbidden,
	gen.ErrCodeNotFound:       http.StatusNotFound,
	gen.ErrCodeTooLarge:       http.StatusRequestEntityTooLarge,
	gen.ErrCodeInternal:       http.StatusInternalServerError,
}

// classify 把错误归到一个错误码、它的 details（写进响应）和文案变量（填 {名字}）。
func classify(err error) (gen.ApiErrorCode, map[string]any, map[string]string) {
	var (
		mb *http.MaxBytesError
		nf *notFoundError
		br *badRequestError
		fb *forbiddenError
	)
	switch {
	case errors.As(err, &br):
		msgs := make([]string, len(br.errs))
		for i, fe := range br.errs {
			msgs[i] = fe.Message
			if fe.Path != "" {
				msgs[i] = fe.Path + "：" + fe.Message
			}
		}
		return gen.ErrCodeBadRequest, map[string]any{"errors": br.errs}, map[string]string{"errors": strings.Join(msgs, "；")}
	case errors.As(err, &fb):
		return gen.ErrCodeForbidden,
			map[string]any{"decision": "deny", "reasons": fb.reasons, "action": fb.action, "target": fb.target},
			map[string]string{"reasons": strings.Join(fb.reasons, "；")}
	case errors.As(err, &nf):
		return gen.ErrCodeNotFound, map[string]any{"resource": nf.resource, "id": nf.id},
			map[string]string{"resource": nf.resource, "id": nf.id}
	case errors.Is(err, errTooLarge), errors.As(err, &mb):
		return gen.ErrCodeTooLarge, map[string]any{"limit": maxBodyText}, map[string]string{"limit": maxBodyText}
	case errors.Is(err, errUnauthorized):
		return gen.ErrCodeUnauthorized, nil, nil
	case errors.Is(err, errOrigin):
		return gen.ErrCodeOriginRejected, nil, nil
	}
	return gen.ErrCodeInternal, nil, nil
}

// errorBody 是 errors.schema.json#/$defs/ErrorResponse 的写法（details 为空时不写）。
type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code      gen.ApiErrorCode `json:"code"`
	Message   string           `json:"message"`
	Details   map[string]any   `json:"details,omitempty"`
	RequestID string           `json:"request_id"`
}

// writeError 是全后端唯一把错误写成 HTTP 响应的地方（B.03.03 §4）。
// 500 时不写 details，错误原文只进日志（G.13-R3、R4）。
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	code, details, vars := classify(err)
	if _, ok := statusOf[code]; !ok {
		code, details, vars = gen.ErrCodeInternal, nil, nil
	}
	if code == gen.ErrCodeInternal {
		logFrom(r.Context()).Error("api: 内部错误", "method", r.Method, "path", r.URL.Path, "err", err)
	}
	msgs, _ := loadMessages()
	writeJSON(w, statusOf[code], errorBody{Error: errorPayload{
		Code:      code,
		Message:   render(msgs[code], vars),
		Details:   details,
		RequestID: requestIDFrom(r.Context()),
	}})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

var placeholderRe = regexp.MustCompile(`\{([a-z_]+)\}`)

// render 把文案里的 {名字} 换成 vars 里同名的值（G.13-R5）；没有的原样留着。
func render(tmpl string, vars map[string]string) string {
	return placeholderRe.ReplaceAllStringFunc(tmpl, func(m string) string {
		if v, ok := vars[m[1:len(m)-1]]; ok {
			return v
		}
		return m
	})
}

// loadMessages 读嵌入的 errors.zh.yaml；statusOf 里的每个错误码都要有文案（缺了 NewServer 就失败）。
var loadMessages = sync.OnceValues(func() (map[gen.ApiErrorCode]string, error) {
	b, err := messages.FS.ReadFile("errors.zh.yaml")
	if err != nil {
		return nil, fmt.Errorf("api: 读不了错误文案：%w", err)
	}
	var m map[string]string
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("api: 错误文案格式不对：%w", err)
	}
	out := make(map[gen.ApiErrorCode]string, len(m))
	for k, v := range m {
		out[gen.ApiErrorCode(k)] = v
	}
	for c := range statusOf {
		if out[c] == "" {
			return nil, fmt.Errorf("api: 错误文案里缺少 %s", c)
		}
	}
	return out, nil
})
