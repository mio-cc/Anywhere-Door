package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
		if st, _ := t.State(leaf); st == "冻结" {
			e.say("- %s %s：已冻结 · %s · 领取于 %s", leaf, name, c.ClaimedBy, c.ClaimedAt)
			if ch := changedDocs(t, c); len(ch) > 0 {
				e.say("    冻结后依据文档改过 %d 份：运行 relay stale", len(ch))
			}
			continue
		}
		e.say("- %s %s：%s · 分支 %s · %s · 领取于 %s", leaf, name, stateName(c), c.Branch, c.ClaimedBy, c.ClaimedAt)
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

func stateName(c *Claim) string {
	switch c.State {
	case "claimed":
		return "进行中"
	case "delivered":
		if c.Slice != "" {
			return "片内已完成，等整片验证与合并"
		}
		return "已交付，等合并"
	case "merged":
		return "已合并，等设计方冻结"
	}
	return c.State
}

// sliceClaims 是工作区里属于这一片的领取记录，按领取时间排（G.02.06-R23）。
func (e *Env) sliceClaims(slice string) []*Claim {
	var out []*Claim
	entries, _ := os.ReadDir(filepath.Join(e.Repo, "docs", "relay"))
	for _, d := range entries {
		if leaf, ok := strings.CutSuffix(d.Name(), ".json"); ok {
			if c, err := e.readClaim(leaf); err == nil && c.Slice == slice {
				out = append(out, c)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ClaimedAt != out[j].ClaimedAt {
			return out[i].ClaimedAt < out[j].ClaimedAt
		}
		return out[i].Leaf < out[j].Leaf
	})
	return out
}

// deliverSlice 检查片里还在做的叶子（①–⑥；⑦ 整片验证时跑一次）。全部通过才把它们标为片内已完成、
// 记下完成时的提交，返回改过的领取记录；有不通过的，打印结果、什么也不改。
func (e *Env) deliverSlice(slice string) (paths []string, ok bool, err error) {
	head, err := gitOut(e.Repo, "rev-parse", "HEAD")
	if err != nil {
		return nil, false, err
	}
	var open []*Claim
	ok = true
	for _, c := range e.sliceClaims(slice) {
		if c.State != "claimed" {
			continue
		}
		res, err := e.CheckResults(c.Leaf, true)
		if err != nil {
			return nil, false, err
		}
		if !allOK(res) {
			e.say("== %s ==", c.Leaf)
			e.printResults(withoutBuild(res))
			ok = false
		}
		open = append(open, c)
	}
	if !ok {
		return nil, false, nil
	}
	for _, c := range open {
		c.State, c.Head = "delivered", head
		if err := e.writeClaim(c); err != nil {
			return nil, false, err
		}
		paths = append(paths, claimPath(c.Leaf))
		e.say("%s 检查通过，标为片内已完成", c.Leaf)
	}
	return paths, true, nil
}

// CheckSlice 是整片验证（G.02.06-R23）：片里每个叶子按自己的范围检查 ①–⑥，整片跑一次 ⑦。
// 全部通过时，把还在做的叶子标为片内已完成（要提交它们的领取记录）。
func (e *Env) CheckSlice(slice string, noBuild bool) (bool, error) {
	claims := e.sliceClaims(slice)
	if len(claims) == 0 {
		return false, fmt.Errorf("工作区里没有片 %s 的领取记录：先切到分支 slice/%s", slice, slice)
	}
	if dirty, err := gitOut(e.Repo, "status", "--porcelain"); err != nil {
		return false, err
	} else if dirty != "" {
		return false, errors.New("代码仓库有未提交的改动：整片验证只针对已提交的内容，先提交")
	}
	all := true
	for _, c := range claims {
		res, err := e.CheckResults(c.Leaf, true)
		if err != nil {
			return false, err
		}
		e.say("## %s（%s）", c.Leaf, stateName(c))
		if !e.printResults(withoutBuild(res)) {
			all = false
		}
	}
	e.say("## 整片")
	r7 := Result{Item: 7, Title: "go run ./tools/build -check", OK: true}
	if noBuild {
		r7.Detail = []string{"已跳过（--no-build，只用于本地快速自查；整片验证不允许跳过）"}
	} else if err := e.buildCheck(); err != nil {
		r7.OK, r7.Detail = false, []string{err.Error()}
	}
	if !e.printResults([]Result{r7}) {
		all = false
	}
	if !all {
		e.say("== 片 %s 的检查没有通过 ==", slice)
		return false, e.pErr
	}
	paths, _, err := e.deliverSlice(slice)
	if err != nil {
		return false, err
	}
	e.say("== 片 %s 的检查全部通过 ==", slice)
	if len(paths) > 0 {
		e.say("提交这些领取记录：%s", strings.Join(paths, "、"))
	}
	return true, e.pErr
}

func allOK(res []Result) bool {
	for _, r := range res {
		if !r.OK {
			return false
		}
	}
	return true
}

func withoutBuild(res []Result) []Result {
	var out []Result
	for _, r := range res {
		if r.Item != 7 {
			out = append(out, r)
		}
	}
	return out
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
// accept 不为空时：复查后代码不用改，把这个叶子的依据更新为当前的设计文档（G.02.06-R21）。
func (e *Env) Stale(accept string) error {
	t, err := e.Tree()
	if err != nil {
		return err
	}
	if accept != "" {
		return e.acceptStale(t, accept)
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
	} else {
		e.say("先 relay diff <叶子> 看改了什么。代码要改的，设计方把叶子改回「已审」，开修复分支；不用改的，relay stale --accept <叶子>（G.02.06-R21）")
	}
	return e.pErr
}

func (e *Env) acceptStale(t *Tree, leaf string) error {
	st, err := t.State(leaf)
	if err != nil {
		return err
	}
	if st != "冻结" {
		return fmt.Errorf("%s 的状态是「%s」：stale --accept 只用于冻结的叶子；没冻结的用 start %s --resume --accept-docs", leaf, st, leaf)
	}
	if dirty, err := gitOut(t.Dir, "status", "--porcelain"); err != nil {
		return fmt.Errorf("设计文档目录不是 git 仓库：%w", err)
	} else if dirty != "" {
		return errors.New("设计文档目录有未提交的修改：依据必须是已提交的版本，先在设计仓库提交")
	}
	c, err := e.readClaim(leaf)
	if err != nil {
		return err
	}
	changed := changedDocs(t, c)
	if len(changed) == 0 {
		e.say("%s 的依据文档冻结后没有改过，不用更新", leaf)
		return e.pErr
	}
	docsCommit, err := gitOut(t.Dir, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	_, docs, err := BuildPack(t, leaf, docsCommit, goPackages(e.Repo))
	if err != nil {
		return err
	}
	if err := e.rebase(c, docsCommit, docs); err != nil {
		return err
	}
	e.say("已把 %s 的依据更新为设计仓库 %s（复查过的 %d 份：%s）；在 RELAY.md 记一条复查结论（读了什么、为什么代码不用改），和 %s 一起提交，按合并请求流程合并（G.02.06-R21）", leaf, short(docsCommit), len(changed), strings.Join(changed, "、"), claimPath(leaf))
	return e.pErr
}
