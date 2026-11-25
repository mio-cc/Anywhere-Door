// Command typenamecheck 检查调度、校验、运行代码里没有按原子节点类型名写分支（设计文档 G.05.05.01、G.05.05-R3、INV-02）：
//
//	go run ./tools/lint/typenamecheck [目录…]
//
// 缺省检查仓库的 engine/sched、engine/schema、engine/run（不存在的跳过）。找 ==、!=、带标签的 switch 的 case、
// strings.HasPrefix / HasSuffix / Contains / EqualFold 的操作数：字符串字面量等于某个原子节点类型名，或者是 gen 包里
// NodeType 开头的常量。节点的性质要从 NodeDef、TypeDef 和生成的表读。有问题退出码 1，读不了文件退出码 2。
package main

import (
	"bufio"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var defaultDirs = []string{"engine/sched", "engine/schema", "engine/run"}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	root, err := repoRoot()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "typenamecheck：", err)
		return 2
	}
	names, err := nodeTypeNames(root)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "typenamecheck：", err)
		return 2
	}
	dirs := args
	if len(dirs) == 0 {
		for _, d := range defaultDirs {
			if fi, err := os.Stat(filepath.Join(root, d)); err == nil && fi.IsDir() {
				dirs = append(dirs, filepath.Join(root, d))
			}
		}
	}
	var files []string
	for _, d := range dirs {
		err := filepath.WalkDir(d, func(p string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if e.IsDir() {
				if p != d && e.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
				files = append(files, p)
			}
			return nil
		})
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "typenamecheck：读不了 %s：%v\n", d, err)
			return 2
		}
	}
	sort.Strings(files)
	var found []finding
	for _, f := range files {
		hits, err := check(f, names)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "typenamecheck：%s：%v\n", rel(root, f), err)
			return 2
		}
		found = append(found, hits...)
	}
	for _, x := range found {
		_, _ = fmt.Fprintf(stdout, "%s:%d: 比较了节点类型名 %s：%s\n", rel(root, x.file), x.line, x.name, x.code)
	}
	if len(found) > 0 {
		_, _ = fmt.Fprintf(stdout, "typenamecheck：%d 处节点类型名的比较（节点的性质要从 NodeDef、TypeDef 和生成的表读，INV-02）\n", len(found))
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "typenamecheck：检查了 %d 个文件，没有节点类型名的比较\n", len(files))
	return 0
}

type finding struct {
	file, name, code string
	line             int
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
			return "", errors.New("找不到仓库根目录（go.mod）")
		}
		d = p
	}
}

func rel(root, p string) string {
	if r, err := filepath.Rel(root, p); err == nil && !strings.HasPrefix(r, "..") {
		return filepath.ToSlash(r)
	}
	return filepath.ToSlash(p)
}

// nodeTypeNames 是原子节点类型名：schema/builtin/nodedefs/*.yaml 的 name；读不到时用 gen 的 NodeType* 常量值。
func nodeTypeNames(root string) (map[string]bool, error) {
	names := map[string]bool{}
	files, _ := filepath.Glob(filepath.Join(root, "schema", "builtin", "nodedefs", "*.yaml"))
	for _, f := range files {
		n, err := yamlName(f)
		if err != nil {
			return nil, err
		}
		if n != "" {
			names[n] = true
		}
	}
	if len(names) > 0 {
		return names, nil
	}
	f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, "gen", "consts.go"), nil, 0)
	if err != nil {
		return nil, fmt.Errorf("读不到节点类型名：%w", err)
	}
	ast.Inspect(f, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, id := range vs.Names {
			if strings.HasPrefix(id.Name, "NodeType") && i < len(vs.Values) {
				if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if s, err := strconv.Unquote(lit.Value); err == nil {
						names[s] = true
					}
				}
			}
		}
		return true
	})
	if len(names) == 0 {
		return nil, errors.New("读不到节点类型名：schema/builtin/nodedefs/ 和 gen/consts.go 里都没有")
	}
	return names, nil
}

// yamlName 读定义文件顶层的 name（只认 "name: 值" 这一种写法，内置定义都是这样写的）。
func yamlName(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if v, ok := strings.CutPrefix(line, "name:"); ok {
			v = strings.TrimSpace(v)
			if u, err := strconv.Unquote(v); err == nil {
				v = u
			}
			return strings.Trim(v, `'`), nil
		}
	}
	return "", sc.Err()
}

var stringsFuncs = map[string]bool{"HasPrefix": true, "HasSuffix": true, "Contains": true, "EqualFold": true}

func check(path string, names map[string]bool) ([]finding, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("解析失败：%w", err)
	}
	lines := strings.Split(string(src), "\n")
	var out []finding
	add := func(e ast.Expr) {
		name, ok := typeName(e, names)
		if !ok {
			return
		}
		ln := fset.Position(e.Pos()).Line
		code := ""
		if ln-1 < len(lines) {
			code = strings.TrimSpace(lines[ln-1])
		}
		out = append(out, finding{file: path, name: name, code: code, line: ln})
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.BinaryExpr:
			if x.Op == token.EQL || x.Op == token.NEQ {
				add(x.X)
				add(x.Y)
			}
		case *ast.SwitchStmt:
			if x.Tag != nil {
				for _, st := range x.Body.List {
					if cc, ok := st.(*ast.CaseClause); ok {
						for _, e := range cc.List {
							add(e)
						}
					}
				}
			}
		case *ast.CallExpr:
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok && stringsFuncs[sel.Sel.Name] {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == "strings" {
					for _, a := range x.Args {
						add(a)
					}
				}
			}
		}
		return true
	})
	return out, nil
}

// typeName：e 是节点类型名的字符串字面量，或 gen.NodeType* 常量（包括包一层转换、括号的写法）。
func typeName(e ast.Expr, names map[string]bool) (string, bool) {
	switch x := e.(type) {
	case *ast.ParenExpr:
		return typeName(x.X, names)
	case *ast.CallExpr:
		if len(x.Args) == 1 {
			return typeName(x.Args[0], names)
		}
	case *ast.BasicLit:
		if x.Kind == token.STRING {
			if s, err := strconv.Unquote(x.Value); err == nil && names[s] {
				return s, true
			}
		}
	case *ast.SelectorExpr:
		if id, ok := x.X.(*ast.Ident); ok && id.Name == "gen" && strings.HasPrefix(x.Sel.Name, "NodeType") && x.Sel.Name != "NodeTypeRef" {
			return "gen." + x.Sel.Name, true
		}
	}
	return "", false
}
