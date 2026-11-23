package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

const tsHeader = "/* 由 tools/gen 生成，请勿手改 */\n\n"

// tsString 是 TS 的字符串字面量（JSON 写法，不转义 HTML 字符）。
func tsString(s string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimRight(b.String(), "\n")
}

func tsDoc(doc string) string {
	return strings.ReplaceAll(doc, "*/", "* /")
}

func (m *model) tsExpr(t *typeRef) string {
	if t.name != "" {
		return t.name
	}
	switch t.kind {
	case kString:
		if len(t.lits) > 0 {
			var ls []string
			for _, l := range t.lits {
				ls = append(ls, tsString(l))
			}
			return strings.Join(ls, " | ")
		}
		return "string"
	case kInt, kNumber:
		return "number"
	case kBool:
		return "boolean"
	case kStrOrNum:
		return "string | number"
	case kOpenMap:
		return "{ [key: string]: unknown }"
	case kArray:
		e := m.tsExpr(t.elem)
		if strings.Contains(e, " ") {
			e = "(" + e + ")"
		}
		return e + "[]"
	case kMap:
		return "{ [key: string]: " + m.tsExpr(t.elem) + " }"
	}
	return "unknown"
}

func (m *model) tsField(w *strings.Builder, f *field, indent string) {
	if f.doc != "" {
		fmt.Fprintf(w, "%s/** %s */\n", indent, tsDoc(f.doc))
	}
	opt := ""
	if !f.required {
		opt = "?"
	}
	e := m.tsExpr(f.t)
	if f.nullable {
		e += " | null"
	}
	fmt.Fprintf(w, "%s%s%s: %s;\n", indent, f.json, opt, e)
}

// emitTS 是 types.ts：全部 schema 类型、同名常量对象、withDefaults（C.14 §3、§4）。
func emitTS(m *model, cs *constSet) ([]byte, error) {
	var w strings.Builder
	var errs genErrors
	w.WriteString(tsHeader)
	w.WriteString("// 来源：schema/*.schema.json（C.14）。字段名是 schema 的 snake_case；前端只经 lib/protocol 使用（W.04.04-R1）。\n\n")
	for _, name := range sortedKeys(m.types) {
		n := m.types[name]
		if n.doc != "" {
			fmt.Fprintf(&w, "/** %s */\n", tsDoc(n.doc))
		}
		switch n.kind {
		case kStruct:
			if n.union == nil {
				fmt.Fprintf(&w, "export interface %s {\n", name)
				for _, f := range n.fields {
					m.tsField(&w, f, "  ")
				}
				w.WriteString("}\n\n")
				m.tsDefaults(&w, n)
				continue
			}
			common := name + "Common"
			if _, clash := m.types[common]; clash {
				errs.add("TS 类型 %s 和已有类型重名", common)
			}
			fmt.Fprintf(&w, "export interface %s {\n", common)
			for _, f := range n.fields {
				if !f.disc && !f.unionField {
					m.tsField(&w, f, "  ")
				}
			}
			fmt.Fprintf(&w, "}\n\nexport type %s = %s & (\n", name, common)
			for _, b := range n.union.branches {
				fmt.Fprintf(&w, "  | { %s: %s; %s: %s }\n", n.union.disc, tsString(b.value), n.union.field, b.typ)
			}
			w.WriteString(");\n\n")
		case kWholeUnion:
			var bs []string
			for _, b := range n.union.branches {
				bs = append(bs, b.typ)
			}
			fmt.Fprintf(&w, "export type %s = %s;\n\n", name, strings.Join(bs, " | "))
		case kOpenMap:
			fmt.Fprintf(&w, "export type %s = { [key: string]: unknown };\n\n", name)
		case kMap:
			fmt.Fprintf(&w, "export type %s = { [key: string]: %s };\n\n", name, m.tsExpr(n.elem))
		case kArray:
			fmt.Fprintf(&w, "export type %s = %s;\n\n", name, m.tsExpr(&typeRef{kind: kArray, elem: n.elem}))
		case kString:
			switch {
			case n.brand:
				fmt.Fprintf(&w, "export type %s = string & { readonly __brand: %s };\n\n", name, tsString(name))
			case len(n.enum) > 0:
				fmt.Fprintf(&w, "export type %s = %s;\n\n", name, m.tsExpr(&typeRef{kind: kString, lits: n.enum}))
				if g := cs.byType[name]; g != nil {
					tsConstObject(&w, g)
				}
			default:
				fmt.Fprintf(&w, "export type %s = string;\n\n", name)
			}
		case kTimestamp:
			fmt.Fprintf(&w, "export type %s = string;\n\n", name)
		default:
			fmt.Fprintf(&w, "export type %s = %s;\n\n", name, m.tsExpr(&typeRef{kind: n.kind}))
		}
	}
	return []byte(strings.TrimRight(w.String(), "\n") + "\n"), errs.err()
}

func (m *model) tsDefaults(w *strings.Builder, n *named) {
	var props []string
	for _, f := range n.fields {
		if f.def == nil || f.required {
			continue
		}
		k := m.kindOf(f.t)
		if f.t.name != "" && m.types[f.t.name].brand {
			continue
		}
		lit := defaultLit(k, f.def)
		if lit == "" {
			continue
		}
		if s, ok := f.def.(string); ok {
			lit = tsString(s)
		}
		props = append(props, f.json+": "+lit)
	}
	if len(props) == 0 {
		return
	}
	fmt.Fprintf(w, "/** 把没填的可选字段设为 schema 的默认值（C.14 §3）。 */\nexport function withDefaults%[1]s(v: %[1]s): %[1]s {\n  return { %[2]s, ...v };\n}\n\n", n.name, strings.Join(props, ", "))
}
