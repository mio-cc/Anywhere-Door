package op_test

// 流水线测试（外部测试包：要用 engine.NewOpStore，而 engine 引用了 op）共用的辅助函数。E.05.01 建；
// 后面的叶子（E.05.02、E.05.04）只调用、不修改，需要别的写在自己的测试文件里。

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mio-cc/anywhere-door/engine"
	"github.com/mio-cc/anywhere-door/engine/authz"
	"github.com/mio-cc/anywhere-door/engine/op"
	"github.com/mio-cc/anywhere-door/engine/schema"
	"github.com/mio-cc/anywhere-door/engine/store"
	"github.com/mio-cc/anywhere-door/engine/store/memstore"
	"github.com/mio-cc/anywhere-door/engine/store/sqlite"
	"github.com/mio-cc/anywhere-door/gen"
)

var ctx = context.Background()

var (
	regOnce sync.Once
	regView schema.View
	regErr  error
)

// builtinView 是仓库 schema/ 的内置注册表（只加载一次）。
func builtinView(t testing.TB) schema.View {
	t.Helper()
	regOnce.Do(func() {
		r := schema.NewRegistry(schema.Options{})
		regErr = r.LoadBuiltin(os.DirFS(filepath.Join("..", "..", "schema")))
		regView = r.Snapshot()
	})
	if regErr != nil {
		t.Fatalf("内置注册表加载失败：\n%v", regErr)
	}
	return regView
}

var (
	user  = gen.Actor{Kind: gen.ActorKindUser, ID: "local"}
	model = gen.Actor{Kind: gen.ActorKindModel, ID: "chat:01JCQS00000000000000000001"}
)

type idGen struct {
	prefix string // 5 个字符
	n      atomic.Int64
}

func (g *idGen) next() string { return fmt.Sprintf("%s%021d", g.prefix, g.n.Add(1)) }

type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *testClock) Add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

type recBus struct {
	mu  sync.Mutex
	evs []gen.GraphEvent
}

func (b *recBus) Publish(evs []gen.GraphEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.evs = append(b.evs, evs...)
}

func (b *recBus) all() []gen.GraphEvent {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]gen.GraphEvent(nil), b.evs...)
}

type recRuns struct {
	mu       sync.Mutex
	versions []int64
}

func (r *recRuns) GraphChanged(_, _ string, v int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.versions = append(r.versions, v)
}

type harness struct {
	t     testing.TB
	s     store.Store
	opt   op.Options
	p     *op.Pipeline
	bus   *recBus
	runs  *recRuns
	clock *testClock
	gid   string
}

func newStore(t testing.TB, backend string) store.Store {
	t.Helper()
	if backend != "sqlite" {
		return memstore.New()
	}
	s, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "op.db"), sqlite.Options{})
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

// setup 建存储（mem 或 sqlite）、流水线和一张空图（版本 0、格式 1.0）。
func setup(t testing.TB, backend string, mod ...func(*op.Options)) *harness {
	t.Helper()
	h := &harness{t: t, s: newStore(t, backend), bus: &recBus{}, runs: &recRuns{},
		clock: &testClock{t: time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)}}
	ids := &idGen{prefix: "01JCQ"}
	h.opt = op.Options{Repo: engine.NewOpStore(h.s), View: builtinView(t), Authz: authz.Fixed{},
		Bus: h.bus, Runs: h.runs, Now: h.clock.Now, NewID: ids.next}
	for _, f := range mod {
		f(&h.opt)
	}
	h.p = op.New(h.opt)
	h.gid = h.newGraph("1.0")
	return h
}

func (h *harness) newGraph(schemaVersion string) string {
	h.t.Helper()
	id := h.opt.NewID()
	now := gen.Timestamp{Time: h.clock.Now()}
	d := &gen.Graph{ID: gen.Ulid(id), Branch: "main", SchemaVersion: schemaVersion, Root: gen.Subgraph{Nodes: []gen.Node{}, Edges: []gen.Edge{}}}
	meta := gen.GraphMeta{ID: gen.Ulid(id), Stats: gen.GraphMetaStats{Layers: 1}, CreatedAt: now, UpdatedAt: now}
	if err := h.s.Graphs().Create(ctx, meta, d, user); err != nil {
		h.t.Fatal(err)
	}
	return id
}

// reopen 是同一个存储上的新流水线：缓存是空的，读图要从存储重放。
func (h *harness) reopen() *op.Pipeline { return op.New(h.opt) }

func (h *harness) head() int64 {
	h.t.Helper()
	g, _, err := h.p.Read(ctx, h.gid, "main", op.Head)
	if err != nil {
		h.t.Fatal(err)
	}
	return g.Version()
}

func (h *harness) batch(actor gen.Actor, base int64, ops ...gen.Op) op.Batch {
	return op.Batch{GraphID: h.gid, Branch: "main", BaseVersion: base, Actor: actor, Ops: ops}
}

// submit 以 head 为 base 提交。
func (h *harness) submit(actor gen.Actor, ops ...gen.Op) (*op.Result, error) {
	return h.p.Submit(ctx, h.batch(actor, h.head(), ops...))
}

func (h *harness) must(actor gen.Actor, ops ...gen.Op) *op.Result {
	h.t.Helper()
	res, err := h.submit(actor, ops...)
	if err != nil {
		h.t.Fatalf("提交失败：%v", err)
	}
	return res
}

func (h *harness) doc() *gen.Graph {
	h.t.Helper()
	g, _, err := h.p.Read(ctx, h.gid, "main", op.Head)
	if err != nil {
		h.t.Fatal(err)
	}
	return g.Doc()
}

func (h *harness) records() []gen.OpRecord {
	h.t.Helper()
	recs, err := h.s.Ops().Range(ctx, h.gid, "main", 0, 1<<40)
	if err != nil {
		h.t.Fatal(err)
	}
	return recs
}

func asJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func nid(n int) gen.Ulid { return gen.Ulid(fmt.Sprintf("01JCN%021d", n)) }
func eid(n int) gen.Ulid { return gen.Ulid(fmt.Sprintf("01JCE%021d", n)) }
func tid(n int) gen.Ulid { return gen.Ulid(fmt.Sprintf("01JCT%021d", n)) }

func mkop(kind gen.OpKind, payload any) gen.Op {
	return gen.Op{Kind: kind, Payload: json.RawMessage(asJSON(payload))}
}

func withReason(o gen.Op, r string) gen.Op {
	o.Reason = &r
	return o
}

// addNode：kv 是参数名、值交替。
func addNode(n int, typ, scope string, kv ...any) gen.Op {
	nd := gen.Node{ID: nid(n), Type: gen.NodeTypeRef(typ), Pos: gen.Pos{X: int64(n) * 20}}
	if len(kv) > 0 {
		p := map[string]json.RawMessage{}
		for i := 0; i+1 < len(kv); i += 2 {
			p[kv[i].(string)] = json.RawMessage(asJSON(kv[i+1]))
		}
		nd.Params = &p
	}
	return mkop(gen.OpAddNode, gen.PayloadAddNode{Scope: scope, Node: nd})
}

func connect(n, from int, fport string, to int, tport string) gen.Op {
	return mkop(gen.OpConnect, gen.PayloadConnect{Edge: gen.Edge{ID: eid(n),
		From: gen.PortRef{Node: nid(from), Port: fport}, To: gen.PortRef{Node: nid(to), Port: tport}}})
}

func setLabel(node int, to string) gen.Op {
	return mkop(gen.OpSetLabel, gen.PayloadSetLabel{Node: nid(node), To: to})
}

func setParam(node int, key string, to any) gen.Op {
	return mkop(gen.OpSetParam, gen.PayloadSetParam{Node: nid(node), Key: key, To: json.RawMessage(asJSON(to))})
}

func moveNode(node int, x, y int64) gen.Op {
	return mkop(gen.OpMoveNodes, gen.PayloadMoveNodes{Moves: []gen.PayloadMoveNodesMovesItem{{ID: nid(node), To: gen.Pos{X: x, Y: y}}}})
}

func removeNodes(ids ...int) gen.Op {
	var u []gen.Ulid
	for _, i := range ids {
		u = append(u, nid(i))
	}
	return mkop(gen.OpRemoveNodes, gen.PayloadRemoveNodes{IDs: u})
}
