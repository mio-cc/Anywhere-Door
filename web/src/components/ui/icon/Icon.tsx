import { createElement, type JSX } from 'react';
import { ICONS, type IconName, type IconShape } from '@/styles/icons';
import { resolveIconName } from './iconSvg';
import './Icon.css';

export interface IconProps {
  readonly name: IconName | (string & {});
  readonly size?: 14 | 16 | 32;
  readonly label?: string;
}

function renderShape(sh: IconShape, idx: number, nonScaling: boolean): JSX.Element {
  const ve = nonScaling ? 'non-scaling-stroke' : undefined;
  switch (sh.t) {
    case 'path':
      return createElement('path', { key: idx, d: sh.d, vectorEffect: ve });
    case 'circle':
      return createElement('circle', { key: idx, cx: sh.cx, cy: sh.cy, r: sh.r, vectorEffect: ve });
    case 'rect':
      return createElement('rect', {
        key: idx,
        x: sh.x,
        y: sh.y,
        width: sh.w,
        height: sh.h,
        rx: sh.rx,
        vectorEffect: ve,
      });
    case 'line':
      return createElement('line', {
        key: idx,
        x1: sh.x1,
        y1: sh.y1,
        x2: sh.x2,
        y2: sh.y2,
        vectorEffect: ve,
      });
    case 'polyline':
      return createElement('polyline', { key: idx, points: sh.points, vectorEffect: ve });
  }
}

export function Icon(p: IconProps): JSX.Element {
  const { name, size = 16, label } = p;
  const resolved = resolveIconName(name);
  const def = ICONS[resolved];
  const nonScaling = size === 32 && !def.solid;

  return (
    <svg
      className={`hx-icon hx-icon--${String(size)}`}
      viewBox="0 0 16 16"
      fill={def.solid ? 'currentColor' : 'none'}
      stroke={def.solid ? 'none' : 'currentColor'}
      strokeWidth={def.solid ? undefined : 1.5}
      strokeLinecap={def.solid ? undefined : 'round'}
      strokeLinejoin={def.solid ? undefined : 'round'}
      role={label ? 'img' : undefined}
      aria-label={label}
      aria-hidden={label ? undefined : 'true'}
      focusable="false"
    >
      {def.shapes.map((sh, idx) => renderShape(sh, idx, nonScaling))}
    </svg>
  );
}
