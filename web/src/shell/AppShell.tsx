import {
  useEffect,
  useLayoutEffect,
  useRef,
  useSyncExternalStore,
  type JSX,
  type PointerEvent,
  type ReactNode,
} from 'react';
import type { UiStoreApi } from '@/state/uiStore';
import { BannerSlot } from './BannerSlot';
import { bpOf, isOverlayRightSidebar } from './breakpoints';
import { CanvasArea, type CanvasAreaProps } from './CanvasArea';
import { keys, type ShortcutRegistryApi } from './keys';
import './AppShell.css';

export interface AppShellProps {
  readonly mode: 'session' | 'page';
  readonly uiStore: UiStoreApi;
  readonly keyRegistry?: ShortcutRegistryApi;
  readonly page?: ReactNode;
  readonly welcomePage?: boolean;
  readonly canvas?: ReactNode;
  readonly canvasOverlay?: ReactNode;
  readonly cover?: ReactNode;
  readonly topBar?: ReactNode;
  readonly leftSidebar?: ReactNode;
  readonly rightSidebar?: ReactNode;
  readonly banners?: ReactNode;
  readonly floatProps?: CanvasAreaProps['floatProps'];
}

export function AppShell(p: AppShellProps): JSX.Element {
  const {
    mode,
    uiStore,
    keyRegistry = keys,
    page,
    welcomePage = false,
    canvas,
    canvasOverlay,
    cover,
    topBar,
    leftSidebar,
    rightSidebar,
    banners,
    floatProps,
  } = p;

  const layout = useSyncExternalStore(
    (onChange) => uiStore.subscribe(onChange),
    () => uiStore.getState().layout,
    () => uiStore.getState().layout,
  );

  const appRef = useRef<HTMLDivElement | null>(null);
  const topRef = useRef<HTMLElement | null>(null);
  const lsRef = useRef<HTMLElement | null>(null);
  const rafRef = useRef<number | null>(null);

  // W.02.02 §6.5：mode='page' 时左栏 inert，/welcome 时顶部条也 inert
  useLayoutEffect(() => {
    const lsEl = lsRef.current;
    if (lsEl) {
      if (mode === 'page') lsEl.setAttribute('inert', '');
      else lsEl.removeAttribute('inert');
    }
    const topEl = topRef.current;
    if (topEl) {
      if (mode === 'page' && welcomePage) topEl.setAttribute('inert', '');
      else topEl.removeAttribute('inert');
    }
  }, [mode, welcomePage]);

  // W.02.02 §4.3、§9：ResizeObserver 观察窗口/根容器宽度，用 rAF 合并每帧最多一次写 bp
  useEffect(() => {
    const el = appRef.current;
    if (!el || typeof ResizeObserver === 'undefined') return undefined;

    const applyWidth = (w: number): void => {
      if (w <= 0) return;
      uiStore.actions.setBp(bpOf(w));
    };

    const ro = new ResizeObserver((entries) => {
      const entry = entries[0];
      const w = entry ? entry.contentRect.width : el.getBoundingClientRect().width;
      if (rafRef.current !== null) cancelAnimationFrame(rafRef.current);
      rafRef.current = requestAnimationFrame(() => {
        rafRef.current = null;
        applyWidth(w);
      });
    });

    ro.observe(el);
    return () => {
      ro.disconnect();
      if (rafRef.current !== null) {
        cancelAnimationFrame(rafRef.current);
        rafRef.current = null;
      }
    };
  }, [uiStore]);

  // W.02.02 §4.4、§6.4：注册 [、]、I、F 四个外壳布局快捷键
  useEffect(() => {
    if (mode !== 'session') return undefined;
    const offLs = keyRegistry.on('ls.toggle', () => {
      const cur = uiStore.getState().layout;
      uiStore.actions.setLs(!cur.lsOpen);
      return true;
    });
    const offRs = keyRegistry.on('rs.toggle', () => {
      const cur = uiStore.getState().layout;
      uiStore.actions.setRs(!cur.rsOpen);
      return true;
    });
    const offInsp = keyRegistry.on('rs.insp', () => {
      uiStore.actions.setRs(true, 'insp');
      return true;
    });
    const offFocus = keyRegistry.on('focus.toggle', () => {
      uiStore.actions.toggleFocus();
      return true;
    });
    return () => {
      offLs();
      offRs();
      offInsp();
      offFocus();
    };
  }, [mode, uiStore, keyRegistry]);

  const rsOverlay = isOverlayRightSidebar(layout.bp);
  const rsVisible = mode === 'session' && layout.rsOpen;
  const rsDocked = rsVisible && !rsOverlay;

  const mainClasses = [
    'hx-app__main',
    !layout.lsOpen ? 'is-ls-collapsed' : '',
    rsDocked ? 'is-rs-docked' : '',
  ]
    .filter(Boolean)
    .join(' ');

  const rsClasses = ['hx-app__rs', !rsVisible ? 'is-hidden' : '', rsVisible && rsOverlay ? 'is-overlay' : '']
    .filter(Boolean)
    .join(' ');

  // K309 D10：只在点击画布空白时关闭覆盖式右栏，排除浮动层/覆盖层/页面/节点（不误关）
  const handleCanvasPointerDown = (e: PointerEvent<HTMLDivElement>): void => {
    if (!rsVisible || !rsOverlay) return;
    const target = e.target;
    if (!(target instanceof Element)) return;
    if (
      target.closest(
        '.hx-canvas__float, .hx-canvas__overlay, .hx-canvas__cover, .hx-canvas__page, [data-hx-node], [data-hx-edge], [data-hx-note]',
      ) !== null
    ) {
      return;
    }
    uiStore.actions.setRs(false);
  };

  return (
    <div ref={appRef} className="hx-app" data-bp={layout.bp} data-mode={mode}>
      <header ref={topRef} className="hx-app__top">
        {topBar}
      </header>
      <BannerSlot bp={layout.bp}>{banners}</BannerSlot>
      <div className={mainClasses}>
        <aside ref={lsRef} className="hx-app__ls" data-open={layout.lsOpen ? 'true' : 'false'}>
          <div className="hx-app__ls-inner">{leftSidebar}</div>
        </aside>
        <CanvasArea
          mode={mode}
          bp={layout.bp}
          canvas={canvas}
          page={page}
          canvasOverlay={canvasOverlay}
          cover={cover}
          floatProps={{
            ...floatProps,
            onFitInsetsChange: (insets) => {
              uiStore.actions.setFitInsets(insets);
              floatProps?.onFitInsetsChange?.(insets);
            },
          }}
          onCanvasPointerDown={handleCanvasPointerDown}
        />
        <aside className={rsClasses} data-open={rsVisible ? 'true' : 'false'} data-tab={layout.rsTab}>
          <div className="hx-app__rs-inner">{rsVisible ? rightSidebar : null}</div>
        </aside>
      </div>
    </div>
  );
}
