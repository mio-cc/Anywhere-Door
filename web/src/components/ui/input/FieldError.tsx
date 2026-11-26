import type { JSX } from 'react';
import './FieldError.css';

export interface FieldErrorProps {
  readonly id: string;
  readonly text: string;
}

export function FieldError(p: FieldErrorProps): JSX.Element {
  return (
    <p id={p.id} className="hx-ferr" role="alert">
      {p.text}
    </p>
  );
}
