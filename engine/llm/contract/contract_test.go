package contract

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/mio-cc/anywhere-door/engine/llm"
	"github.com/mio-cc/anywhere-door/gen"
)

// mini 是测试里写的最小协议：请求体带上用例名（Req.Metadata），响应是一行一个 JSON 的 SSE。
type mini struct{}

func (mini) Name() string { return llm.ProtoChat }
func (mini) DefaultPaths() map[string]string {
	return map[string]string{"complete": "/complete", "models": "/models"}
}
func (mini) DefaultAuth() llm.Auth             { return llm.Auth{Type: llm.AuthBearer} }
func (mini) DefaultHeaders() map[string]string { return nil }
func (mini) Encode(r llm.Req, upstream string, _ llm.Caps) (map[string]any, error) {
	return map[string]any{"model": upstream, "case": r.Metadata[CaseKey], "stream": true}, nil
}
func (mini) Decode(body io.Reader, _ llm.Caps) llm.Stream {
	return &miniStream{sse: llm.NewSSEReader(body)}
}
func (mini) DecodeError(_ int, body []byte) (string, string) {
	var e struct{ Error string }
	_ = json.Unmarshal(body, &e)
	return "", e.Error
}
func (mini) DecodeModels(body []byte) ([]string, error) {
	var m struct{ Data []struct{ ID string } }
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	var out []string
	for _, d := range m.Data {
		out = append(out, d.ID)
	}
	return out, nil
}

type miniStream struct{ sse *llm.SSEReader }

func (s *miniStream) Next() (llm.Delta, error) {
	ev, err := s.sse.Next()
	if err != nil {
		return llm.Delta{}, err
	}
	var m struct {
		T  string
		TC *struct {
			I        int
			ID, N, A string
		}
		Done *struct {
			Finish          string
			In, Cached, Out int
		}
	}
	if err := json.Unmarshal([]byte(ev.Data), &m); err != nil {
		return llm.Delta{}, llm.Errorf(gen.FailureClassInternal, "bad_event", "%v", err)
	}
	switch {
	case m.Done != nil:
		return llm.Delta{Done: true, Resp: &llm.Resp{Finish: m.Done.Finish, Model: "mini-1", Usage: llm.Usage{In: m.Done.In, Cached: m.Done.Cached, Out: m.Done.Out}}}, nil
	case m.TC != nil:
		return llm.Delta{ToolCall: &llm.ToolCallDelta{Index: m.TC.I, ID: m.TC.ID, Name: m.TC.N, Args: m.TC.A}}, nil
	}
	return llm.Delta{Text: m.T}, nil
}

func (s *miniStream) Close() error { return nil }

func sse(events ...string) string {
	var b strings.Builder
	for _, e := range events {
		b.WriteString("data: " + e + "\n\n")
	}
	return b.String()
}

func TestRun(t *testing.T) {
	t.Run("E1001-T2 契约测试框架", func(t *testing.T) {
		var mu sync.Mutex
		seen := map[string]bool{}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/models" {
				mu.Lock()
				seen["models"] = true
				mu.Unlock()
				_, _ = io.WriteString(w, `{"data":[{"id":"mini-1"}]}`)
				return
			}
			var body struct{ Case string }
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			seen[body.Case] = true
			mu.Unlock()
			switch body.Case {
			case "error_auth":
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = io.WriteString(w, `{"error":"bad key"}`)
				return
			case "error_rate":
				w.Header().Set("Retry-After", "7")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = io.WriteString(w, `{"error":"slow down"}`)
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			done := `{"done":{"finish":"stop","in":12,"cached":4,"out":3}}`
			switch body.Case {
			case "tool_call":
				_, _ = io.WriteString(w, sse(`{"tc":{"i":0,"id":"call12345","n":"get_weather","a":"{\"ci"}}`, `{"tc":{"i":0,"a":"ty\":\"Paris\"}"}}`,
					`{"done":{"finish":"tool_calls","in":20,"out":9}}`))
			case "structured":
				_, _ = io.WriteString(w, sse(`{"t":"{\"name\":\"Alice\","}`, `{"t":"\"age\":30}"}`, done))
			default:
				_, _ = io.WriteString(w, sse(`{"t":"hel"}`, `{"t":"lo"}`, done))
			}
		}))
		defer srv.Close()

		dir := t.TempDir()
		for _, c := range Cases {
			if err := os.WriteFile(filepath.Join(dir, c+".json"), []byte(`{}`), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		Run(t, Suite{
			Protocol:  mini{},
			Provider:  llm.ProviderConfig{Name: "mini", Protocol: llm.ProtoChat, BaseURL: srv.URL, Auth: llm.Auth{Type: llm.AuthBearer, Secret: "mini"}},
			Model:     "mini-1",
			Cassettes: dir,
		})
		var got []string
		for k := range seen {
			got = append(got, k)
		}
		sort.Strings(got)
		want := append([]string(nil), Cases...)
		sort.Strings(want)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("§5.6 的每个用例都应当执行到：想要 %v，实际 %v", want, got)
		}

		// 缺录放文件时用例失败并说明文件名
		err := checkCassette(t.TempDir(), "tool_call")
		if err == nil || !strings.Contains(err.Error(), "tool_call.json") {
			t.Errorf("缺录放文件时应当说明文件名：%v", err)
		}
		if err := checkCassette(dir, "tool_call"); err != nil {
			t.Errorf("文件存在时不报错：%v", err)
		}
	})

	t.Run("E1001-T2 请求固定", func(t *testing.T) {
		// 录制和回放发出的请求必须相同：同一个用例两次得到的请求完全一样
		for _, c := range Cases {
			a, _ := json.Marshal(Request(c, "m"))
			b, _ := json.Marshal(Request(c, "m"))
			if string(a) != string(b) {
				t.Errorf("%s 的请求不固定", c)
			}
		}
		if r := Request("tool_result", "m"); len(r.Messages) != 4 || r.Messages[3].ToolCallID != CallID || r.Messages[2].ToolCalls[0].ID != CallID {
			t.Errorf("tool_result 的请求：%+v", r)
		}
		if EnvSecret("my-key") != "ANYDOOR_DEV_SECRET_MY_KEY" {
			t.Errorf("EnvSecret：%s", EnvSecret("my-key"))
		}
	})
}
