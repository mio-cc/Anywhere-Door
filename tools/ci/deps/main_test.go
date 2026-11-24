package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	t.Run("O02-T2 deps", func(t *testing.T) {
		gomod := `module example.com/x

go 1.27

require github.com/single/dep v1.0.0

require (
	github.com/oklog/ulid/v2 v2.1.2
	go.opentelemetry.io/otel/sdk v1.0.0
	github.com/not/listed v0.1.0
	github.com/some/indirect v1.0.0 // indirect
)
`
		gotGo := GoDirect(gomod)
		if want := []string{"github.com/single/dep", "github.com/oklog/ulid/v2", "go.opentelemetry.io/otel/sdk", "github.com/not/listed"}; !reflect.DeepEqual(gotGo, want) {
			t.Errorf("go.mod 的直接依赖：%v，应当是 %v", gotGo, want)
		}
		npm, err := NpmDirect([]byte(`{"dependencies":{"react":"19.0.0"},"devDependencies":{"vite":"8.3.8","leftpad":"1.0.0"}}`))
		if err != nil {
			t.Fatal(err)
		}
		allow := ReadAllow("# Go\ngithub.com/oklog/ulid/v2\n\ngo.opentelemetry.io/otel\ngithub.com/single/dep\n# 前端\nreact\nvite\n")
		bad := Check(append(gotGo, npm...), allow)
		if want := []string{"github.com/not/listed", "leftpad"}; !reflect.DeepEqual(bad, want) {
			t.Errorf("不在清单里的：%v，应当是 %v（// indirect 不查，子路径算登记过）", bad, want)
		}
		if Check([]string{"github.com/oklog/ulid/v2x"}, allow) == nil {
			t.Error("名字只是前缀相同不算登记过")
		}
		// 仓库里真实的 go.mod、web/package.json 和本叶子写的 allow.txt
		bad, err = run(filepath.Join("..", "..", ".."))
		if err != nil {
			t.Fatal(err)
		}
		if len(bad) != 0 {
			t.Errorf("仓库现有的直接依赖都应当在 allow.txt 里：%v", bad)
		}
		// 读不到 web/package.json 时失败，不能悄悄只查 go.mod
		dir := t.TempDir()
		must := func(err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		}
		must(os.MkdirAll(filepath.Join(dir, "tools", "deps"), 0o755))
		must(os.WriteFile(filepath.Join(dir, "tools", "deps", "allow.txt"), []byte("github.com/single/dep\n"), 0o644))
		must(os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n\nrequire github.com/single/dep v1.0.0\n"), 0o644))
		if _, err := run(dir); err == nil || !strings.Contains(err.Error(), "web/package.json") {
			t.Errorf("没有 web/package.json 时应当报错，得到 %v", err)
		}
		must(os.MkdirAll(filepath.Join(dir, "web"), 0o755))
		must(os.WriteFile(filepath.Join(dir, "web", "package.json"), []byte("{"), 0o644))
		if _, err := run(dir); err == nil {
			t.Error("web/package.json 不是合法的 JSON 时应当报错")
		}
	})
}
