import {
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type ChangeEvent,
  type JSX,
  type KeyboardEvent,
  type PointerEvent as ReactPointerEvent,
} from 'react';
import { useT } from '@/lib/i18n';
import '../base.css';
import { Icon } from '../icon/Icon';
import { Overlay } from '../overlay/Overlay';
import { Tooltip } from '../overlay/Tooltip';
import { isImeComposing } from '../types';
import { FieldError } from './FieldError';
import {
  clamp,
  decimals,
  DRAG_THRESHOLD,
  dragDelta,
  HINT_MS,
  IDLE_END,
  nudge,
  parseTyped,
  REPEAT_DELAY,
  REPEAT_EVERY,
  roundTo,
} from './stepMath';
import type { FieldBase } from './TextInput';
import './NumberStepper.css';

export type NumberStepperProps = Omit<FieldBase, 'mono'> & {
  readonly value: number;
  readonly min?: number;
  readonly max?: number;
  readonly step?: number;
  readonly onStart: () => void;
  readonly onChange: (v: number) => void;
  readonly onEnd: (v: number) => void;
  readonly format?: (v: number) => string;
  readonly ariaLabel: string;
};

type Mode = 'idle' | 'dragging' | 'repeat' | 'keys' | 'wheel' | 'editing';

const stepperWarn = { invalidStep: false };

function logInvalidStepOnce(rawStep: number): void {
  if (!stepperWarn.invalidStep) {
    stepperWarn.invalidStep = true;
    console.error('[ui] NumberStepper invalid step, falling back to 1:', rawStep);
  }
}

export function NumberStepper(p: NumberStepperProps): JSX.Element {
  const {
    size = 'md',
    id: propId,
    disabled = false,
    disabledReason,
    readOnly = false,
    lockReason,
    invalid = false,
    error,
    value,
    min,
    max,
    step: rawStep = 1,
    onStart,
    onChange,
    onEnd,
    format,
    ariaLabel,
  } = p;

  if (!Number.isFinite(rawStep) || rawStep <= 0) {
    if (import.meta.env.DEV) {
      throw new Error(`NumberStepper step must be a positive finite number, got ${String(rawStep)}`);
    }
    logInvalidStepOnce(rawStep);
  }
  const step = Number.isFinite(rawStep) && rawStep > 0 ? rawStep : 1;

  const t = useT();
  const autoId = useId();
  const errId = useId();
  const spinId = propId ?? autoId;

  const softDisabled = disabled && Boolean(disabledReason);
  const nativeDisabled = disabled && !disabledReason;
  const isLocked = Boolean(lockReason);
  const blocked = disabled || readOnly || isLocked;

  const [editing, setEditing] = useState(false);
  const [editText, setEditText] = useState('');
  const [editInvalid, setEditInvalid] = useState(false);
  const [hint, setHint] = useState<string | null>(null);

  const valRef = useRef<HTMLSpanElement | null>(null);
  const floatTipRef = useRef<HTMLDivElement | null>(null);
  const editRef = useRef<HTMLInputElement | null>(null);

  const modeRef = useRef<Mode>('idle');
  const gestureOpenRef = useRef(false);
  const curValRef = useRef(value);
  const propValRef = useRef(value);
  const pendingDragRef = useRef<{
    pointerId: number;
    startX: number;
    lastX: number;
    moved: boolean;
    acc: number;
    rafId: number;
    queuedVal: number | null;
    pointerLocked: boolean;
    requestedLock: boolean;
  } | null>(null);
  const repeatDelayRef = useRef<number | null>(null);
  const repeatIntRef = useRef<number | null>(null);
  const idleTimerRef = useRef<number | null>(null);
  const hintTimerRef = useRef<number | null>(null);
  const skipEditBlurRef = useRef(false);

  const cbRef = useRef({ onStart, onChange, onEnd, format, min, max, step });
  useLayoutEffect(() => {
    cbRef.current = { onStart, onChange, onEnd, format, min, max, step };
    propValRef.current = value;
    if (!gestureOpenRef.current) {
      curValRef.current = value;
    }
  });

  const formatVal = (v: number, f: ((n: number) => string) | undefined, st: number): string => {
    if (f) return f(v);
    const d = decimals(st);
    return d > 0 ? roundTo(v, st).toFixed(d) : String(roundTo(v, st));
  };

  const fmt = (v: number): string => formatVal(v, cbRef.current.format, cbRef.current.step);
  const displayValue = formatVal(value, format, step);

  const positionFloatTip = (text: string): void => {
    const tip = floatTipRef.current;
    const anchor = valRef.current;
    if (!tip || !anchor) return;
    const rect = anchor.getBoundingClientRect();
    tip.textContent = text;
    tip.style.position = 'fixed';
    tip.style.left = `${String(Math.round(rect.left + rect.width / 2))}px`;
    tip.style.top = `${String(Math.round(rect.top - 6))}px`;
    tip.classList.add('is-on');
  };

  const showHint = (msg: string): void => {
    setHint(msg);
    if (hintTimerRef.current !== null) window.clearTimeout(hintTimerRef.current);
    hintTimerRef.current = window.setTimeout(() => {
      hintTimerRef.current = null;
      setHint(null);
    }, HINT_MS);
  };

  const clearTimers = (): void => {
    if (repeatDelayRef.current !== null) {
      window.clearTimeout(repeatDelayRef.current);
      repeatDelayRef.current = null;
    }
    if (repeatIntRef.current !== null) {
      window.clearInterval(repeatIntRef.current);
      repeatIntRef.current = null;
    }
    if (idleTimerRef.current !== null) {
      window.clearTimeout(idleTimerRef.current);
      idleTimerRef.current = null;
    }
  };

  const startGesture = (mode: Mode): void => {
    if (!gestureOpenRef.current) {
      gestureOpenRef.current = true;
      modeRef.current = mode;
      curValRef.current = clamp(propValRef.current, cbRef.current.min, cbRef.current.max);
      cbRef.current.onStart();
    }
  };

  const endGesture = (): void => {
    clearTimers();
    const drag = pendingDragRef.current;
    if (drag) {
      if (drag.rafId !== 0) cancelAnimationFrame(drag.rafId);
      if (drag.queuedVal !== null && drag.queuedVal !== curValRef.current) {
        curValRef.current = drag.queuedVal;
        cbRef.current.onChange(drag.queuedVal);
      }
      const hasLock =
        drag.pointerLocked || (valRef.current !== null && document.pointerLockElement === valRef.current);
      pendingDragRef.current = null;
      if (hasLock && typeof document.exitPointerLock === 'function') {
        try {
          document.exitPointerLock();
        } catch {
          // ignore
        }
      }
    }
    if (valRef.current) {
      valRef.current.classList.remove('is-dragging');
    }
    if (floatTipRef.current) {
      floatTipRef.current.classList.remove('is-on');
    }
    if (gestureOpenRef.current) {
      gestureOpenRef.current = false;
      modeRef.current = 'idle';
      cbRef.current.onEnd(curValRef.current);
    } else {
      modeRef.current = 'idle';
    }
  };

  const applyStepVal = (next: number, directDom: boolean): void => {
    const rounded = clamp(roundTo(next, cbRef.current.step), cbRef.current.min, cbRef.current.max);
    if (rounded === curValRef.current) return;
    curValRef.current = rounded;
    if (directDom && valRef.current) {
      valRef.current.textContent = fmt(rounded);
      if (floatTipRef.current) {
        floatTipRef.current.textContent = fmt(rounded);
      }
    }
    cbRef.current.onChange(rounded);
  };

  // 组件被禁用/锁定时补发 onEnd（W.08.02 §7、§8）
  useEffect(() => {
    if (blocked && gestureOpenRef.current) {
      endGesture();
    }
  }, [blocked]);

  // 监听 pointerlockchange（锁定成功切换 movementX；锁定意外丢失时结束手势，评审 #16）
  useEffect(() => {
    const onLockChange = (): void => {
      const drag = pendingDragRef.current;
      if (!drag) return;
      if (valRef.current !== null && document.pointerLockElement === valRef.current) {
        drag.pointerLocked = true;
      } else if (drag.pointerLocked) {
        drag.pointerLocked = false;
        endGesture();
      }
    };
    document.addEventListener('pointerlockchange', onLockChange);
    return () => {
      document.removeEventListener('pointerlockchange', onLockChange);
    };
  }, []);

  useEffect(() => {
    return () => {
      if (hintTimerRef.current !== null) window.clearTimeout(hintTimerRef.current);
      if (gestureOpenRef.current) {
        endGesture();
      } else {
        clearTimers();
      }
    };
  }, []);

  // 原生 wheel 监听：依赖 editing，退出编辑重挂新的数值节点后自动重新绑定（评审 #16）
  useEffect(() => {
    const el = valRef.current;
    if (!el || editing) return;
    const onWheel = (e: WheelEvent): void => {
      if (blocked || document.activeElement !== el) return;
      e.preventDefault();
      const dir: 1 | -1 = e.deltaY < 0 ? 1 : -1;
      startGesture('wheel');
      const next = nudge(
        curValRef.current,
        dir,
        cbRef.current.step,
        e.shiftKey,
        cbRef.current.min,
        cbRef.current.max,
      );
      applyStepVal(next, false);
      if (idleTimerRef.current !== null) window.clearTimeout(idleTimerRef.current);
      idleTimerRef.current = window.setTimeout(() => {
        endGesture();
      }, IDLE_END);
    };
    el.addEventListener('wheel', onWheel, { passive: false });
    return () => {
      el.removeEventListener('wheel', onWheel);
    };
  }, [blocked, editing]);

  // 拖动交互（直接写文本节点与 Overlay tooltip，每帧最多一次 onChange，不 setState）
  const handleValPointerDown = (e: ReactPointerEvent<HTMLSpanElement>): void => {
    if (blocked || editing || e.button !== 0) return;
    pendingDragRef.current = {
      pointerId: e.pointerId,
      startX: e.clientX,
      lastX: e.clientX,
      moved: false,
      acc: 0,
      rafId: 0,
      queuedVal: null,
      pointerLocked: false,
      requestedLock: false,
    };
    if (typeof e.currentTarget.setPointerCapture === 'function') {
      try {
        e.currentTarget.setPointerCapture(e.pointerId);
      } catch {
        // ignore
      }
    }
  };

  const handleValPointerMove = (e: ReactPointerEvent<HTMLSpanElement>): void => {
    const drag = pendingDragRef.current;
    if (!drag || drag.pointerId !== e.pointerId) return;

    const dx = drag.pointerLocked ? e.movementX : e.clientX - drag.lastX;
    drag.lastX = e.clientX;

    if (!drag.moved) {
      if (Math.abs(e.clientX - drag.startX) < DRAG_THRESHOLD) return;
      drag.moved = true;
      startGesture('dragging');
      if (valRef.current) {
        valRef.current.classList.add('is-dragging');
      }
      positionFloatTip(fmt(curValRef.current));
      if (!drag.requestedLock && typeof e.currentTarget.requestPointerLock === 'function') {
        drag.requestedLock = true;
        try {
          const maybe = e.currentTarget.requestPointerLock() as unknown;
          if (maybe && typeof (maybe as Promise<unknown>).then === 'function') {
            (maybe as Promise<unknown>)
              .then(() => {
                if (pendingDragRef.current === drag) {
                  drag.pointerLocked = true;
                }
              })
              .catch(() => {
                // 失败则继续用指针捕获
              });
          } else if (document.pointerLockElement === e.currentTarget) {
            drag.pointerLocked = true;
          }
        } catch {
          // 降级使用指针捕获
        }
      }
    }

    const { steps, acc } = dragDelta(drag.acc, dx, { shift: e.shiftKey, alt: e.altKey });
    drag.acc = acc;
    if (steps === 0) return;

    const base = drag.queuedVal ?? curValRef.current;
    const next = clamp(
      roundTo(base + steps * cbRef.current.step, cbRef.current.step),
      cbRef.current.min,
      cbRef.current.max,
    );
    drag.queuedVal = next;

    // 拖动中直接更新 DOM 文本与浮出层，不触发 setState（W.08 §7、W.08.02 §9）
    const text = fmt(next);
    if (valRef.current) {
      valRef.current.textContent = text;
    }
    if (floatTipRef.current) {
      floatTipRef.current.textContent = text;
    }

    if (drag.rafId === 0) {
      drag.rafId = requestAnimationFrame(() => {
        const cur = pendingDragRef.current;
        if (!cur) return;
        cur.rafId = 0;
        if (cur.queuedVal !== null && cur.queuedVal !== curValRef.current) {
          const v = cur.queuedVal;
          cur.queuedVal = null;
          curValRef.current = v;
          cbRef.current.onChange(v);
        }
      });
    }
  };

  const handleValPointerEnd = (e: ReactPointerEvent<HTMLSpanElement>): void => {
    const drag = pendingDragRef.current;
    if (!drag || drag.pointerId !== e.pointerId) return;
    if (typeof e.currentTarget.releasePointerCapture === 'function') {
      try {
        e.currentTarget.releasePointerCapture(e.pointerId);
      } catch {
        // ignore
      }
    }
    if (drag.moved) {
      endGesture();
    } else {
      pendingDragRef.current = null;
    }
  };

  const handleLostPointerCapture = (): void => {
    const drag = pendingDragRef.current;
    if (!drag || drag.pointerLocked) return;
    if (drag.moved || modeRef.current === 'dragging') {
      endGesture();
    } else {
      pendingDragRef.current = null;
    }
  };

  // ± 按钮长按（按下立即 1 步，400ms 后每 60ms 1 步）
  const startRepeat = (dir: 1 | -1): void => {
    if (blocked) return;
    startGesture('repeat');
    const stepOnce = (): void => {
      const next = nudge(
        curValRef.current,
        dir,
        cbRef.current.step,
        false,
        cbRef.current.min,
        cbRef.current.max,
      );
      applyStepVal(next, false);
    };
    stepOnce();
    repeatDelayRef.current = window.setTimeout(() => {
      repeatDelayRef.current = null;
      stepOnce();
      repeatIntRef.current = window.setInterval(stepOnce, REPEAT_EVERY);
    }, REPEAT_DELAY);
  };

  const stopRepeat = (): void => {
    if (modeRef.current === 'repeat') {
      endGesture();
    }
  };

  // 键盘调整（方向键 / PageUp / PageDown / Home / End；Enter 进入编辑）
  const handleSpinKeyDown = (e: KeyboardEvent<HTMLSpanElement>): void => {
    if (isImeComposing(e.nativeEvent) || blocked) return;
    if (e.key === 'Enter') {
      e.preventDefault();
      e.stopPropagation();
      enterEdit();
      return;
    }
    const baseVal = gestureOpenRef.current ? curValRef.current : clamp(propValRef.current, min, max);
    let next: number | null = null;
    if (e.key === 'ArrowUp' || e.key === 'ArrowRight') {
      next = nudge(baseVal, 1, step, e.shiftKey, min, max);
    } else if (e.key === 'ArrowDown' || e.key === 'ArrowLeft') {
      next = nudge(baseVal, -1, step, e.shiftKey, min, max);
    } else if (e.key === 'PageUp') {
      next = nudge(baseVal, 1, step, true, min, max);
    } else if (e.key === 'PageDown') {
      next = nudge(baseVal, -1, step, true, min, max);
    } else if (e.key === 'Home' && min !== undefined) {
      next = min;
    } else if (e.key === 'End' && max !== undefined) {
      next = max;
    }
    if (next === null) return;
    e.preventDefault();
    e.stopPropagation();
    startGesture('keys');
    applyStepVal(next, false);
    if (idleTimerRef.current !== null) window.clearTimeout(idleTimerRef.current);
    idleTimerRef.current = window.setTimeout(() => {
      endGesture();
    }, IDLE_END);
  };

  const enterEdit = (): void => {
    if (blocked) return;
    if (gestureOpenRef.current) endGesture();
    modeRef.current = 'editing';
    setEditText(String(value));
    setEditInvalid(false);
    setEditing(true);
  };

  useEffect(() => {
    if (editing && editRef.current) {
      editRef.current.focus();
      editRef.current.select();
    }
  }, [editing]);

  const commitEdit = (): void => {
    const parsed = parseTyped(editText);
    if (parsed === null) {
      setEditInvalid(true);
      return;
    }
    let clamped = roundTo(parsed, step);
    if (max !== undefined && clamped > max) {
      clamped = max;
      showHint(t('ui.max', { n: max }));
    } else if (min !== undefined && clamped < min) {
      clamped = min;
      showHint(t('ui.min', { n: min }));
    }
    setEditing(false);
    setEditInvalid(false);
    modeRef.current = 'idle';
    onStart();
    onChange(clamped);
    onEnd(clamped);
  };

  const cancelEdit = (): void => {
    skipEditBlurRef.current = true;
    setEditing(false);
    setEditInvalid(false);
    modeRef.current = 'idle';
    valRef.current?.focus();
  };

  const atMin = min !== undefined && value <= min;
  const atMax = max !== undefined && value >= max;
  const isInvalid = invalid || Boolean(error) || editInvalid;

  const classes = [
    'hx-step',
    'hx-ctl',
    size === 'sm' ? 'hx-ctl--sm' : '',
    isInvalid ? 'is-invalid' : '',
    softDisabled || nativeDisabled ? 'is-disabled' : '',
    isLocked ? 'is-locked' : '',
  ]
    .filter(Boolean)
    .join(' ');

  const control = (
    <div
      className={classes}
      role="group"
      aria-label={ariaLabel}
      aria-disabled={softDisabled ? 'true' : undefined}
    >
      <button
        type="button"
        className="hx-step__btn"
        tabIndex={-1}
        aria-label={t('ui.decrease')}
        disabled={blocked || atMin}
        onPointerDown={(e) => {
          if (e.button !== 0) return;
          startRepeat(-1);
        }}
        onPointerUp={stopRepeat}
        onPointerLeave={stopRepeat}
        onPointerCancel={stopRepeat}
      >
        <Icon name="minus" size={14} />
      </button>

      {editing ? (
        <input
          ref={editRef}
          id={spinId}
          type="text"
          className="hx-step__edit hx-focusring"
          value={editText}
          aria-label={ariaLabel}
          aria-invalid={editInvalid ? 'true' : undefined}
          onChange={(e: ChangeEvent<HTMLInputElement>) => {
            setEditText(e.target.value);
            if (editInvalid) setEditInvalid(false);
          }}
          onKeyDown={(e: KeyboardEvent<HTMLInputElement>) => {
            if (isImeComposing(e.nativeEvent)) return;
            if (e.key === 'Enter') {
              e.preventDefault();
              e.stopPropagation();
              commitEdit();
            } else if (e.key === 'Escape') {
              e.preventDefault();
              e.stopPropagation();
              cancelEdit();
            }
          }}
          onBlur={() => {
            if (skipEditBlurRef.current) {
              skipEditBlurRef.current = false;
              return;
            }
            commitEdit();
          }}
        />
      ) : (
        <span
          ref={valRef}
          id={spinId}
          className="hx-step__v hx-focusring"
          role="spinbutton"
          tabIndex={nativeDisabled ? -1 : 0}
          aria-valuenow={value}
          aria-valuemin={min}
          aria-valuemax={max}
          aria-valuetext={displayValue}
          aria-label={ariaLabel}
          aria-disabled={softDisabled || nativeDisabled ? 'true' : undefined}
          aria-invalid={isInvalid ? 'true' : undefined}
          aria-describedby={error ? errId : undefined}
          onPointerDown={handleValPointerDown}
          onPointerMove={handleValPointerMove}
          onPointerUp={handleValPointerEnd}
          onPointerCancel={handleValPointerEnd}
          onLostPointerCapture={handleLostPointerCapture}
          onDoubleClick={enterEdit}
          onKeyDown={handleSpinKeyDown}
          onBlur={() => {
            if (modeRef.current === 'keys' || modeRef.current === 'wheel') {
              endGesture();
            }
          }}
        >
          {displayValue}
        </span>
      )}

      <button
        type="button"
        className="hx-step__btn"
        tabIndex={-1}
        aria-label={t('ui.increase')}
        disabled={blocked || atMax}
        onPointerDown={(e) => {
          if (e.button !== 0) return;
          startRepeat(1);
        }}
        onPointerUp={stopRepeat}
        onPointerLeave={stopRepeat}
        onPointerCancel={stopRepeat}
      >
        <Icon name="plus" size={14} />
      </button>

      {isLocked ? (
        <span className="hx-step__lock">
          <Icon name="lock" size={14} label={t('ui.locked')} />
        </span>
      ) : null}
    </div>
  );

  const tip = softDisabled ? disabledReason : lockReason;
  return (
    <div className="hx-step__field">
      {tip ? <Tooltip content={tip}>{control}</Tooltip> : control}
      {/* 拖动中的数值浮层挂在 Overlay layer='tooltip'，仅直接写 DOM 文本与坐标，零 React 重渲染（W.08.02 §6.4、评审 #16） */}
      <Overlay layer="tooltip" open className="hx-step__float" contentRef={floatTipRef}>
        {''}
      </Overlay>
      {hint ? (
        <Overlay layer="tooltip" open role="status" className="hx-step__hint">
          {hint}
        </Overlay>
      ) : null}
      {error ? <FieldError id={errId} text={error} /> : null}
    </div>
  );
}
