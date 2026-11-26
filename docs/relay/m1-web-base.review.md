# 评审 · m1-web-base（M1 第 ⑧ 片）

- 评审者：Codex（web-accept · gpt-6.1-sol） · 2026-11-26
- 第 2 轮复核：Codex（web-accept · zai-glm-5-3） · 2026-11-26
- 依据：设计仓库 ebedbc3、595d1e1（W.12、W.10、W.04.03；P.09 K272、K300、K301、K302）；代码分支 slice/m1-web-base（提交 0b61968、d3e4738、ffb6d92、3bab176、4cb8b4d；检查基线 094a2d8）

| # | 实施卡 / 小节 | 结论 | 说明 |
|---|---|---|---|
| 1 | W.12 / 依赖、K272 | 符合 | `web/package.json` 的运行时及开发依赖均能在所给 `tools/deps/allow.txt` 找到；版本均为精确版本，没有 `^`、`~`；`typescript` 为 K272 要求的 `6.0.3`。依赖检查、生产依赖审计结果亦通过。 |
| 2 | W.12 / TypeScript 严格选项、保留桩 | 符合 | `web/tsconfig.json` 明确开启 `strict`、`noUncheckedIndexedAccess`、`exactOptionalPropertyTypes`、`noImplicitOverride`、`verbatimModuleSyntax`，并保留 `isolatedModules`、`noEmit`。所给完整 diff 没有修改 `web/index.html`、`web/src/main.ts`，符合 K300 的延期边界。 |
| 3 | W.12 §6.4 / ESLint 分层与自定义规则 | 不符合 | `strictTypeChecked`、三条 `hx/*` 规则及各自至少 3 个正反例已落地，但导入限制存在确定缺口：`web/eslint.config.js` 的 core 配置只按 `paths` 禁止准确的 `react-dom`，允许 `react-dom/client`；UI 配置使用 `**/state/**` 等模式，漏掉 `@/state`、`@/ops` 等目录入口；协议限制只禁止 `generated`，允许目录外直接导入 `@/lib/protocol/mapping`、`@/lib/protocol/types`，违反 W.04.03 §4 的唯一入口要求。此外，`hx/no-bare-store` 已在通用源码配置中启用，而 W.12 §6.4 明确规定由 W.03.05 启用。 已处理（复核：Codex zai-glm-5-3，2026-11-26） |
| 4 | W.12 / Stylelint，G.06.03、W.02.02 §4.3 | 不符合 | `web/stylelint.config.js` 已配置禁 hex、`!important`、ID、类名及变量前缀、宽度媒体查询；但颜色禁值规则只检查少数属性，且正则锚定值的开头。`border: 1px solid rgb(0 0 0)`、`background: linear-gradient(rgb(0 0 0), rgb(255 255 255))`、`box-shadow: 0 0 2px rgba(0,0,0,.2)` 均绕过裸色限制。颜色禁值数组中还混入 `{ message: ... }` 对象，消息应放在规则的次级选项位置。现有通过记录没有证明这些 CSS 反例被拒绝。 已处理（复核：Codex zai-glm-5-3，2026-11-26） |
| 5 | W.12 §6.2、W12-T3、T9 / 隔离与恢复 | 不符合 | `web/tests/kit/setup.ts` 的三种网络阻断、控制台核验、树卸载及存储清理已实现；但没有恢复全部 mock、全局替身和属性修改。`resetDomStubs()` 只清内部状态，`resetFakeTime()` 仅在 `fakeActive` 为真时恢复真实定时器，直接调用 `vi.useFakeTimers()` 的测试会泄漏状态。现有 `kit.test.tsx` 的平台测试已出现具体恢复错误：修改的是 `navigator` 自有 `platform`，恢复的却是 `Navigator.prototype.platform`，留下自有的 `Win32`。这不满足 §6.2“恢复全部替身”和测试不依赖顺序的要求。 已处理（复核：Codex zai-glm-5-3，2026-11-26） |
| 6 | W.12 §6.2、§6.8，W12-T17、T18 / 双时区与编号、构建检查 | 不符合 | `run-tz.mjs` 正确通过子进程分别设置两个 TZ，纽约阶段选择 i18n 与 `.tz.test.` 文件；但 `ids.mjs` 的 `TEST_CALL_RE` 只识别裸 `test(...)`、`it(...)`，完全漏掉合法的 `test.each(...)(...)`，重复编号可以因此不被发现，违反 §6.8 扫描全部测试名的要求。另，`lint.test.ts` 的 T18 只搜索 `tests/kit` 和一条网络错误文字；`raf.ts` 等工具代码不含这两段标记，打包后也可能没有路径文字，因此该断言不能证明 kit 代码没有进入产物。 已处理（复核：Codex zai-glm-5-3，2026-11-26） |
| 7 | W.10 §4.2–§4.5 / 文案表与生成消息 | 符合 | `web/src/lib/i18n/zh.ts` 的 52 个键仅属于定稿的 `common`、`fmt`、`ops.precheck`、`ui` 四组，实际区段顺序为 §4.3 要求的 common → fmt → ops → ui。键值均为单行单引号字面量；定稿文字、空格和标点与表一致，未命中所给 `terms.ts` 的禁用规则。`generated/messages.ts` 无修改；`messages.test.ts` 检查 ASSIGN 恰好六条及非空 `ERRORS.internal`。 |
| 8 | W.10 §6.1、§8 / `t`、`useT`、`hasText` | 符合 | `web/src/lib/i18n/t.ts` 使用自有属性检查，依次查询 zh、ERRORS、ASSIGN；八种格式分支及变量类型判断齐全，变量支持原名和 snake/camel 备用名。缺键开发模式返回括号标记并按键警告一次，生产模式返回原键并报错一次；缺变量保留占位符。`useT()` 直接返回同一个 `t`，没有订阅。 |
| 9 | W.10 §6.3、§10 验收 1 / 格式化边界 | 不符合 | 金额的整数拆分与解析、单位进位、午夜及当地日期计算均可在 `format.ts` 核对到；但 `fmtAbsTime` 和 `fmtRelTime` 的跨年日期分支都使用 `String(getFullYear())`，没有补足四位年份。合法输入 `0001-06-15T12:00:00Z` 的日期部分会输出 `1-06-15`，不符合规定的 `YYYY-MM-DD`。`format.test.ts` 只用现代年份，未覆盖该边界；此外，要求统一处理的 Infinity 和多数函数的 NaN／非法时间分支也没有对应测试，尚未满足“每个函数的每条边界都有用例”。 已处理（复核：Codex zai-glm-5-3，2026-11-26） |
| 10 | W.10 §6.4，W10-T12 / 时钟与重渲染验证 | 不符合 | `clock.ts` 的实现确有共享单个 timer、隐藏清理、可见刷新、冻结不排 timer、最后订阅者卸载清理等分支；`RelTime.test.tsx` 也验证了父组件不随分钟刷新。但 `clock.test.ts` 在“没有待执行定时器”“卸载后定时器归零”的位置，实际只断言 `renders`，从未检查待执行定时器数量；卸载前又已冻结时钟，无法验证活动时钟的卸载清理。也没有两个订阅者共用一个 timer 的断言。注释声称覆盖了资源约束，测试实际没有验证，未完成 W10-T12。 已处理（复核：Codex zai-glm-5-3，2026-11-26） |
| 11 | W.10 §6.2 / `errorText` | 符合 | `web/src/lib/i18n/errors.ts` 将三种 kind 分别映射到 `common.net.*`；非空 message 原样返回；已知码使用 `t.dyn` 和 details 变量，未知码返回 `ERRORS.internal`。`detailsToVars` 保留字符串、数字并支持数组文本化；测试包含 camelCase 的 `retryAfterMs`。本目录实现没有导入 `lib/protocol`。 |
| 12 | W.04.03 §5.1、§6.1、§6.2 / 映射及未知值回退 | 不符合 | `mapping.ts` 正常路径只读 WIRE_TYPES，raw 同引用、map 值转换、联合正反转换、undefined 省略及非普通对象返回均已实现；但 `fromWireObject`、`toWireObject` 直接使用 `union.branches[discVal]`，没有限定自有键。未知判别值 `constructor` 会取得继承的 Object 构造函数，随后作为类型名递归并在读取 `meta.union` 时抛错，而非按要求回退为 raw。`meta.fields[k]`／`meta.fields[sk]` 同样存在继承键问题，例如未知 `constructor` 字段的值为普通对象时也会走错分支。须对动态查表补自有键检查及双向反例。 已处理（复核：Codex zai-glm-5-3，2026-11-26） |
| 13 | W.04.03 / W0403-T1、T5 | 符合 | `mapping.test.ts` 的 T1 遍历解析出的全部合法 manifest 条目，根类型从 aliases roots 解析，`#/$defs/X` 取 X；四份向量分别按 Op、RunEvent、GraphEvent、SessionEvent 检查合法项的往返。T5 从所有 `WIRE_TYPES[*].fields` 收集字段名并逐项断言 `snakeKey(camelKey(k)) === k`，没有手写字段抽样名单。 |
| 14 | W.04.03 §5.2 / 类型别名及入口导出 | 符合 | `types.ts` 的别名统一为 `FromWire<gen.X>`，并提供四类联合提取工具；`index.ts` 导出五个 consts 对象、十二个枚举对象及六张表。T9 从生成源码提取导出名后求缺项，并核对实际入口对象；`mapping.test-d.ts` 纳入 TypeScript 检查，覆盖 registryHash、开放 params、嵌套 showIf 及 Event／Op 收窄。 |
| 15 | 整片 / 文件边界、测试强度与门禁 | 不符合 | 所给 diff 未修改禁改目录，未发现删除既有测试或使用 skip；提供的检查及扫描均通过。但新增 lint 测试没有达到 W.12 §6.4 要求的“bad 每个文件恰好且只报期望规则”：`web/tests/kit/lint.test.ts` 和 `web/src/lib/i18n/zh.test.ts` 都只用 `toContain` 检查规则子集。当前 canvas-core 反例在普通函数 `bad()` 中调用 useState，还会触发 `react-hooks/rules-of-hooks`，却被子集断言放过。需要按实际期望规则及次数严格核对，修正反例，不能以当前全绿作为该验收已完成的证明。 已处理（复核：Codex zai-glm-5-3，2026-11-26） |
| 16 | W.12 §4.2 / `loadExample` 接口 | 不符合 | `web/tests/kit/data.ts` 的 `loadExample<T = unknown>` 直接返回 `JSON.parse(raw) as T`，没有调用 `fromWire('Graph', ...)`，也没有规定的 Graph 返回类型。§4.2 明确要求读取示例后进行字段映射；K300 没有延期此函数。当前任意泛型断言会让调用方把 wire 对象误当作 camelCase 产品类型。 已处理（复核：Codex zai-glm-5-3，2026-11-26） |
| 17 | W.12 §4.2、§6.7 / ARIA 查询语义 | 不符合 | `web/tests/kit/render.tsx` 的 `queryAllByRoleIn` 只检查元素自身的 `aria-hidden`，仍会找到 `aria-hidden="true"` 祖先下的按钮，形成可访问性假通过；`accessibleName` 没有处理 input 关联的 label，且把 aria-label 放在 aria-labelledby 之前；`implicitRole` 还把 hidden、password 输入默认认作 textbox。这不符合“按 ARIA 角色和可访问名查找”的接口承诺。T1 只测直接 aria-label 按钮，未覆盖这些基本语义。 已处理（复核：Codex zai-glm-5-3，2026-11-26） |
| 18 | W.12 §4.2，W12-T2 / `user` 的产品交互语义 | 不符合 | `web/tests/kit/user.ts` 的 `type` 先执行 `el.value += tok`，再派发 input。对于 React 受控输入框，该赋值会同步更新 React 的 value tracker，后续事件无法可靠触发 onChange；现有 T2 只检查无 onChange 的输入框值和原生键盘日志，掩盖了问题。`tab` 则完全不派发 Tab 的 keydown／keyup，也不尊重 preventDefault，无法检验焦点陷阱等键盘处理。须补受控输入和 Tab 事件反例。 已处理（复核：Codex zai-glm-5-3，2026-11-26） |
| 19 | W.04.03 §5.1、§6.1、§6.2 / map 数据键保留 | 不符合 | `web/src/lib/protocol/mapping.ts` 的 `walkFrom`、`walkTo` 在 map 分支用普通 `{}` 后执行 `out[k] = ...`。来自 JSON 的自有 `__proto__` 数据键会调用原型 setter，变成输出对象的原型，而不是保留为自有数据键，导致序列化及往返丢失数据。违反 map“键是数据、原样”的要求。应采用不会触发该 setter 的建对象方式，并使用 JSON.parse 构造双向反例。 已处理（复核：Codex zai-glm-5-3，2026-11-26） |
| 20 | W.12 §6.2 / 测试环境分配 | 不符合 | `web/vitest.config.ts` 的 node 项目和 jsdom 排除项，对 ops、state、protocol、geom、layout 均只写 `**/*.test.ts`；相同目录中的 `.test.tsx` 会落入 jsdom。`tools/lint/eslint/**/*.test.tsx` 则不在任何项目 include 中。§6.2 是按目录指定 node 环境，没有排除 TSX 文件；不能只因当前测试恰好使用 `.ts` 就认定环境分配完整。 已处理（复核：Codex zai-glm-5-3，2026-11-26） |
| 21 | W.04.03 §10 验收 5 / snake_case 属性门禁 | 不符合 | `web/eslint.config.js` 没有配置 `@typescript-eslint/naming-convention`。目录外自行定义并访问 `cost_micros` 这样的属性，不会被现有 ESLint 规则拒绝，缺少验收 5 明确要求的属性命名检查。应按规定限定检查范围，并保留开放对象字符串键的例外。 已处理（复核：Codex zai-glm-5-3，2026-11-26） |
| 22 | W.12 §6.3，W12-T9 / 浏览器替身的接口与返回值 | 不符合 | `web/tests/kit/dom.ts` 的 `resize()` 将 `{}` 强转为 ResizeObserver 传给回调第二参数；合法回调调用该参数的 `disconnect()`／`unobserve()` 会立即抛错。`makeFake2dContext().measureText()` 还自行按 `text.length * 7` 猜测宽度，没有让测试显式给出测量结果，违反 §6.3“只模拟接口、不模拟浏览器计算结果”的要求。现有 T9 未覆盖这两处。 已处理（复核：Codex zai-glm-5-3，2026-11-26） |

## 总结

- 第 1 轮共 22 项：符合 7 项，不符合 15 项，有疑问 0 项。
- 第 2 轮复核结论：15 项不符合均已彻底处理，0 项未处理；返工未发现绕过、未修复或新引入的不符合项，未追加第 23 行及后续问题。
- 第 2 轮 `go run ./tools/relay slice m1-web-base`（`094a2d8` / `4cb8b4d`）：全部通过。W.12 ①–⑥ 11/11、W.10 ①–⑥ 15/15、W.04.03 ①–⑥ 9/9；⑦ `tools/build -check` 中 gen 9 个生成物改 0 个、golangci-lint 0 issues、depcheck 48 包 0 违规、quirkscheck 29 文件通过、typenamecheck 33 文件通过、`pnpm -C web lint` 通过、`go test ./...` 全绿、`pnpm -C web test` 双时区全绿。
- **允许合并**。
- K300 明确延期的外壳替换、相关假对象、端到端、视觉、体积和性能事项未计为不符合。
- 本次仅依据所给材料评审，未调用工具。以下原样汇总第 1 轮提供的本机检查与扫描结果：

1. `go run ./tools/relay slice m1-web-base`：全部通过（W.12 ①–⑥ 11/11、W.10 ①–⑥ 15/15、W.04.03 ①–⑥ 9/9；⑦ `tools/build -check`：gen 9 个生成物改 0 个、golangci-lint 0 issues、depcheck 48 包 0 违规、quirkscheck 29 文件通过、typenamecheck 33 文件通过、pnpm -C web lint 通过、go test ./... 全绿、pnpm -C web test 双时区 Asia/Shanghai 14 文件 39 测试 + America/New_York 8 文件 17 测试全绿）。
2. `go run ./tools/ci/deps`：直接依赖都在 `tools/deps/allow.txt` 里。
3. `pnpm -C web audit --audit-level high --prod`：No known vulnerabilities found。
4. `go run golang.org/x/vuln/cmd/govulncheck@v1.10.0 ./...`：No vulnerabilities found（exit 0）。
5. `go run github.com/zricethezav/gitleaks/v8@v8.30.1 git --redact --gitleaks-ignore-path tools/gitleaks/.gitleaksignore .`：60 commits scanned, no leaks found（exit 0）。
6. `pnpm -C web build && go run ./tools/build -preview && go run ./tools/release -smoke bin/anydoor.exe`：构建 windows/amd64 0.1.0-preview-dev.22，冒烟通过，已清理输出目录。


## 实施者处理（Gemini · 2026-11-26）

| # | 处理 |
|---|---|
| 3 | `web/eslint.config.js`：`canvas/core/**` 补禁 `react/*`、`react-dom/*`、`zustand/react/*` 与 `**/panels`/`**/components` 入口；`components/ui/**` 补全 `@/state`、`@/ops`、`@/shell` 等目录入口禁令；通用源码 `no-restricted-imports` 改为禁止 `@/lib/protocol/*`（`!@/lib/protocol`）和 `**/lib/protocol/*`，防止绕过唯一入口；按 `W.12` §6.4 去掉通用配置里提前启用的 `hx/no-bare-store`（留待 `W.03.05` 启用）。`lint.test.ts` 补对应 4 条反例与 `hx/no-bare-store` 未启用断言。 |
| 4 | `web/stylelint.config.js`：`declaration-property-value-disallowed-list` 改为对 `/.*/` 禁止任意位置出现 `rgb/rgba/hsl/hsla/hwb/lab/lch/oklab/oklch/color/color-mix(`，并将 `{ message }` 放到规则次级选项位置。`lint.test.ts` 用 `stylelint.lint()` 实测 `border: 1px solid rgb(0 0 0)`、`linear-gradient(...)`、`box-shadow: ... rgba(...)`、宽度媒体查询全部报错，合法令牌通过。 |
| 5 | `web/tests/kit/setup.ts` 与 `time.ts`：`afterEach` 增加 `vi.restoreAllMocks()`、`vi.unstubAllGlobals()`、`vi.unstubAllEnvs()`、恢复 `document.hidden=false`、重装网络守卫；`resetFakeTime` 改用 `vi.isFakeTimers()` 无条件恢复直接调 `vi.useFakeTimers()` 的测试；`kit.test.tsx` 改用 `vi.spyOn(navigator, 'platform', 'get')` 并 `mockRestore()`，不再污染实例属性。 |
| 6 | `web/tests/kit/ids.mjs` 的 `TEST_CALL_RE` 补齐 `(?:\.(?:each|for)\s*\([^)]*\))?`，`W12-T17` 补 `test.each` 识别与重复反例；`W12-T18` 改为调用 Vite JS API (`viteBuild({ build: { write: false } })`) 检查产包 chunk 的完整模块 ID 列表不含 `/tests/kit/` 与 `/tests/visual/gallery/`。 |
| 9 | `web/src/lib/i18n/format.ts` 新增 `pad4()` 补足四位年份用于 `fmtAbsTime` 与 `fmtRelTime` 跨年分支；`format.test.ts` 补 `0001-06-15T12:00:00Z`、非法 ISO 与全部格式化函数的 `NaN` / `Infinity` 边界用例。 |
| 10 | `web/src/lib/i18n/clock.test.ts` 重写 `W10-T12/clock`：断言双订阅者共用 1 个定时器（`vi.getTimerCount() === 1`）、隐藏后归零（`0`）、可见后恢复（`1`）、活动状态卸载第一个剩 `1` 卸载最后一个归 `0`，以及冻结后 `0` 定时器。 |
| 12 | `web/src/lib/protocol/mapping.ts` 全部动态查表（`union.branches`、`meta.fields`、读 `v[disc]`）改为先经 `hasOwn()` 判断自有属性；`mapping.test.ts` 的 `W0403-T4` 补未知判别值 `constructor`（字段联合 + 整体联合）与未知字段名 `constructor` 的双向安全回退测试。 |
| 15 | `web/tests/lint/canvas-core/bad/src/canvas/core/bad.ts` 函数改名为 `useBadCanvasHook`，消除无关的 `react-hooks/rules-of-hooks`；`lint.test.ts` 与 `zh.test.ts` 全部改为 `toEqual` 精确比对规则数组与条数。 |
| 16 | `web/tests/kit/data.ts` 的 `loadExample(name: string): Graph` 改为调用 `fromWire<Graph>('Graph', raw)` 返回 camelCase `Graph`，`W12-T1` 补验证 `schemaVersion` 存在且无 `schema_version`。 |
| 17 | `web/tests/kit/render.tsx`：`isAriaHidden` 沿祖先链检查 `aria-hidden="true"` 与 `hidden`；`accessibleName` 调整为 `aria-labelledby` 优先于 `aria-label` 并支持 `<label for>` 与包裹式 `<label>`；`implicitRole` 排除 `hidden`/`password` input。`W12-T1` 补齐全部反例。 |
| 18 | `web/tests/kit/user.ts`：`type()` 改用 `HTMLInputElement/HTMLTextAreaElement.prototype` 原生 `value` setter（`Reflect.apply`）以正常触发 React 受控 `onChange`；`tab()` 完整派发 `keydown`/`keyup` 并尊重 `preventDefault()`。`W12-T2` 改用 React 受控输入框并补 Tab 焦点陷阱反例。 |
| 19 | `web/src/lib/protocol/mapping.ts` 的 map 与对象写入全部改用 `setOwn`（`Object.defineProperty`），防止 `__proto__` 触发原型 setter；`W0403-T3` 用 `JSON.parse` 构造带自有 `"__proto__"` 键的 `NodeDef.params` 双向往返反例。 |
| 20 | `web/vitest.config.ts` 统一 `NODE_DIRS` 使用 `**/*.test.{ts,tsx}`，同时包含 `../tools/lint/eslint/**/*.test.{ts,tsx}`。 |
| 21 | `web/eslint.config.js` 在 `lib/protocol/` 以外源码启用 `@typescript-eslint/naming-convention`（属性禁 snake_case，保留引号键例外）+ `no-restricted-syntax` 拦截非计算成员访问的 snake_case 属性，`lint.test.ts` 补 snake_case 反例与开放对象字符串键正例。 |
| 22 | `web/tests/kit/dom.ts`：`resize()` 传入真实 `StubResizeObserver` 实例；`fake2d(o?)` 默认 `measureText` 返回 `{ width: 0 }`（不猜宽度），允许测试经 `o.measureText` 显式给定返回值。`W12-T9` 补对应断言。 |
| 说明 | 返工改动按 K238 / K302 记入片内最后一个叶子 `W.04.03` 实施卡白名单（设计仓库 `595d1e1`）；`W.10`、`W.04.03` 已 `--resume --accept-docs` 更新依据，整片将重跑 `relay slice m1-web-base` 后请独立评审者复核。 |
