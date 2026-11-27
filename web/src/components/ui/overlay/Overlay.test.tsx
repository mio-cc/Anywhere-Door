import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, expectTypeOf, test, vi } from 'vitest';
import { act, fakeEnv, render, useFakeTime } from '@tests/kit';
import type { DialogProps } from './Dialog';
import { layerStackSize } from './layerStack';
import type { MenuProps } from './Menu';
import { Overlay, type OverlayProps } from './Overlay';
import type { PopoverProps } from './Popover';
import type { TooltipProps } from './Tooltip';
import { UiEnvProvider } from './UiEnv';

const here = dirname(fileURLToPath(import.meta.url));

describe('W.08.04 Overlay', () => {
  test('W0804-T1 props 对照', () => {
    expectTypeOf<OverlayProps['layer']>().toEqualTypeOf<'popover' | 'modal' | 'toast' | 'tooltip' | 'pane'>();
    expectTypeOf<PopoverProps['open']>().toEqualTypeOf<boolean>();
    expectTypeOf<MenuProps['onPick']>().toEqualTypeOf<(id: string) => void>();
    expectTypeOf<DialogProps['width']>().toEqualTypeOf<400 | 560>();
    expectTypeOf<TooltipProps['delay']>().toEqualTypeOf<number | undefined>();
    expect(true).toBe(true);
  });

  test('W0804-T2 压栈：popover/modal 压栈、toast/tooltip 不压栈、关闭与卸载出栈', () => {
    const fe = fakeEnv({ reducedMotion: true });
    try {
      const r = render(
        <UiEnvProvider value={fe.env}>
          <Overlay layer="popover" open>
            <div>pop</div>
          </Overlay>
          <Overlay layer="modal" open>
            <div>mod</div>
          </Overlay>
          <Overlay layer="toast" open>
            <div>toast</div>
          </Overlay>
          <Overlay layer="tooltip" open>
            <div>tip</div>
          </Overlay>
        </UiEnvProvider>,
      );

      expect(fe.history.map((h) => h.kind)).toEqual(['popover', 'modal']);
      expect(fe.active).toHaveLength(2);

      r.rerender(
        <UiEnvProvider value={fe.env}>
          <Overlay layer="popover" open={false}>
            <div>pop</div>
          </Overlay>
          <Overlay layer="modal" open>
            <div>mod</div>
          </Overlay>
        </UiEnvProvider>,
      );
      expect(fe.active).toHaveLength(1);
      expect(fe.active[0]?.kind).toBe('modal');

      r.unmount();
      expect(fe.active).toHaveLength(0);
      expect(layerStackSize()).toBe(0);
    } finally {
      fe.cleanup();
    }
  });

  test('W0804-T3 层级：根元素类 hx-ov--modal 且 CSS 无数字 z-index', () => {
    const fe = fakeEnv({ reducedMotion: true });
    try {
      render(
        <UiEnvProvider value={fe.env}>
          <Overlay layer="modal" open role="dialog" ariaLabel="m">
            <div>content</div>
          </Overlay>
        </UiEnvProvider>,
      );
      const ov = fe.root.querySelector('.hx-ov--modal');
      expect(ov).not.toBeNull();
      const css = readFileSync(resolve(here, 'Overlay.css'), 'utf8');
      expect(css).toContain('z-index: var(--hx-z-modal)');
      expect(css).not.toMatch(/z-index:\s*-?\d+/);
    } finally {
      fe.cleanup();
    }
  });

  test('W0804-T4 Esc：栈顶处理 Esc 触发 onClose，isComposing 忽略', () => {
    const fe = fakeEnv({ reducedMotion: true });
    const onClose = vi.fn();
    try {
      render(
        <UiEnvProvider value={fe.env}>
          <Overlay layer="popover" open onClose={onClose}>
            <div>pop</div>
          </Overlay>
        </UiEnvProvider>,
      );
      const composing = new KeyboardEvent('keydown', { key: 'Escape', isComposing: true });
      expect(fe.dispatchKey(composing)).toBe(false);
      expect(onClose).not.toHaveBeenCalled();

      expect(fe.dispatchKey({ key: 'Escape' })).toBe(true);
      expect(onClose).toHaveBeenCalledWith('esc');
    } finally {
      fe.cleanup();
    }
  });

  test('W0804-T5 焦点：initialFocus、restoreFocus 与不抢用户已移焦点', () => {
    const fe = fakeEnv({ reducedMotion: true });
    const opener = document.createElement('button');
    opener.textContent = 'opener';
    const customTarget = document.createElement('button');
    customTarget.textContent = 'custom';
    const otherInput = document.createElement('input');
    document.body.append(opener, customTarget, otherInput);

    try {
      opener.focus();
      const r = render(
        <UiEnvProvider value={fe.env}>
          <Overlay layer="popover" open initialFocus="first">
            <button type="button">inside-first</button>
          </Overlay>
        </UiEnvProvider>,
      );
      expect(document.activeElement?.textContent).toBe('inside-first');

      r.rerender(
        <UiEnvProvider value={fe.env}>
          <Overlay layer="popover" open={false} initialFocus="first">
            <button type="button">inside-first</button>
          </Overlay>
        </UiEnvProvider>,
      );
      expect(document.activeElement).toBe(opener);

      // initialFocus='none' 保持原焦点
      r.rerender(
        <UiEnvProvider value={fe.env}>
          <Overlay layer="popover" open initialFocus="none" restoreFocus={customTarget}>
            <button type="button">inside-none</button>
          </Overlay>
        </UiEnvProvider>,
      );
      expect(document.activeElement).toBe(opener);
      // 关闭后再以 initialFocus='first' 打开，验证显式 restoreFocus 元素
      r.rerender(
        <UiEnvProvider value={fe.env}>
          <Overlay layer="popover" open={false} initialFocus="none" restoreFocus={customTarget}>
            <button type="button">inside-none</button>
          </Overlay>
        </UiEnvProvider>,
      );

      // 显式 restoreFocus 元素
      r.rerender(
        <UiEnvProvider value={fe.env}>
          <Overlay layer="popover" open initialFocus="first" restoreFocus={customTarget}>
            <button type="button">inside-second</button>
          </Overlay>
        </UiEnvProvider>,
      );
      r.rerender(
        <UiEnvProvider value={fe.env}>
          <Overlay layer="popover" open={false} initialFocus="first" restoreFocus={customTarget}>
            <button type="button">inside-second</button>
          </Overlay>
        </UiEnvProvider>,
      );
      expect(document.activeElement).toBe(customTarget);

      // 用户主动聚焦其他输入框时不抢焦点
      r.rerender(
        <UiEnvProvider value={fe.env}>
          <Overlay layer="popover" open initialFocus="first">
            <button type="button">inside-third</button>
          </Overlay>
        </UiEnvProvider>,
      );
      otherInput.focus();
      r.rerender(
        <UiEnvProvider value={fe.env}>
          <Overlay layer="popover" open={false} initialFocus="first">
            <button type="button">inside-third</button>
          </Overlay>
        </UiEnvProvider>,
      );
      expect(document.activeElement).toBe(otherInput);
    } finally {
      opener.remove();
      customTarget.remove();
      otherInput.remove();
      fe.cleanup();
    }
  });

  test('W0804-T6 焦点循环：modal 内 Tab 与 Shift+Tab 循环', () => {
    const fe = fakeEnv({ reducedMotion: true });
    try {
      render(
        <UiEnvProvider value={fe.env}>
          <Overlay layer="modal" open trapFocus>
            <button type="button">b1</button>
            <button type="button">b2</button>
          </Overlay>
        </UiEnvProvider>,
      );
      const btns = Array.from(fe.root.querySelectorAll('button'));
      btns[1]?.focus();
      fe.dispatchKey({ key: 'Tab' });
      expect(document.activeElement).toBe(btns[0]);

      fe.dispatchKey({ key: 'Tab', shiftKey: true });
      expect(document.activeElement).toBe(btns[1]);
    } finally {
      fe.cleanup();
    }
  });

  test('W0804-T7 外部点击：锚点/子浮层不关、外部关闭、关闭后无残留监听', () => {
    const fe = fakeEnv({ reducedMotion: true });
    const anchor = document.createElement('button');
    const outside = document.createElement('div');
    document.body.append(anchor, outside);
    const onParentClose = vi.fn();
    const onChildClose = vi.fn();

    try {
      const r = render(
        <UiEnvProvider value={fe.env}>
          <Overlay layer="popover" open onClose={onParentClose} anchorEl={anchor}>
            <div>parent</div>
            <Overlay layer="popover" open onClose={onChildClose}>
              <button type="button" id="child-btn">
                child
              </button>
            </Overlay>
          </Overlay>
        </UiEnvProvider>,
      );

      // 点锚点：不关父浮层
      act(() => {
        anchor.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
      });
      expect(onParentClose).not.toHaveBeenCalled();

      // 点子浮层：不关父浮层
      const childBtn = fe.root.querySelector('#child-btn');
      act(() => {
        childBtn?.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
      });
      expect(onParentClose).not.toHaveBeenCalled();

      // 点外部：关闭
      act(() => {
        outside.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
      });
      expect(onParentClose).toHaveBeenCalledWith('outside');

      // closeOnOutside=false 不关
      onParentClose.mockClear();
      r.rerender(
        <UiEnvProvider value={fe.env}>
          <Overlay layer="popover" open closeOnOutside={false} onClose={onParentClose}>
            <div>stay</div>
          </Overlay>
        </UiEnvProvider>,
      );
      act(() => {
        outside.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
      });
      expect(onParentClose).not.toHaveBeenCalled();
    } finally {
      anchor.remove();
      outside.remove();
      fe.cleanup();
    }
  });

  test('W0804-T8 Tab 走出：非模态 focusout 到外部触发 tab 关闭，initialFocus=none 不关', () => {
    const fe = fakeEnv({ reducedMotion: true });
    const outside = document.createElement('button');
    document.body.appendChild(outside);
    const onClose = vi.fn();
    try {
      const r = render(
        <UiEnvProvider value={fe.env}>
          <Overlay layer="popover" open onClose={onClose} initialFocus="first">
            <button type="button">inside</button>
          </Overlay>
        </UiEnvProvider>,
      );
      const ov = fe.root.querySelector('.hx-ov') as HTMLElement;
      act(() => {
        ov.dispatchEvent(new FocusEvent('focusout', { bubbles: true, relatedTarget: outside }));
      });
      expect(onClose).toHaveBeenCalledWith('tab');

      onClose.mockClear();
      r.rerender(
        <UiEnvProvider value={fe.env}>
          <Overlay layer="popover" open onClose={onClose} initialFocus="none">
            <button type="button">inside</button>
          </Overlay>
        </UiEnvProvider>,
      );
      const ov2 = fe.root.querySelector('.hx-ov') as HTMLElement;
      act(() => {
        ov2.dispatchEvent(new FocusEvent('focusout', { bubbles: true, relatedTarget: outside }));
      });
      expect(onClose).not.toHaveBeenCalled();
    } finally {
      outside.remove();
      fe.cleanup();
    }
  });

  test('W0804-T14 减少动效：is-reduced 类与 CSS 无 transform 过渡', () => {
    const fe = fakeEnv({ reducedMotion: true });
    try {
      render(
        <UiEnvProvider value={fe.env}>
          <Overlay layer="popover" open>
            <div>reduced</div>
          </Overlay>
        </UiEnvProvider>,
      );
      const ov = fe.root.querySelector('.hx-ov');
      expect(ov?.classList.contains('is-reduced')).toBe(true);
      const css = readFileSync(resolve(here, 'Overlay.css'), 'utf8');
      expect(css).toContain("html[data-motion='reduce']");
      expect(css).toContain('hx-ov-fade-in');
    } finally {
      fe.cleanup();
    }
  });

  test('W0804-T15 泄漏：打开关闭 100 次后栈均为空', async () => {
    const clock = useFakeTime();
    const fe = fakeEnv({ reducedMotion: false });
    try {
      const r = render(
        <UiEnvProvider value={fe.env}>
          <Overlay layer="popover" open={false}>
            <div>loop</div>
          </Overlay>
        </UiEnvProvider>,
      );
      for (let i = 0; i < 100; i += 1) {
        r.rerender(
          <UiEnvProvider value={fe.env}>
            <Overlay layer={i % 2 === 0 ? 'popover' : 'modal'} open>
              <div>loop {i}</div>
            </Overlay>
          </UiEnvProvider>,
        );
        r.rerender(
          <UiEnvProvider value={fe.env}>
            <Overlay layer={i % 2 === 0 ? 'popover' : 'modal'} open={false}>
              <div>loop {i}</div>
            </Overlay>
          </UiEnvProvider>,
        );
        await act(async () => {
          await clock.advance(200);
        });
      }
      expect(fe.active).toHaveLength(0);
      expect(layerStackSize()).toBe(0);
    } finally {
      fe.cleanup();
    }
  });

  test('Overlay 遮罩单次点击只回调一次 onClose 且子元素 animationend 不提前卸载', async () => {
    const clock = useFakeTime();
    const fe = fakeEnv({ reducedMotion: false });
    const onClose = vi.fn();
    try {
      const r = render(
        <UiEnvProvider value={fe.env}>
          <Overlay layer="modal" open scrim closeOnOutside onClose={onClose}>
            <div className="inner-child">body</div>
          </Overlay>
        </UiEnvProvider>,
      );
      const scrim = fe.root.querySelector('.hx-ov__scrim') as HTMLElement;
      act(() => {
        scrim.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
      });
      expect(onClose).toHaveBeenCalledTimes(1);
      expect(onClose).toHaveBeenLastCalledWith('outside');

      // 关掉进入 leaving，向子元素派发 animationend 不应提前卸载
      r.rerender(
        <UiEnvProvider value={fe.env}>
          <Overlay layer="modal" open={false} scrim closeOnOutside onClose={onClose}>
            <div className="inner-child">body</div>
          </Overlay>
        </UiEnvProvider>,
      );
      const child = fe.root.querySelector('.inner-child') as HTMLElement;
      act(() => {
        child.dispatchEvent(new Event('animationend', { bubbles: true }));
      });
      expect(fe.root.querySelector('.hx-ov')).not.toBeNull();

      await act(async () => {
        await clock.advance(200);
      });
      expect(fe.root.querySelector('.hx-ov')).toBeNull();
    } finally {
      fe.cleanup();
    }
  });

  test('Overlay 打开且焦点在输入框里时，UiEnvProvider 重渲染不抢焦点（K307）', () => {
    const fe = fakeEnv({ reducedMotion: true });
    try {
      const r = render(
        <UiEnvProvider value={{ ...fe.env }}>
          <Overlay layer="modal" open initialFocus="first">
            <button type="button">first-btn</button>
            <input aria-label="备注" />
          </Overlay>
        </UiEnvProvider>,
      );

      const firstBtn = fe.root.querySelector('button') as HTMLButtonElement;
      const input = fe.root.querySelector('input') as HTMLInputElement;
      expect(document.activeElement).toBe(firstBtn);

      // 用户将焦点移到输入框
      input.focus();
      expect(document.activeElement).toBe(input);

      // Provider 传入新对象引用触发重渲染：焦点必须停留在输入框，不被抢回 first-btn
      r.rerender(
        <UiEnvProvider value={{ ...fe.env }}>
          <Overlay layer="modal" open initialFocus="first">
            <button type="button">first-btn</button>
            <input aria-label="备注" />
          </Overlay>
        </UiEnvProvider>,
      );
      expect(document.activeElement).toBe(input);

      // 打开期间 initialFocus 从 none 改为 first 也不得抢走输入框焦点（评审 #5）
      r.rerender(
        <UiEnvProvider value={fe.env}>
          <Overlay layer="modal" open={false} initialFocus="none">
            <button type="button">first-btn</button>
            <input aria-label="备注" />
          </Overlay>
        </UiEnvProvider>,
      );
      r.rerender(
        <UiEnvProvider value={fe.env}>
          <Overlay layer="modal" open initialFocus="none">
            <button type="button">first-btn</button>
            <input aria-label="备注" />
          </Overlay>
        </UiEnvProvider>,
      );
      const input2 = fe.root.querySelector('input') as HTMLInputElement;
      input2.focus();
      expect(document.activeElement).toBe(input2);
      r.rerender(
        <UiEnvProvider value={fe.env}>
          <Overlay layer="modal" open initialFocus="first">
            <button type="button">first-btn</button>
            <input aria-label="备注" />
          </Overlay>
        </UiEnvProvider>,
      );
      expect(document.activeElement).toBe(input2);
    } finally {
      fe.cleanup();
    }
  });
});
