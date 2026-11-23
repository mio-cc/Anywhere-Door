# 评审 · C.02.01（冻结后修复）

- 评审者：Codex · 2026-11-23
- 依据：设计仓库 858c124；任务包与领取记录 `docs/relay/C.02.01.json`；K228 修复说明与实施记录 `RELAY.md`

| # | 实施卡 / 小节 | 结论 | 说明 |
|---|---|---|---|
| 1 | C.02.01 / K228 示例图资源节点写法 | 符合 | `schema/examples/graph_loop.json` 的 diff 只改了 5 个资源节点的 `value`：2 处单模型改为 `{provider, model}`，1 处列表改为 3 个模型引用组成的 JSON 数组，2 处小模型改为 `{provider, model}`；节点、连线、其他参数和结构内容没有别的改动。 |
| 2 | C.02.01 / E.14.01、E.10.01 测试 | 符合 | `resourceValueProblem` 按资源 `kind` 区分：模型要求 `{provider, model}` 且只允许 `provider`、`model`、`sampling`、`fallback`；列表要求 JSON 数组或 `ws://` 工作区引用；提示词要求写 `template` 且不写 `value`，符合 E.14.01、E.10.01。抽查旧写法反例：临时把一个模型 `value` 改回 `"large-model"` 后，`TestGraphExamples` 的 T3 报 E.14.01 错误并由 0 个失败变为 1 个用例失败；恢复后反向核实，工作区没有临时改动残留。 |

## 总结

- 1 个叶子符合；0 个叶子不符合；0 个叶子有疑问。
- 不合并原因：（没有）。
- 给设计方的问题：（没有）。