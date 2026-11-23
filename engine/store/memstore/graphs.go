package memstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mio-cc/anywhere-door/engine/store"
	"github.com/mio-cc/anywhere-door/gen"
)

type graphs struct{ s *memStore }

func (d *data) meta(id string) (gen.GraphMeta, error) {
	b, ok := d.graphs[id]
	if !ok {
		return gen.GraphMeta{}, store.ErrNotFound
	}
	return dec[gen.GraphMeta](b)
}

func (d *data) putMeta(m gen.GraphMeta) error {
	b, err := enc(m)
	if err == nil {
		d.graphs[string(m.ID)] = b
	}
	return err
}

// branch 取分支；不存在或已删除：ErrNotFound。
func (d *data) branch(gid, name string) (gen.BranchMeta, error) {
	b, ok := d.branches[key(gid, name)]
	if !ok {
		return gen.BranchMeta{}, store.ErrNotFound
	}
	br, err := dec[gen.BranchMeta](b)
	if err == nil && br.DeletedAt != nil {
		return br, store.ErrNotFound
	}
	return br, err
}

func (d *data) putBranch(gid string, br gen.BranchMeta) error {
	b, err := enc(br)
	if err == nil {
		d.branches[key(gid, string(br.Name))] = b
	}
	return err
}

// opRange 取 after < version ≤ upTo 的 Op，升序。
func (d *data) opRange(gid, branch string, after, upTo int64) ([]gen.OpRecord, error) {
	pre := prefix(gid, branch)
	var keys []string
	for k := range d.ops {
		if strings.HasPrefix(k, pre) {
			if v := lastNum(k); v > after && v <= upTo {
				keys = append(keys, k)
			}
		}
	}
	sort.Strings(keys)
	out := []gen.OpRecord{}
	for _, k := range keys {
		r, err := dec[gen.OpRecord](d.ops[k])
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// fullDoc 取分支上版本 ≤ v 的最近一份全文。
func (d *data) fullDoc(gid, branch string, v int64) (int64, *gen.Graph, bool, error) {
	pre := prefix(gid, branch)
	best := int64(-1)
	for k, row := range d.versions {
		if row.Doc != nil && strings.HasPrefix(k, pre) {
			if n := lastNum(k); n <= v && n > best {
				best = n
			}
		}
	}
	if best < 0 {
		return 0, nil, false, nil
	}
	g, err := dec[gen.Graph](d.versions[key(gid, branch, num(best))].Doc)
	return best, &g, err == nil, err
}

func (r graphs) Create(ctx context.Context, meta gen.GraphMeta, doc *gen.Graph, by gen.Actor) error {
	return r.s.write(func(d *data) error {
		id := string(meta.ID)
		if _, ok := d.graphs[id]; ok {
			return store.ErrConflict
		}
		meta.Version = doc.Version
		if err := d.putMeta(meta); err != nil {
			return err
		}
		b := by
		if err := d.putBranch(id, gen.BranchMeta{Name: "main", Head: doc.Version, CreatedBy: &b, CreatedAt: meta.CreatedAt}); err != nil {
			return err
		}
		full, err := enc(doc)
		if err != nil {
			return err
		}
		d.versions[key(id, "main", num(doc.Version))] = versionRow{Doc: full}
		return nil
	})
}

func (r graphs) Meta(ctx context.Context, id string) (m gen.GraphMeta, err error) {
	err = r.s.read(func(d *data) error { m, err = d.meta(id); return err })
	return m, err
}

func (r graphs) List(ctx context.Context, q store.GraphQuery) (p store.Page[gen.GraphMeta], err error) {
	err = r.s.read(func(d *data) error {
		var items []gen.GraphMeta
		for id := range d.graphs {
			m, err := d.meta(id)
			if err != nil {
				return err
			}
			if m.DeletedAt == nil {
				items = append(items, m)
			}
		}
		at := func(m gen.GraphMeta) time.Time { return m.UpdatedAt.Time }
		idOf := func(m gen.GraphMeta) string { return string(m.ID) }
		sortKeyed(items, at, idOf, true)
		p, err = page(items, at, idOf, q.Cursor, q.Limit)
		return err
	})
	return p, err
}

func (r graphs) Commit(ctx context.Context, c store.Commit) error {
	return r.s.write(func(d *data) error {
		m, err := d.meta(c.GraphID)
		if err != nil {
			return err
		}
		br, err := d.branch(c.GraphID, c.Branch)
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
			if _, dup := d.opKeys[oid]; dup || seen[oid] {
				return fmt.Errorf("%w：Op id %s 重复", store.ErrConflict, oid)
			}
			seen[oid] = true
		}
		for _, rec := range c.Ops {
			b, err := enc(rec)
			if err != nil {
				return err
			}
			k := key(c.GraphID, c.Branch, num(rec.Version))
			d.ops[k] = b
			d.opKeys[string(*rec.Op.ID)] = k
		}
		last := c.Ops[len(c.Ops)-1].Version
		row := versionRow{Hash: c.Hash}
		if c.Doc != nil {
			if row.Doc, err = enc(c.Doc); err != nil {
				return err
			}
		}
		d.versions[key(c.GraphID, c.Branch, num(last))] = row
		br.Head = last
		if err := d.putBranch(c.GraphID, br); err != nil {
			return err
		}
		if c.Branch == "main" {
			m.Version, m.Title, m.Stats = last, c.Title, c.Stats
		}
		m.UpdatedAt = ts(c.At)
		return d.putMeta(m)
	})
}

func (r graphs) Base(ctx context.Context, id, branch string, version int64) (snap store.Snapshot, err error) {
	err = r.s.read(func(d *data) error {
		if _, err := d.meta(id); err != nil {
			return err
		}
		br, err := d.branch(id, branch)
		if err != nil {
			return err
		}
		if version > br.Head || version < br.BaseVersion {
			return store.ErrNotFound
		}
		fv, doc, ok, err := d.fullDoc(id, branch, version)
		if err != nil {
			return err
		}
		if ok {
			ops, err := d.opRange(id, branch, fv, version)
			snap = store.Snapshot{Version: fv, Doc: doc, Ops: ops}
			return err
		}
		if branch == "main" {
			return fmt.Errorf("store: 图 %s 在版本 %d 以前没有全文", id, version)
		}
		fv, doc, ok, err = d.fullDoc(id, "main", br.BaseVersion)
		if err != nil || !ok {
			return fmt.Errorf("store: 图 %s 的 main 在版本 %d 以前没有全文（%v）", id, br.BaseVersion, err)
		}
		mainOps, err := d.opRange(id, "main", fv, br.BaseVersion)
		if err != nil {
			return err
		}
		own, err := d.opRange(id, branch, br.BaseVersion, version)
		snap = store.Snapshot{Version: fv, Doc: doc, Ops: append(mainOps, own...)}
		return err
	})
	return snap, err
}

func (r graphs) SoftDelete(ctx context.Context, id string, at time.Time) error {
	return r.s.write(func(d *data) error {
		m, err := d.meta(id)
		if err != nil {
			return err
		}
		if m.DeletedAt == nil {
			t := ts(at)
			m.DeletedAt = &t
		}
		return d.putMeta(m)
	})
}

func (r graphs) Branches(ctx context.Context, id string) (out []gen.BranchMeta, err error) {
	err = r.s.read(func(d *data) error {
		if _, err := d.meta(id); err != nil {
			return err
		}
		main, err := d.branch(id, "main")
		if err != nil {
			return err
		}
		pre := prefix(id)
		var names []string
		for k := range d.branches {
			if strings.HasPrefix(k, pre) {
				names = append(names, strings.TrimPrefix(k, pre))
			}
		}
		sort.Strings(names)
		out = []gen.BranchMeta{}
		for _, n := range names {
			br, err := d.branch(id, n)
			if errors.Is(err, store.ErrNotFound) {
				continue
			}
			if err != nil {
				return err
			}
			if n != "main" {
				br.Behind, br.Ahead = main.Head-br.BaseVersion, br.Head-br.BaseVersion
			}
			out = append(out, br)
		}
		return nil
	})
	return out, err
}

func (r graphs) CreateBranch(ctx context.Context, id string, b gen.BranchMeta) error {
	return r.s.write(func(d *data) error {
		if _, err := d.meta(id); err != nil {
			return err
		}
		if !store.IsDraftBranch(string(b.Name)) {
			return fmt.Errorf("%w：分支名 %q 不是 draft/…", store.ErrConflict, b.Name)
		}
		if _, ok := d.branches[key(id, string(b.Name))]; ok {
			return fmt.Errorf("%w：分支 %s 已存在", store.ErrConflict, b.Name)
		}
		main, err := d.branch(id, "main")
		if err != nil {
			return err
		}
		if b.BaseVersion < 0 || b.BaseVersion > main.Head {
			return fmt.Errorf("%w：分出点 %d 超过 main 的 head %d", store.ErrConflict, b.BaseVersion, main.Head)
		}
		b.Head, b.Behind, b.Ahead, b.DeletedAt = b.BaseVersion, 0, 0, nil
		return d.putBranch(id, b)
	})
}

func (r graphs) DeleteBranch(ctx context.Context, id, name string, at time.Time) error {
	return r.s.write(func(d *data) error {
		if name == "main" || !store.IsDraftBranch(name) {
			return fmt.Errorf("%w：不能删除分支 %q", store.ErrConflict, name)
		}
		br, err := d.branch(id, name)
		if err != nil {
			return err
		}
		t := ts(at)
		br.DeletedAt = &t
		return d.putBranch(id, br)
	})
}

type ops struct{ s *memStore }

func (r ops) Range(ctx context.Context, graphID, branch string, after, upTo int64) (out []gen.OpRecord, err error) {
	err = r.s.read(func(d *data) error { out, err = d.opRange(graphID, branch, after, upTo); return err })
	return out, err
}

func (d *data) op(opID string) (string, gen.OpRecord, error) {
	k, ok := d.opKeys[opID]
	if !ok {
		return "", gen.OpRecord{}, store.ErrNotFound
	}
	rec, err := dec[gen.OpRecord](d.ops[k])
	return k, rec, err
}

func (r ops) Get(ctx context.Context, opID string) (rec gen.OpRecord, err error) {
	err = r.s.read(func(d *data) error { _, rec, err = d.op(opID); return err })
	return rec, err
}

func (r ops) Coalesce(ctx context.Context, opID string, payload json.RawMessage) error {
	return r.s.write(func(d *data) error {
		k, rec, err := d.op(opID)
		if err != nil {
			return err
		}
		rec.Op.Payload = append(json.RawMessage{}, payload...)
		b, err := enc(rec)
		if err == nil {
			d.ops[k] = b
		}
		return err
	})
}

func undoKey(e store.UndoEntry) string { return key(e.GraphID, e.Branch, e.ActorID, num(e.Seq)) }

func (r ops) UndoPush(ctx context.Context, e store.UndoEntry) error {
	return r.s.write(func(d *data) error {
		k := undoKey(e)
		if _, ok := d.undo[k]; ok {
			return fmt.Errorf("%w：撤销栈第 %d 项已存在", store.ErrConflict, e.Seq)
		}
		d.undo[k] = e
		return nil
	})
}

func (r ops) UndoTop(ctx context.Context, graphID, branch, actorID string, undone bool) (top store.UndoEntry, ok bool, err error) {
	err = r.s.read(func(d *data) error {
		for _, e := range d.undo {
			if e.GraphID == graphID && e.Branch == branch && e.ActorID == actorID && e.Undone == undone && (!ok || e.Seq > top.Seq) {
				top, ok = e, true
			}
		}
		return nil
	})
	return top, ok, err
}

func (r ops) UndoMark(ctx context.Context, e store.UndoEntry, undone bool) error {
	return r.s.write(func(d *data) error {
		k := undoKey(e)
		cur, ok := d.undo[k]
		if !ok {
			return store.ErrNotFound
		}
		cur.Undone = undone
		d.undo[k] = cur
		pre := prefix(e.GraphID, e.Branch)
		for k2, b := range d.ops {
			if !strings.HasPrefix(k2, pre) {
				continue
			}
			rec, err := dec[gen.OpRecord](b)
			if err != nil {
				return err
			}
			if rec.Op.GroupID != nil && string(*rec.Op.GroupID) == cur.GroupID && rec.Undone != undone {
				rec.Undone = undone
				if d.ops[k2], err = enc(rec); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
