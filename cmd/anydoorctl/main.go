// STUB(B.08): 命令行工具的桩，只做 version。B.08 实施时整体替换（G.02.06-R10）。

package main

import (
	"fmt"
	"os"
)

// 由 tools/build 用 -ldflags -X 注入（O.01 §4.2）。
var (
	version = "0.0.0-dev"
	commit  = "unknown"
	date    = "1970-01-01T00:00:00Z"
)

func main() {
	restore := utf8Console()
	code := run(os.Args[1:])
	restore()
	os.Exit(code)
}

func run(args []string) int {
	if len(args) >= 1 && (args[0] == "version" || args[0] == "--version") {
		fmt.Printf("anydoorctl %s\n提交：%s\n构建时间：%s\n", version, commit, date)
		return 0
	}
	fmt.Fprintln(os.Stderr, "anydoorctl：这个命令还没有实现（桩，B.08）；现在只有 anydoorctl version")
	return 2
}
