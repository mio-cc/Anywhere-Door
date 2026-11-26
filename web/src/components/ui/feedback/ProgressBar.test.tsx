import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, test } from 'vitest';
import { expectConsole, render } from '@tests/kit';
import { ProgressBar } from './ProgressBar';

const here = dirname(fileURLToPath(import.meta.url));

describe('W.08.05 ProgressBar', () => {
  test('W0805-T8 ProgressBar：夹紧边界、NaN 回退 0、只用 transform 过渡', () => {
    const r = render(<ProgressBar value={1.4} ariaLabel="运行进度" />);
    const bar = r.getByRole('progressbar', { name: '运行进度' });
    expect(bar.getAttribute('aria-valuenow')).toBe('100');
    const fill = bar.querySelector('.hx-progress__fill') as HTMLElement;
    expect(fill.style.getPropertyValue('--_v')).toBe('1');

    expectConsole('warn', /not finite/, 1);
    r.rerender(<ProgressBar value={Number.NaN} ariaLabel="运行进度" />);
    expect(r.getByRole('progressbar', { name: '运行进度' }).getAttribute('aria-valuenow')).toBe('0');

    const css = readFileSync(resolve(here, 'ProgressBar.css'), 'utf8');
    expect(css).toContain('transform: scaleX(var(--_v, 0))');
    expect(css).not.toMatch(/transition-property:\s*width/);
  });
});
