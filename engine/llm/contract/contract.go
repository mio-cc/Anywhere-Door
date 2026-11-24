// Package contract 是三种协议共用的契约测试框架（设计文档 E.10.01 §4.6、§5.6，E.10-R3、Q.06-R4）。
// 每种协议在自己的测试里调用 Run，用录放数据（E.10.08）跑同一组用例；测试不调真模型（E.10-R5）。
package contract

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/mio-cc/anywhere-door/engine/llm"
	"github.com/mio-cc/anywhere-door/gen"
)

// Suite 是一种协议的契约测试配置。
type Suite struct {
	Protocol  llm.Protocol
	Provider  llm.ProviderConfig // 协议的测试用提供方配置（录制时 BaseURL 是真实地址；回放不联网）
	Model     string             // Provider 里的模型 id
	Cassettes string             // 录放目录；每个用例的录放文件是 <Cassettes>/<用例名>.json

	// 以下两项是 E.10.01 §4.6 之外补的（RELAY.md 写明）。
	Caps      llm.CapsTable                                                             // 能力表（协议测试传 quirks 表）；nil 时用 DefaultCaps
	Transport func(t testing.TB, dir, name string, secrets ...string) http.RoundTripper // 录放传输层（传 cassette.Transport）；nil 时用缺省传输，只用于本包自测
}

// Cases 是契约测试的用例名，也是录放文件名（E.10.01 §5.6）。
var Cases = []string{"text", "tool_call", "tool_result", "structured", "usage", "error_auth", "error_rate", "models"}

// 用例里固定的输入：录制和回放必须发出完全一样的请求（E.10.08 §5 按请求体匹配）。
const (
	System      = "You are a terse assistant. Follow the instructions exactly."
	TextPrompt  = "Reply with the single word: hello"
	ToolPrompt  = "What is the weather in Paris right now? Use the get_weather tool."
	ToolResult  = `{"temp_c":18,"sky":"sunny"}`
	ExtractText = "Extract the person from this sentence: Alice is 30 years old."
	CallID      = "call12345" // 9 位字母数字：有的服务只认这种工具调用 id
	CaseKey     = "contract_case"
)

// WeatherTool 是工具用例里的工具。
var WeatherTool = llm.ToolSpec{
	Name:        "get_weather",
	Description: "Get the current weather for a city.",
	Schema:      json.RawMessage(`{"type":"object","properties":{"city":{"type":"string","description":"City name"}},"required":["city"]}`),
}

// PersonSchema 是结构化输出用例的 schema。
var PersonSchema = json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer"}},"required":["name","age"],"additionalProperties":false}`)

// DefaultCaps 是 Suite.Caps 为 nil 时用的能力表：工具和原生结构化输出都支持。
var DefaultCaps llm.CapsTable = staticCaps{llm.Caps{Window: 32000, MaxOutput: 1024, Tools: true, NativeSchema: true}}

type staticCaps struct{ c llm.Caps }

func (s staticCaps) Lookup(string, string) llm.Caps                      { return s.c }
func (staticCaps) ErrorClass(string, int, string, string) (string, bool) { return "", false }

// EnvSecret 是开发机上提供密钥的环境变量名：ANYDOOR_DEV_SECRET_<密钥名大写>（G.11-R3、Q.06-R7）。
func EnvSecret(name string) string {
	return "ANYDOOR_DEV_SECRET_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

// envSecrets 从环境变量取密钥；没有设时给一个假的（回放不需要真密钥）。
type envSecrets struct{}

func (envSecrets) Resolve(_ context.Context, name string) (string, error) {
	if v := os.Getenv(EnvSecret(name)); v != "" {
		return v, nil
	}
	return "contract-test-key", nil
}

// Recording 是否在 -record 录制（cassette 包注册的 -record 标志）。
func Recording() bool {
	f := flag.Lookup("record")
	return f != nil && f.Value.String() == "true"
}

// Request 是用例 name 发出的请求。
func Request(name, model string) llm.Req {
	t0 := float32(0)
	sys := llm.Message{Role: llm.RoleSystem, Parts: []llm.Part{{Type: llm.PartText, Text: System}}}
	user := func(s string) llm.Message {
		return llm.Message{Role: llm.RoleUser, Parts: []llm.Part{{Type: llm.PartText, Text: s}}}
	}
	r := llm.Req{Model: model, Temperature: &t0, Metadata: map[string]string{CaseKey: name}}
	switch name {
	case "tool_call":
		r.Messages = []llm.Message{sys, user(ToolPrompt)}
		r.Tools, r.ToolChoice, r.MaxTokens = []llm.ToolSpec{WeatherTool}, "auto", 256
	case "tool_result":
		r.Messages = []llm.Message{sys, user(ToolPrompt),
			{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: CallID, Name: WeatherTool.Name, Args: json.RawMessage(`{"city":"Paris"}`)}}},
			{Role: llm.RoleTool, ToolCallID: CallID, Parts: []llm.Part{{Type: llm.PartText, Text: ToolResult}}},
		}
		r.Tools, r.MaxTokens = []llm.ToolSpec{WeatherTool}, 256
	case "structured":
		r.Messages = []llm.Message{sys, user(ExtractText)}
		r.Output, r.MaxTokens = &llm.OutputSpec{Name: "person", Schema: PersonSchema, Strict: true}, 256
	default: // text、usage、error_auth、error_rate
		r.Messages = []llm.Message{sys, user(TextPrompt)}
		r.MaxTokens = 64
	}
	return r
}

// Run 跑 §5.6 的全部用例，最后验证一次测试守卫（E.10-R5）。每个用例的录放文件名就是用例名。
func Run(t *testing.T, s Suite) {
	t.Helper()
	for _, name := range Cases {
		t.Run(name, func(t *testing.T) {
			if err := checkCassette(s.Cassettes, name); err != nil {
				t.Fatal(err)
			}
			reg, err := s.registry(t, name)
			if err != nil {
				t.Fatal(err)
			}
			runCase(t, reg, s, name)
		})
	}
	t.Run("guard", func(t *testing.T) { checkGuard(t, s) })
}

// checkCassette：回放时录放文件必须存在（录制时由 cassette 写出）。
func checkCassette(dir, name string) error {
	if Recording() {
		return nil
	}
	p := filepath.Join(dir, name+".json")
	if _, err := os.Stat(p); err != nil {
		return fmt.Errorf("缺少录放文件 %s（契约用例 %s）：在开发机上用 -record 录制，或按官方文档手写（E.10.08 §5）", p, name)
	}
	return nil
}

func (s Suite) registry(t *testing.T, name string) (*llm.Registry, error) {
	var rt http.RoundTripper
	if s.Transport != nil {
		var keys []string
		if s.Provider.Auth.Secret != "" {
			if v := os.Getenv(EnvSecret(s.Provider.Auth.Secret)); v != "" {
				keys = append(keys, v)
			}
		}
		rt = s.Transport(t, s.Cassettes, name, keys...)
	}
	caps := s.Caps
	if caps == nil {
		caps = DefaultCaps
	}
	reg := llm.NewRegistry([]llm.Protocol{s.Protocol}, caps, envSecrets{}, llm.Options{Transport: rt, Logger: slog.New(slog.NewTextHandler(logWriter{t}, nil))})
	return reg, reg.SetProviders([]llm.ProviderConfig{s.Provider})
}

type logWriter struct{ t *testing.T }

func (w logWriter) Write(p []byte) (int, error) {
	w.t.Log(strings.TrimRight(string(p), "\n"))
	return len(p), nil
}

func runCase(t *testing.T, reg *llm.Registry, s Suite, name string) {
	ctx := context.Background()
	if name == "models" {
		ms, err := reg.ListModels(ctx, s.Provider.Name)
		if err != nil {
			t.Fatal(err)
		}
		if len(ms) == 0 {
			t.Error("模型列表为空")
		}
		for _, m := range ms {
			if m == "" {
				t.Errorf("模型列表里有空的名字：%v", ms)
			}
		}
		return
	}
	c, err := reg.Client(s.Provider.Name, s.Model)
	if err != nil {
		t.Fatal(err)
	}
	st, err := c.Complete(ctx, Request(name, s.Model))
	switch name {
	case "error_auth":
		_ = expectError(t, st, err, gen.FailureClassAuthDenied, http.StatusUnauthorized)
		return
	case "error_rate":
		if e := expectError(t, st, err, gen.FailureClassRateLimited, http.StatusTooManyRequests); e != nil && e.RetryAfter <= 0 {
			t.Errorf("429 应当带 Retry-After：%+v", e)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	r, streamed, toolDeltas := drain(t, st)
	if r.Model == "" {
		t.Error("Resp.Model 为空")
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(r.Raw, &raw); err != nil || len(raw) == 0 {
		t.Errorf("Resp.Raw 应当是 SSE 的 data 组成的数组：%s", r.Raw)
	}
	switch name {
	case "text":
		if strings.TrimSpace(r.Text) == "" || r.Text != streamed || r.Finish != llm.FinishStop || len(r.ToolCalls) != 0 {
			t.Errorf("流式文本：拼好的 %q、流里的 %q、Finish %q、工具调用 %v", r.Text, streamed, r.Finish, r.ToolCalls)
		}
	case "tool_call":
		if len(r.ToolCalls) == 0 || toolDeltas == 0 || r.Finish != llm.FinishToolCalls {
			t.Fatalf("应当有工具调用：%+v", r)
		}
		tc := r.ToolCalls[0]
		var args struct {
			City string `json:"city"`
		}
		if tc.ID == "" || tc.Name != WeatherTool.Name || json.Unmarshal(tc.Args, &args) != nil || args.City == "" {
			t.Errorf("工具调用的 ID / Name / Args 不对：%+v（%s）", tc, tc.Args)
		}
	case "tool_result":
		if strings.TrimSpace(r.Text) == "" || r.Finish != llm.FinishStop || len(r.ToolCalls) != 0 {
			t.Errorf("带工具结果的第二轮应当返回文本：%+v", r)
		}
	case "structured":
		if err := validate(PersonSchema, r.Structured); err != nil || len(r.ToolCalls) != 0 || r.Finish != llm.FinishStop {
			t.Errorf("结构化输出 %s 不符合 schema 或多了工具调用：%v；%+v", r.Structured, err, r)
		}
	case "usage":
		u := r.Usage
		if u.In <= 0 || u.Out <= 0 || u.Cached < 0 || u.Cached > u.In || u.Reasoning < 0 {
			t.Errorf("用量不对：%+v", u)
		}
	}
}

// drain 读完流：返回最后的 Resp、流里拼起来的文本、工具调用片段数。最后一个 Delta 必须是 Done 并带 Resp，之后是 io.EOF。
func drain(t *testing.T, st llm.Stream) (*llm.Resp, string, int) {
	t.Helper()
	defer func() { _ = st.Close() }()
	var text strings.Builder
	tools := 0
	for {
		d, err := st.Next()
		if err != nil {
			t.Fatalf("流在 Done 之前出错：%v", err)
		}
		text.WriteString(d.Text)
		if d.ToolCall != nil {
			tools++
		}
		if d.Done {
			if d.Resp == nil {
				t.Fatal("最后一个 Delta 没有带 Resp")
			}
			if _, err := st.Next(); !errors.Is(err, io.EOF) {
				t.Errorf("Done 之后应当是 io.EOF：%v", err)
			}
			return d.Resp, text.String(), tools
		}
	}
}

func expectError(t *testing.T, st llm.Stream, err error, class gen.FailureClass, status int) *llm.Error {
	t.Helper()
	if err == nil {
		_ = st.Close()
		t.Errorf("应当返回 %s 错误", class)
		return nil
	}
	var e *llm.Error
	if !errors.As(err, &e) || e.Class != class || e.Status != status {
		t.Errorf("应当是 HTTP %d / %s：%v", status, class, err)
		return nil
	}
	return e
}

// checkGuard：缺省传输层在测试里拒绝真实地址（E.10-R5、Q.06-R2）。
func checkGuard(t *testing.T, s Suite) {
	t.Setenv(llm.EnvRecord, "")
	p := s.Provider
	p.BaseURL, p.Proxy = "https://example.invalid/v1", ""
	reg := llm.NewRegistry([]llm.Protocol{s.Protocol}, DefaultCaps, envSecrets{}, llm.Options{})
	if err := reg.SetProviders([]llm.ProviderConfig{p}); err != nil {
		t.Fatal(err)
	}
	c, err := reg.Client(p.Name, s.Model)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Complete(context.Background(), Request("text", s.Model))
	var e *llm.Error
	if !errors.As(err, &e) || e.Code != llm.CodeTestGuard {
		t.Errorf("测试里请求真实地址应当被守卫拦下：%v", err)
	}
}

const schemaURL = "https://anydoor.invalid/contract/output.schema.json"

func validate(schema, inst json.RawMessage) error {
	if len(inst) == 0 {
		return errors.New("没有结构化输出")
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema))
	if err != nil {
		return err
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(schemaURL, doc); err != nil {
		return err
	}
	sch, err := c.Compile(schemaURL)
	if err != nil {
		return err
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(inst))
	if err != nil {
		return err
	}
	return sch.Validate(v)
}
