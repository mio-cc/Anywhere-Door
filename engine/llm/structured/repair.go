package structured

import (
	"encoding/json"
	"strings"
)

// 修复的名字（Result.Repairs）。
const (
	RepairCodeFence     = "code_fence"
	RepairExtract       = "extract"
	RepairComment       = "comment"
	RepairSingleQuote   = "single_quote"
	RepairTrailingComma = "trailing_comma"
	RepairClose         = "close"
)

// Repair 从模型输出的文本里取出一个 JSON 对象或数组（E.10.06 §5 第 4 步）。
// 本身是合法 JSON 时原样返回、不算修复；修不成合法 JSON 时 ok 为 false。
func Repair(text string) (raw json.RawMessage, repairs []string, ok bool) {
	t := strings.TrimSpace(text)
	if isBlock(t) && json.Valid([]byte(t)) {
		return json.RawMessage(t), nil, true
	}
	if inner, found := fenced(t); found {
		repairs = append(repairs, RepairCodeFence)
		t = strings.TrimSpace(inner)
		if isBlock(t) && json.Valid([]byte(t)) {
			return json.RawMessage(t), repairs, true
		}
	}
	start := strings.IndexAny(t, "{[")
	if start < 0 {
		return nil, repairs, false
	}
	fixed, rest, reps := fix(t[start:])
	if start > 0 || strings.TrimSpace(rest) != "" {
		repairs = append(repairs, RepairExtract)
	}
	repairs = append(repairs, reps...)
	if !json.Valid([]byte(fixed)) {
		return nil, repairs, false
	}
	return json.RawMessage(fixed), repairs, true
}

func isBlock(t string) bool { return strings.HasPrefix(t, "{") || strings.HasPrefix(t, "[") }

// fenced 取第一个 ``` 代码块里的内容（第一行的语言名去掉）；没有结尾的 ``` 时取到文本末尾。
func fenced(t string) (string, bool) {
	i := strings.Index(t, "```")
	if i < 0 {
		return "", false
	}
	body := t[i+3:]
	if nl := strings.IndexByte(body, '\n'); nl >= 0 && !strings.ContainsAny(body[:nl], "{[") {
		body = body[nl+1:]
	}
	if j := strings.Index(body, "```"); j >= 0 {
		body = body[:j]
	}
	return body, true
}

// fix 从 s 开头（{ 或 [）起修复第一个完整的块：去注释、单引号改双引号、去尾逗号、补闭合。
// 返回修好的块、块后面剩下的文本和做过的修复。按字节处理：用到的符号都是 ASCII，不会落在多字节字符中间。
func fix(s string) (out string, rest string, repairs []string) {
	var b []byte
	var stack []byte
	done := map[string]bool{}
	note := func(r string) {
		if !done[r] {
			done[r] = true
			repairs = append(repairs, r)
		}
	}
	trimComma := func() {
		k := len(b)
		for k > 0 && (b[k-1] == ' ' || b[k-1] == '\t' || b[k-1] == '\n' || b[k-1] == '\r') {
			k--
		}
		if k > 0 && b[k-1] == ',' {
			b = append(b[:k-1], b[k:]...)
			note(RepairTrailingComma)
		}
	}
	var quote byte // 0：不在字符串里；'"' 或 '\''：在这种引号的字符串里
	i := 0
	for ; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			switch {
			case c == '\\' && i+1 < len(s):
				if quote == '\'' && s[i+1] == '\'' {
					b = append(b, '\'')
				} else {
					b = append(b, c, s[i+1])
				}
				i++
			case c == quote:
				b = append(b, '"')
				quote = 0
			case quote == '\'' && c == '"':
				b = append(b, '\\', '"')
			default:
				b = append(b, c)
			}
			continue
		}
		switch {
		case c == '"':
			quote = '"'
			b = append(b, c)
		case c == '\'':
			quote = '\''
			b = append(b, '"')
			note(RepairSingleQuote)
		case c == '/' && i+1 < len(s) && s[i+1] == '/':
			for i < len(s) && s[i] != '\n' {
				i++
			}
			i--
			note(RepairComment)
		case c == '/' && i+1 < len(s) && s[i+1] == '*':
			end := strings.Index(s[i+2:], "*/")
			if end < 0 {
				i = len(s)
			} else {
				i += 2 + end + 1
			}
			note(RepairComment)
		case c == '{' || c == '[':
			stack = append(stack, c)
			b = append(b, c)
		case c == '}' || c == ']':
			trimComma()
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			b = append(b, c)
			if len(stack) == 0 {
				return string(b), s[i+1:], repairs
			}
		default:
			b = append(b, c)
		}
	}
	if quote != 0 {
		b = append(b, '"')
		note(RepairClose)
	}
	for len(stack) > 0 {
		trimComma()
		open := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if open == '{' {
			b = append(b, '}')
		} else {
			b = append(b, ']')
		}
		note(RepairClose)
	}
	return string(b), "", repairs
}
