package wsread_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mio-cc/anywhere-door/engine/authz"
	"github.com/mio-cc/anywhere-door/engine/blob"
	"github.com/mio-cc/anywhere-door/engine/blob/fsblob"
	"github.com/mio-cc/anywhere-door/engine/tools"
	"github.com/mio-cc/anywhere-door/engine/tools/builtin/wsread"
	"github.com/mio-cc/anywhere-door/engine/tools/fake"
	"github.com/mio-cc/anywhere-door/gen"
)

type seenSet map[blob.Ref]bool

func (s seenSet) Seen(r blob.Ref) bool { return s[r] }

// countingReader 记下 Get 调了几次。
type countingReader struct {
	blob.Reader
	gets int
}

func (c *countingReader) Get(ctx context.Context, r blob.Ref) (io.ReadCloser, blob.Meta, error) {
	c.gets++
	return c.Reader.Get(ctx, r)
}

func put(t *testing.T, s blob.Store, text string) blob.Ref {
	t.Helper()
	ref, err := s.Put(context.Background(), strings.NewReader(text), "text/plain; charset=utf-8")
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func newStore(t *testing.T) blob.Store {
	t.Helper()
	s, err := fsblob.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func call(t *testing.T, impl tools.Impl, args string) (tools.Raw, error) {
	t.Helper()
	return impl.Invoke(context.Background(), tools.Call{Args: json.RawMessage(args)})
}

func meta(t *testing.T, r tools.Raw) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(r.Data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func code(err error) string {
	var te *tools.Error
	if errors.As(err, &te) && te.Class == gen.FailureClassUserInput {
		return te.Code
	}
	return ""
}

func TestWsRead(t *testing.T) {
	s := newStore(t)
	text := strings.Repeat("天地玄黄宇宙洪荒", 1250) // 1 万字
	ref := put(t, s, text)

	t.Run("E1108-T1 分段读取", func(t *testing.T) {
		w := wsread.New(s, seenSet{ref: true})
		r, err := call(t, w, `{"ref":"`+string(ref)+`","offset":9000,"limit":4000}`)
		if err != nil {
			t.Fatal(err)
		}
		m := meta(t, r)
		if utf8.RuneCountInString(r.Text) != 1000 || m["next_offset"].(float64) != -1 || m["total"].(float64) != 10000 {
			t.Errorf("读到结尾：%d 字，%v", utf8.RuneCountInString(r.Text), m)
		}
		if r.Text != string([]rune(text)[9000:]) {
			t.Error("内容不对")
		}
		r, err = call(t, w, `{"ref":"`+string(ref)+`"}`)
		if err != nil {
			t.Fatal(err)
		}
		if m := meta(t, r); utf8.RuneCountInString(r.Text) != 4000 || m["next_offset"].(float64) != 4000 {
			t.Errorf("默认读前 4000 字：%d，%v", utf8.RuneCountInString(r.Text), m)
		}
		if len(r.Sources) != 1 || r.Sources[0] != string(ref) {
			t.Errorf("来源应是这个引用：%v", r.Sources)
		}
	})

	t.Run("E1108-T2 越权", func(t *testing.T) {
		cr := &countingReader{Reader: s}
		_, err := call(t, wsread.New(cr, seenSet{}), `{"ref":"`+string(ref)+`"}`)
		if code(err) != "ref_not_in_run" || cr.gets != 0 {
			t.Errorf("本运行没出现过的引用应被拒且不读工作区：%v，读了 %d 次", err, cr.gets)
		}
		_, err = call(t, wsread.New(cr, nil), `{"ref":"`+string(ref)+`"}`)
		if code(err) != "ref_not_in_run" {
			t.Errorf("没有 Seen 时一律拒绝：%v", err)
		}
	})

	t.Run("E1108-T3 关键词", func(t *testing.T) {
		doc := put(t, s, "第一段 Apple 苹果\n\n第二段 香蕉\n\n第三段 apple 派")
		r, err := call(t, wsread.New(s, seenSet{doc: true}), `{"ref":"`+string(doc)+`","query":"APPLE"}`)
		if err != nil {
			t.Fatal(err)
		}
		if r.Text != "第一段 Apple 苹果\n\n第三段 apple 派" {
			t.Errorf("只留命中的段落：%q", r.Text)
		}
	})

	t.Run("E1108-T4 参数与引用", func(t *testing.T) {
		missing := blob.RefOf(strings.Repeat("a", 64))
		w := wsread.New(s, seenSet{ref: true, missing: true})
		for args, want := range map[string]string{
			`{"ref":"ws://xx"}`:                             "bad_ref",
			`{"ref":"` + string(missing) + `"}`:             "ref_not_found",
			`{"ref":"` + string(ref) + `","limit":30000}`:   "bad_args",
			`{"ref":"` + string(ref) + `","offset":-1}`:     "bad_args",
			`{"ref":"` + string(ref) + `","path":"/etc/x"}`: "bad_args",
		} {
			if _, err := call(t, w, args); code(err) != want {
				t.Errorf("%s：应为 %s，实际 %v", args, want, err)
			}
		}
		reg := tools.NewRegistry()
		if err := reg.Register(w); err != nil {
			t.Fatalf("应能注册：%v", err)
		}
		h := tools.NewHandle(w, tools.Env{
			RunID: "r", ExecKey: "n", Authz: authz.Fixed{}, Ledger: fake.NewLedger(), Blobs: s,
			Trim: tools.TrimOpts{Mode: tools.TrimNone},
		})
		res, err := h.Call(context.Background(), json.RawMessage(`{"ref":"`+string(ref)+`","limit":10}`), false)
		if err != nil {
			t.Fatal(err)
		}
		if !res.Taint || len(res.TaintSrc) != 1 || res.TaintSrc[0] != string(ref) || res.Summary != string([]rune(text)[:10]) {
			t.Errorf("经 Handle 调用应带污染和来源：%+v", res)
		}
	})
}
