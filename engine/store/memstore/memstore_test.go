package memstore_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mio-cc/anywhere-door/engine/store"
	"github.com/mio-cc/anywhere-door/engine/store/memstore"
	"github.com/mio-cc/anywhere-door/engine/store/storetest"
	"github.com/mio-cc/anywhere-door/gen"
)

func TestContract(t *testing.T) {
	t.Run("E0705-T1 契约测试", func(t *testing.T) {
		storetest.Run(t, func(t *testing.T) store.Store { return memstore.New() })
	})
}

func TestNoSharing(t *testing.T) {
	t.Run("E0705-T2 不共享内存", func(t *testing.T) {
		ctx := context.Background()
		s := memstore.New()
		gid := "01JD8X3Q5V7W9Y2Z4A00000001"
		meta := gen.GraphMeta{ID: gen.Ulid(gid), Title: "原标题"}
		doc := &gen.Graph{ID: gen.Ulid(gid), SchemaVersion: "1.0", Branch: "main", Root: gen.Subgraph{Nodes: []gen.Node{}, Edges: []gen.Edge{}}}
		if err := s.Graphs().Create(ctx, meta, doc, gen.Actor{Kind: "user", ID: "local"}); err != nil {
			t.Fatal(err)
		}
		m, err := s.Graphs().Meta(ctx, gid)
		if err != nil {
			t.Fatal(err)
		}
		m.Title = "改了"
		if again, _ := s.Graphs().Meta(ctx, gid); again.Title != "原标题" {
			t.Errorf("改读出的值影响了存储：%q", again.Title)
		}
		oid := gen.Ulid("01JD8X3Q5V7W9Y2Z4A00000002")
		recs := []gen.OpRecord{{GraphID: gen.Ulid(gid), Branch: "main", Version: 1,
			Op: gen.Op{ID: &oid, Kind: "set_graph_title", Payload: json.RawMessage(`{"to":"a"}`)}}}
		if err := s.Graphs().Commit(ctx, store.Commit{GraphID: gid, Branch: "main", Ops: recs}); err != nil {
			t.Fatal(err)
		}
		recs[0].Op.Payload[7] = 'b'
		recs[0].Version = 9
		got, err := s.Ops().Get(ctx, string(oid))
		if err != nil {
			t.Fatal(err)
		}
		if got.Version != 1 || string(got.Op.Payload) != `{"to":"a"}` {
			t.Errorf("改调用方的切片影响了存储：%+v", got)
		}
		v := json.RawMessage(`"dark"`)
		if err := s.Settings().Put(ctx, "theme", v, m.UpdatedAt.Time); err != nil {
			t.Fatal(err)
		}
		v[1] = 'D'
		if got, _, _ := s.Settings().Get(ctx, "theme"); string(got) != `"dark"` {
			t.Errorf("改调用方的值影响了设置：%s", got)
		}
	})
}
