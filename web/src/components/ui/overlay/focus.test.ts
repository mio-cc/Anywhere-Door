import { describe, expect, test } from 'vitest';
import { cycleTab, focusables, focusInitial, resolveRestore } from './focus';

describe('W.08.04 focus 工具', () => {
  test('focusables / focusInitial / cycleTab / resolveRestore', () => {
    const root = document.createElement('div');
    root.tabIndex = -1;
    const b1 = document.createElement('button');
    b1.textContent = 'one';
    const b2 = document.createElement('button');
    b2.textContent = 'two';
    b2.setAttribute('data-autofocus', '');
    const disabled = document.createElement('button');
    disabled.disabled = true;
    root.append(b1, b2, disabled);
    document.body.appendChild(root);

    try {
      expect(focusables(root)).toEqual([b1, b2]);
      focusInitial(root, 'first');
      expect(document.activeElement).toBe(b2);

      const tabEv = new KeyboardEvent('keydown', { key: 'Tab', cancelable: true });
      expect(cycleTab(root, tabEv)).toBe(true);
      expect(tabEv.defaultPrevented).toBe(true);
      expect(document.activeElement).toBe(b1);

      const shiftTab = new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true, cancelable: true });
      cycleTab(root, shiftTab);
      expect(document.activeElement).toBe(b2);

      const detached = document.createElement('button');
      expect(resolveRestore(detached, b1)).toBe(b1);
      expect(resolveRestore(false, b1)).toBeNull();
    } finally {
      root.remove();
    }
  });
});
