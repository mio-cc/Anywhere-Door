# CI（GitHub Actions）

> 设计文档 `O.02`（CI 门禁）、`O.03`（打包）、`G.08`（门禁清单）。本页写仓库里放不下的设置、当前状态和本地复现的命令。

## 1. 合并流程（M0，`P.09` K241）
- `main` **不设分支保护**。合并按 `G.08-R3`：验证者原样贴出的 `go run ./tools/relay check` / `relay slice` 全部通过、评审记录里没有未处理的"不符合 / 有疑问"后，由验证者在本地 squash 合并（`G.07-R6`）并推送。
- CI 在推送 `main` 后做**事后检查**：下表里"已到期"的任务红了就是缺陷，下一个叶子开工前修好（`Q.12`）。没到期的任务照常运行，红了只记录。
- 以后改成合并请求流程时：在 GitHub 设置里给 `main` 加分支保护（只能经合并请求、必过检查为下表已到期的任务、只允许 squash、不允许管理员绕过），并先确认 `commitcheck` 的规则和提交写法一致（`P.09` K243 ③）。
- `ci-drill/*`：门禁演练用的分支，只推送、不合并，演练完删掉（`O.02` §10 验收 2）。
- 标签 `v*` 只在里程碑验收时由验证者按用户的决定推送（`G.07-R8`）。M0 结束推 `v0.1.0`，`release.yml` 发未签名的 `0.1.0-preview` 预发布（K241）。

## 2. 任务与到期情况
| 任务 | 运行在 | 已到期（红了是缺陷） | 备注 |
|---|---|---|---|
| `gen` | ubuntu | 是（M0） | |
| `go-test` | ubuntu、windows、macos | 是（M0） | ubuntu 加 `-race` 和覆盖率报告；覆盖率 M0 不设门槛（K243） |
| `go-lint` | ubuntu | 是（M0） | `depcheck`、`quirkscheck`、`typenamecheck` 还是桩（见 §3） |
| `web` | ubuntu、windows | 是（M0） | 前端 `lint` 只是 `tsc --noEmit`，`test` 还是桩（见 §3） |
| `build` | ubuntu、windows、macos | 是（M0） | 构建预览版并冒烟（`O.03` §6.4） |
| `security` | ubuntu | 是（M0） | gitleaks（全部历史）、govulncheck、pnpm audit、依赖清单 |
| `pr-meta` | ubuntu | 只在合并请求上运行 | M0 不走合并请求（K241） |
| `e2e` | ubuntu | 否（M1，`W.12` 落地起） | 现在调用的是桩脚本 |
| `perf` | — | 否（M1） | 还没写进 `ci.yml`：`pnpm -C web bench`、`perf` 脚本出现时由那个叶子加（K243） |

每晚（北京时间 03:00，`nightly.yml`）：在 `main` 上重跑漏洞扫描；同一提交在两个干净的运行器上各构建一次并比较 `anydoor` 的 SHA-256。失败时开一个 Issue（漏洞扫描的标签为 `security`：仓库里要先建好这个标签，否则开 Issue 那一步会失败），2 个工作日内处理。

到期变化记录（到期时在这里加一行）：
| 日期 | 变化 |
|---|---|
| 2026-11-24 | 首版：M0 的 `gen`、`go-test`、`go-lint`、`web`、`build`、`security` 到期 |

## 3. 还是桩的检查
CI 照常调用它们，桩以 0 退出（`P.09` K243）。对应叶子实施时整体替换，届时在这里删掉一行。
| 命令 | 桩所在 | 由谁替换 |
|---|---|---|
| `go run ./tools/depcheck` | `tools/depcheck/main.go` | `G.03` |
| `go run ./tools/lint/quirkscheck` | `tools/lint/quirkscheck/main.go` | `G.05.05` |
| `go run ./tools/lint/typenamecheck` | `tools/lint/typenamecheck/main.go` | `G.05.05` |
| `pnpm -C web test`、`test:e2e`、`test:visual` | `web/scripts/stub.mjs` | `W.12` |
| 前端界面 | `web/` 的空白页 | `W.01`、`W.02` |
| 覆盖率门槛 | `tools/ci/cover` 不带 `-min` | `Q.01` 复查后开（门槛见 `Q.01-R5`） |

## 4. 工具版本
- Go：`go.mod` 的 `toolchain`；Node、pnpm：`web/package.json` 的 `engines`、`packageManager`（`corepack enable`）。
- 其余：`tools/versions.txt`。两列的行（`golangci-lint`）本地和 CI 都用，`go run ./tools/build -check` 会检查本机装的版本；三列、第三列是 `ci` 的行（`gitleaks`、`govulncheck`）只在 CI 上安装（`P.09` K246）。
- 第三方 Actions 只用 `actions/*` 官方的几个，固定到提交哈希，行末注释写版本标签（`S.12-R6`）。升级时同时改哈希和注释。

## 5. 本地复现
在仓库根运行（Windows 用 Git Bash 或把命令改成 PowerShell 写法）：
| 任务 | 命令 |
|---|---|
| `gen` | `pnpm -C web install --frozen-lockfile && go run ./tools/gen && git status --porcelain`（应为空） |
| `go-test` | `go test ./... -count=1`；覆盖率：`go test ./... -count=1 -coverpkg=./... -coverprofile=cover.out && go run ./tools/ci/cover cover.out` |
| `go-lint` | `golangci-lint run ./... && go run ./tools/depcheck && go run ./tools/lint/quirkscheck && go run ./tools/lint/typenamecheck` |
| `web` | `pnpm -C web lint && pnpm -C web test && pnpm -C web build` |
| `build` | `pnpm -C web build && go run ./tools/build -preview && go run ./tools/release -smoke bin/anydoor`（Windows：`bin/anydoor.exe`） |
| `security` | `go run github.com/zricethezav/gitleaks/v8@v<版本> git --redact --verbose .`、`go run golang.org/x/vuln/cmd/govulncheck@v<版本> ./...`、`pnpm -C web audit --audit-level high --prod`、`go run ./tools/ci/deps`（版本见 `tools/versions.txt`） |
| `pr-meta` | `go run ./tools/ci/commitcheck -title "<PR 标题>" -base <目标提交> -head HEAD` 等，参数见 `O.02` 实施卡 |
| 整体 | `go run ./tools/build -check`（`make check` / `task check`）通过时，`gen`、`go-test`（本机平台）、`go-lint`、`web` 也会通过（`O.02` §7） |

## 6. 门禁演练记录（`O.02` §10 验收 2，人工验收）
推送到 `ci-drill/<名字>` 分支，每个问题一个提交，记录被哪个任务拦下。
| 日期 | 问题 | 被拦下的任务 | 结果 |
|---|---|---|---|
| （待做） | 手改生成物 | `gen` | |
| （待做） | Windows 上才失败的路径处理 | `go-test`（windows） | |
| （待做） | 前端类型错误 | `web` | |
| （待做） | 新增未登记依赖 | `security`（`tools/ci/deps`） | |
| （待做） | 提交里放一个假 API 密钥 | `security`（gitleaks） | |

以后再演练（对应的桩被替换、门槛开启、有了合并请求流程之后）：依赖方向（`depcheck`）、覆盖率门槛、前端中文字面量、提交信息、白名单越界。
