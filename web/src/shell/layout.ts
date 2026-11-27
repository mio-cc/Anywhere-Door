import type { ShellLayout } from '@/state/uiStore';
import { isOverlayRightSidebar } from './breakpoints';

// 适应画布避让几何常量（U.09.01-R1、W.02.02 §6.3，与 demo v0.5 fitTarget 一致）
export const FIT_TOP = 28;
export const FIT_RIGHT = 28;
export const FIT_LEFT_ROOT = 64;
export const FIT_LEFT_SUBGRAPH = 304;
export const FIT_BOTTOM_BASE = 44;
export const FIT_FLOAT_REPLY_MAX = 80;

export interface FitInsetsInput {
  readonly inSubgraph: boolean;
  readonly composerHeight: number;
  readonly floatReplyOpen: boolean;
  readonly floatReplyHeight: number;
}

export function computeFitInsets(input: FitInsetsInput): ShellLayout['fitInsets'] {
  const composerH =
    Number.isFinite(input.composerHeight) && input.composerHeight > 0 ? input.composerHeight : 0;
  const replyRaw =
    input.floatReplyOpen && Number.isFinite(input.floatReplyHeight) && input.floatReplyHeight > 0
      ? input.floatReplyHeight
      : 0;
  const replyH = Math.min(FIT_FLOAT_REPLY_MAX, replyRaw);

  return {
    top: FIT_TOP,
    right: FIT_RIGHT,
    bottom: composerH + replyH + FIT_BOTTOM_BASE,
    left: input.inSubgraph ? FIT_LEFT_SUBGRAPH : FIT_LEFT_ROOT,
  };
}

export interface RectLike {
  readonly left: number;
  readonly right: number;
  readonly top: number;
  readonly bottom: number;
}

export function rectsIntersect(a: RectLike, b: RectLike): boolean {
  return a.left < b.right && a.right > b.left && a.top < b.bottom && a.bottom > b.top;
}

/**
 * 小地图避让规则（U.09.01-R4、U.09.03、W.02.02 §6.2）：
 * bp < 1100 时始终隐藏；否则与底部对话框相交时隐藏。
 */
export function shouldHideMinimap(
  bp: ShellLayout['bp'],
  minimapRect: RectLike | null,
  composerRect: RectLike | null,
): boolean {
  if (isOverlayRightSidebar(bp)) return true;
  if (!minimapRect || !composerRect) return false;
  return rectsIntersect(minimapRect, composerRect);
}
