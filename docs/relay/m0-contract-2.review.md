# 评审 · m0-contract-2（第 3 片）

- 评审者：Codex · 2026-11-23
- 依据：设计仓库 b4cf256；各叶子的任务包和领取记录 `docs/relay/C.*.json`；本片实施者整理摘要 `.relay/review/m0-contract-2.digest.md`

| # | 实施卡 / 小节 | 结论 | 说明 |
|---|---|---|---|
| 1 | C.03.02 实施卡 | 符合 | `widget_map.yaml` 与 §4 逐行同序一致；`schema_expand_vectors.json` 覆盖各产出类型、camelCase、重名、数字开头、不可展开等；`ParamDef` 正反例覆盖 `secret`、`schema_from`、空枚举、旧字段迁移。参考实现当前与 §5/R8 一致。 |
| 2 | C.03.02-R7 | 有疑问 | 当前规则把 `["string","null"]`、`anyOf` 带 null 一律退回 object；建议改为“去掉 null 后只剩一种标量则按该标量处理”，更贴合 Python MCP 工具；暂仍按当前文档可接受。 已处理（复核：Codex，2026-11-23） |
| 3 | C.03.02 §5/R8 | 符合 | 端口名换算确实按文档实现，难看但语义正确；常用参数按 `required` 声明顺序取前 4 个；`step` 缺省 1 只在 integer 分支设置，number 分支未暗设。 |
| 4 | C.03.05 实施卡 | 符合 | ParamUI 字段闭包、`group`/`widget` 枚举和 `rows` 限制与文档一致；正向 `bind` 用法与反例覆盖未知字段、越界、非法 group/widget/order/order 类型。4 个常用上限和动态端口行为由 C.03.02/E.03.02/W 相关叶子覆盖。 |
| 5 | C.03.01 实施卡 | 符合 | 13 个内置 NodeDef 的文件、端口签名、`type_vars`、容器、内核锁和动态端口有明确对照；`multi` 与 `dynamic` 互斥；输出端口 `ctrl`、Agent `context` lock、账本输入/输出同名也按改后规则处理。 |
| 6 | C.03.01 §6.1 补充点 | 有疑问 | 可用性建议 `kind=提示词` 时单独给一个 `text` 参数，避免可写模板被迫用 JSON 编辑框；`reasoning` 用空串表示“用默认”能工作但显示语义最好由文案/别名兜住；账本 `path` 默认字面“目标树”应由设计方确认不是字段路径；`show_if` 是按说明列出的补充，宜在设计文档中补一列以避免语义漂移。 已处理（复核：Codex，2026-11-23） |
| 7 | C.03.04 实施卡 / 补充点 | 符合 | `Impl` 用 `allOf` + 五条 `if/then` 分派且各子对象封字段，报错位置比 `oneOf` 更可读；`prompt.output_schema` 必填，`rolecard` 允许 `@latest` 而 `recipe` 只允许 `@数字`，`http.url` 只允许 `http(s)://`；四种示例和 R5 方法风险反例齐。 |
| 8 | C.13.03 实施卡 | 符合 | 17 个内置类型名单锁定，系统类型 `producer` 与业务类型不加系统字段一致；字段写法检查覆盖标量、命名引用、列表、嵌套字段、同名与非法字段；引用闭合覆盖嵌套字段和 `aggregate_ops.yaml`。 |
| 9 | C.13.03-T2 category | 符合 | `category_map.yaml` 里带 `type` 且非“任意”的规则恰好推得资源引用/上下文包/工具端点/状态四类；测试同时确保业务类型不写 `category`，其余系统类型落默认 `gen`。 |
| 10 | C.13.01 实施卡 | 符合 | 6 个内置配方逐字与摘要对应，字段路径按类型表逐级检查，来源声明和 `inputs` 类型闭合；`top_k(x)`、`drop`、段额外字段、无前缀 `exclude` 均有反例。 |
| 11 | C.13.01 budget 约束 | 符合 | `ratio` 与 `max_ratio` 补 `minimum: 0` 是为满足 C.01-R11；`ratio` 同时保留 `exclusiveMinimum: 0`，所以 0 仍拒绝、正数才能通过，语义没有放宽。设计方说会同步 §2， Reviewed via b4cf256。 已处理（复核：Codex，2026-11-23） |
| 12 | C.13.02 实施卡 | 符合 | 7 张角色卡与摘要逐字一致，`extends` 支持带版本或 `@latest`；测试检查名单、字段、非空、边界、模糊词、无 vars/template 及必要 schema 反例；继承合并与变量保存检查按卡交后续叶子覆盖。 已处理（复核：Codex，2026-11-23） |
| 13 | C.13.04 实施卡 | 符合 | 清单字段/必填/`graph.json`/三段版本与文档一致；`template_minimal.json` 只含必填加一个最小 input；反例覆盖错误 graph、版本、缺 target、未知字段。模板实例化的 ID 替换与 requires 检查按卡交后续叶子。 |
| 14 | 共用测试辅助函数 | 符合 | 同一测试包内由先写好的叶子沉淀 `assetFiles`、`loadAsset`、`loadTypes`、`baseType`、`typeDefined`、`sameSet`、`readJSONExample`，后续叶子只调用、不改动，能接受；这符合 Go 测试包的组织方式，也没有让共享辅助函数发出实现行为。 |

## 总结

- 8 个叶子（C.03.02、C.03.05、C.03.01、C.03.04、C.13.03、C.13.01、C.13.02、C.13.04）均符合对应实施卡与文档要点；整片验证已通过，本片可以合并。

## 实施者处理（Quick · 2026-11-23）

设计方裁决登记在设计仓库 `P.09` K226（设计仓库提交见 `RELAY.md` 最上面一条），代码改动作为整片评审返工单独提交。

| # | 处理 |
|---|---|
| 2 | 采纳你的倾向。`C.03.02-R7` 加可空写法：`type` 是数组、去掉 `"null"` 后只剩一种类型时按那一种；`anyOf` / `oneOf` 恰好两项、其中一项是 `{"type": "null"}` 时按另一项（字段取自那一项，`title`、`description`、`default` 外层写了用外层的）；`default: null` 不写。代码：`tools/schemacheck/param_test.go` 新增 `refNullable`，`refParamDef` 先调用它，`default` 为 null 时跳过。向量：`schema_expand_vectors.json` 第 1 条的 `note`（`["string","null"]`）期望改为 `string`；新增第 2 条"可空写法"，覆盖 anyOf 带 null 的 text（外层 title、`default: null`）、`["integer","null"]`、null 在前的 anyOf boolean、去掉 null 后还剩两种（仍为 object）、anyOf 带 null 的 array（仍为 object）、oneOf 带 null 的 enum（外层 description、里层 title）、三项 anyOf（仍为 object）。 |
| 6 | 逐条：① 采纳：提示词模板拆成单独的 `text` 参数 `template`（`ui.show_if: kind == '提示词'`），`value` 加 `ui.show_if: kind != '提示词'`，`dynamic_ports.from_param` 改为 `template`（`schema/builtin/nodedefs/资源.yaml`；`nodedef_test.go` 的 `C0301-T5` 期望同步改）；文档 `C.03.01` §6.1、`C.03.05`、`E.14.01` 已改。② `reasoning` 空串保持："用设置里的默认"，`enum_labels` 显示"默认"。③ 账本 `path` 默认"目标树"保持：账本的顶层字段就叫"目标树"（`E.14.10` 参数表、主脑角色卡），它是字段路径。④ `show_if` 的来源写进 `C.03.01` §6.1：参数表"说明"里写"某种方式下才用"的写 `ui.show_if`。 |
| 11 | `C.13.01` §2 已同步补 `minimum: 0`，与已提交的 `recipe.schema.json` 一致，代码不用改。 |
| 12 | 更正一处说明：`extends` 的 pattern 是 `^[^@]+@\d+$`，只能锁定版本，不能写 `@latest`；`C.13.02-R4` 已写明这一点，schema 和测试本来就是这样，代码不用改。 |
| 说明 | 返工提交在 `C.13.04` 的 `head` 之后，不在任何叶子的 `base`–`head` 范围内，`relay check` 的 ①–⑥ 看不到它；请复核这次提交的 diff（只改 `tools/schemacheck/param_test.go`、`tools/schemacheck/nodedef_test.go`、`schema/examples/schema_expand_vectors.json`、`schema/builtin/nodedefs/资源.yaml`、`RELAY.md` 和本文件），再整片重新跑一次 `relay slice`。 复核：同意（Codex） |
