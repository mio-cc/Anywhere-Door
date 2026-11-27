import { describe, expect, test, vi } from 'vitest';
import { fakeEnv, render } from '@tests/kit';
import { Dialog } from './Dialog';
import { UiEnvProvider } from './UiEnv';

describe('W.08.04 Dialog', () => {
  test('W0804-T12 Dialog：危险按钮最右、alertdialog、初始焦点在取消、error 与 busy、dismissible=false', () => {
    const fe = fakeEnv({ reducedMotion: true });
    const onClose = vi.fn();
    const onDelete = vi.fn();
    const onCancel = vi.fn();

    try {
      const r = render(
        <UiEnvProvider value={fe.env}>
          <Dialog
            title="删除会话"
            width={400}
            open
            onClose={onClose}
            error="删除失败"
            actions={[
              { label: '删除', variant: 'danger', onClick: onDelete },
              { label: '取消', variant: 'secondary', onClick: onCancel },
            ]}
          >
            <p>确认永久删除？</p>
          </Dialog>
        </UiEnvProvider>,
      );

      const dlg = fe.root.querySelector('[role="alertdialog"]');
      expect(dlg).not.toBeNull();
      const buttons = Array.from(fe.root.querySelectorAll('button')).map((b) => b.textContent);
      expect(buttons).toEqual(['取消', '删除']);
      expect(document.activeElement?.textContent).toBe('取消');
      const ferr = fe.root.querySelector('.hx-ferr');
      const bodyEl = fe.root.querySelector('.hx-dialog__body');
      expect(ferr?.textContent).toBe('删除失败');
      expect(dlg?.getAttribute('aria-describedby')).toBe(`${bodyEl?.id ?? ''} ${ferr?.id ?? ''}`);

      // busy 时全部按钮 disabled；dismissible=false 时 Esc 被吞掉但不调 onClose
      r.rerender(
        <UiEnvProvider value={fe.env}>
          <Dialog
            title="冲突"
            width={560}
            open
            dismissible={false}
            busy
            onClose={onClose}
            actions={[
              { label: '覆盖', variant: 'danger', onClick: onDelete },
              { label: '保留', variant: 'secondary', onClick: onCancel },
            ]}
          >
            <p>发现并发修改</p>
          </Dialog>
        </UiEnvProvider>,
      );
      for (const btn of Array.from(fe.root.querySelectorAll('button'))) {
        expect(btn.disabled).toBe(true);
      }
      expect(fe.dispatchKey({ key: 'Escape' })).toBe(true);
      expect(onClose).not.toHaveBeenCalled();
    } finally {
      fe.cleanup();
    }
  });

  test('Dialog 全 danger 动作且正文无焦点元素时焦点落在面板本身，不聚焦危险按钮', () => {
    const fe = fakeEnv({ reducedMotion: true });
    try {
      render(
        <UiEnvProvider value={fe.env}>
          <Dialog
            title="危险操作"
            width={400}
            open
            onClose={vi.fn()}
            actions={[{ label: '彻底清空', variant: 'danger', onClick: vi.fn() }]}
          >
            <p>不可撤销</p>
          </Dialog>
        </UiEnvProvider>,
      );
      const dangerBtn = fe.root.querySelector('button');
      const dlgWrap = fe.root.querySelector('.hx-dialog__wrap');
      expect(document.activeElement).toBe(dlgWrap);
      expect(document.activeElement).not.toBe(dangerBtn);
    } finally {
      fe.cleanup();
    }
  });
});
