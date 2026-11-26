package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

// 数据表的 YAML 结构（C.14 §5.3）。不认识的键报错。
type containerRuleYAML struct {
	Type     string `yaml:"type"`
	Children string `yaml:"children"`
	Ref      string `yaml:"ref"`
}

type widgetRuleYAML struct {
	Types []string `yaml:"types"`
	When  struct {
		SpanSteps     int64 `yaml:"span_steps"`
		MaxOptions    int64 `yaml:"max_options"`
		MaxLabelChars int64 `yaml:"max_label_chars"`
		HasSchema     bool  `yaml:"has_schema"`
	} `yaml:"when"`
	Widget  string `yaml:"widget"`
	Subform bool   `yaml:"subform"`
}

type categoryMapYAML struct {
	Categories []string `yaml:"categories"`
	Rules      []struct {
		Type     string `yaml:"type"`
		List     bool   `yaml:"list"`
		Default  bool   `yaml:"default"`
		Category string `yaml:"category"`
	} `yaml:"rules"`
}

type authzYAML struct {
	Rows []struct {
		Actions []string `yaml:"actions"`
		Levels  []any    `yaml:"levels"`
	} `yaml:"rows"`
	Fixed     map[string]string `yaml:"fixed"`
	OpActions map[string]string `yaml:"op_actions"`
}

type machineYAML struct {
	States      []string            `yaml:"states"`
	Terminal    []string            `yaml:"terminal"`
	Transitions map[string][]string `yaml:"transitions"`
}

type stateMachineYAML struct {
	Run  machineYAML `yaml:"run"`
	Node machineYAML `yaml:"node"`
}

type shortcutYAML struct {
	ID      string   `yaml:"id"`
	Keys    []string `yaml:"keys"`
	Layer   string   `yaml:"layer"`
	InInput bool     `yaml:"in_input"`
	Hold    bool     `yaml:"hold"`
	Label   string   `yaml:"label"`
	Group   string   `yaml:"group"`
}

var (
	authzDecisions  = []string{"allow", "propose", "ask", "deny"}
	authzConditions = []string{"overrides_ops", "in_overrides_models", "overrides_allow_spawn", "within_tools_max_risk"}
	shortcutLayers  = []string{"global", "canvas", "nest", "composer"}
	reservedKeys    = map[string]bool{
		"Mod+W": true, "Mod+T": true, "Mod+N": true, "Mod+L": true, "Mod+R": true,
		"Mod+Shift+[": true, "Mod+Shift+]": true,
		"F5": true, "F11": true, "F12": true,
	}
	shortcutModRank = map[string]int{"Mod": 0, "Ctrl": 1, "Alt": 2, "Shift": 3}
	shortcutSpecial = map[string]bool{
		"[": true, "]": true, ",": true, ".": true, "/": true,
		"Enter": true, "Escape": true, "Delete": true, "Backspace": true,
		"Space": true, "Tab": true, "ContextMenu": true,
		"ArrowUp": true, "ArrowDown": true, "ArrowLeft": true, "ArrowRight": true,
		"F1": true, "F2": true, "F3": true, "F4": true, "F5": true, "F6": true,
		"F7": true, "F8": true, "F9": true, "F10": true, "F11": true, "F12": true,
	}
)

func validateShortcutCombo(combo string) (hasMod bool, isSinglePrintable bool, err error) {
	if combo == "" || strings.TrimSpace(combo) != combo {
		return false, false, fmt.Errorf("组合 %q 不能为空或含首尾空白", combo)
	}
	parts := strings.Split(combo, "+")
	main := parts[len(parts)-1]
	mods := parts[:len(parts)-1]
	lastRank := -1
	for _, mod := range mods {
		rank, ok := shortcutModRank[mod]
		if !ok {
			return false, false, fmt.Errorf("组合 %q 含未知修饰键 %q（仅允许 Mod/Ctrl/Alt/Shift）", combo, mod)
		}
		if rank <= lastRank {
			return false, false, fmt.Errorf("组合 %q 的修饰键顺序错误或重复（应按 Mod+Ctrl+Alt+Shift，W.02.04 §5.1）", combo)
		}
		lastRank = rank
		hasMod = true
	}
	isLetter := len(main) == 1 && main[0] >= 'A' && main[0] <= 'Z'
	isDigit := len(main) == 1 && main[0] >= '0' && main[0] <= '9'
	isSym := main == "[" || main == "]" || main == "," || main == "." || main == "/"
	if !isLetter && !isDigit && !shortcutSpecial[main] {
		return false, false, fmt.Errorf("组合 %q 含非法主键 %q（W.02.04 §5.1）", combo, main)
	}
	return hasMod, !hasMod && (isLetter || isDigit || isSym), nil
}

func readYAMLStrict(path string, v any) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := yaml.UnmarshalWithOptions(b, v, yaml.DisallowUnknownField()); err != nil {
		return nil, fmt.Errorf("%s：%w", filepath.Base(path), err)
	}
	return b, nil
}

// yamlAsTS 把一份 YAML 原样转成 TS 字面量（as const）。
func yamlAsTS(b []byte) (string, error) {
	j, err := yaml.YAMLToJSON(b)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := json.Indent(&out, j, "", "  "); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func enumOf(m *model, name string) []string {
	if n, ok := m.types[name]; ok {
		return n.enum
	}
	return nil
}

func goStrings(typ string, l []string) string {
	var q []string
	for _, s := range l {
		q = append(q, fmt.Sprintf("%q", s))
	}
	return "[]" + typ + "{" + strings.Join(q, ", ") + "}"
}

// emitTables 生成 gen/tables.go 和 generated/tables.ts（C.14 §5.3）。
func emitTables(m *model, dir string) (goSrc, ts []byte, err error) {
	var errs genErrors
	var gw, tw strings.Builder
	gw.WriteString(goHeader + "package gen\n\n")
	tw.WriteString(tsHeader)
	tw.WriteString("// 来源：schema/builtin/*.yaml（C.14 §5.3），照原样输出，键不转换；只有 lib/protocol 读这些表（W.04.04-R1）。\n")
	tw.WriteString("import type { NodeState, RunStatus } from \"./types\";\n\n")
	addTS := func(name, file, doc string, raw []byte) {
		lit, e := yamlAsTS(raw)
		if e != nil {
			errs.add("%s：%v", file, e)
			return
		}
		fmt.Fprintf(&tw, "/** %s（%s） */\nexport const %s = %s as const;\n\n", doc, file, name, lit)
	}

	// 容器规则
	var cr []containerRuleYAML
	if raw, e := readYAMLStrict(filepath.Join(dir, "container_rules.yaml"), &cr); e != nil {
		errs.add("%v", e)
	} else {
		gw.WriteString("// ContainerRule 是 container_rules.yaml 的一行：节点类型必须 / 可以带子图（C.02.04 §5）。\ntype ContainerRule struct {\n\tType     string\n\tChildren string\n\tRef      string\n}\n\n")
		gw.WriteString("// ContainerRules 是 container_rules.yaml。\nvar ContainerRules = []ContainerRule{\n")
		for _, r := range cr {
			fmt.Fprintf(&gw, "\t{Type: %q, Children: %q, Ref: %q},\n", r.Type, r.Children, r.Ref)
		}
		gw.WriteString("}\n\n")
		addTS("CONTAINER_RULES", "container_rules.yaml", "容器规则（C.02.04 §5）", raw)
	}

	// 控件映射
	var wm struct {
		Rules []widgetRuleYAML `yaml:"rules"`
	}
	if raw, e := readYAMLStrict(filepath.Join(dir, "widget_map.yaml"), &wm); e != nil {
		errs.add("%v", e)
	} else {
		gw.WriteString("// WidgetWhen 是控件映射一条规则的附加条件；0 / false 表示没写（C.03.02 §4）。\ntype WidgetWhen struct {\n\tSpanSteps     int64\n\tMaxOptions    int64\n\tMaxLabelChars int64\n\tHasSchema     bool\n}\n\n")
		gw.WriteString("// WidgetRule 是 widget_map.yaml 的一条规则。\ntype WidgetRule struct {\n\tTypes   []string\n\tWhen    WidgetWhen\n\tWidget  string\n\tSubform bool\n}\n\n")
		gw.WriteString("// WidgetRules 是 widget_map.yaml，按顺序匹配（C.03.02 §4）。\nvar WidgetRules = []WidgetRule{\n")
		for _, r := range wm.Rules {
			fmt.Fprintf(&gw, "\t{Types: %s, When: WidgetWhen{SpanSteps: %d, MaxOptions: %d, MaxLabelChars: %d, HasSchema: %t}, Widget: %q, Subform: %t},\n",
				goStrings("string", r.Types), r.When.SpanSteps, r.When.MaxOptions, r.When.MaxLabelChars, r.When.HasSchema, r.Widget, r.Subform)
		}
		gw.WriteString("}\n\n")
		addTS("WIDGET_MAP", "widget_map.yaml", "参数控件映射（C.03.02 §4）", raw)
	}

	// 显示类别
	var cm categoryMapYAML
	if raw, e := readYAMLStrict(filepath.Join(dir, "category_map.yaml"), &cm); e != nil {
		errs.add("%v", e)
	} else {
		gw.WriteString("// CategoryRule 是 category_map.yaml 的一条规则（C.04.01 §3）。\ntype CategoryRule struct {\n\tType     string\n\tList     bool\n\tDefault  bool\n\tCategory string\n}\n\n")
		fmt.Fprintf(&gw, "// Categories 是 8 个显示类别，与 Category 同序。\nvar Categories = %s\n\n", goStrings("string", cm.Categories))
		gw.WriteString("// CategoryRules 是 category_map.yaml，按顺序匹配。\nvar CategoryRules = []CategoryRule{\n")
		for _, r := range cm.Rules {
			fmt.Fprintf(&gw, "\t{Type: %q, List: %t, Default: %t, Category: %q},\n", r.Type, r.List, r.Default, r.Category)
		}
		gw.WriteString("}\n\n")
		addTS("CATEGORY_MAP", "category_map.yaml", "数据类型 → 显示类别（C.04.01 §3）", raw)
	}

	// 聚合算子
	var ops map[string]string
	if raw, e := readYAMLStrict(filepath.Join(dir, "aggregate_ops.yaml"), &ops); e != nil {
		errs.add("%v", e)
	} else {
		gw.WriteString("// AggregateOps 是 aggregate_ops.yaml：聚合算子 → 输出类型（C.04.03 §2）。\nvar AggregateOps = map[string]string{\n")
		for _, k := range sortedKeys(ops) {
			fmt.Fprintf(&gw, "\t%q: %q,\n", k, ops[k])
		}
		gw.WriteString("}\n\n")
		addTS("AGGREGATE_OPS", "aggregate_ops.yaml", "聚合算子 → 输出类型（C.04.03 §2）", raw)
	}

	// 授权判定表
	var az authzYAML
	if raw, e := readYAMLStrict(filepath.Join(dir, "authz_matrix.yaml"), &az); e != nil {
		errs.add("%v", e)
	} else {
		actions, kinds, actors := enumOf(m, "Action"), enumOf(m, "OpKind"), enumOf(m, "ActorKind")
		gw.WriteString("// AuthzCell 是判定表的一格：Decision 是判定；条件格 Decision 为空，按 If 选 Then 或 Else（C.09 §5.1）。\ntype AuthzCell struct {\n\tDecision string\n\tIf       string\n\tThen     string\n\tElse     string\n}\n\n")
		gw.WriteString("// AuthzRow 是判定表的一行：一组动作在档位 0–4 的判定。\ntype AuthzRow struct {\n\tActions []Action\n\tLevels  [5]AuthzCell\n}\n\n")
		gw.WriteString("// AuthzRows 是 authz_matrix.yaml 的 rows（C.09 §5）。\nvar AuthzRows = []AuthzRow{\n")
		for i, r := range az.Rows {
			for _, a := range r.Actions {
				if !contains(actions, a) {
					errs.add("authz_matrix.yaml rows[%d]：动作 %s 不在 Action 里", i, a)
				}
			}
			if len(r.Levels) != 5 {
				errs.add("authz_matrix.yaml rows[%d]：levels 要正好 5 格", i)
				continue
			}
			var cells []string
			for j, lv := range r.Levels {
				switch c := lv.(type) {
				case string:
					if !contains(authzDecisions, c) {
						errs.add("authz_matrix.yaml rows[%d].levels[%d]：判定 %s 不认识", i, j, c)
					}
					cells = append(cells, fmt.Sprintf("{Decision: %q}", c))
				case map[string]any:
					cond, then, els := fmt.Sprint(c["if"]), fmt.Sprint(c["then"]), fmt.Sprint(c["else"])
					if len(c) != 3 || !contains(authzConditions, cond) || !contains(authzDecisions, then) || !contains(authzDecisions, els) {
						errs.add("authz_matrix.yaml rows[%d].levels[%d]：条件格要写 {if, then, else}，条件只有 4 种", i, j)
					}
					cells = append(cells, fmt.Sprintf("{If: %q, Then: %q, Else: %q}", cond, then, els))
				default:
					errs.add("authz_matrix.yaml rows[%d].levels[%d]：写法不认识", i, j)
					cells = append(cells, "{}")
				}
			}
			fmt.Fprintf(&gw, "\t{Actions: %s, Levels: [5]AuthzCell{%s}},\n", goStrings("Action", r.Actions), strings.Join(cells, ", "))
		}
		gw.WriteString("}\n\n")
		gw.WriteString("// AuthzFixed 是不走档位表的动作：只有这类 actor 能做（C.09 §4.2）。\nvar AuthzFixed = map[Action]ActorKind{\n")
		for _, k := range sortedKeys(az.Fixed) {
			if !contains(actions, k) || !contains(actors, az.Fixed[k]) {
				errs.add("authz_matrix.yaml fixed：%s: %s 不是动作 / actor", k, az.Fixed[k])
			}
			fmt.Fprintf(&gw, "\t%q: %q,\n", k, az.Fixed[k])
		}
		gw.WriteString("}\n\n")
		gw.WriteString("// AuthzOpActions 是 Op 种类 → 判定用的动作；没列出的是 op.<kind>，per_sub_op 表示按子 Op 逐条判定（C.09 §4.3）。\nvar AuthzOpActions = map[OpKind]string{\n")
		for _, k := range sortedKeys(az.OpActions) {
			v := az.OpActions[k]
			if !contains(kinds, k) || (v != "per_sub_op" && !contains(actions, v)) {
				errs.add("authz_matrix.yaml op_actions：%s: %s 不是 Op 种类 / 动作", k, v)
			}
			fmt.Fprintf(&gw, "\t%q: %q,\n", k, v)
		}
		gw.WriteString("}\n\n")
		addTS("AUTHZ_MATRIX", "authz_matrix.yaml", "授权默认判定表（C.09 §5.1）", raw)
	}

	// 状态机
	var sm stateMachineYAML
	if raw, e := readYAMLStrict(filepath.Join(dir, "state_machine.yaml"), &sm); e != nil {
		errs.add("%v", e)
	} else {
		for _, mc := range []struct {
			name, typ string
			mach      machineYAML
		}{{"Run", "RunStatus", sm.Run}, {"Node", "NodeState", sm.Node}} {
			enum := enumOf(m, mc.typ)
			if strings.Join(enum, ",") != strings.Join(mc.mach.States, ",") {
				errs.add("state_machine.yaml %s.states 应与 %s 的取值同序（C.07-R9）", strings.ToLower(mc.name), mc.typ)
			}
			for _, s := range append(append([]string{}, mc.mach.Terminal...), sortedKeys(mc.mach.Transitions)...) {
				if !contains(enum, s) {
					errs.add("state_machine.yaml %s：%s 不是 %s 的取值", strings.ToLower(mc.name), s, mc.typ)
				}
			}
			lower := strings.ToLower(mc.name)
			fmt.Fprintf(&gw, "var %sTransitions = map[%s][]%s{\n", lower, mc.typ, mc.typ)
			for _, k := range sortedKeys(mc.mach.Transitions) {
				for _, s := range mc.mach.Transitions[k] {
					if !contains(enum, s) {
						errs.add("state_machine.yaml %s：%s 不是 %s 的取值", lower, s, mc.typ)
					}
				}
				fmt.Fprintf(&gw, "\t%q: %s,\n", k, goStrings(mc.typ, mc.mach.Transitions[k]))
			}
			gw.WriteString("}\n\n")
			fmt.Fprintf(&gw, "var %sTerminal = map[%s]bool{\n", lower, mc.typ)
			for _, s := range mc.mach.Terminal {
				fmt.Fprintf(&gw, "\t%q: true,\n", s)
			}
			gw.WriteString("}\n\n")
			fmt.Fprintf(&gw, `// %[1]sCanTransition：状态 from 能不能进入 to（C.07，state_machine.yaml）。
func %[1]sCanTransition(from, to %[2]s) bool {
	for _, s := range %[3]sTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// %[1]sIsTerminal：s 是不是终态。
func %[1]sIsTerminal(s %[2]s) bool {
	return %[3]sTerminal[s]
}

`, mc.name, mc.typ, lower)
		}
		addTS("STATE_MACHINE", "state_machine.yaml", "运行和节点的状态机（C.07 §6）", raw)
		tw.WriteString(`const runTransitions: Readonly<Record<string, readonly string[]>> = STATE_MACHINE.run.transitions;
const nodeTransitions: Readonly<Record<string, readonly string[]>> = STATE_MACHINE.node.transitions;
const runTerminal: readonly string[] = STATE_MACHINE.run.terminal;
const nodeTerminal: readonly string[] = STATE_MACHINE.node.terminal;

/** 运行状态 from 能不能进入 to（C.07）。 */
export function runCanTransition(from: RunStatus, to: RunStatus): boolean {
  return (runTransitions[from] ?? []).includes(to);
}

/** 运行状态 s 是不是终态。 */
export function runIsTerminal(s: RunStatus): boolean {
  return runTerminal.includes(s);
}

/** 节点状态 from 能不能进入 to（C.07）。 */
export function nodeCanTransition(from: NodeState, to: NodeState): boolean {
  return (nodeTransitions[from] ?? []).includes(to);
}

/** 节点状态 s 是不是终态。 */
export function nodeIsTerminal(s: NodeState): boolean {
  return nodeTerminal.includes(s);
}
`)
	}

	// 快捷键总表（只给前端，W.02.04 §4.1、C.14 §5.3）
	var scs []shortcutYAML
	if raw, e := readYAMLStrict(filepath.Join(dir, "shortcuts.yaml"), &scs); e != nil {
		errs.add("%v", e)
	} else {
		if len(scs) == 0 {
			errs.add("shortcuts.yaml：快捷键表不能为空")
		}
		seen := map[string]bool{}
		layerCombos := map[string]string{}
		for i, sc := range scs {
			if sc.ID == "" || strings.TrimSpace(sc.ID) != sc.ID {
				errs.add("shortcuts.yaml[%d]：id 不能为空或含首尾空白", i)
			} else if seen[sc.ID] {
				errs.add("shortcuts.yaml[%d]：id %q 重复", i, sc.ID)
			} else {
				seen[sc.ID] = true
			}
			if len(sc.Keys) == 0 {
				errs.add("shortcuts.yaml[%d]（%s）：keys 不能为空", i, sc.ID)
			}
			if !contains(shortcutLayers, sc.Layer) {
				errs.add("shortcuts.yaml[%d]（%s）：layer %q 不在 %v 中", i, sc.ID, sc.Layer, shortcutLayers)
			}
			for j, k := range sc.Keys {
				_, isSingle, kerr := validateShortcutCombo(k)
				if kerr != nil {
					errs.add("shortcuts.yaml[%d].keys[%d]（%s）：%v", i, j, sc.ID, kerr)
					continue
				}
				if reservedKeys[k] {
					errs.add("shortcuts.yaml[%d].keys[%d]（%s）：不能占用保留键 %q（U.13.04-R2）", i, j, sc.ID, k)
				}
				if isSingle && sc.InInput && sc.Layer != "composer" {
					errs.add("shortcuts.yaml[%d].keys[%d]（%s）：非 composer 层的单键 %q 不能设 in_input: true（U.13.04-R1）", i, j, sc.ID, k)
				}
				lk := sc.Layer + "\x00" + k
				if prev, dup := layerCombos[lk]; dup {
					errs.add("shortcuts.yaml[%d].keys[%d]（%s）：同层 %s 组合 %q 与 %s 冲突（W.02.04 §6.3）", i, j, sc.ID, sc.Layer, k, prev)
				} else {
					layerCombos[lk] = sc.ID
				}
			}
			if !strings.HasPrefix(sc.Label, "shortcut.") {
				errs.add("shortcuts.yaml[%d]（%s）：label %q 必须以 shortcut. 开头", i, sc.ID, sc.Label)
			}
			if strings.TrimSpace(sc.Group) == "" {
				errs.add("shortcuts.yaml[%d]（%s）：group 不能为空", i, sc.ID)
			}
		}
		tw.WriteString("\n")
		addTS("SHORTCUTS", "shortcuts.yaml", "快捷键总表（U.13.04、W.02.04 §4.1）", raw)
	}
	if err := errs.err(); err != nil {
		return nil, nil, err
	}
	goSrc, err = formatGo("gen/tables.go", gw.String())
	return goSrc, []byte(tw.String()), err
}
