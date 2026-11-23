/* 由 tools/gen 生成，请勿手改 */

// 前端字段映射表（C.14-R8、W.04.03 §5.1）：按类型列出每个字段的值怎么转换，字段名一律转 camelCase。
//   "raw"      值整体原样（标量、开放对象、没有类型约束、值都不用转换的数组或映射）
//   { t }      结构类型或判别联合，查 WIRE_TYPES
//   { list }   数组，逐项转换
//   { map }    映射：键是数据、原样，值转换
// union：判别联合。有 field 的，field 的值按 disc 的取值查 branches；没有 field 的，整个对象按 branches 查。
// 表里没有的字段（服务端更新的版本）和不认识的判别值：字段名照常转换，值原样（C.01-R18）。

export type WireNode =
  | "raw"
  | { readonly t: WireTypeName }
  | { readonly list: WireNode }
  | { readonly map: WireNode };

export interface WireUnion {
  readonly disc: string;
  readonly field?: string;
  readonly branches: { readonly [value: string]: WireTypeName };
}

export interface WireType {
  readonly fields: { readonly [key: string]: WireNode };
  readonly union?: WireUnion;
}

export type WireTypeName =
  | "Actor"
  | "Assign"
  | "BlockApplied"
  | "BlockApproval"
  | "BlockError"
  | "BlockFocus"
  | "BlockGraph"
  | "BlockProposal"
  | "BlockProposalCounts"
  | "BlockRun"
  | "BranchMeta"
  | "Budget"
  | "DataApprovalRequested"
  | "DataApprovalResolved"
  | "DataBranchUpdated"
  | "DataBriefUpdated"
  | "DataBudgetCharged"
  | "DataBudgetExceeded"
  | "DataBudgetWarn"
  | "DataChatStatus"
  | "DataCheckpointSaved"
  | "DataCtxAssembled"
  | "DataEdgeFlow"
  | "DataEngineIllegalTransition"
  | "DataEscalationMerged"
  | "DataEscalationRaised"
  | "DataEscalationResolved"
  | "DataEscalationRouted"
  | "DataEstopReleased"
  | "DataEstopTriggered"
  | "DataLedgerChanged"
  | "DataLlmCall"
  | "DataLlmFallback"
  | "DataLoopRound"
  | "DataMandateChanged"
  | "DataMessageBlockAdded"
  | "DataMessageCompleted"
  | "DataMessageCreated"
  | "DataMessageDelta"
  | "DataMessageFailed"
  | "DataModelActivity"
  | "DataNodeChunk"
  | "DataNodeInput"
  | "DataNodeLog"
  | "DataNodeOutput"
  | "DataNodeProgress"
  | "DataNodeState"
  | "DataNodeThrottled"
  | "DataOpApplied"
  | "DataOpCoalesced"
  | "DataOrchExpanded"
  | "DataOrchFinished"
  | "DataOrchStarted"
  | "DataOrchToolCall"
  | "DataPoolUsage"
  | "DataPresence"
  | "DataPromptRendered"
  | "DataProposalCreated"
  | "DataProposalDecided"
  | "DataProposalUpdated"
  | "DataRunCancelRequested"
  | "DataRunCreated"
  | "DataRunGraphSwapped"
  | "DataRunStatus"
  | "DataSessionGraphAttached"
  | "DataSessionGraphDetached"
  | "DataSessionUpdated"
  | "DataToolCall"
  | "DataToolIntent"
  | "DataValidate"
  | "Diag"
  | "DynamicPorts"
  | "Edge"
  | "ErrorResponse"
  | "ErrorResponseError"
  | "Event"
  | "Expiry"
  | "Field"
  | "GhostMark"
  | "GhostMarkTarget"
  | "Graph"
  | "GraphEvent"
  | "GraphMeta"
  | "GraphMetaStats"
  | "GraphPatch"
  | "GraphPatchExpectedEffect"
  | "Impl"
  | "ImplBuiltin"
  | "ImplHttp"
  | "ImplHttpResponse"
  | "ImplMcp"
  | "ImplPrompt"
  | "ImplSubgraph"
  | "ImplSubgraphExposeValue"
  | "ImplSubgraphRef"
  | "Iter"
  | "Loc"
  | "LocScopesItem"
  | "Mandate"
  | "MandateMount"
  | "Message"
  | "MessageBlock"
  | "MessageError"
  | "MessageRef"
  | "MessageUsage"
  | "Node"
  | "NodeDef"
  | "NodeDefPorts"
  | "NodeError"
  | "NodeStateRow"
  | "Note"
  | "NoteSize"
  | "Op"
  | "OpRecord"
  | "OpTarget"
  | "OpsApplied"
  | "OpsProposed"
  | "OpsRequest"
  | "Origin"
  | "Overrides"
  | "ParamDef"
  | "ParamUI"
  | "PatchOp"
  | "PayloadAddNode"
  | "PayloadAddNote"
  | "PayloadApplyPatch"
  | "PayloadConnect"
  | "PayloadCreateBranch"
  | "PayloadDeleteBranch"
  | "PayloadDemoteParam"
  | "PayloadDisconnect"
  | "PayloadGroup"
  | "PayloadImportApply"
  | "PayloadMergeBranch"
  | "PayloadMigrate"
  | "PayloadMoveNodes"
  | "PayloadMoveNodesMovesItem"
  | "PayloadPromoteParam"
  | "PayloadRemoveNodes"
  | "PayloadRemoveNotes"
  | "PayloadRestoreNodes"
  | "PayloadRevertOp"
  | "PayloadRevertTo"
  | "PayloadSetGraphMeta"
  | "PayloadSetGraphMetaTo"
  | "PayloadSetGraphTitle"
  | "PayloadSetLabel"
  | "PayloadSetLock"
  | "PayloadSetMandate"
  | "PayloadSetNodeUi"
  | "PayloadSetParam"
  | "PayloadSetSubgraphRef"
  | "PayloadSetTags"
  | "PayloadUngroup"
  | "PayloadUpdateNote"
  | "PayloadUpdateNoteTo"
  | "PortDef"
  | "PortRef"
  | "Pos"
  | "Proposal"
  | "ProposalPreview"
  | "Recipe"
  | "RecipeBudget"
  | "RefFile"
  | "RefNode"
  | "RefSession"
  | "RefSkill"
  | "Removed"
  | "RemovedEdgesItem"
  | "RemovedNodesItem"
  | "Resolution"
  | "ResolutionObject"
  | "RewireItem"
  | "Rolecard"
  | "RunEvent"
  | "RunMeta"
  | "RunMetaHeadline"
  | "RunMetaUsage"
  | "RunSteps"
  | "Section"
  | "Selection"
  | "Session"
  | "SessionEvent"
  | "SessionLastLocation"
  | "Subgraph"
  | "SubgraphRef"
  | "Template"
  | "TemplateInputsItem"
  | "TemplateRequires"
  | "TypeDef"
  ;

export const WIRE_TYPES: { readonly [K in WireTypeName]: WireType } = {
  Actor: {
    fields: {
      "id": "raw",
      "instance_id": "raw",
      "kind": "raw",
      "tags": "raw",
    },
  },
  Assign: {
    fields: {
      "expand": "raw",
      "migrate": "raw",
      "ok": "raw",
      "reason": "raw",
      "rule": "raw",
    },
  },
  BlockApplied: {
    fields: {
      "graph_id": "raw",
      "kind": "raw",
      "op_ids": "raw",
      "summary": "raw",
      "version": "raw",
    },
  },
  BlockApproval: {
    fields: {
      "approval_id": "raw",
      "approval_kind": "raw",
      "kind": "raw",
      "summary": "raw",
    },
  },
  BlockError: {
    fields: {
      "code": "raw",
      "kind": "raw",
      "message": "raw",
    },
  },
  BlockFocus: {
    fields: {
      "graph_id": "raw",
      "kind": "raw",
      "node_ids": "raw",
      "scope": "raw",
    },
  },
  BlockGraph: {
    fields: {
      "graph_id": "raw",
      "kind": "raw",
      "version": "raw",
    },
  },
  BlockProposal: {
    fields: {
      "counts": { t: "BlockProposalCounts" },
      "kind": "raw",
      "proposal_id": "raw",
      "summary": "raw",
    },
  },
  BlockProposalCounts: {
    fields: {
      "add": "raw",
      "modify": "raw",
      "remove": "raw",
    },
  },
  BlockRun: {
    fields: {
      "kind": "raw",
      "run_id": "raw",
      "status": "raw",
    },
  },
  BranchMeta: {
    fields: {
      "ahead": "raw",
      "base_version": "raw",
      "behind": "raw",
      "created_at": "raw",
      "created_by": { t: "Actor" },
      "deleted_at": "raw",
      "head": "raw",
      "name": "raw",
    },
  },
  Budget: {
    fields: {
      "calls": "raw",
      "cost_micros": "raw",
      "time_ms": "raw",
      "tokens": "raw",
    },
  },
  DataApprovalRequested: {
    fields: {
      "approval_id": "raw",
      "detail_ref": "raw",
      "kind": "raw",
      "options": "raw",
      "summary": "raw",
    },
  },
  DataApprovalResolved: {
    fields: {
      "approval_id": "raw",
      "note": "raw",
      "outcome": "raw",
      "scope": "raw",
      "value_ref": "raw",
    },
  },
  DataBranchUpdated: {
    fields: {
      "ahead": "raw",
      "behind": "raw",
      "head": "raw",
      "name": "raw",
    },
  },
  DataBriefUpdated: {
    fields: {
      "conclusion": "raw",
      "cost": "raw",
      "criteria_check": "raw",
      "goal_id": "raw",
      "status": "raw",
      "superseded": "raw",
    },
  },
  DataBudgetCharged: {
    fields: {
      "cost_micros": "raw",
      "scope_path": "raw",
      "tokens": "raw",
    },
  },
  DataBudgetExceeded: {
    fields: {
      "scope_path": "raw",
    },
  },
  DataBudgetWarn: {
    fields: {
      "ratio": "raw",
      "scope_path": "raw",
    },
  },
  DataChatStatus: {
    fields: {
      "note": "raw",
      "state": "raw",
    },
  },
  DataCheckpointSaved: {
    fields: {
      "ref": "raw",
      "round": "raw",
      "scope_path": "raw",
    },
  },
  DataCtxAssembled: {
    fields: {
      "budget_tokens": "raw",
      "dropped": "raw",
      "sections": "raw",
    },
  },
  DataEdgeFlow: {
    fields: {
      "edge_id": "raw",
    },
  },
  DataEngineIllegalTransition: {
    fields: {
      "cause": "raw",
      "from": "raw",
      "loc": { t: "Loc" },
      "to": "raw",
    },
  },
  DataEscalationMerged: {
    fields: {
      "count": "raw",
      "escalation_id": "raw",
      "into": "raw",
    },
  },
  DataEscalationRaised: {
    fields: {
      "escalation_id": "raw",
      "kind": "raw",
      "merge_key": "raw",
      "options": "raw",
      "question": "raw",
      "severity": "raw",
    },
  },
  DataEscalationResolved: {
    fields: {
      "escalation_id": "raw",
      "outcome": "raw",
    },
  },
  DataEscalationRouted: {
    fields: {
      "escalation_id": "raw",
      "to_path": "raw",
    },
  },
  DataEstopReleased: {
    fields: {
      "scope": "raw",
    },
  },
  DataEstopTriggered: {
    fields: {
      "scope": "raw",
    },
  },
  DataLedgerChanged: {
    fields: {
      "by": "raw",
      "path": "raw",
      "store": "raw",
      "version": "raw",
    },
  },
  DataLlmCall: {
    fields: {
      "cost_micros": "raw",
      "latency_ms": "raw",
      "model": "raw",
      "pool": "raw",
      "provider": "raw",
      "req_ref": "raw",
      "resp_ref": "raw",
      "strategy": "raw",
      "usage": "raw",
    },
  },
  DataLlmFallback: {
    fields: {
      "from_strategy": "raw",
      "reason": "raw",
      "to_strategy": "raw",
    },
  },
  DataLoopRound: {
    fields: {
      "carry_ref": "raw",
      "exit": "raw",
      "max": "raw",
      "round": "raw",
    },
  },
  DataMandateChanged: {
    fields: {
      "anchor": "raw",
      "cause": "raw",
      "level": "raw",
      "mandate_id": "raw",
      "superseded_id": "raw",
    },
  },
  DataMessageBlockAdded: {
    fields: {
      "block": { t: "MessageBlock" },
      "message_id": "raw",
    },
  },
  DataMessageCompleted: {
    fields: {
      "content_ref": "raw",
      "message_id": "raw",
      "usage": "raw",
    },
  },
  DataMessageCreated: {
    fields: {
      "message": { t: "Message" },
    },
  },
  DataMessageDelta: {
    fields: {
      "delta": "raw",
      "message_id": "raw",
    },
  },
  DataMessageFailed: {
    fields: {
      "error": { t: "MessageError" },
      "message_id": "raw",
    },
  },
  DataModelActivity: {
    fields: {
      "actor": { t: "Actor" },
      "state": "raw",
      "target": "raw",
    },
  },
  DataNodeChunk: {
    fields: {
      "delta": "raw",
      "port": "raw",
    },
  },
  DataNodeInput: {
    fields: {
      "port": "raw",
      "value_ref": "raw",
    },
  },
  DataNodeLog: {
    fields: {
      "kv": "raw",
      "level": "raw",
      "msg": "raw",
    },
  },
  DataNodeOutput: {
    fields: {
      "partial": "raw",
      "port": "raw",
      "value_ref": "raw",
    },
  },
  DataNodeProgress: {
    fields: {
      "note": "raw",
      "pct": "raw",
      "sub": "raw",
    },
  },
  DataNodeState: {
    fields: {
      "active_outs": "raw",
      "approval_id": "raw",
      "attempt": "raw",
      "backoff_ms": "raw",
      "cache_key": "raw",
      "cause": "raw",
      "cost": "raw",
      "error": { t: "NodeError" },
      "from": "raw",
      "idem_key": "raw",
      "inputs_ref": "raw",
      "outputs_ref": "raw",
      "partial": "raw",
      "reason": "raw",
      "to": "raw",
    },
  },
  DataNodeThrottled: {
    fields: {
      "pool": "raw",
      "waiting": "raw",
    },
  },
  DataOpApplied: {
    fields: {
      "op": { t: "Op" },
      "version": "raw",
    },
  },
  DataOpCoalesced: {
    fields: {
      "op_id": "raw",
      "to": "raw",
      "version": "raw",
    },
  },
  DataOrchExpanded: {
    fields: {
      "graph_version_after": "raw",
      "graph_version_before": "raw",
      "patch_id": "raw",
      "patch_ref": "raw",
      "subgraph_ref": "raw",
    },
  },
  DataOrchFinished: {
    fields: {
      "result": "raw",
      "stats": "raw",
    },
  },
  DataOrchStarted: {
    fields: {
      "mode": "raw",
      "orch_path": "raw",
    },
  },
  DataOrchToolCall: {
    fields: {
      "ok": "raw",
      "target_node": "raw",
      "tool": "raw",
    },
  },
  DataPoolUsage: {
    fields: {
      "pools": "raw",
    },
  },
  DataPresence: {
    fields: {
      "actor": { t: "Actor" },
      "cursor": "raw",
      "editing": "raw",
      "scope": "raw",
    },
  },
  DataPromptRendered: {
    fields: {
      "ref": "raw",
      "segments": "raw",
    },
  },
  DataProposalCreated: {
    fields: {
      "by": "raw",
      "patch_ref": "raw",
      "proposal_id": "raw",
      "summary": "raw",
    },
  },
  DataProposalDecided: {
    fields: {
      "accepted_op_ids": "raw",
      "proposal_id": "raw",
      "rejected_op_ids": "raw",
      "status": "raw",
    },
  },
  DataProposalUpdated: {
    fields: {
      "base_version": "raw",
      "proposal_id": "raw",
      "status": "raw",
    },
  },
  DataRunCancelRequested: {
    fields: {
      "by": "raw",
      "reason": "raw",
    },
  },
  DataRunCreated: {
    fields: {
      "branch": "raw",
      "budget": "raw",
      "graph_id": "raw",
      "graph_version": "raw",
      "inputs_ref": "raw",
      "registry_hash": "raw",
      "snapshot_ref": "raw",
    },
  },
  DataRunGraphSwapped: {
    fields: {
      "at_round": "raw",
      "from_version": "raw",
      "op_ids": "raw",
      "scope_path": "raw",
      "source": "raw",
      "to_version": "raw",
    },
  },
  DataRunStatus: {
    fields: {
      "at_safepoint": "raw",
      "cause": "raw",
      "cost": "raw",
      "from": "raw",
      "to": "raw",
    },
  },
  DataSessionGraphAttached: {
    fields: {
      "graph_id": "raw",
      "title": "raw",
    },
  },
  DataSessionGraphDetached: {
    fields: {
      "graph_id": "raw",
    },
  },
  DataSessionUpdated: {
    fields: {
      "active_graph_id": "raw",
      "pinned": "raw",
      "title": "raw",
      "updated_at": "raw",
    },
  },
  DataToolCall: {
    fields: {
      "idem_key": "raw",
      "latency_ms": "raw",
      "queued_ms": "raw",
      "req_ref": "raw",
      "result_ref": "raw",
      "risk": "raw",
      "status": "raw",
      "taint": "raw",
      "tool": "raw",
    },
  },
  DataToolIntent: {
    fields: {
      "idem_key": "raw",
      "req_ref": "raw",
      "risk": "raw",
      "tool": "raw",
    },
  },
  DataValidate: {
    fields: {
      "diags": "raw",
      "version": "raw",
    },
  },
  Diag: {
    fields: {
      "code": "raw",
      "level": "raw",
      "msg": "raw",
      "node": "raw",
      "param": "raw",
      "port": "raw",
      "scope": "raw",
    },
  },
  DynamicPorts: {
    fields: {
      "from_param": "raw",
      "max": "raw",
      "type": "raw",
    },
  },
  Edge: {
    fields: {
      "from": { t: "PortRef" },
      "id": "raw",
      "label": "raw",
      "origin": { t: "Origin" },
      "to": { t: "PortRef" },
    },
  },
  ErrorResponse: {
    fields: {
      "error": { t: "ErrorResponseError" },
    },
  },
  ErrorResponseError: {
    fields: {
      "code": "raw",
      "details": "raw",
      "message": "raw",
      "request_id": "raw",
    },
  },
  Event: {
    fields: {
      "actor": { t: "Actor" },
      "cause_seq": "raw",
      "data": "raw",
      "graph_id": "raw",
      "kind": "raw",
      "loc": { t: "Loc" },
      "run_id": "raw",
      "seq": "raw",
      "session_id": "raw",
      "ts": "raw",
    },
  },
  Expiry: {
    fields: {
      "kind": "raw",
      "run_id": "raw",
      "until_at": "raw",
    },
  },
  Field: {
    fields: {
      "append_only": "raw",
      "enum": "raw",
      "fields": { list: { t: "Field" } },
      "inherit": "raw",
      "label": "raw",
      "max_chars": "raw",
      "name": "raw",
      "readonly_except": "raw",
      "require_reason": "raw",
      "required": "raw",
      "since_minor": "raw",
      "type": "raw",
    },
  },
  GhostMark: {
    fields: {
      "ghost": "raw",
      "new": "raw",
      "old": "raw",
      "op_id": "raw",
      "reason": "raw",
      "target": { t: "GhostMarkTarget" },
    },
  },
  GhostMarkTarget: {
    fields: {
      "edge": "raw",
      "node": "raw",
      "note": "raw",
      "param": "raw",
    },
  },
  Graph: {
    fields: {
      "branch": "raw",
      "id": "raw",
      "mandates": { list: { t: "MandateMount" } },
      "meta": "raw",
      "packages": "raw",
      "root": { t: "Subgraph" },
      "schema_version": "raw",
      "title": "raw",
      "version": "raw",
    },
  },
  GraphEvent: {
    fields: {
      "actor": { t: "Actor" },
      "cause_seq": "raw",
      "data": "raw",
      "graph_id": "raw",
      "kind": "raw",
      "loc": { t: "Loc" },
      "run_id": "raw",
      "seq": "raw",
      "session_id": "raw",
      "ts": "raw",
    },
    union: {
      disc: "kind",
      field: "data",
      branches: {
        "branch.updated": "DataBranchUpdated",
        "mandate.changed": "DataMandateChanged",
        "model.activity": "DataModelActivity",
        "op.applied": "DataOpApplied",
        "op.coalesced": "DataOpCoalesced",
        "presence": "DataPresence",
        "proposal.created": "DataProposalCreated",
        "proposal.decided": "DataProposalDecided",
        "proposal.updated": "DataProposalUpdated",
        "validate": "DataValidate",
      },
    },
  },
  GraphMeta: {
    fields: {
      "created_at": "raw",
      "deleted_at": "raw",
      "id": "raw",
      "stats": { t: "GraphMetaStats" },
      "title": "raw",
      "updated_at": "raw",
      "version": "raw",
    },
  },
  GraphMetaStats: {
    fields: {
      "layers": "raw",
      "nodes": "raw",
      "subgraphs": "raw",
    },
  },
  GraphPatch: {
    fields: {
      "base_version": "raw",
      "branch": "raw",
      "expected_effect": { t: "GraphPatchExpectedEffect" },
      "fingerprint": "raw",
      "id": "raw",
      "ops": { list: { t: "PatchOp" } },
      "origin": "raw",
      "parent_patch": "raw",
      "rationale": "raw",
      "scope": "raw",
    },
  },
  GraphPatchExpectedEffect: {
    fields: {
      "cost": "raw",
      "latency": "raw",
      "quality": "raw",
      "risk": "raw",
    },
  },
  Impl: {
    fields: {},
    union: {
      disc: "type",
      branches: {
        "builtin": "ImplBuiltin",
        "http": "ImplHttp",
        "mcp": "ImplMcp",
        "prompt": "ImplPrompt",
        "subgraph": "ImplSubgraph",
      },
    },
  },
  ImplBuiltin: {
    fields: {
      "entry": "raw",
      "type": "raw",
    },
  },
  ImplHttp: {
    fields: {
      "body": "raw",
      "headers": "raw",
      "method": "raw",
      "response": { t: "ImplHttpResponse" },
      "risk": "raw",
      "timeout_ms": "raw",
      "type": "raw",
      "url": "raw",
    },
  },
  ImplHttpResponse: {
    fields: {
      "map": "raw",
      "result_type": "raw",
    },
  },
  ImplMcp: {
    fields: {
      "args": "raw",
      "risk": "raw",
      "server": "raw",
      "tool": "raw",
      "type": "raw",
    },
  },
  ImplPrompt: {
    fields: {
      "output_schema": "raw",
      "recipe": "raw",
      "rolecard": "raw",
      "template": "raw",
      "type": "raw",
    },
  },
  ImplSubgraph: {
    fields: {
      "expose": { map: { t: "ImplSubgraphExposeValue" } },
      "ref": { t: "ImplSubgraphRef" },
      "type": "raw",
    },
  },
  ImplSubgraphExposeValue: {
    fields: {
      "node": "raw",
      "param": "raw",
    },
  },
  ImplSubgraphRef: {
    fields: {
      "name": "raw",
      "version": "raw",
    },
  },
  Iter: {
    fields: {
      "attempt": "raw",
      "event": "raw",
      "fan": "raw",
      "round": "raw",
    },
  },
  Loc: {
    fields: {
      "iter": { t: "Iter" },
      "node_id": "raw",
      "path": "raw",
      "scopes": { list: { t: "LocScopesItem" } },
    },
  },
  LocScopesItem: {
    fields: {
      "iter": { t: "Iter" },
      "node_id": "raw",
    },
  },
  Mandate: {
    fields: {
      "branch": "raw",
      "created_at": "raw",
      "created_by": { t: "Actor" },
      "delegable": "raw",
      "deny": "raw",
      "except": "raw",
      "expires": { t: "Expiry" },
      "grantee": "raw",
      "id": "raw",
      "l3_skip_human": "raw",
      "level": "raw",
      "name": "raw",
      "overrides": { t: "Overrides" },
      "revoked_at": "raw",
      "taint": "raw",
    },
  },
  MandateMount: {
    fields: {
      "anchor": "raw",
      "mandate_id": "raw",
      "priority": "raw",
    },
  },
  Message: {
    fields: {
      "actor": { t: "Actor" },
      "blocks": { list: { t: "MessageBlock" } },
      "content": "raw",
      "content_ref": "raw",
      "created_at": "raw",
      "error": { t: "MessageError" },
      "id": "raw",
      "refs": { list: { t: "MessageRef" } },
      "role": "raw",
      "selection": { t: "Selection" },
      "session_id": "raw",
      "status": "raw",
      "usage": { t: "MessageUsage" },
    },
  },
  MessageBlock: {
    fields: {},
    union: {
      disc: "kind",
      branches: {
        "applied": "BlockApplied",
        "approval": "BlockApproval",
        "error": "BlockError",
        "focus": "BlockFocus",
        "graph": "BlockGraph",
        "proposal": "BlockProposal",
        "run": "BlockRun",
      },
    },
  },
  MessageError: {
    fields: {
      "code": "raw",
      "message": "raw",
    },
  },
  MessageRef: {
    fields: {},
    union: {
      disc: "kind",
      branches: {
        "file": "RefFile",
        "node": "RefNode",
        "session": "RefSession",
        "skill": "RefSkill",
      },
    },
  },
  MessageUsage: {
    fields: {
      "cost_micros": "raw",
      "in": "raw",
      "out": "raw",
    },
  },
  Node: {
    fields: {
      "children": { t: "Subgraph" },
      "id": "raw",
      "label": "raw",
      "lock": "raw",
      "origin": { t: "Origin" },
      "params": "raw",
      "pos": { t: "Pos" },
      "promoted": "raw",
      "ref": { t: "SubgraphRef" },
      "tags": "raw",
      "type": "raw",
      "ui": "raw",
    },
  },
  NodeDef: {
    fields: {
      "category": "raw",
      "container": "raw",
      "description": "raw",
      "dynamic_ports": { t: "DynamicPorts" },
      "examples": "raw",
      "icon": "raw",
      "impl": { t: "Impl" },
      "kernel_rules": "raw",
      "kind": "raw",
      "name": "raw",
      "params": { map: { t: "ParamDef" } },
      "ports": { t: "NodeDefPorts" },
      "type_vars": "raw",
      "version": "raw",
    },
  },
  NodeDefPorts: {
    fields: {
      "inputs": { map: { t: "PortDef" } },
      "outputs": { map: { t: "PortDef" } },
    },
  },
  NodeError: {
    fields: {
      "class": "raw",
      "code": "raw",
      "message": "raw",
      "retryable": "raw",
    },
  },
  NodeStateRow: {
    fields: {
      "attempt": "raw",
      "cost_micros": "raw",
      "ended_at": "raw",
      "error": { t: "NodeError" },
      "exec_key": "raw",
      "last_seq": "raw",
      "loc": { t: "Loc" },
      "node_id": "raw",
      "progress": "raw",
      "round": "raw",
      "started_at": "raw",
      "state": "raw",
      "tokens": "raw",
    },
  },
  Note: {
    fields: {
      "color": "raw",
      "id": "raw",
      "origin": { t: "Origin" },
      "pos": { t: "Pos" },
      "size": { t: "NoteSize" },
      "text": "raw",
    },
  },
  NoteSize: {
    fields: {
      "h": "raw",
      "w": "raw",
    },
  },
  Op: {
    fields: {
      "actor": { t: "Actor" },
      "approver": { t: "Actor" },
      "base_version": "raw",
      "branch": "raw",
      "coalesce_key": "raw",
      "graph_id": "raw",
      "group_id": "raw",
      "id": "raw",
      "inverse": { list: { t: "Op" } },
      "kind": "raw",
      "mandate_id": "raw",
      "payload": "raw",
      "reason": "raw",
      "taint": "raw",
      "target": { t: "OpTarget" },
      "ts": "raw",
    },
    union: {
      disc: "kind",
      field: "payload",
      branches: {
        "add_node": "PayloadAddNode",
        "add_note": "PayloadAddNote",
        "apply_patch": "PayloadApplyPatch",
        "connect": "PayloadConnect",
        "create_branch": "PayloadCreateBranch",
        "delete_branch": "PayloadDeleteBranch",
        "demote_param": "PayloadDemoteParam",
        "disconnect": "PayloadDisconnect",
        "group": "PayloadGroup",
        "import_apply": "PayloadImportApply",
        "merge_branch": "PayloadMergeBranch",
        "migrate": "PayloadMigrate",
        "move_nodes": "PayloadMoveNodes",
        "promote_param": "PayloadPromoteParam",
        "remove_nodes": "PayloadRemoveNodes",
        "remove_notes": "PayloadRemoveNotes",
        "restore_nodes": "PayloadRestoreNodes",
        "revert_op": "PayloadRevertOp",
        "revert_to": "PayloadRevertTo",
        "set_graph_meta": "PayloadSetGraphMeta",
        "set_graph_title": "PayloadSetGraphTitle",
        "set_label": "PayloadSetLabel",
        "set_lock": "PayloadSetLock",
        "set_mandate": "PayloadSetMandate",
        "set_node_ui": "PayloadSetNodeUi",
        "set_param": "PayloadSetParam",
        "set_subgraph_ref": "PayloadSetSubgraphRef",
        "set_tags": "PayloadSetTags",
        "ungroup": "PayloadUngroup",
        "update_note": "PayloadUpdateNote",
      },
    },
  },
  OpRecord: {
    fields: {
      "branch": "raw",
      "graph_id": "raw",
      "op": { t: "Op" },
      "undone": "raw",
      "version": "raw",
    },
  },
  OpTarget: {
    fields: {
      "edges": "raw",
      "nodes": "raw",
      "notes": "raw",
      "scope": "raw",
    },
  },
  OpsApplied: {
    fields: {
      "applied": "raw",
      "diags": { list: { t: "Diag" } },
      "id_map": "raw",
      "rebased": "raw",
      "version": "raw",
    },
  },
  OpsProposed: {
    fields: {
      "proposal_id": "raw",
      "status": "raw",
    },
  },
  OpsRequest: {
    fields: {
      "base_version": "raw",
      "branch": "raw",
      "group_id": "raw",
      "ops": { list: { t: "Op" } },
    },
  },
  Origin: {
    fields: {
      "actor": { t: "Actor" },
      "op_id": "raw",
      "reason": "raw",
    },
  },
  Overrides: {
    fields: {
      "allow_spawn": "raw",
      "budget": { t: "Budget" },
      "max_depth": "raw",
      "max_nodes": "raw",
      "models": "raw",
      "ops": "raw",
      "scope": "raw",
      "tools_allow": "raw",
      "tools_max_risk": "raw",
    },
  },
  ParamDef: {
    fields: {
      "default": "raw",
      "enum": "raw",
      "enum_labels": "raw",
      "help": "raw",
      "label": "raw",
      "lock": "raw",
      "max": "raw",
      "max_length": "raw",
      "min": "raw",
      "promotable": "raw",
      "schema": "raw",
      "schema_from": "raw",
      "secret": "raw",
      "step": "raw",
      "type": "raw",
      "ui": { t: "ParamUI" },
      "unit": "raw",
    },
  },
  ParamUI: {
    fields: {
      "bind": "raw",
      "connection_type": "raw",
      "group": "raw",
      "order": "raw",
      "placeholder": "raw",
      "rows": "raw",
      "show_if": "raw",
      "widget": "raw",
    },
  },
  PatchOp: {
    fields: {
      "id": "raw",
      "kind": "raw",
      "payload": "raw",
      "reason": "raw",
    },
  },
  PayloadAddNode: {
    fields: {
      "node": { t: "Node" },
      "scope": "raw",
    },
  },
  PayloadAddNote: {
    fields: {
      "note": { t: "Note" },
      "scope": "raw",
    },
  },
  PayloadApplyPatch: {
    fields: {
      "ops": { list: { t: "Op" } },
      "patch_id": "raw",
    },
  },
  PayloadConnect: {
    fields: {
      "edge": { t: "Edge" },
    },
  },
  PayloadCreateBranch: {
    fields: {
      "at_version": "raw",
      "from": "raw",
      "name": "raw",
    },
  },
  PayloadDeleteBranch: {
    fields: {
      "name": "raw",
    },
  },
  PayloadDemoteParam: {
    fields: {
      "key": "raw",
      "node": "raw",
    },
  },
  PayloadDisconnect: {
    fields: {
      "ids": "raw",
    },
  },
  PayloadGroup: {
    fields: {
      "label": "raw",
      "new_id": "raw",
      "node_ids": "raw",
      "rewire": { list: { t: "RewireItem" } },
      "scope": "raw",
    },
  },
  PayloadImportApply: {
    fields: {
      "ops": { list: { t: "Op" } },
      "source": "raw",
    },
  },
  PayloadMergeBranch: {
    fields: {
      "base": "raw",
      "from": "raw",
      "into": "raw",
      "ours": "raw",
      "resolutions": { list: { t: "Resolution" } },
      "theirs": "raw",
    },
  },
  PayloadMigrate: {
    fields: {
      "from_schema": "raw",
      "to_schema": "raw",
    },
  },
  PayloadMoveNodes: {
    fields: {
      "moves": { list: { t: "PayloadMoveNodesMovesItem" } },
    },
  },
  PayloadMoveNodesMovesItem: {
    fields: {
      "id": "raw",
      "to": { t: "Pos" },
    },
  },
  PayloadPromoteParam: {
    fields: {
      "key": "raw",
      "node": "raw",
    },
  },
  PayloadRemoveNodes: {
    fields: {
      "ids": "raw",
    },
  },
  PayloadRemoveNotes: {
    fields: {
      "ids": "raw",
    },
  },
  PayloadRestoreNodes: {
    fields: {
      "removed": { t: "Removed" },
    },
  },
  PayloadRevertOp: {
    fields: {
      "op_id": "raw",
    },
  },
  PayloadRevertTo: {
    fields: {
      "ops": { list: { t: "Op" } },
      "version": "raw",
    },
  },
  PayloadSetGraphMeta: {
    fields: {
      "to": { t: "PayloadSetGraphMetaTo" },
    },
  },
  PayloadSetGraphMetaTo: {
    fields: {
      "description": "raw",
    },
  },
  PayloadSetGraphTitle: {
    fields: {
      "to": "raw",
    },
  },
  PayloadSetLabel: {
    fields: {
      "node": "raw",
      "to": "raw",
    },
  },
  PayloadSetLock: {
    fields: {
      "node": "raw",
      "to": "raw",
    },
  },
  PayloadSetMandate: {
    fields: {
      "anchor": "raw",
      "to": "raw",
    },
  },
  PayloadSetNodeUi: {
    fields: {
      "node": "raw",
      "to": "raw",
    },
  },
  PayloadSetParam: {
    fields: {
      "from": "raw",
      "key": "raw",
      "node": "raw",
      "path": "raw",
      "to": "raw",
    },
  },
  PayloadSetSubgraphRef: {
    fields: {
      "children": { t: "Subgraph" },
      "node": "raw",
      "to": { t: "SubgraphRef" },
    },
  },
  PayloadSetTags: {
    fields: {
      "node": "raw",
      "to": "raw",
    },
  },
  PayloadUngroup: {
    fields: {
      "id": "raw",
      "rewire": { list: { t: "RewireItem" } },
    },
  },
  PayloadUpdateNote: {
    fields: {
      "id": "raw",
      "to": { t: "PayloadUpdateNoteTo" },
    },
  },
  PayloadUpdateNoteTo: {
    fields: {
      "color": "raw",
      "pos": { t: "Pos" },
      "size": { t: "NoteSize" },
      "text": "raw",
    },
  },
  PortDef: {
    fields: {
      "category": "raw",
      "dynamic": "raw",
      "label": "raw",
      "lock": "raw",
      "multi": "raw",
      "required": "raw",
      "stream": "raw",
      "type": "raw",
    },
  },
  PortRef: {
    fields: {
      "node": "raw",
      "port": "raw",
    },
  },
  Pos: {
    fields: {
      "x": "raw",
      "y": "raw",
    },
  },
  Proposal: {
    fields: {
      "actor": { t: "Actor" },
      "base_version": "raw",
      "branch": "raw",
      "created_at": "raw",
      "decided_at": "raw",
      "decisions": "raw",
      "graph_id": "raw",
      "id": "raw",
      "patch": { t: "GraphPatch" },
      "run_id": "raw",
      "session_id": "raw",
      "status": "raw",
    },
  },
  ProposalPreview: {
    fields: {
      "ghost_graph": { t: "Graph" },
      "id_map": "raw",
      "marks": { list: { t: "GhostMark" } },
    },
  },
  Recipe: {
    fields: {
      "budget": { t: "RecipeBudget" },
      "description": "raw",
      "exclude": "raw",
      "inputs": "raw",
      "name": "raw",
      "on_overflow": "raw",
      "sections": { list: { t: "Section" } },
      "version": "raw",
    },
  },
  RecipeBudget: {
    fields: {
      "max_ratio": "raw",
      "ratio": "raw",
    },
  },
  RefFile: {
    fields: {
      "kind": "raw",
      "path": "raw",
      "ref": "raw",
    },
  },
  RefNode: {
    fields: {
      "graph_id": "raw",
      "kind": "raw",
      "node_id": "raw",
      "scope": "raw",
    },
  },
  RefSession: {
    fields: {
      "kind": "raw",
      "session_id": "raw",
    },
  },
  RefSkill: {
    fields: {
      "kind": "raw",
      "package_id": "raw",
    },
  },
  Removed: {
    fields: {
      "edges": { list: { t: "RemovedEdgesItem" } },
      "nodes": { list: { t: "RemovedNodesItem" } },
    },
  },
  RemovedEdgesItem: {
    fields: {
      "edge": { t: "Edge" },
      "scope": "raw",
    },
  },
  RemovedNodesItem: {
    fields: {
      "node": { t: "Node" },
      "scope": "raw",
    },
  },
  Resolution: {
    fields: {
      "object": { t: "ResolutionObject" },
      "pick": "raw",
      "value": "raw",
    },
  },
  ResolutionObject: {
    fields: {
      "edge": "raw",
      "field": "raw",
      "node": "raw",
      "note": "raw",
      "param": "raw",
    },
  },
  RewireItem: {
    fields: {
      "added": { list: { t: "Edge" } },
      "removed": { t: "Edge" },
    },
  },
  Rolecard: {
    fields: {
      "author": { t: "Actor" },
      "boundaries": "raw",
      "examples": "raw",
      "extends": "raw",
      "fit_models": "raw",
      "good_output": "raw",
      "identity": "raw",
      "method": "raw",
      "mission": "raw",
      "name": "raw",
      "reason": "raw",
      "status": "raw",
      "vars": "raw",
      "version": "raw",
      "when_stuck": "raw",
    },
  },
  RunEvent: {
    fields: {
      "actor": { t: "Actor" },
      "cause_seq": "raw",
      "data": "raw",
      "graph_id": "raw",
      "kind": "raw",
      "loc": { t: "Loc" },
      "run_id": "raw",
      "seq": "raw",
      "session_id": "raw",
      "ts": "raw",
    },
    union: {
      disc: "kind",
      field: "data",
      branches: {
        "approval.requested": "DataApprovalRequested",
        "approval.resolved": "DataApprovalResolved",
        "brief.updated": "DataBriefUpdated",
        "budget.charged": "DataBudgetCharged",
        "budget.exceeded": "DataBudgetExceeded",
        "budget.warn": "DataBudgetWarn",
        "checkpoint.saved": "DataCheckpointSaved",
        "ctx.assembled": "DataCtxAssembled",
        "edge.flow": "DataEdgeFlow",
        "engine.illegal_transition": "DataEngineIllegalTransition",
        "escalation.merged": "DataEscalationMerged",
        "escalation.raised": "DataEscalationRaised",
        "escalation.resolved": "DataEscalationResolved",
        "escalation.routed": "DataEscalationRouted",
        "estop.released": "DataEstopReleased",
        "estop.triggered": "DataEstopTriggered",
        "ledger.changed": "DataLedgerChanged",
        "llm.call": "DataLlmCall",
        "llm.fallback": "DataLlmFallback",
        "loop.round": "DataLoopRound",
        "node.chunk": "DataNodeChunk",
        "node.input": "DataNodeInput",
        "node.log": "DataNodeLog",
        "node.output": "DataNodeOutput",
        "node.progress": "DataNodeProgress",
        "node.state": "DataNodeState",
        "node.throttled": "DataNodeThrottled",
        "orch.expanded": "DataOrchExpanded",
        "orch.finished": "DataOrchFinished",
        "orch.started": "DataOrchStarted",
        "orch.tool_call": "DataOrchToolCall",
        "pool.usage": "DataPoolUsage",
        "prompt.rendered": "DataPromptRendered",
        "run.cancel_requested": "DataRunCancelRequested",
        "run.created": "DataRunCreated",
        "run.graph_swapped": "DataRunGraphSwapped",
        "run.status": "DataRunStatus",
        "tool.call": "DataToolCall",
        "tool.intent": "DataToolIntent",
      },
    },
  },
  RunMeta: {
    fields: {
      "branch": "raw",
      "cost_micros": "raw",
      "created_at": "raw",
      "ended_at": "raw",
      "fork_of": "raw",
      "fork_seq": "raw",
      "graph_id": "raw",
      "graph_version": "raw",
      "headline": { t: "RunMetaHeadline" },
      "id": "raw",
      "kind": "raw",
      "last_seq": "raw",
      "no": "raw",
      "registry_hash": "raw",
      "replay_of": "raw",
      "session_id": "raw",
      "snapshot_ref": "raw",
      "started_by": { t: "Actor" },
      "status": "raw",
      "steps": { t: "RunSteps" },
      "usage": { t: "RunMetaUsage" },
    },
  },
  RunMetaHeadline: {
    fields: {
      "kind": "raw",
      "node_label": "raw",
      "steps": { t: "RunSteps" },
    },
  },
  RunMetaUsage: {
    fields: {
      "cached": "raw",
      "in": "raw",
      "out": "raw",
    },
  },
  RunSteps: {
    fields: {
      "done": "raw",
      "total": "raw",
    },
  },
  Section: {
    fields: {
      "budget": "raw",
      "format": "raw",
      "from": "raw",
      "name": "raw",
      "priority": "raw",
      "required": "raw",
      "transform": "raw",
      "var": "raw",
    },
  },
  Selection: {
    fields: {
      "edge_ids": "raw",
      "graph_id": "raw",
      "node_ids": "raw",
      "run_id": "raw",
      "scope": "raw",
      "version": "raw",
    },
  },
  Session: {
    fields: {
      "active_graph_id": "raw",
      "created_at": "raw",
      "deleted_at": "raw",
      "entry_graph_id": "raw",
      "graph_ids": "raw",
      "id": "raw",
      "last_location": { t: "SessionLastLocation" },
      "mandates": { list: { t: "MandateMount" } },
      "orchestrator_effort": "raw",
      "orchestrator_model": "raw",
      "pinned": "raw",
      "title": "raw",
      "updated_at": "raw",
    },
  },
  SessionEvent: {
    fields: {
      "actor": { t: "Actor" },
      "cause_seq": "raw",
      "data": "raw",
      "graph_id": "raw",
      "kind": "raw",
      "loc": { t: "Loc" },
      "run_id": "raw",
      "seq": "raw",
      "session_id": "raw",
      "ts": "raw",
    },
    union: {
      disc: "kind",
      field: "data",
      branches: {
        "chat.status": "DataChatStatus",
        "message.block_added": "DataMessageBlockAdded",
        "message.completed": "DataMessageCompleted",
        "message.created": "DataMessageCreated",
        "message.delta": "DataMessageDelta",
        "message.failed": "DataMessageFailed",
        "session.graph_attached": "DataSessionGraphAttached",
        "session.graph_detached": "DataSessionGraphDetached",
        "session.updated": "DataSessionUpdated",
      },
    },
  },
  SessionLastLocation: {
    fields: {
      "graph_id": "raw",
      "scope": "raw",
    },
  },
  Subgraph: {
    fields: {
      "edges": { list: { t: "Edge" } },
      "nodes": { list: { t: "Node" } },
      "notes": { list: { t: "Note" } },
    },
  },
  SubgraphRef: {
    fields: {
      "name": "raw",
      "overrides": "raw",
      "pin": "raw",
      "version": "raw",
    },
  },
  Template: {
    fields: {
      "cost_hint": "raw",
      "graph": "raw",
      "inputs": { list: { t: "TemplateInputsItem" } },
      "name": "raw",
      "requires": { t: "TemplateRequires" },
      "summary": "raw",
      "tags": "raw",
      "title": "raw",
      "version": "raw",
    },
  },
  TemplateInputsItem: {
    fields: {
      "default": "raw",
      "key": "raw",
      "label": "raw",
      "target": "raw",
      "type": "raw",
    },
  },
  TemplateRequires: {
    fields: {
      "models": "raw",
      "packages": "raw",
      "subgraphs": "raw",
    },
  },
  TypeDef: {
    fields: {
      "category": "raw",
      "description": "raw",
      "extends": "raw",
      "fields": { list: { t: "Field" } },
      "major": "raw",
      "migrate_from": "raw",
      "minor": "raw",
      "name": "raw",
      "producer": "raw",
      "scalar": "raw",
      "system": "raw",
    },
  },
};
