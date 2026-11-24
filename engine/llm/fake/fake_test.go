package fake

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/mio-cc/anywhere-door/engine/llm"
	"github.com/mio-cc/anywhere-door/gen"
)

func TestFake(t *testing.T) {
	t.Run("E1008-T4 fake 脚本", func(t *testing.T) {
		rate := &llm.Error{Class: gen.FailureClassRateLimited, Status: 429, Code: "rate", Message: "慢点"}
		c := New(llm.Caps{Tools: true},
			Step{Deltas: []llm.Delta{{Text: "你"}, {Text: "好"}, {ToolCall: &llm.ToolCallDelta{Index: 0, ID: "c1", Name: "f", Args: `{"a":`}}, {ToolCall: &llm.ToolCallDelta{Index: 0, Args: `1}`}}}},
			Step{Resp: &llm.Resp{Text: "直接给结果", Finish: llm.FinishStop, Usage: llm.Usage{In: 3, Out: 4}}},
			Step{Err: rate},
			Step{Deltas: []llm.Delta{{Text: "半"}}, Err: &llm.Error{Class: gen.FailureClassTransient, Message: "断了"}},
		)
		if !c.Caps().Tools {
			t.Error("Caps 应当是建的时候给的")
		}
		ctx := context.Background()

		st, err := c.Complete(ctx, llm.Req{Model: "a"})
		if err != nil {
			t.Fatal(err)
		}
		r, err := llm.Collect(st)
		if err != nil {
			t.Fatal(err)
		}
		want := []llm.ToolCall{{ID: "c1", Name: "f", Args: json.RawMessage(`{"a":1}`)}}
		if r.Text != "你好" || !reflect.DeepEqual(r.ToolCalls, want) || r.Finish != llm.FinishToolCalls {
			t.Errorf("按 Deltas 返回、最后补 Done：%+v", r)
		}

		st, err = c.Complete(ctx, llm.Req{Model: "b"})
		if err != nil {
			t.Fatal(err)
		}
		if r, err := llm.Collect(st); err != nil || r.Text != "直接给结果" || r.Usage.Out != 4 {
			t.Errorf("只给 Resp：%+v %v", r, err)
		}

		if _, err := c.Complete(ctx, llm.Req{Model: "c"}); !errors.Is(err, rate) {
			t.Errorf("Err 应当原样返回：%v", err)
		}

		st, err = c.Complete(ctx, llm.Req{Model: "d"})
		if err != nil {
			t.Fatal(err)
		}
		var le *llm.Error
		if _, err := llm.Collect(st); !errors.As(err, &le) || le.Class != gen.FailureClassTransient {
			t.Errorf("有 Deltas 时吐完再返回 Err：%v", err)
		}

		// 脚本用完后再调用 → 错误
		if _, err := c.Complete(ctx, llm.Req{Model: "e"}); !errors.As(err, &le) || le.Code != "fake_exhausted" {
			t.Errorf("脚本用完后应当报错：%v", err)
		}
		reqs := c.Requests()
		if len(reqs) != 5 || reqs[0].Model != "a" || reqs[4].Model != "e" {
			t.Errorf("Requests 应当记下每次的请求：%+v", reqs)
		}
	})
}
