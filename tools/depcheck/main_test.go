package main

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestRules(t *testing.T) {
	t.Run("G0301-T1 规则文件", func(t *testing.T) {
		ok := "rules:\n  - pkg: engine/**\n    deny: [internal/**]\n    tests: true\n    why: A\n  - pkg: gen/**\n    allow: []\n    why: B\n  - pkg: engine/store/**\n    allow: [gen/**]\n    except: [engine/store/x]\n    why: C\n"
		rs, err := LoadRules([]byte(ok), "r.yaml")
		if err != nil {
			t.Fatal(err)
		}
		if len(rs) != 3 || !rs[0].Tests || rs[0].HasAllow || !rs[1].HasAllow || len(rs[1].Allow) != 0 || !rs[2].HasAllow || rs[2].Except[0] != "engine/store/x" {
			t.Errorf("解析结果不对（allow: [] 和不写 allow 要分开）：%+v", rs)
		}
		bad := []struct{ name, text, want string }{
			{"语法错", "rules:\n  - pkg: [a\n", "r.yaml 第 "},
			{"缺 pkg", "rules:\n  - deny: [a]\n    why: x\n", "第 1 条的 pkg"},
			{"缺 why", "rules:\n  - pkg: a\n    deny: [b]\n", "第 1 条的 why"},
			{"allow 和 deny 都没写", "rules:\n  - pkg: a\n    why: x\n", "第 1 条的 allow / deny"},
			{"不认识的字段", "rules:\n  - pkg: a\n    deny: [b]\n    why: x\n    colour: red\n", "r.yaml"},
			{"空段", "rules:\n  - pkg: a\n    deny: [b]\n    why: x\n  - pkg: a//b\n    deny: [c]\n    why: y\n", "第 2 条的 pkg"},
			{"** 不在末尾", "rules:\n  - pkg: a/**/b\n    deny: [c]\n    why: x\n", "第 1 条的 pkg"},
			{"以 / 开头", "rules:\n  - pkg: a\n    deny: [/b]\n    why: x\n", "第 1 条的 deny"},
			{"单独的 **", "rules:\n  - pkg: '**'\n    deny: [b]\n    why: x\n", "第 1 条的 pkg"},
			{"没有规则", "rules: []\n", "没有规则"},
		}
		for _, tc := range bad {
			_, err := LoadRules([]byte(tc.text), "r.yaml")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("%s：应当报错并含 %q，实际 %v", tc.name, tc.want, err)
			}
		}
		if _, err := LoadRules([]byte("rules:\n  - pkg: a\n    deny: [b]\n    why: x\n    colour: red\n"), "r.yaml"); err == nil || !strings.Contains(err.Error(), " 行") {
			t.Errorf("不认识的字段应当带行号：%v", err)
		}
	})
}

func TestMatch(t *testing.T) {
	t.Run("G0301-T2 模式匹配", func(t *testing.T) {
		cases := []struct {
			pattern, path string
			want          bool
		}{
			{"a/**", "a", true},
			{"a/**", "a/b", true},
			{"a/**", "a/b/c", true},
			{"a/**", "ab", false},
			{"a/**", "ab/c", false},
			{"engine/llm", "engine/llm", true},
			{"engine/llm", "engine/llm/quirks", false},
			{"net/http", "net/http", true},
			{"net/http", "net/http/httptest", false},
			{"net/http/**", "net/http/httptest", true},
			{"net/http/**", "net/http", true},
		}
		for _, c := range cases {
			if got := match(c.pattern, c.path); got != c.want {
				t.Errorf("match(%q, %q) = %v，想要 %v", c.pattern, c.path, got, c.want)
			}
		}
	})
}

// writeTree 在 dir 下按相对路径写文件。
func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, s := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func goFile(pkg string, imports ...string) string {
	var b strings.Builder
	b.WriteString("package " + pkg + "\n\n")
	for _, im := range imports {
		b.WriteString("import \"" + im + "\"\n")
	}
	return b.String()
}

const testRules = `rules:
  - pkg: engine/**
    deny: [internal/**, cmd/**]
    tests: true
    why: 规则A
  - pkg: engine/**
    deny: [net/http, net/http/**]
    except: [engine/llm/**]
    why: 规则B
  - pkg: engine/store/**
    allow: [gen/**, engine/store/**]
    why: 规则C
  - pkg: internal/platform/**
    allow: [gen/**, internal/platform/**]
    why: 规则D
`

func TestCheck(t *testing.T) {
	t.Run("G0301-T3 违规检出", func(t *testing.T) {
		dir := t.TempDir()
		m := "example.com/m/"
		writeTree(t, dir, map[string]string{
			"go.mod":                   "module example.com/m\n\ngo 1.27\n",
			"rules.yaml":               testRules,
			"bad.yaml":                 "rules:\n  - pkg: a\n    why: x\n",
			"engine/x/x.go":            goFile("x", m+"internal/y"),
			"engine/x/x_test.go":       goFile("x", m+"internal/y"),
			"engine/store/s.go":        goFile("store", m+"engine/graph", m+"gen", "github.com/goccy/go-yaml"),
			"engine/store/s_test.go":   goFile("store", "net/http/httptest", m+"engine/graph"),
			"engine/blob/b.go":         goFile("blob", "net/http"),
			"engine/llm/l.go":          goFile("llm", "net/http"),
			"engine/graph/g.go":        goFile("graph"),
			"internal/platform/a/a.go": goFile("a", m+"internal/api", m+"internal/platform/b"),
			"internal/platform/b/b.go": goFile("b"),
			"internal/api/api.go":      goFile("api"),
			"internal/y/y.go":          goFile("y"),
			"cmd/tool/main.go":         goFile("main", m+"internal/y", m+"engine/x"),
			"engine/testdata/bad/t.go": goFile("bad", m+"internal/y"),
			"engine/_x/t.go":           goFile("x", m+"internal/y"),
			"engine/.x/t.go":           goFile("x", m+"internal/y"),
			"engine/nested/go.mod":     "module example.com/nested\n",
			"engine/nested/n.go":       goFile("n", m+"internal/y"),
			"engine/nested/sub/n.go":   goFile("sub", m+"internal/y"),
		})
		rs, err := LoadRules([]byte(testRules), "rules.yaml")
		if err != nil {
			t.Fatal(err)
		}
		pkgs, err := Scan(dir)
		if err != nil {
			t.Fatal(err)
		}
		vs := Check(pkgs, rs, "example.com/m")
		var got []string
		for _, v := range vs {
			got = append(got, v.Format("rules.yaml"))
		}
		want := []string{
			"engine/blob/b.go:3: engine/blob import net/http：规则B（rules.yaml 第 2 条）",
			"engine/store/s.go:3: engine/store import engine/graph：规则C（rules.yaml 第 3 条）",
			"engine/x/x.go:3: engine/x import internal/y：规则A（rules.yaml 第 1 条）",
			"engine/x/x_test.go:3: engine/x import internal/y：规则A（rules.yaml 第 1 条）",
			"internal/platform/a/a.go:3: internal/platform/a import internal/api：规则D（rules.yaml 第 4 条）",
		}
		sort.Strings(got)
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("违规清单不对：\n实际：\n%s\n想要：\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
		}

		var out, errOut bytes.Buffer
		if code := run([]string{"-rules", filepath.Join(dir, "rules.yaml"), dir}, &out, &errOut); code != 1 || !strings.Contains(out.String(), "engine/x/x.go:3:") || !strings.Contains(out.String(), "5 处") {
			t.Errorf("有违规时退出码应当为 1：%d\n%s%s", code, out.String(), errOut.String())
		}
		out.Reset()
		errOut.Reset()
		if code := run([]string{"-rules", filepath.Join(dir, "bad.yaml"), dir}, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "bad.yaml") {
			t.Errorf("规则文件写错时退出码应当为 2：%d\n%s%s", code, out.String(), errOut.String())
		}
		if code := run([]string{"-rules", filepath.Join(dir, "missing.yaml"), dir}, &out, &errOut); code != 2 {
			t.Errorf("规则文件不存在时退出码应当为 2：%d", code)
		}
	})
}

func TestRepo(t *testing.T) {
	t.Run("G0301-T4 仓库自检", func(t *testing.T) {
		var out, errOut bytes.Buffer
		if code := run(nil, &out, &errOut); code != 0 {
			t.Fatalf("仓库自己应当没有违规，退出码 %d：\n%s%s", code, out.String(), errOut.String())
		}
		if !strings.Contains(out.String(), "没有违规") {
			t.Errorf("输出：%s", out.String())
		}
		root, err := findRoot()
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(root, "tools", "depcheck", "rules.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		rs, err := LoadRules(b, "rules.yaml")
		if err != nil {
			t.Fatal(err)
		}
		// G.03.01 §5.3 的表，按 rules.yaml 的顺序逐行比较约束字段（why 只要求非空，LoadRules 已检查）
		type row struct {
			pkg                 string
			allow, deny, except []string
			hasAllow, tests     bool
		}
		a := func(ps ...string) []string { return ps }
		want := []row{
			{pkg: "engine/**", deny: a("internal/**", "cmd/**"), tests: true},
			{pkg: "engine/**", deny: a("net/http", "net/http/**"), except: a("engine/llm/**", "engine/tools/builtin/http/**", "engine/tools/builtin/fetch/**", "engine/mcp/**")},
			{pkg: "engine/**", hasAllow: true, allow: a("gen/**", "engine/**")},
			{pkg: "engine/graph/**", hasAllow: true, allow: a("gen/**", "engine/graph/**")},
			{pkg: "engine/schema/**", hasAllow: true, allow: a("gen/**", "engine/graph/**", "engine/schema/**")},
			{pkg: "engine/op/**", hasAllow: true, allow: a("gen/**", "engine/graph/**", "engine/schema/**", "engine/authz/**", "engine/op/**")},
			{pkg: "engine/authz/**", hasAllow: true, allow: a("gen/**", "engine/graph/**", "engine/authz/**")},
			{pkg: "engine/store/**", hasAllow: true, allow: a("gen/**", "engine/store/**")},
			{pkg: "engine/blob/**", hasAllow: true, allow: a("engine/blob/**")},
			{pkg: "engine/obs/**", hasAllow: true, allow: a("gen/**", "engine/obs/**")},
			{pkg: "engine/llm/**", hasAllow: true, allow: a("gen/**", "engine/obs/**", "engine/llm/**")},
			{pkg: "engine/tools/**", hasAllow: true, allow: a("gen/**", "engine/blob/**", "engine/obs/**", "engine/authz/**", "engine/tools/**")},
			{pkg: "engine/mcp/**", hasAllow: true, allow: a("gen/**", "engine/tools/**", "engine/mcp/**")},
			{pkg: "engine/ctxasm/**", hasAllow: true, allow: a("gen/**", "engine/schema/**", "engine/blob/**", "engine/llm", "engine/ctxasm/**")},
			{pkg: "engine/exec/**", hasAllow: true, allow: a("gen/**", "engine/graph/**", "engine/schema/**", "engine/ctxasm/**", "engine/llm", "engine/tools", "engine/obs/**", "engine/blob/**", "engine/exec/**")},
			{pkg: "engine/sched/**", hasAllow: true, allow: a("gen/**", "engine/graph/**", "engine/obs/**", "engine/exec", "engine/sched/**")},
			{pkg: "engine/run/**", deny: a("engine/orch/**")},
			{pkg: "engine/orch/**", hasAllow: true, allow: a("gen/**", "engine/graph/**", "engine/op/**", "engine/schema/**", "engine/authz/**", "engine/tools/**", "engine/exec", "engine/orch/**")},
			{pkg: "internal/platform/**", hasAllow: true, allow: a("gen/**", "internal/platform/**")},
			{pkg: "internal/app/**", hasAllow: true, allow: a("gen/**", "engine/**", "internal/platform/**", "internal/app/**")},
			{pkg: "internal/sse/**", hasAllow: true, allow: a("gen/**", "engine/obs/**", "engine/store/**", "internal/sse/**")},
			{pkg: "internal/api/**", hasAllow: true, allow: a("gen/**", "engine", "internal/app/**", "internal/sse/**", "internal/platform/auth", "schema/builtin/messages", "internal/api/**")},
			{pkg: "gen/**", hasAllow: true, allow: a()},
			{pkg: "schema/**", hasAllow: true, allow: a()},
		}
		norm := func(s []string) string { return strings.Join(s, ",") }
		if len(rs) != len(want) {
			t.Errorf("rules.yaml 有 %d 条规则，§5.3 的表有 %d 行", len(rs), len(want))
		}
		for i := 0; i < len(rs) && i < len(want); i++ {
			r, w := rs[i], want[i]
			if r.Pkg != w.pkg || r.HasAllow != w.hasAllow || r.Tests != w.tests || norm(r.Allow) != norm(w.allow) || norm(r.Deny) != norm(w.deny) || norm(r.Except) != norm(w.except) {
				t.Errorf("rules.yaml 第 %d 条和 §5.3 的表不一致：\n实际 pkg=%s allow(%v)=%v deny=%v except=%v tests=%v\n想要 pkg=%s allow(%v)=%v deny=%v except=%v tests=%v",
					i+1, r.Pkg, r.HasAllow, r.Allow, r.Deny, r.Except, r.Tests, w.pkg, w.hasAllow, w.allow, w.deny, w.except, w.tests)
			}
			if strings.TrimSpace(r.Why) == "" {
				t.Errorf("rules.yaml 第 %d 条没有 why", i+1)
			}
		}
	})
}
