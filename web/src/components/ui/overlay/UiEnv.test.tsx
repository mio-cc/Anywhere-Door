// W.08.04 浮层类单元测试（W0804-T1..T15）将在同目录组件测试文件中展开；本文件聚焦 UiEnv 与 fallback 行为
import { describe, expect, test } from 'vitest';
import { expectConsole, fakeEnv, render } from '@tests/kit';
import { FALLBACK_ENV, UiEnvProvider, useUiEnv } from './UiEnv';

function Probe() {
  const env = useUiEnv();
  return <div data-testid="probe">{env.reducedMotion() ? 'reduce' : 'normal'}</div>;
}

describe('W.08.04 UiEnv', () => {
  test('无 Provider 时返回 FALLBACK_ENV 并读 html[data-motion]', () => {
    document.documentElement.setAttribute('data-motion', 'reduce');
    try {
      const r = render(<Probe />);
      expect(r.getByText('reduce')).toBeTruthy();
      expectConsole('warn', /UiEnvProvider/, 1);
      const pop = FALLBACK_ENV.pushOverlay('popover', () => false);
      pop();
    } finally {
      document.documentElement.removeAttribute('data-motion');
    }
  });

  test('UiEnvProvider 透传 fakeEnv', () => {
    const fe = fakeEnv({ reducedMotion: true });
    try {
      const r = render(
        <UiEnvProvider value={fe.env}>
          <Probe />
        </UiEnvProvider>,
      );
      expect(r.getByText('reduce')).toBeTruthy();
    } finally {
      fe.cleanup();
    }
  });
});
