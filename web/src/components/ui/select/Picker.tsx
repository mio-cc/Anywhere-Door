import {
  useEffect,
  useId,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
  type JSX,
  type KeyboardEvent as ReactKeyboardEvent,
  type ReactNode,
  type Ref,
} from 'react';
import { ScrollArea } from '../feedback/ScrollArea';
import { Icon } from '../icon/Icon';
import { Tooltip } from '../overlay/Tooltip';
import { isImeComposing, type IconName } from '../types';
import { matchSpan, step, WINDOW_BUFFER, WINDOW_MIN, windowOf, type NavRow } from './listNav';
import './Picker.css';

export interface PickerItem {
  readonly id: string;
  readonly label: string;
  readonly icon?: IconName;
  readonly desc?: string;
  readonly right?: ReactNode;
  readonly disabled?: boolean;
  readonly disabledReason?: string;
}

export interface PickerGroup {
  readonly title?: string;
  readonly items: readonly PickerItem[];
}

export interface PickerHandle {
  key(e: KeyboardEvent | ReactKeyboardEvent): boolean;
}

export interface PickerProps {
  readonly groups: readonly PickerGroup[];
  readonly activeId: string | null;
  readonly onPick: (id: string, how: 'enter' | 'tab' | 'click') => void;
  readonly onActiveChange: (id: string | null) => void;
  readonly emptyText: string;
  readonly footer?: ReactNode;
  readonly ariaLabel: string;
  readonly listId?: string;
  readonly query?: string;
  readonly maxHeight?: 'sm' | 'md';
  readonly ref?: Ref<PickerHandle>;
}

const ROW_H_SINGLE = 32;
const ROW_H_DESC = 44;
const GROUP_H = 24;
const MAX_H_PX = { sm: 240, md: 320 } as const;

const pickerWarn = { dupId: false };

function logDupPickerIdOnce(id: string): void {
  if (!pickerWarn.dupId) {
    pickerWarn.dupId = true;
    console.error('[ui] Picker duplicate item id:', id);
  }
}

// K307：总安全且单射的 UTF-16 编码（输出仅含 [A-Za-z0-9_]，绝不含 '-' 或空白，永不因未配对代理项抛 URIError）
function encodeDomIdToken(s: string): string {
  let out = '';
  for (let i = 0; i < s.length; i += 1) {
    const code = s.charCodeAt(i);
    const isAsciiAlphaNum =
      (code >= 0x30 && code <= 0x39) || (code >= 0x41 && code <= 0x5a) || (code >= 0x61 && code <= 0x7a);
    if (isAsciiAlphaNum) {
      out += s[i] ?? '';
    } else {
      out += `_${code.toString(16)}_`;
    }
  }
  return out;
}

// K307：同时编码 listId 与 itemId 并以 '-' 分隔，保证任意 JS string 无空白、无异常、无跨实例碰撞
export function pickerOptionId(listId: string, itemId: string): string {
  return `hx-opt-${encodeDomIdToken(listId)}-${encodeDomIdToken(itemId)}`;
}

function renderMatchedLabel(label: string, query: string | undefined): ReactNode {
  if (!query) return label;
  const span = matchSpan(label, query);
  if (!span) return label;
  const [s, e] = span;
  return (
    <>
      {label.slice(0, s)}
      <b>{label.slice(s, e)}</b>
      {label.slice(e)}
    </>
  );
}

type FlatDisplay =
  | { readonly kind: 'group'; readonly key: string; readonly title: string; readonly height: number }
  | { readonly kind: 'item'; readonly item: PickerItem; readonly height: number };

interface NormalizedGroup {
  readonly key: string;
  readonly title: string | undefined;
  readonly items: readonly PickerItem[];
}

export function Picker(p: PickerProps): JSX.Element {
  const {
    groups,
    activeId,
    onPick,
    onActiveChange,
    emptyText,
    footer,
    ariaLabel,
    listId: propListId,
    query,
    maxHeight = 'md',
    ref,
  } = p;

  const autoId = useId();
  const listId = propListId ?? autoId;
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const lastPointerRef = useRef<{ x: number; y: number } | null>(null);
  const fromKeyboardRef = useRef(false);
  const [scrollTop, setScrollTop] = useState(0);
  const scrollTopRef = useRef(0);

  const { navRows, displayRows, normalizedGroups, offsets, totalHeight, itemById, displayIndexById } =
    useMemo(() => {
      const seen = new Set<string>();
      const nav: NavRow[] = [];
      const disp: FlatDisplay[] = [];
      const normGroups: NormalizedGroup[] = [];
      const map = new Map<string, PickerItem>();
      const idxMap = new Map<string, number>();

      for (let gi = 0; gi < groups.length; gi += 1) {
        const g = groups[gi];
        if (!g) continue;
        const validItems: PickerItem[] = [];
        for (const it of g.items) {
          if (seen.has(it.id)) {
            if (import.meta.env.DEV) {
              throw new Error(`Picker duplicate item id: ${it.id}`);
            }
            logDupPickerIdOnce(it.id);
            continue;
          }
          seen.add(it.id);
          validItems.push(it);
          map.set(it.id, it);
        }
        if (validItems.length === 0) continue;
        normGroups.push({ key: `g-${String(gi)}-${g.title ?? ''}`, title: g.title, items: validItems });
        if (g.title) {
          disp.push({ kind: 'group', key: `g-${String(gi)}-${g.title}`, title: g.title, height: GROUP_H });
        }
        for (const it of validItems) {
          const h = it.desc ? ROW_H_DESC : ROW_H_SINGLE;
          nav.push({ id: it.id, disabled: Boolean(it.disabled), height: h });
          idxMap.set(it.id, disp.length);
          disp.push({ kind: 'item', item: it, height: h });
        }
      }

      const offs: number[] = [];
      let y = 0;
      for (const d of disp) {
        offs.push(y);
        y += d.height;
      }

      return {
        navRows: nav,
        displayRows: disp,
        normalizedGroups: normGroups,
        offsets: offs,
        totalHeight: y,
        itemById: map,
        displayIndexById: idxMap,
      };
    }, [groups]);

  const effectiveActiveId = activeId !== null && itemById.has(activeId) ? activeId : null;
  const windowed = navRows.length > WINDOW_MIN;
  const viewportH = MAX_H_PX[maxHeight];

  useImperativeHandle(
    ref,
    () => ({
      key(e: KeyboardEvent | ReactKeyboardEvent): boolean {
        const native = 'nativeEvent' in e ? e.nativeEvent : e;
        if (isImeComposing(native)) return false;
        if (e.key === 'ArrowDown') {
          const next = step(navRows, effectiveActiveId, 1, true);
          if (next === null) return false;
          fromKeyboardRef.current = true;
          onActiveChange(next);
          return true;
        }
        if (e.key === 'ArrowUp') {
          const next = step(navRows, effectiveActiveId, -1, true);
          if (next === null) return false;
          fromKeyboardRef.current = true;
          onActiveChange(next);
          return true;
        }
        if (e.key === 'Enter') {
          if (effectiveActiveId === null) return false;
          const it = itemById.get(effectiveActiveId);
          if (!it || it.disabled) return false;
          onPick(it.id, 'enter');
          return true;
        }
        if (e.key === 'Tab' && !e.shiftKey && !e.ctrlKey && !e.metaKey && !e.altKey) {
          if (effectiveActiveId === null) return false;
          const it = itemById.get(effectiveActiveId);
          if (!it || it.disabled) return false;
          onPick(it.id, 'tab');
          return true;
        }
        return false;
      },
    }),
    [navRows, effectiveActiveId, itemById, onActiveChange, onPick],
  );

  // 窗口化下通过偏移数组直接调整 scrollTop，确保 active 行不在当前 DOM 窗口时也能滚入并渲染（评审 #20、K307：用 ref 读 scrollTop，避免用户滚动后被拉回当前项）
  useEffect(() => {
    if (effectiveActiveId === null) return;
    if (windowed) {
      const idx = displayIndexById.get(effectiveActiveId);
      if (idx === undefined) return;
      const top = offsets[idx] ?? 0;
      const rowH = displayRows[idx]?.height ?? ROW_H_SINGLE;
      const bottom = top + rowH;
      const sc = scrollRef.current;
      const curTop = sc ? sc.scrollTop : scrollTopRef.current;
      const viewH = sc && sc.clientHeight > 0 ? sc.clientHeight : viewportH;
      let nextTop = curTop;
      if (top < curTop) {
        nextTop = top;
      } else if (bottom > curTop + viewH) {
        nextTop = Math.max(0, bottom - viewH);
      }
      if (nextTop !== curTop) {
        if (sc) sc.scrollTop = nextTop;
        scrollTopRef.current = nextTop;
        setScrollTop(nextTop);
      }
      fromKeyboardRef.current = false;
      return;
    }
    if (!fromKeyboardRef.current) return;
    fromKeyboardRef.current = false;
    const el = document.getElementById(pickerOptionId(listId, effectiveActiveId));
    if (el && typeof el.scrollIntoView === 'function') {
      el.scrollIntoView({ block: 'nearest' });
    }
  }, [effectiveActiveId, listId, windowed, displayIndexById, offsets, displayRows, viewportH]);

  const { start, end } = windowed
    ? windowOf(offsets, scrollTop, viewportH, WINDOW_BUFFER)
    : { start: 0, end: displayRows.length };

  const visibleSlice = displayRows.slice(start, end);
  const topPad = windowed ? (offsets[start] ?? 0) : 0;
  const lastRow = displayRows[end - 1];
  const bottomEdge = end > 0 ? (offsets[end - 1] ?? 0) + (lastRow?.height ?? 0) : 0;
  const bottomPad = windowed ? Math.max(0, totalHeight - bottomEdge) : 0;

  const renderItem = (it: PickerItem): JSX.Element => {
    const isActive = it.id === effectiveActiveId;
    const row = (
      <div
        key={it.id}
        id={pickerOptionId(listId, it.id)}
        role="option"
        aria-selected={isActive ? 'true' : 'false'}
        aria-disabled={it.disabled ? 'true' : undefined}
        className={`hx-picker__item${it.desc ? ' hx-picker__item--desc' : ''}${isActive ? ' is-active' : ''}${it.disabled ? ' is-disabled' : ''}`}
        onPointerMove={(e) => {
          const prev = lastPointerRef.current;
          if (prev && prev.x === e.clientX && prev.y === e.clientY) return;
          lastPointerRef.current = { x: e.clientX, y: e.clientY };
          if (!it.disabled && effectiveActiveId !== it.id) {
            onActiveChange(it.id);
          }
        }}
        onPointerDown={(e) => {
          // W.08.03 §6.5：点击前 preventDefault，焦点留在外部输入框
          e.preventDefault();
        }}
        onClick={() => {
          if (it.disabled) return;
          onPick(it.id, 'click');
        }}
      >
        {it.icon ? (
          <span className="hx-picker__icon" aria-hidden="true">
            <Icon name={it.icon} size={16} />
          </span>
        ) : null}
        <span className="hx-picker__text">
          <span className="hx-picker__label">{renderMatchedLabel(it.label, query)}</span>
          {it.desc ? <span className="hx-picker__desc">{it.desc}</span> : null}
        </span>
        {it.right ? <span className="hx-picker__right">{it.right}</span> : null}
      </div>
    );
    if (it.disabled && it.disabledReason) {
      return (
        <Tooltip key={it.id} content={it.disabledReason} placement="right">
          {row}
        </Tooltip>
      );
    }
    return row;
  };

  return (
    <div className="hx-picker">
      <ScrollArea
        ref={scrollRef}
        className={`hx-picker__scroll hx-picker__scroll--${maxHeight}`}
        onScroll={(e) => {
          if (windowed) {
            const top = e.currentTarget.scrollTop;
            scrollTopRef.current = top;
            setScrollTop(top);
          }
        }}
      >
        <div id={listId} role="listbox" aria-label={ariaLabel} className="hx-picker__list">
          {navRows.length === 0 ? (
            <div className="hx-picker__empty">{emptyText}</div>
          ) : windowed ? (
            <>
              {topPad > 0 ? <div style={{ height: `${String(topPad)}px` }} aria-hidden="true" /> : null}
              {visibleSlice.map((entry) => {
                if (entry.kind === 'group') {
                  return (
                    <div key={entry.key} className="hx-picker__group" role="group" aria-label={entry.title}>
                      {entry.title}
                    </div>
                  );
                }
                return renderItem(entry.item);
              })}
              {bottomPad > 0 ? <div style={{ height: `${String(bottomPad)}px` }} aria-hidden="true" /> : null}
            </>
          ) : (
            normalizedGroups.map((g) =>
              g.title ? (
                <div key={g.key} role="group" aria-label={g.title}>
                  <div className="hx-picker__group" aria-hidden="true">
                    {g.title}
                  </div>
                  {g.items.map(renderItem)}
                </div>
              ) : (
                <div key={g.key}>{g.items.map(renderItem)}</div>
              ),
            )
          )}
        </div>
      </ScrollArea>
      {footer ? <div className="hx-picker__footer">{footer}</div> : null}
    </div>
  );
}
