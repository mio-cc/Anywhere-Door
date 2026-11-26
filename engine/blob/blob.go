// Package blob 是内容寻址存储（设计文档 E.08）：大文本、模型请求与响应、工具结果、上下文包、检查点状态、上传文件
// 都按 sha256 存一份，用 ws://sha256:<hex> 引用。只依赖标准库（G.03-R2）；文件系统实现在 fsblob。
package blob

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Ref 是内容的引用："ws://sha256:<64 位小写十六进制>"（C.01-R13）。
type Ref string

const refPrefix = "ws://sha256:"

var refRe = regexp.MustCompile(`^ws://sha256:[0-9a-f]{64}$`)

// 错误（E.08 §7），用 errors.Is 判断。
var (
	ErrNotFound = errors.New("blob: 没有这个内容")
	ErrBadRef   = errors.New("blob: 引用格式不对")
	ErrCorrupt  = errors.New("blob: 内容和引用里的哈希不一致")
)

// ParseRef 检查引用的格式。
func ParseRef(s string) (Ref, error) {
	if !refRe.MatchString(s) {
		return "", fmt.Errorf("%w：%q", ErrBadRef, s)
	}
	return Ref(s), nil
}

// RefOf 是 sha256 十六进制摘要对应的引用。
func RefOf(hash string) Ref { return Ref(refPrefix + hash) }

// Hash 是引用里的 sha256 十六进制摘要。
func (r Ref) Hash() string { return strings.TrimPrefix(string(r), refPrefix) }

// Meta 是内容的元信息。
type Meta struct {
	Size      int64
	MediaType string
}

// Reader 是给执行器的只读视图。读出的内容在读到结尾时校验哈希，不一致时那次 Read 返回 ErrCorrupt。
type Reader interface {
	Get(ctx context.Context, ref Ref) (io.ReadCloser, Meta, error)
	Stat(ctx context.Context, ref Ref) (Meta, error)
}

// Store 是可写的内容存储。同样的内容写多次引用相同、只存一份。
type Store interface {
	Reader
	Put(ctx context.Context, r io.Reader, mediaType string) (Ref, error)
}
