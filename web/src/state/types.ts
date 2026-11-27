import type { StoreApi } from 'zustand/vanilla';
import type { Ulid } from '@/lib/protocol';

/** 当前层：根图或容器节点 ID（W.03 §4.2、P.09 K68）。 */
export type Scope = 'root' | Ulid;

/** 外部只读 store 视图，禁止外部直接 setState（W.03 §4.2、G.06.02-R2）。 */
export type ReadonlyStore<T> = Pick<StoreApi<T>, 'getState' | 'subscribe'>;

/** 选中归属键：会话 ID，或回放画布 `replay:<运行 ID>`（W.03 §4.2、W.03.03 §5.2）。 */
export type SelectionKey = Ulid | `replay:${Ulid}`;

/** 全前端唯一断点定义（U.09.03-R2、W.02.02 §4.3、P.09 K71、K309）：宽度 < 值 时进入该档。 */
export type Breakpoint = 'xl' | 'lg' | 'md' | 'sm' | 'xs';

export const BREAKPOINTS = {
  lg: 1180,
  md: 1100,
  sm: 900,
  xs: 760,
} as const;

export function bpOf(width: number): Breakpoint {
  if (width < BREAKPOINTS.xs) return 'xs';
  if (width < BREAKPOINTS.sm) return 'sm';
  if (width < BREAKPOINTS.md) return 'md';
  if (width < BREAKPOINTS.lg) return 'lg';
  return 'xl';
}
