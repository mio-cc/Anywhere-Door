// Package store 定义持久化接口（E.07.01）：所有持久化读写都通过它。
// 实现在 engine/store/sqlite 和 engine/store/memstore，两者跑同一套契约测试 storetest（E.07-R3）。
// 存储不读时钟、不生成 ID（E.07-R5），不提供绕过 Op 流水线改图内容的方法（E.07-R4）。
package store

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mio-cc/anywhere-door/gen"
)

// Store 是本片的 7 个仓库（E.07-R6）。
type Store interface {
	Graphs() GraphRepo
	Ops() OpRepo
	Runs() RunRepo
	Events() EventRepo
	Checkpoints() CheckpointRepo
	ToolCalls() ToolCallRepo
	Settings() SettingsRepo
	// Tx：fn 返回错误即回滚并原样返回；fn panic 时回滚并返回错误；嵌套调用复用外层事务。
	Tx(ctx context.Context, fn func(Store) error) error
	Close() error
}

// GraphRepo 管图的元信息、版本和分支；图内容只能经 Commit 写（E.07-R4）。
type GraphRepo interface {
	Create(ctx context.Context, meta gen.GraphMeta, doc *gen.Graph, by gen.Actor) error
	Meta(ctx context.Context, id string) (gen.GraphMeta, error)
	List(ctx context.Context, q GraphQuery) (Page[gen.GraphMeta], error)
	Commit(ctx context.Context, c Commit) error
	Base(ctx context.Context, id, branch string, version int64) (Snapshot, error)
	SoftDelete(ctx context.Context, id string, at time.Time) error
	Branches(ctx context.Context, id string) ([]gen.BranchMeta, error)
	CreateBranch(ctx context.Context, id string, b gen.BranchMeta) error
	DeleteBranch(ctx context.Context, id, name string, at time.Time) error
}

// Commit 是一次提交：追加 Op、存版本、推进分支、更新元信息，在同一个事务里。
type Commit struct {
	GraphID, Branch string
	Ops             []gen.OpRecord // 版本号从分支 head+1 起连续；每条的 GraphID、Branch 与外层相同
	Doc             *gen.Graph     // 非空时为最后一个版本存全文（engine/op 决定）
	Hash            string         // 最后一个版本的图内容哈希（sha256:…）
	Title           string         // 提交后的标题与规模，写进 GraphMeta
	Stats           gen.GraphMetaStats
	At              time.Time
}

// Snapshot 是 Base 的结果：最近的全文和之后的 Op，重放由 engine/op 做。
type Snapshot struct {
	Version int64 // Doc 的版本（≤ 请求的版本）
	Doc     *gen.Graph
	Ops     []gen.OpRecord // Version 之后、到请求的版本为止的 Op，升序
}

// GraphQuery：updated_at 倒序、同时间按 id 倒序；不含已删除；Limit ≤ 0 时取 50，最多 200。
type GraphQuery struct {
	Limit  int
	Cursor string
}

// OpRepo 管 Op 日志和撤销栈。
type OpRepo interface {
	// Range 取 after < version ≤ upTo 的 Op，升序。
	Range(ctx context.Context, graphID, branch string, after, upTo int64) ([]gen.OpRecord, error)
	Get(ctx context.Context, opID string) (gen.OpRecord, error)
	// Coalesce 只替换已有 Op 的 payload（C.05.04 §4）。
	Coalesce(ctx context.Context, opID string, payload json.RawMessage) error
	UndoPush(ctx context.Context, e UndoEntry) error
	UndoTop(ctx context.Context, graphID, branch, actorID string, undone bool) (UndoEntry, bool, error)
	UndoMark(ctx context.Context, e UndoEntry, undone bool) error
}

// UndoEntry 是撤销栈的一项；Seq 由调用方给，同一 (图, 分支, actor) 递增。
type UndoEntry struct {
	GraphID, Branch, ActorID string
	Seq                      int64
	GroupID                  string
	Undone                   bool
}

// RunRepo 管运行的元信息、租约、节点状态投影和视图快照。
type RunRepo interface {
	Create(ctx context.Context, m gen.RunMeta) (gen.RunMeta, error) // 分配 no，返回写入的 RunMeta
	Get(ctx context.Context, id string) (gen.RunMeta, error)
	List(ctx context.Context, q RunQuery) (Page[gen.RunMeta], error)
	SetStatus(ctx context.Context, id string, s gen.RunStatus, endedAt *time.Time) error
	AddUsage(ctx context.Context, id string, u Usage) error
	SetProgress(ctx context.Context, id string, headline *gen.RunMetaHeadline, steps gen.RunSteps) error
	AcquireLease(ctx context.Context, id, owner string, now, until time.Time) (bool, error)
	RenewLease(ctx context.Context, id, owner string, until time.Time) error
	Unfinished(ctx context.Context) ([]gen.RunMeta, error)
	PutNodeStates(ctx context.Context, runID string, rows []gen.NodeStateRow) error
	NodeStates(ctx context.Context, runID string) ([]gen.NodeStateRow, error)
	PutView(ctx context.Context, runID string, seq int64, ref string) error
	ViewBefore(ctx context.Context, runID string, seq int64) (viewSeq int64, ref string, ok bool, err error)
}

// RunQuery：created_at 倒序、同时间按 id 倒序；过滤条件都是"等于"，空值不过滤。
type RunQuery struct {
	GraphID, SessionID string
	Kind               string
	Status             gen.RunStatus
	Limit              int
	Cursor             string
}

// Usage 是一次累加的用量（E.07-R1：不用 obs 的类型）。
type Usage struct{ In, Out, Cached, CostMicros int64 }

// EventRepo 按流分方法；图协同流不进事件表（P.09 K237）。
type EventRepo interface {
	// AppendRun 在同一个事务里给 evs 原地填 seq（接着 last_seq），并更新 RunMeta.last_seq。
	AppendRun(ctx context.Context, runID string, evs []gen.RunEvent) (last int64, err error)
	RunRange(ctx context.Context, runID string, after int64, limit int) ([]gen.RunEvent, error)
	RunByNode(ctx context.Context, runID, nodeID string, after int64) ([]gen.RunEvent, error)
	RunUntil(ctx context.Context, runID string, upTo int64) ([]gen.RunEvent, error)
	AppendSession(ctx context.Context, sessionID string, evs []gen.SessionEvent) (last int64, err error)
	SessionRange(ctx context.Context, sessionID string, after int64, limit int) ([]gen.SessionEvent, error)
}

// CheckpointRepo 管执行器的检查点。
type CheckpointRepo interface {
	Put(ctx context.Context, cp Checkpoint, keepHistory bool) error
	Latest(ctx context.Context, runID, execKey string) (Checkpoint, bool, error)
	At(ctx context.Context, runID, execKey string, seq int64) (Checkpoint, bool, error)
}

// Checkpoint 是一个节点执行在某个 seq 的状态引用。
type Checkpoint struct {
	RunID, ExecKey string
	Seq            int64
	Executor       string
	StateRef       string
	TS             time.Time
}

// ToolCallRepo 按幂等键记录工具调用（INV-14）。
type ToolCallRepo interface {
	Begin(ctx context.Context, r ToolCall) (existing *ToolCall, err error)
	Finish(ctx context.Context, idemKey, status, resultRef string, endedAt time.Time) error
	Get(ctx context.Context, idemKey string) (ToolCall, error)
}

// ToolCall 的 Status 是 started / succeeded / failed。
type ToolCall struct {
	IdemKey, RunID, ExecKey, Tool string
	Risk                          int
	Status                        string
	ArgsRef, ResultRef            string
	StartedAt                     time.Time
	EndedAt                       *time.Time
}

// 工具调用的状态。
const (
	ToolStarted   = "started"
	ToolSucceeded = "succeeded"
	ToolFailed    = "failed"
)

// SettingsRepo 是键值设置，value 是任意 JSON。
type SettingsRepo interface {
	Get(ctx context.Context, key string) (json.RawMessage, bool, error)
	Put(ctx context.Context, key string, value json.RawMessage, at time.Time) error
	All(ctx context.Context) (map[string]json.RawMessage, error)
}

// Page 是一页结果；NextCursor 为空表示没有下一页，游标对调用方不透明。
type Page[T any] struct {
	Items      []T
	NextCursor string
}

// 列表和事件读取的默认条数。
const (
	DefaultLimit      = 50
	MaxLimit          = 200
	DefaultEventLimit = 1000
)

// ClampLimit 把列表的 Limit 规整到 1–MaxLimit（≤ 0 时为 DefaultLimit）。
func ClampLimit(n int) int {
	switch {
	case n <= 0:
		return DefaultLimit
	case n > MaxLimit:
		return MaxLimit
	}
	return n
}

// draftRe 与 op.schema.json 的 Branch 一致（main 或 draft/…）。
var draftRe = regexp.MustCompile(`^draft/[A-Za-z0-9][A-Za-z0-9_.-]{0,79}$`)

// IsDraftBranch 判断 name 是不是合法的草稿分支名。
func IsDraftBranch(name string) bool { return draftRe.MatchString(name) }

// ErrBadCursor 是游标解不开时的错误。
var ErrBadCursor = errors.New("store: 游标无效")

// EncodeCursor 把上一页最后一项的排序键（时间、id）编成游标；两种实现共用，调用方不解析它。
func EncodeCursor(at time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(at.UnixMilli(), 10) + "|" + id))
}

// DecodeCursor 是 EncodeCursor 的逆。
func DecodeCursor(c string) (time.Time, string, error) {
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return time.Time{}, "", ErrBadCursor
	}
	ms, id, ok := strings.Cut(string(b), "|")
	n, err := strconv.ParseInt(ms, 10, 64)
	if !ok || err != nil || id == "" {
		return time.Time{}, "", ErrBadCursor
	}
	return time.UnixMilli(n).UTC(), id, nil
}
