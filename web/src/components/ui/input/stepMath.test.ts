import { describe, expect, test } from 'vitest';
import { clamp, decimals, dragDelta, nudge, parseTyped, roundTo } from './stepMath';

describe('W.08.02 stepMath', () => {
  test('W0802-T5 stepMath：decimals、roundTo、clamp、dragDelta、nudge、parseTyped', () => {
    expect(decimals(0.05)).toBe(2);
    expect(decimals(1)).toBe(0);
    expect(roundTo(0.1 + 0.2, 0.1)).toBe(0.3);
    expect(clamp(40, 1, 32)).toBe(32);
    expect(clamp(-5, 0, 10)).toBe(0);

    expect(dragDelta(0, 9, { shift: false, alt: false })).toEqual({ steps: 2, acc: 0.25 });
    expect(dragDelta(0, 9, { shift: true, alt: false })).toEqual({ steps: 22, acc: 0.5 });

    // alt 精细拖动：累计超过 1 步时产出 1 步
    const a1 = dragDelta(0, 20, { shift: false, alt: true });
    expect(a1.steps).toBe(0);
    const a2 = dragDelta(a1.acc, 20, { shift: false, alt: true });
    expect(a2.steps).toBe(1);

    expect(nudge(10, 1, 1, true, 0, 100)).toBe(20);
    expect(nudge(0.1, 1, 0.2, false, 0, 1)).toBe(0.3);

    expect(parseTyped('3*2')).toBeNull();
    expect(parseTyped('abc')).toBeNull();
    expect(parseTyped(' -1.5 ')).toBe(-1.5);
    expect(parseTyped('+42')).toBe(42);
  });
});
