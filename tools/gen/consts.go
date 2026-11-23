package main

import (
	"fmt"
	"strings"
)

// constGroup 是一组常量：Go 里一个 const 块，TS 里一个常量对象（C.14-R6、§5.2）。
type constGroup struct {
	goType  string // Go 常量的类型；为空时是无类型常量
	tsName  string // TS 常量对象名
	inTypes bool   // TS 常量对象和类型同名，放在 types.ts 里类型旁边
	doc     string
	items   []constItem
}

type constItem struct{ goName, tsKey, value string }

type constSet struct {
	groups []*constGroup
	byType map[string]*constGroup // 具名枚举的类型名 → 它的常量组（同名放 types.ts 的）
}

// 有特殊前缀的具名枚举（C.14-R6）。
var constPrefixes = map[string][2]string{ // 类型名 → {Go 前缀, TS 常量对象名}
	"OpKind":       {"Op", "OpKind"},
	"ApiErrorCode": {"ErrCode", "ErrCode"},
	"DiagCode":     {"Diag", "DiagCode"},
	"Action":       {"Act", "Act"},
}

func isEventKind(n *named) bool {
	return strings.HasSuffix(n.name, "EventKind") && strings.HasSuffix(n.file, "-event.schema.json")
}

func collectConsts(m *model, nodes map[string]string, sse []string) (*constSet, error) {
	cs := &constSet{byType: map[string]*constGroup{}}
	var errs genErrors
	ev := &constGroup{tsName: "Ev", doc: "三条事件流的事件名（C.06）"}
	for _, name := range sortedKeys(m.types) {
		n := m.types[name]
		if n.kind != kString || len(n.enum) == 0 {
			continue
		}
		goPrefix, tsName := name, name
		if p, ok := constPrefixes[name]; ok {
			goPrefix, tsName = p[0], p[1]
		}
		g := &constGroup{goType: name, tsName: tsName, inTypes: tsName == name, doc: name + " 的取值"}
		if isEventKind(n) {
			goPrefix, g.tsName, g.inTypes = "Ev", "", false
		}
		for _, v := range n.enum {
			gk, err := constName(v, true)
			if err != nil {
				errs.add("%s：%v", name, err)
				continue
			}
			tk, _ := constName(v, false)
			it := constItem{goPrefix + gk, tk, v}
			g.items = append(g.items, it)
			if isEventKind(n) {
				ev.items = append(ev.items, it)
			}
		}
		cs.groups = append(cs.groups, g)
		if g.inTypes {
			cs.byType[name] = g
		}
	}
	if len(ev.items) > 0 {
		cs.groups = append(cs.groups, ev)
	}
	if len(nodes) > 0 {
		g := &constGroup{tsName: "NodeType", doc: "原子节点类型名（C.14-R7，别名见 schema/builtin/aliases.yaml）"}
		if _, ok := m.types["NodeTypeRef"]; ok {
			g.goType = "NodeTypeRef"
		}
		for _, k := range sortedKeys(nodes) {
			if !identRe.MatchString(nodes[k]) {
				errs.add("aliases.yaml：节点 %s 的别名 %q 不是合法的标识符", k, nodes[k])
				continue
			}
			g.items = append(g.items, constItem{"NodeType" + nodes[k], nodes[k], k})
		}
		cs.groups = append(cs.groups, g)
	}
	if len(sse) > 0 {
		g := &constGroup{tsName: "SseControl", doc: "SSE 控制帧名（C.06.01 §4）：不是事件"}
		for _, v := range sse {
			k, err := constName(v, false)
			if err != nil {
				errs.add("sse_control.yaml：%v", err)
				continue
			}
			g.items = append(g.items, constItem{"Sse" + k, k, v})
		}
		cs.groups = append(cs.groups, g)
	}
	// 重名检查：Go 常量之间、和类型名；TS 常量对象名、对象内的键
	goNames := map[string]string{}
	tsNames := map[string]bool{}
	for _, g := range cs.groups {
		if g.tsName != "" {
			if tsNames[g.tsName] {
				errs.add("TS 常量对象 %s 重名", g.tsName)
			}
			tsNames[g.tsName] = true
			if _, isType := m.types[g.tsName]; isType && !g.inTypes {
				errs.add("TS 常量对象 %s 和类型重名", g.tsName)
			}
		}
		keys := map[string]bool{}
		for _, it := range g.items {
			if g.tsName != "" {
				if keys[it.tsKey] {
					errs.add("TS 常量对象 %s 里的键 %s 重名", g.tsName, it.tsKey)
				}
				keys[it.tsKey] = true
			}
			if g.tsName == "Ev" {
				continue // 事件名已经按流各算过一次 Go 常量
			}
			if o, dup := goNames[it.goName]; dup {
				errs.add("Go 常量 %s 重名（%s 和 %s）", it.goName, o, it.value)
			}
			goNames[it.goName] = it.value
			if _, isType := m.types[it.goName]; isType {
				errs.add("Go 常量 %s 和类型重名", it.goName)
			}
		}
	}
	return cs, errs.err()
}

func emitGoConsts(m *model, cs *constSet) ([]byte, error) {
	var w strings.Builder
	w.WriteString(goHeader)
	w.WriteString("package gen\n\n")
	for _, g := range cs.groups {
		if g.tsName == "Ev" {
			continue
		}
		fmt.Fprintf(&w, "// %s（C.14-R6）。\nconst (\n", g.doc)
		for _, it := range g.items {
			if g.goType != "" {
				fmt.Fprintf(&w, "\t%s %s = %q\n", it.goName, g.goType, it.value)
			} else {
				fmt.Fprintf(&w, "\t%s = %q\n", it.goName, it.value)
			}
		}
		w.WriteString(")\n\n")
	}
	return formatGo("gen/consts.go", w.String())
}

func tsConstObject(w *strings.Builder, g *constGroup) {
	fmt.Fprintf(w, "/** %s（C.14-R6） */\nexport const %s = {\n", g.doc, g.tsName)
	for _, it := range g.items {
		fmt.Fprintf(w, "  %s: %s,\n", it.tsKey, tsString(it.value))
	}
	w.WriteString("} as const;\n\n")
}

// emitTSConsts 是 consts.ts：和类型不同名的常量对象（Ev、ErrCode、Act、NodeType、SseControl）。
func emitTSConsts(cs *constSet) []byte {
	var w strings.Builder
	w.WriteString(tsHeader)
	for _, g := range cs.groups {
		if g.tsName == "" || g.inTypes {
			continue
		}
		tsConstObject(&w, g)
	}
	return []byte(strings.TrimRight(w.String(), "\n") + "\n")
}
