import { describe, expect, test } from 'vitest';
import { act, render, useFakeTime } from '@tests/kit';
import { RelTime } from './RelTime';
import { setClock } from './clock';

describe('W.10 RelTime 组件', () => {
  test('W10-T12 RelTime 整分钟刷新、title 与卸载', async () => {
    const ft = useFakeTime({ now: '2026-01-15T10:01:00+08:00' });
    const iso = '2026-01-15T10:00:00+08:00';
    let renders = 0;
    function Wrap() {
      renders += 1;
      return <RelTime iso={iso} className="hx-rel" />;
    }

    const view = render(<Wrap />);
    const timeEl = view.container.querySelector('time');
    expect(timeEl).not.toBeNull();
    expect(timeEl?.getAttribute('dateTime')).toBe(iso);
    expect(timeEl?.getAttribute('title')).toMatch(/（UTC[+-]\d{2}:\d{2}）$/);
    expect(timeEl?.textContent).toBe('1 分钟前');
    expect(renders).toBe(1);

    await ft.advance(60_100);
    expect(view.container.querySelector('time')?.textContent).toBe('2 分钟前');
    // 只有 RelTime 订阅了 useNow，父组件不应重渲染
    expect(renders).toBe(1);

    // 冻结后 5 分钟不变化
    act(() => {
      setClock({ now: () => ft.now(), frozen: true });
    });
    await ft.advance(300_000);
    expect(view.container.querySelector('time')?.textContent).toBe('2 分钟前');

    view.unmount();
  });
});
