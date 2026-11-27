import { createStore } from 'zustand/vanilla';
import type { Ulid } from '@/lib/protocol';
import { bpOf, type Breakpoint, type ReadonlyStore, type Scope, type SelectionKey } from './types';

// —— W.02.02 §4.2 原样收编 ——
export interface ShellLayout {
  readonly lsOpen: boolean;
  readonly rsOpen: boolean;
  readonly rsTab: 'insp' | 'chat' | 'mon';
  readonly lsFold: Readonly<Record<'sess' | 'tree' | 'lib', boolean>>;
  readonly focus: null | { readonly ls: boolean; readonly rs: boolean };
  readonly bp: Breakpoint;
  readonly fitInsets: {
    readonly top: number;
    readonly right: number;
    readonly bottom: number;
    readonly left: number;
  };
}

export interface CanvasSelection {
  readonly scope: Scope;
  readonly nodeIds: readonly Ulid[];
  readonly edgeIds: readonly Ulid[];
  readonly noteIds: readonly Ulid[];
}

export interface MonitorPrefs {
  readonly range: 'session' | 'all';
  readonly status: 'all' | 'running' | 'pending' | 'failed' | 'done';
}

export type CenterSection = 'packages' | 'connections' | 'library' | 'settings';

export type NetState = 'online' | 'reconnecting' | 'down' | 'closed';

export interface ToastAction {
  readonly label: string;
  readonly run: () => void;
}

export interface Toast {
  readonly id: string;
  readonly level: 'info' | 'ok' | 'wait' | 'error' | 'model';
  readonly text: string;
  readonly detail?: string;
  readonly action?: ToastAction;
  readonly actions?: readonly ToastAction[];
  readonly sticky?: boolean;
  readonly groupKey?: string;
}

export interface UiState {
  readonly layout: ShellLayout;
  readonly selections: ReadonlyMap<SelectionKey, CanvasSelection>;
  readonly tool: 'select' | 'pan';
  readonly listView: boolean;
  readonly monitor: MonitorPrefs;
  readonly center: null | { readonly section: CenterSection; readonly item?: string };
  readonly net: NetState;
  readonly toasts: readonly Toast[];
  readonly composerPrefill: null | {
    readonly text: string;
    readonly attachSelection: boolean;
    readonly seq: number;
  };
  readonly history: null | { readonly sessionId: Ulid; readonly runId?: Ulid; readonly seq?: number };
  readonly tour: boolean;
  readonly libQuery: null | { readonly text: string; readonly seq: number };
}

export interface UiActions {
  setLs(open: boolean): void;
  setRs(open: boolean, tab?: ShellLayout['rsTab']): void;
  toggleFocus(): void;
  setLsFold(sec: keyof ShellLayout['lsFold'], folded: boolean): void;
  setBp(bp: ShellLayout['bp']): void;
  setFitInsets(i: ShellLayout['fitInsets']): void;
  select(key: SelectionKey, sel: CanvasSelection): void;
  clearSelection(key: SelectionKey, scope?: Scope): void;
  pruneSelection(key: SelectionKey, alive: (kind: 'node' | 'edge' | 'note', id: Ulid) => boolean): void;
  dropSelection(key: SelectionKey): void;
  setTool(t: UiState['tool']): void;
  setListView(on: boolean): void;
  setMonitor(p: Partial<MonitorPrefs>): void;
  openCenter(section: CenterSection, item?: string): void;
  closeCenter(): void;
  setNet(n: NetState): void;
  notify(t: Omit<Toast, 'id'>): string;
  dismiss(id: string): void;
  prefillComposer(p: { readonly text: string; readonly attachSelection?: boolean }): void;
  openHistory(h: { readonly sessionId: Ulid; readonly runId?: Ulid; readonly seq?: number }): void;
  closeHistory(): void;
  setTour(on: boolean): void;
  revealInLibrary(text: string): void;
}

export type UiStoreApi = ReadonlyStore<UiState> & { readonly actions: UiActions };

const KEY_LS_OPEN = 'hx:ls-open';
const KEY_RS_OPEN = 'hx:rs-open';
const KEY_RS_TAB = 'hx:rs-tab';
const KEY_LS_FOLD = 'hx:ls-fold';
const KEY_LIST_VIEW = 'hx:list-view';
const KEY_MON = 'hx:mon';
const KEY_TOUR = 'hx:tour';

const MAX_TOASTS = 20;
const MAX_ATTACH_NODES = 50;

const DEFAULT_LS_FOLD: ShellLayout['lsFold'] = { sess: false, tree: false, lib: true };
const DEFAULT_MON: MonitorPrefs = { range: 'session', status: 'all' };
const DEFAULT_FIT_INSETS: ShellLayout['fitInsets'] = { top: 28, right: 28, bottom: 44, left: 64 };

function safeGet(storage: Storage | undefined, key: string): string | null {
  if (!storage) return null;
  try {
    return storage.getItem(key);
  } catch {
    return null;
  }
}

function safeSet(storage: Storage | undefined, key: string, value: string): void {
  if (!storage) return;
  try {
    storage.setItem(key, value);
  } catch {
    // 隐私模式或配额满：只在内存生效（W.03.03 §8）
  }
}

function safeRemove(storage: Storage | undefined, key: string): void {
  if (!storage) return;
  try {
    storage.removeItem(key);
  } catch {
    // 忽略存储异常
  }
}

function readBoolFlag(storage: Storage | undefined, key: string, fallback: boolean): boolean {
  const raw = safeGet(storage, key);
  if (raw === '1') return true;
  if (raw === '0') return false;
  return fallback;
}

function readRsTab(storage: Storage | undefined): ShellLayout['rsTab'] {
  const raw = safeGet(storage, KEY_RS_TAB);
  if (raw === 'insp' || raw === 'chat' || raw === 'mon') return raw;
  return 'insp';
}

function readLsFold(storage: Storage | undefined): ShellLayout['lsFold'] {
  const raw = safeGet(storage, KEY_LS_FOLD);
  if (!raw) return DEFAULT_LS_FOLD;
  try {
    const parsed: unknown = JSON.parse(raw);
    if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
      return DEFAULT_LS_FOLD;
    }
    const rec = parsed as Record<string, unknown>;
    return {
      sess: typeof rec['sess'] === 'boolean' ? rec['sess'] : DEFAULT_LS_FOLD.sess,
      tree: typeof rec['tree'] === 'boolean' ? rec['tree'] : DEFAULT_LS_FOLD.tree,
      lib: typeof rec['lib'] === 'boolean' ? rec['lib'] : DEFAULT_LS_FOLD.lib,
    };
  } catch {
    return DEFAULT_LS_FOLD;
  }
}

function readMonitor(storage: Storage | undefined): MonitorPrefs {
  const raw = safeGet(storage, KEY_MON);
  if (!raw) return DEFAULT_MON;
  try {
    const parsed: unknown = JSON.parse(raw);
    if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
      return DEFAULT_MON;
    }
    const rec = parsed as Record<string, unknown>;
    const range = rec['range'] === 'session' || rec['range'] === 'all' ? rec['range'] : DEFAULT_MON.range;
    const st = rec['status'];
    const status =
      st === 'all' || st === 'running' || st === 'pending' || st === 'failed' || st === 'done'
        ? st
        : DEFAULT_MON.status;
    return { range, status };
  } catch {
    return DEFAULT_MON;
  }
}

function dedupeIds(ids: readonly Ulid[]): readonly Ulid[] {
  if (ids.length <= 1) return [...ids];
  const seen = new Set<Ulid>();
  const out: Ulid[] = [];
  for (const id of ids) {
    if (!seen.has(id)) {
      seen.add(id);
      out.push(id);
    }
  }
  return out;
}

function arraysEqual<T>(a: readonly T[], b: readonly T[]): boolean {
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i += 1) {
    if (a[i] !== b[i]) return false;
  }
  return true;
}

function selectionEqual(a: CanvasSelection | undefined, b: CanvasSelection): boolean {
  if (!a) return false;
  return (
    a.scope === b.scope &&
    arraysEqual(a.nodeIds, b.nodeIds) &&
    arraysEqual(a.edgeIds, b.edgeIds) &&
    arraysEqual(a.noteIds, b.noteIds)
  );
}

function defaultStorage(): Storage | undefined {
  if (typeof window === 'undefined') return undefined;
  try {
    return window.localStorage;
  } catch {
    return undefined;
  }
}

export function createUiStore(o: { readonly width: number; readonly storage?: Storage }): UiStoreApi {
  const storage = o.storage ?? defaultStorage();
  const initialLsDefault = o.width >= 900;

  const initialState: UiState = {
    layout: {
      lsOpen: readBoolFlag(storage, KEY_LS_OPEN, initialLsDefault),
      rsOpen: readBoolFlag(storage, KEY_RS_OPEN, false),
      rsTab: readRsTab(storage),
      lsFold: readLsFold(storage),
      focus: null,
      bp: bpOf(o.width),
      fitInsets: DEFAULT_FIT_INSETS,
    },
    selections: new Map<SelectionKey, CanvasSelection>(),
    tool: 'select',
    listView: readBoolFlag(storage, KEY_LIST_VIEW, false),
    monitor: readMonitor(storage),
    center: null,
    net: 'online',
    toasts: [],
    composerPrefill: null,
    history: null,
    tour: safeGet(storage, KEY_TOUR) === '1',
    libQuery: null,
  };

  const store = createStore<UiState>(() => initialState);
  let toastSeq = 0;
  let prefillSeq = 0;
  let libQuerySeq = 0;

  const actions: UiActions = {
    setLs(open: boolean): void {
      const cur = store.getState().layout;
      const exitingFocus = open && cur.focus !== null;
      if (!exitingFocus && cur.lsOpen === open) return;
      const next: ShellLayout = {
        ...cur,
        lsOpen: open,
        focus: exitingFocus ? null : cur.focus,
      };
      store.setState({ layout: next });
      safeSet(storage, KEY_LS_OPEN, open ? '1' : '0');
      // K309 D01：退出专注模式只打开左栏时，把右栏当前关闭态同步写盘，避免刷新后漂移
      if (exitingFocus) {
        safeSet(storage, KEY_RS_OPEN, next.rsOpen ? '1' : '0');
      }
    },

    setRs(open: boolean, tab?: ShellLayout['rsTab']): void {
      const cur = store.getState().layout;
      const nextTab = tab ?? cur.rsTab;
      const exitingFocus = open && cur.focus !== null;
      if (!exitingFocus && cur.rsOpen === open && cur.rsTab === nextTab) return;
      const next: ShellLayout = {
        ...cur,
        rsOpen: open,
        rsTab: nextTab,
        focus: exitingFocus ? null : cur.focus,
      };
      store.setState({ layout: next });
      safeSet(storage, KEY_RS_OPEN, open ? '1' : '0');
      if (exitingFocus) {
        safeSet(storage, KEY_LS_OPEN, next.lsOpen ? '1' : '0');
      }
      if (tab !== undefined) {
        safeSet(storage, KEY_RS_TAB, nextTab);
      }
    },

    toggleFocus(): void {
      const cur = store.getState().layout;
      if (cur.focus === null) {
        // 进入专注模式：记录原开合，两侧收起，不写 localStorage（W.03.03 §6.1）
        store.setState({
          layout: {
            ...cur,
            lsOpen: false,
            rsOpen: false,
            focus: { ls: cur.lsOpen, rs: cur.rsOpen },
          },
        });
      } else {
        store.setState({
          layout: {
            ...cur,
            lsOpen: cur.focus.ls,
            rsOpen: cur.focus.rs,
            focus: null,
          },
        });
      }
    },

    setLsFold(sec: keyof ShellLayout['lsFold'], folded: boolean): void {
      const cur = store.getState().layout;
      if (cur.lsFold[sec] === folded) return;
      const nextFold: ShellLayout['lsFold'] = { ...cur.lsFold, [sec]: folded };
      store.setState({ layout: { ...cur, lsFold: nextFold } });
      safeSet(storage, KEY_LS_FOLD, JSON.stringify(nextFold));
    },

    setBp(bp: ShellLayout['bp']): void {
      const cur = store.getState().layout;
      if (cur.bp === bp) return;
      store.setState({ layout: { ...cur, bp } });
    },

    setFitInsets(i: ShellLayout['fitInsets']): void {
      const cur = store.getState().layout;
      const f = cur.fitInsets;
      if (f.top === i.top && f.right === i.right && f.bottom === i.bottom && f.left === i.left) {
        return;
      }
      store.setState({
        layout: {
          ...cur,
          fitInsets: { top: i.top, right: i.right, bottom: i.bottom, left: i.left },
        },
      });
    },

    select(key: SelectionKey, sel: CanvasSelection): void {
      // W.03.03 §8：防御性校验非空 key
      if (!(key as unknown)) {
        if (import.meta.env.DEV) {
          throw new Error('uiStore.select called with null/empty SelectionKey');
        }
        return;
      }
      const normalized: CanvasSelection = {
        scope: sel.scope,
        nodeIds: dedupeIds(sel.nodeIds),
        edgeIds: dedupeIds(sel.edgeIds),
        noteIds: dedupeIds(sel.noteIds),
      };
      const curMap = store.getState().selections;
      if (selectionEqual(curMap.get(key), normalized)) return;
      const nextMap = new Map(curMap);
      nextMap.set(key, normalized);
      store.setState({ selections: nextMap });
    },

    clearSelection(key: SelectionKey, scope?: Scope): void {
      const curMap = store.getState().selections;
      const prev = curMap.get(key);
      const targetScope: Scope = scope ?? prev?.scope ?? 'root';
      const empty: CanvasSelection = { scope: targetScope, nodeIds: [], edgeIds: [], noteIds: [] };
      if (selectionEqual(prev, empty)) return;
      const nextMap = new Map(curMap);
      nextMap.set(key, empty);
      store.setState({ selections: nextMap });
    },

    pruneSelection(key: SelectionKey, alive: (kind: 'node' | 'edge' | 'note', id: Ulid) => boolean): void {
      const curMap = store.getState().selections;
      const prev = curMap.get(key);
      if (!prev) return;
      const nodeIds = prev.nodeIds.filter((id) => alive('node', id));
      const edgeIds = prev.edgeIds.filter((id) => alive('edge', id));
      const noteIds = prev.noteIds.filter((id) => alive('note', id));
      if (
        nodeIds.length === prev.nodeIds.length &&
        edgeIds.length === prev.edgeIds.length &&
        noteIds.length === prev.noteIds.length
      ) {
        return;
      }
      const nextMap = new Map(curMap);
      nextMap.set(key, { scope: prev.scope, nodeIds, edgeIds, noteIds });
      store.setState({ selections: nextMap });
    },

    dropSelection(key: SelectionKey): void {
      const curMap = store.getState().selections;
      if (!curMap.has(key)) return;
      const nextMap = new Map(curMap);
      nextMap.delete(key);
      store.setState({ selections: nextMap });
    },

    setTool(t: UiState['tool']): void {
      if (store.getState().tool === t) return;
      store.setState({ tool: t });
    },

    setListView(on: boolean): void {
      if (store.getState().listView === on) return;
      store.setState({ listView: on });
      safeSet(storage, KEY_LIST_VIEW, on ? '1' : '0');
    },

    setMonitor(p: Partial<MonitorPrefs>): void {
      const cur = store.getState().monitor;
      const next: MonitorPrefs = {
        range: p.range ?? cur.range,
        status: p.status ?? cur.status,
      };
      if (next.range === cur.range && next.status === cur.status) return;
      store.setState({ monitor: next });
      safeSet(storage, KEY_MON, JSON.stringify(next));
    },

    openCenter(section: CenterSection, item?: string): void {
      const cur = store.getState().center;
      if (cur && cur.section === section && cur.item === item) return;
      store.setState({
        center: item !== undefined ? { section, item } : { section },
      });
    },

    closeCenter(): void {
      if (store.getState().center === null) return;
      store.setState({ center: null });
    },

    setNet(n: NetState): void {
      if (store.getState().net === n) return;
      store.setState({ net: n });
    },

    notify(t: Omit<Toast, 'id'>): string {
      toastSeq += 1;
      const id = `toast-${String(toastSeq)}`;
      const entry: Toast = { ...t, id };
      const cur = store.getState().toasts;
      let next: Toast[];
      if (entry.groupKey !== undefined) {
        const idx = cur.findIndex((x) => x.groupKey === entry.groupKey);
        if (idx >= 0) {
          next = cur.slice();
          next[idx] = entry;
          store.setState({ toasts: next });
          return id;
        }
      }
      next = [...cur, entry];
      if (next.length > MAX_TOASTS) {
        next = next.slice(next.length - MAX_TOASTS);
      }
      store.setState({ toasts: next });
      return id;
    },

    dismiss(id: string): void {
      const cur = store.getState().toasts;
      const next = cur.filter((x) => x.id !== id);
      if (next.length === cur.length) return;
      store.setState({ toasts: next });
    },

    prefillComposer(p: { readonly text: string; readonly attachSelection?: boolean }): void {
      prefillSeq += 1;
      store.setState({
        composerPrefill: {
          text: p.text,
          attachSelection: p.attachSelection ?? false,
          seq: prefillSeq,
        },
      });
    },

    openHistory(h: { readonly sessionId: Ulid; readonly runId?: Ulid; readonly seq?: number }): void {
      store.setState({
        history: {
          sessionId: h.sessionId,
          ...(h.runId !== undefined ? { runId: h.runId } : {}),
          ...(h.seq !== undefined ? { seq: h.seq } : {}),
        },
      });
    },

    closeHistory(): void {
      if (store.getState().history === null) return;
      store.setState({ history: null });
    },

    setTour(on: boolean): void {
      if (store.getState().tour === on) return;
      store.setState({ tour: on });
      if (on) {
        safeSet(storage, KEY_TOUR, '1');
      } else {
        safeRemove(storage, KEY_TOUR);
      }
    },

    revealInLibrary(text: string): void {
      actions.setLs(true);
      actions.setLsFold('lib', false);
      libQuerySeq += 1;
      store.setState({ libQuery: { text, seq: libQuerySeq } });
    },
  };

  return {
    getState: store.getState,
    subscribe: store.subscribe,
    actions,
  };
}

export function selectionOf(s: UiState, key: SelectionKey | null): CanvasSelection | null {
  if (key === null) return null;
  return s.selections.get(key) ?? null;
}

export function rsDot(s: UiState, key: SelectionKey | null): boolean {
  if (s.layout.rsOpen) return false;
  const sel = selectionOf(s, key);
  if (!sel) return false;
  return sel.nodeIds.length > 0 || sel.edgeIds.length > 0 || sel.noteIds.length > 0;
}

export function attachable(sel: CanvasSelection | null): {
  readonly nodeIds: readonly Ulid[];
  readonly truncatedFrom?: number;
} {
  if (!sel || sel.nodeIds.length === 0) {
    return { nodeIds: [] };
  }
  if (sel.nodeIds.length > MAX_ATTACH_NODES) {
    return {
      nodeIds: sel.nodeIds.slice(0, MAX_ATTACH_NODES),
      truncatedFrom: sel.nodeIds.length,
    };
  }
  return { nodeIds: sel.nodeIds };
}

export function canvasReadonly(s: UiState): boolean {
  return s.net !== 'online';
}
