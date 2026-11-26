import type { JSX } from 'react';
import './StatusDot.css';

export type DotState = 'idle' | 'running' | 'waiting' | 'failed' | 'done';

export interface StatusDotProps {
  readonly state: DotState;
  readonly label: string;
  readonly breathing?: boolean;
}

const dotWarn = { emptyLabel: false };

function logEmptyDotLabelOnce(): void {
  if (!dotWarn.emptyLabel) {
    dotWarn.emptyLabel = true;
    console.error('[ui] StatusDot label must not be empty; falling back to state.');
  }
}

export function StatusDot(p: StatusDotProps): JSX.Element {
  const { state, label, breathing = state === 'running' } = p;
  let ariaLabel = label.trim();
  if (ariaLabel === '') {
    if (import.meta.env.DEV) {
      throw new Error('StatusDot requires a non-empty label (U.16-R2).');
    }
    logEmptyDotLabelOnce();
    ariaLabel = state;
  }

  const classes = ['hx-dot', `hx-dot--${state}`, breathing && state === 'running' ? 'is-breathing' : '']
    .filter(Boolean)
    .join(' ');

  return <span className={classes} role="img" aria-label={ariaLabel} />;
}
