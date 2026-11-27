import { useSyncExternalStore } from 'react';
import { themeController, type ThemeControllerApi, type ThemeState } from './themeController';

/**
 * 订阅主题状态（W.02.03 §4.2、P.09 K309 D02）。
 * useSyncExternalStore 快照直接取稳定的 ThemeState 引用，避免 selector 返回新对象时触发无限重渲染。
 */
export function useTheme<T>(
  selector: (s: ThemeState) => T,
  controller: ThemeControllerApi = themeController,
): T {
  const state = useSyncExternalStore(
    (onStoreChange) =>
      controller.subscribe(() => {
        onStoreChange();
      }),
    () => controller.get(),
    () => controller.get(),
  );
  return selector(state);
}
