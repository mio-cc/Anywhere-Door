import { describe, expect, test, vi } from 'vitest';
import { act, render, user } from '@tests/kit';
import { SearchInput } from './SearchInput';

describe('W.08.02 SearchInput', () => {
  test('W0802-T13 SearchInput：有值 Esc 清空并拦截，无值 blur 冒泡，onEscape=none 不处理，ariaControls 切换 combobox', async () => {
    const onClear = vi.fn();
    const parentEsc = vi.fn();

    const r = render(
      <div onKeyDown={parentEsc}>
        <SearchInput value="agent" ariaLabel="搜索节点" onChange={vi.fn()} onClear={onClear} />
      </div>,
    );
    const box = r.getByRole('searchbox', { name: '搜索节点' }) as HTMLInputElement;
    act(() => {
      box.focus();
    });
    await user.key('Escape', box);
    expect(onClear).toHaveBeenCalledTimes(1);
    expect(parentEsc).not.toHaveBeenCalled();

    // 无值 Esc → blur 且不拦截
    r.rerender(
      <div onKeyDown={parentEsc}>
        <SearchInput value="" ariaLabel="搜索节点" onChange={vi.fn()} onClear={onClear} />
      </div>,
    );
    const box2 = r.getByRole('searchbox', { name: '搜索节点' }) as HTMLInputElement;
    act(() => {
      box2.focus();
    });
    await user.key('Escape', box2);
    expect(document.activeElement).not.toBe(box2);
    expect(parentEsc).toHaveBeenCalledTimes(1);

    // onEscape='none' + ariaControls → role="combobox" 且 Esc 交给外层
    parentEsc.mockClear();
    onClear.mockClear();
    r.rerender(
      <div onKeyDown={parentEsc}>
        <SearchInput
          value="abc"
          ariaLabel="筛选列表"
          onChange={vi.fn()}
          onClear={onClear}
          onEscape="none"
          ariaControls="list-1"
          ariaActiveDescendant="list-1-a"
        />
      </div>,
    );
    const combo = r.getByRole('combobox', { name: '筛选列表' });
    expect(combo.getAttribute('aria-controls')).toBe('list-1');
    expect(combo.getAttribute('aria-activedescendant')).toBe('list-1-a');
    await user.key('Escape', combo);
    expect(onClear).not.toHaveBeenCalled();
    expect(parentEsc).toHaveBeenCalledTimes(1);
  });
});
