/* 由 tools/gen 生成，请勿手改 */

/** 三条事件流的事件名（C.06）（C.14-R6） */
export const Ev = {
  Hello: "hello",
  Bye: "bye",
} as const;

/** 原子节点类型名（C.14-R7，别名见 schema/builtin/aliases.yaml）（C.14-R6） */
export const NodeType = {
  Alpha: "甲",
} as const;

/** SSE 控制帧名（C.06.01 §4）：不是事件（C.14-R6） */
export const SseControl = {
  Resync: "resync",
} as const;
