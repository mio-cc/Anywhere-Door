package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	outStyleTokensCSS = "web/src/styles/tokens.css"
	outStyleThemesCSS = "web/src/styles/themes.css"
	outStyleThemesTS  = "web/src/styles/themes.ts"
)

const (
	cssTokensHeader = "/* 由 tools/gen tokens 生成，请勿手改 */\n\n"
	tsTokensHeader  = "/* 由 tools/gen tokens 生成，请勿手改 */\n"
)

// themeOrder 是内置主题在 themes.ts 中的固定顺序（U.03.05 §1，paper 默认第一）。
var themeOrder = []string{"paper", "graphite", "whale", "amber"}

// requiredThemeTokens 是每套内置主题在浅色、深色下都必须定义的 62 个颜色与阴影令牌（U.03.05 §5 + U.03.01）。
var requiredThemeTokens = []string{
	"--hx-bg-0", "--hx-bg-1", "--hx-bg-2", "--hx-bg-3", "--hx-bg-4", "--hx-bg-5",
	"--hx-bg-glass", "--hx-bg-glide", "--hx-bg-thumb", "--hx-bg-seg",
	"--hx-line-1", "--hx-line-2", "--hx-line-3", "--hx-highlight",
	"--hx-text-1", "--hx-text-2", "--hx-text-3", "--hx-text-inv",
	"--hx-accent", "--hx-accent-strong", "--hx-accent-weak", "--hx-accent-line",
	"--hx-primary", "--hx-primary-fg",
	"--hx-ok", "--hx-ok-weak",
	"--hx-danger", "--hx-danger-weak",
	"--hx-wait", "--hx-wait-weak",
	"--hx-model", "--hx-model-weak", "--hx-model-line",
	"--hx-focus",
	"--hx-type-any", "--hx-type-ctx", "--hx-type-ref", "--hx-type-list",
	"--hx-type-tool", "--hx-type-gen", "--hx-type-ctrl", "--hx-type-state",
	"--hx-mark-c1", "--hx-mark-c1-weak",
	"--hx-mark-c2", "--hx-mark-c2-weak",
	"--hx-mark-c3", "--hx-mark-c3-weak",
	"--hx-mark-c4", "--hx-mark-c4-weak",
	"--hx-mark-c5", "--hx-mark-c5-weak",
	"--hx-mark-c6", "--hx-mark-c6-weak",
	"--hx-grid-dot", "--hx-marquee", "--hx-marquee-line", "--hx-scrim",
	"--hx-shadow-node", "--hx-shadow-node-drag", "--hx-shadow-float", "--hx-shadow-pop",
}

var (
	themeMetaRe = regexp.MustCompile(`@theme-(key|name|ref|desc)\s*:\s*(.+)`)
	rgbFuncRe   = regexp.MustCompile(`^rgba?\(\s*([0-9.]+)\s*,\s*([0-9.]+)\s*,\s*([0-9.]+)(?:\s*,\s*([0-9.]+))?\s*\)$`)
)

type themeMeta struct {
	key  string
	name string
	ref  string
	desc string
}

type parsedTheme struct {
	meta  themeMeta
	light map[string]string
	dark  map[string]string
}

type cssRule struct {
	selector string
	decls    []cssDecl
}

type cssDecl struct {
	prop string
	val  string
}

type rgbaColor struct {
	r, g, b, a float64
}

func normalizeLF(b []byte) string {
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return s
}

func parseThemeMeta(src, file, wantKey string) (themeMeta, error) {
	var m themeMeta
	seen := map[string]bool{}
	for _, sub := range themeMetaRe.FindAllStringSubmatch(src, -1) {
		k := sub[1]
		val := strings.TrimSpace(sub[2])
		if seen[k] {
			return m, fmt.Errorf("%s：@theme-%s 重复定义", file, k)
		}
		seen[k] = true
		switch k {
		case "key":
			m.key = val
		case "name":
			m.name = val
		case "ref":
			m.ref = val
		case "desc":
			m.desc = val
		}
	}
	if m.key == "" || m.name == "" || m.ref == "" || m.desc == "" {
		return m, fmt.Errorf("%s：缺少 @theme-key / @theme-name / @theme-ref / @theme-desc 元数据注释", file)
	}
	if wantKey != "" && m.key != wantKey {
		return m, fmt.Errorf("%s：@theme-key=%q 应为 %q", file, m.key, wantKey)
	}
	return m, nil
}

// stripCSSComments 逐字符移除 CSS 注释（保留字符串字面量内的 /* */），并拒绝未闭合注释。
func stripCSSComments(src, file string) (string, error) {
	var out strings.Builder
	var quote byte
	for i := 0; i < len(src); i++ {
		c := src[i]
		if quote != 0 {
			out.WriteByte(c)
			if c == '\\' && i+1 < len(src) {
				i++
				out.WriteByte(src[i])
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			out.WriteByte(c)
			continue
		}
		if c == '/' && i+1 < len(src) && src[i+1] == '*' {
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				return "", fmt.Errorf("%s：存在未闭合的 CSS 注释", file)
			}
			i += 2 + end + 1
			continue
		}
		out.WriteByte(c)
	}
	return out.String(), nil
}

// parseTopLevelCSS 将 CSS 解析为顶层规则和顶层 @ 规则（支持括号深度匹配，拒绝残余声明与非法语法）。
func parseTopLevelCSS(src, file string) (rules []cssRule, atRules []string, err error) {
	clean, err := stripCSSComments(src, file)
	if err != nil {
		return nil, nil, err
	}
	i := 0
	n := len(clean)
	for i < n {
		for i < n && (clean[i] == ' ' || clean[i] == '\t' || clean[i] == '\n' || clean[i] == '\r') {
			i++
		}
		if i >= n {
			break
		}
		start := i
		if clean[i] == '@' {
			var quote byte
			depth := 0
			for i < n {
				c := clean[i]
				if quote != 0 {
					if c == '\\' && i+1 < n {
						i += 2
						continue
					}
					if c == quote {
						quote = 0
					}
					i++
					continue
				}
				if c == '\'' || c == '"' {
					quote = c
					i++
					continue
				}
				if c == ';' && depth == 0 {
					i++
					break
				}
				if c == '{' {
					depth++
				} else if c == '}' {
					depth--
					if depth == 0 {
						i++
						break
					}
					if depth < 0 {
						return nil, nil, fmt.Errorf("%s：多余的 '}'", file)
					}
				}
				i++
			}
			if depth != 0 {
				return nil, nil, fmt.Errorf("%s：@ 规则花括号未闭合", file)
			}
			atRules = append(atRules, strings.TrimSpace(clean[start:i]))
			continue
		}

		// 读取选择器直到 '{'
		var quote byte
		for i < n {
			c := clean[i]
			if quote != 0 {
				if c == '\\' && i+1 < n {
					i += 2
					continue
				}
				if c == quote {
					quote = 0
				}
				i++
				continue
			}
			if c == '\'' || c == '"' {
				quote = c
				i++
				continue
			}
			if c == '{' {
				break
			}
			if c == '}' {
				return nil, nil, fmt.Errorf("%s：意外的 '}'", file)
			}
			i++
		}
		if i >= n || clean[i] != '{' {
			return nil, nil, fmt.Errorf("%s：存在未闭合的规则或孤立内容 %q", file, strings.TrimSpace(clean[start:]))
		}
		sel := strings.TrimSpace(clean[start:i])
		i++ // skip '{'
		bodyStart := i
		depth := 1
		quote = 0
		for i < n && depth > 0 {
			c := clean[i]
			if quote != 0 {
				if c == '\\' && i+1 < n {
					i += 2
					continue
				}
				if c == quote {
					quote = 0
				}
				i++
				continue
			}
			if c == '\'' || c == '"' {
				quote = c
				i++
				continue
			}
			switch c {
			case '{':
				depth++
			case '}':
				depth--
			}
			i++
		}
		if depth != 0 {
			return nil, nil, fmt.Errorf("%s：规则 %q 的花括号未闭合", file, sel)
		}
		body := clean[bodyStart : i-1]
		if strings.Contains(body, "{") {
			return nil, nil, fmt.Errorf("%s：规则 %q 不允许嵌套块", file, sel)
		}
		decls, err := parseCSSDecls(body, file, sel)
		if err != nil {
			return nil, nil, err
		}
		rules = append(rules, cssRule{selector: sel, decls: decls})
	}
	return rules, atRules, nil
}

func parseCSSDecls(body, file, sel string) ([]cssDecl, error) {
	var decls []cssDecl
	var cur strings.Builder
	var quote byte
	paren := 0
	flush := func() error {
		s := strings.TrimSpace(cur.String())
		cur.Reset()
		if s == "" {
			return nil
		}
		colon := strings.IndexByte(s, ':')
		if colon <= 0 {
			return fmt.Errorf("%s [%s]：非法 CSS 声明 %q", file, sel, s)
		}
		prop := strings.TrimSpace(s[:colon])
		val := strings.TrimSpace(s[colon+1:])
		if prop == "" || val == "" {
			return fmt.Errorf("%s [%s]：非法 CSS 声明 %q", file, sel, s)
		}
		decls = append(decls, cssDecl{prop: prop, val: val})
		return nil
	}
	for i := 0; i < len(body); i++ {
		c := body[i]
		if quote != 0 {
			cur.WriteByte(c)
			if c == '\\' && i+1 < len(body) {
				i++
				cur.WriteByte(body[i])
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			cur.WriteByte(c)
			continue
		}
		if c == '(' {
			paren++
			cur.WriteByte(c)
			continue
		}
		if c == ')' {
			paren--
			cur.WriteByte(c)
			continue
		}
		if c == ';' && paren == 0 {
			if err := flush(); err != nil {
				return nil, err
			}
			continue
		}
		cur.WriteByte(c)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return decls, nil
}

// canonSelector 将选择器列表标准化为排序后的去空白、双引号形式，用于精确匹配。
func canonSelectorList(sel string) []string {
	parts := strings.Split(sel, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.Join(strings.Fields(p), " ")
		s = strings.ReplaceAll(s, `'`, `"`)
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func sameStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func extractColorThemeDecls(errs *genErrors, file, label string, decls []cssDecl, wantScheme string, allowed map[string]bool) map[string]string {
	out := map[string]string{}
	seenScheme := false
	for _, d := range decls {
		if d.prop == "color-scheme" {
			if seenScheme {
				errs.add("%s（%s）：color-scheme 重复声明", file, label)
			}
			seenScheme = true
			if d.val != wantScheme {
				errs.add("%s（%s）：color-scheme 应为 %q，实际 %q", file, label, wantScheme, d.val)
			}
			continue
		}
		if !allowed[d.prop] {
			errs.add("%s（%s）：主题块只能定义颜色/阴影令牌与 color-scheme（U.03.05-R6），禁止 %q", file, label, d.prop)
			continue
		}
		if _, dup := out[d.prop]; dup {
			errs.add("%s（%s）：令牌 %q 重复定义", file, label, d.prop)
			continue
		}
		out[d.prop] = d.val
	}
	if !seenScheme {
		errs.add("%s（%s）：缺少 color-scheme: %s", file, label, wantScheme)
	}
	return out
}

func parseStrictThemeFile(errs *genErrors, src, file, key string, allowed map[string]bool) (light, dark map[string]string) {
	rules, atRules, err := parseTopLevelCSS(src, file)
	if err != nil {
		errs.add("%v", err)
		return map[string]string{}, map[string]string{}
	}
	if len(atRules) > 0 {
		errs.add("%s：主题文件禁止 @ 规则（U.03.05-R6），发现：%s", file, atRules[0])
	}
	wantLight := canonSelectorList(fmt.Sprintf(`[data-palette="%s"], [data-palette="%s"][data-theme="light"]`, key, key))
	wantDark := canonSelectorList(fmt.Sprintf(`[data-palette="%s"][data-theme="dark"]`, key))
	for _, r := range rules {
		got := canonSelectorList(r.selector)
		switch {
		case sameStringSlice(got, wantLight):
			if light != nil {
				errs.add("%s：浅色主题块重复出现", file)
				continue
			}
			light = extractColorThemeDecls(errs, file, key+"/light", r.decls, "light", allowed)
		case sameStringSlice(got, wantDark):
			if dark != nil {
				errs.add("%s：深色主题块重复出现", file)
				continue
			}
			dark = extractColorThemeDecls(errs, file, key+"/dark", r.decls, "dark", allowed)
		default:
			errs.add("%s：不允许的选择器 %q（只允许精确的浅色与深色两段，U.03.05-R7）", file, r.selector)
		}
	}
	if light == nil {
		errs.add("%s：缺少浅色主题块 [data-palette=%q], [data-palette=%q][data-theme=\"light\"]", file, key, key)
		light = map[string]string{}
	}
	if dark == nil {
		errs.add("%s：缺少深色主题块 [data-palette=%q][data-theme=\"dark\"]", file, key)
		dark = map[string]string{}
	}
	return light, dark
}

func parseStrictBaseTokensFile(errs *genErrors, src, file string, allowed map[string]bool) (light, dark map[string]string) {
	rules, atRules, err := parseTopLevelCSS(src, file)
	if err != nil {
		errs.add("%v", err)
		return map[string]string{}, map[string]string{}
	}
	wantRoot := canonSelectorList(`:root`)
	wantMotion := canonSelectorList(`html[data-motion="reduce"]`)
	for _, at := range atRules {
		brace := strings.IndexByte(at, '{')
		if brace <= 0 || !strings.HasSuffix(at, "}") {
			errs.add("%s：不允许的 @ 规则 %q", file, at)
			continue
		}
		header := strings.Join(strings.Fields(at[:brace]), " ")
		if header != "@media (prefers-reduced-motion: reduce)" {
			errs.add("%s：不允许的 @ 规则 %q（只允许精确的 @media (prefers-reduced-motion: reduce)）", file, header)
			continue
		}
		innerBody := at[brace+1 : len(at)-1]
		innerRules, innerAt, err := parseTopLevelCSS(innerBody, file+" @media")
		if err != nil {
			errs.add("%v", err)
			continue
		}
		if len(innerAt) > 0 {
			errs.add("%s [@media]：不允许嵌套 @ 规则 %q", file, innerAt[0])
		}
		if len(innerRules) == 0 {
			errs.add("%s [@media]：空的 @media 块", file)
		}
		for _, ir := range innerRules {
			got := canonSelectorList(ir.selector)
			if !sameStringSlice(got, wantRoot) && !sameStringSlice(got, wantMotion) {
				errs.add("%s [@media]：不允许的选择器 %q（只允许 :root 或 html[data-motion=\"reduce\"]）", file, ir.selector)
				continue
			}
			for _, d := range ir.decls {
				if !strings.HasPrefix(d.prop, "--hx-t-") {
					errs.add("%s [@media %s]：只允许动效时长令牌 --hx-t-*，发现 %q", file, ir.selector, d.prop)
				}
			}
		}
	}
	wantLight := canonSelectorList(`:root, [data-theme="light"], [data-palette="amber"], [data-palette="amber"][data-theme="light"]`)
	wantDark := canonSelectorList(`[data-theme="dark"], [data-palette="amber"][data-theme="dark"]`)
	seenRoot := false
	for _, r := range rules {
		got := canonSelectorList(r.selector)
		switch {
		case sameStringSlice(got, wantLight):
			if light != nil {
				errs.add("%s：amber 浅色主题块重复出现", file)
				continue
			}
			light = extractColorThemeDecls(errs, file, "amber/light", r.decls, "light", allowed)
		case sameStringSlice(got, wantDark):
			if dark != nil {
				errs.add("%s：amber 深色主题块重复出现", file)
				continue
			}
			dark = extractColorThemeDecls(errs, file, "amber/dark", r.decls, "dark", allowed)
		case sameStringSlice(got, wantRoot):
			seenRoot = true
			for _, d := range r.decls {
				if !strings.HasPrefix(d.prop, "--hx-") {
					errs.add("%s [:root]：只允许 --hx-* 自定义属性，发现 %q", file, d.prop)
				} else if allowed[d.prop] {
					errs.add("%s [:root]：颜色/阴影令牌 %q 只能放在浅色/深色主题块中", file, d.prop)
				}
			}
		case sameStringSlice(got, wantMotion):
			for _, d := range r.decls {
				if !strings.HasPrefix(d.prop, "--hx-t-") {
					errs.add("%s [html[data-motion=\"reduce\"]]：只允许动效时长令牌 --hx-t-*，发现 %q", file, d.prop)
				}
			}
		default:
			errs.add("%s：不允许的选择器 %q", file, r.selector)
		}
	}
	if !seenRoot {
		errs.add("%s：缺少与主题无关的 :root 令牌块", file)
	}
	if light == nil {
		errs.add("%s：缺少 amber 浅色主题块", file)
		light = map[string]string{}
	}
	if dark == nil {
		errs.add("%s：缺少 amber 深色主题块", file)
		dark = map[string]string{}
	}
	return light, dark
}

func parseCSSColor(s string) (rgbaColor, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "#") {
		h := s[1:]
		if len(h) == 3 {
			h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
		}
		if len(h) != 6 && len(h) != 8 {
			return rgbaColor{}, fmt.Errorf("无法解析十六进制色值 %q", s)
		}
		rv, err1 := strconv.ParseUint(h[0:2], 16, 8)
		gv, err2 := strconv.ParseUint(h[2:4], 16, 8)
		bv, err3 := strconv.ParseUint(h[4:6], 16, 8)
		if err1 != nil || err2 != nil || err3 != nil {
			return rgbaColor{}, fmt.Errorf("无法解析十六进制色值 %q", s)
		}
		a := 1.0
		if len(h) == 8 {
			av, err4 := strconv.ParseUint(h[6:8], 16, 8)
			if err4 != nil {
				return rgbaColor{}, fmt.Errorf("无法解析十六进制色值 %q", s)
			}
			a = float64(av) / 255.0
		}
		return rgbaColor{r: float64(rv), g: float64(gv), b: float64(bv), a: a}, nil
	}
	if m := rgbFuncRe.FindStringSubmatch(s); m != nil {
		rv, err1 := strconv.ParseFloat(m[1], 64)
		gv, err2 := strconv.ParseFloat(m[2], 64)
		bv, err3 := strconv.ParseFloat(m[3], 64)
		a := 1.0
		var err4 error
		if m[4] != "" {
			a, err4 = strconv.ParseFloat(m[4], 64)
		}
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil ||
			rv < 0 || rv > 255 || gv < 0 || gv > 255 || bv < 0 || bv > 255 || a < 0 || a > 1 {
			return rgbaColor{}, fmt.Errorf("非法 rgb/rgba 色值 %q", s)
		}
		return rgbaColor{r: rv, g: gv, b: bv, a: a}, nil
	}
	return rgbaColor{}, fmt.Errorf("不支持的颜色格式 %q（需要 #hex 或 rgb()/rgba()）", s)
}

func compositeOver(fg, bg rgbaColor) rgbaColor {
	if fg.a >= 1 {
		return rgbaColor{r: fg.r, g: fg.g, b: fg.b, a: 1}
	}
	return rgbaColor{
		r: fg.r*fg.a + bg.r*(1-fg.a),
		g: fg.g*fg.a + bg.g*(1-fg.a),
		b: fg.b*fg.a + bg.b*(1-fg.a),
		a: 1,
	}
}

func relLuminance(c rgbaColor) float64 {
	ch := func(v float64) float64 {
		s := v / 255.0
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(c.r) + 0.7152*ch(c.g) + 0.0722*ch(c.b)
}

func contrastRatio(fgVal, bgVal string) (float64, error) {
	fg, err := parseCSSColor(fgVal)
	if err != nil {
		return 0, err
	}
	bg, err := parseCSSColor(bgVal)
	if err != nil {
		return 0, err
	}
	if bg.a < 1 {
		return 0, fmt.Errorf("背景色 %q 不能是半透明色", bgVal)
	}
	effFg := compositeOver(fg, bg)
	l1, l2 := relLuminance(effFg), relLuminance(bg)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05), nil
}

func hueOf(val string) (float64, error) {
	c, err := parseCSSColor(val)
	if err != nil {
		return 0, err
	}
	rf, gf, bf := c.r/255.0, c.g/255.0, c.b/255.0
	mx := math.Max(rf, math.Max(gf, bf))
	mn := math.Min(rf, math.Min(gf, bf))
	d := mx - mn
	if d == 0 {
		return 0, nil
	}
	var h float64
	switch mx {
	case rf:
		h = math.Mod((gf-bf)/d, 6)
	case gf:
		h = (bf-rf)/d + 2
	default:
		h = (rf-gf)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return h, nil
}

func hueDiff(c1, c2 string) (float64, error) {
	h1, err := hueOf(c1)
	if err != nil {
		return 0, err
	}
	h2, err := hueOf(c2)
	if err != nil {
		return 0, err
	}
	d := math.Abs(h1 - h2)
	if d > 180 {
		d = 360 - d
	}
	return d, nil
}

func validateThemeVariant(errs *genErrors, name string, props map[string]string) {
	var missing []string
	for _, k := range requiredThemeTokens {
		if strings.TrimSpace(props[k]) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		errs.add("%s：缺少必需颜色/阴影令牌（U.03.05-R2）：%s", name, strings.Join(missing, ", "))
		return
	}
	fgs := []string{
		"--hx-text-1", "--hx-text-2", "--hx-text-3",
		"--hx-accent-strong", "--hx-ok", "--hx-danger", "--hx-wait", "--hx-model",
	}
	bgs := []string{"--hx-bg-1", "--hx-bg-2", "--hx-bg-4"}
	for _, fg := range fgs {
		for _, bg := range bgs {
			ratio, err := contrastRatio(props[fg], props[bg])
			if err != nil {
				errs.add("%s：%s / %s：%v", name, fg, bg, err)
				continue
			}
			minRatio := 4.5
			if bg == "--hx-bg-4" && fg == "--hx-text-3" {
				minRatio = 4.0
			}
			if ratio < minRatio {
				errs.add("%s：%s(%s) 在 %s(%s) 上对比度 %.2f < %.1f（U.03.05-R3）", name, fg, props[fg], bg, props[bg], ratio, minRatio)
			}
		}
	}
	if ratio, err := contrastRatio(props["--hx-primary-fg"], props["--hx-primary"]); err != nil {
		errs.add("%s：--hx-primary-fg / --hx-primary：%v", name, err)
	} else if ratio < 4.5 {
		errs.add("%s：--hx-primary-fg(%s) 在 --hx-primary(%s) 上对比度 %.2f < 4.5（U.03.05-R3）", name, props["--hx-primary-fg"], props["--hx-primary"], ratio)
	}
	graphicTokens := []string{
		"--hx-type-any", "--hx-type-ctx", "--hx-type-ref", "--hx-type-list",
		"--hx-type-tool", "--hx-type-gen", "--hx-type-ctrl", "--hx-type-state",
		"--hx-mark-c1", "--hx-mark-c2", "--hx-mark-c3", "--hx-mark-c4", "--hx-mark-c5", "--hx-mark-c6",
	}
	for _, tok := range graphicTokens {
		ratio, err := contrastRatio(props[tok], props["--hx-bg-3"])
		if err != nil {
			errs.add("%s：%s / --hx-bg-3：%v", name, tok, err)
			continue
		}
		if ratio < 3.0 {
			errs.add("%s：%s(%s) 在 --hx-bg-3(%s) 上对比度 %.2f < 3.0（U.03.05-R3）", name, tok, props[tok], props["--hx-bg-3"], ratio)
		}
	}
	// U.03.05-R4 + K303：模型色与主色色相差 ≥ 35°（兼容已定稿的蓝-紫主题）
	if hd, err := hueDiff(props["--hx-accent"], props["--hx-model"]); err != nil {
		errs.add("%s：--hx-accent / --hx-model：%v", name, err)
	} else if hd < 35.0 {
		errs.add("%s：--hx-accent(%s) 与 --hx-model(%s) 色相差 %.1f° < 35°（U.03.05-R4、K303）", name, props["--hx-accent"], props["--hx-model"], hd)
	}
}

// tsQuote 产出单引号安全 TS 字符串字面量，完整转义控制字符、换行与行分隔符。
func tsQuote(s string) string {
	var b strings.Builder
	b.WriteByte('\'')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\u2028':
			b.WriteString(`\u2028`)
		case '\u2029':
			b.WriteString(`\u2029`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\x%02x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('\'')
	return b.String()
}

func emitThemesTS(themes map[string]*parsedTheme) []byte {
	var b strings.Builder
	b.WriteString(tsTokensHeader)
	b.WriteString("export interface ThemeMeta {\n")
	b.WriteString("  readonly key: 'paper' | 'graphite' | 'whale' | 'amber';\n")
	b.WriteString("  readonly name: string;\n")
	b.WriteString("  readonly ref: string;\n")
	b.WriteString("  readonly desc: string;\n")
	b.WriteString("  readonly preview: {\n")
	b.WriteString("    readonly light: readonly [string, string, string, string];\n")
	b.WriteString("    readonly dark: readonly [string, string, string, string];\n")
	b.WriteString("  };\n")
	b.WriteString("}\n\n")
	b.WriteString("export const THEMES = [\n")
	for _, key := range themeOrder {
		th := themes[key]
		preview := func(m map[string]string) string {
			return fmt.Sprintf("[%s, %s, %s, %s]",
				tsQuote(m["--hx-bg-1"]), tsQuote(m["--hx-bg-2"]), tsQuote(m["--hx-primary"]), tsQuote(m["--hx-model"]))
		}
		fmt.Fprintf(&b, "  {\n")
		fmt.Fprintf(&b, "    key: %s,\n", tsQuote(th.meta.key))
		fmt.Fprintf(&b, "    name: %s,\n", tsQuote(th.meta.name))
		fmt.Fprintf(&b, "    ref: %s,\n", tsQuote(th.meta.ref))
		fmt.Fprintf(&b, "    desc: %s,\n", tsQuote(th.meta.desc))
		fmt.Fprintf(&b, "    preview: {\n")
		fmt.Fprintf(&b, "      light: %s,\n", preview(th.light))
		fmt.Fprintf(&b, "      dark: %s,\n", preview(th.dark))
		fmt.Fprintf(&b, "    },\n")
		fmt.Fprintf(&b, "  },\n")
	}
	b.WriteString("] as const satisfies readonly ThemeMeta[];\n\n")
	b.WriteString("export const DEFAULT_PALETTE = 'paper' as const;\n")
	return []byte(b.String())
}

// generateTokens 读取 design/tokens.css 与 design/themes/*.css，校验后产出 3 个生成物。
func generateTokens(designDir string) ([]Output, error) {
	var errs genErrors
	allowed := map[string]bool{}
	for _, k := range requiredThemeTokens {
		allowed[k] = true
	}

	tokensPath := filepath.Join(designDir, "tokens.css")
	rawTokens, err := os.ReadFile(tokensPath)
	if err != nil {
		return nil, fmt.Errorf("读取 %s：%w", tokensPath, err)
	}
	tokensText := normalizeLF(rawTokens)
	amberMeta, err := parseThemeMeta(tokensText, "design/tokens.css", "amber")
	if err != nil {
		errs.add("%v", err)
	}
	amberLight, amberDark := parseStrictBaseTokensFile(&errs, tokensText, "design/tokens.css", allowed)
	themes := map[string]*parsedTheme{
		"amber": {meta: amberMeta, light: amberLight, dark: amberDark},
	}

	var themesCSS strings.Builder
	themesCSS.WriteString(cssTokensHeader)
	for i, key := range []string{"paper", "graphite", "whale"} {
		p := filepath.Join(designDir, "themes", key+".css")
		raw, err := os.ReadFile(p)
		if err != nil {
			errs.add("读取 design/themes/%s.css：%v", key, err)
			continue
		}
		text := normalizeLF(raw)
		meta, err := parseThemeMeta(text, "design/themes/"+key+".css", key)
		if err != nil {
			errs.add("%v", err)
		}
		light, dark := parseStrictThemeFile(&errs, text, "design/themes/"+key+".css", key, allowed)
		themes[key] = &parsedTheme{meta: meta, light: light, dark: dark}
		if i > 0 {
			themesCSS.WriteString("\n")
		}
		themesCSS.WriteString(text)
	}

	for _, key := range themeOrder {
		th, ok := themes[key]
		if !ok {
			errs.add("缺少主题 %q", key)
			continue
		}
		validateThemeVariant(&errs, key+"/light", th.light)
		validateThemeVariant(&errs, key+"/dark", th.dark)
	}
	if err := errs.err(); err != nil {
		return nil, err
	}
	return []Output{
		{outStyleTokensCSS, []byte(cssTokensHeader + tokensText)},
		{outStyleThemesCSS, []byte(themesCSS.String())},
		{outStyleThemesTS, emitThemesTS(themes)},
	}, nil
}
