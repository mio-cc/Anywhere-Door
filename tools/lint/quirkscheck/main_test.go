package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQuirkscheck(t *testing.T) {
	t.Run("E1005-T4 quirkscheck", func(t *testing.T) {
		dir := t.TempDir()
		bad := "package x\n\nimport \"strings\"\n\nfunc f(model, name string) bool {\n\tif model == \"gpt-4o\" {\n\t\treturn true\n\t}\n\treturn strings.HasPrefix(name, \"claude\")\n}\n"
		good := "package x\n\nfunc g(p, s string) bool {\n\tswitch p {\n\tcase \"chat_completions\", \"anthropic_messages\":\n\t\treturn true\n\t}\n\treturn s == \"hello\" || p != \"responses\"\n}\n"
		// 测试文件和 quirks/、testdata/ 目录不检查
		ignored := "package x\n\nfunc h(m string) bool { return m == \"gemini-pro\" }\n"
		for name, src := range map[string]string{"bad.go": bad, "good.go": good, "bad_test.go": ignored, "quirks/q.go": ignored, "testdata/t.go": ignored} {
			p := filepath.Join(dir, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		fs, files, err := scan(dir, append(fixedWords, quirksWords([]byte("models:\n  - match: { model: \"zai-glm-*\" }\n"))...))
		if err != nil {
			t.Fatal(err)
		}
		if files != 2 || len(fs) != 2 {
			t.Fatalf("应当检查 2 个文件、报出 2 处：%d 个文件，%v", files, fs)
		}
		if fs[0].Line != 6 || fs[0].Lit != "gpt-4o" || !strings.HasSuffix(fs[0].File, "bad.go") || fs[1].Line != 9 || fs[1].Lit != "claude" {
			t.Errorf("行号或字面量不对：%v", fs)
		}
		if !strings.Contains(fs[0].String(), "bad.go:6:") || !strings.Contains(fs[0].String(), `model == "gpt-4o"`) {
			t.Errorf("报告应当写出文件、行号和那段代码：%s", fs[0])
		}
		if got := quirksWords([]byte("  - match: { model: \"zai-glm-*\" }\n  - match: { protocol: \"*\", model: \"*\" }\n")); len(got) != 1 || got[0] != "zai-glm" {
			t.Errorf("quirks.yaml 的词：%v", got)
		}

		// 在仓库的 engine/llm 上运行不报
		var out, errOut bytes.Buffer
		if code := run(nil, &out, &errOut); code != 0 {
			t.Errorf("仓库的 engine/llm 应当没有问题，退出码 %d：\n%s%s", code, out.String(), errOut.String())
		}
		// 有问题时退出码 1
		out.Reset()
		if code := run([]string{dir}, &out, &errOut); code != 1 || !strings.Contains(out.String(), "gpt-4o") {
			t.Errorf("有问题时退出码应当为 1：%d\n%s", code, out.String())
		}
	})
}
