import { clockNow } from './clock';
import { zh } from './zh';

export const CURRENCY = { symbol: '\u00A5', code: 'CNY' } as const;

const DASH = '\u2014';
const warned = new Set<string>();

function isDev(): boolean {
  return (
    typeof import.meta !== 'undefined' &&
    (import.meta as { env?: { MODE?: string } }).env?.MODE !== 'production' &&
    (typeof process === 'undefined' || process.env['NODE_ENV'] !== 'production')
  );
}

function warnOnce(key: string, msg: string): void {
  if (!isDev() || warned.has(key)) return;
  warned.add(key);
  console.warn(msg);
}

function pad2(n: number): string {
  return n < 10 ? `0${String(n)}` : String(n);
}

function pad4(n: number): string {
  const neg = n < 0;
  const s = String(Math.abs(n)).padStart(4, '0');
  return neg ? `-${s}` : s;
}

function trimDecimal(num: number): string {
  const s = num.toFixed(1);
  return s.endsWith('.0') ? s.slice(0, -2) : s;
}

function addCommas(intPart: number): string {
  const raw = String(intPart);
  if (raw.length <= 3) return raw;
  let out = '';
  let count = 0;
  for (let i = raw.length - 1; i >= 0; i -= 1) {
    if (count > 0 && count % 3 === 0) out = `,${out}`;
    out = `${raw.charAt(i)}${out}`;
    count += 1;
  }
  return out;
}

function fillFmt(template: string, vars: Record<string, string | number>): string {
  return template.replace(/\{([a-zA-Z0-9_]+)\}/g, (m, k: string) => {
    const v = vars[k];
    return v === undefined ? m : String(v);
  });
}

export function fmtCount(n: number): string {
  if (!Number.isFinite(n)) {
    warnOnce('fmtCount:invalid', `fmtCount: invalid number ${String(n)}`);
    return DASH;
  }
  const rounded = Math.round(n);
  if (rounded === 0) return '0';
  const neg = rounded < 0;
  const abs = Math.abs(rounded);
  const sign = neg ? '-' : '';
  if (abs < 10_000) {
    return `${sign}${String(abs)}`;
  }
  if (Math.round(abs / 10_000) >= 10_000) {
    const yi = Math.round(abs / 10_000_000) / 10;
    return `${sign}${fillFmt(zh['fmt.yi'], { n: trimDecimal(yi) })}`;
  }
  const wan = Math.round(abs / 1_000) / 10;
  return `${sign}${fillFmt(zh['fmt.wan'], { n: trimDecimal(wan) })}`;
}

export function fmtTokens(n: number): string {
  return fmtCount(n);
}

export function fmtCost(micros: number): string {
  if (!Number.isFinite(micros)) {
    warnOnce('fmtCost:invalid', `fmtCost: invalid number ${String(micros)}`);
    return DASH;
  }
  const rounded = Math.round(micros);
  if (rounded === 0) return `${CURRENCY.symbol}0`;
  const neg = rounded < 0;
  const abs = Math.abs(rounded);
  if (abs < 5_000) {
    return `${neg ? '-' : ''}<${CURRENCY.symbol}0.01`;
  }
  const cents = Math.floor((abs + 5_000) / 10_000);
  const yuan = Math.floor(cents / 100);
  const frac = pad2(cents % 100);
  return `${neg ? '-' : ''}${CURRENCY.symbol}${addCommas(yuan)}.${frac}`;
}

export function fmtCostInput(micros: number): string {
  if (!Number.isFinite(micros)) {
    warnOnce('fmtCostInput:invalid', `fmtCostInput: invalid number ${String(micros)}`);
    return DASH;
  }
  const rounded = Math.round(micros);
  if (rounded === 0) return '0';
  const neg = rounded < 0;
  const abs = Math.abs(rounded);
  const intPart = Math.floor(abs / 1_000_000);
  const fracPart = abs % 1_000_000;
  const sign = neg ? '-' : '';
  if (fracPart === 0) return `${sign}${String(intPart)}`;
  const fracStr = String(fracPart).padStart(6, '0').replace(/0+$/, '');
  return `${sign}${String(intPart)}.${fracStr}`;
}

export function parseCostInput(text: string): number | null {
  let s = text.trim();
  if (s === '') return null;
  let norm = '';
  for (let i = 0; i < s.length; i += 1) {
    const code = s.charCodeAt(i);
    if (code >= 0xff10 && code <= 0xff19) {
      norm += String.fromCharCode(code - 0xff10 + 0x30);
    } else if (code === 0xff0e) {
      norm += '.';
    } else if (code === 0xff0c) {
      norm += ',';
    } else if (code === 0xffe5) {
      norm += CURRENCY.symbol;
    } else {
      norm += s.charAt(i);
    }
  }
  s = norm;
  if (s.startsWith(CURRENCY.symbol)) {
    s = s.slice(1);
  }
  s = s.replace(/,/g, '');
  if (!/^\d{1,9}(?:\.\d{0,6})?$/.test(s)) {
    return null;
  }
  const [intStr = '0', fracStr = ''] = s.split('.');
  const intVal = Number.parseInt(intStr, 10);
  const fracPadded = fracStr.padEnd(6, '0');
  const fracVal = fracPadded === '' ? 0 : Number.parseInt(fracPadded, 10);
  return intVal * 1_000_000 + fracVal;
}

export function fmtDuration(ms: number): string {
  if (!Number.isFinite(ms)) {
    warnOnce('fmtDuration:invalid', `fmtDuration: invalid number ${String(ms)}`);
    return DASH;
  }
  if (ms < 0) return DASH;
  if (ms < 1_000) return zh['fmt.lt1s'];
  if (ms < 10_000) {
    const sec = Math.round(ms / 100) / 10;
    return fillFmt(zh['fmt.sec'], { n: trimDecimal(sec) });
  }
  if (ms < 60_000) {
    return fillFmt(zh['fmt.sec'], { n: Math.floor(ms / 1_000) });
  }
  if (ms < 3_600_000) {
    const totalSec = Math.floor(ms / 1_000);
    const m = Math.floor(totalSec / 60);
    const s = pad2(totalSec % 60);
    return fillFmt(zh['fmt.minSec'], { m, s });
  }
  const totalMin = Math.floor(ms / 60_000);
  const h = Math.floor(totalMin / 60);
  const m = pad2(totalMin % 60);
  return fillFmt(zh['fmt.hourMin'], { h, m });
}

const BYTE_UNITS = ['B', 'KB', 'MB', 'GB', 'TB'] as const;

export function fmtBytes(n: number): string {
  if (!Number.isFinite(n)) {
    warnOnce('fmtBytes:invalid', `fmtBytes: invalid number ${String(n)}`);
    return DASH;
  }
  if (n < 0) return DASH;
  if (n < 1024) {
    const r = Math.round(n);
    if (r < 1024) return `${String(r)} B`;
    return '1 KB';
  }
  let val = n;
  let unit = 0;
  while (val >= 1024 && unit < BYTE_UNITS.length - 1) {
    val /= 1024;
    unit += 1;
  }
  let rounded = val < 100 ? Math.round(val * 10) / 10 : Math.round(val);
  if (rounded >= 1024 && unit < BYTE_UNITS.length - 1) {
    val = rounded / 1024;
    unit += 1;
    rounded = val < 100 ? Math.round(val * 10) / 10 : Math.round(val);
  }
  const numStr = rounded < 100 ? trimDecimal(rounded) : String(rounded);
  return `${numStr} ${BYTE_UNITS[unit] ?? 'TB'}`;
}

export function fmtPercent(ratio: number): string {
  if (!Number.isFinite(ratio)) {
    warnOnce('fmtPercent:invalid', `fmtPercent: invalid number ${String(ratio)}`);
    return DASH;
  }
  if (ratio > 0 && ratio < 0.005) return '<1%';
  const pct = Math.round(ratio * 100);
  return `${String(pct === 0 ? 0 : pct)}%`;
}

export function fmtAbsTime(iso: string, o?: { seconds?: boolean; zone?: boolean }): string {
  const ts = Date.parse(iso);
  if (Number.isNaN(ts)) {
    warnOnce(`fmtAbsTime:invalid:${iso}`, `fmtAbsTime: invalid ISO time "${iso}"`);
    return DASH;
  }
  const d = new Date(ts);
  const y = pad4(d.getFullYear());
  const mo = pad2(d.getMonth() + 1);
  const day = pad2(d.getDate());
  const hh = pad2(d.getHours());
  const mm = pad2(d.getMinutes());
  const showSec = o?.seconds ?? true;
  let out = showSec
    ? `${y}-${mo}-${day} ${hh}:${mm}:${pad2(d.getSeconds())}`
    : `${y}-${mo}-${day} ${hh}:${mm}`;
  if (o?.zone) {
    const offsetMin = -d.getTimezoneOffset();
    const sign = offsetMin >= 0 ? '+' : '-';
    const absMin = Math.abs(offsetMin);
    const offStr = `${sign}${pad2(Math.floor(absMin / 60))}:${pad2(absMin % 60)}`;
    out += fillFmt(zh['fmt.zone'], { offset: offStr });
  }
  return out;
}

function localDayNum(d: Date): number {
  return Math.floor(Date.UTC(d.getFullYear(), d.getMonth(), d.getDate()) / 86_400_000);
}

export function fmtRelTime(iso: string, now?: number): string {
  const ts = Date.parse(iso);
  if (Number.isNaN(ts)) {
    warnOnce(`fmtRelTime:invalid:${iso}`, `fmtRelTime: invalid ISO time "${iso}"`);
    return DASH;
  }
  const cur = now ?? clockNow();
  if (!Number.isFinite(cur)) {
    warnOnce('fmtRelTime:invalidNow', `fmtRelTime: invalid now ${String(cur)}`);
    return DASH;
  }
  const diff = cur - ts;
  if (diff < -5 * 60_000) {
    return fmtAbsTime(iso, { seconds: false });
  }
  if (diff < 45_000) {
    return zh['fmt.justNow'];
  }
  if (diff < 60 * 60_000) {
    const mins = Math.max(1, Math.floor(diff / 60_000));
    return fillFmt(zh['fmt.minAgo'], { n: mins });
  }
  if (diff < 24 * 60 * 60_000) {
    const hours = Math.floor(diff / 3_600_000);
    return fillFmt(zh['fmt.hourAgo'], { n: hours });
  }
  const tsDate = new Date(ts);
  const curDate = new Date(cur);
  const dayDiff = localDayNum(curDate) - localDayNum(tsDate);
  if (dayDiff === 1) {
    return zh['fmt.yesterday'];
  }
  if (dayDiff > 1 && dayDiff < 7) {
    return fillFmt(zh['fmt.dayAgo'], { n: dayDiff });
  }
  const y = pad4(tsDate.getFullYear());
  const mo = pad2(tsDate.getMonth() + 1);
  const day = pad2(tsDate.getDate());
  if (tsDate.getFullYear() === curDate.getFullYear()) {
    return `${mo}-${day}`;
  }
  return `${y}-${mo}-${day}`;
}
