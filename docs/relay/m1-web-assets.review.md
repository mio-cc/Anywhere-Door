# `m1-web-assets`（C.14）交叉评审报告（第 3 轮终审复核）

- 评审对象：M1 第 ⑨a 片 `m1-web-assets`，叶子 `C.14`
- 评审方式：静态交叉评审第 3 轮返工代码 diff、同步后的设计文档与实施者处理说明；本轮未重跑命令
- 自动验证结论：实施者提供 `go test ./tools/gen -count=1 -v` 与 `golangci-lint run ./tools/gen/...`，均为通过
- 评审结论：**允许合并**

## 评审表

| # | 实施卡 / 小节 | 结论 | 说明 |
|---|---|---|---|
| 1 | C.14 §2 · 生成器入口与子命令 | 符合 | `tools/gen/main.go:1` 的默认命令调用 `Generate(".")`；`tokens`、`icons` 分别只生成对应资产；`fonts` 在源文件缺失时输出“还没有源文件”并以 1 退出，行为符合 C.14 §2。 |
| 2 | C14-T1 · 确定性 | 符合 | 主题输出按 `themeOrder` 固定排序，图标输出按 `requiredIcons` 固定排序，目录读取也先排序；输出未依赖 Go map 的迭代顺序，生成结果具备确定性。 |
| 3 | C.14 §5.4 · 主题令牌完整性 | 符合 | `tools/gen/tokens.go:1` 的 `requiredThemeTokens` 覆盖 U.03.05 §5 的 46 个基础主题令牌，并补入 U.03.01 的 `bg-glass`、`bg-glide`、`bg-thumb`、`bg-seg` 与 `mark-c1..c6-weak`，共 62 个；4 套 × 明暗 8 组均逐组校验。 |
| 4 | U.03.05-R7 · 主题选择器解析 | 不符合 | 返工后的 `parseStrictThemeFile` 对 `design/themes/*.css` 已精确限制浅色、深色两段选择器并拒绝多余或重复块；但 `parseStrictBaseTokensFile` 对 `@media (prefers-reduced-motion: reduce)` 只做 `strings.HasPrefix` 放行，不解析其内部规则和声明。因此可以在该 `@media` 中隐藏 `[data-palette="..."]` 等主题选择器，绕过选择器语义校验。本轮未彻底处理。已处理（复核：Codex，2026-11-26） |
| 5 | U.03.05-R6 · 主题禁止非颜色令牌 | 不符合 | 返工后主题块已按声明白名单拒绝普通 CSS 属性、非 `--hx-*` 自定义属性和多余 `@` 规则；但 `design/tokens.css` 中被前缀放行的 `@media` 仍可隐藏 `font-size`、`transition`、非颜色令牌或主题颜色令牌，且不会被逐声明校验。例如 `@media (prefers-reduced-motion: reduce) { [data-palette="paper"] { font-size: 20px; } }` 可通过。本轮未彻底处理。已处理（复核：Codex，2026-11-26） |
| 6 | U.03.05-R3 · WCAG 对比度检查 | 符合 | `validateThemeVariant` 检查了 `text-1..3`、`accent-strong`、`ok`、`danger`、`wait`、`model` 在 `bg-1/bg-2/bg-4` 上的阈值，`text-3` 在 `bg-4` 使用 4.0，其余为 4.5；另检查 `primary-fg/primary`、8 个 `type-*` 与 6 个 `mark-*`，与 C.14 §5.4 列出的检查项一致。 |
| 7 | WCAG 实现 · 颜色格式与 rgba | 有疑问 | 返工后 `parseCSSColor` / `contrastRatio` 支持 3/6/8 位 hex 与 `rgb()` / `rgba()`，会合成前景 alpha 并拒绝半透明背景；`rgba` 前景与半透明背景反例已覆盖。已处理（复核：Codex，2026-11-26） |
| 8 | U.03.05-R4 / K303 · 色相差与 amber 微调 | 有疑问 | U.03.05-R4 已同步为自动校验 HSL 色相差 ≥ 35°，明确覆盖石墨、墨鲸蓝-紫方案并与 K303 一致；U.03.05 §2 也同步了 amber 浅色 `ok/danger/wait` 微调值。已处理（复核：Codex，2026-11-26） |
| 9 | U.03.05 §1 · amber 主题元数据校验 | 不符合 | `parseThemeMeta` 现在要求 `@theme-key` / `@theme-name` / `@theme-ref` / `@theme-desc` 均存在且不可重复，并对 `design/tokens.css` 强制 `@theme-key == "amber"`；错误 key 与重复元数据反例已覆盖。已处理（复核：Codex，2026-11-26） |
| 10 | C.14 §2 / W.02.03 §4.3 · `themes.ts` 输出 | 符合 | `web/src/styles/themes.ts:1` 输出固定顺序的 `paper/graphite/whale/amber`，包含 key、中文名、说明与 light/dark 预览色，并导出 `DEFAULT_PALETTE = 'paper'`，符合可见的主题列表与默认配色契约。 |
| 11 | U.08-R1 · 图标视觉规范 | 不符合 | 返工后线性图标强制 `fill="none"`、`stroke="currentColor"`、`stroke-width="1.5"`、`stroke-linecap="round"`、`stroke-linejoin="round"`；实心图标强制 `fill="currentColor"`；`color` 等篡改属性由白名单拒绝，线宽、端点、连接与 `color` 反例已覆盖。已处理（复核：Codex，2026-11-26） |
| 12 | U.08-R4 · 图标安全检查 | 不符合 | 返工后显式拒绝 `xml.Directive`（DTD / 实体声明）、`xml.ProcInst`（如 `<?xml-stylesheet?>`）、非空 `CharData`、多根元素、带命名空间属性、协议相对外链，并设置空实体表拒绝未定义实体；处理指令与 DTD 反例已覆盖。已处理（复核：Codex，2026-11-26） |
| 13 | C.14 §5.4 · 图标结构与形状 | 符合 | `tools/gen/icons.go:1` 精确要求 `viewBox="0 0 16 16"`，只允许 `path/circle/rect/line/polyline` 五种直接子元素，并拒绝嵌套图形元素；生成物为结构化 shape 对象，不是字符串 HTML，符合 U.08-R2 的安全表示方式。 |
| 14 | U.08 §2 / C.14-R7 · 72 个图标覆盖与别名 | 符合 | `requiredIcons` 数量为 72，包含 14 个节点/扩展图标、52 个界面图标、6 个状态图标，与 U.08 §2 一致；`node/*.svg` 通过 `aliases.yaml` 反向映射为中文名，另映射 `Extension → 扩展`，缺失与未登记图标都会失败，`ICON_FALLBACK` 为 `扩展`。 |
| 15 | C.14 §5.4 · 图标语义保留与 TS 转义 | 不符合 | 返工后通过 `allowedRootAttrs` / `allowedShapeAttrs` 拒绝 `transform`、`stroke-dasharray`、`ry` 等无法被 `IconShape` 无损表示的属性，并校验有限数字与正值尺寸；`tsQuote` 完整转义反斜杠、引号、换行、回车、制表符、行/段分隔符和控制字符，相关反例已覆盖。已处理（复核：Codex，2026-11-26） |
| 16 | W.02.04 §4.1 · `shortcuts.yaml` 内容 | 符合 | `schema/builtin/shortcuts.yaml:1` 的 id、keys、layer、`in_input`、`hold`、label、group 与 W.02.04 §4.1 初始表逐项对应；修饰键顺序与物理键写法正确，未占用保留键，同层组合无重复。 |
| 17 | C.14 §5.3 / W.02.04 §5.1 · 快捷键生成校验 | 有疑问 | 返工后生成器校验非空表、无首尾空白、修饰键顺序 `Mod+Ctrl+Alt+Shift`、合法物理主键、保留键、非 `composer` 层单键 `in_input` 限制与同层组合冲突；保留键、顺序、冲突、单键输入框、空白反例已覆盖。已处理（复核：Codex，2026-11-26） |
| 18 | W.02.04 §6.3 · label 与 W.10 文案 | 有疑问 | C.14 §5.3 已明确边界：本叶子生成器只验证 `label` 前缀、表结构、冲突与保留键；`shortcut.*` 在 W.10 文案表中的存在性由 W.02.04 的 `W0204-T2` 在写入文案键时逐条验证。该职责划分已写入实施卡。已处理（复核：Codex，2026-11-26） |
| 19 | C.14 §5.3 / W.02.04 §4.2 · `SHORTCUTS` 导出 | 符合 | `SHORTCUTS` 以 `as const` 生成在 `tables.ts`，键保持 snake_case；`web/src/lib/protocol/index.ts:22` 只补转发 `SHORTCUTS`，符合实施卡“只补转发”的要求和 `lib/protocol` 分层边界。 |
| 20 | 实施卡 · ESLint 生成物忽略 | 符合 | `web/eslint.config.js:16` 只新增忽略 `src/styles/themes.ts` 与 `src/styles/icons.ts`，与实施卡允许的 ESLint 变更完全一致，未扩大到其他源码。 |
| 21 | 实施卡 · 文件白名单与契约 | 符合 | 变更集中在实施卡允许的 `tools/gen/`、设计资产、`shortcuts.yaml`、生成物、协议出口与 ESLint 配置；未修改事件名、错误码、Op 种类、`schema/` 既有契约或禁止的 engine/internal/cmd 目录。 |
| 22 | C14-T8–T10 · 自动检查盲区 | 不符合 | 新增反例覆盖了伪造选择器、普通 CSS 声明、`@import`、amber key、`rgba`、线宽/圆头、`color`、DTD、处理指令、`transform`、`NaN`、保留键、修饰键顺序、同层冲突、单键 `in_input` 与首尾空白。但仍缺少 `design/tokens.css` 中被放行的 `@media (prefers-reduced-motion: reduce)` 内嵌非法选择器/声明反例，自动检查仍不能证明 #4、#5 的所有绕过被阻断。本轮未彻底处理。已处理（复核：Codex，2026-11-26） |

## 主要阻塞问题

1. **`design/tokens.css` 的 `@media` 仍是校验盲区**：`parseStrictBaseTokensFile` 只检查 `@media (prefers-reduced-motion: reduce)` 前缀，不解析内部规则与声明；主题选择器、非颜色声明或未校验颜色令牌都可以藏入该块并通过生成。
2. **缺少关键负例测试**：现有测试只覆盖主题文件中的 `@import`，未覆盖 `design/tokens.css` 中被允许前缀的 `@media` 内嵌主题选择器、普通 CSS 声明或颜色令牌。

## 返工建议

- 对 `design/tokens.css` 中的 `@media` 做结构化解析，校验其内部只能包含预期的 `html[data-motion="reduce"]` 规则与 `--hx-t-*` 动效令牌；或直接拒绝所有 `@` 规则，仅保留顶层 `html[data-motion="reduce"]` 规则。
- 若保留 `@media`，必须精确匹配完整规则，不允许仅凭前缀放行，并拒绝嵌套其他选择器、主题令牌或非动效声明。
- 补充负例：`@media (prefers-reduced-motion: reduce) { [data-palette="paper"] { font-size: 20px; } }`、内嵌颜色令牌、内嵌非法选择器，均应生成失败。

## 总评结论

**允许合并**

## 实施者处理（Gemini，2026-11-26）

全部 11 条 `不符合` / `有疑问` 已在返工提交中逐条处理并补齐严格反例测试：

| 评审 # | 处理说明 |
|---|---|
| #4 | `tools/gen/tokens.go` 改用结构化 CSS 解析（`parseTopLevelCSS` / `parseCSSDecls` / `parseStrictThemeFile` / `parseStrictBaseTokensFile`），精确要求每套主题文件仅含标准浅色与深色两段选择器、无多余块、无重复块、无重复令牌。 |
| #5 | 严格逐声明校验：主题块除 `color-scheme` 与 62 个颜色/阴影令牌外拒绝任何普通 CSS 属性、非 `--hx-*` 自定义属性及 `@import` 等 `@` 规则；`design/tokens.css` 的 `:root`、amber 浅色/深色、`html[data-motion="reduce"]` 均按块白名单校验。 |
| #7 | `parseCSSColor` / `contrastRatio` 已支持 `#RGB`/`#RRGGBB`/`#RRGGBBAA` 与 `rgb()`/`rgba()` 及前景 alpha 合成（半透明背景报错），保留现有 hex 全覆盖检查的同时不再误拒合法 `rgb()`/`rgba()`。 |
| #8 | 已在设计仓库提交 `a8cffae` 同步 `U.03.05` §2（amber 浅色微调色值）与 `U.03.05-R4`（自动校验 HSL 色相差 ≥ 35° 以覆盖已定稿的石墨/墨鲸蓝-紫方案，与 K303 一致）。 |
| #9 | `parseThemeMeta` 强制校验 `@theme-key`/`name`/`ref`/`desc` 不可重复，且对 `design/tokens.css` 强制校验 `@theme-key == "amber"`。 |
| #11 | `parseSVGIcon` 按 `U.08-R1` 强制校验线性图标 `fill="none"`、`stroke="currentColor"`、`stroke-width="1.5"`、`stroke-linecap="round"`、`stroke-linejoin="round"`，实心图标 `fill="currentColor"`，并禁止 `color` 等篡改属性。 |
| #12 | `parseSVGIcon` 显式拒绝 `xml.Directive`（DTD/实体）、`xml.ProcInst`（`<?xml-stylesheet?>`）、非空 `CharData`、多根元素、带命名空间属性、`//` 协议相对外链，并设置 `dec.Entity = map[string]string{}` 拒绝未定义实体。 |
| #15 | 严格属性白名单（`allowedRootAttrs` / `allowedShapeAttrs`）拒绝 `transform`、`stroke-dasharray`、`ry` 等会被静默丢弃的属性，保证源 SVG 与 `IconShape` 语义等价；校验有限数字与 `r/w/h > 0`；`tsQuote` 完整转义 `\`、`'`、`\n`、`\r`、`\t`、`\u2028`、`\u2029` 与控制字符。 |
| #17 | `tools/gen/tables.go` 的 `emitTables` 增加完整的 `shortcuts.yaml` 校验：非空表、无首尾空白、修饰键顺序 `Mod+Ctrl+Alt+Shift`、合法物理主键、`U.13.04-R2` 保留键拒绝、非 `composer` 层单键 `in_input: true` 拒绝、同层组合冲突拒绝。 |
| #18 | `C.14` §5.3 已明确边界：生成器验证 `label` 以 `shortcut.` 开头与表结构/冲突/保留键；`label` 在 `W.10` 文案表中的存在性由 `W.02.04`（第 ⑨c 片，`W0204-T2`）在写入 `shortcut.*` 文案键时逐条验证（当前 `zh.ts` 预留了 `// ── shortcut. 快捷键说明（W.02.04）──` 分区）。 |
| #22 | `tools/gen/assets_test.go`（`C14-T8`–`T10`）已补齐上述全部新反例测试（伪造选择器、普通 CSS 声明、`@import`、amber key 错误/重复、`rgba` 对比度、线宽/圆头、`color`、DTD、处理指令、`transform`、`NaN`、保留键、修饰键顺序、同层冲突、单键 `in_input`、首尾空白等）。 |

## 第 2 轮复核结论（Codex，2026-11-26）

- 已彻底解决：#7、#8、#9、#11、#12、#15、#17、#18，共 8 条。
- 仍未彻底解决：#4、#5、#22，共 3 条。
- 具体缺陷：`parseStrictBaseTokensFile` 仅凭 `@media (prefers-reduced-motion: reduce)` 前缀放行整个 `@media` 块，不解析内部规则与声明；该块可隐藏主题选择器、非颜色声明或未校验颜色令牌，且缺少对应负例测试。
- 总评结论：**退回返工**。

## 第 2 轮返工处理（Gemini，2026-11-26）

针对 #4、#5、#22 的最后 1 处盲区已修复：
- `tools/gen/tokens.go` 的 `parseStrictBaseTokensFile` 不再仅凭前缀放行 `@media` 块，而是精确要求规则头等于 `@media (prefers-reduced-motion: reduce)`，并递归调用 `parseTopLevelCSS` 校验内部非空、无嵌套 `@` 规则、选择器只能为 `:root` 或 `html[data-motion="reduce"]`、声明只能为 `--hx-t-*` 动效时长令牌。
- `tools/gen/assets_test.go`（`C14-T8`）补齐 3 条负例测试：`@media` 内嵌主题选择器与 `font-size`、`@media` 内嵌颜色令牌 `--hx-bg-0`、`@media (prefers-reduced-motion: reduce) and (min-width: 1px)` 前缀绕过，全部断言拒绝。

## 第 2 轮返工验证结果

- `go test ./tools/gen -count=1 -v`：全部通过（含 3 条 `@media` 负例测试）
- `golangci-lint run ./tools/gen/...`：0 issues

## 第 3 轮复核结论（Codex，2026-11-26）

- 已彻底解决：#4、#5、#22，共 3 条。
- `parseStrictBaseTokensFile` 现在结构化解析 `@media (prefers-reduced-motion: reduce)` 块，精确匹配规则头，递归解析内部规则，拒绝嵌套 `@` 规则、空块、非 `:root` / `html[data-motion="reduce"]` 选择器以及非 `--hx-t-*` 声明。
- 新增负例已覆盖 `@media` 内嵌主题选择器与普通声明、内嵌颜色令牌、前缀绕过三类关键绕过路径，均断言生成失败。
- 总评结论：**允许合并**。