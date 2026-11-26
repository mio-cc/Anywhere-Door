import type { JSX } from 'react';
import type { Tone } from '../types';
import './Badge.css';

export interface BadgeProps {
  readonly tone: Tone;
  readonly text: string;
  readonly dot?: boolean;
}

export function Badge(p: BadgeProps): JSX.Element {
  const { tone, text, dot = false } = p;
  return (
    <span className={`hx-badge hx-badge--${tone}`}>
      {dot ? <span className="hx-badge__dot" aria-hidden="true" /> : null}
      <span className="hx-badge__text">{text}</span>
    </span>
  );
}
