package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mio-cc/anywhere-door/engine/store"
	"github.com/mio-cc/anywhere-door/gen"
)

type runs struct{ s *sqlStore }

func getRun(ctx context.Context, q querier, id string) (gen.RunMeta, error) {
	return one[gen.RunMeta](ctx, q, `SELECT doc FROM hx_runs WHERE id = ?`, id)
}

// putRun 写回 RunMeta 和它冗余出来的列。
func putRun(ctx context.Context, q querier, m gen.RunMeta) error {
	doc, err := enc(m)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx, `UPDATE hx_runs SET status = ?, last_seq = ?, doc = ? WHERE id = ?`, string(m.Status), m.LastSeq, doc, string(m.ID))
	return wrap(err)
}

func (r runs) update(ctx context.Context, id string, fn func(m *gen.RunMeta)) error {
	return r.s.write(ctx, func(q querier) error {
		m, err := getRun(ctx, q, id)
		if err != nil {
			return err
		}
		fn(&m)
		return putRun(ctx, q, m)
	})
}

func runExists(ctx context.Context, q querier, id string) error {
	ok, err := exists(ctx, q, `SELECT 1 FROM hx_runs WHERE id = ?`, id)
	if err == nil && !ok {
		return store.ErrNotFound
	}
	return err
}

func (r runs) Create(ctx context.Context, m gen.RunMeta) (gen.RunMeta, error) {
	err := r.s.write(ctx, func(q querier) error {
		dup, err := exists(ctx, q, `SELECT 1 FROM hx_runs WHERE id = ?`, string(m.ID))
		if err != nil {
			return err
		}
		if dup {
			return fmt.Errorf("%w：运行 %s 已存在", store.ErrConflict, m.ID)
		}
		if err := q.QueryRowContext(ctx, `SELECT coalesce(max(no), 0) + 1 FROM hx_runs WHERE graph_id = ?`, string(m.GraphID)).Scan(&m.No); err != nil {
			return wrap(err)
		}
		m.LastSeq = 0
		doc, err := enc(m)
		if err != nil {
			return err
		}
		var sid any
		if m.SessionID != nil {
			sid = string(*m.SessionID)
		}
		_, err = q.ExecContext(ctx, `INSERT INTO hx_runs (id, graph_id, session_id, kind, status, no, created_at, last_seq, doc) VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?)`,
			string(m.ID), string(m.GraphID), sid, string(m.Kind), string(m.Status), m.No, fmtTime(m.CreatedAt.Time), doc)
		return wrap(err)
	})
	if err != nil {
		return gen.RunMeta{}, err
	}
	return m, nil
}

func (r runs) Get(ctx context.Context, id string) (m gen.RunMeta, err error) {
	err = r.s.read(ctx, func(q querier) error { m, err = getRun(ctx, q, id); return err })
	return m, err
}

func (r runs) List(ctx context.Context, qy store.RunQuery) (p store.Page[gen.RunMeta], err error) {
	limit := store.ClampLimit(qy.Limit)
	where, args, err := cursorWhere(qy.Cursor, "created_at", "id")
	if err != nil {
		return p, err
	}
	for _, f := range []struct{ col, v string }{{"graph_id", qy.GraphID}, {"session_id", qy.SessionID}, {"kind", qy.Kind}, {"status", string(qy.Status)}} {
		if f.v != "" {
			where += " AND " + f.col + " = ?"
			args = append(args, f.v)
		}
	}
	err = r.s.read(ctx, func(q querier) error {
		items, err := many[gen.RunMeta](ctx, q, `SELECT doc FROM hx_runs WHERE 1 = 1`+where+` ORDER BY created_at DESC, id DESC LIMIT ?`, append(args, limit+1)...)
		p = pageOf(items, limit, func(m gen.RunMeta) time.Time { return m.CreatedAt.Time }, func(m gen.RunMeta) string { return string(m.ID) })
		return err
	})
	return p, err
}

func (r runs) SetStatus(ctx context.Context, id string, s gen.RunStatus, endedAt *time.Time) error {
	return r.update(ctx, id, func(m *gen.RunMeta) {
		m.Status = s
		if endedAt != nil {
			t := gen.Timestamp{Time: endedAt.UTC()}
			m.EndedAt = &t
		}
	})
}

func (r runs) AddUsage(ctx context.Context, id string, u store.Usage) error {
	return r.update(ctx, id, func(m *gen.RunMeta) {
		m.Usage.In += u.In
		m.Usage.Out += u.Out
		m.Usage.Cached += u.Cached
		m.CostMicros += u.CostMicros
	})
}

func (r runs) SetProgress(ctx context.Context, id string, headline *gen.RunMetaHeadline, steps gen.RunSteps) error {
	return r.update(ctx, id, func(m *gen.RunMeta) { m.Headline, m.Steps = headline, steps })
}

// lease 读租约；运行不存在：ErrNotFound。
func lease(ctx context.Context, q querier, id string) (owner string, until time.Time, err error) {
	var o, u sql.NullString
	err = q.QueryRowContext(ctx, `SELECT lease_owner, lease_until FROM hx_runs WHERE id = ?`, id).Scan(&o, &u)
	if errors.Is(err, sql.ErrNoRows) {
		return "", until, store.ErrNotFound
	}
	if err != nil {
		return "", until, wrap(err)
	}
	if u.Valid {
		if until, err = parseTime(u.String); err != nil {
			return "", until, err
		}
	}
	return o.String, until, nil
}

func setLease(ctx context.Context, q querier, id, owner string, until time.Time) error {
	_, err := q.ExecContext(ctx, `UPDATE hx_runs SET lease_owner = ?, lease_until = ? WHERE id = ?`, owner, fmtTime(until), id)
	return wrap(err)
}

func (r runs) AcquireLease(ctx context.Context, id, owner string, now, until time.Time) (ok bool, err error) {
	err = r.s.write(ctx, func(q querier) error {
		cur, curUntil, err := lease(ctx, q, id)
		if err != nil {
			return err
		}
		if cur != "" && cur != owner && curUntil.After(now) {
			return nil
		}
		ok = true
		return setLease(ctx, q, id, owner, until)
	})
	return ok, err
}

func (r runs) RenewLease(ctx context.Context, id, owner string, until time.Time) error {
	return r.s.write(ctx, func(q querier) error {
		cur, _, err := lease(ctx, q, id)
		if err != nil {
			return err
		}
		if cur != owner {
			return store.ErrLeaseHeld
		}
		return setLease(ctx, q, id, owner, until)
	})
}

func (r runs) Unfinished(ctx context.Context) (out []gen.RunMeta, err error) {
	err = r.s.read(ctx, func(q querier) error {
		out, err = many[gen.RunMeta](ctx, q, `SELECT doc FROM hx_runs WHERE status IN ('pending', 'running', 'paused') ORDER BY created_at, id`)
		return err
	})
	return out, err
}

func (r runs) PutNodeStates(ctx context.Context, runID string, rows []gen.NodeStateRow) error {
	return r.s.write(ctx, func(q querier) error {
		if err := runExists(ctx, q, runID); err != nil {
			return err
		}
		for _, row := range rows {
			doc, err := enc(row)
			if err != nil {
				return err
			}
			if _, err := q.ExecContext(ctx, `INSERT INTO hx_run_node_state (run_id, exec_key, doc) VALUES (?, ?, ?) ON CONFLICT (run_id, exec_key) DO UPDATE SET doc = excluded.doc`,
				runID, row.ExecKey, doc); err != nil {
				return wrap(err)
			}
		}
		return nil
	})
}

func (r runs) NodeStates(ctx context.Context, runID string) (out []gen.NodeStateRow, err error) {
	err = r.s.read(ctx, func(q querier) error {
		out, err = many[gen.NodeStateRow](ctx, q, `SELECT doc FROM hx_run_node_state WHERE run_id = ? ORDER BY exec_key`, runID)
		return err
	})
	return out, err
}

func (r runs) PutView(ctx context.Context, runID string, seq int64, ref string) error {
	return r.s.write(ctx, func(q querier) error {
		if err := runExists(ctx, q, runID); err != nil {
			return err
		}
		_, err := q.ExecContext(ctx, `INSERT INTO hx_run_views (run_id, seq, view_ref) VALUES (?, ?, ?) ON CONFLICT (run_id, seq) DO UPDATE SET view_ref = excluded.view_ref`,
			runID, seq, ref)
		return wrap(err)
	})
}

func (r runs) ViewBefore(ctx context.Context, runID string, seq int64) (viewSeq int64, ref string, ok bool, err error) {
	err = r.s.read(ctx, func(q querier) error {
		err := q.QueryRowContext(ctx, `SELECT seq, view_ref FROM hx_run_views WHERE run_id = ? AND seq <= ? ORDER BY seq DESC LIMIT 1`, runID, seq).Scan(&viewSeq, &ref)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil
		case err != nil:
			return wrap(err)
		}
		ok = true
		return nil
	})
	return viewSeq, ref, ok, err
}

type events struct{ s *sqlStore }

func (r events) AppendRun(ctx context.Context, runID string, evs []gen.RunEvent) (last int64, err error) {
	err = r.s.write(ctx, func(q querier) error {
		m, err := getRun(ctx, q, runID)
		if err != nil {
			return err
		}
		for i := range evs {
			evs[i].Seq = m.LastSeq + 1 + int64(i)
			doc, err := enc(evs[i])
			if err != nil {
				return err
			}
			var node any
			if evs[i].Loc != nil {
				node = string(evs[i].Loc.NodeID)
			}
			if _, err := q.ExecContext(ctx, `INSERT INTO hx_run_events (run_id, seq, kind, node_id, doc) VALUES (?, ?, ?, ?, ?)`,
				runID, evs[i].Seq, string(evs[i].Kind), node, doc); err != nil {
				return wrap(err)
			}
		}
		m.LastSeq += int64(len(evs))
		last = m.LastSeq
		return putRun(ctx, q, m)
	})
	return last, err
}

func eventLimit(n int) int {
	if n <= 0 {
		return store.DefaultEventLimit
	}
	return n
}

func (r events) RunRange(ctx context.Context, runID string, after int64, limit int) (out []gen.RunEvent, err error) {
	err = r.s.read(ctx, func(q querier) error {
		out, err = many[gen.RunEvent](ctx, q, `SELECT doc FROM hx_run_events WHERE run_id = ? AND seq > ? ORDER BY seq LIMIT ?`, runID, after, eventLimit(limit))
		return err
	})
	return out, err
}

func (r events) RunByNode(ctx context.Context, runID, nodeID string, after int64) (out []gen.RunEvent, err error) {
	err = r.s.read(ctx, func(q querier) error {
		out, err = many[gen.RunEvent](ctx, q, `SELECT doc FROM hx_run_events WHERE run_id = ? AND node_id = ? AND seq > ? ORDER BY seq`, runID, nodeID, after)
		return err
	})
	return out, err
}

func (r events) RunUntil(ctx context.Context, runID string, upTo int64) (out []gen.RunEvent, err error) {
	err = r.s.read(ctx, func(q querier) error {
		out, err = many[gen.RunEvent](ctx, q, `SELECT doc FROM hx_run_events WHERE run_id = ? AND seq <= ? ORDER BY seq`, runID, upTo)
		return err
	})
	return out, err
}

func (r events) AppendSession(ctx context.Context, sessionID string, evs []gen.SessionEvent) (last int64, err error) {
	err = r.s.write(ctx, func(q querier) error {
		if err := q.QueryRowContext(ctx, `SELECT coalesce(max(seq), 0) FROM hx_session_events WHERE session_id = ?`, sessionID).Scan(&last); err != nil {
			return wrap(err)
		}
		for i := range evs {
			last++
			evs[i].Seq = last
			doc, err := enc(evs[i])
			if err != nil {
				return err
			}
			if _, err := q.ExecContext(ctx, `INSERT INTO hx_session_events (session_id, seq, kind, doc) VALUES (?, ?, ?, ?)`,
				sessionID, last, string(evs[i].Kind), doc); err != nil {
				return wrap(err)
			}
		}
		return nil
	})
	return last, err
}

func (r events) SessionRange(ctx context.Context, sessionID string, after int64, limit int) (out []gen.SessionEvent, err error) {
	err = r.s.read(ctx, func(q querier) error {
		out, err = many[gen.SessionEvent](ctx, q, `SELECT doc FROM hx_session_events WHERE session_id = ? AND seq > ? ORDER BY seq LIMIT ?`, sessionID, after, eventLimit(limit))
		return err
	})
	return out, err
}
