import { describe, expect, test, vi } from 'vitest';
import { act, fakeEnv, render } from '@tests/kit';
import { UiEnvProvider } from '../overlay/UiEnv';
import { Slider } from './Slider';

describe('W.08.02 Slider', () => {
  test('W0802-T12 Slider：手势成对调用且 --_p 等于百分比', () => {
    const onStart = vi.fn();
    const onChange = vi.fn();
    const onEnd = vi.fn();

    const r = render(
      <Slider
        value={0.5}
        min={0}
        max={2}
        step={0.1}
        ariaLabel="温度"
        onStart={onStart}
        onChange={onChange}
        onEnd={onEnd}
      />,
    );
    const input = r.getByRole('slider', { name: '温度' }) as HTMLInputElement;
    expect(input.style.getPropertyValue('--_p')).toBe('25%');

    act(() => {
      input.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
    });
    expect(onStart).toHaveBeenCalledTimes(1);

    act(() => {
      const desc = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value');
      if (desc?.set) {
        // eslint-disable-next-line @typescript-eslint/unbound-method -- 显式通过 Reflect.apply 绑定 this
        Reflect.apply(desc.set, input, ['1.2']);
      }
      input.dispatchEvent(new Event('input', { bubbles: true }));
    });
    expect(onChange).toHaveBeenCalledWith(1.2);

    act(() => {
      input.dispatchEvent(new PointerEvent('pointerup', { bubbles: true }));
    });
    expect(onEnd).toHaveBeenCalledTimes(1);
  });

  test('Slider 手势中 disabled 变化补发 onEnd，拖动时浮出数值写明定位坐标', () => {
    const fe = fakeEnv({ reducedMotion: true });
    const onStart = vi.fn();
    const onEnd = vi.fn();
    try {
      const r = render(
        <UiEnvProvider value={fe.env}>
          <Slider
            value={1}
            min={0}
            max={2}
            step={0.1}
            ariaLabel="温度"
            onStart={onStart}
            onChange={vi.fn()}
            onEnd={onEnd}
          />
        </UiEnvProvider>,
      );
      const input = r.getByRole('slider', { name: '温度' });
      input.getBoundingClientRect = () => ({
        x: 100,
        y: 80,
        left: 100,
        top: 80,
        width: 200,
        height: 4,
        right: 300,
        bottom: 84,
        toJSON: () => ({}),
      });
      act(() => {
        input.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
      });
      const valFloat = fe.root.querySelector('.hx-slider__val') as HTMLElement;
      expect(valFloat.style.left).toBe('200px');
      expect(valFloat.style.top).toBe('74px');

      // 手势进行中变为 disabled → 自动补发 onEnd
      r.rerender(
        <UiEnvProvider value={fe.env}>
          <Slider
            value={1}
            min={0}
            max={2}
            step={0.1}
            disabled
            ariaLabel="温度"
            onStart={onStart}
            onChange={vi.fn()}
            onEnd={onEnd}
          />
        </UiEnvProvider>,
      );
      expect(onEnd).toHaveBeenCalledTimes(1);
    } finally {
      fe.cleanup();
    }
  });
});
