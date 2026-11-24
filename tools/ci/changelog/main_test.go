package main

import "testing"

func TestCheck(t *testing.T) {
	t.Run("O02-T5 changelog", func(t *testing.T) {
		if f := Check([]string{"engine/op/apply.go", "docs/x.md"}, nil); f != "engine/op/apply.go" {
			t.Errorf("改了 engine/ 非测试文件且没改 CHANGELOG.md 应当失败，得到 %q", f)
		}
		if f := Check([]string{"engine/op/apply.go", "CHANGELOG.md"}, nil); f != "" {
			t.Errorf("改了 CHANGELOG.md 应当通过：%q", f)
		}
		if f := Check([]string{"engine/op/apply_test.go", "internal/api/testdata/x.json", "web/src/a.test.ts"}, nil); f != "" {
			t.Errorf("只改测试应当通过：%q", f)
		}
		if f := Check([]string{"cmd/anydoor/run.go"}, []string{"ci", " no-changelog"}); f != "" {
			t.Errorf("带 no-changelog 标签应当通过：%q", f)
		}
		if f := Check([]string{"tools/gen/main.go", "README.md", "schema/op.schema.json"}, nil); f != "" {
			t.Errorf("cmd/、engine/、internal/、web/src/ 以外的改动不要求变更日志：%q", f)
		}
		if f := Check([]string{"web/src/app.tsx"}, []string{""}); f != "web/src/app.tsx" {
			t.Errorf("web/src/ 也要变更日志：%q", f)
		}
	})
}
