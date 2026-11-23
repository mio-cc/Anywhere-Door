package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
)

// schemaSet 是 schema 目录下的全部 *.schema.json。
type schemaSet struct {
	names []string                  // 文件名，排好序
	files map[string]map[string]any // 文件名 → 内容（数字保留原文，json.Number）
}

func loadSchemas(dir string) (*schemaSet, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	s := &schemaSet{files: map[string]map[string]any{}}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".schema.json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		m, err := parseJSONObject(b)
		if err != nil {
			return nil, fmt.Errorf("%s：%w", e.Name(), err)
		}
		s.files[e.Name()] = m
		s.names = append(s.names, e.Name())
	}
	sort.Strings(s.names)
	return s, nil
}

// newSchemaSet 用于测试：直接给出文件内容。
func newSchemaSet(files map[string]string) (*schemaSet, error) {
	s := &schemaSet{files: map[string]map[string]any{}}
	for _, name := range sortedKeys(files) {
		m, err := parseJSONObject([]byte(files[name]))
		if err != nil {
			return nil, fmt.Errorf("%s：%w", name, err)
		}
		s.files[name] = m
		s.names = append(s.names, name)
	}
	return s, nil
}

func parseJSONObject(b []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

// loc 是 schema 里的一个位置：文件名和 JSON Pointer（根为空串）。
type loc struct{ file, ptr string }

func (l loc) String() string { return l.file + "#" + l.ptr }

// resolve 解析 from 文件里的 $ref，返回目标位置和那里的 schema。
func (s *schemaSet) resolve(from, ref string) (loc, map[string]any, error) {
	file, frag, _ := strings.Cut(ref, "#")
	if file == "" {
		file = from
	}
	root, ok := s.files[file]
	if !ok {
		return loc{}, nil, fmt.Errorf("%s：$ref %q 指向的文件不存在", from, ref)
	}
	var node any = root
	if frag != "" {
		for _, tok := range strings.Split(strings.TrimPrefix(frag, "/"), "/") {
			tok = strings.ReplaceAll(strings.ReplaceAll(tok, "~1", "/"), "~0", "~")
			m, ok := node.(map[string]any)
			if !ok {
				return loc{}, nil, fmt.Errorf("%s：$ref %q 解析不到", from, ref)
			}
			if node, ok = m[tok]; !ok {
				return loc{}, nil, fmt.Errorf("%s：$ref %q 解析不到", from, ref)
			}
		}
	}
	m, ok := node.(map[string]any)
	if !ok {
		return loc{}, nil, fmt.Errorf("%s：$ref %q 指向的不是 schema 对象", from, ref)
	}
	return loc{file, frag}, m, nil
}

func ptrJoin(ptr string, toks ...string) string {
	for _, t := range toks {
		ptr += "/" + strings.ReplaceAll(strings.ReplaceAll(t, "~", "~0"), "/", "~1")
	}
	return ptr
}

// aliases 是 schema/builtin/aliases.yaml（C.14-R5、R7）。
type aliases struct {
	Roots map[string]string `yaml:"roots"`
	Nodes map[string]string `yaml:"nodes"`
}

func readAliases(path string) (*aliases, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var a aliases
	if err := yaml.UnmarshalWithOptions(b, &a, yaml.DisallowUnknownField()); err != nil {
		return nil, fmt.Errorf("%s：%w", filepath.Base(path), err)
	}
	return &a, nil
}

func readStringList(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var l []string
	if err := yaml.Unmarshal(b, &l); err != nil {
		return nil, fmt.Errorf("%s：%w", filepath.Base(path), err)
	}
	return l, nil
}

// str 取 m[k] 的字符串值。
func str(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return s
}

func obj(m map[string]any, k string) map[string]any {
	o, _ := m[k].(map[string]any)
	return o
}

func strList(v any) []string {
	l, _ := v.([]any)
	var out []string
	for _, x := range l {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
