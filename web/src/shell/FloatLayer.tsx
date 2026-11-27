import { useEffect, useLayoutEffect, useRef, useState, type JSX, type ReactNode } from 'react';
import type { ShellLayout } from '@/state/uiStore';
import { computeFitInsets, shouldHideMinimap } from './layout';

export interface FloatLayerProps {
  readonly bp: ShellLayout['bp'];
  readonly inSubgraph?: boolean;
  readonly floatReplyOpen?: boolean;
  readonly dock?: ReactNode;
  readonly nestCard?: ReactNode;
  readonly minimap?: ReactNode;
  readonly floatReply?: ReactNode;
  readonly proposalBar?: ReactNode;
  readonly composer?: ReactNode;
  readonly onFitInsetsChange?: (insets: ShellLayout['fitInsets']) => void;
}

/** 画布区域内浮动层：停靠栏、父图位置卡、小地图、浮动回复、提议栏与底部对话框（W.02.02 §6.2）。 */
export function FloatLayer(p: FloatLayerProps): JSX.Element {
  const {
    bp,
    inSubgraph = false,
    floatReplyOpen = false,
    dock,
    nestCard,
    minimap,
    floatReply,
    proposalBar,
    composer,
    onFitInsetsChange,
  } = p;

  const minimapRef = useRef<HTMLDivElement | null>(null);
  const composerRef = useRef<HTMLDivElement | null>(null);
  const floatReplyRef = useRef<HTMLDivElement | null>(null);
  const onFitRef = useRef(onFitInsetsChange);
  const [minimapHidden, setMinimapHidden] = useState<boolean>(() => shouldHideMinimap(bp, null, null));

  useLayoutEffect(() => {
    onFitRef.current = onFitInsetsChange;
  });

  useEffect(() => {
    const measure = (): void => {
      const compEl = composerRef.current;
      const replyEl = floatReplyRef.current;
      const miniEl = minimapRef.current;

      const compRect = compEl ? compEl.getBoundingClientRect() : null;
      const miniRect = miniEl ? miniEl.getBoundingClientRect() : null;
      const replyRect = replyEl ? replyEl.getBoundingClientRect() : null;

      setMinimapHidden(shouldHideMinimap(bp, miniRect, compRect));

      const insets = computeFitInsets({
        inSubgraph,
        composerHeight: compRect?.height ?? 0,
        floatReplyOpen,
        floatReplyHeight: replyRect?.height ?? 0,
      });
      onFitRef.current?.(insets);
    };

    measure();

    if (typeof ResizeObserver === 'undefined') return undefined;
    let rafId: number | null = null;
    // K309 D11：ResizeObserver 回调经 rAF 合并，每帧最多测量一次
    const ro = new ResizeObserver(() => {
      if (rafId !== null) cancelAnimationFrame(rafId);
      rafId = requestAnimationFrame(() => {
        rafId = null;
        measure();
      });
    });
    if (composerRef.current) ro.observe(composerRef.current);
    if (minimapRef.current) ro.observe(minimapRef.current);
    if (floatReplyRef.current) ro.observe(floatReplyRef.current);
    return () => {
      ro.disconnect();
      if (rafId !== null) {
        cancelAnimationFrame(rafId);
        rafId = null;
      }
    };
  }, [bp, inSubgraph, floatReplyOpen, minimap, composer, floatReply]);

  return (
    <div className="hx-canvas__float">
      {dock ? <div className="hx-canvas__dock">{dock}</div> : null}
      {inSubgraph && nestCard ? <div className="hx-canvas__nest">{nestCard}</div> : null}
      {minimap ? (
        <div
          ref={minimapRef}
          className={`hx-canvas__minimap${minimapHidden ? ' is-hidden' : ''}`}
          aria-hidden={minimapHidden ? 'true' : undefined}
        >
          {minimap}
        </div>
      ) : null}
      <div className="hx-canvas__bottom">
        {floatReply ? (
          <div ref={floatReplyRef} className="hx-canvas__reply">
            {floatReply}
          </div>
        ) : null}
        {proposalBar ? <div className="hx-canvas__proposal">{proposalBar}</div> : null}
        {composer ? (
          <div ref={composerRef} className="hx-canvas__composer">
            {composer}
          </div>
        ) : null}
      </div>
    </div>
  );
}
