package ctxasm

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Transform 是段的变换（C.13.01 §2）。
type Transform struct {
	Kind     string // as_is | pointer | summary | top_k | fields | template
	N        int
	Fields   []string
	Template string
}

// 变换的种类。
const (
	TransformAsIs     = "as_is"
	TransformPointer  = "pointer"
	TransformSummary  = "summary"
	TransformTopK     = "top_k"
	TransformFields   = "fields"
	TransformTemplate = "template"
)

// 渲染格式。
const (
	FormatText    = "text"
	FormatBullets = "bullets"
	FormatYAML    = "yaml"
	FormatJSON    = "json"
)

var (
	transformCallRe = regexp.MustCompile(`^(top_k|fields|template)\((.*)\)$`)
	fieldNameRe     = regexp.MustCompile(`^[^\s,(){}]+$`)
	placeholderRe   = regexp.MustCompile(`\{\{\s*([^{}\s]+)\s*\}\}`)
)

// ParseTransform 解析 transform 的写法。
func ParseTransform(s string) (Transform, error) {
	s = strings.TrimSpace(s)
	switch s {
	case "", TransformAsIs:
		return Transform{Kind: TransformAsIs}, nil
	case TransformPointer, TransformSummary:
		return Transform{Kind: s}, nil
	}
	m := transformCallRe.FindStringSubmatch(s)
	if m == nil {
		return Transform{}, fmt.Errorf("不认识的 transform：%q", s)
	}
	arg := strings.TrimSpace(m[2])
	switch m[1] {
	case TransformTopK:
		n, err := strconv.Atoi(arg)
		if err != nil || n < 1 {
			return Transform{}, fmt.Errorf("top_k 的参数要是正整数：%q", s)
		}
		return Transform{Kind: TransformTopK, N: n}, nil
	case TransformFields:
		var fs []string
		for _, f := range strings.Split(arg, ",") {
			f = strings.TrimSpace(f)
			if !fieldNameRe.MatchString(f) {
				return Transform{}, fmt.Errorf("fields 的字段名写法不对：%q", s)
			}
			fs = append(fs, f)
		}
		return Transform{Kind: TransformFields, Fields: fs}, nil
	}
	if arg == "" {
		return Transform{}, fmt.Errorf("template 不能为空：%q", s)
	}
	return Transform{Kind: TransformTemplate, Template: arg}, nil
}

// Apply 做 top_k、fields、template；pointer、summary、as_is 原样返回（由装配和溢出处理做）。
func (t Transform) Apply(data any) any {
	switch t.Kind {
	case TransformTopK:
		if l, ok := data.([]any); ok && len(l) > t.N {
			return l[:t.N]
		}
	case TransformFields:
		return pickFields(data, t.Fields)
	case TransformTemplate:
		if l, ok := data.([]any); ok {
			lines := make([]string, 0, len(l))
			for _, x := range l {
				lines = append(lines, fill(t.Template, x))
			}
			return strings.Join(lines, "\n")
		}
		return fill(t.Template, data)
	}
	return data
}

func pickFields(data any, fs []string) any {
	switch d := data.(type) {
	case map[string]any:
		out := map[string]any{}
		for _, f := range fs {
			if v, ok := d[f]; ok {
				out[f] = v
			}
		}
		return out
	case []any:
		out := make([]any, len(d))
		for i, x := range d {
			out[i] = pickFields(x, fs)
		}
		return out
	}
	return data
}

func fill(tpl string, v any) string {
	return placeholderRe.ReplaceAllStringFunc(tpl, func(m string) string {
		name := placeholderRe.FindStringSubmatch(m)[1]
		return RenderText(lookup(v, strings.Split(name, ".")), FormatText)
	})
}

// lookup 按字段路径取值；遇到列表时对每一项取，结果去掉 nil 后组成列表。
func lookup(data any, path []string) any {
	if len(path) == 0 {
		return data
	}
	switch d := data.(type) {
	case map[string]any:
		return lookup(d[path[0]], path[1:])
	case []any:
		var out []any
		for _, x := range d {
			if v := lookup(x, path); v != nil {
				out = append(out, v)
			}
		}
		if out == nil {
			return nil
		}
		return out
	}
	return nil
}

// RenderText 把值渲染成文字（E.13.01 §5）。渲染是确定的：对象的键按字典序。
func RenderText(data any, format string) string {
	switch format {
	case FormatJSON:
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return fmt.Sprint(data)
		}
		return string(b)
	case FormatBullets:
		l, ok := data.([]any)
		if !ok {
			l = []any{data}
		}
		lines := make([]string, 0, len(l))
		for _, x := range l {
			lines = append(lines, "- "+inline(x))
		}
		return strings.Join(lines, "\n")
	case FormatYAML:
		return strings.Join(yamlLines(data, 0), "\n")
	}
	switch data.(type) {
	case map[string]any, []any:
		return strings.Join(yamlLines(data, 0), "\n")
	}
	return scalar(data)
}

func scalar(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case float64:
		if x == math.Trunc(x) && math.Abs(x) < 1e15 {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case json.Number:
		return x.String()
	}
	return fmt.Sprint(v)
}

func sortedKeys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// inline 是一项的单行写法：对象写成"键：值；键：值"。
func inline(v any) string {
	switch x := v.(type) {
	case map[string]any:
		parts := make([]string, 0, len(x))
		for _, k := range sortedKeys(x) {
			parts = append(parts, k+"："+inline(x[k]))
		}
		return strings.Join(parts, "；")
	case []any:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			parts = append(parts, inline(e))
		}
		return strings.Join(parts, "、")
	}
	return strings.ReplaceAll(scalar(v), "\n", " ")
}

// yamlLines 按两格缩进渲染；多行字符串用 |。
func yamlLines(v any, indent int) []string {
	pad := strings.Repeat(" ", indent)
	switch x := v.(type) {
	case map[string]any:
		if len(x) == 0 {
			return []string{pad + "{}"}
		}
		var out []string
		for _, k := range sortedKeys(x) {
			out = append(out, keyLines(pad, k, x[k], indent)...)
		}
		return out
	case []any:
		if len(x) == 0 {
			return []string{pad + "[]"}
		}
		var out []string
		for _, e := range x {
			switch e.(type) {
			case map[string]any, []any:
				sub := yamlLines(e, indent+2)
				if isEmptyColl(e) {
					out = append(out, pad+"- "+strings.TrimSpace(sub[0]))
					continue
				}
				sub[0] = pad + "- " + strings.TrimPrefix(sub[0], pad+"  ")
				out = append(out, sub...)
			default:
				out = append(out, scalarLines(pad+"-", e, indent+2)...)
			}
		}
		return out
	}
	var out []string
	for _, l := range strings.Split(scalar(v), "\n") {
		out = append(out, pad+l)
	}
	return out
}

func keyLines(pad, k string, v any, indent int) []string {
	switch v.(type) {
	case map[string]any, []any:
		if isEmptyColl(v) {
			return []string{pad + k + ": " + strings.TrimSpace(yamlLines(v, 0)[0])}
		}
		return append([]string{pad + k + ":"}, yamlLines(v, indent+2)...)
	}
	return scalarLines(pad+k+":", v, indent+2)
}

// scalarLines 写"前缀 值"；值为空时只写前缀；多行时写"前缀 |"再逐行缩进。
func scalarLines(prefix string, v any, indent int) []string {
	s := scalar(v)
	if s == "" {
		return []string{prefix}
	}
	if !strings.Contains(s, "\n") {
		return []string{prefix + " " + s}
	}
	pad := strings.Repeat(" ", indent)
	out := []string{prefix + " |"}
	for _, l := range strings.Split(s, "\n") {
		out = append(out, pad+l)
	}
	return out
}

func isEmptyColl(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		return len(x) == 0
	case []any:
		return len(x) == 0
	}
	return false
}
