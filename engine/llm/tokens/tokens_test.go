package tokens

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/mio-cc/anywhere-door/engine/llm"
)

func text(role llm.Role, s string) llm.Message {
	return llm.Message{Role: role, Parts: []llm.Part{{Type: llm.PartText, Text: s}}}
}

func TestApprox(t *testing.T) {
	t.Run("E1007-T2 approx 估算", func(t *testing.T) {
		for _, c := range []struct {
			s    string
			want int
		}{{"", 0}, {"hello world", 4}, {"你好世界", 5}, {"你好 world", 5}, {"abcd", 2}} {
			if got := Approx(c.s); got != c.want {
				t.Errorf("Approx(%q) = %d，应当是 %d", c.s, got, c.want)
			}
		}
		msgs := []llm.Message{
			text(llm.RoleSystem, "hello world"),
			{Role: llm.RoleUser, Parts: []llm.Part{{Type: llm.PartText, Text: "你好世界"}, {Type: llm.PartImage, Mime: "image/png"}}},
			{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "1", Name: "abcd", Args: json.RawMessage(`{}`)}}},
		}
		want := (4 + 4) + (4 + 5 + 1000) + (4 + 2 + 1)
		if got := count(msgs, nil); got != want {
			t.Errorf("消息开销、图片、工具调用：%d，应当是 %d", got, want)
		}
		tool := llm.ToolSpec{Name: "search", Description: "搜", Schema: json.RawMessage(`{"type":"object"}`)}
		b, _ := json.Marshal(struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Schema      json.RawMessage `json:"schema,omitempty"`
		}{tool.Name, tool.Description, tool.Schema})
		if got, w := count(nil, []llm.ToolSpec{tool}), int(math.Ceil(float64(len(b))/3.5)); got != w {
			t.Errorf("工具定义按长度 ÷ 3.5：%d，应当是 %d", got, w)
		}
		if For("o200k_base", nil, "m").CountText("hello world") != 4 {
			t.Error("没有校准表时比值为 1")
		}
	})
}

func TestCalibrate(t *testing.T) {
	t.Run("E1007-T3 校准", func(t *testing.T) {
		s := NewStats()
		for i := 0; i < 50; i++ {
			s.Observe("a", 100, 130)
		}
		if r := s.Ratio("a"); r < 1.28 || r > 1.32 {
			t.Errorf("比值 1.3 观察 50 次后应当在 1.28–1.32：%v", r)
		}
		if s.Ratio("b") != 1 {
			t.Error("不同模型互不影响、没有记录时为 1")
		}
		for i := 0; i < 50; i++ {
			s.Observe("hi", 100, 300)
			s.Observe("lo", 100, 10)
		}
		if s.Ratio("hi") != 1.5 || s.Ratio("lo") != 0.7 {
			t.Errorf("比值限制在 0.7–1.5：%v %v", s.Ratio("hi"), s.Ratio("lo"))
		}
		before := s.Ratio("a")
		s.Observe("a", 0, 100)
		s.Observe("a", 100, 0)
		if s.Ratio("a") != before {
			t.Error("估算或实际为 0 时忽略")
		}
		c := For("cl100k_base", s, "hi")
		if got := c.CountText("hello world"); got != 6 {
			t.Errorf("计数随比值变化：4 × 1.5 = 6，实际 %d", got)
		}
		if got := c.Count([]llm.Message{text(llm.RoleUser, "hello world")}, nil); got != int(math.Ceil(8*1.5)) {
			t.Errorf("整次请求也乘比值：%d", got)
		}
	})
}
