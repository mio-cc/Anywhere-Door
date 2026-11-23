# 评审 · m0-contract（第 2 片）

- 评审者：Codex · 2026-11-23
- 依据：设计仓库 aa95c2c；各叶子任务包与领取记录 `docs/relay/C.*.json`

| # | 实施卡 / 小节 | 结论 | 说明 |
|---|---|---|---|
| 1 | C.01 实施卡 `C01-T5` / §6 R22 | 不符合 | 清单路径只用 `path.Clean` 并检查前缀 `../`（`tools/schemacheck/check.go:378`），随后用平台路径读文件（`tools/schemacheck/check.go:434`）。在 Windows 上反斜杠路径（如 `..\outside.json`）不会被该检查拦住，`filepath.Join` / `os.ReadFile` 会归一化并尝试读取 `schema/examples/` 之外的 JSON，违反“file 是相对 `schema/examples/` 的 .json 路径”；`check_test.go:209` 的 T5 反例也没有覆盖反斜杠路径。 已处理（复核：Codex，2026-11-23） |
| 2 | C.07 实施卡 `C07-T1`、`C07-T2` / §6 | 符合 | `RunStatus`、`NodeState` 与 §2、§3 完全一致且顺序相同；`state_machine.yaml` 的 run 6×6、node 9×9 转换逐格符合 §2、§3.1，终态出边仅保留文档允许的 `failed → queued`。 |
| 3 | C.06.05 实施卡 `C0605-T1`、`C0605-T2` / §2–§5 | 符合 | `Iter`、`Loc` 与 §3 相同；9 条路径向量覆盖 §4 要求的 8 类情况。抽 3 条人工推演：第 1 条得路径 `根图/大模型`、键 `…A1\|a1`；第 4 条得 `根图/闭环迭代#r1/执行池#2/执行者@2`、键含 `r1\|f2\|a2`；第 8 条全角替换为 `／＃＠～`，都与 `want` 一致。`refPathKey` 的容器后缀、自身 `fN.aN` 键、重名短 ID 和全角替换符合 R1、R2、R5。 |
| 4 | C.11 实施卡 `C11-T1`–`C11-T4` / §5、§6、G.13 | 符合 | API 错误码、失败类别、诊断码与级别逐项一致；`errors.zh.yaml` 恰好覆盖 19 个 API 错误码，`diags.zh.yaml` 恰好覆盖 18 个诊断码且级别一致。抽查文案均为中文，能说明发生了什么和用户可以怎么做，符合 G.13-R2、R3。 |
| 5 | C.04.01 实施卡 `C0401-T1`、`C0401-T2` / §2、§3 | 符合 | `TypeRef` 外形、合法与非法例子与 §2 一致；`Category` 8 值与顺序一致；`category_map.yaml` 覆盖任意、列表、上下文包、资源引用、工具端点、状态和默认 gen，最后一条确实是 default，且没有规则产出 ctrl。 |
| 6 | C.04.02 实施卡 `C0402-T1`–`C0402-T3` / §2–§5 | 符合 | 46 条向量覆盖 C0–C10 各至少 3 条。随机抽 10 条人工推演（第 7、15、20、24、29、34、37、40、44、46 条）均与规则表和 `want` 一致：C1 先于 C2、C3 同主版本、C4 迁移/拒绝、C5 继承、C7 展开、C9 元素递归、容器目标不展开并落 C10。`refAssign` 与规则表一致；`assign.zh.yaml` 恰好 6 条提示模板。 |
| 7 | C.04.03 实施卡 `C0403-T1` / §2 | 符合 | `aggregate_ops.yaml` 的 11 个键与 §2 表完全一致，输出类型全部通过 `TypeRef`；类型变量只使用 `$T`。 |
| 8 | C.02.01 实施卡 `C0201-T1`–`C0203` / §4、§7 | 符合 | 顶层结构与 §4 一致；`graph_loop.json` 为根图、闭环迭代、资料采集三层；节点 ID 全树唯一，连线两端同层。RELAY 记录的 4 处 demo 差异均有契约依据：扇出补子图（C.02.04 §5）、`任务卡@1[]` 改为 `列表[任务卡@1]`（C.04.01 §2）、上下文 `budget` 因单位未定不写、根图摘要端口取消并改为 `循环.final → 人工.request`；抽核端口名符合 C.03.01 §6。 |
| 9 | C.02.02 实施卡 `C0202-T1`–`C0203` / §4 | 符合 | `Node`、`NodeTypeRef`、`Origin` 与 §4 一致；13 个原子类型及扩展类型名符合契约；`children` / `ref` 互斥；`params`、`ui` 开放而节点封闭，反例报错位置正确。 |
| 10 | C.02.03 实施卡 `C0203-T1`、`C0203-T2` / §4、§6 | 符合 | `PortRef` 只定义在 `common.schema.json`；端口名模式拒绝大写、负索引、四位数索引并允许 0–999；跨子图示例登记为 `valid: true`、`diag: V05`，符合 C.01-R23 与 §6。 |
| 11 | C.02.04 实施卡 `C0204-T1`–`C0204-T3` / §4–§9 | 符合 | `Subgraph`、`SubgraphRef` 与 §4 一致；容器规则表三条与 §5 相同；缺子图、缺出口、9 层、5 层四个语义示例的 `diag` 分别为 V11、V11、V12、V12，深度计算和 §6 一致。 |
| 12 | C.02.05 实施卡 `C0205-T1`、`C0205-T2` / §3 | 符合 | `Pos` 强制整数坐标；`NodeUi` 为开放对象且已有字段仍受约束；`Note.size` 封闭并要求 `w`、`h`。合法与非法示例及报错位置符合实施卡。 |

## 总结

- 12 个叶子均覆盖评审；11 个叶子符合，1 个叶子（C.01）不符合。
- 不合并原因：`C01-T5` 的清单路径检查在 Windows 上不能可靠限制在 `schema/examples/` 内，存在检查语义缺口；需要实施者修复并补反例测试。
- 给设计方的问题：（没有）。

## 实施者处理（Quick · 2026-11-23）

| # | 处理 |
|---|---|
| 1 | 修了。`tools/schemacheck/check.go` 新增 `relExample`：清单的 `file` 必须以 `.json` 结尾、不含反斜杠和冒号（盘符）、不以 `/` 开头、`path.Clean` 后不变、不是 `..`、不以 `../` 开头；`CheckExamples` 和 `checkEntry` 都先过这一关（测试辅助 `manifestEntry` 直接调 `checkEntry`，也拦得住）。测试：`check_test.go` 的 `C01-T5 示例清单` 加 5 项反例（`..\outside.json`、`../outside.json`、`/abs.json`、`C:/x.json`、`invalid\passes.json`），并在 `schema/` 下放一个真实的 `outside.json`，断言它不会被读到、清单报 5 条"file 要写相对 schema/examples/ 的 .json 路径"，总数 10 → 15；另加 `TestRelExample`（`C01-T5/清单路径`）逐个测 4 个合法、12 个非法写法 |
| 说明 | 修复提交在 `C.02.05` 完成之后，不在任何叶子的 `base`–`head` 范围内，`relay check` 看不到它，属于整片评审的返工；请评审者复核这次提交的 diff（只改了 `tools/schemacheck/check.go`、`check_test.go` 和本文件），复核通过后整片重新跑一次 `relay slice m0-contract` 再合并 复核：同意（Codex） |
