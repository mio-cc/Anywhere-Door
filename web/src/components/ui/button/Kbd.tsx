import type { JSX } from 'react';
import './Kbd.css';

export interface KbdProps {
  readonly text: string;
  readonly variant?: 'inline' | 'key' | 'tip';
}

export function Kbd(p: KbdProps): JSX.Element {
  const { text, variant = 'inline' } = p;
  return <kbd className={`hx-kbd hx-kbd--${variant}`}>{text}</kbd>;
}
