//go:build windows

package datadir

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsDACL(t *testing.T) {
	t.Run("B0705-T2 Windows DACL", func(t *testing.T) {
		d, err := Open(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if err := WritePrivate(d.Token(), []byte("x")); err != nil {
			t.Fatal(err)
		}
		for _, p := range []string{d.Root, d.Token()} {
			sd, err := windows.GetNamedSecurityInfo(p, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
			if err != nil {
				t.Fatal(err)
			}
			ctrl, _, err := sd.Control()
			if err != nil {
				t.Fatal(err)
			}
			if ctrl&windows.SE_DACL_PROTECTED == 0 {
				t.Errorf("%s 的 DACL 应当受保护（不继承上级）", p)
			}
			dacl, _, err := sd.DACL()
			if err != nil || dacl == nil {
				t.Fatalf("%s 读不到 DACL：%v", p, err)
			}
			if dacl.AceCount != 1 {
				t.Errorf("%s 的 DACL 有 %d 条，应当只有 1 条", p, dacl.AceCount)
			}
		}
	})
}
