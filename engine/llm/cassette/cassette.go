// Package cassette 是 HTTP 层的录放（设计文档 E.10.08）：录下真实的请求和响应（包括 SSE 原文），之后原样回放，
// 协议的编码、解码因此是对着真实格式测的（契约测试，E.10-R3）。
//
// 只回放：按顺序找 dir/<name>.json 里的记录，找不到或请求不一致就让测试失败，不回退到真实调用。
// go test ... -record：真发请求（经过 llm 的测试守卫，E.10.01 §5.3），把请求和响应写进 dir/<name>.json。
package cassette

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mio-cc/anywhere-door/engine/llm"
)

// Record 为真时真实调用模型并更新录放。
var Record = flag.Bool("record", false, "真实调用模型并更新录放（只在开发机上用，Q.06-R7）")

// File 是一份录放：一个用例里按顺序发生的若干次请求（E.10.08 §4）。
type File struct {
	RecordedAt   string        `json:"recorded_at"`
	Note         string        `json:"note"`
	Interactions []Interaction `json:"interactions"`
}

// Interaction 是一次请求和它的响应。
type Interaction struct {
	Request  Request  `json:"request"`
	Response Response `json:"response"`
}

// Request 是录下的请求：不写请求头（S.03-R7）。
type Request struct {
	Method string          `json:"method"`
	Path   string          `json:"path"`
	Body   json.RawMessage `json:"body,omitempty"`
}

// Response 是录下的响应：响应头只留 Content-Type、Retry-After。
type Response struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body"`
}

var keptHeaders = []string{"Content-Type", "Retry-After"}

// Transport 是给 llm.Options.Transport 用的录放传输层。secrets 是录制时要从文件里抹掉的密钥原文。
func Transport(t testing.TB, dir, name string, secrets ...string) http.RoundTripper {
	t.Helper()
	p := filepath.Join(dir, name+".json")
	var keys []string
	for _, s := range secrets {
		if s != "" {
			keys = append(keys, s)
		}
	}
	if *Record {
		t.Setenv(llm.EnvRecord, "1")
		next, err := llm.NewTransport("")
		if err != nil {
			t.Errorf("cassette: %v", err)
			return failing{err}
		}
		r := &recorder{t: t, path: p, keys: keys, next: next}
		t.Cleanup(r.save)
		return r
	}
	pl := &player{t: t, path: p, keys: keys}
	b, err := os.ReadFile(p)
	if err != nil {
		pl.loadErr = fmt.Errorf("录放文件 %s 读不了：%v（在开发机上用 -record 录制，或按官方文档手写，E.10.08 §5）", p, err)
	} else if err := json.Unmarshal(b, &pl.file); err != nil {
		pl.loadErr = fmt.Errorf("录放文件 %s 的格式不对：%v", p, err)
	}
	if pl.loadErr != nil {
		t.Errorf("cassette: %v", pl.loadErr)
	}
	return pl
}

type failing struct{ err error }

func (f failing) RoundTrip(r *http.Request) (*http.Response, error) {
	_, _ = readBody(r)
	return nil, f.err
}

// player 按顺序回放。
type player struct {
	t       testing.TB
	path    string
	keys    []string
	file    File
	loadErr error

	mu sync.Mutex
	n  int
}

func (p *player) RoundTrip(req *http.Request) (*http.Response, error) {
	body, err := readBody(req)
	if err != nil {
		return nil, err
	}
	if p.loadErr != nil {
		return nil, p.loadErr
	}
	p.mu.Lock()
	n := p.n
	p.n++
	p.mu.Unlock()
	if n >= len(p.file.Interactions) {
		err := fmt.Errorf("录放文件 %s 的记录不够：第 %d 次请求没有对应的记录（文件里只有 %d 条）", p.path, n+1, len(p.file.Interactions))
		p.t.Errorf("cassette: %v", err)
		return nil, err
	}
	it := p.file.Interactions[n]
	if diff := compare(it.Request, req.Method, req.URL.Path, redactBody(body, p.keys)); diff != "" {
		err := fmt.Errorf("录放文件 %s 的第 %d 次请求和记录不一致：\n%s", p.path, n+1, diff)
		p.t.Errorf("cassette: %v", err)
		return nil, err
	}
	return it.Response.toHTTP(req), nil
}

func (r Response) toHTTP(req *http.Request) *http.Response {
	h := http.Header{}
	for k, v := range r.Headers {
		h.Set(k, v)
	}
	return &http.Response{
		Status:        strconv.Itoa(r.Status) + " " + http.StatusText(r.Status),
		StatusCode:    r.Status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        h,
		Body:          io.NopCloser(strings.NewReader(r.Body)),
		ContentLength: int64(len(r.Body)),
		Request:       req,
	}
}

// compare 比较方法、路径和规范化后的请求体（键排序）；一致时返回 ""，否则返回差异说明。
func compare(want Request, method, path string, body json.RawMessage) string {
	var diffs []string
	if want.Method != method {
		diffs = append(diffs, fmt.Sprintf("方法：记录 %s，实际 %s", want.Method, method))
	}
	if want.Path != path {
		diffs = append(diffs, fmt.Sprintf("路径：记录 %s，实际 %s", want.Path, path))
	}
	a, b := canonical(want.Body), canonical(body)
	if a != b {
		diffs = append(diffs, "请求体不一致："+lineDiff(a, b))
	}
	return strings.Join(diffs, "\n")
}

// canonical 是缩进、键排序后的 JSON；不是 JSON 时原样返回。
func canonical(b json.RawMessage) string {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		return ""
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return string(b)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(b)
	}
	return string(out)
}

// lineDiff 写出第一处不同的行和前后几行。
func lineDiff(a, b string) string {
	al, bl := strings.Split(a, "\n"), strings.Split(b, "\n")
	i := 0
	for i < len(al) && i < len(bl) && al[i] == bl[i] {
		i++
	}
	ctx := func(ls []string) string {
		lo, hi := max(0, i-3), min(len(ls), i+4)
		return strings.Join(ls[lo:hi], "\n")
	}
	return fmt.Sprintf("第 %d 行起不同\n--- 记录\n%s\n+++ 实际\n%s", i+1, ctx(al), ctx(bl))
}

// recorder 真发请求，把请求和响应记下来，测试结束时写文件。
type recorder struct {
	t    testing.TB
	path string
	keys []string
	next http.RoundTripper

	mu    sync.Mutex
	items []Interaction
	hosts []string // 录制时请求过的地址（scheme://host），写进 note
}

func (r *recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	body, err := readBody(req)
	if err != nil {
		return nil, err
	}
	out := req.Clone(req.Context())
	out.Body = io.NopCloser(bytes.NewReader(body))
	out.ContentLength = int64(len(body))
	if len(body) == 0 {
		out.Body = nil
	}
	resp, err := r.next.RoundTrip(out)
	if err != nil {
		return nil, err
	}
	rb, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	hs := map[string]string{}
	for _, k := range keptHeaders {
		if v := resp.Header.Get(k); v != "" {
			hs[k] = v
		}
	}
	r.mu.Lock()
	if h := req.URL.Scheme + "://" + req.URL.Host; !slices.Contains(r.hosts, h) {
		r.hosts = append(r.hosts, h)
	}
	r.items = append(r.items, Interaction{
		Request:  Request{Method: req.Method, Path: req.URL.Path, Body: redactBody(body, r.keys)},
		Response: Response{Status: resp.StatusCode, Headers: hs, Body: llm.Redact(string(rb), r.keys...)},
	})
	r.mu.Unlock()
	resp.Body = io.NopCloser(bytes.NewReader(rb))
	resp.ContentLength = int64(len(rb))
	return resp, nil
}

func (r *recorder) save() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.items) == 0 {
		return
	}
	f := File{RecordedAt: time.Now().UTC().Format(time.RFC3339), Note: "真实录制（go test -record）：" + strings.Join(r.hosts, "、"), Interactions: r.items}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(f); err != nil {
		r.t.Errorf("cassette: 写录放 %s 失败：%v", r.path, err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		r.t.Errorf("cassette: 写录放 %s 失败：%v", r.path, err)
		return
	}
	if err := os.WriteFile(r.path, buf.Bytes(), 0o644); err != nil {
		r.t.Errorf("cassette: 写录放 %s 失败：%v", r.path, err)
	}
}

func readBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}
	defer func() { _ = req.Body.Close() }()
	return io.ReadAll(req.Body)
}
