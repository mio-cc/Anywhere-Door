// Command relay 是代码接力工具（设计文档 G.02.06、O.01 §4.5）：
//
//	go run ./tools/relay status                  本仓库的接力状态
//	go run ./tools/relay next                    可以领取的叶子
//	go run ./tools/relay start <叶子> [--name 简称] [--by 实施者] [--resume [--accept-docs]]
//	go run ./tools/relay check <叶子> [--no-build]
//	go run ./tools/relay diff <叶子>
//	go run ./tools/relay stale [--accept <叶子>]
//
// 设计文档的位置：环境变量 ANYDOOR_DOCS；没设时为 ../sj/项目树_v0.5（G.02.06-R1）。
// 只用 Go 标准库，通过 os/exec 调用 git。退出码：0 通过，1 不通过，2 用法错误。
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const usage = `用法：
  go run ./tools/relay status
  go run ./tools/relay next
  go run ./tools/relay start <叶子> [--name 简称] [--by 实施者] [--resume [--accept-docs]]
  go run ./tools/relay check <叶子> [--no-build]
  go run ./tools/relay diff <叶子>
  go run ./tools/relay stale [--accept <叶子>]`

// errFailed 表示检查没通过（退出码 1），详细原因已经打印。
var errFailed = errors.New("没有通过")

func main() { os.Exit(run(os.Args[1:], os.Stdout)) }

func run(args []string, out io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, usage)
		return 2
	}
	repo, err := gitOut(".", "rev-parse", "--show-toplevel")
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误：不在 git 仓库里：", err)
		return 1
	}
	docs := os.Getenv("ANYDOOR_DOCS")
	if docs == "" {
		docs = filepath.Join(repo, "..", "sj", "项目树_v0.5")
	}
	env := &Env{Repo: repo, Docs: docs, Out: out, Now: time.Now}
	err = dispatch(env, args)
	switch {
	case err == nil:
		return 0
	case errors.Is(err, errUsage):
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr, usage)
		return 2
	case errors.Is(err, errFailed):
		return 1
	default:
		fmt.Fprintln(os.Stderr, "错误：", err)
		return 1
	}
}

var errUsage = errors.New("用法错误")

func dispatch(env *Env, args []string) error {
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "status":
		return env.Status()
	case "next":
		return env.Next()
	case "stale":
		fset := flag.NewFlagSet("stale", flag.ContinueOnError)
		fset.SetOutput(io.Discard)
		accept := fset.String("accept", "", "复查后代码不用改：把这个冻结叶子的依据更新为当前的设计文档")
		if err := fset.Parse(rest); err != nil || fset.NArg() > 0 {
			return fmt.Errorf("%w：stale 只接受 --accept <叶子>", errUsage)
		}
		return env.Stale(*accept)
	case "start", "check", "diff":
	default:
		return fmt.Errorf("%w：不认识的命令 %q", errUsage, cmd)
	}
	if len(rest) == 0 || strings.HasPrefix(rest[0], "-") {
		return fmt.Errorf("%w：%s 后面要写叶子编号", errUsage, cmd)
	}
	leaf := rest[0]
	fset := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fset.SetOutput(io.Discard)
	name := fset.String("name", "work", "分支名里的简称（feat/<叶子>-<简称>）")
	by := fset.String("by", os.Getenv("ANYDOOR_AGENT"), "实施者")
	resume := fset.Bool("resume", false, "只重新生成任务包")
	noBuild := fset.Bool("no-build", false, "跳过第 ⑦ 项（只用于本地快速自查）")
	acceptDocs := fset.Bool("accept-docs", false, "和 --resume 一起用：读过 relay diff 之后，把依据更新为当前的设计文档")
	if err := fset.Parse(rest[1:]); err != nil {
		return fmt.Errorf("%w：%v", errUsage, err)
	}
	switch cmd {
	case "start":
		env.By = *by
		if *acceptDocs && !*resume {
			return fmt.Errorf("%w：--accept-docs 只能和 --resume 一起用", errUsage)
		}
		env.AcceptDocs = *acceptDocs
		return env.Start(leaf, *name, *resume)
	case "check":
		ok, err := env.Check(leaf, *noBuild)
		if err != nil {
			return err
		}
		if !ok {
			return errFailed
		}
		return nil
	default:
		return env.Diff(leaf)
	}
}

// Env 是一次运行的环境；测试里直接构造它。
type Env struct {
	Repo, Docs string
	Out        io.Writer
	Now        func() time.Time
	By         string
	AcceptDocs bool // start --resume --accept-docs：把依据更新为当前文档（G.02.06-R13）

	tree *Tree
	pErr error
}

func (e *Env) Tree() (*Tree, error) {
	if e.tree == nil {
		t, err := LoadTree(e.Docs)
		if err != nil {
			return nil, err
		}
		e.tree = t
	}
	return e.tree, nil
}

// say 往输出写一行；写失败只记第一个错误（输出是终端或测试缓冲区，失败了也没有别的办法）。
func (e *Env) say(format string, a ...any) {
	if e.pErr == nil {
		_, e.pErr = fmt.Fprintf(e.Out, format+"\n", a...)
	}
}

// Claim 是 docs/relay/<叶子>.json（G.02.06-R6、O.01 §5）。
type Claim struct {
	Leaf       string            `json:"leaf"`
	Branch     string            `json:"branch"`
	ClaimedBy  string            `json:"claimed_by"`
	ClaimedAt  string            `json:"claimed_at"`
	DocsCommit string            `json:"docs_commit"`
	Docs       map[string]string `json:"docs"`
	Stubs      []string          `json:"stubs"`
	State      string            `json:"state"` // claimed / delivered / merged

	foundOn string // 在哪个分支（或工作区）找到的
}

func claimPath(leaf string) string { return "docs/relay/" + leaf + ".json" }

func (e *Env) readClaim(leaf string) (*Claim, error) {
	b, err := os.ReadFile(filepath.Join(e.Repo, filepath.FromSlash(claimPath(leaf))))
	if err != nil {
		return nil, fmt.Errorf("没有 %s：这个叶子还没有领取（先 relay start %s）", claimPath(leaf), leaf)
	}
	var c Claim
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("%s 格式不对：%w", claimPath(leaf), err)
	}
	c.foundOn = "工作区"
	return &c, nil
}

func (e *Env) writeClaim(c *Claim) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	p := filepath.Join(e.Repo, filepath.FromSlash(claimPath(c.Leaf)))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, append(b, '\n'), 0o644)
}

// allClaims 收集工作区和所有本地、远端分支上的领取记录：领取记录提交在各自的分支上，
// 要在别的分支上也看得见，才能判断"已被领取"和白名单重叠（G.02.06-R4、R7）。
func (e *Env) allClaims() (map[string]*Claim, error) {
	claims := map[string]*Claim{}
	rank := map[string]int{"claimed": 1, "delivered": 2, "merged": 3}
	add := func(c *Claim, where string) {
		c.foundOn = where
		if old, ok := claims[c.Leaf]; !ok || rank[c.State] > rank[old.State] {
			claims[c.Leaf] = c
		}
	}
	entries, _ := os.ReadDir(filepath.Join(e.Repo, "docs", "relay"))
	for _, d := range entries {
		if leaf, ok := strings.CutSuffix(d.Name(), ".json"); ok {
			if c, err := e.readClaim(leaf); err == nil {
				add(c, "工作区")
			}
		}
	}
	refs, err := gitOut(e.Repo, "for-each-ref", "--format=%(refname:short)", "refs/heads", "refs/remotes")
	if err != nil {
		return nil, err
	}
	for _, ref := range strings.Fields(refs) {
		names, err := gitOut(e.Repo, "ls-tree", "--name-only", ref, "docs/relay/")
		if err != nil || names == "" {
			continue
		}
		for _, f := range strings.Split(names, "\n") {
			if !strings.HasSuffix(f, ".json") {
				continue
			}
			body, err := gitOut(e.Repo, "show", ref+":"+f)
			if err != nil {
				continue
			}
			var c Claim
			if json.Unmarshal([]byte(body), &c) == nil && c.Leaf != "" {
				add(&c, ref)
			}
		}
	}
	return claims, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// gitOut 在 dir 里执行 git，返回去掉首尾空白的标准输出。路径一律不转义（core.quotepath=false）。
func gitOut(dir string, args ...string) (string, error) {
	full := append([]string{"-C", dir, "-c", "core.quotepath=false"}, args...)
	out, err := exec.Command("git", full...).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", fmt.Errorf("git %s：%s", strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("git %s：%w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func lines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

var cst = time.FixedZone("CST", 8*3600)
