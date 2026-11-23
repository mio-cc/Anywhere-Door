package main

// 本包测试共用的辅助函数（C.01 实施卡）。后面的 C 叶子只调用、不修改；需要别的辅助函数写在自己的测试文件里。

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// repoRoot 是仓库根目录（测试在 tools/schemacheck 下运行）。
const repoRoot = "../.."

// repoSet 读仓库的 schema/，有读取问题时测试失败。
func repoSet(t *testing.T) *Set {
	t.Helper()
	s, probs := Load(repoRoot)
	for _, p := range probs {
		t.Error(p)
	}
	if t.Failed() {
		t.FailNow()
	}
	return s
}

// parse 把 JSON 文本读成校验器要的值。
func parse(t *testing.T, text string) any {
	t.Helper()
	v, err := jsonschema.UnmarshalJSON(strings.NewReader(text))
	if err != nil {
		t.Fatalf("测试里的 JSON 写错了：%v\n%s", err, text)
	}
	return v
}

// mustValid：text 按 ref（如 common.schema.json#/$defs/Actor）校验应当通过。
func mustValid(t *testing.T, s *Set, ref, text string) {
	t.Helper()
	if err := s.Validate(ref, parse(t, text)); err != nil {
		t.Errorf("%s 应当接受 %s：%v", ref, text, err)
	}
}

// mustInvalid：text 按 ref 校验应当失败，且每个报错位置都是 at 或在它下面（at 为 "" 表示整个值）。
func mustInvalid(t *testing.T, s *Set, ref, text, at string) {
	t.Helper()
	err := s.Validate(ref, parse(t, text))
	var ve *jsonschema.ValidationError
	switch {
	case err == nil:
		t.Errorf("%s 应当拒绝 %s", ref, text)
	case !errors.As(err, &ve):
		t.Errorf("%s 校验出错（不是校验失败）：%v", ref, err)
	default:
		if out := Outside(ve, at); len(out) > 0 {
			t.Errorf("%s 拒绝 %s 的位置应当在 %q 下，实际还有 %v", ref, text, at, out)
		}
	}
}

// readYAML 读仓库里的 YAML 文件（相对仓库根目录），不认识的键报错。
func readYAML(t *testing.T, rel string, v any) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.UnmarshalWithOptions(b, v, yaml.DisallowUnknownField()); err != nil {
		t.Fatalf("%s：%v", rel, err)
	}
}

// manifestEntry 取示例清单里 file 那一项，并确认它按清单校验没有问题（C.01-R22）。
func manifestEntry(t *testing.T, s *Set, file string) Entry {
	t.Helper()
	entries, err := ReadManifest(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	for i, e := range entries {
		if e.File == file {
			for _, p := range s.checkEntry(e, "") {
				t.Errorf("清单第 %d 项：%s", i, p)
			}
			return e
		}
	}
	t.Fatalf("示例清单里没有 %s", file)
	return Entry{}
}
