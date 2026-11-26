package tools_test

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mio-cc/anywhere-door/engine/blob"
	"github.com/mio-cc/anywhere-door/engine/blob/fsblob"
	"github.com/mio-cc/anywhere-door/engine/tools"
	"github.com/mio-cc/anywhere-door/gen"
)

// runeCounter 一个字算一个 token，让测试里的长度好算。
type runeCounter struct{}

func (runeCounter) CountText(s string) int { return utf8.RuneCountInString(s) }

func newBlobs(t *testing.T) blob.Store {
	t.Helper()
	s, err := fsblob.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func readRef(t *testing.T, s blob.Store, ref blob.Ref) string {
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

// filler 是不含查询词的填充文字。
func filler(n int) string {
	const unit = "平常的文字内容，"
	var b strings.Builder
	for utf8.RuneCountInString(b.String()) < n {
		b.WriteString(unit)
	}
	return string([]rune(b.String())[:n])
}

func snippetTokens(r tools.Result) int {
	n := 0
	for _, s := range r.Snippets {
		n += utf8.RuneCountInString(s.Text)
	}
	return n
}

// badStore 写入总是失败。
type badStore struct{}

func (badStore) Get(context.Context, blob.Ref) (io.ReadCloser, blob.Meta, error) {
	return nil, blob.Meta{}, blob.ErrNotFound
}
func (badStore) Stat(context.Context, blob.Ref) (blob.Meta, error) {
	return blob.Meta{}, blob.ErrNotFound
}
func (badStore) Put(context.Context, io.Reader, string) (blob.Ref, error) {
	return "", errors.New("磁盘满了")
}

func TestTrim(t *testing.T) {
	ctx := context.Background()
	cnt := runeCounter{}

	t.Run("E1102-T1 裁剪长度", func(t *testing.T) {
		s := newBlobs(t)
		var b strings.Builder
		for i := 0; i < 200; i++ {
			b.WriteString("第" + strconv.Itoa(i) + "段：" + filler(500) + "\n\n") // 200 × 500 字以上
		}
		text := b.String()
		if utf8.RuneCountInString(text) < 100000 {
			t.Fatalf("测试文本不够长：%d", utf8.RuneCountInString(text))
		}
		for _, mode := range []tools.TrimMode{tools.TrimRelevant, tools.TrimHead} {
			r, err := tools.Trim(ctx, tools.Raw{Text: text}, tools.TrimOpts{Mode: mode, MaxTokens: 2000}, s, cnt)
			if err != nil {
				t.Fatal(err)
			}
			if n := snippetTokens(r); n == 0 || n > 2000 {
				t.Errorf("%s：片段 %d token，应在 1–2000", mode, n)
			}
			if r.Tokens > 2000+utf8.RuneCountInString(r.Summary) {
				t.Errorf("%s：Tokens %d 超出", mode, r.Tokens)
			}
			if r.Ref == "" || readRef(t, s, r.Ref) != text {
				t.Errorf("%s：引用读不回原文", mode)
			}
			if utf8.RuneCountInString(r.Summary) > 300 || r.Bytes != len(text) {
				t.Errorf("%s：摘要 %d 字、Bytes %d", mode, utf8.RuneCountInString(r.Summary), r.Bytes)
			}
		}
		// 只有一块且超过上限：从块尾截
		one := filler(3000)
		r, err := tools.Trim(ctx, tools.Raw{Text: one}, tools.TrimOpts{Mode: tools.TrimHead, MaxTokens: 100}, s, cnt)
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Snippets) != 1 || utf8.RuneCountInString(r.Snippets[0].Text) != 100 {
			t.Errorf("超长单块应截到 100 字：%+v", r.Snippets)
		}
	})

	t.Run("E1102-T2 相关性", func(t *testing.T) {
		s := newBlobs(t)
		var b strings.Builder
		for i := 0; i < 60; i++ {
			p := filler(450)
			if i == 37 {
				p = filler(200) + "量子退火的成本很高" + filler(241)
			}
			b.WriteString(p + "\n\n")
		}
		r, err := tools.Trim(ctx, tools.Raw{Text: b.String()}, tools.TrimOpts{MaxTokens: 1000, Query: "量子退火 成本"}, s, cnt)
		if err != nil {
			t.Fatal(err)
		}
		hit := false
		for i, sn := range r.Snippets {
			if strings.Contains(sn.Text, "量子退火") {
				hit = true
				if sn.Score <= 0 {
					t.Errorf("命中的片段分数应大于 0：%v", sn.Score)
				}
			}
			if i > 0 && r.Snippets[i-1].Offset >= sn.Offset {
				t.Errorf("片段应按原文位置排序：%d ≥ %d", r.Snippets[i-1].Offset, sn.Offset)
			}
		}
		if !hit {
			t.Fatalf("没有选出含关键词的片段：%d 段", len(r.Snippets))
		}
		if !strings.Contains(r.Snippets[0].Text, "量子退火") && len(r.Snippets) > 2 {
			t.Errorf("关键词段应该排在前面被选中；第一段：%.20s", r.Snippets[0].Text)
		}
	})

	t.Run("E1102-T3 引用必有", func(t *testing.T) {
		s := newBlobs(t)
		small := "一段很短的结果"
		r, err := tools.Trim(ctx, tools.Raw{Text: small}, tools.TrimOpts{Mode: tools.TrimNone}, s, cnt)
		if err != nil {
			t.Fatal(err)
		}
		if r.Ref == "" || len(r.Snippets) != 1 || r.Snippets[0].Text != small {
			t.Errorf("none 小结果整段返回：%+v", r)
		}
		big := filler(4000) // 12000 字节，超过 8KB
		r, err = tools.Trim(ctx, tools.Raw{Text: big}, tools.TrimOpts{Mode: tools.TrimNone}, s, cnt)
		if err != nil {
			t.Fatal(err)
		}
		if r.Ref == "" || len(r.Snippets) != 0 || r.Summary == "" {
			t.Errorf("none 大结果只留摘要和引用：%d 段，摘要 %q", len(r.Snippets), r.Summary)
		}
		r, err = tools.Trim(ctx, tools.Raw{Data: []byte(`{"a":1}`)}, tools.TrimOpts{}, s, cnt)
		if err != nil {
			t.Fatal(err)
		}
		if r.Ref == "" || string(r.Data) != `{"a":1}` || readRef(t, s, r.Ref) != `{"a":1}` {
			t.Errorf("只有 Data 时用它的 JSON 作原文：%+v", r)
		}
		_, err = tools.Trim(ctx, tools.Raw{Text: small}, tools.TrimOpts{}, badStore{}, cnt)
		var te *tools.Error
		if !errors.As(err, &te) || te.Class != gen.FailureClassTransient {
			t.Errorf("写工作区失败应为 transient：%v", err)
		}
		_, err = tools.Trim(ctx, tools.Raw{Text: small}, tools.TrimOpts{Mode: "middle"}, s, cnt)
		if !errors.As(err, &te) || te.Class != gen.FailureClassUserInput {
			t.Errorf("不认识的方式应为 user_input：%v", err)
		}
	})
}
