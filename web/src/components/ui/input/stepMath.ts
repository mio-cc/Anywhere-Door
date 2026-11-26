export const DRAG_THRESHOLD = 3;
export const PX_PER_STEP = 4;
export const REPEAT_DELAY = 400;
export const REPEAT_EVERY = 60;
export const IDLE_END = 500;
export const HINT_MS = 3000;

const DECIMAL_RE = /^[+-]?(?:\d+(?:\.\d*)?|\.\d+)$/;

export function decimals(step: number): number {
  if (!Number.isFinite(step) || step <= 0) return 0;
  const s = String(step).toLowerCase();
  const expIdx = s.indexOf('e-');
  if (expIdx >= 0) {
    const exp = Number.parseInt(s.slice(expIdx + 2), 10);
    const mantissa = s.slice(0, expIdx);
    const dot = mantissa.indexOf('.');
    const frac = dot >= 0 ? mantissa.length - dot - 1 : 0;
    return exp + frac;
  }
  const dot = s.indexOf('.');
  return dot >= 0 ? s.length - dot - 1 : 0;
}

export function roundTo(v: number, step: number): number {
  const safeStep = Number.isFinite(step) && step > 0 ? step : 1;
  const d = decimals(safeStep);
  const factor = Math.pow(10, d);
  const rounded = Math.round((v + Number.EPSILON) * factor) / factor;
  return Object.is(rounded, -0) ? 0 : rounded;
}

export function clamp(v: number, min?: number, max?: number): number {
  let out = v;
  if (min !== undefined && Number.isFinite(min) && out < min) out = min;
  if (max !== undefined && Number.isFinite(max) && out > max) out = max;
  return out;
}

export function dragDelta(
  acc: number,
  movementX: number,
  mod: { readonly shift: boolean; readonly alt: boolean },
): { readonly steps: number; readonly acc: number } {
  const scale = mod.shift ? 10 : mod.alt ? 0.1 : 1;
  const total = acc + (movementX / PX_PER_STEP) * scale;
  const steps = Math.trunc(total);
  const rem = Number((total - steps).toFixed(10));
  return { steps, acc: Object.is(rem, -0) ? 0 : rem };
}

export function nudge(
  v: number,
  dir: 1 | -1,
  step: number,
  big: boolean,
  min?: number,
  max?: number,
): number {
  const safeStep = Number.isFinite(step) && step > 0 ? step : 1;
  const delta = dir * safeStep * (big ? 10 : 1);
  return clamp(roundTo(v + delta, safeStep), min, max);
}

export function parseTyped(text: string): number | null {
  const trimmed = text.trim();
  if (!DECIMAL_RE.test(trimmed)) return null;
  const num = Number(trimmed);
  return Number.isFinite(num) ? num : null;
}
