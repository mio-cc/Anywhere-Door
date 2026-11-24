// Command deps 检查直接依赖都已登记（设计文档 O.02 §4.3、G.10-R1）：
//
//	go run ./tools/ci/deps
//
// 读仓库根的 go.mod（require 里不带 // indirect 的）和 web/package.json（dependencies、devDependencies），
// 对照 tools/deps/allow.txt。匹配规则和 tools/relay 的 DepAllowed 一样：完全相同，或是登记项加 / 的子路径（P.09 K246）。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ReadAllow 读 allow.txt：每行一个包名，# 开头为注释，空行忽略。
func ReadAllow(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			out = append(out, line)
		}
	}
	return out
}

// GoDirect 是 go.mod 里不带 // indirect 的 require。
func GoDirect(gomod string) []string {
	var out []string
	in := false
	for _, line := range strings.Split(strings.ReplaceAll(gomod, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "require (":
			in = true
			continue
		case in && line == ")":
			in = false
			continue
		case strings.HasPrefix(line, "require ") && !strings.HasSuffix(line, "("):
			line = strings.TrimPrefix(line, "require ")
		case !in:
			continue
		}
		if line == "" || strings.HasPrefix(line, "//") || strings.Contains(line, "// indirect") {
			continue
		}
		if f := strings.Fields(line); len(f) >= 2 {
			out = append(out, f[0])
		}
	}
	return out
}

// NpmDirect 是 package.json 的 dependencies 和 devDependencies 的包名。
func NpmDirect(pkg []byte) ([]string, error) {
	var p struct {
		Deps    map[string]string `json:"dependencies"`
		DevDeps map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(pkg, &p); err != nil {
		return nil, fmt.Errorf("web/package.json 不是合法的 JSON：%w", err)
	}
	var out []string
	for _, m := range []map[string]string{p.Deps, p.DevDeps} {
		for k := range m {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out, nil
}

func allowed(dep string, allow []string) bool {
	for _, a := range allow {
		if dep == a || strings.HasPrefix(dep, a+"/") {
			return true
		}
	}
	return false
}

// Check 返回不在清单里的依赖。
func Check(deps, allow []string) []string {
	var out []string
	for _, d := range deps {
		if !allowed(d, allow) {
			out = append(out, d)
		}
	}
	return out
}

func run(root string) ([]string, error) {
	a, err := os.ReadFile(filepath.Join(root, "tools", "deps", "allow.txt"))
	if err != nil {
		return nil, fmt.Errorf("读不了 tools/deps/allow.txt：%w", err)
	}
	m, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil, fmt.Errorf("读不了 go.mod：%w", err)
	}
	p, err := os.ReadFile(filepath.Join(root, "web", "package.json"))
	if err != nil {
		// 读不到就失败：不能让前端依赖的检查悄悄消失（第 ⑧b 片评审第 4 行）
		return nil, fmt.Errorf("读不了 web/package.json：%w", err)
	}
	n, err := NpmDirect(p)
	if err != nil {
		return nil, err
	}
	return Check(append(GoDirect(string(m)), n...), ReadAllow(string(a))), nil
}

func main() {
	bad, err := run(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误：", err)
		os.Exit(1)
	}
	for _, d := range bad {
		fmt.Printf("%s：不在 tools/deps/allow.txt 里（按 G.10-R2 上报，登记后同步改 allow.txt）\n", d)
	}
	if len(bad) > 0 {
		os.Exit(1)
	}
	fmt.Println("deps：直接依赖都在 tools/deps/allow.txt 里")
}
