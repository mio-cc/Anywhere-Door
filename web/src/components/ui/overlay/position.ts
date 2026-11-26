import {
  autoUpdate,
  computePosition,
  flip,
  offset as offsetMw,
  shift,
  size as sizeMw,
  type Middleware,
  type Placement as FloatingPlacement,
  type ReferenceElement,
} from '@floating-ui/dom';
import type { Anchor, Placement } from '../types';

export const VIEWPORT_PADDING = 8;

export interface PlaceOptions {
  readonly placement: Placement;
  readonly offset: number;
  readonly padding: number;
  readonly matchWidth?: boolean;
  readonly onAnchorGone?: () => void;
}

function toReference(anchor: Anchor): ReferenceElement {
  if (anchor instanceof HTMLElement) {
    return anchor;
  }
  return {
    getBoundingClientRect() {
      return anchor;
    },
  };
}

function isZeroSize(r: Pick<DOMRect, 'width' | 'height'>): boolean {
  return r.width === 0 && r.height === 0;
}

/** 同步计算首帧近似位置，避免在 computePosition Promise 落地前闪到 (0,0)（评审 #4）。 */
function syncInitialPosition(
  refRect: Pick<DOMRect, 'left' | 'top' | 'right' | 'bottom' | 'width' | 'height'>,
  floatRect: Pick<DOMRect, 'width' | 'height'>,
  floating: HTMLElement,
  placement: Placement,
  offset: number,
  matchWidth: boolean,
): void {
  if (matchWidth && refRect.width > 0) {
    floating.style.width = `${String(Math.round(refRect.width))}px`;
  }
  const fw = floatRect.width || refRect.width;
  const fh = floatRect.height;
  let x: number;
  let y: number;

  if (placement === 'top' || placement === 'top-start' || placement === 'top-end') {
    y = refRect.top - fh - offset;
    x =
      placement === 'top-end'
        ? refRect.right - fw
        : placement === 'top'
          ? refRect.left + (refRect.width - fw) / 2
          : refRect.left;
  } else if (placement === 'bottom' || placement === 'bottom-start' || placement === 'bottom-end') {
    y = refRect.bottom + offset;
    x =
      placement === 'bottom-end'
        ? refRect.right - fw
        : placement === 'bottom'
          ? refRect.left + (refRect.width - fw) / 2
          : refRect.left;
  } else if (placement === 'right' || placement === 'right-start') {
    x = refRect.right + offset;
    y = placement === 'right' ? refRect.top + (refRect.height - fh) / 2 : refRect.top;
  } else {
    x = refRect.left - fw - offset;
    y = placement === 'left' ? refRect.top + (refRect.height - fh) / 2 : refRect.top;
  }

  floating.style.position = 'fixed';
  floating.style.left = `${String(Math.round(x))}px`;
  floating.style.top = `${String(Math.round(y))}px`;
  floating.setAttribute('data-placement', placement);
}

/** 唯一 import @floating-ui/dom 的定位模块（W.08.04 §4.1、§6.2）。 */
export function place(anchor: Anchor, floating: HTMLElement, o: PlaceOptions): () => void {
  const reference = toReference(anchor);
  const middleware: Middleware[] = [
    offsetMw(o.offset),
    flip({ padding: o.padding }),
    shift({ padding: o.padding }),
  ];
  if (o.matchWidth) {
    middleware.push(
      sizeMw({
        padding: o.padding,
        apply({ rects, elements }) {
          elements.floating.style.width = `${String(Math.round(rects.reference.width))}px`;
        },
      }),
    );
  }

  let disposed = false;
  let retryRaf = 0;

  const applyPos = (retried: boolean): void => {
    if (disposed) return;
    if (anchor instanceof HTMLElement && !anchor.isConnected) {
      o.onAnchorGone?.();
      return;
    }
    const refRect = reference.getBoundingClientRect();
    const floatRect = floating.getBoundingClientRect();
    // 浮层或元素锚点任一尺寸为 0 时：仍立即做同步首帧定位并计算一次，同时下一帧重试一次（W.08.04 §8）
    if (!retried && (isZeroSize(floatRect) || (anchor instanceof HTMLElement && isZeroSize(refRect)))) {
      retryRaf = requestAnimationFrame(() => {
        applyPos(true);
      });
    }
    syncInitialPosition(refRect, floatRect, floating, o.placement, o.offset, Boolean(o.matchWidth));
    void computePosition(reference, floating, {
      strategy: 'fixed',
      placement: o.placement as FloatingPlacement,
      middleware,
    })
      .then((res) => {
        if (disposed) return;
        floating.style.position = 'fixed';
        floating.style.left = `${String(Math.round(res.x))}px`;
        floating.style.top = `${String(Math.round(res.y))}px`;
        floating.setAttribute('data-placement', res.placement);
      })
      .catch(() => {
        if (disposed) return;
        floating.style.position = 'fixed';
        floating.style.left = `${String(Math.round(refRect.left))}px`;
        floating.style.top = `${String(Math.round(refRect.top))}px`;
      });
  };

  const stopAuto = autoUpdate(reference, floating, () => {
    applyPos(false);
  });

  return () => {
    disposed = true;
    if (retryRaf !== 0) cancelAnimationFrame(retryRaf);
    stopAuto();
  };
}
