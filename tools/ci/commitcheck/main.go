// Command commitcheck 检查 PR 标题和每条提交信息（设计文档 O.02 §4.3、G.07-R3、R4）：
//
//	go run ./tools/ci/commitcheck -title <PR 标题> -base <提交> -head <提交>
//
// 只在合并请求上运行（M0 不走合并请求，P.09 K241）。只用标准库。
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// Problem 是一处不合规范。
type Problem struct{ Where, Msg string }

func (p Problem) String() string { return p.Where + "：" + p.Msg }

// types 是 G.07-R3 的 9 种类型。
var types = map[string]bool{
	"feat": true, "fix": true, "refactor": true, "perf": true, "test": true,
	"docs": true, "build": true, "ci": true, "chore": true,
}

var (
	headRe  = regexp.MustCompile(`^([a-z]+)\(([^()]+)\): (\S.*)$`)
	treeRe  = regexp.MustCompile(`^[PGCEBWUSQOXTDR](\.\d{2}){1,4}$`)
	refsRe  = regexp.MustCompile(`^Refs: (.+)$`)
	scopeOK = map[string]bool{"deps": true, "ci": true, "repo": true}
)

// Commit 是一条提交：首行和正文。
type Commit struct{ ID, Subject, Body string }

// Check 检查 PR 标题和提交；返回全部问题。
func Check(title string, commits []Commit) []Problem {
	var probs []Problem
	if title != "" {
		for _, m := range checkHead(title) {
			probs = append(probs, Problem{"PR 标题", m})
		}
	}
	for _, c := range commits {
		where := "提交 " + short(c.ID)
		for _, m := range checkHead(c.Subject) {
			probs = append(probs, Problem{where, m})
		}
		if t := typeOf(c.Subject); t == "feat" || t == "fix" {
			if m := checkRefs(c.Body); m != "" {
				probs = append(probs, Problem{where, m})
			}
		}
	}
	return probs
}

func checkHead(s string) []string {
	m := headRe.FindStringSubmatch(s)
	if m == nil {
		return []string{fmt.Sprintf("首行 %q 不是 <类型>(<范围>): <描述>", s)}
	}
	var out []string
	if !types[m[1]] {
		out = append(out, fmt.Sprintf("类型 %q 不在 G.07-R3 的 9 种里", m[1]))
	}
	if !scopeOK[m[2]] && !treeRe.MatchString(m[2]) {
		out = append(out, fmt.Sprintf("范围 %q 不是树里的编号，也不是 deps、ci、repo", m[2]))
	}
	return out
}

func typeOf(subject string) string {
	if m := headRe.FindStringSubmatch(subject); m != nil {
		return m[1]
	}
	return ""
}

// checkRefs：正文最后一个非空行是 Refs: <编号>（可以用"，"或", "分隔多个；上报单写 反馈/<文件名>）。
func checkRefs(body string) string {
	lines := strings.Split(strings.TrimRight(strings.ReplaceAll(body, "\r\n", "\n"), "\n"), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	m := refsRe.FindStringSubmatch(last)
	if m == nil {
		return "feat、fix 的正文最后一行要写 Refs: <编号>（G.07-R4）"
	}
	for _, id := range strings.FieldsFunc(m[1], func(r rune) bool { return r == ',' || r == '，' || r == ' ' }) {
		if !treeRe.MatchString(id) && !strings.HasPrefix(id, "反馈/") {
			return fmt.Sprintf("Refs 里的 %q 不是树里的编号", id)
		}
	}
	return ""
}

func short(id string) string {
	if len(id) > 7 {
		return id[:7]
	}
	return id
}

// commitsBetween 读 base..head 的提交（不含合并提交）。
func commitsBetween(base, head string) ([]Commit, error) {
	out, err := exec.Command("git", "log", "--no-merges", "--format=%H%x00%s%x00%b%x1e", base+".."+head).Output()
	if err != nil {
		return nil, fmt.Errorf("git log %s..%s：%w", base, head, err)
	}
	var cs []Commit
	for _, rec := range bytes.Split(out, []byte{0x1e}) {
		parts := strings.SplitN(strings.TrimLeft(string(rec), "\n"), "\x00", 3)
		if len(parts) == 3 {
			cs = append(cs, Commit{ID: parts[0], Subject: parts[1], Body: parts[2]})
		}
	}
	return cs, nil
}

func main() {
	title := flag.String("title", "", "PR 标题")
	base := flag.String("base", "", "PR 的目标提交")
	head := flag.String("head", "", "PR 的最新提交")
	flag.Parse()
	var commits []Commit
	if *base != "" && *head != "" {
		var err error
		if commits, err = commitsBetween(*base, *head); err != nil {
			fmt.Fprintln(os.Stderr, "错误：", err)
			os.Exit(1)
		}
	}
	probs := Check(*title, commits)
	for _, p := range probs {
		fmt.Println(p)
	}
	if len(probs) > 0 {
		fmt.Printf("commitcheck：%d 处不符合 G.07-R3、R4\n", len(probs))
		os.Exit(1)
	}
	fmt.Printf("commitcheck：PR 标题和 %d 条提交都符合规范\n", len(commits))
}
