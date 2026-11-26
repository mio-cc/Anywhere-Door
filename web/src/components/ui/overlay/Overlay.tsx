import {
  useContext,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type AnimationEvent,
  type FocusEvent,
  type JSX,
  type ReactNode,
  type Ref,
} from 'react';
import { createPortal } from 'react-dom';
import { isImeComposing, type CloseReason, type Layer } from '../types';
import { cycleTab, focusInitial, resolveRestore } from './focus';
import { inTree, OverlayParent, pushLayer } from './layerStack';
import { useUiEnv } from './UiEnv';
import './Overlay.css';

export interface OverlayProps {
  readonly layer: Layer;
  readonly open: boolean;
  readonly onClose?: (reason: CloseReason) => void;
  readonly trapFocus?: boolean;
  readonly restoreFocus?: boolean | HTMLElement | (() => HTMLElement | null);
  readonly initialFocus?: 'first' | 'container' | 'none';
  readonly closeOnOutside?: boolean;
  readonly onKey?: (e: KeyboardEvent) => boolean;
  readonly scrim?: boolean;
  readonly anchorEl?: HTMLElement | null;
  readonly role?: 'dialog' | 'alertdialog' | 'menu' | 'listbox' | 'tooltip' | 'status' | 'alert' | 'region';
  readonly ariaLabel?: string;
  readonly ariaLabelledBy?: string;
  readonly ariaDescribedBy?: string;
  readonly ariaActiveDescendant?: string;
  readonly className?: string;
  readonly contentRef?: Ref<HTMLDivElement>;
  readonly children: ReactNode;
}

const DEFAULT_LEAVE_FALLBACK_MS = 180;

function readLeaveFallbackMs(el: HTMLElement | null): number {
  if (!el) return DEFAULT_LEAVE_FALLBACK_MS;
  const raw = getComputedStyle(el).getPropertyValue('--hx-t-2').trim();
  if (raw.endsWith('ms')) {
    const n = Number.parseFloat(raw.slice(0, -2));
    if (Number.isFinite(n) && n >= 0) return Math.ceil(n) + 20;
  } else if (raw.endsWith('s')) {
    const n = Number.parseFloat(raw.slice(0, -1));
    if (Number.isFinite(n) && n >= 0) return Math.ceil(n * 1000) + 20;
  }
  return DEFAULT_LEAVE_FALLBACK_MS;
}

function assignRef<T>(ref: Ref<T> | undefined, value: T | null): void {
  if (!ref) return;
  if (typeof ref === 'function') {
    ref(value);
  } else {
    (ref as { current: T | null }).current = value;
  }
}

export function Overlay(p: OverlayProps): JSX.Element | null {
  const {
    layer,
    open,
    onClose,
    trapFocus = layer === 'modal',
    restoreFocus = true,
    initialFocus: rawInitialFocus,
    closeOnOutside = layer === 'popover' || layer === 'pane',
    onKey,
    scrim = false,
    anchorEl = null,
    role,
    ariaLabel,
    ariaLabelledBy,
    ariaDescribedBy,
    ariaActiveDescendant,
    className,
    contentRef,
    children,
  } = p;

  const initialFocus = layer === 'toast' || layer === 'tooltip' ? 'none' : (rawInitialFocus ?? 'first');
  const env = useUiEnv();
  const parentId = useContext(OverlayParent);
  const id = useId();
  const contentElRef = useRef<HTMLDivElement | null>(null);
  const openerRef = useRef<Element | null>(null);
  const wasOpenRef = useRef(false);

  const [prevOpen, setPrevOpen] = useState(open);
  const [leaving, setLeaving] = useState(false);

  const isReduced = env.reducedMotion();
  const instantClose = isReduced || layer === 'toast' || layer === 'tooltip';

  if (open !== prevOpen) {
    setPrevOpen(open);
    if (open) {
      if (leaving) setLeaving(false);
    } else if (!instantClose) {
      setLeaving(true);
    } else if (leaving) {
      setLeaving(false);
    }
  }

  const onCloseRef = useRef(onClose);
  const onKeyRef = useRef(onKey);
  const trapRef = useRef(trapFocus);
  const closeOnOutsideRef = useRef(closeOnOutside);
  const anchorRef = useRef(anchorEl);
  const restoreRef = useRef(restoreFocus);

  useLayoutEffect(() => {
    onCloseRef.current = onClose;
    onKeyRef.current = onKey;
    trapRef.current = trapFocus;
    closeOnOutsideRef.current = closeOnOutside;
    anchorRef.current = anchorEl;
    restoreRef.current = restoreFocus;
  });

  const doRestoreFocus = (): void => {
    const content = contentElRef.current;
    const active = document.activeElement;
    const shouldRestore =
      active === null || active === document.body || (content !== null && content.contains(active));
    if (!shouldRestore) return;
    const target = resolveRestore(restoreRef.current, openerRef.current);
    target?.focus({ preventScroll: true });
  };

  useLayoutEffect(() => {
    if (open) {
      if (!wasOpenRef.current) {
        openerRef.current = document.activeElement;
        wasOpenRef.current = true;
      }
    } else if (wasOpenRef.current) {
      wasOpenRef.current = false;
      doRestoreFocus();
    }
  }, [open]);

  useEffect(() => {
    if (!leaving) return;
    const delay = readLeaveFallbackMs(contentElRef.current);
    const timer = window.setTimeout(() => {
      setLeaving(false);
    }, delay);
    return () => {
      window.clearTimeout(timer);
    };
  }, [leaving]);

  useLayoutEffect(() => {
    if (!open) return;

    const popLayer = pushLayer({
      id,
      el: () => contentElRef.current,
      anchor: () => anchorRef.current,
      parent: parentId,
    });

    let popOverlay: (() => void) | null = null;
    if (layer === 'modal' || layer === 'popover' || layer === 'pane') {
      const kind = layer === 'modal' ? 'modal' : 'popover';
      popOverlay = env.pushOverlay(kind, (e: KeyboardEvent) => {
        if (isImeComposing(e)) return false;
        if (e.key === 'Escape') {
          if (onCloseRef.current) {
            onCloseRef.current('esc');
            return true;
          }
          return onKeyRef.current?.(e) ?? false;
        }
        if (e.key === 'Tab' && trapRef.current && contentElRef.current) {
          return cycleTab(contentElRef.current, e);
        }
        return onKeyRef.current?.(e) ?? false;
      });
    }

    const onPointerDown = (e: PointerEvent): void => {
      if (!closeOnOutsideRef.current) return;
      const target = e.target;
      if (!(target instanceof Node)) return;
      if (inTree(id, target)) return;
      onCloseRef.current?.('outside');
    };

    document.addEventListener('pointerdown', onPointerDown, true);

    if (contentElRef.current && initialFocus !== 'none') {
      focusInitial(contentElRef.current, initialFocus);
    }

    return () => {
      document.removeEventListener('pointerdown', onPointerDown, true);
      popOverlay?.();
      popLayer();
    };
  }, [open, id, layer, parentId, env, initialFocus]);

  useEffect(() => {
    return () => {
      if (wasOpenRef.current) {
        wasOpenRef.current = false;
        doRestoreFocus();
      }
    };
  }, []);

  if (!open && !leaving) return null;

  const rootClasses = [
    'hx-ov',
    `hx-ov--${layer}`,
    leaving ? 'is-leaving' : 'is-open',
    isReduced ? 'is-reduced' : '',
    className ?? '',
  ]
    .filter(Boolean)
    .join(' ');

  const handleBlur = (e: FocusEvent<HTMLDivElement>): void => {
    if (!open || layer === 'modal' || initialFocus === 'none') return;
    const next = e.relatedTarget;
    if (next instanceof Node && !inTree(id, next)) {
      onCloseRef.current?.('tab');
    }
  };

  // 仅当浮层自身退出动画结束时卸载，忽略子元素冒泡的 animationend（评审 #3）
  const handleAnimationEnd = (e: AnimationEvent<HTMLDivElement>): void => {
    if (e.target !== e.currentTarget) return;
    if (leaving) {
      setLeaving(false);
    }
  };

  return createPortal(
    <OverlayParent.Provider value={id}>
      {scrim ? (
        <div
          className={`hx-ov__scrim hx-ov--${layer}${leaving ? ' is-leaving' : ''}${isReduced ? ' is-reduced' : ''}`}
          aria-hidden="true"
        />
      ) : null}
      <div
        ref={(node) => {
          contentElRef.current = node;
          assignRef(contentRef, node);
        }}
        className={rootClasses}
        role={role}
        aria-label={ariaLabel}
        aria-labelledby={ariaLabelledBy}
        aria-describedby={ariaDescribedBy}
        aria-activedescendant={ariaActiveDescendant}
        aria-modal={layer === 'modal' ? 'true' : undefined}
        tabIndex={initialFocus === 'none' ? undefined : -1}
        onBlur={handleBlur}
        onAnimationEnd={handleAnimationEnd}
      >
        {children}
      </div>
    </OverlayParent.Provider>,
    env.portalRoot(),
  );
}
