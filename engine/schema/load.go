package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/mio-cc/anywhere-door/gen"
)

const (
	docBase  = "https://anydoor.local/schema/" // C.01-R2：跨文件引用只从本地解析
	typeBase = "https://anydoor.local/types/"
)

// builtinDirs 是内置项的目录、对应的 schema 和注册方法，按这个顺序加载（E.03.01 §5）。
var builtinDirs = []struct {
	dir, schema string
	add         func(st *state, j []byte) (string, error)
}{
	{"builtin/types", "typedef.schema.json", func(st *state, j []byte) (string, error) {
		var t gen.TypeDef
		if err := json.Unmarshal(j, &t); err != nil {
			return "", err
		}
		return t.Name, st.addType(builtinOwner, &t)
	}},
	{"builtin/recipes", "recipe.schema.json", func(st *state, j []byte) (string, error) {
		var c gen.Recipe
		if err := json.Unmarshal(j, &c); err != nil {
			return "", err
		}
		return c.Name, st.addRecipe(builtinOwner, &c)
	}},
	{"builtin/rolecards", "rolecard.schema.json", func(st *state, j []byte) (string, error) {
		var c gen.Rolecard
		if err := json.Unmarshal(j, &c); err != nil {
			return "", err
		}
		return c.Name, st.addRoleCard(builtinOwner, &c)
	}},
	{"builtin/nodedefs", "nodedef.schema.json", func(st *state, j []byte) (string, error) {
		var d gen.NodeDef
		if err := json.Unmarshal(j, &d); err != nil {
			return "", err
		}
		return d.Name, st.addNodeDef(builtinOwner, &d)
	}},
}

// LoadBuiltin 加载内置内容（E.03.01 §5）。fsys 的根是仓库的 schema/。遇到错误继续加载其他文件，最后返回全部错误（Errors）；
// 有错误时注册表不变。
func (r *Registry) LoadBuiltin(fsys fs.FS) error {
	st := newState()
	docs, errs := compileDocs(fsys)
	st.docs = docs
	if m, err := LoadMessages(fsys, r.log); err != nil {
		errs = append(errs, &RegistryError{Msg: err.Error()})
	} else {
		st.msgs = m
	}
	for _, d := range builtinDirs {
		files, _ := fs.Glob(fsys, d.dir+"/*.yaml")
		sort.Strings(files)
		for _, f := range files {
			j, rerr := readItem(fsys, f, d.schema, docs[d.schema])
			if rerr != nil {
				errs = append(errs, rerr)
				continue
			}
			item, err := d.add(st, j)
			if err != nil {
				errs = append(errs, &RegistryError{File: f, Line: 1, Item: item, Msg: err.Error()})
			}
		}
	}
	errs = append(errs, st.link()...)
	if len(errs) > 0 {
		return errs
	}
	r.mu.Lock()
	r.cur = st
	r.mu.Unlock()
	r.log.Info("内置注册表已加载", "nodedefs", len(st.nodedefs), "types", len(st.types), "recipes", len(st.recipes), "rolecards", len(st.rolecards), "hash", st.hash)
	return nil
}

func compileDocs(fsys fs.FS) (map[string]*jsonschema.Schema, Errors) {
	docs := map[string]*jsonschema.Schema{}
	var errs Errors
	names, _ := fs.Glob(fsys, "*.schema.json")
	sort.Strings(names)
	if len(names) == 0 {
		return docs, Errors{{Msg: "没有找到 *.schema.json（fsys 的根应当是仓库的 schema/）"}}
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat()
	var ok []string
	for _, name := range names {
		b, err := fs.ReadFile(fsys, name)
		if err != nil {
			errs = append(errs, &RegistryError{File: name, Msg: "读不了：" + err.Error()})
			continue
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
		if err != nil {
			errs = append(errs, &RegistryError{File: name, Msg: "不是合法的 JSON：" + err.Error()})
			continue
		}
		if err := c.AddResource(docBase+name, doc); err != nil {
			errs = append(errs, &RegistryError{File: name, Msg: "登记失败：" + err.Error()})
			continue
		}
		ok = append(ok, name)
	}
	for _, name := range ok {
		s, err := c.Compile(docBase + name)
		if err != nil {
			errs = append(errs, &RegistryError{File: name, Msg: "编译失败：" + err.Error()})
			continue
		}
		docs[name] = s
	}
	return docs, errs
}

// yamlPos 从 goccy 的错误里取行号和原因。
func yamlPos(err error) (int, string) {
	msg := strings.TrimSpace(strings.SplitN(err.Error(), "\n", 2)[0])
	m := posRe.FindStringSubmatch(msg)
	if m == nil {
		return 0, msg
	}
	line, _ := strconv.Atoi(m[1])
	return line, strings.TrimSpace(posRe.ReplaceAllString(msg, ""))
}

// readItem 读一个 YAML 文件，按 schema 校验，返回 JSON。出错时带文件名和行号（C.13-R1）。
func readItem(fsys fs.FS, name, schemaName string, sch *jsonschema.Schema) ([]byte, *RegistryError) {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, &RegistryError{File: name, Msg: "读不了：" + err.Error()}
	}
	f, err := parser.ParseBytes(b, 0)
	if err != nil {
		line, msg := yamlPos(err)
		return nil, &RegistryError{File: name, Line: line, Msg: "YAML 格式不对：" + msg}
	}
	j, err := yaml.YAMLToJSON(b)
	if err != nil {
		line, msg := yamlPos(err)
		return nil, &RegistryError{File: name, Line: line, Msg: "YAML 格式不对：" + msg}
	}
	if sch == nil {
		return nil, &RegistryError{File: name, Msg: "没有可用的 " + schemaName}
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(j))
	if err != nil {
		return nil, &RegistryError{File: name, Msg: "不是合法的 JSON：" + err.Error()}
	}
	if err := sch.Validate(v); err != nil {
		var ve *jsonschema.ValidationError
		if !errors.As(err, &ve) {
			return nil, &RegistryError{File: name, Msg: err.Error()}
		}
		loc, msg := firstLeaf(ve)
		return nil, &RegistryError{File: name, Line: lineOf(f, loc), Msg: "不符合 " + schemaName + "：" + msg}
	}
	return j, nil
}

func pointer(tokens []string) string {
	var b strings.Builder
	for _, t := range tokens {
		b.WriteString("/" + strings.ReplaceAll(strings.ReplaceAll(t, "~", "~0"), "/", "~1"))
	}
	return b.String()
}

// firstLeaf 是位置最靠前的最底层原因：它的位置和说明。
func firstLeaf(ve *jsonschema.ValidationError) ([]string, string) {
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
	sort.SliceStable(leaves, func(i, j int) bool { return pointer(leaves[i].InstanceLocation) < pointer(leaves[j].InstanceLocation) })
	leaf := leaves[0]
	at := pointer(leaf.InstanceLocation)
	lines := strings.Split(ve.Error(), "\n")
	msg := strings.TrimSpace(lines[len(lines)-1])
	for _, l := range lines {
		if strings.Contains(l, "'"+at+"'") {
			msg = strings.TrimSpace(l)
			break
		}
	}
	msg = strings.TrimPrefix(msg, "- ")
	if at == "" {
		at = "/"
	}
	return leaf.InstanceLocation, "位置 " + at + "：" + msg
}

// lineOf 是 YAML 里 loc 对应节点的行号；找不到时退到上一级，最后是第 1 行。
func lineOf(f *ast.File, loc []string) int {
	for k := len(loc); k > 0; k-- {
		b := (&yaml.PathBuilder{}).Root()
		for _, s := range loc[:k] {
			if n, err := strconv.Atoi(s); err == nil && n >= 0 {
				b = b.Index(uint(n))
			} else {
				b = b.Child(s)
			}
		}
		node, err := b.Build().FilterFile(f)
		if err == nil && node != nil && node.GetToken() != nil {
			return node.GetToken().Position.Line
		}
	}
	return 1
}

// checkNodeDef 是节点定义自身的检查（E.03.01 §5 第 2 条）。
func checkNodeDef(d *gen.NodeDef) error {
	var probs []string
	if n := len(d.Ports.Inputs); n > 8 {
		probs = append(probs, fmt.Sprintf("输入端口有 %d 个，最多 8 个（C.03.01-R5）", n))
	}
	if n := len(d.Ports.Outputs); n > 8 {
		probs = append(probs, fmt.Sprintf("输出端口有 %d 个，最多 8 个（C.03.01-R5）", n))
	}
	for _, name := range sortedKeys(d.Params) {
		p := d.Params[name]
		if _, ok := d.Ports.Inputs[name]; ok {
			probs = append(probs, "参数 "+name+" 和输入端口重名（C.03.01-R3）")
		}
		if _, ok := d.Ports.Outputs[name]; ok {
			probs = append(probs, "参数 "+name+" 和输出端口重名（C.03.01-R3）")
		}
		if p.UI != nil && p.UI.Widget != nil && !widgetOK(p.Type, *p.UI.Widget) {
			probs = append(probs, "参数 "+name+" 的控件 "+*p.UI.Widget+" 不能用于 "+p.Type+" 类型（C.03.02-R2）")
		}
		if p.Secret != nil && *p.Secret {
			if p.Type != "string" {
				probs = append(probs, "密钥参数 "+name+" 只能是 string 类型（C.03.02-R3）")
			}
			var def string
			if len(p.Default) > 0 && json.Unmarshal(p.Default, &def) == nil && LooksLikeSecret(def) {
				probs = append(probs, "密钥参数 "+name+" 的缺省值像密钥值，只能写密钥名（C.03.02-R3）")
			}
		}
	}
	if len(probs) > 0 {
		return errors.New(strings.Join(probs, "；"))
	}
	return nil
}

// widgetOK：显式指定的控件能不能用于这种参数（按 widget_map.yaml，C.03.02 §4、R2）。connection 只用于 string。
func widgetOK(typ, w string) bool {
	switch w {
	case "auto":
		return true
	case "connection":
		return typ == "string"
	}
	for _, r := range gen.WidgetRules {
		if !r.Subform && r.Widget == w && slices.Contains(r.Types, typ) {
			return true
		}
	}
	return false
}

// LooksLikeSecret：看起来像密钥值（以 sk-、AIza 开头，或长度 > 40 且没有空白），C.03.02-R3。
func LooksLikeSecret(s string) bool {
	if strings.HasPrefix(s, "sk-") || strings.HasPrefix(s, "AIza") {
		return true
	}
	return utf8.RuneCountInString(s) > 40 && !strings.ContainsAny(s, " \t\r\n")
}

// typeKeys 是一个 TypeRef 里引用的命名类型（名称@主版本）；任意、类型变量没有。写法不对时返回错误。
func typeKeys(ref string) ([]string, error) {
	for strings.HasPrefix(ref, "列表[") {
		if !strings.HasSuffix(ref, "]") {
			return nil, fmt.Errorf("类型写法不对：%s", ref)
		}
		ref = strings.TrimSuffix(strings.TrimPrefix(ref, "列表["), "]")
	}
	if ref == "任意" || (len(ref) == 2 && ref[0] == '$' && ref[1] >= 'A' && ref[1] <= 'Z') {
		return nil, nil
	}
	name, major, ok := splitNamed(ref)
	if !ok {
		return nil, fmt.Errorf("类型写法不对：%s", ref)
	}
	return []string{typeKey(name, major)}, nil
}

// checkNodeDefTypes：端口、动态端口和 type_ref 参数的缺省值引用的类型都已注册。
func (st *state) checkNodeDefTypes(d *gen.NodeDef) []string {
	var probs []string
	check := func(what, ref string) {
		keys, err := typeKeys(ref)
		if err != nil {
			probs = append(probs, what+"："+err.Error())
			return
		}
		for _, k := range keys {
			if len(st.types[k]) == 0 {
				probs = append(probs, what+" 引用的类型 "+k+" 没有注册")
			}
		}
	}
	for _, name := range sortedKeys(d.Ports.Inputs) {
		check("输入端口 "+name, string(d.Ports.Inputs[name].Type))
	}
	for _, name := range sortedKeys(d.Ports.Outputs) {
		check("输出端口 "+name, string(d.Ports.Outputs[name].Type))
	}
	if d.DynamicPorts != nil && d.DynamicPorts.Type != nil {
		check("动态端口", string(*d.DynamicPorts.Type))
	}
	for _, name := range sortedKeys(d.Params) {
		p := d.Params[name]
		var def string
		if p.Type == "type_ref" && len(p.Default) > 0 && json.Unmarshal(p.Default, &def) == nil {
			check("参数 "+name+" 的缺省值", def)
		}
	}
	return probs
}

func typeURL(key string) string { return typeBase + url.PathEscape(key) + ".json" }

// compileTypes 为每个类型（每个主版本取最高次版本）生成 JSON Schema 并编译（C.13.03-R3）。
func (st *state) compileTypes() Errors {
	var errs Errors
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	keys := sortedKeys(st.types)
	var ok []string
	for _, k := range keys {
		l := st.types[k]
		doc, err := typeSchemaDoc(l[len(l)-1])
		if err == nil {
			err = c.AddResource(typeURL(k), doc)
		}
		if err != nil {
			errs = append(errs, &RegistryError{Item: k, Msg: "生成 JSON Schema 失败：" + err.Error()})
			continue
		}
		ok = append(ok, k)
	}
	out := map[string]*jsonschema.Schema{}
	for _, k := range ok {
		s, err := c.Compile(typeURL(k))
		if err != nil {
			errs = append(errs, &RegistryError{Item: k, Msg: "编译 JSON Schema 失败：" + err.Error()})
			continue
		}
		out[k] = s
	}
	st.typeSchemas = out
	return errs
}

// typeSchemaDoc 是类型的 JSON Schema（经 JSON 往返，交给 jsonschema 用）。
func typeSchemaDoc(t *gen.TypeDef) (any, error) {
	var doc map[string]any
	if t.Scalar != nil {
		doc = map[string]any{"type": *t.Scalar}
	} else {
		obj, err := fieldsSchema(t.Fields)
		if err != nil {
			return nil, err
		}
		doc = obj
		if t.Extends != nil {
			keys, err := typeKeys(string(*t.Extends))
			if err != nil || len(keys) != 1 {
				return nil, fmt.Errorf("extends 写法不对：%s", *t.Extends)
			}
			doc = map[string]any{"allOf": []any{map[string]any{"$ref": typeURL(keys[0])}, obj}}
		}
	}
	doc["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	b, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}

func fieldsSchema(fields *[]gen.Field) (map[string]any, error) {
	props := map[string]any{}
	var req []string
	if fields != nil {
		for _, f := range *fields {
			s, err := fieldSchema(f.Type, f)
			if err != nil {
				return nil, fmt.Errorf("字段 %s：%w", f.Name, err)
			}
			props[f.Name] = s
			if f.Required != nil && *f.Required {
				req = append(req, f.Name)
			}
		}
	}
	out := map[string]any{"type": "object", "properties": props}
	if len(req) > 0 {
		out["required"] = req
	}
	return out, nil
}

func fieldSchema(typ string, f gen.Field) (map[string]any, error) {
	switch {
	case typ == "任意":
		return map[string]any{}, nil
	case strings.HasPrefix(typ, "列表[") && strings.HasSuffix(typ, "]"):
		items, err := fieldSchema(strings.TrimSuffix(strings.TrimPrefix(typ, "列表["), "]"), f)
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "array", "items": items}, nil
	case typ == "string":
		s := map[string]any{"type": "string"}
		if f.Enum != nil {
			s["enum"] = *f.Enum
		}
		if f.MaxChars != nil {
			s["maxLength"] = *f.MaxChars
		}
		return s, nil
	case typ == "integer", typ == "number", typ == "boolean":
		return map[string]any{"type": typ}, nil
	case typ == "object":
		if f.Fields != nil {
			return fieldsSchema(f.Fields)
		}
		return map[string]any{"type": "object"}, nil
	}
	keys, err := typeKeys(typ)
	if err != nil || len(keys) != 1 {
		return nil, fmt.Errorf("字段类型写法不对：%s", typ)
	}
	return map[string]any{"$ref": typeURL(keys[0])}, nil
}
