import { afterEach, beforeEach, vi } from 'vitest';
import { setClock } from '@/lib/i18n';
import { installConsoleGuard, resetConsoleExpectations, verifyConsoleExpectations } from './console';
import { installDomStubs, resetDomStubs } from './dom';
import { cleanupRendered } from './render';
import { resetFakeTime } from './time';

// 启用 React 19 act() 测试环境标记
(globalThis as Record<string, unknown>)['IS_REACT_ACT_ENVIRONMENT'] = true;

installDomStubs();
installConsoleGuard();

function blockedNetwork(api: string): never {
  throw new Error(`单元测试禁止访问真实网络（${api}）；需要模拟请求请显式注入假 fetch（W.12 §6.2、INV-21）`);
}

function installNetworkGuard(): void {
  Object.defineProperty(globalThis, 'fetch', {
    configurable: true,
    writable: true,
    value: (input: unknown) => {
      blockedNetwork(`fetch(${typeof input === 'string' ? input : 'request'})`);
    },
  });
  Object.defineProperty(globalThis, 'EventSource', {
    configurable: true,
    writable: true,
    value: function BlockedEventSource(url: string) {
      blockedNetwork(`EventSource(${url})`);
    },
  });
  Object.defineProperty(globalThis, 'WebSocket', {
    configurable: true,
    writable: true,
    value: function BlockedWebSocket(url: string) {
      blockedNetwork(`WebSocket(${url})`);
    },
  });
}

installNetworkGuard();

beforeEach(() => {
  resetConsoleExpectations();
  installNetworkGuard();
});

afterEach(() => {
  cleanupRendered();
  if (typeof document !== 'undefined') {
    document.body.innerHTML = '';
    Object.defineProperty(document, 'hidden', { configurable: true, value: false });
  }
  if (typeof localStorage !== 'undefined') {
    localStorage.clear();
  }
  if (typeof sessionStorage !== 'undefined') {
    sessionStorage.clear();
  }
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
  resetFakeTime();
  setClock(null);
  resetDomStubs();
  installNetworkGuard();
  const consoleErrs = verifyConsoleExpectations();
  if (consoleErrs.length > 0) {
    throw new Error(`控制台输出不符合预期：\n  - ${consoleErrs.join('\n  - ')}`);
  }
});
