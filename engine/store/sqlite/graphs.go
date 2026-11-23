package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/mio-cc/anywhere-door/engine/store"
	"github.com/mio-cc/anywhere-door/gen"
)

type graphs struct{ s *sqlStore }

func getMeta(ctx context.Context, q querier, id string) (gen.GraphMeta, error) {
	return one[gen.GraphMeta](ctx, q, `SELECT doc FROM hx_graphs WHERE id = ?`, id)
}

func putMeta(ctx context.Context, q querier, m gen.GraphMeta) error {
	doc, err := enc(m)
	if err != nil {
		return err
	}
	var deleted any
	if m.DeletedAt != nil {
		deleted = fmtTime(m.DeletedAt.Time)
	}
	_, err = q.ExecContext(ctx, `UPDATE hx_graphs SET updated_at = ?, deleted_at = ?, doc = ? WHERE id = ?`,
		fmtTime(m.UpdatedAt.Time), deleted, doc, string(m.ID))
	return wrap(err)
}

// getBranch 取分支；不存在或已删除：ErrNotFound。
func getBranch(ctx context.Context, q querier, gid, name string) (gen.BranchMeta, error) {
	return one[gen.BranchMeta](ctx, q, `SELECT doc FROM hx_branches WHERE graph_id = ? AND name = ? AND deleted_at IS NULL`, gid, name)
}

func putBranch(ctx context.Context, q querier, gid string, b gen.BranchMeta, insert bool) error {
	doc, err := enc(b)
	if err != nil {
		return err
	}
	var deleted any
	if b.DeletedAt != nil {
		deleted = fmtTime(b.DeletedAt.Time)
	}
	if insert {
		_, err = q.ExecContext(ctx, `INSERT INTO hx_branches (graph_id, name, base_version, head_version, deleted_at, doc) VALUES (?, ?, ?, ?, ?, ?)`,
			gid, string(b.Name), b.BaseVersion, b.Head, deleted, doc)
	} else {
		_, err = q.ExecContext(ctx, `UPDATE hx_branches SET head_version = ?, deleted_at = ?, doc = ? WHERE graph_id = ? AND name = ?`,
			b.Head, deleted, doc, gid, string(b.Name))
	}
	return wrap(err)
}

func opRange(ctx context.Context, q querier, gid, branch string, after, upTo int64) ([]gen.OpRecord, error) {
	return many[gen.OpRecord](ctx, q, `SELECT doc FROM hx_ops WHERE graph_id = ? AND branch = ? AND version > ? AND version <= ? ORDER BY version`,
		gid, branch, after, upTo)
}

// fullDoc 取分支上版本 ≤ v 的最近一份全文。
func fullDoc(ctx context.Context, q querier, gid, branch string, v int64) (int64, *gen.Graph, bool, error) {
	var ver int64
	var b []byte
	err := q.QueryRowContext(ctx, `SELECT version, doc FROM hx_graph_versions WHERE graph_id = ? AND branch = ? AND version <= ? AND doc IS NOT NULL ORDER BY version DESC LIMIT 1`,
		gid, branch, v).Scan(&ver, &b)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil, false, nil
	}
	if err != nil {
		return 0, nil, false, wrap(err)
	}
	g, err := decode[gen.Graph](b)
	return ver, &g, err == nil, err
}

func (r graphs) Create(ctx context.Context, meta gen.GraphMeta, doc *gen.Graph, by gen.Actor) error {
	return r.s.write(ctx, func(q querier) error {
		meta.Version = doc.Version
		m, err := enc(meta)
		if err != nil {
			return err
		}
		if _, err := q.ExecContext(ctx, `INSERT INTO hx_graphs (id, updated_at, deleted_at, doc) VALUES (?, ?, NULL, ?)`,
			string(meta.ID), fmtTime(meta.UpdatedAt.Time), m); err != nil {
			return wrap(err)
		}
		b := by
		if err := putBranch(ctx, q, string(meta.ID), gen.BranchMeta{Name: "main", Head: doc.Version, CreatedBy: &b, CreatedAt: meta.CreatedAt}, true); err != nil {
			return err
		}
		full, err := enc(doc)
		if err != nil {
			return err
		}
		_, err = q.ExecContext(ctx, `INSERT INTO hx_graph_versions (graph_id, branch, version, hash, doc) VALUES (?, 'main', ?, '', ?)`,
			string(meta.ID), doc.Version, full)
		return wrap(err)
	})
}

func (r graphs) Meta(ctx context.Context, id string) (m gen.GraphMeta, err error) {
	err = r.s.read(ctx, func(q querier) error { m, err = getMeta(ctx, q, id); return err })
	return m, err
}

func (r graphs) List(ctx context.Context, qy store.GraphQuery) (p store.Page[gen.GraphMeta], err error) {
	limit := store.ClampLimit(qy.Limit)
	where, args, err := cursorWhere(qy.Cursor, "updated_at", "id")
	if err != nil {
		return p, err
	}
	err = r.s.read(ctx, func(q querier) error {
		items, err := many[gen.GraphMeta](ctx, q, `SELECT doc FROM hx_graphs WHERE deleted_at IS NULL`+where+` ORDER BY updated_at DESC, id DESC LIMIT ?`,
			append(args, limit+1)...)
		p = pageOf(items, limit, func(m gen.GraphMeta) time.Time { return m.UpdatedAt.Time }, func(m gen.GraphMeta) string { return string(m.ID) })
		return err
	})
	return p, err
}

func (r graphs) Commit(ctx context.Context, c store.Commit) error {
	return r.s.write(ctx, func(q querier) error {
		m, err := getMeta(ctx, q, c.GraphID)
		if err != nil {
			return err
		}
		br, err := getBranch(ctx, q, c.GraphID, c.Branch)
		if err != nil {
			return err
		}
		if len(c.Ops) == 0 {
			return fmt.Errorf("%w：没有 Op", store.ErrConflict)
		}
		seen := map[string]bool{}
		for i, rec := range c.Ops {
			if rec.Version != br.Head+1+int64(i) || string(rec.GraphID) != c.GraphID || string(rec.Branch) != c.Branch {
				return fmt.Errorf("%w：第 %d 条 Op 的版本是 %d，应当 %d", store.ErrConflict, i, rec.Version, br.Head+1+int64(i))
			}
			if rec.Op.ID == nil {
				return fmt.Errorf("%w：第 %d 条 Op 没有 id", store.ErrConflict, i)
			}
			oid := string(*rec.Op.ID)
			dup, err := exists(ctx, q, `SELECT 1 FROM hx_ops WHERE id = ?`, oid)
			if err != nil {
				return err
			}
			if dup || seen[oid] {
				return fmt.Errorf("%w：Op id %s 重复", store.ErrConflict, oid)
			}
			seen[oid] = true
		}
		for _, rec := range c.Ops {
			doc, err := enc(rec)
			if err != nil {
				return err
			}
			var group any
			if rec.Op.GroupID != nil {
				group = string(*rec.Op.GroupID)
			}
			if _, err := q.ExecContext(ctx, `INSERT INTO hx_ops (graph_id, branch, version, id, group_id, undone, doc) VALUES (?, ?, ?, ?, ?, ?, ?)`,
				c.GraphID, c.Branch, rec.Version, string(*rec.Op.ID), group, boolInt(rec.Undone), doc); err != nil {
				return wrap(err)
			}
		}
		last := c.Ops[len(c.Ops)-1].Version
		var full any
		if c.Doc != nil {
			if full, err = enc(c.Doc); err != nil {
				return err
			}
		}
		if _, err := q.ExecContext(ctx, `INSERT INTO hx_graph_versions (graph_id, branch, version, hash, doc) VALUES (?, ?, ?, ?, ?)`,
			c.GraphID, c.Branch, last, c.Hash, full); err != nil {
			return wrap(err)
		}
		br.Head = last
		if err := putBranch(ctx, q, c.GraphID, br, false); err != nil {
			return err
		}
		if c.Branch == "main" {
			m.Version, m.Title, m.Stats = last, c.Title, c.Stats
		}
		m.UpdatedAt = gen.Timestamp{Time: c.At.UTC()}
		return putMeta(ctx, q, m)
	})
}

func (r graphs) Base(ctx context.Context, id, branch string, version int64) (snap store.Snapshot, err error) {
	err = r.s.read(ctx, func(q querier) error {
		if _, err := getMeta(ctx, q, id); err != nil {
			return err
		}
		br, err := getBranch(ctx, q, id, branch)
		if err != nil {
			return err
		}
		if version > br.Head || version < br.BaseVersion {
			return store.ErrNotFound
		}
		fv, doc, ok, err := fullDoc(ctx, q, id, branch, version)
		if err != nil {
			return err
		}
		if ok {
			ops, err := opRange(ctx, q, id, branch, fv, version)
			snap = store.Snapshot{Version: fv, Doc: doc, Ops: ops}
			return err
		}
		if branch == "main" {
			return fmt.Errorf("store: 图 %s 在版本 %d 以前没有全文", id, version)
		}
		fv, doc, ok, err = fullDoc(ctx, q, id, "main", br.BaseVersion)
		if err != nil || !ok {
			return fmt.Errorf("store: 图 %s 的 main 在版本 %d 以前没有全文（%v）", id, br.BaseVersion, err)
		}
		mainOps, err := opRange(ctx, q, id, "main", fv, br.BaseVersion)
		if err != nil {
			return err
		}
		own, err := opRange(ctx, q, id, branch, br.BaseVersion, version)
		snap = store.Snapshot{Version: fv, Doc: doc, Ops: append(mainOps, own...)}
		return err
	})
	return snap, err
}

func (r graphs) SoftDelete(ctx context.Context, id string, at time.Time) error {
	return r.s.write(ctx, func(q querier) error {
		m, err := getMeta(ctx, q, id)
		if err != nil {
			return err
		}
		if m.DeletedAt == nil {
			t := gen.Timestamp{Time: at.UTC()}
			m.DeletedAt = &t
		}
		return putMeta(ctx, q, m)
	})
}

func (r graphs) Branches(ctx context.Context, id string) (out []gen.BranchMeta, err error) {
	err = r.s.read(ctx, func(q querier) error {
		if _, err := getMeta(ctx, q, id); err != nil {
			return err
		}
		main, err := getBranch(ctx, q, id, "main")
		if err != nil {
			return err
		}
		out, err = many[gen.BranchMeta](ctx, q, `SELECT doc FROM hx_branches WHERE graph_id = ? AND deleted_at IS NULL ORDER BY name`, id)
		for i := range out {
			if out[i].Name != "main" {
				out[i].Behind, out[i].Ahead = main.Head-out[i].BaseVersion, out[i].Head-out[i].BaseVersion
			}
		}
		return err
	})
	return out, err
}

func (r graphs) CreateBranch(ctx context.Context, id string, b gen.BranchMeta) error {
	return r.s.write(ctx, func(q querier) error {
		if _, err := getMeta(ctx, q, id); err != nil {
			return err
		}
		if !store.IsDraftBranch(string(b.Name)) {
			return fmt.Errorf("%w：分支名 %q 不是 draft/…", store.ErrConflict, b.Name)
		}
		dup, err := exists(ctx, q, `SELECT 1 FROM hx_branches WHERE graph_id = ? AND name = ?`, id, string(b.Name))
		if err != nil {
			return err
		}
		if dup {
			return fmt.Errorf("%w：分支 %s 已存在", store.ErrConflict, b.Name)
		}
		main, err := getBranch(ctx, q, id, "main")
		if err != nil {
			return err
		}
		if b.BaseVersion < 0 || b.BaseVersion > main.Head {
			return fmt.Errorf("%w：分出点 %d 超过 main 的 head %d", store.ErrConflict, b.BaseVersion, main.Head)
		}
		b.Head, b.Behind, b.Ahead, b.DeletedAt = b.BaseVersion, 0, 0, nil
		return putBranch(ctx, q, id, b, true)
	})
}

func (r graphs) DeleteBranch(ctx context.Context, id, name string, at time.Time) error {
	return r.s.write(ctx, func(q querier) error {
		if name == "main" || !store.IsDraftBranch(name) {
			return fmt.Errorf("%w：不能删除分支 %q", store.ErrConflict, name)
		}
		br, err := getBranch(ctx, q, id, name)
		if err != nil {
			return err
		}
		t := gen.Timestamp{Time: at.UTC()}
		br.DeletedAt = &t
		return putBranch(ctx, q, id, br, false)
	})
}

type ops struct{ s *sqlStore }

func (r ops) Range(ctx context.Context, graphID, branch string, after, upTo int64) (out []gen.OpRecord, err error) {
	err = r.s.read(ctx, func(q querier) error { out, err = opRange(ctx, q, graphID, branch, after, upTo); return err })
	return out, err
}

func (r ops) Get(ctx context.Context, opID string) (rec gen.OpRecord, err error) {
	err = r.s.read(ctx, func(q querier) error {
		rec, err = one[gen.OpRecord](ctx, q, `SELECT doc FROM hx_ops WHERE id = ?`, opID)
		return err
	})
	return rec, err
}

func putOp(ctx context.Context, q querier, rec gen.OpRecord) error {
	doc, err := enc(rec)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx, `UPDATE hx_ops SET undone = ?, doc = ? WHERE id = ?`, boolInt(rec.Undone), doc, string(*rec.Op.ID))
	return wrap(err)
}

func (r ops) Coalesce(ctx context.Context, opID string, payload json.RawMessage) error {
	return r.s.write(ctx, func(q querier) error {
		rec, err := one[gen.OpRecord](ctx, q, `SELECT doc FROM hx_ops WHERE id = ?`, opID)
		if err != nil {
			return err
		}
		rec.Op.Payload = append(json.RawMessage{}, payload...)
		return putOp(ctx, q, rec)
	})
}

func (r ops) UndoPush(ctx context.Context, e store.UndoEntry) error {
	return r.s.write(ctx, func(q querier) error {
		_, err := q.ExecContext(ctx, `INSERT INTO hx_undo_stack (graph_id, branch, actor_id, seq, group_id, undone) VALUES (?, ?, ?, ?, ?, ?)`,
			e.GraphID, e.Branch, e.ActorID, e.Seq, e.GroupID, boolInt(e.Undone))
		return wrap(err)
	})
}

func (r ops) UndoTop(ctx context.Context, graphID, branch, actorID string, undone bool) (top store.UndoEntry, ok bool, err error) {
	err = r.s.read(ctx, func(q querier) error {
		top = store.UndoEntry{GraphID: graphID, Branch: branch, ActorID: actorID, Undone: undone}
		err := q.QueryRowContext(ctx, `SELECT seq, group_id FROM hx_undo_stack WHERE graph_id = ? AND branch = ? AND actor_id = ? AND undone = ? ORDER BY seq DESC LIMIT 1`,
			graphID, branch, actorID, boolInt(undone)).Scan(&top.Seq, &top.GroupID)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			top = store.UndoEntry{}
			return nil
		case err != nil:
			return wrap(err)
		}
		ok = true
		return nil
	})
	return top, ok, err
}

func (r ops) UndoMark(ctx context.Context, e store.UndoEntry, undone bool) error {
	return r.s.write(ctx, func(q querier) error {
		var group string
		err := q.QueryRowContext(ctx, `SELECT group_id FROM hx_undo_stack WHERE graph_id = ? AND branch = ? AND actor_id = ? AND seq = ?`,
			e.GraphID, e.Branch, e.ActorID, e.Seq).Scan(&group)
		if errors.Is(err, sql.ErrNoRows) {
			return store.ErrNotFound
		}
		if err != nil {
			return wrap(err)
		}
		if _, err := q.ExecContext(ctx, `UPDATE hx_undo_stack SET undone = ? WHERE graph_id = ? AND branch = ? AND actor_id = ? AND seq = ?`,
			boolInt(undone), e.GraphID, e.Branch, e.ActorID, e.Seq); err != nil {
			return wrap(err)
		}
		recs, err := many[gen.OpRecord](ctx, q, `SELECT doc FROM hx_ops WHERE graph_id = ? AND branch = ? AND group_id = ?`, e.GraphID, e.Branch, group)
		if err != nil {
			return err
		}
		for _, rec := range recs {
			rec.Undone = undone
			if err := putOp(ctx, q, rec); err != nil {
				return err
			}
		}
		return nil
	})
}
