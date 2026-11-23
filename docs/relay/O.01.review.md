# 评审 · O.01 构建流水线

- 评审者：Codex · 2026-11-23
- 依据：任务包 .relay/tasks/O.01.md（设计仓库提交 0d140e701659a812a024eef57353d1dbd1a883ab）

| # | 实施卡 / 小节 | 结论 | 说明（不符合或有疑问时写依据：文档编号和小节、代码文件和行号） |
|---|---|---|---|
| 1 | O.01 §4.1 目标与 Make/Task 一致 | 符合 | `Makefile` 5-27 与 `Taskfile.yaml` 5-28 逐项对应，八个目标同名同序，都只有一行命令；`tools/build/main.go` 220-254 的 `check` 顺序是“gen → 生成物无差异 → lint → test”，符合 §4.1。 |
| 2 | O.01 §4.2 版本号、提交时间与可复现构建 | 符合 | `tools/build/version.go` 21-97 完整实现标签、`-dev.3`、`+dirty`、`-preview`、无标签回退和提交时间规则；`tools/build/build_slow_test.go` 15-21 对应验收 3。 |
| 3 | O.01 §4.3 `tools/dev` 双进程退出 | 符合 | `tools/dev/main.go` 25-50、92-148 与 `tools/dev/proc_windows.go` 11-35、`proc_other.go` 11-27 一起实现参数透传、输出前缀、双进程启动、任一退出和 Ctrl+C 时停止另一端；`tools/dev/dev_test.go` 47-48 覆盖 `O01-T6`。 |
| 4 | O.01 §4.5 `tools/relay` 六个命令 | 符合 | `tools/relay/main.go` 41-90、`tools/relay/pack.go` 36-163、`tools/relay/check.go` 25-129、`tools/relay/status.go` 25-91 覆盖 `status/next/start/check/diff/stale`，对应测试在 `tools/relay/relay_test.go` 115-395。 |
| 5 | O.01 §5 数据结构与文档指纹 | 符合 | `tools/relay/docs.go` 312-329 的 `NormHash` 按规范去掉状态、最后修改和变更记录；`tools/relay/pack.go` 302-309 生成 `docs_commit`、`docs`、`stubs`、`state`，与 §5 的 JSON 结构一致。 |
| 6 | O.01 §6 行为：嵌入目录、失败即停、注入与回填 | 符合 | `tools/build/main.go` 93-126 按顺序检查前端、复制 `web/dist`、注入构建信息并构建二进制；`cmd/anydoor/main.go` 28-59、60-99 正确注入版本、启动、监听并支持优雅退出；`tools/build/build_test.go` 179-187 覆盖缺前端时的退出 1。 |
| 7 | O.01 §7 不变量与 §8 错误处理 | 符合 | `Makefile`、`Taskfile`、`tools/build/main.go` 220-254、`tools/dev/main.go` 25-50 均做到失败即停、退出码明确；`cmd/anydoor/main.go` 60-99 也符合上游约束与桩范围。 |
| 8 | O.01 §10 验收 1–6 | 符合 | 已实测构建、`--version`、健康接口、`/` fallback、`some/route` fallback、可复现哈希和 dev 退出测试，与 §10 意图一致。 |
| 9 | 实施卡 `O01-T1`–`T5`：版本、目标、复制、缺前端、可复现 | 符合 | `tools/build/build_test.go` 56-187 与 `tools/build/build_slow_test.go` 15-21 分别覆盖 `O01-T1`–`T5`，测试名都以编号开头，且断言点与实施卡条目对应。 |
| 10 | 实施卡 `O01-T6`–`T11`：开发退出、relay 全流程 | 符合 | `tools/dev/dev_test.go` 47-48、`tools/relay/relay_test.go` 115-395 分别覆盖 `O01-T6`–`T11`，包括领取拒绝、白名单、差异、状态输出等场景。 |
| 11 | 任务包最小集（G.02.02） | 符合 | `tools/relay/pack.go` 36-94、165-176 按“必读 + 祖先 + 本叶子 + 按任务类型附加 + 直接引用”生成任务包，且对其他实施叶子剔除实施卡；当前任务包未漏掉所需守则或关键依赖文档，也没有明显多余内容。 |
| 12 | Windows 控制台编码（K198） | 有疑问 | `cmd/anydoor/console_windows.go` 20-28、`cmd/anydoorctl/console_windows.go` 20-28 已按 B.02/B.08 要求设置并恢复控制台代码页，行为本身正确；但 `console_windows.go`、`console_other.go` 未在 O.01 实施卡的桩清单中逐一列出，只在 RELAY 与 `docs/relay/O.01.json` 登记。建议后续把这两个新增桩回写进 O.01 实施卡，避免读者误解为越界。已处理（复核：Codex，2026-11-23） |
| 13 | 领取记录与 `--accept-docs` 后的依据哈希 | 有疑问 | `docs/relay/O.01.json` 第 6 行已更新为设计文档提交 `0d140e701659a812a024eef57353d1dbd1a883ab`，但 `RELAY.md` 第 22 行仍写着旧依据 `2a585f6`，且第 23-24 行未体现 `--accept-docs` 后的新提交号。建议在最终交付前同步这条记录，避免两个“当前依据”不一致。已处理（复核：Codex，2026-11-23） |
| 14 | `tools/relay status` 未处理评审意见扫描 | 不符合 | `tools/relay/status.go` 137-152 只按整行是否包含“不符合”或“有疑问”判断，未排除表头或说明文字。实际输出把 `docs/relay/O.01.review.md` 的列头也列为“没处理完的评审意见”，这与 O01-T11 想要的准确状态输出不一致，应改为解析表格结果或用显式标记。已处理（复核：Codex，2026-11-23） |

## 总结

代码总体满足 O.01 的目标、版本号、dev 退出、relay 七项检查和任务包生成要求；但 `tools/relay status` 的未处理评审意见扫描会误报表头，属于需要修正的实际缺陷。另有两点建议核对：新增控制台编码桩应回写进 O.01 实施卡；`RELAY.md` 的“依据”哈希应与 `--accept-docs` 后的当前提交保持一致。

## 实施者处理（Quick · 2026-11-23）

> 按 `G.02.06-R18`：实施者在这里逐条写怎么处理的；评审者复核后，在上表对应行的"说明"末尾加"已处理（复核：评审者，日期）"。

| # | 处理 |
|---|---|
| 12 | 设计方裁决：采纳。`O.01` 实施卡的桩清单补上 `cmd/anydoor/console_windows.go`、`console_other.go`、`cmd/anydoorctl/console_windows.go`、`console_other.go`（设计仓库下一次提交，`P.09` K199）。代码不变 |
| 13 | 设计方裁决：采纳。`RELAY.md` 本棒记录的依据改为 `--accept-docs` 之后的提交，并写明每次更新依据时读过的改动和处理方式。这次设计文档又改了（K199），验证者执行 `--accept-docs` 后，以 `docs/relay/O.01.json` 里的 `docs_commit` 为准 |
| 14 | 修了。`tools/relay/status.go` 的 `openReviewRows`：找表头里"结论"那一列，只有结论一格以"不符合"或"有疑问"开头、而且这一行没有"已处理"的才列出；表头、分隔行、表格外的说明文字都不算。`O01-T11` 的样例评审记录改成这种表格，加了断言：表头、"符合"的行、已处理的行、说明文字都不出现在输出里。规则同时写进设计文档 `G.02.06-R18`、`O.01` §4.5 |
| 另 | 第二轮验证时发现：`P.09` 被几乎每个叶子引用，每加一条裁决都会让依据失效。设计方裁决：任务包不再附 `P.09`（`G.02.02` §4，`P.09` K199）；`tools/relay/pack.go` 的 `neverAttach` 加上它，`O01-T8` 加了断言。复核：同意（Codex） |

## 第 2 轮 · fix/O.01-stale（K200）

- 评审者：Codex · 2026-11-23
- 依据：设计仓库提交 `260c067`（O.01 §4.5、G.02.06-R21、P.09 K200）

| # | 实施卡 / 小节 | 结论 | 说明 |
|---|---|---|---|
| 1 | O.01 §4.5 `stale --accept`：只用于冻结叶子 | 符合 | `tools/relay/status.go` 257-265 先读设计文档状态，非“冻结”时拒绝并指向 `start --resume --accept-docs`；`tools/relay/relay_test.go` 373-374 用 X.02 断言未冻结叶子会被拒绝。 |
| 2 | O.01 §4.5 `stale --accept`：设计仓库必须已提交 | 符合 | `tools/relay/status.go` 266-270 检查设计文档仓库 `git status --porcelain`，有未提交修改即拒绝；`tools/relay/relay_test.go` 376-378 构造脏设计仓库并断言报错。 |
| 3 | O.01 §4.5 `stale --accept`：依据没变不动、成功后 state 不变 | 符合 | `tools/relay/status.go` 274-277 对未变化叶子只提示“不用更新”并返回，未写领取记录；283-288 通过 `rebase` 更新 `docs_commit` 和 `docs`。`tools/relay/pack.go` 216-226 只改依据字段；测试 `tools/relay/relay_test.go` 381-388 断言提交号更新且 `state == merged`，393-395 断言再次 `--accept` 不动作。 |
| 4 | O.01 §4.5：冻结叶子的 `status` 显示 | 符合 | `tools/relay/status.go` 41-47 优先读取设计文档状态，冻结叶子输出“已冻结”，不再列分支、最后提交和测试覆盖；依据变化时提示“运行 relay stale”。`tools/relay/relay_test.go` 367-371 对应断言，并确认不再出现“等设计方冻结”。 |
| 5 | 实施卡 `O01-T10` 新断言 | 符合 | `tools/relay/relay_test.go` 355-395 覆盖：只冻结不算过期、冻结后正文变化列出、status 显示和提示、未冻结拒绝、设计仓库脏拒接、更新成功、`docs_commit` 更新、`state` 不变、后续 `stale` 清空、重复 `--accept` 提示不用更新；与 O.01 第 140 行实施卡一致。 |
| 6 | `G.02.06-R21` 冻结后复查流程 | 符合 | `tools/relay/status.go` 221-255 对冻结且依据变化的叶子输出复查前必须先看 `diff`，并区分“代码要改则改回已审开修复分支”和“不用改则 stale --accept”；257-290 的 `stale --accept` 更新依据后提示在 `RELAY.md` 记录复查结论、随领取记录按合并请求流程合并，与 G.02.06-R21 一致。 |
| 7 | `rebase` 抽取后的 `start --resume --accept-docs` | 符合 | `tools/relay/pack.go` 216-226 把原依据更新逻辑抽成 `rebase`，`Start` 在 `pack.go` 282-287 按同一条件调用，不建分支、不改领取记录；`tools/relay/relay_test.go` 322-346 仍覆盖 `diff` → 第 ⑥ 项失败 → `--accept-docs` 后通过。 |
| 8 | 命令行用法与退出约定 | 符合 | `tools/relay/main.go` 5-37 的注释和用法文本都标为 `stale [--accept <叶子>]`；79-89 解析 `--accept`，多余位置参数按用法错误返回；运行 `go vet ./...`、`golangci-lint run ./...` 和全量测试均通过。 |

## 第 2 轮总结

`stale --accept` 和冻结叶子状态显示与 K200、O.01 §4.5、G.02.06-R21 一致；`rebase` 抽取未改变 `start --resume --accept-docs` 的既有行为。本轮没有“不符合”或“有疑问”。

## 第 3 轮 · fix/O.01-slice（K208）

- 评审者：Codex · 2026-11-23
- 依据：设计仓库提交 `0b7ffcc`（G.02.06-R23/R24、O.01 §4.5、§5、实施卡、P.09 K208）

| # | 实施卡 / 小节 | 结论 | 说明 |
|---|---|---|---|
| 1 | `G.02.06-R23`：`start --slice <片名>` 与分支规则 | 符合 | `tools/relay/main.go` 5-37、79-137 加入 `--slice` 和 `slice` 命令；`tools/relay/pack.go` 226-272 校验片名，首个叶子从 `main` 建 `slice/<片名>`，已有分支则切过去。`tools/relay/relay_test.go` 453-470 覆盖不合规片名、分支名和首个叶子的 `base`。 |
| 2 | `G.02.06-R23`：领下一个前自动检查上一个 | 符合 | `tools/relay/pack.go` 313-330 先执行 `deliverSlice` 再领取下一个；`tools/relay/status.go` 113-176 汇总检查同一片仍在做的叶子，全部通过才写入 `delivered` 和 `head`，失败则返回不改记录。`tools/relay/relay_test.go` 471-482 覆盖越界拒绝、修正后可领取。 |
| 3 | `G.02.06-R23`：每个叶子按自己的 `base/head` 负责 | 符合 | `tools/relay/check.go` 171-207 用 `claimRange` 和 `changedFiles(base, head)` 限定检查区间；`tools/relay/pack.go` 297-307 为片内领取写入 `Slice/Base`，`deliverSlice` 写入 `Head`。测试 483-501 断言前叶子的 `head` 等于下一个领取前提交，后叶子改动不计入前叶子的越界检查。 |
| 4 | `G.02.06-R23`：`slice` 整片验证，一次 ⑦ | 符合 | `tools/relay/status.go` 178-224 检查工作区必须干净，逐个叶子输出 ①–⑥，整片只跑一次 ⑦，通过后再统一标记 `delivered`；`main.go` 77-93 解析 `--no-build`。测试 514-525 覆盖脏工作区拒绝、按片名找不到时报错和最后一个叶子标记。 |
| 5 | `O.01` §4.5 `--slice` 规则 | 符合 | 片名规则、分支创建/切换、上一个叶子 ①–⑥ 检查、失败拒绝、成功标 `delivered` 并写 `head`、与新领取记录一起提交、新 `base` 取当前提交、同片跳过白名单重叠，分别对应 `pack.go` 226-330、`340-363` 与测试 453-504。 |
| 6 | `O.01` §4.5 `check` 新增范围与白名单 | 符合 | `tools/relay/check.go` 69-120 按领取记录求 `base/head`，同片领取记录和 `<片名>.review.md` 纳入允许项，并把实施卡逐个写出的文件放在“不许碰”目录之前。测试 476-478 和 489-501 分别覆盖前叶子和后叶子按各自范围检查。 |
| 7 | `G.02.06-R24`：逐个写出的文件优先 | 符合 | `tools/relay/check.go` 93-114 的 `exactlyListed` 只有完全同名字符串才优先，目录禁令仍拦住未写出的其他文件；第 ② 项也只豁免逐个写出的 `schema/` 文件。`relay_test.go` 255-264 覆盖 `pkg/b/shared.go` 允许、`pkg/b/other.go` 拒绝，`testdata/tree/X/X.01_甲.md` 加入该用例。 |
| 8 | `O.01` §5 三个新字段 | 符合 | `tools/relay/main.go` 184-193 的 `Claim` 新增 `slice/base/head` 且 `json omitempty`，与 §5“只出现在片内领取的叶子上”一致；`pack.go` 297-307 写入 Slice/Base，`status.go` 141-169 按 `Head` 记录完成提交。 |
| 9 | 实施卡 `O01-T12` 与 `O01-T9` 新增断言 | 符合 | `relay_test.go` 453-526 逐项覆盖片名、base、拒绝领取、修正后领取、head、范围检查、status、脏工作区、整片通过、最终标记和无该片名；255-264 覆盖逐个写出的文件优先于禁碰目录，与设计文档实施卡一致。 |
| 10 | `claimRange` 在提交不存在时退回分叉点 | 不符合 | `O.01` §4.5 写明：已标为 `delivered` 的叶子只看 `head` 为止，“记录里的提交已经不存在时退回分叉点”。但 `tools/relay/check.go` 171-188 只有 `base` 不存在才整体退回 `baseCommit`；若记录的 `base` 存在而 `head` 已不存在，代码会保留该片叶子的 `base` 并把 `head` 当空值检查到当前工作区，没有退回分叉点。这会让后续叶子的改动被误算进已合并不在原分支上的叶子。测试中也没有覆盖“head 提交丢失”的场景。已处理（复核：Codex，2026-11-23） |
| 11 | `deliverSlice` 是否先全部检查完再写记录 | 符合 | `tools/relay/status.go` 113-176 先收集所有仍在做叶子的检查结果，只要有一项失败即返回；全部通过后才循环写入 `state=head`。该流程满足“先全部检查、全部通过才写记录”。 |
| 12 | 对非片内领取的影响 | 符合 | 普通领取没有 `Slice/Base/Head`，`claimRange` 仍走 `baseCommit()`，分支仍是 `feat/<叶子>-<简称>`；非片内 `check` 的 ③④ 仍在分叉点到工作区比较。逐个写出的文件优先于禁碰目录是 R24 对所有实施卡的明确新规则，不算回归。 |

## 第 3 轮总结

片内连续领取、片名校验、同片范围内检查、整片一次 ⑦、三个新字段和 `O01-T12/T9` 断言总体落实；但 `claimRange` 在记录的 `head` 提交丢失时没有按要求退回分叉点，属于需要修正的实际缺陷。

## 第 3 轮 · 实施者处理（Quick · 2026-11-23）

| # | 处理 |
|---|---|
| 10 | 修了。`tools/relay/check.go` 的 `claimRange`：`base`、`head` 要么一起用，要么一起不用——`base` 存在、且（没记 `head` 或 `head` 也存在）时才用记录里的范围，否则整体退回分叉点、`head` 为空。`O01-T12` 末尾补了断言：只有 `head` 不存在、只有 `base` 不存在两种情况都退回分叉点且 `head` 为空（用 X.02 的记录，它的 `base` 不是分叉点，修复前的代码会返回它的 `base`，断言能区分）；两个提交都在时用记录里的范围。设计文档 `O.01` 的 `O01-T12` 同时补了这一句 |
| 另 | 设计文档另有一处改动：用户授权以后合并不再逐次批准，`G.08-R3` 改为"门禁和评审都通过、没有未处理意见时由验证者合并，事后报告；实施者不能合并自己的 PR"（`P.09` K209）；`G.02.06`、`O.02` 相应修改。代码不受影响 复核：同意（Codex） |
