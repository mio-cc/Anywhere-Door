# 评审 · C.14（第 5 片）

- 评审者：Codex · 2026-11-24
- 依据：设计仓库 `72bc633`；C.14 任务包和领取记录 `docs/relay/C.14.json`

| # | 实施卡 / 小节 | 结论 | 说明 |
|---|---|---|---|
| 1 | C.14 §2 输出清单、`main.go` 子命令 | 符合 | `tools/gen/generate.go`、`main.go` 对应文档 §2 的清单；`tokens`、`icons`、`fonts` 子命令均按“缺省”返回 `1`，未越权扩展生成器职责。 |
| 2 | C.14 §3 类型映射、`model.go` / `golang.go` / `typescript.go` | 符合 | `anon`、`fill`、`fieldsOf`、`refType`、`nameForLoc` 与 Go/TS 字段映射一致；`Timestamp` 走自定义类型，Go 与 TS 都保留精度并按文档输出; `gen/types.go` 与 `web/src/lib/protocol/generated/types.ts` 中的 `Op`、`Session`、`MessageBlock` 已抽查，字段与文档一致。 |
| 3 | C.14 §4 R1–R3 判别联合、`unionSource` | 符合 | `Op`、三个事件流按判别字段生成；`MessageBlock`、`MessageRef`、`Impl` 正确识别为整体联合。校验型 `if/then` 的 `Expiry`、`DataNodeState` 未被误当成联合，`cause` 仍按 `to` 约束。 |
| 4 | C.14 §4 R5–R7、§5.2 命名与常量 | 符合 | `node_ids`、`tool.L0`、事件常量按流取类型、`NodeTypeResource`、`SseResync` 等命名均与文档相符；`names.go` / `consts.go` 含重名检查。 |
| 5 | C.14 §4 R8 字段映射、`wire.go` / `wire.ts` | 符合 | `WIRE_TYPES` 按类型生成，`Mandate.overrides`、`NodeDef.params`、`Node.params`、`Message.usage` 等字段映射与文档示例一致；判别联合与整体联合的区分正确。 |
| 6 | C.14 §4 R9 字段名检查、`checkFieldName` | 符合 | `checkFieldName` 拒绝 `x_2`、尾部下划线和连续下划线等非法字段名，字段名往返无损。 |
| 7 | C.14 §5.3 数据表与文案、`tables.go` / `messages.go` | 符合 | YAML 读取严格限定字段，动作、Op 种类、actor、状态与 schema 枚举一致; `ERRORS`、`ASSIGN` 键集合检查存在。 |
| 8 | C.14 测试 C14-T1–T7 | 符合 | 测试期望按文档手写，不是从生成物反抄；T5 覆盖状态机表，T6 覆盖字段映射，T7 覆盖数据表与文案。 |
| 9 | C.14 RELAY.md 自主决定 | 符合 | 字段排序、`DecodeAs[T]`、整体联合 `{Kind, Raw}`、默认值仅处理标量层面等自主决定与 §3、§4、§5 的边界一致，未引入未约定的行为。 |

## 实施者处理
