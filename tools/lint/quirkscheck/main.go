// Command quirkscheck 检查 engine/llm 里没有厂商名、模型名的字符串比较（设计文档 E.10.05 §5.1、G.05.05-R2、INV-18）：
// 厂商差异只能写在 quirks 表和提供方配置里，协议代码不知道自己连的是哪一家。
//
//	go run ./tools/lint/quirkscheck [目录]   缺省检查仓库的 engine/llm；有问题退出码 1
//
// 扫描非测试的 .go 文件（跳过 quirks/ 和 testdata/）：==、!=、switch 的 case、
// strings.HasPrefix / HasSuffix / Contains / EqualFold 里的字符串字面量，含有厂商或模型的词就报错。
// 词表 = 固定词表 + quirks.yaml 里 model 通配去掉 * 的前缀；协议名不算。
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// fixedWords 是固定的厂商、模型词表（E.10.05 §5.1）。
var fixedWords = []string{"openai", "anthropic", "claude", "gemini", "google", "mistral", "deepseek", "gpt", "qwen", "glm", "moonshot", "kimi", "llama", "ollama"}

// protocolNames 是协议名：字面量里出现协议名不算（E.10.05 §5.1）。
var protocolNames = []string{"chat_completions", "responses", "anthropic_messages"}

var checkedCalls = []string{"HasPrefix", "HasSuffix", "Contains", "EqualFold"}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, out, errOut io.Writer) int {
	root, err := repoRoot()
	if err != nil {
		_, _ = fmt.Fprintln(errOut, "quirkscheck：", err)
		return 2
	}
	dir := filepath.Join(root, "engine", "llm")
	if len(args) > 0 {
		dir = args[0]
	}
	words := slices.Clone(fixedWords)
	if b, err := os.ReadFile(filepath.Join(root, "engine", "llm", "quirks", "quirks.yaml")); err == nil {
		words = append(words, quirksWords(b)...)
	}
	findings, files, err := scan(dir, words)
	if err != nil {
		_, _ = fmt.Fprintln(errOut, "quirkscheck：", err)
		return 2
	}
	for _, f := range findings {
		_, _ = fmt.Fprintln(out, f)
	}
	if len(findings) > 0 {
		_, _ = fmt.Fprintf(out, "quirkscheck：发现 %d 处厂商名或模型名的比较；厂商差异写进 engine/llm/quirks/quirks.yaml 或提供方配置（INV-18）\n", len(findings))
		return 1
	}
	_, _ = fmt.Fprintf(out, "quirkscheck：检查了 %d 个文件，没有厂商名、模型名的比较\n", files)
	return 0
}

// repoRoot 从当前目录往上找 go.mod。
func repoRoot() (string, error) {
	d, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d, nil
		}
		p := filepath.Dir(d)
		if p == d {
			return "", fmt.Errorf("找不到仓库根目录（go.mod）")
		}
		d = p
	}
}

var quirksModelRe = regexp.MustCompile(`\bmodel:\s*"?([^",}\s]+)`)

// quirksWords 从 quirks.yaml 的 model 通配里取前缀（去掉 *）。
func quirksWords(b []byte) []string {
	var out []string
	for _, m := range quirksModelRe.FindAllSubmatch(b, -1) {
		w := strings.ToLower(strings.TrimRight(strings.ReplaceAll(string(m[1]), "*", ""), "-_."))
		if len(w) >= 3 && !slices.Contains(out, w) {
			out = append(out, w)
		}
	}
	return out
}

// Finding 是一处违规。
type Finding struct {
	File string
	Line int
	Lit  string
	Code string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s:%d: 比较了含厂商或模型名的字符串 %q：%s", f.File, f.Line, f.Lit, f.Code)
}

// scan 检查 dir 下所有非测试的 .go 文件，返回违规和检查过的文件数。
func scan(dir string, words []string) ([]Finding, int, error) {
	var out []Finding
	files := 0
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != dir && (d.Name() == "quirks" || d.Name() == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		files++
		found, err := checkFile(p, words)
		out = append(out, found...)
		return err
	})
	return out, files, err
}

func checkFile(p string, words []string) ([]Finding, error) {
	src, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, p, src, 0)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")
	var out []Finding
	report := func(e ast.Expr) {
		lit, ok := e.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil || !vendorish(s, words) {
			return
		}
		line := fset.Position(lit.Pos()).Line
		code := ""
		if line-1 < len(lines) {
			code = strings.TrimSpace(lines[line-1])
		}
		out = append(out, Finding{File: filepath.ToSlash(p), Line: line, Lit: s, Code: code})
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.BinaryExpr:
			if x.Op == token.EQL || x.Op == token.NEQ {
				report(x.X)
				report(x.Y)
			}
		case *ast.SwitchStmt:
			if x.Tag == nil {
				break
			}
			for _, s := range x.Body.List {
				if cc, ok := s.(*ast.CaseClause); ok {
					for _, e := range cc.List {
						report(e)
					}
				}
			}
		case *ast.CallExpr:
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == "strings" && slices.Contains(checkedCalls, sel.Sel.Name) {
					for _, a := range x.Args {
						report(a)
					}
				}
			}
		}
		return true
	})
	return out, nil
}

// vendorish：去掉协议名后，字面量里含有厂商或模型的词（不区分大小写）。
func vendorish(s string, words []string) bool {
	l := strings.ToLower(s)
	for _, p := range protocolNames {
		l = strings.ReplaceAll(l, p, " ")
	}
	for _, w := range words {
		if strings.Contains(l, w) {
			return true
		}
	}
	return false
}
