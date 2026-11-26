export interface Box {
  readonly x: number;
  readonly y: number;
  readonly w: number;
  readonly h: number;
}

/** 计算条目相对分组容器左上角的矩形（W.08.01 §4.1）。 */
export function relBox(
  item: Pick<DOMRect, 'left' | 'top' | 'width' | 'height'>,
  group: Pick<DOMRect, 'left' | 'top'>,
): Box {
  return {
    x: item.left - group.left,
    y: item.top - group.top,
    w: item.width,
    h: item.height,
  };
}

/** FLIP 起始变换：将目标大小的底块视觉上对齐到 prev 位置和尺寸（transform-origin: 0 0）。 */
export function flipFrom(prev: Box, next: Box): string {
  const sx = next.w > 0 ? prev.w / next.w : 1;
  const sy = next.h > 0 ? prev.h / next.h : 1;
  return `translate(${String(prev.x)}px,${String(prev.y)}px) scale(${String(sx)},${String(sy)})`;
}

/** 底块目标位置变换（仅 translate，避免布局抖动）。 */
export function at(next: Box): string {
  return `translate(${String(next.x)}px,${String(next.y)}px)`;
}
