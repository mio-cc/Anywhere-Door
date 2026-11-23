# Anywhere Door · 接手须知

> 给接手这个仓库的模型（和人）。**先读完这一页再动手。** 本页只是入口；规则以设计文档为准，两者冲突时以设计文档为准，并把本页当缺陷修掉（`G.02.06-R2`）。

## 1. 这是什么
Anywhere Door：装在本机的多 Agent"积木编排台"。画布居中，人和模型用同一套操作（Op）改图；档位决定模型能改多少，急停只有人能解除；子图可以无限嵌套；运行靠事件日志续跑和回放。后端 Go（`engine/` 内核、`internal/` 产品层），前端 Vite + React + TypeScript（`web/`），打包成单个可执行文件 `anydoor`。

## 2. 两个仓库
| 仓库 | 放什么 | 本地位置 |
|---|---|---|
| `mio-cc/Anywhere-Door`（本仓库） | 代码 | `C:\Users\Administrator\Desktop\harness` |
| `mio-cc/miocode` | 设计文档（**唯一依据**） | `C:\Users\Administrator\Desktop\sj\项目树_v0.5` |

找设计文档：环境变量 `ANYDOOR_DOCS`；没设时用 `../sj/项目树_v0.5`。

## 3. 开工五步（每一棒都要做，按顺序）
1. 读完本页。
2. 运行 `go run ./tools/relay status`：谁领着什么、停在哪、有哪些桩。（`tools/relay` 写好之前：读 `RELAY.md` 最上面一条，按设计文档 `G.02.06` §5 手工检查。）
3. 读 `RELAY.md` 最上面一条：上一棒停在哪、下一步是什么。**上一棒说"做完了"不算数，以 `relay check` 的结果为准。**
4. 续做：`go run ./tools/relay start <叶子> --resume`；领新的：`go run ./tools/relay next` 选一个，`go run ./tools/relay start <叶子>`。
5. **只读生成的任务包** `.relay/tasks/<叶子>.md`，不要自己去设计树里挑文档；任务包里缺了东西就上报（设计文档 `G.02.03`）。

## 4. 硬规则（摘要；全文是设计文档 `G.02`、`G.02.05`、`G.02.06`）
- 只实现任务包里那一个叶子写明的内容；文档没写的不做，写了的不省略。
- 不改契约：`schema/`、`gen/`、`*/generated/`、事件名、错误码、Op 种类。
- 只改任务包白名单里的文件；要动别的文件、要加依赖、发现文档矛盾：停下，写上报单（放在设计仓库 `sj/反馈/`），在 `RELAY.md` 写明阻塞。
- 测试不通过就修代码；不删、不跳过、不放宽测试。
- 不改 `tools/relay/`、`tools/ci/`、`tools/build/` 来让自己的检查通过。
- 任何测试都不调用真实模型 API。
- 每完成实施卡一条就提交一次；提交信息 `类型(叶子编号): 中文描述`，正文最后一行 `Refs: <叶子编号>`。

## 5. 交付和收工
1. `go run ./tools/relay check <叶子>` 全部通过。
2. 由**另一方**验证（在开发机上跑命令、原样贴输出）和评审（对照实施卡逐条看），评审记录写在 `docs/relay/<叶子>.review.md`。自己说"测过了"不算。
3. 在 `RELAY.md` 最上面写完本棒记录（模板在它顶部）。
4. 收工或感觉上下文快用完时，先做第 3 步，再停。

## 6. 常用命令
| 做什么 | 命令（Windows 不装 make 时用 `task <目标>` 或直接执行右边） |
|---|---|
| 重新生成代码 | `make gen` / `go run ./tools/gen` |
| 开发模式 | `make dev` / `go run ./tools/dev` |
| 构建 | `make build` |
| 提交前完整检查 | `make check` / `go run ./tools/build -check` |
| 接力 | `go run ./tools/relay status` · `next` · `start` · `check` · `diff` · `stale` |

## 7. 名字
产品名 **Anywhere Door**；程序 `anydoor`（服务）、`anydoorctl`（命令行）；Go 模块 `github.com/mio-cc/anywhere-door`；环境变量前缀 `ANYDOOR_`。代码里的 `hx` 前缀（`--hx-*` 令牌、`hx_` 表名、`hx:` 本地存储键）是内部前缀，保持不变。
