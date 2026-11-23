// Command schemacheck 检查 schema/ 下的契约文件和示例（设计文档 C.01 §6）：
//
//	go run ./tools/schemacheck [仓库根目录]
//
// 检查元校验、写法（C.01-R1、R2、R3、R21）、命名（R4、R7–R13）、封闭对象（R19）、示例清单（R22、R23）。
// 逐条列出问题；退出码：0 通过，1 有问题，2 用法错误。同样的检查也在 go test 里跑。
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout)) }

func run(args []string, out io.Writer) int {
	say := func(format string, a ...any) { _, _ = fmt.Fprintf(out, format+"\n", a...) }
	root := "."
	switch len(args) {
	case 0:
	case 1:
		root = args[0]
	default:
		say("用法：go run ./tools/schemacheck [仓库根目录]")
		return 2
	}
	if st, err := os.Stat(filepath.Join(root, schemaDir)); err != nil || !st.IsDir() {
		say("错误：%s 下没有 schema/ 目录", root)
		return 2
	}
	probs := Check(root)
	for _, p := range probs {
		say("%s", p)
	}
	if len(probs) > 0 {
		say("== schema 检查没有通过：%d 个问题 ==", len(probs))
		return 1
	}
	s, _ := Load(root)
	entries, _ := ReadManifest(root)
	say("== schema 检查通过：%d 个 schema 文件，%d 个示例 ==", len(s.Files), len(entries))
	return 0
}
