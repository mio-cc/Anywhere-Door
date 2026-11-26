import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { fromWire, type Graph } from '@/lib/protocol';

const here = dirname(fileURLToPath(import.meta.url));
const examplesDir = resolve(here, '..', '..', '..', 'schema', 'examples');

export function memStorage(): Storage {
  const map = new Map<string, string>();
  return {
    get length() {
      return map.size;
    },
    clear() {
      map.clear();
    },
    getItem(key: string) {
      return map.get(key) ?? null;
    },
    key(index: number) {
      return Array.from(map.keys())[index] ?? null;
    },
    removeItem(key: string) {
      map.delete(key);
    },
    setItem(key: string, value: string) {
      map.set(key, value);
    },
  };
}

export type VectorName =
  'assign' | 'hash' | 'layout' | 'show_if' | 'op' | 'run_event' | 'graph_event' | 'session_event' | 'path';

// eslint-disable-next-line @typescript-eslint/no-unnecessary-type-parameters -- 测试夹具按 W.12 §4.2 签名支持泛型返回类型
export function loadVectors<T = unknown>(name: VectorName): readonly T[] {
  const raw = readFileSync(resolve(examplesDir, `${name}_vectors.json`), 'utf8');
  return JSON.parse(raw) as T[];
}

// 读 schema/examples/<name>.json 并经 fromWire('Graph', ...) 转成 camelCase 的 Graph（W.12 §4.2）
export function loadExample(name: string): Graph {
  const file = name.endsWith('.json') ? name : `${name}.json`;
  const raw: unknown = JSON.parse(readFileSync(resolve(examplesDir, file), 'utf8'));
  return fromWire<Graph>('Graph', raw);
}
