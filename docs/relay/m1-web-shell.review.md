# m1-web-shell 整片交叉评审

## 元信息

| 项 | 内容 |
|---|---|
| 评审对象 | `slice/m1-web-shell`，M1 第 ⑨c 片 `m1-web-shell` |
| 评审轮次 | 第 2 轮独立复核 |
| 基线 | `main` `bc699f0` |
| 最新提交 | `8b880ee` |
| 提交范围 | `bc699f0..8b880ee` |
| 返工增量 | `a48c296..8b880ee` |
| 评审者 | `Codex CLI (web-accept · zai-glm-5-3 · max)` |
| 依据 | `P.09` K305 / K307 / K308 / K309；叶子 `W.03.03`、`W.02.03`、`W.02.04`、`W.02.01`、`W.02.02` |
| 评审方式 | 只读静态复核；未调用 shell、未修改文件、未运行测试 |
| 复核范围 | 第 1 轮报告、返工增量 diff、全部受影响最新源码与新增回归用例 |
| 明确结论 | **允许合并** |

---

## 逐项检查表

### `W.03.03`：`types.ts`、`uiStore.ts`

| # | 检查项 | 复核证据 | 结论 |
|---|---|---|---|
| 1 | `ShellLayout` 持久化与非法值不覆写 | `web/src/state/uiStore.ts:96` 的键名仍限定为 `hx:*`；`readBoolFlag()` 在 `web/src/state/uiStore.ts:136` 只识别 `1/0`，非法值直接回退默认，不在读取阶段写回存储；初始化逻辑在 `web/src/state/uiStore.ts:228` 保持宽 `<900` 时左栏默认关闭。 | 符合 |
| 2 | 专注模式进入 / 退出 / `setLs(true)` / `setRs(true)` | `setLs(true)` 在 `web/src/state/uiStore.ts:259` 退出专注后，除写入 `KEY_LS_OPEN` 外还同步写入 `KEY_RS_OPEN`；`setRs(true)` 在 `web/src/state/uiStore.ts:275` 对称同步写入 `KEY_LS_OPEN`。两侧实际开合与刷新后的持久化值一致；`web/src/state/uiStore.test.ts:66` 增加重建 store 的断言。 | 已处理 |
| 3 | `select` 跨层替换、去重、同值不 `set`、`pruneSelection`、`attachable` 50 上限 | `select()` 在 `web/src/state/uiStore.ts:339` 先归一化并去重，再以 `selectionEqual()` 避免同值写入；`pruneSelection()` 仅在过滤结果变化时替换 Map；`attachable()` 保持在 `web/src/state/uiStore.ts:520` 截断到 50 并返回 `truncatedFrom`。 | 符合 |
| 4 | `rsDot` 不自动打开右栏、toast 分组替换与 20 条上限 | `rsDot()` 在 `web/src/state/uiStore.ts:437` 只读取 selection，不调用 `setRs()`；`notify()` 对同 `groupKey` 原地替换，非分组超过 20 条时移除最旧项。 | 符合 |

### `W.02.03`：`boot.js`、`index.html`、`base.css`、`themeController.ts`、`useTheme.ts`

| # | 检查项 | 复核证据 | 结论 |
|---|---|---|---|
| 5 | `boot.js` 体积、纯 JS、无模块依赖、CSP、属性一致性 | `web/public/boot.js:1` 仍为纯 IIFE，无 import/export；`web/index.html:6` 只引入启动脚本，未放宽 CSP；`data-palette`、`data-theme`、`data-motion` 与 controller 默认值和解析规则一致。返工未改变该行为。 | 符合 |
| 6 | `data-theme-switching` 两帧移除与零时长令牌 | `web/src/shell/theme/themeController.ts:143` 保持嵌套 rAF 移除，并在重复切换时取消旧 rAF；`web/src/styles/base.css:3` 继续用 `0s` 令牌覆盖全局时长。 | 符合 |
| 7 | `syncFromServer` 服务端优先、未知配色回退并只警告一次 | `web/src/shell/theme/themeController.ts:313` 保持合法服务端值优先写入本地状态；未知 palette 回退默认配色，并通过一次性标记避免重复警告。 | 符合 |
| 8 | `PATCH /settings` 300ms 合并、失败重试、storage 跨标签同步 | `web/src/shell/theme/themeController.ts:174` 保持 300ms 尾随合并；失败后下一次 PATCH 携带当前完整 palette/mode；`web/src/shell/theme/themeController.ts:270` 的 storage 同步不触发服务端 PATCH。 | 符合 |
| 9 | `useTheme` 精确订阅与 React 19 快照稳定性 | `web/src/shell/theme/useTheme.ts:11` 先让 `useSyncExternalStore` 快照稳定的 `controller.get()`，再执行 selector；selector 返回新对象不再参与快照比较。`web/src/shell/theme/themeController.test.ts:207` 覆盖新对象 selector 的单次更新渲染。 | 已处理 |

### `W.02.04`：`normalize.ts`、`registry.ts`、`zh.ts`、`eslint.config.js`

| # | 检查项 | 复核证据 | 结论 |
|---|---|---|---|
| 10 | 物理键位 `e.code`、macOS `Mod=Meta`、其他平台 `Mod=Ctrl` | `web/src/shell/keys/normalize.ts:58` 主键仍优先使用 `e.code` 并映射字母、数字、F 键、方向键等；`web/src/shell/keys/normalize.ts:92` 保持 macOS 用 `metaKey`、其他平台用 `ctrlKey`。 | 符合 |
| 11 | `validateShortcuts` 五项检查完整性 | `isBareSingleKey()` 在 `web/src/shell/keys/registry.ts:123` 改为“非空且不含 `+`”，覆盖 `Escape`、`Delete`、`Backspace`、方向键、F 键等所有裸命名键；`validateShortcuts()` 继续检查重复 ID、label、保留键、层内冲突与期望 ID 集合。`web/src/shell/keys/registry.test.ts:64` 补充命名裸键断言。 | 已处理 |
| 12 | 7 层分发顺序与 modal 只放行 `estop.trigger` | `web/src/shell/keys/registry.ts:295` 分发顺序仍为 modal → popover → peek → composer → editable → nest/canvas → global；modal 未处理时仅尝试 `estop.trigger`，其余直接终止。 | 符合 |
| 13 | IME、`e.repeat`、hold `keyup`、`window.blur`、异常不落层 | `web/src/shell/keys/registry.ts:99` 处理 `isComposing`/`keyCode === 229`；`web/src/shell/keys/registry.ts:257` 限制 repeat；hold down 记录 binding，keyup 或 blur 补发；handler 异常在 invoke 层捕获并视为已处理。 | 符合 |
| 14 | 全局 `keydown` / `keyup` ESLint 门禁完整且不误伤 | `web/eslint.config.js:191` 拦截 `addEventListener('keydown'|'keyup')` 字面量调用；`web/eslint.config.js:199` 拦截 `onkeydown` / `onkeyup` 属性赋值；文案明确为“禁止直接调用/赋值”，控件内建议 React `onKeyDown`，语义一致。`web/src/shell/keys/registry.test.ts:309` 覆盖赋值形态。 | 已处理 |
| 15 | 44 条 `shortcut.*` 文案 | `web/src/lib/i18n/zh.ts:38` 的 `shortcut.*` key 数量仍为 44，与 `EXPECTED_SHORTCUT_IDS` 数量一致；返工未减少文案。 | 符合 |

### `W.02.01`：`nav.ts`、`SessionRoute.tsx`、`routes/index.tsx`、`eslint.config.js`

| # | 检查项 | 复核证据 | 结论 |
|---|---|---|---|
| 16 | `toUrl` / `parseUrl` 严格 ULID 校验与往返 | `toUrl()` 在 `web/src/routes/nav.ts:26` 运行时校验 session、graph、scope，非法值抛 `RangeError`；`parseUrl()` 在 `web/src/routes/nav.ts:47` 使用精确路径正则，拒绝空路径段、尾斜杠、空 scope、重复 scope、非法 ULID，并用 `getAll('scope')` 保证唯一 scope。`web/src/routes/nav.test.ts:29` 覆盖这些非法形态。 | 已处理 |
| 17 | `leaveTo` 命中上一条历史时 `navigate(-1)` | `web/src/routes/nav.ts:220` 的 `NavHistoryTracker` 仍维护内存栈；目标等于 `previousPath()` 时调用 `navigate(-1)`，否则正常 push 目标 URL。 | 符合 |
| 18 | `createLocationSaver` 1 秒尾随节流 | `web/src/routes/nav.ts:180` 每个 session 独立维护 pending 与 timer，重复调用重置 1 秒计时，最终只保存最后一次位置；`flush()` 与 `dispose()` 行为保留。 | 符合 |
| 19 | 路由解析、404、非法 scope、归属图检查 | `graphBelongs` 在 `web/src/routes/SessionRoute.tsx:228` 要求 `graphIds.includes(gid)` 或 `activeGraphId === gid`，不再把缺失归属信息判为通过；`scopeValid` 在 `web/src/routes/SessionRoute.tsx:238` 要求非 root scope 必须存在 `scopeExists` 且返回 true。无效图或 scope 均重定向，不激活。 | 已处理 |
| 20 | `hx:last-session` 与标题更新 | `HomeRoute`、`WelcomeRoute`、`FallbackRoute` 分别在 `web/src/routes/SessionRoute.tsx:129`、`web/src/routes/SessionRoute.tsx:159`、`web/src/routes/SessionRoute.tsx:180` 将 `document.title` 复位为 `Anywhere Door`；SessionRoute 仍写入合法最近会话并更新会话标题。 | 已处理 |
| 21 | `useNavigate` ESLint 限制是否严密 | `web/eslint.config.js:84` 同时限制 `react-router` 与 `react-router-dom` 的 `useNavigate` 导入；patterns 覆盖 `react-router/*`、`react-router-dom/*`；`web/eslint.config.js:203` 拦截 `.useNavigate` 成员访问。`web/src/routes/nav.test.ts:95` 覆盖 dom 导入与 namespace 成员访问。 | 已处理 |
| 22 | `SessionRoute` effect 依赖稳定性与重复副作用 | `parsed` 在 `web/src/routes/SessionRoute.tsx:212` 经 `useMemo` 稳定；激活/保存 effect 与标题 effect 拆分，标题更新在 `web/src/routes/SessionRoute.tsx:312` 独立执行；同 key 激活由 `lastActivatedKeyRef` 防重。`web/src/routes/SessionRoute.test.tsx:168` 验证仅审批数变化时不重复激活/保存。 | 已处理 |

### `W.02.02`：断点、布局、外壳、路由集成与 CSS

| # | 检查项 | 复核证据 | 结论 |
|---|---|---|---|
| 23 | 断点边界 759/760/899/900/1099/1100/1179/1180 | `bpOf()` 在 `web/src/state/types.ts:23` 保持 `<760` 为 `xs`、`<900` 为 `sm`、`<1100` 为 `md`、`<1180` 为 `lg`、否则 `xl`，边界值未漂移。 | 符合 |
| 24 | 全前端唯一断点表 | 断点常量与 `bpOf()` 唯一收敛在 `web/src/state/types.ts:13`；`web/src/shell/breakpoints.ts:1` 仅转发 `BREAKPOINTS`、`bpOf` 与类型，`uiStore` 复用同一实现，无双源阈值。 | 已处理 |
| 25 | `fitInsets` 几何常量与浮动回复 80 夹紧 | `web/src/shell/layout.ts:10` 保持顶部/右侧/根图左侧/子图左侧/底部基础值；`web/src/shell/layout.ts:25` 继续将浮动回复高度夹紧到 80，并把回复高度计入底部 inset。 | 符合 |
| 26 | 覆盖式右栏不改主列宽、点击画布空白关闭 | `web/src/shell/AppShell.tsx:154` 只在覆盖式右栏可见时处理画布 pointerdown，并用 `closest()` 排除 `.hx-canvas__float`、`.hx-canvas__overlay`、`.hx-canvas__cover`、`.hx-canvas__page` 及 node/edge/note；CSS 仍保持覆盖式右栏不改主列宽。`web/src/shell/AppShell.test.tsx:86` 覆盖浮动层不误关与空白关闭。 | 已处理 |
| 27 | 小地图避让与 `ResizeObserver` rAF 合并 | `web/src/shell/FloatLayer.tsx:68` 的 RO 回调先取消 pending rAF 再重新调度，同帧多次回调只执行一次 `measure()`；cleanup 同时断开 RO 并取消 pending rAF。初始 measure 仍同步执行。 | 已处理 |
| 28 | 小地图隐藏后的稳定性 | `web/src/shell/CanvasArea.css:58` 将 `.is-hidden` 改为 `visibility: hidden; pointer-events: none`，保留布局 rect，RO 不会再因 `display:none` 的 0 尺寸触发显示/隐藏振荡。 | 已处理 |
| 29 | `mode='page'` 与 `/welcome` 的 `inert` | `RouteShellProps` 在 `web/src/routes/SessionRoute.tsx:26` 增加 `welcomePage`；`WelcomeRoute` 在 `web/src/routes/SessionRoute.tsx:156` 传 `true`；`web/src/App.tsx:40` 透传给 `AppShell`。`mode='page'` 下左栏 inert，welcome 下顶部条也 inert。`web/src/shell/AppShell.test.tsx:205` 覆盖 `/welcome` 集成。 | 已处理 |
| 30 | `cover` 隐藏浮动层但保持 `<Canvas/>` 挂载 | `web/src/shell/CanvasArea.tsx:57` 保持 session 模式下画布 stage 始终渲染；存在 cover 时不渲染 FloatLayer，cover 以更高层级覆盖画布。 | 符合 |
| 31 | `App.tsx` 使用稳定 `shellUiEnv` | `web/src/shell/uiEnv.ts:15` 保持模块级固定常量；`web/src/App.tsx:27` 默认使用该常量，仅显式注入 `p.uiEnv` 时替换，避免 Overlay effect 因环境对象变化重跑。 | 符合 |
| 32 | CSS Stylelint：无媒体查询、无裸色/字号/圆角/层级/时长 | `web/src/shell/AppShell.css:105` reduced-motion 改为 `transition: none`，组件 CSS 不再直接写裸时长；颜色、字号、圆角、层级仍使用 token，新增样式未引入媒体查询。 | 已处理 |
| 33 | 入口替换与删除 `main.ts` | `web/index.html:9` 指向 `/src/main.tsx`；`web/src/main.tsx:1` 为当前入口，旧 `main.ts` 已删除。返工未回退入口。 | 符合 |

---

## D01–D14 返工闭环汇总

| 缺陷 | 闭环复核 | 状态 |
|---|---|---|
| D01 | `setLs(true)` 退出专注时同步写右栏关闭态，`setRs(true)` 对称同步写左栏关闭态；测试通过重建 store 验证刷新后的两侧布局。 | 已解决 |
| D02 | `useSyncExternalStore` 快照改为稳定 `controller.get()`，selector 在快照之后执行；新增新对象 selector 回归测试。 | 已解决 |
| D03 | `isBareSingleKey` 覆盖所有非空且不含 `+` 的组合，显式测试 `Escape`、`Delete`、`ArrowUp`、`F2`。 | 已解决 |
| D04 | ESLint 同时覆盖 `addEventListener('keydown'|'keyup')` 与 `onkeydown` / `onkeyup` 属性赋值，报错文案明确禁止直接监听。 | 已解决 |
| D05 | `toUrl` 运行时校验 session/graph/scope；`parseUrl` 使用严格路径正则和 `getAll('scope')`，拒绝空段、尾斜杠、空/重复/非法 scope。 | 已解决 |
| D06 | 归属图必须命中 `graphIds` 或 `activeGraphId`；非 root scope 必须由 `scopeExists` 验证，失败时重定向，不激活。 | 已解决 |
| D07 | `useNavigate` 限制覆盖 `react-router`、`react-router-dom`、子路径导入与 `.useNavigate` 成员访问，并补充对应 ESLint 测试。 | 已解决 |
| D08 | `parsed` memo 化；激活/保存 effect 与标题 effect 拆分；同 location 防重；测试验证审批数变化只更新标题。 | 已解决 |
| D09 | 断点表与 `bpOf` 唯一收敛在 `state/types.ts`，`shell/breakpoints.ts` 仅转发，`uiStore` 复用同一实现。 | 已解决 |
| D10 | 覆盖式右栏关闭处理排除浮动层、overlay、cover、page、node、edge、note；测试覆盖 composer 点击不误关与画布空白关闭。 | 已解决 |
| D11 | FloatLayer 的 RO 回调经 rAF 合并，重复回调先取消旧 rAF；卸载时取消 pending rAF。 | 已解决 |
| D12 | minimap 隐藏态改为 `visibility: hidden; pointer-events: none`，保留 rect，避免隐藏/显示振荡。 | 已解决 |
| D13 | `RouteShellProps` 增加 `welcomePage`，WelcomeRoute 传 `true`，App 透传给 AppShell；welcome 下顶部条与左栏均 inert。 | 已解决 |
| D14 | reduced-motion 使用 `transition: none`，组件 CSS 零裸时长。 | 已解决 |

---

## 总结

- 33 项检查结果为：18 项维持 `符合`，15 项确认 `已处理`，0 项 `不符合`。
- D01–D14 全部闭环；第 1 轮的 #20 标题复位疑问也已由 Home / Welcome / Fallback 统一复位关闭。
- 返工扫描未发现新引入的真实缺陷：断点仍为单一来源，未新增直接键盘监听或 `useNavigate` 绕过，覆盖右栏点击保护完整，CSS 无新的裸时长或媒体查询。
- 本轮按要求未调用 shell、未修改文件、未复跑测试；结论基于题面提供的最新源码、返工 diff 与回归用例进行只读复核。

**明确结论：允许合并。**
