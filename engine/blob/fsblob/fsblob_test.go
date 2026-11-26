package fsblob

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mio-cc/anywhere-door/engine/blob"
)

var ctx = context.Background()

func open(t *testing.T, dir string) blob.Store {
	t.Helper()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func readAll(t *testing.T, s blob.Store, ref blob.Ref) ([]byte, blob.Meta, error) {
	t.Helper()
	rc, m, err := s.Get(ctx, ref)
	if err != nil {
		return nil, m, err
	}
	defer func() { _ = rc.Close() }()
	b, err := io.ReadAll(rc)
	return b, m, err
}

// files 是 dir 下的普通文件（相对路径），不含 .tmp/ 里的。
func files(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() && d.Name() == ".tmp" {
			return filepath.SkipDir
		}
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(dir, p)
			out = append(out, rel)
		}
		return nil
	})
	return out
}

type failingReader struct{ n int }

func (r *failingReader) Read(p []byte) (int, error) {
	if r.n <= 0 {
		return 0, errors.New("读到一半断了")
	}
	k := min(len(p), r.n)
	for i := range p[:k] {
		p[i] = 'x'
	}
	r.n -= k
	return k, nil
}

func TestFSBlob(t *testing.T) {
	t.Run("E08-T1 往返校验", func(t *testing.T) {
		s := open(t, t.TempDir())
		n := 1000
		if os.Getenv("ANYDOOR_PERF") == "1" {
			n = 10000
		}
		refs := make([]blob.Ref, n)
		data := make([][]byte, n)
		for i := range n {
			data[i] = make([]byte, 1+i%700)
			_, _ = rand.Read(data[i])
			ref, err := s.Put(ctx, bytes.NewReader(data[i]), "application/octet-stream")
			if err != nil {
				t.Fatal(err)
			}
			refs[i] = ref
		}
		for i := range n {
			b, m, err := readAll(t, s, refs[i])
			if err != nil || !bytes.Equal(b, data[i]) || m.Size != int64(len(data[i])) || m.MediaType != "application/octet-stream" {
				t.Fatalf("第 %d 个读回不对：%v %+v", i, err, m)
			}
		}
		if _, err := blob.ParseRef(string(refs[0])); err != nil {
			t.Error(err)
		}
		for _, bad := range []string{"", "ws://sha256:ABC", "sha256:" + refs[0].Hash(), "ws://sha256:" + strings.Repeat("g", 64)} {
			if _, err := blob.ParseRef(bad); !errors.Is(err, blob.ErrBadRef) {
				t.Errorf("%q 应当是 ErrBadRef：%v", bad, err)
			}
		}
		if _, err := s.Stat(ctx, "ws://bad"); !errors.Is(err, blob.ErrBadRef) {
			t.Errorf("Stat 格式不对的引用：%v", err)
		}
	})

	t.Run("E08-T2 去重", func(t *testing.T) {
		dir := t.TempDir()
		s := open(t, dir)
		a, err1 := s.Put(ctx, strings.NewReader("同样的内容"), "text/plain")
		b, err2 := s.Put(ctx, strings.NewReader("同样的内容"), "text/plain")
		if err1 != nil || err2 != nil || a != b {
			t.Fatalf("引用应当相同：%s %s %v %v", a, b, err1, err2)
		}
		if f := files(t, dir); len(f) != 2 { // 内容和 .meta.json
			t.Errorf("应当只有一份内容和一份元信息：%v", f)
		}
	})

	t.Run("E08-T4 原子写", func(t *testing.T) {
		dir := t.TempDir()
		s := open(t, dir)
		if _, err := s.Put(ctx, &failingReader{n: 5000}, "text/plain"); err == nil {
			t.Fatal("读到一半出错时应当失败")
		}
		if f := files(t, dir); len(f) != 0 {
			t.Errorf("不应当留下文件：%v", f)
		}
		left := filepath.Join(dir, ".tmp", "put-残留")
		if err := os.WriteFile(left, []byte("半个"), 0o600); err != nil {
			t.Fatal(err)
		}
		open(t, dir)
		if _, err := os.Stat(left); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("New 应当清掉 .tmp/ 里的旧文件：%v", err)
		}
	})

	t.Run("E08-T5 损坏", func(t *testing.T) {
		dir := t.TempDir()
		s := open(t, dir)
		ref, err := s.Put(ctx, strings.NewReader("原来的内容"), "text/plain")
		if err != nil {
			t.Fatal(err)
		}
		h := ref.Hash()
		p := filepath.Join(dir, h[:2], h[2:4], h)
		b, _ := os.ReadFile(p)
		b[0] ^= 1
		if err := os.WriteFile(p, b, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := readAll(t, s, ref); !errors.Is(err, blob.ErrCorrupt) {
			t.Errorf("改过的内容读出时应当 ErrCorrupt：%v", err)
		}
		if _, _, err := s.Get(ctx, blob.RefOf(strings.Repeat("0", 64))); !errors.Is(err, blob.ErrNotFound) {
			t.Errorf("没有的内容应当 ErrNotFound：%v", err)
		}
	})
}
