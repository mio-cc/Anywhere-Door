import type { PickerGroup, PickerItem } from './Picker';

export interface NavRow {
  readonly id: string;
  readonly disabled: boolean;
  readonly height: number;
}

export const WINDOW_MIN = 200;
export const WINDOW_BUFFER = 10;

export function flatten(groups: readonly PickerGroup[], rowH: (it: PickerItem) => number): NavRow[] {
  const out: NavRow[] = [];
  for (const g of groups) {
    for (const it of g.items) {
      out.push({ id: it.id, disabled: Boolean(it.disabled), height: rowH(it) });
    }
  }
  return out;
}

export function step(
  rows: readonly NavRow[],
  fromId: string | null,
  dir: 1 | -1,
  wrap: boolean,
): string | null {
  const n = rows.length;
  if (n === 0) return null;
  const fromIdx = fromId === null ? -1 : rows.findIndex((r) => r.id === fromId);
  if (fromIdx < 0) {
    return edge(rows, dir === 1 ? 'first' : 'last');
  }
  let idx = fromIdx + dir;
  while (idx >= 0 && idx < n) {
    const r = rows[idx];
    if (r && !r.disabled) return r.id;
    idx += dir;
  }
  if (!wrap) return null;
  idx = dir === 1 ? 0 : n - 1;
  while (idx !== fromIdx) {
    const r = rows[idx];
    if (r && !r.disabled) return r.id;
    idx += dir;
  }
  const cur = rows[fromIdx];
  return cur && !cur.disabled ? cur.id : null;
}

export function edge(rows: readonly NavRow[], which: 'first' | 'last'): string | null {
  if (which === 'first') {
    for (const r of rows) {
      if (!r.disabled) return r.id;
    }
    return null;
  }
  for (let i = rows.length - 1; i >= 0; i -= 1) {
    const r = rows[i];
    if (r && !r.disabled) return r.id;
  }
  return null;
}

function foldForMatch(s: string): string {
  // 全角 ASCII（U+FF01..U+FF5E）与全角空格（U+3000）归一为半角并转小写（1:1 长度对应，便于返回准确索引）
  let out = '';
  for (let i = 0; i < s.length; i += 1) {
    const code = s.charCodeAt(i);
    if (code >= 0xff01 && code <= 0xff5e) {
      out += String.fromCharCode(code - 0xfee0).toLowerCase();
    } else if (code === 0x3000) {
      out += ' ';
    } else {
      out += (s[i] ?? '').toLowerCase();
    }
  }
  return out;
}

export function typeahead(
  labels: readonly { readonly id: string; readonly label: string; readonly disabled: boolean }[],
  buf: string,
  fromId: string | null,
): string | null {
  const q = foldForMatch(buf.trim());
  if (q === '' || labels.length === 0) return null;
  const startIdx = fromId === null ? -1 : labels.findIndex((it) => it.id === fromId);
  for (let offset = 1; offset <= labels.length; offset += 1) {
    const idx = (startIdx + offset) % labels.length;
    const it = labels[idx];
    if (it && !it.disabled && foldForMatch(it.label).startsWith(q)) {
      return it.id;
    }
  }
  return null;
}

/** 给定每行顶边偏移数组 offsets（长度 N 或 N+1），计算 [start, end) 窗口范围（含前后 buffer）。 */
export function windowOf(
  offsets: readonly number[],
  scrollTop: number,
  viewport: number,
  buffer: number,
): { readonly start: number; readonly end: number } {
  const n = offsets.length;
  if (n === 0) return { start: 0, end: 0 };

  let lo = 0;
  let hi = n - 1;
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1;
    if ((offsets[mid] ?? 0) <= scrollTop) {
      lo = mid;
    } else {
      hi = mid - 1;
    }
  }
  const firstVisible = lo;
  const bottom = scrollTop + viewport;

  let endVis = firstVisible;
  while (endVis < n && (offsets[endVis] ?? 0) < bottom) {
    endVis += 1;
  }

  return {
    start: Math.max(0, firstVisible - buffer),
    end: Math.min(n, endVis + buffer),
  };
}

/** 不分大小写、全半角不敏感的第一处匹配下标区间 [start, end)（W.08.03 §4.1）。 */
export function matchSpan(label: string, q: string): [number, number] | null {
  const fq = foldForMatch(q.trim());
  if (fq === '') return null;
  const fl = foldForMatch(label);
  const idx = fl.indexOf(fq);
  if (idx < 0) return null;
  return [idx, idx + fq.length];
}
