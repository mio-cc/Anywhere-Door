import { useEffect, useRef, type CSSProperties, type JSX, type KeyboardEvent } from 'react';
import '../base.css';
import { Tooltip } from '../overlay/Tooltip';
import { useUiEnv } from '../overlay/UiEnv';
import { isImeComposing, type Size } from '../types';
import './Segmented.css';

export interface SegmentedItem<V extends string = string> {
  readonly value: V;
  readonly label: string;
  readonly tip?: string;
  readonly disabled?: boolean;
  readonly disabledReason?: string;
}

export interface SegmentedProps<V extends string = string> {
  readonly value: V;
  readonly items: readonly SegmentedItem<V>[];
  readonly onChange: (v: V) => void;
  readonly size?: Size;
  readonly ariaLabel: string;
  readonly disabled?: boolean;
  readonly disabledReason?: string;
}

const segWarn = { count: false, unknown: false };

export function Segmented<V extends string = string>(p: SegmentedProps<V>): JSX.Element {
  const { value, items, onChange, size = 'md', ariaLabel, disabled = false, disabledReason } = p;
  const env = useUiEnv();
  const btnRefs = useRef<Array<HTMLButtonElement | null>>([]);
  const selectedIdx = items.findIndex((it) => it.value === value);

  useEffect(() => {
    if ((items.length < 2 || items.length > 5) && import.meta.env.DEV && !segWarn.count) {
      segWarn.count = true;
      console.warn(`[ui] Segmented expects 2-5 items, got ${String(items.length)}.`);
    }
  }, [items.length]);

  useEffect(() => {
    if (selectedIdx < 0 && items.length > 0 && import.meta.env.DEV && !segWarn.unknown) {
      segWarn.unknown = true;
      console.warn(`[ui] Segmented value "${value}" not found in items.`);
    }
  }, [selectedIdx, items.length, value]);

  const firstEnabledIdx = items.findIndex((it) => !it.disabled);
  const focusableIdx = selectedIdx >= 0 && !items[selectedIdx]?.disabled ? selectedIdx : firstEnabledIdx;

  const isReduced = env.reducedMotion();
  const softDisabled = disabled && Boolean(disabledReason);
  const nativeDisabled = disabled && !disabledReason;

  const move = (from: number, dir: 1 | -1): void => {
    const n = items.length;
    if (n === 0) return;
    for (let step = 1; step <= n; step += 1) {
      const idx = (from + dir * step + n) % n;
      const candidate = items[idx];
      if (candidate && !candidate.disabled) {
        onChange(candidate.value);
        btnRefs.current[idx]?.focus();
        return;
      }
    }
  };

  const handleKeyDown = (idx: number, e: KeyboardEvent<HTMLButtonElement>): void => {
    if (isImeComposing(e.nativeEvent) || disabled) return;
    if (e.key === 'ArrowRight' || e.key === 'ArrowDown') {
      e.preventDefault();
      e.stopPropagation();
      move(idx, 1);
    } else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') {
      e.preventDefault();
      e.stopPropagation();
      move(idx, -1);
    } else if (e.key === 'Home') {
      e.preventDefault();
      e.stopPropagation();
      const first = items.findIndex((it) => !it.disabled);
      const firstItem = first >= 0 ? items[first] : undefined;
      if (firstItem) {
        onChange(firstItem.value);
        btnRefs.current[first]?.focus();
      }
    } else if (e.key === 'End') {
      e.preventDefault();
      e.stopPropagation();
      for (let i = items.length - 1; i >= 0; i -= 1) {
        const lastItem = items[i];
        if (lastItem && !lastItem.disabled) {
          onChange(lastItem.value);
          btnRefs.current[i]?.focus();
          break;
        }
      }
    }
  };

  const rootStyle = {
    '--_n': String(Math.max(1, items.length)),
    '--_i': String(Math.max(0, selectedIdx)),
  } as CSSProperties;

  const rootClasses = [
    'hx-seg',
    'hx-ctl',
    size === 'sm' ? 'hx-ctl--sm' : '',
    isReduced ? 'is-reduced' : 'has-motion',
    softDisabled || nativeDisabled ? 'is-disabled' : '',
  ]
    .filter(Boolean)
    .join(' ');

  const group = (
    <div
      className={rootClasses}
      role="radiogroup"
      aria-label={ariaLabel}
      aria-disabled={softDisabled || nativeDisabled ? 'true' : undefined}
      style={rootStyle}
    >
      {selectedIdx >= 0 ? <span className="hx-seg__thumb" aria-hidden="true" /> : null}
      {items.map((it, idx) => {
        const checked = idx === selectedIdx;
        const itemDisabled = disabled || Boolean(it.disabled);
        const itemSoft = Boolean(it.disabled && it.disabledReason);
        const itemNative = itemDisabled && !itemSoft && !softDisabled;
        const btn = (
          <button
            key={it.value}
            ref={(el) => {
              btnRefs.current[idx] = el;
            }}
            type="button"
            role="radio"
            className={`hx-seg__item hx-focusring${checked ? ' is-selected' : ''}${itemSoft ? ' is-disabled' : ''}`}
            aria-checked={checked ? 'true' : 'false'}
            aria-disabled={itemSoft || softDisabled ? 'true' : undefined}
            disabled={itemNative}
            tabIndex={idx === focusableIdx ? 0 : -1}
            onClick={() => {
              if (itemDisabled || checked) return;
              onChange(it.value);
            }}
            onKeyDown={(e) => {
              handleKeyDown(idx, e);
            }}
          >
            {it.label}
          </button>
        );
        const tip = itemSoft ? it.disabledReason : it.tip;
        return tip ? (
          <Tooltip key={it.value} content={tip}>
            {btn}
          </Tooltip>
        ) : (
          btn
        );
      })}
    </div>
  );

  return softDisabled && disabledReason ? <Tooltip content={disabledReason}>{group}</Tooltip> : group;
}
