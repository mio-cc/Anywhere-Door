import { describe, expect, test } from 'vitest';
import { edge, flatten, matchSpan, step, typeahead, windowOf, type NavRow } from './listNav';

describe('W.08.03 listNav', () => {
  test('W0803-T2 listNav：step/edge/typeahead/windowOf/matchSpan', () => {
    const rows: NavRow[] = [
      { id: 'a', disabled: false, height: 28 },
      { id: 'b', disabled: true, height: 28 },
      { id: 'c', disabled: false, height: 28 },
    ];
    expect(step(rows, 'a', 1, false)).toBe('c');
    expect(step(rows, 'c', 1, false)).toBeNull();
    expect(step(rows, 'c', 1, true)).toBe('a');
    expect(edge(rows, 'first')).toBe('a');
    expect(edge(rows, 'last')).toBe('c');

    const labels = [
      { id: '1', label: 'Alpha', disabled: false },
      { id: '2', label: 'Json-Tool', disabled: false },
      { id: '3', label: 'Join', disabled: false },
    ];
    expect(typeahead(labels, 'j', '1')).toBe('2');
    expect(typeahead(labels, 'j', '2')).toBe('3');

    // 1000 行 × 28，视口 320，scrollTop=2800，buffer=10 → start=90, end=122
    const offsets = Array.from({ length: 1000 }, (_, i) => i * 28);
    expect(windowOf(offsets, 2800, 320, 10)).toEqual({ start: 90, end: 122 });

    expect(matchSpan('Large-Model', 'lar')).toEqual([0, 3]);
    expect(matchSpan('Large-Model', 'ｌａｒ')).toEqual([0, 3]);

    const flat = flatten([{ items: [{ id: 'x', label: 'X' }] }], () => 32);
    expect(flat).toEqual([{ id: 'x', disabled: false, height: 32 }]);
  });
});
