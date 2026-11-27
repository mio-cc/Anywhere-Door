import {
  createContext,
  useCallback,
  useContext,
  useRef,
  type JSX,
  type PointerEventHandler,
  type ReactNode,
} from 'react';
import type { ShellLayout } from '@/state/uiStore';
import { FloatLayer, type FloatLayerProps } from './FloatLayer';
import './CanvasArea.css';

const CanvasAreaRectContext = createContext<() => DOMRect | null>(() => null);

/** 读取画布区域页面矩形（W.02.02 §4.1、P.09 K111）。 */
export function useCanvasAreaRect(): () => DOMRect | null {
  return useContext(CanvasAreaRectContext);
}

export interface CanvasAreaProps {
  readonly mode: 'session' | 'page';
  readonly bp: ShellLayout['bp'];
  readonly canvas?: ReactNode;
  readonly page?: ReactNode;
  readonly canvasOverlay?: ReactNode;
  readonly cover?: ReactNode;
  readonly floatProps?: Omit<FloatLayerProps, 'bp'>;
  readonly onCanvasPointerDown?: PointerEventHandler<HTMLDivElement>;
}

export function CanvasArea(p: CanvasAreaProps): JSX.Element {
  const { mode, bp, canvas, page, canvasOverlay, cover, floatProps, onCanvasPointerDown } = p;
  const rootRef = useRef<HTMLDivElement | null>(null);

  const getRect = useCallback((): DOMRect | null => {
    return rootRef.current ? rootRef.current.getBoundingClientRect() : null;
  }, []);

  const hasCover = mode === 'session' && cover !== undefined && cover !== null;

  return (
    <CanvasAreaRectContext.Provider value={getRect}>
      <main ref={rootRef} className="hx-canvas" onPointerDown={onCanvasPointerDown}>
        {mode === 'session' ? (
          <>
            <div className="hx-canvas__stage">{canvas}</div>
            {canvasOverlay ? <div className="hx-canvas__overlay">{canvasOverlay}</div> : null}
            {!hasCover ? <FloatLayer bp={bp} {...floatProps} /> : null}
            {hasCover ? <div className="hx-canvas__cover">{cover}</div> : null}
          </>
        ) : (
          <div className="hx-canvas__page">{page}</div>
        )}
      </main>
    </CanvasAreaRectContext.Provider>
  );
}
