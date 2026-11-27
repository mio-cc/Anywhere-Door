import { RouterProvider } from 'react-router';
import { describe, expect, test, vi } from 'vitest';
import { act, memStorage, render } from '@tests/kit';
import type { Ulid } from '@/lib/protocol';
import { createTestRouter } from './index';
import { createNavHistoryTracker, toUrl, useNav, type NavApi } from './nav';
import { RouteServicesProvider, type RouteServices } from './SessionRoute';

const S1 = '01JD8X3Q5V7W9Y2Z4A6B8C0D1E' as Ulid;
const S2 = '01JD8X3Q5V7W9Y2Z4A6B8C0D2F' as Ulid;
const G1 = '01JD8X3Q5V7W9Y2Z4A6B8C0D3G' as Ulid;
const G2 = '01JD8X3Q5V7W9Y2Z4A6B8C0D4H' as Ulid;
const L1 = '01JD8X3Q5V7W9Y2Z4A6B8C0D5J' as Ulid;
const L2 = '01JD8X3Q5V7W9Y2Z4A6B8C0D6K' as Ulid;
const GHOST_SCOPE = '01JD8X3Q5V7W9Y2Z4A6B8C0D9Z' as Ulid;

function mountRouter(initialEntries: string[], services: RouteServices) {
  const router = createTestRouter(initialEntries);
  const rendered = render(
    <RouteServicesProvider value={services}>
      <RouterProvider router={router} />
    </RouteServicesProvider>,
  );
  return { router, rendered };
}

describe('W.02.01 SessionRoute 与路由解析', () => {
  test('W0201-T2 / 解析：引导未完成→/welcome，有会话→最近会话，无会话→留在 /', () => {
    // 1) 引导未完成 → /welcome
    const r1 = mountRouter(['/'], { onboardingDone: false, sessions: [] });
    expect(r1.router.state.location.pathname).toBe('/welcome');
    r1.rendered.unmount();

    // 2) 有会话且 hx:last-session 指向 S2 → 跳到 /c/S2/g/G2
    const storage = memStorage();
    storage.setItem('hx:last-session', S2);
    const r2 = mountRouter(['/'], {
      onboardingDone: true,
      storage,
      sessions: [
        { id: S1, title: '会话一', activeGraphId: G1 },
        { id: S2, title: '会话二', activeGraphId: G2 },
      ],
    });
    expect(r2.router.state.location.pathname).toBe(`/c/${S2}/g/${G2}`);
    r2.rendered.unmount();

    // 3) 无会话 → 留在 /
    const r3 = mountRouter(['/'], {
      onboardingDone: true,
      storage: memStorage(),
      sessions: [],
      renderHome: () => <div>首次空白首页</div>,
    });
    expect(r3.router.state.location.pathname).toBe('/');
    expect(r3.rendered.getByText('首次空白首页')).toBeTruthy();
    r3.rendered.unmount();
  });

  test('W0201-T3 返回不重复入栈：进入第 3 层后 leaveTo 两次回根图不新增历史，浏览器前进/后退不来回跳', async () => {
    const tracker = createNavHistoryTracker();
    let capturedNav: NavApi | null = null;
    function Capture() {
      capturedNav = useNav(tracker);
      return null;
    }

    const { router, rendered } = mountRouter([toUrl({ sessionId: S1, graphId: G1, scope: 'root' })], {
      onboardingDone: true,
      storage: memStorage(),
      historyTracker: tracker,
      sessions: [{ id: S1, title: '主会话', activeGraphId: G1 }],
      scopeExists: () => true,
      renderCanvasOverlay: () => <Capture />,
    });

    try {
      expect(capturedNav).not.toBeNull();
      act(() => {
        capturedNav!.enterLayer(L1);
      });
      act(() => {
        capturedNav!.enterLayer(L2);
      });
      expect(router.state.location.search).toBe(`?scope=${L2}`);
      expect(tracker.snapshot().entries).toHaveLength(3);

      // Esc / leaveTo 返回 L1 → 命中上一条，走 navigate(-1)，不追加栈
      act(() => {
        capturedNav!.leaveTo(L1);
      });
      expect(router.state.location.search).toBe(`?scope=${L1}`);
      expect(tracker.snapshot().entries).toHaveLength(3);

      // 再返回 root → 同样命中上一条，走 navigate(-1)
      act(() => {
        capturedNav!.leaveTo('root');
      });
      expect(router.state.location.search).toBe('');
      expect(tracker.snapshot().entries).toHaveLength(3);
      expect(tracker.snapshot().index).toBe(0);

      // 前进回到 L1，再前进回到 L2，不存在重复的 root/L1 历史项
      await act(async () => {
        await router.navigate(1);
      });
      expect(router.state.location.search).toBe(`?scope=${L1}`);
    } finally {
      rendered.unmount();
    }
  });

  test('W0201-T4 跨会话后退：会话 A 第 3 层切到 B 再后退回到 A 第 3 层', async () => {
    const tracker = createNavHistoryTracker();
    let capturedNav: NavApi | null = null;
    function Capture() {
      capturedNav = useNav(tracker);
      return null;
    }

    const { router, rendered } = mountRouter([toUrl({ sessionId: S1, graphId: G1, scope: L2 })], {
      onboardingDone: true,
      storage: memStorage(),
      historyTracker: tracker,
      sessions: [
        { id: S1, title: '会话 A', activeGraphId: G1 },
        { id: S2, title: '会话 B', activeGraphId: G2 },
      ],
      scopeExists: () => true,
      renderCanvasOverlay: () => <Capture />,
    });

    try {
      expect(router.state.location.pathname).toBe(`/c/${S1}/g/${G1}`);
      expect(router.state.location.search).toBe(`?scope=${L2}`);

      act(() => {
        capturedNav!.switchSession(S2);
      });
      expect(router.state.location.pathname).toBe(`/c/${S2}/g/${G2}`);
      expect(router.state.location.search).toBe('');

      await act(async () => {
        await router.navigate(-1);
      });
      expect(router.state.location.pathname).toBe(`/c/${S1}/g/${G1}`);
      expect(router.state.location.search).toBe(`?scope=${L2}`);
    } finally {
      rendered.unmount();
    }
  });

  test('W0201-T5 scope 不存在：替换为根图并提示“那一层已经不存在了”', () => {
    const notify = vi.fn();
    const { router, rendered } = mountRouter([toUrl({ sessionId: S1, graphId: G1, scope: GHOST_SCOPE })], {
      onboardingDone: true,
      storage: memStorage(),
      sessions: [{ id: S1, title: '会话 A', activeGraphId: G1 }],
      scopeExists: (_gid, sc) => sc !== GHOST_SCOPE,
      notify,
    });

    try {
      expect(router.state.location.pathname).toBe(`/c/${S1}/g/${G1}`);
      expect(router.state.location.search).toBe('');
      expect(notify).toHaveBeenCalledWith('那一层已经不存在了');
    } finally {
      rendered.unmount();
    }
  });

  test('SessionRoute 严格归属校验与重渲染不重复激活/保存（K309 D06/D08）', () => {
    const onActivate = vi.fn();
    const saveLastLocation = vi.fn();
    const sessions = [{ id: S1, title: '主会话', activeGraphId: G1 }];
    const storage = memStorage();
    const router = createTestRouter([toUrl({ sessionId: S1, graphId: G1, scope: 'root' })]);

    const r = render(
      <RouteServicesProvider
        value={{ onboardingDone: true, storage, sessions, pendingApprovals: 0, onActivate, saveLastLocation }}
      >
        <RouterProvider router={router} />
      </RouteServicesProvider>,
    );
    expect(onActivate).toHaveBeenCalledTimes(1);
    expect(saveLastLocation).toHaveBeenCalledTimes(1);

    // 仅 pendingApprovals 变化重渲染：只更新标题，不重复激活/保存
    r.rerender(
      <RouteServicesProvider
        value={{
          onboardingDone: true,
          storage,
          sessions,
          pendingApprovals: 2,
          pendingTitlePrefix: '（待审批）',
          onActivate,
          saveLastLocation,
        }}
      >
        <RouterProvider router={router} />
      </RouteServicesProvider>,
    );
    expect(onActivate).toHaveBeenCalledTimes(1);
    expect(saveLastLocation).toHaveBeenCalledTimes(1);
    expect(document.title).toBe('（待审批）主会话 · Anywhere Door');
    r.unmount();

    // 会话无 activeGraphId/graphIds 时访问 /g/:gid → 重定向回 /c/:sid，不静默放行
    const rNoGraph = mountRouter([toUrl({ sessionId: S1, graphId: G2, scope: 'root' })], {
      onboardingDone: true,
      storage: memStorage(),
      sessions: [{ id: S1, title: '空图会话' }],
    });
    expect(rNoGraph.router.state.location.pathname).toBe(`/c/${S1}`);
    rNoGraph.rendered.unmount();
  });
});
