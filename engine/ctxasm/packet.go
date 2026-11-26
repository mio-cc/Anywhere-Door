package ctxasm

import (
	"encoding/json"

	"github.com/mio-cc/anywhere-door/engine/blob"
	"github.com/mio-cc/anywhere-door/gen"
)

// PacketSection 是上下文包里的一段（E.13.02 §4）。
type PacketSection struct {
	Name, Text, Source string
	Ref                blob.Ref // 原文在工作区的引用（指针形式、被收缩过时有）
	Tokens             int
	Taint, Locked      bool
	TaintSrc           []string
}

// Packet 是一次装配的结果（上下文包@1，E.13.02 §4）。
type Packet struct {
	Recipe   string // 名称@版本
	Sections []PacketSection
	Tokens   int
	Budget   int
	Clamped  bool
	Dropped  []Dropped
	Vars     map[string]string // 配方段的 var（C.13.01-R6）
	Inherit  map[string]any    // 透传约束（E.13.06）
	Taint    bool
	TaintSrc []string
	Pointers bool // 有段是指针形式：Agent 要能用 ws_read（E.14.02 注入）
	Warnings []string
}

// Value 按 上下文包@1 的字段给出包的值：sections[{name, content, tokens, source}]、tokens、dropped、inherit、vars。
func (p Packet) Value() map[string]any {
	secs := make([]any, 0, len(p.Sections))
	for _, s := range p.Sections {
		m := map[string]any{"name": s.Name, "content": s.Text, "tokens": s.Tokens}
		if s.Source != "" {
			m["source"] = s.Source
		}
		secs = append(secs, m)
	}
	inherit := map[string]any{}
	for k, v := range p.Inherit {
		inherit[k] = v
	}
	out := map[string]any{"sections": secs, "tokens": p.Tokens, "dropped": droppedList(p.Dropped), "inherit": inherit}
	if len(p.Vars) > 0 {
		vars := map[string]any{}
		for k, v := range p.Vars {
			vars[k] = v
		}
		out["vars"] = vars
	}
	return out
}

func droppedList(ds []Dropped) []any {
	out := make([]any, 0, len(ds))
	for _, d := range ds {
		m := map[string]any{"section": d.Section, "mode": d.Mode, "saved": d.Saved}
		if d.Ref != "" {
			m["ref"] = string(d.Ref)
		}
		out = append(out, m)
	}
	return out
}

type eventSection struct {
	Name   string   `json:"name"`
	Tokens int      `json:"tokens"`
	Source string   `json:"source,omitempty"`
	Ref    blob.Ref `json:"ref,omitempty"`
	Taint  bool     `json:"taint"`
	Locked bool     `json:"locked"`
}

// Event 是 ctx.assembled 事件的数据（E.13-R2）：sections[{name, tokens, source, ref, taint, locked}]、dropped、budget_tokens。
// 不含段的内容，事件由执行器写（E.14.03）。
func (p Packet) Event() gen.DataCtxAssembled {
	secs := make([]eventSection, 0, len(p.Sections))
	for _, s := range p.Sections {
		secs = append(secs, eventSection{Name: s.Name, Tokens: s.Tokens, Source: s.Source, Ref: s.Ref, Taint: s.Taint, Locked: s.Locked})
	}
	dropped := p.Dropped
	if dropped == nil {
		dropped = []Dropped{}
	}
	sb, _ := json.Marshal(secs)
	db, _ := json.Marshal(dropped)
	return gen.DataCtxAssembled{BudgetTokens: int64(p.Budget), Sections: sb, Dropped: db}
}
