import { ICON_FALLBACK, ICONS, type IconName, type IconShape } from '@/styles/icons';

const SVG_NS = 'http://www.w3.org/2000/svg';
const templateCache = new Map<IconName, SVGSVGElement>();
const warnedUnknown = new Set<string>();

export function hasIcon(name: string): name is IconName {
  return Object.prototype.hasOwnProperty.call(ICONS, name);
}

export function resolveIconName(name: string): IconName {
  if (hasIcon(name)) return name;
  if (import.meta.env.DEV && !warnedUnknown.has(name)) {
    warnedUnknown.add(name);
    console.warn(`[ui] Unknown icon name "${name}", falling back to "${ICON_FALLBACK}".`);
  }
  return ICON_FALLBACK;
}

function buildShapeNode(sh: IconShape): SVGElement {
  const el = document.createElementNS(SVG_NS, sh.t);
  switch (sh.t) {
    case 'path':
      el.setAttribute('d', sh.d);
      break;
    case 'circle':
      el.setAttribute('cx', String(sh.cx));
      el.setAttribute('cy', String(sh.cy));
      el.setAttribute('r', String(sh.r));
      break;
    case 'rect':
      el.setAttribute('x', String(sh.x));
      el.setAttribute('y', String(sh.y));
      el.setAttribute('width', String(sh.w));
      el.setAttribute('height', String(sh.h));
      if (sh.rx !== undefined) el.setAttribute('rx', String(sh.rx));
      break;
    case 'line':
      el.setAttribute('x1', String(sh.x1));
      el.setAttribute('y1', String(sh.y1));
      el.setAttribute('x2', String(sh.x2));
      el.setAttribute('y2', String(sh.y2));
      break;
    case 'polyline':
      el.setAttribute('points', sh.points);
      break;
  }
  return el;
}

function getTemplate(name: IconName): SVGSVGElement {
  const cached = templateCache.get(name);
  if (cached) return cached;
  const def = ICONS[name];
  const svg = document.createElementNS(SVG_NS, 'svg');
  svg.setAttribute('viewBox', '0 0 16 16');
  svg.setAttribute('aria-hidden', 'true');
  svg.setAttribute('focusable', 'false');
  if (def.solid) {
    svg.setAttribute('fill', 'currentColor');
    svg.setAttribute('stroke', 'none');
  } else {
    svg.setAttribute('fill', 'none');
    svg.setAttribute('stroke', 'currentColor');
    svg.setAttribute('stroke-width', '1.5');
    svg.setAttribute('stroke-linecap', 'round');
    svg.setAttribute('stroke-linejoin', 'round');
  }
  for (const sh of def.shapes) {
    svg.appendChild(buildShapeNode(sh));
  }
  templateCache.set(name, svg);
  return svg;
}

/** 非 React SVG 元素构造器（给 W.06 画布引擎的 EngineDeps.icon 使用，W.08.01 §6.4）。 */
export function iconSvg(name: string, size: 14 | 16 = 16): SVGSVGElement {
  const key = resolveIconName(name);
  const clone = getTemplate(key).cloneNode(true) as SVGSVGElement;
  clone.setAttribute('class', `hx-icon hx-icon--${String(size)}`);
  return clone;
}
