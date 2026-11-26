import {
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type ChangeEvent,
  type CSSProperties,
  type JSX,
  type KeyboardEvent,
} from 'react';
import '../base.css';
import { Overlay } from '../overlay/Overlay';
import { Tooltip } from '../overlay/Tooltip';
import { isImeComposing } from '../types';
import { FieldError } from './FieldError';
import { clamp, IDLE_END } from './stepMath';
import type { FieldBase } from './TextInput';
import './Slider.css';

export type SliderProps = Omit<FieldBase, 'mono' | 'lockReason'> & {
  readonly value: number;
  readonly min: number;
  readonly max: number;
  readonly step: number;
  readonly onStart: () => void;
  readonly onChange: (v: number) => void;
  readonly onEnd: (v: number) => void;
  readonly format?: (v: number) => string;
  readonly ariaLabel: string;
};

const THUMB_HALF_PX = 6;
const FLOAT_GAP_PX = 6;

export function Slider(p: SliderProps): JSX.Element {
  const {
    id: propId,
    disabled = false,
    disabledReason,
    readOnly = false,
    invalid = false,
    error,
    value,
    min,
    max,
    step,
    onStart,
    onChange,
    onEnd,
    format,
    ariaLabel,
  } = p;

  const autoId = useId();
  const errId = useId();
  const inputId = propId ?? autoId;

  const [active, setActive] = useState(false);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const floatRef = useRef<HTMLDivElement | null>(null);
  const gestureOpenRef = useRef(false);
  const curValRef = useRef(value);
  const keyIdleRef = useRef<number | null>(null);
  const cbRef = useRef({ onStart, onChange, onEnd });

  useLayoutEffect(() => {
    cbRef.current = { onStart, onChange, onEnd };
    curValRef.current = value;
  });

  const softDisabled = disabled && Boolean(disabledReason);
  const nativeDisabled = disabled && !disabledReason;
  const blocked = disabled || readOnly;

  const begin = (): void => {
    if (blocked) return;
    if (!gestureOpenRef.current) {
      gestureOpenRef.current = true;
      setActive(true);
      cbRef.current.onStart();
    }
  };

  const finish = (): void => {
    if (keyIdleRef.current !== null) {
      window.clearTimeout(keyIdleRef.current);
      keyIdleRef.current = null;
    }
    if (gestureOpenRef.current) {
      gestureOpenRef.current = false;
      setActive(false);
      cbRef.current.onEnd(curValRef.current);
    }
  };

  // 手势进行中若变为 disabled 或 readOnly，立即补发 onEnd 保证成对不变量（W.08.02 §7、评审 #17）
  useEffect(() => {
    if (blocked && gestureOpenRef.current) {
      finish();
    }
  }, [blocked]);

  useEffect(() => {
    return () => {
      if (keyIdleRef.current !== null) window.clearTimeout(keyIdleRef.current);
      if (gestureOpenRef.current) {
        gestureOpenRef.current = false;
        cbRef.current.onEnd(curValRef.current);
      }
    };
  }, []);

  const span = max - min;
  const ratio = span > 0 ? clamp((value - min) / span, 0, 1) : 0;
  const pct = `${String(Number((ratio * 100).toFixed(4)))}%`;
  const formatted = format ? format(value) : String(value);

  // 浮出数值跟随拇指位置（W.08.02 §6.5、评审 #17）
  useLayoutEffect(() => {
    if (!active || !inputRef.current || !floatRef.current) return;
    const rect = inputRef.current.getBoundingClientRect();
    const usable = Math.max(0, rect.width - THUMB_HALF_PX * 2);
    const thumbX = rect.left + THUMB_HALF_PX + usable * ratio;
    const topY = rect.top - FLOAT_GAP_PX;
    floatRef.current.style.position = 'fixed';
    floatRef.current.style.left = `${String(Math.round(thumbX))}px`;
    floatRef.current.style.top = `${String(Math.round(topY))}px`;
  }, [active, ratio, value]);

  const handleInput = (e: ChangeEvent<HTMLInputElement>): void => {
    if (blocked) return;
    const next = Number(e.target.value);
    curValRef.current = next;
    cbRef.current.onChange(next);
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>): void => {
    if (isImeComposing(e.nativeEvent) || blocked) return;
    if (
      e.key === 'ArrowLeft' ||
      e.key === 'ArrowRight' ||
      e.key === 'ArrowUp' ||
      e.key === 'ArrowDown' ||
      e.key === 'PageUp' ||
      e.key === 'PageDown' ||
      e.key === 'Home' ||
      e.key === 'End'
    ) {
      e.stopPropagation();
      begin();
      if (keyIdleRef.current !== null) window.clearTimeout(keyIdleRef.current);
      keyIdleRef.current = window.setTimeout(() => {
        finish();
      }, IDLE_END);
    }
  };

  const isInvalid = invalid || Boolean(error);
  const wrapClasses = [
    'hx-slider',
    isInvalid ? 'is-invalid' : '',
    softDisabled || nativeDisabled ? 'is-disabled' : '',
  ]
    .filter(Boolean)
    .join(' ');

  const rangeStyle = { '--_p': pct } as CSSProperties;

  const ctrl = (
    <div className={wrapClasses}>
      <input
        ref={inputRef}
        id={inputId}
        type="range"
        className="hx-slider__input hx-focusring"
        style={rangeStyle}
        min={min}
        max={max}
        step={step}
        value={value}
        disabled={nativeDisabled}
        aria-disabled={softDisabled ? 'true' : undefined}
        aria-label={ariaLabel}
        aria-valuetext={formatted}
        aria-invalid={isInvalid ? 'true' : undefined}
        aria-describedby={error ? errId : undefined}
        onPointerDown={() => {
          begin();
        }}
        onPointerUp={() => {
          finish();
        }}
        onPointerCancel={() => {
          finish();
        }}
        onChange={handleInput}
        onKeyDown={handleKeyDown}
        onBlur={() => {
          finish();
        }}
      />
      <Overlay layer="tooltip" open={active} className="hx-slider__val" contentRef={floatRef}>
        {formatted}
      </Overlay>
    </div>
  );

  return (
    <div className="hx-slider__field">
      {softDisabled && disabledReason ? <Tooltip content={disabledReason}>{ctrl}</Tooltip> : ctrl}
      {error ? <FieldError id={errId} text={error} /> : null}
    </div>
  );
}
