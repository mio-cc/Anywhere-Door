package responses

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mio-cc/anywhere-door/engine/llm"
	"github.com/mio-cc/anywhere-door/engine/llm/cassette"
	"github.com/mio-cc/anywhere-door/engine/llm/contract"
	"github.com/mio-cc/anywhere-door/engine/llm/quirks"
	"github.com/mio-cc/anywhere-door/gen"
)

func table(t *testing.T) quirks.Table {
	t.Helper()
	tb, err := quirks.Load(quirks.Files)
	if err != nil {
		t.Fatal(err)
	}
	return tb
}

func serve(t *testing.T, status int, body string, logs io.Writer) llm.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if status == http.StatusOK {
			w.Header().Set("Content-Type", "text/event-stream")
		} else {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	o := llm.Options{}
	if logs != nil {
		o.Logger = slog.New(slog.NewTextHandler(logs, nil))
	}
	reg := llm.NewRegistry([]llm.Protocol{New()}, table(t), nil, o)
	if err := reg.SetProviders([]llm.ProviderConfig{{Name: "t", Protocol: llm.ProtoResponses, BaseURL: srv.URL + "/v1", Auth: llm.Auth{Type: llm.AuthNone}}}); err != nil {
		t.Fatal(err)
	}
	c, err := reg.Client("t", "m-1")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func complete(c llm.Client) (*llm.Resp, error) {
	st, err := c.Complete(context.Background(), llm.Req{Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{{Type: llm.PartText, Text: "hi"}}}}})
	if err != nil {
		return nil, err
	}
	return llm.Collect(st)
}

// sse 按 type 写出 event: 行和 data: 行。
func sse(events ...string) string {
	var b strings.Builder
	for _, e := range events {
		var m struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal([]byte(e), &m)
		b.WriteString("event: " + m.Type + "\ndata: " + e + "\n\n")
	}
	return b.String()
}

const completed = `{"type":"response.completed","response":{"model":"m-1-2026","status":"completed","usage":{"input_tokens":10,"output_tokens":5}}}`

func TestResponses(t *testing.T) {
	t.Run("E1004-T1 契约测试", func(t *testing.T) {
		contract.Run(t, contract.Suite{
			Protocol: New(),
			Provider: llm.ProviderConfig{
				Name: "mistral", Protocol: llm.ProtoResponses, BaseURL: "https://api.mistral.ai/v1",
				Auth:   llm.Auth{Type: llm.AuthBearer, Secret: "mistral"},
				Models: []llm.ModelEntry{{ID: "small", Upstream: "mistral-small-latest"}},
			},
			Model:     "small",
			Cassettes: filepath.Join("testdata", "cassettes"),
			Caps:      table(t),
			Transport: cassette.Transport,
		})
	})

	t.Run("E1004-T2 流式拼接", func(t *testing.T) {
		body := sse(
			`{"type":"response.created","response":{"model":"m-1","status":"in_progress","usage":null}}`,
			`{"type":"response.output_text.delta","output_index":0,"delta":"先"}`,
			`{"type":"response.reasoning_summary_text.delta","output_index":0,"delta":"想"}`,
			`{"type":"response.output_item.added","output_index":1,"item":{"type":"function_call","id":"fc_1","call_id":"call_a","name":"f","arguments":""}}`,
			`{"type":"response.output_item.added","output_index":2,"item":{"type":"function_call","id":"fc_2","call_id":"call_b","name":"g","arguments":""}}`,
			`{"type":"response.function_call_arguments.delta","output_index":1,"delta":"{\"x\":"}`,
			`{"type":"response.something_new","output_index":1,"delta":"不该出现"}`,
			`{"type":"response.function_call_arguments.delta","output_index":2,"delta":"{\"y\":"}`,
			`{"type":"response.function_call_arguments.delta","output_index":1,"delta":"1}"}`,
			`{"type":"response.function_call_arguments.delta","output_index":2,"delta":"2}"}`,
			`{"type":"response.function_call_arguments.done","output_index":1,"arguments":"{\"x\":1}"}`,
			`{"type":"response.output_item.done","output_index":1,"item":{"type":"function_call","call_id":"call_a","name":"f","arguments":"{\"x\":1}"}}`,
			completed,
		)
		r, err := complete(serve(t, 200, body, nil))
		want := []llm.ToolCall{{ID: "call_a", Name: "f", Args: json.RawMessage(`{"x":1}`)}, {ID: "call_b", Name: "g", Args: json.RawMessage(`{"y":2}`)}}
		if err != nil || !reflect.DeepEqual(r.ToolCalls, want) || r.Text != "先" || r.Thinking != "想" || r.Finish != llm.FinishToolCalls || r.Model != "m-1-2026" {
			t.Errorf("按 output_index 拼接、忽略不认识的事件：%+v %v", r, err)
		}

		// 只在 output_item.done 里给完整参数
		body = sse(`{"type":"response.output_item.done","output_index":0,"item":{"type":"function_call","call_id":"c","name":"h","arguments":"{\"z\":3}"}}`, completed)
		if r, err := complete(serve(t, 200, body, nil)); err != nil || len(r.ToolCalls) != 1 || string(r.ToolCalls[0].Args) != `{"z":3}` || r.ToolCalls[0].ID != "c" {
			t.Errorf("output_item.done 给完整参数：%+v %v", r, err)
		}

		// completed 前断开 → transient
		var le *llm.Error
		body = sse(`{"type":"response.output_text.delta","output_index":0,"delta":"半"}`)
		if _, err := complete(serve(t, 200, body, nil)); !errors.As(err, &le) || le.Class != gen.FailureClassTransient {
			t.Errorf("断开应当是 transient：%v", err)
		}
		body = "data: {oops\n\n"
		if _, err := complete(serve(t, 200, body, nil)); !errors.As(err, &le) || le.Class != gen.FailureClassInternal {
			t.Errorf("事件不是合法 JSON 应当是 internal：%v", err)
		}
	})

	t.Run("E1004-T3 用量映射", func(t *testing.T) {
		body := sse(`{"type":"response.output_text.delta","delta":"a"}`,
			`{"type":"response.completed","response":{"model":"m","usage":{"input_tokens":40,"input_tokens_details":{"cached_tokens":32},"output_tokens":9,"output_tokens_details":{"reasoning_tokens":6},"total_tokens":49}}}`)
		if r, err := complete(serve(t, 200, body, nil)); err != nil || r.Usage != (llm.Usage{In: 40, Cached: 32, Out: 9, Reasoning: 6}) {
			t.Errorf("用量：%+v %v", r, err)
		}
		var logs bytes.Buffer
		body = sse(`{"type":"response.output_text.delta","delta":"a"}`, `{"type":"response.completed","response":{"model":"m"}}`)
		if r, err := complete(serve(t, 200, body, &logs)); err != nil || r.Usage != (llm.Usage{}) || !strings.Contains(logs.String(), "没有返回用量") {
			t.Errorf("没有用量 → 0 并记 warn：%+v %v", r, err)
		}
	})

	t.Run("E1004-T4 结束与错误", func(t *testing.T) {
		for reason, want := range map[string]string{"max_output_tokens": llm.FinishLength, "content_filter": llm.FinishFiltered} {
			body := sse(`{"type":"response.output_text.delta","delta":"a"}`,
				`{"type":"response.incomplete","response":{"model":"m","incomplete_details":{"reason":"`+reason+`"},"usage":{"input_tokens":1,"output_tokens":1}}}`)
			if r, err := complete(serve(t, 200, body, nil)); err != nil || r.Finish != want || r.Text != "a" {
				t.Errorf("incomplete %s 应当是 %s：%+v %v", reason, want, r, err)
			}
		}
		var le *llm.Error
		body := sse(`{"type":"response.failed","response":{"status":"failed","error":{"code":"server_error","message":"The model failed"}}}`)
		if _, err := complete(serve(t, 200, body, nil)); !errors.As(err, &le) || le.Code != "server_error" || le.Message != "The model failed" || le.Class != gen.FailureClassTransient {
			t.Errorf("response.failed 按 error.code 分类：%v", err)
		}
		body = sse(`{"type":"error","code":"rate_limit_exceeded","message":"slow down","param":null}`)
		if _, err := complete(serve(t, 200, body, nil)); !errors.As(err, &le) || le.Code != "rate_limit_exceeded" || le.Class != gen.FailureClassRateLimited {
			t.Errorf("error 事件：%v", err)
		}
		if _, err := complete(serve(t, 400, `{"error":{"message":"bad input","type":"invalid_request_error","param":"input","code":null}}`, nil)); !errors.As(err, &le) || le.Code != "invalid_request_error" || le.Message != "bad input" || le.Class != gen.FailureClassUserInput {
			t.Errorf("非 2xx 错误体：%v", err)
		}
	})

	t.Run("E1004-T5 请求编码", func(t *testing.T) {
		half, top := float32(0.5), float32(0.9)
		req := llm.Req{
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Parts: []llm.Part{{Type: llm.PartText, Text: "S"}}},
				{Role: llm.RoleSystem, Parts: []llm.Part{{Type: llm.PartText, Text: "S2"}}},
				{Role: llm.RoleUser, Parts: []llm.Part{{Type: llm.PartText, Text: "look"}, {Type: llm.PartImage, Mime: "image/png", Data: []byte{1, 2, 3}}}},
				{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "call1", Name: "get_weather", Args: json.RawMessage(`{"city":"Paris"}`)}}},
				{Role: llm.RoleTool, ToolCallID: "call1", Parts: []llm.Part{{Type: llm.PartText, Text: `{"temp":18}`}}},
				{Role: llm.RoleAssistant, Parts: []llm.Part{{Type: llm.PartText, Text: "It is 18."}}},
				{Role: llm.RoleUser, Parts: []llm.Part{{Type: llm.PartText, Text: "thanks"}}},
			},
			Tools: []llm.ToolSpec{
				{Name: "get_weather", Description: "Weather", Schema: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`)},
				{Name: "noop"},
			},
			ToolChoice:  "get_weather",
			Output:      &llm.OutputSpec{Name: "answer", Schema: json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}}}`), Strict: true},
			Temperature: &half, TopP: &top, MaxTokens: 100, Reasoning: "medium", Stop: []string{"END"},
		}
		for _, tc := range []struct {
			file string
			caps llm.Caps
		}{
			{"native.json", llm.Caps{NativeSchema: true, Tools: true, Efforts: []string{"low", "medium", "high"}}},
			{"forced.json", llm.Caps{Tools: true, Efforts: []string{"low", "high"}}},
		} {
			body, err := New().Encode(req, "up-1", tc.caps)
			if err != nil {
				t.Fatal(err)
			}
			b, _ := json.Marshal(body)
			var got, want any
			_ = json.Unmarshal(b, &got)
			wb, err := os.ReadFile(filepath.Join("testdata", "encode", tc.file))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(wb, &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				g, _ := json.MarshalIndent(got, "", "  ")
				t.Errorf("%s：编码结果不一致，实际：\n%s", tc.file, g)
			}
			if m := body; m["store"] != false || m["previous_response_id"] != nil {
				t.Errorf("总是 store: false，没有 previous_response_id：%v", m)
			}
		}
	})
}
