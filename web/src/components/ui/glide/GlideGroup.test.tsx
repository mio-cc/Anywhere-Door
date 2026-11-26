import { describe, expect, test, vi } from 'vitest';
import { act, fakeEnv, fakeRaf, render, stubElementFromPoint } from '@tests/kit';
import { UiEnvProvider } from '../overlay/UiEnv';
import { GlideGroup } from './GlideGroup';

function makeRect(left: number, top: number, width: number, height: number): DOMRect {
  return {
    x: left,
    y: top,
    left,
    top,
    width,
    height,
    right: left + width,
    bottom: top + height,
    toJSON: () => ({}),
  };
}

describe('W.08.01 GlideGroup', () => {
  test('W0801-T11 GlideGroup：FLIP 过渡、离开淡出、分组外隐藏且全程零 React 重渲染', () => {
    const raf = fakeRaf();
    const rafSpy = vi.spyOn(window, 'requestAnimationFrame').mockImplementation(raf.raf);
    const cafSpy = vi.spyOn(window, 'cancelAnimationFrame').mockImplementation(raf.caf);
    const fe = fakeEnv({ reducedMotion: false });
    let renderCount = 0;

    function Host() {
      renderCount += 1;
      return (
        <UiEnvProvider value={fe.env}>
          <GlideGroup itemSelector=".hx-item" variant="row">
            <div className="hx-item" data-idx="1">
              one
            </div>
            <div className="hx-item" data-idx="2">
              two
            </div>
            <div className="hx-item" data-idx="3">
              three
            </div>
          </GlideGroup>
        </UiEnvProvider>
      );
    }

    try {
      const r = render(<Host />);
      const grp = r.container.querySelector('.hx-glide') as HTMLElement;
      const blk = grp.querySelector('.hx-glide__block') as HTMLElement;
      const items = Array.from(grp.querySelectorAll<HTMLElement>('.hx-item'));
      grp.getBoundingClientRect = () => makeRect(100, 200, 220, 120);
      items[0]!.getBoundingClientRect = () => makeRect(110, 240, 200, 28);
      items[2]!.getBoundingClientRect = () => makeRect(110, 272, 200, 32);

      const transforms: string[] = [];
      const desc = Object.getOwnPropertyDescriptor(CSSStyleDeclaration.prototype, 'transform');
      Object.defineProperty(blk.style, 'transform', {
        configurable: true,
        set(v: string) {
          transforms.push(v);
          if (desc?.set) {
            // eslint-disable-next-line @typescript-eslint/unbound-method -- 显式通过 Reflect.apply 绑定 this
            Reflect.apply(desc.set, this, [v]);
          }
        },
        get() {
          return transforms[transforms.length - 1] ?? '';
        },
      });

      // 命中第 1 项
      stubElementFromPoint(() => items[0]!);
      act(() => {
        grp.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, clientX: 120, clientY: 250 }));
        raf.step();
      });
      expect(blk.classList.contains('is-on')).toBe(true);
      expect(transforms[transforms.length - 1]).toBe('translate(10px,40px)');

      // 滑到第 3 项：先写 flipFrom 再写 at
      transforms.length = 0;
      stubElementFromPoint(() => items[2]!);
      act(() => {
        grp.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, clientX: 120, clientY: 280 }));
        raf.step();
      });
      expect(transforms).toEqual(['translate(10px,40px) scale(1,0.875)', 'translate(10px,72px)']);

      // 命中分组外元素 → 隐藏
      const outside = document.createElement('div');
      stubElementFromPoint(() => outside);
      act(() => {
        grp.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, clientX: 0, clientY: 0 }));
        raf.step();
      });
      expect(blk.classList.contains('is-on')).toBe(false);

      // 重新显示后再离开 → 去掉 is-on
      stubElementFromPoint(() => items[0]!);
      act(() => {
        grp.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, clientX: 120, clientY: 250 }));
        raf.step();
      });
      act(() => {
        grp.dispatchEvent(new PointerEvent('pointerout', { bubbles: true, relatedTarget: document.body }));
      });
      expect(blk.classList.contains('is-on')).toBe(false);
      expect(renderCount).toBe(1);
    } finally {
      rafSpy.mockRestore();
      cafSpy.mockRestore();
      fe.cleanup();
    }
  });

  test('W0801-T12 减少动效：reducedMotion=true 直接到位不写 scale', () => {
    const raf = fakeRaf();
    const rafSpy = vi.spyOn(window, 'requestAnimationFrame').mockImplementation(raf.raf);
    const cafSpy = vi.spyOn(window, 'cancelAnimationFrame').mockImplementation(raf.caf);
    const fe = fakeEnv({ reducedMotion: true });
    try {
      const r = render(
        <UiEnvProvider value={fe.env}>
          <GlideGroup itemSelector=".hx-item" variant="row">
            <div className="hx-item">one</div>
            <div className="hx-item">two</div>
          </GlideGroup>
        </UiEnvProvider>,
      );
      const grp = r.container.querySelector('.hx-glide') as HTMLElement;
      const blk = grp.querySelector('.hx-glide__block') as HTMLElement;
      const items = Array.from(grp.querySelectorAll<HTMLElement>('.hx-item'));
      grp.getBoundingClientRect = () => makeRect(0, 0, 200, 100);
      items[0]!.getBoundingClientRect = () => makeRect(0, 0, 200, 28);
      items[1]!.getBoundingClientRect = () => makeRect(0, 32, 200, 32);

      const transforms: string[] = [];
      Object.defineProperty(blk.style, 'transform', {
        configurable: true,
        set(v: string) {
          transforms.push(v);
        },
        get() {
          return transforms[transforms.length - 1] ?? '';
        },
      });

      stubElementFromPoint(() => items[0]!);
      act(() => {
        grp.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, clientX: 10, clientY: 10 }));
        raf.step();
      });
      stubElementFromPoint(() => items[1]!);
      act(() => {
        grp.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, clientX: 10, clientY: 40 }));
        raf.step();
      });
      expect(transforms.some((t) => t.includes('scale('))).toBe(false);
    } finally {
      rafSpy.mockRestore();
      cafSpy.mockRestore();
      fe.cleanup();
    }
  });
});
