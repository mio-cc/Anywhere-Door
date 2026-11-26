package sched

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// forbiddenTime 是内核里不能直接调用的 time 函数（E.15.04 §5）。
var forbiddenTime = map[string]bool{"Now": true, "After": true, "Sleep": true, "NewTimer": true, "NewTicker": true, "Tick": true}

// directTimeCalls 是一份源码里直接调用 time 包这些函数的位置（"文件:行"）。
func directTimeCalls(name, src string) ([]string, error) {
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, name, src, 0)
	if err != nil {
		return nil, err
	}
	alias := ""
	for _, im := range f.Imports {
		if strings.Trim(im.Path.Value, `"`) == "time" {
			alias = "time"
			if im.Name != nil {
				alias = im.Name.Name
			}
		}
	}
	if alias == "" {
		return nil, nil
	}
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == alias && forbiddenTime[sel.Sel.Name] {
				out = append(out, fs.Position(call.Pos()).String())
			}
		}
		return true
	})
	return out, nil
}

func TestClockCheck(t *testing.T) {
	t.Run("E1504-T3 直接调用检查", func(t *testing.T) {
		bad := "package x\nimport \"time\"\nfunc f() { _ = time.Now(); time.Sleep(1) }\n"
		if got, err := directTimeCalls("bad.go", bad); err != nil || len(got) != 2 {
			t.Errorf("应当报出两处：%v %v", got, err)
		}
		alias := "package x\nimport tm \"time\"\nfunc f() { _ = tm.After(1); _ = tm.Duration(1) }\n"
		if got, _ := directTimeCalls("alias.go", alias); len(got) != 1 {
			t.Errorf("别名导入也要报出来：%v", got)
		}
		for _, pkg := range []string{"sched", "run", "op", "exec", "obs"} {
			dir := filepath.Join("..", pkg)
			_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
					return nil
				}
				if pkg == "sched" && filepath.Base(p) == "clock.go" {
					return nil
				}
				b, err := os.ReadFile(p)
				if err != nil {
					t.Error(err)
					return nil
				}
				got, err := directTimeCalls(p, string(b))
				if err != nil {
					t.Error(err)
				}
				for _, at := range got {
					t.Errorf("%s：内核里不能直接调用 time 包的时间函数，用注入的 Clock（E.15-R3）", at)
				}
				return nil
			})
		}
	})
}
