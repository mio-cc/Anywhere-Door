// Package engine 是内核的组装入口（设计文档 E.01 §2）。本片只有 Op 引擎用的存储适配器（E.05-R4）。
package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mio-cc/anywhere-door/engine/op"
	"github.com/mio-cc/anywhere-door/engine/store"
	"github.com/mio-cc/anywhere-door/gen"
)

// NewOpStore 把 store.Store 包成 op.Repo：方法一一转给存储，哨兵错误换成 op.ErrRepoNotFound、op.ErrRepoConflict。
func NewOpStore(s store.Store) op.Repo { return opStore{s: s} }

type opStore struct{ s store.Store }

func wrapStore(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, store.ErrNotFound):
		return fmt.Errorf("%w：%w", op.ErrRepoNotFound, err)
	case errors.Is(err, store.ErrConflict):
		return fmt.Errorf("%w：%w", op.ErrRepoConflict, err)
	}
	return err
}

func (o opStore) Tx(ctx context.Context, fn func(op.Repo) error) error {
	return wrapStore(o.s.Tx(ctx, func(s store.Store) error { return fn(opStore{s: s}) }))
}

func (o opStore) Head(ctx context.Context, graphID, branch string) (int64, error) {
	bs, err := o.s.Graphs().Branches(ctx, graphID)
	if err != nil {
		return 0, wrapStore(err)
	}
	for _, b := range bs {
		if string(b.Name) == branch {
			return b.Head, nil
		}
	}
	return 0, fmt.Errorf("%w：图 %s 没有分支 %s", op.ErrRepoNotFound, graphID, branch)
}

func (o opStore) Base(ctx context.Context, graphID, branch string, version int64) (op.Snapshot, error) {
	s, err := o.s.Graphs().Base(ctx, graphID, branch, version)
	if err != nil {
		return op.Snapshot{}, wrapStore(err)
	}
	return op.Snapshot{Version: s.Version, Doc: s.Doc, Ops: s.Ops}, nil
}

func (o opStore) Range(ctx context.Context, graphID, branch string, after, upTo int64) ([]gen.OpRecord, error) {
	recs, err := o.s.Ops().Range(ctx, graphID, branch, after, upTo)
	return recs, wrapStore(err)
}

func (o opStore) Op(ctx context.Context, opID string) (gen.OpRecord, error) {
	r, err := o.s.Ops().Get(ctx, opID)
	return r, wrapStore(err)
}

func (o opStore) Commit(ctx context.Context, c op.Commit) error {
	return wrapStore(o.s.Graphs().Commit(ctx, store.Commit{
		GraphID: c.GraphID, Branch: c.Branch, Ops: c.Ops, Doc: c.Doc,
		Hash: c.Hash, Title: c.Title, Stats: c.Stats, At: c.At,
	}))
}

func (o opStore) Coalesce(ctx context.Context, opID string, payload json.RawMessage) error {
	return wrapStore(o.s.Ops().Coalesce(ctx, opID, payload))
}

func toStore(e op.UndoEntry) store.UndoEntry {
	return store.UndoEntry{GraphID: e.GraphID, Branch: e.Branch, ActorID: e.ActorID, Seq: e.Seq, GroupID: e.GroupID, Undone: e.Undone}
}

func (o opStore) UndoPush(ctx context.Context, e op.UndoEntry) error {
	return wrapStore(o.s.Ops().UndoPush(ctx, toStore(e)))
}

func (o opStore) UndoTop(ctx context.Context, graphID, branch, actorID string, undone bool) (op.UndoEntry, bool, error) {
	e, ok, err := o.s.Ops().UndoTop(ctx, graphID, branch, actorID, undone)
	return op.UndoEntry{GraphID: e.GraphID, Branch: e.Branch, ActorID: e.ActorID, Seq: e.Seq, GroupID: e.GroupID, Undone: e.Undone}, ok, wrapStore(err)
}

func (o opStore) UndoMark(ctx context.Context, e op.UndoEntry, undone bool) error {
	return wrapStore(o.s.Ops().UndoMark(ctx, toStore(e), undone))
}
