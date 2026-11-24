package quirks

import (
	"encoding/json"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mio-cc/anywhere-door/engine/llm"
)

func load(t *testing.T, yamlText string) Table {
	t.Helper()
	tb, err := Load(fstest.MapFS{QuirksFile: {Data: []byte(yamlText)}})
	if err != nil {
		t.Fatal(err)
	}
	return tb
}

func builtin(t *testing.T) Table {
	t.Helper()
	tb, err := Load(Files)
	if err != nil {
		t.Fatalf("内置的 quirks.yaml：%v", err)
	}
	return tb
}

func TestTable(t *testing.T) {
	t.Run("E1005-T1 合并顺序", func(t *testing.T) {
		tb := load(t, `
models:
  - match: { protocol: "*", model: "*" }
    caps: { window: 1000, max_output: 100, tools: true, vision: false, tokenizer: approx }
  - match: { protocol: chat_completions }
    caps: { window: 2000 }
  - match: { model: "foo-*" }
    caps: { vision: true }
    reasoning: { values: [low, high] }
  - match: { protocol: responses, model: "foo-*" }
    caps: { max_output: 999 }
`)
		cases := []struct {
			protocol, model string
			want            llm.Caps
		}{
			{"chat_completions", "foo-1", llm.Caps{Window: 2000, MaxOutput: 100, Tools: true, Vision: true, Tokenizer: "approx", Efforts: []string{"low", "high"}, EffortMap: map[string]string{}, EffortBudget: map[string]int{}}},
			{"responses", "foo-1", llm.Caps{Window: 1000, MaxOutput: 999, Tools: true, Vision: true, Tokenizer: "approx", Efforts: []string{"low", "high"}, EffortMap: map[string]string{}, EffortBudget: map[string]int{}}},
			{"chat_completions", "bar", llm.Caps{Window: 2000, MaxOutput: 100, Tools: true, Tokenizer: "approx"}},
			{"anthropic_messages", "bar", llm.Caps{Window: 1000, MaxOutput: 100, Tools: true, Tokenizer: "approx"}},
		}
		for _, tc := range cases {
			if got := tb.Lookup(tc.protocol, tc.model); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Lookup(%s, %s)：想要 %+v，实际 %+v", tc.protocol, tc.model, tc.want, got)
			}
		}
		// 内置表：兜底 + 具体规则
		b := builtin(t)
		c := b.Lookup("anthropic_messages", "claude-x-1")
		if c.Window != 200000 || !c.PromptCache || !c.Tools || c.EffortBudget["high"] != 16384 || c.Tokenizer != "approx" {
			t.Errorf("内置表 claude-*：%+v", c)
		}
		if c := b.Lookup("chat_completions", "claude-x-1"); c.Window != 32000 || c.PromptCache {
			t.Errorf("claude-* 的规则只对 anthropic_messages 生效：%+v", c)
		}
		if c := b.Lookup("responses", "mistral-small-latest"); c.Window != 128000 || !c.NativeSchema || !c.ParallelTools {
			t.Errorf("内置表 mistral-*（不写协议算匹配）：%+v", c)
		}
	})

	t.Run("E1005-T2 思考强度映射", func(t *testing.T) {
		b := builtin(t)
		glm := b.Lookup("chat_completions", "zai-glm-5")
		if v, warn := llm.ResolveEffort("medium", glm); v != "high" || warn == "" {
			t.Errorf("medium 对 zai-glm 应当映射为 high 并记 warn：%q %q", v, warn)
		}
		if v, warn := llm.ResolveEffort("max", glm); v != "max" || warn != "" {
			t.Errorf("支持的级原样用：%q %q", v, warn)
		}
		if v, warn := llm.ResolveEffort("xhigh", llm.Caps{Efforts: []string{"low", "medium", "high"}}); v != "high" || warn == "" {
			t.Errorf("xhigh 对只支持 low/medium/high 的模型取 high 并记 warn：%q %q", v, warn)
		}
		claude := b.Lookup("anthropic_messages", "claude-x")
		if v, _ := llm.ResolveEffort("off", claude); v != "" {
			t.Errorf("off 返回 \"\"：%q", v)
		}
		mistral := b.Lookup("chat_completions", "mistral-small-latest")
		if v, warn := llm.ResolveEffort("high", mistral); v != "" || warn != "" {
			t.Errorf("没有 reasoning 的模型返回 \"\"：%q %q", v, warn)
		}
		if v, _ := llm.ResolveEffort("medium", claude); v != "medium" || claude.EffortBudget[v] != 8192 {
			t.Errorf("budget 按级取值：%q %d", v, claude.EffortBudget[v])
		}
	})

	t.Run("E1005-T3 三层覆盖", func(t *testing.T) {
		b := builtin(t)
		reg := llm.NewRegistry([]llm.Protocol{stubProto{}}, b, nil, llm.Options{})
		w, f := 50000, false
		if err := reg.SetProviders([]llm.ProviderConfig{{
			Name: "m", Protocol: "chat_completions", BaseURL: "http://127.0.0.1:1/v1", Auth: llm.Auth{Type: llm.AuthNone},
			Caps:   &llm.CapsOverride{Window: &w},
			Models: []llm.ModelEntry{{ID: "small", Upstream: "mistral-small-latest", Caps: &llm.CapsOverride{NativeSchema: &f}}, {ID: "large", Upstream: "mistral-large-latest"}},
		}}); err != nil {
			t.Fatal(err)
		}
		small, err := reg.Client("m", "small")
		if err != nil {
			t.Fatal(err)
		}
		if c := small.Caps(); c.Window != 50000 || c.NativeSchema || !c.ParallelTools || c.MaxOutput != 4096 {
			t.Errorf("本表 → 提供方 → 模型条目：%+v", c)
		}
		large, _ := reg.Client("m", "large")
		if c := large.Caps(); c.Window != 50000 || !c.NativeSchema {
			t.Errorf("没有模型条目 caps 时：%+v", c)
		}
	})

	t.Run("E1005-T5 常用厂商", func(t *testing.T) {
		vs, err := Vendors(Files)
		if err != nil {
			t.Fatal(err)
		}
		if len(vs) < 3 {
			t.Fatalf("常用厂商太少：%d", len(vs))
		}
		ids := map[string]bool{}
		for _, v := range vs {
			if ids[v.ID] {
				t.Errorf("id 重名：%s", v.ID)
			}
			ids[v.ID] = true
			if !slices.Contains(llm.KnownProtocols, v.Provider.Protocol) {
				t.Errorf("%s 的协议 %q 不是已知的三种", v.ID, v.Provider.Protocol)
			}
			p := v.Provider
			p.Name = v.ID
			if p.Auth.Type != llm.AuthNone {
				p.Auth.Secret = v.ID
			}
			raw, _ := json.Marshal([]llm.ProviderConfig{p})
			if _, _, err := llm.ParseProviders(raw); err != nil {
				t.Errorf("%s 填上名字和密钥名后应当通过配置校验：%v", v.ID, err)
			}
			if p.Vendor != v.ID {
				t.Errorf("%s 的 Vendor 字段应当是厂商 id：%q", v.ID, p.Vendor)
			}
		}
		if _, err := Vendors(fstest.MapFS{VendorsFile: {Data: []byte("- id: a\n  name: A\n  provider: { protocol: chat_completions, base_url: \"https://x\", auth: { type: bearer }, name: bad }\n")}}); err == nil {
			t.Error("provider 里写了 name 应当报错")
		}
	})

	t.Run("E1005-T6 错误规则", func(t *testing.T) {
		b := builtin(t)
		hit := []struct {
			protocol      string
			status        int
			code, message string
			want          string
		}{
			{"chat_completions", 400, "context_length_exceeded", "too long", "user_input"},         // 状态 + 错误码
			{"anthropic_messages", 529, "", "", "transient"},                                       // 状态
			{"responses", 400, "", "Your request was blocked by our Content Policy", "structural"}, // 说明正则
			{"anthropic_messages", 200, "overloaded_error", "Overloaded", "transient"},             // 流里的错误事件
		}
		for _, h := range hit {
			if c, ok := b.ErrorClass(h.protocol, h.status, h.code, h.message); !ok || c != h.want {
				t.Errorf("ErrorClass(%s, %d, %q, %q) = %q, %v；想要 %q", h.protocol, h.status, h.code, h.message, c, ok, h.want)
			}
		}
		if c, ok := b.ErrorClass("chat_completions", 400, "context_length_exceeded", ""); !ok || c != "user_input" {
			t.Errorf("错误码命中：%q %v", c, ok)
		}
		if _, ok := b.ErrorClass("anthropic_messages", 400, "context_length_exceeded", ""); ok {
			t.Error("协议不同时不应当命中")
		}
		if _, ok := b.ErrorClass("chat_completions", 418, "teapot", "short and stout"); ok {
			t.Error("都不命中时 ok 应当为 false")
		}

		// 格式错的 quirks.yaml 报出文件和行号
		for _, bad := range []string{
			"models:\n  - match: { protocol: chat\n    caps: [\n",
			"models:\n  - match: { protocol: x }\n    colour: red\n",
		} {
			_, err := Load(fstest.MapFS{QuirksFile: {Data: []byte(bad)}})
			if err == nil || !strings.Contains(err.Error(), QuirksFile) || !strings.Contains(err.Error(), "第 ") || !strings.Contains(err.Error(), " 行") {
				t.Errorf("格式错应当报出文件和行号：%v", err)
			}
		}
		// 内容错写明是第几条
		_, err := Load(fstest.MapFS{QuirksFile: {Data: []byte("errors:\n  - match: { status: 400 }\n    class: oops\n")}})
		if err == nil || !strings.Contains(err.Error(), "errors[0]") || !strings.Contains(err.Error(), "oops") {
			t.Errorf("不认识的 class 应当写明位置：%v", err)
		}
		_, err = Load(fstest.MapFS{QuirksFile: {Data: []byte("models:\n  - match: { model: x }\n    reasoning: { values: [low, turbo] }\n")}})
		if err == nil || !strings.Contains(err.Error(), "models[0]") {
			t.Errorf("不认识的强度应当报错：%v", err)
		}
	})
}

// stubProto 只为了能注册提供方；E1005-T3 不发请求。
type stubProto struct{}

func (stubProto) Name() string { return "chat_completions" }
func (stubProto) DefaultPaths() map[string]string {
	return map[string]string{"complete": "/c", "models": "/m"}
}
func (stubProto) DefaultAuth() llm.Auth             { return llm.Auth{Type: llm.AuthBearer} }
func (stubProto) DefaultHeaders() map[string]string { return nil }
func (stubProto) Encode(llm.Req, string, llm.Caps) (map[string]any, error) {
	return map[string]any{}, nil
}
func (stubProto) Decode(io.Reader, llm.Caps) llm.Stream    { return nil }
func (stubProto) DecodeError(int, []byte) (string, string) { return "", "" }
func (stubProto) DecodeModels([]byte) ([]string, error)    { return nil, nil }
