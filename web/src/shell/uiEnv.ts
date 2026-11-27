import type { UiEnv } from '@/components/ui';
import { keys } from './keys';
import { themeController } from './theme';

const LAYERS_ROOT_ID = 'hx-layers';

export function layersRoot(doc: Document = document): HTMLElement {
  const existing = doc.getElementById(LAYERS_ROOT_ID);
  if (existing) return existing;
  const el = doc.createElement('div');
  el.id = LAYERS_ROOT_ID;
  doc.body.appendChild(el);
  return el;
}

/**
 * 提供给基础组件库的稳定界面环境（W.08 §4.3、W.02 §4.1、P.09 K121、K307）。
 * 模块级固定常量引用，避免传给 UiEnvProvider 时因重渲染触发 Overlay effect 重跑。
 */
export const shellUiEnv: UiEnv = {
  pushOverlay: (kind, handler) => keys.pushOverlay(kind, handler),
  reducedMotion: () => themeController.get().reducedMotion,
  portalRoot: () => layersRoot(),
};
