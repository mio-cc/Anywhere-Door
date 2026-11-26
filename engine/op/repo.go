package op

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/mio-cc/anywhere-door/gen"
)

// Repo 是 op 用的持久化接口（E.05-R4）：op 不 import engine/store，由 engine.NewOpStore 把 store.Store 包成它。
type Repo interface {
	// Tx：fn 返回错误即回滚并原样返回；fn 里拿到的 Repo 的读写都在同一个事务里。
	Tx(ctx context.Context, fn func(Repo) error) error
	// Head 是分支的 head 版本；图或分支不存在时 ErrRepoNotFound。
	Head(ctx context.Context, graphID, branch string) (int64, error)
	Base(ctx context.Context, graphID, branch string, version int64) (Snapshot, error)
	// Range 取 after < version ≤ upTo 的 Op，升序。
	Range(ctx context.Context, graphID, branch string, after, upTo int64) ([]gen.OpRecord, error)
	Op(ctx context.Context, opID string) (gen.OpRecord, error)
	Commit(ctx context.Context, c Commit) error
	Coalesce(ctx context.Context, opID string, payload json.RawMessage) error
	UndoPush(ctx context.Context, e UndoEntry) error
	UndoTop(ctx context.Context, graphID, branch, actorID string, undone bool) (UndoEntry, bool, error)
	UndoMark(ctx context.Context, e UndoEntry, undone bool) error
}

// Snapshot 是 Base 的结果：最近的全文和之后的 Op（重放在 op 里做）。
type Snapshot struct {
	Version int64
	Doc     *gen.Graph
	Ops     []gen.OpRecord
}

// Commit 是一次提交（E.07.01 的 store.Commit）。
type Commit struct {
	GraphID, Branch string
	Ops             []gen.OpRecord
	Doc             *gen.Graph // 非空时为最后一个版本存全文
	Hash, Title     string
	Stats           gen.GraphMetaStats
	At              time.Time
}

// UndoEntry 是撤销栈的一项（E.07.01 的 store.UndoEntry）。
type UndoEntry struct {
	GraphID, Branch, ActorID string
	Seq                      int64
	GroupID                  string
	Undone                   bool
}

// 适配器把存储的哨兵错误换成这两个（errors.Is 判断）。
var (
	ErrRepoNotFound = errors.New("op: 存储里没有")
	ErrRepoConflict = errors.New("op: 存储冲突")
)
