package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// ErrSchemaTooNew：数据库版本比程序新（C.01-R20）。
var ErrSchemaTooNew = errors.New("sqlite: 数据库版本比程序新，请升级 Anywhere Door")

var migrationName = regexp.MustCompile(`^(\d{4})_[^/\\]+\.sql$`)

const createVersionTable = `CREATE TABLE IF NOT EXISTS hx_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`

type migration struct {
	version int
	name    string
	up      string
}

// SchemaVersion 是已执行的最大迁移序号；没有版本表时为 0。
func SchemaVersion(ctx context.Context, db *sql.DB) (int, error) {
	var n int
	err := db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'hx_migrations'`).Scan(&n)
	if err != nil || n == 0 {
		return 0, err
	}
	var v sql.NullInt64
	if err := db.QueryRowContext(ctx, `SELECT max(version) FROM hx_migrations`).Scan(&v); err != nil {
		return 0, err
	}
	return int(v.Int64), nil
}

// readMigrations 读 fsys 根目录下的迁移文件：序号从 1 起连续、不重名。
func readMigrations(fsys fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("sqlite: 读迁移目录：%w", err)
	}
	var out []migration
	for _, e := range entries {
		m := migrationName.FindStringSubmatch(e.Name())
		if e.IsDir() || m == nil {
			continue
		}
		v, _ := strconv.Atoi(m[1])
		b, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return nil, err
		}
		up, err := upSection(string(b))
		if err != nil {
			return nil, fmt.Errorf("sqlite: 迁移 %s：%w", e.Name(), err)
		}
		out = append(out, migration{version: v, name: e.Name(), up: up})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	for i, m := range out {
		if m.version != i+1 {
			return nil, fmt.Errorf("sqlite: 迁移序号应当从 0001 起连续，%s 的位置应当是 %04d", m.name, i+1)
		}
	}
	return out, nil
}

// upSection 取 "-- +goose Up" 和 "-- +goose Down" 之间的部分。
func upSection(s string) (string, error) {
	const upMark, downMark = "-- +goose Up", "-- +goose Down"
	i, j := strings.Index(s, upMark), strings.Index(s, downMark)
	if i < 0 || j < i {
		return "", errors.New(`要有 "-- +goose Up" 和在它后面的 "-- +goose Down" 两行`)
	}
	return strings.TrimSpace(s[i+len(upMark) : j]), nil
}

// Migrate 执行比当前版本新的迁移；fsys 为 nil 时用打进二进制的 migrations/。
func Migrate(ctx context.Context, db *sql.DB, fsys fs.FS) (from, to int, err error) {
	if fsys == nil {
		if fsys, err = fs.Sub(migrationFS, "migrations"); err != nil {
			return 0, 0, err
		}
	}
	ms, err := readMigrations(fsys)
	if err != nil {
		return 0, 0, err
	}
	if _, err := db.ExecContext(ctx, createVersionTable); err != nil {
		return 0, 0, fmt.Errorf("sqlite: 建版本表：%w", err)
	}
	if from, err = SchemaVersion(ctx, db); err != nil {
		return 0, 0, err
	}
	if from > len(ms) {
		return from, from, ErrSchemaTooNew
	}
	to = from
	for _, m := range ms[from:] {
		if err := apply(ctx, db, m); err != nil {
			return from, to, err
		}
		to = m.version
	}
	return from, to, nil
}

// apply 在一个事务里执行一个迁移并登记；失败时回滚。迁移不在虚拟时钟下跑，applied_at 用真实时间（E.07-R5 的例外）。
func apply(ctx context.Context, db *sql.DB, m migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: 迁移 %s 失败：%w", m.name, err)
	}
	if _, err := tx.ExecContext(ctx, m.up); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("sqlite: 迁移 %s 失败：%w", m.name, err)
	}
	now := time.Now().UTC().Format(timeLayout)
	if _, err := tx.ExecContext(ctx, `INSERT INTO hx_migrations (version, applied_at) VALUES (?, ?)`, m.version, now); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("sqlite: 迁移 %s 失败：%w", m.name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: 迁移 %s 失败：%w", m.name, err)
	}
	return nil
}

// timeLayout 是时间列的写法：RFC 3339 UTC 毫秒（INV-23）。
const timeLayout = "2006-01-02T15:04:05.000Z"

// Backup 把数据库复制到 <path>.bak-<version>，同目录下只留序号最大的 keep 份（keep ≤ 0 时为 3）。
func Backup(ctx context.Context, db *sql.DB, path string, version, keep int) (string, error) {
	if keep <= 0 {
		keep = 3
	}
	target := path + ".bak-" + strconv.Itoa(version)
	if err := os.Remove(target); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("sqlite: 删除旧备份：%w", err)
	}
	if _, err := db.ExecContext(ctx, `VACUUM INTO '`+strings.ReplaceAll(target, "'", "''")+`'`); err != nil {
		return "", fmt.Errorf("sqlite: 备份到 %s：%w", target, err)
	}
	olds, err := filepath.Glob(path + ".bak-*")
	if err != nil {
		return target, err
	}
	type bak struct {
		path string
		n    int
	}
	var baks []bak
	for _, p := range olds {
		if n, err := strconv.Atoi(strings.TrimPrefix(p, path+".bak-")); err == nil {
			baks = append(baks, bak{p, n})
		}
	}
	sort.Slice(baks, func(i, j int) bool { return baks[i].n > baks[j].n })
	for _, b := range baks[min(keep, len(baks)):] {
		if err := os.Remove(b.path); err != nil {
			return target, fmt.Errorf("sqlite: 删除旧备份 %s：%w", b.path, err)
		}
	}
	return target, nil
}
