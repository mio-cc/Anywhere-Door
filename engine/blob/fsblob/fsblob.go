// Package fsblob 是 blob.Store 的文件系统实现（E.08 §5）：<dir>/<前 2 位>/<3–4 位>/<hash>，
// 媒体类型和大小写在旁边的 <hash>.meta.json；先写 <dir>/.tmp/ 再改名，写到一半出错不留下半个文件。
package fsblob

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/mio-cc/anywhere-door/engine/blob"
)

type store struct{ dir, tmp string }

// New 打开 dir（不存在就建），并清掉 .tmp/ 里上次中途崩溃留下的临时文件。
func New(dir string) (blob.Store, error) {
	tmp := filepath.Join(dir, ".tmp")
	if err := os.MkdirAll(tmp, 0o700); err != nil {
		return nil, fmt.Errorf("fsblob: 建目录失败：%w", err)
	}
	entries, err := os.ReadDir(tmp)
	if err != nil {
		return nil, fmt.Errorf("fsblob: 读临时目录失败：%w", err)
	}
	for _, e := range entries {
		_ = os.RemoveAll(filepath.Join(tmp, e.Name()))
	}
	return &store{dir: dir, tmp: tmp}, nil
}

func (s *store) path(h string) string { return filepath.Join(s.dir, h[:2], h[2:4], h) }

type metaFile struct {
	Size      int64  `json:"size"`
	MediaType string `json:"media_type"`
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// writeTemp 把 r 写进 .tmp/ 下的一个临时文件；出错时删掉它。
func (s *store) writeTemp(r io.Reader, h hash.Hash) (name string, n int64, err error) {
	f, err := os.CreateTemp(s.tmp, "put-*")
	if err != nil {
		return "", 0, err
	}
	name = f.Name()
	w := io.Writer(f)
	if h != nil {
		w = io.MultiWriter(f, h)
	}
	n, err = io.Copy(w, r)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(name)
		return "", 0, err
	}
	return name, n, nil
}

func (s *store) Put(ctx context.Context, r io.Reader, mediaType string) (blob.Ref, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	h := sha256.New()
	tmp, n, err := s.writeTemp(r, h)
	if err != nil {
		return "", fmt.Errorf("fsblob: 写入失败：%w", err)
	}
	sum := hex.EncodeToString(h.Sum(nil))
	ref := blob.RefOf(sum)
	dst := s.path(sum)
	if exists(dst) {
		_ = os.Remove(tmp)
		return ref, nil // 去重
	}
	fail := func(err error) (blob.Ref, error) {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("fsblob: 写入失败：%w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return fail(err)
	}
	mb, err := json.Marshal(metaFile{Size: n, MediaType: mediaType})
	if err != nil {
		return fail(err)
	}
	mtmp, _, err := s.writeTemp(bytes.NewReader(mb), nil)
	if err != nil {
		return fail(err)
	}
	if err := os.Rename(mtmp, dst+".meta.json"); err != nil {
		_ = os.Remove(mtmp)
		return fail(err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		if exists(dst) { // 另一个写入者先到了
			_ = os.Remove(tmp)
			return ref, nil
		}
		return fail(err)
	}
	return ref, nil
}

func (s *store) open(ref blob.Ref) (string, error) {
	if _, err := blob.ParseRef(string(ref)); err != nil {
		return "", err
	}
	p := s.path(ref.Hash())
	if !exists(p) {
		return "", fmt.Errorf("%w：%s", blob.ErrNotFound, ref)
	}
	return p, nil
}

func (s *store) Stat(ctx context.Context, ref blob.Ref) (blob.Meta, error) {
	if err := ctx.Err(); err != nil {
		return blob.Meta{}, err
	}
	p, err := s.open(ref)
	if err != nil {
		return blob.Meta{}, err
	}
	fi, err := os.Stat(p)
	if err != nil {
		return blob.Meta{}, err
	}
	m := blob.Meta{Size: fi.Size()}
	if b, err := os.ReadFile(p + ".meta.json"); err == nil {
		var mf metaFile
		if json.Unmarshal(b, &mf) == nil {
			m.MediaType = mf.MediaType
		}
	}
	return m, nil
}

func (s *store) Get(ctx context.Context, ref blob.Ref) (io.ReadCloser, blob.Meta, error) {
	m, err := s.Stat(ctx, ref)
	if err != nil {
		return nil, blob.Meta{}, err
	}
	f, err := os.Open(s.path(ref.Hash()))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, blob.Meta{}, fmt.Errorf("%w：%s", blob.ErrNotFound, ref)
	}
	if err != nil {
		return nil, blob.Meta{}, err
	}
	return &verifier{f: f, h: sha256.New(), want: ref.Hash()}, m, nil
}

// verifier 边读边算 sha256，读到结尾时和引用里的不一致就返回 ErrCorrupt。
type verifier struct {
	f    *os.File
	h    hash.Hash
	want string
}

func (v *verifier) Read(p []byte) (int, error) {
	n, err := v.f.Read(p)
	_, _ = v.h.Write(p[:n])
	if errors.Is(err, io.EOF) && hex.EncodeToString(v.h.Sum(nil)) != v.want {
		return n, fmt.Errorf("%w：ws://sha256:%s", blob.ErrCorrupt, v.want)
	}
	return n, err
}

func (v *verifier) Close() error { return v.f.Close() }
