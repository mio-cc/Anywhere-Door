package messages

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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

func serve(t *testing.T, status int, body string) llm.Client {
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
	reg := llm.NewRegistry([]llm.Protocol{New()}, table(t), nil, llm.Options{})
	if err := reg.SetProviders([]llm.ProviderConfig{{Name: "t", Protocol: llm.ProtoMessages, BaseURL: srv.URL + "/v1", Auth: llm.Auth{Type: llm.AuthNone}}}); err != nil {
		t.Fatal(err)
	}
	c, err := reg.Client("t", "claude-test-1")
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

const (
	start = `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","content":[],"model":"claude-test-1-2026","stop_reason":null,"usage":{"input_tokens":50,"cache_read_input_tokens":20,"output_tokens":1}}}`
	stop  = `{"type":"message_stop"}`
)

func TestMessages(t *testing.T) {
	t.Run("E1003-T1 契约测试", func(t *testing.T) {
		contract.Run(t, contract.Suite{
			Protocol: New(),
			Provider: llm.ProviderConfig{
				Name: "anthropic", Protocol: llm.ProtoMessages, BaseURL: "https://api.anthropic.com/v1",
				Auth:   llm.Auth{Secret: "anthropic"}, // 缺省鉴权：请求头 x-api-key
				Models: []llm.ModelEntry{{ID: "sonnet", Upstream: "claude-sonnet-4-5"}},
			},
			Model:     "sonnet",
			Cassettes: filepath.Join("testdata", "cassettes"),
			Caps:      table(t),
			Transport: cassette.Transport,
		})
	})

	t.Run("E1003-T2 流式拼接", func(t *testing.T) {
		body := sse(start,
			`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"想一想"}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"sig"}}`,
			`{"type":"content_block_stop","index":0}`,
			`{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
			`{"type": "ping"}`,
			`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"先查"}}`,
			`{"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"toolu_1","name":"get_weather","input":{}}}`,
			`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":""}}`,
			`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"city\": \"Pa"}}`,
			`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"一下"}}`,
			`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"ris\"}"}}`,
			`{"type":"content_block_stop","index":2}`,
			`{"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":33}}`,
			stop)
		r, err := complete(serve(t, 200, body))
		want := []llm.ToolCall{{ID: "toolu_1", Name: "get_weather", Args: json.RawMessage(`{"city":"Paris"}`)}}
		if err != nil || !reflect.DeepEqual(r.ToolCalls, want) || r.Text != "先查一下" || r.Thinking != "想一想" || r.Finish != llm.FinishToolCalls || r.Model != "claude-test-1-2026" {
			t.Errorf("文本块和工具块交错、参数分片：%+v %v", r, err)
		}
		var le *llm.Error
		body = sse(start, `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"半"}}`)
		if _, err := complete(serve(t, 200, body)); !errors.As(err, &le) || le.Class != gen.FailureClassTransient {
			t.Errorf("message_stop 前断开应当是 transient：%v", err)
		}
		for reason, want := range map[string]string{"end_turn": llm.FinishStop, "stop_sequence": llm.FinishStop, "max_tokens": llm.FinishLength, "refusal": llm.FinishFiltered} {
			body = sse(start, `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"a"}}`,
				`{"type":"message_delta","delta":{"stop_reason":"`+reason+`"},"usage":{"output_tokens":1}}`, stop)
			if r, err := complete(serve(t, 200, body)); err != nil || r.Finish != want {
				t.Errorf("stop_reason %s 应当是 %s：%+v %v", reason, want, r, err)
			}
		}
	})

	t.Run("E1003-T3 用量映射", func(t *testing.T) {
		body := sse(start, `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"a"}}`,
			`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":12}}`, stop)
		if r, err := complete(serve(t, 200, body)); err != nil || r.Usage != (llm.Usage{In: 50, Cached: 20, Out: 12}) {
			t.Errorf("输入、缓存读、输出：%+v %v", r, err)
		}
	})

	t.Run("E1003-T4 错误映射", func(t *testing.T) {
		var le *llm.Error
		body := sse(start, `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`)
		if _, err := complete(serve(t, 200, body)); !errors.As(err, &le) || le.Code != "overloaded_error" || le.Message != "Overloaded" || le.Class != gen.FailureClassTransient {
			t.Errorf("error 事件：%v", err)
		}
		cases := []struct {
			status int
			code   string
			class  gen.FailureClass
		}{
			{400, "invalid_request_error", gen.FailureClassUserInput},
			{401, "authentication_error", gen.FailureClassAuthDenied},
			{529, "overloaded_error", gen.FailureClassTransient},
		}
		for _, tc := range cases {
			b := `{"type":"error","error":{"type":"` + tc.code + `","message":"出错了"}}`
			if _, err := complete(serve(t, tc.status, b)); !errors.As(err, &le) || le.Code != tc.code || le.Message != "出错了" || le.Class != tc.class || le.Status != tc.status {
				t.Errorf("%d %s：%v", tc.status, tc.code, err)
			}
		}
		if c, ok := table(t).ErrorClass(llm.ProtoMessages, 529, "", ""); !ok || c != string(gen.FailureClassTransient) {
			t.Errorf("529 经 quirks 规则成 transient：%q %v", c, ok)
		}
	})

	t.Run("E1003-T5 请求编码", func(t *testing.T) {
		half, top := float32(0.5), float32(0.9)
		req := llm.Req{
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Parts: []llm.Part{{Type: llm.PartText, Text: "S"}}, Cache: true},
				{Role: llm.RoleSystem, Parts: []llm.Part{{Type: llm.PartText, Text: "S2"}}},
				{Role: llm.RoleUser, Parts: []llm.Part{{Type: llm.PartText, Text: "look"}, {Type: llm.PartImage, Mime: "image/png", Data: []byte{1, 2, 3}}}},
				{Role: llm.RoleUser, Parts: []llm.Part{{Type: llm.PartText, Text: "more"}}},
				{Role: llm.RoleAssistant, Parts: []llm.Part{{Type: llm.PartText, Text: "checking"}}, ToolCalls: []llm.ToolCall{{ID: "call1", Name: "get_weather", Args: json.RawMessage(`{"city":"Paris"}`)}}},
				{Role: llm.RoleTool, ToolCallID: "call1", Parts: []llm.Part{{Type: llm.PartText, Text: `{"temp":18}`}}},
				{Role: llm.RoleUser, Parts: []llm.Part{{Type: llm.PartText, Text: "thanks"}}, Cache: true},
			},
			Tools: []llm.ToolSpec{
				{Name: "get_weather", Description: "Weather", Schema: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`)},
				{Name: "noop"},
			},
			ToolChoice:  "required",
			Output:      &llm.OutputSpec{Name: "answer", Schema: json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}}}`), Strict: true},
			Temperature: &half, TopP: &top, Reasoning: "medium", Stop: []string{"END"},
		}
		caps := llm.Caps{Tools: true, PromptCache: true, NativeSchema: true, MaxOutput: 8192,
			Efforts: []string{"off", "low", "medium", "high", "max"}, EffortBudget: map[string]int{"low": 2048, "medium": 8192, "high": 16384, "max": 32000}}
		body, err := New().Encode(req, "up-1", caps)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(body)
		var got, want any
		_ = json.Unmarshal(b, &got)
		wb, err := os.ReadFile(filepath.Join("testdata", "encode", "full.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(wb, &want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			g, _ := json.MarshalIndent(got, "", "  ")
			t.Errorf("编码结果不一致，实际：\n%s", g)
		}
		// 发出的 messages 里 user / assistant 交替
		var prev string
		for _, m := range body["messages"].([]any) {
			role := m.(map[string]any)["role"].(string)
			if role == prev {
				t.Errorf("相邻两条消息都是 %s", role)
			}
			prev = role
		}
		// tool_choice 的写法；不支持工具时不能做结构化输出；off 不写 thinking
		for in, want := range map[string]string{"auto": `{"type":"auto"}`, "required": `{"type":"any"}`, "none": `{"type":"none"}`, "f": `{"name":"f","type":"tool"}`} {
			body, _ := New().Encode(llm.Req{ToolChoice: in, Reasoning: "off"}, "m", caps)
			if g, _ := json.Marshal(body["tool_choice"]); string(g) != want || body["thinking"] != nil || body["max_tokens"] != 8192 {
				t.Errorf("tool_choice %s：%s；thinking %v；max_tokens %v", in, g, body["thinking"], body["max_tokens"])
			}
		}
		if _, err := New().Encode(llm.Req{Output: req.Output}, "m", llm.Caps{NativeSchema: true}); err == nil {
			t.Error("不支持工具时应当报错（这个协议的结构化输出用强制工具）")
		}
	})
}
