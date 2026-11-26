# 评审 · m1-llm（第 ⑤a 片）

- 评审者：Codex · 2026-11-26
- 依据：设计仓库 `423e24c`（`E.10.01`、`E.10.06`、`E.10.07`、`E.10.09`、`E.10.10`、`E.15.02`；`P.09` K268）；评审材料 `.relay/review/m1-llm.digest.md` 的 15 条重点；代码 `engine/llm/{registry,provider,keys,wrap,retry,breaker}.go`、`engine/llm/structured/`、`engine/llm/tokens/`、`engine/sched/pool.go`（片内提交 `247711f`、`af27e64`、`b080bba`、`21a6549`、`a4a55ac`、`9d75fc7`、`a9f9d58`、`612802b`、`0dc4f2d`、`afcfa45`、`66dc9bc`、`0586be8`）
- 结论：15 条全部符合，没有不符合 / 有疑问，可以合并。

| # | 实施卡 / 小节 | 结论 | 说明 |
|---|---|---|---|
| 1 | `E.15.02` §5 / `Penalize` | 符合 | `pool.go:206-210` `Penalize` 只在 `p.rate != nil` 时委托 `p.rate.Penalize`，不限速的池直接返回。冻结后修改（`G.02.06-R21`）只加了这一个方法和 `E1502-T5`（`af27e64` 只动 `pool.go`、`pool_test.go`），原有代码和测试不变。 |
| 2 | `E.10.01` §5.1 / 鉴权密钥 | 符合 | `registry.go:134-143` `ClientWithKey` 先校验指定名在 `cfg.Keys` 里（否则 `key_not_found`），再经 `clientFor`（`:174-199`）把密钥名传进 `client`；`newRequest:350-353` `c.key` 非空时用指定密钥，否则写了 `keys` 用第一个；密钥仍在 `newRequest` 组请求时才经 `SecretResolver.Resolve` 现取（E.10-R4），`Client` 的请求头里是那个密钥的原文。 |
| 3 | `E.10.01` §4 / fallback 与 keys 校验 | 符合 | `provider.go:204-219`：fallback 指向自己、重复、指向不存在的提供方都报错并写位置；`provider.go:184-196`：keys 空名、重名报错；只有 `weight`（`:197-199`）以及 `auth.secret` 与 `keys` 同写、http 明文远程主机记 warn。`registry.go:156-172` `Fallbacks` 用 `slices.SortStableFunc` 按目标 `priority` 从大到小、一样大保持原顺序，返回副本。
| 4 | `E.10.07` §5 / 估算与校准 | 符合 | `tokens.go:21-30` 常数（CJK ×1.1、其余 4 字符、安全系数 1.1、消息开销 4、附件 1000、工具序列化长度 ÷3.5、比值 0.7–1.5、EWMA 0.8）与 §5 一致；`Approx`（`:50`）空串 0、`math.Ceil`；`Stats.Observe`（`:94-106`）`math.Min(maxRatio, math.Max(minRatio, r))` 夹住、持互斥锁、估算或实际 ≤ 0 忽略；`For` 的 Counter 每次计数取最新比值（`:132-137`），结果 `math.Ceil(估算 × 比值)`。 |
| 5 | `E.10.06` §5 第 4 步 / `Repair` | 符合 | `repair.go:20-47` 本身（去首尾空白后）是合法 JSON 对象 / 数组时原样返回、`repairs` 为 nil；否则依次代码块（`fenced:50`）、抽取、去注释、单引号转双引号、去尾逗号、补闭合，名字与 §4 一致。`fix`（`:67`）按字节扫：注释、单引号、`}`、`]` 前逗号、补闭合只在字符串外做（字符串内只转义和还原），用到的符号都是 ASCII，不会切进多字节字符。 |
| 6 | `E.10.06` §5 第 2 步 / `Prepare` | 符合 | `structured.go:84-118` 先 `slices.Clone` `Messages`、`Tools` 并清 `Output`，不改调用方切片；Native 设 `Output`（副本）；ForcedTool 只加 `StructuredTool` 和 `ToolChoice`、不设 `Output`；TextParse 把只输出一个 JSON 对象加紧凑 Schema 追加到第一条 system 消息末尾（没有则在最前面加一条）。 |
| 7 | `E.10.06` §5 第 5 步 / 宽松修正与校验 | 符合 | `coerce`（`structured.go:201-264`）只看 `type`、`properties`、`items`、`default`，只做数字字符串（`integerRe`/`numberRe` 按 JSON 数字写法）、单值转数组、填默认值三种，记 `number_string`、`single_to_array`、`default:<路径>`；`Decode`（`:120-199`）校验不通过返回 `format_invalid`（`CodeSchemaMismatch`），`toErrors`（`:267-313`）取叶子原因和 `InstanceLocation`（JSON Pointer 转义），最多 20 处；找不到 JSON 返回 `CodeNoJSON`。 |
| 8 | `E.10.10` §5 / KeyRing | 符合 | `keys.go:34-56` 每个提供方一个游标，从上次用过的下一个开始轮询、跳过冷却中的（过期的顺手删），都冷却时 `ok` 为 false 且 `wait` 是最早恢复的剩余时间；`Cool`（`:58-65`）已在冷却时取更晚的；`CoolFor`（`:68-80`）`rate_limited` 用 `RetryAfter`（没有时 60 秒）、`auth_denied` 10 分钟、其他 0。冷却只在内存、按 `now` 判断到期。
| 9 | `E.10.09` §5 / `OnCall` 恰好一次 | 符合 | `wrap.go:147-229` `try` 失败立刻 `record`（`:201-202`）；成功交给 `wstream`，`finish`（`:303-316`）用 `sync.Once`：`Done`（`:295-297`）、提前 `Close`（`:299-302`）各一次；熔断打开（`:187-190`）、密钥都在冷却（`:165-171`）、ctx 取消（`:154-156`、`:227`）时不发请求也不记录。`E1009-T4` 验证成功 / 失败 / 提前 Close 三条记录的 `Attempt`、`Key`、`Latency`，以及 ctx 取消时不记录、双 `Close` 只记一次。 |
| 10 | `E.10.09` §5 / 重试 | 符合 | `retry.go:14-16` `retryable` 只认 `transient`、`rate_limited`；`wrap.go:213` `retries+1 >= Retry.Max` 停，一共最多 `Max` 次真实调用，换密钥（`switches`，`:206-211`）不等待、不算重试次数；`retry.go:26-47` `rate_limited` 有 `Retry-After` 时等它，超过 `MaxRetryAfter` 返回负数（不等、当作失败），否则 `Base × 2^n` 封 `MaxDelay` 再乘抖动；`wrap.go:221` 429 退避时 `Penalize(0.8, 5m)`（常数 `:56-60`）。 |
| 11 | `E.10.09` §5 / 第一个片段之后 | 符合 | `open`（`wrap.go:231-244`）只发请求并读第一个片段；交出第一个片段后 `wstream.Next`（`:277-301`）把错误经 `asError` 原样交给调用方，不退避、不重试、不换密钥（`E1009-T1` 的断了用例：只调一次、记录一条带 `Err` 的调用）。 |
| 12 | `E.10.09` §5 / 熔断 | 符合 | `breaker.go`：窗口裁剪（`:141-145`）、`MinRequests`（`:146`）、失败率超过 `ErrorRate` 打开（`:150-155`）；`Allow`（`:99-115`）打开时拒绝、半开时只放一个试探（`probing`）；`Record`（`:119-159`）半开时成功关闭、失败再打开，`Neutral` 不计（`:138-139`，半开时放下一个试探）。`DefaultBreaker` 与 §4 的窗口 / 最少请求数 / 失败率 / 打开时长一致。 |
| 13 | `E.10.09` §5 / 故障转移 | 符合 | `failover`（`wrap.go:97-104`）只认 `transient`、`rate_limited`、`auth_denied`；`Complete`（`:106-145`）按 `Fallbacks` 顺序试，备用提供方没有这个模型（`Client` 报错）时跳过，每换一次回调一次 `OnFallback`（带原因），只转一层——备用提供方上只调 `try`，不再跟它自己的 fallback；ctx 取消时停止。
| 14 | 整片 / 依赖方向 | 符合 | 片内新增的非测试文件只 import 标准库、`gen`、`engine/llm/**` 和第三方 `jsonschema/v6`（`structured.go`）；`engine/sched` 只出现在 `wrap_test.go:14` 的测试导入里。`go run ./tools/depcheck`：48 个包没有违规；`go run ./tools/ci/deps`：直接依赖都在 `tools/deps/allow.txt` 里。 |
| 15 | 整片 / 测试没有放宽 | 符合 | 片内测试没有 `t.Skip`、`testing.Short` 或放宽断言。时间注入的 `clock`（`wrap_test.go:21-48`）把 `Sleep` 记成时长并推进时钟，不靠真实等待；`E1009-T5` 的 `time.Sleep(5ms)`（`wrap_test.go:340`）只是让 10 个并发调用重叠，断言写死峰值不超 2（`:380-382`）和排队时 `OnWait` 收到池名 `llm:p/m`（`:383-385`），不依赖时序。`E1006-T3` 用 rapid 随机验证合法输入修复后语义不变、无修复记录。 |

## 验证者处理（E.10.09）
- 验证者：Codex · 2026-11-26。`gofmt -l engine/llm/` 为空（`wrap.go` 交回时的两处格式问题已由实施者 `gofmt -w` 修好，暂存目录里的副本与工作区一致）。
- `wrap_test.go` 的 errcheck：之前 `golangci-lint` 报 3 处裸 `call(c)`（261、262、290），同类默认只报 3 条，295 行还有一处被藏。实施者已把 4 处都改成检查返回错误：261、262 改成瞬时失败两次必须返回错误（`e == nil` 时 `t.Fatal`），290 改成 `e != nil` 时 `t.Fatalf`，295 改成 UserInput 失败（`e == nil`）时 `t.Fatal`——都是收紧，没有放宽。本轮 `golangci-lint run ./... --max-same-issues=0 --max-issues-per-linter=0`：0 issues，没有再被藏起来的同类问题。
- `go vet ./engine/llm/... ./engine/sched/...`：windows/amd64、GOOS=linux、GOOS=darwin 三个平台都 exit 0。

## 合并前检查（`O.02` §4.2，2026-11-26，分支 `slice/m1-llm`）
- 整片验证 `go run ./tools/relay slice m1-llm`：6 个叶子（E.15.02、E.10.01、E.10.07、E.10.06、E.10.10、E.10.09）①–⑥、整片 ⑦（`go run ./tools/build -check`）全部通过；`E1009-T1`–`T6` 覆盖 6/6。
- 其他平台编译：`GOOS=linux`、`GOOS=darwin` 对改到的包（`engine/llm/...`、`engine/sched/...`）`go vet`，exit 0。
- 依赖清单 `go run ./tools/ci/deps`：直接依赖都在 `tools/deps/allow.txt` 里。
- 漏洞扫描 `go run golang.org/x/vuln/cmd/govulncheck@v1.10.0 ./...`：No vulnerabilities found；`pnpm -C web audit --audit-level high --prod`：No known vulnerabilities found。
- 前端审计：`pnpm -C web lint`（tsc --noEmit）在整片 ⑦ 里通过；本片没改 `cmd/`、`internal/api/`、`web/`、`tools/release/`，前端构建与冒烟按 §4.2 跳过。
- 密钥扫描 `go run github.com/zricethezav/gitleaks/v8@v8.30.1 git --redact --gitleaks-ignore-path tools/gitleaks/.gitleaksignore .`：55 commits scanned，no leaks found。

## 实施者处理
- 实施者：Quick（AWS Quick）· 2026-11-26。十五条都符合时不用处理；E.10.09 的 gofmt 和 4 处 errcheck 已按上节修好。

## 已知、不算不符合
- tiktoken 精确计数在 M5（`E1007-T1`）；回放、预算预留在 M2；`weight` 还不生效（只记 warn）；指标 `hx_llm_repairs_total` 在 M5。
- `-race` 需要本机没有的 C 编译器，留给 Linux CI（`G.05.02` §3）。
- `engine/store/sqlite` 的 `E0704-T1 并发写`（要求 <10s）在整仓负载下偶有抖动；本轮两次整仓 `go test ./...` 都通过（11.4s / 19.5s 是整个包的耗时，不是这个断言的耗时）。