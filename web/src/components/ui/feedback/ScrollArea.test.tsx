import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createRef } from 'react';
import { describe, expect, test } from 'vitest';
import { render } from '@tests/kit';
import { ScrollArea } from './ScrollArea';

const here = dirname(fileURLToPath(import.meta.url));

describe('W.08.05 ScrollArea', () => {
  test('W0805-T10 ScrollArea：类名、gutter、ariaLabel、ref 与滑块令牌', () => {
    const ref = createRef<HTMLDivElement>();
    const r = render(
      <ScrollArea ref={ref} gutter="stable" ariaLabel="侧栏列表">
        <div>item</div>
      </ScrollArea>,
    );
    const el = r.getByRole('region', { name: '侧栏列表' });
    expect(el.classList.contains('hx-scroll--stable')).toBe(true);
    expect(el.tabIndex).toBe(0);
    expect(ref.current).toBe(el);

    const css = readFileSync(resolve(here, 'ScrollArea.css'), 'utf8');
    expect(css).toContain('var(--hx-bg-thumb)');
    expect(css).toContain('width: var(--hx-s-3)');
  });
});
