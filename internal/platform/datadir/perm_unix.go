//go:build !windows

package datadir

import (
	"fmt"
	"os"
)

// Restrict 把 path 收紧到只有当前用户能访问：目录 0700，文件 0600。
func Restrict(p string) error {
	fi, err := os.Stat(p)
	if err != nil {
		return fmt.Errorf("datadir: 收紧权限 %s：%w", p, err)
	}
	mode := os.FileMode(0o600)
	if fi.IsDir() {
		mode = 0o700
	}
	if err := os.Chmod(p, mode); err != nil {
		return fmt.Errorf("datadir: 收紧权限 %s：%w", p, err)
	}
	return nil
}

// IsPrivate 判断 path 是否只有当前用户能访问（同组和其他用户没有任何权限）。
func IsPrivate(p string) (bool, error) {
	fi, err := os.Stat(p)
	if err != nil {
		return false, err
	}
	return fi.Mode().Perm()&0o077 == 0, nil
}
