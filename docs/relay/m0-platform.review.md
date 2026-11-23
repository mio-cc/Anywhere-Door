# 评审 · m0-platform（第 ⑦ 片）

- 评审者：Codex · 2026-11-24
- 依据：设计仓库 `5ca37a9`；各叶子的任务包和领取记录 `docs/relay/B.07.*.json`；评审材料 `.relay/review/m0-platform.digest.md`
- 结论：全部符合，未发现越界或行为不一致。

| # | 实施卡 / 小节 | 结论 | 说明 |
|---|---|---|---|
| 1 | B.07-R1、B.07-R4 / 平台层依赖方向 | 符合 | `auth`、`secret`、`config`、`lock` 全部只依赖 `datadir`；没有引入 `engine/**`、`internal/api` 或 `internal/app`，`auth` 用本地 `KV` 小接口，`secret` 只按 `Resolve` 的签名实现，不违反平台层边界。 |
| 2 | B.07-R5、B.07-R6 / 时间与文件 | 符合 | 会话过期和错误冷却统一走 `Options.Now`；token、密钥文件、索引和配置等都从 `datadir.Dir` 取路径，并用 `datadir.WritePrivate` 落盘，文件来源和生命周期没有绕开平台层。 |
| 3 | B.07.05 §4、§5 / `internal/platform/datadir` | 符合 | 默认位置、`Resolve` 优先级、`Open` 的 7 个子目录与 tmp 清空行为与文档一致；Windows DACL 用受保护的当前用户单 ACE（`FILE_ALL_ACCESS`）而不是 `GENERIC_ALL`，原子写失败会清理临时文件。 |
| 4 | B.07.04 §5、§8 / `internal/platform/lock` | 符合 | 锁文件在远处偏移上保持独占，文件头仍可读 PID；释放时删端口文件；`Probe` 只访问 `127.0.0.1`、不跟随跳转并限制响应体 1 KB，行为与 S.02-R10 和实施卡一致。 |
| 5 | B.07.01 §5、§8 / `internal/platform/auth` | 符合 | token 为 43 字符 `base64url` 并用常量时间比较；会话只存 sha256 和 30 天过期时间，超限挤掉最早过期的，`Rotate` 作废全部会话；KV 中没有 token 或 cookie 原值。 |
| 6 | B.07.02 §5、§8 / `internal/platform/secret` | 符合 | 密钥名规则、系统密钥库探测回退、索引不含密钥值、`last4` 截断、Argon2id + XChaCha20-Poly1305 文件头与 `check` 字段、5 次错误冷却 30 秒、开发模式环境变量均符合文档边界，没有缓存明文。 |
| 7 | B.07.03 §5、§8 / `internal/platform/config` | 符合 | 登记表逐项覆盖 G.11 §3 的 37 个路径；优先级、来源、只读项、`data.dir`、`packages.*`、未知项、环境变量解析与校验范围一致，类型和枚举没有越界。 |
| 8 | B.07.03 §5–§8 / `internal/platform/config` | 符合 | `Watch` 阻塞到 ctx 且按文件层重建，Restart 项保持旧值进入 `Pending`，坏文件不回调；`Write` 走语法树替换，能保留注释、未知项、`packages.*`，失败时不写文件。 |
| 9 | B.07.01–B.07.03、B.07.04、B.07.05 / 自主决定与测试意图 | 符合 | 各测试按文档指定的行为构造断言，不是从实现反抄；`Watch` 阻塞、`Write` 原样值、`datadir.IsPrivate/ExpandPath` 共用、`TestLockHelper` 不 `Skip` 等自主决定都落在文档或施工卡允许的边界内。 |

## 实施者处理

- 实施者：Quick（AWS Quick）· 2026-11-24
- 9 行都是"符合"，没有要处理的意见。
- 片内修过一处：`perm_windows.go` 目录 ACE 原来用 `GENERIC_ALL` 加继承标志，Windows 拆成两条，`B0705-T2` 不过；改为 `FILE_ALL_ACCESS`（`0x1F01FF`），在 `B.07.05` 提交前修好。
- 后续事项（不属于本片的缺陷）：
  - 监听 127.0.0.1、Host / Origin 校验、token 换 cookie、安全响应头、单实例转交在第 ⑧ 片（`B.02`、`B.03.05`、`B.04.14`，K239、K240）。
  - `E.10` 复查时把 `secret.ErrLocked` 映射为 `provider_unavailable`"密钥库已锁定，请先解锁"（`B.07.02` §5）。
