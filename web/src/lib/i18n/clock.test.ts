import { createElement } from 'react';
import { describe, expect, test, vi } from 'vitest';
import { act, render, useFakeTime } from '@tests/kit';
import { setClock, useNow } from './clock';

describe('W.10 全局整分钟时钟', () => {
  test('W10-T12/clock 隐藏暂停、可见刷新、冻结与卸载清理', async () => {
    const ft = useFakeTime({ now: '2026-01-15T10:00:30+08:00' });
    let rendersA = 0;
    let rendersB = 0;
    function ProbeA() {
      rendersA += 1;
      return createElement('span', null, String(useNow()));
    }
    function ProbeB() {
      rendersB += 1;
      return createElement('span', null, String(useNow()));
    }

    // 两个订阅者共用 1 个定时器
    const viewA = render(createElement(ProbeA));
    const viewB = render(createElement(ProbeB));
    expect(vi.getTimerCount()).toBe(1);
    expect(rendersA).toBe(1);
    expect(rendersB).toBe(1);

    // 跨过整分钟（30s + 50ms）触发 1 次刷新，仍只有 1 个待执行定时器
    await ft.advance(30_100);
    expect(rendersA).toBe(2);
    expect(rendersB).toBe(2);
    expect(vi.getTimerCount()).toBe(1);

    // 页面隐藏时清掉定时器，推进 2 分钟不刷新
    act(() => {
      Object.defineProperty(document, 'hidden', { configurable: true, value: true });
      document.dispatchEvent(new Event('visibilitychange'));
    });
    expect(vi.getTimerCount()).toBe(0);
    await ft.advance(120_000);
    expect(rendersA).toBe(2);

    // 恢复可见立即刷新 1 次并重新排 1 个定时器
    act(() => {
      Object.defineProperty(document, 'hidden', { configurable: true, value: false });
      document.dispatchEvent(new Event('visibilitychange'));
    });
    expect(rendersA).toBe(3);
    expect(rendersB).toBe(3);
    expect(vi.getTimerCount()).toBe(1);

    // 活动状态下卸载第一个订阅者仍保留 1 个定时器；卸载最后一个后定时器归零
    viewA.unmount();
    expect(vi.getTimerCount()).toBe(1);
    viewB.unmount();
    expect(vi.getTimerCount()).toBe(0);

    // 冻结时钟时不排定时器，5 分钟内不重渲染
    const viewFrozen = render(createElement(ProbeA));
    const beforeFreeze = rendersA;
    act(() => {
      setClock({ now: () => Date.parse('2026-01-15T10:05:00+08:00'), frozen: true });
    });
    expect(vi.getTimerCount()).toBe(0);
    const afterFreeze = rendersA;
    expect(afterFreeze).toBeGreaterThanOrEqual(beforeFreeze);
    await ft.advance(300_000);
    expect(rendersA).toBe(afterFreeze);
    viewFrozen.unmount();
    expect(vi.getTimerCount()).toBe(0);
  });
});
