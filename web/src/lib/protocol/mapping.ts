import { WIRE_TYPES, type WireNode, type WireTypeName } from './generated/wire';

export type WireQuery = Readonly<Record<string, string | number | boolean | readonly string[] | undefined>>;

type CamelKey<S extends string> = S extends `${infer H}_${infer T}` ? `${H}${Capitalize<CamelKey<T>>}` : S;

export type FromWire<T> = T extends string | number | boolean | null | undefined
  ? T
  : T extends readonly (infer E)[]
    ? readonly FromWire<E>[]
    : T extends object
      ? string extends keyof T
        ? { readonly [k: string]: FromWire<T[string & keyof T]> }
        : { [K in keyof T as K extends string ? CamelKey<K> : K]: FromWire<T[K]> }
      : T;

const camelCache = new Map<string, string>();
const snakeCache = new Map<string, string>();

export function camelKey(k: string): string {
  const hit = camelCache.get(k);
  if (hit !== undefined) return hit;
  const out = k.replace(/_([a-z])/g, (_, ch: string) => ch.toUpperCase());
  camelCache.set(k, out);
  return out;
}

export function snakeKey(k: string): string {
  const hit = snakeCache.get(k);
  if (hit !== undefined) return hit;
  const out = k.replace(/[A-Z]/g, (ch) => `_${ch.toLowerCase()}`);
  snakeCache.set(k, out);
  return out;
}

function hasOwn(obj: object, key: string): boolean {
  return Object.prototype.hasOwnProperty.call(obj, key);
}

// 用 defineProperty 写自有数据属性，避免 "__proto__" 键触发原型 setter 丢失数据（P.09 K302）。
function setOwn(target: Record<string, unknown>, key: string, value: unknown): void {
  Object.defineProperty(target, key, {
    value,
    writable: true,
    enumerable: true,
    configurable: true,
  });
}

function isPlainObject(v: unknown): v is Record<string, unknown> {
  if (typeof v !== 'object' || v === null) return false;
  const proto: unknown = Object.getPrototypeOf(v);
  return proto === Object.prototype || proto === null;
}

function lookupBranch(
  branches: { readonly [value: string]: WireTypeName },
  discVal: unknown,
): WireTypeName | undefined {
  if (typeof discVal !== 'string' || !hasOwn(branches, discVal)) return undefined;
  return branches[discVal];
}

function lookupField(fields: { readonly [key: string]: WireNode }, key: string): WireNode {
  if (!hasOwn(fields, key)) return 'raw';
  return fields[key] ?? 'raw';
}

function walkFrom(node: WireNode, v: unknown): unknown {
  if (node === 'raw') return v;
  if ('list' in node) {
    if (!Array.isArray(v)) return v;
    const elem = node.list;
    return v.map((item) => walkFrom(elem, item));
  }
  if ('map' in node) {
    if (!isPlainObject(v)) return v;
    const valNode = node.map;
    const out: Record<string, unknown> = {};
    for (const [k, val] of Object.entries(v)) {
      setOwn(out, k, walkFrom(valNode, val));
    }
    return out;
  }
  return fromWireObject(node.t, v);
}

function fromWireObject(type: WireTypeName, v: unknown): unknown {
  if (!isPlainObject(v)) return v;
  const meta = WIRE_TYPES[type];
  const union = meta.union;
  if (union && union.field === undefined) {
    const discVal = hasOwn(v, union.disc) ? v[union.disc] : undefined;
    const branch = lookupBranch(union.branches, discVal);
    if (branch !== undefined) {
      return fromWireObject(branch, v);
    }
    const fallback: Record<string, unknown> = {};
    for (const [k, val] of Object.entries(v)) {
      setOwn(fallback, camelKey(k), val);
    }
    return fallback;
  }

  const out: Record<string, unknown> = {};
  for (const [k, val] of Object.entries(v)) {
    let fieldNode: WireNode = 'raw';
    if (union && union.field === k) {
      const discVal = hasOwn(v, union.disc) ? v[union.disc] : undefined;
      const branch = lookupBranch(union.branches, discVal);
      if (branch !== undefined) {
        fieldNode = { t: branch };
      }
    } else {
      fieldNode = lookupField(meta.fields, k);
    }
    setOwn(out, camelKey(k), walkFrom(fieldNode, val));
  }
  return out;
}

function walkTo(node: WireNode, v: unknown): unknown {
  if (node === 'raw') return v;
  if ('list' in node) {
    if (!Array.isArray(v)) return v;
    const elem = node.list;
    return v.map((item) => walkTo(elem, item));
  }
  if ('map' in node) {
    if (!isPlainObject(v)) return v;
    const valNode = node.map;
    const out: Record<string, unknown> = {};
    for (const [k, val] of Object.entries(v)) {
      if (val === undefined) continue;
      setOwn(out, k, walkTo(valNode, val));
    }
    return out;
  }
  return toWireObject(node.t, v);
}

function toWireObject(type: WireTypeName, v: unknown): unknown {
  if (!isPlainObject(v)) return v;
  const meta = WIRE_TYPES[type];
  const union = meta.union;
  if (union && union.field === undefined) {
    const discKey = camelKey(union.disc);
    const discVal = hasOwn(v, discKey) ? v[discKey] : undefined;
    const branch = lookupBranch(union.branches, discVal);
    if (branch !== undefined) {
      return toWireObject(branch, v);
    }
    const fallback: Record<string, unknown> = {};
    for (const [k, val] of Object.entries(v)) {
      if (val === undefined) continue;
      setOwn(fallback, snakeKey(k), val);
    }
    return fallback;
  }

  const out: Record<string, unknown> = {};
  for (const [k, val] of Object.entries(v)) {
    if (val === undefined) continue;
    const sk = snakeKey(k);
    let fieldNode: WireNode = 'raw';
    if (union && union.field === sk) {
      const discKey = camelKey(union.disc);
      const discVal = hasOwn(v, discKey) ? v[discKey] : undefined;
      const branch = lookupBranch(union.branches, discVal);
      if (branch !== undefined) {
        fieldNode = { t: branch };
      }
    } else {
      fieldNode = lookupField(meta.fields, sk);
    }
    setOwn(out, sk, walkTo(fieldNode, val));
  }
  return out;
}

// eslint-disable-next-line @typescript-eslint/no-unnecessary-type-parameters -- W.04.03 §4 规定的泛型签名
export function fromWire<T>(type: WireTypeName, v: unknown): T {
  return fromWireObject(type, v) as T;
}

export function toWire(type: WireTypeName, v: unknown): unknown {
  return toWireObject(type, v);
}

export function toWireQuery(q: WireQuery): URLSearchParams {
  const params = new URLSearchParams();
  for (const [k, v] of Object.entries(q)) {
    if (v === undefined) continue;
    const sk = snakeKey(k);
    if (Array.isArray(v)) {
      params.set(sk, v.join(','));
    } else {
      params.set(sk, String(v));
    }
  }
  return params;
}
