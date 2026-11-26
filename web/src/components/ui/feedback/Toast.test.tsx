import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, expectTypeOf, test, vi } from 'vitest';
import { act, expectConsole, render, user } from '@tests/kit';
import type { BadgeProps } from './Badge';
import type { EmptyStateProps } from './EmptyState';
import type { ErrorBoundaryProps } from './ErrorBoundary';
import type { ProgressBarProps } from './ProgressBar';
import type { ScrollAreaProps } from './ScrollArea';
import type { SpinnerProps } from './Spinner';
import type { StatusDotProps } from './StatusDot';
import { Toast, type ToastLevel, type ToastProps } from './Toast';

const here = dirname(fileURLToPath(import.meta.url));

describe('W.08.05 Toast 与 props 对照', () => {
  test('W0805-T1 props 对照', () => {
    expectTypeOf<ToastProps['actions']>().toEqualTypeOf<
      readonly { readonly label: string; readonly onClick: () => void }[] | undefined
    >();
    expectTypeOf<BadgeProps['tone']>().toEqualTypeOf<
      'neutral' | 'accent' | 'ok' | 'danger' | 'wait' | 'model'
    >();
    expectTypeOf<StatusDotProps['state']>().toEqualTypeOf<
      'idle' | 'running' | 'waiting' | 'failed' | 'done'
    >();
    expectTypeOf<SpinnerProps['size']>().toEqualTypeOf<12 | 16>();
    expectTypeOf<ProgressBarProps['value']>().toEqualTypeOf<number>();
    expectTypeOf<EmptyStateProps['title']>().toEqualTypeOf<string>();
    expectTypeOf<ScrollAreaProps['gutter']>().toEqualTypeOf<'stable' | undefined>();
    expectTypeOf<ErrorBoundaryProps['name']>().toEqualTypeOf<string>();
    expect(true).toBe(true);
  });

  test('W0805-T2 Toast 级别：五种级别 role、图标/AI 徽标与 detail 第二行', () => {
    const levels: Array<[ToastLevel, string]> = [
      ['info', 'status'],
      ['ok', 'status'],
      ['wait', 'status'],
      ['error', 'alert'],
      ['model', 'status'],
    ];
    for (const [lv, wantRole] of levels) {
      const r = render(<Toast level={lv} text={`消息-${lv}`} detail="补充详情" onClose={vi.fn()} />);
      const card = r.container.querySelector('.hx-toastcard');
      expect(card?.getAttribute('role')).toBe(wantRole);
      expect(r.container.querySelector('.hx-toastcard__detail')?.textContent).toBe('补充详情');
      if (lv === 'model') {
        expect(r.container.querySelector('.hx-badge--model')?.textContent).toBe('AI');
      } else {
        expect(r.container.querySelector('.hx-toastcard__lead svg')).not.toBeNull();
      }
      r.unmount();
    }
  });

  test('W0805-T3 Toast 交互：动作回调、关闭按钮 aria-label、最多 3 个动作', async () => {
    const onClose = vi.fn();
    const clicks = [vi.fn(), vi.fn(), vi.fn(), vi.fn()];
    expectConsole('warn', /at most 3 actions/, 1);
    const r = render(
      <Toast
        level="info"
        text="操作完成"
        onClose={onClose}
        actions={[
          { label: '应用', onClick: clicks[0]! },
          { label: '不应用', onClick: clicks[1]! },
          { label: '查看', onClick: clicks[2]! },
          { label: '多余', onClick: clicks[3]! },
        ]}
      />,
    );
    const actionBtns = r.container.querySelectorAll('.hx-toastcard__actions button');
    expect(actionBtns).toHaveLength(3);
    await user.click(actionBtns[1]!);
    expect(clicks[1]).toHaveBeenCalledTimes(1);

    const closeBtn = r.getByRole('button', { name: '关闭' });
    await user.click(closeBtn);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  test('W0805-T4 Toast 悬停：指针离开但焦点在内不触发 false，焦点离开才触发一次', () => {
    const onHover = vi.fn();
    const outside = document.createElement('button');
    document.body.appendChild(outside);
    try {
      const r = render(
        <Toast
          level="info"
          text="悬停暂停"
          onClose={vi.fn()}
          onHover={onHover}
          actions={[{ label: '撤销', onClick: vi.fn() }]}
        />,
      );
      const card = r.container.querySelector('.hx-toastcard') as HTMLElement;
      const undoBtn = r.getByRole('button', { name: '撤销' });

      act(() => {
        card.dispatchEvent(new PointerEvent('pointerover', { bubbles: true, relatedTarget: document.body }));
      });
      expect(onHover).toHaveBeenLastCalledWith(true);

      act(() => {
        undoBtn.focus();
      });
      act(() => {
        card.dispatchEvent(new PointerEvent('pointerout', { bubbles: true, relatedTarget: document.body }));
      });
      // 焦点还在内部，不能触发 onHover(false)
      expect(onHover).toHaveBeenCalledTimes(1);

      act(() => {
        outside.focus();
      });
      expect(onHover).toHaveBeenCalledTimes(2);
      expect(onHover).toHaveBeenLastCalledWith(false);
    } finally {
      outside.remove();
    }
  });

  test('W0805-T12 减少动效：CSS 在 html[data-motion="reduce"] 下替代旋转/呼吸/平移动画', () => {
    const spinCss = readFileSync(resolve(here, 'Spinner.css'), 'utf8');
    const dotCss = readFileSync(resolve(here, 'StatusDot.css'), 'utf8');
    const toastCss = readFileSync(resolve(here, 'Toast.css'), 'utf8');
    expect(spinCss).toContain("html[data-motion='reduce'] .hx-spin__svg");
    expect(spinCss).toContain('animation: none');
    expect(dotCss).toContain("html[data-motion='reduce'] .hx-dot--running.is-breathing::after");
    expect(dotCss).toContain('animation: none');
    expect(toastCss).toContain("html[data-motion='reduce'] .hx-toastcard");
    expect(toastCss).toContain('hx-toast-fade');
  });
});
