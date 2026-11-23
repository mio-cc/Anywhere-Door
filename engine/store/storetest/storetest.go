// Package storetest 是 store.Store 的契约测试（E.07-R3、E.07.01 §8）：
// SQLite 和 memstore 的测试都调用 Run，跑同一套用例。
package storetest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/mio-cc/anywhere-door/engine/store"
	"github.com/mio-cc/anywhere-door/gen"
)

// Factory 每次返回一个新的空 Store；关闭由 Factory 自己用 t.Cleanup 安排。
type Factory func(t *testing.T) store.Store

type tcase struct {
	name string
	fn   func(t *testing.T, s store.Store)
}

// Run 跑全部契约测试（E0701-T1–T3），每个子测试用一个新的 Store。
func Run(t *testing.T, newStore Factory) {
	t.Run("E0701-T1 契约测试全集", func(t *testing.T) {
		groups := []struct {
			name  string
			cases []tcase
		}{
			{"图", graphCases()},
			{"Op", opCases()},
			{"运行", runCases()},
			{"事件", eventCases()},
			{"检查点", checkpointCases()},
			{"工具调用", toolCallCases()},
			{"设置", settingsCases()},
		}
		for _, g := range groups {
			t.Run(g.name, func(t *testing.T) {
				for _, c := range g.cases {
					t.Run(c.name, func(t *testing.T) { c.fn(t, newStore(t)) })
				}
			})
		}
	})
	t.Run("E0701-T2 Tx 回滚", func(t *testing.T) {
		for _, c := range txCases() {
			t.Run(c.name, func(t *testing.T) { c.fn(t, newStore(t)) })
		}
	})
	t.Run("E0701-T3 seq 连续", func(t *testing.T) { seqContinuous(t, newStore(t)) })
}

var ctx = context.Background()

// t0 是测试的虚拟时间起点；存储不读时钟（E.07-R5）。
var t0 = time.Date(2026, 11, 24, 8, 0, 0, 0, time.UTC)

func at(sec int) time.Time       { return t0.Add(time.Duration(sec) * time.Second) }
func ts(sec int) gen.Timestamp   { return gen.Timestamp{Time: at(sec)} }
func tsp(sec int) *gen.Timestamp { v := ts(sec); return &v }

// id 生成合法的 ULID 字符串（26 位，只用数字）。
func id(n int) string { return fmt.Sprintf("01JD8X3Q5V7W9Y2Z4A%08d", n) }

var user = gen.Actor{Kind: "user", ID: "local"}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func isErr(t *testing.T, what string, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Errorf("%s：错误是 %v，应当是 %v", what, err, want)
	}
}

// same 按 JSON 比较（读出的值经过编码、解码，按 JSON 相等即可）。
func same(t *testing.T, what string, got, want any) {
	t.Helper()
	g, err1 := json.Marshal(got)
	w, err2 := json.Marshal(want)
	if err1 != nil || err2 != nil {
		t.Fatalf("%s：编码失败 %v %v", what, err1, err2)
	}
	if string(g) != string(w) {
		t.Errorf("%s：\n得到 %s\n应当 %s", what, g, w)
	}
}

// jsonEq：两个值按 JSON 相等。
func jsonEq(a, b any) bool {
	x, err1 := json.Marshal(a)
	y, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && string(x) == string(y)
}

func title(v int64) *string { s := "v" + strconv.FormatInt(v, 10); return &s }

// doc 是版本 v 的全文，标题 vN 用来分辨取到的是哪一份。
func doc(gid string, branch string, v int64) *gen.Graph {
	return &gen.Graph{ID: gen.Ulid(gid), SchemaVersion: "1.0", Version: v, Branch: branch, Title: title(v),
		Root: gen.Subgraph{Nodes: []gen.Node{}, Edges: []gen.Edge{}}}
}

func graphMeta(gid string, sec int) gen.GraphMeta {
	return gen.GraphMeta{ID: gen.Ulid(gid), Title: "图 " + gid[len(gid)-2:], Stats: gen.GraphMetaStats{Layers: 1},
		CreatedAt: ts(sec), UpdatedAt: ts(sec)}
}

// newGraph 建图 id(n)，版本 0，main 分支。
func newGraph(t *testing.T, s store.Store, n, sec int) string {
	t.Helper()
	gid := id(n)
	must(t, s.Graphs().Create(ctx, graphMeta(gid, sec), doc(gid, "main", 0), user))
	return gid
}

// opID 让每个 (分支, 版本) 的 Op 有不同的 id。
func opID(branch string, v int64) string {
	if branch == "main" {
		return id(1000 + int(v))
	}
	return id(10000 + int(branch[len(branch)-1])*100 + int(v))
}

func opRec(gid, branch string, v int64, group string) gen.OpRecord {
	oid := gen.Ulid(opID(branch, v))
	a := user
	r := gen.OpRecord{GraphID: gen.Ulid(gid), Branch: gen.Branch(branch), Version: v,
		Op: gen.Op{Kind: "set_graph_title", Payload: json.RawMessage(`{"to":"` + *title(v) + `"}`), ID: &oid, Actor: &a, Ts: tsp(int(v))}}
	if group != "" {
		g := gen.Ulid(group)
		r.Op.GroupID = &g
	}
	return r
}

func opRecs(gid, branch string, from, to int64) []gen.OpRecord {
	var out []gen.OpRecord
	for v := from; v <= to; v++ {
		out = append(out, opRec(gid, branch, v, ""))
	}
	return out
}

func commitOf(gid, branch string, ops []gen.OpRecord, d *gen.Graph, sec int) store.Commit {
	return store.Commit{GraphID: gid, Branch: branch, Ops: ops, Doc: d, Hash: "sha256:" + strconv.Itoa(len(ops)),
		Title: "提交于 " + strconv.Itoa(sec), Stats: gen.GraphMetaStats{Layers: 1, Nodes: int64(len(ops))}, At: at(sec)}
}

func commit(t *testing.T, s store.Store, gid, branch string, from, to int64, withDoc bool, sec int) {
	t.Helper()
	var d *gen.Graph
	if withDoc {
		d = doc(gid, branch, to)
	}
	must(t, s.Graphs().Commit(ctx, commitOf(gid, branch, opRecs(gid, branch, from, to), d, sec)))
}

func versions(recs []gen.OpRecord) []int64 {
	out := []int64{}
	for _, r := range recs {
		out = append(out, r.Version)
	}
	return out
}

func runMeta(rid, gid string, sec int) gen.RunMeta {
	return gen.RunMeta{ID: gen.Ulid(rid), Kind: "run", GraphID: gen.Ulid(gid), Branch: "main", GraphVersion: 1,
		Status: "pending", StartedBy: user, CreatedAt: ts(sec)}
}

func newRun(t *testing.T, s store.Store, n, sec int) string {
	t.Helper()
	m, err := s.Runs().Create(ctx, runMeta(id(n), id(1), sec))
	must(t, err)
	return string(m.ID)
}

func runEvent(rid, node string, sec int) gen.RunEvent {
	e := gen.RunEvent{Ts: ts(sec), Kind: "node.progress", Actor: user, RunID: gen.Ulid(rid), Data: json.RawMessage(`{"pct":` + strconv.Itoa(sec%100) + `}`)}
	if node != "" {
		e.Loc = &gen.Loc{NodeID: gen.Ulid(node), Path: "根图/" + node[len(node)-2:]}
	}
	return e
}

func runEvents(rid, node string, n, sec int) []gen.RunEvent {
	out := make([]gen.RunEvent, n)
	for i := range out {
		out[i] = runEvent(rid, node, sec+i)
	}
	return out
}

func seqsOf(evs []gen.RunEvent) []int64 {
	out := []int64{}
	for _, e := range evs {
		out = append(out, e.Seq)
	}
	return out
}

// seqContinuous：100 批、共 1 万条 P 级事件，读出的 seq 从 1 到 10000 无空洞（C0601-T1）。
func seqContinuous(t *testing.T, s store.Store) {
	rid := newRun(t, s, 500, 0)
	for b := 0; b < 100; b++ {
		last, err := s.Events().AppendRun(ctx, rid, runEvents(rid, id(7), 100, b))
		must(t, err)
		if last != int64(b+1)*100 {
			t.Fatalf("第 %d 批之后 last 是 %d，应当 %d", b+1, last, (b+1)*100)
		}
	}
	var got []int64
	after := int64(0)
	for {
		evs, err := s.Events().RunRange(ctx, rid, after, 0)
		must(t, err)
		if len(evs) == 0 {
			break
		}
		if len(evs) > store.DefaultEventLimit {
			t.Fatalf("limit 为 0 时一次读出 %d 条，应当最多 %d", len(evs), store.DefaultEventLimit)
		}
		got = append(got, seqsOf(evs)...)
		after = evs[len(evs)-1].Seq
	}
	if len(got) != 10000 {
		t.Fatalf("读出 %d 条，应当 10000", len(got))
	}
	for i, v := range got {
		if v != int64(i+1) {
			t.Fatalf("第 %d 条的 seq 是 %d，应当 %d", i+1, v, i+1)
		}
	}
	m, err := s.Runs().Get(ctx, rid)
	must(t, err)
	if m.LastSeq != 10000 {
		t.Errorf("RunMeta.last_seq 是 %d，应当 10000", m.LastSeq)
	}
}
