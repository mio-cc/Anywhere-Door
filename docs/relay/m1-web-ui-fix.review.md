# m1-web-ui-fix 整片交叉评审

## 元信息

| 项目 | 内容 |
|---|---|
| 评审对象 | `m1-web-ui-fix` 修复片 |
| 评审轮次 | 第 2 轮独立复核 |
| 分支 | `slice/m1-web-ui-fix` |
| 基线 | `main` `bc699f0` |
| 提交范围 | `da84a2e..59c2c88`，共 9 个提交 |
| 返工增量 | `a471501..59c2c88`，共 2 个提交 |
| 评审者 | `Codex CLI (web-accept · zai-glm-5-3 · max)` |
| 任务依据 | `P.09` K307；包含 `W.08.05`、`W.08.04`、`W.08.03` |
| 评审方式 | 静态交叉评审；基于提供的返工增量 diff、全量 diff 与最新源码；未调用 shell、未修改文件、未执行测试 |
| 评审结论 | `允许合并` |

## 逐项检查

| # | 检查项 | 位置 | 结论 | 说明与证据 |
|---:|---|---|---|---|
| 1 | `Select` / `Picker` 窗口化滚动的 `onScroll` 必须绑定在真正滚动的外层 `ScrollArea`，且 `ScrollArea` 透传 `onScroll` | `web/src/components/ui/feedback/ScrollArea.tsx:10`，`web/src/components/ui/feedback/ScrollArea.tsx:29`，`web/src/components/ui/select/Select.tsx:474`，`web/src/components/ui/select/Picker.tsx:329` | 符合 | `ScrollAreaProps` 新增 `onScroll?: UIEventHandler<HTMLDivElement>`，并直接绑定在滚动根节点 `<div className="hx-scroll">` 上。`Select` 与 `Picker` 均把 `onScroll` 传给 `<ScrollArea>`，不再绑定在内层 `role="listbox"` 容器上。测试分别对 `.hx-select__scroll`、`.hx-picker__scroll` 设置 `scrollTop=5000` 并派发 `scroll`，证明事件落在真实滚动根节点。 |
| 2 | `Picker` “滚到当前项” effect 依赖不得包含 `scrollTop`，需用 ref 读取，防止用户滚动后被拉回；同时检查 `Select` | `web/src/components/ui/select/Picker.tsx:139`，`web/src/components/ui/select/Picker.tsx:239`，`web/src/components/ui/select/Select.tsx:140`，`web/src/components/ui/select/Select.tsx:262` | 符合 | `Picker` 与 `Select` 均维护 `scrollTopRef`，真实滚动与程序性滚动时同步 ref；滚动到当前项时优先读取 `scroll.scrollTop`，无 DOM 时读取 ref。两个 effect 依赖均不含 `scrollTop` state，用户仅滚动不会触发 active-item effect 重新执行。 |
| 3 | 补充 600 项窗口化滚动测试（含 `W0803-T12`）：滚动容器触发 `scroll(scrollTop=5000)`，断言渲染窗口切到对应位置且不被拉回当前项，`Select` 与 `Picker` 均测 | `web/src/components/ui/select/Select.test.tsx:203`，`web/src/components/ui/select/Picker.test.tsx:126` | 符合 | `Select` 与 `Picker` 均在 600 项场景中对真实滚动容器设置 `scrollTop=5000` 并派发 `scroll`。测试断言滚动位置保持 5000，分别渲染 `Model 178` 与 `项 156`，且首项不再出现在 DOM 窗口中，同时覆盖“切窗”和“不被当前项拉回”两个关键行为。 |
| 4 | `Menu` 点坐标锚点按 `at.x`、`at.y` 用 `useMemo` 缓存 `DOMRect`；悬停/方向键重渲染不重复触发 `place()`，并补测试 | `web/src/components/ui/overlay/Menu.tsx:101`，`web/src/components/ui/overlay/Menu.tsx:104`，`web/src/components/ui/overlay/Menu.tsx:280`，`web/src/components/ui/overlay/Menu.test.tsx:175` | 已处理 | 点坐标通过 `ptX`、`ptY` 用 `useMemo` 生成稳定 `pointAnchor`，方向键与悬停造成的 `activeIdx` 重渲染不会改变 anchor 引用。测试先执行两次 `ArrowDown` 并断言第二项 active，再对第一项派发 `pointerover`，随后断言第一项 active、第二项失活，实际制造 `activeIdx 1 → 0` 的状态变化；最终 `mockComputePosition` 仍只调用一次。 |
| 5 | `Overlay.focusInitial` 只在 `open` 从 `false` 变为 `true` 时调用一次，不依赖 `env`，不破坏 `pushLayer` 顺序与嵌套浮层行为，并补 Provider 重渲染不抢焦点测试 | `web/src/components/ui/overlay/Overlay.tsx:121`，`web/src/components/ui/overlay/Overlay.tsx:210`，`web/src/components/ui/overlay/Overlay.test.tsx:421`，`web/src/components/ui/overlay/Overlay.test.tsx:449` | 已处理 | 初始焦点 effect 依赖改为且仅为 `[open]`，不再依赖 `env` 或 `initialFocus`。`initialFocusRef` 由更早声明的 layout effect 同步，保证首次打开时读取当前值。首次进入 `open=true` 时，effect 先设置 `focusedForOpenRef.current = true`，再按 `mode !== 'none'` 决定是否调用 `focusInitial`，因此在首帧即完成锁定。`pushLayer` effect 仍先于焦点 effect 执行，层级顺序不变。新增测试覆盖 Provider 重渲染，以及打开期间 `initialFocus` 从 `none` 改为 `first` 时输入框焦点保持不变。 |
| 6 | `Select` / `Picker` 选项 DOM `id` 不直接拼原始 value，改用安全编码或序号，补含空格值测试，并检查碰撞与边界 | `web/src/components/ui/select/Select.tsx:68`，`web/src/components/ui/select/Select.tsx:85`，`web/src/components/ui/select/Picker.tsx:69`，`web/src/components/ui/select/Picker.tsx:87`，`web/src/components/ui/select/Picker.test.tsx:174`，`web/src/components/ui/select/Select.test.tsx:236` | 已处理 | `encodeDomIdToken` 逐个处理 UTF-16 code unit：ASCII 字母数字直接保留，其他 code unit 输出 `_` + 十六进制码点 + `_`。直接原子不含 `_`，转义 payload 也不含 `_`，两类原子首字符互斥，因此 token 流可从左到右唯一解析，`encodeDomIdToken` 对任意 JS 字符串是单射函数。该实现不调用 `encodeURIComponent`，不处理 Unicode 标量，而是逐 code unit 读取 `charCodeAt`，因此未配对代理项不会抛 `URIError`。输出仅包含 ASCII 字母数字、`_`、最终固定 `-`，不含空白；token 内部也不含 `-`，`listId` 与 `itemId` 编码后由固定 `-` 分隔，二元组仍可唯一还原，无分隔符碰撞。`Select` 与 `Picker` 的辅助函数行为一致，并均同时编码 `listId` 与选项值。测试覆盖空串、空格、字面量 `%20`、未配对代理项、带空格 `listId`、ID 唯一性、DOM 可查找性与分隔符边界碰撞。 |
| 7 | `ErrorBoundary` 使用独立 `hasError: boolean`，不以 `error === null` 判断，覆盖 `throw null` | `web/src/components/ui/feedback/ErrorBoundary.tsx:10`，`web/src/components/ui/feedback/ErrorBoundary.tsx:23`，`web/src/components/ui/feedback/ErrorBoundary.tsx:31`，`web/src/components/ui/feedback/ErrorBoundary.test.tsx:66` | 符合 | state 使用独立 `hasError` 标志；`getDerivedStateFromError` 返回 `{ hasError: true, error }`；`componentDidUpdate`、`retry` 与 `render` 均以 `hasError` 判断。新增测试覆盖 `throw null`、显示 fallback、retry 后恢复成功，不依赖 `error === null`。 |
| 8 | `Menu` 默认 `ariaLabel` 使用 `t('ui.menu')`，`zh.ts` 增加 `'ui.menu': '菜单'`；`Dialog` 有 error 时把 `FieldError` id 加入 `aria-describedby` 并补断言 | `web/src/components/ui/overlay/Menu.tsx:104`，`web/src/lib/i18n/zh.ts:90`，`web/src/components/ui/overlay/Menu.test.tsx:149`，`web/src/components/ui/overlay/Dialog.tsx:96`，`web/src/components/ui/overlay/Dialog.test.tsx:37` | 符合 | `Menu` 使用 `ariaLabel ?? t('ui.menu')`，中文文案为 `'菜单'`，测试断言 `role="menu"` 的 `aria-label`。`Dialog` 在存在 error 时传入 `${bodyId} ${errId}`，`FieldError` 同帧携带 `errId`，测试断言 `aria-describedby` 同时包含 body 与 error 的 ID。 |

## 返工复核摘要

- `Menu` 悬停测试已制造真实 `activeIdx 1 → 0` 变化，并保留方向键重渲染路径；`computePosition` 仍只调用一次。
- `Overlay` 初始焦点 effect 依赖已收敛为 `[open]`，首帧锁定 `focusedForOpenRef`，打开期间 `initialFocus` 变化不会抢焦点。
- 选项 ID 编码对任意 JS 字符串无异常、无空白、无分隔符碰撞，并保持 `listId + itemId` 二元组单射。
- 对返工增量与全量 diff 做了新缺陷扫描，未发现引入新的行为、可访问性、层级栈或测试回归。
- 第 1 轮三项关注点均已彻底关闭。

**明确结论：允许合并**
