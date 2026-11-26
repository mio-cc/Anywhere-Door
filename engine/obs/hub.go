package obs

import (
	"sync"
	"sync/atomic"
)

// 流的种类。
const (
	StreamRun     = "run"
	StreamSession = "session"
)

// StreamKey 是一条流：运行流或会话流。
type StreamKey struct{ Kind, ID string }

// Hub 把落库后的事件广播给订阅者（SSE，B.05）。只发实时的；补发历史由订阅者从存储读（B.05.02）。
type Hub struct {
	mu   sync.Mutex
	buf  int
	subs map[StreamKey]map[*Sub]struct{}
}

// Sub 是一个订阅：C 收到 gen.RunEvent 或 gen.SessionEvent；被断开或 Close 后 C 关闭。
type Sub struct {
	C      <-chan any
	c      chan any
	h      *Hub
	k      StreamKey
	lagged atomic.Bool
}

// NewHub：buffer 是每个订阅者的缓冲，≤ 0 时为 256。
func NewHub(buffer int) *Hub {
	if buffer <= 0 {
		buffer = 256
	}
	return &Hub{buf: buffer, subs: map[StreamKey]map[*Sub]struct{}{}}
}

// Subscribe 订阅一条流。
func (h *Hub) Subscribe(k StreamKey) *Sub {
	c := make(chan any, h.buf)
	s := &Sub{C: c, c: c, h: h, k: k}
	h.mu.Lock()
	defer h.mu.Unlock()
	m := h.subs[k]
	if m == nil {
		m = map[*Sub]struct{}{}
		h.subs[k] = m
	}
	m[s] = struct{}{}
	return s
}

// drop 在锁里调用：去掉订阅并关闭 channel（只关一次）。
func (h *Hub) drop(s *Sub) {
	m := h.subs[s.k]
	if _, ok := m[s]; !ok {
		return
	}
	delete(m, s)
	if len(m) == 0 {
		delete(h.subs, s.k)
	}
	close(s.c)
}

// Publish 发给这条流的所有订阅者；缓冲满的订阅者被断开（Lagged），不阻塞写入。
func (h *Hub) Publish(k StreamKey, ev any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs[k] {
		select {
		case s.c <- ev:
		default:
			s.lagged.Store(true)
			h.drop(s)
		}
	}
}

// Close 取消订阅。
func (s *Sub) Close() {
	s.h.mu.Lock()
	defer s.h.mu.Unlock()
	s.h.drop(s)
}

// Lagged：订阅者太慢、缓冲满了被断开，要重新同步（B.05）。
func (s *Sub) Lagged() bool { return s.lagged.Load() }
