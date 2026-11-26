import { describe, expect, test } from 'vitest';
import { render } from '@tests/kit';
import { Kbd } from './Kbd';

describe('W.08.01 Kbd', () => {
  test('W0801-T7 Kbd：三种 variant 类名与原样输出文本', () => {
    const r = render(
      <div>
        <Kbd text="⌘K" variant="inline" />
        <Kbd text="Ctrl+Enter" variant="key" />
        <Kbd text="Esc" variant="tip" />
      </div>,
    );
    const kbds = Array.from(r.container.querySelectorAll('kbd'));
    expect(kbds).toHaveLength(3);
    expect(kbds[0]?.classList.contains('hx-kbd--inline')).toBe(true);
    expect(kbds[0]?.textContent).toBe('⌘K');
    expect(kbds[1]?.classList.contains('hx-kbd--key')).toBe(true);
    expect(kbds[2]?.classList.contains('hx-kbd--tip')).toBe(true);
  });
});
