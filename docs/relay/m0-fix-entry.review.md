# 评审 · m0-fix-entry（P.09 K233 修复片）

- 评审者：Codex · 2026-11-23
- 依据：设计仓库 3212ee2；各叶子的任务包和领取记录 `docs/relay/C.*.json`

| # | 实施卡 / 小节 | 结论 | 说明 |
|---|---|---|---|
| 1 | C.03.05 实施卡 / R2、R5 | 符合 | `rolecard_ref` 的正例确实允许 `ui.bind: role`，对应 R2 对 `资源引用@1` 端口的例外；`bind` 的端口存在和类型兼容按文档由 NodeDef 与 `E.03.02` 检查，本叶测试只验证参数界面声明可写，未越界实现运行时规则。Agent `rolecard` 的实际 `bind: role` 在 C.03.01 核对。 |
| 2 | C.03.01 实施卡 / §6、§6.1、R1、R3 | 符合 | Agent 输入端口新增 `role`（`资源引用@1`，标签“角色卡”），与 §6 Agent 行一致；参数 `rolecard` 的 `ui.bind: role` 符合 §6.1 和 E.14.02 §2，端口名与参数名不重名，测试覆盖 R3。资源节点 `kind` 增“角色卡”，`rolecard` 仅在 `kind == '角色卡'` 时显示；`value` 的 `show_if` 与 §6.1 原文一致；常用参数为 kind、value、template、rolecard 正好 4 个，未超过 R1 上限。 |
| 3 | C.13.03 实施卡 / §5.1、C1303-T4 | 符合 | `资源引用.yaml` 的 `kind` 枚举为模型、提示词、角色卡、列表、常量，并在 description 中补“角色卡”，与 §5.1 资源引用定义一致；`ref` 注释也区分角色卡“名称@版本”。C1303-T4 读取资源节点参数 `kind` 后用 `slices.Equal` 比较 `资源引用.kind` 与资源节点 `kind`，能同时防止名单漏项、多出和顺序漂移。 |
| 4 | C.10 实施卡 / §2、R9、C10-T1、C10-T3 | 符合 | `entry_graph_id` 为可选 ULID，说明为“这个会话用的对话入口图；不填用设置 chat.entry_graph”；`orchestrator_model` 为可选字符串，说明为对话框上的临时覆盖，有值时替换入口图里对话 Agent 的模型上游，不填用入口图模型，与 R9 逐句一致。示例带 `entry_graph_id`，C10-T1 断言其存在；C10-T3 增加 `entry_graph_id` 不是 ULID 的反例。跨图关系和入口图内连线不属于 schema 校验职责。 |

## 实施者处理