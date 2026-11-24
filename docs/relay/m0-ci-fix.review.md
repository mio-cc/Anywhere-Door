# 评审 · m0-ci-fix（第 ⑧b 片修复片）

- 评审者：Codex · 2026-11-24
- 依据：设计仓库 `0ed340f`；领取记录 `docs/relay/O.02.json`；设计文档 `O.02` §1、§4.1、§4.2；五项合并前检查的本机实际输出
- 结论：工作流已删除、检查清单与设计一致、工具版本处理正确、误报忽略精确且可复现，四行均符合。

| # | 实施卡 / 小节 | 结论 | 说明 |
|---|---|---|---|
| 1 | `O.02` §1、§4.1、K248 / 工作流已删除 | 符合 | 仓库里没有 `.github/workflows/`。对受 Git 跟踪的文件（排除 `docs/relay/`、`RELAY.md`、`CHANGELOG.md`）搜索 `ci.yml`、`nightly.yml`、`release.yml`、`docs/ci.md`，只命中 `docs/checks.md` 变化记录中的历史说明，以及 `tools/relay/docs.go` 中 `.golangci.yml` 的 `.yml` 子串；没有仍在引用三个工作流或 `docs/ci.md` 的地方。 |
| 2 | `O.02` §4.2 / 检查清单 | 符合 | `docs/checks.md` 的检查项与 `O.02` §4.2 一致：整片 `relay slice`；改到的包做 Linux、macOS `go vet`；`tools/ci/deps`；`govulncheck` 与 `pnpm audit`；带 `tools/gitleaks/.gitleaksignore` 的 `gitleaks`；前端构建与冒烟；覆盖率报告。版本、失败条件和误报处理也与设计一致；本片五项检查已在开发机上实际运行并通过。 |
| 3 | `O-R2`、K246、K248 / `tools/versions.txt` | 符合 | `gitleaks 8.30.1 run`、`govulncheck 1.10.0 run` 都是三列；`tools/build/main.go` 的 `pinnedTools` 只在 `len(f) == 2` 时收入工具，所以 `build -check` 不会要求预装这两个 `go run` 工具。 |
| 4 | `O.02` §4.2、K249 / `tools/gitleaks/.gitleaksignore` | 符合 | 忽略文件只有一条指纹，精确指向 `8d3377c982ba5e870246e8213bb36bea20b11399:cmd/anydoor/run_test.go:generic-api-key:296`。`git show 8d3377c:cmd/anydoor/run_test.go` 的 296 行是 RFC 6455 §1.3 示例 `Sec-WebSocket-Key: dGhl***ZQ==`（解码为 “the sample nonce”），不是凭据。不带 `--gitleaks-ignore-path` 临时扫描正好只报这 1 条，提交、文件、规则、行号均与指纹一致；带忽略清单扫描为 `no leaks found`。 |

## 实施者处理
- 实施者：Quick（AWS Quick）· 2026-11-24。四行都符合时不用处理。
