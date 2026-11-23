package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// alwaysRead 是每个任务包都要附的守则（G.02-§1、G.02.02-R1、G.02.06）。
var alwaysRead = []string{"G.02", "G.02.05", "G.02.06", "Q.01"}

// extraByBranch 是 G.02.02 §3"按任务类型的附加集合"。
var extraByBranch = map[string][]string{
	"E": {"E.01", "G.05", "Q.02"},
	"B": {"B.01", "B.03", "G.05", "G.13"},
	"W": {"W.01", "G.06", "U.02"},
	"O": {"G.11", "O.01"},
}

// neverAttach 是 G.02.02 §4"不应该附的"。P.09 是裁决记录：每条裁决都已写进对应的文档，
// 附上它只会让每一棒的依据都因为新增一条裁决而失效（P.09 K199）。
func neverAttach(id string) bool {
	return id == "P.03" || id == "P.09" || id == "P.10" || strings.HasPrefix(id, "R")
}

// PackDoc 是任务包里附上的一份文档。
type PackDoc struct {
	ID, Role, Path, Hash string
	Text                 string
	Missing              bool
}

// BuildPack 按 G.02.01 §3、G.02.02 拼出任务包（G.02.06-R5），返回正文和附上的文档。
func BuildPack(t *Tree, leaf, docsCommit string, existingPkgs []string) (string, []PackDoc, error) {
	n, err := t.Node(leaf)
	if err != nil {
		return "", nil, err
	}
	leafText, err := t.Text(leaf)
	if err != nil {
		return "", nil, fmt.Errorf("读不到 %s：%w", leaf, err)
	}
	card, ok := ParseCard(leafText)
	if !ok {
		return "", nil, fmt.Errorf("%s 没有实施卡", leaf)
	}

	var docs []PackDoc
	seen := map[string]bool{}
	add := func(id, role string, stripCard bool) {
		if seen[id] {
			return
		}
		node, ok := t.Nodes[id]
		if !ok {
			return
		}
		seen[id] = true
		text, err := t.Text(id)
		if err != nil {
			docs = append(docs, PackDoc{ID: id, Role: role, Path: node.Path, Missing: true})
			return
		}
		d := PackDoc{ID: id, Role: role, Path: node.Path, Hash: NormHash(text), Text: text}
		if stripCard {
			d.Text = withoutCard(text)
		}
		docs = append(docs, d)
	}
	for _, id := range alwaysRead {
		add(id, "守则", false)
	}
	for _, a := range t.Ancestors(leaf) {
		add(a.ID, "祖先", false)
	}
	add(leaf, "本叶子", false)
	branch := leaf[:1]
	extras := append([]string{}, extraByBranch[branch]...)
	if strings.HasPrefix(leaf, "W.06.") {
		extras = append(extras, "U.10", "U.17")
	}
	for _, id := range extras {
		add(id, "按任务类型附加（G.02.02 §3）", false)
	}
	for _, id := range RefCodes(leafText) {
		if id == leaf || neverAttach(id) {
			continue
		}
		if node, ok := t.Nodes[id]; ok {
			add(id, "本叶子引用", node.IsImplLeaf())
		}
	}

	var b strings.Builder
	p := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }
	p("# 任务包 · %s %s", leaf, n.Name)
	p("")
	p("> 由 `go run ./tools/relay start %s` 生成（G.02.06-R5）。**只读这份任务包**；缺了必须读的东西说明文档漏了引用，按 G.02.03 上报，不要自己去设计树里补读。", leaf)
	p("> 设计仓库提交：%s", docsCommit)
	p("")
	p("## A. 必读（按顺序）")
	for i, d := range docs {
		mark := ""
		if d.Missing {
			mark = "（还没有写，读不到）"
		}
		p("%d. %s %s · %s%s", i+1, d.ID, t.Nodes[d.ID].Name, d.Role, mark)
	}
	p("")
	p("引用的其他实施叶子只附正文，不附它们的实施卡（G.02.02-R3）。")
	p("")
	p("## B. 文件白名单")
	p("- 可新建 / 可修改（实施卡\"要建 / 要改的文件\"）：")
	for _, f := range card.Create {
		p("  - `%s`", f)
	}
	p("- 不许碰（实施卡）：")
	for _, f := range card.Forbid {
		p("  - `%s`", f)
	}
	p("- 总是允许：`RELAY.md`、`CHANGELOG.md`、`docs/relay/%s.json`、`docs/relay/%s.review.md`，以及登记在 `docs/relay/%s.json` 的 `stubs` 里的桩（G.02.06-R10）。", leaf, leaf, leaf)
	p("- 以 `/` 结尾的是目录：只允许在里面**新建**文件（G.02.01-R5）。")
	p("")
	p("## C. 现有代码上下文")
	p("只看已有包的导出接口，不看实现（G.02.02-R4）。仓库里现有的 Go 包：")
	if len(existingPkgs) == 0 {
		p("- （还没有）")
	}
	for _, pkg := range existingPkgs {
		p("- `%s`", pkg)
	}
	p("")
	p("## D. 验收")
	p("- 实施卡里的测试用例（测试名必须以编号开头，G.02.06-R14 ⑤）：%s", strings.Join(card.Tests, "、"))
	p("- 必须通过：`go run ./tools/relay check %s`（七项，G.02.06-R14）", leaf)
	p("")
	p("## E. 交付")
	p("- 按 G.02 §6 写交付说明；按 G.02.06-R20 在 `RELAY.md` 最上面写完本棒记录。")
	p("- 由另一方验证与评审（G.02.06-R17）；评审记录写在 `docs/relay/%s.review.md`。", leaf)
	p("")
	p("## F. 依据")
	p("- 设计仓库提交：%s", docsCommit)
	p("- 每份文档的指纹：见 `docs/relay/%s.json` 的 `docs`（去掉文件头状态行和变更记录后算 SHA-256）。", leaf)
	p("")
	p("---")
	p("")
	p("# 附：文档原文")
	for _, d := range docs {
		if d.Missing {
			continue
		}
		p("")
		p("<!-- ======== %s · %s · %s ======== -->", d.ID, d.Role, d.Path)
		p("")
		b.WriteString(d.Text)
		if !strings.HasSuffix(d.Text, "\n") {
			b.WriteString("\n")
		}
	}
	return b.String(), docs, nil
}

// withoutCard 去掉"### 实施卡"一节（到下一个二级标题为止）。
func withoutCard(text string) string {
	i := strings.Index(text, "### 实施卡")
	if i < 0 {
		return text
	}
	rest := text[i:]
	if j := strings.Index(rest, "\n## "); j >= 0 {
		return text[:i] + "（实施卡已省略，G.02.02-R3）\n" + rest[j:]
	}
	return text[:i] + "（实施卡已省略，G.02.02-R3）\n"
}

// goPackages 列出仓库里的 Go 包目录（任务包 C 节）。
func goPackages(repo string) []string {
	set := map[string]bool{}
	_ = filepath.WalkDir(repo, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDir(d.Name()) && p != repo {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			rel, err := filepath.Rel(repo, filepath.Dir(p))
			if err == nil {
				set[filepath.ToSlash(rel)] = true
			}
		}
		return nil
	})
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func skipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "testdata", "dist", "bin", ".relay", ".anydoor-dev", ".perf", ".test", "dist-perf":
		return true
	}
	return false
}

// Start 领取一个叶子（G.02.06-R4–R7）；resume 时只重新生成任务包。
func (e *Env) Start(leaf, name string, resume bool) error {
	t, err := e.Tree()
	if err != nil {
		return err
	}
	n, err := t.Node(leaf)
	if err != nil {
		return err
	}
	if !n.IsImplLeaf() {
		return fmt.Errorf("%s 不是实施类叶子，不能领取（G.02.01-R1）", leaf)
	}
	state, err := t.State(leaf)
	if err != nil {
		return err
	}
	if state != "已审" {
		return fmt.Errorf("%s 的状态是「%s」，只有「已审」的叶子能领取（G.02.01-R2）", leaf, state)
	}
	if dirty, err := gitOut(t.Dir, "status", "--porcelain"); err != nil {
		return fmt.Errorf("设计文档目录不是 git 仓库：%w", err)
	} else if dirty != "" {
		return errors.New("设计文档目录有未提交的修改：依据必须是已提交的版本，先在设计仓库提交")
	}
	docsCommit, err := gitOut(t.Dir, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if !resume {
		if err := e.checkClaimable(t, leaf); err != nil {
			return err
		}
		if dirty, err := gitOut(e.Repo, "status", "--porcelain"); err != nil {
			return err
		} else if dirty != "" {
			return errors.New("代码仓库有未提交的改动：先提交或 git stash，再领取")
		}
	}
	text, docs, err := BuildPack(t, leaf, docsCommit, goPackages(e.Repo))
	if err != nil {
		return err
	}
	packPath := filepath.Join(e.Repo, ".relay", "tasks", leaf+".md")
	if err := os.MkdirAll(filepath.Dir(packPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(packPath, []byte(text), 0o644); err != nil {
		return err
	}
	if resume {
		c, err := e.readClaim(leaf)
		if err != nil {
			return err
		}
		changed := changedDocs(t, c)
		switch {
		case len(changed) > 0 && e.AcceptDocs:
			c.DocsCommit, c.Docs = docsCommit, map[string]string{}
			for _, d := range docs {
				if !d.Missing {
					c.Docs[d.Path] = d.Hash
				}
			}
			if err := e.writeClaim(c); err != nil {
				return err
			}
			e.say("已把依据更新为设计仓库 %s（%d 份文档改过：%s）；把读过的改动和处理方式记进 RELAY.md 本棒记录，再提交 %s", short(docsCommit), len(changed), strings.Join(changed, "、"), claimPath(leaf))
		case len(changed) > 0:
			e.say("注意：领取之后这些文档改过，先运行 relay diff %s 看改了什么；确认代码怎么处理后，用 --resume --accept-docs 更新依据（G.02.06-R13）：%s", leaf, strings.Join(changed, "、"))
		}
		e.say("已重新生成任务包：%s", filepath.ToSlash(packPath))
		return e.pErr
	}

	branch := "feat/" + leaf + "-" + name
	base := "main"
	if _, err := gitOut(e.Repo, "rev-parse", "--verify", "--quiet", "main"); err != nil {
		base = "HEAD"
	}
	if _, err := gitOut(e.Repo, "switch", "-c", branch, base); err != nil {
		return err
	}
	by := e.By
	if by == "" {
		by = "未注明"
	}
	c := &Claim{
		Leaf: leaf, Branch: branch, ClaimedBy: by, ClaimedAt: e.Now().UTC().Format("2006-01-02T15:04:05Z"),
		DocsCommit: docsCommit, Docs: map[string]string{}, Stubs: []string{}, State: "claimed",
	}
	for _, d := range docs {
		if !d.Missing {
			c.Docs[d.Path] = d.Hash
		}
	}
	if err := e.writeClaim(c); err != nil {
		return err
	}
	if err := e.addRelayEntry(c); err != nil {
		return err
	}
	if _, err := gitOut(e.Repo, "add", claimPath(leaf), "RELAY.md"); err != nil {
		return err
	}
	if _, err := gitOut(e.Repo, "commit", "-q", "-m", "chore("+leaf+"): 领取 "+n.Name, "-m", "依据设计仓库 "+short(docsCommit)+"。", "-m", "Refs: "+leaf); err != nil {
		return err
	}
	e.say("已领取 %s %s", leaf, n.Name)
	e.say("  分支：%s", branch)
	e.say("  依据：设计仓库 %s，%d 份文档（%s）", short(docsCommit), len(c.Docs), claimPath(leaf))
	e.say("  任务包：%s", filepath.ToSlash(packPath))
	return e.pErr
}

// checkClaimable：没有被别人领着，白名单不和别的已领取叶子重叠（G.02.06-R4、R7）。
func (e *Env) checkClaimable(t *Tree, leaf string) error {
	claims, err := e.allClaims()
	if err != nil {
		return err
	}
	if c, ok := claims[leaf]; ok && c.State != "merged" {
		return fmt.Errorf("%s 已被领取（%s，分支 %s，在 %s 上找到）", leaf, c.ClaimedBy, c.Branch, c.foundOn)
	}
	text, err := t.Text(leaf)
	if err != nil {
		return err
	}
	mine, _ := ParseCard(text)
	for _, other := range sortedKeys(claims) {
		c := claims[other]
		if other == leaf || c.State == "merged" {
			continue
		}
		otext, err := t.Text(other)
		if err != nil {
			continue
		}
		theirs, _ := ParseCard(otext)
		if ov := Overlap(mine.Create, theirs.Create); len(ov) > 0 {
			return fmt.Errorf("%s 的文件白名单和已领取的 %s 重叠，不能并行（G.02.06-R7）：%s", leaf, other, strings.Join(ov, "；"))
		}
	}
	return nil
}

const relayHeader = "# 接力日志（代码）\n\n> 每一棒在最上面加一条（新的在上）。\n\n---\n"

// addRelayEntry 在 RELAY.md 第一条分隔线（---）后面插入领取记录。
func (e *Env) addRelayEntry(c *Claim) error {
	p := filepath.Join(e.Repo, "RELAY.md")
	b, err := os.ReadFile(p)
	if err != nil {
		b = []byte(relayHeader)
	}
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	entry := fmt.Sprintf(`
## %s · %s · %s · 领取
- 依据：设计仓库提交 %s（%s）
- 完成：无
- 进行中：刚领取
- 下一步：读任务包 .relay/tasks/%s.md，从实施卡第 1 条开始
- 桩：无
- 上报单 / 阻塞：无
- relay check：未运行
- 评审：未开始
- 提交 / 推送：领取提交 / 未推送
`, e.Now().In(cst).Format("2006-01-02 15:04"), c.Leaf, c.ClaimedBy, short(c.DocsCommit), claimPath(c.Leaf), c.Leaf)
	i := strings.Index(s, "\n---\n")
	if i < 0 {
		s = s + "\n---\n" + entry
	} else {
		s = s[:i+5] + entry + s[i+5:]
	}
	return os.WriteFile(p, []byte(s), 0o644)
}

func short(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}
