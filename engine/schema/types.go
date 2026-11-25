package schema

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mio-cc/anywhere-door/gen"
)

// Kind 是 TypeRef 的种类（C.04.01 §2）。
type Kind int

// TypeRef 的四种写法。
const (
	Any   Kind = iota // 任意
	Named             // 方案@1、方案@1.2
	List              // 列表[X]
	Var               // $T
)

// Type 是解析后的 TypeRef。
type Type struct {
	Kind         Kind
	Name         string
	Major, Minor int
	Elem         *Type
	Var          string

	minorSet bool // 原文写了次版本
}

// ErrBadTypeRef：类型写法不对。
var ErrBadTypeRef = errors.New("schema: 类型写法不对")

// Parse 解析 TypeRef（C.04.01 §2）：任意 | $大写字母 | 列表[TypeRef] | 名称@主版本[.次版本]，名称 1–30 个字符、不含 @ [ ] / 空白。
func Parse(ref string) (*Type, error) {
	t, err := parse(ref)
	if err != nil {
		return nil, fmt.Errorf("%w：%q", ErrBadTypeRef, ref)
	}
	return t, nil
}

func parse(ref string) (*Type, error) {
	switch {
	case ref == "任意":
		return &Type{Kind: Any}, nil
	case strings.HasPrefix(ref, "$"):
		if len(ref) == 2 && ref[1] >= 'A' && ref[1] <= 'Z' {
			return &Type{Kind: Var, Var: ref[1:]}, nil
		}
		return nil, ErrBadTypeRef
	case strings.HasPrefix(ref, "列表["):
		if !strings.HasSuffix(ref, "]") {
			return nil, ErrBadTypeRef
		}
		inner := strings.TrimSuffix(strings.TrimPrefix(ref, "列表["), "]")
		if inner == "" {
			return nil, ErrBadTypeRef
		}
		e, err := parse(inner)
		if err != nil {
			return nil, err
		}
		return &Type{Kind: List, Elem: e}, nil
	}
	name, ver, ok := strings.Cut(ref, "@")
	if !ok || name == "" || utf8.RuneCountInString(name) > 30 || strings.ContainsAny(name, "@[]/") || strings.IndexFunc(name, unicode.IsSpace) >= 0 {
		return nil, ErrBadTypeRef
	}
	majS, minS, hasMinor := strings.Cut(ver, ".")
	major, err := digits(majS)
	if err != nil {
		return nil, err
	}
	t := &Type{Kind: Named, Name: name, Major: major}
	if hasMinor {
		if t.Minor, err = digits(minS); err != nil {
			return nil, err
		}
		t.minorSet = true
	}
	return t, nil
}

func digits(s string) (int, error) {
	if s == "" {
		return 0, ErrBadTypeRef
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, ErrBadTypeRef
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, ErrBadTypeRef
	}
	return n, nil
}

// String 写回 TypeRef 的写法（与 Parse 往返）。
func (t *Type) String() string {
	if t == nil {
		return ""
	}
	switch t.Kind {
	case Any:
		return "任意"
	case Var:
		return "$" + t.Var
	case List:
		return "列表[" + t.Elem.String() + "]"
	}
	s := t.Name + "@" + strconv.Itoa(t.Major)
	if t.minorSet {
		s += "." + strconv.Itoa(t.Minor)
	}
	return s
}

// key 是命名类型的查找键（名称@主版本）。
func (t *Type) key() string { return t.Name + "@" + strconv.Itoa(t.Major) }

// HasVar：类型里还有类型变量。
func (t *Type) HasVar() bool {
	switch {
	case t == nil:
		return false
	case t.Kind == Var:
		return true
	case t.Kind == List:
		return t.Elem.HasVar()
	}
	return false
}

// Subst 把类型变量换成 binds 里的类型；没有绑定的变量原样保留。返回新的值，不改 t。
func (t *Type) Subst(binds map[string]*Type) *Type {
	switch {
	case t == nil:
		return nil
	case t.Kind == Var:
		if b, ok := binds[t.Var]; ok && b != nil {
			return b
		}
	case t.Kind == List:
		c := *t
		c.Elem = t.Elem.Subst(binds)
		return &c
	}
	return t
}

// anyVars 把剩下的类型变量都换成 任意（推断不出时按任意处理，C.04.03 §3 第 6 步）。
func (t *Type) anyVars() *Type {
	switch {
	case t == nil:
		return nil
	case t.Kind == Var:
		return &Type{Kind: Any}
	case t.Kind == List:
		c := *t
		c.Elem = t.Elem.anyVars()
		return &c
	}
	return t
}

// Category 是端口的显示类别（C.04.01-R4）：portCategory 写了就用它，否则按 gen.CategoryRules 从类型推出。
func Category(t *Type, portCategory string, _ View) string {
	if portCategory != "" {
		return portCategory
	}
	for _, r := range gen.CategoryRules {
		switch {
		case r.Default:
			return r.Category
		case r.List:
			if t != nil && t.Kind == List {
				return r.Category
			}
		case t != nil && t.Kind == Any && r.Type == "任意", t != nil && t.Kind == Named && t.Name == r.Type:
			return r.Category
		}
	}
	return ""
}
