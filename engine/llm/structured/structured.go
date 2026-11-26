// Package structured 保证 Agent 拿到符合 output_schema 的结构化结果（设计文档 E.10.06）：选策略、降级、
// 改写请求、取 JSON、修复、宽松修正、校验和生成反馈。原生写法和强制工具的取值在 engine/llm（E.10.01 §5.1）。
package structured

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/mio-cc/anywhere-door/engine/llm"
	"github.com/mio-cc/anywhere-door/gen"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Strategy 是拿结构化输出的办法；取值和 llm.call 事件的 strategy 一致（C.06.02 §5）。
type Strategy string

// 三种策略。
const (
	Native     Strategy = "native_schema" // 协议原生的结构化输出（Req.Output，Caps.NativeSchema）
	ForcedTool Strategy = "tool_call"     // 强制调用 emit_<名字> 工具（Caps.Tools）
	TextParse  Strategy = "text_repair"   // 文本里找 JSON 再修复
)

// 错误码（*llm.Error 的 Code，分类都是 format_invalid）。
const (
	CodeNoJSON         = "no_json"
	CodeSchemaMismatch = "schema_mismatch"
)

// 宽松修正的名字（Result.Repairs）；填默认值记作 "default:<路径>"。
const (
	RepairNumberString  = "number_string"
	RepairSingleToArray = "single_to_array"
)

const maxErrors = 20

// Validator 校验一个值（jsonschema.UnmarshalJSON 的结果）；*jsonschema.Schema 满足它。
type Validator interface{ Validate(v any) error }

// ValidationError 是一处校验不通过：JSON Pointer 写法的位置和说明。
type ValidationError struct{ Path, Message string }

// Result 是 Decode 的结果。
type Result struct {
	JSON    json.RawMessage
	Repairs []string
	Errors  []ValidationError
}

// Pick 按能力选策略：原生 → 强制工具 → 文本解析。
func Pick(c llm.Caps) Strategy {
	switch {
	case c.NativeSchema:
		return Native
	case c.Tools:
		return ForcedTool
	}
	return TextParse
}

// Next 是结果不合格时的降级：Native → ForcedTool（模型支持工具时）→ TextParse；TextParse 没有下一级。
func Next(s Strategy, c llm.Caps) (Strategy, bool) {
	switch s {
	case Native:
		if c.Tools {
			return ForcedTool, true
		}
		return TextParse, true
	case ForcedTool:
		return TextParse, true
	}
	return s, false
}

// Prepare 按策略改写请求，返回副本（E.10.06 §5 第 2 步）。
func Prepare(req llm.Req, s Strategy, out llm.OutputSpec) llm.Req {
	r := req
	r.Messages = slices.Clone(req.Messages)
	r.Tools = slices.Clone(req.Tools)
	r.Output = nil
	switch s {
	case Native:
		o := out
		r.Output = &o
	case ForcedTool:
		tool := llm.StructuredTool(&out)
		r.Tools = append(r.Tools, tool)
		r.ToolChoice = tool.Name
	default:
		hint := llm.Part{Type: llm.PartText, Text: textHint(out)}
		if len(r.Messages) > 0 && r.Messages[0].Role == llm.RoleSystem {
			m := r.Messages[0]
			m.Parts = append(slices.Clone(m.Parts), hint)
			r.Messages[0] = m
		} else {
			r.Messages = append([]llm.Message{{Role: llm.RoleSystem, Parts: []llm.Part{hint}}}, r.Messages...)
		}
	}
	return r
}

func textHint(out llm.OutputSpec) string {
	var buf bytes.Buffer
	if len(out.Schema) > 0 && json.Compact(&buf, out.Schema) != nil {
		buf.Reset()
		buf.Write(out.Schema)
	}
	return "只输出一个 JSON 对象，不要输出别的文字。它要符合下面的 JSON Schema：\n" + buf.String()
}

// Decode 从响应里取出结构化结果（E.10.06 §5 第 3–6 步）。v 为 nil 时不校验。
func Decode(resp *llm.Resp, s Strategy, out llm.OutputSpec, v Validator) (Result, error) {
	var res Result
	if resp == nil {
		return res, noJSON()
	}
	var raw json.RawMessage
	switch s {
	case Native:
		raw = resp.Structured
	case ForcedTool:
		name := llm.EmitPrefix + out.Name
		for _, tc := range resp.ToolCalls {
			if tc.Name == name {
				raw = tc.Args
				break
			}
		}
		if len(raw) == 0 {
			raw = resp.Structured
		}
	}
	got, reps, ok := json.RawMessage(nil), []string(nil), false
	if len(raw) > 0 {
		got, reps, ok = Repair(string(raw))
	}
	if !ok {
		got, reps, ok = Repair(resp.Text)
	}
	if !ok {
		return res, noJSON()
	}
	res.Repairs = append(res.Repairs, reps...)

	dec := json.NewDecoder(bytes.NewReader(got))
	dec.UseNumber()
	var val any
	if err := dec.Decode(&val); err != nil {
		return res, noJSON()
	}
	var sch map[string]any
	if len(out.Schema) > 0 {
		_ = json.Unmarshal(out.Schema, &sch)
	}
	val = coerce(val, sch, "", &res.Repairs)
	b, err := json.Marshal(val)
	if err != nil {
		return res, noJSON()
	}
	res.JSON = b
	if v == nil {
		return res, nil
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err == nil {
		err = v.Validate(inst)
	}
	if err != nil {
		res.Errors = toErrors(err)
		first := res.Errors[0]
		return res, &llm.Error{Class: gen.FailureClassFormatInvalid, Code: CodeSchemaMismatch,
			Message: fmt.Sprintf("输出不符合要求的格式：%s %s", first.Path, first.Message)}
	}
	return res, nil
}

func noJSON() error {
	return &llm.Error{Class: gen.FailureClassFormatInvalid, Code: CodeNoJSON, Message: "模型的输出里找不到 JSON"}
}

var (
	integerRe = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
	numberRe  = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)
)

func note(reps *[]string, r string) {
	if !slices.Contains(*reps, r) {
		*reps = append(*reps, r)
	}
}

// coerce 按 schema 原文做宽松修正（E.10.06 §5 第 5 步）：只看 type、properties、items、default。
func coerce(v any, s map[string]any, path string, reps *[]string) any {
	if s == nil {
		return v
	}
	typ, _ := s["type"].(string)
	switch typ {
	case "integer", "number":
		if str, ok := v.(string); ok {
			t := strings.TrimSpace(str)
			re := numberRe
			if typ == "integer" {
				re = integerRe
			}
			if re.MatchString(t) {
				note(reps, RepairNumberString)
				return json.Number(t)
			}
		}
	case "array":
		if v == nil {
			return v
		}
		arr, ok := v.([]any)
		if !ok {
			note(reps, RepairSingleToArray)
			arr = []any{v}
		}
		if items, ok := s["items"].(map[string]any); ok {
			for i := range arr {
				arr[i] = coerce(arr[i], items, path+"/"+strconv.Itoa(i), reps)
			}
		}
		return arr
	}
	obj, ok := v.(map[string]any)
	props, _ := s["properties"].(map[string]any)
	if !ok || props == nil {
		return v
	}
	names := make([]string, 0, len(props))
	for n := range props {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		ps, _ := props[n].(map[string]any)
		p := path + "/" + n
		if cur, present := obj[n]; present {
			obj[n] = coerce(cur, ps, p, reps)
		} else if def, has := ps["default"]; has {
			obj[n] = def
			note(reps, "default:"+p)
		}
	}
	return obj
}

func pointer(tokens []string) string {
	var b strings.Builder
	for _, t := range tokens {
		b.WriteString("/" + strings.ReplaceAll(strings.ReplaceAll(t, "~", "~0"), "/", "~1"))
	}
	return b.String()
}

// toErrors 把校验错误拆成每处（最底层的原因）：位置和说明。
func toErrors(err error) []ValidationError {
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return []ValidationError{{Path: "/", Message: err.Error()}}
	}
	var leaves []*jsonschema.ValidationError
	var visit func(e *jsonschema.ValidationError)
	visit = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			leaves = append(leaves, e)
			return
		}
		for _, c := range e.Causes {
			visit(c)
		}
	}
	visit(ve)
	lines := strings.Split(ve.Error(), "\n")
	var out []ValidationError
	seen := map[string]bool{}
	for _, leaf := range leaves {
		at := pointer(leaf.InstanceLocation)
		msg := "不符合要求"
		for _, l := range lines {
			l = strings.TrimPrefix(strings.TrimSpace(l), "- ")
			if prefix := "at '" + at + "'"; strings.HasPrefix(l, prefix) {
				msg = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(l, prefix), ":"))
				break
			}
		}
		if at == "" {
			at = "/"
		}
		if seen[at+msg] {
			continue
		}
		seen[at+msg] = true
		out = append(out, ValidationError{Path: at, Message: msg})
		if len(out) == maxErrors {
			break
		}
	}
	if len(out) == 0 {
		out = []ValidationError{{Path: "/", Message: strings.TrimSpace(lines[len(lines)-1])}}
	}
	return out
}

// Feedback 生成"上次的输出不符合要求"的 user 消息，Agent 执行器追加它重试一次（E.14.02）。
func Feedback(errs []ValidationError) llm.Message {
	var b strings.Builder
	b.WriteString("上次的输出不符合要求，请改正后只输出一个 JSON 对象：")
	for _, e := range errs {
		b.WriteString("\n- " + e.Path + "：" + e.Message)
	}
	return llm.Message{Role: llm.RoleUser, Parts: []llm.Part{{Type: llm.PartText, Text: b.String()}}}
}
