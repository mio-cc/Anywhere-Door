package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Status 列出接力状态（G.02.06-R22）：已领取的叶子、测试用例覆盖、桩、没处理完的评审意见。
func (e *Env) Status() error {
	t, err := e.Tree()
	if err != nil {
		return err
	}
	branch, _ := gitOut(e.Repo, "rev-parse", "--abbrev-ref", "HEAD")
	dirty, _ := gitOut(e.Repo, "status", "--porcelain")
	state := "干净"
	if dirty != "" {
		state = "有 " + strconv.Itoa(len(lines(dirty))) + " 个未提交的改动"
	}
	e.say("当前分支：%s（工作区%s）", branch, state)
	e.say("设计文档：%s", e.Docs)
	e.say("")

	claims, err := e.allClaims()
	if err != nil {
		return err
	}
	e.say("## 已领取的叶子")
	if len(claims) == 0 {
		e.say("（没有）")
	}
	for _, leaf := range sortedKeys(claims) {
		c := claims[leaf]
		name := ""
		if n, ok := t.Nodes[leaf]; ok {
			name = n.Name
		}
		e.say("- %s %s：%s · 分支 %s · %s · 领取于 %s", leaf, name, stateName(c.State), c.Branch, c.ClaimedBy, c.ClaimedAt)
		if last, err := gitOut(e.Repo, "log", "-1", "--format=%h %s（%cr）", c.Branch); err == nil {
			e.say("    最后一次提交：%s", last)
		} else {
			e.say("    分支不在本地（记录在 %s 上找到）", c.foundOn)
		}
		if c.Branch == branch {
			if text, err := t.Text(leaf); err == nil {
				card, _ := ParseCard(text)
				found := e.foundTestIDs(card.Tests)
				var missing []string
				for _, id := range card.Tests {
					if !found[id] {
						missing = append(missing, id)
					}
				}
				line := "测试用例：" + strconv.Itoa(len(card.Tests)-len(missing)) + "/" + strconv.Itoa(len(card.Tests))
				if len(missing) > 0 {
					line += "，还缺 " + strings.Join(missing, "、")
				}
				e.say("    %s", line)
			}
			if ch := changedDocs(t, c); len(ch) > 0 {
				e.say("    依据文档领取后改过 %d 份：运行 relay diff %s", len(ch), leaf)
			}
		} else {
			e.say("    测试用例覆盖：切到分支 %s 后查看", c.Branch)
		}
	}
	e.say("")

	e.say("## 桩（G.02.06-R10）")
	stubs := e.scanStubs()
	if len(stubs) == 0 {
		e.say("（没有）")
	}
	for _, s := range stubs {
		e.say("- %s", s)
	}
	e.say("")

	e.say("## 没处理完的评审意见（G.02.06-R18）")
	open := e.openReviews()
	if len(open) == 0 {
		e.say("（没有）")
	}
	for _, s := range open {
		e.say("- %s", s)
	}
	return e.pErr
}

func stateName(s string) string {
	switch s {
	case "claimed":
		return "进行中"
	case "delivered":
		return "已交付，等合并"
	case "merged":
		return "已合并，等设计方冻结"
	}
	return s
}

// scanStubs 找第一、二行写着 STUB(…) 的文件。
func (e *Env) scanStubs() []string {
	var out []string
	_ = filepath.WalkDir(e.Repo, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != e.Repo && skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			return nil
		}
		defer func() { _ = f.Close() }()
		sc := bufio.NewScanner(f)
		for i := 0; i < 2 && sc.Scan(); i++ {
			line := sc.Text()
			if j := strings.Index(line, "STUB("); j >= 0 {
				rel, _ := filepath.Rel(e.Repo, p)
				out = append(out, filepath.ToSlash(rel)+"："+strings.TrimSpace(line[j:]))
				break
			}
		}
		return nil
	})
	return out
}

// openReviews 列出评审记录里还没处理完的条目（G.02.06-R18）。评审记录是一张表，表头里有"结论"一列；
// 结论一格以"不符合"或"有疑问"开头、而且这一行没有写"已处理"的，算没处理完。表头、说明文字不算。
func (e *Env) openReviews() []string {
	var out []string
	files, _ := filepath.Glob(filepath.Join(e.Repo, "docs", "relay", "*.review.md"))
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, row := range openReviewRows(string(b)) {
			out = append(out, filepath.Base(f)+"："+row)
		}
	}
	return out
}

func openReviewRows(text string) []string {
	var out []string
	col := -1
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			col = -1 // 表格结束
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		switch {
		case col < 0:
			for i, c := range cells {
				if c == "结论" {
					col = i
				}
			}
		case col >= len(cells) || strings.HasPrefix(cells[col], "---"):
		case (strings.HasPrefix(cells[col], "不符合") || strings.HasPrefix(cells[col], "有疑问")) && !strings.Contains(line, "已处理"):
			out = append(out, strings.Join(cells[:min(col+1, len(cells))], " | "))
		}
	}
	return out
}

// Next 按树的顺序列出状态为"已审"、还没有被领取的实施叶子。
func (e *Env) Next() error {
	t, err := e.Tree()
	if err != nil {
		return err
	}
	claims, err := e.allClaims()
	if err != nil {
		return err
	}
	count := 0
	for _, id := range t.Order {
		n := t.Nodes[id]
		if !n.IsImplLeaf() {
			continue
		}
		if _, taken := claims[id]; taken {
			continue
		}
		if st, err := t.State(id); err == nil && st == "已审" {
			e.say("%s %s", id, n.Name)
			count++
		}
	}
	if count == 0 {
		e.say("没有可以领取的叶子：需要设计方先把叶子改为「已审」（G.02.01-R2）")
	}
	return e.pErr
}

// Stale 列出状态为"冻结"、依据文档又被改过的叶子（G.02.06-R21）：它们的代码需要复查。
func (e *Env) Stale() error {
	t, err := e.Tree()
	if err != nil {
		return err
	}
	claims, err := e.allClaims()
	if err != nil {
		return err
	}
	count := 0
	for _, leaf := range sortedKeys(claims) {
		st, err := t.State(leaf)
		if err != nil || st != "冻结" {
			continue
		}
		if ch := changedDocs(t, claims[leaf]); len(ch) > 0 {
			count++
			e.say("%s：冻结后依据文档改过，代码需要复查", leaf)
			for _, p := range ch {
				e.say("    %s", p)
			}
		}
	}
	if count == 0 {
		e.say("没有需要复查的冻结叶子")
	}
	return e.pErr
}
