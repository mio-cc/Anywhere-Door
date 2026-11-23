package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	draft2020    = "https://json-schema.org/draft/2020-12/schema"
	baseURL      = "https://anydoor.local/schema/" // C.01-R2
	schemaDir    = "schema"
	exampleDir   = "schema/examples"
	manifestName = "manifest.yaml"
)

// Problem 是一条检查结果：哪个文件、文件里的哪个位置（JSON Pointer）、违反哪条规则。
type Problem struct {
	File string // 相对仓库根目录，用 /
	At   string // 文件里的位置；整个文件时为空
	Rule string // 规则编号，如 C.01-R4
	Msg  string
}

func (p Problem) String() string {
	loc := p.File
	if p.At != "" {
		loc += " " + p.At
	}
	return loc + "：" + p.Rule + "：" + p.Msg
}

// Set 是 schema/ 下的全部 schema 文件。
type Set struct {
	Root  string                    // 仓库根目录
	Files []string                  // 文件名（不含目录），排好序
	Raw   map[string]map[string]any // encoding/json 读出的原文，用来做写法和命名检查

	compiler *jsonschema.Compiler
	compiled map[string]*jsonschema.Schema
}

// Load 读 root/schema/*.schema.json。读不了、不是 JSON 对象的文件记为问题，不进 Set。
func Load(root string) (*Set, []Problem) {
	s := &Set{Root: root, Raw: map[string]map[string]any{}, compiled: map[string]*jsonschema.Schema{}}
	s.compiler = jsonschema.NewCompiler()
	s.compiler.DefaultDraft(jsonschema.Draft2020)
	s.compiler.AssertFormat()
	var probs []Problem
	paths, _ := filepath.Glob(filepath.Join(root, schemaDir, "*.schema.json"))
	sort.Strings(paths)
	for _, p := range paths {
		name := filepath.Base(p)
		file := schemaDir + "/" + name
		b, err := os.ReadFile(p)
		if err != nil {
			probs = append(probs, Problem{File: file, Rule: "C.01-R1", Msg: "读不了：" + err.Error()})
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal(b, &raw); err != nil {
			probs = append(probs, Problem{File: file, Rule: "C.01-R1", Msg: "不是合法的 JSON 对象：" + err.Error()})
			continue
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
		if err != nil {
			probs = append(probs, Problem{File: file, Rule: "C.01-R1", Msg: "不是合法的 JSON：" + err.Error()})
			continue
		}
		s.Files = append(s.Files, name)
		s.Raw[name] = raw
		// 一律按 baseURL + 文件名登记：跨文件引用只从本地 schema/ 解析，不联网（C.01 §6）
		if err := s.compiler.AddResource(baseURL+name, doc); err != nil {
			probs = append(probs, Problem{File: file, Rule: "C.01-R1", Msg: "登记失败：" + err.Error()})
		}
	}
	return s, probs
}

// Validate 按 ref（文件名，可以带 #/$defs/…）校验一个已经用 jsonschema.UnmarshalJSON 读出的值。
func (s *Set) Validate(ref string, inst any) error {
	sch, ok := s.compiled[ref]
	if !ok {
		var err error
		if sch, err = s.compiler.Compile(baseURL + ref); err != nil {
			return fmt.Errorf("schema %s 编译失败：%w", ref, err)
		}
		s.compiled[ref] = sch
	}
	return sch.Validate(inst)
}

// Check 是全部检查（C.01 §6）：元校验、写法、命名、封闭对象、示例。
func Check(root string) []Problem {
	s, probs := Load(root)
	probs = append(probs, s.CheckMeta()...)
	probs = append(probs, s.CheckStyle()...)
	probs = append(probs, s.CheckNames()...)
	probs = append(probs, s.CheckClosed()...)
	probs = append(probs, s.CheckExamples()...)
	return probs
}

// CheckMeta：每个文件通过 2020-12 元校验并能编译（引用都解析得到）。
func (s *Set) CheckMeta() []Problem {
	var probs []Problem
	for _, name := range s.Files {
		if _, err := s.compiler.Compile(baseURL + name); err != nil {
			probs = append(probs, Problem{File: schemaDir + "/" + name, Rule: "C.01-R1", Msg: "元校验或编译失败：" + err.Error()})
		}
	}
	return probs
}

var (
	fileNameRe = regexp.MustCompile(`^[a-z0-9]+([-_][a-z0-9]+)*\.schema\.json$`)
	versionRe  = regexp.MustCompile(`^\d+\.\d+$`)
)

// CheckStyle：R1（文件名、$schema）、R2（$id）、R21（x-version）、R3（公共定义不重复）。
func (s *Set) CheckStyle() []Problem {
	var probs []Problem
	add := func(name, rule, msg string) {
		probs = append(probs, Problem{File: schemaDir + "/" + name, Rule: rule, Msg: msg})
	}
	common := defNames(s.Raw["common.schema.json"])
	for _, name := range s.Files {
		raw := s.Raw[name]
		if !fileNameRe.MatchString(name) {
			add(name, "C.01-R1", "文件名要小写，多个单词用连字符或下划线，以 .schema.json 结尾")
		}
		if raw["$schema"] != draft2020 {
			add(name, "C.01-R1", fmt.Sprintf("$schema 应为 %s", draft2020))
		}
		if raw["$id"] != baseURL+name {
			add(name, "C.01-R2", fmt.Sprintf("$id 应为 %s", baseURL+name))
		}
		if v, ok := raw["x-version"].(string); !ok || !versionRe.MatchString(v) {
			add(name, "C.01-R21", `顶层要有 "x-version": "主.次"`)
		}
		if name == "common.schema.json" {
			continue
		}
		for _, d := range sortedKeys(defNames(raw)) {
			if common[d] {
				add(name, "C.01-R3", "$defs/"+d+" 已经在 common.schema.json 里定义，这里只能引用")
			}
		}
	}
	return probs
}

func defNames(raw map[string]any) map[string]bool {
	out := map[string]bool{}
	if defs, ok := raw["$defs"].(map[string]any); ok {
		for k := range defs {
			out[k] = true
		}
	}
	return out
}

// 取值是 schema 的关键字（C.01 §6 的检查只沿这些关键字往下走，不进 enum、const、default、examples）。
var (
	schemaKeys    = []string{"items", "additionalProperties", "not", "if", "then", "else", "contains", "propertyNames", "unevaluatedItems", "unevaluatedProperties"}
	schemaMapKeys = []string{"properties", "patternProperties", "$defs", "dependentSchemas"}
	schemaArrKeys = []string{"allOf", "anyOf", "oneOf", "prefixItems"}
)

// walk 访问 node 里的每一个子 schema（包括 node 自己），ptr 是它在文件里的 JSON Pointer。
func walk(node any, ptr string, visit func(m map[string]any, ptr string)) {
	m, ok := node.(map[string]any)
	if !ok {
		return
	}
	visit(m, ptr)
	for _, k := range schemaKeys {
		if v, ok := m[k]; ok {
			walk(v, ptr+"/"+esc(k), visit)
		}
	}
	for _, k := range schemaMapKeys {
		if sub, ok := m[k].(map[string]any); ok {
			for _, name := range sortedKeys(sub) {
				walk(sub[name], ptr+"/"+esc(k)+"/"+esc(name), visit)
			}
		}
	}
	for _, k := range schemaArrKeys {
		if arr, ok := m[k].([]any); ok {
			for i, v := range arr {
				walk(v, fmt.Sprintf("%s/%s/%d", ptr, esc(k), i), visit)
			}
		}
	}
}

func esc(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

var fieldNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// CheckNames：字段名和按名字约定的类型（C.01-R4、R7–R13）。
func (s *Set) CheckNames() []Problem {
	var probs []Problem
	for _, name := range s.Files {
		file := schemaDir + "/" + name
		walk(s.Raw[name], "", func(m map[string]any, ptr string) {
			props, ok := m["properties"].(map[string]any)
			if !ok {
				return
			}
			required := map[string]bool{}
			if req, ok := m["required"].([]any); ok {
				for _, r := range req {
					if r, ok := r.(string); ok {
						required[r] = true
					}
				}
			}
			for _, field := range sortedKeys(props) {
				sub, _ := props[field].(map[string]any)
				at := ptr + "/properties/" + esc(field)
				for _, p := range fieldProblems(field, sub, required[field]) {
					p.File, p.At = file, at
					probs = append(probs, p)
				}
			}
		})
	}
	return probs
}

func fieldProblems(field string, sub map[string]any, required bool) []Problem {
	var probs []Problem
	add := func(rule, msg string) {
		probs = append(probs, Problem{Rule: rule, Msg: "字段 " + field + "：" + msg})
	}
	ref, _ := sub["$ref"].(string)
	typ, _ := sub["type"].(string)
	if !fieldNameRe.MatchString(field) {
		add("C.01-R4", "字段名只用小写字母、数字、下划线，以字母开头")
	}
	isTimestamp := strings.HasSuffix(ref, "#/$defs/Timestamp")
	switch {
	case isTimestamp && !strings.HasSuffix(field, "_at") && field != "ts":
		add("C.01-R7", "时间字段的名字以 _at 结尾（事件里叫 ts）")
	case !isTimestamp && (strings.HasSuffix(field, "_at") || field == "ts"):
		add("C.01-R7", "以 _at 结尾（或叫 ts）的字段要引用 Timestamp")
	}
	for _, c := range []struct{ suffix, rule, what string }{
		{"_ms", "C.01-R8", "时长"}, {"_micros", "C.01-R9", "金额"}, {"tokens", "C.01-R10", "token 数"},
	} {
		if strings.HasSuffix(field, c.suffix) && ref == "" && typ != "integer" {
			add(c.rule, c.what+"一律是整数（type: integer）")
		}
	}
	if strings.HasSuffix(field, "ratio") && ref == "" && (typ != "number" || !isNum(sub["minimum"], 0) || !isNum(sub["maximum"], 1)) {
		add("C.01-R11", "比例是 0–1 的小数：type: number、minimum: 0、maximum: 1")
	}
	if typ == "boolean" {
		if _, ok := sub["default"]; !ok && !required {
			add("C.01-R12", "不是必填的布尔字段要写 default")
		}
		for _, neg := range []string{"no_", "not_", "non_"} {
			if strings.HasPrefix(field, neg) {
				add("C.01-R12", "布尔字段不用否定词")
			}
		}
		if field == "disabled" {
			add("C.01-R12", "用 enabled，不用 disabled")
		}
	}
	if strings.HasSuffix(field, "_ref") && !strings.HasSuffix(ref, "#/$defs/WsRef") {
		add("C.01-R13", "以 _ref 结尾的字段指向大内容，要引用 WsRef")
	}
	return probs
}

func isNum(v any, want float64) bool {
	f, ok := v.(float64)
	return ok && f == want
}

// CheckClosed：每个 "type": "object" 要么写 additionalProperties，要么标 "x-open": true（C.01-R19）。
func (s *Set) CheckClosed() []Problem {
	var probs []Problem
	for _, name := range s.Files {
		file := schemaDir + "/" + name
		walk(s.Raw[name], "", func(m map[string]any, ptr string) {
			if xo, ok := m["x-open"]; ok && xo != true {
				probs = append(probs, Problem{File: file, At: ptr, Rule: "C.01-R19", Msg: `"x-open" 只能写 true`})
			}
			if !hasType(m["type"], "object") {
				return
			}
			ap, hasAP := m["additionalProperties"]
			open := m["x-open"] == true
			switch {
			case !hasAP && !open:
				probs = append(probs, Problem{File: file, At: ptr, Rule: "C.01-R19", Msg: `对象要写 additionalProperties，开放对象标 "x-open": true`})
			case open && ap == false:
				probs = append(probs, Problem{File: file, At: ptr, Rule: "C.01-R19", Msg: `"x-open": true 和 additionalProperties: false 矛盾`})
			}
		})
	}
	return probs
}

func hasType(t any, want string) bool {
	switch t := t.(type) {
	case string:
		return t == want
	case []any:
		for _, x := range t {
			if x == want {
				return true
			}
		}
	}
	return false
}

// Entry 是示例清单的一项（C.01-R22）。
type Entry struct {
	File    string  `yaml:"file"`
	Schema  string  `yaml:"schema"`
	Valid   *bool   `yaml:"valid"`
	ErrorAt *string `yaml:"error_at"`
	Diag    string  `yaml:"diag"`
}

// ReadManifest 读 root/schema/examples/manifest.yaml；不认识的键报错。
func ReadManifest(root string) ([]Entry, error) {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(exampleDir), manifestName))
	if err != nil {
		return nil, err
	}
	var entries []Entry
	if err := yaml.UnmarshalWithOptions(b, &entries, yaml.DisallowUnknownField()); err != nil {
		return nil, err
	}
	return entries, nil
}

var diagRe = regexp.MustCompile(`^V\d{2}$`)

// CheckExamples：清单里每一项按声明的 schema 校验，结果与清单一致；目录里没有清单以外的文件（C.01-R22、R23）。
func (s *Set) CheckExamples() []Problem {
	dir := filepath.Join(s.Root, filepath.FromSlash(exampleDir))
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	mfile := exampleDir + "/" + manifestName
	entries, err := ReadManifest(s.Root)
	if err != nil {
		return []Problem{{File: mfile, Rule: "C.01-R22", Msg: "读不了示例清单：" + err.Error()}}
	}
	var probs []Problem
	listed := map[string]bool{}
	for i, e := range entries {
		at := fmt.Sprintf("/%d", i)
		bad := func(msg string) {
			probs = append(probs, Problem{File: mfile, At: at, Rule: "C.01-R22", Msg: e.File + "：" + msg})
		}
		if !relExample(e.File) {
			bad("file 要写相对 schema/examples/ 的 .json 路径")
			continue
		}
		if listed[e.File] {
			bad("重复登记")
			continue
		}
		listed[e.File] = true
		probs = append(probs, s.checkEntry(e, at)...)
	}
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		if rel != manifestName && !listed[rel] && !strings.HasSuffix(rel, "_vectors.json") {
			probs = append(probs, Problem{File: exampleDir + "/" + rel, Rule: "C.01-R22", Msg: "没有登记在 manifest.yaml 里，也不是测试向量（*_vectors.json）"})
		}
		return nil
	})
	return probs
}

// relExample：file 是相对 schema/examples/ 的 .json 路径，只用 /，不能是绝对路径、不能带盘符、不能用 .. 跳出目录（C.01-R22）。
// 反斜杠一律拒绝：Windows 上 filepath.Join 会把它当成分隔符，..\x.json 就能读到目录外面的文件。
func relExample(file string) bool {
	return file != "" && strings.HasSuffix(file, ".json") &&
		!strings.ContainsAny(file, "\\:") && !strings.HasPrefix(file, "/") &&
		path.Clean(file) == file && file != ".." && !strings.HasPrefix(file, "../")
}

func (s *Set) checkEntry(e Entry, at string) []Problem {
	mfile := exampleDir + "/" + manifestName
	file := exampleDir + "/" + e.File
	var probs []Problem
	bad := func(rule, msg string) {
		probs = append(probs, Problem{File: mfile, At: at, Rule: rule, Msg: e.File + "：" + msg})
	}
	if !relExample(e.File) {
		bad("C.01-R22", "file 要写相对 schema/examples/ 的 .json 路径")
		return probs
	}
	schemaFile, _, _ := strings.Cut(e.Schema, "#")
	if _, ok := s.Raw[schemaFile]; !ok {
		bad("C.01-R22", "schema "+e.Schema+" 不在 schema/ 里")
		return probs
	}
	if e.Valid == nil {
		bad("C.01-R22", "要写 valid")
		return probs
	}
	semantic := strings.HasPrefix(e.File, "semantic/")
	switch {
	case *e.Valid && e.ErrorAt != nil:
		bad("C.01-R22", "valid 为 true 时不写 error_at")
	case !*e.Valid && e.ErrorAt == nil:
		bad("C.01-R22", "valid 为 false 时要写 error_at")
	}
	switch {
	case e.Diag != "" && !diagRe.MatchString(e.Diag):
		bad("C.01-R23", "diag 是 V 加两位数字")
	case e.Diag != "" && !semantic:
		bad("C.01-R23", "写了 diag 的示例放在 semantic/ 下")
	case semantic && (e.Diag == "" || !*e.Valid):
		bad("C.01-R23", "semantic/ 下的示例 valid 为 true，并写 diag")
	}
	b, err := os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(file)))
	if err != nil {
		bad("C.01-R22", "读不了："+err.Error())
		return probs
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		probs = append(probs, Problem{File: file, Rule: "C.01-R22", Msg: "不是合法的 JSON：" + err.Error()})
		return probs
	}
	verr := s.Validate(e.Schema, inst)
	var ve *jsonschema.ValidationError
	switch {
	case verr != nil && !errors.As(verr, &ve):
		probs = append(probs, Problem{File: file, Rule: "C.01-R22", Msg: verr.Error()})
	case *e.Valid && verr != nil:
		probs = append(probs, Problem{File: file, Rule: "C.01-R22", Msg: "应当通过 " + e.Schema + "，实际失败：" + verr.Error()})
	case !*e.Valid && verr == nil:
		probs = append(probs, Problem{File: file, Rule: "C.01-R22", Msg: "应当失败，实际通过了 " + e.Schema})
	case !*e.Valid && e.ErrorAt != nil:
		if outside := Outside(ve, *e.ErrorAt); len(outside) > 0 {
			probs = append(probs, Problem{File: file, Rule: "C.01-R22", Msg: fmt.Sprintf("报错位置应当都在 %q 或它下面，实际还有 %s", *e.ErrorAt, strings.Join(outside, "、"))})
		}
	}
	return probs
}

// Locations 是校验错误里每个最底层原因的实例位置（JSON Pointer），去重排序。
func Locations(ve *jsonschema.ValidationError) []string {
	set := map[string]bool{}
	var visit func(e *jsonschema.ValidationError)
	visit = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			set[pointer(e.InstanceLocation)] = true
			return
		}
		for _, c := range e.Causes {
			visit(c)
		}
	}
	visit(ve)
	return sortedKeys(set)
}

// Outside 是 Locations 里不在 at 或它下面的位置。
func Outside(ve *jsonschema.ValidationError, at string) []string {
	var out []string
	for _, loc := range Locations(ve) {
		if !Under(loc, at) {
			out = append(out, strconvQuote(loc))
		}
	}
	return out
}

// Under：loc 是 at 或在它下面。
func Under(loc, at string) bool {
	return at == "" || loc == at || strings.HasPrefix(loc, at+"/")
}

func pointer(tokens []string) string {
	var b strings.Builder
	for _, t := range tokens {
		b.WriteString("/" + esc(t))
	}
	return b.String()
}

func strconvQuote(s string) string { return fmt.Sprintf("%q", s) }

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
