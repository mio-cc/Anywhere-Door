import { describe, expect, expectTypeOf, test, vi } from 'vitest';
import { act, fakeEnv, render, user } from '@tests/kit';
import { UiEnvProvider } from '../overlay/UiEnv';
import type { NumberStepperProps } from './NumberStepper';
import type { SearchInputProps } from './SearchInput';
import type { SliderProps } from './Slider';
import type { TextAreaProps } from './TextArea';
import { TextInput, type TextInputProps } from './TextInput';

describe('W.08.02 TextInput 与 props 对照', () => {
  test('W0802-T1 props 对照', () => {
    expectTypeOf<TextInputProps['value']>().toEqualTypeOf<string>();
    expectTypeOf<TextInputProps['onCommit']>().toEqualTypeOf<((v: string) => void) | undefined>();
    expectTypeOf<TextAreaProps['autosize']>().toEqualTypeOf<boolean | undefined>();
    expectTypeOf<NumberStepperProps['onStart']>().toEqualTypeOf<() => void>();
    expectTypeOf<SliderProps['step']>().toEqualTypeOf<number>();
    expectTypeOf<SearchInputProps['onClear']>().toEqualTypeOf<() => void>();
    expect(true).toBe(true);
  });

  test('W0802-T2 提交与取消：Enter/blur 提交，Esc 取消后 blur 不提交，无 onCancel 冒泡', async () => {
    const onCommit = vi.fn();
    const onCancel = vi.fn();
    const parentEsc = vi.fn();

    const r = render(
      <div
        onKeyDown={(e) => {
          if (e.key === 'Escape') parentEsc();
        }}
      >
        <TextInput value="abc" ariaLabel="名称" onChange={vi.fn()} onCommit={onCommit} onCancel={onCancel} />
      </div>,
    );
    const input = r.getByRole('textbox', { name: '名称' }) as HTMLInputElement;
    const parentEnter = vi.fn();
    r.rerender(
      <div
        onKeyDown={(e) => {
          if (e.key === 'Escape') parentEsc();
          if (e.key === 'Enter') parentEnter();
        }}
      >
        <TextInput value="abc" ariaLabel="名称" onChange={vi.fn()} onCommit={onCommit} onCancel={onCancel} />
      </div>,
    );
    act(() => {
      input.focus();
    });

    await user.key('Enter', input);
    expect(onCommit).toHaveBeenCalledTimes(1);
    expect(onCommit).toHaveBeenLastCalledWith('abc');
    expect(parentEnter).not.toHaveBeenCalled();

    act(() => {
      input.blur();
    });
    expect(onCommit).toHaveBeenCalledTimes(2);

    // Esc → onCancel 1 次、stopPropagation，随后 blur 不再 onCommit
    act(() => {
      input.focus();
    });
    await user.key('Escape', input);
    expect(onCancel).toHaveBeenCalledTimes(1);
    expect(parentEsc).not.toHaveBeenCalled();
    act(() => {
      input.blur();
    });
    expect(onCommit).toHaveBeenCalledTimes(2);

    // 无 onCancel 时 Esc 冒泡
    r.rerender(
      <div
        onKeyDown={(e) => {
          if (e.key === 'Escape') parentEsc();
        }}
      >
        <TextInput value="abc" ariaLabel="名称" onChange={vi.fn()} onCommit={onCommit} />
      </div>,
    );
    await user.key('Escape', r.getByRole('textbox', { name: '名称' }));
    expect(parentEsc).toHaveBeenCalledTimes(1);
  });

  test('W0802-T3 组字：isComposing=true 或 keyCode=229 按 Enter 不提交', () => {
    const onCommit = vi.fn();
    const r = render(<TextInput value="中" ariaLabel="名称" onChange={vi.fn()} onCommit={onCommit} />);
    const input = r.getByRole('textbox', { name: '名称' });

    act(() => {
      input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', isComposing: true, bubbles: true }));
      const ev229 = new KeyboardEvent('keydown', { key: 'Enter', bubbles: true });
      Object.defineProperty(ev229, 'keyCode', { value: 229 });
      input.dispatchEvent(ev229);
    });
    expect(onCommit).not.toHaveBeenCalled();
  });

  test('W0802-T4 错误与锁定：error 渲染 .hx-ferr/aria-invalid/aria-describedby，lockReason 只读带锁图标与提示框', () => {
    const fe = fakeEnv({ reducedMotion: true });
    try {
      const r = render(
        <UiEnvProvider value={fe.env}>
          <TextInput
            value="v"
            ariaLabel="表达式"
            onChange={vi.fn()}
            error="找不到字段 achived"
            lockReason="由节点类型定义"
          />
        </UiEnvProvider>,
      );
      const input = r.getByRole('textbox', { name: '表达式' }) as HTMLInputElement;
      const err = r.container.querySelector('.hx-ferr');
      expect(err?.textContent).toBe('找不到字段 achived');
      expect(input.getAttribute('aria-invalid')).toBe('true');
      expect(input.getAttribute('aria-describedby')).toBe(err?.id);
      expect(input.readOnly).toBe(true);
      const lockIcon = r.container.querySelector('.hx-input__lock svg');
      expect(lockIcon?.getAttribute('role')).toBe('img');
      expect(lockIcon?.getAttribute('aria-label')).toBe('已锁定');

      const wrap = r.container.querySelector('.hx-input') as HTMLElement;
      Object.defineProperty(wrap, 'matches', { configurable: true, value: () => true });
      act(() => {
        wrap.dispatchEvent(
          new PointerEvent('pointerover', {
            bubbles: true,
            pointerType: 'mouse',
            relatedTarget: document.body,
          }),
        );
        input.focus();
      });
      expect(fe.root.textContent).toContain('由节点类型定义');
    } finally {
      fe.cleanup();
    }
  });
});
