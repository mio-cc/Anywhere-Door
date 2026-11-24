// Package quirks 是厂商差异表（设计文档 E.10.05）：各家模型的能力、错误规则和常用厂商的填法都写成数据，
// 协议代码只查表、不写特判（INV-18、E.10-R2）。数据文件打进二进制（Files）。
package quirks

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/mio-cc/anywhere-door/engine/llm"
	"github.com/mio-cc/anywhere-door/gen"
)

// Files 是内置的 quirks.yaml 和 vendors.yaml。
//
//go:embed quirks.yaml vendors.yaml
var Files embed.FS

// 数据文件名。
const (
	QuirksFile  = "quirks.yaml"
	VendorsFile = "vendors.yaml"
)

// Table 是能力表和错误规则，实现 llm.CapsTable。
type Table struct {
	models []modelRule
	errors []errorRule
}

type modelRule struct {
	protocol, model string
	o               llm.CapsOverride
}

type errorRule struct {
	protocol string
	status   int
	code     string
	message  *regexp.Regexp
	class    string
}

type matchYAML struct {
	Protocol string `yaml:"protocol"`
	Model    string `yaml:"model"`
	Status   int    `yaml:"status"`
	Code     string `yaml:"code"`
	Message  string `yaml:"message"`
}

type capsYAML struct {
	Window        *int    `yaml:"window"`
	MaxOutput     *int    `yaml:"max_output"`
	Tools         *bool   `yaml:"tools"`
	ParallelTools *bool   `yaml:"parallel_tools"`
	Vision        *bool   `yaml:"vision"`
	PromptCache   *bool   `yaml:"prompt_cache"`
	NativeSchema  *bool   `yaml:"native_schema"`
	Tokenizer     *string `yaml:"tokenizer"`
}

type reasoningYAML struct {
	Values []string          `yaml:"values"`
	Map    map[string]string `yaml:"map"`
	Budget map[string]int    `yaml:"budget"`
}

type fileYAML struct {
	Models []struct {
		Match     matchYAML      `yaml:"match"`
		Caps      *capsYAML      `yaml:"caps"`
		Reasoning *reasoningYAML `yaml:"reasoning"`
	} `yaml:"models"`
	Errors []struct {
		Match matchYAML `yaml:"match"`
		Class string    `yaml:"class"`
	} `yaml:"errors"`
}

var classes = []gen.FailureClass{
	gen.FailureClassTransient, gen.FailureClassRateLimited, gen.FailureClassFormatInvalid, gen.FailureClassBudgetExceeded,
	gen.FailureClassAuthDenied, gen.FailureClassUserInput, gen.FailureClassStructural, gen.FailureClassTimeout,
	gen.FailureClassCancelled, gen.FailureClassInternal,
}

// Load 读 fsys 里的 quirks.yaml；格式错时报出文件和行号（启动失败，B.01 §3）。
func Load(fsys fs.FS) (Table, error) {
	b, err := fs.ReadFile(fsys, QuirksFile)
	if err != nil {
		return Table{}, fmt.Errorf("quirks: 读不了 %s：%w", QuirksFile, err)
	}
	var f fileYAML
	if err := yaml.UnmarshalWithOptions(b, &f, yaml.DisallowUnknownField()); err != nil {
		return Table{}, posError(QuirksFile, err)
	}
	var t Table
	for i, m := range f.Models {
		at := func(msg string) error { return fmt.Errorf("quirks: %s 的 models[%d]：%s", QuirksFile, i, msg) }
		if err := checkPattern(m.Match.Protocol, m.Match.Model); err != nil {
			return Table{}, at(err.Error())
		}
		if m.Match.Status != 0 || m.Match.Code != "" || m.Match.Message != "" {
			return Table{}, at("models 的 match 只能写 protocol、model")
		}
		r := modelRule{protocol: m.Match.Protocol, model: m.Match.Model}
		if c := m.Caps; c != nil {
			r.o = llm.CapsOverride{Window: c.Window, MaxOutput: c.MaxOutput, Tools: c.Tools, ParallelTools: c.ParallelTools,
				Vision: c.Vision, PromptCache: c.PromptCache, NativeSchema: c.NativeSchema, Tokenizer: c.Tokenizer}
		}
		if rs := m.Reasoning; rs != nil {
			if err := checkReasoning(rs); err != nil {
				return Table{}, at(err.Error())
			}
			r.o.Efforts = append([]string{}, rs.Values...)
			r.o.EffortMap = map[string]string{}
			for k, v := range rs.Map {
				r.o.EffortMap[k] = v
			}
			r.o.EffortBudget = map[string]int{}
			for k, v := range rs.Budget {
				r.o.EffortBudget[k] = v
			}
		}
		t.models = append(t.models, r)
	}
	for i, e := range f.Errors {
		at := func(msg string) error { return fmt.Errorf("quirks: %s 的 errors[%d]：%s", QuirksFile, i, msg) }
		if err := checkPattern(e.Match.Protocol, ""); err != nil {
			return Table{}, at(err.Error())
		}
		if e.Match.Model != "" {
			return Table{}, at("errors 的 match 不能写 model")
		}
		if !slices.Contains(classes, gen.FailureClass(e.Class)) {
			return Table{}, at(fmt.Sprintf("不认识的 class %q", e.Class))
		}
		r := errorRule{protocol: e.Match.Protocol, status: e.Match.Status, code: e.Match.Code, class: e.Class}
		if e.Match.Message != "" {
			re, err := regexp.Compile(e.Match.Message)
			if err != nil {
				return Table{}, at(fmt.Sprintf("message 不是合法的正则：%v", err))
			}
			r.message = re
		}
		t.errors = append(t.errors, r)
	}
	return t, nil
}

func checkPattern(ps ...string) error {
	for _, p := range ps {
		if _, err := path.Match(p, ""); err != nil {
			return fmt.Errorf("%q 不是合法的通配", p)
		}
	}
	return nil
}

func checkReasoning(r *reasoningYAML) error {
	if len(r.Values) == 0 {
		return fmt.Errorf("reasoning.values 不能为空（不支持调节就不写 reasoning）")
	}
	for _, v := range r.Values {
		if !slices.Contains(llm.EffortLevels, v) {
			return fmt.Errorf("reasoning.values 里的 %q 不是 %s 之一", v, strings.Join(llm.EffortLevels, " / "))
		}
	}
	for k, v := range r.Map {
		if !slices.Contains(llm.EffortLevels, k) || !slices.Contains(r.Values, v) {
			return fmt.Errorf("reasoning.map 的 %s: %s 要从 7 级之一映射到 values 里的一级", k, v)
		}
	}
	for k, v := range r.Budget {
		if !slices.Contains(r.Values, k) || v <= 0 {
			return fmt.Errorf("reasoning.budget 的 %s 要是 values 里的一级，预算为正数", k)
		}
	}
	return nil
}

func matches(pattern, s string) bool {
	if pattern == "" {
		return true
	}
	ok, _ := path.Match(pattern, s)
	return ok
}

// Lookup 按文件顺序累积所有匹配的规则（E.10.05 §5）。
func (t Table) Lookup(protocol, model string) llm.Caps {
	var c llm.Caps
	for i := range t.models {
		r := &t.models[i]
		if matches(r.protocol, protocol) && matches(r.model, model) {
			c = c.Apply(&r.o)
		}
	}
	return c
}

// ErrorClass 返回第一条匹配的错误规则的类别。
func (t Table) ErrorClass(protocol string, status int, code, message string) (string, bool) {
	for _, r := range t.errors {
		if !matches(r.protocol, protocol) || (r.status != 0 && r.status != status) || (r.code != "" && r.code != code) {
			continue
		}
		if r.message != nil && !r.message.MatchString(message) {
			continue
		}
		return r.class, true
	}
	return "", false
}

// Vendor 是一个常用厂商的填法（E.10.05 §4）。
type Vendor struct {
	ID, Name string
	Provider llm.ProviderConfig // 不含 name 和密钥名，由用户填
	Models   []string
}

// Vendors 读 fsys 里的 vendors.yaml。
func Vendors(fsys fs.FS) ([]Vendor, error) {
	b, err := fs.ReadFile(fsys, VendorsFile)
	if err != nil {
		return nil, fmt.Errorf("quirks: 读不了 %s：%w", VendorsFile, err)
	}
	var raw []struct {
		ID       string         `yaml:"id"`
		Name     string         `yaml:"name"`
		Provider map[string]any `yaml:"provider"`
		Models   []string       `yaml:"models"`
	}
	if err := yaml.UnmarshalWithOptions(b, &raw, yaml.DisallowUnknownField()); err != nil {
		return nil, posError(VendorsFile, err)
	}
	var out []Vendor
	seen := map[string]bool{}
	for i, r := range raw {
		at := func(msg string) error {
			return fmt.Errorf("quirks: %s 的第 %d 项（%s）：%s", VendorsFile, i+1, r.ID, msg)
		}
		if r.ID == "" || r.Name == "" {
			return nil, at("要写 id 和 name")
		}
		if seen[r.ID] {
			return nil, at("id 重名")
		}
		seen[r.ID] = true
		j, err := json.Marshal(jsonable(r.Provider))
		if err != nil {
			return nil, at(err.Error())
		}
		dec := json.NewDecoder(bytes.NewReader(j))
		dec.DisallowUnknownFields()
		var p llm.ProviderConfig
		if err := dec.Decode(&p); err != nil {
			return nil, at("provider：" + err.Error())
		}
		if p.Name != "" || p.Auth.Secret != "" {
			return nil, at("provider 里不写 name 和密钥名（由用户填）")
		}
		p.Vendor = r.ID
		out = append(out, Vendor{ID: r.ID, Name: r.Name, Provider: p, Models: r.Models})
	}
	return out, nil
}

// jsonable 把 YAML 解出的值里的 map[any]any 换成 map[string]any，encoding/json 才能序列化。
func jsonable(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[k] = jsonable(e)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[fmt.Sprint(k)] = jsonable(e)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = jsonable(e)
		}
		return out
	}
	return v
}

var posRe = regexp.MustCompile(`\[(\d+):(\d+)\]`)

// posError 把 goccy 的错误（"[行:列] 原因"）写成"<文件> 第 N 行：原因"。
func posError(file string, err error) error {
	msg := strings.TrimSpace(strings.SplitN(err.Error(), "\n", 2)[0])
	if m := posRe.FindStringSubmatch(msg); m != nil {
		return fmt.Errorf("quirks: %s 第 %s 行：%s", file, m[1], strings.TrimSpace(posRe.ReplaceAllString(msg, "")))
	}
	return fmt.Errorf("quirks: %s：%s", file, msg)
}
