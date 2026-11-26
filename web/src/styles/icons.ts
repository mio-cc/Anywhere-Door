/* 由 tools/gen icons 生成，请勿手改 */
export type IconShape =
  | { readonly t: 'path'; readonly d: string }
  | { readonly t: 'circle'; readonly cx: number; readonly cy: number; readonly r: number }
  | {
      readonly t: 'rect';
      readonly x: number;
      readonly y: number;
      readonly w: number;
      readonly h: number;
      readonly rx?: number;
    }
  | { readonly t: 'line'; readonly x1: number; readonly y1: number; readonly x2: number; readonly y2: number }
  | { readonly t: 'polyline'; readonly points: string };

export type IconName =
  | '资源'
  | 'Agent'
  | '上下文装配'
  | '工具'
  | '扇出'
  | '汇合'
  | '聚合'
  | '分支'
  | '循环'
  | '账本'
  | '人工'
  | '子图端口'
  | '子图引用'
  | '扩展'
  | 'chat'
  | 'graph'
  | 'search'
  | 'history'
  | 'moon'
  | 'sun'
  | 'play'
  | 'stop'
  | 'pause'
  | 'branch'
  | 'expand'
  | 'shrink'
  | 'collapse'
  | 'plus'
  | 'minus'
  | 'x'
  | 'check'
  | 'chevl'
  | 'chevr'
  | 'chevd'
  | 'lib'
  | 'fit'
  | 'pointer'
  | 'hand'
  | 'group'
  | 'layout'
  | 'undo'
  | 'redo'
  | 'trash'
  | 'copy'
  | 'enter'
  | 'lock'
  | 'port'
  | 'model'
  | 'cmd'
  | 'gauge'
  | 'bolt'
  | 'panelL'
  | 'panelR'
  | 'gear'
  | 'arrowUp'
  | 'tree'
  | 'peek'
  | 'bell'
  | 'more'
  | 'star'
  | 'pin'
  | 'user'
  | 'pkg'
  | 'link'
  | 'grip'
  | 'info'
  | 'queued'
  | 'done'
  | 'failed'
  | 'waiting'
  | 'skipped'
  | 'cached';

export interface IconDef {
  readonly shapes: readonly IconShape[];
  readonly solid?: true;
}

export const ICONS: Readonly<Record<IconName, IconDef>> = {
  资源: {
    shapes: [
      { t: 'path', d: 'M3 3.8a5 1.9 0 1 0 10 0a5 1.9 0 1 0 -10 0' },
      { t: 'path', d: 'M3 3.8v8.4c0 1 2.2 1.9 5 1.9s5-.9 5-1.9V3.8' },
      { t: 'path', d: 'M3 8c0 1 2.2 1.9 5 1.9S13 9 13 8' },
    ],
  },
  Agent: {
    shapes: [
      { t: 'circle', cx: 8, cy: 8, r: 2.4 },
      {
        t: 'path',
        d: 'M8 1.6v2.1M8 12.3v2.1M1.6 8h2.1M12.3 8h2.1M3.5 3.5l1.5 1.5M11 11l1.5 1.5M3.5 12.5 5 11M11 5l1.5-1.5',
      },
    ],
  },
  上下文装配: {
    shapes: [
      { t: 'path', d: 'M8 1.8 14 4.8 8 7.8 2 4.8Z' },
      { t: 'path', d: 'm2 8 6 3 6-3' },
      { t: 'path', d: 'm2 11.2 6 3 6-3' },
    ],
  },
  工具: {
    shapes: [
      {
        t: 'path',
        d: 'M10.4 2.2a3.1 3.1 0 0 0-3.2 4.2l-4.8 4.8a1.4 1.4 0 0 0 2 2l4.8-4.8a3.1 3.1 0 0 0 4.2-3.2l-1.9 1.9-1.8-.4-.4-1.8Z',
      },
    ],
  },
  扇出: {
    shapes: [
      { t: 'path', d: 'M1.8 8h3.6' },
      { t: 'path', d: 'M5.4 8c2.2 0 2.2-4.4 4.6-4.4h4M5.4 8H14M5.4 8c2.2 0 2.2 4.4 4.6 4.4h4' },
      { t: 'path', d: 'm12.4 2.2 1.6 1.4-1.6 1.4M12.4 11 14 12.4l-1.6 1.4' },
    ],
  },
  汇合: {
    shapes: [
      {
        t: 'path',
        d: 'M2 3.6h3.4c2.4 0 2.4 4.4 4.6 4.4M2 8h8M2 12.4h3.4c2.4 0 2.4-4.4 4.6-4.4H14',
      },
      { t: 'path', d: 'm12.4 6.6 1.6 1.4-1.6 1.4' },
    ],
  },
  聚合: {
    shapes: [{ t: 'path', d: 'M2 2.8h12L9.4 8.4v4.4l-2.8 1.4V8.4Z' }],
  },
  分支: {
    shapes: [
      { t: 'circle', cx: 4, cy: 3.4, r: 1.6 },
      { t: 'circle', cx: 4, cy: 12.6, r: 1.6 },
      { t: 'circle', cx: 12, cy: 5.2, r: 1.6 },
      { t: 'path', d: 'M4 5v6M12 6.8c0 3.2-8 1.8-8 4.2' },
    ],
  },
  循环: {
    shapes: [
      { t: 'path', d: 'M13.2 6.4A5.2 5.2 0 0 0 3.5 4.7' },
      { t: 'path', d: 'M2.8 9.6a5.2 5.2 0 0 0 9.7 1.7' },
      { t: 'path', d: 'M3.2 2.2v2.9h2.9M12.8 13.8v-2.9H9.9' },
    ],
  },
  账本: {
    shapes: [
      { t: 'rect', x: 3, y: 1.8, w: 10, h: 12.4, rx: 1.6 },
      { t: 'path', d: 'M6 5.4h4M6 8h4M6 10.6h2.2' },
    ],
  },
  人工: {
    shapes: [
      { t: 'circle', cx: 8, cy: 5.4, r: 2.6 },
      { t: 'path', d: 'M2.8 14.2c.6-2.8 2.6-4.2 5.2-4.2s4.6 1.4 5.2 4.2' },
    ],
  },
  子图端口: {
    shapes: [
      { t: 'rect', x: 1.8, y: 3.8, w: 7, h: 8.4, rx: 1.6 },
      { t: 'path', d: 'M8.8 8h5.4M12.2 6l2 2-2 2' },
    ],
  },
  子图引用: {
    shapes: [
      { t: 'rect', x: 1.8, y: 5, w: 9.2, h: 9.2, rx: 1.6 },
      { t: 'path', d: 'M5 2.2h7.2a1.8 1.8 0 0 1 1.8 1.8v7.2' },
    ],
  },
  扩展: {
    shapes: [
      {
        t: 'path',
        d: 'M6.4 2.6a1.4 1.4 0 1 1 2.8 0v1h2.4a1.2 1.2 0 0 1 1.2 1.2v2.4h1a1.4 1.4 0 1 1 0 2.8h-1v2.4a1.2 1.2 0 0 1-1.2 1.2H3.6a1.2 1.2 0 0 1-1.2-1.2V4.8a1.2 1.2 0 0 1 1.2-1.2h2.8v-1Z',
      },
    ],
  },
  chat: {
    shapes: [
      {
        t: 'path',
        d: 'M2.5 4.2a2 2 0 0 1 2-2h7a2 2 0 0 1 2 2v5a2 2 0 0 1-2 2H7l-3 2.6v-2.6h0a2 2 0 0 1-1.5-2Z',
      },
    ],
  },
  graph: {
    shapes: [
      { t: 'rect', x: 1.8, y: 2.2, w: 4.4, h: 3.6, rx: 1 },
      { t: 'rect', x: 9.8, y: 10.2, w: 4.4, h: 3.6, rx: 1 },
      { t: 'rect', x: 9.8, y: 2.2, w: 4.4, h: 3.6, rx: 1 },
      { t: 'path', d: 'M6.2 4h3.6M6.2 4c2 0 1.6 8 3.6 8' },
    ],
  },
  search: {
    shapes: [
      { t: 'circle', cx: 7, cy: 7, r: 4.4 },
      { t: 'path', d: 'm10.4 10.4 3.4 3.4' },
    ],
  },
  history: {
    shapes: [
      { t: 'path', d: 'M2.4 8a5.6 5.6 0 1 0 1.6-3.9' },
      { t: 'path', d: 'M2.2 2.4v2.8H5' },
      { t: 'path', d: 'M8 5v3.2l2.2 1.4' },
    ],
  },
  moon: {
    shapes: [{ t: 'path', d: 'M13.4 9.6A5.6 5.6 0 0 1 6.4 2.6a5.6 5.6 0 1 0 7 7Z' }],
  },
  sun: {
    shapes: [
      { t: 'circle', cx: 8, cy: 8, r: 2.8 },
      {
        t: 'path',
        d: 'M8 1.4v1.4M8 13.2v1.4M1.4 8h1.4M13.2 8h1.4M3.3 3.3l1 1M11.7 11.7l1 1M3.3 12.7l1-1M11.7 4.3l1-1',
      },
    ],
  },
  play: {
    shapes: [{ t: 'path', d: 'M4.6 2.8v10.4L13 8Z' }],
    solid: true,
  },
  stop: {
    shapes: [{ t: 'rect', x: 3.4, y: 3.4, w: 9.2, h: 9.2, rx: 1.6 }],
    solid: true,
  },
  pause: {
    shapes: [{ t: 'path', d: 'M5.4 3.4v9.2M10.6 3.4v9.2' }],
  },
  branch: {
    shapes: [
      { t: 'circle', cx: 4.6, cy: 3.6, r: 1.6 },
      { t: 'circle', cx: 4.6, cy: 12.4, r: 1.6 },
      { t: 'circle', cx: 11.4, cy: 5.6, r: 1.6 },
      { t: 'path', d: 'M4.6 5.2v5.6M11.4 7.2c0 2.6-6.8 1.6-6.8 3.6' },
    ],
  },
  expand: {
    shapes: [{ t: 'path', d: 'M9.6 2.4h4v4M6.4 13.6h-4v-4M13.6 2.4 9.2 6.8M2.4 13.6l4.4-4.4' }],
  },
  shrink: {
    shapes: [{ t: 'path', d: 'M13.6 6.4h-4v-4M2.4 9.6h4v4M9.6 6.4l4-4M6.4 9.6l-4 4' }],
  },
  collapse: {
    shapes: [{ t: 'path', d: 'M3.2 10.4 8 5.6l4.8 4.8' }],
  },
  plus: {
    shapes: [{ t: 'path', d: 'M8 3v10M3 8h10' }],
  },
  minus: {
    shapes: [{ t: 'path', d: 'M3 8h10' }],
  },
  x: {
    shapes: [{ t: 'path', d: 'm4 4 8 8M12 4l-8 8' }],
  },
  check: {
    shapes: [{ t: 'path', d: 'm3 8.4 3.2 3.2L13 4.6' }],
  },
  chevl: {
    shapes: [{ t: 'path', d: 'M10 3.4 5.4 8l4.6 4.6' }],
  },
  chevr: {
    shapes: [{ t: 'path', d: 'M6 3.4 10.6 8 6 12.6' }],
  },
  chevd: {
    shapes: [{ t: 'path', d: 'M3.4 6 8 10.6 12.6 6' }],
  },
  lib: {
    shapes: [
      { t: 'rect', x: 2, y: 2, w: 5, h: 5, rx: 1.2 },
      { t: 'rect', x: 9, y: 2, w: 5, h: 5, rx: 1.2 },
      { t: 'rect', x: 2, y: 9, w: 5, h: 5, rx: 1.2 },
      { t: 'path', d: 'M11.5 9v5M9 11.5h5' },
    ],
  },
  fit: {
    shapes: [
      {
        t: 'path',
        d: 'M2.4 5.6V3.4a1 1 0 0 1 1-1h2.2M10.4 2.4h2.2a1 1 0 0 1 1 1v2.2M13.6 10.4v2.2a1 1 0 0 1-1 1h-2.2M5.6 13.6H3.4a1 1 0 0 1-1-1v-2.2',
      },
      { t: 'rect', x: 5.4, y: 5.4, w: 5.2, h: 5.2, rx: 1 },
    ],
  },
  pointer: {
    shapes: [{ t: 'path', d: 'M3.2 2.4 12.8 7l-4.2 1.4-2 4.2Z' }],
  },
  hand: {
    shapes: [
      {
        t: 'path',
        d: 'M5.4 8V3.8a1.1 1.1 0 0 1 2.2 0V7.4M7.6 7V2.8a1.1 1.1 0 0 1 2.2 0V7.4M9.8 7.4V3.8a1.1 1.1 0 0 1 2.2 0v5.4c0 2.8-1.8 4.8-4.6 4.8-1.8 0-3-.8-4-2.4L2 9a1.1 1.1 0 0 1 1.8-1.2L5.4 9.6',
      },
    ],
  },
  group: {
    shapes: [
      { t: 'rect', x: 1.8, y: 1.8, w: 12.4, h: 12.4, rx: 2 },
      { t: 'rect', x: 4.4, y: 4.4, w: 3.4, h: 3, rx: 0.8 },
      { t: 'rect', x: 8.2, y: 8.6, w: 3.4, h: 3, rx: 0.8 },
    ],
  },
  layout: {
    shapes: [
      { t: 'rect', x: 1.8, y: 2.4, w: 4, h: 3.2, rx: 0.8 },
      { t: 'rect', x: 10.2, y: 2.4, w: 4, h: 3.2, rx: 0.8 },
      { t: 'rect', x: 10.2, y: 10.4, w: 4, h: 3.2, rx: 0.8 },
      { t: 'path', d: 'M5.8 4h4.4M8 4v8h2.2' },
    ],
  },
  undo: {
    shapes: [
      { t: 'path', d: 'M5 3.6 2.4 6.2 5 8.8' },
      { t: 'path', d: 'M2.6 6.2h7a3.6 3.6 0 0 1 0 7.2H6.4' },
    ],
  },
  redo: {
    shapes: [
      { t: 'path', d: 'm11 3.6 2.6 2.6L11 8.8' },
      { t: 'path', d: 'M13.4 6.2h-7a3.6 3.6 0 0 0 0 7.2h3.2' },
    ],
  },
  trash: {
    shapes: [{ t: 'path', d: 'M2.6 4.4h10.8M6.4 4.4V2.8h3.2v1.6M4 4.4l.6 9h6.8l.6-9' }],
  },
  copy: {
    shapes: [
      { t: 'rect', x: 5.2, y: 5.2, w: 8.6, h: 8.6, rx: 1.6 },
      {
        t: 'path',
        d: 'M10.8 5.2V3.6a1.4 1.4 0 0 0-1.4-1.4H3.6a1.4 1.4 0 0 0-1.4 1.4v5.8a1.4 1.4 0 0 0 1.4 1.4h1.6',
      },
    ],
  },
  enter: {
    shapes: [
      { t: 'path', d: 'M6 2.4h5.8a1.8 1.8 0 0 1 1.8 1.8v7.6a1.8 1.8 0 0 1-1.8 1.8H6' },
      { t: 'path', d: 'M2.4 8h7.2M7 5.4 9.6 8 7 10.6' },
    ],
  },
  lock: {
    shapes: [
      { t: 'rect', x: 3.2, y: 7, w: 9.6, h: 7, rx: 1.6 },
      { t: 'path', d: 'M5.4 7V5a2.6 2.6 0 0 1 5.2 0v2' },
    ],
  },
  port: {
    shapes: [
      { t: 'circle', cx: 8, cy: 8, r: 2.6 },
      { t: 'path', d: 'M1.6 8h3.8M10.6 8h3.8' },
    ],
  },
  model: {
    shapes: [{ t: 'path', d: 'M8 1.8 9.4 6.6 14.2 8 9.4 9.4 8 14.2 6.6 9.4 1.8 8 6.6 6.6Z' }],
  },
  cmd: {
    shapes: [
      { t: 'path', d: 'M5.4 5.4h5.2v5.2H5.4Z' },
      {
        t: 'path',
        d: 'M5.4 5.4V4a1.6 1.6 0 1 0-1.6 1.4h1.6M10.6 5.4V4a1.6 1.6 0 1 1 1.6 1.4h-1.6M5.4 10.6V12a1.6 1.6 0 1 1-1.6-1.4h1.6M10.6 10.6V12a1.6 1.6 0 1 0 1.6-1.4h-1.6',
      },
    ],
  },
  gauge: {
    shapes: [
      { t: 'path', d: 'M2.6 11.6a5.6 5.6 0 1 1 10.8 0' },
      { t: 'path', d: 'm8 9 2.4-3' },
    ],
  },
  bolt: {
    shapes: [{ t: 'path', d: 'M8.8 1.8 3.6 9h4l-.8 5.2L12.4 7h-4Z' }],
  },
  panelL: {
    shapes: [
      { t: 'rect', x: 2, y: 2.4, w: 12, h: 11.2, rx: 1.6 },
      { t: 'line', x1: 6, y1: 2.4, x2: 6, y2: 13.6 },
    ],
  },
  panelR: {
    shapes: [
      { t: 'rect', x: 2, y: 2.4, w: 12, h: 11.2, rx: 1.6 },
      { t: 'line', x1: 10, y1: 2.4, x2: 10, y2: 13.6 },
    ],
  },
  gear: {
    shapes: [
      { t: 'circle', cx: 8, cy: 8, r: 2.2 },
      {
        t: 'path',
        d: 'M8 1.8v1.6M8 12.6v1.6M1.8 8h1.6M12.6 8h1.6M3.6 3.6l1.1 1.1M11.3 11.3l1.1 1.1M3.6 12.4l1.1-1.1M11.3 4.7l1.1-1.1',
      },
    ],
  },
  arrowUp: {
    shapes: [
      { t: 'path', d: 'M8 13V3.2' },
      { t: 'polyline', points: '3.8,7.4 8,3.2 12.2,7.4' },
    ],
  },
  tree: {
    shapes: [
      { t: 'rect', x: 2.2, y: 2, w: 4, h: 3, rx: 0.8 },
      { t: 'rect', x: 9.6, y: 6.5, w: 4.2, h: 3, rx: 0.8 },
      { t: 'rect', x: 9.6, y: 11, w: 4.2, h: 3, rx: 0.8 },
      { t: 'path', d: 'M4.2 5v7.5h5.4M4.2 8h5.4' },
    ],
  },
  peek: {
    shapes: [
      { t: 'path', d: 'M1.8 8s2.4-4 6.2-4 6.2 4 6.2 4-2.4 4-6.2 4-6.2-4-6.2-4Z' },
      { t: 'circle', cx: 8, cy: 8, r: 1.8 },
    ],
  },
  bell: {
    shapes: [
      { t: 'path', d: 'M4 6.6a4 4 0 0 1 8 0c0 3 1.2 4.2 1.6 4.6H2.4c.4-.4 1.6-1.6 1.6-4.6Z' },
      { t: 'path', d: 'M6.4 13.2a1.8 1.8 0 0 0 3.2 0' },
    ],
  },
  more: {
    shapes: [
      { t: 'circle', cx: 3.5, cy: 8, r: 1.2 },
      { t: 'circle', cx: 8, cy: 8, r: 1.2 },
      { t: 'circle', cx: 12.5, cy: 8, r: 1.2 },
    ],
    solid: true,
  },
  star: {
    shapes: [{ t: 'path', d: 'M8 2.2 9.8 5.9l4 .6-2.9 2.8.7 4-3.6-1.9-3.6 1.9.7-4-2.9-2.8 4-.6Z' }],
  },
  pin: {
    shapes: [
      { t: 'path', d: 'M5.2 2.4h5.6l-.8 4 2 2v1.2H4V8.4l2-2-.8-4Z' },
      { t: 'line', x1: 8, y1: 9.6, x2: 8, y2: 14 },
    ],
  },
  user: {
    shapes: [
      { t: 'circle', cx: 8, cy: 5.2, r: 2.6 },
      { t: 'path', d: 'M3 13.8c.7-2.6 2.6-3.8 5-3.8s4.3 1.2 5 3.8' },
    ],
  },
  pkg: {
    shapes: [
      { t: 'path', d: 'M8 1.8 13.6 4.8v6.4L8 14.2 2.4 11.2V4.8Z' },
      { t: 'path', d: 'M2.4 4.8 8 7.8l5.6-3M8 7.8v6.4' },
    ],
  },
  link: {
    shapes: [
      { t: 'path', d: 'M6.6 9.4a2.8 2.8 0 0 1 0-4l1.6-1.6a2.8 2.8 0 0 1 4 4l-.8.8' },
      { t: 'path', d: 'M9.4 6.6a2.8 2.8 0 0 1 0 4l-1.6 1.6a2.8 2.8 0 0 1-4-4l.8-.8' },
    ],
  },
  grip: {
    shapes: [
      { t: 'circle', cx: 5.6, cy: 4, r: 1.1 },
      { t: 'circle', cx: 10.4, cy: 4, r: 1.1 },
      { t: 'circle', cx: 5.6, cy: 8, r: 1.1 },
      { t: 'circle', cx: 10.4, cy: 8, r: 1.1 },
      { t: 'circle', cx: 5.6, cy: 12, r: 1.1 },
      { t: 'circle', cx: 10.4, cy: 12, r: 1.1 },
    ],
    solid: true,
  },
  info: {
    shapes: [
      { t: 'circle', cx: 8, cy: 8, r: 6 },
      { t: 'line', x1: 8, y1: 7.2, x2: 8, y2: 11 },
      { t: 'line', x1: 8, y1: 4.8, x2: 8, y2: 4.8 },
    ],
  },
  queued: {
    shapes: [
      { t: 'circle', cx: 8, cy: 8, r: 5 },
      { t: 'line', x1: 8, y1: 5.4, x2: 8, y2: 8 },
      { t: 'line', x1: 8, y1: 8, x2: 10, y2: 9.4 },
    ],
  },
  done: {
    shapes: [
      { t: 'circle', cx: 8, cy: 8, r: 6 },
      { t: 'path', d: 'm5.2 8.2 1.9 1.9 3.8-4' },
    ],
  },
  failed: {
    shapes: [
      { t: 'circle', cx: 8, cy: 8, r: 6 },
      { t: 'path', d: 'm5.8 5.8 4.4 4.4M10.2 5.8l-4.4 4.4' },
    ],
  },
  waiting: {
    shapes: [
      { t: 'circle', cx: 8, cy: 8, r: 6 },
      { t: 'path', d: 'M6.6 5.6v4.8M9.4 5.6v4.8' },
    ],
  },
  skipped: {
    shapes: [
      { t: 'circle', cx: 8, cy: 8, r: 6 },
      { t: 'line', x1: 5, y1: 8, x2: 11, y2: 8 },
    ],
  },
  cached: {
    shapes: [{ t: 'path', d: 'M8.8 2.4 4.4 8.6h3.4l-.6 5 4.4-6.2H8.2Z' }],
  },
};

export const ICON_FALLBACK: IconName = '扩展';
