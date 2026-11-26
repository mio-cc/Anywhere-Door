import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, test } from 'vitest';
import { render } from '@tests/kit';
import type { Tone } from '../types';
import { Badge } from './Badge';

const here = dirname(fileURLToPath(import.meta.url));

describe('W.08.05 Badge', () => {
  test('W0805-T5 Badge：六种 tone 类名、dot 圆点与令牌 CSS', () => {
    const tones: Tone[] = ['neutral', 'accent', 'ok', 'danger', 'wait', 'model'];
    for (const tone of tones) {
      const r = render(<Badge tone={tone} text={tone} dot />);
      const el = r.container.querySelector(`.hx-badge--${tone}`);
      expect(el).not.toBeNull();
      expect(el?.querySelector('.hx-badge__dot')).not.toBeNull();
      r.unmount();
    }
    const css = readFileSync(resolve(here, 'Badge.css'), 'utf8');
    expect(css).toContain('var(--hx-accent-weak)');
    expect(css).toContain('var(--hx-accent-strong)');
    expect(css).toContain('var(--hx-model-weak)');
    expect(css).not.toMatch(/#[0-9a-f]{3,6}/i);
  });
});
