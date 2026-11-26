import { useEffect, useLayoutEffect, useRef, type JSX, type PointerEvent, type ReactNode } from 'react';
import { useUiEnv } from '../overlay/UiEnv';
import { at, flipFrom, relBox, type Box } from './glide';
import './GlideGroup.css';

export interface GlideGroupProps {
  readonly itemSelector: string;
  readonly variant: 'row' | 'icon';
  readonly className?: string;
  readonly children: ReactNode;
}

function findScrollAncestor(el: HTMLElement | null): HTMLElement | null {
  let cur = el?.parentElement ?? null;
  while (cur) {
    const oy = getComputedStyle(cur).overflowY;
    if (oy === 'auto' || oy === 'scroll' || cur.classList.contains('hx-scroll')) {
      return cur;
    }
    cur = cur.parentElement;
  }
  return null;
}

export function GlideGroup(p: GlideGroupProps): JSX.Element {
  const { itemSelector, variant, className, children } = p;
  const env = useUiEnv();
  const groupRef = useRef<HTMLDivElement | null>(null);
  const blockRef = useRef<HTMLElement | null>(null);
  const prevBoxRef = useRef<Box | null>(null);
  const prevItemRef = useRef<Element | null>(null);
  const rafRef = useRef<number>(0);
  const posRef = useRef<{ x: number; y: number } | null>(null);
  const scrollElRef = useRef<HTMLElement | null>(null);
  const selectorBrokenRef = useRef(false);

  const hide = (): void => {
    const blk = blockRef.current;
    if (blk) {
      blk.classList.remove('is-on');
    }
    prevBoxRef.current = null;
    prevItemRef.current = null;
  };

  const applyDirect = (blk: HTMLElement, next: Box): void => {
    blk.classList.add('is-instant');
    blk.style.width = `${String(next.w)}px`;
    blk.style.height = `${String(next.h)}px`;
    blk.style.transform = at(next);
    blk.classList.add('is-on');
  };

  const pick = (directOnly: boolean): void => {
    rafRef.current = 0;
    if (selectorBrokenRef.current) return;
    const grp = groupRef.current;
    const blk = blockRef.current;
    const pos = posRef.current;
    if (!grp || !blk || !pos) {
      hide();
      return;
    }
    const hit = document.elementFromPoint(pos.x, pos.y);
    if (!hit) {
      hide();
      return;
    }
    let item: Element | null;
    try {
      item = hit.closest(itemSelector);
    } catch (err) {
      if (import.meta.env.DEV) {
        throw err;
      }
      selectorBrokenRef.current = true;
      console.error('[ui] GlideGroup invalid itemSelector:', itemSelector, err);
      hide();
      return;
    }
    if (
      !item ||
      !grp.contains(item) ||
      (item instanceof HTMLElement && item.offsetParent === null && item.getBoundingClientRect().width === 0)
    ) {
      hide();
      return;
    }

    const next = relBox(item.getBoundingClientRect(), grp.getBoundingClientRect());
    const prev = prevBoxRef.current;
    const isSame = prevItemRef.current === item && blk.classList.contains('is-on');

    if (directOnly || !prev || !blk.classList.contains('is-on') || env.reducedMotion() || isSame) {
      applyDirect(blk, next);
    } else {
      blk.classList.add('is-instant');
      blk.style.width = `${String(next.w)}px`;
      blk.style.height = `${String(next.h)}px`;
      blk.style.transform = flipFrom(prev, next);
      // 触发一次样式落盘，再以纯 transform 过渡到目标位置（W.08.01 §6.5）
      if (blk.offsetWidth < 0) return;
      blk.classList.remove('is-instant');
      blk.style.transform = at(next);
      blk.classList.add('is-on');
    }

    prevBoxRef.current = next;
    prevItemRef.current = item;
  };

  const detachScroll = (): void => {
    const sc = scrollElRef.current;
    if (sc) {
      sc.removeEventListener('scroll', onScrollRef.current);
      scrollElRef.current = null;
    }
  };

  const onScrollRef = useRef<() => void>(() => {
    // updated in layout effect
  });
  useLayoutEffect(() => {
    onScrollRef.current = () => {
      if (posRef.current) {
        pick(true);
      }
    };
  });

  useEffect(() => {
    return () => {
      if (rafRef.current !== 0) cancelAnimationFrame(rafRef.current);
      detachScroll();
    };
  }, []);

  const handlePointerMove = (e: PointerEvent<HTMLDivElement>): void => {
    posRef.current = { x: e.clientX, y: e.clientY };
    if (!scrollElRef.current && groupRef.current) {
      const sc = findScrollAncestor(groupRef.current);
      if (sc) {
        scrollElRef.current = sc;
        sc.addEventListener('scroll', onScrollRef.current, { passive: true });
      }
    }
    if (rafRef.current === 0) {
      rafRef.current = requestAnimationFrame(() => {
        pick(false);
      });
    }
  };

  const handlePointerLeave = (): void => {
    posRef.current = null;
    if (rafRef.current !== 0) {
      cancelAnimationFrame(rafRef.current);
      rafRef.current = 0;
    }
    detachScroll();
    hide();
  };

  const classes = ['hx-glide', `hx-glide--${variant}`, className ?? ''].filter(Boolean).join(' ');

  return (
    <div
      ref={groupRef}
      className={classes}
      onPointerMove={handlePointerMove}
      onPointerLeave={handlePointerLeave}
    >
      <i ref={blockRef} className="hx-glide__block" aria-hidden="true" />
      {children}
    </div>
  );
}
