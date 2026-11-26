import type { JSX } from 'react';
import { useNow } from './clock';
import { fmtAbsTime, fmtRelTime } from './format';

export interface RelTimeProps {
  iso: string;
  className?: string;
}

export function RelTime({ iso, className }: RelTimeProps): JSX.Element {
  const now = useNow();
  const title = fmtAbsTime(iso, { zone: true });
  const text = fmtRelTime(iso, now);
  return className !== undefined ? (
    <time dateTime={iso} title={title} className={className}>
      {text}
    </time>
  ) : (
    <time dateTime={iso} title={title}>
      {text}
    </time>
  );
}
