package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/mio-cc/anywhere-door/engine/store"
)

type checkpoints struct{ s *sqlStore }

const cpCols = `run_id, exec_key, seq, executor, state_ref, ts`

func (r checkpoints) Put(ctx context.Context, cp store.Checkpoint, keepHistory bool) error {
	return r.s.write(ctx, func(q querier) error {
		args := []any{cp.RunID, cp.ExecKey, cp.Seq, cp.Executor, cp.StateRef, fmtTime(cp.TS)}
		if _, err := q.ExecContext(ctx, `INSERT INTO hx_checkpoints (`+cpCols+`) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (run_id, exec_key) DO UPDATE SET seq = excluded.seq, executor = excluded.executor, state_ref = excluded.state_ref, ts = excluded.ts`, args...); err != nil {
			return wrap(err)
		}
		if !keepHistory {
			return nil
		}
		_, err := q.ExecContext(ctx, `INSERT INTO hx_checkpoint_history (`+cpCols+`) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (run_id, exec_key, seq) DO UPDATE SET executor = excluded.executor, state_ref = excluded.state_ref, ts = excluded.ts`, args...)
		return wrap(err)
	})
}

func scanCheckpoint(row *sql.Row) (store.Checkpoint, bool, error) {
	var cp store.Checkpoint
	var ts string
	err := row.Scan(&cp.RunID, &cp.ExecKey, &cp.Seq, &cp.Executor, &cp.StateRef, &ts)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return store.Checkpoint{}, false, nil
	case err != nil:
		return store.Checkpoint{}, false, wrap(err)
	}
	if cp.TS, err = parseTime(ts); err != nil {
		return store.Checkpoint{}, false, err
	}
	return cp, true, nil
}

func (r checkpoints) Latest(ctx context.Context, runID, execKey string) (cp store.Checkpoint, ok bool, err error) {
	err = r.s.read(ctx, func(q querier) error {
		cp, ok, err = scanCheckpoint(q.QueryRowContext(ctx, `SELECT `+cpCols+` FROM hx_checkpoints WHERE run_id = ? AND exec_key = ?`, runID, execKey))
		return err
	})
	return cp, ok, err
}

func (r checkpoints) At(ctx context.Context, runID, execKey string, seq int64) (cp store.Checkpoint, ok bool, err error) {
	err = r.s.read(ctx, func(q querier) error {
		cp, ok, err = scanCheckpoint(q.QueryRowContext(ctx, `SELECT `+cpCols+` FROM hx_checkpoint_history WHERE run_id = ? AND exec_key = ? AND seq <= ? ORDER BY seq DESC LIMIT 1`,
			runID, execKey, seq))
		return err
	})
	return cp, ok, err
}

type toolCalls struct{ s *sqlStore }

const toolCols = `idem_key, run_id, exec_key, tool, risk, status, args_ref, result_ref, started_at, ended_at`

func getToolCall(ctx context.Context, q querier, key string) (store.ToolCall, error) {
	var c store.ToolCall
	var started string
	var ended sql.NullString
	err := q.QueryRowContext(ctx, `SELECT `+toolCols+` FROM hx_tool_calls WHERE idem_key = ?`, key).
		Scan(&c.IdemKey, &c.RunID, &c.ExecKey, &c.Tool, &c.Risk, &c.Status, &c.ArgsRef, &c.ResultRef, &started, &ended)
	if errors.Is(err, sql.ErrNoRows) {
		return c, store.ErrNotFound
	}
	if err != nil {
		return c, wrap(err)
	}
	if c.StartedAt, err = parseTime(started); err != nil {
		return c, err
	}
	if ended.Valid {
		t, err := parseTime(ended.String)
		if err != nil {
			return c, err
		}
		c.EndedAt = &t
	}
	return c, nil
}

func (r toolCalls) Begin(ctx context.Context, c store.ToolCall) (existing *store.ToolCall, err error) {
	err = r.s.write(ctx, func(q querier) error {
		old, err := getToolCall(ctx, q, c.IdemKey)
		if err == nil {
			existing = &old
			return nil
		}
		if !errors.Is(err, store.ErrNotFound) {
			return err
		}
		if c.Status == "" {
			c.Status = store.ToolStarted
		}
		_, err = q.ExecContext(ctx, `INSERT INTO hx_tool_calls (`+toolCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			c.IdemKey, c.RunID, c.ExecKey, c.Tool, c.Risk, c.Status, c.ArgsRef, c.ResultRef, fmtTime(c.StartedAt), nullTime(c.EndedAt))
		return wrap(err)
	})
	return existing, err
}

func (r toolCalls) Finish(ctx context.Context, idemKey, status, resultRef string, endedAt time.Time) error {
	return r.s.write(ctx, func(q querier) error {
		c, err := getToolCall(ctx, q, idemKey)
		if err != nil {
			return err
		}
		if c.Status != store.ToolStarted || (status != store.ToolSucceeded && status != store.ToolFailed) {
			return fmt.Errorf("%w：工具调用不能从 %s 改成 %s", store.ErrConflict, c.Status, status)
		}
		_, err = q.ExecContext(ctx, `UPDATE hx_tool_calls SET status = ?, result_ref = ?, ended_at = ? WHERE idem_key = ?`,
			status, resultRef, fmtTime(endedAt), idemKey)
		return wrap(err)
	})
}

func (r toolCalls) Get(ctx context.Context, idemKey string) (c store.ToolCall, err error) {
	err = r.s.read(ctx, func(q querier) error { c, err = getToolCall(ctx, q, idemKey); return err })
	return c, err
}

type settings struct{ s *sqlStore }

func (r settings) Get(ctx context.Context, key string) (v json.RawMessage, ok bool, err error) {
	err = r.s.read(ctx, func(q querier) error {
		var b []byte
		err := q.QueryRowContext(ctx, `SELECT value FROM hx_settings WHERE key = ?`, key).Scan(&b)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil
		case err != nil:
			return wrap(err)
		}
		v, ok = json.RawMessage(b), true
		return nil
	})
	return v, ok, err
}

func (r settings) Put(ctx context.Context, key string, value json.RawMessage, at time.Time) error {
	if !json.Valid(value) {
		return fmt.Errorf("store: 设置 %s 的值不是合法的 JSON", key)
	}
	return r.s.write(ctx, func(q querier) error {
		_, err := q.ExecContext(ctx, `INSERT INTO hx_settings (key, value, updated_at) VALUES (?, ?, ?)
ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`, key, string(value), fmtTime(at))
		return wrap(err)
	})
}

func (r settings) All(ctx context.Context) (out map[string]json.RawMessage, err error) {
	err = r.s.read(ctx, func(q querier) error {
		rows, err := q.QueryContext(ctx, `SELECT key, value FROM hx_settings ORDER BY key`)
		if err != nil {
			return wrap(err)
		}
		defer func() { _ = rows.Close() }()
		out = map[string]json.RawMessage{}
		for rows.Next() {
			var k string
			var b []byte
			if err := rows.Scan(&k, &b); err != nil {
				return wrap(err)
			}
			out[k] = json.RawMessage(b)
		}
		return wrap(rows.Err())
	})
	return out, err
}
