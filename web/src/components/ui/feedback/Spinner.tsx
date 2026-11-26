import { useId, type JSX } from 'react';
import { useT } from '@/lib/i18n';
import './Spinner.css';

export interface SpinnerProps {
  readonly size: 12 | 16;
  readonly label?: string | null;
}

export function Spinner(p: SpinnerProps): JSX.Element {
  const { size, label } = p;
  const t = useT();
  const id = useId();
  const isDecorative = label === null;
  const srText = label === undefined ? t('ui.loading') : label;

  return (
    <span
      className={`hx-spin hx-spin--${String(size)}`}
      role={isDecorative ? undefined : 'status'}
      aria-hidden={isDecorative ? 'true' : undefined}
      aria-labelledby={isDecorative ? undefined : id}
    >
      <svg
        className="hx-spin__svg"
        viewBox="0 0 16 16"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        aria-hidden="true"
        focusable="false"
      >
        <circle className="hx-spin__arc" cx="8" cy="8" r="5.6" strokeDasharray="21.1 35.2" />
      </svg>
      {isDecorative ? null : (
        <span id={id} className="hx-spin__sr">
          {srText}
        </span>
      )}
    </span>
  );
}
