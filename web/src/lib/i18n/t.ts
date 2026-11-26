import {
  fmtAbsTime,
  fmtBytes,
  fmtCost,
  fmtCount,
  fmtDuration,
  fmtPercent,
  fmtRelTime,
  fmtTokens,
} from './format';
import { ASSIGN, ERRORS } from './generated/messages';
import { zh } from './zh';

export type TextKey = keyof typeof zh;
export type Vars = Readonly<Record<string, string | number>>;
export type Translate = {
  (key: TextKey, vars?: Vars): string;
  dyn(key: string, vars?: Vars): string;
};

const PLACEHOLDER_RE = /\{([a-zA-Z][a-zA-Z0-9_]*)(?::([a-z]+))?\}/g;
const warnedKeys = new Set<string>();
const erroredKeys = new Set<string>();

function isDev(): boolean {
  return (
    typeof import.meta !== 'undefined' &&
    (import.meta as { env?: { MODE?: string } }).env?.MODE !== 'production' &&
    (typeof process === 'undefined' || process.env['NODE_ENV'] !== 'production')
  );
}

function warnOnce(id: string, msg: string): void {
  if (!isDev() || warnedKeys.has(id)) return;
  warnedKeys.add(id);
  console.warn(msg);
}

function errorOnce(id: string, msg: string): void {
  if (erroredKeys.has(id)) return;
  erroredKeys.add(id);
  console.error(msg);
}

function snakeToCamel(s: string): string {
  return s.replace(/_([a-z0-9])/g, (_, ch: string) => ch.toUpperCase());
}

function camelToSnake(s: string): string {
  return s.replace(/[A-Z]/g, (ch) => `_${ch.toLowerCase()}`);
}

function lookupTemplate(key: string): string | undefined {
  if (Object.prototype.hasOwnProperty.call(zh, key)) {
    return (zh as Record<string, string>)[key];
  }
  if (key.startsWith('err.')) {
    const code = key.slice(4);
    if (Object.prototype.hasOwnProperty.call(ERRORS, code)) {
      return ERRORS[code];
    }
  }
  if (key.startsWith('assign.')) {
    const rule = key.slice(7);
    if (Object.prototype.hasOwnProperty.call(ASSIGN, rule)) {
      return ASSIGN[rule];
    }
  }
  return undefined;
}

function lookupVar(vars: Vars | undefined, name: string): string | number | undefined {
  if (!vars) return undefined;
  if (Object.prototype.hasOwnProperty.call(vars, name)) return vars[name];
  const camel = snakeToCamel(name);
  if (camel !== name && Object.prototype.hasOwnProperty.call(vars, camel)) return vars[camel];
  const snake = camelToSnake(name);
  if (snake !== name && Object.prototype.hasOwnProperty.call(vars, snake)) return vars[snake];
  return undefined;
}

function applyFormat(key: string, name: string, fmt: string, val: string | number): string {
  switch (fmt) {
    case 'count':
      if (typeof val === 'number') return fmtCount(val);
      break;
    case 'tokens':
      if (typeof val === 'number') return fmtTokens(val);
      break;
    case 'cost':
      if (typeof val === 'number') return fmtCost(val);
      break;
    case 'duration':
      if (typeof val === 'number') return fmtDuration(val);
      break;
    case 'bytes':
      if (typeof val === 'number') return fmtBytes(val);
      break;
    case 'percent':
      if (typeof val === 'number') return fmtPercent(val);
      break;
    case 'rel':
      if (typeof val === 'string') return fmtRelTime(val);
      break;
    case 'abs':
      if (typeof val === 'string') return fmtAbsTime(val);
      break;
    default:
      break;
  }
  warnOnce(`fmt:${key}:${name}:${fmt}`, `i18n: invalid format "${fmt}" for "${key}" (${typeof val})`);
  return String(val);
}

export function renderTemplate(key: string, tpl: string, vars?: Vars): string {
  if (!tpl.includes('{')) {
    return tpl;
  }
  return tpl.replace(PLACEHOLDER_RE, (match, name: string, fmt: string | undefined) => {
    const val = lookupVar(vars, name);
    if (val === undefined) {
      warnOnce(`var:${key}:${name}`, `i18n: missing var "${name}" for key "${key}"`);
      return match;
    }
    if (!fmt) {
      return String(val);
    }
    return applyFormat(key, name, fmt, val);
  });
}

function translate(key: string, vars?: Vars): string {
  const tpl = lookupTemplate(key);
  if (tpl === undefined) {
    if (isDev()) {
      warnOnce(`missing:${key}`, `i18n: missing key "${key}"`);
      return `\u27E6${key}\u27E7`;
    }
    errorOnce(`missing:${key}`, `i18n: missing key "${key}"`);
    return key;
  }
  return renderTemplate(key, tpl, vars);
}

const tImpl = ((key: TextKey, vars?: Vars) => translate(key, vars)) as Translate;
tImpl.dyn = (key: string, vars?: Vars) => translate(key, vars);

export const t: Translate = tImpl;

export function useT(): Translate {
  return t;
}

export function hasText(key: string): boolean {
  return lookupTemplate(key) !== undefined;
}
