import { describe, expect, test, vi } from 'vitest';
import { act, memStorage, render } from '@tests/kit';
import { App } from '@/App';
import { createTestRouter } from '@/routes';
import { createUiStore } from '@/state/uiStore';
import { AppShell } from './AppShell';
import { createKeyRegistry } from './keys';

function makeKey(code: string, key: string): KeyboardEvent {
  return new KeyboardEvent('keydown', { code, key, bubbles: true, cancelable: true });
}

describe('W.02.02 AppShell 外壳布局', () => {
  test('W0202-T3 专注模式：F 两次恢复原状，专注中按 [ 退出专注并只打开左栏', () => {
    const storage = memStorage();
    const ui = createUiStore({ width: 1440, storage });
    ui.actions.setRs(true, 'chat');
    const keys = createKeyRegistry({ isMac: () => false });

    const r = render(
      <AppShell
        mode="session"
        uiStore={ui}
        keyRegistry={keys}
        topBar={<div>top</div>}
        leftSidebar={<div>left</div>}
        rightSidebar={<div>right</div>}
        canvas={<div>canvas</div>}
      />,
    );

    expect(r.container.querySelector('.hx-app__ls')?.getAttribute('data-open')).toBe('true');
    expect(r.container.querySelector('.hx-app__rs')?.getAttribute('data-open')).toBe('true');

    // F 一次进入专注：两侧收起
    act(() => {
      keys.dispatchEvent(makeKey('KeyF', 'f'), 'down');
    });
    expect(r.container.querySelector('.hx-app__ls')?.getAttribute('data-open')).toBe('false');
    expect(r.container.querySelector('.hx-app__rs')?.getAttribute('data-open')).toBe('false');

    // F 第二次恢复原状
    act(() => {
      keys.dispatchEvent(makeKey('KeyF', 'f'), 'down');
    });
    expect(r.container.querySelector('.hx-app__ls')?.getAttribute('data-open')).toBe('true');
    expect(r.container.querySelector('.hx-app__rs')?.getAttribute('data-open')).toBe('true');

    // 专注中按 [：退出专注并只打开左栏
    act(() => {
      keys.dispatchEvent(makeKey('KeyF', 'f'), 'down');
    });
    act(() => {
      keys.dispatchEvent(makeKey('BracketLeft', '['), 'down');
    });
    expect(ui.getState().layout.focus).toBeNull();
    expect(ui.getState().layout.lsOpen).toBe(true);
    expect(ui.getState().layout.rsOpen).toBe(false);
    expect(r.container.querySelector('.hx-app__ls')?.getAttribute('data-open')).toBe('true');
    expect(r.container.querySelector('.hx-app__rs')?.getAttribute('data-open')).toBe('false');
  });

  test('W0202-T4 覆盖式右栏：宽 1000 时打开右栏主体列宽不变，画布按下关闭右栏', () => {
    const ui = createUiStore({ width: 1000, storage: memStorage() });
    ui.actions.setRs(true, 'insp');

    const r = render(
      <AppShell
        mode="session"
        uiStore={ui}
        topBar={<div>top</div>}
        leftSidebar={<div>left</div>}
        rightSidebar={<div>right</div>}
        canvas={<div>canvas</div>}
        floatProps={{
          composer: <button type="button">composer-btn</button>,
        }}
      />,
    );

    const app = r.container.querySelector('.hx-app');
    const main = r.container.querySelector('.hx-app__main');
    const rs = r.container.querySelector('.hx-app__rs');
    expect(app?.getAttribute('data-bp')).toBe('md');
    // 覆盖式右栏不加 is-rs-docked（主体列宽不变），而是 is-overlay
    expect(main?.classList.contains('is-rs-docked')).toBe(false);
    expect(rs?.classList.contains('is-overlay')).toBe(true);

    // K309 D10：点击浮动层内的对话框不误关右栏
    const compBtn = r.getByRole('button', { name: 'composer-btn' });
    act(() => {
      compBtn.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
    });
    expect(ui.getState().layout.rsOpen).toBe(true);

    // 在画布空白处按下鼠标 → 关闭右栏
    const canvasArea = r.container.querySelector('.hx-canvas__stage') as HTMLElement;
    act(() => {
      canvasArea.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
    });
    expect(ui.getState().layout.rsOpen).toBe(false);
    expect(r.container.querySelector('.hx-app__rs')?.classList.contains('is-hidden')).toBe(true);
  });

  test('W0202-T5 小地图避让：宽 1180 画布窄到与对话框相交时小地图隐藏', () => {
    const ui = createUiStore({ width: 1180, storage: memStorage() });
    const spyRect = vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (
      this: HTMLElement,
    ): DOMRect {
      if (this.classList.contains('hx-canvas__minimap')) {
        return {
          x: 12,
          y: 600,
          width: 160,
          height: 120,
          left: 12,
          right: 172,
          top: 600,
          bottom: 720,
          toJSON: () => ({}),
        };
      }
      if (this.classList.contains('hx-canvas__composer')) {
        return {
          x: 150,
          y: 650,
          width: 500,
          height: 70,
          left: 150,
          right: 650,
          top: 650,
          bottom: 720,
          toJSON: () => ({}),
        };
      }
      return {
        x: 0,
        y: 0,
        width: 0,
        height: 0,
        left: 0,
        right: 0,
        top: 0,
        bottom: 0,
        toJSON: () => ({}),
      };
    });

    try {
      const r = render(
        <AppShell
          mode="session"
          uiStore={ui}
          canvas={<div>canvas</div>}
          floatProps={{
            minimap: <div>mini</div>,
            composer: <div>comp</div>,
          }}
        />,
      );
      const mini = r.container.querySelector('.hx-canvas__minimap');
      expect(mini?.classList.contains('is-hidden')).toBe(true);
    } finally {
      spyRect.mockRestore();
    }
  });

  test('W0202-T6 持久化：刷新后左栏开合、分区折叠、右栏开合与标签页保持，/welcome 时侧栏与顶栏 inert', () => {
    const storage = memStorage();
    const ui1 = createUiStore({ width: 1440, storage });
    ui1.actions.setLs(false);
    ui1.actions.setLsFold('sess', true);
    ui1.actions.setRs(true, 'mon');

    // 模拟刷新：基于同一 storage 新建 uiStore
    const ui2 = createUiStore({ width: 1440, storage });
    const r = render(
      <AppShell
        mode="session"
        uiStore={ui2}
        topBar={<div>top</div>}
        leftSidebar={<div>left</div>}
        rightSidebar={<div>right</div>}
        canvas={<div>canvas</div>}
      />,
    );
    expect(ui2.getState().layout.lsOpen).toBe(false);
    expect(ui2.getState().layout.lsFold.sess).toBe(true);
    expect(ui2.getState().layout.rsOpen).toBe(true);
    expect(ui2.getState().layout.rsTab).toBe('mon');
    expect(r.container.querySelector('.hx-app__rs')?.getAttribute('data-tab')).toBe('mon');

    // /welcome 页面模式：左栏与顶栏 inert，右栏收起
    r.rerender(
      <AppShell
        mode="page"
        welcomePage
        uiStore={ui2}
        topBar={<div>top</div>}
        leftSidebar={<div>left</div>}
        rightSidebar={<div>right</div>}
        page={<div>welcome</div>}
      />,
    );
    expect(r.container.querySelector('.hx-app__ls')?.hasAttribute('inert')).toBe(true);
    expect(r.container.querySelector('.hx-app__top')?.hasAttribute('inert')).toBe(true);
    expect(r.container.querySelector('.hx-app__rs')?.classList.contains('is-hidden')).toBe(true);

    // K309 D13：经 App + WelcomeRoute 挂载 /welcome 时顶部条与左栏均自动 inert
    const welcomeRouter = createTestRouter(['/welcome']);
    const rApp = render(
      <App
        uiStore={createUiStore({ width: 1440, storage: memStorage() })}
        router={welcomeRouter}
        services={{ onboardingDone: false, sessions: [] }}
      />,
    );
    expect(rApp.container.querySelector('.hx-app__top')?.hasAttribute('inert')).toBe(true);
    expect(rApp.container.querySelector('.hx-app__ls')?.hasAttribute('inert')).toBe(true);
    rApp.unmount();
  });
});
