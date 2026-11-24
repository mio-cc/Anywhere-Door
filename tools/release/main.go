// Command release 打预览版发布包（设计文档 O.03）：
//
//	go run ./tools/release [-version-check] [-only os/arch] [-out dist] [-prev-sizes <文件>]   交叉编译、检查、打包、校验和
//	go run ./tools/release -smoke <anydoor 程序>                                              只做启动冒烟（O.03 §6.4）
//
// 逐个平台调用 go run ./tools/build -preview（O.01），不自己编译。只用标准库。
package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// root 是仓库根（LICENSE、NOTICE、CHANGELOG.md、go.sum、cmd/anydoor/dist 从这里读）；测试里替换。
var root = "."

// Target 是一个目标平台。
type Target struct{ OS, Arch string }

func (t Target) String() string { return t.OS + "/" + t.Arch }

// Ext 是归档的扩展名：Windows 用 zip，其他用 tar.gz（O.03 §4）。
func (t Target) Ext() string {
	if t.OS == "windows" {
		return ".zip"
	}
	return ".tar.gz"
}

// Exe 是程序在这个平台上的文件名。
func (t Target) Exe(name string) string {
	if t.OS == "windows" {
		return name + ".exe"
	}
	return name
}

// Targets 是 O.03 §4 的六个平台。
func Targets() []Target {
	return []Target{
		{"windows", "amd64"}, {"windows", "arm64"},
		{"darwin", "arm64"}, {"darwin", "amd64"},
		{"linux", "amd64"}, {"linux", "arm64"},
	}
}

func warnf(format string, args ...any) { fmt.Fprintf(os.Stderr, "警告："+format+"\n", args...) }

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	fset := flag.NewFlagSet("release", flag.ContinueOnError)
	versionCheck := fset.Bool("version-check", false, "当前提交必须正好在一个 v* 标签上、工作区干净")
	only := fset.String("only", "", "只打一个平台，形如 linux/amd64")
	out := fset.String("out", "dist", "输出目录")
	smoke := fset.String("smoke", "", "只对这个 anydoor 程序做启动冒烟")
	prev := fset.String("prev-sizes", "", "上一个版本的 sizes.txt；有就比较大小")
	if err := fset.Parse(args); err != nil {
		return 2
	}
	if *smoke != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		if err := Smoke(ctx, *smoke); err != nil {
			fmt.Fprintln(os.Stderr, "冒烟失败：", err)
			return 1
		}
		fmt.Println("冒烟通过：", *smoke)
		return 0
	}
	if *versionCheck {
		if err := checkVersion(root); err != nil {
			fmt.Fprintln(os.Stderr, "错误：", err)
			return 1
		}
	}
	if err := release(*out, *only, *prev); err != nil {
		fmt.Fprintln(os.Stderr, "错误：", err)
		return 1
	}
	return 0
}

// checkVersion：HEAD 正好在一个 v* 标签上，工作区干净（O.03 §6.1 第 1 步）。
func checkVersion(dir string) error {
	tags, err := gitOut(dir, "tag", "--points-at", "HEAD", "--list", "v*")
	if err != nil {
		return err
	}
	if n := len(strings.Fields(tags)); n != 1 {
		return fmt.Errorf("当前提交上有 %d 个 v* 标签，应当正好一个（发布只能在标签上，G.07-R8）", n)
	}
	if st, err := gitOut(dir, "status", "--porcelain"); err != nil {
		return err
	} else if st != "" {
		return errors.New("工作区有未提交的改动，不能发布")
	}
	return nil
}

func gitOut(dir string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		return "", fmt.Errorf("git %s：%w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Dir = root
	cmd.Stderr = os.Stderr
	return cmd
}

var builtRe = regexp.MustCompile(`已构建 .+（([^，]+)，`)

// release 是 O.03 §6.1 第 2–5 步。
func release(out, only, prevSizes string) error {
	targets := Targets()
	if only != "" {
		targets = nil
		for _, t := range Targets() {
			if t.String() == only {
				targets = []Target{t}
			}
		}
		if targets == nil {
			return fmt.Errorf("-only %s 不在六个平台里", only)
		}
	}
	ct, err := gitOut(root, "show", "-s", "--format=%ct", "HEAD")
	if err != nil {
		return err
	}
	sec, err := strconv.ParseInt(ct, 10, 64)
	if err != nil {
		return fmt.Errorf("提交时间格式不对：%q", ct)
	}
	mtime := time.Unix(sec, 0).UTC()
	if err := os.RemoveAll(out); err != nil {
		return err
	}
	pnpm := command("pnpm", "-C", "web", "build")
	pnpm.Stdout = os.Stdout
	if err := pnpm.Run(); err != nil {
		return fmt.Errorf("前端构建失败：%w", err)
	}
	version := ""
	sizes := map[string]int64{}
	var archives []string
	for _, t := range targets {
		dir := filepath.Join(out, "tmp", t.OS+"-"+t.Arch)
		b, err := command("go", "run", "./tools/build", "-os", t.OS, "-arch", t.Arch, "-out", dir, "-preview").Output()
		fmt.Print(string(b))
		if err != nil {
			return fmt.Errorf("%s 编译失败：%w", t, err)
		}
		m := builtRe.FindStringSubmatch(string(b))
		if m == nil {
			return fmt.Errorf("%s：tools/build 的输出里找不到版本号", t)
		}
		if version != "" && m[1] != version {
			return fmt.Errorf("%s 的版本号 %s 和前面的 %s 不同", t, m[1], version)
		}
		version = m[1]
		bin := filepath.Join(dir, t.Exe("anydoor"))
		if err := Inspect(bin, t); err != nil {
			return fmt.Errorf("%s：%w", t, err)
		}
		if fi, err := os.Stat(bin); err == nil {
			sizes[t.String()] = fi.Size()
		}
		p, err := Archive(out, t, version, mtime)
		if err != nil {
			return fmt.Errorf("%s 打包失败：%w", t, err)
		}
		archives = append(archives, p)
		fmt.Println("已打包", p)
	}
	if err := writeSums(out, archives); err != nil {
		return err
	}
	if err := writeSizes(out, sizes, prevSizes); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(out, "tmp"))
}

// writeSums 写 SHA256SUMS.txt：每个归档一行"<哈希>  <文件名>"，按文件名排序。
func writeSums(out string, archives []string) error {
	sort.Strings(archives)
	var b strings.Builder
	for _, p := range archives {
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		_ = f.Close()
		if err != nil {
			return err
		}
		fmt.Fprintf(&b, "%s  %s\n", hex.EncodeToString(h.Sum(nil)), filepath.Base(p))
	}
	return os.WriteFile(filepath.Join(out, "SHA256SUMS.txt"), []byte(b.String()), 0o644)
}

// writeSizes 写 sizes.txt（"<平台> <字节数>"），和上一个版本相比增长超过 20% 时打印警告（不失败，O.03 §6.3）。
func writeSizes(out string, sizes map[string]int64, prev string) error {
	old := map[string]int64{}
	if prev != "" {
		if f, err := os.Open(prev); err == nil {
			sc := bufio.NewScanner(f)
			for sc.Scan() {
				if fs := strings.Fields(sc.Text()); len(fs) == 2 {
					if n, err := strconv.ParseInt(fs[1], 10, 64); err == nil {
						old[fs[0]] = n
					}
				}
			}
			_ = f.Close()
		} else {
			warnf("读不了上一个版本的大小 %s：%v", prev, err)
		}
	}
	keys := make([]string, 0, len(sizes))
	for k := range sizes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s %d\n", k, sizes[k])
		if o := old[k]; o > 0 && float64(sizes[k]) > float64(o)*1.2 {
			warnf("%s 的 anydoor 从 %d 字节涨到 %d 字节，超过 20%%", k, o, sizes[k])
		}
	}
	return os.WriteFile(filepath.Join(out, "sizes.txt"), []byte(b.String()), 0o644)
}
