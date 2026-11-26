import { useEffect, useRef, type FocusEvent, type JSX } from 'react';
import { useT } from '@/lib/i18n';
import { Button } from '../button/Button';
import { IconButton } from '../button/IconButton';
import { Icon } from '../icon/Icon';
import type { IconName } from '../types';
import { Badge } from './Badge';
import './Toast.css';

export type ToastLevel = 'info' | 'ok' | 'wait' | 'error' | 'model';

export interface ToastAction {
  readonly label: string;
  readonly onClick: () => void;
}

export interface ToastProps {
  readonly level: ToastLevel;
  readonly text: string;
  readonly detail?: string;
  readonly actions?: readonly ToastAction[];
  readonly onClose: () => void;
  readonly glass?: boolean;
  readonly onHover?: (on: boolean) => void;
}

const LEVEL_ICON: Record<Exclude<ToastLevel, 'model'>, IconName> = {
  info: 'info',
  ok: 'check',
  wait: 'pause',
  error: 'failed',
};

const toastWarn = { tooMany: false };

export function Toast(p: ToastProps): JSX.Element {
  const { level, text, detail, actions = [], onClose, glass = false, onHover } = p;
  const t = useT();
  const rootRef = useRef<HTMLDivElement | null>(null);
  const pointerInsideRef = useRef(false);
  const focusInsideRef = useRef(false);
  const activeRef = useRef(false);

  const syncHover = (): void => {
    const next = pointerInsideRef.current || focusInsideRef.current;
    if (next !== activeRef.current) {
      activeRef.current = next;
      onHover?.(next);
    }
  };

  useEffect(() => {
    if (actions.length > 3 && import.meta.env.DEV && !toastWarn.tooMany) {
      toastWarn.tooMany = true;
      console.warn('[ui] Toast supports at most 3 actions; extra actions are ignored.');
    }
  }, [actions.length]);
  const visibleActions = actions.slice(0, 3);
  const inlineAction = visibleActions.length === 1;

  const classes = ['hx-toastcard', `hx-toastcard--${level}`, glass ? 'hx-toastcard--glass' : '']
    .filter(Boolean)
    .join(' ');

  return (
    <div
      ref={rootRef}
      className={classes}
      role={level === 'error' ? 'alert' : 'status'}
      onPointerEnter={() => {
        pointerInsideRef.current = true;
        syncHover();
      }}
      onPointerLeave={() => {
        pointerInsideRef.current = false;
        syncHover();
      }}
      onFocus={() => {
        focusInsideRef.current = true;
        syncHover();
      }}
      onBlur={(e: FocusEvent<HTMLDivElement>) => {
        const next = e.relatedTarget;
        if (next instanceof Node && rootRef.current?.contains(next)) return;
        focusInsideRef.current = false;
        syncHover();
      }}
    >
      <span className="hx-toastcard__lead">
        {level === 'model' ? (
          <Badge tone="model" text={t('ui.ai')} />
        ) : (
          <Icon name={LEVEL_ICON[level]} size={16} />
        )}
      </span>
      <div className="hx-toastcard__body">
        <div className="hx-toastcard__head">
          <div className="hx-toastcard__text">{text}</div>
          {inlineAction && visibleActions[0] ? (
            <Button variant="ghost" size="sm" onClick={visibleActions[0].onClick}>
              {visibleActions[0].label}
            </Button>
          ) : null}
        </div>
        {detail ? <div className="hx-toastcard__detail">{detail}</div> : null}
        {!inlineAction && visibleActions.length > 0 ? (
          <div className="hx-toastcard__actions">
            {visibleActions.map((act, idx) => (
              <Button key={`${act.label}-${String(idx)}`} variant="ghost" size="sm" onClick={act.onClick}>
                {act.label}
              </Button>
            ))}
          </div>
        ) : null}
      </div>
      <IconButton icon="x" size={24} tone="quiet" label={t('ui.close')} onClick={onClose} />
    </div>
  );
}
