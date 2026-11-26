import type { JSX, KeyboardEvent, MouseEvent, Ref } from 'react';
import '../base.css';
import { Icon } from '../icon/Icon';
import { Tooltip } from '../overlay/Tooltip';
import type { IconName, Placement } from '../types';
import './IconButton.css';

export interface IconButtonProps {
  readonly icon: IconName;
  readonly label: string;
  readonly shortcut?: string;
  readonly size?: 28 | 24;
  readonly tone?: 'default' | 'quiet';
  readonly pressed?: boolean;
  readonly dot?: 'accent' | 'danger' | 'wait' | 'model' | 'focus';
  readonly disabled?: boolean;
  readonly disabledReason?: string;
  readonly onClick?: (e: MouseEvent<HTMLButtonElement>) => void;
  readonly tipPlacement?: Placement;
  readonly ariaHasPopup?: 'menu' | 'dialog' | 'listbox';
  readonly ariaExpanded?: boolean;
  readonly ref?: Ref<HTMLButtonElement>;
}

const iconBtnWarn = { emptyLabel: false };

function logEmptyIconBtnLabelOnce(): void {
  if (!iconBtnWarn.emptyLabel) {
    iconBtnWarn.emptyLabel = true;
    console.error('[ui] IconButton label is empty; falling back to icon name.');
  }
}

export function IconButton(p: IconButtonProps): JSX.Element {
  const {
    icon,
    label,
    shortcut,
    size = 28,
    tone = 'default',
    pressed,
    dot,
    disabled = false,
    disabledReason,
    onClick,
    tipPlacement = 'bottom',
    ariaHasPopup,
    ariaExpanded,
    ref,
  } = p;

  let resolvedLabel = label.trim();
  if (resolvedLabel === '') {
    if (import.meta.env.DEV) {
      throw new Error('IconButton requires a non-empty label (U.08-R5).');
    }
    logEmptyIconBtnLabelOnce();
    resolvedLabel = icon;
  }

  const softDisabled = disabled && Boolean(disabledReason);
  const nativeDisabled = disabled && !disabledReason;

  const classes = [
    'hx-iconbtn',
    `hx-iconbtn--${String(size)}`,
    tone === 'quiet' ? 'hx-iconbtn--quiet' : '',
    pressed ? 'is-pressed' : '',
    softDisabled ? 'is-disabled' : '',
    'hx-focusring',
  ]
    .filter(Boolean)
    .join(' ');

  const tipContent =
    softDisabled && disabledReason ? (
      <span className="hx-iconbtn__tip">
        <span>{resolvedLabel}</span>
        <span className="hx-iconbtn__tipreason">{disabledReason}</span>
      </span>
    ) : (
      resolvedLabel
    );

  const handleClick = (e: MouseEvent<HTMLButtonElement>): void => {
    if (disabled) {
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

  return (
    <Tooltip content={tipContent} {...(shortcut !== undefined ? { shortcut } : {})} placement={tipPlacement}>
      <button
        ref={ref}
        type="button"
        className={classes}
        disabled={nativeDisabled}
        aria-disabled={softDisabled ? 'true' : undefined}
        aria-label={resolvedLabel}
        aria-pressed={pressed === undefined ? undefined : pressed ? 'true' : 'false'}
        aria-haspopup={ariaHasPopup}
        aria-expanded={ariaExpanded}
        onClick={handleClick}
        onKeyDown={handleKeyDown}
      >
        <Icon name={icon} size={size === 24 ? 14 : 16} />
        {dot ? <span className={`hx-iconbtn__dot hx-iconbtn__dot--${dot}`} aria-hidden="true" /> : null}
      </button>
    </Tooltip>
  );
}
