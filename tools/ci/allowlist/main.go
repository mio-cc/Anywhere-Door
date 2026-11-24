// Command allowlist 比较实施分支改动的文件和 PR 说明里的"文件白名单"（设计文档 O.02 §4.3、G.02-R3）：
//
//	go run ./tools/ci/allowlist -branch <分支名> -body <PR 说明文件> -base <提交> -head <提交>
//
// 只对 feat/<编号>-…、fix/<编号>-… 分支检查。白名单是否和任务包一致由人工评审核对（G.08 §3）。只用标准库。
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
)

var (
	branchRe = regexp.MustCompile(`^(feat|fix)/[PGCEBWUSQOXTDR](\.\d{2}){1,4}-`)
	tickRe   = regexp.MustCompile("`([^`]+)`")
)

// alwaysAllowed 不在白名单里也总是允许。
var alwaysAllowed = map[string]bool{"CHANGELOG.md": true}

// List 是从 PR 说明里读出的白名单。
type List struct {
	Create []string // "可新建"行里的：以 / 结尾的是目录前缀
	Modify []string // 只"可修改"的：必须是完整路径
	Found  bool     // 找到了"## 文件白名单"一节
}

// Parse 读 PR 说明里"## 文件白名单"一节（任务包 B 节原样复制）：
// 行里带"可新建"的条目可以是目录前缀；只带"可修改"的必须是完整路径；"总是允许"的条目当作可修改；"不许碰"的不算。
func Parse(body string) List {
	var l List
	in, kind := false, ""
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "## ") {
			in = strings.HasPrefix(t, "## 文件白名单")
			l.Found = l.Found || in
			continue
		}
		if !in {
			continue
		}
		if strings.HasPrefix(line, "- ") { // 顶层条目：决定下面子条目的类别
			switch {
			case strings.Contains(t, "可新建"):
				kind = "create"
			case strings.Contains(t, "可修改"), strings.Contains(t, "总是允许"):
				kind = "modify"
			default:
				kind = ""
			}
		}
		for _, m := range tickRe.FindAllStringSubmatch(t, -1) {
			p := strings.TrimPrefix(strings.TrimSpace(m[1]), "./")
			if p == "" || strings.ContainsAny(p, " {}") {
				continue
			}
			switch kind {
			case "create":
				l.Create = append(l.Create, p)
			case "modify":
				l.Modify = append(l.Modify, p)
			}
		}
	}
	return l
}

func match(file, pattern string, prefixOK bool) bool {
	switch {
	case strings.HasSuffix(pattern, "/"):
		return prefixOK && strings.HasPrefix(file, pattern)
	case strings.Contains(pattern, "*"):
		ok, _ := path.Match(pattern, file)
		return ok
	}
	return file == pattern
}

// Check 返回越界的文件；branch 不是实施分支时不检查。
func Check(branch string, l List, changed []string) (bad []string, err error) {
	if !branchRe.MatchString(branch) {
		return nil, nil
	}
	if !l.Found {
		return nil, fmt.Errorf("实施分支 %s 的 PR 说明里没有\"## 文件白名单\"一节（从任务包 B 节原样复制，G.02.01-R5）", branch)
	}
	for _, f := range changed {
		if alwaysAllowed[f] || matchAny(f, l.Create, true) || matchAny(f, l.Modify, false) {
			continue
		}
		bad = append(bad, f)
	}
	return bad, nil
}

func matchAny(f string, ps []string, prefixOK bool) bool {
	for _, p := range ps {
		if match(f, p, prefixOK) {
			return true
		}
	}
	return false
}

func changedFiles(base, head string) ([]string, error) {
	out, err := exec.Command("git", "-c", "core.quotepath=false", "diff", "--name-only", base+"..."+head).Output()
	if err != nil {
		return nil, fmt.Errorf("git diff %s...%s：%w", base, head, err)
	}
	var fs []string
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if l != "" {
			fs = append(fs, l)
		}
	}
	return fs, nil
}

func main() {
	branch := flag.String("branch", "", "PR 的分支名")
	bodyFile := flag.String("body", "", "PR 说明（写在文件里）")
	base := flag.String("base", "", "PR 的目标提交")
	head := flag.String("head", "", "PR 的最新提交")
	flag.Parse()
	if !branchRe.MatchString(*branch) {
		fmt.Printf("allowlist：%s 不是实施分支（feat/<编号>-…、fix/<编号>-…），不检查\n", *branch)
		return
	}
	body, err := os.ReadFile(*bodyFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误：读不了 PR 说明：", err)
		os.Exit(1)
	}
	changed, err := changedFiles(*base, *head)
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误：", err)
		os.Exit(1)
	}
	bad, err := Check(*branch, Parse(string(body)), changed)
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误：", err)
		os.Exit(1)
	}
	for _, f := range bad {
		fmt.Printf("%s：不在文件白名单里（G.02-R3）\n", f)
	}
	if len(bad) > 0 {
		os.Exit(1)
	}
	fmt.Printf("allowlist：%d 个改动的文件都在白名单里\n", len(changed))
}
