import type { KeyboardEvent } from 'react';
import { describe, expect, test, vi } from 'vitest';
import { act, render, user } from '@tests/kit';
import { TextArea } from './TextArea';

describe('W.08.02 TextArea', () => {
  test('W0802-T14 TextArea：onKeyDown preventDefault 拦截、Mod+Enter 提交、fitRows 夹在 3-12 行', async () => {
    const onCommit = vi.fn();
    const r = render(
      <TextArea
        value="line1"
        ariaLabel="说明"
        onChange={vi.fn()}
        onCommit={onCommit}
        onKeyDown={(e) => {
          if (e.key === 'Enter' && !e.ctrlKey && !e.metaKey) {
            e.preventDefault();
          }
        }}
        counter={{ max: 4 }}
      />,
    );
    const el = r.getByRole('textbox', { name: '说明' }) as HTMLTextAreaElement;
    // 普通 Enter 被 onKeyDown preventDefault
    await user.key('Enter', el);
    expect(onCommit).not.toHaveBeenCalled();

    // Mod+Enter 触发 onCommit
    await user.key('Mod+Enter', el);
    expect(onCommit).toHaveBeenCalledWith('line1');

    // counter 超出变红不截断
    const counter = r.container.querySelector('.hx-textarea__counter');
    expect(counter?.textContent).toBe('5 / 4');
    expect(counter?.classList.contains('is-over')).toBe(true);

    // fitRows mock scrollHeight / lineHeight 20，并验证多次输入只读一次 computed style 且 Mod+Enter 不冒泡
    const origGetCS = window.getComputedStyle.bind(window);
    const csSpy = vi.spyOn(window, 'getComputedStyle').mockImplementation((target, pseudo) => {
      if (target instanceof HTMLTextAreaElement) {
        return {
          lineHeight: '20px',
          paddingTop: '0px',
          paddingBottom: '0px',
          fontSize: '13px',
        } as CSSStyleDeclaration;
      }
      return origGetCS(target, pseudo);
    });
    const onOuterKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
      if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
        throw new Error('Mod+Enter should not bubble');
      }
    };
    const r2 = render(
      <div onKeyDown={onOuterKeyDown}>
        <TextArea
          value="a"
          ariaLabel="行高测试"
          onChange={vi.fn()}
          onCommit={vi.fn()}
          minRows={3}
          maxRows={12}
        />
      </div>,
    );
    const el2 = r2.getByRole('textbox', { name: '行高测试' }) as HTMLTextAreaElement;
    const callsAfterMount = csSpy.mock.calls.length;
    expect(callsAfterMount).toBe(1);
    Object.defineProperty(el2, 'scrollHeight', { configurable: true, value: 400 });
    act(() => {
      r2.rerender(
        <div onKeyDown={onOuterKeyDown}>
          <TextArea
            value={'a\n'.repeat(20)}
            ariaLabel="行高测试"
            onChange={vi.fn()}
            onCommit={vi.fn()}
            minRows={3}
            maxRows={12}
          />
        </div>,
      );
      r2.rerender(
        <div onKeyDown={onOuterKeyDown}>
          <TextArea
            value={'a\n'.repeat(25)}
            ariaLabel="行高测试"
            onChange={vi.fn()}
            onCommit={vi.fn()}
            minRows={3}
            maxRows={12}
          />
        </div>,
      );
    });
    expect(csSpy.mock.calls.length).toBe(callsAfterMount);
    expect(Number.parseInt(el2.style.height, 10)).toBe(240);
    await user.key('Mod+Enter', el2);
    csSpy.mockRestore();
  });
});
