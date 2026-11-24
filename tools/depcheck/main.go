// Command depcheck 检查 Go 包的依赖方向（设计文档 G.03.01，G.03-R2–R4、E.01 §1、B.01 §2、INV-19）：
//
//	go run ./tools/depcheck [-rules <文件>] [目录]
//
// 目录缺省为从当前目录往上找到的 go.mod 所在目录；规则文件缺省为 <模块根>/tools/depcheck/rules.yaml。
// 退出码：0 没有违规；1 有违规；2 规则文件、go.mod 或 Go 源文件读不了、写错了。
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, out, errOut io.Writer) int {
	fset := flag.NewFlagSet("depcheck", flag.ContinueOnError)
	fset.SetOutput(errOut)
	rulesPath := fset.String("rules", "", "规则文件（缺省 <模块根>/tools/depcheck/rules.yaml）")
	if err := fset.Parse(args); err != nil {
		return 2
	}
	if fset.NArg() > 1 {
		_, _ = fmt.Fprintln(errOut, "depcheck：最多一个目录参数")
		return 2
	}
	root := fset.Arg(0)
	if root == "" {
		r, err := findRoot()
		if err != nil {
			_, _ = fmt.Fprintln(errOut, err)
			return 2
		}
		root = r
	}
	module, err := modulePath(root)
	if err != nil {
		_, _ = fmt.Fprintln(errOut, err)
		return 2
	}
	rp := *rulesPath
	if rp == "" {
		rp = filepath.Join(root, "tools", "depcheck", "rules.yaml")
	}
	b, err := os.ReadFile(rp)
	if err != nil {
		_, _ = fmt.Fprintf(errOut, "depcheck: 读不了规则文件：%v\n", err)
		return 2
	}
	name := filepath.Base(rp)
	rules, err := LoadRules(b, name)
	if err != nil {
		_, _ = fmt.Fprintln(errOut, err)
		return 2
	}
	pkgs, err := Scan(root)
	if err != nil {
		_, _ = fmt.Fprintln(errOut, err)
		return 2
	}
	vs := Check(pkgs, rules, module)
	for _, v := range vs {
		_, _ = fmt.Fprintln(out, v.Format(name))
	}
	if len(vs) > 0 {
		_, _ = fmt.Fprintf(out, "depcheck：发现 %d 处依赖方向违规（规则见 %s，设计文档 G.03.01）\n", len(vs), filepath.ToSlash(rp))
		return 1
	}
	_, _ = fmt.Fprintf(out, "depcheck：检查了 %d 个包，没有违规\n", len(pkgs))
	return 0
}

// findRoot 从当前目录往上找 go.mod。
func findRoot() (string, error) {
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
			return "", errors.New("depcheck: 找不到仓库根目录（go.mod）")
		}
		d = p
	}
}
