package ctxasm_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/mio-cc/anywhere-door/engine/blob"
	"github.com/mio-cc/anywhere-door/engine/blob/fsblob"
	"github.com/mio-cc/anywhere-door/engine/ctxasm"
	"github.com/mio-cc/anywhere-door/gen"
)

func newBlobs(t *testing.T) blob.Store {
	t.Helper()
	s, err := fsblob.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func readBlob(t *testing.T, s blob.Store, ref blob.Ref) string {
	t.Helper()
	rc, _, err := s.Get(context.Background(), ref)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rc.Close() }()
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// fakeSum 记下收到的请求，返回固定长度的摘要。
type fakeSum struct {
	texts []string
	goals []int
}

func (f *fakeSum) Summarize(_ context.Context, text string, _ []string, maxTokens int) (string, error) {
	f.texts = append(f.texts, text)
	f.goals = append(f.goals, maxTokens)
	return strings.Repeat("摘", min(maxTokens, 30)), nil
}

// wideCounter 只计非 ASCII 字符。
type wideCounter struct{}

func (wideCounter) CountText(s string) int {
	n := 0
	for _, r := range s {
		if r > 127 {
			n++
		}
	}
	return n
}

func part(name string, n, prio, order int) ctxasm.Part {
	return ctxasm.Part{Name: name, Text: strings.Repeat("字", n-1) + "。", Tokens: n, Priority: prio, Order: order}
}

func total(ps []ctxasm.Part) int {
	n := 0
	for _, p := range ps {
		n += p.Tokens
	}
	return n
}

func TestFit(t *testing.T) {
	ctx := context.Background()

	t.Run("E1304-T1 pointer", func(t *testing.T) {
		s := newBlobs(t)
		in := []ctxasm.Part{part("任务", 100, 0, 0), part("参考", 1000, 3, 1)}
		orig := in[1].Text
		out, dropped, err := ctxasm.Fit(ctx, in, 500, "", ctxasm.FitEnv{Blobs: s, Counter: runeCounter{}})
		if err != nil {
			t.Fatal(err)
		}
		p := out[1]
		if !p.Pointer || !strings.HasPrefix(p.Text, "【参考】") || !strings.Contains(p.Text, "[原文: ws://sha256:") {
			t.Errorf("应变成指针：%q", p.Text)
		}
		if readBlob(t, s, p.Ref) != orig || total(out) > 500 || out[0].Text != in[0].Text {
			t.Errorf("引用读回原文、总量不超、高优先级不动")
		}
		if len(dropped) != 1 || dropped[0].Mode != "pointer" || dropped[0].Section != "参考" || dropped[0].Saved != 1000-p.Tokens {
			t.Errorf("Dropped 不对：%+v", dropped)
		}
		if got := ctxasm.PointerText("甲", "第一句话。第二句", "ws://sha256:x"); got != "【甲】第一句话。 [原文: ws://sha256:x]" {
			t.Errorf("PointerText：%q", got)
		}
	})

	t.Run("E1304-T2 summarize", func(t *testing.T) {
		s := newBlobs(t)
		fs := &fakeSum{}
		in := []ctxasm.Part{part("任务", 100, 0, 0), part("资料", 1000, 2, 1), part("旁注", 100, 3, 2)}
		out, dropped, err := ctxasm.Fit(ctx, in, 600, "summarize", ctxasm.FitEnv{Blobs: s, Counter: runeCounter{}, Summarizer: fs})
		if err != nil {
			t.Fatal(err)
		}
		// 旁注目标 0 → 改用指针，但指针不比原文短，不动；资料目标 400 → 摘要
		if total(out) > 600 || len(fs.texts) != 1 || fs.texts[0] != in[1].Text || fs.goals[0] != 400 {
			t.Fatalf("摘要员应收到资料原文和目标 400：%d %v", total(out), fs.goals)
		}
		if len(dropped) != 1 || dropped[0].Mode != "summarize" || dropped[0].Section != "资料" || out[2].Text != in[2].Text {
			t.Errorf("Dropped 不对：%+v", dropped)
		}
		// 目标小于 50 时改用 pointer（这里用不计 ASCII 的计数，让指针足够短）
		fs = &fakeSum{}
		doc := ctxasm.Part{Name: "资料", Text: "短句。" + strings.Repeat("字", 397), Tokens: 400, Priority: 2, Order: 1}
		out, dropped, err = ctxasm.Fit(ctx, []ctxasm.Part{part("任务", 10, 0, 0), doc}, 50, "summarize",
			ctxasm.FitEnv{Blobs: s, Counter: wideCounter{}, Summarizer: fs})
		if err != nil {
			t.Fatal(err)
		}
		if len(fs.texts) != 0 || !out[1].Pointer || len(dropped) != 1 || dropped[0].Mode != "pointer" {
			t.Errorf("目标 40 应改用 pointer：%+v", dropped)
		}
		// transform: summary 的段不超预算也摘要
		fs = &fakeSum{}
		sp := part("经验", 100, 1, 0)
		sp.Summary = true
		_, dropped, err = ctxasm.Fit(ctx, []ctxasm.Part{sp}, 10000, "pointer", ctxasm.FitEnv{Blobs: s, Counter: runeCounter{}, Summarizer: fs})
		if err != nil || len(fs.texts) != 1 || fs.goals[0] != 10000 || dropped[0].Mode != "summarize" {
			t.Errorf("summary 段应摘要：%v %v %+v", err, fs.goals, dropped)
		}
		if _, _, err := ctxasm.Fit(ctx, []ctxasm.Part{sp}, 10000, "pointer", ctxasm.FitEnv{Blobs: s}); err == nil {
			t.Error("没有摘要执行器应报错")
		}
	})

	t.Run("E1304-T3 truncate", func(t *testing.T) {
		s := newBlobs(t)
		in := []ctxasm.Part{part("任务", 100, 0, 0), part("日志", 1000, 2, 1)}
		out, dropped, err := ctxasm.Fit(ctx, in, 400, "truncate", ctxasm.FitEnv{Blobs: s, Counter: runeCounter{}})
		if err != nil {
			t.Fatal(err)
		}
		p := out[1]
		if total(out) > 400 || !strings.HasPrefix(p.Text, "字字字") || !strings.HasSuffix(p.Text, "）") || !strings.Contains(p.Text, "已截断，原文: ws://") {
			t.Errorf("截断后不超出、末尾有引用：%d", total(out))
		}
		if total(out) < 390 || dropped[0].Mode != "truncate" || readBlob(t, s, p.Ref) != in[1].Text {
			t.Errorf("应尽量用满预算并保存原文：%d %+v", total(out), dropped)
		}
	})

	t.Run("E1304-T4 reject", func(t *testing.T) {
		_, _, err := ctxasm.Fit(ctx, []ctxasm.Part{part("a", 300, 1, 0)}, 100, "reject", ctxasm.FitEnv{Blobs: newBlobs(t), Counter: runeCounter{}})
		var ce *ctxasm.Error
		if !errors.Is(err, ctxasm.ErrContextOverflow) || !errors.As(err, &ce) || ce.Class != gen.FailureClassUserInput {
			t.Errorf("reject 超出应为 ErrContextOverflow（user_input）：%v", err)
		}
		out, _, err := ctxasm.Fit(ctx, []ctxasm.Part{part("a", 50, 1, 0)}, 100, "reject", ctxasm.FitEnv{Counter: runeCounter{}})
		if err != nil || out[0].Tokens != 50 {
			t.Errorf("不超出时 reject 不动：%v", err)
		}
		if _, _, err := ctxasm.Fit(ctx, nil, 100, "drop", ctxasm.FitEnv{}); err == nil {
			t.Error("不认识的方式应报错")
		}
	})

	t.Run("E1304-T5 优先级与锁定", func(t *testing.T) {
		s := newBlobs(t)
		env := ctxasm.FitEnv{Blobs: s, Counter: runeCounter{}}
		lock := part("约束", 300, 0, 1)
		lock.Locked = true
		req := part("任务", 300, 0, 0)
		req.Required = true
		in := []ctxasm.Part{req, lock, part("甲", 600, 2, 2), part("乙", 600, 3, 3), part("丙", 600, 3, 4)}
		out, dropped, err := ctxasm.Fit(ctx, in, 2000, "pointer", env)
		if err != nil {
			t.Fatal(err)
		}
		if len(dropped) != 1 || dropped[0].Section != "丙" {
			t.Errorf("先收缩优先级数字大、位置靠后的：%+v", dropped)
		}
		if out[0].Text != req.Text || out[1].Text != lock.Text || out[2].Name != "甲" || out[4].Name != "丙" {
			t.Errorf("必需段、锁定段不动，按位置排序")
		}
		_, _, err = ctxasm.Fit(ctx, in, 500, "pointer", env)
		if !errors.Is(err, ctxasm.ErrContextOverflow) {
			t.Errorf("只剩必需段和锁定段还超出应报错：%v", err)
		}
		// 段自己的预算上限先生效：总量够，但"甲"超过自己的 10%
		a := part("甲", 300, 0, 0)
		a.Budget = 0.1
		out, dropped, err = ctxasm.Fit(ctx, []ctxasm.Part{a, part("乙", 100, 5, 1)}, 2000, "truncate", env)
		if err != nil {
			t.Fatal(err)
		}
		if out[0].Tokens > 200 || len(dropped) != 1 || dropped[0].Section != "甲" || out[1].Tokens != 100 {
			t.Errorf("段上限 200：%d %+v", out[0].Tokens, dropped)
		}
	})
}
