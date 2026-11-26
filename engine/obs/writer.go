// Package obs 是可观测（设计文档 E.09）：事件写入（先落库再广播）、广播 Hub、运行路径。只依赖 gen（E.01 §1）：
// 存储和时钟用本包的接口，由组装处接上（P.09 K265）。
package obs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/mio-cc/anywhere-door/gen"
)

// EventStore 是事件落库用的接口；store.EventRepo 满足它（E.07.01）。
type EventStore interface {
	AppendRun(ctx context.Context, runID string, evs []gen.RunEvent) (int64, error)
	AppendSession(ctx context.Context, sessionID string, evs []gen.SessionEvent) (int64, error)
}

// WriterOpts 是写入器的选项。
type WriterOpts struct {
	Now     func() time.Time // 必填
	MaxData int              // data 的上限，默认 4096 字节（C.06.01-R4）
	SPerSec int              // S 级事件每个执行每秒最多落库几条，默认 4
}

// 错误。
var (
	ErrNoActor      = errors.New("obs: 事件没有 actor（C.06.01-R2）")
	ErrDataTooLarge = errors.New("obs: 事件的 data 太大，大内容先写进工作区，这里只放引用（C.06.01-R4）")
)

// sLevel 是按执行节流落库的 S 级事件（E.09.01 §5 第 2 条）。
var sLevel = map[gen.RunEventKind]bool{gen.EvNodeProgress: true, gen.EvNodeThrottled: true}

// Writer 是事件写入器：返回时事件已落库并广播（INV-11）。
type Writer struct {
	s   EventStore
	hub *Hub
	o   WriterOpts
	mu  sync.Mutex
	win map[string]*window
}

type window struct {
	sec int64
	n   int
}

// NewWriter 建写入器；hub 可为 nil（不广播）。
func NewWriter(s EventStore, hub *Hub, o WriterOpts) *Writer {
	if o.Now == nil {
		panic("obs.NewWriter：WriterOpts.Now 必填")
	}
	if o.MaxData <= 0 {
		o.MaxData = 4096
	}
	if o.SPerSec <= 0 {
		o.SPerSec = 4
	}
	return &Writer{s: s, hub: hub, o: o, win: map[string]*window{}}
}

func check(actor gen.Actor, data []byte, limit int) error {
	if actor.Kind == "" || actor.ID == "" {
		return ErrNoActor
	}
	if len(data) > limit {
		return fmt.Errorf("%w：%d 字节，上限 %d", ErrDataTooLarge, len(data), limit)
	}
	return nil
}

// allow：这个执行这一秒还能不能落库一条 S 级事件。
func (w *Writer) allow(key string, now time.Time) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	sec := now.Unix()
	win := w.win[key]
	if win == nil || win.sec != sec {
		if len(w.win) > 10000 {
			w.win = map[string]*window{}
		}
		win = &window{sec: sec}
		w.win[key] = win
	}
	if win.n >= w.o.SPerSec {
		return false
	}
	win.n++
	return true
}

func (w *Writer) publish(k StreamKey, ev any) {
	if w.hub != nil {
		w.hub.Publish(k, ev)
	}
}

// EmitRun 写运行流的事件（E.09.01 §5）：一次调用一次 AppendRun（seq 在同一事务里分配），成功后按顺序广播。
// 有一条没有 actor 或 data 太大时一条都不写。被节流的 S 级事件只广播、seq 为 0。落库失败时返回错误、不广播。
func (w *Writer) EmitRun(ctx context.Context, runID string, evs ...gen.RunEvent) error {
	if len(evs) == 0 {
		return nil
	}
	now := w.o.Now().UTC()
	out := make([]gen.RunEvent, len(evs))
	for i, e := range evs {
		if err := check(e.Actor, e.Data, w.o.MaxData); err != nil {
			return fmt.Errorf("第 %d 条（%s）：%w", i+1, e.Kind, err)
		}
		e.RunID, e.Ts, e.Seq = gen.Ulid(runID), gen.Timestamp{Time: now}, 0
		out[i] = e
	}
	var idx []int
	var stored []gen.RunEvent
	for i, e := range out {
		if sLevel[e.Kind] && e.Loc != nil && !w.allow(ExecKey(*e.Loc), now) {
			continue
		}
		idx = append(idx, i)
		stored = append(stored, e)
	}
	if len(stored) > 0 {
		last, err := w.s.AppendRun(ctx, runID, stored)
		if err != nil {
			return fmt.Errorf("obs: 事件落库失败：%w", err)
		}
		first := last - int64(len(stored)) + 1 // seq 连续分配（C.06.01-R1）
		for j, i := range idx {
			out[i].Seq = first + int64(j)
		}
	}
	k := StreamKey{Kind: StreamRun, ID: runID}
	for _, e := range out {
		w.publish(k, e)
	}
	return nil
}

// EmitSession 写会话流的事件：同 EmitRun，不节流。
func (w *Writer) EmitSession(ctx context.Context, sessionID string, evs ...gen.SessionEvent) error {
	if len(evs) == 0 {
		return nil
	}
	now := w.o.Now().UTC()
	out := make([]gen.SessionEvent, len(evs))
	for i, e := range evs {
		if err := check(e.Actor, e.Data, w.o.MaxData); err != nil {
			return fmt.Errorf("第 %d 条（%s）：%w", i+1, e.Kind, err)
		}
		e.SessionID, e.Ts, e.Seq = gen.Ulid(sessionID), gen.Timestamp{Time: now}, 0
		out[i] = e
	}
	last, err := w.s.AppendSession(ctx, sessionID, out)
	if err != nil {
		return fmt.Errorf("obs: 事件落库失败：%w", err)
	}
	for i := range out {
		out[i].Seq = last - int64(len(out)) + 1 + int64(i)
	}
	k := StreamKey{Kind: StreamSession, ID: sessionID}
	for _, e := range out {
		w.publish(k, e)
	}
	return nil
}

// EphemeralRun 是 E 级运行事件：seq 0，只广播、不落库。
func (w *Writer) EphemeralRun(ev gen.RunEvent) {
	ev.Ts, ev.Seq = gen.Timestamp{Time: w.o.Now().UTC()}, 0
	w.publish(StreamKey{Kind: StreamRun, ID: string(ev.RunID)}, ev)
}

// EphemeralSession 是 E 级会话事件。
func (w *Writer) EphemeralSession(ev gen.SessionEvent) {
	ev.Ts, ev.Seq = gen.Timestamp{Time: w.o.Now().UTC()}, 0
	w.publish(StreamKey{Kind: StreamSession, ID: string(ev.SessionID)}, ev)
}
