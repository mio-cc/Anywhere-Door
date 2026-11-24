package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mio-cc/anywhere-door/gen"
)

// recorded 是 httptest 收到的一次请求。
type recorded struct {
	method, path, query string
	header              http.Header
	body                map[string]any
}

type recorder struct {
	mu   sync.Mutex
	reqs []recorded
}

func (r *recorder) last(t *testing.T) recorded {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.reqs) == 0 {
		t.Fatal("服务端没有收到请求")
	}
	return r.reqs[len(r.reqs)-1]
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.reqs)
}

// recordingServer 记下收到的请求，返回一个最短的成功流。
func recordingServer(t *testing.T) (*httptest.Server, *recorder) {
	t.Helper()
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		b, _ := io.ReadAll(req.Body)
		var body map[string]any
		_ = json.Unmarshal(b, &body)
		rec.mu.Lock()
		rec.reqs = append(rec.reqs, recorded{method: req.Method, path: req.URL.Path, query: req.URL.RawQuery, header: req.Header.Clone(), body: body})
		rec.mu.Unlock()
		if strings.HasSuffix(req.URL.Path, "/models") {
			_, _ = io.WriteString(w, `{"data":[{"id":"m1"},{"id":"m2"}]}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, sse(`{"t":"ok"}`, `{"done":{"finish":"stop","in":1,"out":1}}`))
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

func call(t *testing.T, reg *Registry, provider, model string) error {
	t.Helper()
	c, err := reg.Client(provider, model)
	if err != nil {
		return err
	}
	st, err := c.Complete(context.Background(), Req{Model: model, Messages: []Message{{Role: RoleUser, Parts: []Part{{Type: PartText, Text: "hi"}}}}})
	if err != nil {
		return err
	}
	_, err = Collect(st)
	return err
}

func secretRef(name string) string { return "{{" + "secret" + ":" + name + "}}" }

func TestRequest(t *testing.T) {
	t.Run("E1001-T4 请求组装", func(t *testing.T) {
		srv, rec := recordingServer(t)
		sec := &testSecrets{vals: map[string]string{"k": "key-AAAA1111", "h": "hdr-BBBB2222"}}
		reg := NewRegistry([]Protocol{testProto{}}, nil, sec, Options{})
		ps := []ProviderConfig{
			{Name: "bearer", Protocol: ProtoChat, BaseURL: srv.URL + "/v1/", Auth: Auth{Type: AuthBearer, Secret: "k"},
				Headers: map[string]string{"X-Extra": "a-" + secretRef("h") + "-b", "X-Plain": "p"}},
			{Name: "default", Protocol: ProtoChat, BaseURL: srv.URL + "/v1", Auth: Auth{Secret: "k"}, Paths: map[string]string{"complete": "/chat/go"}},
			{Name: "header", Protocol: ProtoChat, BaseURL: srv.URL, Auth: Auth{Type: AuthHeader, Name: "x-api-key", Secret: "k"}},
			{Name: "query", Protocol: ProtoChat, BaseURL: srv.URL, Auth: Auth{Type: AuthQuery, Name: "key", Secret: "k"}},
			{Name: "none", Protocol: ProtoChat, BaseURL: srv.URL, Auth: Auth{Type: AuthNone},
				Models: []ModelEntry{{ID: "small", Upstream: "up-small-1"}, {ID: "big"}},
				Override: []BodyOp{
					{Op: OpSet, Path: "opts.deep.flag", Value: json.RawMessage(`true`)},
					{Op: OpSet, Path: "messages.0", Value: json.RawMessage(`"changed"`)},
					{Op: OpDelete, Path: "stream"},
					{Op: OpDelete, Path: "no.such.path"},
					{Op: OpRename, Path: "max_tokens", To: "max_completion_tokens"},
					{Op: OpRename, Path: "absent", To: "x"},
					{Op: OpSet, Path: "only_small", Value: json.RawMessage(`1`), Models: []string{"up-small-*"}},
				}},
		}
		if err := reg.SetProviders(ps); err != nil {
			t.Fatal(err)
		}
		if sec.count() != 0 {
			t.Errorf("SetProviders 时不应当调用 SecretResolver，调用了 %d 次", sec.count())
		}

		if err := call(t, reg, "bearer", "m"); err != nil {
			t.Fatal(err)
		}
		r := rec.last(t)
		if r.method != http.MethodPost || r.path != "/v1/complete" {
			t.Errorf("地址应当是 BaseURL（去掉末尾 /）+ 默认路径：%s %s", r.method, r.path)
		}
		if r.header.Get("Authorization") != "Bearer key-AAAA1111" || r.header.Get("X-Extra") != "a-hdr-BBBB2222-b" || r.header.Get("X-Plain") != "p" ||
			r.header.Get("X-Proto") != "1" || r.header.Get("Content-Type") != "application/json" || r.header.Get("Accept") != "text/event-stream" {
			t.Errorf("请求头不对：%v", r.header)
		}
		if sec.count() != 2 {
			t.Errorf("一次请求应当为鉴权和请求头里的引用各取一次密钥，实际 %d 次", sec.count())
		}

		if err := call(t, reg, "default", "m"); err != nil {
			t.Fatal(err)
		}
		r = rec.last(t)
		if r.path != "/v1/chat/go" || r.header.Get("Authorization") != "Bearer key-AAAA1111" {
			t.Errorf("Paths 覆盖、缺省鉴权（协议的 bearer）：%s %v", r.path, r.header)
		}
		if sec.count() != 3 {
			t.Errorf("每次请求取一次密钥，实际累计 %d 次", sec.count())
		}

		if err := call(t, reg, "header", "m"); err != nil {
			t.Fatal(err)
		}
		r = rec.last(t)
		if r.header.Get("X-Api-Key") != "key-AAAA1111" || r.header.Get("Authorization") != "" {
			t.Errorf("header 鉴权：%v", r.header)
		}

		if err := call(t, reg, "query", "m"); err != nil {
			t.Fatal(err)
		}
		r = rec.last(t)
		if q, _ := url.ParseQuery(r.query); q.Get("key") != "key-AAAA1111" || r.header.Get("Authorization") != "" {
			t.Errorf("query 鉴权：%q %v", r.query, r.header)
		}

		before := sec.count()
		c, err := reg.Client("none", "small")
		if err != nil {
			t.Fatal(err)
		}
		st, err := c.Complete(context.Background(), Req{MaxTokens: 9, Messages: []Message{{Role: RoleUser, Parts: []Part{{Type: PartText, Text: "a"}, {Type: PartText, Text: "b"}}}}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Collect(st); err != nil {
			t.Fatal(err)
		}
		r = rec.last(t)
		if sec.count() != before || r.header.Get("Authorization") != "" {
			t.Errorf("none 不取密钥、不带鉴权头：%d %v", sec.count()-before, r.header)
		}
		b := r.body
		opts, _ := b["opts"].(map[string]any)
		deep, _ := opts["deep"].(map[string]any)
		msgs, _ := b["messages"].([]any)
		if b["model"] != "up-small-1" || deep["flag"] != true || len(msgs) != 2 || msgs[0] != "changed" || msgs[1] != "b" {
			t.Errorf("upstream 映射、set（建中间对象、数组下标）不对：%v", b)
		}
		if _, ok := b["stream"]; ok {
			t.Errorf("delete 应当删掉 stream：%v", b)
		}
		if _, ok := b["max_tokens"]; ok || b["max_completion_tokens"] != float64(9) || b["only_small"] != float64(1) {
			t.Errorf("rename、按模型过滤的 set 不对：%v", b)
		}
		if _, ok := b["x"]; ok {
			t.Errorf("rename 的原路径不存在时什么也不做：%v", b)
		}
		if err := call(t, reg, "none", "big"); err != nil {
			t.Fatal(err)
		}
		if r = rec.last(t); r.body["model"] != "big" || r.body["only_small"] != nil {
			t.Errorf("upstream 为空时用 id；Models 过滤：%v", r.body)
		}

		// Models 不为空时列表外的名字报错
		var le *Error
		if _, err := reg.Client("none", "other"); !errors.As(err, &le) || le.Code != "model_not_found" {
			t.Errorf("列表外的模型名应当报错：%v", err)
		}
		if _, err := reg.Client("nobody", "m"); !errors.As(err, &le) || le.Code != "provider_not_found" {
			t.Errorf("没有的提供方应当报错：%v", err)
		}

		// 密钥库锁定 → secret_locked；密钥不存在 → secret_missing；都不发出请求
		n := rec.count()
		sec.locked = true
		if err := call(t, reg, "bearer", "m"); !errors.As(err, &le) || le.Code != CodeSecretLocked || le.Class != gen.FailureClassAuthDenied || !strings.Contains(le.Message, "请先解锁") {
			t.Errorf("密钥库锁定：%v", err)
		}
		sec.locked = false
		delete(sec.vals, "k")
		if err := call(t, reg, "default", "m"); !errors.As(err, &le) || le.Code != CodeSecretMissing || le.Class != gen.FailureClassAuthDenied || !strings.Contains(le.Message, "密钥 k 不存在") {
			t.Errorf("密钥不存在：%v", err)
		}
		if rec.count() != n {
			t.Errorf("取不到密钥时不应当发出请求")
		}

		// 路径穿过非对象 → user_input，写明第几条
		bad := []ProviderConfig{{Name: "bad", Protocol: ProtoChat, BaseURL: srv.URL, Auth: Auth{Type: AuthNone},
			Override: []BodyOp{{Op: OpSet, Path: "model.x", Value: json.RawMessage(`1`)}}}}
		if err := reg.SetProviders(bad); err != nil {
			t.Fatal(err)
		}
		if err := call(t, reg, "bad", "m"); !errors.As(err, &le) || le.Class != gen.FailureClassUserInput || !strings.Contains(le.Message, "第 1 条") {
			t.Errorf("覆盖路径穿过字符串应当报配置错：%v", err)
		}

		// 模型列表
		ms, err := reg.ListModels(context.Background(), "bad")
		if err != nil || len(ms) != 2 || ms[0] != "m1" {
			t.Errorf("模型列表：%v %v", ms, err)
		}
		if r = rec.last(t); r.method != http.MethodGet || r.path != "/models" || r.header.Get("Accept") != "application/json" {
			t.Errorf("模型列表的请求：%s %s %v", r.method, r.path, r.header)
		}
	})
}

func TestRawRedact(t *testing.T) {
	t.Run("E1001-T4 Raw 里没有密钥", func(t *testing.T) {
		// 上游把收到的密钥回显在响应里：Raw 里要脱敏，结构不变
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, sse(`{"t":"your key is `+key+`"}`, `{"t":" token sk-abcdefghijklmnop"}`, `{"done":{"finish":"stop","in":1,"out":1}}`))
		}))
		t.Cleanup(srv.Close)
		reg := NewRegistry([]Protocol{testProto{}}, nil, &testSecrets{vals: map[string]string{"k": "key-RAW-998877"}}, Options{})
		if err := reg.SetProviders([]ProviderConfig{{Name: "p", Protocol: ProtoChat, BaseURL: srv.URL, Auth: Auth{Type: AuthBearer, Secret: "k"}}}); err != nil {
			t.Fatal(err)
		}
		c, _ := reg.Client("p", "m")
		st, err := c.Complete(context.Background(), Req{})
		if err != nil {
			t.Fatal(err)
		}
		r, err := Collect(st)
		if err != nil {
			t.Fatal(err)
		}
		raw := string(r.Raw)
		if strings.Contains(raw, "key-RAW-998877") || strings.Contains(raw, "sk-abcdefghijklmnop") || !strings.Contains(raw, "[REDACTED]") || !json.Valid(r.Raw) {
			t.Errorf("Raw 里的密钥原文和像密钥的串应当被替换，且仍是合法 JSON：%s", raw)
		}
		if got := redactRaw(json.RawMessage(`["abcd","z"]`), []string{`abcd"`}); got != nil {
			t.Errorf("替换后不是合法 JSON 时不保留 Raw：%s", got)
		}
	})
}

// errorServer 对每个请求返回固定的状态、头和正文。
func errorServer(t *testing.T, status int, header map[string]string, body string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		for k, v := range header {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestErrors(t *testing.T) {
	t.Run("E1001-T5 错误分类", func(t *testing.T) {
		caps := testCaps{rules: map[string]string{"special": string(gen.FailureClassStructural)}}
		cases := []struct {
			status int
			code   string
			want   gen.FailureClass
		}{
			{408, "", gen.FailureClassTransient},
			{500, "", gen.FailureClassTransient},
			{503, "", gen.FailureClassTransient},
			{429, "", gen.FailureClassRateLimited},
			{401, "", gen.FailureClassAuthDenied},
			{403, "", gen.FailureClassAuthDenied},
			{400, "", gen.FailureClassUserInput},
			{404, "", gen.FailureClassUserInput},
			{413, "", gen.FailureClassUserInput},
			{422, "", gen.FailureClassUserInput},
			{409, "", gen.FailureClassInternal},
			{500, "special", gen.FailureClassStructural}, // quirks 的错误规则优先
		}
		for _, tc := range cases {
			u := errorServer(t, tc.status, nil, `{"error":{"code":"`+tc.code+`","message":"出错了"}}`)
			reg := newTestRegistry(t, caps, localProvider(u))
			var le *Error
			if err := call(t, reg, "p", "m"); !errors.As(err, &le) || le.Class != tc.want || le.Status != tc.status || le.Code != tc.code || le.Message != "出错了" {
				t.Errorf("%d %q：想要 %s，实际 %v", tc.status, tc.code, tc.want, err)
			}
		}

		// 网络错误 → transient
		dead := httptest.NewServer(http.NotFoundHandler())
		deadURL := dead.URL
		dead.Close()
		var le *Error
		if err := call(t, newTestRegistry(t, nil, localProvider(deadURL)), "p", "m"); !errors.As(err, &le) || le.Class != gen.FailureClassTransient || le.Status != 0 {
			t.Errorf("网络错误应当是 transient：%v", err)
		}
		// 超时 → transient
		// 先读完请求体：服务端读完请求体后才会发现客户端断开连接，否则 r.Context() 不会结束，Close 会一直等
		stop := make(chan struct{})
		slow := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			_, _ = io.Copy(io.Discard, r.Body)
			select {
			case <-r.Context().Done():
			case <-stop:
			}
		}))
		t.Cleanup(slow.Close)
		t.Cleanup(func() { close(stop) }) // 后注册的先执行：先放开处理函数，再关服务
		p := localProvider(slow.URL)
		p.TimeoutS = 1
		start := time.Now()
		if err := call(t, newTestRegistry(t, nil, p), "p", "m"); !errors.As(err, &le) || le.Class != gen.FailureClassTransient || le.Code != CodeTimeout || time.Since(start) > 5*time.Second {
			t.Errorf("超时应当是 transient / timeout：%v", err)
		}

		// Retry-After：秒数和 HTTP 日期
		u := errorServer(t, 429, map[string]string{"Retry-After": "7"}, "")
		if err := call(t, newTestRegistry(t, nil, localProvider(u)), "p", "m"); !errors.As(err, &le) || le.Class != gen.FailureClassRateLimited || le.RetryAfter != 7*time.Second {
			t.Errorf("Retry-After 秒数：%v", err)
		}
		now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
		if d := parseRetryAfter(now.Add(90*time.Second).Format(http.TimeFormat), now); d != 90*time.Second {
			t.Errorf("Retry-After HTTP 日期：%v", d)
		}
		if d := parseRetryAfter("soon", now); d != 0 {
			t.Errorf("不认识的 Retry-After 为 0：%v", d)
		}
		if le.Message != "Too Many Requests" {
			t.Errorf("错误体里没有说明时用状态文本：%q", le.Message)
		}

		// 错误说明里的密钥原文被替换
		u = errorServer(t, 401, nil, `{"error":{"code":"bad_key","message":"key key-123456 is invalid; also sk-abcdefghijklmnop"}}`)
		reg := NewRegistry([]Protocol{testProto{}}, nil, &testSecrets{vals: map[string]string{"k": "key-123456"}}, Options{})
		if err := reg.SetProviders([]ProviderConfig{{Name: "p", Protocol: ProtoChat, BaseURL: u, Auth: Auth{Type: AuthBearer, Secret: "k"}}}); err != nil {
			t.Fatal(err)
		}
		if err := call(t, reg, "p", "m"); !errors.As(err, &le) || strings.Contains(le.Error(), "key-123456") || strings.Contains(le.Error(), "sk-abc") || !strings.Contains(le.Message, "[REDACTED]") {
			t.Errorf("错误说明里的密钥应当被替换：%v", err)
		}
		// 说明截到 500 字
		u = errorServer(t, 400, nil, `{"error":{"message":"`+strings.Repeat("长", 800)+`"}}`)
		if err := call(t, newTestRegistry(t, nil, localProvider(u)), "p", "m"); !errors.As(err, &le) || len([]rune(le.Message)) > 501 {
			t.Errorf("说明应当截到 500 字：%d", len([]rune(le.Message)))
		}
		// 流里的错误事件也按规则分类
		srv := sseServer(t, sse(`{"t":"a"}`, `{"err":{"code":"special","message":"坏了"}}`))
		if err := call(t, newTestRegistry(t, caps, localProvider(srv.URL)), "p", "m"); !errors.As(err, &le) || le.Class != gen.FailureClassStructural || le.Provider != "p" {
			t.Errorf("流里的错误事件：%v", err)
		}
	})
}

func TestGuard(t *testing.T) {
	t.Run("E1001-T6 测试守卫", func(t *testing.T) {
		t.Setenv(EnvRecord, "")
		var logs bytes.Buffer
		reg := NewRegistry([]Protocol{testProto{}}, nil, nil, Options{Logger: slog.New(slog.NewTextHandler(&logs, nil))})
		if err := reg.SetProviders([]ProviderConfig{{Name: "remote", Protocol: ProtoChat, BaseURL: "https://example.invalid/v1", Auth: Auth{Type: AuthNone}}}); err != nil {
			t.Fatal(err)
		}
		var le *Error
		err := call(t, reg, "remote", "m")
		if !errors.As(err, &le) || le.Code != CodeTestGuard || !strings.Contains(le.Message, "测试中不允许调用真实模型") {
			t.Errorf("测试里请求真实地址应当被守卫拦下：%v", err)
		}
		if _, err := reg.ListModels(context.Background(), "remote"); !errors.As(err, &le) || le.Code != CodeTestGuard {
			t.Errorf("模型列表也应当被守卫拦下：%v", err)
		}
		// 回环地址照常
		srv, _ := recordingServer(t)
		if err := call(t, newTestRegistry(t, nil, localProvider(srv.URL)), "p", "m"); err != nil {
			t.Errorf("回环地址应当放行：%v", err)
		}
		for _, h := range []string{"http://127.0.0.1:9/x", "http://[::1]:9/x", "http://localhost:9/x"} {
			u, _ := url.Parse(h)
			if guardBlocks(u) {
				t.Errorf("%s 是回环地址，不应当拦", h)
			}
		}
		remote, _ := url.Parse("https://example.invalid/v1")
		if !guardBlocks(remote) {
			t.Error("没有 ANYDOOR_LLM_RECORD=1 时应当拦下远程地址")
		}
		t.Setenv(EnvRecord, "1")
		if guardBlocks(remote) {
			t.Error("ANYDOOR_LLM_RECORD=1 时守卫应当放行")
		}
	})
}
