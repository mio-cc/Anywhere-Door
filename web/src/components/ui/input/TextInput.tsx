import { useId, useRef, type ChangeEvent, type JSX, type KeyboardEvent, type Ref } from 'react';
import { useT } from '@/lib/i18n';
import '../base.css';
import { Icon } from '../icon/Icon';
import { Tooltip } from '../overlay/Tooltip';
import { isImeComposing, type Size } from '../types';
import { FieldError } from './FieldError';
import './TextInput.css';

export interface FieldBase {
  readonly size?: Size;
  readonly id?: string;
  readonly ariaLabel?: string;
  readonly disabled?: boolean;
  readonly disabledReason?: string;
  readonly readOnly?: boolean;
  readonly lockReason?: string;
  readonly invalid?: boolean;
  readonly error?: string;
  readonly mono?: boolean;
}

export interface TextInputProps extends FieldBase {
  readonly value: string;
  readonly onChange: (v: string) => void;
  readonly onCommit?: (v: string) => void;
  readonly onCancel?: () => void;
  readonly placeholder?: string;
  readonly autoFocus?: boolean;
  readonly maxLength?: number;
  readonly inputRef?: Ref<HTMLInputElement>;
}

export function TextInput(p: TextInputProps): JSX.Element {
  const {
    size = 'md',
    id: propId,
    ariaLabel,
    disabled = false,
    disabledReason,
    readOnly = false,
    lockReason,
    invalid = false,
    error,
    mono = false,
    value,
    onChange,
    onCommit,
    onCancel,
    placeholder,
    autoFocus,
    maxLength,
    inputRef,
  } = p;

  const t = useT();
  const autoId = useId();
  const errId = useId();
  const inputId = propId ?? autoId;
  const skipBlurCommitRef = useRef(false);

  const softDisabled = disabled && Boolean(disabledReason);
  const nativeDisabled = disabled && !disabledReason;
  const isLocked = Boolean(lockReason);
  const effectiveReadOnly = readOnly || isLocked || softDisabled;
  const isInvalid = invalid || Boolean(error);

  const wrapClasses = [
    'hx-input',
    'hx-ctl',
    size === 'sm' ? 'hx-ctl--sm' : '',
    mono ? 'hx-input--mono' : '',
    isInvalid ? 'is-invalid' : '',
    softDisabled || nativeDisabled ? 'is-disabled' : '',
    isLocked ? 'is-locked' : '',
  ]
    .filter(Boolean)
    .join(' ');

  const handleChange = (e: ChangeEvent<HTMLInputElement>): void => {
    if (effectiveReadOnly) return;
    onChange(e.target.value);
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>): void => {
    if (isImeComposing(e.nativeEvent)) return;
    if (e.key === 'Enter') {
      e.preventDefault();
      e.stopPropagation();
      if (!effectiveReadOnly) {
        onCommit?.(value);
      }
      return;
    }
    if (e.key === 'Escape' && onCancel) {
      e.stopPropagation();
      skipBlurCommitRef.current = true;
      onCancel();
      e.currentTarget.blur();
    }
  };

  const handleBlur = (): void => {
    if (skipBlurCommitRef.current) {
      skipBlurCommitRef.current = false;
      return;
    }
    if (!effectiveReadOnly) {
      onCommit?.(value);
    }
  };

  const field = (
    <div className={wrapClasses}>
      <input
        ref={inputRef}
        id={inputId}
        type="text"
        className="hx-input__el hx-focusring"
        value={value}
        placeholder={placeholder}
        autoFocus={autoFocus}
        maxLength={maxLength}
        disabled={nativeDisabled}
        readOnly={effectiveReadOnly}
        aria-label={ariaLabel}
        aria-disabled={softDisabled ? 'true' : undefined}
        aria-invalid={isInvalid ? 'true' : undefined}
        aria-describedby={error ? errId : undefined}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        onBlur={handleBlur}
      />
      {isLocked ? (
        <span className="hx-input__lock">
          <Icon name="lock" size={14} label={t('ui.locked')} />
        </span>
      ) : null}
    </div>
  );

  const tip = softDisabled ? disabledReason : lockReason;
  return (
    <div className="hx-input__field">
      {tip ? <Tooltip content={tip}>{field}</Tooltip> : field}
      {error ? <FieldError id={errId} text={error} /> : null}
    </div>
  );
}
