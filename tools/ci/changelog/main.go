// Command changelog 检查需要写变更日志的 PR 改了 CHANGELOG.md（设计文档 O.02 §4.3、G.09-R4）：
//
//	go run ./tools/ci/changelog -labels <逗号分隔的标签> -base <提交> -head <提交>
//
// 改了 cmd/、engine/、internal/、web/src/ 下的非测试文件，就要在 CHANGELOG.md 的"未发布"下加一行；
// PR 带 no-changelog 标签时跳过。只用标准库。
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var userFacing = []string{"cmd/", "engine/", "internal/", "web/src/"}

// isTest：测试文件和测试数据不算对用户可见的改动。
func isTest(f string) bool {
	return strings.HasSuffix(f, "_test.go") || strings.HasSuffix(f, ".test.ts") || strings.HasSuffix(f, ".test.tsx") ||
		strings.HasSuffix(f, ".spec.ts") || strings.Contains(f, "/testdata/") || strings.HasPrefix(f, "testdata/")
}

// Check：需要变更日志却没改时，返回触发的文件（第一个）；否则返回空。
func Check(changed, labels []string) string {
	for _, l := range labels {
		if strings.TrimSpace(l) == "no-changelog" {
			return ""
		}
	}
	trigger := ""
	for _, f := range changed {
		if f == "CHANGELOG.md" {
			return ""
		}
		if trigger == "" && !isTest(f) {
			for _, p := range userFacing {
				if strings.HasPrefix(f, p) {
					trigger = f
				}
			}
		}
	}
	return trigger
}

func main() {
	labels := flag.String("labels", "", "PR 的标签，逗号分隔")
	base := flag.String("base", "", "PR 的目标提交")
	head := flag.String("head", "", "PR 的最新提交")
	flag.Parse()
	out, err := exec.Command("git", "-c", "core.quotepath=false", "diff", "--name-only", *base+"..."+*head).Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误：git diff：", err)
		os.Exit(1)
	}
	var changed []string
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if l != "" {
			changed = append(changed, l)
		}
	}
	if f := Check(changed, strings.Split(*labels, ",")); f != "" {
		fmt.Printf("changelog：改了 %s 等对用户可见的代码，请在 CHANGELOG.md 的\"未发布\"下加一行（G.09-R4）；只改内部实现或测试时给 PR 加 no-changelog 标签\n", f)
		os.Exit(1)
	}
	fmt.Println("changelog：通过")
}
