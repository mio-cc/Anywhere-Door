package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"
	"time"

	"github.com/mio-cc/anywhere-door/engine/store"

	_ "modernc.org/sqlite" // 驱动名 sqlite
)

// Options 是 Open 的选项。
type Options struct {
	Logger     *slog.Logger
	BackupKeep int // ≤ 0 时为 3
}

// querier 是 *sql.DB 和 *sql.Tx 共有的方法。
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type sqlStore struct {
	wdb, rdb *sql.DB
	w        *writer
	tx       *sql.Tx // 非空：在 Tx 里，所有读写都用它
	log      *slog.Logger
}

// Open 打开（没有就新建）path：有迁移要执行且版本 > 0 时先备份，再迁移（E.07.02 §4）。
func Open(ctx context.Context, path string, opt Options) (store.Store, error) {
	log := opt.Logger
	if log == nil {
		log = slog.Default()
	}
	wdb, err := sql.Open("sqlite", dsnFor(path))
	if err != nil {
		return nil, fmt.Errorf("sqlite: 打开 %s：%w", path, err)
	}
	wdb.SetMaxOpenConns(1)
	fail := func(err error) (store.Store, error) { _ = wdb.Close(); return nil, err }
	cur, err := SchemaVersion(ctx, wdb)
	if err != nil {
		return fail(fmt.Errorf("sqlite: 读 schema 版本：%w", err))
	}
	sub, err := fs.Sub(migrationFS, "migrations")
	if err != nil {
		return fail(err)
	}
	ms, err := readMigrations(sub)
	if err != nil {
		return fail(err)
	}
	if cur > 0 && cur < len(ms) {
		bak, err := Backup(ctx, wdb, path, cur, opt.BackupKeep)
		if err != nil {
			return fail(err)
		}
		log.Info("sqlite: 迁移前已备份", "path", bak)
	}
	from, to, err := Migrate(ctx, wdb, sub)
	if err != nil {
		return fail(err)
	}
	if from != to {
		log.Info("sqlite: 数据库已迁移", "from", from, "to", to)
	}
	rdb, err := sql.Open("sqlite", dsnFor(path))
	if err != nil {
		return fail(err)
	}
	rdb.SetMaxOpenConns(8)
	return &sqlStore{wdb: wdb, rdb: rdb, w: newWriter(wdb, log), log: log}, nil
}

func (s *sqlStore) Graphs() store.GraphRepo           { return graphs{s} }
func (s *sqlStore) Ops() store.OpRepo                 { return ops{s} }
func (s *sqlStore) Runs() store.RunRepo               { return runs{s} }
func (s *sqlStore) Events() store.EventRepo           { return events{s} }
func (s *sqlStore) Checkpoints() store.CheckpointRepo { return checkpoints{s} }
func (s *sqlStore) ToolCalls() store.ToolCallRepo     { return toolCalls{s} }
func (s *sqlStore) Settings() store.SettingsRepo      { return settings{s} }

// Close 关闭写队列（把排队的写完）和两个连接池。
func (s *sqlStore) Close() error {
	if s.tx != nil {
		return errors.New("store: 不能在 Tx 里 Close")
	}
	err := s.w.Close(10 * time.Second)
	return errors.Join(err, s.rdb.Close(), s.wdb.Close())
}

// Tx：fn 里的读写都在同一个写事务里；嵌套调用直接执行 fn。
func (s *sqlStore) Tx(ctx context.Context, fn func(store.Store) error) error {
	if s.tx != nil {
		return fn(s)
	}
	return s.w.Do(ctx, func(tx *sql.Tx) error {
		return fn(&sqlStore{wdb: s.wdb, rdb: s.rdb, w: s.w, tx: tx, log: s.log})
	})
}

// write 执行一个写方法：在 Tx 里用保存点（出错时只撤销这个方法的写入），否则经写队列各开一个事务。
func (s *sqlStore) write(ctx context.Context, fn func(q querier) error) error {
	if s.tx == nil {
		return s.w.Do(ctx, func(tx *sql.Tx) error { return fn(tx) })
	}
	if _, err := s.tx.ExecContext(ctx, `SAVEPOINT hx_method`); err != nil {
		return wrap(err)
	}
	if err := fn(s.tx); err != nil {
		_, _ = s.tx.ExecContext(ctx, `ROLLBACK TO hx_method`)
		_, _ = s.tx.ExecContext(ctx, `RELEASE hx_method`)
		return err
	}
	_, err := s.tx.ExecContext(ctx, `RELEASE hx_method`)
	return wrap(err)
}

func (s *sqlStore) read(ctx context.Context, fn func(q querier) error) error {
	if s.tx != nil {
		return fn(s.tx)
	}
	return fn(s.rdb)
}

// wrap 把约束冲突映射为 ErrConflict，其余包上 "store:"。
func wrap(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, store.ErrNotFound), errors.Is(err, store.ErrConflict), errors.Is(err, store.ErrLeaseHeld):
		return err
	case strings.Contains(err.Error(), "constraint failed"):
		return fmt.Errorf("%w：%v", store.ErrConflict, err)
	}
	return fmt.Errorf("store: %w", err)
}

func enc(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("store: 编码失败：%w", err)
	}
	return string(b), nil
}

func decode[T any](b []byte) (T, error) {
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return v, fmt.Errorf("store: 解码失败：%w", err)
	}
	return v, nil
}

// one 取一行的 doc 列并解码；没有这一行：ErrNotFound。
func one[T any](ctx context.Context, q querier, query string, args ...any) (T, error) {
	var b []byte
	if err := q.QueryRowContext(ctx, query, args...).Scan(&b); err != nil {
		var zero T
		if errors.Is(err, sql.ErrNoRows) {
			return zero, store.ErrNotFound
		}
		return zero, wrap(err)
	}
	return decode[T](b)
}

// many 取多行的 doc 列并解码，没有时返回空切片。
func many[T any](ctx context.Context, q querier, query string, args ...any) ([]T, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, wrap(err)
	}
	defer func() { _ = rows.Close() }()
	out := []T{}
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, wrap(err)
		}
		v, err := decode[T](b)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, wrap(rows.Err())
}

// exists 判断查询有没有结果。
func exists(ctx context.Context, q querier, query string, args ...any) (bool, error) {
	var x int
	err := q.QueryRowContext(ctx, query, args...).Scan(&x)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, wrap(err)
}

func fmtTime(t time.Time) string { return t.UTC().Format(timeLayout) }

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return t, fmt.Errorf("store: 时间 %q 解析失败：%w", s, err)
	}
	return t.UTC(), nil
}

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return fmtTime(*t)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// cursorWhere 是键集分页的条件：排在游标之后（倒序）。
func cursorWhere(cursor, timeCol, idCol string) (string, []any, error) {
	if cursor == "" {
		return "", nil, nil
	}
	t, id, err := store.DecodeCursor(cursor)
	if err != nil {
		return "", nil, err
	}
	ts := fmtTime(t)
	return " AND (" + timeCol + " < ? OR (" + timeCol + " = ? AND " + idCol + " < ?))", []any{ts, ts, id}, nil
}

// pageOf 从多取了一条的结果里切出一页。
func pageOf[T any](items []T, limit int, at func(T) time.Time, id func(T) string) store.Page[T] {
	if len(items) <= limit {
		return store.Page[T]{Items: items}
	}
	last := items[limit-1]
	return store.Page[T]{Items: items[:limit], NextCursor: store.EncodeCursor(at(last), id(last))}
}
