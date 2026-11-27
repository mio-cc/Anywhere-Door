import { useMemo, type JSX } from 'react';
import { RouterProvider } from 'react-router';
import { UiEnvProvider, type UiEnv } from '@/components/ui';
import { useT } from '@/lib/i18n';
import { createAppRouter, RouteServicesProvider, type RouteServices } from '@/routes';
import { AppShell } from '@/shell/AppShell';
import { shellUiEnv } from '@/shell/uiEnv';
import { createUiStore, type UiStoreApi } from '@/state/uiStore';

export interface AppProps {
  readonly uiStore?: UiStoreApi;
  readonly uiEnv?: UiEnv;
  readonly router?: ReturnType<typeof createAppRouter>;
  readonly services?: Partial<RouteServices>;
}

/**
 * Anywhere Door 顶层应用入口（W.02 §4.1、P.09 K305、K307）。
 * 传给 UiEnvProvider 的 value 缺省为模块级固定常量 shellUiEnv，避免重渲染触发浮层焦点 effect 重跑。
 */
export function App(p: AppProps = {}): JSX.Element {
  const t = useT();
  const uiStore = useMemo(
    () => p.uiStore ?? createUiStore({ width: typeof window !== 'undefined' ? window.innerWidth : 1280 }),
    [p.uiStore],
  );
  const router = useMemo(() => p.router ?? createAppRouter(), [p.router]);
  const uiEnv = p.uiEnv ?? shellUiEnv;

  const services: RouteServices = useMemo(
    () => ({
      onboardingDone: true,
      sessions: [],
      pendingTitlePrefix: t('shell.titlePending'),
      renderShell: (shellProps) => (
        <AppShell
          mode={shellProps.mode}
          uiStore={uiStore}
          {...(shellProps.welcomePage !== undefined ? { welcomePage: shellProps.welcomePage } : {})}
          page={shellProps.page}
          canvasOverlay={shellProps.canvasOverlay}
          cover={shellProps.cover}
        />
      ),
      ...p.services,
    }),
    [p.services, t, uiStore],
  );

  return (
    <UiEnvProvider value={uiEnv}>
      <RouteServicesProvider value={services}>
        <RouterProvider router={router} />
      </RouteServicesProvider>
    </UiEnvProvider>
  );
}
