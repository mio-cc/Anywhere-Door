import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, expectTypeOf, test } from 'vitest';
import { WIRE_TYPES, type WireTypeName } from './generated/wire';
import type * as gen from './generated/types';
import * as proto from './index';
import {
  camelKey,
  fromWire,
  snakeKey,
  toWire,
  toWireQuery,
  type EventOf,
  type FromWire,
  type Graph,
  type Node,
  type NodeDef,
  type OpOf,
} from './index';

const here = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(here, '..', '..', '..', '..');
const schemaDir = resolve(repoRoot, 'schema');
const examplesDir = resolve(schemaDir, 'examples');

interface ManifestEntry {
  file: string;
  schema: string;
  valid: boolean;
}

function stripComment(line: string): string {
  const idx = line.indexOf(' #');
  return (idx >= 0 ? line.slice(0, idx) : line).trim();
}

function unquote(s: string): string {
  if ((s.startsWith('"') && s.endsWith('"')) || (s.startsWith("'") && s.endsWith("'"))) {
    return s.slice(1, -1);
  }
  return s;
}

// 极简解析 schema/examples/manifest.yaml（两层 list-of-map，不引入 yaml 依赖，K300）
function parseManifestYaml(text: string): ManifestEntry[] {
  const out: ManifestEntry[] = [];
  let cur: Partial<ManifestEntry> | null = null;
  for (const rawLine of text.split(/\r?\n/)) {
    const trimmed = rawLine.trim();
    if (trimmed === '' || trimmed.startsWith('#')) continue;
    const clean = stripComment(rawLine);
    if (clean.startsWith('- ')) {
      if (cur?.file && cur.schema !== undefined && cur.valid !== undefined) {
        out.push(cur as ManifestEntry);
      }
      cur = {};
      const rest = clean.slice(2).trim();
      const sep = rest.indexOf(':');
      if (sep > 0) {
        const k = rest.slice(0, sep).trim();
        const v = unquote(rest.slice(sep + 1).trim());
        if (k === 'file') cur.file = v;
      }
      continue;
    }
    if (!cur) continue;
    const sep = clean.indexOf(':');
    if (sep <= 0) continue;
    const k = clean.slice(0, sep).trim();
    const v = unquote(clean.slice(sep + 1).trim());
    if (k === 'file') cur.file = v;
    else if (k === 'schema') cur.schema = v;
    else if (k === 'valid') cur.valid = v === 'true';
  }
  if (cur?.file && cur.schema !== undefined && cur.valid !== undefined) {
    out.push(cur as ManifestEntry);
  }
  return out;
}

// 极简解析 schema/builtin/aliases.yaml 的 roots 段
function parseAliasRoots(text: string): Record<string, WireTypeName> {
  const roots: Record<string, WireTypeName> = {};
  let inRoots = false;
  for (const rawLine of text.split(/\r?\n/)) {
    const trimmed = rawLine.trim();
    if (trimmed === '' || trimmed.startsWith('#')) continue;
    if (!rawLine.startsWith(' ')) {
      inRoots = trimmed.startsWith('roots:');
      continue;
    }
    if (!inRoots) continue;
    const clean = stripComment(trimmed);
    const sep = clean.indexOf(':');
    if (sep > 0) {
      const k = clean.slice(0, sep).trim();
      const v = clean.slice(sep + 1).trim() as WireTypeName;
      roots[k] = v;
    }
  }
  return roots;
}

function resolveWireType(schemaRef: string, roots: Record<string, WireTypeName>): WireTypeName {
  const hashIdx = schemaRef.indexOf('#/$defs/');
  if (hashIdx >= 0) {
    return schemaRef.slice(hashIdx + '#/$defs/'.length) as WireTypeName;
  }
  const root = roots[schemaRef];
  if (!root) {
    throw new Error(`无法从 aliases.yaml 解析 schema 根类型：${schemaRef}`);
  }
  return root;
}

describe('W.04.03 字段映射与协议入口', () => {
  test('W0403-T1 示例清单与四份测试向量往返无损', () => {
    const roots = parseAliasRoots(readFileSync(resolve(schemaDir, 'builtin', 'aliases.yaml'), 'utf8'));
    const entries = parseManifestYaml(readFileSync(resolve(examplesDir, 'manifest.yaml'), 'utf8'));
    const validEntries = entries.filter((e) => e.valid);
    expect(validEntries.length).toBeGreaterThan(15);

    for (const entry of validEntries) {
      const wireType = resolveWireType(entry.schema, roots);
      const raw: unknown = JSON.parse(readFileSync(resolve(examplesDir, entry.file), 'utf8'));
      const camel = fromWire<unknown>(wireType, raw);
      const back = toWire(wireType, camel);
      expect(back, `manifest ${entry.file} (${wireType})`).toEqual(raw);
    }

    const opVectors = JSON.parse(readFileSync(resolve(examplesDir, 'op_vectors.json'), 'utf8')) as Array<{
      name: string;
      kind: string;
      payload: unknown;
      valid: boolean;
    }>;
    let opChecked = 0;
    for (const v of opVectors) {
      if (!v.valid) continue;
      const rawOp = { kind: v.kind, payload: v.payload };
      expect(toWire('Op', fromWire<unknown>('Op', rawOp)), `op_vectors ${v.name}`).toEqual(rawOp);
      opChecked += 1;
    }
    expect(opChecked).toBeGreaterThan(10);

    const eventFiles: Array<[string, WireTypeName]> = [
      ['run_event_vectors.json', 'RunEvent'],
      ['graph_event_vectors.json', 'GraphEvent'],
      ['session_event_vectors.json', 'SessionEvent'],
    ];
    for (const [file, wireType] of eventFiles) {
      const vecs = JSON.parse(readFileSync(resolve(examplesDir, file), 'utf8')) as Array<{
        name: string;
        event: unknown;
        valid: boolean;
      }>;
      let checked = 0;
      for (const v of vecs) {
        if (!v.valid) continue;
        expect(toWire(wireType, fromWire<unknown>(wireType, v.event)), `${file} ${v.name}`).toEqual(v.event);
        checked += 1;
      }
      expect(checked).toBeGreaterThan(0);
    }
  });

  test('W0403-T2 开放对象与 raw 位置原样保留同一引用', () => {
    const params = { max_rounds: 3, aB: { c_d: 1 } };
    const nodeWire = { id: '01J00000000000000000000001', type: '循环', label: 'loop', params };
    const nodeCamel = fromWire<{ params: Record<string, unknown> }>('Node', nodeWire);
    expect(nodeCamel.params).toBe(params);
    expect(nodeCamel.params).toEqual({ max_rounds: 3, aB: { c_d: 1 } });

    const toVal = { a_b: 1 };
    const opWire = {
      kind: 'set_param',
      payload: { node_id: '01J00000000000000000000001', key: 'x', to: toVal },
    };
    const opCamel = fromWire<{ payload: { nodeId: string; to: Record<string, unknown> } }>('Op', opWire);
    expect(opCamel.payload.nodeId).toBe('01J00000000000000000000001');
    expect(opCamel.payload.to).toBe(toVal);

    const ndWire = {
      name: 'Agent',
      category: '推理',
      params: {
        max_rounds: {
          type: 'integer',
          enum_labels: { a_b: 'A' },
          ui: { show_if: 'x == 1', group: '常用' },
        },
      },
    };
    const ndCamel = fromWire<{
      params: Record<string, { enumLabels: Record<string, string>; ui: { showIf: string } }>;
    }>('NodeDef', ndWire);
    expect(Object.keys(ndCamel.params)).toEqual(['max_rounds']);
    expect(ndCamel.params['max_rounds']?.ui.showIf).toBe('x == 1');
    expect(ndCamel.params['max_rounds']?.enumLabels).toEqual({ a_b: 'A' });

    const mandateCamel = fromWire<{ overrides: { toolsMaxRisk: string } }>('Mandate', {
      overrides: { tools_max_risk: 'L2' },
    });
    expect(mandateCamel.overrides.toolsMaxRisk).toBe('L2');

    const subRefOverrides = { max_rounds: 5 };
    const subRefCamel = fromWire<{ overrides: Record<string, unknown> }>('SubgraphRef', {
      name: 'sub',
      version: '1.0.0',
      overrides: subRefOverrides,
    });
    expect(subRefCamel.overrides).toBe(subRefOverrides);
  });

  test('W0403-T3 映射键原样保留且 toWire 可还原', () => {
    const sessRaw = { orchestrator_effort: { gpt_4o: 'high' } };
    const sessCamel = fromWire<{ orchestratorEffort: Record<string, string> }>('Session', sessRaw);
    expect(sessCamel).toEqual({ orchestratorEffort: { gpt_4o: 'high' } });
    expect(toWire('Session', sessCamel)).toEqual(sessRaw);

    const propRaw = { decisions: { '01J0000000000000000000000A': 'accepted' } };
    const propCamel = fromWire<{ decisions: Record<string, string> }>('Proposal', propRaw);
    expect(propCamel).toEqual({ decisions: { '01J0000000000000000000000A': 'accepted' } });
    expect(toWire('Proposal', propCamel)).toEqual(propRaw);

    const appliedRaw = { version: 2, applied: [], id_map: { 'tmp:1': '01J0000000000000000000000B' } };
    const appliedCamel = fromWire<{ idMap: Record<string, string> }>('OpsApplied', appliedRaw);
    expect(appliedCamel.idMap).toEqual({ 'tmp:1': '01J0000000000000000000000B' });
    expect(toWire('OpsApplied', appliedCamel)).toEqual(appliedRaw);

    // map 中包含 JSON 自有 "__proto__" 数据键时必须保留为自有键且双向序列化无损（P.09 K302）
    const protoParamJson =
      '{"name":"Agent","category":"推理","params":{"__proto__":{"type":"string","ui":{"show_if":"x == 1"}}}}';
    const protoParamRaw: unknown = JSON.parse(protoParamJson);
    const protoParamCamel = fromWire<{ params: Record<string, { ui: { showIf: string } }> }>(
      'NodeDef',
      protoParamRaw,
    );
    expect(Object.prototype.hasOwnProperty.call(protoParamCamel.params, '__proto__')).toBe(true);
    expect(Object.getPrototypeOf(protoParamCamel.params)).toBe(Object.prototype);
    expect(protoParamCamel.params['__proto__']?.ui.showIf).toBe('x == 1');
    const protoParamBack = toWire('NodeDef', protoParamCamel) as { params: Record<string, unknown> };
    expect(Object.prototype.hasOwnProperty.call(protoParamBack.params, '__proto__')).toBe(true);
    expect(JSON.stringify(protoParamBack)).toBe(protoParamJson);
  });

  test('W0403-T4 字段联合与整体联合转换（含继承键/未知判别值安全回退）', () => {
    const evWire = {
      seq: 3,
      kind: 'node.state',
      run_id: '01J00000000000000000000001',
      data: { exec_key: 'k1', cause_seq: 2, to: 'running' },
    };
    const evCamel = fromWire<{ runId: string; data: { execKey: string; causeSeq: number; to: string } }>(
      'RunEvent',
      evWire,
    );
    expect(evCamel.runId).toBe('01J00000000000000000000001');
    expect(evCamel.data).toEqual({ execKey: 'k1', causeSeq: 2, to: 'running' });
    expect(toWire('RunEvent', evCamel)).toEqual(evWire);

    const opWire = {
      kind: 'remove_nodes',
      payload: { node_ids: ['01J00000000000000000000001'] },
    };
    const opCamel = fromWire<{ payload: { nodeIds: string[] } }>('Op', opWire);
    expect(opCamel.payload.nodeIds).toEqual(['01J00000000000000000000001']);
    expect(toWire('Op', opCamel)).toEqual(opWire);

    const grpWire = {
      kind: 'group',
      payload: { node_ids: ['01J00000000000000000000001'], new_id: '01J00000000000000000000002' },
    };
    const grpCamel = fromWire<{ payload: { nodeIds: string[]; newId: string } }>('Op', grpWire);
    expect(grpCamel.payload.newId).toBe('01J00000000000000000000002');
    expect(toWire('Op', grpCamel)).toEqual(grpWire);

    // 整体联合：MessageBlock / MessageRef / Impl
    const blockWire = {
      kind: 'approval',
      approval_id: '01J00000000000000000000003',
      approval_kind: 'tool',
      summary: 's',
    };
    const blockCamel = fromWire<{ approvalId: string; approvalKind: string }>('MessageBlock', blockWire);
    expect(blockCamel.approvalId).toBe('01J00000000000000000000003');
    expect(blockCamel.approvalKind).toBe('tool');
    expect(toWire('MessageBlock', blockCamel)).toEqual(blockWire);

    const refWire = {
      kind: 'node',
      graph_id: '01J00000000000000000000004',
      node_id: '01J00000000000000000000005',
    };
    const refCamel = fromWire<{ graphId: string; nodeId: string }>('MessageRef', refWire);
    expect(refCamel).toEqual({
      kind: 'node',
      graphId: '01J00000000000000000000004',
      nodeId: '01J00000000000000000000005',
    });
    expect(toWire('MessageRef', refCamel)).toEqual(refWire);

    const implWire = { type: 'http', base_url: 'https://example.com' };
    const implCamel = fromWire<{ baseUrl: string }>('Impl', implWire);
    expect(implCamel.baseUrl).toBe('https://example.com');
    expect(toWire('Impl', implCamel)).toEqual(implWire);

    // 未知判别值为 Object 继承键名（constructor）以及未知字段名为 constructor 时不能抛错，按 raw 回退（P.09 K302）
    const unknownUnionField = {
      kind: 'constructor',
      payload: { inner_key: 1 },
      constructor: { raw_obj: true },
    };
    const unknownUnionCamel = fromWire<Record<string, unknown>>('Op', unknownUnionField);
    expect(unknownUnionCamel['kind']).toBe('constructor');
    expect(unknownUnionCamel['payload']).toBe(unknownUnionField.payload);
    expect(unknownUnionCamel['constructor']).toBe(unknownUnionField.constructor);
    expect(toWire('Op', unknownUnionCamel)).toEqual(unknownUnionField);

    const unknownWholeUnion = { kind: 'constructor', extra_field: { keep_raw: 1 } };
    const unknownWholeCamel = fromWire<Record<string, unknown>>('MessageBlock', unknownWholeUnion);
    expect(unknownWholeCamel['kind']).toBe('constructor');
    expect(unknownWholeCamel['extraField']).toBe(unknownWholeUnion.extra_field);
    expect(toWire('MessageBlock', unknownWholeCamel)).toEqual(unknownWholeUnion);
  });

  test('W0403-T5 WIRE_TYPES 全部字段名往返一致', () => {
    const allFields = new Set<string>();
    for (const meta of Object.values(WIRE_TYPES)) {
      for (const k of Object.keys(meta.fields)) {
        allFields.add(k);
      }
    }
    expect(allFields.size).toBeGreaterThan(100);
    for (const k of allFields) {
      expect(snakeKey(camelKey(k)), `field ${k}`).toBe(k);
    }
  });

  test('W0403-T6 toWire 省略 undefined、保留 null 及 toWireQuery', () => {
    const res = toWire('PayloadSetSubgraphRef', {
      nodeId: '01J00000000000000000000001',
      to: null,
      extra: undefined,
    }) as Record<string, unknown>;
    expect(res).toEqual({ node_id: '01J00000000000000000000001', to: null });
    expect('extra' in res).toBe(false);

    const qs = toWireQuery({
      sinceVersion: 12,
      status: ['pending', 'running'],
      includeBlobs: true,
      omitted: undefined,
    });
    expect(qs.get('since_version')).toBe('12');
    expect(qs.get('status')).toBe('pending,running');
    expect(qs.get('include_blobs')).toBe('true');
    expect(qs.has('omitted')).toBe(false);
  });

  test('W0403-T7 非普通对象 Blob 与 FormData 原样返回', () => {
    const blob = new Blob(['hello'], { type: 'text/plain' });
    const fd = new FormData();
    fd.append('k', 'v');
    expect(fromWire<Blob>('Graph', blob)).toBe(blob);
    expect(toWire('Graph', blob)).toBe(blob);
    expect(fromWire<FormData>('Graph', fd)).toBe(fd);
    expect(toWire('Graph', fd)).toBe(fd);
  });

  test('W0403-T8 FromWire 类型推导与判别联合收窄', () => {
    expectTypeOf<FromWire<gen.GraphRead>['registryHash']>().toEqualTypeOf<string>();
    expectTypeOf<Extract<keyof Graph, 'registryHash'>>().toEqualTypeOf<never>();
    expectTypeOf<FromWire<gen.Node>['params']>().toEqualTypeOf<
      { readonly [k: string]: unknown } | undefined
    >();
    expectTypeOf<NonNullable<Node['params']>>().toEqualTypeOf<{ readonly [k: string]: unknown }>();
    expectTypeOf<NonNullable<NonNullable<NodeDef['params']>[string]['ui']>['showIf']>().toEqualTypeOf<
      string | undefined
    >();
    expectTypeOf<EventOf<'node.state'>['data']['backoffMs']>().toEqualTypeOf<number | undefined>();
    expectTypeOf<OpOf<'set_param'>['payload']['node']>().toEqualTypeOf<gen.Ulid>();
    expect(true).toBe(true);
  });

  test('W0403-T9 types.ts 与 index.ts 覆盖全部生成类型、常量与数据表', () => {
    const genTypesSrc = readFileSync(resolve(here, 'generated', 'types.ts'), 'utf8');
    const genConstsSrc = readFileSync(resolve(here, 'generated', 'consts.ts'), 'utf8');
    const genTablesSrc = readFileSync(resolve(here, 'generated', 'tables.ts'), 'utf8');
    const aliasSrc = readFileSync(resolve(here, 'types.ts'), 'utf8');

    const genTypeNames = Array.from(
      new Set(Array.from(genTypesSrc.matchAll(/^export (?:type|interface) (\w+)/gm), (m) => m[1]!)),
    ).sort();
    const aliasNames = new Set(Array.from(aliasSrc.matchAll(/^export type (\w+)\s*=/gm), (m) => m[1]!));
    const missingTypes = genTypeNames.filter((n) => !aliasNames.has(n));
    expect(missingTypes, 'types.ts 缺少生成类型别名').toEqual([]);

    const constObjects = [
      ...Array.from(genConstsSrc.matchAll(/^export const (\w+)/gm), (m) => m[1]!),
      ...Array.from(genTypesSrc.matchAll(/^export const (\w+)/gm), (m) => m[1]!),
      ...Array.from(genTablesSrc.matchAll(/^export const (\w+)/gm), (m) => m[1]!),
    ].sort();
    const protoExports = proto as Record<string, unknown>;
    const missingExports = constObjects.filter((n) => protoExports[n] === undefined);
    expect(missingExports, 'index.ts 缺少常量或表导出').toEqual([]);
  });
});
