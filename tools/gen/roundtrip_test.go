package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/mio-cc/anywhere-door/gen"
)

// semEqual：两段 JSON 语义相同。
func semEqual(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}

var (
	rawType = reflect.TypeOf(json.RawMessage(nil))
	tsType  = reflect.TypeOf(gen.Timestamp{})
)

// rtChecker 做往返检查，并记下每个判别联合覆盖到的分支（C14-T2）。
type rtChecker struct {
	t    *testing.T
	m    *model
	hits map[string]map[string]bool
	n    int
}

func (c *rtChecker) check(name string, raw []byte, v any) {
	c.t.Helper()
	c.n++
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		c.t.Errorf("%s：严格解码失败：%v", name, err)
		return
	}
	out, err := json.Marshal(v)
	if err != nil {
		c.t.Errorf("%s：编码失败：%v", name, err)
		return
	}
	if !semEqual(raw, out) {
		c.t.Errorf("%s：编码后和原文不同\n  原文：%s\n  编码：%s", name, raw, out)
	}
	c.walk(name, reflect.ValueOf(v))
}

func (c *rtChecker) walk(at string, v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			c.walk(at, v.Elem())
		}
	case reflect.Slice, reflect.Array:
		if v.Type() == rawType {
			return
		}
		for i := 0; i < v.Len(); i++ {
			c.walk(fmt.Sprintf("%s[%d]", at, i), v.Index(i))
		}
	case reflect.Map:
		keys := v.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
		for _, k := range keys {
			c.walk(at+"."+k.String(), v.MapIndex(k))
		}
	case reflect.Struct:
		if v.Type() == tsType {
			return
		}
		if n, ok := c.m.types[v.Type().Name()]; ok && n.union != nil {
			c.decodeUnion(at, n, v)
		}
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				c.walk(at+"."+v.Type().Field(i).Name, v.Field(i))
			}
		}
	}
}

// decodeUnion 解出联合的分支：分支再编码要与原文相同，然后继续往里检查。
func (c *rtChecker) decodeUnion(at string, n *named, v reflect.Value) {
	u := n.union
	method, rawField := "Decode", "Raw"
	if n.kind != kWholeUnion {
		method, rawField = "Decode"+goName(u.field), goName(u.field)
	}
	raw := v.FieldByName(rawField).Bytes()
	kindVal := v.FieldByName(goName(u.disc)).String()
	p := reflect.New(v.Type())
	p.Elem().Set(v)
	res := p.MethodByName(method).Call(nil)
	if err, _ := res[1].Interface().(error); err != nil {
		c.t.Errorf("%s：%s.%s 失败：%v", at, n.name, method, err)
		return
	}
	branch := res[0].Interface()
	out, err := json.Marshal(branch)
	if err != nil || !semEqual(raw, out) {
		c.t.Errorf("%s：%s 的分支 %s 再编码和原文不同（%v）\n  原文：%s\n  编码：%s", at, n.name, kindVal, err, raw, out)
	}
	if c.hits[n.name] == nil {
		c.hits[n.name] = map[string]bool{}
	}
	c.hits[n.name][kindVal] = true
	c.walk(at+"("+kindVal+")", reflect.ValueOf(branch))
}

type vector struct {
	Name    string          `json:"name"`
	Kind    string          `json:"kind"`
	Payload json.RawMessage `json:"payload"`
	Event   json.RawMessage `json:"event"`
	Valid   bool            `json:"valid"`
}

func readVectors(t *testing.T, file string) []vector {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, "schema", "examples", file))
	if err != nil {
		t.Fatal(err)
	}
	var vs []vector
	if err := json.Unmarshal(b, &vs); err != nil {
		t.Fatalf("%s：%v", file, err)
	}
	return vs
}

// exampleTypes 是示例清单里出现的 schema → 生成的类型。清单里出现新的 schema 时在这里补一行。
var exampleTypes = map[string]func() any{
	"graph.schema.json":                              func() any { return new(gen.Graph) },
	"nodedef.schema.json":                            func() any { return new(gen.NodeDef) },
	"template.schema.json":                           func() any { return new(gen.Template) },
	"graph_patch.schema.json":                        func() any { return new(gen.GraphPatch) },
	"graph_patch.schema.json#/$defs/Proposal":        func() any { return new(gen.Proposal) },
	"graph_patch.schema.json#/$defs/ProposalPreview": func() any { return new(gen.ProposalPreview) },
	"session.schema.json#/$defs/Message":             func() any { return new(gen.Message) },
	"session.schema.json#/$defs/Session":             func() any { return new(gen.Session) },
	"mandate.schema.json#/$defs/Mandate":             func() any { return new(gen.Mandate) },
	"common.schema.json#/$defs/Loc":                  func() any { return new(gen.Loc) },
	"errors.schema.json#/$defs/ErrorResponse":        func() any { return new(gen.ErrorResponse) },
	"errors.schema.json#/$defs/Diag":                 func() any { return new(gen.Diag) },
}

// builtinTypes 是 schema/builtin/ 下的资产目录 → 生成的类型。
var builtinTypes = map[string]func() any{
	"nodedefs":  func() any { return new(gen.NodeDef) },
	"types":     func() any { return new(gen.TypeDef) },
	"recipes":   func() any { return new(gen.Recipe) },
	"rolecards": func() any { return new(gen.Rolecard) },
}

func TestRoundTrip(t *testing.T) {
	t.Run("C14-T2 往返", func(t *testing.T) {
		c := &rtChecker{t: t, m: repoModel(t), hits: map[string]map[string]bool{}}
		for _, v := range readVectors(t, "op_vectors.json") {
			if v.Valid {
				raw, _ := json.Marshal(map[string]any{"kind": v.Kind, "payload": v.Payload})
				c.check("op_vectors："+v.Name, raw, new(gen.Op))
			}
		}
		for file, newV := range map[string]func() any{
			"run_event_vectors.json":     func() any { return new(gen.RunEvent) },
			"graph_event_vectors.json":   func() any { return new(gen.GraphEvent) },
			"session_event_vectors.json": func() any { return new(gen.SessionEvent) },
		} {
			for _, v := range readVectors(t, file) {
				if v.Valid {
					c.check(file+"："+v.Name, v.Event, newV())
				}
			}
		}
		mb, err := os.ReadFile(filepath.Join(repoRoot, "schema", "examples", "manifest.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		var entries []struct {
			File   string `yaml:"file"`
			Schema string `yaml:"schema"`
			Valid  bool   `yaml:"valid"`
		}
		if err := yaml.Unmarshal(mb, &entries); err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if !e.Valid {
				continue
			}
			newV, ok := exampleTypes[e.Schema]
			if !ok {
				t.Errorf("示例 %s 的 schema %s 不在 exampleTypes 里，补一行", e.File, e.Schema)
				continue
			}
			raw, err := os.ReadFile(filepath.Join(repoRoot, "schema", "examples", filepath.FromSlash(e.File)))
			if err != nil {
				t.Fatal(err)
			}
			c.check("示例 "+e.File, raw, newV())
		}
		for _, dir := range sortedKeys(builtinTypes) {
			full := filepath.Join(repoRoot, "schema", "builtin", dir)
			ents, err := os.ReadDir(full)
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range ents {
				if !strings.HasSuffix(f.Name(), ".yaml") {
					continue
				}
				b, err := os.ReadFile(filepath.Join(full, f.Name()))
				if err != nil {
					t.Fatal(err)
				}
				raw, err := yaml.YAMLToJSON(b)
				if err != nil {
					t.Fatalf("%s/%s：%v", dir, f.Name(), err)
				}
				c.check("内置 "+dir+"/"+f.Name(), raw, builtinTypes[dir]())
			}
		}
		for _, name := range sortedKeys(c.m.types) {
			n := c.m.types[name]
			if n.union == nil {
				continue
			}
			for _, b := range n.union.branches {
				if !c.hits[name][b.value] {
					t.Errorf("联合 %s 的分支 %s 没有被任何向量、示例或内置资产覆盖（C.14-R4）", name, b.value)
				}
			}
		}
		t.Logf("往返检查了 %d 份 JSON", c.n)
	})
}
