import { DEFAULT_PALETTE, THEMES } from '@/styles/themes';

export type PaletteKey = (typeof THEMES)[number]['key'];
export type ThemeMode = 'light' | 'dark' | 'system';
export type MotionPref = 'auto' | 'reduce';
export type ThemeChangeField = 'palette' | 'theme' | 'motion';

export interface ThemeState {
  readonly palette: PaletteKey;
  readonly mode: ThemeMode;
  readonly resolved: 'light' | 'dark';
  readonly motion: MotionPref;
  readonly reducedMotion: boolean;
}

/** 服务端设置中与主题相关的最小结构（K305：解耦尚未落地的 Settings 生成类型）。 */
export interface ThemeSettingsLike {
  readonly ui?: {
    readonly theme?: {
      readonly palette?: unknown;
      readonly mode?: unknown;
    };
  };
}

export type ThemePatchFn = (patch: Readonly<Record<string, string>>) => Promise<unknown>;

export type ThemeListener = (s: ThemeState, changed: ThemeChangeField[]) => void;

export interface ThemeControllerApi {
  get(): ThemeState;
  setPalette(k: PaletteKey): void;
  setMode(m: ThemeMode): void;
  setMotion(m: MotionPref): void;
  syncFromServer(settings: ThemeSettingsLike): void;
  subscribe(fn: ThemeListener): () => void;
  setPatchHandler(fn: ThemePatchFn | null): void;
  lastSaveFailed(): boolean;
  dispose(): void;
}

export interface CreateThemeControllerOptions {
  readonly doc?: Document;
  readonly win?: Window;
  readonly storage?: Storage;
  readonly patchSettings?: ThemePatchFn;
  readonly debounceMs?: number;
}

const KEY_PALETTE = 'hx:theme-palette';
const KEY_MODE = 'hx:theme-mode';
const KEY_MOTION = 'hx:motion';
const DEFAULT_DEBOUNCE_MS = 300;

const VALID_PALETTES = new Set<string>(THEMES.map((t) => t.key));

function isPaletteKey(v: unknown): v is PaletteKey {
  return typeof v === 'string' && VALID_PALETTES.has(v);
}

function isThemeMode(v: unknown): v is ThemeMode {
  return v === 'light' || v === 'dark' || v === 'system';
}

function isMotionPref(v: unknown): v is MotionPref {
  return v === 'auto' || v === 'reduce';
}

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
    // 忽略存储不可用或配额满（W.02.03 §8）
  }
}

function queryMatches(win: Window | undefined, query: string): boolean {
  if (!win || typeof win.matchMedia !== 'function') return false;
  try {
    return win.matchMedia(query).matches;
  } catch {
    return false;
  }
}

function readSystemDark(win: Window | undefined): boolean {
  return queryMatches(win, '(prefers-color-scheme: dark)');
}

function readSystemReducedMotion(win: Window | undefined): boolean {
  return queryMatches(win, '(prefers-reduced-motion: reduce)');
}

export function createThemeController(opts: CreateThemeControllerOptions = {}): ThemeControllerApi {
  const win = opts.win ?? (typeof window !== 'undefined' ? window : undefined);
  const doc = opts.doc ?? (typeof document !== 'undefined' ? document : undefined);
  const storage =
    opts.storage ??
    (() => {
      try {
        return win?.localStorage;
      } catch {
        return undefined;
      }
    })();
  const debounceMs = opts.debounceMs ?? DEFAULT_DEBOUNCE_MS;

  let patchHandler: ThemePatchFn | null = opts.patchSettings ?? null;
  let saveFailed = false;
  let warnedInvalidServerPalette = false;
  let patchTimer: number | null = null;
  let pendingPatch: Record<string, string> = {};
  let raf1: number | null = null;
  let raf2: number | null = null;
  const listeners = new Set<ThemeListener>();

  const parseLocalPalette = (): PaletteKey => {
    const raw = safeGet(storage, KEY_PALETTE);
    return isPaletteKey(raw) ? raw : DEFAULT_PALETTE;
  };

  const parseLocalMode = (): ThemeMode => {
    const raw = safeGet(storage, KEY_MODE);
    return isThemeMode(raw) ? raw : 'light';
  };

  const parseLocalMotion = (): MotionPref => {
    const raw = safeGet(storage, KEY_MOTION);
    return isMotionPref(raw) ? raw : 'auto';
  };

  const buildState = (palette: PaletteKey, mode: ThemeMode, motion: MotionPref): ThemeState => {
    const resolved: 'light' | 'dark' = mode === 'system' ? (readSystemDark(win) ? 'dark' : 'light') : mode;
    const reducedMotion = motion === 'reduce' ? true : readSystemReducedMotion(win);
    return { palette, mode, resolved, motion, reducedMotion };
  };

  let state: ThemeState = buildState(parseLocalPalette(), parseLocalMode(), parseLocalMotion());

  const cancelSwitchingRafs = (): void => {
    if (!win) return;
    if (raf1 !== null) {
      win.cancelAnimationFrame(raf1);
      raf1 = null;
    }
    if (raf2 !== null) {
      win.cancelAnimationFrame(raf2);
      raf2 = null;
    }
  };

  const writeDomAttrs = (s: ThemeState, withSwitchingGuard: boolean): void => {
    if (!doc) return;
    const root = doc.documentElement;
    if (withSwitchingGuard) {
      cancelSwitchingRafs();
      root.setAttribute('data-theme-switching', '');
    }
    root.setAttribute('data-palette', s.palette);
    root.setAttribute('data-theme', s.resolved);
    if (s.reducedMotion) {
      root.setAttribute('data-motion', 'reduce');
    } else {
      root.removeAttribute('data-motion');
    }
    if (withSwitchingGuard && win && typeof win.requestAnimationFrame === 'function') {
      raf1 = win.requestAnimationFrame(() => {
        raf1 = null;
        raf2 = win.requestAnimationFrame(() => {
          raf2 = null;
          root.removeAttribute('data-theme-switching');
        });
      });
    } else if (withSwitchingGuard) {
      root.removeAttribute('data-theme-switching');
    }
  };

  // 初始化同步一次 DOM 属性（无切换动画标记）
  writeDomAttrs(state, false);

  const schedulePatch = (fields: Record<string, string>): void => {
    if (!patchHandler) return;
    pendingPatch = { ...pendingPatch, ...fields };
    if (saveFailed) {
      pendingPatch = {
        'ui.theme.palette': state.palette,
        'ui.theme.mode': state.mode,
        ...pendingPatch,
      };
    }
    if (patchTimer !== null) {
      clearTimeout(patchTimer);
    }
    patchTimer = setTimeout(() => {
      patchTimer = null;
      const body = pendingPatch;
      pendingPatch = {};
      if (!patchHandler || Object.keys(body).length === 0) return;
      void patchHandler(body).then(
        () => {
          saveFailed = false;
        },
        () => {
          saveFailed = true;
        },
      );
    }, debounceMs) as unknown as number;
  };

  const emit = (next: ThemeState, changed: ThemeChangeField[]): void => {
    for (const fn of listeners) {
      fn(next, changed);
    }
  };

  const applyTransition = (
    nextPalette: PaletteKey,
    nextMode: ThemeMode,
    nextMotion: MotionPref,
    options: { readonly writeStorage: boolean; readonly patchServer: boolean },
  ): void => {
    const prev = state;
    const next = buildState(nextPalette, nextMode, nextMotion);
    const changed: ThemeChangeField[] = [];
    if (prev.palette !== next.palette) changed.push('palette');
    if (prev.mode !== next.mode || prev.resolved !== next.resolved) changed.push('theme');
    if (prev.motion !== next.motion || prev.reducedMotion !== next.reducedMotion) changed.push('motion');

    if (changed.length === 0) return;

    state = next;
    const themeOrPaletteChanged = changed.includes('palette') || changed.includes('theme');
    writeDomAttrs(next, themeOrPaletteChanged);

    if (options.writeStorage) {
      if (prev.palette !== next.palette) safeSet(storage, KEY_PALETTE, next.palette);
      if (prev.mode !== next.mode) safeSet(storage, KEY_MODE, next.mode);
      if (prev.motion !== next.motion) safeSet(storage, KEY_MOTION, next.motion);
    }

    if (options.patchServer) {
      const patchFields: Record<string, string> = {};
      if (prev.palette !== next.palette) patchFields['ui.theme.palette'] = next.palette;
      if (prev.mode !== next.mode) patchFields['ui.theme.mode'] = next.mode;
      if (Object.keys(patchFields).length > 0) {
        schedulePatch(patchFields);
      }
    }

    emit(next, changed);
  };

  const darkMql =
    win && typeof win.matchMedia === 'function' ? win.matchMedia('(prefers-color-scheme: dark)') : null;
  const motionMql =
    win && typeof win.matchMedia === 'function' ? win.matchMedia('(prefers-reduced-motion: reduce)') : null;

  const onDarkMediaChange = (): void => {
    if (state.mode !== 'system') return;
    applyTransition(state.palette, state.mode, state.motion, {
      writeStorage: false,
      patchServer: false,
    });
  };

  const onMotionMediaChange = (): void => {
    if (state.motion !== 'auto') return;
    applyTransition(state.palette, state.mode, state.motion, {
      writeStorage: false,
      patchServer: false,
    });
  };

  const onStorage = (e: StorageEvent): void => {
    if (e.key !== null && e.key !== KEY_PALETTE && e.key !== KEY_MODE && e.key !== KEY_MOTION) {
      return;
    }
    const nextPalette =
      e.key === KEY_PALETTE ? (isPaletteKey(e.newValue) ? e.newValue : DEFAULT_PALETTE) : parseLocalPalette();
    const nextMode = e.key === KEY_MODE ? (isThemeMode(e.newValue) ? e.newValue : 'light') : parseLocalMode();
    const nextMotion =
      e.key === KEY_MOTION ? (isMotionPref(e.newValue) ? e.newValue : 'auto') : parseLocalMotion();

    // 来自其他标签页的同步：更新本页但不重复 PATCH（W.02.03 §6.4）
    applyTransition(nextPalette, nextMode, nextMotion, {
      writeStorage: false,
      patchServer: false,
    });
  };

  darkMql?.addEventListener('change', onDarkMediaChange);
  motionMql?.addEventListener('change', onMotionMediaChange);
  win?.addEventListener('storage', onStorage);

  return {
    get(): ThemeState {
      return state;
    },

    setPalette(k: PaletteKey): void {
      if (!isPaletteKey(k)) return;
      applyTransition(k, state.mode, state.motion, { writeStorage: true, patchServer: true });
    },

    setMode(m: ThemeMode): void {
      if (!isThemeMode(m)) return;
      applyTransition(state.palette, m, state.motion, { writeStorage: true, patchServer: true });
    },

    setMotion(m: MotionPref): void {
      if (!isMotionPref(m)) return;
      // 减少动效只存本地，不发 PATCH（W.02.03 §5）
      applyTransition(state.palette, state.mode, m, { writeStorage: true, patchServer: false });
    },

    syncFromServer(settings: ThemeSettingsLike): void {
      const themeCfg = settings.ui?.theme;
      if (!themeCfg) return;

      let nextPalette = state.palette;
      if (themeCfg.palette !== undefined) {
        if (isPaletteKey(themeCfg.palette)) {
          nextPalette = themeCfg.palette;
        } else {
          nextPalette = DEFAULT_PALETTE;
          if (!warnedInvalidServerPalette) {
            warnedInvalidServerPalette = true;
            console.warn('[theme] unknown server palette:', themeCfg.palette);
          }
        }
      }

      const nextMode = themeCfg.mode !== undefined && isThemeMode(themeCfg.mode) ? themeCfg.mode : state.mode;

      applyTransition(nextPalette, nextMode, state.motion, {
        writeStorage: true,
        patchServer: false,
      });
    },

    subscribe(fn: ThemeListener): () => void {
      listeners.add(fn);
      return () => {
        listeners.delete(fn);
      };
    },

    setPatchHandler(fn: ThemePatchFn | null): void {
      patchHandler = fn;
    },

    lastSaveFailed(): boolean {
      return saveFailed;
    },

    dispose(): void {
      if (patchTimer !== null) {
        clearTimeout(patchTimer);
        patchTimer = null;
      }
      cancelSwitchingRafs();
      darkMql?.removeEventListener('change', onDarkMediaChange);
      motionMql?.removeEventListener('change', onMotionMediaChange);
      win?.removeEventListener('storage', onStorage);
      listeners.clear();
    },
  };
}

export const themeController: ThemeControllerApi = createThemeController();
