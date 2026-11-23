//go:build !windows

package datadir

import (
	"os"
	"testing"
)

func TestUnixPerm(t *testing.T) {
	t.Run("B0705-T2 Unix 权限位", func(t *testing.T) {
		d, err := Open(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if err := WritePrivate(d.Token(), []byte("x")); err != nil {
			t.Fatal(err)
		}
		for p, want := range map[string]os.FileMode{d.Root: 0o700, d.Tmp(): 0o700, d.Token(): 0o600} {
			fi, err := os.Stat(p)
			if err != nil {
				t.Fatal(err)
			}
			if got := fi.Mode().Perm(); got != want {
				t.Errorf("%s 的权限是 %o，应当 %o", p, got, want)
			}
		}
	})
}
