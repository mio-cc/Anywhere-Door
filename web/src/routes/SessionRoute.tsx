import {
  createContext,
  useContext,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  type JSX,
  type ReactNode,
} from 'react';
import { useLocation } from 'react-router';
import { useT } from '@/lib/i18n';
import type { Ulid } from '@/lib/protocol';
import type { Scope } from '@/state/types';
import { parseUrl, toUrl, useNav, type NavHistoryTracker } from './nav';

export interface RouteSessionSummary {
  readonly id: Ulid;
  readonly title: string;
  readonly activeGraphId?: Ulid;
  readonly graphIds?: readonly Ulid[];
  readonly lastLocation?: {
    readonly graphId: Ulid;
    readonly scope: Scope;
  };
}

export interface RouteShellProps {
  readonly mode: 'session' | 'page';
  readonly welcomePage?: boolean;
  readonly page?: ReactNode;
  readonly canvasOverlay?: ReactNode;
  readonly cover?: ReactNode;
}

export interface RouteServices {
  readonly onboardingDone: boolean;
  readonly sessions: readonly RouteSessionSummary[];
  readonly pendingApprovals?: number;
  readonly pendingTitlePrefix?: string;
  readonly storage?: Storage;
  readonly historyTracker?: NavHistoryTracker;
  readonly scopeExists?: (graphId: Ulid, scope: Scope) => boolean;
  readonly onActivate?: (loc: {
    readonly sessionId: Ulid;
    readonly graphId: Ulid | null;
    readonly scope: Scope;
  }) => void;
  readonly saveLastLocation?: (
    sessionId: Ulid,
    loc: { readonly graphId: Ulid; readonly scope: Scope },
  ) => void;
  readonly notify?: (text: string) => void;
  readonly onSessionOrGraphChange?: () => void;
  readonly renderShell?: (props: RouteShellProps) => JSX.Element;
  readonly renderHome?: () => ReactNode;
  readonly renderWelcome?: () => ReactNode;
  readonly renderCanvasOverlay?: () => ReactNode;
  readonly renderCover?: (sessionId: Ulid) => ReactNode;
}

const KEY_LAST_SESSION = 'hx:last-session';
const DEFAULT_TITLE = 'Anywhere Door';

const DEFAULT_SERVICES: RouteServices = {
  onboardingDone: true,
  sessions: [],
};

const RouteServicesContext = createContext<RouteServices>(DEFAULT_SERVICES);

export function RouteServicesProvider(p: {
  readonly value: RouteServices;
  readonly children: ReactNode;
}): JSX.Element {
  return <RouteServicesContext.Provider value={p.value}>{p.children}</RouteServicesContext.Provider>;
}

export function useRouteServices(): RouteServices {
  return useContext(RouteServicesContext);
}

function getStorage(svc: RouteServices): Storage | undefined {
  if (svc.storage) return svc.storage;
  if (typeof window === 'undefined') return undefined;
  try {
    return window.localStorage;
  } catch {
    return undefined;
  }
}

function safeReadLastSession(storage: Storage | undefined): string | null {
  if (!storage) return null;
  try {
    return storage.getItem(KEY_LAST_SESSION);
  } catch {
    return null;
  }
}

function safeWriteLastSession(storage: Storage | undefined, sid: Ulid): void {
  if (!storage) return;
  try {
    storage.setItem(KEY_LAST_SESSION, sid);
  } catch {
    // 忽略存储异常
  }
}

function renderDefaultShell(props: RouteShellProps): JSX.Element {
  return (
    <div data-hx-route-shell={props.mode} data-welcome={props.welcomePage ? 'true' : undefined}>
      {props.mode === 'page' ? props.page : null}
      {props.mode === 'session' ? (
        <>
          {props.canvasOverlay}
          {props.cover}
        </>
      ) : null}
    </div>
  );
}

export function HomeRoute(): JSX.Element {
  const svc = useRouteServices();
  const nav = useNav(svc.historyTracker);

  useEffect(() => {
    if (typeof document !== 'undefined') {
      document.title = DEFAULT_TITLE;
    }
    if (!svc.onboardingDone) {
      nav.replacePath('/welcome');
      return;
    }
    if (svc.sessions.length > 0) {
      const storage = getStorage(svc);
      const last = safeReadLastSession(storage);
      const matched = last ? svc.sessions.find((s) => s.id === last) : undefined;
      const target = matched ?? svc.sessions[0];
      if (target) {
        nav.replacePath(`/c/${target.id}`);
      }
    }
  }, [svc.onboardingDone, svc.sessions, svc.storage, nav]);

  const renderShell = svc.renderShell ?? renderDefaultShell;
  return renderShell({
    mode: 'session',
    canvasOverlay: svc.renderHome ? svc.renderHome() : null,
  });
}

export function WelcomeRoute(): JSX.Element {
  const svc = useRouteServices();
  const nav = useNav(svc.historyTracker);

  useEffect(() => {
    if (typeof document !== 'undefined') {
      document.title = DEFAULT_TITLE;
    }
    if (svc.onboardingDone) {
      nav.replacePath('/');
    }
  }, [svc.onboardingDone, nav]);

  const renderShell = svc.renderShell ?? renderDefaultShell;
  return renderShell({
    mode: 'page',
    welcomePage: true,
    page: svc.renderWelcome ? svc.renderWelcome() : null,
  });
}

export function FallbackRoute(): JSX.Element {
  const svc = useRouteServices();
  const nav = useNav(svc.historyTracker);

  useEffect(() => {
    if (typeof document !== 'undefined') {
      document.title = DEFAULT_TITLE;
    }
    nav.replacePath('/');
  }, [nav]);

  return <div />;
}

export function SessionRoute(): JSX.Element {
  const svc = useRouteServices();
  const nav = useNav(svc.historyTracker);
  const location = useLocation();
  const t = useT();
  const prevSgRef = useRef<string | null>(null);
  const lastActivatedKeyRef = useRef<string | null>(null);

  const svcRef = useRef(svc);
  const navRef = useRef(nav);
  const tRef = useRef(t);
  useLayoutEffect(() => {
    svcRef.current = svc;
    navRef.current = nav;
    tRef.current = t;
  });

  const fullPath = `${location.pathname}${location.search}`;
  // K309 D08：memo 化 parsed，避免每次 render 产生新对象触发 effect 重复激活/保存
  const parsed = useMemo(() => parseUrl(new URL(fullPath, 'http://localhost')), [fullPath]);

  const session = useMemo(
    () => (parsed?.sessionId ? svc.sessions.find((s) => s.id === parsed.sessionId) : undefined),
    [parsed, svc.sessions],
  );

  // K309 D06：归属图与非 root scope 严格校验，不静默放行
  const graphBelongs = useMemo(() => {
    if (!session || !parsed?.graphId) return false;
    if (session.graphIds !== undefined) {
      return session.graphIds.includes(parsed.graphId);
    }
    return session.activeGraphId === parsed.graphId;
  }, [session, parsed]);

  const scopeValid = useMemo(() => {
    if (!parsed?.graphId) return true;
    const sc: Scope = parsed.scope ?? 'root';
    if (sc === 'root') return true;
    if (!svc.scopeExists) return false;
    return svc.scopeExists(parsed.graphId, sc);
  }, [parsed, svc]);

  // 路由解析与激活副作用（与标题副作用拆分，K309 D08）
  useEffect(() => {
    const curSvc = svcRef.current;
    const curNav = navRef.current;

    if (!curSvc.onboardingDone) {
      lastActivatedKeyRef.current = null;
      curNav.replacePath('/welcome');
      return;
    }

    if (!parsed?.sessionId || !session) {
      lastActivatedKeyRef.current = null;
      curNav.replacePath('/');
      return;
    }

    safeWriteLastSession(getStorage(curSvc), session.id);

    if (!parsed.graphId) {
      if (session.activeGraphId) {
        const lastScope =
          session.lastLocation && session.lastLocation.graphId === session.activeGraphId
            ? session.lastLocation.scope
            : 'root';
        const targetScope: Scope =
          lastScope !== 'root' && curSvc.scopeExists && !curSvc.scopeExists(session.activeGraphId, lastScope)
            ? 'root'
            : lastScope;
        curNav.replaceLocation({ sessionId: session.id, graphId: session.activeGraphId, scope: targetScope });
        return;
      }
      const actKey = `${session.id}:none:root`;
      if (lastActivatedKeyRef.current === actKey) return;
      lastActivatedKeyRef.current = actKey;

      const sgKey = `${session.id}:none`;
      if (prevSgRef.current !== null && prevSgRef.current !== sgKey) {
        curSvc.onSessionOrGraphChange?.();
      }
      prevSgRef.current = sgKey;
      curSvc.onActivate?.({ sessionId: session.id, graphId: null, scope: 'root' });
      return;
    }

    if (!graphBelongs) {
      lastActivatedKeyRef.current = null;
      curNav.replacePath(`/c/${session.id}`);
      return;
    }

    const scope: Scope = parsed.scope ?? 'root';
    if (scope !== 'root' && !scopeValid) {
      lastActivatedKeyRef.current = null;
      curSvc.notify?.(tRef.current('common.scopeGone'));
      curNav.replaceLocation({ sessionId: session.id, graphId: parsed.graphId, scope: 'root' });
      return;
    }

    const actKey = `${session.id}:${parsed.graphId}:${scope}`;
    if (lastActivatedKeyRef.current === actKey) return;
    lastActivatedKeyRef.current = actKey;

    const sgKey = `${session.id}:${parsed.graphId}`;
    if (prevSgRef.current !== null && prevSgRef.current !== sgKey) {
      curSvc.onSessionOrGraphChange?.();
    }
    prevSgRef.current = sgKey;

    curSvc.onActivate?.({ sessionId: session.id, graphId: parsed.graphId, scope });
    curSvc.saveLastLocation?.(session.id, { graphId: parsed.graphId, scope });
  }, [svc.onboardingDone, parsed, session, graphBelongs, scopeValid]);

  // 页面标题独立 effect：仅随会话名与待审批数变化，不触发 onActivate/saveLastLocation
  useEffect(() => {
    if (typeof document === 'undefined' || !session) return;
    const prefix = (svc.pendingApprovals ?? 0) > 0 ? (svc.pendingTitlePrefix ?? '') : '';
    document.title = `${prefix}${session.title} \u00B7 Anywhere Door`;
  }, [session, svc.pendingApprovals, svc.pendingTitlePrefix]);

  const renderShell = svc.renderShell ?? renderDefaultShell;
  const sid = parsed?.sessionId;
  const cover = sid && svc.renderCover ? svc.renderCover(sid) : undefined;
  const canvasOverlay = svc.renderCanvasOverlay ? svc.renderCanvasOverlay() : undefined;

  return renderShell({
    mode: 'session',
    ...(canvasOverlay !== undefined ? { canvasOverlay } : {}),
    ...(cover !== undefined ? { cover } : {}),
  });
}

export { toUrl };
