import { describe, expect, test } from 'vitest';
import { expectConsole } from '@tests/kit';
import { hasText, setClock, t, useT } from './index';
import { renderTemplate } from './t';

describe('W.10 翻译与占位符', () => {
  test('W10-T1 取文案与 useT 稳定性', () => {
    expect(t('common.cancel')).toBe('取消');
    expect(t.dyn('err.internal')).toContain('程序出了问题');
    expect(t.dyn('assign.C2', { dst_name: '上下文包', producer: '上下文装配' })).toBe(
      '上下文包只能来自上下文装配节点',
    );
    expect(hasText('common.ok')).toBe(true);
    expect(hasText('err.internal')).toBe(true);
    expect(hasText('assign.C2')).toBe(true);
    expect(hasText('not.exists')).toBe(false);
    expect(useT()).toBe(useT());
    expect(useT()).toBe(t);
  });

  test('W10-T2 占位符与 8 种格式化', () => {
    setClock({ now: () => Date.parse('2026-01-15T10:05:00+08:00'), frozen: true });

    expect(renderTemplate('test.raw', '端口 {n}', { n: 18080 })).toBe('端口 18080');
    expect(renderTemplate('test.count', '已选中 {n:count} 个节点', { n: 12345 })).toBe(
      '已选中 1.2 万 个节点',
    );
    expect(renderTemplate('test.tokens', '{n:tokens} tokens', { n: 12345 })).toBe('1.2 万 tokens');
    expect(renderTemplate('test.cost', '本次运行 {c:cost}', { c: 420_000 })).toBe('本次运行 ¥0.42');
    expect(renderTemplate('test.duration', '已运行 {ms:duration}', { ms: 185_000 })).toBe(
      '已运行 3 分 05 秒',
    );
    expect(renderTemplate('test.bytes', '占用 {b:bytes}', { b: 1_288_490_189 })).toBe('占用 1.2 GB');
    expect(renderTemplate('test.percent', '已用 {r:percent}', { r: 0.724 })).toBe('已用 72%');
    expect(renderTemplate('test.rel', '更新于 {at:rel}', { at: '2026-01-15T10:03:00+08:00' })).toBe(
      '更新于 2 分钟前',
    );
    const absRes = renderTemplate('test.abs', '开始于 {at:abs}', { at: '2026-01-15T02:03:05Z' });
    expect(absRes).toMatch(/^开始于 \d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/);
    expect(renderTemplate('test.snake', '落后 {head_version}', { headVersion: 7 })).toBe('落后 7');
  });

  test('W10-T3 缺键与缺变量处理', () => {
    expectConsole('warn', 'missing key "x.y"', 1);
    expect(t.dyn('x.y')).toBe('⟦x.y⟧');
    expect(t.dyn('x.y')).toBe('⟦x.y⟧');
    expect(t.dyn('x.y')).toBe('⟦x.y⟧');

    expectConsole('warn', 'missing var "label"', 1);
    expect(t('ops.precheck.frozen')).toBe('「{label}」已冻结。先解除冻结再修改');

    const prevEnv = process.env['NODE_ENV'];
    try {
      process.env['NODE_ENV'] = 'production';
      expectConsole('error', 'missing key "prod.missing"', 1);
      expect(t.dyn('prod.missing')).toBe('prod.missing');
      expect(t.dyn('prod.missing')).toBe('prod.missing');
    } finally {
      if (prevEnv === undefined) {
        delete process.env['NODE_ENV'];
      } else {
        process.env['NODE_ENV'] = prevEnv;
      }
    }
  });
});
