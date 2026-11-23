/* 由 tools/gen 生成，请勿手改 */

// 来源：schema/*.schema.json（C.14）。字段名是 schema 的 snake_case；前端只经 lib/protocol 使用（W.04.04-R1）。

/** 授权动作名（C.09 §4.2），43 个 */
export type Action = "graph.read" | "run.read" | "trace.read" | "op.add_node" | "op.remove_nodes" | "op.restore_nodes" | "op.connect" | "op.disconnect" | "op.set_param" | "op.promote_param" | "op.demote_param" | "op.set_label" | "op.set_tags" | "op.group" | "op.ungroup" | "op.set_subgraph_ref" | "op.set_graph_title" | "op.set_graph_meta" | "op.create_branch" | "op.delete_branch" | "op.merge_branch" | "op.revert_op" | "op.revert_to" | "op.view" | "op.set_lock" | "op.migrate" | "res.edit_rolecard" | "res.edit_recipe" | "res.switch_model" | "res.define_type" | "authz.set_mandate" | "authz.request_elevation" | "run.start" | "run.pause" | "run.cancel" | "orch.spawn" | "orch.self_modify" | "tool.L0" | "tool.L1" | "tool.L2" | "tool.L3" | "estop.trigger" | "estop.release";

export interface Actor {
  id: string;
  /** model 专用：模型实例，如 run:<run_id>/<node_id> 或 chat:<session_id> */
  instance_id?: string;
  kind: ActorKind;
  tags?: string[];
}

export type ActorKind = "user" | "model" | "runtime" | "external";

/** ActorKind 的取值（C.14-R6） */
export const ActorKind = {
  User: "user",
  Model: "model",
  Runtime: "runtime",
  External: "external",
} as const;

/** API 错误码（C.11 §3） */
export type ApiErrorCode = "bad_request" | "unauthorized" | "forbidden" | "node_locked" | "origin_rejected" | "not_found" | "graph_conflict" | "idempotency_mismatch" | "proposal_stale" | "approval_closed" | "run_state" | "too_large" | "invalid_graph" | "extension_invalid" | "estopped" | "rate_limited" | "internal" | "provider_error" | "provider_unavailable";

/** 两个端口能不能连的结果（C.04.02 §3） */
export interface Assign {
  /** C7：对列表的每一项各执行一次下游节点 */
  expand?: boolean;
  /** C4：迁移函数名，写作 <名称>@<旧主版本>→<新主版本> */
  migrate?: string;
  ok: boolean;
  /** 中文提示，直接显示；可连时不写 */
  reason?: string;
  rule: "C0" | "C1" | "C2" | "C3" | "C4" | "C5" | "C6" | "C7" | "C8" | "C9" | "C10";
}

/** 把没填的可选字段设为 schema 的默认值（C.14 §3）。 */
export function withDefaultsAssign(v: Assign): Assign {
  return { expand: false, ...v };
}

/** 已修改卡片 */
export interface BlockApplied {
  graph_id: Ulid;
  kind: "applied";
  op_ids: Ulid[];
  summary: string;
  version: number;
}

/** 审批卡片；审批类型字段叫 approval_kind（P.09 K229） */
export interface BlockApproval {
  approval_id: Ulid;
  /** 同 approval.requested 的 kind */
  approval_kind: "tool" | "run" | "elevation" | "taint" | "budget" | "human_node";
  kind: "approval";
  summary: string;
}

/** 错误提示 */
export interface BlockError {
  code: string;
  kind: "error";
  message: string;
}

/** 看这里：画布进入对应层并高亮节点 */
export interface BlockFocus {
  graph_id: Ulid;
  kind: "focus";
  node_ids: Ulid[];
  scope: string;
}

/** 已创建 / 已打开图 */
export interface BlockGraph {
  graph_id: Ulid;
  kind: "graph";
  version: number;
}

/** 提议卡片 */
export interface BlockProposal {
  /** 新增、修改、删除各几处 */
  counts: BlockProposalCounts;
  kind: "proposal";
  proposal_id: Ulid;
  summary: string;
}

/** 新增、修改、删除各几处 */
export interface BlockProposalCounts {
  add: number;
  modify: number;
  remove: number;
}

/** 运行卡片 */
export interface BlockRun {
  kind: "run";
  run_id: Ulid;
  status: RunStatus;
}

export type Branch = string;

/** 预算（C.09 §3） */
export interface Budget {
  calls?: number;
  cost_micros?: number;
  time_ms?: number;
  tokens?: number;
}

/** 8 个显示类别（C.04.01 §3），决定端口和连线的颜色 */
export type Category = "any" | "ctx" | "ref" | "list" | "tool" | "gen" | "ctrl" | "state";

/** Category 的取值（C.14-R6） */
export const Category = {
  Any: "any",
  Ctx: "ctx",
  Ref: "ref",
  List: "list",
  Tool: "tool",
  Gen: "gen",
  Ctrl: "ctrl",
  State: "state",
} as const;

/** C.06.02 §7（P 级） */
export interface DataApprovalRequested {
  approval_id: Ulid;
  detail_ref: WsRef;
  kind: "tool" | "run" | "elevation" | "taint" | "budget" | "human_node";
  options: unknown;
  summary: unknown;
}

/** C.06.02 §7；outcome 含人工节点自定义出口，不写枚举（P 级） */
export interface DataApprovalResolved {
  approval_id: Ulid;
  note?: unknown;
  outcome: unknown;
  scope: "once" | "always";
  value_ref?: WsRef;
}

/** C.06.03 §2（P 级） */
export interface DataBranchUpdated {
  ahead: number;
  behind: number;
  head: number;
  name: unknown;
}

/** C.06.02 §7（P 级） */
export interface DataBriefUpdated {
  conclusion: unknown;
  cost: unknown;
  criteria_check: unknown;
  goal_id: Ulid;
  status: unknown;
  superseded: boolean;
}

/** C.06.02 §7（P 级） */
export interface DataBudgetCharged {
  cost_micros: number;
  scope_path: unknown;
  tokens: number;
}

/** C.06.02 §7（P 级） */
export interface DataBudgetExceeded {
  scope_path: unknown;
}

/** C.06.02 §7（P 级） */
export interface DataBudgetWarn {
  ratio: number;
  scope_path: unknown;
}

/** C.06.04 §2（E 级） */
export interface DataChatStatus {
  note?: unknown;
  state: "idle" | "thinking" | "tool" | "editing" | "waiting" | "stopped";
}

/** C.06.02 §4（P 级） */
export interface DataCheckpointSaved {
  ref: WsRef;
  round?: number;
  scope_path: unknown;
}

/** C.06.02 §5（P 级） */
export interface DataCtxAssembled {
  budget_tokens: number;
  dropped: unknown;
  sections: unknown;
}

/** C.06.02 §8；画布上的光点动画（E 级） */
export interface DataEdgeFlow {
  edge_id: Ulid;
}

/** C.06.02 §2；状态机拒绝了一次转换（P 级） */
export interface DataEngineIllegalTransition {
  cause: unknown;
  from: unknown;
  loc?: Loc;
  to: unknown;
}

/** C.06.02 §7（P 级） */
export interface DataEscalationMerged {
  count: number;
  escalation_id: Ulid;
  into: unknown;
}

/** C.06.02 §7（P 级） */
export interface DataEscalationRaised {
  escalation_id: Ulid;
  kind: unknown;
  merge_key: unknown;
  options: unknown;
  question: unknown;
  severity: unknown;
}

/** C.06.02 §7（P 级） */
export interface DataEscalationResolved {
  escalation_id: Ulid;
  outcome: unknown;
}

/** C.06.02 §7（P 级） */
export interface DataEscalationRouted {
  escalation_id: Ulid;
  to_path: unknown;
}

/** C.06.02 §7；actor 必须是 user（INV-10，E 层检查）（P 级） */
export interface DataEstopReleased {
  scope: "global" | "run";
}

/** C.06.02 §7（P 级） */
export interface DataEstopTriggered {
  scope: "global" | "run";
}

/** C.06.02 §4（P 级） */
export interface DataLedgerChanged {
  by: unknown;
  path: unknown;
  store: unknown;
  version: number;
}

/** C.06.02 §5（P 级） */
export interface DataLlmCall {
  cost_micros: number;
  latency_ms: number;
  model: unknown;
  pool: unknown;
  provider: unknown;
  req_ref: WsRef;
  resp_ref: WsRef;
  strategy: "native_schema" | "tool_call" | "text_repair";
  usage: unknown;
}

/** C.06.02 §5（P 级） */
export interface DataLlmFallback {
  from_strategy: unknown;
  reason: unknown;
  to_strategy: unknown;
}

/** C.06.02 §4（P 级） */
export interface DataLoopRound {
  carry_ref: WsRef;
  exit?: boolean;
  max: number;
  round: number;
}

/** 把没填的可选字段设为 schema 的默认值（C.14 §3）。 */
export function withDefaultsDataLoopRound(v: DataLoopRound): DataLoopRound {
  return { exit: false, ...v };
}

/** C.06.03 §2（P 级） */
export interface DataMandateChanged {
  anchor: unknown;
  cause: "user" | "rule";
  level: unknown;
  mandate_id: Ulid;
  superseded_id?: Ulid;
}

/** C.06.04 §2（P 级） */
export interface DataMessageBlockAdded {
  block: MessageBlock;
  message_id: Ulid;
}

/** C.06.04 §2（P 级） */
export interface DataMessageCompleted {
  content_ref?: WsRef;
  message_id: Ulid;
  usage?: unknown;
}

/** C.06.04 §2；助手消息此时 status=streaming（P 级） */
export interface DataMessageCreated {
  message: Message;
}

/** C.06.04 §2；不落库（R2）（E 级） */
export interface DataMessageDelta {
  delta: unknown;
  message_id: Ulid;
}

/** C.06.04 §2（P 级） */
export interface DataMessageFailed {
  error: MessageError;
  message_id: Ulid;
}

/** C.06.03 §2；驱动“模型正在编辑…”提示和 AI 光标（E 级） */
export interface DataModelActivity {
  actor: Actor;
  state: "idle" | "thinking" | "editing" | "waiting" | "stopped";
  target?: unknown;
}

/** C.06.02 §3；流式输出片段（E 级） */
export interface DataNodeChunk {
  delta: unknown;
  port: unknown;
}

/** C.06.02 §3（P 级） */
export interface DataNodeInput {
  port: unknown;
  value_ref: WsRef;
}

/** C.06.02 §3；info 及以上落库（P 级） */
export interface DataNodeLog {
  kv: unknown;
  level: unknown;
  msg: unknown;
}

/** C.06.02 §3（P 级） */
export interface DataNodeOutput {
  partial: boolean;
  port: unknown;
  value_ref: WsRef;
}

/** C.06.02 §3（S 级） */
export interface DataNodeProgress {
  note?: unknown;
  pct?: number;
  sub?: unknown;
}

/** C.06.02 §3；唯一的节点状态事件，按 to 附加的字段都可选（C.06.01-R7）（P 级） */
export interface DataNodeState {
  active_outs?: unknown;
  approval_id?: Ulid;
  attempt: number;
  backoff_ms?: number;
  cache_key?: unknown;
  cause: unknown;
  cost?: unknown;
  error?: NodeError;
  from: NodeState;
  idem_key?: unknown;
  inputs_ref?: WsRef;
  outputs_ref?: WsRef;
  partial?: boolean;
  reason?: unknown;
  to: NodeState;
}

/** 把没填的可选字段设为 schema 的默认值（C.14 §3）。 */
export function withDefaultsDataNodeState(v: DataNodeState): DataNodeState {
  return { partial: false, ...v };
}

/** C.06.02 §3；在资源池排队（S 级） */
export interface DataNodeThrottled {
  pool: unknown;
  waiting: number;
}

/** C.06.03 §2（P 级） */
export interface DataOpApplied {
  op: Op;
  version: number;
}

/** C.06.03 §2、R4：to 是合并后的完整新 payload（P 级） */
export interface DataOpCoalesced {
  op_id: Ulid;
  to: unknown;
  version: number;
}

/** C.06.02 §6；回放时据此复现（INV-13）（P 级） */
export interface DataOrchExpanded {
  graph_version_after: number;
  graph_version_before: number;
  patch_id: Ulid;
  patch_ref: WsRef;
  subgraph_ref: WsRef;
}

/** C.06.02 §6（P 级） */
export interface DataOrchFinished {
  result: "patch" | "proposal" | "none";
  stats: unknown;
}

/** C.06.02 §6（P 级） */
export interface DataOrchStarted {
  mode: "design" | "runtime" | "post_run";
  orch_path: unknown;
}

/** C.06.02 §6；驱动画布上的 AI 光标（E 级） */
export interface DataOrchToolCall {
  ok: boolean;
  target_node?: unknown;
  tool: unknown;
}

/** C.06.02 §8（E 级） */
export interface DataPoolUsage {
  pools: unknown;
}

/** C.06.03 §2、R3（E 级） */
export interface DataPresence {
  actor: Actor;
  cursor?: unknown;
  editing: unknown;
  /** 用户当前所在的层 */
  scope?: string;
}

/** C.06.02 §5（P 级） */
export interface DataPromptRendered {
  ref: WsRef;
  segments: unknown;
}

/** C.06.03 §2（P 级） */
export interface DataProposalCreated {
  by: unknown;
  patch_ref: WsRef;
  proposal_id: Ulid;
  summary: unknown;
}

/** C.06.03 §2（P 级） */
export interface DataProposalDecided {
  accepted_op_ids: Ulid[];
  proposal_id: Ulid;
  rejected_op_ids: Ulid[];
  status: unknown;
}

/** C.06.03 §2（P 级） */
export interface DataProposalUpdated {
  base_version: number;
  proposal_id: Ulid;
  status: unknown;
}

/** C.06.02 §2（P 级） */
export interface DataRunCancelRequested {
  by: unknown;
  reason: "user" | "estop" | "budget" | "timeout";
}

/** C.06.02 §2（P 级） */
export interface DataRunCreated {
  branch: unknown;
  budget: unknown;
  graph_id: Ulid;
  graph_version: number;
  inputs_ref: WsRef;
  registry_hash: unknown;
  snapshot_ref: WsRef;
}

/** C.06.02 §2；安全点改图（P 级） */
export interface DataRunGraphSwapped {
  at_round?: number;
  from_version: number;
  op_ids: Ulid[];
  scope_path: unknown;
  source: "user" | "model";
  to_version: number;
}

/** C.06.02 §2；to 取值见 C.07 §2（P 级） */
export interface DataRunStatus {
  at_safepoint?: boolean;
  cause: unknown;
  cost?: unknown;
  from: RunStatus;
  to: RunStatus;
}

/** 把没填的可选字段设为 schema 的默认值（C.14 §3）。 */
export function withDefaultsDataRunStatus(v: DataRunStatus): DataRunStatus {
  return { at_safepoint: false, ...v };
}

/** C.06.04 §2（P 级） */
export interface DataSessionGraphAttached {
  graph_id: Ulid;
  title: unknown;
}

/** C.06.04 §2（P 级） */
export interface DataSessionGraphDetached {
  graph_id: Ulid;
}

/** C.06.04 §2（P 级） */
export interface DataSessionUpdated {
  active_graph_id?: Ulid;
  pinned?: boolean;
  title?: unknown;
  updated_at: Timestamp;
}

/** 把没填的可选字段设为 schema 的默认值（C.14 §3）。 */
export function withDefaultsDataSessionUpdated(v: DataSessionUpdated): DataSessionUpdated {
  return { pinned: false, ...v };
}

/** C.06.02 §5（P 级） */
export interface DataToolCall {
  idem_key: unknown;
  latency_ms: number;
  queued_ms: number;
  req_ref: WsRef;
  result_ref: WsRef;
  risk: unknown;
  status: "ok" | "error" | "denied" | "deduplicated";
  taint: boolean;
  tool: unknown;
}

/** C.06.02 §5；调用前落库（INV-14）（P 级） */
export interface DataToolIntent {
  idem_key: unknown;
  req_ref: WsRef;
  risk: unknown;
  tool: unknown;
}

/** C.06.03 §2（E 级） */
export interface DataValidate {
  diags: unknown;
  version: number;
}

export interface Diag {
  code: DiagCode;
  level: DiagLevel;
  /** 中文，指出问题和改法 */
  msg: string;
  node?: Ulid;
  param?: string;
  port?: string;
  scope?: string;
}

/** 图校验诊断码（C.11 §5） */
export type DiagCode = "V01" | "V02" | "V03" | "V04" | "V05" | "V06" | "V07" | "V08" | "V09" | "V10" | "V11" | "V12" | "V13" | "V14" | "V15" | "V16" | "V17" | "V18";

/** DiagCode 的取值（C.14-R6） */
export const DiagCode = {
  V01: "V01",
  V02: "V02",
  V03: "V03",
  V04: "V04",
  V05: "V05",
  V06: "V06",
  V07: "V07",
  V08: "V08",
  V09: "V09",
  V10: "V10",
  V11: "V11",
  V12: "V12",
  V13: "V13",
  V14: "V14",
  V15: "V15",
  V16: "V16",
  V17: "V17",
  V18: "V18",
} as const;

export type DiagLevel = "error" | "warn";

/** DiagLevel 的取值（C.14-R6） */
export const DiagLevel = {
  Error: "error",
  Warn: "warn",
} as const;

/** 从一个参数生成输入端口（C.03.05-R4）：文本参数按 {{变量}} 抽取；schema_from 对象参数按 C.03.02 §5 每个子参数一个 */
export interface DynamicPorts {
  from_param: string;
  max?: number;
  /** 只用于文本变量；schema_from 展开的端口一律 任意 */
  type?: TypeRef;
}

/** 把没填的可选字段设为 schema 的默认值（C.14 §3）。 */
export function withDefaultsDynamicPorts(v: DynamicPorts): DynamicPorts {
  return { max: 8, type: "任意", ...v };
}

/** 连线（C.02.03） */
export interface Edge {
  /** 输出端口 */
  from: PortRef;
  id: Ulid;
  label?: string;
  origin?: Origin;
  /** 输入端口 */
  to: PortRef;
}

export interface ErrorResponse {
  error: ErrorResponseError;
}

export interface ErrorResponseError {
  code: ApiErrorCode;
  /** 按 code 不同，见 C.11 §3 表 */
  details?: { [key: string]: unknown };
  /** 给用户看的中文，一句话说清楚发生了什么、怎么办 */
  message: string;
  request_id: Ulid;
}

/** 所有事件共用的外层结构（C.06.01 §2）；三条流各自的 schema 按 kind 约束 data 和 seq（R6–R8） */
export interface Event {
  actor: Actor;
  /** 引起这条事件的上一条事件的 seq（同一流内），用于因果追踪 */
  cause_seq?: number;
  /** 按 kind 判别（R6） */
  data: { [key: string]: unknown };
  /** 图协同流必填 */
  graph_id?: Ulid;
  /** 事件名，按流各自的枚举约束 */
  kind: string;
  /** 节点级事件必填，见 C.06.05 */
  loc?: Loc;
  /** 运行流必填 */
  run_id?: Ulid;
  /** 流内单调递增；SSE 帧的 id。P、S 级从 1 开始；E 级事件不占序号，值为 0（R8） */
  seq: number;
  /** 会话流必填 */
  session_id?: Ulid;
  ts: Timestamp;
}

/** 有效期（C.09 §3、R4）：until 要写 until_at、run 要写 run_id，每种只写自己的那个字段，session、forever 两个都不写 */
export interface Expiry {
  kind: "run" | "session" | "forever" | "until";
  run_id?: Ulid;
  until_at?: Timestamp;
}

/** 节点失败类别（C.11 §4） */
export type FailureClass = "transient" | "rate_limited" | "format_invalid" | "budget_exceeded" | "auth_denied" | "user_input" | "structural" | "timeout" | "cancelled" | "internal";

/** FailureClass 的取值（C.14-R6） */
export const FailureClass = {
  Transient: "transient",
  RateLimited: "rate_limited",
  FormatInvalid: "format_invalid",
  BudgetExceeded: "budget_exceeded",
  AuthDenied: "auth_denied",
  UserInput: "user_input",
  Structural: "structural",
  Timeout: "timeout",
  Cancelled: "cancelled",
  Internal: "internal",
} as const;

export interface Field {
  append_only?: boolean;
  enum?: string[];
  /** type 为 object 或 列表[object] 时的成员字段 */
  fields?: Field[];
  /** 沿数据流向下游强制透传，配方不能排除（E.13.06） */
  inherit?: boolean;
  label?: string;
  max_chars?: number;
  name: string;
  /** 写入账本时只有这些节点标签能改 */
  readonly_except?: string[];
  require_reason?: boolean;
  required?: boolean;
  /** 第几个次版本加入的字段 */
  since_minor?: number;
  /** string | integer | number | boolean | object | TypeRef | 列表[…]；object 和 列表[object] 用 fields 写成员 */
  type: string;
}

/** 把没填的可选字段设为 schema 的默认值（C.14 §3）。 */
export function withDefaultsField(v: Field): Field {
  return { append_only: false, inherit: false, require_reason: false, required: false, ...v };
}

/** 预览里一个受影响的元素 */
export interface GhostMark {
  ghost: "add" | "modify" | "remove";
  /** modify 时的新值 */
  new?: unknown;
  /** modify 时的旧值 */
  old?: unknown;
  op_id: Ulid;
  reason?: string;
  target: GhostMarkTarget;
}

export interface GhostMarkTarget {
  edge?: Ulid;
  node?: Ulid;
  note?: Ulid;
  param?: string;
}

/** 图文档（C.02）：纯数据，不含运行状态、密钥和视口位置 */
export interface Graph {
  branch: string;
  id: Ulid;
  mandates?: MandateMount[];
  meta?: Meta;
  packages?: string[];
  root: Subgraph;
  schema_version: "1.0";
  title?: string;
  version: number;
}

/** 把没填的可选字段设为 schema 的默认值（C.14 §3）。 */
export function withDefaultsGraph(v: Graph): Graph {
  return { title: "", ...v };
}

export interface GraphEventCommon {
  actor: Actor;
  /** 引起这条事件的上一条事件的 seq（同一流内），用于因果追踪 */
  cause_seq?: number;
  /** 图协同流必填 */
  graph_id: Ulid;
  /** 节点级事件必填，见 C.06.05 */
  loc?: Loc;
  /** 运行流必填 */
  run_id?: Ulid;
  /** 流内单调递增；SSE 帧的 id。P、S 级从 1 开始；E 级事件不占序号，值为 0（R8） */
  seq: number;
  /** 会话流必填 */
  session_id?: Ulid;
  ts: Timestamp;
}

export type GraphEvent = GraphEventCommon & (
  | { kind: "branch.updated"; data: DataBranchUpdated }
  | { kind: "mandate.changed"; data: DataMandateChanged }
  | { kind: "model.activity"; data: DataModelActivity }
  | { kind: "op.applied"; data: DataOpApplied }
  | { kind: "op.coalesced"; data: DataOpCoalesced }
  | { kind: "presence"; data: DataPresence }
  | { kind: "proposal.created"; data: DataProposalCreated }
  | { kind: "proposal.decided"; data: DataProposalDecided }
  | { kind: "proposal.updated"; data: DataProposalUpdated }
  | { kind: "validate"; data: DataValidate }
);

export type GraphEventKind = "op.applied" | "op.coalesced" | "branch.updated" | "proposal.created" | "proposal.updated" | "proposal.decided" | "mandate.changed" | "validate" | "presence" | "model.activity";

export interface GraphPatch {
  base_version: number;
  branch?: Branch;
  expected_effect?: GraphPatchExpectedEffect;
  /** 规范化 ops 的哈希；服务器计算，用于发现重复提议 */
  fingerprint?: string;
  /** 服务器收到后填写 */
  id?: Ulid;
  ops: PatchOp[];
  origin: "design" | "runtime" | "post_run";
  parent_patch?: Ulid;
  /** 整体理由，给用户看 */
  rationale: string;
  /** 主要作用的子图 */
  scope: string;
}

/** 把没填的可选字段设为 schema 的默认值（C.14 §3）。 */
export function withDefaultsGraphPatch(v: GraphPatch): GraphPatch {
  return { branch: "main", ...v };
}

export interface GraphPatchExpectedEffect {
  cost?: string;
  latency?: string;
  quality?: string;
  risk?: string;
}

/** 执行方式（C.03.01 §4、C.03.04 §3）。按 type 判别：字段由对应的 Impl* 定义封闭；判别用 if/then 而不是 oneOf，type 写错时只在 /type 报一处错 */
export type Impl = ImplBuiltin | ImplHttp | ImplMcp | ImplPrompt | ImplSubgraph;

/** Go 执行器：原子节点，以及编译进程序的编程式扩展节点（C.03.04-R6） */
export interface ImplBuiltin {
  /** 编程式扩展节点的注册名；原子节点不写 */
  entry?: string;
  type: "builtin";
}

/** 一次 HTTP 调用，展开为一个工具节点（C.03.04 §3.2） */
export interface ImplHttp {
  /** 请求体，任意 JSON；null 表示没有 */
  body?: unknown;
  /** 可以用 {{secret:密钥名}}（C.03.04-R4） */
  headers?: { [key: string]: string };
  method: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  response?: ImplHttpResponse;
  /** 风险等级（S.04） */
  risk: "L0" | "L1" | "L2" | "L3";
  timeout_ms?: number;
  type: "http";
  /** 可以用 {{参数名}}；查询参数里可以用 {{secret:密钥名}} */
  url: string;
}

export interface ImplHttpResponse {
  /** JSONPath，取出的值作为 result */
  map?: string;
  result_type?: TypeRef;
}

/** 调用一个 MCP 工具，展开为一个工具节点（C.03.04 §3.3） */
export interface ImplMcp {
  /** 参数映射，值可以引用 {{参数名}}、{{inputs.端口名}} */
  args?: { [key: string]: string };
  /** 必须不低于服务器声明的风险等级；服务器没声明时必填，由加载时检查（E.04） */
  risk?: "L0" | "L1" | "L2" | "L3";
  /** 设置里配置的 MCP 服务器名 */
  server: string;
  tool: string;
  type: "mcp";
}

/** 一次模型调用，展开为 上下文装配 → Agent（C.03.04 §3.1） */
export interface ImplPrompt {
  output_schema: TypeRef;
  /** 配方（C.13.01）；不写时用单输入直传的内置配方 */
  recipe?: string;
  /** 角色卡（C.13.02） */
  rolecard?: string;
  /** 没有角色卡时用的提示词模板，变量写 {{参数名}}、{{inputs.端口名}}；不能引用密钥（C.03.04-R4） */
  template?: string;
  type: "prompt";
}

/** 把子图库里的一段子图打包成节点类型，展开为一个子图引用节点，pin 固定为 snapshot（C.03.04 §3.4） */
export interface ImplSubgraph {
  /** 节点参数名 → 子图内部的参数 */
  expose?: { [key: string]: ImplSubgraphExposeValue };
  ref: ImplSubgraphRef;
  type: "subgraph";
}

export interface ImplSubgraphExposeValue {
  node: Ulid;
  param: string;
}

export interface ImplSubgraphRef {
  name: string;
  version: string;
}

export type ImplType = "builtin" | "prompt" | "http" | "mcp" | "subgraph";

/** ImplType 的取值（C.14-R6） */
export const ImplType = {
  Builtin: "builtin",
  Prompt: "prompt",
  Http: "http",
  Mcp: "mcp",
  Subgraph: "subgraph",
} as const;

/** 一层的迭代号（C.06.05 §3） */
export interface Iter {
  /** 第几次尝试 */
  attempt?: number;
  /** 事件模式循环第几次唤醒 */
  event?: number;
  /** 扇出第几份，或 C7 展开第几项 */
  fan?: number;
  /** 循环第几轮 */
  round?: number;
}

/** 把没填的可选字段设为 schema 的默认值（C.14 §3）。 */
export function withDefaultsIter(v: Iter): Iter {
  return { attempt: 1, ...v };
}

/** 一次节点执行的位置（C.06.05 §3） */
export interface Loc {
  /** 本节点自身的迭代（C7 展开的份号、重试次数） */
  iter?: Iter;
  node_id: Ulid;
  path: string;
  /** 从外到内每一层容器节点及其迭代号，用来唯一确定一次执行 */
  scopes?: LocScopesItem[];
}

export interface LocScopesItem {
  iter?: Iter;
  node_id: Ulid;
}

/** 授权书（C.09 §3）；挂载点 MandateMount 在 common.schema.json */
export interface Mandate {
  /** 模型的修改写到哪：草稿分支或直接写 main（只有档位 4 能选 main） */
  branch?: "draft" | "main";
  created_at: Timestamp;
  /** 必须是 user（C.09-R4） */
  created_by: Actor;
  /** 编排者生成子编排者时能否下放（E.06.02） */
  delegable?: "none" | "narrower" | "same";
  /** 任何档位：这些动作直接拒绝 */
  deny?: Action[];
  /** 档位 3、4：这些动作改为需要审核 */
  except?: Action[];
  expires: Expiry;
  /** 授予对象：model:* 所有模型实例；tag:编排者 带这个标签的实例；external:* 外部调用 */
  grantee: string;
  id: Ulid;
  /** 只有用户能设为 true：L3 工具上游可以不接人工节点 */
  l3_skip_human?: boolean;
  /** 档位：0 只读 · 1 建议 · 2 部分 · 3 高度 · 4 完全 */
  level: number;
  /** 如 谨慎协作 */
  name: string;
  /** 只在档位 2 生效 */
  overrides?: Overrides;
  revoked_at?: Timestamp;
  /** 被污染的动作怎么处理（INV-16） */
  taint?: "ask" | "allow";
}

/** 把没填的可选字段设为 schema 的默认值（C.14 §3）。 */
export function withDefaultsMandate(v: Mandate): Mandate {
  return { branch: "draft", delegable: "narrower", l3_skip_human: false, taint: "ask", ...v };
}

/** 授权书挂载点；图文档和会话都用（C.09 §3、P.09 K203） */
export interface MandateMount {
  /** session 只出现在会话文档里 */
  anchor: string;
  mandate_id: Ulid;
  priority?: number;
}

/** 把没填的可选字段设为 schema 的默认值（C.14 §3）。 */
export function withDefaultsMandateMount(v: MandateMount): MandateMount {
  return { priority: 0, ...v };
}

/** 消息（C.10 §3） */
export interface Message {
  /** assistant 消息为对话 Agent 的模型实例 */
  actor?: Actor;
  blocks?: MessageBlock[];
  /** Markdown */
  content?: string;
  /** 超长时内容放工作区 */
  content_ref?: WsRef;
  created_at: Timestamp;
  error?: MessageError;
  id: Ulid;
  /** 只有 user 消息有：对话框里 @ 引用和 + 附件（C.10-R7） */
  refs?: MessageRef[];
  role: "user" | "assistant" | "system";
  /** 只有 user 消息有：发送时附带的画布选中 */
  selection?: Selection;
  session_id: Ulid;
  status: "streaming" | "done" | "failed";
  usage?: MessageUsage;
}

/** 消息里的卡片块（C.10 §4）；按 kind 判别，写法同 op.schema.json 的 Op（C.10-R8） */
export type MessageBlock = BlockApplied | BlockApproval | BlockError | BlockFocus | BlockGraph | BlockProposal | BlockRun;

export type MessageBlockKind = "graph" | "proposal" | "applied" | "approval" | "run" | "focus" | "error";

/** MessageBlockKind 的取值（C.14-R6） */
export const MessageBlockKind = {
  Graph: "graph",
  Proposal: "proposal",
  Applied: "applied",
  Approval: "approval",
  Run: "run",
  Focus: "focus",
  Error: "error",
} as const;

export interface MessageError {
  code: string;
  message: string;
}

/** 对话框里的 @ 引用和 + 附件（C.10 §5.1）；按 kind 判别 */
export type MessageRef = RefFile | RefNode | RefSession | RefSkill;

export type MessageRefKind = "node" | "file" | "session" | "skill";

/** MessageRefKind 的取值（C.14-R6） */
export const MessageRefKind = {
  Node: "node",
  File: "file",
  Session: "session",
  Skill: "skill",
} as const;

export interface MessageUsage {
  cost_micros?: number;
  in?: number;
  out?: number;
}

/** 开放对象，未知字段原样保留（C.01-R19） */
export type Meta = { [key: string]: unknown };

/** 节点实例（C.02.02） */
export interface Node {
  children?: Subgraph;
  id: Ulid;
  label?: string;
  /** 实例锁（C.03.03） */
  lock?: "none" | "user" | "frozen";
  origin?: Origin;
  /** 开放对象；按 NodeDef.params 校验；只存和默认值不同的键；未知键原样保留 */
  params?: { [key: string]: unknown };
  pos: Pos;
  promoted?: string[];
  ref?: SubgraphRef;
  tags?: string[];
  type: NodeTypeRef;
  ui?: NodeUi;
}

/** 把没填的可选字段设为 schema 的默认值（C.14 §3）。 */
export function withDefaultsNode(v: Node): Node {
  return { lock: "none", ...v };
}

export interface NodeDef {
  category: "资源" | "智能体" | "状态" | "工具" | "拓扑" | "聚合" | "控制" | "人工" | "边界" | "扩展";
  /** 与 container_rules.yaml 一致，见 C.02.04 */
  container?: "none" | "required" | "one_of";
  description?: string;
  dynamic_ports?: DynamicPorts;
  examples?: string[];
  /** 图标名，取自 U.08 图标表；扩展节点缺省为 扩展 */
  icon?: string;
  impl: Impl;
  /** 内核强制规则的编号，只用于展示，如 INV-07 */
  kernel_rules?: string[];
  kind: "atomic" | "extension";
  name: string;
  params: { [key: string]: ParamDef };
  ports: NodeDefPorts;
  /** 类型变量的来源，如 {"T": "param:output_schema"}，见 C.04.03 */
  type_vars?: { [key: string]: string };
  version: string;
}

/** 把没填的可选字段设为 schema 的默认值（C.14 §3）。 */
export function withDefaultsNodeDef(v: NodeDef): NodeDef {
  return { container: "none", ...v };
}

export interface NodeDefPorts {
  inputs: { [key: string]: PortDef };
  outputs: { [key: string]: PortDef };
}

/** 节点失败（C.06.02 §3、C.11 §4） */
export interface NodeError {
  class: FailureClass;
  code: string;
  message: string;
  retryable: boolean;
}

/** 节点状态（C.07 §3） */
export type NodeState = "idle" | "queued" | "running" | "waiting" | "done" | "failed" | "cancelled" | "skipped" | "cached";

/** NodeState 的取值（C.14-R6） */
export const NodeState = {
  Idle: "idle",
  Queued: "queued",
  Running: "running",
  Waiting: "waiting",
  Done: "done",
  Failed: "failed",
  Cancelled: "cancelled",
  Skipped: "skipped",
  Cached: "cached",
} as const;

/** 原子节点：中文名；扩展节点：包名/节点名@主版本 */
export type NodeTypeRef = string;

/** 开放对象，未知字段原样保留（C.02.05） */
export type NodeUi = { [key: string]: unknown };

/** 便签：画布上的说明文字，不连线、不参与运行 */
export interface Note {
  color?: "none" | "c1" | "c2" | "c3" | "c4" | "c5" | "c6";
  id: Ulid;
  origin?: Origin;
  pos: Pos;
  size?: NoteSize;
  /** Markdown，只支持粗体、斜体、列表、链接 */
  text: string;
}

/** 把没填的可选字段设为 schema 的默认值（C.14 §3）。 */
export function withDefaultsNote(v: Note): Note {
  return { color: "none", ...v };
}

export interface NoteSize {
  h: number;
  w: number;
}

/** 所有 Op 共用的外层结构（C.05.01 §2）。payload 按 kind 判别：每种 kind 一条 if/then 指向封闭的 Payload… 定义（C.05.02 §6.1），写法同 nodedef.schema.json 的 Impl，kind 写错时只在 /kind 报一处错 */
export interface OpCommon {
  /** 服务器根据鉴权结果填写，客户端填写的值被忽略 */
  actor?: Actor;
  /** 采纳提议时由服务器填写：actor 是模型，approver 是用户 */
  approver?: Actor;
  /** 由请求体决定；同一批 Op 相同 */
  base_version?: number;
  /** 由请求体决定 */
  branch?: Branch;
  coalesce_key?: string;
  /** 由请求路径决定，客户端可以不填 */
  graph_id?: Ulid;
  group_id?: Ulid;
  /** 客户端可以先生成；缺省由服务器生成 */
  id?: Ulid;
  /** 逆操作（C.05.03），服务器应用时计算并写入；客户端填的值被忽略 */
  inverse?: Op[];
  mandate_id?: Ulid;
  /** actor 为 model 时必填 */
  reason?: string;
  /** 由外部内容引出（INV-16） */
  taint?: boolean;
  target?: OpTarget;
  /** 服务器写入 */
  ts?: Timestamp;
}

export type Op = OpCommon & (
  | { kind: "add_node"; payload: PayloadAddNode }
  | { kind: "add_note"; payload: PayloadAddNote }
  | { kind: "apply_patch"; payload: PayloadApplyPatch }
  | { kind: "connect"; payload: PayloadConnect }
  | { kind: "create_branch"; payload: PayloadCreateBranch }
  | { kind: "delete_branch"; payload: PayloadDeleteBranch }
  | { kind: "demote_param"; payload: PayloadDemoteParam }
  | { kind: "disconnect"; payload: PayloadDisconnect }
  | { kind: "group"; payload: PayloadGroup }
  | { kind: "import_apply"; payload: PayloadImportApply }
  | { kind: "merge_branch"; payload: PayloadMergeBranch }
  | { kind: "migrate"; payload: PayloadMigrate }
  | { kind: "move_nodes"; payload: PayloadMoveNodes }
  | { kind: "promote_param"; payload: PayloadPromoteParam }
  | { kind: "remove_nodes"; payload: PayloadRemoveNodes }
  | { kind: "remove_notes"; payload: PayloadRemoveNotes }
  | { kind: "restore_nodes"; payload: PayloadRestoreNodes }
  | { kind: "revert_op"; payload: PayloadRevertOp }
  | { kind: "revert_to"; payload: PayloadRevertTo }
  | { kind: "set_graph_meta"; payload: PayloadSetGraphMeta }
  | { kind: "set_graph_title"; payload: PayloadSetGraphTitle }
  | { kind: "set_label"; payload: PayloadSetLabel }
  | { kind: "set_lock"; payload: PayloadSetLock }
  | { kind: "set_mandate"; payload: PayloadSetMandate }
  | { kind: "set_node_ui"; payload: PayloadSetNodeUi }
  | { kind: "set_param"; payload: PayloadSetParam }
  | { kind: "set_subgraph_ref"; payload: PayloadSetSubgraphRef }
  | { kind: "set_tags"; payload: PayloadSetTags }
  | { kind: "ungroup"; payload: PayloadUngroup }
  | { kind: "update_note"; payload: PayloadUpdateNote }
);

/** C.05.02 §2–§6 表里的 30 种 */
export type OpKind = "add_node" | "remove_nodes" | "restore_nodes" | "connect" | "disconnect" | "set_param" | "promote_param" | "demote_param" | "set_label" | "set_tags" | "set_lock" | "group" | "ungroup" | "set_subgraph_ref" | "move_nodes" | "set_node_ui" | "add_note" | "update_note" | "remove_notes" | "set_graph_title" | "set_graph_meta" | "set_mandate" | "apply_patch" | "import_apply" | "revert_op" | "revert_to" | "migrate" | "create_branch" | "delete_branch" | "merge_branch";

/** OpKind 的取值（C.14-R6） */
export const OpKind = {
  AddNode: "add_node",
  RemoveNodes: "remove_nodes",
  RestoreNodes: "restore_nodes",
  Connect: "connect",
  Disconnect: "disconnect",
  SetParam: "set_param",
  PromoteParam: "promote_param",
  DemoteParam: "demote_param",
  SetLabel: "set_label",
  SetTags: "set_tags",
  SetLock: "set_lock",
  Group: "group",
  Ungroup: "ungroup",
  SetSubgraphRef: "set_subgraph_ref",
  MoveNodes: "move_nodes",
  SetNodeUi: "set_node_ui",
  AddNote: "add_note",
  UpdateNote: "update_note",
  RemoveNotes: "remove_notes",
  SetGraphTitle: "set_graph_title",
  SetGraphMeta: "set_graph_meta",
  SetMandate: "set_mandate",
  ApplyPatch: "apply_patch",
  ImportApply: "import_apply",
  RevertOp: "revert_op",
  RevertTo: "revert_to",
  Migrate: "migrate",
  CreateBranch: "create_branch",
  DeleteBranch: "delete_branch",
  MergeBranch: "merge_branch",
} as const;

/** 这个 Op 主要作用于哪里；用于授权和冲突检测。服务器会按 payload 重新计算并校验 */
export interface OpTarget {
  edges?: Ulid[];
  nodes?: Ulid[];
  notes?: Ulid[];
  /** root，或容器节点 ID */
  scope: string;
}

/** 200 已应用（C.05.01 §4） */
export interface OpsApplied {
  applied: Ulid[];
  diags: Diag[];
  /** 只在应用 GraphPatch 时非空（C.08-R3） */
  id_map: { [key: string]: Ulid };
  rebased: boolean;
  version: number;
}

/** 202 已转为提议（C.05.01 §4、C.08） */
export interface OpsProposed {
  proposal_id: Ulid;
  status: "open";
}

/** POST /api/v1/graphs/{id}/ops 的请求体（C.05.01 §4） */
export interface OpsRequest {
  base_version: number;
  branch: Branch;
  group_id?: Ulid;
  ops: Op[];
}

/** 最后一次修改这个元素的 Op；前端据此显示 AI 角标 */
export interface Origin {
  actor?: Actor;
  op_id?: Ulid;
  reason?: string;
}

/** 档位 2 的细化（C.09 §3） */
export interface Overrides {
  allow_spawn?: boolean;
  budget?: Budget;
  max_depth?: number;
  /** 单个 GraphPatch 最多新增几个节点 */
  max_nodes?: number;
  /** 允许使用的模型；空 = 不限 */
  models?: string[];
  ops?: { [key: string]: "allow" | "propose" | "deny" };
  /** 允许修改的范围：容器节点 ID 或 root；空 = 挂载点以下全部（C.09-R4） */
  scope?: string[];
  tools_allow?: string[];
  tools_max_risk?: number;
}

/** 把没填的可选字段设为 schema 的默认值（C.14 §3）。 */
export function withDefaultsOverrides(v: Overrides): Overrides {
  return { allow_spawn: false, tools_max_risk: 0, ...v };
}

/** 参数描述（C.03.02 §3） */
export interface ParamDef {
  default?: unknown;
  enum?: (string | number)[];
  /** 枚举值 → 界面显示名 */
  enum_labels?: { [key: string]: string };
  help?: string;
  label: string;
  lock?: "none" | "def" | "kernel";
  max?: number;
  max_length?: number;
  min?: number;
  promotable?: boolean;
  /** type=object 时的 JSON Schema */
  schema?: { [key: string]: unknown };
  /** type=object 时：结构取自另一个参数所选对象的入参 schema，如 param:tool，按 C.03.02 §5 展开 */
  schema_from?: string;
  /** 只能填密钥名，不能填密钥值（S.03） */
  secret?: boolean;
  step?: number;
  type: "integer" | "number" | "string" | "text" | "boolean" | "enum" | "type_ref" | "recipe_ref" | "rolecard_ref" | "model_ref" | "tool_ref" | "expr" | "object";
  /** 界面提示：分组、控件、接线、条件显示（C.03.05） */
  ui?: ParamUI;
  /** 显示单位，如 s、% */
  unit?: string;
}

/** 把没填的可选字段设为 schema 的默认值（C.14 §3）。 */
export function withDefaultsParamDef(v: ParamDef): ParamDef {
  return { lock: "none", promotable: true, secret: false, ...v };
}

/** 参数的界面提示（C.03.05 §4）；只影响显示，不进入执行（C.03.05-R6） */
export interface ParamUI {
  /** 同名输入端口。端口有连线时，参数显示为『由上游「X」提供』，运行时用上游的值 */
  bind?: string;
  /** widget=connection 时可选的连接类型（C.16） */
  connection_type?: string;
  /** 常用：显示在节点上；高级：只在右栏属性里 */
  group?: "常用" | "高级";
  /** 同组内排序，小的在前 */
  order?: number;
  placeholder?: string;
  /** textarea 的默认行数；节点上固定 3 行，右栏可拉高 */
  rows?: number;
  /** 条件表达式（与分支节点同一套），为假时隐藏，如 kind == '提示词' */
  show_if?: string;
  /** auto 时按 C.03.02 §4 推出（schema/builtin/widget_map.yaml） */
  widget?: "auto" | "input" | "textarea" | "stepper" | "slider" | "segmented" | "select" | "switch" | "picker" | "code" | "model" | "rolecard" | "connection";
}

/** 把没填的可选字段设为 schema 的默认值（C.14 §3）。 */
export function withDefaultsParamUI(v: ParamUI): ParamUI {
  return { group: "高级", widget: "auto", ...v };
}

/** 模型提交的一条 Op；payload 里可以用临时 ID，服务器换成 ULID 后再按 op.schema.json#/$defs/Op 校验（C.08-R3） */
export interface PatchOp {
  /** 服务器收到后填写；提议的 decisions 按它记 */
  id?: Ulid;
  kind: OpKind;
  payload: { [key: string]: unknown };
  /** 这一条的理由（模型的 Op 必须写，C.05.01 §3） */
  reason: string;
}

/** C.05.02 §2 */
export interface PayloadAddNode {
  node: Node;
  /** root，或容器节点 ID */
  scope: string;
}

/** C.05.02 §3 */
export interface PayloadAddNote {
  note: Note;
  /** root，或容器节点 ID */
  scope: string;
}

/** C.05.02 §5；子 Op 单独授权、单独校验（C.08） */
export interface PayloadApplyPatch {
  ops: Op[];
  patch_id: Ulid;
}

/** C.05.02 §2；被替换的旧连线由服务器记进 inverse */
export interface PayloadConnect {
  edge: Edge;
}

/** C.05.02 §6 */
export interface PayloadCreateBranch {
  at_version?: number;
  from: Branch;
  /** 只能是 draft/… */
  name: Branch;
}

/** C.05.02 §6；不能是 main（E 层检查） */
export interface PayloadDeleteBranch {
  name: Branch;
}

/** C.05.02 §2 */
export interface PayloadDemoteParam {
  /** 参数名 */
  key: string;
  node: Ulid;
}

/** C.05.02 §2 */
export interface PayloadDisconnect {
  ids: Ulid[];
}

/** C.05.02 §2；rewire 只由服务器填写 */
export interface PayloadGroup {
  label: string;
  new_id: Ulid;
  node_ids: Ulid[];
  rewire?: Rewire;
  /** root，或容器节点 ID */
  scope: string;
}

/** C.05.02 §5 */
export interface PayloadImportApply {
  ops: Op[];
  source: "yaml" | "json";
}

/** C.05.02 §6；三方合并（C.05.04 §3） */
export interface PayloadMergeBranch {
  base: number;
  from: Branch;
  into: Branch;
  ours: number;
  resolutions: Resolution[];
  theirs: number;
}

/** C.05.02 §5；actor 必须是 runtime（E 层检查） */
export interface PayloadMigrate {
  from_schema: string;
  to_schema: string;
}

/** C.05.02 §3；可合并（C.05.04 §4） */
export interface PayloadMoveNodes {
  moves: PayloadMoveNodesMovesItem[];
}

/** 一个节点的新坐标 */
export interface PayloadMoveNodesMovesItem {
  id: Ulid;
  to: Pos;
}

/** C.05.02 §2 */
export interface PayloadPromoteParam {
  /** 参数名 */
  key: string;
  node: Ulid;
}

/** C.05.02 §2 */
export interface PayloadRemoveNodes {
  ids: Ulid[];
}

/** C.05.02 §3 */
export interface PayloadRemoveNotes {
  ids: Ulid[];
}

/** C.05.02 §2；只能作为逆操作出现 */
export interface PayloadRestoreNodes {
  removed: Removed;
}

/** C.05.02 §5 */
export interface PayloadRevertOp {
  op_id: Ulid;
}

/** C.05.02 §5；ops 只由服务器填写 */
export interface PayloadRevertTo {
  ops?: Op[];
  version: number;
}

/** C.05.02 §4 */
export interface PayloadSetGraphMeta {
  to: PayloadSetGraphMetaTo;
}

export interface PayloadSetGraphMetaTo {
  description?: string;
}

/** C.05.02 §4 */
export interface PayloadSetGraphTitle {
  to: string;
}

/** C.05.02 §2 */
export interface PayloadSetLabel {
  node: Ulid;
  to: string;
}

/** C.05.02 §2；actor 必须是 user（E 层检查） */
export interface PayloadSetLock {
  node: Ulid;
  to: "none" | "user" | "frozen";
}

/** C.05.02 §4；授权规则见 C.09 §5 */
export interface PayloadSetMandate {
  anchor: string;
  to: Ulid | null;
}

/** C.05.02 §3；整体替换 ui */
export interface PayloadSetNodeUi {
  node: Ulid;
  to: NodeUi;
}

/** C.05.02 §2、§6.1 */
export interface PayloadSetParam {
  /** 旧值，任意 JSON；只用于冲突检测，不被信任（C.05-R1） */
  from?: unknown;
  /** 参数名 */
  key: string;
  node: Ulid;
  /** 改 schema_from 对象参数的子参数时写：[属性名]（原样，C.03.02-R7） */
  path?: string[];
  /** 新值，任意 JSON */
  to: unknown;
}

/** C.05.02 §2；to 为 null 时要给 children（E 层检查） */
export interface PayloadSetSubgraphRef {
  children?: Subgraph;
  node: Ulid;
  to: SubgraphRef | null;
}

/** C.05.02 §2；to 的写法同 Node.tags */
export interface PayloadSetTags {
  node: Ulid;
  to: string[];
}

/** C.05.02 §2；rewire 只由服务器填写 */
export interface PayloadUngroup {
  id: Ulid;
  rewire?: Rewire;
}

/** C.05.02 §3 */
export interface PayloadUpdateNote {
  id: Ulid;
  to: PayloadUpdateNoteTo;
}

export interface PayloadUpdateNoteTo {
  color?: "none" | "c1" | "c2" | "c3" | "c4" | "c5" | "c6";
  pos?: Pos;
  size?: NoteSize;
  text?: string;
}

/** 端口定义（C.03.01 §4） */
export interface PortDef {
  /** 显示类别；缺省由 type 推出（C.04.01） */
  category?: Category;
  /** 动态插口 in[0]、in[1]… */
  dynamic?: boolean;
  label?: string;
  lock?: "none" | "kernel";
  /** 允许多条连线，见 C.04.04 */
  multi?: boolean;
  required?: boolean;
  /** 支持流式片段，见 E.16 */
  stream?: boolean;
  type: TypeRef;
}

/** 把没填的可选字段设为 schema 的默认值（C.14 §3）。 */
export function withDefaultsPortDef(v: PortDef): PortDef {
  return { dynamic: false, lock: "none", multi: false, required: false, stream: false, ...v };
}

/** 端口引用（C.02.03 §4）；连线和事件都用 */
export interface PortRef {
  node: Ulid;
  /** 动态插口写作 in[0]、in[1] */
  port: string;
}

/** 世界坐标，单位 px，节点左上角；前端负责吸附到 20px 网格，后端不强制 */
export interface Pos {
  x: number;
  y: number;
}

/** 提议（C.08 §4） */
export interface Proposal {
  actor: Actor;
  base_version: number;
  branch: string;
  created_at: Timestamp;
  decided_at?: Timestamp;
  /** PatchOp 的 id → 处理结果 */
  decisions?: { [key: string]: "accepted" | "rejected" };
  graph_id: Ulid;
  id: Ulid;
  patch: GraphPatch;
  /** 来自运行时编排时填 */
  run_id?: Ulid;
  /** 来自对话 Agent 时填 */
  session_id?: Ulid;
  status: "open" | "partially_applied" | "applied" | "rejected" | "stale";
}

/** 幽灵预览（C.08 §4.3）：在当前图上叠加提议后的视图；临时 ID 换成预览用的 ULID */
export interface ProposalPreview {
  ghost_graph: Graph;
  id_map: { [key: string]: Ulid };
  marks: GhostMark[];
}

export interface Recipe {
  budget: RecipeBudget;
  description?: string;
  /** 排除的内容：type:方案（按类型）、path:*.reasoning（按字段路径） */
  exclude?: string[];
  /** 来源名 → 数据类型；装配时按类型把 in 上的输入分给各来源（R4） */
  inputs?: { [key: string]: TypeRef };
  /** 如 执行者配方 */
  name: string;
  on_overflow: "pointer" | "summarize" | "truncate" | "reject";
  sections: Section[];
  /** 引用写作 执行者配方@1 */
  version: number;
}

export interface RecipeBudget {
  /** 上下文装配节点的 budget 参数不能超过它（R3） */
  max_ratio?: number;
  /** 占模型窗口的比例，必须大于 0（R3） */
  ratio: number;
}

/** 引用一个文件：ref（上传到会话工作区）和 path（允许目录里的文件，S.06）恰好一个 */
export interface RefFile {
  kind: "file";
  path?: string;
  ref?: WsRef;
}

/** 引用一个节点 */
export interface RefNode {
  graph_id: Ulid;
  kind: "node";
  node_id: Ulid;
  scope: string;
}

/** 引用另一个会话 */
export interface RefSession {
  kind: "session";
  session_id: Ulid;
}

/** 引用一个技能包；package_id 按 C.01-R6 是 ULID */
export interface RefSkill {
  kind: "skill";
  package_id: Ulid;
}

/** 被删除的内容（C.05.02 §6.1）；被删节点的子图在 node.children 里 */
export interface Removed {
  edges: RemovedEdgesItem[];
  nodes: RemovedNodesItem[];
}

/** 被删除的连线和它所在的层 */
export interface RemovedEdgesItem {
  edge: Edge;
  /** root，或容器节点 ID */
  scope: string;
}

/** 被删除的节点和它所在的层 */
export interface RemovedNodesItem {
  node: Node;
  /** root，或容器节点 ID */
  scope: string;
}

/** 三方合并的一处冲突怎么解决（C.05.02 §6） */
export interface Resolution {
  /** {node} / {node, param} / {edge} / {node, field} / {note} */
  object: ResolutionObject;
  pick: "ours" | "theirs" | "manual";
  /** pick=manual 时的值 */
  value?: unknown;
}

/** {node} / {node, param} / {edge} / {node, field} / {note} */
export interface ResolutionObject {
  edge?: Ulid;
  field?: "label" | "pos";
  node?: Ulid;
  note?: Ulid;
  param?: string;
}

/** 改接记录（C.05.02 §6.1） */
export type Rewire = RewireItem[];

/** 一条被删的连线和替代它的连线 */
export interface RewireItem {
  added: Edge[];
  removed: Edge;
}

export interface Rolecard {
  author?: Actor;
  /** 不能做什么 */
  boundaries?: string[];
  examples?: WsRef[];
  /** 继承的父角色卡 */
  extends?: string;
  /** 适合的模型（只作提示） */
  fit_models?: string[];
  /** 好结果长什么样 */
  good_output?: string;
  /** 你是谁 */
  identity: string;
  method?: string[];
  /** 你要完成什么 */
  mission: string;
  name: string;
  /** 这一版改了什么 */
  reason?: string;
  status?: "draft" | "active" | "archived";
  /** 模板变量，由上下文包提供 */
  vars?: string[];
  version: number;
  /** 卡住时怎么上报 */
  when_stuck?: string;
}

/** 把没填的可选字段设为 schema 的默认值（C.14 §3）。 */
export function withDefaultsRolecard(v: Rolecard): Rolecard {
  return { status: "active", ...v };
}

export interface RunEventCommon {
  actor: Actor;
  /** 引起这条事件的上一条事件的 seq（同一流内），用于因果追踪 */
  cause_seq?: number;
  /** 图协同流必填 */
  graph_id?: Ulid;
  /** 节点级事件必填，见 C.06.05 */
  loc?: Loc;
  /** 运行流必填 */
  run_id: Ulid;
  /** 流内单调递增；SSE 帧的 id。P、S 级从 1 开始；E 级事件不占序号，值为 0（R8） */
  seq: number;
  /** 会话流必填 */
  session_id?: Ulid;
  ts: Timestamp;
}

export type RunEvent = RunEventCommon & (
  | { kind: "approval.requested"; data: DataApprovalRequested }
  | { kind: "approval.resolved"; data: DataApprovalResolved }
  | { kind: "brief.updated"; data: DataBriefUpdated }
  | { kind: "budget.charged"; data: DataBudgetCharged }
  | { kind: "budget.exceeded"; data: DataBudgetExceeded }
  | { kind: "budget.warn"; data: DataBudgetWarn }
  | { kind: "checkpoint.saved"; data: DataCheckpointSaved }
  | { kind: "ctx.assembled"; data: DataCtxAssembled }
  | { kind: "edge.flow"; data: DataEdgeFlow }
  | { kind: "engine.illegal_transition"; data: DataEngineIllegalTransition }
  | { kind: "escalation.merged"; data: DataEscalationMerged }
  | { kind: "escalation.raised"; data: DataEscalationRaised }
  | { kind: "escalation.resolved"; data: DataEscalationResolved }
  | { kind: "escalation.routed"; data: DataEscalationRouted }
  | { kind: "estop.released"; data: DataEstopReleased }
  | { kind: "estop.triggered"; data: DataEstopTriggered }
  | { kind: "ledger.changed"; data: DataLedgerChanged }
  | { kind: "llm.call"; data: DataLlmCall }
  | { kind: "llm.fallback"; data: DataLlmFallback }
  | { kind: "loop.round"; data: DataLoopRound }
  | { kind: "node.chunk"; data: DataNodeChunk }
  | { kind: "node.input"; data: DataNodeInput }
  | { kind: "node.log"; data: DataNodeLog }
  | { kind: "node.output"; data: DataNodeOutput }
  | { kind: "node.progress"; data: DataNodeProgress }
  | { kind: "node.state"; data: DataNodeState }
  | { kind: "node.throttled"; data: DataNodeThrottled }
  | { kind: "orch.expanded"; data: DataOrchExpanded }
  | { kind: "orch.finished"; data: DataOrchFinished }
  | { kind: "orch.started"; data: DataOrchStarted }
  | { kind: "orch.tool_call"; data: DataOrchToolCall }
  | { kind: "pool.usage"; data: DataPoolUsage }
  | { kind: "prompt.rendered"; data: DataPromptRendered }
  | { kind: "run.cancel_requested"; data: DataRunCancelRequested }
  | { kind: "run.created"; data: DataRunCreated }
  | { kind: "run.graph_swapped"; data: DataRunGraphSwapped }
  | { kind: "run.status"; data: DataRunStatus }
  | { kind: "tool.call"; data: DataToolCall }
  | { kind: "tool.intent"; data: DataToolIntent }
);

export type RunEventKind = "run.created" | "run.status" | "run.cancel_requested" | "run.graph_swapped" | "engine.illegal_transition" | "node.state" | "node.progress" | "node.chunk" | "node.input" | "node.output" | "node.throttled" | "node.log" | "loop.round" | "checkpoint.saved" | "ledger.changed" | "llm.call" | "llm.fallback" | "tool.intent" | "tool.call" | "ctx.assembled" | "prompt.rendered" | "orch.started" | "orch.tool_call" | "orch.expanded" | "orch.finished" | "approval.requested" | "approval.resolved" | "escalation.raised" | "escalation.merged" | "escalation.routed" | "escalation.resolved" | "budget.charged" | "budget.warn" | "budget.exceeded" | "estop.triggered" | "estop.released" | "brief.updated" | "edge.flow" | "pool.usage";

/** 运行状态（C.07 §2） */
export type RunStatus = "pending" | "running" | "paused" | "done" | "failed" | "cancelled";

/** RunStatus 的取值（C.14-R6） */
export const RunStatus = {
  Pending: "pending",
  Running: "running",
  Paused: "paused",
  Done: "done",
  Failed: "failed",
  Cancelled: "cancelled",
} as const;

export interface Section {
  /** 本段占总预算的比例；0 = 不单独限制 */
  budget?: number;
  format?: "markdown" | "yaml" | "bullets";
  /** 输入来源：in（全部输入）、in[0]、in.task.instruction（字段路径） */
  from: string;
  /** 任务 / 约束 / 参考资料 / 差距报告 / 经验 */
  name: string;
  /** 超预算时先裁优先级低的（数字大 = 低） */
  priority?: number;
  /** 必需段被裁掉时直接失败（on_overflow 不适用） */
  required?: boolean;
  transform?: string;
  /** 这一段渲染后的文字同时放进上下文包的 vars（R6） */
  var?: string;
}

/** 把没填的可选字段设为 schema 的默认值（C.14 §3）。 */
export function withDefaultsSection(v: Section): Section {
  return { budget: 0, format: "markdown", priority: 5, required: false, transform: "as_is", ...v };
}

/** 发送消息时附带的画布选中（C.10 §5） */
export interface Selection {
  edge_ids?: Ulid[];
  graph_id: Ulid;
  node_ids?: Ulid[];
  /** 用户在看某次运行时附带 */
  run_id?: Ulid;
  /** 用户当前所在的层 */
  scope: string;
  version: number;
}

/** 会话（C.10 §2） */
export interface Session {
  /** 画布当前显示的图 */
  active_graph_id?: Ulid;
  created_at: Timestamp;
  deleted_at?: Timestamp;
  /** 这个会话用的对话入口图；不填用设置 chat.entry_graph（C.10-R9） */
  entry_graph_id?: Ulid;
  graph_ids?: Ulid[];
  id: Ulid;
  /** 用户上次在这个会话里所在的图和层，切回时恢复（U.09.04） */
  last_location?: SessionLastLocation;
  /** anchor 只能是 session（C.10 §2） */
  mandates?: MandateMount[];
  /** 按模型记住的思考强度：模型引用 → 强度（U.09.06-R7）；没有记录的模型用设置里的默认强度 */
  orchestrator_effort?: { [key: string]: "off" | "minimal" | "low" | "medium" | "high" | "xhigh" | "max" };
  /** 对话框上为这个会话临时选的模型：覆盖对话入口图里 Agent 连的模型；不填用入口图里的（C.10-R9） */
  orchestrator_model?: string;
  pinned?: boolean;
  /** 缺省由第一条消息生成 */
  title: string;
  updated_at: Timestamp;
}

/** 把没填的可选字段设为 schema 的默认值（C.14 §3）。 */
export function withDefaultsSession(v: Session): Session {
  return { pinned: false, ...v };
}

export interface SessionEventCommon {
  actor: Actor;
  /** 引起这条事件的上一条事件的 seq（同一流内），用于因果追踪 */
  cause_seq?: number;
  /** 图协同流必填 */
  graph_id?: Ulid;
  /** 节点级事件必填，见 C.06.05 */
  loc?: Loc;
  /** 运行流必填 */
  run_id?: Ulid;
  /** 流内单调递增；SSE 帧的 id。P、S 级从 1 开始；E 级事件不占序号，值为 0（R8） */
  seq: number;
  /** 会话流必填 */
  session_id: Ulid;
  ts: Timestamp;
}

export type SessionEvent = SessionEventCommon & (
  | { kind: "chat.status"; data: DataChatStatus }
  | { kind: "message.block_added"; data: DataMessageBlockAdded }
  | { kind: "message.completed"; data: DataMessageCompleted }
  | { kind: "message.created"; data: DataMessageCreated }
  | { kind: "message.delta"; data: DataMessageDelta }
  | { kind: "message.failed"; data: DataMessageFailed }
  | { kind: "session.graph_attached"; data: DataSessionGraphAttached }
  | { kind: "session.graph_detached"; data: DataSessionGraphDetached }
  | { kind: "session.updated"; data: DataSessionUpdated }
);

export type SessionEventKind = "session.updated" | "session.graph_attached" | "session.graph_detached" | "message.created" | "message.delta" | "message.block_added" | "message.completed" | "message.failed" | "chat.status";

/** 用户上次在这个会话里所在的图和层，切回时恢复（U.09.04） */
export interface SessionLastLocation {
  graph_id?: Ulid;
  /** 当前层：root = 根图，否则为所在容器节点的 ID（与 Selection.scope、前端路由 ?scope= 同一写法） */
  scope?: string;
}

/** 子图（C.02.04）；根图也是一个 Subgraph */
export interface Subgraph {
  edges: Edge[];
  nodes: Node[];
  notes?: Note[];
}

/** 引用子图库里的一个版本（C.02.04） */
export interface SubgraphRef {
  /** 子图库里的名字，如 资料采集 */
  name: string;
  /** 内部节点 ID 或 参数路径 → 替换值（资源、配方、策略引用），不打开子图也能替换 */
  overrides?: { [key: string]: string };
  pin: "snapshot" | "link";
  version: string;
}

export interface Template {
  /** 如 一次运行约 5 万 tokens */
  cost_hint?: string;
  graph: "graph.json";
  /** 实例化时问用户的问题，答案写进指定参数 */
  inputs?: TemplateInputsItem[];
  name: string;
  requires?: TemplateRequires;
  /** 一句话：用来做什么 */
  summary: string;
  tags?: string[];
  title: string;
  version: string;
}

export interface TemplateInputsItem {
  default?: unknown;
  key: string;
  /** 如 你想调研什么？ */
  label: string;
  /** <占位节点 ID>.params.<参数名>，或 inputs.<根图入口端口名> */
  target: string;
  type?: "string" | "text" | "integer" | "enum";
}

export interface TemplateRequires {
  /** 需要的模型能力 */
  models?: ("any" | "json_schema" | "tools" | "long_context")[];
  packages?: string[];
  subgraphs?: string[];
}

export type Timestamp = string;

/** GraphPatch 里的临时 ID（C.08-R1、R3） */
export type TmpId = string;

export interface TypeDef {
  /** 业务类型只能是 gen */
  category?: Category;
  description?: string;
  /** 继承的父类型，子类型可以接到父类型端口（C.04.02 C5） */
  extends?: TypeRef;
  fields?: Field[];
  major: number;
  /** 旧主版本 → 迁移表达式（expr-lang），如 {"1": "{title: v.name, ...v}"} */
  migrate_from?: { [key: string]: string };
  minor: number;
  name: string;
  /** 只能由这种节点产出（C.04.02 C2）；只用于系统类型 */
  producer?: NodeTypeRef;
  /** 标量类型（如 整数@1）；写了 scalar 就不写 fields */
  scalar?: "string" | "integer" | "number" | "boolean";
  /** 系统类型（C.04.01 §4）：只有内置，扩展包不能声明 */
  system?: boolean;
}

/** 把没填的可选字段设为 schema 的默认值（C.14 §3）。 */
export function withDefaultsTypeDef(v: TypeDef): TypeDef {
  return { category: "gen", system: false, ...v };
}

/** 数据类型写法（C.04.01 §2）：任意、$T、列表[…]、名称@主版本[.次版本]。只检查外形，列表里面的写法和类型变量出现的位置由解析器检查（E.03.02） */
export type TypeRef = string;

export type Ulid = string & { readonly __brand: "Ulid" };

export type WsRef = string;
