//go:build windows

package datadir

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// renameReplace 改名覆盖。Go 在 Windows 上打开文件不带 FILE_SHARE_DELETE，目标这时正被别的句柄打开
// （例如 config.Watch 在轮询读它）时 MoveFileEx 返回 ERROR_ACCESS_DENIED 或 ERROR_SHARING_VIOLATION；
// 遇到这两种错误就退避重试（1ms 起、每次翻倍、最长 100ms 一次，nextWait），等待不越过 retryFor（P.09 K244）。
func renameReplace(from, to string, retryFor time.Duration) error {
	deadline := time.Now().Add(retryFor)
	wait := time.Millisecond
	for {
		err := os.Rename(from, to)
		if err == nil || !inUse(err) {
			return err
		}
		left := time.Until(deadline)
		if left <= 0 {
			return err
		}
		time.Sleep(min(wait, left))
		wait = nextWait(wait)
	}
}

func inUse(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}
