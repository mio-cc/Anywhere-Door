# 评审 · m0-llm（第 ⑨ 片）

- 评审者：Codex · 2026-11-24
- 依据：设计仓库 `c19d8a0`；各叶子的任务包和领取记录 `docs/relay/E.10.*.json`；评审材料 `.relay/review/m0-llm.digest.md`
- 结论：复核后全部符合。

| # | 实施卡 / 小节 | 结论 | 说明 |
|---|---|---|---|
| 1 | E.10-R2、E.10-R6、ADR-011 / 依赖与厂商特判 | 符合 | 人工检查 `engine/llm` 非测试代码没有厂商名、模型名的字符串比较；`quirkscheck` 的扫描结果与实现一致。`go.mod`/`go.sum` 未变，`tools/deps/allow.txt` 只删除了 openai-go、anthropic-sdk-go、genai 三行。 |
| 2 | E.10.01 §4.3、§7、E1001-T3 / `ParseProviders` | 符合 | 名字、重名、协议、绝对地址、路径键与前导斜杠、四种鉴权、请求头名、模型 id 重名、覆盖、超时、代理均有校验，错误写明第几个提供方和字段；`keys`、`priority`、`weight`、`fallback` 不生效并有 warn，远程 http 加鉴权也有 warn。 |
| 3 | E.10.01 §5.1 第 3 步、§6、INV-22 / `Resp.Raw` | 不符合 | `engine/llm/collect.go:154` 把 `s.tap.raw()` 原样赋给 `Resp.Raw`，`collect.go:247` 直接保留上游 SSE 的 `data`；`stream.secrets` 只在错误 `fill` 中用于脱敏。若上游在响应正文或错误回显密钥，`Raw` 会原样返回并被排查/回放保存，违反“提供方配置、错误、日志、`Raw` 里都没有密钥原文”的不变量。现有测试只验证了 `Raw` 的结构，没有覆盖该脱敏路径。已处理（复核：Codex，2026-11-24） |
| 4 | E.10.01 §5.2、E1001-T4 / 请求体覆盖 | 有疑问 | `set` 建中间对象、`delete`/`rename` 缺源静默、路径穿过非对象报 `user_input` 并带第几条、`Models` 通配过滤都符合；但 `validateOp` 只校验 `Path`，没有按同样的点分规则校验 `rename` 的 `To`（例如 `a..b` 可被接受，运行时生成空键），这是否应作为配置错误需要确认。已处理（复核：Codex，2026-11-24） |
| 5 | E.10-R5、Q.06-R2 / 测试守卫 | 符合 | `NewTransport` 在测试二进制且 `ANYDOOR_LLM_RECORD` 不为 `1` 时拦截非回环地址；回环放行；`cassette` 的 player 只回放不联网；三协议的 `contract.Run` 都执行了 `guard` 子用例。 |
| 6 | E.10.01 §6、三协议 §6 / 流不变量 | 符合 | 最后 Delta 为 `Done=true` 且带完整 `Resp`；结束标志前 EOF 映射为 `stream_truncated`/transient；结构化输出的强制工具提取后不出现在 `ToolCalls`；没有用量记 warn。上述流行为均经三协议契约及单元测试覆盖。 |
| 7 | E.10.02、E.10.04、E.10.03 §5 / 编码 | 符合 | 抽查 `testdata/encode` 与实现：chat 的 `tool_calls[].function.arguments` 是 JSON 字符串；responses 总写 `store: false`、不写 `previous_response_id`，system 进 `instructions`；messages 相邻同角色合并、工具结果在 user、`max_tokens` 有缺省、结构化输出强制工具。 |
| 8 | 三协议 §5 / 解码与结束原因 | 符合 | chat 按 `index`、responses 按 `output_index`、messages 按 `index` 拼工具分片；结束原因映射、未知事件忽略、坏 JSON 转为 `internal` 均与文档对应。 |
| 9 | E.10.08 §5、S.03-R7、Q.06-R6 / 录放与脱敏 | 有疑问 | 请求头不写、响应头只留 `Content-Type`/`Retry-After`；回放比较方法、路径和键排序后的请求体，不一致打印差异；五份 chat 和四份 responses 真实录放的状态均为 200，手写录放有文档地址，录放中未见 `mstrl_` 或认证头文本。但九份真实录放的 `note` 都是 `真实录制（go test -record）`（`engine/llm/cassette/cassette.go:256`），没有写明 Mistral 来源；按评审材料“note 写明来源和文档地址”的要求，需要确认通用 note 是否足够。已处理（复核：Codex，2026-11-24） |
| 10 | E.10.05 §4、§5 / quirks 与常用厂商 | 符合 | 内置表按文件顺序累积，能力按 quirks → 提供方 → 模型条目覆盖；`ResolveEffort` 先查映射再取最接近且同距离取较高级；错误规则第一条匹配；`vendors.yaml` 七项填名字和密钥名后通过 `ParseProviders`；新增错误码规则的协议和分类合理。 |
| 11 | E.10.08 §4 / fake | 符合 | 按调用次序取 `Step`，`Requests()` 记录每次请求（包括耗尽后的失败调用），脚本用完返回 `fake_exhausted`；`Deltas`、`Resp`、`Err` 的组合行为与交付说明一致。 |
| 12 | E.10.01 §4.6 / 自主决定边界 | 有疑问 | `contract.Suite` 比设计 §4.6 多了导出的 `Caps`、`Transport` 两个字段；交付记录已说明，字段也为测试注入所需且向后兼容，但它改变了公开接口，超出“实施卡没规定的内部实现细节”的字面范围，需设计方确认或补充设计记录。已处理（复核：Codex，2026-11-24） |
| 13 | E.10.02 / 测试密钥边界 | 符合 | 用户已明确决定把 Mistral 测试密钥直接交给验证者；录制只在临时命令环境变量中使用，未写入文件、脚本或提交；录放扫描未见密钥或认证头。 |
| 14 | E.10.01 实施卡 E1001-T1–T7 | 有疑问 | 核心接口、配置、请求组装、错误分类、守卫、能力合并测试均通过；但受第 3 行的 `Raw` 不符合和第 12 行的接口扩展问题影响，不能判定整个叶子全部符合。已处理（复核：Codex，2026-11-24） |
| 15 | E.10.05 实施卡 E1005-T1–T6 | 符合 | quirks 合并、思考强度、三层覆盖、quirkscheck、常用厂商、错误规则测试均通过；没有碰 `engine/llm/protocol/`。 |
| 16 | E.10.08 实施卡 E1008-T1–T4 | 有疑问 | 录放往返、缺记录/不一致、脱敏、fake 测试均通过；真实录制 note 的来源标注仍需按第 9 行确认。已处理（复核：Codex，2026-11-24） |
| 17 | E.10.02 实施卡 E1002-T1–T5 | 符合 | Chat 契约、流式拼接、用量、错误和编码测试通过；真实录制五个用例及三份手写录放齐全。 |
| 18 | E.10.04 实施卡 E1004-T1–T5 | 符合 | Responses 契约、流式拼接、用量、结束/错误和编码测试通过；四个真实录制及四份手写录放齐全。 |
| 19 | E.10.03 实施卡 E1003-T1–T5 | 符合 | Messages 契约、流式拼接、用量、错误和编码测试通过；八份手写录放均有文档地址。 |

## 实施者处理

- 实施者：Quick（AWS Quick）· 2026-11-24；设计方裁决见设计仓库 `P.09` K252
- 第 3 行（不符合）：已改。`Resp.Raw` 按 §5.5 的规则脱敏（`engine/llm/collect.go` 的 `redactRaw`），替换后不再是合法 JSON 时不保留 `Raw`；新增 `TestRawRedact`（`E1001-T4`），上游把收到的密钥和一个 `sk-` 串回显在响应里，断言 `Raw` 里都换成 `[REDACTED]` 且仍是合法 JSON。`E.10.01` §5.1 第 4 步已写明。
- 第 4 行（有疑问）：按配置错误处理。`rename` 的 `to` 和 `path` 用同一条规则（非空、没有空段，`provider.go` 的 `dottedPath`）；`E1001-T3` 补了 `path` 为 `a..b`、`to` 为 `b..c` 两条反例。
- 第 9 行（有疑问）：改。录制写出的 `note` 改为"真实录制（go test -record）："加请求过的地址，`E1008-T1` 断言；已录好的 9 份 `note` 手工补上 Mistral、`https://api.mistral.ai`、`mistral-small-latest`、2026-11-24，录放内容不动。`E.10.08` §5 已写明。
- 第 12 行（有疑问）：认可。契约框架要能注入 quirks 表和录放传输层，而 `E.10.08` 在 `E.10.01` 之后领取，不能直接 import；两个字段已补进 `E.10.01` §4.6。
- 第 14、16 行：随第 3、9、12 行处理。
- 修复的文件属于片内已完成的 `E.10.01`、`E.10.08`、`E.10.02`、`E.10.04`，按 K238 的做法记在 `E.10.03` 的实施卡范围里（设计文档已改）。
