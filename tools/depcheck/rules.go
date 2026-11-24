package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
)

// Rule 是 rules.yaml 里的一条规则（设计文档 G.03.01 §4）。
type Rule struct {
	Pkg      string   `yaml:"pkg"`
	Allow    []string `yaml:"allow"`
	Deny     []string `yaml:"deny"`
	Except   []string `yaml:"except"`
	Tests    bool     `yaml:"tests"`
	Why      string   `yaml:"why"`
	HasAllow bool     `yaml:"-"` // 写了 allow（包括 allow: []）
}

type rulesFile struct {
	Rules []Rule `yaml:"rules"`
}

// LoadRules 解析并校验规则文件；name 是文件名，写进错误里。
func LoadRules(b []byte, name string) ([]Rule, error) {
	var f rulesFile
	if err := yaml.UnmarshalWithOptions(b, &f, yaml.DisallowUnknownField()); err != nil {
		return nil, posError(name, err)
	}
	// allow: [] 和不写 allow 要分开：再按 map 读一遍，看键在不在
	var raw struct {
		Rules []map[string]any `yaml:"rules"`
	}
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return nil, posError(name, err)
	}
	if len(f.Rules) == 0 {
		return nil, fmt.Errorf("depcheck: %s 里没有规则（要写 rules:）", name)
	}
	for i := range f.Rules {
		r := &f.Rules[i]
		if i < len(raw.Rules) {
			_, r.HasAllow = raw.Rules[i]["allow"]
		}
		at := func(field, msg string) error {
			return fmt.Errorf("depcheck: %s 第 %d 条的 %s：%s", name, i+1, field, msg)
		}
		if r.Pkg == "" {
			return nil, at("pkg", "必填")
		}
		if strings.TrimSpace(r.Why) == "" {
			return nil, at("why", "必填（违规时显示的理由）")
		}
		if !r.HasAllow && len(r.Deny) == 0 {
			return nil, at("allow / deny", "至少写一个")
		}
		for field, ps := range map[string][]string{"pkg": {r.Pkg}, "allow": r.Allow, "deny": r.Deny, "except": r.Except} {
			for _, p := range ps {
				if err := checkPattern(p); err != nil {
					return nil, at(field, err.Error())
				}
			}
		}
	}
	return f.Rules, nil
}

// checkPattern：不能为空、不能以 / 开头或结尾、不能有空段；* 只能出现在末尾的 /**。
func checkPattern(p string) error {
	if p == "" {
		return errors.New("模式不能为空")
	}
	if strings.HasPrefix(p, "/") || strings.HasSuffix(p, "/") {
		return fmt.Errorf("模式 %q 不能以 / 开头或结尾", p)
	}
	segs := strings.Split(p, "/")
	for i, s := range segs {
		switch {
		case s == "":
			return fmt.Errorf("模式 %q 有空段", p)
		case s == "**" && i == len(segs)-1 && i > 0:
		case strings.Contains(s, "*"):
			return fmt.Errorf("模式 %q 里 ** 只能写在末尾（x/**），不支持别的通配", p)
		}
	}
	return nil
}

// match：x/** 匹配 x 本身和它下面的所有包；其他写法完全相同才匹配。
func match(pattern, path string) bool {
	if base, ok := strings.CutSuffix(pattern, "/**"); ok {
		return path == base || strings.HasPrefix(path, base+"/")
	}
	return path == pattern
}

func matchAny(patterns []string, path string) bool {
	for _, p := range patterns {
		if match(p, path) {
			return true
		}
	}
	return false
}

var posRe = regexp.MustCompile(`\[(\d+):(\d+)\]`)

// posError 把 goccy 的错误（"[行:列] 原因"）写成"<文件> 第 N 行：原因"。
func posError(file string, err error) error {
	msg := strings.TrimSpace(strings.SplitN(err.Error(), "\n", 2)[0])
	if m := posRe.FindStringSubmatch(msg); m != nil {
		return fmt.Errorf("depcheck: %s 第 %s 行：%s", file, m[1], strings.TrimSpace(posRe.ReplaceAllString(msg, "")))
	}
	return fmt.Errorf("depcheck: %s：%s", file, msg)
}
