package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const typesDir = "schema/builtin/types"

// systemTypes 是 C.13.03 §5.1 的 7 个系统类型 → producer（C.04.01 §4）；整数没有 producer。
var systemTypes = map[string]string{
	"资源引用": "资源", "上下文包": "上下文装配", "工具端点": "工具", "状态": "账本",
	"简报": "子图端口", "升级": "子图端口", "整数": "",
}

// businessTypes 是 C.13.03 §5.2 的 10 个业务类型。
var businessTypes = []string{"方案", "簇标注", "任务卡", "执行结果", "判定", "目标", "差距报告", "聚合结果", "统计", "分歧报告"}

// typeNameRe 是命名类型的写法：名称@主版本[.次版本]（types.schema.json 的 TypeRef 的一种）。
var typeNameRe = regexp.MustCompile(`^[^@\[\]/\s]{1,30}@\d+(\.\d+)?$`)

// typeDoc 是类型声明里测试要用到的部分；完整结构由 typedef.schema.json 校验。
type typeDoc struct {
	Name     string      `json:"name"`
	Major    int         `json:"major"`
	Minor    int         `json:"minor"`
	System   *bool       `json:"system"`
	Producer *string     `json:"producer"`
	Category *string     `json:"category"`
	Scalar   string      `json:"scalar"`
	Fields   []typeField `json:"fields"`
}

type typeField struct {
	Name   string      `json:"name"`
	Type   string      `json:"type"`
	Enum   []string    `json:"enum"`
	Fields []typeField `json:"fields"`
}

// assetFiles 列出 dir（相对仓库根目录）下的 .yaml 文件名，不含扩展名，排好序。C.13 的内置资产都按这种目录放。
func assetFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(repoRoot, filepath.FromSlash(dir)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			t.Errorf("%s 下只放 .yaml 文件：%s", dir, e.Name())
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), ".yaml"))
	}
	sort.Strings(names)
	return names
}

// loadAsset 读 dir 下的 name.yaml：返回给校验器的值，并把内容解到 v。
func loadAsset(t *testing.T, dir, name string, v any) any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(dir), name+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	j, err := yaml.YAMLToJSON(b)
	if err != nil {
		t.Fatalf("%s/%s.yaml 不是合法的 YAML：%v", dir, name, err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(j))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(j, v); err != nil {
		t.Fatalf("%s/%s.yaml：%v", dir, name, err)
	}
	return inst
}

// sameSet：两个列表的元素相同（不看顺序）。
func sameSet(got, want []string) bool {
	a := append([]string(nil), got...)
	b := append([]string(nil), want...)
	sort.Strings(a)
	sort.Strings(b)
	return strings.Join(a, "\x00") == strings.Join(b, "\x00")
}

// loadTypes 读 schema/builtin/types/ 下的全部类型；schema 校验在 C1303-T1。
func loadTypes(t *testing.T) map[string]typeDoc {
	t.Helper()
	out := map[string]typeDoc{}
	for _, name := range assetFiles(t, typesDir) {
		var d typeDoc
		loadAsset(t, typesDir, name, &d)
		out[name] = d
	}
	return out
}

// listElem：s 是 列表[X] 时返回 X。
func listElem(s string) (string, bool) {
	if strings.HasPrefix(s, "列表[") && strings.HasSuffix(s, "]") {
		return strings.TrimSuffix(strings.TrimPrefix(s, "列表["), "]"), true
	}
	return s, false
}

// baseType 去掉所有层的 列表[…]。
func baseType(s string) string {
	for {
		inner, ok := listElem(s)
		if !ok {
			return s
		}
		s = inner
	}
}

// fieldTypeOK：字段 type 的写法（C1303-T3）。
func fieldTypeOK(s string) bool {
	switch s {
	case "string", "integer", "number", "boolean", "object", "任意":
		return true
	}
	if inner, ok := listElem(s); ok {
		return fieldTypeOK(inner)
	}
	return typeNameRe.MatchString(s)
}

// typeDefined：命名类型 ref（名称@主版本[.次版本]）在 types 里有定义，且主版本相同。
func typeDefined(types map[string]typeDoc, ref string) bool {
	name, ver, ok := strings.Cut(ref, "@")
	if !ok {
		return false
	}
	major, _, _ := strings.Cut(ver, ".")
	d, ok := types[name]
	return ok && fmt.Sprint(d.Major) == major
}

// walkTypeFields 依次访问 fs 和嵌套的成员字段；at 是字段路径，如 steps.desc。
func walkTypeFields(fs []typeField, at string, visit func(f typeField, at string)) {
	for _, f := range fs {
		p := f.Name
		if at != "" {
			p = at + "." + f.Name
		}
		visit(f, p)
		walkTypeFields(f.Fields, p, visit)
	}
}

// checkTypeFields 检查同一层的字段（C1303-T3），再进到成员字段。
func checkTypeFields(t *testing.T, typ string, fs []typeField, at string) {
	t.Helper()
	seen := map[string]bool{}
	for _, f := range fs {
		p := f.Name
		if at != "" {
			p = at + "." + f.Name
		}
		if seen[f.Name] {
			t.Errorf("%s：字段 %s 重复", typ, p)
		}
		seen[f.Name] = true
		if !fieldTypeOK(f.Type) {
			t.Errorf("%s：字段 %s 的 type %q 不是 string、integer、number、boolean、object、任意、命名类型或 列表[…]", typ, p, f.Type)
		}
		if len(f.Fields) > 0 && f.Type != "object" && f.Type != "列表[object]" {
			t.Errorf("%s：字段 %s 有成员字段，type 只能是 object 或 列表[object]，实际是 %q", typ, p, f.Type)
		}
		if len(f.Enum) > 0 && f.Type != "string" {
			t.Errorf("%s：字段 %s 写了 enum，type 应为 string，实际是 %q", typ, p, f.Type)
		}
		checkTypeFields(t, typ, f.Fields, p)
	}
}

func TestTypeDefs(t *testing.T) {
	t.Run("C1303-T1 内置类型校验", func(t *testing.T) {
		s := repoSet(t)
		for _, name := range assetFiles(t, typesDir) {
			var d typeDoc
			inst := loadAsset(t, typesDir, name, &d)
			if err := s.Validate("typedef.schema.json", inst); err != nil {
				t.Errorf("%s.yaml 没有通过 typedef.schema.json：%v", name, err)
			}
			if d.Name != name {
				t.Errorf("%s.yaml：name 是 %q，应与文件名相同", name, d.Name)
			}
			if d.Major != 1 || d.Minor != 0 {
				t.Errorf("%s.yaml：major %d、minor %d，内置类型都应是 1、0", name, d.Major, d.Minor)
			}
		}
	})
	t.Run("C1303-T2 名单", func(t *testing.T) {
		types := loadTypes(t)
		all := append(sortedKeys(systemTypes), businessTypes...)
		if got := sortedKeys(types); !sameSet(got, all) {
			t.Fatalf("%s 下是 %v，应为 C.13.03 §5 的 17 个：%v", typesDir, got, all)
		}
		// category_map.yaml 里按名称推出类别的规则（任意 除外）
		var cm map[string]any
		readYAML(t, "schema/builtin/category_map.yaml", &cm)
		rules, _ := cm["rules"].([]any)
		byType := map[string]string{}
		for _, r := range rules {
			m, _ := r.(map[string]any)
			typ, _ := m["type"].(string)
			cat, _ := m["category"].(string)
			if typ != "" && typ != "任意" {
				byType[typ] = cat
			}
		}
		if got, wantCat := sortedKeys(byType), []string{"上下文包", "工具端点", "状态", "资源引用"}; !sameSet(got, wantCat) {
			t.Errorf("category_map.yaml 按名称推出类别的类型是 %v，应为 %v", got, wantCat)
		}
		for _, name := range sortedKeys(types) {
			d := types[name]
			producer, system := systemTypes[name]
			switch {
			case !system:
				if d.System != nil || d.Producer != nil || d.Category != nil {
					t.Errorf("%s 是业务类型，不写 system、producer、category", name)
				}
			case d.System == nil || !*d.System:
				t.Errorf("%s 是系统类型，要写 system: true", name)
			case producer == "":
				if d.Producer != nil || d.Scalar != "integer" {
					t.Errorf("%s 应为 scalar: integer，且没有 producer", name)
				}
			case d.Producer == nil || *d.Producer != producer:
				t.Errorf("%s 的 producer 应为 %s（C.04.01 §4）", name, producer)
			}
			cat, has := byType[name]
			switch {
			case has && (d.Category == nil || *d.Category != cat):
				t.Errorf("%s 要写 category: %s，与 category_map.yaml 一致", name, cat)
			case !has && d.Category != nil:
				t.Errorf("%s 不写 category（按默认 gen）", name)
			}
		}
	})
	t.Run("C1303-T3 字段写法", func(t *testing.T) {
		types := loadTypes(t)
		for _, name := range sortedKeys(types) {
			checkTypeFields(t, name, types[name].Fields, "")
		}
	})
	t.Run("C1303-T4 引用闭合", func(t *testing.T) {
		types := loadTypes(t)
		var ops map[string]string
		readYAML(t, "schema/builtin/aggregate_ops.yaml", &ops)
		for _, op := range sortedKeys(ops) {
			b := baseType(ops[op])
			if strings.HasPrefix(b, "$") {
				continue
			}
			if !typeDefined(types, b) {
				t.Errorf("aggregate_ops.yaml 里 %s 的输出 %s 在 %s 下没有定义", op, ops[op], typesDir)
			}
		}
		for _, name := range sortedKeys(types) {
			walkTypeFields(types[name].Fields, "", func(f typeField, at string) {
				if b := baseType(f.Type); typeNameRe.MatchString(b) && !typeDefined(types, b) {
					t.Errorf("%s：字段 %s 的类型 %s 没有定义", name, at, f.Type)
				}
			})
		}
	})
	t.Run("C1303-T5 schema 反例", func(t *testing.T) {
		s := repoSet(t)
		const ref = "typedef.schema.json"
		const head = `"name": "样例", "major": 1, "minor": 0`
		mustValid(t, s, ref, `{`+head+`, "fields": [{"name": "plan", "type": "string"}]}`)
		mustValid(t, s, ref, `{`+head+`, "scalar": "integer"}`)
		mustInvalid(t, s, ref, `{`+head+`, "scalar": "integer", "fields": []}`, "")
		mustInvalid(t, s, ref, `{`+head+`}`, "")
		mustInvalid(t, s, ref, `{`+head+`, "fields": [{"name": "Plan", "type": "string"}]}`, "/fields/0/name")
		mustInvalid(t, s, ref, `{"name": "方案@1", "major": 1, "minor": 0, "scalar": "string"}`, "/name")
		mustInvalid(t, s, ref, `{`+head+`, "fields": [{"name": "plan", "type": "string", "readonly": true}]}`, "/fields/0")
	})
}
