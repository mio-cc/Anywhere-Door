const DEFAULT_LINE_HEIGHT = 20;

export interface RowMetrics {
  readonly lineHeight: number;
  readonly padY: number;
}

const metricsCache = new WeakMap<HTMLTextAreaElement, RowMetrics>();

/** 读取并缓存一次行高与上下内边距（W.08.02 §6.3：挂载时从计算样式读一次，避免每次输入读 getComputedStyle）。 */
export function measureRowMetrics(el: HTMLTextAreaElement): RowMetrics {
  const cached = metricsCache.get(el);
  if (cached) return cached;
  const cs = getComputedStyle(el);
  let lh = Number.parseFloat(cs.lineHeight);
  if (!Number.isFinite(lh) || lh <= 0) {
    const fs = Number.parseFloat(cs.fontSize);
    lh = Number.isFinite(fs) && fs > 0 ? fs * 1.55 : DEFAULT_LINE_HEIGHT;
  }
  const padY = (Number.parseFloat(cs.paddingTop) || 0) + (Number.parseFloat(cs.paddingBottom) || 0);
  const m: RowMetrics = { lineHeight: lh, padY };
  metricsCache.set(el, m);
  return m;
}

/** 按 [minRows, maxRows] 自动调整 TextArea 高度（写 style.height 几何值，W.08.02 §6.3）。 */
export function fitRows(
  el: HTMLTextAreaElement,
  minRows: number,
  maxRows: number,
  metrics?: RowMetrics,
): void {
  const { lineHeight, padY } = metrics ?? measureRowMetrics(el);
  const minH = minRows * lineHeight + padY;
  const maxH = maxRows * lineHeight + padY;

  el.style.height = 'auto';
  const raw = el.scrollHeight;
  const nextH = Math.min(maxH, Math.max(minH, raw));
  el.style.height = `${String(Math.round(nextH))}px`;
  el.style.overflowY = raw > maxH ? 'auto' : 'hidden';
}
