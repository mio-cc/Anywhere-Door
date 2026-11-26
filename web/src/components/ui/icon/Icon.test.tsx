import { describe, expect, test } from 'vitest';
import { ICONS } from '@/styles/icons';
import { expectConsole, render } from '@tests/kit';
import { Icon } from './Icon';

describe('W.08.01 Icon', () => {
  test('W0801-T8 Icon：viewBox、子元素数、尺寸类、label 与未知图标回退扩展且只警告一次', () => {
    const r = render(<Icon name="chat" />);
    const svg = r.container.querySelector('svg');
    expect(svg?.getAttribute('viewBox')).toBe('0 0 16 16');
    expect(svg?.getAttribute('aria-hidden')).toBe('true');
    expect(svg?.children.length).toBe(ICONS.chat.shapes.length);

    r.rerender(<Icon name="failed" size={14} label="失败" />);
    const svg2 = r.container.querySelector('svg');
    expect(svg2?.classList.contains('hx-icon--14')).toBe(true);
    expect(svg2?.getAttribute('role')).toBe('img');
    expect(svg2?.getAttribute('aria-label')).toBe('失败');
    expect(svg2?.getAttribute('aria-hidden')).toBeNull();

    expectConsole('warn', /Unknown icon/, 1);
    r.rerender(<Icon name="not-exist-icon-x" />);
    r.rerender(<Icon name="not-exist-icon-x" />);
    expect(r.container.querySelector('svg')?.children.length).toBe(ICONS.扩展.shapes.length);
  });
});
