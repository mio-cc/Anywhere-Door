/* 由 tools/gen 生成，请勿手改 */

// 来源：schema/builtin/messages/errors.zh.yaml、assign.zh.yaml（W.10 §4.5）。

/** 错误码 → 文案（C.11 §3） */
export const ERRORS: Readonly<Record<string, string>> = {
  "approval_closed": "这个审批已经处理过，或者已经过期。",
  "bad_request": "请求的格式不对：{errors}",
  "estopped": "急停中，AI 的操作和新的运行都已暂停。解除急停后才能继续。",
  "extension_invalid": "扩展包或节点定义不合法：{errors}",
  "forbidden": "这个操作没有被授权：{reasons}",
  "graph_conflict": "图已被修改，你的版本落后 {behind} 个。{resolution}",
  "idempotency_mismatch": "同一个请求编号对应了不同的内容，已拒绝。请刷新后重试。",
  "internal": "程序出了问题，已记录。可以复制错误详情反馈给开发者。",
  "invalid_graph": "图里还有问题，修好后才能继续。",
  "node_locked": "「{label}」已锁定，只有你能修改。要让 AI 修改，请先解锁。",
  "not_found": "找不到这个{resource}，可能已经被删除。",
  "origin_rejected": "请求不是从本机的 Anywhere Door 页面发出的，已拒绝。",
  "proposal_stale": "这个提议已经过期：提出之后图又被修改过。",
  "provider_error": "模型厂商返回了错误（{provider}，{status}）。",
  "provider_unavailable": "还没有可用的模型。在「连接 › 模型」里添加一个。",
  "rate_limited": "请求太频繁，请 {retry_after_ms} 毫秒后再试。",
  "run_state": "运行现在是「{status}」，不能进行这个操作。",
  "too_large": "内容太大，超过了上限 {limit}。",
  "unauthorized": "本机访问令牌无效。请重新打开 Anywhere Door 的页面。",
};

/** 兼容规则号 → 提示（C.04.02 §5） */
export const ASSIGN: Readonly<Record<string, string>> = {
  "C1": "任意类型只能接到「任意」端口。先接一个设置了输出类型的 Agent 来转换",
  "C10": "{src} 不能接到 {dst}",
  "C2": "{dst_name}只能来自{producer}节点",
  "C4": "{src} 和 {dst} 版本不同，且没有转换方法",
  "C6": "父类型不能接到子类型端口",
  "C8": "单个值不能接到列表端口，中间加一个汇合节点",
};
