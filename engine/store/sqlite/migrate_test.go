package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	_ "modernc.org/sqlite"
)

func openRaw(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", dsnFor(path))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func tableNames(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func columns(t *testing.T, db *sql.DB, table string) string {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?) ORDER BY cid`, table)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		out = append(out, n)
	}
	return strings.Join(out, ",")
}

func mig(up string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte("-- +goose Up\n" + up + "\n-- +goose Down\nSELECT 1;\n")}
}

var chain = []string{
	`CREATE TABLE a (x INTEGER NOT NULL); INSERT INTO a (x) VALUES (1), (2);`,
	`ALTER TABLE a ADD COLUMN y TEXT NOT NULL DEFAULT '';`,
	`CREATE TABLE a2 (x INTEGER NOT NULL, y TEXT NOT NULL, z INTEGER NOT NULL DEFAULT 0);
INSERT INTO a2 (x, y) SELECT x, y FROM a;
DROP TABLE a;
ALTER TABLE a2 RENAME TO a;`,
}

func chainFS(n int, last string) fstest.MapFS {
	fsys := fstest.MapFS{}
	names := []string{"0001_create.sql", "0002_add_column.sql", "0003_rebuild.sql"}
	for i := 0; i < n; i++ {
		up := chain[i]
		if i == n-1 && last != "" {
			up = last
		}
		fsys[names[i]] = mig(up)
	}
	return fsys
}

func TestMigrate(t *testing.T) {
	ctx := context.Background()
	t.Run("E0703-T1 空库", func(t *testing.T) {
		db := openRaw(t, filepath.Join(t.TempDir(), "a.db"))
		from, to, err := Migrate(ctx, db, nil)
		if err != nil || from != 0 || to != 1 {
			t.Fatalf("Migrate 得到 %d→%d %v，应当 0→1", from, to, err)
		}
		if v, err := SchemaVersion(ctx, db); err != nil || v != 1 {
			t.Errorf("SchemaVersion 是 %d %v，应当 1", v, err)
		}
		want := []string{"hx_branches", "hx_checkpoint_history", "hx_checkpoints", "hx_graph_versions", "hx_graphs", "hx_migrations", "hx_ops",
			"hx_run_events", "hx_run_node_state", "hx_run_views", "hx_runs", "hx_session_events", "hx_settings", "hx_tool_calls", "hx_undo_stack"}
		sort.Strings(want)
		if got := tableNames(t, db); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("表是 %v，应当 %v", got, want)
		}
		from, to, err = Migrate(ctx, db, nil)
		if err != nil || from != 1 || to != 1 {
			t.Errorf("再执行一次得到 %d→%d %v，应当 1→1", from, to, err)
		}
	})
	t.Run("E0703-T2 迁移链", func(t *testing.T) {
		var results []string
		for start := 0; start <= 2; start++ {
			db := openRaw(t, filepath.Join(t.TempDir(), "c.db"))
			if start > 0 {
				if _, _, err := Migrate(ctx, db, chainFS(start, "")); err != nil {
					t.Fatal(err)
				}
			}
			from, to, err := Migrate(ctx, db, chainFS(3, ""))
			if err != nil || from != start || to != 3 {
				t.Fatalf("从 %d 迁移得到 %d→%d %v", start, from, to, err)
			}
			var n int
			if err := db.QueryRow(`SELECT count(*) FROM a WHERE z = 0`).Scan(&n); err != nil {
				t.Fatal(err)
			}
			results = append(results, columns(t, db, "a")+"|"+strings.Repeat("r", n))
		}
		for i, r := range results {
			if r != "x,y,z|rr" {
				t.Errorf("从版本 %d 迁移的结果是 %q，应当 x,y,z|rr", i, r)
			}
		}
		db := openRaw(t, filepath.Join(t.TempDir(), "bad.db"))
		_, to, err := Migrate(ctx, db, chainFS(3, `CREATE TABLE a2 (x INTEGER); INSERT INTO a2 (x) SELECT nope FROM a;`))
		if err == nil || !strings.Contains(err.Error(), "0003_rebuild.sql") {
			t.Errorf("写错的迁移应当返回带文件名的错误，得到 %v", err)
		}
		if v, _ := SchemaVersion(ctx, db); v != 2 || to != 2 {
			t.Errorf("失败后版本应当停在 2，是 %d（to %d）", v, to)
		}
		if got := columns(t, db, "a"); got != "x,y" {
			t.Errorf("失败后表 a 应当是第 2 版的样子 x,y，是 %s", got)
		}
		for _, n := range tableNames(t, db) {
			if n == "a2" {
				t.Error("失败的迁移建的表应当回滚")
			}
		}
	})
	t.Run("E0703-T3 版本过新拒绝", func(t *testing.T) {
		db := openRaw(t, filepath.Join(t.TempDir(), "n.db"))
		if _, _, err := Migrate(ctx, db, nil); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO hx_migrations (version, applied_at) VALUES (99, '2026-11-24T00:00:00.000Z')`); err != nil {
			t.Fatal(err)
		}
		if _, _, err := Migrate(ctx, db, nil); !errors.Is(err, ErrSchemaTooNew) {
			t.Errorf("应当返回 ErrSchemaTooNew，得到 %v", err)
		}
		gap := fstest.MapFS{"0001_a.sql": mig(chain[0]), "0003_c.sql": mig(chain[1])}
		db2 := openRaw(t, filepath.Join(t.TempDir(), "g.db"))
		if _, _, err := Migrate(ctx, db2, gap); err == nil {
			t.Error("序号不连续的迁移目录应当返回错误")
		}
		dup := fstest.MapFS{"0001_a.sql": mig(chain[0]), "0001_b.sql": mig(chain[1])}
		if _, _, err := Migrate(ctx, db2, dup); err == nil {
			t.Error("序号重复的迁移目录应当返回错误")
		}
		noDown := fstest.MapFS{"0001_a.sql": &fstest.MapFile{Data: []byte("-- +goose Up\nSELECT 1;\n")}}
		if _, _, err := Migrate(ctx, db2, noDown); err == nil {
			t.Error("没有 Down 段的迁移文件应当返回错误")
		}
	})
	t.Run("E0703-T4 备份", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "anydoor.db")
		db := openRaw(t, path)
		if _, err := db.Exec(`CREATE TABLE a (x INTEGER); INSERT INTO a (x) VALUES (42);`); err != nil {
			t.Fatal(err)
		}
		for v := 1; v <= 5; v++ {
			got, err := Backup(ctx, db, path, v, 3)
			if err != nil {
				t.Fatal(err)
			}
			if want := path + ".bak-" + string(rune('0'+v)); got != want {
				t.Errorf("备份文件是 %s，应当 %s", got, want)
			}
		}
		left, _ := filepath.Glob(path + ".bak-*")
		sort.Strings(left)
		var names []string
		for _, p := range left {
			names = append(names, filepath.Base(p))
		}
		if strings.Join(names, ",") != "anydoor.db.bak-3,anydoor.db.bak-4,anydoor.db.bak-5" {
			t.Errorf("应当只留最近 3 份，实际 %v", names)
		}
		if _, err := os.Stat(path + ".bak-5"); err != nil {
			t.Fatal(err)
		}
		bak := openRaw(t, path+".bak-5")
		var x int
		if err := bak.QueryRow(`SELECT x FROM a`).Scan(&x); err != nil || x != 42 {
			t.Errorf("备份里的数据应当在：%d %v", x, err)
		}
	})
}
