# 评审 · m1-base（第 ④ 片）

- 评审者：Codex · 2026-11-26
- 依据：设计仓库 `24b270e`（`E.08`、`E.09.01`、`E.15.01`–`.04`；`P.09` K265、K266）；评审材料 `.relay/review/m1-base.digest.md` 的 15 条重点；代码 `engine/sched/`、`engine/blob/`、`engine/blob/fsblob/`、`engine/obs/`（片内提交 `3c5536c`、`f3b3e0a`、`771430e`、`1910fbc`、`e13728c`、`f7aee84`）
- 结论：15 条全部符合，没有"不符合 / 有疑问"，可以合并。

| # | 实施卡 / 小节 | 结论 | 说明 |
|---|---|---|---|
| 1 | `E.15.04` §5 / 虚拟时钟触发顺序、Stop、Reset | 符合 | `vclock.go:79-104` `Advance` 每轮在到期不晚于 target 的定时器里取 `at` 最小、同时到期取 `seq` 小的（`:92`）；ticker 跨过周期时每周期送一次（`:99-101`）。`vTimer.Stop`（`:132-138`）返回是否在等、移除后不再触发；`vTimer.Reset`（`:140-148`）返回值同 `time.Timer`（改前的 active 状态），`d≤0` 立刻送一次不登记（`:69-74`）。 |
| 2 | `E.15.04` §5 / 时钟检查 | 符合 | `clockcheck_test.go:44-77` 用 go/parser 扫 sched、run、op、exec、obs 五个目录的非测试 `.go`，只豁免 `sched/clock.go`（`:66-68`）；别名导入按 import 名解析（`:31-41`），自测 `tm.After` 报、`tm.Duration` 不报（`:56-58`）。run、exec 目录不存在时跳过。 |
| 3 | `E.15.03` §5 / 速率、取消、降速 | 符合 | `rate.go:62-71` `refill` 按 last→now 分段（以 `r.until` 切开降速窗口）补令牌、上限 burst；`Wait`（`:85-105`）每轮先查 `ctx.Err()`，select 同时等 ctx.Done 与定时器、取消时停表立刻返回；`Penalize`（`:108-130`）夹因子 0.01–1、叠加取 `math.Min`、恢复时间取最晚，恢复后在 refill 里重置（`:72-75`）。 |
| 4 | `E.15.02` §5 / Acquire 名额与释放 | 符合 | `pool.go:98-125` 先 `tryAcquire`，拿不到才算 waiting（defer 减回，`:111-118`）、ctx 取消时 `sem.acquire` 返回错误、不占名额；`onceRelease`（`:80-83`）用 `sync.Once`，release 多次只放一次（`pool_test.go:56-59` 双调用验证）。 |
| 5 | `E.15.02` §4 / 热更新与共享 | 符合 | `pool.go:67-96`：`Reconfigure` 建新 generation，已取的快照持旧 generation 不受影响；同一 generation 的快照共享全局 Gate（`:82`），`Get` 按键缓存同一个 Pool（`:85-96`）。 |
| 6 | `E.15.01` §5 / Judge 五步 | 符合 | `ready.go:63-104`：`startable`（`:55-59`，K266 拆出的小函数）→ 没有入边 Ready → 上游 failed/cancelled 返 NotReady（`:73-74`）→ 未到终态时 `Early(arrived, total)`（`:81-86`）→ 全部终态：arrived==0 或必填端口全未激活返 Skip（`:87-102`），否则 Ready 带 empty；empty 只含未激活的可选端口并排序（`optionalEmpty` `:106-122`）。 |
| 7 | `E.15.01` §5 / 优先级 | 符合 | `prio.go:11-16` 按字段顺序 Escalation→Critical→Round→User 比较，小者优先；`Less`（`:33-37`）同优先级按 seq（入队顺序），`Push` 自增 seq（`:44-48`），稳定。 |
| 8 | `E.08` §5 / 原子写与去重 | 符合 | `fsblob.go:74-117` 内容先写 `.tmp/` 再改名：meta 先改（`:105-108`）内容后改（`:109-115`），任何失败都删 tmp、不留半个文件；`New`（`:25-38`）清 `.tmp/`；内容已存在（`:86-89`）或并发竞态（`:110-113`）只存一份。 |
| 9 | `E.08` §6 / 读校验与引用 | 符合 | `blob.go:21` `refRe` 只认 `ws://sha256:` + 64 位小写 hex，`ParseRef`（`:27-32`）格式不对返 `ErrBadRef`，拼不出目录外的路径；`verifier.Read`（`fsblob.go:174-181`）读到 EOF 时校验、不一致返 `ErrCorrupt`；`open`（`:119-128`）先 ParseRef 再取路径。 |
| 10 | `E.09.01` §5 / 先落库再广播 | 符合 | `writer.go:103-140` `EmitRun` 先全量 check（actor、data 上限，`:109-115`）有一条不合规整批不写；一次 `AppendRun` 落库（`:126`），成功后按顺序广播（`:135-138`），落库失败返回错误、不广播（`:127-129`）；seq 由 last 倒推连续分配（`:130-133`）。`EmitSession` 同构（`:143-168`）。 |
| 11 | `E.09.01` §5 第 2 条 / S 级节流 | 符合 | `writer.go:35` `sLevel` 只有 `node.progress`、`node.throttled`；`:118-124` 按 `ExecKey(*e.Loc)` 每秒计数（`window.sec`），超限的只跳过落库、seq 保持 0 照样广播（`:113`、`:136-138`）；表超过 1 万个执行时清空（`:82-84`）；没有 loc 的不节流（`:119`）。 |
| 12 | `E.01` §1 / obs 依赖 | 符合 | `writer.go:5-13` 只 import 标准库和 `gen`，`hub.go:3-6`、`path.go:3-8` 同样；存储和时钟用本包接口，由组装处接上（包注释 `writer.go:1-2`）。 |
| 13 | `E.09.01` §5 / Hub | 符合 | `hub.go:70-81` `Publish` 持锁、缓冲满的订阅者置 lagged 并 `drop`（`:57-67` 关 channel，缓冲里已有的还能读出），不阻塞写入；`Sub.Close`（`:84-88`）经 drop 的 map 成员检查（`:59-61`），两次 Close 不会重复 close。 |
| 14 | `C.06.05` §4、R5 / 路径向量 | 符合 | `path.go:47-49` `PathOf` 的 `@<重试>` 只写在链最后一项且 attempt>1；`ExecKey`（`:79-89`）容器用 `iterKey(..., false)` 不写 attempt（`:84`），自身写（`:87`）；E0901-T4 对 `schema/examples/path_vectors.json` 全部一致（测试通过）。 |
| 15 | 整片 / 测试没有放宽 | 符合 | 三个新包的测试里没有 `t.Skip`、TODO 或放宽断言；虚拟时钟测试不靠真实睡眠。`pool_test.go:34` 的 `time.Sleep(2ms)` 是并发测试里占住名额的持有时间、不等条件，断言写死峰值 4（`pool_test.go:40-42`），不依赖时序。本机无 C 编译器、`-race` 要求 cgo，`-race` 留给 Linux CI（`G.05.02` §3）。 |

## 实施者处理
- 实施者：Quick（AWS Quick）· 2026-11-26。十五条都符合时不用处理。

## 已知、不算不符合
- blob 的引用计数和 GC 在 `B.09`（`E08-T3`）；投影、视图快照、背压在 `E.16.01`（`E0901-T3`）；节点级并发上限由执行器自己做（`E.15.02` §5）。
- `-race` 留给 Linux CI（`G.05.02` §3）。
- 合并后在 main 上复跑全量测试时，`engine/store/sqlite` 的 `E0704-T1 并发写`（要求 <10s）因整机负载抖动用了 11.58s，单独重跑 2.88s 通过（3.36s），属已知抖动。