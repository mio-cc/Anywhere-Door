import { describe, expect, expectTypeOf, test, vi } from 'vitest';
import { act, fakeEnv, render, useFakeTime, user } from '@tests/kit';
import { UiEnvProvider } from '../overlay/UiEnv';
import type { PickerProps } from './Picker';
import type { SegmentedProps } from './Segmented';
import { Select, type SelectOption, type SelectProps } from './Select';
import type { SwitchProps } from './Switch';
import type { TabsProps } from './Tabs';

vi.mock('@floating-ui/dom', () => ({
  computePosition: vi.fn().mockResolvedValue({ x: 10, y: 40, placement: 'bottom-start' }),
  autoUpdate: vi.fn((_r: unknown, _f: unknown, u: () => void) => {
    u();
    return vi.fn();
  }),
  offset: vi.fn((n: number) => ({ name: 'offset', n })),
  flip: vi.fn((o: unknown) => ({ name: 'flip', o })),
  shift: vi.fn((o: unknown) => ({ name: 'shift', o })),
  size: vi.fn((o: unknown) => ({ name: 'size', o })),
}));

describe('W.08.03 Select 与 props 对照', () => {
  test('W0803-T1 props 对照', () => {
    expectTypeOf<SelectProps['value']>().toEqualTypeOf<string | null>();
    expectTypeOf<SegmentedProps['value']>().toEqualTypeOf<string>();
    expectTypeOf<SwitchProps['checked']>().toEqualTypeOf<boolean>();
    expectTypeOf<TabsProps['variant']>().toEqualTypeOf<'underline' | 'pill'>();
    expectTypeOf<PickerProps['emptyText']>().toEqualTypeOf<string>();
    expect(true).toBe(true);
  });

  test('W0803-T3 Select 键盘：打开压栈、方向键跳过禁用、Enter 选中并焦点回位、Esc 取消、Tab 选中', async () => {
    const fe = fakeEnv({ reducedMotion: true });
    const onChange = vi.fn();
    const options: SelectOption[] = [
      { value: 'a', label: '选项 A' },
      { value: 'b', label: '选项 B', disabled: true, disabledReason: '暂不可用' },
      { value: 'c', label: '选项 C' },
      { value: 'd', label: '选项 D' },
    ];

    try {
      const r = render(
        <UiEnvProvider value={fe.env}>
          <Select value="a" options={options} onChange={onChange} ariaLabel="模型选择" />
        </UiEnvProvider>,
      );
      const trigger = r.getByRole('combobox', { name: '模型选择' });
      trigger.focus();

      await user.key('ArrowDown', trigger);
      expect(fe.active).toHaveLength(1);

      // ↓ 跳过禁用 b 到 c，再 ↓ 到 d，Enter 选中 d
      act(() => {
        fe.dispatchKey({ key: 'ArrowDown' });
      });
      act(() => {
        fe.dispatchKey({ key: 'ArrowDown' });
      });
      act(() => {
        fe.dispatchKey({ key: 'Enter' });
      });
      expect(onChange).toHaveBeenCalledWith('d');
      expect(fe.active).toHaveLength(0);
      expect(document.activeElement).toBe(trigger);

      // 再次打开按 Esc 不触发 onChange
      onChange.mockClear();
      await user.key('ArrowDown', trigger);
      act(() => {
        fe.dispatchKey({ key: 'ArrowDown' });
      });
      act(() => {
        fe.dispatchKey({ key: 'Escape' });
      });
      expect(onChange).not.toHaveBeenCalled();

      // 再次打开按 Tab 选中当前高亮项 c
      await user.key('ArrowDown', trigger);
      act(() => {
        fe.dispatchKey({ key: 'ArrowDown' });
      });
      act(() => {
        fe.dispatchKey({ key: 'Tab' });
      });
      expect(onChange).toHaveBeenCalledWith('c');
    } finally {
      fe.cleanup();
    }
  });

  test('W0803-T4 Select 过滤：9 项自动出过滤框并聚焦、filterable=false 无过滤框、无结果与同值不回调', async () => {
    const fe = fakeEnv({ reducedMotion: true });
    const onChange = vi.fn();
    const nine: SelectOption[] = Array.from({ length: 9 }, (_, i) => ({
      value: `v${String(i)}`,
      label: `Option ${String(i)}`,
    }));

    try {
      const r = render(
        <UiEnvProvider value={fe.env}>
          <Select value="v0" options={nine} onChange={onChange} ariaLabel="模型列表" />
        </UiEnvProvider>,
      );
      const trigger = r.getByRole('combobox', { name: '模型列表' });
      await user.click(trigger);

      const filterInput = fe.root.querySelector('input[aria-label="筛选"]') as HTMLInputElement;
      expect(filterInput).not.toBeNull();
      expect(document.activeElement).toBe(filterInput);

      // 选同值 v0 不调 onChange
      const opt0 = fe.root.querySelector('[role="option"]') as HTMLElement;
      await user.click(opt0);
      expect(onChange).not.toHaveBeenCalled();

      // 重新打开并输入无匹配内容 → ui.noMatch
      await user.click(trigger);
      const filter2 = fe.root.querySelector('input[aria-label="筛选"]') as HTMLInputElement;
      await user.type(filter2, 'zzzz');
      expect(fe.root.textContent).toContain('没有匹配的选项');

      // filterable=false 时不出现过滤框
      r.rerender(
        <UiEnvProvider value={fe.env}>
          <Select value="v0" options={nine} filterable={false} onChange={onChange} ariaLabel="模型列表" />
        </UiEnvProvider>,
      );
      await user.click(r.getByRole('combobox', { name: '模型列表' }));
      expect(fe.root.querySelector('input[aria-label="筛选"]')).toBeNull();
    } finally {
      fe.cleanup();
    }
  });

  test('W0803-T5 Select 禁用项：点击不选不关，悬停显示 disabledReason', async () => {
    const clock = useFakeTime();
    const fe = fakeEnv({ reducedMotion: true });
    const onChange = vi.fn();
    const options: SelectOption[] = [
      { value: 'ok', label: '可用' },
      { value: 'bad', label: '不可用', disabled: true, disabledReason: '缺少模型密钥' },
    ];

    try {
      const r = render(
        <UiEnvProvider value={fe.env}>
          <Select value="ok" options={options} onChange={onChange} ariaLabel="选择项" />
        </UiEnvProvider>,
      );
      await user.click(r.getByRole('combobox', { name: '选择项' }));
      const disabledOpt = Array.from(fe.root.querySelectorAll('[role="option"]')).find(
        (el) => el.getAttribute('aria-disabled') === 'true',
      ) as HTMLElement;

      await user.click(disabledOpt);
      expect(onChange).not.toHaveBeenCalled();
      expect(fe.active).toHaveLength(1);

      await act(async () => {
        disabledOpt.dispatchEvent(
          new PointerEvent('pointerover', {
            bubbles: true,
            pointerType: 'mouse',
            relatedTarget: document.body,
          }),
        );
        await clock.advance(400);
      });
      expect(fe.root.textContent).toContain('缺少模型密钥');
    } finally {
      fe.cleanup();
    }
  });

  test('Select 250 项窗口化：打开深位置当前项可见、desc 匹配加粗、无嵌套 listbox', async () => {
    const fe = fakeEnv({ reducedMotion: true });
    const many: SelectOption[] = Array.from({ length: 250 }, (_, i) => ({
      value: `m${String(i)}`,
      label: `Model ${String(i)}`,
      desc: i === 240 ? '12.8 万上下文' : `ctx ${String(i)}`,
    }));

    try {
      const r = render(
        <UiEnvProvider value={fe.env}>
          <Select value="m240" options={many} onChange={vi.fn()} ariaLabel="长下拉" />
        </UiEnvProvider>,
      );
      await user.click(r.getByRole('combobox', { name: '长下拉' }));
      // 只有一个 listbox（外层 Popover 为 dialog，无嵌套 listbox）
      expect(fe.root.querySelectorAll('[role="listbox"]')).toHaveLength(1);
      // 第 240 项已滚入渲染窗口
      expect(fe.root.textContent).toContain('Model 240');

      // 过滤匹配 desc 时 desc 同样加粗
      const filterInput = fe.root.querySelector('input[aria-label="筛选"]') as HTMLInputElement;
      await user.type(filterInput, '12.8');
      expect(fe.root.querySelector('.hx-select__optdesc b')?.textContent).toBe('12.8');
    } finally {
      fe.cleanup();
    }
  });

  test('Select 600 项滚动窗口化与含空格 value 的安全 DOM id（K307）', async () => {
    const fe = fakeEnv({ reducedMotion: true });
    const many: SelectOption[] = Array.from({ length: 600 }, (_, i) => ({
      value: `model ${String(i)}`,
      label: `Model ${String(i)}`,
    }));

    try {
      const r = render(
        <UiEnvProvider value={fe.env}>
          <Select value="model 0" options={many} onChange={vi.fn()} ariaLabel="超长下拉" />
        </UiEnvProvider>,
      );
      await user.click(r.getByRole('combobox', { name: '超长下拉' }));

      // 含空格 value 生成无空格 DOM id，并与过滤输入框 aria-activedescendant 匹配
      const filterInput = fe.root.querySelector('input[aria-label="筛选"]') as HTMLInputElement;
      const activeDesc = filterInput.getAttribute('aria-activedescendant');
      expect(activeDesc).toBeTruthy();
      expect(/\s/.test(activeDesc ?? '')).toBe(false);
      expect(document.getElementById(activeDesc ?? '')?.textContent).toContain('Model 0');

      // 对滚动容器 ScrollArea 触发 scroll(scrollTop=5000)，渲染行切到对应深位置且不被拉回当前项
      const scrollEl = fe.root.querySelector('.hx-select__scroll') as HTMLDivElement;
      act(() => {
        scrollEl.scrollTop = 5000;
        scrollEl.dispatchEvent(new Event('scroll'));
      });
      expect(scrollEl.scrollTop).toBe(5000);
      // 行高 28px，5000 / 28 ≈ 178
      expect(fe.root.textContent).toContain('Model 178');
      expect(fe.root.textContent).not.toContain('Model 0');

      // 含空格、空串、字面量 %20、未配对代理项均生成唯一且无空格 DOM id，不抛异常
      r.rerender(
        <UiEnvProvider value={fe.env}>
          <Select
            value="a b"
            options={[
              { value: '', label: '空值' },
              { value: 'a b', label: '含空格' },
              { value: 'a%20b', label: '字面量编码' },
              { value: '\uD800', label: '孤立代理项' },
            ]}
            onChange={vi.fn()}
            ariaLabel="边界下拉"
          />
        </UiEnvProvider>,
      );
      const edgeOpts = Array.from(fe.root.querySelectorAll('[role="option"]'));
      const edgeDomIds = edgeOpts.map((el) => el.id);
      expect(new Set(edgeDomIds).size).toBe(4);
      for (const id of edgeDomIds) {
        expect(/\s/.test(id)).toBe(false);
        expect(document.getElementById(id)).not.toBeNull();
      }
    } finally {
      fe.cleanup();
    }
  });
});
