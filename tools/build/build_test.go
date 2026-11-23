package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// gitEnv 让测试里的 git 不受本机全局配置影响，并固定作者与提交时间。
func gitEnv(t *testing.T) {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(cfg, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
	t.Setenv("GIT_AUTHOR_DATE", "2026-01-02T03:04:05+00:00")
	t.Setenv("GIT_COMMITTER_DATE", "2026-01-02T03:04:05+00:00")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v：%v\n%s", args, err, out)
	}
}

func commitFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", name)
	runGit(t, dir, "commit", "-q", "-m", "c "+name)
}

func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	commitFile(t, dir, "a.txt", "1")
	return dir
}

func TestVersion(t *testing.T) {
	gitEnv(t)
	const wantDate = "2026-01-02T03:04:05Z"

	t.Run("O01-T1/在标签上", func(t *testing.T) {
		dir := newRepo(t)
		runGit(t, dir, "tag", "v0.1.0")
		v, c, d, err := Version(dir, false)
		if err != nil || v != "0.1.0" || d != wantDate || len(c) != 40 {
			t.Fatalf("得到 %q %q %q %v", v, c, d, err)
		}
		if pv, _, _, _ := Version(dir, true); pv != "0.1.0-preview" {
			t.Fatalf("预览版得到 %q", pv)
		}
	})
	t.Run("O01-T1/标签后3个提交", func(t *testing.T) {
		dir := newRepo(t)
		runGit(t, dir, "tag", "v0.1.0")
		for _, n := range []string{"b.txt", "c.txt", "d.txt"} {
			commitFile(t, dir, n, n)
		}
		if v, _, _, _ := Version(dir, false); v != "0.1.0-dev.3" {
			t.Fatalf("得到 %q", v)
		}
		if v, _, _, _ := Version(dir, true); v != "0.1.0-preview-dev.3" {
			t.Fatalf("预览版得到 %q", v)
		}
	})
	t.Run("O01-T1/有未提交修改", func(t *testing.T) {
		dir := newRepo(t)
		runGit(t, dir, "tag", "v0.1.0")
		if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("2"), 0o644); err != nil {
			t.Fatal(err)
		}
		if v, _, _, _ := Version(dir, false); v != "0.1.0+dirty" {
			t.Fatalf("得到 %q", v)
		}
	})
	t.Run("O01-T1/没有标签", func(t *testing.T) {
		dir := newRepo(t)
		if v, _, _, _ := Version(dir, false); v != "0.0.0-dev" {
			t.Fatalf("得到 %q", v)
		}
	})
	t.Run("O01-T1/不是git仓库", func(t *testing.T) {
		v, c, d, err := Version(t.TempDir(), false)
		if err == nil || v != "0.0.0-dev" || c != unknownCommit || d != unknownDate {
			t.Fatalf("得到 %q %q %q %v", v, c, d, err)
		}
	})
}

var (
	makeTargetRe = regexp.MustCompile(`(?m)^([a-z0-9]+):`)
	taskTargetRe = regexp.MustCompile(`(?m)^  ([a-z0-9]+):\s*$`)
)

func TestTargets(t *testing.T) {
	t.Run("O01-T2 Makefile 与 Taskfile 的目标一致", func(t *testing.T) {
		want := []string{"build", "check", "clean", "dev", "e2e", "gen", "lint", "test"}
		mk, err := os.ReadFile(filepath.Join("..", "..", "Makefile"))
		if err != nil {
			t.Fatal(err)
		}
		tf, err := os.ReadFile(filepath.Join("..", "..", "Taskfile.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if got := names(makeTargetRe, string(mk)); !reflect.DeepEqual(got, want) {
			t.Fatalf("Makefile 的目标：%v", got)
		}
		if got := names(taskTargetRe, string(tf)); !reflect.DeepEqual(got, want) {
			t.Fatalf("Taskfile.yaml 的目标：%v", got)
		}
	})
}

func names(re *regexp.Regexp, s string) []string {
	var out []string
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	sort.Strings(out)
	return out
}

func TestCopyDist(t *testing.T) {
	t.Run("O01-T3 复制前端产物", func(t *testing.T) {
		src, dst := t.TempDir(), t.TempDir()
		files := map[string]string{"index.html": "<html>", "assets/app.js": "js", "assets/img/a.png": "png"}
		for name, body := range files {
			p := filepath.Join(src, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(dst, placeholder), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, "old.js"), []byte("旧文件"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := copyDist(src, dst); err != nil {
			t.Fatal(err)
		}
		for name, body := range files {
			b, err := os.ReadFile(filepath.Join(dst, filepath.FromSlash(name)))
			if err != nil || string(b) != body {
				t.Fatalf("%s：%q %v", name, b, err)
			}
		}
		if _, err := os.Stat(filepath.Join(dst, "old.js")); !os.IsNotExist(err) {
			t.Fatal("旧文件没有被清掉")
		}
		if _, err := os.Stat(filepath.Join(dst, placeholder)); err != nil {
			t.Fatal("占位文件被删了")
		}
	})
}

func TestCheckFrontend(t *testing.T) {
	t.Run("O01-T4 缺前端", func(t *testing.T) {
		root := t.TempDir()
		err := checkFrontend(root)
		if err == nil || !strings.Contains(err.Error(), "前端还没有构建") {
			t.Fatalf("应当报「前端还没有构建」，得到 %v", err)
		}
		if err := os.MkdirAll(filepath.Join(root, "web", "dist"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "web", "dist", "index.html"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := checkFrontend(root); err != nil {
			t.Fatal(err)
		}
	})
}
