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
