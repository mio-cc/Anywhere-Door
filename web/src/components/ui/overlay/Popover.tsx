import { useLayoutEffect, useRef, type JSX, type ReactNode } from 'react';
import type { Anchor, CloseReason, Placement } from '../types';
import { Overlay, type OverlayProps } from './Overlay';
import { place, VIEWPORT_PADDING } from './position';
import './Popover.css';

export interface PopoverProps {
  readonly anchor: Anchor;
  readonly placement?: Placement;
  readonly offset?: number;
  readonly open: boolean;
  readonly onClose: (reason: CloseReason) => void;
  readonly width?: number | 'anchor';
  readonly padding?: 'none' | 'sm' | 'md';
  readonly glass?: boolean;
  readonly initialFocus?: OverlayProps['initialFocus'];
  readonly restoreFocus?: OverlayProps['restoreFocus'];
  readonly onKey?: OverlayProps['onKey'];
  readonly role?: 'dialog' | 'listbox' | 'menu';
  readonly ariaLabel: string;
  readonly ariaActiveDescendant?: string;
  readonly className?: string;
  readonly children: ReactNode;
}

export function Popover(p: PopoverProps): JSX.Element | null {
  const {
    anchor,
    placement = 'bottom-start',
    offset = 4,
    open,
    onClose,
    width,
    padding = 'md',
    glass = false,
    initialFocus,
    restoreFocus,
    onKey,
    role = 'dialog',
    ariaLabel,
    ariaActiveDescendant,
    className,
    children,
  } = p;

  const contentRef = useRef<HTMLDivElement | null>(null);
  const onCloseRef = useRef(onClose);

  useLayoutEffect(() => {
    onCloseRef.current = onClose;
  });

  useLayoutEffect(() => {
    if (!open || !contentRef.current) return;
    const el = contentRef.current;
    if (typeof width === 'number') {
      el.style.width = `${String(width)}px`;
    } else if (width === undefined) {
      el.style.removeProperty('width');
    }
    return place(anchor, el, {
      placement,
      offset,
      padding: VIEWPORT_PADDING,
      matchWidth: width === 'anchor',
      onAnchorGone: () => {
        onCloseRef.current('outside');
      },
    });
  }, [open, anchor, placement, offset, width]);

  const classes = ['hx-pop', `hx-pop--pad-${padding}`, glass ? 'hx-pop--glass' : '', className ?? '']
    .filter(Boolean)
    .join(' ');

  return (
    <Overlay
      layer="popover"
      open={open}
      onClose={onClose}
      closeOnOutside
      anchorEl={anchor instanceof HTMLElement ? anchor : null}
      {...(initialFocus !== undefined ? { initialFocus } : {})}
      {...(restoreFocus !== undefined ? { restoreFocus } : {})}
      {...(onKey !== undefined ? { onKey } : {})}
      role={role}
      ariaLabel={ariaLabel}
      {...(ariaActiveDescendant !== undefined ? { ariaActiveDescendant } : {})}
      className={classes}
      contentRef={contentRef}
    >
      {children}
    </Overlay>
  );
}
