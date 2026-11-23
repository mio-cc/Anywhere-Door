package main

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

// kind 是生成器认识的几种类型形状（C.14 §3）。
type kind int

const (
	kStruct     kind = iota // 有 properties 的对象
	kOpenMap                // 开放对象（x-open）
	kMap                    // 只有 additionalProperties 的对象
	kArray                  // 数组
	kString                 // 字符串（含枚举、const）
	kInt                    // integer
	kNumber                 // number
	kBool                   // boolean
	kRaw                    // 没有类型约束
	kStrOrNum               // type: [string, number]
	kTimestamp              // common.schema.json#/$defs/Timestamp
	kWholeUnion             // 整体联合（C.14-R1）
)

// typeRef 是字段或元素的类型：具名类型（name 不为空），或匿名的内置类型。
type typeRef struct {
	name string
	kind kind
	elem *typeRef // kArray、kMap 的元素
	lits []string // kString 的字面量（内联枚举、const），只有 TS 用
}

// named 是一个具名类型：$defs、根类型、内联对象、判别字段的内联枚举。
type named struct {
	name, file, ptr, doc string
	kind                 kind
	brand                bool     // Ulid（TS 品牌类型）
	enum                 []string // 具名枚举的取值
	elem                 *typeRef // kArray、kMap
	fields               []*field // kStruct，按字段名排序
	union                *union   // 字段联合（kStruct）或整体联合（kWholeUnion）
	done                 bool
}

type field struct {
	json, goName, doc string
	t                 *typeRef
	required          bool
	nullable          bool
	def               any  // 标量默认值：string、bool、json.Number
	unionField        bool // 字段联合的联合字段（payload、data）
	disc              bool // 判别字段
}

// union 是一个判别联合（C.14-R1）。field 为空是整体联合。
type union struct {
	disc, field, discType string
	branches              []branch // 按取值排序
}

type branch struct{ value, typ string }

type model struct {
	set   *schemaSet
	roots map[string]string
	types map[string]*named
	byLoc map[string]string // 位置 → 类型名
	errs  genErrors
}

// buildModel 把全部 schema 转成类型模型。第一遍登记 $defs 和根类型的名字，第二遍填内容（内联类型在第二遍登记）。
func buildModel(set *schemaSet, roots map[string]string) (*model, error) {
	m := &model{set: set, roots: roots, types: map[string]*named{}, byLoc: map[string]string{}}
	for _, f := range sortedKeys(roots) {
		if _, ok := set.files[f]; !ok {
			m.errs.add("aliases.yaml 的 roots 登记了不存在的文件 %s", f)
		}
	}
	for _, f := range set.names {
		raw := set.files[f]
		for _, d := range sortedKeys(obj(raw, "$defs")) {
			m.claim(d, loc{f, ptrJoin("", "$defs", d)})
		}
		_, registered := roots[f]
		switch {
		case hasRoot(raw) && !registered:
			m.errs.add("%s 有根结构，但根类型名没有登记在 aliases.yaml 的 roots（C.14 §3）", f)
		case hasRoot(raw):
			m.claim(roots[f], loc{f, ""})
		case registered:
			m.errs.add("%s 没有根结构，aliases.yaml 的 roots 不该登记它", f)
		}
	}
	for _, f := range set.names {
		raw := set.files[f]
		defs := obj(raw, "$defs")
		for _, d := range sortedKeys(defs) {
			n := m.types[m.byLoc[loc{f, ptrJoin("", "$defs", d)}.String()]]
			node, ok := defs[d].(map[string]any)
			if n != nil && ok {
				m.fill(n, node)
			}
		}
		if n := m.types[m.byLoc[loc{f, ""}.String()]]; n != nil && hasRoot(raw) {
			m.fill(n, raw)
		}
	}
	return m, m.errs.err()
}

func hasRoot(raw map[string]any) bool {
	_, p := raw["properties"]
	_, t := raw["type"]
	_, a := raw["allOf"]
	return p || t || a
}

func (m *model) claim(name string, l loc) *named {
	if !identRe.MatchString(name) {
		m.errs.add("%s：类型名 %q 不是合法的标识符", l, name)
		return nil
	}
	if old, ok := m.types[name]; ok {
		m.errs.add("类型名 %s 重名：%s 和 %s（C.14 §3）", name, loc{old.file, old.ptr}, l)
		return nil
	}
	n := &named{name: name, file: l.file, ptr: l.ptr}
	m.types[name] = n
	m.byLoc[l.String()] = name
	return n
}

func isStructish(node map[string]any) bool {
	if _, ok := node["properties"]; ok {
		return true
	}
	if l, ok := node["allOf"].([]any); ok && len(l) > 0 {
		t, _ := node["type"].(string)
		return t == "object" || t == ""
	}
	return false
}

func isEmptyClosed(node map[string]any) bool {
	ap, ok := node["additionalProperties"].(bool)
	return str(node, "type") == "object" && ok && !ap
}

func isPureRef(im map[string]any) bool {
	_, ok := im["$ref"]
	return ok && im["if"] == nil
}

// fill 填一个具名类型的内容。
func (m *model) fill(n *named, node map[string]any) {
	if n.done {
		return
	}
	n.done = true
	n.doc = oneLine(str(node, "description"))
	if n.file == "common.schema.json" && n.ptr == "/$defs/Timestamp" {
		n.kind = kTimestamp
		return
	}
	if node["x-open"] == true {
		n.kind = kOpenMap
		return
	}
	if isStructish(node) || isEmptyClosed(node) {
		u := m.unionOf(n, node)
		if u != nil && u.field == "" {
			n.kind, n.union = kWholeUnion, u
			return
		}
		n.kind = kStruct
		n.fields = m.fieldsOf(n, node)
		if u != nil {
			n.union = u
			found := false
			for _, f := range n.fields {
				if f.json == u.field {
					f.unionField, found = true, true
				}
				if f.json == u.disc {
					f.disc, f.t = true, &typeRef{name: u.discType}
				}
			}
			if !found {
				m.errs.add("%s：联合字段 %s 没有在 properties 里声明", loc{n.file, n.ptr}, u.field)
			}
		}
		return
	}
	t, _ := m.anon(n.file, n.ptr, node, n.name)
	if t.name != "" {
		m.errs.add("%s：$defs 直接等于另一个类型 %s，生成器不支持", loc{n.file, n.ptr}, t.name)
		return
	}
	n.kind, n.elem = t.kind, t.elem
	if t.kind == kString {
		n.enum = strList(node["enum"])
		n.brand = n.file == "common.schema.json" && n.name == "Ulid"
	}
}

// unionOf 识别判别联合（C.14-R1）；不是联合时返回 nil。
func (m *model) unionOf(n *named, node map[string]any) *union {
	items, _ := node["allOf"].([]any)
	var u *union
	others := false
	for _, it := range items {
		im, _ := it.(map[string]any)
		if isPureRef(im) {
			continue
		}
		disc, val, fieldName, target, ok := discBranch(im)
		if !ok {
			others = true
			continue
		}
		if u == nil {
			u = &union{disc: disc, field: fieldName}
		}
		if u.disc != disc || u.field != fieldName {
			m.errs.add("%s：判别联合的分支写法不一致（C.14-R1）", loc{n.file, n.ptr})
			return nil
		}
		tr := m.refType(n.file, target, "")
		if tr.name == "" {
			m.errs.add("%s：分支 %s 的 %s 不是具名类型（C.14-R1）", loc{n.file, n.ptr}, val, target)
			return nil
		}
		u.branches = append(u.branches, branch{val, tr.name})
	}
	if u == nil {
		return nil
	}
	if others {
		m.errs.add("%s：allOf 里判别分支和其他 if/then 混在一起，生成器分不清（C.14-R1）", loc{n.file, n.ptr})
		return nil
	}
	sort.Slice(u.branches, func(i, j int) bool { return u.branches[i].value < u.branches[j].value })
	for i := 1; i < len(u.branches); i++ {
		if u.branches[i].value == u.branches[i-1].value {
			m.errs.add("%s：判别值 %s 有两个分支", loc{n.file, n.ptr}, u.branches[i].value)
		}
	}
	dnode, dloc := m.propNode(n.file, n.ptr, node, u.disc)
	if dnode == nil {
		m.errs.add("%s：判别字段 %s 没有在 properties 里声明", loc{n.file, n.ptr}, u.disc)
		return nil
	}
	var values []string
	if r := str(dnode, "$ref"); r != "" {
		tr := m.refType(dloc.file, r, "")
		u.discType = tr.name
		if _, target, err := m.set.resolve(dloc.file, r); err == nil {
			values = strList(target["enum"])
		}
	} else {
		name := n.name + goName(u.disc)
		if d := m.claim(name, dloc); d != nil {
			d.done, d.kind, d.enum, d.doc = true, kString, strList(dnode["enum"]), oneLine(str(dnode, "description"))
		}
		u.discType = name
		values = strList(dnode["enum"])
	}
	var got []string
	for _, b := range u.branches {
		got = append(got, b.value)
	}
	sort.Strings(values)
	if strings.Join(values, "\x00") != strings.Join(got, "\x00") {
		m.errs.add("%s：判别字段 %s 的枚举 %v 和分支 %v 不一致（C.14-R1）", loc{n.file, n.ptr}, u.disc, values, got)
	}
	return u
}

// discBranch 认一条 allOf 项是不是判别分支：if 只写判别字段的 const，then 是 $ref（整体联合）或某个字段是 $ref（字段联合）。
func discBranch(im map[string]any) (disc, val, fieldName, target string, ok bool) {
	ifs, then := obj(im, "if"), obj(im, "then")
	if ifs == nil || then == nil {
		return "", "", "", "", false
	}
	req, props := strList(ifs["required"]), obj(ifs, "properties")
	if len(req) != 1 || len(props) != 1 {
		return "", "", "", "", false
	}
	p := obj(props, req[0])
	v, isStr := p["const"].(string)
	if p == nil || !isStr {
		return "", "", "", "", false
	}
	if r := str(then, "$ref"); r != "" && len(then) == 1 {
		return req[0], v, "", r, true
	}
	tp := obj(then, "properties")
	for _, k := range sortedKeys(tp) {
		if r := str(obj(tp, k), "$ref"); r != "" {
			if target != "" {
				return "", "", "", "", false
			}
			fieldName, target = k, r
		}
	}
	if target == "" {
		return "", "", "", "", false
	}
	return req[0], v, fieldName, target, true
}

// propNode 找字段 name 的 schema：先找本对象的 properties，再找 allOf 里 $ref 合并进来的对象。
func (m *model) propNode(file, ptr string, node map[string]any, name string) (map[string]any, loc) {
	if p := obj(obj(node, "properties"), name); p != nil {
		return p, loc{file, ptrJoin(ptr, "properties", name)}
	}
	items, _ := node["allOf"].([]any)
	for _, it := range items {
		im, _ := it.(map[string]any)
		if !isPureRef(im) {
			continue
		}
		if l, base, err := m.set.resolve(file, str(im, "$ref")); err == nil {
			if p, pl := m.propNode(l.file, l.ptr, base, name); p != nil {
				return p, pl
			}
		}
	}
	return nil, loc{}
}

type prop struct {
	l     loc
	node  map[string]any
	owner string
}

// collectProps 收集字段：allOf 里 $ref 的对象先收，本对象写的同名字段覆盖（C.14 §3）。
func (m *model) collectProps(file, ptr string, node map[string]any, owner string, props map[string]prop, req map[string]bool) {
	items, _ := node["allOf"].([]any)
	for _, it := range items {
		im, _ := it.(map[string]any)
		if !isPureRef(im) {
			continue
		}
		l, base, err := m.set.resolve(file, str(im, "$ref"))
		if err != nil {
			m.errs.add("%v", err)
			continue
		}
		m.collectProps(l.file, l.ptr, base, m.byLoc[l.String()], props, req)
	}
	ps := obj(node, "properties")
	for _, k := range sortedKeys(ps) {
		pn, ok := ps[k].(map[string]any)
		if !ok {
			m.errs.add("%s：字段 %s 的 schema 不是对象", loc{file, ptr}, k)
			continue
		}
		props[k] = prop{loc{file, ptrJoin(ptr, "properties", k)}, pn, owner}
	}
	for _, r := range strList(node["required"]) {
		req[r] = true
	}
}

func (m *model) fieldsOf(n *named, node map[string]any) []*field {
	props, req := map[string]prop{}, map[string]bool{}
	m.collectProps(n.file, n.ptr, node, n.name, props, req)
	var out []*field
	seen := map[string]string{}
	for _, k := range sortedKeys(props) {
		p := props[k]
		if err := checkFieldName(k); err != nil {
			m.errs.add("%s：%v", p.l, err)
			continue
		}
		g := goName(k)
		if o, dup := seen[g]; dup {
			m.errs.add("%s：字段 %s 和 %s 的 Go 名字都是 %s", n.name, o, k, g)
			continue
		}
		seen[g] = k
		owner := p.owner
		if owner == "" {
			owner = n.name
		}
		t, nullable := m.anon(p.l.file, p.l.ptr, p.node, owner+g)
		f := &field{json: k, goName: g, doc: oneLine(str(p.node, "description")), t: t, required: req[k], nullable: nullable}
		switch d := p.node["default"].(type) {
		case string, bool, json.Number:
			f.def = d
		}
		out = append(out, f)
	}
	return out
}

func litsOf(node map[string]any) []string {
	if e, ok := node["enum"].([]any); ok {
		var out []string
		for _, v := range e {
			s, ok := v.(string)
			if !ok {
				return nil
			}
			out = append(out, s)
		}
		return out
	}
	if c, ok := node["const"].(string); ok {
		return []string{c}
	}
	return nil
}

// anon 是一个属性或元素的类型；第二个返回值表示可空（C.14 §3）。
func (m *model) anon(file, ptr string, node map[string]any, hint string) (*typeRef, bool) {
	if r := str(node, "$ref"); r != "" {
		return m.refType(file, r, hint), false
	}
	if l, ok := node["anyOf"].([]any); ok {
		if len(l) == 2 {
			for i, it := range l {
				im, _ := it.(map[string]any)
				if str(im, "type") == "null" && len(im) == 1 {
					other, _ := l[1-i].(map[string]any)
					t, _ := m.anon(file, ptrJoin(ptr, "anyOf", strconv.Itoa(1-i)), other, hint)
					return t, true
				}
			}
		}
		return &typeRef{kind: kRaw}, false
	}
	if node["x-open"] == true {
		return &typeRef{kind: kOpenMap}, false
	}
	switch tv := node["type"].(type) {
	case []any:
		ts := strList(tv)
		sort.Strings(ts)
		if strings.Join(ts, ",") == "number,string" {
			return &typeRef{kind: kStrOrNum}, false
		}
		if len(ts) == 2 && (ts[0] == "null" || ts[1] == "null") {
			other := ts[0]
			if other == "null" {
				other = ts[1]
			}
			cp := map[string]any{}
			for k, v := range node {
				cp[k] = v
			}
			cp["type"] = other
			t, _ := m.anon(file, ptr, cp, hint)
			return t, true
		}
		m.errs.add("%s：type %v 生成器不支持（C.14 §3）", loc{file, ptr}, ts)
		return &typeRef{kind: kRaw}, false
	case string:
		switch tv {
		case "object":
			if isStructish(node) || isEmptyClosed(node) {
				return m.inline(file, ptr, node, hint), false
			}
			if ap, ok := node["additionalProperties"].(map[string]any); ok {
				e, _ := m.anon(file, ptrJoin(ptr, "additionalProperties"), ap, hint+"Value")
				return &typeRef{kind: kMap, elem: e}, false
			}
			return &typeRef{kind: kOpenMap}, false
		case "array":
			items, ok := node["items"].(map[string]any)
			if !ok {
				return &typeRef{kind: kArray, elem: &typeRef{kind: kRaw}}, false
			}
			e, _ := m.anon(file, ptrJoin(ptr, "items"), items, hint+"Item")
			return &typeRef{kind: kArray, elem: e}, false
		case "string":
			return &typeRef{kind: kString, lits: litsOf(node)}, false
		case "integer":
			return &typeRef{kind: kInt}, false
		case "number":
			return &typeRef{kind: kNumber}, false
		case "boolean":
			return &typeRef{kind: kBool}, false
		}
		m.errs.add("%s：type %q 生成器不支持（C.14 §3）", loc{file, ptr}, tv)
		return &typeRef{kind: kRaw}, false
	}
	if isStructish(node) {
		return m.inline(file, ptr, node, hint), false
	}
	if lits := litsOf(node); lits != nil {
		return &typeRef{kind: kString, lits: lits}, false
	}
	return &typeRef{kind: kRaw}, false
}

// inline 是内联对象的具名类型：同一个位置只登记一次。
func (m *model) inline(file, ptr string, node map[string]any, hint string) *typeRef {
	key := loc{file, ptr}.String()
	if name, ok := m.byLoc[key]; ok {
		m.fill(m.types[name], node)
		return &typeRef{name: name}
	}
	if hint == "" {
		m.errs.add("%s：内联对象没有名字可取", key)
		return &typeRef{kind: kRaw}
	}
	n := m.claim(hint, loc{file, ptr})
	if n == nil {
		return &typeRef{kind: kRaw}
	}
	m.fill(n, node)
	return &typeRef{name: hint}
}

// refType 是 $ref 指向的类型：指向 $defs、根时是那个具名类型；指到 properties 里时按位置取名（NoteSize）。
func (m *model) refType(from, ref, hint string) *typeRef {
	l, node, err := m.set.resolve(from, ref)
	if err != nil {
		m.errs.add("%v", err)
		return &typeRef{kind: kRaw}
	}
	if name, ok := m.byLoc[l.String()]; ok {
		return &typeRef{name: name}
	}
	name := m.nameForLoc(l)
	if name == "" {
		name = hint
	}
	t, _ := m.anon(l.file, l.ptr, node, name)
	return t
}

// nameForLoc 按位置给内联对象取名：最近的具名祖先 + 沿途的字段名（items 加 Item，additionalProperties 加 Value）。
func (m *model) nameForLoc(l loc) string {
	toks := strings.Split(strings.TrimPrefix(l.ptr, "/"), "/")
	if l.ptr == "" {
		toks = nil
	}
	for i := len(toks); i >= 0; i-- {
		prefix := ""
		if i > 0 {
			prefix = "/" + strings.Join(toks[:i], "/")
		}
		name, ok := m.byLoc[loc{l.file, prefix}.String()]
		if !ok {
			continue
		}
		rest := toks[i:]
		for j := 0; j < len(rest); j++ {
			switch rest[j] {
			case "properties":
				if j+1 < len(rest) {
					name += goName(strings.ReplaceAll(strings.ReplaceAll(rest[j+1], "~1", "/"), "~0", "~"))
					j++
				}
			case "items":
				name += "Item"
			case "additionalProperties":
				name += "Value"
			case "anyOf":
				j++
			}
		}
		return name
	}
	return ""
}

// kindOf 是类型的形状（具名类型取它自己的形状）。
func (m *model) kindOf(t *typeRef) kind {
	if t.name != "" {
		if n, ok := m.types[t.name]; ok {
			return n.kind
		}
	}
	return t.kind
}
