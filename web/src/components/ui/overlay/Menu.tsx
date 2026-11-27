import { useEffect, useId, useMemo, useRef, useState, type JSX } from 'react';
import { useT } from '@/lib/i18n';
import { Kbd } from '../button/Kbd';
import { Icon } from '../icon/Icon';
import type { CloseReason, IconName, Point } from '../types';
import type { OverlayProps } from './Overlay';
import { Popover } from './Popover';
import { Tooltip } from './Tooltip';
import './Menu.css';

export interface MenuAction {
  readonly id: string;
  readonly label: string;
  readonly icon?: IconName;
  readonly shortcut?: string;
  readonly disabled?: boolean;
  readonly disabledReason?: string;
  readonly danger?: boolean;
  readonly model?: boolean;
  readonly checked?: boolean;
  readonly children?: readonly MenuItem[];
  readonly run?: () => void;
}

export type MenuEntry = MenuAction;
export type MenuItem = MenuAction | 'sep' | { readonly group: string };

export interface MenuProps {
  readonly at: Point | HTMLElement;
  readonly items: readonly MenuItem[];
  readonly onPick: (id: string) => void;
  readonly onClose: (reason: CloseReason) => void;
  readonly open?: boolean;
  readonly restoreFocus?: OverlayProps['restoreFocus'];
  readonly ariaLabel?: string;
}

interface InternalMenuProps extends MenuProps {
  readonly isSubmenu?: boolean;
  readonly autoHighlightFirst?: boolean;
  readonly onPickRoot?: (id: string) => void;
  readonly onCloseRoot?: (reason: CloseReason) => void;
  readonly onPointerEnterMenu?: () => void;
  readonly onPointerLeaveMenu?: () => void;
}

const SUBMENU_OPEN_DELAY = 150;
const SUBMENU_CLOSE_DELAY = 300;
const TYPEAHEAD_RESET_MS = 500;

const menuWarnState = { dupId: false };

function warnDupMenuIdOnce(id: string): void {
  if (!menuWarnState.dupId) {
    menuWarnState.dupId = true;
    console.error('[ui] Menu duplicate item id:', id);
  }
}

function isAction(it: MenuItem): it is MenuAction {
  return typeof it === 'object' && 'id' in it;
}

function pointToRect(pt: Point): DOMRect {
  return {
    x: pt.x,
    y: pt.y,
    width: 0,
    height: 0,
    top: pt.y,
    left: pt.x,
    right: pt.x,
    bottom: pt.y,
    toJSON() {
      return this;
    },
  };
}

function MenuInner(p: InternalMenuProps): JSX.Element | null {
  const {
    at,
    items,
    onPick,
    onClose,
    open = true,
    restoreFocus,
    ariaLabel,
    isSubmenu = false,
    autoHighlightFirst = false,
    onPickRoot,
    onCloseRoot,
    onPointerEnterMenu,
    onPointerLeaveMenu,
  } = p;

  const t = useT();
  const resolvedAriaLabel = ariaLabel ?? t('ui.menu');
  const menuId = useId();
  const isElementAnchor = at instanceof HTMLElement;
  const ptX = isElementAnchor ? 0 : at.x;
  const ptY = isElementAnchor ? 0 : at.y;
  // K307：按点坐标缓存 DOMRect，避免悬停/方向键重渲染触发 Popover 反复 place()
  const pointAnchor = useMemo(() => pointToRect({ x: ptX, y: ptY }), [ptX, ptY]);
  const [activeIdx, setActiveIdx] = useState<number>(autoHighlightFirst ? 0 : -1);
  const [subState, setSubState] = useState<{
    readonly id: string;
    readonly anchor: HTMLDivElement;
    readonly autoHighlight: boolean;
  } | null>(null);
  const [prevOpen, setPrevOpen] = useState(open);

  const itemElsRef = useRef<Map<string, HTMLDivElement>>(new Map());
  const openTimerRef = useRef<number | null>(null);
  const closeTimerRef = useRef<number | null>(null);
  const typeBufRef = useRef('');
  const typeTimerRef = useRef<number | null>(null);

  if (open !== prevOpen) {
    setPrevOpen(open);
    if (!open) {
      setActiveIdx(-1);
      setSubState(null);
    } else if (autoHighlightFirst) {
      setActiveIdx(0);
    }
  }

  const seenIds = new Set<string>();
  const normalized: MenuItem[] = [];
  const actions: MenuAction[] = [];
  let dupId: string | null = null;
  for (const item of items) {
    if (!isAction(item)) {
      normalized.push(item);
      continue;
    }
    if (seenIds.has(item.id)) {
      if (import.meta.env.DEV) {
        throw new Error(`Menu duplicate item id: ${item.id}`);
      }
      dupId = item.id;
      continue;
    }
    seenIds.add(item.id);
    normalized.push(item);
    actions.push(item);
  }

  useEffect(() => {
    if (dupId !== null) warnDupMenuIdOnce(dupId);
  }, [dupId]);

  useEffect(() => {
    return () => {
      if (openTimerRef.current !== null) window.clearTimeout(openTimerRef.current);
      if (closeTimerRef.current !== null) window.clearTimeout(closeTimerRef.current);
      if (typeTimerRef.current !== null) window.clearTimeout(typeTimerRef.current);
    };
  }, []);

  if (!open) return null;

  const cancelSubTimers = (): void => {
    if (openTimerRef.current !== null) {
      window.clearTimeout(openTimerRef.current);
      openTimerRef.current = null;
    }
    if (closeTimerRef.current !== null) {
      window.clearTimeout(closeTimerRef.current);
      closeTimerRef.current = null;
    }
  };

  const scheduleSubClose = (): void => {
    if (openTimerRef.current !== null) {
      window.clearTimeout(openTimerRef.current);
      openTimerRef.current = null;
    }
    if (closeTimerRef.current !== null) {
      window.clearTimeout(closeTimerRef.current);
    }
    closeTimerRef.current = window.setTimeout(() => {
      closeTimerRef.current = null;
      setSubState(null);
    }, SUBMENU_CLOSE_DELAY);
  };

  const openSubFor = (id: string, autoHighlight: boolean): void => {
    const anchor = itemElsRef.current.get(id);
    if (!anchor) return;
    setSubState({ id, anchor, autoHighlight });
  };

  const pickAction = (act: MenuAction): void => {
    if (act.disabled) return;
    if (act.children && act.children.length > 0) {
      openSubFor(act.id, true);
      return;
    }
    act.run?.();
    if (onPickRoot) {
      onPickRoot(act.id);
    } else {
      onPick(act.id);
    }
    if (onCloseRoot) {
      onCloseRoot('action');
    } else {
      onClose('action');
    }
  };

  const handleKey = (e: KeyboardEvent): boolean => {
    if (actions.length === 0) return false;
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActiveIdx((prev) => (prev < 0 ? 0 : (prev + 1) % actions.length));
      return true;
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActiveIdx((prev) => (prev < 0 ? actions.length - 1 : (prev - 1 + actions.length) % actions.length));
      return true;
    }
    if (e.key === 'Home') {
      e.preventDefault();
      setActiveIdx(0);
      return true;
    }
    if (e.key === 'End') {
      e.preventDefault();
      setActiveIdx(actions.length - 1);
      return true;
    }
    if (e.key === 'ArrowRight') {
      const cur = activeIdx >= 0 ? actions[activeIdx] : undefined;
      if (cur && !cur.disabled && cur.children && cur.children.length > 0) {
        e.preventDefault();
        openSubFor(cur.id, true);
        return true;
      }
      return false;
    }
    if (e.key === 'ArrowLeft' && isSubmenu) {
      e.preventDefault();
      onClose('esc');
      return true;
    }
    if (e.key === 'Enter' || e.key === ' ') {
      const cur = activeIdx >= 0 ? actions[activeIdx] : undefined;
      if (!cur) return false;
      e.preventDefault();
      if (!cur.disabled) {
        pickAction(cur);
      }
      return true;
    }
    if (e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
      typeBufRef.current += e.key.toLowerCase();
      if (typeTimerRef.current !== null) window.clearTimeout(typeTimerRef.current);
      typeTimerRef.current = window.setTimeout(() => {
        typeBufRef.current = '';
        typeTimerRef.current = null;
      }, TYPEAHEAD_RESET_MS);
      const q = typeBufRef.current;
      const start = activeIdx >= 0 ? activeIdx : -1;
      for (let step = 1; step <= actions.length; step += 1) {
        const idx = (start + step) % actions.length;
        const item = actions[idx];
        if (item?.label.toLowerCase().startsWith(q)) {
          setActiveIdx(idx);
          return true;
        }
      }
    }
    return false;
  };

  const anchor = isElementAnchor ? at : pointAnchor;
  const placement = isSubmenu ? 'right-start' : isElementAnchor ? 'bottom-start' : 'right-start';
  const offset = isSubmenu ? -4 : isElementAnchor ? 4 : 0;

  const activeAction = activeIdx >= 0 ? actions[activeIdx] : undefined;
  const activeOptionId = activeAction ? `${menuId}-${activeAction.id}` : undefined;
  const openSubmenu = subState
    ? actions.find((a) => a.id === subState.id && a.children && a.children.length > 0)
    : undefined;

  return (
    <Popover
      anchor={anchor}
      placement={placement}
      offset={offset}
      open={open}
      onClose={onClose}
      padding="sm"
      role="menu"
      initialFocus="container"
      {...(restoreFocus !== undefined ? { restoreFocus } : {})}
      {...(activeOptionId !== undefined ? { ariaActiveDescendant: activeOptionId } : {})}
      onKey={handleKey}
      ariaLabel={resolvedAriaLabel}
      className="hx-menu"
    >
      <div
        className="hx-menu__list"
        onPointerEnter={() => {
          onPointerEnterMenu?.();
        }}
        onPointerLeave={() => {
          if (subState !== null) {
            scheduleSubClose();
          }
          onPointerLeaveMenu?.();
        }}
      >
        {normalized.map((item, idx) => {
          if (item === 'sep') {
            return <div key={`sep-${String(idx)}`} className="hx-menu__sep" role="separator" />;
          }
          if (!isAction(item)) {
            return (
              <div key={`grp-${item.group}-${String(idx)}`} className="hx-menu__group" role="presentation">
                {item.group}
              </div>
            );
          }

          const actionIndex = actions.indexOf(item);
          const isHighlighted = actionIndex === activeIdx;
          const hasSub = Boolean(item.children && item.children.length > 0);
          const rowClasses = [
            'hx-menu__item',
            isHighlighted ? 'is-active' : '',
            item.disabled ? 'is-disabled' : '',
            item.danger ? 'hx-menu__item--danger' : '',
            item.model ? 'hx-menu__item--model' : '',
          ]
            .filter(Boolean)
            .join(' ');

          const rowEl = (
            <div
              key={item.id}
              id={`${menuId}-${item.id}`}
              ref={(el) => {
                if (el) itemElsRef.current.set(item.id, el);
                else itemElsRef.current.delete(item.id);
              }}
              className={rowClasses}
              role={item.checked !== undefined ? 'menuitemcheckbox' : 'menuitem'}
              aria-checked={item.checked !== undefined ? item.checked : undefined}
              aria-disabled={item.disabled ? 'true' : undefined}
              aria-haspopup={hasSub ? 'menu' : undefined}
              aria-expanded={hasSub ? subState?.id === item.id : undefined}
              onPointerEnter={() => {
                setActiveIdx(actionIndex);
                cancelSubTimers();
                if (hasSub && !item.disabled) {
                  openTimerRef.current = window.setTimeout(() => {
                    openTimerRef.current = null;
                    openSubFor(item.id, false);
                  }, SUBMENU_OPEN_DELAY);
                } else if (subState !== null) {
                  scheduleSubClose();
                }
              }}
              onPointerLeave={() => {
                if (hasSub && subState?.id === item.id) {
                  scheduleSubClose();
                }
              }}
              onPointerDown={(e) => {
                e.preventDefault();
              }}
              onClick={() => {
                pickAction(item);
              }}
            >
              <span className="hx-menu__lead" aria-hidden="true">
                {item.checked ? (
                  <Icon name="check" size={14} />
                ) : item.icon ? (
                  <Icon name={item.icon} size={16} />
                ) : null}
              </span>
              <span className="hx-menu__label">{item.label}</span>
              {hasSub ? (
                <span className="hx-menu__trail">
                  <Icon name="chevr" size={14} label={t('ui.submenu')} />
                </span>
              ) : item.shortcut ? (
                <span className="hx-menu__trail">
                  <Kbd text={item.shortcut} variant="inline" />
                </span>
              ) : null}
            </div>
          );

          if (item.disabled && item.disabledReason) {
            return (
              <Tooltip key={item.id} content={item.disabledReason} placement="right">
                {rowEl}
              </Tooltip>
            );
          }
          return rowEl;
        })}
      </div>
      {openSubmenu && subState ? (
        <MenuInner
          at={subState.anchor}
          items={openSubmenu.children ?? []}
          onPick={onPick}
          onClose={() => {
            setSubState(null);
          }}
          onPickRoot={onPickRoot ?? onPick}
          onCloseRoot={onCloseRoot ?? onClose}
          onPointerEnterMenu={cancelSubTimers}
          onPointerLeaveMenu={scheduleSubClose}
          open
          isSubmenu
          autoHighlightFirst={subState.autoHighlight}
          ariaLabel={openSubmenu.label}
        />
      ) : null}
    </Popover>
  );
}

export function Menu(p: MenuProps): JSX.Element | null {
  return <MenuInner {...p} />;
}
