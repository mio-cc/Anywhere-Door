package memstore

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mio-cc/anywhere-door/engine/store"
	"github.com/mio-cc/anywhere-door/gen"
)

type runs struct{ s *memStore }

func (d *data) run(id string) (gen.RunMeta, error) {
	b, ok := d.runs[id]
	if !ok {
		return gen.RunMeta{}, store.ErrNotFound
	}
	return dec[gen.RunMeta](b)
}

func (d *data) putRun(m gen.RunMeta) error {
	b, err := enc(m)
	if err == nil {
		d.runs[string(m.ID)] = b
	}
	return err
}

// updateRun 读出、修改、写回一个运行。
func (r runs) updateRun(id string, fn func(m *gen.RunMeta)) error {
	return r.s.write(func(d *data) error {
		m, err := d.run(id)
		if err != nil {
			return err
		}
		fn(&m)
		return d.putRun(m)
	})
}

func (d *data) allRuns() ([]gen.RunMeta, error) {
	out := make([]gen.RunMeta, 0, len(d.runs))
	for id := range d.runs {
		m, err := d.run(id)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func runAt(m gen.RunMeta) time.Time { return m.CreatedAt.Time }
func runID(m gen.RunMeta) string    { return string(m.ID) }

func (r runs) Create(ctx context.Context, m gen.RunMeta) (gen.RunMeta, error) {
	err := r.s.write(func(d *data) error {
		if _, ok := d.runs[string(m.ID)]; ok {
			return fmt.Errorf("%w：运行 %s 已存在", store.ErrConflict, m.ID)
		}
		all, err := d.allRuns()
		if err != nil {
			return err
		}
		m.No, m.LastSeq = 1, 0
		for _, o := range all {
			if o.GraphID == m.GraphID && o.No >= m.No {
				m.No = o.No + 1
			}
		}
		return d.putRun(m)
	})
	if err != nil {
		return gen.RunMeta{}, err
	}
	return m, nil
}

func (r runs) Get(ctx context.Context, id string) (m gen.RunMeta, err error) {
	err = r.s.read(func(d *data) error { m, err = d.run(id); return err })
	return m, err
}

func (r runs) List(ctx context.Context, q store.RunQuery) (p store.Page[gen.RunMeta], err error) {
	err = r.s.read(func(d *data) error {
		all, err := d.allRuns()
		if err != nil {
			return err
		}
		var items []gen.RunMeta
		for _, m := range all {
			sid := ""
			if m.SessionID != nil {
				sid = string(*m.SessionID)
			}
			if (q.GraphID == "" || string(m.GraphID) == q.GraphID) && (q.SessionID == "" || sid == q.SessionID) &&
				(q.Kind == "" || m.Kind == q.Kind) && (q.Status == "" || m.Status == q.Status) {
				items = append(items, m)
			}
		}
		sortKeyed(items, runAt, runID, true)
		p, err = page(items, runAt, runID, q.Cursor, q.Limit)
		return err
	})
	return p, err
}

func (r runs) SetStatus(ctx context.Context, id string, s gen.RunStatus, endedAt *time.Time) error {
	return r.updateRun(id, func(m *gen.RunMeta) {
		m.Status = s
		if endedAt != nil {
			t := ts(*endedAt)
			m.EndedAt = &t
		}
	})
}

func (r runs) AddUsage(ctx context.Context, id string, u store.Usage) error {
	return r.updateRun(id, func(m *gen.RunMeta) {
		m.Usage.In += u.In
		m.Usage.Out += u.Out
		m.Usage.Cached += u.Cached
		m.CostMicros += u.CostMicros
	})
}

func (r runs) SetProgress(ctx context.Context, id string, headline *gen.RunMetaHeadline, steps gen.RunSteps) error {
	return r.updateRun(id, func(m *gen.RunMeta) {
		m.Headline, m.Steps = nil, steps
		if headline != nil {
			h := *headline
			m.Headline = &h
		}
	})
}

func (r runs) AcquireLease(ctx context.Context, id, owner string, now, until time.Time) (ok bool, err error) {
	err = r.s.write(func(d *data) error {
		if _, ok := d.runs[id]; !ok {
			return store.ErrNotFound
		}
		l := d.leases[id]
		if l.owner == "" || l.owner == owner || !l.until.After(now) {
			d.leases[id] = lease{owner: owner, until: until}
			ok = true
		}
		return nil
	})
	return ok, err
}

func (r runs) RenewLease(ctx context.Context, id, owner string, until time.Time) error {
	return r.s.write(func(d *data) error {
		if _, ok := d.runs[id]; !ok {
			return store.ErrNotFound
		}
		if d.leases[id].owner != owner {
			return store.ErrLeaseHeld
		}
		d.leases[id] = lease{owner: owner, until: until}
		return nil
	})
}

func (r runs) Unfinished(ctx context.Context) (out []gen.RunMeta, err error) {
	err = r.s.read(func(d *data) error {
		all, err := d.allRuns()
		if err != nil {
			return err
		}
		out = []gen.RunMeta{}
		for _, m := range all {
			if m.Status == "pending" || m.Status == "running" || m.Status == "paused" {
				out = append(out, m)
			}
		}
		sortKeyed(out, runAt, runID, false)
		return nil
	})
	return out, err
}

func (r runs) PutNodeStates(ctx context.Context, runID string, rows []gen.NodeStateRow) error {
	return r.s.write(func(d *data) error {
		if _, ok := d.runs[runID]; !ok {
			return store.ErrNotFound
		}
		for _, row := range rows {
			b, err := enc(row)
			if err != nil {
				return err
			}
			d.nodeStates[key(runID, row.ExecKey)] = b
		}
		return nil
	})
}

func (r runs) NodeStates(ctx context.Context, runID string) (out []gen.NodeStateRow, err error) {
	err = r.s.read(func(d *data) error {
		pre := prefix(runID)
		var keys []string
		for k := range d.nodeStates {
			if strings.HasPrefix(k, pre) {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		out = []gen.NodeStateRow{}
		for _, k := range keys {
			row, err := dec[gen.NodeStateRow](d.nodeStates[k])
			if err != nil {
				return err
			}
			out = append(out, row)
		}
		return nil
	})
	return out, err
}

func (r runs) PutView(ctx context.Context, runID string, seq int64, ref string) error {
	return r.s.write(func(d *data) error {
		if _, ok := d.runs[runID]; !ok {
			return store.ErrNotFound
		}
		d.views[key(runID, num(seq))] = ref
		return nil
	})
}

func (r runs) ViewBefore(ctx context.Context, runID string, seq int64) (viewSeq int64, ref string, ok bool, err error) {
	err = r.s.read(func(d *data) error {
		pre := prefix(runID)
		for k, v := range d.views {
			if n := lastNum(k); strings.HasPrefix(k, pre) && n <= seq && (!ok || n > viewSeq) {
				viewSeq, ref, ok = n, v, true
			}
		}
		return nil
	})
	return viewSeq, ref, ok, err
}

type events struct{ s *memStore }

func (r events) AppendRun(ctx context.Context, runID string, evs []gen.RunEvent) (last int64, err error) {
	err = r.s.write(func(d *data) error {
		m, err := d.run(runID)
		if err != nil {
			return err
		}
		list := d.runEvents[runID]
		for i := range evs {
			evs[i].Seq = m.LastSeq + 1 + int64(i)
			b, err := enc(evs[i])
			if err != nil {
				return err
			}
			list = append(list, b)
		}
		d.runEvents[runID] = list
		m.LastSeq += int64(len(evs))
		last = m.LastSeq
		return d.putRun(m)
	})
	return last, err
}

// window 取 seq 在 (after, upTo] 里、最多 limit 条（limit ≤ 0 不限）。
func window[T any](list [][]byte, after, upTo int64, limit int) ([]T, error) {
	lo, hi := max(after, 0), min(upTo, int64(len(list)))
	out := []T{}
	for i := lo; i < hi && (limit <= 0 || len(out) < limit); i++ {
		v, err := dec[T](list[i])
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func eventLimit(n int) int {
	if n <= 0 {
		return store.DefaultEventLimit
	}
	return n
}

const maxSeq = int64(1<<62 - 1)

func (r events) RunRange(ctx context.Context, runID string, after int64, limit int) (out []gen.RunEvent, err error) {
	err = r.s.read(func(d *data) error {
		out, err = window[gen.RunEvent](d.runEvents[runID], after, maxSeq, eventLimit(limit))
		return err
	})
	return out, err
}

func (r events) RunByNode(ctx context.Context, runID, nodeID string, after int64) (out []gen.RunEvent, err error) {
	err = r.s.read(func(d *data) error {
		all, err := window[gen.RunEvent](d.runEvents[runID], after, maxSeq, 0)
		out = []gen.RunEvent{}
		for _, e := range all {
			if e.Loc != nil && string(e.Loc.NodeID) == nodeID {
				out = append(out, e)
			}
		}
		return err
	})
	return out, err
}

func (r events) RunUntil(ctx context.Context, runID string, upTo int64) (out []gen.RunEvent, err error) {
	err = r.s.read(func(d *data) error {
		out, err = window[gen.RunEvent](d.runEvents[runID], 0, upTo, 0)
		return err
	})
	return out, err
}

func (r events) AppendSession(ctx context.Context, sessionID string, evs []gen.SessionEvent) (last int64, err error) {
	err = r.s.write(func(d *data) error {
		list := d.sessEvents[sessionID]
		for i := range evs {
			evs[i].Seq = int64(len(list)) + 1
			b, err := enc(evs[i])
			if err != nil {
				return err
			}
			list = append(list, b)
		}
		d.sessEvents[sessionID] = list
		last = int64(len(list))
		return nil
	})
	return last, err
}

func (r events) SessionRange(ctx context.Context, sessionID string, after int64, limit int) (out []gen.SessionEvent, err error) {
	err = r.s.read(func(d *data) error {
		out, err = window[gen.SessionEvent](d.sessEvents[sessionID], after, maxSeq, eventLimit(limit))
		return err
	})
	return out, err
}
