import type { Ref } from 'react';
import { beforeEach, describe, expect, test, vi } from 'vitest';
import { act, fakeEnv, render, useFakeTime } from '@tests/kit';
import { resetTooltipState, Tooltip } from './Tooltip';
import { UiEnvProvider } from './UiEnv';

vi.mock('@floating-ui/dom', () => ({
  computePosition: vi.fn().mockResolvedValue({ x: 20, y: 10, placement: 'top' }),
  autoUpdate: vi.fn((_r: unknown, _f: unknown, u: () => void) => {
    u();
    return vi.fn();
  }),
  offset: vi.fn((n: number) => ({ name: 'offset', n })),
  flip: vi.fn((o: unknown) => ({ name: 'flip', o })),
  shift: vi.fn((o: unknown) => ({ name: 'shift', o })),
  size: vi.fn((o: unknown) => ({ name: 'size', o })),
}));

describe('W.08.04 Tooltip', () => {
  beforeEach(() => {
    resetTooltipState();
  });

  test('W0804-T13 Tooltip：400ms 延迟、暖场立即切换、聚焦立即、只显示一个、Esc 不阻止冒泡', async () => {
    const clock = useFakeTime();
    const fe = fakeEnv({ reducedMotion: true });
    const onEnterA = vi.fn();
    const parentEsc = vi.fn();

    try {
      const r = render(
        <UiEnvProvider value={fe.env}>
          <div onKeyDown={parentEsc}>
            <Tooltip content="提示 A" shortcut="Ctrl+K">
              <button type="button" onPointerEnter={onEnterA}>
                btn-a
              </button>
            </Tooltip>
            <Tooltip content="提示 B">
              <button type="button">btn-b</button>
            </Tooltip>
          </div>
        </UiEnvProvider>,
      );

      const btnA = r.getByRole('button', { name: 'btn-a' });
      const btnB = r.getByRole('button', { name: 'btn-b' });

      // 悬停 399ms 不显示，400ms 显示，且子元素原有 onPointerEnter 被调用
      act(() => {
        btnA.dispatchEvent(
          new PointerEvent('pointerover', {
            bubbles: true,
            pointerType: 'mouse',
            relatedTarget: document.body,
          }),
        );
      });
      expect(onEnterA).toHaveBeenCalledTimes(1);
      await act(async () => {
        await clock.advance(399);
      });
      expect(fe.root.querySelectorAll('[role="tooltip"]')).toHaveLength(0);

      await act(async () => {
        await clock.advance(1);
      });
      expect(fe.root.querySelectorAll('[role="tooltip"]')).toHaveLength(1);
      expect(fe.root.textContent).toContain('提示 A');
      expect(fe.root.querySelector('kbd')?.textContent).toBe('Ctrl+K');

      // 离开后 200ms 内进入 btnB 立即显示且只有一个 tooltip
      act(() => {
        btnA.dispatchEvent(
          new PointerEvent('pointerout', {
            bubbles: true,
            pointerType: 'mouse',
            relatedTarget: document.body,
          }),
        );
      });
      expect(fe.root.querySelectorAll('[role="tooltip"]')).toHaveLength(0);
      await act(async () => {
        await clock.advance(200);
        btnB.dispatchEvent(
          new PointerEvent('pointerover', {
            bubbles: true,
            pointerType: 'mouse',
            relatedTarget: document.body,
          }),
        );
      });
      expect(fe.root.querySelectorAll('[role="tooltip"]')).toHaveLength(1);
      expect(fe.root.textContent).toContain('提示 B');

      // 按 Esc 隐藏提示框且事件继续冒泡到父级
      act(() => {
        btnB.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true }));
      });
      expect(fe.root.querySelectorAll('[role="tooltip"]')).toHaveLength(0);
      expect(parentEsc).toHaveBeenCalledTimes(1);

      // 键盘聚焦立即显示（超过暖场窗口后验证）
      await act(async () => {
        await clock.advance(500);
      });
      Object.defineProperty(btnA, 'matches', { configurable: true, value: () => true });
      act(() => {
        btnA.focus();
      });
      expect(fe.root.querySelectorAll('[role="tooltip"]')).toHaveLength(1);
    } finally {
      fe.cleanup();
    }
  });

  test('Tooltip Esc/pointerdown 隐藏后 300ms 内仍进入暖场，且子组件未透传 ref 在开发模式抛错', async () => {
    const clock = useFakeTime();
    const fe = fakeEnv({ reducedMotion: true });
    try {
      const r = render(
        <UiEnvProvider value={fe.env}>
          <div>
            <Tooltip content="提示 A">
              <button type="button">btn-a</button>
            </Tooltip>
            <Tooltip content="提示 B">
              <button type="button">btn-b</button>
            </Tooltip>
          </div>
        </UiEnvProvider>,
      );
      const btnA = r.getByRole('button', { name: 'btn-a' });
      const btnB = r.getByRole('button', { name: 'btn-b' });

      await act(async () => {
        btnA.dispatchEvent(
          new PointerEvent('pointerover', {
            bubbles: true,
            pointerType: 'mouse',
            relatedTarget: document.body,
          }),
        );
        await clock.advance(400);
      });
      expect(fe.root.querySelectorAll('[role="tooltip"]')).toHaveLength(1);

      // Esc 隐藏后 150ms 移到 btnB 应立即暖场显示
      act(() => {
        btnA.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
      });
      expect(fe.root.querySelectorAll('[role="tooltip"]')).toHaveLength(0);
      await act(async () => {
        await clock.advance(150);
        btnB.dispatchEvent(
          new PointerEvent('pointerover', {
            bubbles: true,
            pointerType: 'mouse',
            relatedTarget: document.body,
          }),
        );
      });
      expect(fe.root.querySelectorAll('[role="tooltip"]')).toHaveLength(1);

      // 不透传 ref 的自定义组件在开发模式抛错
      function BadChild() {
        return <span>no-ref</span>;
      }
      expect(() =>
        render(
          <UiEnvProvider value={fe.env}>
            <Tooltip content="bad">
              <BadChild />
            </Tooltip>
          </UiEnvProvider>,
        ),
      ).toThrow(/forward ref/);

      // 只透传 ref 但丢弃事件/DOM props 的自定义组件同样在开发模式抛错（第 2 轮评审 #11）
      function RefOnlyChild(p: { ref?: Ref<HTMLSpanElement> }) {
        return <span ref={p.ref}>ref-only</span>;
      }
      expect(() =>
        render(
          <UiEnvProvider value={fe.env}>
            <Tooltip content="bad-events">
              <RefOnlyChild />
            </Tooltip>
          </UiEnvProvider>,
        ),
      ).toThrow(/DOM event props/);
    } finally {
      fe.cleanup();
    }
  });
});
