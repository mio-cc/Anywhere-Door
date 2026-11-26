import { useEffect, type CSSProperties, type JSX } from 'react';
import './ProgressBar.css';

export interface ProgressBarProps {
  readonly value: number;
  readonly tone?: 'accent' | 'ok' | 'danger';
  readonly ariaLabel: string;
}

const progressWarn = { invalid: false };

export function ProgressBar(p: ProgressBarProps): JSX.Element {
  const { value, tone = 'accent', ariaLabel } = p;
  const finite = Number.isFinite(value);
  useEffect(() => {
    if (!finite && import.meta.env.DEV && !progressWarn.invalid) {
      progressWarn.invalid = true;
      console.warn('[ui] ProgressBar value is not finite; clamping to 0.');
    }
  }, [finite]);
  const clamped = finite ? Math.min(1, Math.max(0, value)) : 0;

  const pct = Math.round(clamped * 100);
  const fillStyle = { '--_v': String(clamped) } as CSSProperties;

  return (
    <div
      className={`hx-progress hx-progress--${tone}`}
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={pct}
      aria-label={ariaLabel}
    >
      <div className="hx-progress__fill" style={fillStyle} />
    </div>
  );
}
