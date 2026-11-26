import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { ESLint, type Linter } from 'eslint';
import { describe, expect, test } from 'vitest';
import { createConfig } from '../../../eslint.config.js';
import { FORBIDDEN_TERMS } from './terms';
import { zh } from './zh';

const here = dirname(fileURLToPath(import.meta.url));
const webDir = resolve(here, '..', '..', '..');

const ALLOWED_PREFIXES = [
  'common',
  'fmt',
  'shell',
  'shortcut',
  'ops',
  'canvas',
  'top',
  'ls',
  'insp',
  'chat',
  'mon',
  'dock',
  'comp',
  'float',
  'appr',
  'notice',
  'menu',
  'ui',
  'home',
  'center',
  'set',
  'lib',
  'welcome',
  'tour',
  'pkg',
  'conn',
  'hist',
] as const;

const VALID_FORMATS = new Set(['count', 'tokens', 'cost', 'duration', 'bytes', 'percent', 'rel', 'abs']);
const KEY_RE = /^[a-z][a-zA-Z0-9]*(\.[a-z][a-zA-Z0-9]*){1,3}$/;
// 汉字与拉丁字母/数字之间必须有空格（占位符两侧与标点不查）
const CJK_ADJOINING_ALNUM_RE = /[\u4e00-\u9fff][A-Za-z0-9]|[A-Za-z0-9][\u4e00-\u9fff]/;

function stripPlaceholders(s: string): string {
  return s.replace(/\{[a-zA-Z][a-zA-Z0-9_]*(?::[a-z]+)?\}/g, ' ');
}

function validateEntry(key: string, val: string): string[] {
  const errs: string[] = [];
  if (!KEY_RE.test(key)) {
    errs.push(`键名格式不合法：${key}`);
  }
  const prefix = key.split('.')[0] ?? '';
  if (prefix === 'err' || prefix === 'assign') {
    errs.push(`err./assign. 不能出现在 zh.ts：${key}`);
  } else if (!(ALLOWED_PREFIXES as readonly string[]).includes(prefix)) {
    errs.push(`未知前缀：${key}`);
  }
  if (val === '' || val.trim() !== val) {
    errs.push(`值为空或首尾有空白：${key}`);
  }
  if (/[<>]/.test(val) && key !== 'fmt.lt1s') {
    errs.push(`值包含 < 或 >：${key}`);
  }
  if (/[!！]/.test(val)) {
    errs.push(`值包含感叹号：${key}`);
  }
  // 花括号成对与占位符格式合法
  let depth = 0;
  for (let i = 0; i < val.length; i += 1) {
    if (val[i] === '{') depth += 1;
    else if (val[i] === '}') depth -= 1;
    if (depth < 0 || depth > 1) {
      errs.push(`花括号不成对：${key}`);
      break;
    }
  }
  if (depth !== 0) {
    errs.push(`花括号未闭合：${key}`);
  }
  for (const m of val.matchAll(/\{([^}]*)\}/g)) {
    const inner = m[1] ?? '';
    const pm = /^([a-zA-Z][a-zA-Z0-9_]*)(?::([a-z]+))?$/.exec(inner);
    if (!pm) {
      errs.push(`占位符写法不合法：{${inner}} in ${key}`);
    } else if (pm[2] !== undefined && !VALID_FORMATS.has(pm[2])) {
      errs.push(`未知格式名：${pm[2]} in ${key}`);
    }
  }
  if (CJK_ADJOINING_ALNUM_RE.test(stripPlaceholders(val))) {
    errs.push(`汉字与数字/字母之间缺少空格：${key} -> ${val}`);
  }
  for (const rule of FORBIDDEN_TERMS) {
    if (rule.pattern.test(val)) {
      errs.push(`包含禁用术语「${rule.show}」（应为「${rule.use}」）：${key}`);
    }
  }
  return errs;
}

describe('W.10 文案表与中文字面量 lint', () => {
  test('W10-T13 文案表检查（正例与反例）', () => {
    const entries = Object.entries(zh);
    expect(entries.length).toBeGreaterThan(0);

    // 1) 正例：初始 zh.ts 全部通过，且区段顺序与 §4.3 一致
    let lastPrefixIdx = -1;
    for (const [k, v] of entries) {
      expect(validateEntry(k, v), `zh[${k}]`).toEqual([]);
      const idx = (ALLOWED_PREFIXES as readonly string[]).indexOf(k.split('.')[0]!);
      expect(idx, `区段顺序错误：${k}`).toBeGreaterThanOrEqual(lastPrefixIdx);
      lastPrefixIdx = idx;
    }

    // 源码格式：每个键值行都是单引号字面量，便于字体子集扫描（§4.2）
    const rawZh = readFileSync(resolve(here, 'zh.ts'), 'utf8');
    for (const line of rawZh.split('\n')) {
      const trimmed = line.trim();
      if (trimmed.startsWith("'")) {
        expect(trimmed, `单行单引号字面量检查：${trimmed}`).toMatch(/^'[a-zA-Z0-9.]+':\s*'[^']+',$/);
      }
    }

    // 2) 反例逐条触发失败
    expect(validateEntry('badprefix.x', '正常').length).toBeGreaterThan(0);
    expect(validateEntry('err.custom', '正常').length).toBeGreaterThan(0);
    expect(validateEntry('common.bang', '出错了！').length).toBeGreaterThan(0);
    expect(validateEntry('common.term', '打开子系统').length).toBeGreaterThan(0);
    expect(validateEntry('common.space', '运行12秒').length).toBeGreaterThan(0);
    expect(validateEntry('common.fmt', '共 {n:unknown}').length).toBeGreaterThan(0);
  });

  test('W10-T14 中文字面量 lint 样例检查', async () => {
    const eslint = new ESLint({
      cwd: webDir,
      overrideConfigFile: true,
      overrideConfig: createConfig({ enableTypeChecked: false }) as Linter.Config[],
    });

    const okCode = readFileSync(resolve(webDir, 'tests/lint/no-cjk-literal/ok/src/ok.tsx'), 'utf8');
    const [okRes] = await eslint.lintText(okCode, { filePath: resolve(webDir, 'src/ok.tsx') });
    expect(okRes?.messages ?? []).toEqual([]);

    for (const bad of ['bad-str.ts', 'bad-tpl.ts', 'bad-jsx.tsx']) {
      const code = readFileSync(resolve(webDir, 'tests/lint/no-cjk-literal/bad/src', bad), 'utf8');
      const [res] = await eslint.lintText(code, { filePath: resolve(webDir, 'src', bad) });
      const rules = (res?.messages ?? []).map((m) => m.ruleId);
      expect(rules, `${bad} 必须恰好只报 hx/no-cjk-literal`).toEqual(['hx/no-cjk-literal']);
    }
  });
});
