import type { JSX, ReactNode, Ref } from 'react';
import './ScrollArea.css';

export interface ScrollAreaProps {
  readonly children: ReactNode;
  readonly gutter?: 'stable';
  readonly axis?: 'y' | 'x' | 'both';
  readonly className?: string;
  readonly ariaLabel?: string;
  readonly ref?: Ref<HTMLDivElement>;
}

export function ScrollArea(p: ScrollAreaProps): JSX.Element {
  const { children, gutter, axis = 'y', className, ariaLabel, ref } = p;
  const classes = [
    'hx-scroll',
    gutter === 'stable' ? 'hx-scroll--stable' : '',
    axis === 'x' ? 'hx-scroll--x' : axis === 'both' ? 'hx-scroll--both' : '',
    className ?? '',
  ]
    .filter(Boolean)
    .join(' ');

  return (
    <div
      ref={ref}
      className={classes}
      role={ariaLabel ? 'region' : undefined}
      aria-label={ariaLabel}
      tabIndex={ariaLabel ? 0 : undefined}
    >
      {children}
    </div>
  );
}
