package tools

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

var (
	toolIDRe   = regexp.MustCompile(`^(builtin:[a-z0-9_]{1,64}|(mcp|ext):[^/\s]{1,64}/[^/\s]{1,128}|skill:[^\s/]{1,64})$`)
	toolNameRe = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)
)

// MinRisk 是内置工具的风险下限（S.04 §3）；内置工具加一个就加一行。
var MinRisk = map[string]Risk{
	"builtin:ws_read": RiskL0,
}

// Registry 是工具注册表（E.11-R1）：内置、MCP、扩展包、技能都注册到这里。
type Registry struct {
	mu sync.RWMutex
	m  map[string]Impl
}

// NewRegistry 返回空的注册表。
func NewRegistry() *Registry { return &Registry{m: map[string]Impl{}} }

// Register 注册一个工具：校验 ID、Name、风险下限，ID 不能重复。
func (r *Registry) Register(i Impl) error {
	d := i.Def()
	if !toolIDRe.MatchString(d.ID) {
		return fmt.Errorf("工具 ID 写法不对：%q", d.ID)
	}
	if !toolNameRe.MatchString(d.Name) {
		return fmt.Errorf("工具 %s 的名字 %q 只能用小写字母、数字和下划线，最多 64 个字符", d.ID, d.Name)
	}
	if d.Risk > RiskL3 {
		return fmt.Errorf("工具 %s 的风险等级不对：%d", d.ID, d.Risk)
	}
	floor, ok := MinRisk[d.ID]
	if !ok && strings.HasPrefix(d.ID, "builtin:") {
		return fmt.Errorf("内置工具 %s 没有登记风险下限（S.04 §3）", d.ID)
	}
	if ok && d.Risk < floor {
		return fmt.Errorf("工具 %s 声明的风险 %s 低于下限 %s（S.04 §3）", d.ID, d.Risk, floor)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.m[d.ID]; dup {
		return fmt.Errorf("工具 %s 已经注册过", d.ID)
	}
	r.m[d.ID] = i
	return nil
}

// Get 按 ID 取工具。
func (r *Registry) Get(id string) (Impl, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	i, ok := r.m[id]
	return i, ok
}

// List 返回所有工具的定义，按 ID 排序。
func (r *Registry) List() []Def {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Def, 0, len(r.m))
	for _, i := range r.m {
		out = append(out, i.Def())
	}
	sort.Slice(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	return out
}
