import { beforeEach, describe, expect, test, vi } from 'vitest';
import { act, fakeEnv, render, useFakeTime } from '@tests/kit';
import { Menu, type MenuItem } from './Menu';
import { UiEnvProvider } from './UiEnv';

const mockComputePosition = vi.fn().mockResolvedValue({ x: 40, y: 50, placement: 'right-start' });
vi.mock('@floating-ui/dom', () => ({
  computePosition: (...args: unknown[]) => mockComputePosition(...args) as Promise<unknown>,
  autoUpdate: vi.fn((_r: unknown, _f: unknown, u: () => void) => {
    u();
    return vi.fn();
  }),
  offset: vi.fn((n: number) => ({ name: 'offset', n })),
  flip: vi.fn((o: unknown) => ({ name: 'flip', o })),
  shift: vi.fn((o: unknown) => ({ name: 'shift', o })),
  size: vi.fn((o: unknown) => ({ name: 'size', o })),
}));

describe('W.08.04 Menu', () => {
  beforeEach(() => {
    mockComputePosition.mockClear();
  });

  test('W0804-T10 Menu 键盘：跳过分组与分隔线、禁用项可高亮不触发、子菜单与 typeahead', () => {
    const fe = fakeEnv({ reducedMotion: true });
    const order: string[] = [];
    const onPick = vi.fn((id: string) => order.push(`pick:${id}`));
    const onClose = vi.fn((r: string) => order.push(`close:${r}`));
    const items: MenuItem[] = [
      { group: '基本' },
      { id: 'copy', label: '复制', shortcut: 'Ctrl+C', run: () => order.push('run:copy') },
      'sep',
      { id: 'locked', label: '删除', disabled: true, disabledReason: '已冻结' },
      {
        id: 'more',
        label: '更多操作',
        children: [{ id: 'sub1', label: '子项一', run: () => order.push('run:sub1') }],
      },
    ];

    try {
      render(
        <UiEnvProvider value={fe.env}>
          <Menu at={{ x: 100, y: 120 }} items={items} onPick={onPick} onClose={onClose} />
        </UiEnvProvider>,
      );

      // 打开时无高亮
      expect(fe.root.querySelector('.hx-menu__item.is-active')).toBeNull();

      // ArrowDown → 第一项“复制”
      act(() => {
        fe.dispatchKey({ key: 'ArrowDown' });
      });
      expect(fe.root.querySelector('.hx-menu__item.is-active')?.textContent).toContain('复制');

      // 再 ArrowDown → 禁用项“删除”（可高亮）
      act(() => {
        fe.dispatchKey({ key: 'ArrowDown' });
      });
      expect(fe.root.querySelector('.hx-menu__item.is-active')?.textContent).toContain('删除');
      // 禁用项按 Enter 不触发
      act(() => {
        fe.dispatchKey({ key: 'Enter' });
      });
      expect(onPick).not.toHaveBeenCalled();

      // ArrowDown → “更多操作”，ArrowRight 打开子菜单（压栈第二层）
      act(() => {
        fe.dispatchKey({ key: 'ArrowDown' });
      });
      act(() => {
        fe.dispatchKey({ key: 'ArrowRight' });
      });
      expect(fe.active).toHaveLength(2);

      // 子菜单 ArrowLeft 关闭子菜单
      act(() => {
        fe.dispatchKey({ key: 'ArrowLeft' });
      });
      expect(fe.active).toHaveLength(1);

      // typeahead “复” 命中 “复制” 并 Enter 执行：run 先于 onPick 再 onClose('action')
      act(() => {
        fe.dispatchKey({ key: '复' });
      });
      expect(fe.root.querySelector('.hx-menu__item.is-active')?.textContent).toContain('复制');
      act(() => {
        fe.dispatchKey({ key: 'Enter' });
      });
      expect(order).toEqual(['run:copy', 'pick:copy', 'close:action']);
    } finally {
      fe.cleanup();
    }
  });

  test('W0804-T11 Menu 位置：点坐标用 right-start，元素锚点用 bottom-start', () => {
    const fe = fakeEnv({ reducedMotion: true });
    const btn = document.createElement('button');
    document.body.appendChild(btn);
    btn.getBoundingClientRect = () => ({
      x: 10,
      y: 20,
      width: 40,
      height: 28,
      top: 20,
      left: 10,
      right: 50,
      bottom: 48,
      toJSON: () => ({}),
    });
    try {
      const r = render(
        <UiEnvProvider value={fe.env}>
          <Menu at={{ x: 10, y: 20 }} items={[{ id: 'a', label: 'A' }]} onPick={vi.fn()} onClose={vi.fn()} />
        </UiEnvProvider>,
      );
      const opts1 = mockComputePosition.mock.calls[0]?.[2] as { placement: string };
      expect(opts1.placement).toBe('right-start');

      mockComputePosition.mockClear();
      r.rerender(
        <UiEnvProvider value={fe.env}>
          <Menu at={btn} items={[{ id: 'a', label: 'A' }]} onPick={vi.fn()} onClose={vi.fn()} />
        </UiEnvProvider>,
      );
      const opts2 = mockComputePosition.mock.calls[0]?.[2] as { placement: string };
      expect(opts2.placement).toBe('bottom-start');
    } finally {
      btn.remove();
      fe.cleanup();
    }
  });

  test('Menu aria-activedescendant 挂在 role="menu" 根元素，且离开父项 300ms 关闭子菜单', async () => {
    const clock = useFakeTime();
    const fe = fakeEnv({ reducedMotion: true });
    const items: MenuItem[] = [
      { id: 'more', label: '更多', children: [{ id: 'sub1', label: '子项' }] },
      { id: 'other', label: '其他' },
    ];
    try {
      render(
        <UiEnvProvider value={fe.env}>
          <Menu at={{ x: 10, y: 10 }} items={items} onPick={vi.fn()} onClose={vi.fn()} />
        </UiEnvProvider>,
      );
      const menuRoot = fe.root.querySelector('[role="menu"]') as HTMLElement;
      expect(menuRoot.getAttribute('aria-label')).toBe('菜单');
      act(() => {
        fe.dispatchKey({ key: 'ArrowDown' });
      });
      expect(menuRoot.getAttribute('aria-activedescendant')).toContain('-more');

      // 打开子菜单，指针离开父项后 300ms 未进入子菜单则自动关闭
      act(() => {
        fe.dispatchKey({ key: 'ArrowRight' });
      });
      expect(fe.root.querySelectorAll('[role="menu"]')).toHaveLength(2);
      const parentItem = fe.root.querySelector('[role="menuitem"]') as HTMLElement;
      act(() => {
        parentItem.dispatchEvent(
          new PointerEvent('pointerout', { bubbles: true, relatedTarget: document.body }),
        );
      });
      await act(async () => {
        await clock.advance(300);
      });
      expect(fe.root.querySelectorAll('[role="menu"]')).toHaveLength(1);
    } finally {
      fe.cleanup();
    }
  });

  test('Menu 点坐标在方向键与悬停重渲染时不重复触发 place()（K307）', () => {
    const fe = fakeEnv({ reducedMotion: true });
    const items: MenuItem[] = [
      { id: 'a', label: 'A' },
      { id: 'b', label: 'B' },
    ];
    try {
      render(
        <UiEnvProvider value={fe.env}>
          <Menu at={{ x: 100, y: 200 }} items={items} onPick={vi.fn()} onClose={vi.fn()} />
        </UiEnvProvider>,
      );
      expect(mockComputePosition).toHaveBeenCalledTimes(1);

      act(() => {
        fe.dispatchKey({ key: 'ArrowDown' });
      });
      act(() => {
        fe.dispatchKey({ key: 'ArrowDown' });
      });
      const menuItems = fe.root.querySelectorAll('[role="menuitem"]');
      const firstItem = menuItems[0] as HTMLElement;
      const secondItem = menuItems[1] as HTMLElement;
      expect(secondItem.classList.contains('is-active')).toBe(true);

      // 悬停第一项触发 activeIdx 从 1 真实切到 0（非 bailout），验证仍不重复 place()
      act(() => {
        firstItem.dispatchEvent(new PointerEvent('pointerover', { bubbles: true }));
      });
      expect(firstItem.classList.contains('is-active')).toBe(true);
      expect(secondItem.classList.contains('is-active')).toBe(false);
      expect(mockComputePosition).toHaveBeenCalledTimes(1);
    } finally {
      fe.cleanup();
    }
  });
});
