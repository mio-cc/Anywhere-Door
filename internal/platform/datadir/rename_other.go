//go:build !windows

package datadir

import (
	"os"
	"time"
)

// renameReplace 改名覆盖。Unix 上改名不受目标已打开的句柄影响，不需要重试。
func renameReplace(from, to string, _ time.Duration) error { return os.Rename(from, to) }
