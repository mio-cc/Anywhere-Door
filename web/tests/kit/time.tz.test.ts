import { describe, expect, test } from 'vitest';

describe('W.12 双时区测试', () => {
  test('W12-T12 时区偏移随 TZ 变化', () => {
    const tz = process.env['TZ'] ?? '';
    const offset = new Date('2026-01-15T12:00:00Z').getTimezoneOffset();
    if (tz === 'Asia/Shanghai') {
      expect(offset).toBe(-480);
    } else if (tz === 'America/New_York') {
      expect(offset).toBe(300);
    } else {
      // 本地 vitest 单独跑时允许任意有效时区偏移
      expect(Number.isFinite(offset)).toBe(true);
    }
  });
});
