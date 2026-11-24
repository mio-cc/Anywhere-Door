package main

import (
	"reflect"
	"strings"
	"testing"
)

const body = "## 叶子编号\nE.14.07\n\n## 文件白名单\n" +
	"- 可新建 / 可修改（实施卡\"要建 / 要改的文件\"）：\n" +
	"  - `engine/exec/aggregate/`\n" +
	"  - `engine/exec/registry.go`\n" +
	"- 可修改：\n" +
	"  - `docs/`\n" +
	"  - `schema/examples/*.json`\n" +
	"- 不许碰（实施卡）：\n" +
	"  - `gen/`\n" +
	"- 总是允许：`RELAY.md`\n\n" +
	"## 截图\n- `web/whatever.png`\n"

func TestCheck(t *testing.T) {
	t.Run("O02-T3 allowlist", func(t *testing.T) {
		l := Parse(body)
		if !l.Found {
			t.Fatal("应当找到\"## 文件白名单\"")
		}
		if want := []string{"engine/exec/aggregate/", "engine/exec/registry.go"}; !reflect.DeepEqual(l.Create, want) {
			t.Errorf("可新建：%v", l.Create)
		}
		if want := []string{"docs/", "schema/examples/*.json", "RELAY.md"}; !reflect.DeepEqual(l.Modify, want) {
			t.Errorf("可修改：%v", l.Modify)
		}
		ok := []string{"engine/exec/aggregate/vote.go", "engine/exec/aggregate/sub/x.go", "engine/exec/registry.go", "RELAY.md", "CHANGELOG.md", "schema/examples/a.json"}
		bad, err := Check("feat/E.14.07-aggregate", l, ok)
		if err != nil || len(bad) != 0 {
			t.Errorf("白名单内的改动应当通过：%v %v", bad, err)
		}
		bad, _ = Check("feat/E.14.07-aggregate", l, []string{"engine/exec/registry.go", "gen/types.go", "web/whatever.png", "docs/x.md"})
		if want := []string{"gen/types.go", "web/whatever.png", "docs/x.md"}; !reflect.DeepEqual(bad, want) {
			t.Errorf("越界的文件：%v，应当是 %v（目录前缀只对\"可新建\"生效；别的章节里的路径不算）", bad, want)
		}
		if bad, err := Check("docs/readme", Parse(""), []string{"anything.go"}); err != nil || bad != nil {
			t.Errorf("非实施分支不检查：%v %v", bad, err)
		}
		if _, err := Check("fix/B.02-start", Parse("## 叶子编号\nB.02\n"), []string{"cmd/anydoor/run.go"}); err == nil || !strings.Contains(err.Error(), "文件白名单") {
			t.Errorf("实施分支没有白名单一节应当报错：%v", err)
		}
	})
}
