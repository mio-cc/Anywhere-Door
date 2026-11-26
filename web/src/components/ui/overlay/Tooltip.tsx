import {
  cloneElement,
  isValidElement,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type FocusEvent,
  type JSX,
  type KeyboardEvent as ReactKeyboardEvent,
  type PointerEvent as ReactPointerEvent,
  type ReactElement,
  type ReactNode,
  type Ref,
} from 'react';
import { Kbd } from '../button/Kbd';
import type { Placement } from '../types';
import { Overlay } from './Overlay';
import { place, VIEWPORT_PADDING } from './position';
import './Tooltip.css';

export interface TooltipProps {
  readonly content: ReactNode;
  readonly shortcut?: string;
  readonly delay?: number;
  readonly placement?: Placement;
  readonly disabled?: boolean;
  readonly children: ReactElement;
}

const WARM_WINDOW_MS = 300;
const TIP_OFFSET_PX = 6;

const tooltipState = {
  activeHide: null as (() => void) | null,
  lastHiddenAt: -Infinity,
  warnedChild: false,
  warnedDomRef: false,
};

/** 测试用：重置提示框模块级暖场状态，避免用例间污染。 */
export function resetTooltipState(): void {
  tooltipState.activeHide = null;
  tooltipState.lastHiddenAt = -Infinity;
}

function isTooltipWarm(): boolean {
  return Date.now() - tooltipState.lastHiddenAt <= WARM_WINDOW_MS;
}

function activateTooltip(hide: () => void): void {
  if (tooltipState.activeHide && tooltipState.activeHide !== hide) {
    tooltipState.activeHide();
  }
  tooltipState.activeHide = hide;
}

function deactivateTooltip(hide: () => void, wasOpen: boolean): void {
  if (wasOpen) {
    tooltipState.lastHiddenAt = Date.now();
  }
  if (tooltipState.activeHide === hide) {
    tooltipState.activeHide = null;
  }
}

function logInvalidChildOnce(): void {
  if (!tooltipState.warnedChild) {
    tooltipState.warnedChild = true;
    console.error('[ui] Tooltip child must be a single ReactElement.');
  }
}

function logMissingDomRefOnce(): void {
  if (!tooltipState.warnedDomRef) {
    tooltipState.warnedDomRef = true;
    console.error('[ui] Tooltip child must forward ref to a DOM element (W.08.04 §8).');
  }
}

function forwardRef<T>(target: Ref<T> | undefined, node: T | null): void {
  if (!target) return;
  if (typeof target === 'function') {
    target(node);
  } else {
    (target as { current: T | null }).current = node;
  }
}

function isFocusVisible(el: HTMLElement): boolean {
  try {
    return el.matches(':focus-visible');
  } catch {
    return true;
  }
}

interface ChildDomProps {
  ref?: Ref<HTMLElement>;
  onPointerEnter?: (e: ReactPointerEvent<HTMLElement>) => void;
  onPointerLeave?: (e: ReactPointerEvent<HTMLElement>) => void;
  onPointerDown?: (e: ReactPointerEvent<HTMLElement>) => void;
  onFocus?: (e: FocusEvent<HTMLElement>) => void;
  onBlur?: (e: FocusEvent<HTMLElement>) => void;
  onKeyDown?: (e: ReactKeyboardEvent<HTMLElement>) => void;
  'aria-describedby'?: string | undefined;
  'data-hx-tip-bound'?: string;
}

export function Tooltip(p: TooltipProps): JSX.Element {
  const { content, shortcut, delay = 400, placement = 'top', disabled = false, children } = p;
  const tipId = useId();
  const triggerRef = useRef<HTMLElement | null>(null);
  const tipRef = useRef<HTMLDivElement | null>(null);
  const timerRef = useRef<number | null>(null);
  const isOpenRef = useRef(false);
  const hideRef = useRef<() => void>(() => {
    // updated in layout effect
  });
  const [open, setOpen] = useState(false);
  const [prevDisabled, setPrevDisabled] = useState(disabled);

  const validChild = isValidElement(children);

  if (disabled !== prevDisabled) {
    setPrevDisabled(disabled);
    if (disabled && open) {
      setOpen(false);
    }
  }

  const clearDelay = (): void => {
    if (timerRef.current !== null) {
      window.clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  };

  // 所有隐藏路径（离开、失焦、按下、Esc、disabled）均更新暖场时间（评审 #10）
  const hideNow = (): void => {
    clearDelay();
    const wasOpen = isOpenRef.current;
    isOpenRef.current = false;
    setOpen(false);
    deactivateTooltip(hideRef.current, wasOpen);
  };

  useLayoutEffect(() => {
    hideRef.current = () => {
      hideNow();
    };
  });

  useEffect(() => {
    if (disabled && isOpenRef.current) {
      clearDelay();
      isOpenRef.current = false;
      deactivateTooltip(hideRef.current, true);
    }
  }, [disabled]);

  useEffect(() => {
    if (!validChild) {
      logInvalidChildOnce();
    }
  }, [validChild]);

  // 校验子元素必须将 ref 与注入的事件/标记 props 落到 DOM 元素（W.08.04 §6.5、§8、评审 #11）
  useLayoutEffect(() => {
    if (!validChild) return;
    const node = triggerRef.current;
    if (!(node instanceof HTMLElement) || !node.hasAttribute('data-hx-tip-bound')) {
      if (import.meta.env.DEV) {
        throw new Error('Tooltip child must forward ref and DOM event props to a DOM element (W.08.04 §8).');
      }
      logMissingDomRefOnce();
    }
  }, [validChild, children]);

  useEffect(() => {
    return () => {
      clearDelay();
      const wasOpen = isOpenRef.current;
      isOpenRef.current = false;
      deactivateTooltip(hideRef.current, wasOpen);
    };
  }, []);

  const visible = open && !disabled && validChild;

  useLayoutEffect(() => {
    if (!visible || !triggerRef.current || !tipRef.current) return;
    return place(triggerRef.current, tipRef.current, {
      placement,
      offset: TIP_OFFSET_PX,
      padding: VIEWPORT_PADDING,
      onAnchorGone: () => {
        hideRef.current();
      },
    });
  }, [visible, placement]);

  if (!validChild) {
    if (import.meta.env.DEV) {
      throw new Error('Tooltip requires a single valid ReactElement child.');
    }
    return <>{children}</>;
  }

  const showNow = (): void => {
    clearDelay();
    if (disabled || !triggerRef.current) return;
    activateTooltip(hideRef.current);
    isOpenRef.current = true;
    setOpen(true);
  };

  const scheduleShow = (): void => {
    if (disabled) return;
    clearDelay();
    if (isTooltipWarm() || delay <= 0) {
      showNow();
      return;
    }
    timerRef.current = window.setTimeout(() => {
      timerRef.current = null;
      showNow();
    }, delay);
  };

  const child = children as ReactElement<ChildDomProps>;
  const childProps = child.props;
  const childRef = childProps.ref;
  const describedBy = visible
    ? [childProps['aria-describedby'], tipId].filter(Boolean).join(' ')
    : childProps['aria-describedby'];

  const setTriggerNode = (node: HTMLElement | null): void => {
    triggerRef.current = node;
    forwardRef(childRef, node);
  };

  // eslint-disable-next-line react-hooks/refs -- cloneElement 仅注册 DOM 回调 ref，不在渲染期间读取 ref.current
  const cloned = cloneElement(child, {
    ref: setTriggerNode,
    'data-hx-tip-bound': '',
    'aria-describedby': describedBy,
    onPointerEnter: (e: ReactPointerEvent<HTMLElement>) => {
      childProps.onPointerEnter?.(e);
      if (e.pointerType === 'touch' || disabled) return;
      scheduleShow();
    },
    onPointerLeave: (e: ReactPointerEvent<HTMLElement>) => {
      childProps.onPointerLeave?.(e);
      hideNow();
    },
    onPointerDown: (e: ReactPointerEvent<HTMLElement>) => {
      childProps.onPointerDown?.(e);
      hideNow();
    },
    onFocus: (e: FocusEvent<HTMLElement>) => {
      childProps.onFocus?.(e);
      if (disabled) return;
      if (isFocusVisible(e.currentTarget)) {
        showNow();
      }
    },
    onBlur: (e: FocusEvent<HTMLElement>) => {
      childProps.onBlur?.(e);
      hideNow();
    },
    onKeyDown: (e: ReactKeyboardEvent<HTMLElement>) => {
      childProps.onKeyDown?.(e);
      if (e.key === 'Escape' && visible) {
        hideNow();
      }
    },
  });

  return (
    <>
      {cloned}
      <Overlay layer="tooltip" open={visible} role="tooltip" className="hx-tip" contentRef={tipRef}>
        <span id={tipId} className="hx-tip__body">
          <span className="hx-tip__text">{content}</span>
          {shortcut ? <Kbd text={shortcut} variant="tip" /> : null}
        </span>
      </Overlay>
    </>
  );
}
