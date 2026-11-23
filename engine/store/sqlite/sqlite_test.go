package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mio-cc/anywhere-door/engine/store"
	"github.com/mio-cc/anywhere-door/engine/store/storetest"
	"github.com/mio-cc/anywhere-door/gen"
)

// wantSchema 是 E.07.02 §4.1 的表：列（名字 类型 [NN 非空] [PKn 主键第 n 列] [=默认值]）| 索引。
var wantSchema = map[string]string{
	"hx_branches":           "graph_id TEXT NN PK1, name TEXT NN PK2, base_version INTEGER NN, head_version INTEGER NN, deleted_at TEXT, doc TEXT NN",
	"hx_checkpoint_history": "run_id TEXT NN PK1, exec_key TEXT NN PK2, seq INTEGER NN PK3, executor TEXT NN, state_ref TEXT NN, ts TEXT NN",
	"hx_checkpoints":        "run_id TEXT NN PK1, exec_key TEXT NN PK2, seq INTEGER NN, executor TEXT NN, state_ref TEXT NN, ts TEXT NN",
	"hx_graph_versions":     "graph_id TEXT NN PK1, branch TEXT NN PK2, version INTEGER NN PK3, hash TEXT NN, doc TEXT",
	"hx_graphs":             "id TEXT NN PK1, updated_at TEXT NN, deleted_at TEXT, doc TEXT NN | hx_graphs_updated(updated_at DESC, id DESC)",
	"hx_migrations":         "version INTEGER PK1, applied_at TEXT NN",
	"hx_ops":                "graph_id TEXT NN PK1, branch TEXT NN PK2, version INTEGER NN PK3, id TEXT NN, group_id TEXT, undone INTEGER NN =0, doc TEXT NN | hx_ops_group(group_id); unique(id)",
	"hx_run_events":         "run_id TEXT NN PK1, seq INTEGER NN PK2, kind TEXT NN, node_id TEXT, doc TEXT NN | hx_run_events_node(run_id, node_id, seq)",
	"hx_run_node_state":     "run_id TEXT NN PK1, exec_key TEXT NN PK2, doc TEXT NN",
	"hx_run_views":          "run_id TEXT NN PK1, seq INTEGER NN PK2, view_ref TEXT NN",
	"hx_runs":               "id TEXT NN PK1, graph_id TEXT NN, session_id TEXT, kind TEXT NN, status TEXT NN, no INTEGER NN, created_at TEXT NN, last_seq INTEGER NN =0, lease_owner TEXT, lease_until TEXT, doc TEXT NN | hx_runs_created(created_at DESC, id DESC); hx_runs_graph(graph_id, created_at DESC); hx_runs_session(session_id, kind); hx_runs_status(status); unique(graph_id, no)",
	"hx_session_events":     "session_id TEXT NN PK1, seq INTEGER NN PK2, kind TEXT NN, doc TEXT NN",
	"hx_settings":           "key TEXT NN PK1, value TEXT NN, updated_at TEXT NN",
	"hx_tool_calls":         "idem_key TEXT NN PK1, run_id TEXT NN, exec_key TEXT NN, tool TEXT NN, risk INTEGER NN, status TEXT NN, args_ref TEXT NN ='', result_ref TEXT NN ='', started_at TEXT NN, ended_at TEXT | hx_tool_calls_run(run_id)",
	"hx_undo_stack":         "graph_id TEXT NN PK1, branch TEXT NN PK2, actor_id TEXT NN PK3, seq INTEGER NN PK4, group_id TEXT NN, undone INTEGER NN =0",
}

func openTemp(t *testing.T, path string) store.Store {
	t.Helper()
	s, err := Open(context.Background(), path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("Close：%v", err)
		}
	})
	return s
}

func strs(t *testing.T, db *sql.DB, query string, args ...any) []string {
	t.Helper()
	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// describe 按 wantSchema 的写法描述一张表。
func describe(t *testing.T, db *sql.DB, table string) string {
	t.Helper()
	cols := strs(t, db, `SELECT name || ' ' || type || CASE WHEN "notnull" = 1 THEN ' NN' ELSE '' END
 || CASE WHEN pk > 0 THEN ' PK' || pk ELSE '' END || CASE WHEN dflt_value IS NOT NULL THEN ' =' || dflt_value ELSE '' END
 FROM pragma_table_info(?) ORDER BY cid`, table)
	var idx []string
	type ix struct{ name, origin string }
	var list []ix
	rows, err := db.Query(`SELECT name, origin FROM pragma_index_list(?)`, table)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var x ix
		if err := rows.Scan(&x.name, &x.origin); err != nil {
			t.Fatal(err)
		}
		list = append(list, x)
	}
	_ = rows.Close()
	for _, x := range list {
		if x.origin == "pk" {
			continue
		}
		keys := strs(t, db, `SELECT name || CASE WHEN "desc" = 1 THEN ' DESC' ELSE '' END FROM pragma_index_xinfo(?) WHERE key = 1 ORDER BY seqno`, x.name)
		head := x.name + "("
		if x.origin == "u" {
			head = "unique("
		}
		idx = append(idx, head+strings.Join(keys, ", ")+")")
	}
	sort.Strings(idx)
	out := strings.Join(cols, ", ")
	if len(idx) > 0 {
		out += " | " + strings.Join(idx, "; ")
	}
	return out
}

func TestSQLite(t *testing.T) {
	ctx := context.Background()
	t.Run("E0702-T1 表结构比对", func(t *testing.T) {
		s := openTemp(t, filepath.Join(t.TempDir(), "s.db")).(*sqlStore)
		tables := strs(t, s.rdb, `SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name`)
		var want []string
		for n := range wantSchema {
			want = append(want, n)
		}
		sort.Strings(want)
		if strings.Join(tables, ",") != strings.Join(want, ",") {
			t.Fatalf("表是 %v，应当 %v", tables, want)
		}
		for _, n := range want {
			if got := describe(t, s.rdb, n); got != wantSchema[n] {
				t.Errorf("%s：\n得到 %s\n应当 %s", n, got, wantSchema[n])
			}
		}
	})
	t.Run("E0702-T2 冗余列一致", func(t *testing.T) {
		st := openTemp(t, filepath.Join(t.TempDir(), "r.db"))
		s := st.(*sqlStore)
		gid, rid, oid, grp := "01JD8X3Q5V7W9Y2Z4A00000001", "01JD8X3Q5V7W9Y2Z4A00000002", gen.Ulid("01JD8X3Q5V7W9Y2Z4A00000003"), gen.Ulid("01JD8X3Q5V7W9Y2Z4A00000004")
		at := time.Date(2026, 11, 24, 8, 0, 0, 0, time.UTC)
		doc := &gen.Graph{ID: gen.Ulid(gid), SchemaVersion: "1.0", Branch: "main", Root: gen.Subgraph{Nodes: []gen.Node{}, Edges: []gen.Edge{}}}
		if err := st.Graphs().Create(ctx, gen.GraphMeta{ID: gen.Ulid(gid), CreatedAt: gen.Timestamp{Time: at}, UpdatedAt: gen.Timestamp{Time: at}}, doc, gen.Actor{Kind: "user", ID: "local"}); err != nil {
			t.Fatal(err)
		}
		rec := gen.OpRecord{GraphID: gen.Ulid(gid), Branch: "main", Version: 1, Op: gen.Op{ID: &oid, GroupID: &grp, Kind: "set_graph_title", Payload: []byte(`{"to":"a"}`)}}
		if err := st.Graphs().Commit(ctx, store.Commit{GraphID: gid, Branch: "main", Ops: []gen.OpRecord{rec}, At: at.Add(time.Minute)}); err != nil {
			t.Fatal(err)
		}
		e := store.UndoEntry{GraphID: gid, Branch: "main", ActorID: "local", Seq: 1, GroupID: string(grp)}
		if err := st.Ops().UndoPush(ctx, e); err != nil {
			t.Fatal(err)
		}
		if err := st.Ops().UndoMark(ctx, e, true); err != nil {
			t.Fatal(err)
		}
		if _, err := st.Runs().Create(ctx, gen.RunMeta{ID: gen.Ulid(rid), Kind: "run", GraphID: gen.Ulid(gid), Branch: "main", Status: "pending", StartedBy: gen.Actor{Kind: "user", ID: "local"}, CreatedAt: gen.Timestamp{Time: at}}); err != nil {
			t.Fatal(err)
		}
		if err := st.Runs().SetStatus(ctx, rid, "running", nil); err != nil {
			t.Fatal(err)
		}
		if _, err := st.Events().AppendRun(ctx, rid, []gen.RunEvent{{Kind: "node.progress", RunID: gen.Ulid(rid), Data: []byte(`{}`), Ts: gen.Timestamp{Time: at}}}); err != nil {
			t.Fatal(err)
		}
		checks := []struct{ what, col, docExpr string }{
			{"hx_runs.status", `SELECT status FROM hx_runs`, `SELECT json_extract(doc, '$.status') FROM hx_runs`},
			{"hx_runs.last_seq", `SELECT CAST(last_seq AS TEXT) FROM hx_runs`, `SELECT CAST(json_extract(doc, '$.last_seq') AS TEXT) FROM hx_runs`},
			{"hx_ops.undone", `SELECT CAST(undone AS TEXT) FROM hx_ops`, `SELECT CAST(json_extract(doc, '$.undone') AS TEXT) FROM hx_ops`},
			{"hx_graphs.updated_at", `SELECT updated_at FROM hx_graphs`, `SELECT json_extract(doc, '$.updated_at') FROM hx_graphs`},
			{"hx_branches.head_version", `SELECT CAST(head_version AS TEXT) FROM hx_branches`, `SELECT CAST(json_extract(doc, '$.head') AS TEXT) FROM hx_branches`},
		}
		for _, c := range checks {
			col, doc := strs(t, s.rdb, c.col), strs(t, s.rdb, c.docExpr)
			if fmt.Sprint(col) != fmt.Sprint(doc) {
				t.Errorf("%s 与全文不一致：列 %v，全文 %v", c.what, col, doc)
			}
		}
		if got := strs(t, s.rdb, `SELECT status FROM hx_runs`); fmt.Sprint(got) != "[running]" {
			t.Errorf("hx_runs.status 应当是 running，是 %v", got)
		}
		if got := strs(t, s.rdb, `SELECT CAST(undone AS TEXT) FROM hx_ops`); fmt.Sprint(got) != "[1]" {
			t.Errorf("hx_ops.undone 应当是 1，是 %v", got)
		}
	})
	t.Run("E0702-T3 契约测试", func(t *testing.T) {
		storetest.Run(t, func(t *testing.T) store.Store { return openTemp(t, filepath.Join(t.TempDir(), "c.db")) })
	})
	t.Run("E0702-T4 重新打开", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "anydoor.db")
		s, err := Open(ctx, path, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Settings().Put(ctx, "theme", []byte(`"dark"`), time.Unix(0, 0)); err != nil {
			t.Fatal(err)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		s = openTemp(t, path)
		v, ok, err := s.Settings().Get(ctx, "theme")
		if err != nil || !ok || string(v) != `"dark"` {
			t.Errorf("重新打开后读到 %s %v %v", v, ok, err)
		}
		if baks, _ := filepath.Glob(path + ".bak-*"); len(baks) != 0 {
			t.Errorf("版本没变时不应当备份：%v", baks)
		}
		if v, err := SchemaVersion(ctx, s.(*sqlStore).rdb); err != nil || v != 1 {
			t.Errorf("SchemaVersion 是 %d %v，应当 1", v, err)
		}
	})
}
