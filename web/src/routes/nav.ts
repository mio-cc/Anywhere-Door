import { useCallback, useEffect, useMemo } from 'react';
import { useLocation, useNavigate, useNavigationType } from 'react-router';
import type { Ulid } from '@/lib/protocol';
import type { Scope } from '@/state/types';

export interface Location {
  readonly sessionId: Ulid;
  readonly graphId: Ulid;
  readonly scope: Scope;
}

export interface NavApi {
  enterLayer(containerId: Ulid): void;
  leaveTo(scope: Scope): void;
  switchSession(sessionId: Ulid): void;
  switchGraph(graphId: Ulid): void;
  replaceLocation(loc: Location): void;
  replacePath(path: string): void;
}

const ULID_RE = /^[0-9A-HJKMNP-TV-Z]{26}$/;
const SESSION_PATH_RE = /^\/c\/([0-9A-HJKMNP-TV-Z]{26})$/;
const GRAPH_PATH_RE = /^\/c\/([0-9A-HJKMNP-TV-Z]{26})\/g\/([0-9A-HJKMNP-TV-Z]{26})$/;

export function isValidUlid(s: string): s is Ulid {
  return ULID_RE.test(s);
}

/** 将会话、图与层位置编码为 URL，运行时严格校验 ULID 与 scope（W.02.01 §4.2、K309 D05）。 */
export function toUrl(loc: Location): string {
  const sid: string = loc.sessionId;
  const gid: string = loc.graphId;
  const sc: string = loc.scope;
  if (!isValidUlid(sid)) {
    throw new RangeError(`Invalid sessionId ULID: ${sid}`);
  }
  if (!isValidUlid(gid)) {
    throw new RangeError(`Invalid graphId ULID: ${gid}`);
  }
  if (sc !== 'root' && !isValidUlid(sc)) {
    throw new RangeError(`Invalid scope: ${sc}`);
  }
  const base = `/c/${sid}/g/${gid}`;
  if (sc === 'root') return base;
  return `${base}?scope=${sc}`;
}

/**
 * 严格解析 URL 为位置对象（W.02.01 §4.2、§5、K309 D05）。
 * 拒绝空路径段、尾斜杠、空 scope、重复 scope 与非法 ULID。
 */
export function parseUrl(url: URL): Partial<Location> | null {
  const scopeValues = url.searchParams.getAll('scope');

  const sessMatch = SESSION_PATH_RE.exec(url.pathname);
  if (sessMatch) {
    if (scopeValues.length > 0) return null;
    const rawSession = sessMatch[1];
    if (!rawSession || !isValidUlid(rawSession)) return null;
    return { sessionId: rawSession };
  }

  const graphMatch = GRAPH_PATH_RE.exec(url.pathname);
  if (graphMatch) {
    const rawSession = graphMatch[1];
    const rawGraph = graphMatch[2];
    if (!rawSession || !isValidUlid(rawSession) || !rawGraph || !isValidUlid(rawGraph)) {
      return null;
    }
    if (scopeValues.length === 0) {
      return { sessionId: rawSession, graphId: rawGraph, scope: 'root' };
    }
    if (scopeValues.length > 1) return null;
    const rawScope = scopeValues[0];
    if (rawScope === undefined || rawScope === '') return null;
    if (rawScope === 'root') {
      return { sessionId: rawSession, graphId: rawGraph, scope: 'root' };
    }
    if (!isValidUlid(rawScope)) return null;
    return { sessionId: rawSession, graphId: rawGraph, scope: rawScope };
  }

  return null;
}

/** 内存中的历史地址栈：供 leaveTo 判断目标是否等于上一条记录（W.02.01 §6.2）。 */
export interface NavHistoryTracker {
  sync(key: string, fullPath: string, navType: 'POP' | 'PUSH' | 'REPLACE'): void;
  previousPath(): string | null;
  reset(): void;
  snapshot(): { readonly entries: readonly string[]; readonly index: number };
}

interface StackEntry {
  readonly key: string;
  readonly path: string;
}

export function createNavHistoryTracker(): NavHistoryTracker {
  let stack: StackEntry[] = [];
  let index = -1;

  return {
    sync(key: string, fullPath: string, navType: 'POP' | 'PUSH' | 'REPLACE'): void {
      if (stack.length === 0) {
        stack = [{ key, path: fullPath }];
        index = 0;
        return;
      }
      const cur = stack[index];
      if (cur && cur.key === key && cur.path === fullPath) {
        return;
      }
      if (navType === 'PUSH') {
        stack = [...stack.slice(0, index + 1), { key, path: fullPath }];
        index = stack.length - 1;
        return;
      }
      if (navType === 'REPLACE') {
        const next = stack.slice();
        next[index] = { key, path: fullPath };
        stack = next;
        return;
      }
      // POP：按 key 查找既有位置；找不到时回退按相邻路径匹配
      const byKey = stack.findIndex((e) => e.key === key);
      if (byKey >= 0) {
        index = byKey;
        return;
      }
      if (index > 0 && stack[index - 1]?.path === fullPath) {
        index -= 1;
        return;
      }
      if (index + 1 < stack.length && stack[index + 1]?.path === fullPath) {
        index += 1;
        return;
      }
      stack = [{ key, path: fullPath }];
      index = 0;
    },

    previousPath(): string | null {
      if (index <= 0) return null;
      return stack[index - 1]?.path ?? null;
    },

    reset(): void {
      stack = [];
      index = -1;
    },

    snapshot(): { readonly entries: readonly string[]; readonly index: number } {
      return { entries: stack.map((e) => e.path), index };
    },
  };
}

const defaultHistoryTracker = createNavHistoryTracker();

export function resetNavHistoryTracker(): void {
  defaultHistoryTracker.reset();
}

/** 1 秒尾随节流的位置保存器（W.02.01 §6.3 第 2 步、C.10-R3）。 */
export interface LocationSaver {
  save(sessionId: Ulid, loc: { readonly graphId: Ulid; readonly scope: Scope }): void;
  flush(): void;
  dispose(): void;
}

export function createLocationSaver(
  patch: (sessionId: Ulid, loc: { readonly graphId: Ulid; readonly scope: Scope }) => unknown,
  delayMs = 1000,
): LocationSaver {
  const timers = new Map<Ulid, number>();
  const pending = new Map<Ulid, { readonly graphId: Ulid; readonly scope: Scope }>();

  const runFor = (sid: Ulid): void => {
    const tId = timers.get(sid);
    if (tId !== undefined) {
      clearTimeout(tId);
      timers.delete(sid);
    }
    const loc = pending.get(sid);
    if (!loc) return;
    pending.delete(sid);
    try {
      const res = patch(sid, loc);
      if (res instanceof Promise) {
        void res.catch(() => undefined);
      }
    } catch {
      // W.02.01 §8：保存现场失败不提示，下一次层变化时重试
    }
  };

  return {
    save(sessionId: Ulid, loc: { readonly graphId: Ulid; readonly scope: Scope }): void {
      pending.set(sessionId, loc);
      const prev = timers.get(sessionId);
      if (prev !== undefined) {
        clearTimeout(prev);
      }
      const nextTimer = setTimeout(() => {
        runFor(sessionId);
      }, delayMs) as unknown as number;
      timers.set(sessionId, nextTimer);
    },

    flush(): void {
      for (const sid of Array.from(pending.keys())) {
        runFor(sid);
      }
    },

    dispose(): void {
      for (const tId of timers.values()) {
        clearTimeout(tId);
      }
      timers.clear();
      pending.clear();
    },
  };
}

export function useNav(tracker: NavHistoryTracker = defaultHistoryTracker): NavApi {
  const navigate = useNavigate();
  const location = useLocation();
  const navType = useNavigationType();

  const fullPath = `${location.pathname}${location.search}`;
  useEffect(() => {
    tracker.sync(location.key, fullPath, navType);
  }, [tracker, location.key, fullPath, navType]);

  const currentLoc = useMemo(() => {
    const parsed = parseUrl(new URL(fullPath, 'http://localhost'));
    return parsed;
  }, [fullPath]);

  const enterLayer = useCallback(
    (containerId: Ulid): void => {
      if (!currentLoc?.sessionId || !currentLoc.graphId) return;
      const target = toUrl({
        sessionId: currentLoc.sessionId,
        graphId: currentLoc.graphId,
        scope: containerId,
      });
      if (target === fullPath) return;
      void navigate(target);
    },
    [currentLoc, fullPath, navigate],
  );

  const leaveTo = useCallback(
    (scope: Scope): void => {
      if (!currentLoc?.sessionId || !currentLoc.graphId) return;
      const target = toUrl({
        sessionId: currentLoc.sessionId,
        graphId: currentLoc.graphId,
        scope,
      });
      if (target === fullPath) return;
      if (tracker.previousPath() === target) {
        void navigate(-1);
        return;
      }
      void navigate(target);
    },
    [currentLoc, fullPath, navigate, tracker],
  );

  const switchSession = useCallback(
    (sessionId: Ulid): void => {
      const target = `/c/${sessionId}`;
      if (location.pathname === target && location.search === '') return;
      void navigate(target);
    },
    [location.pathname, location.search, navigate],
  );

  const switchGraph = useCallback(
    (graphId: Ulid): void => {
      if (!currentLoc?.sessionId) return;
      const target = toUrl({
        sessionId: currentLoc.sessionId,
        graphId,
        scope: 'root',
      });
      if (target === fullPath) return;
      void navigate(target);
    },
    [currentLoc, fullPath, navigate],
  );

  const replaceLocation = useCallback(
    (loc: Location): void => {
      const target = toUrl(loc);
      if (target === fullPath) return;
      void navigate(target, { replace: true });
    },
    [fullPath, navigate],
  );

  const replacePath = useCallback(
    (path: string): void => {
      if (path === fullPath) return;
      void navigate(path, { replace: true });
    },
    [fullPath, navigate],
  );

  return useMemo(
    () => ({
      enterLayer,
      leaveTo,
      switchSession,
      switchGraph,
      replaceLocation,
      replacePath,
    }),
    [enterLayer, leaveTo, switchSession, switchGraph, replaceLocation, replacePath],
  );
}
