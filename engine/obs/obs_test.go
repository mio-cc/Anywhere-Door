package obs

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mio-cc/anywhere-door/gen"
)

var ctx = context.Background()

// fakeStore 给事件分配 seq（每条流从 1 起连续），可以设成返回错误。
type fakeStore struct {
	mu      sync.Mutex
	fail    error
	run     map[string][]gen.RunEvent
	session map[string][]gen.SessionEvent
}

func newFake() *fakeStore {
	return &fakeStore{run: map[string][]gen.RunEvent{}, session: map[string][]gen.SessionEvent{}}
}

func (f *fakeStore) AppendRun(_ context.Context, id string, evs []gen.RunEvent) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return 0, f.fail
	}
	for i := range evs {
		evs[i].Seq = int64(len(f.run[id]) + 1)
		f.run[id] = append(f.run[id], evs[i])
	}
	return int64(len(f.run[id])), nil
}

func (f *fakeStore) AppendSession(_ context.Context, id string, evs []gen.SessionEvent) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return 0, f.fail
	}
	for i := range evs {
		evs[i].Seq = int64(len(f.session[id]) + 1)
		f.session[id] = append(f.session[id], evs[i])
	}
	return int64(len(f.session[id])), nil
}

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

const runID = "01JCOR00000000000000000001"

var sched = gen.Actor{Kind: gen.ActorKindRuntime, ID: "sched"}

func nodeLoc(id string) *gen.Loc { return &gen.Loc{NodeID: gen.Ulid(id), Path: "根图/x"} }

func ev(kind gen.RunEventKind, loc *gen.Loc) gen.RunEvent {
	return gen.RunEvent{Actor: sched, Kind: kind, Loc: loc, Data: json.RawMessage(`{}`)}
}

func drain(s *Sub) []any {
	var out []any
	for {
		select {
		case e, ok := <-s.C:
			if !ok {
				return out
			}
			out = append(out, e)
		default:
			return out
		}
	}
}

func setup() (*fakeStore, *Hub, *Writer, *clock) {
	st, hub := newFake(), NewHub(0)
	c := &clock{t: time.Date(2026, 11, 26, 1, 0, 0, 0, time.UTC)}
	return st, hub, NewWriter(st, hub, WriterOpts{Now: c.now}), c
}

func TestWriter(t *testing.T) {
	t.Run("E0901-T1 先落库再广播", func(t *testing.T) {
		st, hub, w, _ := setup()
		sub := hub.Subscribe(StreamKey{Kind: StreamRun, ID: runID})
		if err := w.EmitRun(ctx, runID, ev(gen.EvRunCreated, nil), ev(gen.EvNodeState, nodeLoc("01JCON00000000000000000001"))); err != nil {
			t.Fatal(err)
		}
		got := drain(sub)
		if len(got) != 2 || len(st.run[runID]) != 2 {
			t.Fatalf("应当落库并广播 2 条：%d %d", len(got), len(st.run[runID]))
		}
		for i, g := range got {
			e := g.(gen.RunEvent)
			if e.Seq != st.run[runID][i].Seq || e.Seq != int64(i+1) || string(e.RunID) != runID || e.Ts.IsZero() {
				t.Errorf("广播的事件和存储的不一致：%+v", e)
			}
		}
		st.fail = errors.New("磁盘满了")
		if err := w.EmitRun(ctx, runID, ev(gen.EvRunStatus, nil)); err == nil {
			t.Error("落库失败应当返回错误")
		}
		if n := len(drain(sub)); n != 0 {
			t.Errorf("落库失败不应当广播：%d", n)
		}
		st.fail = nil
		noActor := ev(gen.EvRunStatus, nil)
		noActor.Actor = gen.Actor{}
		if err := w.EmitRun(ctx, runID, ev(gen.EvRunStatus, nil), noActor); !errors.Is(err, ErrNoActor) || len(st.run[runID]) != 2 {
			t.Errorf("没有 actor 时一条都不写：%v %d", err, len(st.run[runID]))
		}
		big := ev(gen.EvNodeOutput, nodeLoc("01JCON00000000000000000001"))
		big.Data = json.RawMessage(`"` + strings.Repeat("x", 5000) + `"`)
		if err := w.EmitRun(ctx, runID, big); !errors.Is(err, ErrDataTooLarge) {
			t.Errorf("data 超长应当报错：%v", err)
		}
		ss := hub.Subscribe(StreamKey{Kind: StreamSession, ID: "01JCOS00000000000000000001"})
		se := gen.SessionEvent{Actor: sched, Kind: gen.EvSessionUpdated, Data: json.RawMessage(`{}`)}
		if err := w.EmitSession(ctx, "01JCOS00000000000000000001", se); err != nil {
			t.Fatal(err)
		}
		if g := drain(ss); len(g) != 1 || g[0].(gen.SessionEvent).Seq != 1 {
			t.Errorf("会话流同样先落库再广播：%v", g)
		}
		w.EphemeralRun(gen.RunEvent{Actor: sched, Kind: gen.EvNodeChunk, RunID: runID})
		if g := drain(sub); len(g) != 1 || g[0].(gen.RunEvent).Seq != 0 || len(st.run[runID]) != 2 {
			t.Errorf("E 级只广播、seq 为 0：%v", g)
		}
	})

	t.Run("E0901-T2 节流", func(t *testing.T) {
		st, hub, w, c := setup()
		sub := hub.Subscribe(StreamKey{Kind: StreamRun, ID: runID})
		a, b := nodeLoc("01JCON00000000000000000001"), nodeLoc("01JCON00000000000000000002")
		for i := 0; i < 10; i++ {
			if err := w.EmitRun(ctx, runID, ev(gen.EvNodeProgress, a)); err != nil {
				t.Fatal(err)
			}
		}
		if n, m := len(st.run[runID]), len(drain(sub)); n != 4 || m != 10 {
			t.Errorf("1 秒内 10 条只落库 4 条、广播 10 条：%d %d", n, m)
		}
		_ = w.EmitRun(ctx, runID, ev(gen.EvNodeThrottled, b))
		_ = w.EmitRun(ctx, runID, ev(gen.EvNodeState, a))
		if n := len(st.run[runID]); n != 6 {
			t.Errorf("不同执行各自计数、P 级不节流：%d", n)
		}
		c.add(time.Second)
		_ = w.EmitRun(ctx, runID, ev(gen.EvNodeProgress, a))
		if n := len(st.run[runID]); n != 7 {
			t.Errorf("下一秒又能落库：%d", n)
		}
	})

	t.Run("E0901-T4 路径向量", func(t *testing.T) {
		b, err := os.ReadFile(filepath.Join("..", "..", "schema", "examples", "path_vectors.json"))
		if err != nil {
			t.Fatal(err)
		}
		var vs []struct {
			Name  string `json:"name"`
			Chain []struct {
				NodeID    string   `json:"node_id"`
				Label     string   `json:"label"`
				Iter      gen.Iter `json:"iter"`
				SameLabel bool     `json:"same_label"`
			} `json:"chain"`
			Path string `json:"path"`
			Key  string `json:"key"`
		}
		if err := json.Unmarshal(b, &vs); err != nil || len(vs) < 8 {
			t.Fatalf("向量：%v %d", err, len(vs))
		}
		for _, v := range vs {
			var chain []PathItem
			var scopes []gen.LocScopesItem
			for i, c := range v.Chain {
				chain = append(chain, PathItem{NodeID: c.NodeID, Label: c.Label, Iter: c.Iter, SameLabel: c.SameLabel})
				if i < len(v.Chain)-1 {
					it := c.Iter
					scopes = append(scopes, gen.LocScopesItem{NodeID: gen.Ulid(c.NodeID), Iter: &it})
				}
			}
			last := v.Chain[len(v.Chain)-1]
			loc := gen.Loc{NodeID: gen.Ulid(last.NodeID), Iter: &last.Iter}
			if len(scopes) > 0 {
				loc.Scopes = &scopes
			}
			if got := PathOf(chain); got != v.Path {
				t.Errorf("%s：路径 %q，应当是 %q", v.Name, got, v.Path)
			}
			if got := ExecKey(loc); got != v.Key {
				t.Errorf("%s：键 %q，应当是 %q", v.Name, got, v.Key)
			}
		}
	})

	t.Run("E0901-T6 Hub", func(t *testing.T) {
		hub := NewHub(2)
		k := StreamKey{Kind: StreamRun, ID: runID}
		fast, slow := hub.Subscribe(k), hub.Subscribe(k)
		other := hub.Subscribe(StreamKey{Kind: StreamRun, ID: "01JCOR00000000000000000002"})
		for i := 0; i < 3; i++ {
			hub.Publish(k, i)
			if i < 2 {
				<-fast.C
			}
		}
		if !slow.Lagged() || fast.Lagged() {
			t.Errorf("慢订阅者缓冲满后被断开：%v %v", slow.Lagged(), fast.Lagged())
		}
		if g := drain(slow); len(g) != 2 {
			t.Errorf("被断开前收到的还能读出来，之后 channel 关闭：%v", g)
		}
		if _, ok := <-slow.C; ok {
			t.Error("被断开后 channel 应当关闭")
		}
		if g := drain(other); len(g) != 0 {
			t.Errorf("别的流收不到：%v", g)
		}
		fast.Close()
		fast.Close()
		hub.Publish(k, 9)
		if g := drain(fast); len(g) != 1 || g[0] != 2 {
			t.Errorf("Close 后只剩缓冲里原有的，收不到新的：%v", g)
		}
	})
}

func BenchmarkEmit(b *testing.B) {
	for range b.N {
		emit10k(b)
	}
}

// emit10k 分 100 批写 1 万条（E0901-T5）。
func emit10k(tb testing.TB) time.Duration {
	_, _, w, _ := setup()
	batch := make([]gen.RunEvent, 100)
	for i := range batch {
		batch[i] = ev(gen.EvNodeState, nodeLoc("01JCON00000000000000000001"))
	}
	start := time.Now()
	for i := 0; i < 100; i++ {
		if err := w.EmitRun(ctx, runID, batch...); err != nil {
			tb.Fatal(err)
		}
	}
	return time.Since(start)
}

func TestEmitPerf(t *testing.T) {
	t.Run("E0901-T5 BenchmarkEmit", func(t *testing.T) {
		d := emit10k(t)
		if os.Getenv("ANYDOOR_PERF") == "1" && d > time.Second {
			t.Errorf("1 万条事件写入应当 < 1s，实际 %v", d)
		}
	})
}
