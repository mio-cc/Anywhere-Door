# 评审 · m0-server（第 ⑧a 片）

- 评审者：Codex · 2026-11-24
- 依据：设计仓库 `adf3e79`；领取记录 `docs/relay/B.03.05.json`、`B.07.05.json`、`B.04.14.json`、`B.02.json`；评审材料 `.relay/review/m0-server.digest.md`
- 结论：中间件、鉴权与启动主链总体成立，但错误 details 与 Windows 重试上限各有一处不符合，双凭据优先级、第二实例清理 tmp 及对应测试盲区需要处理。

| # | 实施卡 / 小节 | 结论 | 说明 |
|---|---|---|---|
| 1 | B.03.05 §5 第 1 条、B.03 §3 / 中间件链顺序与范围 | 符合 | `internal/api/middleware.go` 按 `recoverWithID → securityHeaders → checkHost → checkOrigin → authenticate → limits` 包装；ID 在 recover 前生成，panic 的 500 也走统一安全头与 `X-Request-Id`。Host 对所有路径生效，Origin 覆盖非 GET 与 SSE，鉴权只覆盖 `/api/` 且 health 为唯一公开路由；`MaxBytesReader` 与 Content-Length 两条路径都限制 1MB，30 秒时限只加给 `/api/` 的非 SSE 请求。 |
| 2 | S.02-R3–R8、B.03.05 §5 第 2–7 条 / 本机鉴权 | 有疑问 | `internal/api/auth.go` 的 cookie、cookie 属性、302 去 token、Host/Origin 与 CORS 行为符合要求；但 `callerOf` 在 cookie 无效时会继续接受有效 Bearer，因此“cookie 与 Bearer 同时存在时以 cookie 为准”只在 cookie 有效时成立。`B0305-T6` 只测了两者都有效，没有覆盖“坏 cookie + 有效 Bearer”。 已处理（复核：Codex，2026-11-24） |
| 3 | S.09-R2 / `IsUser` 与 shutdown 权限 | 符合 | `internal/api/auth.go` 只把 cookie 的 `user:local` 和 Bearer 的 `external:anydoorctl` 判为用户；`internal/api/admin.go` 对非用户返回 403 且不调用 shutdown，对用户/Bearer 先写 202 再执行只调用一次的关闭函数。 |
| 4 | B.03.03、C.11 §3、G.13-R3–R5 / 错误响应 | 不符合 | `internal/api/errors.go` 的 `reasonError` 让 `bad_request` 的 `details.errors` 变成字符串数组，而 C.11 要求 `[{path,message}]`；`forbidden` 只生成 `reasons`，缺少要求的 `decision`、`action`、`target`，`admin.go` 的 shutdown 403 即受影响。404、413、500 的文案替换和卫生处理正确；`B0305-T8` 只检查了 404 details，JSON Schema 又未约束 details，所以错误形状没有被测试发现。 已处理（复核：Codex，2026-11-24） |
| 5 | B.04.14、B-R3 / health 与 shutdown | 符合 | `internal/api/health.go` 固定只返回 `{ok,version}`，公开但仍经过 Host 与安全头；`admin.go` 的两个成功请求共用同一个 `sync.OnceFunc`，重复请求仍返回 202，拒绝路径不触发关闭。 |
| 6 | B.02 §5、K242 / 启动组装、清理与退出码 | 符合 | `cmd/anydoor/run.go` 的初始化顺序为参数、数据目录、配置、锁、日志、SQLite、auth、secret、监听、API、port、打印、浏览器；清理闭包修正后按相反顺序关闭监听、数据库和锁，日志最后关闭。退出码 1/2/3、`--version` 不碰数据目录、shutdown 15 秒上限均与实施卡一致。 |
| 7 | S.02-R1、R10 / 监听与已有实例转交 | 有疑问 | `cmd/anydoor/listen.go` 只监听 `127.0.0.1` 并尽量监听同端口 `[::1]`，7300/系统端口、1 秒探活、3 秒重试、带 token 地址、`--no-browser` 和两行标准输出都正确。但 `run.go` 在 `lock.Acquire` 前调用 `datadir.Open`，后者会清空 `tmp/`；第二个实例因此会清掉正在运行实例的临时目录。RELAY 已披露此风险并称 M0 暂不使用 tmp，但单实例隔离语义仍不完整。 已处理（复核：Codex，2026-11-24） |
| 8 | B.02 §5 / 静态文件与开发代理 | 符合 | `cmd/anydoor/static.go` 对 assets 加一年 immutable 缓存、index 与普通文件 no-cache、未知前端路由回退 index、缺失 assets 404、缺 index 时显示 make build 说明页；`devproxy.go` 用 `ReverseProxy.Rewrite` 转发到 Vite，支持 WebSocket，开发模式 CSP 才额外放行 `ws://127.0.0.1:5173`。 |
| 9 | B.07.05 §4–§5、K244 / Windows 覆盖重试 | 不符合 | `internal/platform/datadir/rename_windows.go` 只对 ACCESS_DENIED/SHARING_VIOLATION 重试，Unix 不重试，最终失败会删除临时文件；但退避间隔从 64ms 直接翻倍到 128ms，超过文档“最长 100ms 一次”，且截止时间在失败尝试之后才检查，最后一次尝试可越过 2 秒。`B0705-T5` 把期限缩到 200ms 后只断言总耗时小于 2 秒，没有断言 100ms 间隔上限或接近 200ms 返回。 已处理（复核：Codex，2026-11-24） |
| 10 | 测试意图、RELAY 自主决定、依赖 | 有疑问 | 新增依赖只有已登记的 `oklog/ulid/v2` 与 `pkg/browser`；多数测试按文档构造，RELAY 的自主决定也基本在边界内。但 `B0305-T8` 未约束 400/403 details，`B0305-T6` 未覆盖坏 cookie 与有效 Bearer 并存，`B02-T2` 未实际握手 WebSocket，`B0705-T5` 的时限断言过宽，因此上述语义偏差都被绿灯掩盖。 已处理（复核：Codex，2026-11-24） |

## 实施者处理

- 实施者：Quick（AWS Quick）· 2026-11-24。下面的修复都在评审之后的同一个提交里（只改本片各叶子白名单内的文件，依据文档没有改，`relay` 的 ⑥ 不受影响）。
- 第 2 行（有疑问）：按 `P.09` K245 ③ 裁决：两种凭证各自判断，"两者都有"指两者都有效；cookie 无效时照常看 Bearer（anydoorctl 不带 cookie，浏览器不带 Bearer，不存在用坏 cookie 绕过的情形）。文档不改；`internal/api/auth_test.go` 的 `B0305-T6` 加"坏 cookie 加有效 Bearer → `external:anydoorctl`"和"有效 cookie 加坏 Bearer → 200"两个用例。
- 第 4 行（不符合）：照改。`internal/api/errors.go` 去掉 `reasonError`，改为 `badRequestError`（`details.errors` 是 `[{path, message}]`，文案的 `{errors}` 填"path：message"，用"；"连接）和 `forbiddenError`（`details` 是 `decision`、`reasons`、`action`、`target`；M0 的写法按 K245 ①）；`classify` 同时返回 `details` 和文案变量，`render` 只读文案变量。`admin.go` 的 shutdown 403 改用 `forbiddenError`。`errors_test.go` 的 `B0305-T8` 加一条 403 `forbidden` 用例，并逐个检查 400、403 `forbidden`、404 的 `details` 形状，401、403 `origin_rejected` 没有 `details`。
- 第 7 行（有疑问）：确认是缺陷，按 K245 ② 推迟：要改 `B.07.05` §5（`Open` 清空 `tmp/`）和 `B.02` §5 的顺序，M0 没有代码使用 `tmp/`；规定第一个使用 `tmp/` 的叶子开工前先修（`Dir.ClearTmp()` 在取到锁之后调用）。已记进交接说明的"留给以后的点"。
- 第 9 行（不符合）：照改。`internal/platform/datadir/datadir.go` 加 `renameMaxWait`（100ms）和纯函数 `nextWait`（翻倍、不超过 100ms）；`rename_windows.go` 每次等待取 `min(wait, 离时限还剩多少)`，到时限不再等。`B0705-T5` 加退避序列 1、2、4、…、64、100、100… 的断言；Windows 上时限 300ms、目标一直被打开时，用时必须在 [300ms, 800ms) 里。
- 第 10 行（有疑问）：前面三处测试已补（`B0305-T6`、`B0305-T8`、`B0705-T5`）；`cmd/anydoor/run_test.go` 的 `B02-T2` 加一次真实的 WebSocket 握手（假的 Vite 回 101 并回显一行，经开发代理收到 101 和回显）。
- 其余 5 行（符合）不用处理。
