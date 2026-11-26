import { useEffect, type JSX } from 'react';
import { Button } from '../button/Button';
import { Icon } from '../icon/Icon';
import type { IconName } from '../types';
import './EmptyState.css';

export interface EmptyStateAction {
  readonly label: string;
  readonly onClick: () => void;
  readonly variant?: 'secondary' | 'ghost' | 'model';
}

export interface EmptyStateProps {
  readonly icon?: IconName;
  readonly title: string;
  readonly desc?: string;
  readonly actions?: readonly EmptyStateAction[];
  readonly size?: 'panel' | 'page';
}

const emptyWarn = { tooMany: false };

export function EmptyState(p: EmptyStateProps): JSX.Element {
  const { icon, title, desc, actions = [], size = 'panel' } = p;
  useEffect(() => {
    if (actions.length > 2 && import.meta.env.DEV && !emptyWarn.tooMany) {
      emptyWarn.tooMany = true;
      console.warn('[ui] EmptyState supports at most 2 actions; extra actions are ignored.');
    }
  }, [actions.length]);
  const visibleActions = actions.slice(0, 2);

  return (
    <div className={`hx-empty hx-empty--${size}`}>
      {icon ? (
        <span className="hx-empty__icon" aria-hidden="true">
          <Icon name={icon} size={32} />
        </span>
      ) : null}
      <h3 className="hx-empty__title">{title}</h3>
      {desc ? <p className="hx-empty__desc">{desc}</p> : null}
      {visibleActions.length > 0 ? (
        <div className="hx-empty__actions">
          {visibleActions.map((act, idx) => (
            <Button
              key={`${act.label}-${String(idx)}`}
              variant={act.variant ?? 'secondary'}
              onClick={act.onClick}
            >
              {act.label}
            </Button>
          ))}
        </div>
      ) : null}
    </div>
  );
}
