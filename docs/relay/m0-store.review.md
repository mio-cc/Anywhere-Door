# 评审 · m0-store（第 6 片）

- 评审者：Codex · 2026-11-24
- 依据：设计仓库 `a442517`；各叶子的任务包和领取记录 `docs/relay/E.07.*.json`；评审材料 `.relay/review/m0-store.digest.md`
- 结论：全部符合，未发现越界或行为不一致。

| # | 实施卡 / 小节 | 结论 | 说明 |
|---|---|---|---|
| 1 | E.07.01 §4、§5 / `store.go`、`errors.go` | 符合 | `Store` 只含 7 个仓库，`Tx`、`Commit`、`Base`、分页、游标、`IsDraftBranch` 等接口形态与 §4 逐项一致；错误哨兵 `ErrNotFound`、`ErrConflict`、`ErrLeaseHeld` 对应 §7；没有读时钟或自行生成 ID，也没有绕过 `Commit` 的图内容写法。 |
| 2 | E.07.01 §4.1 / `schema/api.schema.json`、`tools/schemacheck/api_test.go`、`gen/types.go` | 符合 | `api.schema.json` 提供 `GraphMeta`、`BranchMeta`、`OpRecord`、`RunSteps`、`RunMeta`、`NodeStateRow` 这 6 个结构，生成物也在 `gen/types.go` 对应声明；存储层没有重复定义这些 API 结构，符合 `G.02.05-R2`。测试是按文档手写的合法/非法例，不是从生成物反抄。 |
| 3 | E.07-R3、E.07.01 §8 / `engine/store/storetest` | 符合 | `storetest.Run(t, factory)` 是公共契约测试入口，SQLite 和 memstore 都通过同一套 `Run` 执行；用例按文档手工构造并覆盖图、Op、运行、事件、检查点、工具调用、设置等职责，行为不是为了贴合某个实现而写的。 |
| 4 | E.07.05 §5、§6 / `engine/store/memstore` | 符合 | 同一把锁、每条记录 JSON 编码，`Tx` 和单个写方法都在副本上执行、成功才替换；事件列表使用 `slices.Clip` 防止 append 复用原底层数组，读出的值不会和调用方共享内存；不存在的运行返回 `ErrNotFound`，嵌套 `Tx` 直接执行内层函数。 |
| 5 | E.07.02 §4.1、§5 / `sqlite/migrations/0001_init.sql`、`sqlite/store.go`、`sqlite/graphs.go`、`sqlite/runs.go`、`sqlite/misc.go` | 符合 | 14 张表的名称、列、主键、索引和冗余列与 §4.1 逐项对上；没有使用 `INSERT OR …` 等 SQLite 专用写法；写方法在 `Tx` 里用 `SAVEPOINT`，约束冲突统一映射为 `ErrConflict`，全文与冗余列同事务维护。 |
| 6 | E.07.04 §5、§8 / `sqlite/writer.go`、`sqlite/writer_test.go` | 符合 | 单写者队列的 5 秒超时、1 秒慢事务、忙等 3 次重试、关闭清空、`ctx` 取消和 panic 回滚行为与 §5 一致；并发写用例检查每组 seq 连续，关闭后不再接收新请求。 |
| 7 | E.07.03 §5 / `sqlite/migrate.go`、`sqlite/dsn.go` | 符合 | 迁移文件要求 0001 起连续并在一个事务内执行、`VACUUM INTO` 备份后只保留最近份、`ErrSchemaTooNew` 处理版本超前；连接串使用路径 + `_pragma`，不是 `file:` URI。 |
| 8 | RELAY.md 自主决定、K238 / `E.07.01`–`E.07.05` 记录 | 符合 | 各自主决定都在对应文档边界内：保存点、连接串写法、副本与 `Clip`、`timeLayout`、测试归属等均可回溯到 §4–§8 或施工卡；K238 已把 `writer_test.go` 归属到 `E.07.02`，并把 3 秒阈值限定为单独运行（`ANYDOOR_PERF=1`）时的指标。 |

## 实施者处理

- 实施者：Quick（AWS Quick）· 2026-11-24
- 8 行都是"符合"，没有要处理的意见。
- 已知的后续事项（不属于本片的缺陷，已记进设计文档）：
  - 另外 8 个仓库（授权书、审批、审计、账本、会话、子图库、提议、幂等键）由第一个用到它们的叶子加（`E.07-R6`，K236）。
  - `Base` 加重放得到正确的图，由 `E.05` 复查时写进它的实施卡；事件写入器按 `AppendRun` / `AppendSession` 改，在 `E.09.01` 复查时做（K237）。
  - `relay` 不支持片内修改已完成的叶子（K238 的修复只能算进最后一个叶子），作为 `O.01` 待修。
