package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/mio-cc/anywhere-door/gen"
)

// testProto 是测试用的最小协议：请求体带上模型名和消息文本；响应是一行一个 JSON 的 SSE：
// {"t":文本} {"k":思考} {"tc":{"i","id","n","a"}} {"done":{"finish","in","cached","out","model"}} {"err":{"code","message"}}。
type testProto struct{}

func (testProto) Name() string { return ProtoChat }
func (testProto) DefaultPaths() map[string]string {
	return map[string]string{"complete": "/complete", "models": "/models"}
}
func (testProto) DefaultAuth() Auth                 { return Auth{Type: AuthBearer} }
func (testProto) DefaultHeaders() map[string]string { return map[string]string{"X-Proto": "1"} }

func (testProto) Encode(r Req, upstream string, c Caps) (map[string]any, error) {
	var texts []string
	for _, m := range r.Messages {
		for _, p := range m.Parts {
			texts = append(texts, p.Text)
		}
	}
	body := map[string]any{"model": upstream, "stream": true, "messages": texts}
	if r.MaxTokens > 0 {
		body["max_tokens"] = r.MaxTokens
	}
	if r.Output != nil && !c.NativeSchema {
		body["tool"] = StructuredTool(r.Output).Name
	}
	if v, _ := ResolveEffort(r.Reasoning, c); v != "" {
		body["effort"] = v
	}
	return body, nil
}

func (testProto) Decode(body io.Reader, _ Caps) Stream { return &testDecoder{sse: NewSSEReader(body)} }

func (testProto) DecodeError(_ int, body []byte) (string, string) {
	var e struct {
		Error struct{ Code, Message string } `json:"error"`
	}
	_ = json.Unmarshal(body, &e)
	return e.Error.Code, e.Error.Message
}

func (testProto) DecodeModels(body []byte) ([]string, error) {
	var m struct {
		Data []struct{ ID string } `json:"data"`
	}
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	var out []string
	for _, d := range m.Data {
		out = append(out, d.ID)
	}
	return out, nil
}

type testDecoder struct{ sse *SSEReader }

func (d *testDecoder) Next() (Delta, error) {
	ev, err := d.sse.Next()
	if err != nil {
		return Delta{}, err
	}
	var m struct {
		T, K string
		TC   *struct {
			I        int
			ID, N, A string
		}
		Done *struct {
			Finish, Model   string
			In, Cached, Out int
		}
		Err *struct{ Code, Message string }
	}
	if err := json.Unmarshal([]byte(ev.Data), &m); err != nil {
		return Delta{}, Errorf(gen.FailureClassInternal, "bad_event", "事件不是合法的 JSON：%v", err)
	}
	switch {
	case m.Err != nil:
		return Delta{}, &Error{Status: 200, Code: m.Err.Code, Message: m.Err.Message}
	case m.Done != nil:
		return Delta{Done: true, Resp: &Resp{Finish: m.Done.Finish, Model: m.Done.Model, Usage: Usage{In: m.Done.In, Cached: m.Done.Cached, Out: m.Done.Out}}}, nil
	case m.TC != nil:
		return Delta{ToolCall: &ToolCallDelta{Index: m.TC.I, ID: m.TC.ID, Name: m.TC.N, Args: m.TC.A}}, nil
	}
	return Delta{Text: m.T, Thinking: m.K}, nil
}

func (d *testDecoder) Close() error { return nil }

// testCaps 是测试用的能力表。
type testCaps struct {
	caps  Caps
	rules map[string]string // 上游错误码 → 类别
}

func (t testCaps) Lookup(string, string) Caps { return t.caps }
func (t testCaps) ErrorClass(_ string, _ int, code, _ string) (string, bool) {
	c, ok := t.rules[code]
	return c, ok
}

// testSecrets 记下每次 Resolve。
type testSecrets struct {
	mu     sync.Mutex
	vals   map[string]string
	locked bool
	calls  int
}

func (s *testSecrets) Resolve(_ context.Context, name string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.locked {
		return "", fmt.Errorf("secret: 密钥库已锁定：%w", ErrSecretLocked)
	}
	v, ok := s.vals[name]
	if !ok {
		return "", errors.New("secret: 没有这个密钥")
	}
	return v, nil
}

func (s *testSecrets) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// sse 把若干 JSON 拼成 SSE 正文。
func sse(events ...string) string {
	var b strings.Builder
	for _, e := range events {
		b.WriteString("data: " + e + "\n\n")
	}
	return b.String()
}

// sseServer 对 /complete 返回固定的 SSE。
func sseServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTestRegistry(t *testing.T, caps CapsTable, ps ...ProviderConfig) *Registry {
	t.Helper()
	r := NewRegistry([]Protocol{testProto{}}, caps, &testSecrets{vals: map[string]string{"k": "key-123456"}}, Options{})
	if err := r.SetProviders(ps); err != nil {
		t.Fatal(err)
	}
	return r
}

func localProvider(url string) ProviderConfig {
	return ProviderConfig{Name: "p", Protocol: ProtoChat, BaseURL: url, Auth: Auth{Type: AuthNone}}
}

// sliceStream 按顺序吐出固定的 Delta，吐完返回 io.EOF。
type sliceStream struct {
	ds     []Delta
	closed bool
}

func (s *sliceStream) Next() (Delta, error) {
	if len(s.ds) == 0 {
		return Delta{}, io.EOF
	}
	d := s.ds[0]
	s.ds = s.ds[1:]
	return d, nil
}

func (s *sliceStream) Close() error { s.closed = true; return nil }

func TestCollect(t *testing.T) {
	t.Run("E1001-T1 Collect", func(t *testing.T) {
		// 直接拼：文本分片、两个工具调用按 Index 交错
		s := &sliceStream{ds: []Delta{
			{Text: "Hel"}, {Thinking: "想"},
			{ToolCall: &ToolCallDelta{Index: 1, ID: "c2", Name: "g", Args: `{"b"`}},
			{ToolCall: &ToolCallDelta{Index: 0, ID: "c1", Name: "f", Args: `{"a":`}},
			{Text: "lo"},
			{ToolCall: &ToolCallDelta{Index: 0, Args: `1}`}},
			{ToolCall: &ToolCallDelta{Index: 1, Args: `:2}`}},
			{Done: true},
		}}
		r, err := Collect(s)
		if err != nil {
			t.Fatal(err)
		}
		want := []ToolCall{{ID: "c1", Name: "f", Args: json.RawMessage(`{"a":1}`)}, {ID: "c2", Name: "g", Args: json.RawMessage(`{"b":2}`)}}
		if r.Text != "Hello" || r.Thinking != "想" || !reflect.DeepEqual(r.ToolCalls, want) || r.Finish != FinishToolCalls {
			t.Errorf("拼接结果不对：%+v", r)
		}
		if !s.closed {
			t.Error("Collect 之后流应当已关闭")
		}

		// 流在没有 Done 时提前结束 → 错误
		_, err = Collect(&sliceStream{ds: []Delta{{Text: "半"}}})
		var le *Error
		if !errors.As(err, &le) || le.Class != gen.FailureClassTransient || le.Code != CodeStreamTruncated {
			t.Errorf("没有 Done 时应当是 transient / stream_truncated：%v", err)
		}

		// 经过注册表：最后一个 Delta 带完整 Resp（拼好的文本、工具调用、用量、Raw）
		body := sse(`{"t":"Hel"}`, `{"t":"lo"}`,
			`{"tc":{"i":0,"id":"c1","n":"f","a":"{\"a\":"}}`,
			`{"tc":{"i":1,"id":"c2","n":"g","a":"{\"b\""}}`,
			`{"tc":{"i":0,"a":"1}"}}`,
			`{"tc":{"i":1,"a":":2}"}}`,
			`{"done":{"finish":"tool_calls","in":3,"out":5}}`) + "data: [DONE]\n\n"
		srv := sseServer(t, body)
		reg := newTestRegistry(t, nil, localProvider(srv.URL))
		c, err := reg.Client("p", "m1")
		if err != nil {
			t.Fatal(err)
		}
		st, err := c.Complete(context.Background(), Req{Messages: []Message{{Role: RoleUser, Parts: []Part{{Type: PartText, Text: "hi"}}}}})
		if err != nil {
			t.Fatal(err)
		}
		var last Delta
		n := 0
		for {
			d, err := st.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			last, n = d, n+1
		}
		if n != 7 || !last.Done || last.Resp == nil {
			t.Fatalf("应当是 6 个片段加 1 个 Done，实际 %d 个，最后一个 %+v", n, last)
		}
		r = last.Resp
		if r.Text != "Hello" || !reflect.DeepEqual(r.ToolCalls, want) || r.Usage != (Usage{In: 3, Out: 5}) || r.Finish != FinishToolCalls || r.Model != "m1" {
			t.Errorf("完整 Resp 不对：%+v", r)
		}
		var raw []json.RawMessage
		if err := json.Unmarshal(r.Raw, &raw); err != nil || len(raw) < 7 || string(raw[0]) != `{"t":"Hel"}` {
			t.Errorf("Raw 应当是 SSE 的 data 逐条组成的数组：%s", r.Raw)
		}

		// 经过注册表：流断开 → transient
		srv2 := sseServer(t, sse(`{"t":"半"}`))
		reg2 := newTestRegistry(t, nil, localProvider(srv2.URL))
		c2, _ := reg2.Client("p", "m1")
		st2, err := c2.Complete(context.Background(), Req{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Collect(st2); !errors.As(err, &le) || le.Class != gen.FailureClassTransient || le.Provider != "p" {
			t.Errorf("流断开应当是带提供方名的 transient：%v", err)
		}
	})

	t.Run("E1001-T1 结构化输出用强制工具", func(t *testing.T) {
		srv := sseServer(t, sse(`{"tc":{"i":0,"id":"c1","n":"emit_person","a":"{\"name\":\"A\"}"}}`, `{"done":{"finish":"tool_calls","in":1,"out":1}}`))
		reg := newTestRegistry(t, testCaps{caps: Caps{Tools: true}}, localProvider(srv.URL))
		c, _ := reg.Client("p", "m1")
		st, err := c.Complete(context.Background(), Req{Output: &OutputSpec{Name: "person", Schema: json.RawMessage(`{"type":"object"}`)}})
		if err != nil {
			t.Fatal(err)
		}
		r, err := Collect(st)
		if err != nil {
			t.Fatal(err)
		}
		if string(r.Structured) != `{"name":"A"}` || len(r.ToolCalls) != 0 || r.Finish != FinishStop {
			t.Errorf("强制工具的参数应当进 Structured、不出现在 ToolCalls 里：%+v", r)
		}
		// 原生写法：文本就是结果
		srv2 := sseServer(t, sse(`{"t":"{\"name\": \"B\"}"}`, `{"done":{"finish":"stop","in":1,"out":1}}`))
		reg2 := newTestRegistry(t, testCaps{caps: Caps{NativeSchema: true}}, localProvider(srv2.URL))
		c2, _ := reg2.Client("p", "m1")
		st2, _ := c2.Complete(context.Background(), Req{Output: &OutputSpec{Name: "person"}})
		if r2, err := Collect(st2); err != nil || string(r2.Structured) != `{"name":"B"}` {
			t.Errorf("原生写法的文本应当进 Structured：%+v %v", r2, err)
		}
		// 都不支持 → user_input
		reg3 := newTestRegistry(t, testCaps{}, localProvider(srv.URL))
		c3, _ := reg3.Client("p", "m1")
		var le *Error
		if _, err := c3.Complete(context.Background(), Req{Output: &OutputSpec{Name: "x"}}); !errors.As(err, &le) || le.Class != gen.FailureClassUserInput {
			t.Errorf("不支持结构化输出时应当是 user_input：%v", err)
		}
	})
}

func TestCapsAndEffort(t *testing.T) {
	t.Run("E1001-T7 能力合并与强度", func(t *testing.T) {
		w2000, f, tok := 2000, false, "cl100k"
		table := testCaps{caps: Caps{Window: 1000, MaxOutput: 100, Tools: true, Vision: true, Efforts: []string{"low", "high"}, Tokenizer: "approx"}}
		reg := newTestRegistry(t, table, ProviderConfig{
			Name: "p", Protocol: ProtoChat, BaseURL: "http://127.0.0.1:1", Auth: Auth{Type: AuthNone},
			Caps:   &CapsOverride{Window: &w2000, Tokenizer: &tok},
			Models: []ModelEntry{{ID: "a", Upstream: "up-a", Caps: &CapsOverride{Tools: &f, Efforts: []string{"medium"}}}, {ID: "b"}},
		})
		c, err := reg.Client("p", "a")
		if err != nil {
			t.Fatal(err)
		}
		got := c.Caps()
		want := Caps{Window: 2000, MaxOutput: 100, Tools: false, Vision: true, Efforts: []string{"medium"}, Tokenizer: "cl100k"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("三层覆盖：想要 %+v，实际 %+v", want, got)
		}
		cb, _ := reg.Client("p", "b")
		if cb.Caps().Window != 2000 || !cb.Caps().Tools || !reflect.DeepEqual(cb.Caps().Efforts, []string{"low", "high"}) {
			t.Errorf("没有模型条目 caps 时只套提供方这一层：%+v", cb.Caps())
		}

		cases := []struct {
			req      string
			caps     Caps
			want     string
			wantWarn bool
		}{
			{"", Caps{Efforts: []string{"low"}}, "", false},
			{"off", Caps{Efforts: []string{"off", "low"}}, "", false},
			{"high", Caps{}, "", false},
			{"high", Caps{Efforts: []string{"low", "high"}}, "high", false},
			{"medium", Caps{Efforts: []string{"low", "high", "max"}, EffortMap: map[string]string{"medium": "high"}}, "high", true},
			{"xhigh", Caps{Efforts: []string{"low", "medium", "high"}}, "high", true},
			{"medium", Caps{Efforts: []string{"low", "high"}}, "high", true}, // 一样近取较高
			{"minimal", Caps{Efforts: []string{"off", "low", "high"}}, "low", true},
			{"max", Caps{Efforts: []string{"low"}}, "low", true},
			{"turbo", Caps{Efforts: []string{"low"}}, "", true},
		}
		for _, tc := range cases {
			v, warn := ResolveEffort(tc.req, tc.caps)
			if v != tc.want || (warn != "") != tc.wantWarn {
				t.Errorf("ResolveEffort(%q, %v) = %q, %q；想要 %q，warn=%v", tc.req, tc.caps.Efforts, v, warn, tc.want, tc.wantWarn)
			}
		}
	})
}

func TestSSE(t *testing.T) {
	t.Run("E1001-T1 SSE 读取", func(t *testing.T) {
		in := ": 注释\r\nevent: a\r\ndata: 1\r\ndata: 2\r\nid: x\r\n\r\n\n\ndata:3\n\ndata: tail"
		r := NewSSEReader(strings.NewReader(in))
		var got []Event
		for {
			ev, err := r.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, ev)
		}
		want := []Event{{Event: "a", Data: "1\n2", ID: "x"}, {Data: "3"}, {Data: "tail"}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("想要 %+v，实际 %+v", want, got)
		}
	})
}
