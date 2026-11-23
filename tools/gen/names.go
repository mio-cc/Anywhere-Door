package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// initialisms 是字段名、常量名里按 Go 习惯整体大写的词（C.14-R5）。
var initialisms = map[string]string{
	"id": "ID", "ids": "IDs", "url": "URL", "http": "HTTP", "json": "JSON",
	"mcp": "MCP", "llm": "LLM", "api": "API", "ui": "UI", "sse": "SSE",
}

var (
	fieldNameRe  = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	underDigitRe = regexp.MustCompile(`_[0-9]`)
	identRe      = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
)

// checkFieldName 检查字段名：C.01-R4，再加 C.14-R9（能无损往返 camelCase）。
func checkFieldName(name string) error {
	if !fieldNameRe.MatchString(name) {
		return fmt.Errorf("字段名 %q 只能用小写字母、数字、下划线，以字母开头（C.01-R4）", name)
	}
	if strings.Contains(name, "__") || underDigitRe.MatchString(name) || strings.HasSuffix(name, "_") {
		return fmt.Errorf("字段名 %q 不能无损往返 snake_case ↔ camelCase（C.14-R9）", name)
	}
	return nil
}

// goName 把 snake_case 字段名换成 Go 的 PascalCase，缩写按 R5（node_ids → NodeIDs）。
func goName(snake string) string {
	var b strings.Builder
	for _, part := range strings.Split(snake, "_") {
		if part == "" {
			continue
		}
		if v, ok := initialisms[part]; ok {
			b.WriteString(v)
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return b.String()
}

// constName 是枚举取值在常量名里的部分（C.14-R6）：按 . _ - : 切开，各段首字母大写；
// goStyle 时套用 R5 的缩写。取值不是 ASCII 字母数字时返回错误。
func constName(value string, goStyle bool) (string, error) {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == '.' || r == '_' || r == '-' || r == ':' })
	var b strings.Builder
	for _, p := range parts {
		if goStyle {
			if v, ok := initialisms[strings.ToLower(p)]; ok && strings.ToLower(p) == p {
				b.WriteString(v)
				continue
			}
		}
		b.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	s := b.String()
	if !identRe.MatchString(s) {
		return "", fmt.Errorf("枚举取值 %q 换不成常量名（得到 %q）", value, s)
	}
	return s, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// oneLine 把说明文字压成一行（生成到注释里）。
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
