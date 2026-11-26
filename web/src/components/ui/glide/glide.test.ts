import { describe, expect, test } from 'vitest';
import { at, flipFrom, relBox } from './glide';

describe('W.08.01 glide 几何计算', () => {
  test('W0801-T10 glide 几何：relBox / flipFrom / at', () => {
    const b1 = relBox({ left: 110, top: 240, width: 200, height: 28 }, { left: 100, top: 200 });
    expect(b1).toEqual({ x: 10, y: 40, w: 200, h: 28 });

    const b2 = { x: 10, y: 72, w: 200, h: 32 };
    expect(flipFrom(b1, b2)).toBe('translate(10px,40px) scale(1,0.875)');
    expect(at(b2)).toBe('translate(10px,72px)');
  });
});
