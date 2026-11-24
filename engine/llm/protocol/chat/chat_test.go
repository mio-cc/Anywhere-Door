package chat

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

// serve 起一个返回固定响应的 httptest，返回指向它的 Client。
func serve(t *testing.T, status int, header map[string]string, body string, logs io.Writer) llm.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for k, v := range header {
			w.Header().Set(k, v)
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
	if err := reg.SetProviders([]llm.ProviderConfig{{Name: "t", Protocol: llm.ProtoChat, BaseURL: srv.URL + "/v1", Auth: llm.Auth{Type: llm.AuthNone}}}); err != nil {
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

func sse(chunks ...string) string {
	var b strings.Builder
	for _, c := range chunks {
		b.WriteString("data: " + c + "\n\n")
	}
	return b.String()
}

func TestChat(t *testing.T) {
	t.Run("E1002-T1 契约测试", func(t *testing.T) {
		contract.Run(t, contract.Suite{
			Protocol: New(),
			Provider: llm.ProviderConfig{
				Name: "mistral", Protocol: llm.ProtoChat, BaseURL: "https://api.mistral.ai/v1",
				Auth:     llm.Auth{Type: llm.AuthBearer, Secret: "mistral"},
				Models:   []llm.ModelEntry{{ID: "small", Upstream: "mistral-small-latest"}},
				Override: []llm.BodyOp{{Op: llm.OpDelete, Path: "stream_options"}},
			},
			Model:     "small",
			Cassettes: filepath.Join("testdata", "cassettes"),
			Caps:      table(t),
			Transport: cassette.Transport,
		})
	})

	t.Run("E1002-T2 流式拼接", func(t *testing.T) {
		// 工具参数分 5 片到达
		body := sse(
			`{"model":"m-1","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}`,
			`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"ci"}}]}}]}`,
			`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ty\":"}}]}}]}`,
			`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"Par"}}]}}]}`,
			`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"is\""}}]}}]}`,
			`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"}"}}]}}]}`,
			`{"model":"m-1-2026","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		) + "data: [DONE]\n\n"
		r, err := complete(serve(t, 200, nil, body, nil))
		want := []llm.ToolCall{{ID: "call_a", Name: "get_weather", Args: json.RawMessage(`{"city":"Paris"}`)}}
		if err != nil || !reflect.DeepEqual(r.ToolCalls, want) || r.Finish != llm.FinishToolCalls || r.Model != "m-1-2026" {
			t.Errorf("5 片参数拼接：%+v %v", r, err)
		}

		// 两个工具按 index 交错；文本和思考
		body = sse(
			`{"choices":[{"index":0,"delta":{"content":"先","reasoning_content":"想"}}]}`,
			`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"a","function":{"name":"f","arguments":"{\"x\":"}}]}}]}`,
			`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"b","function":{"name":"g","arguments":"{\"y\":"}}]}}]}`,
			`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"1}"}}]}}]}`,
			`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"function":{"arguments":"2}"}}]}}]}`,
			`{"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		) + "data: [DONE]\n\n"
		r, err = complete(serve(t, 200, nil, body, nil))
		want = []llm.ToolCall{{ID: "a", Name: "f", Args: json.RawMessage(`{"x":1}`)}, {ID: "b", Name: "g", Args: json.RawMessage(`{"y":2}`)}}
		if err != nil || !reflect.DeepEqual(r.ToolCalls, want) || r.Text != "先" || r.Thinking != "想" {
			t.Errorf("两个工具交错：%+v %v", r, err)
		}

		// 数组形式的 content（文本块和思考块）
		body = sse(`{"choices":[{"index":0,"delta":{"content":[{"type":"thinking","thinking":[{"type":"text","text":"嗯"}]},{"type":"text","text":"好"}]},"finish_reason":"stop"}]}`) + "data: [DONE]\n\n"
		if r, err := complete(serve(t, 200, nil, body, nil)); err != nil || r.Text != "好" || r.Thinking != "嗯" || r.Finish != llm.FinishStop {
			t.Errorf("数组形式的 content：%+v %v", r, err)
		}

		// [DONE] 和 finish_reason 之前断开 → transient
		var le *llm.Error
		body = sse(`{"choices":[{"index":0,"delta":{"content":"半"}}]}`)
		if _, err := complete(serve(t, 200, nil, body, nil)); !errors.As(err, &le) || le.Class != gen.FailureClassTransient {
			t.Errorf("断开应当是 transient：%v", err)
		}
		// 有 finish_reason、没有 [DONE]：照常结束
		body = sse(`{"choices":[{"index":0,"delta":{"content":"完"},"finish_reason":"length"}]}`)
		if r, err := complete(serve(t, 200, nil, body, nil)); err != nil || r.Finish != llm.FinishLength {
			t.Errorf("只有 finish_reason：%+v %v", r, err)
		}
		// chunk 不是合法 JSON → internal，写明第几个事件
		body = sse(`{"choices":[{"index":0,"delta":{"content":"a"}}]}`, `{oops`)
		if _, err := complete(serve(t, 200, nil, body, nil)); !errors.As(err, &le) || le.Class != gen.FailureClassInternal || !strings.Contains(le.Message, "第 2 个事件") {
			t.Errorf("坏 chunk：%v", err)
		}
	})

	t.Run("E1002-T3 用量映射", func(t *testing.T) {
		body := sse(
			`{"choices":[{"index":0,"delta":{"content":"a"},"finish_reason":"stop"}]}`,
			`{"choices":[],"usage":{"prompt_tokens":30,"completion_tokens":7,"total_tokens":37,"prompt_tokens_details":{"cached_tokens":12},"completion_tokens_details":{"reasoning_tokens":5}}}`,
		) + "data: [DONE]\n\n"
		if r, err := complete(serve(t, 200, nil, body, nil)); err != nil || r.Usage != (llm.Usage{In: 30, Cached: 12, Out: 7, Reasoning: 5}) {
			t.Errorf("用量：%+v %v", r, err)
		}
		var logs bytes.Buffer
		body = sse(`{"choices":[{"index":0,"delta":{"content":"a"},"finish_reason":"stop"}]}`) + "data: [DONE]\n\n"
		if r, err := complete(serve(t, 200, nil, body, &logs)); err != nil || r.Usage != (llm.Usage{}) || !strings.Contains(logs.String(), "没有返回用量") {
			t.Errorf("没有用量 → 0 并记 warn：%+v %v %q", r, err, logs.String())
		}
	})

	t.Run("E1002-T4 错误映射", func(t *testing.T) {
		cases := []struct {
			status    int
			body      string
			code, msg string
			class     gen.FailureClass
		}{
			{400, `{"error":{"message":"too long","type":"invalid_request_error","param":null,"code":"context_length_exceeded"}}`, "context_length_exceeded", "too long", gen.FailureClassUserInput},
			{401, `{"error":{"message":"no key","type":"authentication_error","code":null}}`, "authentication_error", "no key", gen.FailureClassAuthDenied},
			{400, `{"error":{"message":"Output blocked by content filter","type":"invalid_request_error"}}`, "invalid_request_error", "Output blocked by content filter", gen.FailureClassStructural},
			{422, `{"object":"error","message":"Extra inputs are not permitted","type":"invalid_request_error","code":null}`, "invalid_request_error", "Extra inputs are not permitted", gen.FailureClassUserInput},
			{503, `{"detail":"Service unavailable"}`, "", "Service unavailable", gen.FailureClassTransient},
		}
		for _, tc := range cases {
			var le *llm.Error
			_, err := complete(serve(t, tc.status, map[string]string{"Content-Type": "application/json"}, tc.body, nil))
			if !errors.As(err, &le) || le.Code != tc.code || le.Message != tc.msg || le.Class != tc.class {
				t.Errorf("%d %s：想要 %s / %q / %s，实际 %v", tc.status, tc.body, tc.code, tc.msg, tc.class, err)
			}
		}
		// 流里的错误 chunk
		var le *llm.Error
		body := sse(`{"choices":[{"index":0,"delta":{"content":"a"}}]}`, `{"error":{"message":"overloaded","type":"server_error"}}`)
		if _, err := complete(serve(t, 200, nil, body, nil)); !errors.As(err, &le) || le.Code != "server_error" || le.Message != "overloaded" {
			t.Errorf("流里的错误 chunk：%v", err)
		}
	})

	t.Run("E1002-T5 请求编码", func(t *testing.T) {
		half, top := float32(0.5), float32(0.9)
		req := llm.Req{
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Parts: []llm.Part{{Type: llm.PartText, Text: "S"}}},
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
			got := roundTrip(t, body)
			b, err := os.ReadFile(filepath.Join("testdata", "encode", tc.file))
			if err != nil {
				t.Fatal(err)
			}
			var want any
			if err := json.Unmarshal(b, &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				g, _ := json.MarshalIndent(got, "", "  ")
				t.Errorf("%s：编码结果不一致，实际：\n%s", tc.file, g)
			}
		}
		// 不认识的内容类型报错
		if _, err := New().Encode(llm.Req{Messages: []llm.Message{{Role: llm.RoleUser, Parts: []llm.Part{{Type: llm.PartFile, Mime: "application/pdf"}, {Type: llm.PartText}}}}}, "m", llm.Caps{}); err == nil {
			t.Error("chat_completions 不支持文件内容，应当报错")
		}
	})
}

func roundTrip(t *testing.T, body map[string]any) any {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}
