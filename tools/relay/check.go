package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Result 是检查的一项（G.02.06-R14 的 ①–⑦）。
type Result struct {
	Item   int
	Title  string
	OK     bool
	Detail []string
}

var itemMarks = []string{"", "①", "②", "③", "④", "⑤", "⑥", "⑦"}

// Check 是交付前、接手时必须跑的检查（G.02.06-R14）。返回是否全部通过。
func (e *Env) Check(leaf string, noBuild bool) (bool, error) {
	results, err := e.CheckResults(leaf, noBuild)
	if err != nil {
		return false, err
	}
	all := e.printResults(results)
	if all {
		e.say("== %s 的检查全部通过 ==", leaf)
	} else {
		e.say("== %s 的检查没有通过 ==", leaf)
	}
	return all, e.pErr
}

// printResults 打印各项结果，返回是否全部通过。
func (e *Env) printResults(results []Result) bool {
	all := true
	for _, r := range results {
		status := "通过"
		if !r.OK {
			status = "不通过"
			all = false
		}
		e.say("%s %s：%s", itemMarks[r.Item], r.Title, status)
		for _, d := range r.Detail {
			e.say("    %s", d)
		}
	}
	return all
}

// CheckResults 逐项检查，不打印。
func (e *Env) CheckResults(leaf string, noBuild bool) ([]Result, error) {
	t, err := e.Tree()
	if err != nil {
		return nil, err
	}
	claim, err := e.readClaim(leaf)
	if err != nil {
		return nil, err
	}
	text, err := t.Text(leaf)
	if err != nil {
		return nil, err
	}
	card, _ := ParseCard(text)
	base, head, err := e.claimRange(claim)
	if err != nil {
		return nil, err
	}
	changed, err := e.changedFiles(base, head)
	if err != nil {
		return nil, err
	}
	always := []string{"RELAY.md", "CHANGELOG.md", claimPath(leaf), "docs/relay/" + leaf + ".review.md"}
	if claim.Slice != "" {
		// 片内连续领取（G.02.06-R23）：下一个叶子领取时会改上一个叶子的领取记录；整片一份评审记录
		always = append(always, "docs/relay/"+claim.Slice+".review.md")
		for _, c := range e.sliceClaims(claim.Slice) {
			always = append(always, claimPath(c.Leaf))
		}
	}
	stubs := claim.Stubs

	var res []Result
	// ① 文件白名单
	r1 := Result{Item: 1, Title: "改动的文件都在白名单里", OK: true}
	for _, f := range changed {
		switch {
		case MatchAny(f, always) || MatchAny(f, stubs):
		case exactlyListed(f, card.Create):
			// 逐个写出的文件优先于「不许碰」里的目录（G.02.06-R24 ④）
		case MatchAny(f, card.Forbid):
			r1.OK = false
			r1.Detail = append(r1.Detail, f+"：在实施卡的「不许碰」里")
		case !MatchAny(f, card.Create):
			r1.OK = false
			r1.Detail = append(r1.Detail, f+"：不在实施卡的「要建 / 要改的文件」里（要改就上报，G.02.03-R4；是桩就登记进 "+claimPath(leaf)+"）")
		}
	}
	res = append(res, r1)
	// ② 契约与生成物
	r2 := Result{Item: 2, Title: "没有改 schema/、gen/、*/generated/", OK: true}
	for _, f := range changed {
		if (strings.HasPrefix(f, "schema/") || strings.HasPrefix(f, "gen/") || strings.Contains(f, "/generated/")) && !exactlyListed(f, card.Create) {
			r2.OK = false
			r2.Detail = append(r2.Detail, f)
		}
	}
	res = append(res, r2)
	// ③ 依赖
	res = append(res, e.checkDeps(t, base, head))
	// ④ 测试没有被删或跳过
	res = append(res, e.checkTests(base, head, changed))
	// ⑤ 测试用例编号
	res = append(res, e.checkTestIDs(card.Tests))
	// ⑥ 依据文档
	r6 := Result{Item: 6, Title: "依据文档在领取后没有改过", OK: true}
	if ch := changedDocs(t, claim); len(ch) > 0 {
		r6.OK = false
		r6.Detail = append(r6.Detail, "这些文档改过，先运行 relay diff "+leaf+" 看改了什么，确认代码要不要跟着改（G.02.06-R13）：")
		r6.Detail = append(r6.Detail, ch...)
	}
	res = append(res, r6)
	// ⑦ 完整构建检查
	r7 := Result{Item: 7, Title: "go run ./tools/build -check", OK: true}
	if noBuild {
		r7.Detail = []string{"已跳过（--no-build，只用于本地快速自查；交付和 CI 不允许跳过）"}
	} else {
		if err := e.buildCheck(); err != nil {
			r7.OK = false
			r7.Detail = []string{err.Error()}
		}
	}
	res = append(res, r7)
	return res, nil
}

// buildCheck 运行 go run ./tools/build -check（G.02.06-R14 ⑦）。
func (e *Env) buildCheck() error {
	cmd := exec.Command("go", "run", "./tools/build", "-check")
	cmd.Dir = e.Repo
	cmd.Stdout, cmd.Stderr = e.Out, e.Out
	return cmd.Run()
}

func exactlyListed(f string, patterns []string) bool {
	for _, p := range patterns {
		if p == f {
			return true
		}
	}
	return false
}

// baseCommit 是当前分支和 main 的分叉点（没有 main 时用 origin/main）。
func (e *Env) baseCommit() (string, error) {
	for _, ref := range []string{"main", "origin/main"} {
		if b, err := gitOut(e.Repo, "merge-base", "HEAD", ref); err == nil {
			return b, nil
		}
	}
	return "", fmt.Errorf("找不到和 main 的分叉点：仓库里没有 main 分支")
}

// claimRange 是一个叶子的检查范围：从 base 到 head。普通叶子 base 是和 main 的分叉点；
// 片内的叶子 base 是领取前的提交（G.02.06-R23）。head 为空表示到工作区为止；片内已完成的叶子
// head 是下一个叶子领取前的提交，只看它自己的那一段。记录里的提交已经不存在（分支合并删除后）时，退回分叉点。
func (e *Env) claimRange(c *Claim) (base, head string, err error) {
	exists := func(rev string) bool {
		_, err := gitOut(e.Repo, "cat-file", "-e", rev+"^{commit}")
		return err == nil
	}
	// base、head 要么都用，要么都不用：head 记了却已经不存在时，只用 base 会把后面叶子的改动算进来
	if c.Base != "" && exists(c.Base) && (c.Head == "" || exists(c.Head)) {
		return c.Base, c.Head, nil
	}
	base, err = e.baseCommit()
	return base, "", err
}

// changedFiles 是 base 之后改过的文件。head 为空时：已提交、已暂存、未暂存和未跟踪的（不含 .gitignore 忽略的）；
// head 不为空时：只看 base 到 head 之间已提交的。
func (e *Env) changedFiles(base, head string) ([]string, error) {
	if head != "" {
		diff, err := gitOut(e.Repo, "diff", "--name-only", base, head)
		if err != nil {
			return nil, err
		}
		return sortedKeys(setOf(lines(diff))), nil
	}
	diff, err := gitOut(e.Repo, "diff", "--name-only", base)
	if err != nil {
		return nil, err
	}
	untracked, err := gitOut(e.Repo, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, f := range append(lines(diff), lines(untracked)...) {
		set[f] = true
	}
	return sortedKeys(set), nil
}

var (
	requireLineRe = regexp.MustCompile(`^\s*(?:require\s+)?([^\s()]+)\s+v[0-9][^\s]*`)
)

// goRequires 取 go.mod 里的直接依赖（去掉 // indirect）。
func goRequires(mod string) map[string]bool {
	out := map[string]bool{}
	inBlock := false
	for _, line := range strings.Split(mod, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "require ("):
			inBlock = true
			continue
		case inBlock && line == ")":
			inBlock = false
			continue
		case !inBlock && !strings.HasPrefix(line, "require "):
			continue
		}
		if strings.Contains(line, "// indirect") {
			continue
		}
		if m := requireLineRe.FindStringSubmatch(line); m != nil {
			out[m[1]] = true
		}
	}
	return out
}

func npmDeps(pkg string) map[string]bool {
	out := map[string]bool{}
	var p struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if json.Unmarshal([]byte(pkg), &p) != nil {
		return out
	}
	for k := range p.Dependencies {
		out[k] = true
	}
	for k := range p.DevDependencies {
		out[k] = true
	}
	return out
}

func (e *Env) fileAt(rev, path string) string {
	s, err := gitOut(e.Repo, "show", rev+":"+path)
	if err != nil {
		return ""
	}
	return s
}

// fileIn 是文件在 head 时的内容；head 为空时读工作区。
func (e *Env) fileIn(head, path string) string {
	if head != "" {
		return e.fileAt(head, path)
	}
	return e.fileNow(path)
}

func setOf(items []string) map[string]bool {
	m := map[string]bool{}
	for _, s := range items {
		m[s] = true
	}
	return m
}

func (e *Env) fileNow(path string) string {
	b, err := os.ReadFile(filepath.Join(e.Repo, filepath.FromSlash(path)))
	if err != nil {
		return ""
	}
	return string(b)
}

// checkDeps：新增的直接依赖都已登记（G.10、G.02.06-R14 ③）。
func (e *Env) checkDeps(t *Tree, base, head string) Result {
	r := Result{Item: 3, Title: "没有新增未登记的依赖", OK: true}
	var added []string
	before, after := goRequires(e.fileAt(base, "go.mod")), goRequires(e.fileIn(head, "go.mod"))
	for _, k := range sortedKeys(after) {
		if !before[k] {
			added = append(added, k)
		}
	}
	nb, na := npmDeps(e.fileAt(base, "web/package.json")), npmDeps(e.fileIn(head, "web/package.json"))
	for _, k := range sortedKeys(na) {
		if !nb[k] {
			added = append(added, k)
		}
	}
	if len(added) == 0 {
		return r
	}
	allowed, source, err := AllowedDeps(e.Repo, t)
	if err != nil {
		r.OK = false
		r.Detail = []string{err.Error()}
		return r
	}
	for _, d := range added {
		if !DepAllowed(d, allowed) {
			r.OK = false
			r.Detail = append(r.Detail, d+"：不在"+source+"里（按 G.10-R2 上报）")
		}
	}
	if r.OK {
		r.Detail = []string{"新增 " + strings.Join(added, "、") + "，都在" + source + "里"}
	}
	return r
}

var (
	goTestFuncRe = regexp.MustCompile(`^\s*func (Test\w+)\(`)
	goRunRe      = regexp.MustCompile(`t\.Run\("([^"]+)"`)
	tsTestRe     = regexp.MustCompile(`\b(?:test|it|describe)\(\s*['"` + "`" + `]([^'"` + "`" + `]+)`)
	skipRe       = regexp.MustCompile(`\bt\.Skip(?:Now|f)?\(|\.skip\(|\.only\(|\btest\.fixme\(`)
)

func isTestFile(f string) bool {
	return strings.HasSuffix(f, "_test.go") || strings.HasSuffix(f, ".test.ts") || strings.HasSuffix(f, ".test.tsx") || strings.HasSuffix(f, ".spec.ts")
}

func testNames(line string) []string {
	var out []string
	if m := goTestFuncRe.FindStringSubmatch(line); m != nil {
		out = append(out, m[1])
	}
	for _, m := range goRunRe.FindAllStringSubmatch(line, -1) {
		out = append(out, m[1])
	}
	for _, m := range tsTestRe.FindAllStringSubmatch(line, -1) {
		out = append(out, m[1])
	}
	return out
}

// checkTests：没有删除已有测试，没有新增跳过（G.02.06-R14 ④、G.02.05-R30）。
func (e *Env) checkTests(base, head string, changed []string) Result {
	r := Result{Item: 4, Title: "没有删除或跳过已有测试", OK: true}
	removed, added := map[string]string{}, map[string]bool{}
	var skips []string
	tracked := map[string]bool{}
	if ls, err := gitOut(e.Repo, "ls-files"); err == nil {
		for _, f := range lines(ls) {
			tracked[f] = true
		}
	}
	for _, f := range changed {
		if !isTestFile(f) {
			continue
		}
		var minus, plus []string
		if tracked[f] || e.fileAt(base, f) != "" {
			args := []string{"diff", "-U0", base}
			if head != "" {
				args = append(args, head)
			}
			d, _ := gitOut(e.Repo, append(args, "--", f)...)
			for _, l := range lines(d) {
				switch {
				case strings.HasPrefix(l, "---") || strings.HasPrefix(l, "+++"):
				case strings.HasPrefix(l, "-"):
					minus = append(minus, l[1:])
				case strings.HasPrefix(l, "+"):
					plus = append(plus, l[1:])
				}
			}
		} else {
			plus = strings.Split(e.fileIn(head, f), "\n")
		}
		for _, l := range minus {
			for _, n := range testNames(l) {
				removed[n] = f
			}
		}
		for _, l := range plus {
			for _, n := range testNames(l) {
				added[n] = true
			}
			if skipRe.MatchString(l) {
				skips = append(skips, f+"："+strings.TrimSpace(l))
			}
		}
	}
	for _, n := range sortedKeys(removed) {
		if !added[n] {
			r.OK = false
			r.Detail = append(r.Detail, "删除了测试 "+n+"（"+removed[n]+"）")
		}
	}
	for _, s := range skips {
		r.OK = false
		r.Detail = append(r.Detail, "新增了跳过："+s)
	}
	return r
}

// checkTestIDs：实施卡里的每个测试用例编号都出现在某个测试名的开头（G.02.06-R14 ⑤、Q-R4）。
func (e *Env) checkTestIDs(ids []string) Result {
	r := Result{Item: 5, Title: "实施卡的测试用例都有对应测试", OK: true}
	found := e.foundTestIDs(ids)
	var missing []string
	for _, id := range ids {
		if !found[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		r.OK = false
		r.Detail = []string{"缺少：" + strings.Join(missing, "、")}
	}
	r.Detail = append(r.Detail, "覆盖 "+strconv.Itoa(len(ids)-len(missing))+"/"+strconv.Itoa(len(ids)))
	return r
}

func (e *Env) foundTestIDs(ids []string) map[string]bool {
	found := map[string]bool{}
	if len(ids) == 0 {
		return found
	}
	var texts []string
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
		if isTestFile(filepath.ToSlash(p)) {
			if b, err := os.ReadFile(p); err == nil {
				texts = append(texts, string(b))
			}
		}
		return nil
	})
	all := strings.Join(texts, "\n")
	for _, id := range ids {
		re := regexp.MustCompile("[\"'`]" + regexp.QuoteMeta(id) + "[ /\"'`]")
		if re.MatchString(all) {
			found[id] = true
		}
	}
	return found
}

// changedDocs 列出依据文档里指纹变了的（G.02.06-R13）。
func changedDocs(t *Tree, c *Claim) []string {
	var out []string
	for _, p := range sortedKeys(c.Docs) {
		b, err := os.ReadFile(filepath.Join(t.Dir, filepath.FromSlash(p)))
		switch {
		case err != nil:
			out = append(out, p+"（已删除或改名）")
		case NormHash(string(b)) != c.Docs[p]:
			out = append(out, p)
		}
	}
	return out
}

var hunkRe = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

// Diff 列出依据文档在领取后改了哪些、改在哪些小节（G.02.06-R13）。
func (e *Env) Diff(leaf string) error {
	t, err := e.Tree()
	if err != nil {
		return err
	}
	c, err := e.readClaim(leaf)
	if err != nil {
		return err
	}
	changed := changedDocs(t, c)
	if len(changed) == 0 {
		e.say("%s 的依据文档在领取后（设计仓库 %s）都没有改过", leaf, short(c.DocsCommit))
		return e.pErr
	}
	e.say("%s 的依据文档里，领取后（设计仓库 %s）改过的：", leaf, short(c.DocsCommit))
	for _, p := range changed {
		e.say("- %s", p)
		if strings.HasSuffix(p, "（已删除或改名）") {
			continue
		}
		d, err := gitOut(t.Dir, "diff", "-U0", c.DocsCommit, "--", p)
		if err != nil {
			e.say("    （读不到差异：%v）", err)
			continue
		}
		cur, _ := os.ReadFile(filepath.Join(t.Dir, filepath.FromSlash(p)))
		for _, h := range sectionsOf(string(cur), d) {
			e.say("    %s", h)
		}
	}
	return e.pErr
}

// sectionsOf 把差异的每一块对到当前文档里离它最近的标题。
func sectionsOf(text, diff string) []string {
	docLines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	seen := map[string]bool{}
	var out []string
	for _, l := range lines(diff) {
		m := hunkRe.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		heading := "（文件开头）"
		for i := start - 1; i >= 0; i-- {
			if i < len(docLines) && strings.HasPrefix(docLines[i], "#") {
				heading = strings.TrimSpace(docLines[i])
				break
			}
		}
		if !seen[heading] {
			seen[heading] = true
			out = append(out, heading)
		}
	}
	sort.Strings(out)
	return out
}
