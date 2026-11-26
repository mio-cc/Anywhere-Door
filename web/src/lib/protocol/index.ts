// 前端协议层唯一对外入口（W.04.03 §4、W.04.04-R1、P.09 K300、K301）。
// lib/protocol/ 以外只允许从 '@/lib/protocol' 导入协议类型、常量和数据表。

export type * from './types';
export { Act, ErrCode, Ev, NodeType, SseControl } from './generated/consts';
export {
  ActorKind,
  Category,
  DiagCode,
  DiagLevel,
  Effort,
  FailureClass,
  ImplType,
  MessageBlockKind,
  MessageRefKind,
  NodeState,
  OpKind,
  RunStatus,
} from './generated/types';
export {
  AGGREGATE_OPS,
  AUTHZ_MATRIX,
  CATEGORY_MAP,
  CONTAINER_RULES,
  SHORTCUTS,
  STATE_MACHINE,
  WIDGET_MAP,
} from './generated/tables';
export type { WireTypeName } from './generated/wire';
export type Page<T> = { items: readonly T[]; nextCursor?: string };
export { fromWire, toWire, toWireQuery, camelKey, snakeKey, type FromWire, type WireQuery } from './mapping';
