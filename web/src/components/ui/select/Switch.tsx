import { useId, type JSX, type KeyboardEvent } from 'react';
import { useT } from '@/lib/i18n';
import '../base.css';
import { Icon } from '../icon/Icon';
import { Tooltip } from '../overlay/Tooltip';
import type { Size } from '../types';
import './Switch.css';

export interface SwitchProps {
  readonly checked: boolean;
  readonly onChange: (v: boolean) => void;
  readonly label: string;
  readonly labelPosition?: 'left' | 'none';
  readonly size?: Size;
  readonly disabled?: boolean;
  readonly disabledReason?: string;
  readonly lockReason?: string;
}

export function Switch(p: SwitchProps): JSX.Element {
  const {
    checked,
    onChange,
    label,
    labelPosition = 'left',
    size = 'md',
    disabled = false,
    disabledReason,
    lockReason,
  } = p;

  const t = useT();
  const id = useId();
  const isLocked = Boolean(lockReason);
  const softDisabled = (disabled && Boolean(disabledReason)) || isLocked;
  const nativeDisabled = disabled && !disabledReason && !isLocked;
  const blocked = disabled || isLocked;

  const toggle = (): void => {
    if (blocked) return;
    onChange(!checked);
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLButtonElement>): void => {
    if (softDisabled && (e.key === ' ' || e.key === 'Enter')) {
      e.preventDefault();
    }
  };

  const track = (
    <button
      id={id}
      type="button"
      role="switch"
      className={`hx-switch__track hx-switch__track--${size} hx-focusring${checked ? ' is-checked' : ''}${softDisabled ? ' is-disabled' : ''}`}
      aria-checked={checked ? 'true' : 'false'}
      aria-label={label}
      aria-disabled={softDisabled ? 'true' : undefined}
      disabled={nativeDisabled}
      onClick={toggle}
      onKeyDown={handleKeyDown}
    >
      <span className="hx-switch__thumb" aria-hidden="true" />
    </button>
  );

  const body =
    labelPosition === 'none' ? (
      <span className="hx-switch hx-switch--bare">
        {track}
        {isLocked ? <Icon name="lock" size={14} label={t('ui.locked')} /> : null}
      </span>
    ) : (
      <div
        className={`hx-switch${blocked ? ' is-disabled' : ''}`}
        onClick={(e) => {
          if (e.target instanceof HTMLElement && e.target.closest('button')) return;
          toggle();
        }}
      >
        <span className="hx-switch__label">{label}</span>
        <span className="hx-switch__right">
          {isLocked ? <Icon name="lock" size={14} label={t('ui.locked')} /> : null}
          {track}
        </span>
      </div>
    );

  const tip = disabled && disabledReason ? disabledReason : lockReason;
  return tip ? <Tooltip content={tip}>{body}</Tooltip> : body;
}
