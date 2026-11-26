import {
  useId,
  useLayoutEffect,
  useRef,
  type ChangeEvent,
  type JSX,
  type KeyboardEvent,
  type Ref,
} from 'react';
import { useT } from '@/lib/i18n';
import '../base.css';
import '../feedback/ScrollArea.css';
import { Icon } from '../icon/Icon';
import { Tooltip } from '../overlay/Tooltip';
import { isImeComposing } from '../types';
import { fitRows, measureRowMetrics, type RowMetrics } from './autosize';
import { FieldError } from './FieldError';
import type { FieldBase } from './TextInput';
import './TextArea.css';

export interface TextAreaProps extends FieldBase {
  readonly value: string;
  readonly onChange: (v: string) => void;
  readonly onCommit?: (v: string) => void;
  readonly onCancel?: () => void;
  readonly placeholder?: string;
  readonly autoFocus?: boolean;
  readonly minRows?: number;
  readonly maxRows?: number;
  readonly autosize?: boolean;
  readonly counter?: { readonly max: number };
  readonly onKeyDown?: (e: KeyboardEvent<HTMLTextAreaElement>) => void;
  readonly onCompositionStart?: () => void;
  readonly onCompositionEnd?: () => void;
  readonly inputRef?: Ref<HTMLTextAreaElement>;
}

function assignRef<T>(ref: Ref<T> | undefined, node: T | null): void {
  if (!ref) return;
  if (typeof ref === 'function') {
    ref(node);
  } else {
    (ref as { current: T | null }).current = node;
  }
}

export function TextArea(p: TextAreaProps): JSX.Element {
  const {
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
    minRows = 3,
    maxRows = 12,
    autosize = true,
    counter,
    onKeyDown,
    onCompositionStart,
    onCompositionEnd,
    inputRef,
  } = p;

  const t = useT();
  const autoId = useId();
  const errId = useId();
  const areaId = propId ?? autoId;
  const elRef = useRef<HTMLTextAreaElement | null>(null);
  const metricsRef = useRef<RowMetrics | null>(null);
  const skipBlurCommitRef = useRef(false);

  const softDisabled = disabled && Boolean(disabledReason);
  const nativeDisabled = disabled && !disabledReason;
  const isLocked = Boolean(lockReason);
  const effectiveReadOnly = readOnly || isLocked || softDisabled;
  const isInvalid = invalid || Boolean(error);

  useLayoutEffect(() => {
    if (!autosize || !elRef.current) return;
    metricsRef.current ??= measureRowMetrics(elRef.current);
    fitRows(elRef.current, minRows, maxRows, metricsRef.current);
  }, [autosize, value, minRows, maxRows]);

  const wrapClasses = [
    'hx-textarea',
    mono ? 'hx-textarea--mono' : '',
    isInvalid ? 'is-invalid' : '',
    softDisabled || nativeDisabled ? 'is-disabled' : '',
    isLocked ? 'is-locked' : '',
  ]
    .filter(Boolean)
    .join(' ');

  const handleChange = (e: ChangeEvent<HTMLTextAreaElement>): void => {
    if (effectiveReadOnly) return;
    onChange(e.target.value);
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>): void => {
    onKeyDown?.(e);
    if (e.defaultPrevented) return;
    if (isImeComposing(e.nativeEvent)) return;
    if (e.key === 'Escape' && onCancel) {
      e.stopPropagation();
      skipBlurCommitRef.current = true;
      onCancel();
      e.currentTarget.blur();
      return;
    }
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
      e.preventDefault();
      e.stopPropagation();
      if (!effectiveReadOnly) {
        onCommit?.(value);
      }
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

  const overLimit = counter !== undefined && value.length > counter.max;

  const box = (
    <div className={wrapClasses}>
      <textarea
        ref={(node) => {
          elRef.current = node;
          assignRef(inputRef, node);
        }}
        id={areaId}
        className="hx-textarea__el hx-scroll hx-focusring"
        rows={minRows}
        value={value}
        placeholder={placeholder}
        autoFocus={autoFocus}
        disabled={nativeDisabled}
        readOnly={effectiveReadOnly}
        aria-label={ariaLabel}
        aria-disabled={softDisabled ? 'true' : undefined}
        aria-invalid={isInvalid ? 'true' : undefined}
        aria-describedby={error ? errId : undefined}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        onBlur={handleBlur}
        onCompositionStart={onCompositionStart}
        onCompositionEnd={onCompositionEnd}
      />
      {isLocked ? (
        <span className="hx-textarea__lock">
          <Icon name="lock" size={14} label={t('ui.locked')} />
        </span>
      ) : null}
      {counter ? (
        <span className={`hx-textarea__counter${overLimit ? ' is-over' : ''}`}>
          {`${String(value.length)} / ${String(counter.max)}`}
        </span>
      ) : null}
    </div>
  );

  const tip = softDisabled ? disabledReason : lockReason;
  return (
    <div className="hx-textarea__field">
      {tip ? <Tooltip content={tip}>{box}</Tooltip> : box}
      {error ? <FieldError id={errId} text={error} /> : null}
    </div>
  );
}
