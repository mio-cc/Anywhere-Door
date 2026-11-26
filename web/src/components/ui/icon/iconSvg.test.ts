import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, test } from 'vitest';
import { ICONS } from '@/styles/icons';
import { expectConsole } from '@tests/kit';
import { iconSvg } from './iconSvg';

const here = dirname(fileURLToPath(import.meta.url));

describe('W.08.01 iconSvg', () => {
  test('W0801-T9 iconSvg：克隆新元素、未知回退扩展、返回 SVGSVGElement 且不使用 innerHTML', () => {
    const a = iconSvg('Agent', 16);
    const b = iconSvg('Agent', 14);
    expect(a).not.toBe(b);
    expect(a.tagName.toLowerCase()).toBe('svg');
    expect(a.classList.contains('hx-icon--16')).toBe(true);
    expect(b.classList.contains('hx-icon--14')).toBe(true);
    expect(a.children.length).toBe(ICONS.Agent.shapes.length);
    expect(a.outerHTML.replace('hx-icon--16', 'hx-icon--14')).toBe(b.outerHTML);

    expectConsole('warn', /Unknown icon/, 1);
    const fb = iconSvg('unknown-svg-icon-y');
    expect(fb.children.length).toBe(ICONS.扩展.shapes.length);

    const src = readFileSync(resolve(here, 'iconSvg.ts'), 'utf8');
    expect(src).not.toContain('innerHTML');
    expect(src).not.toContain('dangerouslySetInnerHTML');
  });
});
