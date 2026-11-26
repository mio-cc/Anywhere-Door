import {
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type JSX,
  type KeyboardEvent as ReactKeyboardEvent,
  type ReactNode,
} from 'react';
import { useT } from '@/lib/i18n';
import '../base.css';
import { ScrollArea } from '../feedback/ScrollArea';
import { Icon } from '../icon/Icon';
import { SearchInput } from '../input/SearchInput';
import { Popover } from '../overlay/Popover';
import { Tooltip } from '../overlay/Tooltip';
import { isImeComposing, type IconName, type Size } from '../types';
import {
  edge,
  matchSpan,
  step,
  typeahead,
  WINDOW_BUFFER,
  WINDOW_MIN,
  windowOf,
  type NavRow,
} from './listNav';
import './Select.css';

export interface SelectOption<V extends string = string> {
  readonly value: V;
  readonly label: string;
  readonly desc?: string;
  readonly icon?: IconName;
  readonly disabled?: boolean;
  readonly disabledReason?: string;
  readonly group?: string;
}

export interface SelectProps<V extends string = string> {
  readonly value: V | null;
  readonly options: readonly SelectOption<V>[];
  readonly onChange: (v: V) => void;
  readonly filterable?: boolean;
  readonly footer?: ReactNode;
  readonly placeholder?: string;
  readonly size?: Size;
  readonly ariaLabel: string;
  readonly id?: string;
  readonly disabled?: boolean;
  readonly disabledReason?: string;
  readonly lockReason?: string;
  readonly invalid?: boolean;
  readonly mono?: boolean;
}

const ROW_H = 28;
const GROUP_H = 24;
const VIEWPORT_H = 320;
const TYPEAHEAD_MS = 500;

const selectWarn = { unknown: false };

function renderBold(text: string, query: string): ReactNode {
  if (!query) return text;
  const span = matchSpan(text, query);
  if (!span) return text;
  const [s, e] = span;
  return (
    <>
      {text.slice(0, s)}
      <b>{text.slice(s, e)}</b>
      {text.slice(e)}
    </>
  );
}

type FlatSelectRow<V extends string> =
  | {
      readonly kind: 'group';
      readonly key: string;
      readonly title: string;
      readonly sep: boolean;
      readonly height: number;
    }
  | { readonly kind: 'opt'; readonly opt: SelectOption<V>; readonly height: number };

export function Select<V extends string = string>(p: SelectProps<V>): JSX.Element {
  const {
    value,
    options,
    onChange,
    filterable,
    footer,
    placeholder = '',
    size = 'md',
    ariaLabel,
    id: propId,
    disabled = false,
    disabledReason,
    lockReason,
    invalid = false,
    mono = false,
  } = p;

  const t = useT();
  const autoId = useId();
  const listId = useId();
  const triggerId = propId ?? autoId;
  const [triggerEl, setTriggerEl] = useState<HTMLButtonElement | null>(null);
  const filterRef = useRef<HTMLInputElement | null>(null);
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const typeBufRef = useRef('');
  const typeTimerRef = useRef<number | null>(null);

  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [activeVal, setActiveVal] = useState<V | null>(null);
  const [scrollTop, setScrollTop] = useState(0);

  const isLocked = Boolean(lockReason);
  const softDisabled = (disabled && Boolean(disabledReason)) || isLocked;
  const nativeDisabled = disabled && !disabledReason && !isLocked;
  const blocked = disabled || isLocked;
  const showFilter = filterable ?? options.length > 8;

  const selectedOpt = useMemo(
    () => (value === null ? null : (options.find((o) => o.value === value) ?? null)),
    [options, value],
  );

  useEffect(() => {
    if (value !== null && !selectedOpt && import.meta.env.DEV && !selectWarn.unknown) {
      selectWarn.unknown = true;
      console.warn(`[ui] Select value "${value}" not found in options.`);
    }
  }, [value, selectedOpt]);

  const filtered = useMemo(() => {
    const q = query.trim();
    if (!showFilter || q === '') return options;
    return options.filter(
      (o) => matchSpan(o.label, q) !== null || (o.desc ? matchSpan(o.desc, q) !== null : false),
    );
  }, [options, query, showFilter]);

  const { flatRows, navRows, offsets, totalHeight, optByValue, flatIndexByValue } = useMemo(() => {
    const flat: FlatSelectRow<V>[] = [];
    const nav: NavRow[] = [];
    const map = new Map<V, SelectOption<V>>();
    const idxMap = new Map<V, number>();
    let lastGroup: string | undefined;
    let hadGroup = false;

    for (const o of filtered) {
      map.set(o.value, o);
      if (o.group && o.group !== lastGroup) {
        flat.push({
          kind: 'group',
          key: `grp-${o.group}-${String(flat.length)}`,
          title: o.group,
          sep: hadGroup,
          height: GROUP_H,
        });
        lastGroup = o.group;
        hadGroup = true;
      }
      idxMap.set(o.value, flat.length);
      flat.push({ kind: 'opt', opt: o, height: ROW_H });
      nav.push({ id: o.value, disabled: Boolean(o.disabled), height: ROW_H });
    }

    const offs: number[] = [];
    let y = 0;
    for (const r of flat) {
      offs.push(y);
      y += r.height;
    }

    return {
      flatRows: flat,
      navRows: nav,
      offsets: offs,
      totalHeight: y,
      optByValue: map,
      flatIndexByValue: idxMap,
    };
  }, [filtered]);

  useEffect(() => {
    return () => {
      if (typeTimerRef.current !== null) window.clearTimeout(typeTimerRef.current);
    };
  }, []);

  const windowed = navRows.length > WINDOW_MIN;

  // 确保 activeVal 在窗口化下也能滚入可见区域并驱动 scrollTop 重算窗口（评审 #19）
  const scrollActiveIntoView = (targetVal: V | null): void => {
    if (targetVal === null) return;
    if (windowed) {
      const flatIdx = flatIndexByValue.get(targetVal);
      if (flatIdx === undefined) return;
      const top = offsets[flatIdx] ?? 0;
      const rowH = flatRows[flatIdx]?.height ?? ROW_H;
      const bottom = top + rowH;
      const sc = scrollRef.current;
      const curTop = sc ? sc.scrollTop : scrollTop;
      const viewH = sc && sc.clientHeight > 0 ? sc.clientHeight : VIEWPORT_H;
      let nextTop = curTop;
      if (top < curTop) {
        nextTop = top;
      } else if (bottom > curTop + viewH) {
        nextTop = Math.max(0, bottom - viewH);
      }
      if (nextTop !== curTop) {
        if (sc) sc.scrollTop = nextTop;
        setScrollTop(nextTop);
      }
      return;
    }
    const el = document.getElementById(`${listId}-${targetVal}`);
    if (el && typeof el.scrollIntoView === 'function') {
      el.scrollIntoView({ block: 'nearest' });
    }
  };

  const openMenu = (): void => {
    if (blocked) return;
    setQuery('');
    const initial: V | null =
      value !== null && options.some((o) => o.value === value && !o.disabled)
        ? value
        : (options.find((o) => !o.disabled)?.value ?? null);
    setActiveVal(initial);
    // 窗口化且当前值在深位置时，直接计算初始 scrollTop 让打开首帧就能渲染出当前值（评审 #19）
    if (initial !== null && options.length > WINDOW_MIN) {
      const idx = options.findIndex((o) => o.value === initial);
      const estTop = Math.max(0, idx * ROW_H - Math.floor(VIEWPORT_H / 2));
      setScrollTop(estTop);
    } else {
      setScrollTop(0);
    }
    setOpen(true);
  };

  const closeMenu = (): void => {
    setOpen(false);
    setQuery('');
  };

  const commit = (v: V): void => {
    const opt = optByValue.get(v);
    if (!opt || opt.disabled) return;
    if (v !== value) {
      onChange(v);
    }
    closeMenu();
  };

  useEffect(() => {
    if (!open) return;
    if (showFilter) {
      filterRef.current?.focus();
    }
  }, [open, showFilter]);

  useEffect(() => {
    if (!open || activeVal === null) return;
    scrollActiveIntoView(activeVal);
  }, [open, activeVal, flatRows.length]);

  const handleTriggerKeyDown = (e: ReactKeyboardEvent<HTMLButtonElement>): void => {
    if (isImeComposing(e.nativeEvent) || blocked) return;
    if (e.key === 'ArrowDown' || e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      openMenu();
    }
  };

  const handleOverlayKey = (e: KeyboardEvent): boolean => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      const next = step(navRows, activeVal, 1, false);
      if (next !== null) setActiveVal(next as V);
      return true;
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault();
      const next = step(navRows, activeVal, -1, false);
      if (next !== null) setActiveVal(next as V);
      return true;
    }
    if (!showFilter && e.key === 'Home') {
      e.preventDefault();
      const first = edge(navRows, 'first');
      if (first !== null) setActiveVal(first as V);
      return true;
    }
    if (!showFilter && e.key === 'End') {
      e.preventDefault();
      const last = edge(navRows, 'last');
      if (last !== null) setActiveVal(last as V);
      return true;
    }
    if (e.key === 'Enter' || e.key === 'Tab') {
      if (activeVal !== null) {
        e.preventDefault();
        commit(activeVal);
        return true;
      }
      return false;
    }
    if (!showFilter && e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
      typeBufRef.current += e.key;
      if (typeTimerRef.current !== null) window.clearTimeout(typeTimerRef.current);
      typeTimerRef.current = window.setTimeout(() => {
        typeBufRef.current = '';
        typeTimerRef.current = null;
      }, TYPEAHEAD_MS);
      const hit = typeahead(
        filtered.map((o) => ({ id: o.value, label: o.label, disabled: Boolean(o.disabled) })),
        typeBufRef.current,
        activeVal,
      );
      if (hit !== null) {
        setActiveVal(hit as V);
        return true;
      }
    }
    return false;
  };

  const triggerClasses = [
    'hx-select',
    'hx-ctl',
    size === 'sm' ? 'hx-ctl--sm' : '',
    mono ? 'hx-select--mono' : '',
    open ? 'is-open' : '',
    invalid ? 'is-invalid' : '',
    softDisabled || nativeDisabled ? 'is-disabled' : '',
    'hx-focusring',
  ]
    .filter(Boolean)
    .join(' ');

  const trigger = (
    <button
      ref={setTriggerEl}
      id={triggerId}
      type="button"
      className={triggerClasses}
      role="combobox"
      aria-haspopup="listbox"
      aria-expanded={open ? 'true' : 'false'}
      aria-controls={open ? listId : undefined}
      aria-label={ariaLabel}
      aria-disabled={softDisabled ? 'true' : undefined}
      aria-invalid={invalid ? 'true' : undefined}
      disabled={nativeDisabled}
      onClick={() => {
        if (open) closeMenu();
        else openMenu();
      }}
      onKeyDown={handleTriggerKeyDown}
    >
      <span className={`hx-select__val${selectedOpt ? '' : ' is-placeholder'}`}>
        {selectedOpt ? selectedOpt.label : placeholder}
      </span>
      <span className="hx-select__tail" aria-hidden="true">
        {isLocked ? (
          <Icon name="lock" size={14} label={t('ui.locked')} />
        ) : (
          <span className={`hx-select__chev${open ? ' is-open' : ''}`}>
            <Icon name="chevd" size={14} />
          </span>
        )}
      </span>
    </button>
  );

  const { start, end } = windowed
    ? windowOf(offsets, scrollTop, VIEWPORT_H, WINDOW_BUFFER)
    : { start: 0, end: flatRows.length };
  const visibleSlice = flatRows.slice(start, end);
  const topPad = windowed ? (offsets[start] ?? 0) : 0;
  const lastRow = flatRows[end - 1];
  const bottomEdge = end > 0 ? (offsets[end - 1] ?? 0) + (lastRow?.height ?? 0) : 0;
  const bottomPad = windowed ? Math.max(0, totalHeight - bottomEdge) : 0;

  const tip = disabled && disabledReason ? disabledReason : lockReason;
  const activeOptionDomId = activeVal !== null ? `${listId}-${activeVal}` : undefined;

  return (
    <>
      {tip ? <Tooltip content={tip}>{trigger}</Tooltip> : trigger}
      {triggerEl ? (
        <Popover
          anchor={triggerEl}
          placement="bottom-start"
          offset={4}
          width="anchor"
          padding="sm"
          open={open}
          onClose={() => {
            closeMenu();
          }}
          initialFocus={showFilter ? 'first' : 'container'}
          restoreFocus={triggerEl}
          onKey={handleOverlayKey}
          role="dialog"
          {...(!showFilter && activeOptionDomId !== undefined
            ? { ariaActiveDescendant: activeOptionDomId }
            : {})}
          ariaLabel={ariaLabel}
          className="hx-select__pop"
        >
          {showFilter ? (
            <div className="hx-select__filter">
              <SearchInput
                inputRef={filterRef}
                size="sm"
                value={query}
                onChange={(v) => {
                  setQuery(v);
                  setScrollTop(0);
                  const firstValid = options.find(
                    (o) =>
                      !o.disabled &&
                      (matchSpan(o.label, v) !== null || (o.desc ? matchSpan(o.desc, v) !== null : false)),
                  );
                  setActiveVal(firstValid ? firstValid.value : null);
                }}
                onClear={() => {
                  setQuery('');
                  setScrollTop(0);
                }}
                ariaLabel={t('ui.filter')}
                onEscape="none"
                ariaControls={listId}
                {...(activeOptionDomId !== undefined ? { ariaActiveDescendant: activeOptionDomId } : {})}
              />
            </div>
          ) : null}
          <ScrollArea ref={scrollRef} className="hx-select__scroll">
            <div
              id={listId}
              role="listbox"
              aria-label={ariaLabel}
              className="hx-select__list"
              onScroll={(e) => {
                if (windowed) setScrollTop(e.currentTarget.scrollTop);
              }}
            >
              {filtered.length === 0 ? (
                <div className="hx-select__empty">{t('ui.noMatch')}</div>
              ) : (
                <>
                  {topPad > 0 ? <div style={{ height: `${String(topPad)}px` }} aria-hidden="true" /> : null}
                  {visibleSlice.map((entry) => {
                    if (entry.kind === 'group') {
                      return (
                        <div
                          key={entry.key}
                          className={`hx-select__group${entry.sep ? ' has-sep' : ''}`}
                          role="presentation"
                        >
                          {entry.title}
                        </div>
                      );
                    }
                    const o = entry.opt;
                    const isSelected = o.value === value;
                    const isActive = o.value === activeVal;
                    const optEl = (
                      <div
                        key={o.value}
                        id={`${listId}-${o.value}`}
                        role="option"
                        aria-selected={isSelected ? 'true' : 'false'}
                        aria-disabled={o.disabled ? 'true' : undefined}
                        className={`hx-select__opt${isActive ? ' is-active' : ''}${isSelected ? ' is-selected' : ''}${o.disabled ? ' is-disabled' : ''}`}
                        onPointerEnter={() => {
                          setActiveVal(o.value);
                        }}
                        onPointerDown={(e) => {
                          e.preventDefault();
                        }}
                        onClick={() => {
                          commit(o.value);
                        }}
                      >
                        {o.icon ? (
                          <span className="hx-select__opticon" aria-hidden="true">
                            <Icon name={o.icon} size={14} />
                          </span>
                        ) : null}
                        <span className="hx-select__optlabel">{renderBold(o.label, query)}</span>
                        {o.desc ? (
                          <span className="hx-select__optdesc">{renderBold(o.desc, query)}</span>
                        ) : null}
                        {isSelected ? (
                          <span className="hx-select__check" aria-hidden="true">
                            <Icon name="check" size={14} />
                          </span>
                        ) : null}
                      </div>
                    );
                    if (o.disabled && o.disabledReason) {
                      return (
                        <Tooltip key={o.value} content={o.disabledReason} placement="right">
                          {optEl}
                        </Tooltip>
                      );
                    }
                    return optEl;
                  })}
                  {bottomPad > 0 ? (
                    <div style={{ height: `${String(bottomPad)}px` }} aria-hidden="true" />
                  ) : null}
                </>
              )}
            </div>
          </ScrollArea>
          {footer ? <div className="hx-select__footer">{footer}</div> : null}
        </Popover>
      ) : null}
    </>
  );
}
