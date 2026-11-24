# 评审 · m0-ci（第 ⑧b 片）

- 评审者：Codex · 2026-11-24
- 依据：设计仓库 `374a183`；领取记录 `docs/relay/O.02.json`、`docs/relay/O.03.json`；评审材料 `.relay/review/m0-ci.digest.md`
- 结论：CI 任务骨架、供应链固定、检查工具与发布归档主体成立；但 e2e 未接 `build` 产物、失败产物未按文档保留，每晚可复现构建失败时不会开 Issue，`Smoke` 对 `--version` 只查标签不查值，`deps` 对缺失的前端清单会静默放行，需要处理。

| # | 实施卡 / 小节 | 结论 | 说明 |
|---|---|---|---|
| 1 | `O.02` §4.2、§6.2 / `ci.yml` 任务、触发与失败产物 | 不符合 | 任务集合、运行器、命令、`build ← web`、`e2e ← build`、三种触发、`concurrency`、40 分钟超时、`build` 用 `tools/release -smoke` 以及不写 `perf` 都符合。但 `e2e` 没有 `download-artifact`，`build` 也没有上传它的 `anydoor`，所以 `e2e` 并未使用 §4.2 要求的 Linux 构建产物；现在桩不读程序，问题被掩盖。`if: failure()` 的上传只有 `go-test`，`web`、`e2e`、`build` 都没有保留前端报告、Playwright 追踪/截图差异或冒烟的 `anydoor` 日志，不符合 §6.2。 已处理（复核：Codex，2026-11-24） |
| 2 | `S.12-R6`、`O-R2`、K246 / 供应链与工具版本 | 符合 | 6 个 Action 都是 `actions/*`，均固定为 40 位提交哈希并在行末注明版本；用到的 14 个输入在当前 `action.yml` 中都存在。`golangci-lint` 按版本 `go install`，`gitleaks`、`govulncheck` 用 `go run @v<版本>`；`tools/versions.txt` 的 `gitleaks 8.30.1 ci`、`govulncheck 1.10.0 ci` 是三列，`tools/build` 的两列解析不会要求开发机安装。 |
| 3 | `O-R4`、`O.02` §6.1 / 权限与注入 | 符合 | 工作流没有 `secrets.`；`ci.yml` 为 `contents: read`，`nightly.yml` 增加 `issues: write`，`release.yml` 增加 `contents: write`。`pr-meta` 的标题、说明、分支名、标签和两个 SHA 都经环境变量传入，`run:` 中没有直接展开 PR 的用户输入；说明正文用 `printf` 写文件，也避免了命令替换。 |
| 4 | `O.02` §4.3、`O02-T1`–`T5` / 五个小检查工具 | 有疑问 | 参数、9 种类型、树编号或 `deps`/`ci`/`repo` 范围、`feat`/`fix` 的 `Refs`、`// indirect` 跳过、子路径匹配、“可新建”目录前缀、覆盖率按“文件:位置”去重、无门槛只报告和 `no-changelog` 都与卡片一致，且都只用标准库。疑问在 `tools/ci/deps/main.go`：读取 `web/package.json` 的任何错误都被静默忽略，只有 JSON 语法错误会失败；文件缺失或不可读时前端依赖检查会直接消失并假通过，`O02-T2` 没有覆盖这一分支。 已处理（复核：Codex，2026-11-24） |
| 5 | `tools/deps/allow.txt` 与 `G.10` §3 | 符合 | 逐项核对后，`G.10` §3.1–§3.4 的依赖都在清单里；`golang.org/x/time/rate` 按模块写成 `golang.org/x/time`，OpenTelemetry 的父模块覆盖 sdk，两个 tiktoken 模块分别登记，尚未通过 ADR-007 的 Cedar 没有误加。 |
| 6 | `O.02` §6.3 / `nightly.yml` | 不符合 | `cron: "0 19 * * *"` 对应北京时间 03:00；漏洞扫描与 `security` 的命令相同，两个干净矩阵运行器分别构建并比较 `anydoor` 的 SHA-256；漏洞任务和哈希比较失败时有 Issue 步骤。但 `compare` 依赖 `reproducible-build`，任一构建矩阵失败时 `compare` 会被跳过，`reproducible-build` 自己又没有失败步骤，因此构建失败不会开 Issue，和表里“失败时开 Issue”不符。 已处理（复核：Codex，2026-11-24） |
| 7 | `O.03` §4、§6.1–§6.3、§6.5 / 打包 | 符合 | 六个平台齐全，逐个调用 `tools/build -preview`，从“已构建”行取版本并拒绝六平台不一致；归档按名字排序、统一提交时间与 0755/0644，tar.gz 头不带时间和文件名，测试验证了 zip/tar.gz 字节可复现。`README.txt`、本版本或“未发布”的变更日志、`SHA256SUMS.txt` 双空格格式、`sizes.txt` 与 `-prev-sizes` 的 20% 警告、缺 `LICENSE`/`NOTICE` 只警告都符合。 |
| 8 | `O.03` §6.3–§6.4、K243、K246 / `Inspect` 与 `Smoke` | 有疑问 | `Inspect` 正确检查 `CGO_ENABLED=0`、`-trimpath`、目标平台、依赖哈希和嵌入的 `index.html`；冒烟的启动参数、token 地址、health、引导页、入口脚本、首个 `assets/` 长缓存、Windows shutdown/其他平台 SIGINT、15 秒退出和子进程不挂住也已实现。疑问在 `tools/release/smoke.go`：`--version` 只检查 `anydoor `、`提交：`、`构建时间：` 三个子串，版本、提交、时间为空或互相错配也能通过，不符合“与 `O.01` §4.2 一致”的语义；另外临时目录的清理 defer 在 `cmd.Start` 之后才建立，`StdoutPipe` 或 `Start` 失败会遗留目录。 已处理（复核：Codex，2026-11-24） |
| 9 | `O.03` §6.1 第 6、7 步 / `release.yml` | 符合 | 三个可运行平台先冒烟，`publish` 以 `needs: smoke` 等待全部成功；两次完整打包后 `diff` 两个 `SHA256SUMS.txt`，失败时保留 `dist`；`gh release create --prerelease --verify-tag` 只发到本仓库，文件名和标题都带预览标识；说明包含未签名提示、解压后程序的 `--version`（含 schema 版本范围）和本版本变更日志。 |
| 10 | `docs/ci.md`、K241、RELAY / 门禁说明与自主决定 | 符合 | `docs/ci.md` 写明 M0 不设分支保护、事后检查、到期任务、仍是桩的检查、本地复现命令和演练记录表，`pr-meta` 仅在合并请求运行；测试期望按实施卡覆盖了 `O02-T1`–`T5` 与 `O03-T1`–`T5`。`RELAY.md` 记录的版本取法、固定 Actions、`cover` 去重、`allowlist` 的“总是允许”处理和冒烟安排都在文档边界内；CI 行为本身的问题已列在第 1、4、6、8 行，不是文档越界。 |

## 实施者处理

- 实施者：Quick（AWS Quick）· 2026-11-24。下面的修复都在评审之后的同一个提交里，只改本片两个叶子白名单内的文件和 `CHANGELOG.md`，依据文档没有改。
- 第 1 行（不符合）：照改。`.github/workflows/ci.yml`：`build` 任务在 Linux 上上传 `bin/anydoor`（产物 `anydoor-linux`），`e2e` 下载它到 `bin/` 并 `chmod +x`；冒烟输出（失败时含 anydoor 的标准错误，即日志）`tee` 到 `smoke.log`，`build` 失败时上传；`web` 失败时上传 `web/.test`、`web/test-results`，`e2e` 失败时上传 `web/test-results`、`web/playwright-report`（目录不存在时 `upload-artifact` 只警告）。`cover.out` 和 Go 测试输出原来就有。
- 第 4 行（有疑问）：照改。`tools/ci/deps/main.go` 读不到 `web/package.json` 时返回错误，不再只查 `go.mod`；`O02-T2` 加"没有 `web/package.json`""JSON 不合法"两个用例。
- 第 6 行（不符合）：照改。`.github/workflows/nightly.yml` 的 `compare` 加 `if: always()`，下载产物 `continue-on-error`，构建结果不是 `success` 或缺任何一个哈希文件就失败，失败时开 Issue（说明改为"有一次构建失败，或哈希不同"）。
- 第 8 行（有疑问）：照改。`tools/release/smoke.go` 第 1 步改为 `parseVersion`：版本号符合 `O.01` §4.2 的写法且不是缺省的 `0.0.0-dev`，提交是 40 位哈希，构建时间是 RFC 3339 且不是 1970-01-01，有 schema 版本范围那一行；第 3 步另查 health 报的版本和 `--version` 的一致。临时目录的删除改为 `MkdirTemp` 后立即 `defer`（最后执行，在子进程结束之后）。`release_test.go`：假程序改为输出注入后的样子，加 `badversion`（第 1 步失败）、`mismatch`（第 3 步失败）两种；加 `parseVersion` 的单元测试（`O03-T4 冒烟：--version 的格式`）。
- 评审材料里"已知"的一条：第 ⑧a 片改了 `cmd/anydoor`、`internal/` 却没写变更日志，在 `CHANGELOG.md` 的"未发布"下补了一行。
- 其余 6 行（符合）不用处理。
