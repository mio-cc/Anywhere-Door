package main

import (
	"io"
	"log/slog"

	"github.com/pkg/browser"
)

// openURL 打开系统浏览器（G.10 登记的 github.com/pkg/browser）；测试里替换。
var openURL = browser.OpenURL

// openBrowser 打开带 token 的地址；失败时写一行警告，并把地址打印出来让用户自己打开（B.02 §5"打开浏览器"）。
func openBrowser(u string, stdout io.Writer, log *slog.Logger) {
	if err := openURL(u); err != nil {
		if log != nil {
			log.Warn("没能自动打开浏览器", "err", err)
		}
		say(stdout, "没能自动打开浏览器，请手动打开：%s", u)
	}
}
