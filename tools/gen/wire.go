package main

import (
	"fmt"
	"strings"
)

// 前端字段映射表（C.14-R8、W.04.03 §5.1，P.09 K235）：按类型生成，不按字段名汇总。
// 每个结构类型列出自己每个字段的值怎么转换；同一个字段名在不同类型里可以不同。

const wireRaw = `"raw"`

// wireType 是 WIRE_TYPES 的一项。
type wireType struct {
	name   string
	fields [][2]string // 字段名（snake_case）、节点的 TS 写法
	union  *union
}

// wireNode 是一个值在映射表里的写法：
//
//	"raw"          值整体原样：标量、开放对象、没有类型约束、值都是 raw 的数组或映射
//	{ t: "X" }     结构类型或判别联合，查 WIRE_TYPES.X
//	{ list: … }    数组，逐项按 …
//	{ map: … }     映射：键是数据、原样，值按 …
func (m *model) wireNode(t *typeRef, depth int) string {
	if depth > 32 {
		m.errs.add("字段映射表：类型嵌套太深（可能有数组或映射引用自己）")
		return wireRaw
	}
	k := m.kindOf(t)
	switch k {
	case kStruct, kWholeUnion:
		if t.name == "" {
			return wireRaw
		}
		return `{ t: ` + tsString(t.name) + ` }`
	case kArray, kMap:
		elem := t.elem
		if t.name != "" {
			elem = m.types[t.name].elem
		}
		if elem == nil {
			return wireRaw
		}
		e := m.wireNode(elem, depth+1)
		if e == wireRaw {
			return wireRaw
		}
		if k == kArray {
			return "{ list: " + e + " }"
		}
		return "{ map: " + e + " }"
	}
	return wireRaw
}

// wireTypes 是全部结构类型和判别联合的映射表，按类型名排序。
func (m *model) wireTypes() []wireType {
	var out []wireType
	for _, name := range sortedKeys(m.types) {
		n := m.types[name]
		if n.kind != kStruct && n.kind != kWholeUnion {
			continue
		}
		wt := wireType{name: name, union: n.union}
		for _, f := range n.fields {
			node := m.wireNode(f.t, 0)
			if f.unionField {
				node = wireRaw // 联合字段按 union.branches 取类型（C.14-R8）
			}
			wt.fields = append(wt.fields, [2]string{f.json, node})
		}
		out = append(out, wt)
	}
	return out
}

// emitWire 是 wire.ts（C.14-R8、W.04.03 §5.1）。
func emitWire(m *model) ([]byte, error) {
	types := m.wireTypes()
	if err := m.errs.err(); err != nil {
		return nil, err
	}
	var w strings.Builder
	w.WriteString(tsHeader)
	w.WriteString(`// 前端字段映射表（C.14-R8、W.04.03 §5.1）：按类型列出每个字段的值怎么转换，字段名一律转 camelCase。
//   "raw"      值整体原样（标量、开放对象、没有类型约束、值都不用转换的数组或映射）
//   { t }      结构类型或判别联合，查 WIRE_TYPES
//   { list }   数组，逐项转换
//   { map }    映射：键是数据、原样，值转换
// union：判别联合。有 field 的，field 的值按 disc 的取值查 branches；没有 field 的，整个对象按 branches 查。
// 表里没有的字段（服务端更新的版本）和不认识的判别值：字段名照常转换，值原样（C.01-R18）。

export type WireNode =
  | "raw"
  | { readonly t: WireTypeName }
  | { readonly list: WireNode }
  | { readonly map: WireNode };

export interface WireUnion {
  readonly disc: string;
  readonly field?: string;
  readonly branches: { readonly [value: string]: WireTypeName };
}

export interface WireType {
  readonly fields: { readonly [key: string]: WireNode };
  readonly union?: WireUnion;
}

`)
	w.WriteString("export type WireTypeName =\n")
	for _, t := range types {
		fmt.Fprintf(&w, "  | %s\n", tsString(t.name))
	}
	w.WriteString("  ;\n\nexport const WIRE_TYPES: { readonly [K in WireTypeName]: WireType } = {\n")
	for _, t := range types {
		fmt.Fprintf(&w, "  %s: {\n    fields: {", t.name)
		if len(t.fields) == 0 {
			w.WriteString("},\n")
		} else {
			w.WriteString("\n")
			for _, f := range t.fields {
				fmt.Fprintf(&w, "      %s: %s,\n", tsString(f[0]), f[1])
			}
			w.WriteString("    },\n")
		}
		if u := t.union; u != nil {
			fmt.Fprintf(&w, "    union: {\n      disc: %s,\n", tsString(u.disc))
			if u.field != "" {
				fmt.Fprintf(&w, "      field: %s,\n", tsString(u.field))
			}
			w.WriteString("      branches: {\n")
			for _, b := range u.branches {
				fmt.Fprintf(&w, "        %s: %s,\n", tsString(b.value), tsString(b.typ))
			}
			w.WriteString("      },\n    },\n")
		}
		w.WriteString("  },\n")
	}
	w.WriteString("};\n")
	return []byte(w.String()), nil
}
