import { createBrowserRouter, createMemoryRouter, type RouteObject } from 'react-router';
import { FallbackRoute, HomeRoute, SessionRoute, WelcomeRoute } from './SessionRoute';

export function createAppRoutes(): RouteObject[] {
  return [
    { path: '/', element: <HomeRoute /> },
    { path: '/welcome', element: <WelcomeRoute /> },
    { path: '/c/:sessionId', element: <SessionRoute /> },
    { path: '/c/:sessionId/g/:graphId', element: <SessionRoute /> },
    { path: '*', element: <FallbackRoute /> },
  ];
}

export function createAppRouter(): ReturnType<typeof createBrowserRouter> {
  return createBrowserRouter(createAppRoutes());
}

export function createTestRouter(initialEntries: string[]): ReturnType<typeof createMemoryRouter> {
  return createMemoryRouter(createAppRoutes(), { initialEntries });
}

export {
  createLocationSaver,
  createNavHistoryTracker,
  isValidUlid,
  parseUrl,
  resetNavHistoryTracker,
  toUrl,
  useNav,
  type Location,
  type LocationSaver,
  type NavApi,
  type NavHistoryTracker,
} from './nav';
export {
  FallbackRoute,
  HomeRoute,
  RouteServicesProvider,
  SessionRoute,
  useRouteServices,
  WelcomeRoute,
  type RouteServices,
  type RouteSessionSummary,
  type RouteShellProps,
} from './SessionRoute';
