package schema

import (
	"sort"

	"github.com/mio-cc/anywhere-door/gen"
)

// Loc 是诊断的位置；空字符串的字段不写进 Diag。
type Loc struct{ Scope, Node, Port, Param string }

// Diag 生成一条诊断（E.03.04）：msg 按诊断码和级别从 diags.zh.yaml 渲染。没有对应的文案时 msg 写成诊断码本身，并记 warn。
func (m *Messages) Diag(code gen.DiagCode, level gen.DiagLevel, at Loc, vars map[string]any) gen.Diag {
	d := gen.Diag{Code: code, Level: level}
	if tpl, ok := m.diags[string(code)][string(level)]; ok {
		d.Msg = m.render(tpl, vars, string(code))
	} else {
		m.log.Warn("诊断码没有这个级别的文案", "code", code, "level", level)
		d.Msg = string(code)
	}
	if at.Scope != "" {
		s := at.Scope
		d.Scope = &s
	}
	if at.Node != "" {
		n := gen.Ulid(at.Node)
		d.Node = &n
	}
	if at.Port != "" {
		p := at.Port
		d.Port = &p
	}
	if at.Param != "" {
		p := at.Param
		d.Param = &p
	}
	return d
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func ulid(p *gen.Ulid) string {
	if p == nil {
		return ""
	}
	return string(*p)
}

func levelRank(l gen.DiagLevel) int {
	if l == gen.DiagLevelError {
		return 0
	}
	return 1
}

// Sort 按 级别（error 在前）→ scope → node → code → port → param → msg 排序，结果确定。
func Sort(ds []gen.Diag) {
	sort.SliceStable(ds, func(i, j int) bool {
		a, b := ds[i], ds[j]
		if ra, rb := levelRank(a.Level), levelRank(b.Level); ra != rb {
			return ra < rb
		}
		ka := [...]string{str(a.Scope), ulid(a.Node), string(a.Code), str(a.Port), str(a.Param), a.Msg}
		kb := [...]string{str(b.Scope), ulid(b.Node), string(b.Code), str(b.Port), str(b.Param), b.Msg}
		for k := range ka {
			if ka[k] != kb[k] {
				return ka[k] < kb[k]
			}
		}
		return false
	})
}

// Dedup 去掉同诊断码、同位置的重复诊断，保留先出现的一条（不改原切片）。
func Dedup(ds []gen.Diag) []gen.Diag {
	seen := map[[5]string]bool{}
	out := make([]gen.Diag, 0, len(ds))
	for _, d := range ds {
		k := [5]string{string(d.Code), str(d.Scope), ulid(d.Node), str(d.Port), str(d.Param)}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, d)
	}
	return out
}

// HasError 判断有没有 error 级别的诊断。
func HasError(ds []gen.Diag) bool {
	for _, d := range ds {
		if d.Level == gen.DiagLevelError {
			return true
		}
	}
	return false
}
