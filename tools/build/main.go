// Command build 是仓库的构建工具（设计文档 O.01 §4.2）：
//
//	go run ./tools/build [-os GOOS] [-arch GOARCH] [-out DIR] [-preview]   构建 anydoor、anydoorctl
//	go run ./tools/build -check                                          提交前的完整检查
//	go run ./tools/build -clean                                          删除输出目录
//
// 只用 Go 标准库，通过 os/exec 调用 go、git、pnpm、golangci-lint；不依赖 make 和 shell。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// programs 是要构建的两个程序（设计文档 O.01 §4.2 第 ③ 步）。
var programs = []string{"anydoor", "anydoorctl"}

// outputDirs 是 -clean 要删除的输出目录（设计文档 O.01 §4.1）。
var outputDirs = []string{"bin", "dist", "web/dist", "web/dist-perf", "web/.perf", "web/.test"}

const placeholder = ".gitkeep"

type options struct {
	goos, goarch, out string
	preview           bool
}

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	fset := flag.NewFlagSet("build", flag.ContinueOnError)
	o := options{}
	fset.StringVar(&o.goos, "os", runtime.GOOS, "目标操作系统（GOOS）")
	fset.StringVar(&o.goarch, "arch", runtime.GOARCH, "目标架构（GOARCH）")
	fset.StringVar(&o.out, "out", "bin", "输出目录")
	fset.BoolVar(&o.preview, "preview", false, "预览版：版本号加 -preview（O-R6）")
	doCheck := fset.Bool("check", false, "提交前的完整检查：gen → 生成物无差异 → lint → test")
	doClean := fset.Bool("clean", false, "删除输出目录")
	if err := fset.Parse(args); err != nil {
		return 2
	}
	root, err := repoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误：", err)
		return 1
	}
	switch {
	case *doClean:
		err = clean(root)
	case *doCheck:
		err = check(root)
	default:
		err = build(root, o)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误：", err)
		return 1
	}
	return 0
}

// repoRoot 从当前目录往上找 go.mod 所在的目录。
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("找不到 go.mod：请在仓库里运行")
		}
		dir = parent
	}
}

// build 按设计文档 O.01 §4.2 的三步构建。
func build(root string, o options) error {
	if err := checkFrontend(root); err != nil {
		return err
	}
	for _, p := range toolProblems(root) {
		fmt.Fprintln(os.Stderr, "警告：", p)
	}
	version, commit, date, verr := Version(root, o.preview)
	if verr != nil {
		fmt.Fprintln(os.Stderr, "警告：", verr)
	}
	if err := copyDist(filepath.Join(root, "web", "dist"), filepath.Join(root, "cmd", "anydoor", "dist")); err != nil {
		return err
	}
	out := o.out
	if !filepath.IsAbs(out) {
		out = filepath.Join(root, out)
	}
	ldflags := fmt.Sprintf("-s -w -X main.version=%s -X main.commit=%s -X main.date=%s", version, commit, date)
	for _, name := range programs {
		bin := filepath.Join(out, name)
		if o.goos == "windows" {
			bin += ".exe"
		}
		cmd := exec.Command("go", "build", "-trimpath", "-ldflags="+ldflags, "-o", bin, "./cmd/"+name)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+o.goos, "GOARCH="+o.goarch)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("构建 %s 失败：%w", name, err)
		}
		fmt.Printf("已构建 %s（%s，%s/%s）\n", bin, version, o.goos, o.goarch)
	}
	return nil
}

// checkFrontend 确认前端已经构建（设计文档 O.01 §4.2 第 ① 步）。
func checkFrontend(root string) error {
	if _, err := os.Stat(filepath.Join(root, "web", "dist", "index.html")); err != nil {
		return errors.New("前端还没有构建：先运行 pnpm -C web build（或 make build）")
	}
	return nil
}

// copyDist 清空 dst（保留占位文件 .gitkeep），再把 src 整个复制进去（设计文档 O.01 §4.2 第 ② 步、§6 第 3 条）。
func copyDist(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(dst)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Name() == placeholder {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	if err := ensurePlaceholder(dst); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(p, target)
	})
}

func ensurePlaceholder(dir string) error {
	p := filepath.Join(dir, placeholder)
	if _, err := os.Stat(p); err == nil {
		return nil
	}
	return os.WriteFile(p, nil, 0o644)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// clean 删除输出目录，并把 cmd/anydoor/dist 恢复成只有占位文件（设计文档 O.01 §4.1）。
func clean(root string) error {
	for _, d := range outputDirs {
		if err := os.RemoveAll(filepath.Join(root, filepath.FromSlash(d))); err != nil {
			return err
		}
	}
	dist := filepath.Join(root, "cmd", "anydoor", "dist")
	if err := os.RemoveAll(dist); err != nil {
		return err
	}
	if err := os.MkdirAll(dist, 0o755); err != nil {
		return err
	}
	if err := ensurePlaceholder(dist); err != nil {
		return err
	}
	fmt.Println("已清理输出目录")
	return nil
}

// check 按设计文档 O.01 §4.1：工具版本 → gen → 生成物与工作区无差异 → lint → test，任何一步失败就停。
func check(root string) error {
	if probs := toolProblems(root); len(probs) > 0 {
		return fmt.Errorf("工具版本不符合要求（O-R2）：\n  %s", strings.Join(probs, "\n  "))
	}
	before, err := worktreeState(root)
	if err != nil {
		return err
	}
	if err := step(root, "生成代码", "go", "run", "./tools/gen"); err != nil {
		return err
	}
	after, err := worktreeState(root)
	if err != nil {
		return err
	}
	if before != after {
		changed, _ := git(root, "status", "--porcelain", "--untracked-files=all")
		return fmt.Errorf("生成物与工作区不一致：重新生成后有文件变了，请提交生成结果：\n%s", changed)
	}
	steps := [][]string{
		{"golangci-lint", "run", "./..."},
		{"go", "run", "./tools/depcheck"},
		{"go", "run", "./tools/lint/quirkscheck"},
		{"go", "run", "./tools/lint/typenamecheck"},
		{"pnpm", "-C", "web", "lint"},
		{"go", "test", "./...", "-count=1"},
		{"pnpm", "-C", "web", "test"},
	}
	for _, s := range steps {
		if err := step(root, strings.Join(s, " "), s[0], s[1:]...); err != nil {
			return err
		}
	}
	fmt.Println("== 检查全部通过 ==")
	return nil
}

func step(root, title, name string, args ...string) error {
	fmt.Printf("== %s ==\n", title)
	cmd := exec.Command(name, args...)
	cmd.Dir = root
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("「%s」失败：%w", title, err)
	}
	return nil
}

// worktreeState 是工作区状态的指纹：文件列表与改动内容都算进去，用来判断 gen 有没有改出新差异。
func worktreeState(root string) (string, error) {
	status, err := git(root, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return "", err
	}
	diff, err := git(root, "diff")
	if err != nil {
		return "", err
	}
	h := sha256.Sum256([]byte(status + "\x00" + diff))
	return hex.EncodeToString(h[:]), nil
}

var numRe = regexp.MustCompile(`\d+`)

// toolProblems 对照固定版本检查本机工具（设计文档 O-R2）；返回不符合的项。
func toolProblems(root string) []string {
	var probs []string
	if want := goToolchain(root); want != "" {
		got, err := output("go", "env", "GOVERSION")
		switch {
		case err != nil:
			probs = append(probs, "go：未安装")
		case got != want:
			probs = append(probs, fmt.Sprintf("go：需要 %s（go.mod 的 toolchain），实际 %s", want, got))
		}
	}
	var pkg struct {
		Engines        map[string]string `json:"engines"`
		PackageManager string            `json:"packageManager"`
	}
	if b, err := os.ReadFile(filepath.Join(root, "web", "package.json")); err == nil && json.Unmarshal(b, &pkg) == nil {
		if want := numRe.FindString(pkg.Engines["node"]); want != "" {
			got, err := output("node", "--version")
			switch {
			case err != nil:
				probs = append(probs, "node：未安装")
			case numRe.FindString(got) != want:
				probs = append(probs, fmt.Sprintf("node：需要 %s.x（web/package.json 的 engines），实际 %s", want, got))
			}
		}
		if want, ok := strings.CutPrefix(pkg.PackageManager, "pnpm@"); ok {
			want, _, _ = strings.Cut(want, "+")
			got, err := output("pnpm", "--version")
			switch {
			case err != nil:
				probs = append(probs, "pnpm：未安装")
			case got != want:
				probs = append(probs, fmt.Sprintf("pnpm：需要 %s（web/package.json 的 packageManager），实际 %s", want, got))
			}
		}
	}
	for tool, want := range pinnedTools(root) {
		got, err := output(tool, "--version")
		switch {
		case err != nil:
			probs = append(probs, tool+"：未安装")
		case !strings.Contains(got, want):
			probs = append(probs, fmt.Sprintf("%s：需要 %s（tools/versions.txt），实际 %s", tool, want, firstLine(got)))
		}
	}
	return probs
}

func goToolchain(root string) string {
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "toolchain "); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// pinnedTools 读 tools/versions.txt：每行"工具名 版本"，# 开头为注释。
func pinnedTools(root string) map[string]string {
	m := map[string]string{}
	b, err := os.ReadFile(filepath.Join(root, "tools", "versions.txt"))
	if err != nil {
		return m
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if f := strings.Fields(line); len(f) == 2 {
			m[f[0]] = f[1]
		}
	}
	return m
}

func output(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return strings.TrimSpace(string(out)), err
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
