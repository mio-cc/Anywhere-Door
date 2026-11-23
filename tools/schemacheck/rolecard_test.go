package main

import (
	"strings"
	"testing"
)

const rolecardsDir = "schema/builtin/rolecards"

var builtinRolecards = []string{"主脑", "智囊", "聚类员", "决策者", "执行者", "验收者", "编排者"}

// vagueWords 是 C.13.02 §4 第三项检查的模糊词。
var vagueWords = []string{"尽量", "适当", "可能的话"}

// rolecardDoc 是角色卡里测试要用到的部分；完整结构由 rolecard.schema.json 校验。
type rolecardDoc struct {
	Name       string   `json:"name"`
	Version    int      `json:"version"`
	Extends    *string  `json:"extends"`
	Identity   string   `json:"identity"`
	Mission    string   `json:"mission"`
	Method     []string `json:"method"`
	GoodOutput string   `json:"good_output"`
	Boundaries []string `json:"boundaries"`
	WhenStuck  string   `json:"when_stuck"`
	Vars       []string `json:"vars"`
}

// texts 是会渲染进提示词的全部正文。
func (d rolecardDoc) texts() []string {
	out := []string{d.Identity, d.Mission, d.GoodOutput, d.WhenStuck}
	out = append(out, d.Method...)
	return append(out, d.Boundaries...)
}

func TestRolecards(t *testing.T) {
	t.Run("C1302-T1 内置角色卡校验", func(t *testing.T) {
		s := repoSet(t)
		files := assetFiles(t, rolecardsDir)
		if !sameSet(files, builtinRolecards) {
			t.Errorf("%s 下是 %v，应为 C.13.02 §5 的 7 张：%v", rolecardsDir, files, builtinRolecards)
		}
		for _, name := range files {
			var d rolecardDoc
			inst := loadAsset(t, rolecardsDir, name, &d)
			if err := s.Validate("rolecard.schema.json", inst); err != nil {
				t.Errorf("%s.yaml 没有通过 rolecard.schema.json：%v", name, err)
			}
			if d.Name != name {
				t.Errorf("%s.yaml：name 是 %q，应与文件名相同", name, d.Name)
			}
			if d.Version != 1 {
				t.Errorf("%s.yaml：version 是 %d，内置角色卡都是 1", name, d.Version)
			}
			if d.Extends != nil {
				t.Errorf("%s.yaml：内置角色卡不用 extends", name)
			}
		}
	})
	t.Run("C1302-T2 内置卡写法", func(t *testing.T) {
		for _, name := range assetFiles(t, rolecardsDir) {
			var d rolecardDoc
			loadAsset(t, rolecardsDir, name, &d)
			if strings.TrimSpace(d.Identity) == "" || strings.TrimSpace(d.Mission) == "" {
				t.Errorf("%s：identity 和 mission 都不能为空", name)
			}
			if len(d.Boundaries) == 0 {
				t.Errorf("%s：boundaries 至少写 1 条", name)
			}
			if d.Vars != nil {
				t.Errorf("%s：内置角色卡不用模板变量（R5），不写 vars", name)
			}
			for _, text := range d.texts() {
				for _, word := range vagueWords {
					if strings.Contains(text, word) {
						t.Errorf("%s：%q 里有模糊词「%s」", name, text, word)
					}
				}
				if strings.Contains(text, "{{") {
					t.Errorf("%s：%q 里有模板变量，内置角色卡不用（R5）", name, text)
				}
			}
		}
	})
	t.Run("C1302-T3 schema 反例", func(t *testing.T) {
		s := repoSet(t)
		const ref = "rolecard.schema.json"
		const head = `"name": "样例", "version": 1, "identity": "你是样例"`
		mustValid(t, s, ref, `{`+head+`, "mission": "完成样例"}`)
		mustValid(t, s, ref, `{`+head+`, "mission": "完成样例", "extends": "基础分析者@2", "vars": ["domain"], "status": "draft"}`)
		mustInvalid(t, s, ref, `{`+head+`}`, "")
		mustInvalid(t, s, ref, `{`+head+`, "mission": "完成样例", "extends": "基础分析者"}`, "/extends")
		mustInvalid(t, s, ref, `{`+head+`, "mission": "完成样例", "vars": ["Goal"]}`, "/vars/0")
		method := `"步骤"` + strings.Repeat(`, "步骤"`, 12)
		mustInvalid(t, s, ref, `{`+head+`, "mission": "完成样例", "method": [`+method+`]}`, "/method")
		mustInvalid(t, s, ref, `{`+head+`, "mission": "完成样例", "status": "deleted"}`, "/status")
	})
}
