import { createRef } from 'react';
import { describe, expect, test, vi } from 'vitest';
import { act, render } from '@tests/kit';
import { Picker, pickerOptionId, type PickerGroup, type PickerHandle } from './Picker';

describe('W.08.03 Picker', () => {
  test('W0803-T9 Picker 键盘：ref.key 处理上下/Enter/Tab，普通字符/isComposing/null 返回 false', () => {
    const ref = createRef<PickerHandle>();
    const onPick = vi.fn();
    const onActive = vi.fn();
    const groups: PickerGroup[] = [
      {
        title: '节点',
        items: [
          { id: 'n1', label: 'Agent' },
          { id: 'n2', label: '工具', disabled: true },
          { id: 'n3', label: '分支' },
        ],
      },
    ];

    const r = render(
      <Picker
        ref={ref}
        groups={groups}
        activeId="n1"
        onPick={onPick}
        onActiveChange={onActive}
        emptyText="无结果"
        ariaLabel="快速添加"
      />,
    );

    expect(ref.current?.key(new KeyboardEvent('keydown', { key: 'ArrowDown' }))).toBe(true);
    expect(onActive).toHaveBeenCalledWith('n3');

    expect(ref.current?.key(new KeyboardEvent('keydown', { key: 'Enter' }))).toBe(true);
    expect(onPick).toHaveBeenCalledWith('n1', 'enter');

    expect(ref.current?.key(new KeyboardEvent('keydown', { key: 'Tab' }))).toBe(true);
    expect(onPick).toHaveBeenCalledWith('n1', 'tab');

    expect(ref.current?.key(new KeyboardEvent('keydown', { key: 'a' }))).toBe(false);
    expect(ref.current?.key(new KeyboardEvent('keydown', { key: 'Enter', isComposing: true }))).toBe(false);

    r.rerender(
      <Picker
        ref={ref}
        groups={groups}
        activeId={null}
        onPick={onPick}
        onActiveChange={onActive}
        emptyText="无结果"
        ariaLabel="快速添加"
      />,
    );
    expect(ref.current?.key(new KeyboardEvent('keydown', { key: 'Enter' }))).toBe(false);
  });

  test('W0803-T10 Picker 指针：坐标不变不改高亮，坐标变化改高亮，pointerdown 被 preventDefault', () => {
    const onActive = vi.fn();
    const onPick = vi.fn();
    const r = render(
      <Picker
        groups={[
          {
            items: [
              { id: 'a', label: 'A' },
              { id: 'b', label: 'B' },
            ],
          },
        ]}
        activeId="a"
        onPick={onPick}
        onActiveChange={onActive}
        emptyText="无"
        ariaLabel="列表"
      />,
    );
    const opts = r.queryAllByRole('option');

    // 先在 a 上移动建立上次坐标
    act(() => {
      opts[0]!.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, clientX: 50, clientY: 60 }));
    });
    // 滚动后同一坐标触发 b 的 pointermove → 不应改高亮
    act(() => {
      opts[1]!.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, clientX: 50, clientY: 60 }));
    });
    expect(onActive).not.toHaveBeenCalled();

    // 坐标变化 → 改高亮
    act(() => {
      opts[1]!.dispatchEvent(new PointerEvent('pointermove', { bubbles: true, clientX: 55, clientY: 65 }));
    });
    expect(onActive).toHaveBeenCalledWith('b');

    // pointerdown 被 preventDefault
    const down = new PointerEvent('pointerdown', { bubbles: true, cancelable: true });
    act(() => {
      opts[1]!.dispatchEvent(down);
    });
    expect(down.defaultPrevented).toBe(true);
  });

  test('W0803-T11 Picker 可访问性：role="listbox"、pickerOptionId、aria-selected 与空态', () => {
    const r = render(
      <Picker
        listId="pk1"
        groups={[{ items: [{ id: 'x', label: 'X' }] }]}
        activeId="x"
        onPick={vi.fn()}
        onActiveChange={vi.fn()}
        emptyText="没有匹配项"
        ariaLabel="命令列表"
      />,
    );
    const lb = r.getByRole('listbox', { name: '命令列表' });
    expect(lb.id).toBe('pk1');
    const opt = r.getByRole('option');
    expect(opt.id).toBe(pickerOptionId('pk1', 'x'));
    expect(opt.getAttribute('aria-selected')).toBe('true');

    r.rerender(
      <Picker
        listId="pk1"
        groups={[]}
        activeId={null}
        onPick={vi.fn()}
        onActiveChange={vi.fn()}
        emptyText="没有匹配项"
        ariaLabel="命令列表"
      />,
    );
    expect(r.getByText('没有匹配项')).toBeTruthy();
    expect(r.queryAllByRole('option')).toHaveLength(0);
  });

  test('W0803-T12 窗口化：600 项只渲染可见窗口，199 项全部渲染', () => {
    const makeItems = (n: number) =>
      Array.from({ length: n }, (_, i) => ({ id: `i${String(i)}`, label: `项 ${String(i)}` }));

    const r600 = render(
      <Picker
        groups={[{ items: makeItems(600) }]}
        activeId="i0"
        onPick={vi.fn()}
        onActiveChange={vi.fn()}
        emptyText="空"
        ariaLabel="长列表"
      />,
    );
    expect(r600.queryAllByRole('option').length).toBeLessThanOrEqual(30);
    // K307：对真正滚动容器 ScrollArea 触发 scroll(scrollTop=5000)，渲染窗口切到对应位置且不被拉回当前项 i0
    const scrollEl = r600.container.querySelector('.hx-picker__scroll') as HTMLDivElement;
    act(() => {
      scrollEl.scrollTop = 5000;
      scrollEl.dispatchEvent(new Event('scroll'));
    });
    expect(scrollEl.scrollTop).toBe(5000);
    // 单行高 32px，5000 / 32 ≈ 156
    expect(r600.container.textContent).toContain('项 156');
    expect(r600.container.textContent).not.toContain('项 0');
    r600.unmount();

    // K307：含空格、空串、字面量 %20、未配对代理项与带空格 listId 均生成安全且无碰撞 DOM id
    const edgeIds = ['', 'node with space', 'node%20with%20space', '\uD800'];
    const rEdge = render(
      <Picker
        listId="pk with space"
        groups={[{ items: edgeIds.map((id, idx) => ({ id, label: `边界 ${String(idx)}` })) }]}
        activeId="node with space"
        onPick={vi.fn()}
        onActiveChange={vi.fn()}
        emptyText="空"
        ariaLabel="边界测试"
      />,
    );
    const domIds = rEdge.queryAllByRole('option').map((el) => el.id);
    expect(domIds).toEqual(edgeIds.map((id) => pickerOptionId('pk with space', id)));
    expect(new Set(domIds).size).toBe(edgeIds.length);
    for (const id of domIds) {
      expect(/\s/.test(id)).toBe(false);
      expect(document.getElementById(id)).not.toBeNull();
    }
    // 跨实例分隔符边界无碰撞
    expect(pickerOptionId('a', '-opt-b')).not.toBe(pickerOptionId('a-opt-', 'b'));
    rEdge.unmount();

    const r199 = render(
      <Picker
        groups={[{ items: makeItems(199) }]}
        activeId="i0"
        onPick={vi.fn()}
        onActiveChange={vi.fn()}
        emptyText="空"
        ariaLabel="短列表"
      />,
    );
    expect(r199.queryAllByRole('option')).toHaveLength(199);
  });

  test('W0803-T13 加粗：query 匹配段用 <b> 文本节点渲染', () => {
    const r = render(
      <Picker
        groups={[{ items: [{ id: 'ag', label: 'Agent' }] }]}
        activeId="ag"
        query="age"
        onPick={vi.fn()}
        onActiveChange={vi.fn()}
        emptyText="空"
        ariaLabel="匹配"
      />,
    );
    const b = r.container.querySelector('.hx-picker__label b');
    expect(b?.textContent).toBe('Age');
    expect(r.container.querySelector('.hx-picker__label')?.textContent).toBe('Agent');
  });

  test('Picker 分组 role="group" 可访问性与 250 项窗口化滚入深位置 activeId', () => {
    const r = render(
      <Picker
        groups={[{ title: '模型分组', items: [{ id: 'a', label: 'A' }] }]}
        activeId="a"
        onPick={vi.fn()}
        onActiveChange={vi.fn()}
        emptyText="空"
        ariaLabel="分组列表"
      />,
    );
    expect(r.getByRole('group', { name: '模型分组' })).toBeTruthy();
    r.unmount();

    const many = Array.from({ length: 250 }, (_, i) => ({ id: `p${String(i)}`, label: `条目 ${String(i)}` }));
    const r2 = render(
      <Picker
        groups={[{ items: many }]}
        activeId="p245"
        onPick={vi.fn()}
        onActiveChange={vi.fn()}
        emptyText="空"
        ariaLabel="长选择器"
      />,
    );
    expect(r2.container.textContent).toContain('条目 245');
  });
});
