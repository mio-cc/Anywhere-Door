import { describe, expect, test } from 'vitest';
import { expectConsole } from '@tests/kit';
import {
  CURRENCY,
  fmtAbsTime,
  fmtBytes,
  fmtCost,
  fmtCostInput,
  fmtCount,
  fmtDuration,
  fmtPercent,
  fmtRelTime,
  fmtTokens,
  parseCostInput,
} from './format';

describe('W.10 格式化与解析', () => {
  test('W10-T4 fmtCount / fmtTokens', () => {
    expect(fmtCount(0)).toBe('0');
    expect(fmtCount(9999)).toBe('9999');
    expect(fmtCount(10000)).toBe('1 万');
    expect(fmtCount(12345)).toBe('1.2 万');
    expect(fmtCount(99999)).toBe('10 万');
    expect(fmtCount(99_995_000)).toBe('1 亿');
    expect(fmtCount(150_000_000)).toBe('1.5 亿');
    expect(fmtCount(-12345)).toBe('-1.2 万');
    expect(fmtTokens(128_000)).toBe('12.8 万');
    expectConsole('warn', 'fmtCount: invalid number', 1);
    expect(fmtCount(Number.NaN)).toBe('—');
    expect(fmtCount(Number.POSITIVE_INFINITY)).toBe('—');
  });

  test('W10-T5 fmtCost / fmtCostInput', () => {
    expect(CURRENCY).toEqual({ symbol: '¥', code: 'CNY' });
    expect(fmtCost(0)).toBe('¥0');
    expect(fmtCost(1)).toBe('<¥0.01');
    expect(fmtCost(4999)).toBe('<¥0.01');
    expect(fmtCost(5000)).toBe('¥0.01');
    expect(fmtCost(420_000)).toBe('¥0.42');
    expect(fmtCost(1_234_500_000)).toBe('¥1,234.50');
    expect(fmtCost(-420_000)).toBe('-¥0.42');
    expect(fmtCost(420_000.4)).toBe('¥0.42');
    expectConsole('warn', 'fmtCost: invalid number', 1);
    expect(fmtCost(Number.NaN)).toBe('—');
    expect(fmtCost(Number.POSITIVE_INFINITY)).toBe('—');

    expect(fmtCostInput(0)).toBe('0');
    expect(fmtCostInput(35_000)).toBe('0.035');
    expect(fmtCostInput(2_000_000)).toBe('2');
    expect(fmtCostInput(1)).toBe('0.000001');
    expect(fmtCostInput(-420_000)).toBe('-0.42');
    expectConsole('warn', 'fmtCostInput: invalid number', 1);
    expect(fmtCostInput(Number.NaN)).toBe('—');
    expect(fmtCostInput(Number.NEGATIVE_INFINITY)).toBe('—');
  });

  test('W10-T6 fmtDuration', () => {
    expect(fmtDuration(-1)).toBe('—');
    expect(fmtDuration(0)).toBe('<1 秒');
    expect(fmtDuration(999)).toBe('<1 秒');
    expect(fmtDuration(1000)).toBe('1 秒');
    expect(fmtDuration(3200)).toBe('3.2 秒');
    expect(fmtDuration(9999)).toBe('10 秒');
    expect(fmtDuration(10_000)).toBe('10 秒');
    expect(fmtDuration(59_999)).toBe('59 秒');
    expect(fmtDuration(60_000)).toBe('1 分 00 秒');
    expect(fmtDuration(185_000)).toBe('3 分 05 秒');
    expect(fmtDuration(3_599_999)).toBe('59 分 59 秒');
    expect(fmtDuration(3_600_000)).toBe('1 小时 00 分');
    expect(fmtDuration(3_900_000)).toBe('1 小时 05 分');
    expectConsole('warn', 'fmtDuration: invalid number', 1);
    expect(fmtDuration(Number.NaN)).toBe('—');
    expect(fmtDuration(Number.POSITIVE_INFINITY)).toBe('—');
  });

  test('W10-T7 fmtBytes / fmtPercent', () => {
    expect(fmtBytes(0)).toBe('0 B');
    expect(fmtBytes(1023)).toBe('1023 B');
    expect(fmtBytes(1024)).toBe('1 KB');
    expect(fmtBytes(1_288_490_189)).toBe('1.2 GB');
    expect(fmtBytes(Math.round(1023.95 * 1024))).toBe('1 MB');
    expect(fmtBytes(-1)).toBe('—');
    expectConsole('warn', 'fmtBytes: invalid number', 1);
    expect(fmtBytes(Number.NaN)).toBe('—');
    expect(fmtBytes(Number.POSITIVE_INFINITY)).toBe('—');

    expect(fmtPercent(0)).toBe('0%');
    expect(fmtPercent(0.004)).toBe('<1%');
    expect(fmtPercent(0.005)).toBe('1%');
    expect(fmtPercent(0.724)).toBe('72%');
    expect(fmtPercent(1.2)).toBe('120%');
    expectConsole('warn', 'fmtPercent: invalid number', 1);
    expect(fmtPercent(Number.NaN)).toBe('—');
    expect(fmtPercent(Number.POSITIVE_INFINITY)).toBe('—');
  });

  test('W10-T8 fmtRelTime 全部边界与跨午夜', () => {
    // 构造一个本地时间 2026-06-15 01:00:00 的时间戳，确保跨午夜用例在任意时区都确定
    const baseDate = new Date(2026, 5, 15, 1, 0, 0);
    const now = baseDate.getTime();
    const ago = (ms: number) => new Date(now - ms).toISOString();

    expect(fmtRelTime(ago(30_000), now)).toBe('刚刚');
    expect(fmtRelTime(ago(44_000), now)).toBe('刚刚');
    expect(fmtRelTime(ago(45_000), now)).toBe('1 分钟前');
    expect(fmtRelTime(ago(59 * 60_000), now)).toBe('59 分钟前');
    expect(fmtRelTime(ago(3 * 3_600_000), now)).toBe('3 小时前');
    // 跨午夜 2 小时前（昨天 23:00）仍是“2 小时前”
    expect(fmtRelTime(ago(2 * 3_600_000), now)).toBe('2 小时前');

    // 本地时间 2026-06-15 20:00:00，25 小时前是昨天 19:00 → 昨天
    const evening = new Date(2026, 5, 15, 20, 0, 0).getTime();
    expect(fmtRelTime(new Date(evening - 25 * 3_600_000).toISOString(), evening)).toBe('昨天');
    expect(fmtRelTime(new Date(2026, 5, 9, 12, 0, 0).toISOString(), evening)).toBe('6 天前');
    expect(fmtRelTime(new Date(2026, 5, 8, 12, 0, 0).toISOString(), evening)).toBe('06-08');
    expect(fmtRelTime(new Date(2025, 2, 1, 12, 0, 0).toISOString(), evening)).toBe('2025-03-01');
    // 早期年份必须补足 4 位 YYYY-MM-DD（P.09 K302）
    expect(fmtRelTime('0001-06-15T12:00:00Z', evening)).toBe('0001-06-15');

    // 未来时间
    expect(fmtRelTime(new Date(evening + 3 * 60_000).toISOString(), evening)).toBe('刚刚');
    expect(fmtRelTime(new Date(evening + 10 * 60_000).toISOString(), evening)).toBe(
      fmtAbsTime(new Date(evening + 10 * 60_000).toISOString(), { seconds: false }),
    );

    expectConsole('warn', 'fmtRelTime: invalid ISO time', 1);
    expect(fmtRelTime('not-a-date', evening)).toBe('—');
  });

  test('W10-T9 fmtAbsTime 补零、选项与双时区偏移', () => {
    const iso = '2026-01-15T01:02:05Z';
    const tz = process.env['TZ'] ?? '';
    if (tz === 'Asia/Shanghai') {
      expect(fmtAbsTime(iso)).toBe('2026-01-15 09:02:05');
      expect(fmtAbsTime(iso, { seconds: false })).toBe('2026-01-15 09:02');
      expect(fmtAbsTime(iso, { zone: true })).toBe('2026-01-15 09:02:05（UTC+08:00）');
    } else if (tz === 'America/New_York') {
      expect(fmtAbsTime(iso)).toBe('2026-01-14 20:02:05');
      expect(fmtAbsTime(iso, { seconds: false })).toBe('2026-01-14 20:02');
      expect(fmtAbsTime(iso, { zone: true })).toBe('2026-01-14 20:02:05（UTC-05:00）');
    } else {
      expect(fmtAbsTime(iso)).toMatch(/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/);
      expect(fmtAbsTime(iso, { seconds: false })).toMatch(/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}$/);
      expect(fmtAbsTime(iso, { zone: true })).toMatch(/（UTC[+-]\d{2}:\d{2}）$/);
    }
    // 4 位年份补零 + 非法时间返回 —（早期年份 LMT 时区偏移可能含秒数，故匹配 \d{2}）
    expect(fmtAbsTime('0001-06-15T12:00:00Z')).toMatch(/^0001-06-15 \d{2}:\d{2}:\d{2}$/);
    expectConsole('warn', 'fmtAbsTime: invalid ISO time', 1);
    expect(fmtAbsTime('invalid-iso')).toBe('—');
  });

  test('W10-T10 parseCostInput 与 fmtCostInput 往返', () => {
    for (const x of [0, 1, 35_000, 420_000, 2_000_000, 123_456_789]) {
      expect(parseCostInput(fmtCostInput(x))).toBe(x);
    }
    expect(parseCostInput('0.1')).toBe(100_000);
    expect(parseCostInput('0.2')).toBe(200_000);
    expect(parseCostInput('0.3')).toBe(300_000);
    expect(parseCostInput('１,２３４.５')).toBe(1_234_500_000);
    expect(parseCostInput('￥0.42')).toBe(420_000);
    expect(parseCostInput('¥0.42')).toBe(420_000);
    expect(parseCostInput('1,2,3')).toBe(123_000_000);

    expect(parseCostInput('-1')).toBeNull();
    expect(parseCostInput('1e3')).toBeNull();
    expect(parseCostInput('')).toBeNull();
    expect(parseCostInput('0.1234567')).toBeNull();
    expect(parseCostInput('1234567890')).toBeNull();
  });
});
