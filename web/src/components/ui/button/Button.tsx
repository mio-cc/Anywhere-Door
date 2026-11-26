import {
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type JSX,
  type KeyboardEvent,
  type MouseEvent,
  type ReactNode,
  type Ref,
} from 'react';
import { useT } from '@/lib/i18n';
import '../base.css';
import { Spinner } from '../feedback/Spinner';
import { Icon } from '../icon/Icon';
import { Tooltip } from '../overlay/Tooltip';
import type { IconName, Size } from '../types';
import './Button.css';

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'model';

export interface ButtonProps {
  readonly variant?: ButtonVariant;
  readonly size?: Size;
  readonly icon?: IconName;
  readonly children: ReactNode;
  readonly loading?: boolean;
  readonly disabled?: boolean;
  readonly disabledReason?: string;
  readonly onClick?: (e: MouseEvent<HTMLButtonElement>) => void;
  readonly type?: 'button' | 'submit';
  readonly width?: 'auto' | 'full';
  readonly ariaLabel?: string;
  readonly autoFocus?: boolean;
  readonly ref?: Ref<HTMLButtonElement>;
}

function assignRef<T>(ref: Ref<T> | undefined, node: T | null): void {
  if (!ref) return;
  if (typeof ref === 'function') {
    ref(node);
  } else {
    (ref as { current: T | null }).current = node;
  }
}

export function Button(p: ButtonProps): JSX.Element {
  const {
    variant = 'secondary',
    size = 'md',
    icon,
    children,
    loading = false,
    disabled = false,
    disabledReason,
    onClick,
    type = 'button',
    width = 'auto',
    ariaLabel,
    autoFocus = false,
    ref,
  } = p;

  const t = useT();
  const btnRef = useRef<HTMLButtonElement | null>(null);
  const lastWidthRef = useRef<number | null>(null);
  const [lockedWidth, setLockedWidth] = useState<number | null>(null);

  useLayoutEffect(() => {
    const el = btnRef.current;
    if (!el) return;
    if (loading) {
      const measured = el.offsetWidth > 0 ? el.offsetWidth : lastWidthRef.current;
      if (measured && measured > 0) {
        setLockedWidth(measured);
      }
    } else {
      if (el.offsetWidth > 0) {
        lastWidthRef.current = el.offsetWidth;
      }
      setLockedWidth(null);
    }
  }, [loading, children, size]);

  const softDisabled = disabled && Boolean(disabledReason);
  const nativeDisabled = disabled && !disabledReason;
  const blocked = disabled || loading;

  const classes = [
    'hx-btn',
    `hx-btn--${variant}`,
    'hx-ctl',
    size === 'sm' ? 'hx-ctl--sm' : '',
    width === 'full' ? 'hx-btn--full' : '',
    loading ? 'is-loading' : '',
    softDisabled ? 'is-disabled' : '',
    'hx-focusring',
  ]
    .filter(Boolean)
    .join(' ');

  const style = lockedWidth !== null ? ({ '--_w': `${String(lockedWidth)}px` } as CSSProperties) : undefined;

  const handleClick = (e: MouseEvent<HTMLButtonElement>): void => {
    if (blocked) {
      e.preventDefault();
      return;
    }
    onClick?.(e);
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLButtonElement>): void => {
    if (softDisabled && (e.key === 'Enter' || e.key === ' ')) {
      e.preventDefault();
    }
  };

  const buttonEl = (
    <button
      ref={(node) => {
        btnRef.current = node;
        assignRef(ref, node);
      }}
      type={type}
      className={classes}
      style={style}
      disabled={nativeDisabled}
      aria-disabled={softDisabled ? 'true' : undefined}
      aria-busy={loading ? 'true' : undefined}
      aria-label={loading ? (ariaLabel ?? t('ui.loading')) : ariaLabel}
      data-autofocus={autoFocus ? '' : undefined}
      autoFocus={autoFocus}
      onClick={handleClick}
      onKeyDown={handleKeyDown}
    >
      <span className="hx-btn__content">
        {icon ? <Icon name={icon} size={size === 'sm' ? 14 : 16} /> : null}
        <span className="hx-btn__label">{children}</span>
      </span>
      {loading ? (
        <span className="hx-btn__spinner">
          <Spinner size={size === 'sm' ? 12 : 16} label={null} />
        </span>
      ) : null}
    </button>
  );

  if (softDisabled && disabledReason) {
    return <Tooltip content={disabledReason}>{buttonEl}</Tooltip>;
  }
  return buttonEl;
}
