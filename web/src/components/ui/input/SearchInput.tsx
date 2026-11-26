import { useRef, useState, type ChangeEvent, type JSX, type KeyboardEvent, type Ref } from 'react';
import { useT } from '@/lib/i18n';
import '../base.css';
import { Kbd } from '../button/Kbd';
import { Icon } from '../icon/Icon';
import { isImeComposing, type Size } from '../types';
import './SearchInput.css';

export interface SearchInputProps {
  readonly value: string;
  readonly onChange: (v: string) => void;
  readonly onClear: () => void;
  readonly inputRef?: Ref<HTMLInputElement>;
  readonly shortcut?: string;
  readonly placeholder?: string;
  readonly ariaLabel: string;
  readonly size?: Size;
  readonly autoFocus?: boolean;
  readonly onEscape?: 'clear' | 'none';
  readonly onKeyDown?: (e: KeyboardEvent<HTMLInputElement>) => void;
  readonly ariaControls?: string;
  readonly ariaActiveDescendant?: string;
}

function assignRef<T>(ref: Ref<T> | undefined, node: T | null): void {
  if (!ref) return;
  if (typeof ref === 'function') {
    ref(node);
  } else {
    (ref as { current: T | null }).current = node;
  }
}

export function SearchInput(p: SearchInputProps): JSX.Element {
  const {
    value,
    onChange,
    onClear,
    inputRef,
    shortcut,
    placeholder,
    ariaLabel,
    size = 'md',
    autoFocus,
    onEscape = 'clear',
    onKeyDown,
    ariaControls,
    ariaActiveDescendant,
  } = p;

  const t = useT();
  const localRef = useRef<HTMLInputElement | null>(null);
  const [focused, setFocused] = useState(false);

  const handleChange = (e: ChangeEvent<HTMLInputElement>): void => {
    onChange(e.target.value);
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>): void => {
    onKeyDown?.(e);
    if (e.defaultPrevented) return;
    if (isImeComposing(e.nativeEvent)) return;
    if (e.key === 'Escape') {
      if (onEscape === 'none') return;
      if (value !== '') {
        e.stopPropagation();
        onClear();
      } else {
        e.currentTarget.blur();
      }
    }
  };

  const classes = ['hx-search', 'hx-ctl', size === 'sm' ? 'hx-ctl--sm' : ''].filter(Boolean).join(' ');
  const isCombobox = Boolean(ariaControls);

  return (
    <div className={classes}>
      <span className="hx-search__icon" aria-hidden="true">
        <Icon name="search" size={14} />
      </span>
      <input
        ref={(node) => {
          localRef.current = node;
          assignRef(inputRef, node);
        }}
        type="text"
        className="hx-search__el hx-focusring"
        role={isCombobox ? 'combobox' : 'searchbox'}
        aria-label={ariaLabel}
        aria-expanded={isCombobox ? 'true' : undefined}
        aria-controls={ariaControls}
        aria-autocomplete={isCombobox ? 'list' : undefined}
        aria-activedescendant={ariaActiveDescendant}
        value={value}
        placeholder={placeholder}
        autoFocus={autoFocus}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        onFocus={() => {
          setFocused(true);
        }}
        onBlur={() => {
          setFocused(false);
        }}
      />
      {value !== '' ? (
        <button
          type="button"
          className="hx-search__clear"
          aria-label={t('ui.clear')}
          tabIndex={-1}
          onPointerDown={(e) => {
            e.preventDefault();
          }}
          onClick={() => {
            onClear();
            localRef.current?.focus();
          }}
        >
          <Icon name="x" size={14} />
        </button>
      ) : !focused && shortcut ? (
        <span className="hx-search__kbd" aria-hidden="true">
          <Kbd text={shortcut} variant="inline" />
        </span>
      ) : null}
    </div>
  );
}
