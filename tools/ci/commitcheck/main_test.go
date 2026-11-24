package main

import (
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	t.Run("O02-T1 commitcheck", func(t *testing.T) {
		ok := []Commit{
			{ID: "aaaaaaaa1", Subject: "feat(E.14.07): 实现聚合执行器", Body: "为什么这样做。\n\nRefs: E.14.07\n"},
			{ID: "aaaaaaaa2", Subject: "docs(repo): 改说明", Body: ""},
			{ID: "aaaaaaaa3", Subject: "fix(B.02): 修启动", Body: "Refs: B.02, B.07.05"},
			{ID: "aaaaaaaa4", Subject: "chore(deps): 升级", Body: ""},
			{ID: "aaaaaaaa5", Subject: "fix(C.01): 修文案", Body: "Refs: 反馈/2026-11-24-文案.md"},
		}
		if p := Check("feat(O.02): CI 门禁", ok); len(p) != 0 {
			t.Errorf("合规的标题和提交应当通过：%v", p)
		}
		cases := []struct {
			name string
			c    Commit
			want string
		}{
			{"类型错", Commit{ID: "b1", Subject: "feature(E.14.07): x"}, "类型"},
			{"范围不是编号", Commit{ID: "b2", Subject: "docs(m0-store): x"}, "范围"},
			{"feat 缺 Refs", Commit{ID: "b3", Subject: "feat(B.02): x", Body: "只有说明"}, "Refs"},
			{"Refs 不是编号", Commit{ID: "b4", Subject: "fix(B.02): x", Body: "Refs: 第 2 片"}, "不是树里的编号"},
			{"首行格式", Commit{ID: "b5", Subject: "随便写的"}, "不是 <类型>"},
		}
		for _, c := range cases {
			p := Check("", []Commit{c.c})
			if len(p) != 1 || !strings.Contains(p[0].Msg, c.want) || !strings.Contains(p[0].Where, c.c.ID) {
				t.Errorf("%s：应当有一处含 %q 的问题，得到 %v", c.name, c.want, p)
			}
		}
		if p := Check("修了点东西", nil); len(p) != 1 || p[0].Where != "PR 标题" {
			t.Errorf("不合规的 PR 标题应当被指出：%v", p)
		}
	})
}
