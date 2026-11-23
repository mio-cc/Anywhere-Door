# 接力日志（代码）

> 每一棒在**最上面**加一条（新的在上）。开工先读最上面一条；收工前必须写完自己这一条（设计文档 `G.02.06-R20`）。
> 只写"人才知道的事"：停在哪、为什么、下一步。进度数字以 `go run ./tools/relay status` 为准，不手抄。

```
## {日期 时间（北京时间）} · {叶子编号} · {实施者} · {领取 / 续做 / 交付}
- 依据：设计仓库提交 {短哈希}（docs/relay/{叶子}.json）
- 完成：实施卡第 … 条；测试用例 …
- 进行中：停在实施卡第 N 条（{一句话}）；没有写"无"
- 下一步：
- 桩：{文件} → {叶子}；没有写"无"
- 上报单 / 阻塞：没有写"无"
- relay check：{通过 / 不通过的项}（验证者：{谁}）
- 评审：{docs/relay/{叶子}.review.md 的结论}（评审者：{谁}）
- 提交 / 推送：{最后一次提交短哈希} / {已推送 / 未推送及原因}
```

---

## 2026-11-23 11:58 · O.01 · Quick（实施）+ Codex（验证、评审）· 交付
- 依据：设计仓库 `6a4e791`（`docs/relay/O.01.json`，`state` 改为 `merged`）。
- 完成：实施卡全部条目；`O01-T1`–`T11` 全部有对应测试。三轮验证；交叉评审 14 条全部符合或已处理（`docs/relay/O.01.review.md`）；`relay status` 没有未处理意见。加了 `CHANGELOG.md`（`G.09-R4`）。
- 合并：用户 2026-11-23 11:56 批准（`G.08-R3`），squash 合并进 `main`（`G.07-R6`），合并提交号见 `git log`；分支 `feat/O.01-build` 合并后删除。
- 进行中：无。
- 下一步：设计方在设计仓库把 `O.01` 改为"冻结"、写上合并提交号（`G.02.06-R21`）。第 2 片：契约层 `schema/` 首版（`C` 枝），开工前由设计方把用到的 `C` 叶子改为"已审"；从第 2 片起按 `G.02.06` 正常领取（`go run ./tools/relay next` / `start`），不再走 §5 例外，实施卡每完成一条就提交一次。
- 桩：同上一条（17 个文件，`relay status` 可以列出）；各自由对应叶子整体替换。
- 上报单 / 阻塞：无。
- relay check：七项全部通过（验证者：Codex）。
- 评审：符合，全部已处理（评审者：Codex）。
- 提交 / 推送：`9017cdc`、`168c843`、`15e435c`、`76bd28b` 加本条的交付提交，squash 进 `main` 后推送。

## 2026-11-23 11:55 · O.01 · Quick（实施）· 续做
- 依据：以 `docs/relay/O.01.json` 的 `docs_commit` 为准。领取时 `2a585f6`；第二轮 `--accept-docs` 更新到 `0d140e7`（读过的改动：`B.02`、`B.08` §5 控制台编码，`O.01` §4.5 `--accept-docs`、§5 指纹，`P.09` K198，都是本棒自己的回写，代码已一致）；本条之后的设计仓库提交（K199：`O.01` 实施卡桩清单与 §4.5、`G.02.06-R18`、`G.02.02` §4）由验证者再 `--accept-docs` 一次。按 `G.02.06` §5 例外人工领取。
- 完成：实施卡全部条目；测试用例 `O01-T1`–`T11` 全部有对应测试。第一轮验证（Codex，11:31）：`go vet`、`go build`、`go test ./...`、`golangci-lint`（0 issues）、`go test -tags slow`（`O01-T5`）、`go run ./tools/build -check`、`relay check O.01` 七项全部通过；两次构建 `anydoor.exe` 哈希相同（`D911856…`）；冒烟：`/api/v1/health` 返回 `{"ok":true,"version":…}`，`/` 与 `/some/route` 都返回 index.html。前端依赖：vite 8.3.8、typescript 7.0.2（`--save-exact`）。
- 第一轮发现 1 个问题：中文 Windows 控制台（代码页 936）把 `anydoor` 的 UTF-8 输出显示成乱码。已修：`cmd/anydoor`、`cmd/anydoorctl` 启动时设控制台为 UTF-8、退出时恢复（新增两个桩文件 `console_windows.go`、`console_other.go`，已登记）；设计文档 `B.02`、`B.08` 补了这条（`P.09` K198）。同时把实施中定下的两个细节回写进 `O.01`（`--accept-docs`、文档指纹算法）。这些设计文档改动会让 `relay check` 第 ⑥ 项不通过，按 `G.02.06-R13` 先 `relay diff`、再 `start --resume --accept-docs`。
- 第二轮（Codex，11:46）：中文显示正常、退出后代码页恢复 936；`relay check` 第 ⑥ 项按预期拦下依据变化，`diff` → `--accept-docs` 后七项全过。交叉评审 14 条：11 符合、2 有疑问（第 12、13 条）、1 不符合（第 14 条：`status` 把评审表头当成未处理意见）。处理见 `docs/relay/O.01.review.md` 的"实施者处理"：第 14 条修了（按"结论"列判断），第 12、13 条采纳；另外任务包不再附 `P.09`（K199）。
- 进行中：等评审者复核第 12–14 条。
- 下一步：验证者：设计仓库提交推送 → `relay diff O.01` → `--accept-docs` → 完整 `relay check` → 评审者复核并标"已处理" → `relay status` 无未处理意见 → 分几次提交 → 推送 `main` 与分支。合并进 `main` 等用户批准（`G.08-R3`）。
- 桩：`cmd/anydoor/` 下的 `main.go`、`embed.go`、`console_windows.go`、`console_other.go` → `B.02`；`cmd/anydoorctl/` 下的 `main.go`、`console_windows.go`、`console_other.go` → `B.08`；`tools/gen/main.go` → `G.16`；`tools/depcheck/main.go` → `G.03`；`tools/lint/quirkscheck/main.go`、`tools/lint/typenamecheck/main.go` → `G.05.05`；`web/` 下的 `package.json`、`pnpm-lock.yaml`、`index.html`、`tsconfig.json`、`src/main.ts`、`scripts/stub.mjs` → `W.01`、`W.02`、`W.12`。都已登记在 `docs/relay/O.01.json`。
- 上报单 / 阻塞：无。
- relay check：第一轮、第二轮七项全部通过（验证者：Codex）；修复后待第三轮。
- 评审：`docs/relay/O.01.review.md`（评审者：Codex），第 12–14 条待复核。
- 提交 / 推送：未提交（本叶子因 `tools/relay` 还不存在，没有按 `G.02.06-R8` 逐条提交，属 §5 例外；从下一个叶子起逐条提交）。

## 2026-11-23 · 初始化（第 0 片）· Quick（实施）+ Codex（验证）
- 依据：设计仓库 `mio-cc/miocode` 提交 `da84086`；按 `G.02.06` §5 例外：`tools/relay` 还没写，领取与检查人工执行。
- 完成：建仓库与接力入口：`AGENTS.md`、`CLAUDE.md`、`README.md`、`RELAY.md`、`.gitattributes`、`.gitignore`。还没有任何代码。
- 进行中：无。
- 下一步：第 1 片 = 叶子 `O.01` 构建流水线（含 `tools/relay`），需要设计方先把 `O`、`O.01` 改为"已审"。`O.01` 的验收要用到还没实现的 `B.02`（`anydoor` 启动与组装），按 `G.02.06-R10` 写桩：只做 `/api/v1/health`、`--version`、嵌入前端与"前端还没有构建"页；前端写 `W.01` 的最小空白页桩。开发机工具：Go 1.27.4、golangci-lint 2.15.0、Node 24.19、pnpm 11.21（`tools/versions.txt` 在第 1 片里建）。注意：golangci-lint 是 v2，设计文档 `G.05.05` 的配置是按 v1 写的，写 `.golangci.yml` 前要先改文档。
- 桩：无。
- 上报单 / 阻塞：推送前等用户决定仓库公开还是私有（设计文档 `P.09` Q27）。
- relay check：不适用（还没有代码）。
- 评审：不适用。
- 提交 / 推送：见本条更新。
