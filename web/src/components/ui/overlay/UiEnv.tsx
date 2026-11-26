import { createContext, useContext, type JSX, type ReactNode } from 'react';

export interface UiEnv {
  /** W.02.04 的 keys.pushOverlay：浮层打开时压栈，返回出栈函数 */
  pushOverlay(kind: 'modal' | 'popover', handler: (e: KeyboardEvent) => boolean): () => void;
  /** 当前是否减少动效（W.02.03 的 themeController.get().reducedMotion） */
  reducedMotion(): boolean;
  /** 浮层挂载的顶层容器（缺省 document.body 下的 <div id="hx-layers">，首次使用时创建） */
  portalRoot(): HTMLElement;
}

let warnedFallback = false;

export function defaultPortalRoot(): HTMLElement {
  const existing = document.getElementById('hx-layers');
  if (existing) return existing;
  const el = document.createElement('div');
  el.id = 'hx-layers';
  document.body.appendChild(el);
  return el;
}

export const FALLBACK_ENV: UiEnv = {
  pushOverlay() {
    if (import.meta.env.DEV && !warnedFallback) {
      warnedFallback = true;
      console.warn('[ui] UiEnvProvider missing; falling back to FALLBACK_ENV without key registry.');
    }
    return () => {
      // no-op
    };
  },
  reducedMotion() {
    if (typeof document === 'undefined') return false;
    return document.documentElement.getAttribute('data-motion') === 'reduce';
  },
  portalRoot: defaultPortalRoot,
};

const UiEnvContext = createContext<UiEnv>(FALLBACK_ENV);

export function UiEnvProvider(p: { value: UiEnv; children: ReactNode }): JSX.Element {
  return <UiEnvContext.Provider value={p.value}>{p.children}</UiEnvContext.Provider>;
}

export function useUiEnv(): UiEnv {
  return useContext(UiEnvContext);
}
