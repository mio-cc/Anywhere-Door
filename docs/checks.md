# 合并前检查（本机）

> 设计文档 `O.02`（CI 门禁）、`O.03`（打包）、`G.08`（门禁清单）。**不用 GitHub Actions**（用户决定 2026-11-24，`P.09` K248）：仓库里没有 `.github/workflows/`，推送不会触发任何自动运行。门禁就是验证者在开发机上原样贴出的输出（`G.08-R3`）。

## 1. 合并流程
- 每片做完：验证者在开发机上跑 §2 的全部检查，原样贴出输出；交叉评审由实施者以外的一方完成，评审记录里没有未处理的"不符合 / 有疑问"。
- 满足后由验证者在本地 squash 合并（`G.07-R6`）进 `main` 并推送，事后向用户报告合并提交号（`G.08-R3`，`P.09` K209）。
- `main` 不设分支保护，不走合并请求（K241）。
- 标签 `v*` 只在里程碑验收时由验证者按用户的决定推送（`G.07-R8`）；M0 结束时预览版怎么发待定（`P.09` Q28）。

## 2. 检查清单（`O.02` §4.2）
在仓库根运行；工具版本见 `tools/versions.txt`（Windows 上用 PowerShell 时把 `$v` 换成版本号）。
| 检查 | 命令 | 什么时候 |
|---|---|---|
| 整片验证 | `go run ./tools/relay slice <片名>`（含 ⑦ `go run ./tools/build -check`：生成物一致、golangci-lint、依赖方向等桩、前端 lint、`go test ./...`、前端测试） | 每片 |
| 其他平台的编译检查 | 改到的包各做一次 `GOOS=linux`、`GOOS=darwin` 的 `go vet` | 每片 |
| 依赖清单 | `go run ./tools/ci/deps` | 每片 |
| 漏洞扫描 | `go run golang.org/x/vuln/cmd/govulncheck@v<版本> ./...`、`pnpm -C web audit --audit-level high --prod` | 每片（有修复版本的高危漏洞算失败，`S.12-R2`） |
| 密钥扫描 | `go run github.com/zricethezav/gitleaks/v8@v<版本> git --redact --gitleaks-ignore-path tools/gitleaks/.gitleaksignore .`（全部历史） | 每片 |
| 前端构建与冒烟 | `pnpm -C web build`、`go run ./tools/build -preview`、`go run ./tools/release -smoke bin/anydoor.exe` | 片里改了 `cmd/`、`internal/api/`、`web/`、`tools/release/` 时 |
| 覆盖率报告 | `go test ./... -count=1 -coverpkg=./... -coverprofile=cover.out`，再 `go run ./tools/ci/cover cover.out` | 需要时（M0 不设门槛，K243） |

密钥扫描有发现时：用 `--redact --report-format json --report-path <文件>` 导出报告（不贴密钥原文），交给设计方判断。真密钥按安全事件处理（作废该密钥、告诉用户），不靠改历史了事；确认是误报的，在设计文档 `P.09` 登记，把指纹加进 `tools/gitleaks/.gitleaksignore`（上一行注释写原因和编号），不改冻结叶子的文件、不改已推送的历史（K249）。

以后加进来：端到端（`W.12`）、前端性能基准（`W.11`），在对应叶子落地时加一行（M1）。

## 3. 还是桩的检查
`go run ./tools/build -check` 照常调用它们，桩以 0 退出（`P.09` K243）。对应叶子实施时整体替换，届时在这里删掉一行。
| 命令 | 桩所在 | 由谁替换 |
|---|---|---|
| `pnpm -C web test:e2e`、`test:visual` | `web/scripts/stub.mjs` | `W.12`（第 ⑫ 片；单元测试与 `pnpm -C web test` 已在第 ⑧ 片落地） |
| 前端界面 | `web/` 的空白页（`index.html`、`src/main.ts`） | `W.02` |
| 覆盖率门槛 | `tools/ci/cover` 不带 `-min` | `Q.01` 复查后开（门槛见 `Q.01-R5`） |

## 4. 工具版本
- Go：`go.mod` 的 `toolchain`；Node、pnpm：`web/package.json` 的 `engines`、`packageManager`。
- 其余：`tools/versions.txt`。两列的行（`golangci-lint`）要装在本机，`go run ./tools/build -check` 会检查版本；三列、第三列是 `run` 的行（`gitleaks`、`govulncheck`）用 `go run <模块>@v<版本>` 运行，不用预装（`P.09` K246、K248）。

## 5. 不在开发机上测的
开发机是 Windows：Linux、macOS 只做编译检查（`go vet`），不跑测试；没有每晚任务，可复现构建在发布时由 `tools/release` 两遍打包比较（`O.03` §6.1）。要恢复 GitHub Actions 时先在 `P.09` 登记、改 `O.02`。

## 6. 变化记录
| 日期 | 变化 |
|---|---|
| 2026-11-24 | 首版（原 `docs/ci.md`）：不用 GitHub Actions，删掉 `ci.yml`、`nightly.yml`、`release.yml`（`P.09` K248） |
| 2026-11-24 | 第一次跑：gitleaks 把测试里 RFC 6455 的示例 WebSocket 密钥报成 API 密钥，确认误报，加 `tools/gitleaks/.gitleaksignore`（`P.09` K249） |
| 2026-11-25 | `tools/lint/typenamecheck` 由 `G.05.05.01` 实现，§3 删掉这一行（`P.09` K256） |
| 2026-11-26 | `W.12` 单元测试与前端 lint 落地，替换 `web/package.json`、`tsconfig.json`、`scripts/stub.mjs` 的单元测试桩（`P.09` K300） |
