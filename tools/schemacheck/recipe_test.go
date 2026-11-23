package main

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

const recipesDir = "schema/builtin/recipes"

var builtinRecipes = []string{"主脑配方", "智囊配方", "聚类员配方", "决策者配方", "执行者配方", "验收者配方"}

// recipeSources 是 C.13.01 §5 的来源表：来源名 → 类型。exp 是保留来源（R5），不在表里。
var recipeSources = map[string]string{
	"goal": "目标@1", "task": "任务卡@1", "gap": "差距报告@1", "plans": "列表[方案@1]", "clusters": "簇标注@1",
	"results": "列表[执行结果@1]", "ledger": "状态@1", "brief": "简报@1", "escalation": "升级@1",
}

var (
	inIndexRe         = regexp.MustCompile(`^in\[\d+\]$`)
	fieldsTransformRe = regexp.MustCompile(`^fields\(([a-z_,]+)\)$`)
)

// recipeDoc 是配方里测试要用到的部分；完整结构由 recipe.schema.json 校验。
type recipeDoc struct {
	Name     string            `json:"name"`
	Version  int               `json:"version"`
	Inputs   map[string]string `json:"inputs"`
	Sections []struct {
		Name      string `json:"name"`
		From      string `json:"from"`
		Transform string `json:"transform"`
	} `json:"sections"`
}

func loadRecipes(t *testing.T) map[string]recipeDoc {
	t.Helper()
	out := map[string]recipeDoc{}
	for _, name := range assetFiles(t, recipesDir) {
		var d recipeDoc
		loadAsset(t, recipesDir, name, &d)
		out[name] = d
	}
	return out
}

// sectionSource 把 from 拆成来源名和字段路径（R1、R4）；from 是 in 或 in[i] 时来源名为空；根不是 in 时 ok 为 false。
func sectionSource(from string) (src string, path []string, ok bool) {
	root, rest, dotted := strings.Cut(from, ".")
	switch {
	case !dotted && (root == "in" || inIndexRe.MatchString(root)):
		return "", nil, true
	case root != "in" || rest == "":
		return "", nil, false
	}
	segs := strings.Split(rest, ".")
	return segs[0], segs[1:], true
}

// findField 在同一层字段里找 name。
func findField(fs []typeField, name string) (typeField, bool) {
	for _, f := range fs {
		if f.Name == name {
			return f, true
		}
	}
	return typeField{}, false
}

// resolvePath 从类型 typ（列表取元素类型）沿字段路径逐级往下走，返回最后一级的成员字段（没有成员时为 nil）。
func resolvePath(types map[string]typeDoc, typ string, path []string) ([]typeField, error) {
	b := baseType(typ)
	if !typeDefined(types, b) {
		return nil, fmt.Errorf("类型 %s 没有定义", typ)
	}
	name, _, _ := strings.Cut(b, "@")
	fs, at := types[name].Fields, b
	for _, seg := range path {
		f, ok := findField(fs, seg)
		if !ok {
			return nil, fmt.Errorf("%s 里没有字段 %s", at, seg)
		}
		at += "." + seg
		fs = f.Fields
		if fb := baseType(f.Type); len(fs) == 0 && typeDefined(types, fb) {
			n, _, _ := strings.Cut(fb, "@")
			fs = types[n].Fields
		}
	}
	return fs, nil
}

func TestRecipes(t *testing.T) {
	t.Run("C1301-T1 内置配方校验", func(t *testing.T) {
		s := repoSet(t)
		files := assetFiles(t, recipesDir)
		if !sameSet(files, builtinRecipes) {
			t.Errorf("%s 下是 %v，应为 C.13.01 §5 的 6 个：%v", recipesDir, files, builtinRecipes)
		}
		for _, name := range files {
			var d recipeDoc
			inst := loadAsset(t, recipesDir, name, &d)
			if err := s.Validate("recipe.schema.json", inst); err != nil {
				t.Errorf("%s.yaml 没有通过 recipe.schema.json：%v", name, err)
			}
			if d.Name != name {
				t.Errorf("%s.yaml：name 是 %q，应与文件名相同", name, d.Name)
			}
			if d.Version != 1 {
				t.Errorf("%s.yaml：version 是 %d，内置配方都是 1", name, d.Version)
			}
		}
	})
	t.Run("C1301-T2 schema 反例", func(t *testing.T) {
		s := repoSet(t)
		const ref = "recipe.schema.json"
		const head = `"name": "样例配方", "version": 1`
		const sec = `{"name": "任务", "from": "in"}`
		const tail = `"budget": {"ratio": 0.1}, "on_overflow": "pointer"`
		mustValid(t, s, ref, `{`+head+`, "sections": [`+sec+`], `+tail+`}`)
		mustValid(t, s, ref, `{`+head+`, "sections": [{"name": "经验", "from": "in.exp", "transform": "top_k(3)", "var": "exp_text"}], "exclude": ["type:方案", "path:*.reasoning"], "budget": {"ratio": 0.1, "max_ratio": 0.25}, "on_overflow": "reject"}`)
		mustInvalid(t, s, ref, `{`+head+`, "sections": [{"name": "经验", "from": "in.exp", "transform": "top_k(x)"}], `+tail+`}`, "/sections/0/transform")
		mustInvalid(t, s, ref, `{`+head+`, "sections": [`+sec+`], "budget": {"ratio": 0.1}, "on_overflow": "drop"}`, "/on_overflow")
		mustInvalid(t, s, ref, `{`+head+`, "sections": [`+sec+`], "budget": {"ratio": 0}, "on_overflow": "pointer"}`, "/budget/ratio")
		mustInvalid(t, s, ref, `{`+head+`, "sections": [{"name": "任务", "from": "in", "weight": 1}], `+tail+`}`, "/sections/0")
		mustInvalid(t, s, ref, `{`+head+`, "sections": [`+sec+`], "exclude": ["方案"], `+tail+`}`, "/exclude/0")
	})
	t.Run("C1301-T3 字段路径", func(t *testing.T) {
		types := loadTypes(t)
		recipes := loadRecipes(t)
		for _, name := range sortedKeys(recipes) {
			for i, sec := range recipes[name].Sections {
				src, path, ok := sectionSource(sec.From)
				if !ok {
					t.Errorf("%s 第 %d 段（%s）：from %q 的根只能是 in（R1）", name, i, sec.Name, sec.From)
					continue
				}
				if src == "" || src == "exp" {
					continue // 全部输入，或经验库条目（不查字段）
				}
				typ, ok := recipeSources[src]
				if !ok {
					t.Errorf("%s 第 %d 段（%s）：来源 %s 不在 §5 的来源表里", name, i, sec.Name, src)
					continue
				}
				fs, err := resolvePath(types, typ, path)
				if err != nil {
					t.Errorf("%s 第 %d 段（%s）：%s：%v", name, i, sec.Name, sec.From, err)
					continue
				}
				if m := fieldsTransformRe.FindStringSubmatch(sec.Transform); m != nil {
					for _, f := range strings.Split(m[1], ",") {
						if _, ok := findField(fs, f); !ok {
							t.Errorf("%s 第 %d 段（%s）：fields(…) 里的 %s 不是 %s 的字段", name, i, sec.Name, f, sec.From)
						}
					}
				}
			}
		}
	})
	t.Run("C1301-T4 来源声明", func(t *testing.T) {
		s := repoSet(t)
		types := loadTypes(t)
		recipes := loadRecipes(t)
		for _, name := range sortedKeys(recipes) {
			d := recipes[name]
			for _, sec := range d.Sections {
				if src, _, _ := sectionSource(sec.From); src != "" && src != "exp" {
					if _, ok := d.Inputs[src]; !ok {
						t.Errorf("%s：段「%s」用了来源 %s，inputs 里没有声明（R4）", name, sec.Name, src)
					}
				}
			}
			for _, src := range sortedKeys(d.Inputs) {
				typ := d.Inputs[src]
				if want, ok := recipeSources[src]; !ok || typ != want {
					t.Errorf("%s：inputs.%s 是 %s，§5 来源表里是 %q", name, src, typ, want)
				}
				if !typeDefined(types, baseType(typ)) {
					t.Errorf("%s：inputs.%s 的类型 %s 在 %s 下没有定义", name, src, typ, typesDir)
				}
			}
		}
		// exp 是保留来源，不能在 inputs 里声明（R5）
		mustInvalid(t, s, "recipe.schema.json", `{"name": "样例配方", "version": 1, "inputs": {"exp": "目标@1"}, "sections": [{"name": "经验", "from": "in.exp"}], "budget": {"ratio": 0.1}, "on_overflow": "pointer"}`, "")
	})
}
