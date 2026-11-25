// Package schema 是节点定义、类型、配方、角色卡的注册表，端口类型检查与类型变量推断，图校验和诊断（设计文档 E.03）。
//
// 只 import gen、engine/graph（E.03-R3）；内置的 schema 文件、YAML 数据和文案由调用方以 fs.FS 传进来，
// 根是仓库的 schema/ 目录（E.03-R4）。节点的性质一律从 NodeDef、TypeDef 和生成的表读，不比较节点类型名（INV-02）。
package schema

import (
	"fmt"
	"io/fs"
	"log/slog"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
)

// 文案文件（C.11 §6、C.04.02 §5；本包只读）。
const (
	DiagsFile  = "builtin/messages/diags.zh.yaml"
	AssignFile = "builtin/messages/assign.zh.yaml"
)

// Messages 是诊断和连线判定的中文文案（E.03.04）。
type Messages struct {
	diags  map[string]map[string]string // 诊断码 → 级别 → 模板
	assign map[string]string            // C.04.02 规则号 → 模板
	log    *slog.Logger
}

// LoadMessages 读 fsys 里的 diags.zh.yaml、assign.zh.yaml；log 为 nil 时丢弃日志。
func LoadMessages(fsys fs.FS, log *slog.Logger) (*Messages, error) {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	m := &Messages{log: log}
	if err := readYAML(fsys, DiagsFile, &m.diags); err != nil {
		return nil, err
	}
	if err := readYAML(fsys, AssignFile, &m.assign); err != nil {
		return nil, err
	}
	return m, nil
}

func readYAML(fsys fs.FS, name string, v any) error {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		return fmt.Errorf("schema: 读不了 %s：%w", name, err)
	}
	if err := yaml.UnmarshalWithOptions(b, v, yaml.DisallowUnknownField()); err != nil {
		return posError(name, err)
	}
	return nil
}

var posRe = regexp.MustCompile(`\[(\d+):(\d+)\]`)

// posError 把 goccy 的错误（"[行:列] 原因"）写成"<文件> 第 N 行：原因"。
func posError(file string, err error) error {
	msg := strings.TrimSpace(strings.SplitN(err.Error(), "\n", 2)[0])
	if m := posRe.FindStringSubmatch(msg); m != nil {
		return fmt.Errorf("schema: %s 第 %s 行：%s", file, m[1], strings.TrimSpace(posRe.ReplaceAllString(msg, "")))
	}
	return fmt.Errorf("schema: %s：%s", file, msg)
}

var varRe = regexp.MustCompile(`\{([^{}\s]+)\}`)

// render 按模板替换 {变量}；缺的变量原样保留并记 warn（E.03.04 §5）。
func (m *Messages) render(tpl string, vars map[string]any, what string) string {
	return varRe.ReplaceAllStringFunc(tpl, func(s string) string {
		k := s[1 : len(s)-1]
		v, ok := vars[k]
		if !ok {
			m.log.Warn("文案缺少变量", "msg", what, "var", k)
			return s
		}
		return fmt.Sprint(v)
	})
}

// Assign 是 C.04.02 规则号对应的判定原因文案；没有这条规则的文案时为 ""。
func (m *Messages) Assign(rule string, vars map[string]any) string {
	tpl, ok := m.assign[rule]
	if !ok {
		return ""
	}
	return m.render(tpl, vars, rule)
}

// Levels 是这个诊断码允许的级别（排序：error 在前）。
func (m *Messages) Levels(code string) []string {
	var out []string
	for _, l := range []string{"error", "warn"} {
		if _, ok := m.diags[code][l]; ok {
			out = append(out, l)
		}
	}
	return out
}
