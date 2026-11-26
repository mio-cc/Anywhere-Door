package op

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mio-cc/anywhere-door/gen"
)

// 流水线的错误（E.05.01 §7）；授权拒绝的三种在 authz_hook.go。API 层用 errors.Is / errors.As 映射到 C.11 §3 的错误码。
var (
	ErrBadRequest   = errors.New("op: 请求不合法")   // 400 bad_request
	ErrInvalid      = errors.New("op: 前置条件不满足") // 400 bad_request
	ErrNotFound     = errors.New("op: 不存在")     // 404 not_found
	ErrConflict     = errors.New("op: 版本冲突")    // 409 graph_conflict
	ErrInvalidGraph = errors.New("op: 图校验不通过")  // 422 invalid_graph
)

// ConflictError 是 409 graph_conflict（GraphConflictDetails：head、missed_ops、conflicts）。
type ConflictError struct {
	Head      int64
	Missed    []gen.OpRecord
	Conflicts []Conflict
	Reason    string // 不是因为写集合相交时写原因（例如目标 Op 已撤销）
}

func (e *ConflictError) Error() string {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "%v（head %d）", ErrConflict, e.Head)
	if e.Reason != "" {
		b.WriteString("：" + e.Reason)
	}
	for _, c := range e.Conflicts {
		_, _ = fmt.Fprintf(&b, "；%s 和错过的修改都改了 %s", c.OpID, c.Object)
	}
	return b.String()
}

func (e *ConflictError) Unwrap() error { return ErrConflict }

// InvalidGraphError 是 422 invalid_graph：这一批新出现的、阻止提交的诊断（P.09 K262 ②）。
type InvalidGraphError struct{ Diags []gen.Diag }

func (e *InvalidGraphError) Error() string {
	msgs := make([]string, 0, len(e.Diags))
	for _, d := range e.Diags {
		msgs = append(msgs, string(d.Code)+" "+d.Msg)
	}
	return ErrInvalidGraph.Error() + "：" + strings.Join(msgs, "；")
}

func (e *InvalidGraphError) Unwrap() error { return ErrInvalidGraph }

func badf(format string, a ...any) error {
	return fmt.Errorf("%w：%s", ErrBadRequest, fmt.Sprintf(format, a...))
}

func invalidf(format string, a ...any) error {
	return fmt.Errorf("%w：%s", ErrInvalid, fmt.Sprintf(format, a...))
}

func notFoundf(format string, a ...any) error {
	return fmt.Errorf("%w：%s", ErrNotFound, fmt.Sprintf(format, a...))
}

// repoErr 把存储的"不存在"换成 ErrNotFound，其余原样。
func repoErr(err error, what string) error {
	if errors.Is(err, ErrRepoNotFound) {
		return fmt.Errorf("%w：%s：%w", ErrNotFound, what, err)
	}
	return err
}
