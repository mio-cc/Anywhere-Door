package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Node 是设计树（树索引.json）里的一个节点。
type Node struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Desc     string  `json:"desc"`
	Type     string  `json:"type"` // 总 / 规 / 实 / 说
	Path     string  `json:"path"` // 相对设计文档目录
	Children []*Node `json:"children"`
	Parent   *Node   `json:"-"`
}

// Tree 是读进来的设计树。
type Tree struct {
	Dir   string
	Nodes map[string]*Node
	Order []string // 树的先序顺序
}

// LoadTree 读设计文档目录里的 树索引.json。
func LoadTree(dir string) (*Tree, error) {
	b, err := os.ReadFile(filepath.Join(dir, "树索引.json"))
	if err != nil {
		return nil, fmt.Errorf("读不到设计树（%s）：%w；用环境变量 ANYDOOR_DOCS 指定设计文档目录", dir, err)
	}
	var top []*Node
	if err := json.Unmarshal(b, &top); err != nil {
		return nil, fmt.Errorf("树索引.json 格式不对：%w", err)
	}
	t := &Tree{Dir: dir, Nodes: map[string]*Node{}}
	var walk func(n, parent *Node)
	walk = func(n, parent *Node) {
		n.Parent = parent
		t.Nodes[n.ID] = n
		t.Order = append(t.Order, n.ID)
		for _, c := range n.Children {
			walk(c, n)
		}
	}
	for _, n := range top {
		walk(n, nil)
	}
	return t, nil
}

// Node 按编号取节点。
func (t *Tree) Node(id string) (*Node, error) {
	n, ok := t.Nodes[id]
	if !ok {
		return nil, fmt.Errorf("设计树里没有 %s", id)
	}
	return n, nil
}

// Text 读节点对应的文档原文；文档还没写时返回 os.ErrNotExist。
func (t *Tree) Text(id string) (string, error) {
	n, err := t.Node(id)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(filepath.Join(t.Dir, filepath.FromSlash(n.Path)))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// State 读文档文件头的"状态"；文档不存在时为"待写"。
func (t *Tree) State(id string) (string, error) {
	text, err := t.Text(id)
	if errors.Is(err, os.ErrNotExist) {
		return "待写", nil
	}
	if err != nil {
		return "", err
	}
	return Header(text)["状态"], nil
}

// Ancestors 返回从一级总则到父节点的祖先链。
func (t *Tree) Ancestors(id string) []*Node {
	var chain []*Node
	n := t.Nodes[id]
	for n != nil && n.Parent != nil {
		chain = append([]*Node{n.Parent}, chain...)
		n = n.Parent
	}
	return chain
}

// IsImplLeaf 是不是可以领取的实施类叶子（G.02.01-R1）。
func (n *Node) IsImplLeaf() bool { return n.Type == "实" && len(n.Children) == 0 }

var headerRowRe = regexp.MustCompile(`^\|\s*([^|]+?)\s*\|\s*(.*?)\s*\|\s*$`)

// Header 读文档第一张表（文件头）里的 项 → 内容。
func Header(text string) map[string]string {
	h := map[string]string{}
	lines := strings.Split(text, "\n")
	if len(lines) > 25 {
		lines = lines[:25]
	}
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		m := headerRowRe.FindStringSubmatch(line)
		switch {
		case m != nil && m[1] != "项" && !strings.HasPrefix(m[1], "---"):
			if _, ok := h[m[1]]; !ok {
				h[m[1]] = m[2]
			}
		case len(h) > 0 && !strings.HasPrefix(line, "|"):
			return h
		}
	}
	return h
}

// Card 是实施卡里接力工具要用的三行。
type Card struct {
	Create []string // 要建 / 要改的文件（路径、目录前缀或通配）
	Forbid []string // 不许碰的文件
	Tests  []string // 测试用例编号
}

var (
	createLineRe = regexp.MustCompile(`^- 要(?:建|改)(?: / 要改)?的文件：(.*)$`)
	forbidLineRe = regexp.MustCompile(`^- 不许碰的文件：(.*)$`)
	tickRe       = regexp.MustCompile("`([^`]+)`")
	testIDRe     = regexp.MustCompile(`\b([A-Z][0-9]{2,6}-T[0-9]+)\b`)
	bareFileRe   = regexp.MustCompile(`^[\w.\-]+\.(go|ts|tsx|js|mjs|css|json|yaml|yml|md|txt|html|sql|svg|tmpl)$`)
)

// rootFiles 是仓库根目录的文件：实施卡里不带目录写的时候指根目录，不接在前一个目录后面。
var rootFiles = map[string]bool{
	"Makefile": true, "Taskfile.yaml": true, "go.mod": true, "go.sum": true, ".gitignore": true,
	".gitattributes": true, ".golangci.yml": true, "LICENSE": true, "NOTICE": true, "CHANGELOG.md": true,
	"README.md": true, "AGENTS.md": true, "CLAUDE.md": true, "RELAY.md": true,
}

// ParseCard 读文档里的实施卡；没有实施卡时 ok 为 false。
func ParseCard(text string) (c Card, ok bool) {
	i := strings.Index(text, "### 实施卡")
	if i < 0 {
		return c, false
	}
	section := text[i:]
	if j := strings.Index(section, "\n## "); j >= 0 {
		section = section[:j]
	}
	inTests := false
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimRight(line, "\r")
		if m := createLineRe.FindStringSubmatch(line); m != nil {
			c.Create = append(c.Create, cardPaths(m[1])...)
		}
		if m := forbidLineRe.FindStringSubmatch(line); m != nil {
			c.Forbid = append(c.Forbid, cardPaths(m[1])...)
		}
		switch {
		case strings.HasPrefix(line, "- 测试用例"):
			inTests = true
		case strings.HasPrefix(line, "- "):
			inTests = false
		case inTests && strings.HasPrefix(strings.TrimSpace(line), "- "):
			if m := tickRe.FindStringSubmatch(line); m != nil {
				if id := testIDRe.FindString(m[1]); id != "" && strings.HasPrefix(m[1], id) {
					c.Tests = append(c.Tests, id)
				}
			}
		}
	}
	return c, true
}

// cardPaths 从实施卡一行里取出路径：带目录的原样取；只有文件名的接在前一个目录后面（根目录文件除外）。
func cardPaths(line string) []string {
	var out []string
	prevDir := ""
	for _, m := range tickRe.FindAllStringSubmatch(line, -1) {
		tok := strings.TrimPrefix(strings.TrimSpace(m[1]), "!")
		tok = strings.TrimPrefix(tok, "./")
		switch {
		case tok == "" || strings.ContainsAny(tok, " {}()=:") || strings.HasPrefix(tok, "@") || strings.HasPrefix(tok, "http"):
			continue
		case rootFiles[tok]:
			out = append(out, tok)
		case strings.Contains(tok, "/"):
			out = append(out, tok)
			last := tok[strings.LastIndex(tok, "/")+1:]
			switch {
			case last == "" || last == "**" || last == "*":
				prevDir = strings.TrimSuffix(strings.TrimSuffix(tok, last), "/")
			case strings.Contains(last, "."):
				prevDir = tok[:strings.LastIndex(tok, "/")]
			default:
				prevDir = tok
			}
		case bareFileRe.MatchString(tok) && prevDir != "":
			out = append(out, prevDir+"/"+tok)
		}
	}
	return out
}

// MatchAny 判断仓库里的一个文件（正斜杠路径）是否匹配实施卡里的某一项。
// 以 / 结尾的是目录前缀；含 * 的是通配（** 跨目录）；其余要求完全相同。
func MatchAny(file string, patterns []string) bool {
	for _, p := range patterns {
		switch {
		case strings.HasSuffix(p, "/"):
			if strings.HasPrefix(file, p) {
				return true
			}
		case strings.Contains(p, "*"):
			if globRe(p).MatchString(file) {
				return true
			}
		case file == p:
			return true
		}
	}
	return false
}

func globRe(p string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(p); i++ {
		switch {
		case strings.HasPrefix(p[i:], "**/"):
			b.WriteString("(?:.*/)?")
			i += 2
		case strings.HasPrefix(p[i:], "**"):
			b.WriteString(".*")
			i++
		case p[i] == '*':
			b.WriteString("[^/]*")
		default:
			b.WriteString(regexp.QuoteMeta(string(p[i])))
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String())
}

// staticPrefix 是一项里第一个 * 之前的部分，用来粗略判断两个白名单是否重叠。
func staticPrefix(p string) string {
	if i := strings.Index(p, "*"); i >= 0 {
		return p[:i]
	}
	return p
}

// Overlap 返回两个白名单可能重叠的项（G.02.06-R7）；判断偏保守。
func Overlap(a, b []string) []string {
	var out []string
	for _, x := range a {
		for _, y := range b {
			px, py := staticPrefix(x), staticPrefix(y)
			if px == py || (isOpen(x) && strings.HasPrefix(py, px)) || (isOpen(y) && strings.HasPrefix(px, py)) {
				out = append(out, x+" ↔ "+y)
			}
		}
	}
	return out
}

func isOpen(p string) bool { return strings.HasSuffix(p, "/") || strings.Contains(p, "*") }

// RefCodes 取出正文里引用的设计树编号（至少两级，如 C.01、W.06.13）。
func RefCodes(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, loc := range refRe.FindAllStringIndex(text, -1) {
		s, e := loc[0], loc[1]
		if s > 0 && isRefGuard(text[s-1]) {
			continue
		}
		if e < len(text) && (isDigit(text[e]) || (text[e] == '.' && e+1 < len(text) && isDigit(text[e+1]))) {
			continue
		}
		code := text[s:e]
		if !seen[code] {
			seen[code] = true
			out = append(out, code)
		}
	}
	return out
}

var refRe = regexp.MustCompile(`[PGCEBWUSQOXTDR](?:\.[0-9]{2}){1,4}`)

func isRefGuard(c byte) bool {
	return c == '_' || c == '.' || c == '/' || isDigit(c) || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// NormHash 是文档内容的指纹：去掉文件头的"状态""最后修改"两行和"变更记录"一节再算 SHA-256，
// 这样冻结（只改状态和变更记录）不算改动，正文改了才算（G.02.06-R13、R21）。
func NormHash(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if i := strings.Index(text, "\n## 变更记录"); i >= 0 {
		text = text[:i]
	}
	lines := strings.Split(text, "\n")
	var kept []string
	for i, line := range lines {
		if i < 25 && (strings.HasPrefix(line, "| 状态 |") || strings.HasPrefix(line, "| 最后修改 |")) {
			continue
		}
		kept = append(kept, line)
	}
	h := sha256.Sum256([]byte(strings.Join(kept, "\n")))
	return "sha256:" + hex.EncodeToString(h[:])
}

// AllowedDeps 读依赖白名单：有 tools/deps/allow.txt 用它；否则读设计文档 G.10 §3 里登记的包名（P.09 K196）。
func AllowedDeps(repo string, t *Tree) ([]string, string, error) {
	if b, err := os.ReadFile(filepath.Join(repo, "tools", "deps", "allow.txt")); err == nil {
		var out []string
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				out = append(out, line)
			}
		}
		return out, "tools/deps/allow.txt", nil
	}
	text, err := t.Text("G.10")
	if err != nil {
		return nil, "", fmt.Errorf("没有 tools/deps/allow.txt，也读不到设计文档 G.10：%w", err)
	}
	if i := strings.Index(text, "## 3."); i >= 0 {
		text = text[i:]
	}
	if j := strings.Index(text, "\n## 4."); j >= 0 {
		text = text[:j]
	}
	var out []string
	for _, m := range tickRe.FindAllStringSubmatch(text, -1) {
		tok := strings.TrimSpace(m[1])
		if tok != "" && !strings.ContainsAny(tok, " ，、") {
			out = append(out, tok)
		}
	}
	return out, "设计文档 G.10 §3", nil
}

// DepAllowed：完全相同，或是登记项的子包（go.opentelemetry.io/otel 覆盖 go.opentelemetry.io/otel/sdk）。
func DepAllowed(dep string, allowed []string) bool {
	for _, a := range allowed {
		if dep == a || strings.HasPrefix(dep, a+"/") {
			return true
		}
	}
	return false
}
