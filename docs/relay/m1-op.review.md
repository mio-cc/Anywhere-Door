# 评审 · m1-op（第 ③ 片）

- 评审者：Codex · 2026-11-26
- 依据：设计仓库 `8feed98`（`E.05`、`E.05.01`–`.05`、`P.09` K261–K264）；评审材料 `.relay/review/m1-op.digest.md` 的 15 条重点；代码 `engine/op/`、`engine/authz/check.go`、`engine/opstore.go`（片内提交 `490e6b0`、`36ca1cd`、`7a4b72b`、`a21b69f`、`e945abf`）
- 结论：15 条全部符合，没有“不符合 / 有疑问”，可以合并。

| # | 实施卡 / 小节 | 结论 | 说明 |
|---|---|---|---|
| 1 | `E.05.03` §5.2、§5.3 / 写集合表与 `Commutes` 例外 | 符合 | `commute.go:89-117` `clash()` 按 W∩W、W∩R、R∩W 取键；两个例外只放行对应的键：两条都只改视图时只跳过 `view:` 前缀（`:100-101`），两条 `remove_nodes` 删同一节点时只跳过 `node:`/`edge:` 前缀（`:101`），`graph:doc` 与任何非空集合都算相交（`:90-92`）。各 kind 的写集合在 `commute.go:158-321` 逐条落表（`add_node` 连子图、`remove_nodes` 连带删除子孙与相连连线、`connect` 独占端口替换、`set_param` 动态端口消失的连线、promote/demote、label/tags/lock 字段键、便签、`graph:title`/`graph:meta`、`revert_to` 递归、未知 kind 落 `graph:doc`）。 |
| 2 | `E.05.03` §5.4 / `Rebase` | 符合 | `rebase.go:17-51`：只从 missed 的 `remove_nodes`（及其 inverse 里的 `restore_nodes`）累计已删节点，再从本批 `remove_nodes` 的 payload 里去掉这些 ID，删空的整条丢掉；其他 Op 一条都不改，payload 只在 ID 列表变短时重写。 |
| 3 | `E.05.05` §5 / 急停、锁与动作 | 符合 | `authz_hook.go:77-80` 急停只在操作者是模型时拦；`:83-90` `set_lock` 只允许 user、`migrate` 只允许 runtime；`checkLocks`（`:173-213`）的作用对象含 `target.nodes`、连线两端节点和容器链，`lockOf`（`:217-240`）沿容器链往上找且 `frozen` 优先于 `user`；`frozen` 拦所有人（`set_lock` 例外，`:204`），`user` 锁不拦 `move_nodes`/`set_node_ui`（`:200、205`）；锁住时在 Check 循环（`:93-97`，先于 `:107-120`）直接返回 `Deny`，不调 `Check`、不转提议。 |
| 4 | `E.05.05` §4 / M1 授权桩 | 符合 | `engine/authz/check.go:1` 首行就是 `// STUB(E.06.02)`；`:53-59` `Fixed.Check` 对任何请求返回 `allow`，原因写 K255。名字和字段与 `E.05.05 §4` 一致，只增不改。 |
| 5 | `E.05.01` §5.1 第 0 步 / 服务器字段与门槛 | 符合 | `pipeline.go:322` `finishOps` 把客户端填的 `Actor`、`Approver`、`Inverse`、`Ts` 全部清空；`:274-276` 模型提交必须写非空 `reason`；`:261-266` M2–M4 的 kind（group/ungroup/set_subgraph_ref/import、set_mandate、apply_patch、分支三件套）与客户端的 `restore_nodes` 都返回 badf（400）。 |
| 6 | `E.05.01` §5.1 第 7 步 / 阻止提交的诊断 | 符合 | `pipeline.go:184-186` `viewOnly` 只有 `move_nodes`（K263），注释写明便签大小、ui 宽度、字数都受结构约束要校验；`:188-192` `blockingCodes` 恰为 V01、V03、V05、V06、V07、V09、V12、V14（K262②）；`:602-615` `blocking()` 只收 **新出现** 的 error（按码+节点+端口+参数+层去重，不比文案）；`:518-522` 只有非 migrating 才判 blocking。 |
| 7 | `E.05.01` / 失败批次不留痕 | 符合 | `pipeline.go:199-221` `submit` 在 `Repo.Tx` 里做，返回错误即回滚；`applyBatch` 在事务内 `Commit`（`:546`），缓存、事件、RunNotifier 全部在事务提交后的 `post`（`:565-573`）里更新；拖动合并路径同样只在 `post` 里更新缓存（`coalesce.go:102-110`）。 |
| 8 | `E.05.01` §5.1、E.05-R7 / 服务器补全与重放 | 符合 | `apply_node.go:73-99` 容器（required/one_of）没带子图时自动加一对 sub_port，且把补全写回 `op.Payload`（落库的是补全后的）；`cache.go:50-77` `load` 用 `p.env(true, true)`（`:62`），重放时不补全、不生成 ID（`applyAddNode` 的 `!e.replay`，`:95`）。 |
| 9 | `E.05.01` §5.1 第 8 步 / 全文份数 | 符合 | `pipeline.go:22-23` `fullEvery = 50`；`:537-540` 版本差 ≥50 且本批最后一条没有 `coalesce_key` 时才存全文；`coalesce_test.go` E0504-T5 验证带 `coalesce_key` 的第 50 版不存全文、第 51 版普通 Op 存全文，重放后内容一致。 |
| 10 | `E.05.01` / 逆操作（`C.05.03` §2） | 符合 | `apply_param.go:64-97` `set_param` 的逆写**整个参数的旧值**（没写过的回退到默认值）；`apply_view.go:86-118` `update_note` 的逆是删掉再整份加回（K261⑦）；`connect`/`disconnect`、`promote`/`demote`、`set_label`/`set_tags`/`set_lock`、`move_nodes`、便签增删的逆在 `apply_edge.go`、`apply_param.go`、`apply_attr.go`、`apply_view.go` 对称实现；`migrate` 不写 inverse（`pipeline.go:488-490`）。 |
| 11 | `E.05.02` §5.1、§5.4 / 撤销栈与重做栈 | 符合 | `pipeline.go:617-657` `pushUndo`：重做栈 ActorID 是 `<actor.id>#redo`（`:646`）；普通批次清空重做栈，重做批次不清（`:642-644`）；撤销批次不推撤销栈（`:622-623`）。`undo.go:63-73` 撤销成功后在同一事务里弹出撤销栈顶、把组推进重做栈；`Redo`（`:79-115`）按服务器补全后的 kind/payload/coalesce_key 重提交。 |
| 12 | `E.05.02` §5.6 / 互相抵消（K264） | 符合 | `undo.go:194-258` `cancelPairs`：① 已撤销的组和撤销它的批次互相去掉；② 剩余的撤销批次和重做它的批次互相去掉。两步都按 `"<组>|<操作者>"` 匹配（`:224、228、236、244`），只认同一个操作者，模型借不了用户的撤销记录；① 用过的撤销批次不再参与 ②（`:237、241-243`）。`E0502-T6 抵消` 的测试覆盖撤销/重做往返。 |
| 13 | `E.05.02` §5.5 / 撤回与回到版本 | 符合 | `revert.go:56-89` `expandRevertOp` 把目标 Op 的 inverse 展开成具体 Op 再落库（`internal=true`，不重走解码）；`:92-121` `expandRevertTo` 取 `Range(v, head)` 记录的 inverse **倒序**拼，遇到不可逆的（`migrate` 无 inverse）返回 `ConflictError`（409，`:111-114`）；`applyRevertTo`（`:125-157`）逐条应用，逆是子 Op 逆操作的倒序。 |
| 14 | `E.05.04` §5 / 合并条件 | 符合 | `coalesce.go:14-52` 只在下列条件全满足时合并：单条 Op、`base_version` 等于 head（只合并分支最新一条）、带 `coalesce_key`、`move_nodes`/`set_param`、上一条同 kind 同 key 且**同一操作者**、2 秒窗口内、作用对象相同；`set_param` 让动态端口消失时不合并（`:59-61`）。`r.Coalesce`（`:87`）只改上一条的 payload，inverse 保留第一次的旧值（撤销一次回到最初位置）。`coalesce_test.go` E0504-T1–T5 逐条验证（59 次合并、超时、跨操作者、插队、端口消失、重放与全文）。 |
| 15 | 整片 / 测试没有放宽 | 符合 | `prop_test.go:24-149` `genOp` 覆盖全部 M1 kind 且刻意生成会违反前置条件的组合（不存在的节点、空集合、非法值）；T2（`:156-187`）检查被拒的 Op 不改图（`:167-172`）、逆序应用逆操作后图哈希复原（`:182-184`）；T8（`:189-215`）逐条比较增量与全量诊断。`coalesce_test.go`、`apply_test.go` 等断言写死具体计数与位置（如 59 个 `op.coalesced`、最终坐标 600），没有放宽。 |

## 实施者处理
- 实施者：Quick（AWS Quick）· 2026-11-26。十五条都符合时不用处理。

## 已知、不算不符合
- 拖动合并后那一版在存储里的哈希不更新（`E.05.04` §5 已知问题）。
- 结构修改的增量校验 500 节点约 20ms（`P.09` K261 ⑨ 待办）。
- 审计、扩展包字段在 M3、M2。