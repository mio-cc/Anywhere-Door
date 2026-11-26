import { describe, expect, test, vi } from 'vitest';
import { render, user } from '@tests/kit';
import { Tabs } from './Tabs';

describe('W.08.03 Tabs', () => {
  test('W0803-T8 Tabs：underline 无底色类、← 循环切换、panelIdOf 与 badge', async () => {
    const onChange = vi.fn();
    const items = [
      { value: 'insp', label: '属性' },
      { value: 'chat', label: '对话' },
      { value: 'mon', label: '监控', badge: 3 },
    ] as const;

    const r = render(
      <Tabs
        value="insp"
        items={items}
        onChange={onChange}
        variant="underline"
        ariaLabel="右侧栏标签"
        panelIdOf={(v) => `panel-${v}`}
      />,
    );

    const tabs = r.queryAllByRole('tab');
    expect(tabs[0]?.classList.contains('is-current')).toBe(true);
    expect(tabs[0]?.getAttribute('aria-controls')).toBe('panel-insp');
    expect(tabs[2]?.querySelector('.hx-badge')?.textContent).toBe('3');

    tabs[0]!.focus();
    await user.key('ArrowLeft', tabs[0]);
    expect(onChange).toHaveBeenCalledWith('mon');
    expect(document.activeElement).toBe(tabs[2]);
  });
});
