package sched

import "container/heap"

// Prio 是就绪队列的优先级（E.15.01 §5，引03 §3.3）：每个字段都是小的优先，按字段顺序比较，最后按入队顺序。
// Escalation：升级派生的写 0、其余 1；Critical：负的下游节点数（关键路径长的优先）；Round：轮次；User：P0–P2。
type Prio struct{ Escalation, Critical, Round, User int }

func (a Prio) less(b Prio) (less, equal bool) {
	for _, x := range [][2]int{{a.Escalation, b.Escalation}, {a.Critical, b.Critical}, {a.Round, b.Round}, {a.User, b.User}} {
		if x[0] != x[1] {
			return x[0] < x[1], false
		}
	}
	return false, true
}

type qitem[T any] struct {
	p   Prio
	seq int64
	v   T
}

type qheap[T any] []qitem[T]

func (h qheap[T]) Len() int { return len(h) }
func (h qheap[T]) Less(i, j int) bool {
	if l, eq := h[i].p.less(h[j].p); !eq {
		return l
	}
	return h[i].seq < h[j].seq
}
func (h qheap[T]) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *qheap[T]) Push(x any)   { *h = append(*h, x.(qitem[T])) }
func (h *qheap[T]) Pop() any {
	old := *h
	it := old[len(old)-1]
	*h = old[:len(old)-1]
	return it
}

// Queue 是就绪队列：优先级小的先出，同优先级按入队顺序。不是并发安全的（调度循环是单写者，E.15-R1）。
type Queue[T any] struct {
	h   qheap[T]
	seq int64
}

// Push 入队。
func (q *Queue[T]) Push(p Prio, v T) {
	q.seq++
	heap.Push(&q.h, qitem[T]{p: p, seq: q.seq, v: v})
}

// Pop 出队；空时 ok 为 false。
func (q *Queue[T]) Pop() (v T, ok bool) {
	if len(q.h) == 0 {
		return v, false
	}
	return heap.Pop(&q.h).(qitem[T]).v, true
}

// Len 是队列长度。
func (q *Queue[T]) Len() int { return len(q.h) }
