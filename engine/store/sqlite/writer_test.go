package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// openPair 打开一个临时库：写连接（1 个）和读连接池，建测试表 t(grp, seq)。
func openPair(t *testing.T) (wdb, rdb *sql.DB) {
	t.Helper()
	dsn := dsnFor(filepath.Join(t.TempDir(), "w.db"))
	var err error
	if wdb, err = sql.Open("sqlite", dsn); err != nil {
		t.Fatal(err)
	}
	wdb.SetMaxOpenConns(1)
	if rdb, err = sql.Open("sqlite", dsn); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rdb.Close(); _ = wdb.Close() })
	if _, err := wdb.Exec(`CREATE TABLE t (grp INTEGER NOT NULL, seq INTEGER NOT NULL, PRIMARY KEY (grp, seq))`); err != nil {
		t.Fatal(err)
	}
	return wdb, rdb
}

func count(t *testing.T, db *sql.DB, where string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM t WHERE ` + where).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func insertNext(tx *sql.Tx, grp, n int) error {
	var top int
	if err := tx.QueryRow(`SELECT coalesce(max(seq), 0) FROM t WHERE grp = ?`, grp).Scan(&top); err != nil {
		return err
	}
	for i := 1; i <= n; i++ {
		if _, err := tx.Exec(`INSERT INTO t (grp, seq) VALUES (?, ?)`, grp, top+i); err != nil {
			return err
		}
	}
	return nil
}

func TestWriter(t *testing.T) {
	ctx := context.Background()
	t.Run("E0704-T1 并发写", func(t *testing.T) {
		wdb, rdb := openPair(t)
		w := newWriter(wdb, nil)
		defer func() { _ = w.Close(10 * time.Second) }()
		per := 10000
		if testing.Short() {
			per = 1000
		}
		start := time.Now()
		var wg sync.WaitGroup
		errs := make(chan error, 10)
		for g := 0; g < 10; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < per/100; i++ {
					if err := w.Do(ctx, func(tx *sql.Tx) error { return insertNext(tx, g, 100) }); err != nil {
						errs <- err
						return
					}
				}
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Fatal(err)
		}
		t.Logf("10 × %d 条用时 %v", per, time.Since(start))
		for g := 0; g < 10; g++ {
			var n, lo, hi int
			if err := rdb.QueryRow(`SELECT count(*), min(seq), max(seq) FROM t WHERE grp = ?`, g).Scan(&n, &lo, &hi); err != nil {
				t.Fatal(err)
			}
			if n != per || lo != 1 || hi != per {
				t.Errorf("第 %d 组：%d 条，seq %d–%d，应当 %d 条、1–%d 连续", g, n, lo, hi, per, per)
			}
		}
		// 3s 是单独运行时的指标（E.07.04 §8）：整仓测试时各包并行、机器忙，只在 ANYDOOR_PERF=1 时按 3s 判；
		// 平时只拦明显的退化（> 10s）。
		limit := 10 * time.Second
		if os.Getenv("ANYDOOR_PERF") == "1" {
			limit = 3 * time.Second
		}
		if d := time.Since(start); !testing.Short() && d > limit {
			t.Errorf("总耗时 %v，应当 < %v", d, limit)
		}
	})
	t.Run("E0704-T2 关闭清空", func(t *testing.T) {
		wdb, rdb := openPair(t)
		w := newWriter(wdb, nil)
		gate := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = w.Do(ctx, func(tx *sql.Tx) error { <-gate; return insertNext(tx, 0, 1) })
		}()
		time.Sleep(20 * time.Millisecond)
		for i := 0; i < 99; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := w.Do(ctx, func(tx *sql.Tx) error { return insertNext(tx, 0, 1) }); err != nil {
					t.Errorf("排队的请求失败：%v", err)
				}
			}()
		}
		time.Sleep(50 * time.Millisecond)
		closed := make(chan error, 1)
		go func() { closed <- w.Close(10 * time.Second) }()
		time.Sleep(20 * time.Millisecond)
		close(gate)
		if err := <-closed; err != nil {
			t.Fatal(err)
		}
		wg.Wait()
		if n := count(t, rdb, "grp = 0"); n != 100 {
			t.Errorf("关闭时应当把 100 个请求都执行完，实际 %d", n)
		}
		if err := w.Do(ctx, func(*sql.Tx) error { return nil }); !errors.Is(err, errWriterClosed) {
			t.Errorf("关闭后 Do 应当返回 errWriterClosed，得到 %v", err)
		}
	})
	t.Run("E0704-T3 回滚与 panic", func(t *testing.T) {
		wdb, rdb := openPair(t)
		w := newWriter(wdb, nil)
		defer func() { _ = w.Close(time.Second) }()
		boom := errors.New("boom")
		err := w.Do(ctx, func(tx *sql.Tx) error {
			if err := insertNext(tx, 1, 5); err != nil {
				return err
			}
			return boom
		})
		if !errors.Is(err, boom) {
			t.Errorf("fn 的错误应当原样返回，得到 %v", err)
		}
		err = w.Do(ctx, func(tx *sql.Tx) error {
			if err := insertNext(tx, 2, 5); err != nil {
				return err
			}
			panic("坏了")
		})
		if err == nil {
			t.Error("fn panic 时应当返回错误")
		}
		if n := count(t, rdb, "1 = 1"); n != 0 {
			t.Errorf("回滚后不应当留下数据，有 %d 条", n)
		}
		if err := w.Do(ctx, func(tx *sql.Tx) error { return insertNext(tx, 3, 1) }); err != nil {
			t.Fatalf("之后的请求应当正常：%v", err)
		}
		if n := count(t, rdb, "grp = 3"); n != 1 {
			t.Errorf("之后的请求应当写入 1 条，有 %d 条", n)
		}
	})
	t.Run("E0704-T4 排队时取消", func(t *testing.T) {
		wdb, _ := openPair(t)
		w := newWriter(wdb, nil)
		defer func() { _ = w.Close(5 * time.Second) }()
		gate := make(chan struct{})
		go func() { _ = w.Do(ctx, func(*sql.Tx) error { <-gate; return nil }) }()
		time.Sleep(20 * time.Millisecond)
		cctx, cancel := context.WithCancel(ctx)
		ran := make(chan struct{}, 1)
		got := make(chan error, 1)
		go func() { got <- w.Do(cctx, func(*sql.Tx) error { ran <- struct{}{}; return nil }) }()
		time.Sleep(20 * time.Millisecond)
		cancel()
		select {
		case err := <-got:
			if !errors.Is(err, context.Canceled) {
				t.Errorf("应当返回 context.Canceled，得到 %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("取消后 Do 没有立刻返回")
		}
		close(gate)
		if err := w.Do(ctx, func(*sql.Tx) error { return nil }); err != nil {
			t.Fatal(err)
		}
		select {
		case <-ran:
			t.Error("取消的请求不应当执行")
		default:
		}
	})
}

func BenchmarkAppendEvents(b *testing.B) {
	dsn := dsnFor(filepath.Join(b.TempDir(), "b.db"))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		b.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`CREATE TABLE t (grp INTEGER NOT NULL, seq INTEGER NOT NULL, PRIMARY KEY (grp, seq))`); err != nil {
		b.Fatal(err)
	}
	w := newWriter(db, nil)
	defer func() { _ = w.Close(10 * time.Second) }()
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := w.Do(ctx, func(tx *sql.Tx) error { return insertNext(tx, 0, 64) }); err != nil {
			b.Fatal(fmt.Errorf("第 %d 次：%w", i, err))
		}
	}
}
