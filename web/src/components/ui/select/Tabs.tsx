import { useRef, type JSX, type KeyboardEvent } from 'react';
import '../base.css';
import { Badge } from '../feedback/Badge';
import { isImeComposing } from '../types';
import './Tabs.css';

export interface TabItem<V extends string = string> {
  readonly value: V;
  readonly label: string;
  readonly badge?: string | number;
  readonly disabled?: boolean;
}

export interface TabsProps<V extends string = string> {
  readonly value: V;
  readonly items: readonly TabItem<V>[];
  readonly onChange: (v: V) => void;
  readonly variant: 'underline' | 'pill';
  readonly ariaLabel: string;
  readonly panelIdOf?: (v: V) => string;
}

export function Tabs<V extends string = string>(p: TabsProps<V>): JSX.Element {
  const { value, items, onChange, variant, ariaLabel, panelIdOf } = p;
  const refs = useRef<Array<HTMLButtonElement | null>>([]);
  const activeIdx = items.findIndex((it) => it.value === value);
  const firstEnabled = items.findIndex((it) => !it.disabled);
  const focusableIdx = activeIdx >= 0 && !items[activeIdx]?.disabled ? activeIdx : firstEnabled;

  const move = (from: number, dir: 1 | -1): void => {
    const n = items.length;
    if (n === 0) return;
    for (let step = 1; step <= n; step += 1) {
      const idx = (from + dir * step + n) % n;
      const candidate = items[idx];
      if (candidate && !candidate.disabled) {
        onChange(candidate.value);
        refs.current[idx]?.focus();
        return;
      }
    }
  };

  const handleKeyDown = (idx: number, e: KeyboardEvent<HTMLButtonElement>): void => {
    if (isImeComposing(e.nativeEvent)) return;
    if (e.key === 'ArrowRight') {
      e.preventDefault();
      e.stopPropagation();
      move(idx, 1);
    } else if (e.key === 'ArrowLeft') {
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
        refs.current[first]?.focus();
      }
    } else if (e.key === 'End') {
      e.preventDefault();
      e.stopPropagation();
      for (let i = items.length - 1; i >= 0; i -= 1) {
        const lastItem = items[i];
        if (lastItem && !lastItem.disabled) {
          onChange(lastItem.value);
          refs.current[i]?.focus();
          break;
        }
      }
    }
  };

  return (
    <div className={`hx-tabs hx-tabs--${variant}`} role="tablist" aria-label={ariaLabel}>
      {items.map((it, idx) => {
        const current = idx === activeIdx;
        return (
          <button
            key={it.value}
            ref={(el) => {
              refs.current[idx] = el;
            }}
            type="button"
            role="tab"
            className={`hx-tabs__tab hx-focusring${current ? ' is-current' : ''}`}
            aria-selected={current ? 'true' : 'false'}
            aria-controls={panelIdOf ? panelIdOf(it.value) : undefined}
            disabled={it.disabled}
            tabIndex={idx === focusableIdx ? 0 : -1}
            onClick={() => {
              if (!it.disabled && !current) {
                onChange(it.value);
              }
            }}
            onKeyDown={(e) => {
              handleKeyDown(idx, e);
            }}
          >
            <span className="hx-tabs__label">{it.label}</span>
            {it.badge !== undefined ? <Badge tone="neutral" text={String(it.badge)} /> : null}
          </button>
        );
      })}
    </div>
  );
}
