//go:build slow

package main

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// 完整构建两次，比较二进制的哈希（设计文档 O.01 §10 验收 3、S.12-R6）。
// 耗时长，只在 go test -tags slow 时运行（P.09 K196）；需要先 pnpm -C web build。
func TestReproducible(t *testing.T) {
	t.Run("O01-T5 同一提交两次构建哈希相同", func(t *testing.T) {
		root, err := filepath.Abs(filepath.Join("..", ".."))
		if err != nil {
			t.Fatal(err)
		}
		if err := checkFrontend(root); err != nil {
			t.Fatal(err)
		}
		var sums [2][32]byte
		for i := range sums {
			out := t.TempDir()
			if err := build(root, options{goos: runtime.GOOS, goarch: runtime.GOARCH, out: out}); err != nil {
				t.Fatal(err)
			}
			name := "anydoor"
			if runtime.GOOS == "windows" {
				name += ".exe"
			}
			b, err := os.ReadFile(filepath.Join(out, name))
			if err != nil {
				t.Fatal(err)
			}
			sums[i] = sha256.Sum256(b)
		}
		if sums[0] != sums[1] {
			t.Fatalf("两次构建的哈希不同：%x / %x", sums[0], sums[1])
		}
	})
}
