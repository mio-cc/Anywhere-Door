package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	outStyleIconsTS = "web/src/styles/icons.ts"
	tsIconsHeader   = "/* 由 tools/gen icons 生成，请勿手改 */\n"
)

// requiredIcons 是 U.08 §2 登记的全部 72 个内置图标名。
var requiredIcons = []string{
	// 原子节点（13）+ 扩展（1）
	"资源", "Agent", "上下文装配", "工具", "扇出", "汇合", "聚合",
	"分支", "循环", "账本", "人工", "子图端口", "子图引用", "扩展",
	// 界面（52）
	"chat", "graph", "search", "history", "moon", "sun", "play", "stop", "pause",
	"branch", "expand", "shrink", "collapse", "plus", "minus", "x", "check",
	"chevl", "chevr", "chevd", "lib", "fit", "pointer", "hand", "group", "layout",
	"undo", "redo", "trash", "copy", "enter", "lock", "port", "model", "cmd",
	"gauge", "bolt", "panelL", "panelR", "gear", "arrowUp", "tree", "peek", "bell",
	"more", "star", "pin", "user", "pkg", "link", "grip", "info",
	// 状态（6）
	"queued", "done", "failed", "waiting", "skipped", "cached",
}

// 严格白名单：只允许生成物 IconShape 能够无损表示的属性，禁止任何会被静默丢弃或篡改颜色的属性（U.08-R1、R4）。
var (
	allowedRootAttrs = map[string]bool{
		"viewBox":         true,
		"fill":            true,
		"stroke":          true,
		"stroke-width":    true,
		"stroke-linecap":  true,
		"stroke-linejoin": true,
		"data-solid":      true,
	}
	allowedShapeAttrs = map[string]map[string]bool{
		"path":     {"d": true},
		"circle":   {"cx": true, "cy": true, "r": true},
		"rect":     {"x": true, "y": true, "width": true, "height": true, "rx": true},
		"line":     {"x1": true, "y1": true, "x2": true, "y2": true},
		"polyline": {"points": true},
	}
)

type iconShape struct {
	tag    string
	d      string
	points string
	nums   map[string]float64
}

type parsedIcon struct {
	name   string
	solid  bool
	shapes []iconShape
}

func checkSafeAttr(file, elem string, attr xml.Attr) error {
	name := strings.ToLower(attr.Name.Local)
	val := strings.TrimSpace(attr.Value)
	valLower := strings.ToLower(val)
	if name == "style" || strings.HasPrefix(name, "on") || name == "href" || strings.HasSuffix(name, ":href") {
		return fmt.Errorf("%s <%s>：禁止属性 %q（U.08-R4）", file, elem, attr.Name.Local)
	}
	if strings.Contains(valLower, "javascript:") || strings.Contains(valLower, "http:") ||
		strings.Contains(valLower, "https:") || strings.Contains(valLower, "data:") ||
		strings.Contains(valLower, "url(") || strings.Contains(valLower, "//") {
		return fmt.Errorf("%s <%s>：属性 %q 禁止外链或脚本（U.08-R4）", file, elem, attr.Name.Local)
	}
	return nil
}

func parseNumAttr(file, elem string, attrs map[string]string, key string, required bool) (float64, bool, error) {
	raw, ok := attrs[key]
	if !ok || strings.TrimSpace(raw) == "" {
		if required {
			return 0, false, fmt.Errorf("%s <%s>：缺少数值属性 %q", file, elem, key)
		}
		return 0, false, nil
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false, fmt.Errorf("%s <%s>：属性 %q=%q 不是合法有限数字", file, elem, key, raw)
	}
	return v, true, nil
}

func parseSVGIcon(file, iconName string, data []byte) (*parsedIcon, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = true
	dec.Entity = map[string]string{} // 拒绝所有非内置 XML 命名实体
	var icon *parsedIcon
	depth := 0
	rootSeen := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s：XML 解析失败：%w", file, err)
		}
		switch t := tok.(type) {
		case xml.Directive:
			return nil, fmt.Errorf("%s：禁止 XML DTD/指令（U.08-R4）", file)
		case xml.ProcInst:
			return nil, fmt.Errorf("%s：禁止 XML 处理指令 <?%s?>（U.08-R4）", file, t.Target)
		case xml.CharData:
			if strings.TrimSpace(string(t)) != "" {
				return nil, fmt.Errorf("%s：禁止文本内容 %q（U.08-R4）", file, strings.TrimSpace(string(t)))
			}
		case xml.StartElement:
			tag := t.Name.Local
			if depth == 0 {
				if rootSeen {
					return nil, fmt.Errorf("%s：禁止多个根元素", file)
				}
				rootSeen = true
				if tag != "svg" {
					return nil, fmt.Errorf("%s：根元素必须是 <svg>，实际 <%s>", file, tag)
				}
				attrs := map[string]string{}
				for _, a := range t.Attr {
					if a.Name.Local == "xmlns" && a.Name.Space == "" {
						if strings.TrimSpace(a.Value) != "http://www.w3.org/2000/svg" {
							return nil, fmt.Errorf("%s <svg>：xmlns 必须为 http://www.w3.org/2000/svg，实际 %q", file, a.Value)
						}
						continue
					}
					if a.Name.Space != "" {
						return nil, fmt.Errorf("%s <svg>：禁止带命名空间属性 %s:%s（U.08-R4）", file, a.Name.Space, a.Name.Local)
					}
					if err := checkSafeAttr(file, tag, a); err != nil {
						return nil, err
					}
					if !allowedRootAttrs[a.Name.Local] {
						return nil, fmt.Errorf("%s <svg>：不允许的属性 %q（U.08-R1、R4）", file, a.Name.Local)
					}
					attrs[a.Name.Local] = strings.TrimSpace(a.Value)
				}
				if attrs["viewBox"] != "0 0 16 16" {
					return nil, fmt.Errorf("%s：viewBox 必须是 \"0 0 16 16\"，实际 %q（U.08-R1）", file, attrs["viewBox"])
				}
				solid := false
				if ds, ok := attrs["data-solid"]; ok {
					if ds != "true" {
						return nil, fmt.Errorf("%s：data-solid 只能为 \"true\"，实际 %q", file, ds)
					}
					solid = true
				}
				if solid {
					if attrs["fill"] != "currentColor" {
						return nil, fmt.Errorf("%s：solid 图标必须声明 fill=\"currentColor\"，实际 %q（U.08-R1）", file, attrs["fill"])
					}
					if st := attrs["stroke"]; st != "" && st != "none" {
						return nil, fmt.Errorf("%s：solid 图标 stroke 只能省略或为 none，实际 %q（U.08-R1）", file, st)
					}
				} else {
					if attrs["fill"] != "none" {
						return nil, fmt.Errorf("%s：线性图标必须声明 fill=\"none\"，实际 %q（U.08-R1）", file, attrs["fill"])
					}
					if attrs["stroke"] != "currentColor" {
						return nil, fmt.Errorf("%s：线性图标必须声明 stroke=\"currentColor\"，实际 %q（U.08-R1）", file, attrs["stroke"])
					}
					if attrs["stroke-width"] != "1.5" {
						return nil, fmt.Errorf("%s：线性图标必须声明 stroke-width=\"1.5\"，实际 %q（U.08-R1）", file, attrs["stroke-width"])
					}
					if attrs["stroke-linecap"] != "round" || attrs["stroke-linejoin"] != "round" {
						return nil, fmt.Errorf("%s：线性图标必须声明 stroke-linecap=\"round\" 且 stroke-linejoin=\"round\"（U.08-R1）", file)
					}
				}
				icon = &parsedIcon{name: iconName, solid: solid}
				depth++
				continue
			}
			if depth != 1 {
				return nil, fmt.Errorf("%s：不支持嵌套元素 <%s>（只允许直接子图形）", file, tag)
			}
			allowedAttrs, ok := allowedShapeAttrs[tag]
			if !ok {
				return nil, fmt.Errorf("%s：禁止或不支持的元素 <%s>（只允许 path/circle/rect/line/polyline，U.08-R4）", file, tag)
			}
			attrs := map[string]string{}
			for _, a := range t.Attr {
				if a.Name.Space != "" {
					return nil, fmt.Errorf("%s <%s>：禁止带命名空间属性 %s:%s（U.08-R4）", file, tag, a.Name.Space, a.Name.Local)
				}
				if err := checkSafeAttr(file, tag, a); err != nil {
					return nil, err
				}
				if !allowedAttrs[a.Name.Local] {
					return nil, fmt.Errorf("%s <%s>：不允许的属性 %q（源 SVG 与 IconShape 必须语义等价，U.08-R1）", file, tag, a.Name.Local)
				}
				attrs[a.Name.Local] = strings.TrimSpace(a.Value)
			}
			sh := iconShape{tag: tag, nums: map[string]float64{}}
			switch tag {
			case "path":
				d := attrs["d"]
				if d == "" {
					return nil, fmt.Errorf("%s <path>：缺少 d 属性", file)
				}
				sh.d = d
			case "circle":
				for _, k := range []string{"cx", "cy", "r"} {
					v, _, err := parseNumAttr(file, tag, attrs, k, true)
					if err != nil {
						return nil, err
					}
					if k == "r" && v <= 0 {
						return nil, fmt.Errorf("%s <circle>：半径 r 必须 > 0，实际 %v", file, v)
					}
					sh.nums[k] = v
				}
			case "rect":
				for _, pair := range [][2]string{{"x", "x"}, {"y", "y"}, {"width", "w"}, {"height", "h"}} {
					v, _, err := parseNumAttr(file, tag, attrs, pair[0], true)
					if err != nil {
						return nil, err
					}
					if (pair[1] == "w" || pair[1] == "h") && v <= 0 {
						return nil, fmt.Errorf("%s <rect>：%s 必须 > 0，实际 %v", file, pair[0], v)
					}
					sh.nums[pair[1]] = v
				}
				if v, ok, err := parseNumAttr(file, tag, attrs, "rx", false); err != nil {
					return nil, err
				} else if ok {
					if v < 0 {
						return nil, fmt.Errorf("%s <rect>：rx 不能为负，实际 %v", file, v)
					}
					sh.nums["rx"] = v
				}
			case "line":
				for _, k := range []string{"x1", "y1", "x2", "y2"} {
					v, _, err := parseNumAttr(file, tag, attrs, k, true)
					if err != nil {
						return nil, err
					}
					sh.nums[k] = v
				}
			case "polyline":
				pts := attrs["points"]
				if pts == "" {
					return nil, fmt.Errorf("%s <polyline>：缺少 points 属性", file)
				}
				sh.points = pts
			}
			icon.shapes = append(icon.shapes, sh)
			depth++
		case xml.EndElement:
			depth--
		}
	}
	if icon == nil || len(icon.shapes) == 0 {
		return nil, fmt.Errorf("%s：图标没有图形元素", file)
	}
	return icon, nil
}

func fmtFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func emitShapeTS(sh iconShape) string {
	switch sh.tag {
	case "path":
		one := fmt.Sprintf("{ t: 'path', d: %s }", tsQuote(sh.d))
		if len("      "+one+",") > 100 {
			return fmt.Sprintf("{\n        t: 'path',\n        d: %s,\n      }", tsQuote(sh.d))
		}
		return one
	case "circle":
		return fmt.Sprintf("{ t: 'circle', cx: %s, cy: %s, r: %s }",
			fmtFloat(sh.nums["cx"]), fmtFloat(sh.nums["cy"]), fmtFloat(sh.nums["r"]))
	case "rect":
		if rx, ok := sh.nums["rx"]; ok {
			return fmt.Sprintf("{ t: 'rect', x: %s, y: %s, w: %s, h: %s, rx: %s }",
				fmtFloat(sh.nums["x"]), fmtFloat(sh.nums["y"]), fmtFloat(sh.nums["w"]), fmtFloat(sh.nums["h"]), fmtFloat(rx))
		}
		return fmt.Sprintf("{ t: 'rect', x: %s, y: %s, w: %s, h: %s }",
			fmtFloat(sh.nums["x"]), fmtFloat(sh.nums["y"]), fmtFloat(sh.nums["w"]), fmtFloat(sh.nums["h"]))
	case "line":
		return fmt.Sprintf("{ t: 'line', x1: %s, y1: %s, x2: %s, y2: %s }",
			fmtFloat(sh.nums["x1"]), fmtFloat(sh.nums["y1"]), fmtFloat(sh.nums["x2"]), fmtFloat(sh.nums["y2"]))
	case "polyline":
		return fmt.Sprintf("{ t: 'polyline', points: %s }", tsQuote(sh.points))
	default:
		return ""
	}
}

func tsPropKey(s string) string {
	// JS/TS 支持 Unicode 标识符（所有 72 个图标名均不含空白或标点），与 Prettier 默认格式一致
	return s
}

func generateIcons(iconsDir, aliasesPath string) ([]Output, error) {
	al, err := readAliases(aliasesPath)
	if err != nil {
		return nil, err
	}
	// 反向映射：英文别名 → 中文原子节点名（C.14-R7），另加 Extension → 扩展（K303）
	aliasToNode := map[string]string{"Extension": "扩展"}
	for zh, en := range al.Nodes {
		aliasToNode[en] = zh
	}

	var errs genErrors
	parsed := map[string]*parsedIcon{}

	loadDir := func(dir string, isNode bool) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			errs.add("读取 %s：%v", dir, err)
			return
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if !strings.HasSuffix(e.Name(), ".svg") {
				errs.add("%s：图标目录里只允许 .svg 文件", filepath.Join(dir, e.Name()))
				continue
			}
			stem := strings.TrimSuffix(e.Name(), ".svg")
			iconName := stem
			relLabel := "design/icons/" + e.Name()
			if isNode {
				relLabel = "design/icons/node/" + e.Name()
				mapped, ok := aliasToNode[stem]
				if !ok {
					errs.add("%s：节点图标别名 %q 不在 aliases.yaml 的 nodes 或 Extension 中", relLabel, stem)
					continue
				}
				iconName = mapped
			}
			if _, dup := parsed[iconName]; dup {
				errs.add("%s：图标名 %q 重复", relLabel, iconName)
				continue
			}
			b, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				errs.add("%s：%v", relLabel, err)
				continue
			}
			ic, err := parseSVGIcon(relLabel, iconName, b)
			if err != nil {
				errs.add("%v", err)
				continue
			}
			parsed[iconName] = ic
		}
	}

	loadDir(iconsDir, false)
	loadDir(filepath.Join(iconsDir, "node"), true)

	// 检查覆盖 U.08 §2 的全部 72 个图标名
	reqSet := map[string]bool{}
	var missing []string
	for _, name := range requiredIcons {
		reqSet[name] = true
		if _, ok := parsed[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		errs.add("缺少 U.08 §2 登记的图标：%s", strings.Join(missing, ", "))
	}
	var extra []string
	for name := range parsed {
		if !reqSet[name] {
			extra = append(extra, name)
		}
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		errs.add("存在未在 U.08 §2 登记的图标（U.08-R6）：%s", strings.Join(extra, ", "))
	}

	if err := errs.err(); err != nil {
		return nil, err
	}

	var b strings.Builder
	b.WriteString(tsIconsHeader)
	b.WriteString("export type IconShape =\n")
	b.WriteString("  | { readonly t: 'path'; readonly d: string }\n")
	b.WriteString("  | { readonly t: 'circle'; readonly cx: number; readonly cy: number; readonly r: number }\n")
	b.WriteString("  | {\n")
	b.WriteString("      readonly t: 'rect';\n")
	b.WriteString("      readonly x: number;\n")
	b.WriteString("      readonly y: number;\n")
	b.WriteString("      readonly w: number;\n")
	b.WriteString("      readonly h: number;\n")
	b.WriteString("      readonly rx?: number;\n")
	b.WriteString("    }\n")
	b.WriteString("  | { readonly t: 'line'; readonly x1: number; readonly y1: number; readonly x2: number; readonly y2: number }\n")
	b.WriteString("  | { readonly t: 'polyline'; readonly points: string };\n\n")

	b.WriteString("export type IconName =\n")
	for i, name := range requiredIcons {
		if i == len(requiredIcons)-1 {
			fmt.Fprintf(&b, "  | %s;\n\n", tsQuote(name))
		} else {
			fmt.Fprintf(&b, "  | %s\n", tsQuote(name))
		}
	}

	b.WriteString("export interface IconDef {\n")
	b.WriteString("  readonly shapes: readonly IconShape[];\n")
	b.WriteString("  readonly solid?: true;\n")
	b.WriteString("}\n\n")

	b.WriteString("export const ICONS: Readonly<Record<IconName, IconDef>> = {\n")
	for _, name := range requiredIcons {
		ic := parsed[name]
		fmt.Fprintf(&b, "  %s: {\n", tsPropKey(name))
		if len(ic.shapes) == 1 {
			one := emitShapeTS(ic.shapes[0])
			inline := "    shapes: [" + one + "],"
			if !strings.Contains(one, "\n") && len(inline) <= 100 {
				b.WriteString(inline + "\n")
			} else {
				b.WriteString("    shapes: [\n")
				sh := ic.shapes[0]
				if sh.tag == "path" {
					fmt.Fprintf(&b, "      {\n        t: 'path',\n        d: %s,\n      },\n", tsQuote(sh.d))
				} else {
					fmt.Fprintf(&b, "      %s,\n", one)
				}
				b.WriteString("    ],\n")
			}
		} else {
			b.WriteString("    shapes: [\n")
			for _, sh := range ic.shapes {
				fmt.Fprintf(&b, "      %s,\n", emitShapeTS(sh))
			}
			b.WriteString("    ],\n")
		}
		if ic.solid {
			b.WriteString("    solid: true,\n")
		}
		b.WriteString("  },\n")
	}
	b.WriteString("};\n\n")
	b.WriteString("export const ICON_FALLBACK: IconName = '扩展';\n")
	return []Output{{outStyleIconsTS, []byte(b.String())}}, nil
}
