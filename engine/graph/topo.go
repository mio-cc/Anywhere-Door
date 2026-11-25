package graph

import (
	"sort"
	"strings"
)

// ErrCycle：子图里有环（INV-06 的判断在 V09，这里只报事实）。
type ErrCycle struct {
	SCCs [][]string // 环上的节点 ID：每个分量内按 ID 排序，分量之间按最小 ID 排序
}

func (e *ErrCycle) Error() string {
	parts := make([]string, len(e.SCCs))
	for i, c := range e.SCCs {
		parts[i] = strings.Join(c, " → ")
	}
	return "graph: 子图里有环：" + strings.Join(parts, "；")
}

// adjacency 是子图内的后继与前驱（只看两端都在本子图的连线，同一对节点只算一次），都按 ID 排序。
func (s *Scope) adjacency() (succ, pred map[string][]string) {
	succ, pred = map[string][]string{}, map[string][]string{}
	seen := map[[2]string]bool{}
	for _, e := range s.edges {
		a, b := string(e.From.Node), string(e.To.Node)
		if !s.nodeSet[a] || !s.nodeSet[b] || seen[[2]string{a, b}] {
			continue
		}
		seen[[2]string{a, b}] = true
		succ[a] = append(succ[a], b)
		pred[b] = append(pred[b], a)
	}
	for _, m := range []map[string][]string{succ, pred} {
		for k := range m {
			sort.Strings(m[k])
		}
	}
	return succ, pred
}

func (s *Scope) ids() []string {
	out := make([]string, len(s.nodes))
	for i, n := range s.nodes {
		out[i] = string(n.ID)
	}
	return out
}

// TopoLevels 按层返回节点 ID（Kahn）：入度为 0 的是第 0 层；同层按 ID 排序。有环时返回 *ErrCycle。
func (s *Scope) TopoLevels() ([][]string, error) {
	succ, pred := s.adjacency()
	indeg := map[string]int{}
	var level []string
	for _, id := range s.ids() {
		indeg[id] = len(pred[id])
		if indeg[id] == 0 {
			level = append(level, id)
		}
	}
	var out [][]string
	done := 0
	for len(level) > 0 {
		out = append(out, level)
		done += len(level)
		var next []string
		for _, id := range level {
			for _, b := range succ[id] {
				indeg[b]--
				if indeg[b] == 0 {
					next = append(next, b)
				}
			}
		}
		sort.Strings(next)
		level = next
	}
	if done < len(s.nodes) {
		return nil, &ErrCycle{SCCs: s.SCCs()}
	}
	return out, nil
}

// SCCs 是强连通分量（Tarjan）：只返回大小 > 1 或有自环的。
func (s *Scope) SCCs() [][]string {
	succ, _ := s.adjacency()
	index, low := map[string]int{}, map[string]int{}
	onStack := map[string]bool{}
	var stack []string
	var out [][]string
	next := 0
	var visit func(v string)
	visit = func(v string) {
		index[v], low[v] = next, next
		next++
		stack = append(stack, v)
		onStack[v] = true
		for _, w := range succ[v] {
			if _, ok := index[w]; !ok {
				visit(w)
				low[v] = min(low[v], low[w])
			} else if onStack[w] {
				low[v] = min(low[v], index[w])
			}
		}
		if low[v] != index[v] {
			return
		}
		var comp []string
		for {
			w := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[w] = false
			comp = append(comp, w)
			if w == v {
				break
			}
		}
		if len(comp) > 1 || hasSelf(succ, v) {
			sort.Strings(comp)
			out = append(out, comp)
		}
	}
	for _, id := range s.ids() {
		if _, ok := index[id]; !ok {
			visit(id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

func hasSelf(succ map[string][]string, v string) bool {
	for _, w := range succ[v] {
		if w == v {
			return true
		}
	}
	return false
}

// reach 是从 start 沿 next 能到的节点（不含 start 本身，除非它在环上也不算），按 ID 排序。
func reach(start string, next map[string][]string) []string {
	seen := map[string]bool{start: true}
	queue := []string{start}
	var out []string
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		for _, w := range next[v] {
			if !seen[w] {
				seen[w] = true
				out = append(out, w)
				queue = append(queue, w)
			}
		}
	}
	sort.Strings(out)
	return out
}

// Upstream 是 nodeID 的所有祖先（按 ID 排序）。
func (s *Scope) Upstream(nodeID string) []string {
	_, pred := s.adjacency()
	return reach(nodeID, pred)
}

// Downstream 是 nodeID 的所有后代（按 ID 排序）。
func (s *Scope) Downstream(nodeID string) []string {
	succ, _ := s.adjacency()
	return reach(nodeID, succ)
}

// Dominated 是只为 nodeID 服务的上游支路（E.02.02 §5）：在 nodeID 的上游里，所有出边最终都只通向 nodeID 的节点。
// 汇合 first、扇出 k_of_n 做软取消用（E.14.06）。按 ID 排序。
func (s *Scope) Dominated(nodeID string) []string {
	succ, pred := s.adjacency()
	up := reach(nodeID, pred)
	in := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for _, u := range up {
			if in[u] || len(succ[u]) == 0 {
				continue
			}
			all := true
			for _, w := range succ[u] {
				if w != nodeID && !in[w] {
					all = false
					break
				}
			}
			if all {
				in[u], changed = true, true
			}
		}
	}
	var out []string
	for _, u := range up {
		if in[u] {
			out = append(out, u)
		}
	}
	return out
}
