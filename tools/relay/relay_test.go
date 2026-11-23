package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const sampleTree = "testdata/tree"

func gitEnv(t *testing.T) {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(cfg, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{
		"GIT_CONFIG_GLOBAL": cfg, "GIT_CONFIG_NOSYSTEM": "1",
		"GIT_AUTHOR_NAME": "test", "GIT_AUTHOR_EMAIL": "test@example.com",
		"GIT_COMMITTER_NAME": "test", "GIT_COMMITTER_EMAIL": "test@example.com",
	} {
		t.Setenv(k, v)
	}
}

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v：%v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, dir, rel, body string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// newEnv 建一个设计文档仓库（样例树）和一个代码仓库（main 上有 RELAY.md、go.mod、一个旧测试）。
func newEnv(t *testing.T) (*Env, *bytes.Buffer) {
	t.Helper()
	gitEnv(t)
	docs := t.TempDir()
	err := filepath.WalkDir(sampleTree, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(sampleTree, p)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		write(t, docs, filepath.ToSlash(rel), string(b))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	gitRun(t, docs, "init", "-q", "-b", "main")
	gitRun(t, docs, "add", "-A")
	gitRun(t, docs, "commit", "-q", "-m", "docs")

	repo := t.TempDir()
	gitRun(t, repo, "init", "-q", "-b", "main")
	write(t, repo, ".gitignore", "/.relay/\n")
	write(t, repo, "RELAY.md", "# 接力日志（代码）\n\n> 说明\n\n---\n")
	write(t, repo, "go.mod", "module example.com/x\n\ngo 1.27\n")
	write(t, repo, "pkg/a/old_test.go", "package a\n\nimport \"testing\"\n\nfunc TestOld(t *testing.T) {}\n")
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-q", "-m", "init")

	out := &bytes.Buffer{}
	env := &Env{Repo: repo, Docs: docs, Out: out, By: "测试", Now: func() time.Time {
		return time.Date(2026, 11, 23, 3, 0, 0, 0, time.UTC)
	}}
	return env, out
}

const compliantTest = "package a\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {\n\tt.Run(\"X01-T1 甲\", func(t *testing.T) {})\n\tt.Run(\"X01-T2 乙\", func(t *testing.T) {})\n}\n"

func claimX01(t *testing.T) *Env {
	t.Helper()
	env, _ := newEnv(t)
	if err := env.Start("X.01", "a", false); err != nil {
		t.Fatal(err)
	}
	write(t, env.Repo, "pkg/a/a.go", "package a\n")
	write(t, env.Repo, "pkg/a/a_test.go", compliantTest)
	return env
}

func TestParse(t *testing.T) {
	t.Run("O01-T7 读设计树与实施卡", func(t *testing.T) {
		tr, err := LoadTree(sampleTree)
		if err != nil {
			t.Fatal(err)
		}
		for id, want := range map[string]string{"X.01": "已审", "X.03": "草稿"} {
			if got, _ := tr.State(id); got != want {
				t.Fatalf("%s 的状态：%q", id, got)
			}
		}
		text, _ := tr.Text("X.01")
		card, ok := ParseCard(text)
		if !ok {
			t.Fatal("没有读到实施卡")
		}
		wantCreate := []string{"pkg/a/a.go", "pkg/a/a_test.go", "Makefile", "pkg/gen/**", "go.mod"}
		if !reflect.DeepEqual(card.Create, wantCreate) {
			t.Fatalf("要建的文件：%v", card.Create)
		}
		if !reflect.DeepEqual(card.Forbid, []string{"pkg/b/"}) {
			t.Fatalf("不许碰：%v", card.Forbid)
		}
		if !reflect.DeepEqual(card.Tests, []string{"X01-T1", "X01-T2"}) {
			t.Fatalf("测试用例：%v", card.Tests)
		}
		if !MatchAny("pkg/gen/x/y.go", card.Create) || MatchAny("pkg/b/b.go", card.Create) || !MatchAny("pkg/b/b.go", card.Forbid) {
			t.Fatal("白名单匹配不对")
		}
		if a := tr.Ancestors("X.01"); len(a) != 1 || a[0].ID != "X" {
			t.Fatalf("祖先链：%v", a)
		}
		refs := strings.Join(RefCodes(text), ",")
		if !strings.Contains(refs, "C.01") || !strings.Contains(refs, "X.02") {
			t.Fatalf("引用：%s", refs)
		}
	})
	t.Run("O01-T7/真实写法的实施卡", func(t *testing.T) {
		got := cardPaths("`tools/relay/main.go`、`docs.go`（读设计文档）、`tools/relay/testdata/tree/**`、`.gitignore`（加 `bin/`、`!cmd/x/.gitkeep`）、`P.09` K196、`define: { X }`")
		want := []string{"tools/relay/main.go", "tools/relay/docs.go", "tools/relay/testdata/tree/**", ".gitignore", "bin/", "cmd/x/.gitkeep"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("得到 %v", got)
		}
	})
	t.Run("O01-T7/指纹不受冻结影响", func(t *testing.T) {
		tr, _ := LoadTree(sampleTree)
		text, _ := tr.Text("X.01")
		frozen := strings.Replace(text, "| 状态 | 已审 |", "| 状态 | 冻结 |", 1) + "| 2026-11-24 | 冻结 | 已合并 |\n"
		if NormHash(text) != NormHash(frozen) {
			t.Fatal("只改状态和变更记录，指纹不应该变")
		}
		if NormHash(text) == NormHash(strings.Replace(text, "甲要快", "甲要更快", 1)) {
			t.Fatal("正文改了，指纹应该变")
		}
	})
}

func TestStart(t *testing.T) {
	t.Run("O01-T8 领取", func(t *testing.T) {
		env, _ := newEnv(t)
		if err := env.Start("X.03", "c", false); err == nil || !strings.Contains(err.Error(), "草稿") {
			t.Fatalf("草稿应当拒绝：%v", err)
		}
		write(t, env.Docs, "X/X.01_甲.md", read(t, env.Docs, "X/X.01_甲.md")+"未提交的修改\n")
		if err := env.Start("X.01", "a", false); err == nil || !strings.Contains(err.Error(), "未提交") {
			t.Fatalf("设计文档有未提交修改时应当拒绝：%v", err)
		}
		gitRun(t, env.Docs, "checkout", "--", ".")

		if err := env.Start("X.01", "a", false); err != nil {
			t.Fatal(err)
		}
		if br := gitRun(t, env.Repo, "rev-parse", "--abbrev-ref", "HEAD"); br != "feat/X.01-a" {
			t.Fatalf("分支：%s", br)
		}
		var c Claim
		if err := json.Unmarshal([]byte(read(t, env.Repo, "docs/relay/X.01.json")), &c); err != nil {
			t.Fatal(err)
		}
		if c.DocsCommit != gitRun(t, env.Docs, "rev-parse", "HEAD") || c.Docs["X/X.01_甲.md"] == "" || c.State != "claimed" {
			t.Fatalf("领取记录：%+v", c)
		}
		if !strings.Contains(read(t, env.Repo, "RELAY.md"), "· X.01 · 测试 · 领取") {
			t.Fatal("RELAY.md 里没有领取记录")
		}
		pack := read(t, env.Repo, ".relay/tasks/X.01.md")
		for _, want := range []string{"代码接力的正文", "契约总则的正文", "乙的正文", "X01-T1", "pkg/gen/**"} {
			if !strings.Contains(pack, want) {
				t.Fatalf("任务包里没有 %q", want)
			}
		}
		if strings.Contains(pack, "pkg/b/b.go") {
			t.Fatal("任务包里不应该有别的叶子的实施卡（G.02.02-R3）")
		}
		if _, ok := c.Docs["P/P.09_裁决.md"]; ok || strings.Contains(pack, "裁决记录的正文") {
			t.Fatal("任务包不附 P.09（G.02.02 §4，P.09 K199）")
		}
		if gitRun(t, env.Repo, "status", "--porcelain") != "" {
			t.Fatal("领取后工作区应当是干净的（领取记录已提交）")
		}

		if err := env.Start("X.01", "a", false); err == nil || !strings.Contains(err.Error(), "已被领取") {
			t.Fatalf("重复领取应当拒绝：%v", err)
		}
		if err := env.Start("X.04", "d", false); err == nil || !strings.Contains(err.Error(), "重叠") {
			t.Fatalf("白名单重叠应当拒绝：%v", err)
		}
	})
}

func itemOK(t *testing.T, env *Env, item int) bool {
	t.Helper()
	res, err := env.CheckResults("X.01", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res {
		if r.Item == item {
			return r.OK
		}
	}
	t.Fatalf("没有第 %d 项", item)
	return false
}

func TestCheck(t *testing.T) {
	t.Run("O01-T9/全部合规", func(t *testing.T) {
		env := claimX01(t)
		ok, err := env.Check("X.01", true)
		if err != nil || !ok {
			t.Fatalf("应当通过：%v\n%s", err, env.Out)
		}
	})
	t.Run("O01-T9/①越界", func(t *testing.T) {
		env := claimX01(t)
		write(t, env.Repo, "pkg/c/c.go", "package c\n")
		if itemOK(t, env, 1) {
			t.Fatal("白名单外的文件应当被拦下")
		}
	})
	t.Run("O01-T9/①桩登记后允许", func(t *testing.T) {
		env := claimX01(t)
		write(t, env.Repo, "pkg/b/b.go", "// STUB(X.02): 测试\npackage b\n")
		if itemOK(t, env, 1) {
			t.Fatal("没登记的桩应当被拦下")
		}
		c, _ := env.readClaim("X.01")
		c.Stubs = []string{"pkg/b/b.go"}
		if err := env.writeClaim(c); err != nil {
			t.Fatal(err)
		}
		if !itemOK(t, env, 1) {
			t.Fatal("登记过的桩应当允许")
		}
	})
	t.Run("O01-T9/②生成物", func(t *testing.T) {
		env := claimX01(t)
		write(t, env.Repo, "gen/x.go", "package gen\n")
		if itemOK(t, env, 2) {
			t.Fatal("改 gen/ 应当被拦下")
		}
	})
	t.Run("O01-T9/③依赖", func(t *testing.T) {
		env := claimX01(t)
		write(t, env.Repo, "go.mod", "module example.com/x\n\ngo 1.27\n\nrequire github.com/evil/pkg v1.0.0\n")
		if itemOK(t, env, 3) {
			t.Fatal("未登记的依赖应当被拦下")
		}
		write(t, env.Repo, "go.mod", "module example.com/x\n\ngo 1.27\n\nrequire (\n\tgithub.com/stretchr/testify v1.9.0\n\tgithub.com/davecgh/go-spew v1.1.1 // indirect\n)\n")
		if !itemOK(t, env, 3) {
			t.Fatal("登记过的依赖应当允许，间接依赖不检查")
		}
	})
	t.Run("O01-T9/④删除测试", func(t *testing.T) {
		env := claimX01(t)
		if err := os.Remove(filepath.Join(env.Repo, "pkg", "a", "old_test.go")); err != nil {
			t.Fatal(err)
		}
		if itemOK(t, env, 4) {
			t.Fatal("删除已有测试应当被拦下")
		}
	})
	t.Run("O01-T9/④跳过", func(t *testing.T) {
		env := claimX01(t)
		skip := "\tt." + "Skip(\"以后再说\")\n"
		write(t, env.Repo, "pkg/a/a_test.go", strings.Replace(compliantTest, "func TestA(t *testing.T) {\n", "func TestA(t *testing.T) {\n"+skip, 1))
		if itemOK(t, env, 4) {
			t.Fatal("新增跳过应当被拦下")
		}
	})
	t.Run("O01-T9/⑤缺用例", func(t *testing.T) {
		env := claimX01(t)
		write(t, env.Repo, "pkg/a/a_test.go", strings.Replace(compliantTest, "X01-T2", "其他", 1))
		if itemOK(t, env, 5) {
			t.Fatal("缺测试用例应当被拦下")
		}
	})
	t.Run("O01-T9/⑥依据变了", func(t *testing.T) {
		env := claimX01(t)
		write(t, env.Docs, "X/X.01_甲.md", strings.Replace(read(t, env.Docs, "X/X.01_甲.md"), "甲要快", "甲要更快", 1))
		gitRun(t, env.Docs, "commit", "-q", "-am", "改规则")
		if itemOK(t, env, 6) {
			t.Fatal("依据文档改过应当被拦下")
		}
	})
}

func TestDiffStale(t *testing.T) {
	t.Run("O01-T10 diff", func(t *testing.T) {
		env := claimX01(t)
		out := env.Out.(*bytes.Buffer)
		write(t, env.Docs, "X/X.01_甲.md", strings.Replace(read(t, env.Docs, "X/X.01_甲.md"), "甲要快", "甲要更快", 1))
		gitRun(t, env.Docs, "commit", "-q", "-am", "改规则")
		out.Reset()
		if err := env.Diff("X.01"); err != nil {
			t.Fatal(err)
		}
		if s := out.String(); !strings.Contains(s, "X/X.01_甲.md") || !strings.Contains(s, "## 2. 规则") {
			t.Fatalf("输出：\n%s", s)
		}
		if itemOK(t, env, 6) {
			t.Fatal("依据变了，第 ⑥ 项应当不通过")
		}
		env.AcceptDocs = true
		if err := env.Start("X.01", "a", true); err != nil {
			t.Fatal(err)
		}
		if !itemOK(t, env, 6) {
			t.Fatal("--accept-docs 之后第 ⑥ 项应当通过")
		}
	})
	t.Run("O01-T10 stale", func(t *testing.T) {
		env := claimX01(t)
		out := env.Out.(*bytes.Buffer)
		c, _ := env.readClaim("X.01")
		c.State = "merged"
		if err := env.writeClaim(c); err != nil {
			t.Fatal(err)
		}
		p := "X/X.01_甲.md"
		write(t, env.Docs, p, strings.Replace(read(t, env.Docs, p), "| 状态 | 已审 |", "| 状态 | 冻结 |", 1))
		gitRun(t, env.Docs, "commit", "-q", "-am", "冻结")
		out.Reset()
		if err := env.Stale(); err != nil || !strings.Contains(out.String(), "没有需要复查") {
			t.Fatalf("只冻结不应该算过期：%v\n%s", err, out)
		}
		write(t, env.Docs, p, strings.Replace(read(t, env.Docs, p), "甲要快", "甲要更快", 1))
		gitRun(t, env.Docs, "commit", "-q", "-am", "冻结后又改")
		out.Reset()
		if err := env.Stale(); err != nil || !strings.Contains(out.String(), "X.01") {
			t.Fatalf("冻结后改了正文应当列出：%v\n%s", err, out)
		}
	})
}

func TestStatus(t *testing.T) {
	t.Run("O01-T11 status", func(t *testing.T) {
		env, out := newEnv(t)
		if err := env.Start("X.01", "a", false); err != nil {
			t.Fatal(err)
		}
		gitRun(t, env.Repo, "switch", "-q", "main")
		if err := env.Start("X.02", "b", false); err != nil {
			t.Fatal(err)
		}
		write(t, env.Repo, "pkg/b/b_test.go", "package b\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) {\n\tt.Run(\"X02-T1 乙\", func(t *testing.T) {})\n}\n")
		write(t, env.Repo, "pkg/z/z.go", "// STUB(X.09): 测试用的桩\npackage z\n")
		write(t, env.Repo, "docs/relay/X.01.review.md", "# 评审 · X.01\n\n说明：结论只写符合 / 不符合 / 有疑问。\n\n"+
			"| # | 实施卡 / 小节 | 结论 | 说明 |\n|---|---|---|---|\n"+
			"| 1 | 第 1 条 | 符合 | — |\n"+
			"| 2 | 第 2 条 | 不符合 | 缺少错误处理 |\n"+
			"| 3 | 第 3 条 | 有疑问 | 已处理：见设计文档 |\n")
		out.Reset()
		if err := env.Status(); err != nil {
			t.Fatal(err)
		}
		s := out.String()
		for _, want := range []string{"X.01 甲", "X.02 乙", "测试用例：1/1", "pkg/z/z.go：STUB(X.09)", "X.01.review.md：2 | 第 2 条 | 不符合"} {
			if !strings.Contains(s, want) {
				t.Fatalf("输出里没有 %q：\n%s", want, s)
			}
		}
		for _, notWant := range []string{"实施卡 / 小节 | 结论", "第 1 条 | 符合", "第 3 条", "说明：结论"} {
			if strings.Contains(s, notWant) {
				t.Fatalf("表头、符合的行、已处理的行、说明文字都不应该列出（%q）：\n%s", notWant, s)
			}
		}
		out.Reset()
		if err := env.Next(); err != nil {
			t.Fatal(err)
		}
		if s := out.String(); !strings.Contains(s, "X.04 丁") || strings.Contains(s, "X.01") || strings.Contains(s, "X.03") {
			t.Fatalf("next 的输出：\n%s", s)
		}
	})
}
