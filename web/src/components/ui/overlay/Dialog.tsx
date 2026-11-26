import { useId, useLayoutEffect, useRef, type JSX, type ReactNode } from 'react';
import { Button, type ButtonVariant } from '../button/Button';
import { FieldError } from '../input/FieldError';
import type { CloseReason } from '../types';
import { focusables } from './focus';
import { Overlay, type OverlayProps } from './Overlay';
import './Dialog.css';

export interface DialogAction {
  readonly label: string;
  readonly variant: ButtonVariant;
  readonly onClick: () => void;
  readonly autoFocus?: boolean;
  readonly disabled?: boolean;
  readonly loading?: boolean;
}

export interface DialogProps {
  readonly title: string;
  readonly width: 400 | 560;
  readonly open: boolean;
  readonly onClose: (reason: CloseReason) => void;
  readonly actions?: readonly DialogAction[];
  readonly dismissible?: boolean;
  readonly busy?: boolean;
  readonly error?: string;
  readonly restoreFocus?: OverlayProps['restoreFocus'];
  readonly children: ReactNode;
}

export function Dialog(p: DialogProps): JSX.Element | null {
  const {
    title,
    width,
    open,
    onClose,
    actions = [],
    dismissible = true,
    busy = false,
    error,
    restoreFocus,
    children,
  } = p;

  const titleId = useId();
  const bodyId = useId();
  const errId = useId();
  const contentRef = useRef<HTMLDivElement | null>(null);
  const bodyRef = useRef<HTMLDivElement | null>(null);

  // U.11 §2：danger 变体的按钮稳定排在最右
  const nonDanger = actions.filter((a) => a.variant !== 'danger');
  const danger = actions.filter((a) => a.variant === 'danger');
  const ordered = [...nonDanger, ...danger];
  const hasDanger = danger.length > 0;

  // W.08.04 §6.4：显式 autoFocus 按钮 → 第一个非 danger 按钮 → 正文第一个可聚焦元素 → 面板本身（绝不默认聚焦危险按钮）
  const explicitAuto = ordered.findIndex((a) => a.autoFocus);
  const defaultFocusIdx = explicitAuto >= 0 ? explicitAuto : ordered.findIndex((a) => a.variant !== 'danger');
  const hasButtonFocus = defaultFocusIdx >= 0;

  useLayoutEffect(() => {
    if (!open || hasButtonFocus) return;
    const bodyFirst = bodyRef.current ? focusables(bodyRef.current)[0] : undefined;
    if (bodyFirst) {
      bodyFirst.focus({ preventScroll: true });
    } else {
      contentRef.current?.focus({ preventScroll: true });
    }
  }, [open, hasButtonFocus]);

  const handleClose = (reason: CloseReason): void => {
    if (!dismissible && (reason === 'esc' || reason === 'outside')) {
      return;
    }
    onClose(reason);
  };

  return (
    <Overlay
      layer="modal"
      open={open}
      {...(dismissible ? { onClose: handleClose } : {})}
      onKey={(e) => {
        // dismissible=false 时仍吞掉 Esc，不落到下层（W.08.04 §6.4）
        if (e.key === 'Escape') return true;
        return false;
      }}
      scrim
      trapFocus
      closeOnOutside={dismissible}
      initialFocus={hasButtonFocus ? 'first' : 'container'}
      {...(restoreFocus !== undefined ? { restoreFocus } : {})}
      role={hasDanger ? 'alertdialog' : 'dialog'}
      ariaLabelledBy={titleId}
      ariaDescribedBy={bodyId}
      className="hx-dialog__wrap"
      contentRef={contentRef}
    >
      <div
        className="hx-dialog"
        style={{ width: `${String(width)}px` }}
        aria-busy={busy ? 'true' : undefined}
      >
        <h2 id={titleId} className="hx-dialog__title">
          {title}
        </h2>
        <div ref={bodyRef} id={bodyId} className="hx-dialog__body">
          {children}
        </div>
        {error ? <FieldError id={errId} text={error} /> : null}
        {ordered.length > 0 ? (
          <div className="hx-dialog__actions">
            {ordered.map((act, idx) => (
              <Button
                key={`${act.label}-${String(idx)}`}
                variant={act.variant}
                disabled={busy || Boolean(act.disabled)}
                loading={Boolean(act.loading)}
                autoFocus={idx === defaultFocusIdx}
                onClick={act.onClick}
              >
                {act.label}
              </Button>
            ))}
          </div>
        ) : null}
      </div>
    </Overlay>
  );
}
