import { describe, expect, test } from 'vitest';
import { fitRows } from './autosize';

describe('W.08.02 autosize', () => {
  test('fitRows 夹紧到 [minRows, maxRows]', () => {
    const el = document.createElement('textarea');
    el.style.lineHeight = '20px';
    el.style.paddingTop = '4px';
    el.style.paddingBottom = '4px';
    document.body.appendChild(el);
    try {
      Object.defineProperty(el, 'scrollHeight', { configurable: true, value: 24 });
      fitRows(el, 3, 12);
      expect(el.style.height).toBe('68px');
      expect(el.style.overflowY).toBe('hidden');

      Object.defineProperty(el, 'scrollHeight', { configurable: true, value: 500 });
      fitRows(el, 3, 12);
      expect(el.style.height).toBe('248px');
      expect(el.style.overflowY).toBe('auto');
    } finally {
      el.remove();
    }
  });
});
