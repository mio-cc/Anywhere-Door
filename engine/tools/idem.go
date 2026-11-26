package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/mio-cc/anywhere-door/engine/blob"
)

// 工具调用的状态（同 store.ToolCall）。CallRecord 还多一种 deduplicated。
const (
	StatusStarted      = "started"
	StatusSucceeded    = "succeeded"
	StatusFailed       = "failed"
	StatusDeduplicated = "deduplicated"
)

// IdemKey 是幂等键：sha256(run_id | exec_key | scope | 规范化的参数)（E.11.01 §5）。
// 规范化：重新编码 JSON，对象的键按字典序、去掉空白、数字原样保留。
func IdemKey(runID, execKey, scope string, args json.RawMessage) (string, error) {
	c, err := canonical(args)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	for _, p := range [][]byte{[]byte(runID), []byte(execKey), []byte(scope), c} {
		_, _ = h.Write(p)
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func canonical(args json.RawMessage) ([]byte, error) {
	if len(bytes.TrimSpace(args)) == 0 {
		return []byte("null"), nil
	}
	dec := json.NewDecoder(bytes.NewReader(args))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

// Record 是工具调用表里的一行（同 store.ToolCall，引用用 blob.Ref）。
type Record struct {
	IdemKey, RunID, ExecKey, Tool string
	Risk                          Risk
	Status                        string
	ArgsRef, ResultRef            blob.Ref
	StartedAt                     time.Time
}

// Ledger 是工具调用表（INV-14）：第 ⑥ 片用 store.ToolCallRepo 的适配器接上。
// Begin 已有同一个键时不写入、返回已有的那一行；Finish 只能从 started 改成 succeeded 或 failed。
type Ledger interface {
	Begin(ctx context.Context, r Record) (existing *Record, err error)
	Finish(ctx context.Context, idemKey, status string, resultRef blob.Ref, endedAt time.Time) error
}
