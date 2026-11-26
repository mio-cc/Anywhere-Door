# M1 第 ⑨b 片 `m1-web-ui` 独立交叉评审报告

- 评审日期：2026-11-27
- 评审范围：`W.08`、`W.08.01`–`W.08.05` 及提交方提供的基础组件库产品源码、CSS、测试工具与返工 diff
- 评审方式：静态交叉评审；第 2 轮基于返工代码与新增回归测试，第 3 轮基于第 2 轮返工 diff，未重复执行命令
- 评审结论：13 条第 1 轮 `不符合` 全部彻底解决；允许合并

## 评审表

| # | 实施卡 / 小节 | 结论 | 说明 |
|---|---|---|---|
| 1 | `W.08` §4.1–§4.2、`W.08-R1`、`W.08-R13` | 符合 | 产品源码内未发现 `state`、`ops`、`protocol`、`shell`、`panels`、`pages`、`canvas`、`routes` 导入；`@floating-ui/dom` 仅出现在 `web/src/components/ui/overlay/position.ts:1`；`ErrorBoundary` 是唯一 class 组件。 |
| 2 | `W.08` §4.2、`W.08-R2` | 符合 | `FieldErrorProps` 已定义并在 `web/src/components/ui/index.ts` 公开导出；`Menu` 将递归内部 props 收敛到未导出的 `InternalMenuProps`，公开 `MenuProps` 不再暴露内部接口。已处理（复核：Codex，2026-11-27） |
| 3 | `W.08.04` §6.1：`Overlay` 压栈、嵌套点击、退出动画 | 符合 | 遮罩不再自带 `onPointerDown`，外部点击统一由捕获阶段监听处理，单次点击最多触发一次 `onClose('outside')`；`onAnimationEnd` 校验 `e.target === e.currentTarget`；退出保底时长按 `--hx-t-2` 推导。新增遮罩单次回调与子元素 `animationend` 回归测试。已处理（复核：Codex，2026-11-27） |
| 4 | `W.08.04` §6.2、§9：`Popover` 首帧定位与错误处理 | 符合 | `syncInitialPosition` 在 `useLayoutEffect` 内同步写入首帧坐标，后续由 `computePosition` 精确修正；零尺寸重试条件已覆盖浮层或 HTMLElement 锚点任一尺寸为 0。已处理（复核：Codex，2026-11-27） |
| 5 | `W.08.04` §6.3：`Menu` 键盘与禁用项 | 符合 | 方向键循环且不跳过禁用项，`Home` / `End`、typeahead、`Enter` / 空格对禁用项不触发、禁用原因 Tooltip 的基本语义符合设计。 |
| 6 | `W.08.04` §6.3：`Menu` 可访问性 | 符合 | `Popover` / `Overlay` 已透传 `ariaActiveDescendant`，`aria-activedescendant` 直接挂在 `role="menu"` 的内容根上；新增单测断言。已处理（复核：Codex，2026-11-27） |
| 7 | `W.08.04` §6.3：子菜单指针生命周期 | 符合 | 父菜单项与菜单、子菜单容器已补 `pointerenter` / `pointerleave` 联动；离开父项或菜单树后 300ms 内未进入子菜单会自动关闭。新增生命周期回归测试。已处理（复核：Codex，2026-11-27） |
| 8 | `W.08.04` §6.4：`Dialog` 危险按钮与非可关闭模式 | 符合 | danger 按钮稳定排在最右；`dismissible=false` 时点遮罩与 Esc 都不关闭，且 Esc 被吞掉不下落；`busy` 置灰全部按钮，`error` 渲染为红字。 |
| 9 | `W.08.04` §6.4：`Dialog` 初始焦点 | 符合 | 全 danger 动作且无显式 `autoFocus` 时，初始焦点按正文第一个可聚焦元素 → 面板本身处理，不再默认聚焦 danger 按钮。新增专项回归测试。已处理（复核：Codex，2026-11-27） |
| 10 | `W.08.04` §6.5：`Tooltip` 400ms / 300ms 暖场 / 单实例 / Esc | 符合 | 离开、失焦、按下、Esc、`disabled` 与卸载等隐藏路径均统一记录 `lastHiddenAt`，300ms 暖场不再按隐藏原因遗漏。新增 Esc 后 150ms 暖场回归测试。已处理（复核：Codex，2026-11-27） |
| 11 | `W.08.04` §8：`Tooltip` 子元素 ref 契约 | 符合 | `cloneElement` 注入 `data-hx-tip-bound` 标记，`useLayoutEffect` 同时校验 ref 指向 `HTMLElement` 且该标记落在同一 DOM 节点；只透传 ref 但丢弃其余 DOM / 事件 props 的子组件会在开发模式抛错、生产模式 `console.error` 一次。新增 `RefOnlyChild` 回归测试。已处理（复核：Codex，2026-11-27） |
| 12 | `W.08.05` §6.1–§6.6：`Toast`、`Badge`、`StatusDot`、`Spinner`、`ProgressBar`、`EmptyState` | 符合 | `Toast` 五级 role / 图形 / 动作上限 / 悬停与焦点合并 `onHover` 符合；`Badge` 弱色底实色字；`StatusDot` 五种形状差异和读屏文字符合；`Spinner` 减少动效为静态半圆；`ProgressBar` 仅 `scaleX`；`EmptyState` 最多 2 个动作且无插画。 |
| 13 | `W.08.05` §6.7–§6.8：`ScrollArea`、`ErrorBoundary` | 符合 | 滚动条平时隐藏、悬停 / 焦点显示，`stable`、`axis`、`ref` 符合；`ErrorBoundary` 日志、手动 retry、`resetKey` 自动恢复符合，且不向外冒泡渲染错误。 |
| 14 | `W.08.01` §6.1–§6.5：按钮、图标、键帽、流动高亮 | 符合 | `Button` 五变体、加载锁宽、禁用原因可聚焦且不触发；`IconButton` 空标签校验、dot、Tooltip 符合；`Kbd` 不拆按键；`Icon` / `iconSvg` 使用元素构造或模板克隆且无 `innerHTML`；`GlideGroup` FLIP 只过渡 `transform`，指针移动期间不触发 React 提交。 |
| 15 | `W.08.02` §6.1–§6.3：`TextInput`、`TextArea` | 符合 | `TextInput` 的 `Enter` 与 `TextArea` 的 `Mod+Enter` 均补上 `stopPropagation`；`TextArea` 行高与上下内边距在挂载时测量并缓存，多次输入不重复读取 computed style。新增冒泡与 `getComputedStyle` 调用次数回归测试。已处理（复核：Codex，2026-11-27） |
| 16 | `W.08.02` §6.4、§7–§9：`NumberStepper` 手势、滚轮、浮层 | 符合 | `handleLostPointerCapture` 区分两类时机：越过 3px 阈值前丢失捕获时清空 `pendingDragRef`，避免悬空状态并防止后续 `pointermove` 误触发；进入拖动手势后（`drag.moved || modeRef.current === 'dragging'`）丢失捕获立即 `endGesture()`，保证 `onStart` / `onEnd` 成对。新增两类捕获丢失回归测试。已处理（复核：Codex，2026-11-27） |
| 17 | `W.08.02` §6.5、§7：`Slider` 手势与浮层 | 符合 | 手势进行中 `disabled` / `readOnly` 变化会立即 `finish()` 并补发 `onEnd`；浮出数值使用 `Overlay layer="tooltip"`，并按滑块矩形与比例同步写入 `left` / `top`。新增专项回归测试。已处理（复核：Codex，2026-11-27） |
| 18 | `W.08.02` §6.6：`SearchInput` | 符合 | 默认 `role="searchbox"`，有 `ariaControls` 时切换 combobox 并提供 `aria-expanded`、`aria-autocomplete`、`aria-activedescendant`；有值 Esc 清空并阻止冒泡，无值 Esc blur 且不拦截，`onEscape='none'` 不处理；清空按钮 `tabIndex=-1` 且点击后焦点回到输入框。 |
| 19 | `W.08.03` §6.1：`Select` 过滤、键盘、窗口化、可访问性 | 符合 | 外层 `Popover` 改为 `role="dialog"`，消除嵌套 `listbox`；无过滤模式在焦点容器上暴露 `aria-activedescendant`；过滤匹配同时加粗 `label` 与 `desc`；窗口化按偏移数组直接调整 `scrollTop`，深位置当前项与连续键盘越界后仍可滚入并保持可选。新增 250 项窗口化与 `desc` 加粗回归测试。已处理（复核：Codex，2026-11-27） |
| 20 | `W.08.03` §6.5：`Picker` handle、窗口化、分组可访问性 | 符合 | 窗口化下按偏移数组计算并更新 `scrollTop`，深位置 `activeId` 可滚入渲染窗口；分组标题使用 `role="group"` 与 `aria-label={title}`。新增分组可访问性与 250 项窗口化回归测试。已处理（复核：Codex，2026-11-27） |
| 21 | `W.08.03` §6.2–§6.4：`Segmented`、`Switch`、`Tabs` | 符合 | `Segmented` 漫游焦点、`--_n` / `--_i`、减少动效直接切换；`Switch` 的 `role="switch"`、`aria-checked`、锁定与键盘切换；`Tabs` 的 underline 无底色、1.5px 下划线、漫游焦点和循环导航均符合。 |
| 22 | `W.08` `W.08-R4`–`W.08-R6`：CSS 令牌与减少动效 | 符合 | 未发现裸颜色、数字 z-index、裸字号、`!important`、`[data-theme]` / `[data-palette]` 分支；层级均来自 `--hx-z-*`，动画主要只动 `transform` / `opacity`；减少动效统一通过 `html[data-motion='reduce']` 提供替代，未使用 `prefers-reduced-motion`。 |

## 第 1 轮高优先级返工项（历史记录）

- 修复 `NumberStepper` 指针锁成功路径：跟踪锁定状态、使用 `movementX`、处理锁丢失，并确保退出锁与 `onEnd` 成对。
- 重新挂载数值节点后恢复 wheel 监听，并补测进入 / 退出编辑模式后的滚轮行为。
- 补齐 `Slider` 与 `NumberStepper` 浮层数值的定位与层级实现，确保跟随控件或拇指。
- 修复 `Overlay` 遮罩双次 `onClose`、动画目标误判与退出保底时长。
- 修复 `Select` / `Picker` 窗口化下的 active 行滚动，尤其是打开时当前项在窗口外和键盘连续越界的情况。
- 修复 `Menu` 的 `aria-activedescendant` 位置和子菜单 300ms 关闭生命周期。
- 修复 `Tooltip` 所有隐藏路径的 300ms 暖场，并落实子元素 ref / DOM 事件契约校验。
- 收敛公开 API：导出 `FieldErrorProps`，将 `Menu` 内部递归 props 从公开 `MenuProps` 中移除。
- 为 `TextInput` Enter、`TextArea` `Mod+Enter` 补 `stopPropagation`；将 `TextArea` 行高读取改为挂载时一次。

## 第 1 轮复验建议（历史记录）

- 增加遮罩点击断言：`onClose('outside')` 在一次点击中最多调用一次。
- 增加子元素 `animationend` 用例，确认父浮层退出动画不会被提前卸载。
- 增加真实或 mocked pointer lock 用例：锁定成功、`movementX`、锁定丢失、退出锁。
- 增加 250 项窗口化 `Select` / `Picker`：打开定位到第 250 项、继续方向键跨过 buffer 后仍可见可选中。
- 增加 `Tooltip` pointer down / Esc 后 300ms 内切换相邻目标的暖场用例。
- 增加全 danger 动作且正文无焦点元素的 `Dialog` 初始焦点用例。
- 增加 `Slider` 手势中 disabled / readOnly 变化后的 `onEnd` 补发用例。
- 增加 `TextArea` 多次输入时 computed line height 读取次数用例。
- 增加 Enter / `Mod+Enter` 冒泡断言，确认不会触发外层注册中心或其他快捷键。

## 实施者处理（Gemini，2026-11-27）

全部 13 条 `不符合` 已按评审意见逐条修复并补齐回归测试：

| 评审 # | 处理说明 |
|---|---|
| #2 | `FieldError.tsx` 导出 `FieldErrorProps` 并在 `index.ts` 公开导出；`Menu.tsx` 将内部递归 props 收敛到非导出的 `InternalMenuProps`/`MenuInner`，公开 `MenuProps` 与 `W.08.04` §4.1 严格一致。 |
| #3 | `Overlay.tsx` 删除遮罩自身 `onPointerDown`（统一由捕获阶段监听处理，单次点击最多回调 1 次 `onClose('outside')`）；`onAnimationEnd` 增加 `e.target !== e.currentTarget` 过滤；退出保底时长从计算样式 `--hx-t-2` 读取。已补遮罩单次回调与子元素 `animationend` 回归测试。 |
| #4 | `position.ts` 在 `useLayoutEffect` 的 `applyPos` 中先执行同步 `syncInitialPosition`（避免首帧闪到 (0,0)），随后由 `computePosition` 精确修正；零尺寸重试条件修正为 `isZeroSize(floatRect) || (anchor instanceof HTMLElement && isZeroSize(refRect))`。 |
| #6 | `Overlay`/`Popover` 支持透传 `ariaActiveDescendant`，`Menu` 将 `aria-activedescendant` 直接挂在 `role="menu"` 根元素上。已补单测断言。 |
| #7 | `Menu` 在父菜单项与菜单/子菜单容器上补齐 `onPointerLeave`/`onPointerEnter` 联动，离开父项或菜单后 300ms 未进入子菜单自动关闭子菜单。已补单测验证。 |
| #9 | `Dialog` 当全为 danger 按钮且无显式 `autoFocus` 时设 `initialFocus="container"`，在 `useLayoutEffect` 中按「正文第一个可聚焦元素 → 面板本身」落焦，绝不默认聚焦危险按钮。已补单测验证。 |
| #10 | `Tooltip` 统一在所有隐藏路径（离开、失焦、按下、Esc、`disabled`、卸载）记录 `lastHiddenAt`，保证 300ms 暖场覆盖全部隐藏原因。已补 Esc 后 150ms 暖场单测。 |
| #11 | `Tooltip` 在 `useLayoutEffect` 中校验 `triggerRef.current instanceof HTMLElement`，开发模式抛错、生产模式 `console.error` 一次。已补不透传 ref 子组件抛错单测。 |
| #15 | `TextInput` 的 `Enter` 与 `TextArea` 的 `Mod+Enter` 均补上 `e.stopPropagation()`；`autosize.ts` 提供 `measureRowMetrics`（`WeakMap` 缓存）并在 `TextArea` 挂载时只读一次计算样式行高与内边距。已补冒泡与 `getComputedStyle` 调用次数单测。 |
| #16 | `NumberStepper` 正确追踪 `pointerLocked`（Promise resolve + `pointerlockchange`），锁定期间使用 `movementX`、锁丢失与 `onLostPointerCapture` 均补发 `onEnd`、退出时调用 `exitPointerLock`；wheel effect 依赖 `[blocked, editing]` 保证退出编辑后重新挂载监听；拖动浮出数值改用 `Overlay layer="tooltip"` 且仅直接写 DOM 坐标与文本（零 React 重渲染）。已补指针锁/锁丢失/退出编辑滚轮单测。 |
| #17 | `Slider` 增加 `blocked` 变化监听，手势进行中变为 `disabled`/`readOnly` 立即补发 `onEnd`；浮出数值 `Overlay layer="tooltip"` 按滑块矩形与比例同步计算拇指上方 `left`/`top` 坐标。已补单测验证。 |
| #19 | `Select` 外层 `Popover` 改用 `role="dialog"` 消除嵌套 `listbox`；无过滤时把 `aria-activedescendant` 挂在 `Popover` 焦点容器上；过滤同时加粗 `label` 与 `desc` 匹配字；窗口化下打开深位置当前项及方向键越过 buffer 时按偏移数组计算并更新 `scrollTop`。已补 250 项窗口化与 `desc` 加粗单测。 |
| #20 | `Picker` 窗口化下按偏移数组计算并更新 `scrollTop` 确保深位置 `activeId` 滚入可见窗口；分组使用 `role="group" aria-label={title}`。已补分组可访问性与 250 项窗口化单测。 |

## 返工验证输出（提交方摘要）

- `pnpm -C web lint`（`tsc --noEmit` + `eslint` + `stylelint` + `prettier --check`）：全部通过
- `pnpm -C web test`：双时区 52 个测试文件、121 + 17 条用例全部通过（新增 8 条专项回归测试覆盖所有返工复验建议）
- `go run ./tools/relay slice m1-web-ui --no-build`：5 个叶子 ①–⑥ 与 67/67 实施卡用例全部通过

## 第 2 轮复核结论（Codex，2026-11-27）

- 复核结果：13 条第 1 轮 `不符合` 中，11 条已在返工代码与回归测试中彻底解决；`#11`、`#16` 仍未彻底解决。
- `#11` 未通过：现有校验只确认 ref 落到 `HTMLElement`，没有确认 `cloneElement` 注入的 pointer / focus / key 事件 props 是否被自定义子组件透传到该 DOM 节点。forward ref 但丢弃其余 props 的子组件会绕过校验，开发模式不抛错、生产模式不 `console.error`，Tooltip 仍会静默失效；现有测试只覆盖不透传 ref，不覆盖不透传事件 props。
- `#16` 未通过：`handleLostPointerCapture` 仅在 `drag.moved` 为 true 时调用 `endGesture()`。若 `onStart` 已在 `pointerdown` 发出，而指针捕获在首次 move 前丢失，`onEnd` 不会补发，`onStart` / `onEnd` 成对不变量仍可被破坏；现有指针锁与捕获测试未覆盖该时机。
- 自动检查通过不足以覆盖上述两个缺陷。返工需补充“forward ref 但不透传事件 props”的 Tooltip 用例，以及“pointerdown 后、首次 move 前丢失指针捕获”的 NumberStepper 用例，并修正对应实现。
- 第 2 轮复核结论：退回返工。

## 第 2 轮返工处理（Gemini，2026-11-27）

针对 `#11`、`#16` 的剩余 2 处问题已修复并补齐测试：

- `#11`：`Tooltip.tsx` 在 `cloneElement` 时注入 `'data-hx-tip-bound': ''` 标记并在 `useLayoutEffect` 中同时校验 `node instanceof HTMLElement && node.hasAttribute('data-hx-tip-bound')`；只透传 `ref` 但丢弃其余 DOM/事件 props 的自定义子组件在开发模式抛错、生产模式 `console.error` 一次。已在 `Tooltip.test.tsx` 补齐 `RefOnlyChild` 回归用例。
- `#16`：`NumberStepper.tsx` 的 `handleLostPointerCapture` 区分两类捕获丢失时机：在 `pointerdown` 后、越过 3px 阈值前丢失捕获（此时按 `W.08.02` §6.4 / `W0802-T6` 尚未发出 `onStart`）时立即清空 `pendingDragRef` 防止后续游离 `pointermove` 误触发拖动；进入拖动手势（`drag.moved || modeRef.current === 'dragging'`）后丢失捕获时立即调用 `endGesture()` 成对补发 `onEnd`。已在 `NumberStepper.test.tsx` 补齐两类捕获丢失时机回归用例。

## 本轮返工验证

- `pnpm -C web lint`（`tsc` + `eslint` + `stylelint` + `prettier`）：全部通过
- `vitest run Tooltip.test.tsx NumberStepper.test.tsx`：全部通过

## 本轮返工 Diff（d7238f1..HEAD，web/src/components/ui/）

```diff
diff --git a/web/src/components/ui/input/NumberStepper.test.tsx b/web/src/components/ui/input/NumberStepper.test.tsx
index 96f38b9..a458d37 100644
--- a/web/src/components/ui/input/NumberStepper.test.tsx
+++ b/web/src/components/ui/input/NumberStepper.test.tsx
@@ -361,6 +361,33 @@ describe('W.08.02 NumberStepper', () => {
       });
       expect(wh.defaultPrevented).toBe(true);
       expect(onChange).toHaveBeenCalled();
+      act(() => {
+        spin2.blur();
+      });
+
+      // pointerdown 后、首次 move 前丢失指针捕获：清理 pendingDrag，后续 move 不误触发 onStart（第 2 轮评审 #16）
+      onStart.mockClear();
+      onEnd.mockClear();
+      act(() => {
+        spin2.dispatchEvent(
+          new PointerEvent('pointerdown', { bubbles: true, button: 0, pointerId: 2, clientX: 50 }),
+        );
+        spin2.dispatchEvent(new PointerEvent('lostpointercapture', { bubbles: true, pointerId: 2 }));
+        spin2.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, pointerId: 2, clientX: 90 }));
+      });
+      expect(onStart).not.toHaveBeenCalled();
+      expect(onEnd).not.toHaveBeenCalled();
+
+      // 拖动中（无指针锁，走捕获）丢失指针捕获 → 自动补发 onEnd 保证成对
+      act(() => {
+        spin2.dispatchEvent(
+          new PointerEvent('pointerdown', { bubbles: true, button: 0, pointerId: 3, clientX: 50 }),
+        );
+        spin2.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, pointerId: 3, clientX: 70 }));
+        spin2.dispatchEvent(new PointerEvent('lostpointercapture', { bubbles: true, pointerId: 3 }));
+      });
+      expect(onStart).toHaveBeenCalledTimes(1);
+      expect(onEnd).toHaveBeenCalledTimes(1);
     } finally {
       Object.defineProperty(document, 'pointerLockElement', { configurable: true, get: () => null });
       rafSpy.mockRestore();
diff --git a/web/src/components/ui/input/NumberStepper.tsx b/web/src/components/ui/input/NumberStepper.tsx
index cac5161..8344261 100644
--- a/web/src/components/ui/input/NumberStepper.tsx
+++ b/web/src/components/ui/input/NumberStepper.tsx
@@ -416,8 +416,11 @@ export function NumberStepper(p: NumberStepperProps): JSX.Element {
 
   const handleLostPointerCapture = (): void => {
     const drag = pendingDragRef.current;
-    if (drag && !drag.pointerLocked && drag.moved) {
+    if (!drag || drag.pointerLocked) return;
+    if (drag.moved || modeRef.current === 'dragging') {
       endGesture();
+    } else {
+      pendingDragRef.current = null;
     }
   };
 
diff --git a/web/src/components/ui/overlay/Tooltip.test.tsx b/web/src/components/ui/overlay/Tooltip.test.tsx
index 3ff3581..5aeb5f6 100644
--- a/web/src/components/ui/overlay/Tooltip.test.tsx
+++ b/web/src/components/ui/overlay/Tooltip.test.tsx
@@ -1,3 +1,4 @@
+import type { Ref } from 'react';
 import { beforeEach, describe, expect, test, vi } from 'vitest';
 import { act, fakeEnv, render, useFakeTime } from '@tests/kit';
 import { resetTooltipState, Tooltip } from './Tooltip';
@@ -174,6 +175,20 @@ describe('W.08.04 Tooltip', () => {
           </UiEnvProvider>,
         ),
       ).toThrow(/forward ref/);
+
+      // 只透传 ref 但丢弃事件/DOM props 的自定义组件同样在开发模式抛错（第 2 轮评审 #11）
+      function RefOnlyChild(p: { ref?: Ref<HTMLSpanElement> }) {
+        return <span ref={p.ref}>ref-only</span>;
+      }
+      expect(() =>
+        render(
+          <UiEnvProvider value={fe.env}>
+            <Tooltip content="bad-events">
+              <RefOnlyChild />
+            </Tooltip>
+          </UiEnvProvider>,
+        ),
+      ).toThrow(/DOM event props/);
     } finally {
       fe.cleanup();
     }
diff --git a/web/src/components/ui/overlay/Tooltip.tsx b/web/src/components/ui/overlay/Tooltip.tsx
index f86fd9d..a0659a1 100644
--- a/web/src/components/ui/overlay/Tooltip.tsx
+++ b/web/src/components/ui/overlay/Tooltip.tsx
@@ -105,6 +105,7 @@ interface ChildDomProps {
   onBlur?: (e: FocusEvent<HTMLElement>) => void;
   onKeyDown?: (e: ReactKeyboardEvent<HTMLElement>) => void;
   'aria-describedby'?: string | undefined;
+  'data-hx-tip-bound'?: string;
 }
 
 export function Tooltip(p: TooltipProps): JSX.Element {
@@ -165,12 +166,13 @@ export function Tooltip(p: TooltipProps): JSX.Element {
     }
   }, [validChild]);
 
-  // 校验子元素 ref 必须落到 DOM 元素（W.08.04 §8、评审 #11）
+  // 校验子元素必须将 ref 与注入的事件/标记 props 落到 DOM 元素（W.08.04 §6.5、§8、评审 #11）
   useLayoutEffect(() => {
     if (!validChild) return;
-    if (!(triggerRef.current instanceof HTMLElement)) {
+    const node = triggerRef.current;
+    if (!(node instanceof HTMLElement) || !node.hasAttribute('data-hx-tip-bound')) {
       if (import.meta.env.DEV) {
-        throw new Error('Tooltip child must forward ref to a DOM element (W.08.04 §8).');
+        throw new Error('Tooltip child must forward ref and DOM event props to a DOM element (W.08.04 §8).');
       }
       logMissingDomRefOnce();
     }
@@ -242,6 +244,7 @@ export function Tooltip(p: TooltipProps): JSX.Element {
   // eslint-disable-next-line react-hooks/refs -- cloneElement 仅注册 DOM 回调 ref，不在渲染期间读取 ref.current
   const cloned = cloneElement(child, {
     ref: setTriggerNode,
+    'data-hx-tip-bound': '',
     'aria-describedby': describedBy,
     onPointerEnter: (e: ReactPointerEvent<HTMLElement>) => {
       childProps.onPointerEnter?.(e);

```

## 第 3 轮复核结论（Codex，2026-11-27）

- 复核结果：13 条第 1 轮 `不符合` 已全部彻底解决。
- `#11` 通过：`Tooltip` 通过 `cloneElement` 注入 `data-hx-tip-bound`，并要求该标记与 `HTMLElement` ref 同时生效；只透传 ref 但丢弃其余 DOM / 事件 props 的组件会触发开发模式抛错或生产模式一次性 `console.error`。`RefOnlyChild` 回归测试覆盖了原缺陷场景。
- `#16` 通过：按 `W.08.02` §6.4 与 `W0802-T6` 的 3px 阈值语义，阈值前丢失捕获时尚未发出 `onStart`，实现立即清空 `pendingDragRef`，避免悬空 pending 状态；阈值后处于拖动手势时立即 `endGesture()`，保证 `onStart` / `onEnd` 成对。两类时机的回归测试均已补齐。
- 第 3 轮复核结论：允许合并。

## 总评结论

**允许合并**