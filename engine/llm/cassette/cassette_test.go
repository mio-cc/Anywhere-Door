package cassette

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// fakeTB 记下失败而不中止测试，用来断言 Transport 让测试失败（E1008-T2）。
type fakeTB struct {
	testing.TB
	mu   sync.Mutex
	errs []string
}

func (f *fakeTB) Helper() {}
func (f *fakeTB) Errorf(format string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errs = append(f.errs, fmt.Sprintf(format, args...))
}
func (f *fakeTB) Fatalf(format string, args ...any) { f.Errorf(format, args...) }
func (f *fakeTB) failed() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strings.Join(f.errs, "\n")
}

func setRecord(t *testing.T, v bool) {
	old := *Record
	*Record = v
	t.Cleanup(func() { *Record = old })
}

type result struct {
	status int
	ctype  string
	retry  string
	body   string
}

func do(t *testing.T, c *http.Client, method, url, body string) result {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rd)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer hdr-token-should-not-be-saved")
	req.Header.Set("x-api-key", "hdr-key-should-not-be-saved")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return result{resp.StatusCode, resp.Header.Get("Content-Type"), resp.Header.Get("Retry-After"), string(b)}
}

func TestCassette(t *testing.T) {
	t.Run("E1008-T1 录放往返", func(t *testing.T) {
		var hits atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			w.Header().Set("X-Other", "dropped")
			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "3")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = io.WriteString(w, `{"error":{"message":"slow down"}}`)
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: {\"x\":\"你好\"}\n\n: ping\n\ndata: [DONE]\n\n")
		}))
		defer srv.Close()
		dir := t.TempDir()

		var recorded []result
		t.Run("录制", func(t *testing.T) {
			setRecord(t, true)
			c := &http.Client{Transport: Transport(t, dir, "roundtrip")}
			recorded = append(recorded, do(t, c, http.MethodPost, srv.URL+"/v1/chat", `{"b":2,"a":[1,"x"]}`))
			recorded = append(recorded, do(t, c, http.MethodGet, srv.URL+"/v1/models", ""))
		})
		if hits.Load() != 2 {
			t.Fatalf("录制时应当真发 2 次请求：%d", hits.Load())
		}
		var f File
		b, err := os.ReadFile(filepath.Join(dir, "roundtrip.json"))
		if err != nil || json.Unmarshal(b, &f) != nil || len(f.Interactions) != 2 || f.RecordedAt == "" {
			t.Fatalf("录制后应当写出 2 条记录：%v\n%s", err, b)
		}
		if !strings.Contains(f.Note, "真实录制") || !strings.Contains(f.Note, srv.URL) {
			t.Errorf("录制的 note 应当写明是真实录制和请求的地址：%q", f.Note)
		}
		if h := f.Interactions[1].Response.Headers; len(h) != 2 || h["Retry-After"] != "3" {
			t.Errorf("响应头只留 Content-Type、Retry-After：%v", h)
		}

		setRecord(t, false)
		c := &http.Client{Transport: Transport(t, dir, "roundtrip")}
		// 请求体的键顺序不同也算一致（规范化后比较）
		got := []result{do(t, c, http.MethodPost, srv.URL+"/v1/chat", `{"a":[1,"x"],"b":2}`), do(t, c, http.MethodGet, srv.URL+"/v1/models", "")}
		for i := range got {
			if got[i] != recorded[i] {
				t.Errorf("第 %d 次回放和录制时不一致：\n录制 %+v\n回放 %+v", i+1, recorded[i], got[i])
			}
		}
		if hits.Load() != 2 {
			t.Errorf("回放时服务端不应当再收到请求：%d", hits.Load())
		}
	})

	t.Run("E1008-T2 缺记录与不一致", func(t *testing.T) {
		setRecord(t, false)
		dir := t.TempDir()
		f := File{Note: "按官方文档手写", Interactions: []Interaction{{
			Request:  Request{Method: http.MethodPost, Path: "/v1/x", Body: json.RawMessage(`{"model":"m","n":1}`)},
			Response: Response{Status: 200, Body: "data: {}\n\n"},
		}}}
		b, _ := json.Marshal(f)
		if err := os.WriteFile(filepath.Join(dir, "one.json"), b, 0o644); err != nil {
			t.Fatal(err)
		}
		send := func(rt http.RoundTripper, body string) error {
			req, _ := http.NewRequest(http.MethodPost, "https://example.invalid/v1/x", strings.NewReader(body))
			resp, err := rt.RoundTrip(req)
			if err == nil {
				_ = resp.Body.Close()
			}
			return err
		}

		// 文件不存在
		ft := &fakeTB{TB: t}
		if err := send(Transport(ft, dir, "missing"), `{}`); err == nil || !strings.Contains(ft.failed(), "missing.json") {
			t.Errorf("文件不存在时测试应当失败并写出文件名：%v / %s", err, ft.failed())
		}
		// 记录比请求少
		ft = &fakeTB{TB: t}
		rt := Transport(ft, dir, "one")
		if err := send(rt, `{"n":1,"model":"m"}`); err != nil || ft.failed() != "" {
			t.Fatalf("第 1 次应当命中：%v / %s", err, ft.failed())
		}
		if err := send(rt, `{"n":1,"model":"m"}`); err == nil || !strings.Contains(ft.failed(), "第 2 次") || !strings.Contains(ft.failed(), "one.json") {
			t.Errorf("记录不够时应当失败并写出请求序号：%v / %s", err, ft.failed())
		}
		// 请求体不一致
		ft = &fakeTB{TB: t}
		if err := send(Transport(ft, dir, "one"), `{"model":"m","n":2}`); err == nil || !strings.Contains(ft.failed(), "不一致") || !strings.Contains(ft.failed(), `"n": 2`) {
			t.Errorf("请求体不一致时应当失败并打印差异：%v / %s", err, ft.failed())
		}
		// 路径不一致
		ft = &fakeTB{TB: t}
		req, _ := http.NewRequest(http.MethodPost, "https://example.invalid/v1/y", strings.NewReader(`{"model":"m","n":1}`))
		if _, err := Transport(ft, dir, "one").RoundTrip(req); err == nil || !strings.Contains(ft.failed(), "/v1/y") {
			t.Errorf("路径不一致时应当失败：%v / %s", err, ft.failed())
		}
	})

	t.Run("E1008-T3 脱敏", func(t *testing.T) {
		secret := "my-secret-value-123"
		leaked := "sk-" + "live1234567890abcdef"
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Set-Cookie", "session=abc")
			_, _ = io.WriteString(w, "data: {\"echo\":\""+secret+"\",\"key\":\""+leaked+"\"}\n\n")
		}))
		defer srv.Close()
		dir := t.TempDir()
		var got result
		t.Run("录制", func(t *testing.T) {
			setRecord(t, true)
			c := &http.Client{Transport: Transport(t, dir, "redact", secret, "")}
			got = do(t, c, http.MethodPost, srv.URL+"/v1/chat", `{"prompt":"use `+secret+` please"}`)
		})
		if !strings.Contains(got.body, secret) {
			t.Errorf("录制时调用方拿到的是原始响应：%s", got.body)
		}
		b, err := os.ReadFile(filepath.Join(dir, "redact.json"))
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		for _, bad := range []string{secret, leaked, "hdr-token-should-not-be-saved", "hdr-key-should-not-be-saved", "Authorization", "x-api-key", "Set-Cookie"} {
			if strings.Contains(s, bad) {
				t.Errorf("录放文件里不应当有 %q：\n%s", bad, s)
			}
		}
		if !strings.Contains(s, "[REDACTED]") {
			t.Errorf("应当换成 [REDACTED]：\n%s", s)
		}
	})
}
