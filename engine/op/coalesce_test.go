package op_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/mio-cc/anywhere-door/gen"
)

func dragKey(n int) string { return fmt.Sprintf("drag:01JCK%021d", n) }

func stepKey(n int) string { return fmt.Sprintf("step:01JCK%021d", n) }

// drag 是带 coalesce_key 的 move_nodes：把这些节点都移到 (x, x)。
func drag(key string, x int64, nodes ...int) gen.Op {
	var mv []gen.PayloadMoveNodesMovesItem
	for _, n := range nodes {
		mv = append(mv, gen.PayloadMoveNodesMovesItem{ID: nid(n), To: gen.Pos{X: x, Y: x}})
	}
	o := mkop(gen.OpMoveNodes, gen.PayloadMoveNodes{Moves: mv})
	o.CoalesceKey = &key
	return o
}

func countKind(evs []gen.GraphEvent, k gen.GraphEventKind) int {
	n := 0
	for _, e := range evs {
		if e.Kind == k {
			n++
		}
	}
	return n
}

func TestCoalesce(t *testing.T) {
	step := 250 * time.Millisecond

	t.Run("E0504-T1 60 次合并", func(t *testing.T) {
		h := setup(t, "mem")
		h.must(user, addNode(1, "资源", "root"))
		orig := h.doc().Root.Nodes[0].Pos
		k := dragKey(1)
		for i := 1; i <= 60; i++ {
			h.clock.Add(step)
			h.must(user, drag(k, int64(i*10), 1))
		}
		if n := len(h.records()); n != 2 {
			t.Fatalf("历史里应当只有添加和一条移动，实际 %d 条", n)
		}
		if v := h.head(); v != 2 {
			t.Errorf("版本号应当不变（2），实际 %d", v)
		}
		if n := countKind(h.bus.all(), gen.EvOpCoalesced); n != 59 {
			t.Errorf("应当有 59 个 op.coalesced，实际 %d 个", n)
		}
		if pos := h.doc().Root.Nodes[0].Pos; pos.X != 600 {
			t.Errorf("应当停在最后的位置：%+v", pos)
		}
		if g, _, err := h.reopen().Read(ctx, h.gid, "main", 2); err != nil || g.Doc().Root.Nodes[0].Pos.X != 600 {
			t.Errorf("重放合并后的日志应当在最后的位置：%v", err)
		}
		h.undo(user)
		if pos := h.doc().Root.Nodes[0].Pos; pos != orig {
			t.Errorf("撤销一次应当回到原位（C0504-T3）：%+v，原来 %+v", pos, orig)
		}
	})

	t.Run("E0504-T2 超时不合并", func(t *testing.T) {
		h := setup(t, "mem")
		h.must(user, addNode(1, "资源", "root"))
		k := dragKey(2)
		h.must(user, drag(k, 10, 1))
		h.clock.Add(2 * time.Second)
		h.must(user, drag(k, 20, 1))
		if n := len(h.records()); n != 3 {
			t.Errorf("间隔 2s 应当是两条移动，实际共 %d 条", n)
		}
	})

	t.Run("E0504-T3 跨操作者不合并", func(t *testing.T) {
		h := setup(t, "mem")
		h.must(user, addNode(1, "资源", "root"))
		k := dragKey(3)
		h.must(user, drag(k, 10, 1))
		h.clock.Add(step)
		h.must(model, withReason(drag(k, 20, 1), "整理布局"))
		h.clock.Add(step)
		h.must(user, drag(k, 30, 1))
		if n := len(h.records()); n != 4 {
			t.Errorf("用户和模型交替应当各是各的，实际共 %d 条", n)
		}
	})

	t.Run("E0504-T4 其他不合并", func(t *testing.T) {
		h := setup(t, "mem")
		h.must(user, addNode(1, "资源", "root"), addNode(2, "资源", "root"))
		k := dragKey(4)
		h.must(user, drag(k, 10, 1))
		h.clock.Add(step)
		h.must(user, setLabel(1, "甲"))
		h.clock.Add(step)
		h.must(user, drag(k, 20, 1))
		if n := len(h.records()); n != 5 { // 添加两条（一批两条 Op 是两条记录）、移动、改名、移动
			t.Errorf("中间插进别的 Op 后不合并，应当共 5 条，实际 %d 条", n)
		}
		k2 := dragKey(5)
		h.must(user, drag(k2, 30, 1))
		h.clock.Add(step)
		h.must(user, drag(k2, 40, 1, 2))
		if n := len(h.records()); n != 7 {
			t.Errorf("节点集合不同不合并，应当共 7 条，实际 %d 条", n)
		}

		// set_param 让动态端口消失时不合并
		h.must(user, addNode(3, "资源", "root", "kind", "提示词", "template", "{{a}}"), addNode(4, "资源", "root"))
		h.must(user, connect(1, 4, "ref", 3, "a"))
		ks := stepKey(6)
		o1 := setParam(3, "template", "{{a}} 和其他")
		o1.CoalesceKey = &ks
		h.must(user, o1)
		before := len(h.records())
		h.clock.Add(step)
		o2 := setParam(3, "template", "没有变量")
		o2.CoalesceKey = &ks
		h.must(user, o2)
		if n := len(h.records()); n != before+1 {
			t.Errorf("动态端口消失时应当是一条新的 Op，实际 %d → %d", before, n)
		}
		if n := len(h.doc().Root.Edges); n != 0 {
			t.Errorf("连到消失端口的线应当删掉：%d", n)
		}
		h.clock.Add(step)
		o3 := setParam(3, "template", "还是没有变量")
		o3.CoalesceKey = &ks
		h.must(user, o3)
		if n := len(h.records()); n != before+1 {
			t.Errorf("端口不变的 set_param 应当合并进上一条：%d", n)
		}
	})

	t.Run("E0504-T5 合并后重放", func(t *testing.T) {
		h := setup(t, "mem")
		h.must(user, addNode(1, "资源", "root"))
		for i := 2; i <= 49; i++ {
			h.must(user, setLabel(1, fmt.Sprintf("标签 %d", i)))
		}
		k := dragKey(7)
		if res := h.must(user, drag(k, 10, 1)); res.Version != 50 {
			t.Fatalf("拖动应当是第 50 版，实际 %d", res.Version)
		}
		for i := 0; i < 3; i++ {
			h.clock.Add(step)
			h.must(user, drag(k, int64(20+i), 1))
		}
		merged := asJSON(h.doc())
		h.must(user, setLabel(1, "最后"))
		last := asJSON(h.doc())
		if s, err := h.s.Graphs().Base(ctx, h.gid, "main", 50); err != nil || s.Version != 0 {
			t.Errorf("带 coalesce_key 的第 50 版不存全文：%d %v", s.Version, err)
		}
		if s, err := h.s.Graphs().Base(ctx, h.gid, "main", 51); err != nil || s.Version != 51 {
			t.Errorf("下一条普通 Op（第 51 版）存全文：%d %v", s.Version, err)
		}
		p2 := h.reopen()
		if g, _, err := p2.Read(ctx, h.gid, "main", 50); err != nil || asJSON(g.Doc()) != merged {
			t.Errorf("清掉缓存后第 50 版应当是合并后的图：%v", err)
		}
		if g, _, err := p2.Read(ctx, h.gid, "main", 51); err != nil || asJSON(g.Doc()) != last {
			t.Errorf("清掉缓存后第 51 版不对：%v", err)
		}
	})
}
