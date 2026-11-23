package main

import (
	"reflect"
	"slices"
	"testing"
)

type machine struct {
	States      []string            `yaml:"states"`
	Terminal    []string            `yaml:"terminal"`
	Transitions map[string][]string `yaml:"transitions"`
}

// enumOf 取 schema 文件 $defs/name 的 enum。
func enumOf(t *testing.T, s *Set, file, name string) []string {
	t.Helper()
	defs, _ := s.Raw[file]["$defs"].(map[string]any)
	def, _ := defs[name].(map[string]any)
	raw, ok := def["enum"].([]any)
	if !ok {
		t.Fatalf("%s 里没有 $defs/%s 的 enum", file, name)
	}
	out := make([]string, len(raw))
	for i, v := range raw {
		out[i], _ = v.(string)
	}
	return out
}

var (
	runStates  = []string{"pending", "running", "paused", "done", "failed", "cancelled"}
	nodeStates = []string{"idle", "queued", "running", "waiting", "done", "failed", "cancelled", "skipped", "cached"}
)

func TestStateEnums(t *testing.T) {
	t.Run("C07-T1 枚举", func(t *testing.T) {
		s := repoSet(t)
		if got := enumOf(t, s, "common.schema.json", "RunStatus"); !reflect.DeepEqual(got, runStates) {
			t.Errorf("RunStatus：%v，应为 %v（C.07 §2）", got, runStates)
		}
		if got := enumOf(t, s, "common.schema.json", "NodeState"); !reflect.DeepEqual(got, nodeStates) {
			t.Errorf("NodeState：%v，应为 %v（C.07 §3）", got, nodeStates)
		}
		mustValid(t, s, "common.schema.json#/$defs/RunStatus", `"done"`)
		mustInvalid(t, s, "common.schema.json#/$defs/RunStatus", `"succeeded"`, "") // C.07-R1：旧名取消
		mustInvalid(t, s, "common.schema.json#/$defs/NodeState", `"approval"`, "")  // C.07-R2：等待审批不是状态
	})
}

func TestStateMachine(t *testing.T) {
	t.Run("C07-T2 状态机表", func(t *testing.T) {
		var sm struct {
			Run  machine `yaml:"run"`
			Node machine `yaml:"node"`
		}
		readYAML(t, "schema/builtin/state_machine.yaml", &sm)
		// 期望表照 C.07 §2、§3.1 逐格写出：want[从][到] 为 true 表示允许
		allow := func(pairs ...[2]string) map[string]map[string]bool {
			m := map[string]map[string]bool{}
			for _, p := range pairs {
				if m[p[0]] == nil {
					m[p[0]] = map[string]bool{}
				}
				m[p[0]][p[1]] = true
			}
			return m
		}
		cases := []struct {
			name     string
			m        machine
			states   []string
			terminal []string
			want     map[string]map[string]bool
		}{
			{"run", sm.Run, runStates, []string{"done", "failed", "cancelled"}, allow(
				[2]string{"pending", "running"}, [2]string{"pending", "cancelled"},
				[2]string{"running", "paused"}, [2]string{"running", "done"}, [2]string{"running", "failed"}, [2]string{"running", "cancelled"},
				[2]string{"paused", "running"}, [2]string{"paused", "cancelled"},
			)},
			{"node", sm.Node, nodeStates, []string{"done", "failed", "cancelled", "skipped", "cached"}, allow(
				[2]string{"idle", "queued"}, [2]string{"idle", "cancelled"}, [2]string{"idle", "skipped"}, [2]string{"idle", "cached"},
				[2]string{"queued", "running"}, [2]string{"queued", "cancelled"},
				[2]string{"running", "queued"}, [2]string{"running", "waiting"}, [2]string{"running", "done"}, [2]string{"running", "failed"}, [2]string{"running", "cancelled"},
				[2]string{"waiting", "running"}, [2]string{"waiting", "failed"}, [2]string{"waiting", "cancelled"},
				[2]string{"failed", "queued"},
			)},
		}
		for _, c := range cases {
			if !reflect.DeepEqual(c.m.States, c.states) {
				t.Errorf("%s.states：%v，应与枚举同序 %v（C.07-R9）", c.name, c.m.States, c.states)
			}
			if !reflect.DeepEqual(c.m.Terminal, c.terminal) {
				t.Errorf("%s.terminal：%v，应为 %v", c.name, c.m.Terminal, c.terminal)
			}
			for from := range c.m.Transitions {
				if !slices.Contains(c.states, from) {
					t.Errorf("%s.transitions 里有不认识的状态 %s", c.name, from)
				}
			}
			for _, from := range c.states { // 逐格比较（6×6、9×9）
				for _, to := range c.states {
					got := slices.Contains(c.m.Transitions[from], to)
					if got != c.want[from][to] {
						t.Errorf("%s：%s → %s 应为 %v，表里是 %v", c.name, from, to, c.want[from][to], got)
					}
				}
				for _, to := range c.m.Transitions[from] {
					if !slices.Contains(c.states, to) {
						t.Errorf("%s：%s → 不认识的状态 %s", c.name, from, to)
					}
				}
				retry := c.name == "node" && from == "failed"
				if slices.Contains(c.terminal, from) && len(c.m.Transitions[from]) > 0 && !retry {
					t.Errorf("%s：终态 %s 不应当有出边（只有节点 failed → queued 例外）", c.name, from)
				}
			}
		}
	})
}
