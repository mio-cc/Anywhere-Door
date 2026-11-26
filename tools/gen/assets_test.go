package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func copyDirFiles(t *testing.T, src, dst string) {
	t.Helper()
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		sp := filepath.Join(src, e.Name())
		dp := filepath.Join(dst, e.Name())
		if e.IsDir() {
			copyDirFiles(t, sp, dp)
			continue
		}
		b, err := os.ReadFile(sp)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dp, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAssetsGeneration(t *testing.T) {
	t.Run("C14-T8 令牌与主题生成", func(t *testing.T) {
		designSrc := filepath.Join(repoRoot, "design")
		outs, err := generateTokens(designSrc)
		if err != nil {
			t.Fatalf("generateTokens 失败：%v", err)
		}
		if len(outs) != 3 {
			t.Fatalf("期望 3 个令牌生成物，实际 %d", len(outs))
		}
		themesTS := string(outs[2].Data)
		for _, want := range []string{
			"key: 'paper'", "key: 'graphite'", "key: 'whale'", "key: 'amber'",
			"export const DEFAULT_PALETTE = 'paper' as const;",
		} {
			if !strings.Contains(themesTS, want) {
				t.Errorf("themes.ts 缺少 %q", want)
			}
		}

		// 验证 rgb/rgba 与半透明前景色合成对比度计算（评审 #7）
		if ratio, err := contrastRatio("rgba(0, 0, 0, 0.9)", "#ffffff"); err != nil || ratio < 12.0 {
			t.Errorf("rgba 合成对比度计算异常：ratio=%v err=%v", ratio, err)
		}
		if _, err := contrastRatio("#000000", "rgba(255, 255, 255, 0.5)"); err == nil {
			t.Error("半透明背景色应当报错")
		}
		if got := tsQuote("a'b\\c\nd\r\u2028"); got != `'a\'b\\c\nd\r\u2028'` {
			t.Errorf("tsQuote 转义不完整：%q", got)
		}

		rawPaper, err := os.ReadFile(filepath.Join(designSrc, "themes", "paper.css"))
		if err != nil {
			t.Fatal(err)
		}
		rawTokens, err := os.ReadFile(filepath.Join(designSrc, "tokens.css"))
		if err != nil {
			t.Fatal(err)
		}

		// 反例 1：缺少必需颜色令牌
		badMissing := t.TempDir()
		copyDirFiles(t, designSrc, badMissing)
		broken := strings.Replace(string(rawPaper), "--hx-model: #46659c;", "", 1)
		if err := os.WriteFile(filepath.Join(badMissing, "themes", "paper.css"), []byte(broken), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := generateTokens(badMissing); err == nil || !strings.Contains(err.Error(), "--hx-model") {
			t.Errorf("缺少 --hx-model 应当生成失败，实际：%v", err)
		}

		// 反例 2：主题文件定义非颜色令牌（U.03.05-R6）
		badNonColor := t.TempDir()
		copyDirFiles(t, designSrc, badNonColor)
		broken2 := strings.Replace(string(rawPaper), "--hx-bg-0: #ebe8df;", "--hx-bg-0: #ebe8df;\n  --hx-fs-13: 14px;", 1)
		if err := os.WriteFile(filepath.Join(badNonColor, "themes", "paper.css"), []byte(broken2), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := generateTokens(badNonColor); err == nil || !strings.Contains(err.Error(), "--hx-fs-13") {
			t.Errorf("主题文件包含 --hx-fs-13 应当生成失败，实际：%v", err)
		}

		// 反例 2b：主题文件含普通 CSS 声明（font-size）或 @import（评审 #5）
		badPlainCSS := t.TempDir()
		copyDirFiles(t, designSrc, badPlainCSS)
		brokenPlain := strings.Replace(string(rawPaper), "--hx-bg-0: #ebe8df;", "--hx-bg-0: #ebe8df;\n  font-size: 14px;", 1)
		if err := os.WriteFile(filepath.Join(badPlainCSS, "themes", "paper.css"), []byte(brokenPlain), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := generateTokens(badPlainCSS); err == nil || !strings.Contains(err.Error(), "font-size") {
			t.Errorf("主题文件包含 font-size 应当生成失败，实际：%v", err)
		}

		badAtRule := t.TempDir()
		copyDirFiles(t, designSrc, badAtRule)
		if err := os.WriteFile(filepath.Join(badAtRule, "themes", "paper.css"), append([]byte("@import url('x.css');\n"), rawPaper...), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := generateTokens(badAtRule); err == nil || !strings.Contains(err.Error(), "@ 规则") {
			t.Errorf("主题文件包含 @import 应当生成失败，实际：%v", err)
		}

		// 反例 2c：伪造选择器（评审 #4）
		badSelector := t.TempDir()
		copyDirFiles(t, designSrc, badSelector)
		brokenSel := strings.Replace(string(rawPaper), "[data-palette='paper'][data-theme='dark']", ".fake [data-palette='paper'][data-theme='dark']", 1)
		if err := os.WriteFile(filepath.Join(badSelector, "themes", "paper.css"), []byte(brokenSel), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := generateTokens(badSelector); err == nil || !strings.Contains(err.Error(), "不允许的选择器") {
			t.Errorf("主题文件伪造选择器应当生成失败，实际：%v", err)
		}

		// 反例 2d：amber 元数据 @theme-key 错误或重复（评审 #9）
		badAmberKey := t.TempDir()
		copyDirFiles(t, designSrc, badAmberKey)
		brokenAmber := strings.Replace(string(rawTokens), "@theme-key: amber", "@theme-key: wrong", 1)
		if err := os.WriteFile(filepath.Join(badAmberKey, "tokens.css"), []byte(brokenAmber), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := generateTokens(badAmberKey); err == nil || !strings.Contains(err.Error(), "amber") {
			t.Errorf("amber @theme-key 错误应当生成失败，实际：%v", err)
		}

		badDupMeta := t.TempDir()
		copyDirFiles(t, designSrc, badDupMeta)
		brokenDupMeta := strings.Replace(string(rawTokens), "@theme-key: amber", "@theme-key: amber\n   @theme-key: amber", 1)
		if err := os.WriteFile(filepath.Join(badDupMeta, "tokens.css"), []byte(brokenDupMeta), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := generateTokens(badDupMeta); err == nil || !strings.Contains(err.Error(), "重复") {
			t.Errorf("重复 @theme-key 应当生成失败，实际：%v", err)
		}

		// 反例 2e：tokens.css 的 @media 块内嵌非法选择器、颜色令牌或前缀绕过（第 2 轮评审 #4/#5/#22）
		for _, tc := range []struct {
			name  string
			extra string
			want  string
		}{
			{"@media 内嵌主题选择器与普通声明", "\n@media (prefers-reduced-motion: reduce) { [data-palette='paper'] { font-size: 20px; } }\n", "不允许的选择器"},
			{"@media 内嵌颜色令牌", "\n@media (prefers-reduced-motion: reduce) { :root { --hx-bg-0: #000000; } }\n", "只允许动效时长令牌"},
			{"@media 前缀绕过", "\n@media (prefers-reduced-motion: reduce) and (min-width: 1px) { :root { --hx-t-1: 0ms; } }\n", "不允许的 @ 规则"},
		} {
			badMedia := t.TempDir()
			copyDirFiles(t, designSrc, badMedia)
			if err := os.WriteFile(filepath.Join(badMedia, "tokens.css"), append(append([]byte{}, rawTokens...), []byte(tc.extra)...), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := generateTokens(badMedia); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("%s：应当失败并包含 %q，实际：%v", tc.name, tc.want, err)
			}
		}

		// 反例 3：低对比度色值（U.03.05-R3）
		badContrast := t.TempDir()
		copyDirFiles(t, designSrc, badContrast)
		broken3 := strings.Replace(string(rawPaper), "--hx-text-2: #524f47;", "--hx-text-2: #d8d5cc;", 1)
		if err := os.WriteFile(filepath.Join(badContrast, "themes", "paper.css"), []byte(broken3), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := generateTokens(badContrast); err == nil || !strings.Contains(err.Error(), "对比度") {
			t.Errorf("低对比度色值应当生成失败，实际：%v", err)
		}

		// 反例 4：主色与模型色色相差过近（U.03.05-R4、K303）
		badHue := t.TempDir()
		copyDirFiles(t, designSrc, badHue)
		broken4 := strings.Replace(string(rawPaper), "--hx-model: #46659c;", "--hx-model: #b05035;", 1)
		if err := os.WriteFile(filepath.Join(badHue, "themes", "paper.css"), []byte(broken4), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := generateTokens(badHue); err == nil || !strings.Contains(err.Error(), "色相差") {
			t.Errorf("主色与模型色色相差过小应当生成失败，实际：%v", err)
		}
	})

	t.Run("C14-T9 图标生成与校验", func(t *testing.T) {
		iconsSrc := filepath.Join(repoRoot, "design", "icons")
		aliasesPath := filepath.Join(repoRoot, "schema", "builtin", "aliases.yaml")
		outs, err := generateIcons(iconsSrc, aliasesPath)
		if err != nil {
			t.Fatalf("generateIcons 失败：%v", err)
		}
		if len(outs) != 1 || outs[0].Path != outStyleIconsTS {
			t.Fatalf("图标生成物路径不对：%+v", outs)
		}
		ts := string(outs[0].Data)
		for _, want := range []string{`资源:`, `上下文装配:`, `扩展:`, `chat:`, `panelL:`, `queued:`, `ICON_FALLBACK: IconName = '扩展'`} {
			if !strings.Contains(ts, want) {
				t.Errorf("icons.ts 缺少 %s", want)
			}
		}

		baseHead := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">`
		badCases := []struct {
			name string
			svg  string
			want string
		}{
			{"非法 viewBox", `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M1 1L15 15"/></svg>`, "viewBox"},
			{"缺少线宽 1.5", `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round"><path d="M1 1L15 15"/></svg>`, "stroke-width"},
			{"缺少圆头圆角", `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M1 1L15 15"/></svg>`, "stroke-linecap"},
			{"含 color 篡改 currentColor", `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" color="#ff0000"><path d="M1 1L15 15"/></svg>`, "不允许的属性"},
			{"含 script", baseHead + `<script>alert(1)</script><path d="M1 1L15 15"/></svg>`, "禁止"},
			{"含 onclick", baseHead + `<path onclick="x()" d="M1 1L15 15"/></svg>`, "禁止属性"},
			{"含 style 属性", baseHead + `<path style="stroke:red" d="M1 1L15 15"/></svg>`, "禁止属性"},
			{"线性图标含颜色 fill", baseHead + `<path fill="#ff0000" d="M1 1L15 15"/></svg>`, "不允许的属性"},
			{"未允许元素 ellipse", baseHead + `<ellipse cx="8" cy="8" rx="4" ry="2"/></svg>`, "不支持的元素"},
			{"含 XML 处理指令", `<?xml-stylesheet type="text/css" href="x.css"?>` + baseHead + `<path d="M1 1L15 15"/></svg>`, "处理指令"},
			{"含 DTD 声明", `<!DOCTYPE svg [<!ENTITY x "1">]>` + baseHead + `<path d="M1 1L15 15"/></svg>`, "DTD"},
			{"子元素含未知属性 transform", baseHead + `<path transform="rotate(45)" d="M1 1L15 15"/></svg>`, "不允许的属性"},
			{"非有限数字 NaN", baseHead + `<circle cx="NaN" cy="8" r="2"/></svg>`, "合法有限数字"},
		}
		for _, tc := range badCases {
			t.Run(tc.name, func(t *testing.T) {
				dir := t.TempDir()
				copyDirFiles(t, iconsSrc, dir)
				if err := os.WriteFile(filepath.Join(dir, "chat.svg"), []byte(tc.svg), 0o644); err != nil {
					t.Fatal(err)
				}
				if _, err := generateIcons(dir, aliasesPath); err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Errorf("%s：应当失败并包含 %q，实际：%v", tc.name, tc.want, err)
				}
			})
		}
	})

	t.Run("C14-T10 快捷键表生成", func(t *testing.T) {
		m := repoModel(t)
		builtinDir := filepath.Join(repoRoot, "schema", "builtin")
		_, tsTables, err := emitTables(m, builtinDir)
		if err != nil {
			t.Fatalf("emitTables 失败：%v", err)
		}
		ts := string(tsTables)
		for _, want := range []string{"export const SHORTCUTS =", `"palette.open"`, `"estop.trigger"`, `"composer.send"`, `"canvas.escape"`} {
			if !strings.Contains(ts, want) {
				t.Errorf("tables.ts 缺少 %s", want)
			}
		}

		badCases := []struct {
			name string
			yaml string
			want string
		}{
			{
				"重复 id",
				"- id: a.b\n  keys: [\"Mod+K\"]\n  layer: global\n  in_input: true\n  hold: false\n  label: shortcut.a.b\n  group: 全局\n- id: a.b\n  keys: [\"Mod+L\"]\n  layer: global\n  in_input: false\n  hold: false\n  label: shortcut.a.b2\n  group: 全局\n",
				"重复",
			},
			{
				"非法 layer",
				"- id: a.b\n  keys: [\"Mod+K\"]\n  layer: modal\n  in_input: true\n  hold: false\n  label: shortcut.a.b\n  group: 全局\n",
				"layer",
			},
			{
				"空 keys",
				"- id: a.b\n  keys: []\n  layer: global\n  in_input: true\n  hold: false\n  label: shortcut.a.b\n  group: 全局\n",
				"keys 不能为空",
			},
			{
				"保留键 Mod+W",
				"- id: a.b\n  keys: [\"Mod+W\"]\n  layer: global\n  in_input: true\n  hold: false\n  label: shortcut.a.b\n  group: 全局\n",
				"保留键",
			},
			{
				"修饰键顺序错误",
				"- id: a.b\n  keys: [\"Shift+Mod+G\"]\n  layer: canvas\n  in_input: false\n  hold: false\n  label: shortcut.a.b\n  group: 画布\n",
				"修饰键顺序",
			},
			{
				"同层组合冲突",
				"- id: a.one\n  keys: [\"Mod+K\"]\n  layer: global\n  in_input: true\n  hold: false\n  label: shortcut.a.one\n  group: 全局\n- id: a.two\n  keys: [\"Mod+K\"]\n  layer: global\n  in_input: false\n  hold: false\n  label: shortcut.a.two\n  group: 全局\n",
				"冲突",
			},
			{
				"非 composer 层单键 in_input",
				"- id: a.single\n  keys: [\"F\"]\n  layer: global\n  in_input: true\n  hold: false\n  label: shortcut.a.single\n  group: 全局\n",
				"in_input",
			},
			{
				"组合带首尾空格",
				"- id: a.sp\n  keys: [\" Mod+K \"]\n  layer: global\n  in_input: true\n  hold: false\n  label: shortcut.a.sp\n  group: 全局\n",
				"首尾空白",
			},
		}
		for _, tc := range badCases {
			t.Run(tc.name, func(t *testing.T) {
				badDir := t.TempDir()
				for _, f := range []string{"container_rules.yaml", "widget_map.yaml", "category_map.yaml", "aggregate_ops.yaml", "authz_matrix.yaml", "state_machine.yaml"} {
					b, err := os.ReadFile(filepath.Join(builtinDir, f))
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(badDir, f), b, 0o644); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.WriteFile(filepath.Join(badDir, "shortcuts.yaml"), []byte(tc.yaml), 0o644); err != nil {
					t.Fatal(err)
				}
				if _, _, err := emitTables(m, badDir); err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Errorf("%s：应当失败并包含 %q，实际：%v", tc.name, tc.want, err)
				}
			})
		}
	})
}
