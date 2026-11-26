import { describe, expect, test, vi } from 'vitest';
import { act, fakeEnv, fakeRaf, render, useFakeTime, user } from '@tests/kit';
import { UiEnvProvider } from '../overlay/UiEnv';
import { NumberStepper } from './NumberStepper';

function setInputValue(el: HTMLInputElement, next: string): void {
  const desc = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value');
  if (desc?.set) {
    // eslint-disable-next-line @typescript-eslint/unbound-method -- 显式通过 Reflect.apply 绑定 this
    Reflect.apply(desc.set, el, [next]);
  }
}

describe('W.08.02 NumberStepper', () => {
  test('W0802-T6 拖动手势：阈值 3px、同帧合并 <=1 次、松开 onEnd 且组件渲染次数不变', () => {
    const raf = fakeRaf();
    const rafSpy = vi.spyOn(window, 'requestAnimationFrame').mockImplementation(raf.raf);
    const cafSpy = vi.spyOn(window, 'cancelAnimationFrame').mockImplementation(raf.caf);
    const onStart = vi.fn();
    const onChange = vi.fn();
    const onEnd = vi.fn();
    let renders = 0;

    function Host() {
      renders += 1;
      return (
        <NumberStepper
          value={10}
          min={0}
          max={100}
          step={1}
          ariaLabel="轮数"
          onStart={onStart}
          onChange={onChange}
          onEnd={onEnd}
        />
      );
    }

    try {
      const r = render(<Host />);
      const spin = r.getByRole('spinbutton', { name: '轮数' });
      expect(renders).toBe(1);

      act(() => {
        spin.dispatchEvent(
          new PointerEvent('pointerdown', { bubbles: true, button: 0, pointerId: 1, clientX: 100 }),
        );
        spin.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, pointerId: 1, clientX: 102 }));
      });
      expect(onStart).not.toHaveBeenCalled();

      act(() => {
        for (let i = 1; i <= 30; i += 1) {
          spin.dispatchEvent(
            new PointerEvent('pointermove', { bubbles: true, pointerId: 1, clientX: 102 + i * 2 }),
          );
        }
      });
      expect(onStart).toHaveBeenCalledTimes(1);
      expect(onChange).toHaveBeenCalledTimes(0);

      act(() => {
        raf.step();
      });
      expect(onChange).toHaveBeenCalledTimes(1);

      act(() => {
        spin.dispatchEvent(new PointerEvent('pointerup', { bubbles: true, pointerId: 1, clientX: 162 }));
      });
      expect(onEnd).toHaveBeenCalledTimes(1);
      expect(renders).toBe(1);
    } finally {
      rafSpy.mockRestore();
      cafSpy.mockRestore();
    }
  });

  test('W0802-T7 长按：按住 + 1 秒产生 12 次 onChange 与 1 对 onStart/onEnd', async () => {
    const clock = useFakeTime();
    const onStart = vi.fn();
    const onChange = vi.fn();
    const onEnd = vi.fn();

    const r = render(
      <NumberStepper
        value={0}
        min={0}
        max={50}
        step={1}
        ariaLabel="数量"
        onStart={onStart}
        onChange={onChange}
        onEnd={onEnd}
      />,
    );
    const inc = r.getByRole('button', { name: '增加' });

    act(() => {
      inc.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0 }));
    });
    expect(onStart).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledTimes(1);

    await act(async () => {
      await clock.advance(1000);
    });
    expect(onChange).toHaveBeenCalledTimes(12);

    act(() => {
      inc.dispatchEvent(new PointerEvent('pointerup', { bubbles: true, button: 0 }));
    });
    expect(onEnd).toHaveBeenCalledTimes(1);
    expect(onEnd).toHaveBeenLastCalledWith(12);
  });

  test('W0802-T8 方向键合并：连按 5 次只产生 1 对手势，Shift+↑ +10 步，End 到 max', async () => {
    const clock = useFakeTime();
    const onStart = vi.fn();
    const onChange = vi.fn();
    const onEnd = vi.fn();

    const r = render(
      <NumberStepper
        value={0}
        min={0}
        max={100}
        step={1}
        ariaLabel="步长"
        onStart={onStart}
        onChange={onChange}
        onEnd={onEnd}
      />,
    );
    const spin = r.getByRole('spinbutton', { name: '步长' });
    act(() => {
      spin.focus();
    });

    for (let i = 0; i < 5; i += 1) {
      await act(async () => {
        spin.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowUp', bubbles: true, cancelable: true }));
        await clock.advance(100);
      });
    }
    expect(onStart).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledTimes(5);
    expect(onEnd).not.toHaveBeenCalled();

    await act(async () => {
      await clock.advance(400);
    });
    expect(onEnd).toHaveBeenCalledTimes(1);
    expect(onEnd).toHaveBeenLastCalledWith(5);

    act(() => {
      spin.dispatchEvent(
        new KeyboardEvent('keydown', { key: 'ArrowUp', shiftKey: true, bubbles: true, cancelable: true }),
      );
    });
    expect(onChange).toHaveBeenLastCalledWith(10);

    await act(async () => {
      spin.dispatchEvent(new KeyboardEvent('keydown', { key: 'End', bubbles: true, cancelable: true }));
      await clock.advance(500);
    });
    expect(onEnd).toHaveBeenLastCalledWith(100);
  });

  test('W0802-T9 滚轮：未聚焦不改值不 preventDefault，聚焦后改值并 preventDefault', () => {
    const onChange = vi.fn();
    const r = render(
      <NumberStepper
        value={5}
        min={0}
        max={20}
        step={1}
        ariaLabel="滚轮测试"
        onStart={vi.fn()}
        onChange={onChange}
        onEnd={vi.fn()}
      />,
    );
    const spin = r.getByRole('spinbutton', { name: '滚轮测试' });

    const evUnfocused = new WheelEvent('wheel', { deltaY: -100, cancelable: true, bubbles: true });
    act(() => {
      spin.dispatchEvent(evUnfocused);
    });
    expect(evUnfocused.defaultPrevented).toBe(false);
    expect(onChange).not.toHaveBeenCalled();

    act(() => {
      spin.focus();
    });
    const evFocused = new WheelEvent('wheel', { deltaY: -100, cancelable: true, bubbles: true });
    act(() => {
      spin.dispatchEvent(evFocused);
    });
    expect(evFocused.defaultPrevented).toBe(true);
    expect(onChange).toHaveBeenCalledWith(6);
  });

  test('W0802-T10 输入编辑：双击全选、非法不提交、越界夹紧并提示最大为 32', async () => {
    const fe = fakeEnv({ reducedMotion: true });
    const onChange = vi.fn();
    const onEnd = vi.fn();
    try {
      const r = render(
        <UiEnvProvider value={fe.env}>
          <NumberStepper
            value={5}
            min={1}
            max={32}
            step={1}
            ariaLabel="并发数"
            onStart={vi.fn()}
            onChange={onChange}
            onEnd={onEnd}
          />
        </UiEnvProvider>,
      );
      const spin = r.getByRole('spinbutton', { name: '并发数' });
      act(() => {
        spin.dispatchEvent(new MouseEvent('dblclick', { bubbles: true }));
      });
      const edit = r.getByRole('textbox', { name: '并发数' }) as HTMLInputElement;
      expect(document.activeElement).toBe(edit);

      act(() => {
        setInputValue(edit, 'abc');
        edit.dispatchEvent(new Event('input', { bubbles: true }));
      });
      await user.key('Enter', edit);
      expect(onChange).not.toHaveBeenCalled();
      expect(edit.getAttribute('aria-invalid')).toBe('true');

      act(() => {
        setInputValue(edit, '99');
        edit.dispatchEvent(new Event('input', { bubbles: true }));
      });
      await user.key('Enter', edit);
      expect(onChange).toHaveBeenCalledWith(32);
      expect(onEnd).toHaveBeenCalledWith(32);
      expect(fe.root.textContent).toContain('最大为 32');
    } finally {
      fe.cleanup();
    }
  });

  test('W0802-T11 卸载补发：拖动中卸载自动补发 onEnd', () => {
    const onStart = vi.fn();
    const onEnd = vi.fn();
    const r = render(
      <NumberStepper
        value={5}
        min={0}
        max={20}
        step={1}
        ariaLabel="卸载测试"
        onStart={onStart}
        onChange={vi.fn()}
        onEnd={onEnd}
      />,
    );
    const spin = r.getByRole('spinbutton', { name: '卸载测试' });
    act(() => {
      spin.dispatchEvent(
        new PointerEvent('pointerdown', { bubbles: true, button: 0, pointerId: 1, clientX: 10 }),
      );
      spin.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, pointerId: 1, clientX: 30 }));
    });
    expect(onStart).toHaveBeenCalledTimes(1);
    r.unmount();
    expect(onEnd).toHaveBeenCalledTimes(1);
  });

  test('NumberStepper 指针锁成功使用 movementX、锁丢失补发 onEnd、退出编辑恢复滚轮监听', async () => {
    const raf = fakeRaf();
    const rafSpy = vi.spyOn(window, 'requestAnimationFrame').mockImplementation(raf.raf);
    const cafSpy = vi.spyOn(window, 'cancelAnimationFrame').mockImplementation(raf.caf);
    const fe = fakeEnv({ reducedMotion: true });
    const onStart = vi.fn();
    const onChange = vi.fn();
    const onEnd = vi.fn();
    const exitSpy = vi.fn();
    let lockEl: Element | null = null;
    Object.defineProperty(document, 'pointerLockElement', {
      configurable: true,
      get: () => lockEl,
    });
    Object.defineProperty(document, 'exitPointerLock', {
      configurable: true,
      value: exitSpy,
    });

    try {
      const r = render(
        <UiEnvProvider value={fe.env}>
          <NumberStepper
            value={10}
            min={0}
            max={100}
            step={1}
            ariaLabel="锁测试"
            onStart={onStart}
            onChange={onChange}
            onEnd={onEnd}
          />
        </UiEnvProvider>,
      );
      const spin = r.getByRole('spinbutton', { name: '锁测试' });
      Object.defineProperty(spin, 'requestPointerLock', {
        configurable: true,
        value: () => {
          lockEl = spin;
          document.dispatchEvent(new Event('pointerlockchange'));
          return Promise.resolve();
        },
      });

      act(() => {
        spin.dispatchEvent(
          new PointerEvent('pointerdown', { bubbles: true, button: 0, pointerId: 1, clientX: 50 }),
        );
        spin.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, pointerId: 1, clientX: 58 }));
      });
      await Promise.resolve();

      // 锁定成功后按 movementX 计算（clientX 不变也能增减）
      const moveEv = new PointerEvent('pointermove', { bubbles: true, pointerId: 1, clientX: 58 });
      Object.defineProperty(moveEv, 'movementX', { value: 16 });
      act(() => {
        spin.dispatchEvent(moveEv);
        raf.step();
      });
      expect(onChange).toHaveBeenCalled();
      expect(fe.root.querySelector('.hx-step__float.is-on')).not.toBeNull();

      // 指针锁意外丢失自动补发 onEnd
      act(() => {
        lockEl = null;
        document.dispatchEvent(new Event('pointerlockchange'));
      });
      expect(onEnd).toHaveBeenCalledTimes(1);

      // 进入编辑再 Esc 退出，滚轮监听必须恢复生效
      act(() => {
        spin.dispatchEvent(new MouseEvent('dblclick', { bubbles: true }));
      });
      const edit = r.getByRole('textbox', { name: '锁测试' });
      await user.key('Escape', edit);
      const spin2 = r.getByRole('spinbutton', { name: '锁测试' });
      act(() => {
        spin2.focus();
      });
      onChange.mockClear();
      const wh = new WheelEvent('wheel', { deltaY: -100, cancelable: true, bubbles: true });
      act(() => {
        spin2.dispatchEvent(wh);
      });
      expect(wh.defaultPrevented).toBe(true);
      expect(onChange).toHaveBeenCalled();
      act(() => {
        spin2.blur();
      });

      // pointerdown 后、首次 move 前丢失指针捕获：清理 pendingDrag，后续 move 不误触发 onStart（第 2 轮评审 #16）
      onStart.mockClear();
      onEnd.mockClear();
      act(() => {
        spin2.dispatchEvent(
          new PointerEvent('pointerdown', { bubbles: true, button: 0, pointerId: 2, clientX: 50 }),
        );
        spin2.dispatchEvent(new PointerEvent('lostpointercapture', { bubbles: true, pointerId: 2 }));
        spin2.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, pointerId: 2, clientX: 90 }));
      });
      expect(onStart).not.toHaveBeenCalled();
      expect(onEnd).not.toHaveBeenCalled();

      // 拖动中（无指针锁，走捕获）丢失指针捕获 → 自动补发 onEnd 保证成对
      act(() => {
        spin2.dispatchEvent(
          new PointerEvent('pointerdown', { bubbles: true, button: 0, pointerId: 3, clientX: 50 }),
        );
        spin2.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, pointerId: 3, clientX: 70 }));
        spin2.dispatchEvent(new PointerEvent('lostpointercapture', { bubbles: true, pointerId: 3 }));
      });
      expect(onStart).toHaveBeenCalledTimes(1);
      expect(onEnd).toHaveBeenCalledTimes(1);
    } finally {
      Object.defineProperty(document, 'pointerLockElement', { configurable: true, get: () => null });
      rafSpy.mockRestore();
      cafSpy.mockRestore();
      fe.cleanup();
    }
  });
});
