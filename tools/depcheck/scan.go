package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Import 是一个 Go 文件里的一条 import。
type Import struct {
	Path string // import 路径（原样）
	File string // 相对模块根的文件路径（/ 分隔）
	Line int
	Test bool // 在 _test.go 里
}

// Package 是一个目录里的 Go 文件（不看构建标签，G.03.01 §5.2）。
type Package struct {
	Path    string // 相对模块根的路径（/ 分隔）；模块根本身为 ""
	Imports []Import
}

// modulePath 读 go.mod 的 module 行。
func modulePath(root string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("depcheck: 读不了 go.mod：%w", err)
	}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if rest, ok := strings.CutPrefix(line, "module"); ok && (rest == "" || rest[0] == ' ' || rest[0] == '\t') {
			m := strings.Trim(strings.TrimSpace(rest), `"`)
			if m != "" {
				return m, nil
			}
		}
	}
	return "", errors.New("depcheck: go.mod 里没有 module 行")
}

// skipDir：以 . 或 _ 开头的目录、testdata、vendor、node_modules（go 命令也不把它们当包）。
func skipDir(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata" || name == "vendor" || name == "node_modules"
}

// Scan 列出 root 下所有 Go 包和它们的 import；跳过嵌套模块。
func Scan(root string) ([]Package, error) {
	byDir := map[string]*Package{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p == root {
				return nil
			}
			if skipDir(d.Name()) {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(p, "go.mod")); err == nil {
				return filepath.SkipDir // 嵌套模块
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		dir := filepath.ToSlash(filepath.Dir(rel))
		if dir == "." {
			dir = ""
		}
		f, err := parser.ParseFile(fset, p, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("depcheck: 解析不了 %s：%w", rel, err)
		}
		pkg := byDir[dir]
		if pkg == nil {
			pkg = &Package{Path: dir}
			byDir[dir] = pkg
		}
		test := strings.HasSuffix(d.Name(), "_test.go")
		for _, s := range f.Imports {
			ip, err := strconv.Unquote(s.Path.Value)
			if err != nil || ip == "C" {
				continue
			}
			pkg.Imports = append(pkg.Imports, Import{Path: ip, File: rel, Line: fset.Position(s.Path.Pos()).Line, Test: test})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]Package, 0, len(byDir))
	for _, p := range byDir {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// Violation 是一处违规。
type Violation struct {
	File   string
	Line   int
	Pkg    string
	Import string
	Why    string
	Rule   int // 第几条规则（从 1 起）
}

// Format 写成"<文件>:<行>: <包> import <路径>：<why>（<规则文件> 第 N 条）"。
func (v Violation) Format(rulesName string) string {
	return fmt.Sprintf("%s:%d: %s import %s：%s（%s 第 %d 条）", v.File, v.Line, v.Pkg, v.Import, v.Why, rulesName, v.Rule)
}

// Check 按规则检查每个包的 import（G.03.01 §5.2）。module 是模块路径。
func Check(pkgs []Package, rules []Rule, module string) []Violation {
	var out []Violation
	for _, p := range pkgs {
		for i, r := range rules {
			if !match(r.Pkg, p.Path) || matchAny(r.Except, p.Path) {
				continue
			}
			for _, im := range p.Imports {
				if im.Test && !r.Tests {
					continue
				}
				target, inRepo := im.Path, false
				if im.Path == module {
					target, inRepo = "", true
				} else if rel, ok := strings.CutPrefix(im.Path, module+"/"); ok {
					target, inRepo = rel, true
				}
				bad := matchAny(r.Deny, target) || (r.HasAllow && inRepo && !matchAny(r.Allow, target))
				if bad {
					shown := target
					if !inRepo {
						shown = im.Path
					}
					out = append(out, Violation{File: im.File, Line: im.Line, Pkg: pkgName(p.Path), Import: shown, Why: r.Why, Rule: i + 1})
				}
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].Rule < out[j].Rule
	})
	return out
}

func pkgName(p string) string {
	if p == "" {
		return "（模块根）"
	}
	return p
}
