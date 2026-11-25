package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const bad = `package p

func f(n struct{ Type string }, def struct{ Name string }) {
	if n.Type == "循环" {
	}
	switch def.Name {
	case "Agent":
	}
	if n.Type == gen.NodeTypeLoop {
	}
}
`

const good = `package p

import "strings"

func g(p, cat, proto, s string) bool {
	if p == "max_rounds" || cat == "边界" || proto == "chat_completions" || s == "你好" {
		return true
	}
	switch p {
	case "role", "port_name":
	}
	return strings.HasPrefix(s, "input:") && s != gen.CategoryBoundary
}
`

func write(t *testing.T, path, src string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTypenamecheck(t *testing.T) {
	t.Run("G050501-T1 违规检出", func(t *testing.T) {
		dir := t.TempDir()
		write(t, filepath.Join(dir, "bad.go"), bad)
		write(t, filepath.Join(dir, "bad_test.go"), bad)
		write(t, filepath.Join(dir, "testdata", "bad.go"), bad)
		var out, errb bytes.Buffer
		if code := run([]string{dir}, &out, &errb); code != 1 {
			t.Fatalf("退出码 %d，应为 1：%s%s", code, out.String(), errb.String())
		}
		var hits []string
		for _, l := range strings.Split(out.String(), "\n") {
			if strings.Contains(l, "比较了节点类型名") {
				hits = append(hits, l)
			}
		}
		if len(hits) != 3 {
			t.Fatalf("应当报 3 处（测试文件、testdata 里的不报）：\n%s", out.String())
		}
		for i, want := range []string{"bad.go:4: 比较了节点类型名 循环", "bad.go:7: 比较了节点类型名 Agent", "bad.go:9: 比较了节点类型名 gen.NodeTypeLoop"} {
			if !strings.Contains(hits[i], want) {
				t.Errorf("第 %d 处：%q，应当含 %q", i+1, hits[i], want)
			}
		}
	})
	t.Run("G050501-T2 不误报", func(t *testing.T) {
		dir := t.TempDir()
		write(t, filepath.Join(dir, "good.go"), good)
		var out, errb bytes.Buffer
		if code := run([]string{dir}, &out, &errb); code != 0 {
			t.Fatalf("退出码 %d，应为 0：%s%s", code, out.String(), errb.String())
		}
	})
	t.Run("G050501-T3 仓库自检", func(t *testing.T) {
		var out, errb bytes.Buffer
		if code := run(nil, &out, &errb); code != 0 {
			t.Fatalf("仓库自检退出码 %d：%s%s", code, out.String(), errb.String())
		}
		if !strings.Contains(out.String(), "没有节点类型名的比较") {
			t.Errorf("输出：%s", out.String())
		}
	})
	t.Run("解析失败退出码 2", func(t *testing.T) {
		dir := t.TempDir()
		write(t, filepath.Join(dir, "broken.go"), "package p\nfunc {")
		var out, errb bytes.Buffer
		if code := run([]string{dir}, &out, &errb); code != 2 {
			t.Fatalf("退出码 %d，应为 2", code)
		}
	})
}
