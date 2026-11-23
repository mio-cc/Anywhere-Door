# 评审 · m0-contract-3（第 4 片）

- 评审者：Codex · 2026-11-23
- 依据：设计仓库 5d21b55；各叶子的任务包和领取记录 `docs/relay/C.*.json`；本片实施者整理摘要 `.relay/review/m0-contract-3.digest.md`
| # | 实施卡 / 小节 | 结论 | 说明 |
|---|---|---|---|
| 1 | C.05.01 实施卡 / §2、§4 | 符合 | `OpKind` 30 个与 C.05.02 §2–§6 表逐一对应；`coalesce_key`、`Branch`、`TmpId` 正则与 C.05.01 §2 逐字一致；`Op`、`OpTarget`、`OpsRequest`、`OpsApplied`、`OpsProposed` 的必填、封闭和引用与 §2、§4 一致；最小 Op 改用 `remove_nodes` 后仍覆盖原信封断言。 |
| 2 | C.05.02 实施卡 / §2–§6.1 | 符合 | 抽查 `set_param`、`group`、`update_note`、`set_mandate`、`create_branch`、`merge_branch` 的字段、必填和类型均与 §2–§6 与 §6.1 一致；`Removed`、`Rewire`、`Resolution` 同 §6.1。五项自主决定均合理：`set_tags.to` 与 `Node.tags` 同型、更新便签和元信息复用原属性、草稿分支收紧 `draft/`、服务器填写的 `rewire`/`ops` 保留为可选以校验日志、图态前置条件明确留给 E.05.01。 |
| 3 | C.09 实施卡 / §3–§5.1、R4 | 符合 | `Action` 43 个与 §4.2 表逐个对应；`authz_matrix.yaml` 与 §5.1 代码块逐字一致；测试的 `authzTable` 按 §5 表另行抄写，能真正比对 YAML。`Expiry` 的 `until_at`/`run_id` 三分支必填与互斥、`created_by.kind=user`、`overrides.scope` 的 root/ULID 均符合 R4；用 `allOf + if/then + not` 表达互斥是合理写法。 |
| 4 | C.08 实施卡 / §2、§2.1、§4、§4.3 | 符合 | `GraphPatch`、`PatchOp`、`Proposal`、`ProposalPreview`、`GhostMark` 的必填、封闭和引用与原文一致；示例第二条 `connect` 的 `edge.from.node` 确是第一条新增节点的 `tmp:1`；`proposal.json` 的 decisions 键属于 `patch.ops[].id` 集合；`Proposal.branch` 照 §4 保持 `type: string`，未额外收紧。 |
| 5 | C.10 实施卡 / §2–§5.1、R8 | 不符合 | 7 个 Block 和 4 个 Ref 的字段、必填和类型均与 §4、R8、§5.1 一致；`RefFile` 的 `oneOf(ref, path)` 与 `RefFile.path` 长度限制合理，`RefSkill.package_id` 因字段名以 `_id` 结尾按 C.01-R6 写 ULID 也成立。但 C.10 §2 规定会话 `mandates` 的“anchor 只能是 session”，而 `schema/session.schema.json:42-49` 仅引用 `MandateMount` 并把限制写在 description，未在数组项上收紧 `anchor`，因此 `graph`/`node:<ULID>` 仍可通过；`tools/schemacheck/session_test.go:69-108` 也只验证示例为 session，没有反例。 已处理（复核：Codex，2026-11-23） |
| 6 | C.06.01 实施卡 / §2、R6–R8 | 符合 | `Event` 的必填、封闭、字段类型与 §2 一致；`seq` 在基础信封为 ≥0，P/S≥1 与 E=0 按 R8 由各流 kind 分支实现，`run_id`/`graph_id`/`session_id` 与节点级 `loc` 交给各流 required/分支也符合 R6；测试覆盖 P、E、`loc`、`cause_seq` 和缺 `actor`、负 `seq`、非对象 `data`、多出字段、非 UTC 时间。 |
| 7 | C.06.02 实施卡 / §2–§8、R7 | 符合 | `runEvents` 39 个的名字与 P/S/E 级别逐项对应 §2–§8；7 个节点级分支均要求 `loc`。抽查 `node.state`、`llm.call`、`tool.call`、`approval.requested`、`approval.resolved`、`budget.warn` 的必填、可选、枚举和引用与表 + R7 一致；`queued`/`cancelled`/`skipped` 的 `cause` 枚举逐项正确；R7 未细分字段写 `{}`、`budget.warn.ratio` 写 0–1 数并保留三个枚举均符合。 |
| 8 | C.06.03 实施卡 / §2、R1、R3、R4 | 符合 | 10 个事件与 §2 逐一对应；只有 `op.applied`、`op.coalesced` 的 `seq` 为图版本号且 ≥1，其余 `const: 0` 符合 R1；`presence.scope` 使用 root/ULID 符合 R3；`op.coalesced.to` 为开放对象并说明为完整新 payload，`op.applied.op` 引用 `op.schema.json#/$defs/Op`，符合 §2、R4。 |
| 9 | C.06.04 实施卡 / §2 | 符合 | 9 个事件的名单、P/E 级别和 `seq` 约束与 §2 一致；`session.updated.updated_at` 必填，其余三个可选字段正确；`message`、`block`、`error` 分别引用 `session.schema.json` 的对应定义；`chat.status` 枚举与 §2 一致。整片复核：三条事件流均为根 `allOf` + 本流 required；`Op`、`MessageBlock`、`MessageRef` 的 `if/then` 判别写法一致；测试姓名单/表格均为按设计文档表手抄后再比对 schema。 |

## 实施者处理（Quick · 2026-11-23）

| # | 处理 |
|---|---|
| 5 | 采纳。实施者的错：§2 写了"anchor 只能是 session"，我只写进 description，没有约束。`schema/session.schema.json` 的 `Session.mandates.items` 改为 `{"$ref": "common.schema.json#/$defs/MandateMount", "properties": {"anchor": {"const": "session"}}}`；`tools/schemacheck/session_test.go` 的 `C10-T3` 加两条反例（`anchor` 为 `graph`、为 `node:<ULID>`，报错位置 `/mandates/0/anchor`）。`MandateMount` 本身不改（图文档也用它）。 |
| 说明 | 返工提交在 `C.06.04` 的 `head` 之后，不在任何叶子的 `base`–`head` 范围内，`relay check` 的 ①–⑥ 看不到它；请复核这次提交的 diff（只改 `schema/session.schema.json`、`tools/schemacheck/session_test.go`、`RELAY.md` 和本文件），再整片重新跑一次 `relay slice`。 复核：同意（Codex） |
