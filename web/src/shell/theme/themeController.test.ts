import { readFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createElement } from 'react';
import { describe, expect, test, vi } from 'vitest';
import { act, expectConsole, fakeRaf, memStorage, render, useFakeTime } from '@tests/kit';
import { createThemeController } from './themeController';
import { useTheme } from './useTheme';

const here = dirname(fileURLToPath(import.meta.url));
const webRoot = resolve(here, '..', '..', '..');

interface FakeMql {
  matches: boolean;
  trigger: (next: boolean) => void;
  mql: MediaQueryList;
}

function createFakeMediaMap(initial: { dark: boolean; reduced: boolean }) {
  const make = (query: string, init: boolean): FakeMql => {
    const target = new EventTarget();
    let current = init;
    Object.defineProperties(target, {
      media: { value: query, enumerable: true },
      onchange: { value: null, writable: true, enumerable: true },
      matches: {
        get() {
          return current;
        },
        enumerable: true,
      },
    });
    return {
      get matches() {
        return current;
      },
      set matches(v: boolean) {
        current = v;
      },
      trigger(next: boolean) {
        current = next;
        target.dispatchEvent(new Event('change'));
      },
      mql: target as unknown as MediaQueryList,
    };
  };

  const dark = make('(prefers-color-scheme: dark)', initial.dark);
  const reduced = make('(prefers-reduced-motion: reduce)', initial.reduced);

  const matchMedia = vi.fn((q: string): MediaQueryList => {
    if (q.includes('prefers-color-scheme')) return dark.mql;
    return reduced.mql;
  });

  return { dark, reduced, matchMedia };
}

describe('W.02.03 themeController 与首屏主题', () => {
  test('W0203-T1 解析：mode=system 跟随系统明暗，motion=auto 跟随系统减少动效', () => {
    const storage = memStorage();
    storage.setItem('hx:theme-mode', 'system');
    storage.setItem('hx:motion', 'auto');

    const media = createFakeMediaMap({ dark: true, reduced: true });
    const spyMatch = vi.spyOn(window, 'matchMedia').mockImplementation(media.matchMedia);

    const ctrl = createThemeController({ storage, win: window, doc: document });
    try {
      expect(ctrl.get().resolved).toBe('dark');
      expect(ctrl.get().reducedMotion).toBe(true);
      expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
      expect(document.documentElement.getAttribute('data-motion')).toBe('reduce');

      // 系统切回浅色与正常动效
      media.dark.trigger(false);
      media.reduced.trigger(false);
      expect(ctrl.get().resolved).toBe('light');
      expect(ctrl.get().reducedMotion).toBe(false);
      expect(document.documentElement.getAttribute('data-theme')).toBe('light');
      expect(document.documentElement.hasAttribute('data-motion')).toBe(false);
    } finally {
      ctrl.dispose();
      spyMatch.mockRestore();
    }
  });

  test('W0203-T2 非法本地值：hx:theme-palette="xyz" 回退宣纸 paper，boot.js <= 1KB', () => {
    const storage = memStorage();
    storage.setItem('hx:theme-palette', 'xyz');
    storage.setItem('hx:theme-mode', 'invalid');
    storage.setItem('hx:motion', 'invalid');

    const ctrl = createThemeController({ storage, win: window, doc: document });
    try {
      expect(ctrl.get().palette).toBe('paper');
      expect(ctrl.get().mode).toBe('light');
      expect(ctrl.get().motion).toBe('auto');
      expect(document.documentElement.getAttribute('data-palette')).toBe('paper');
    } finally {
      ctrl.dispose();
    }

    const bootPath = resolve(webRoot, 'public', 'boot.js');
    const bootBytes = readFileSync(bootPath);
    expect(bootBytes.byteLength).toBeLessThanOrEqual(1024);
  });

  test('W0203-T3 切换无过渡：切换期间存在 data-theme-switching，两帧后移除且 base.css 定义时长 0s', () => {
    const storage = memStorage();
    const raf = fakeRaf();
    const spyRaf = vi.spyOn(window, 'requestAnimationFrame').mockImplementation((cb) => raf.raf(cb));
    const spyCaf = vi.spyOn(window, 'cancelAnimationFrame').mockImplementation((id) => {
      raf.caf(id);
    });

    const ctrl = createThemeController({ storage, win: window, doc: document });
    try {
      ctrl.setPalette('whale');
      expect(document.documentElement.hasAttribute('data-theme-switching')).toBe(true);
      expect(document.documentElement.getAttribute('data-palette')).toBe('whale');

      raf.step();
      expect(document.documentElement.hasAttribute('data-theme-switching')).toBe(true);

      raf.step();
      expect(document.documentElement.hasAttribute('data-theme-switching')).toBe(false);
    } finally {
      ctrl.dispose();
      spyRaf.mockRestore();
      spyCaf.mockRestore();
    }

    const baseCss = readFileSync(resolve(webRoot, 'src', 'styles', 'base.css'), 'utf8');
    expect(baseCss).toContain('html[data-theme-switching]');
    expect(baseCss).toContain('--hx-t-1: 0s');
    expect(baseCss).toContain('--hx-t-4: 0s');
  });

  test('W0203-T4 服务端为准：本地 paper、服务端 whale → 切到 whale 并写回 localStorage', () => {
    const storage = memStorage();
    storage.setItem('hx:theme-palette', 'paper');
    const patch = vi.fn().mockResolvedValue({});
    const ctrl = createThemeController({ storage, win: window, doc: document, patchSettings: patch });
    try {
      ctrl.syncFromServer({ ui: { theme: { palette: 'whale', mode: 'dark' } } });
      expect(ctrl.get().palette).toBe('whale');
      expect(ctrl.get().mode).toBe('dark');
      expect(ctrl.get().resolved).toBe('dark');
      expect(storage.getItem('hx:theme-palette')).toBe('whale');
      expect(storage.getItem('hx:theme-mode')).toBe('dark');
      expect(patch).not.toHaveBeenCalled();

      // 服务端返回未知配色时回退 paper 并警告一次
      expectConsole('warn', /unknown server palette/, 1);
      ctrl.syncFromServer({ ui: { theme: { palette: 'neon-future' } } });
      expect(ctrl.get().palette).toBe('paper');
    } finally {
      ctrl.dispose();
    }
  });

  test('W0203-T5 PATCH 合并：300ms 内连续切换 3 次只发 1 次 PATCH', async () => {
    const clock = useFakeTime();
    const storage = memStorage();
    const patch = vi.fn().mockResolvedValue({});
    const ctrl = createThemeController({ storage, win: window, doc: document, patchSettings: patch });
    try {
      ctrl.setPalette('graphite');
      await clock.advance(100);
      ctrl.setMode('dark');
      await clock.advance(100);
      ctrl.setPalette('whale');
      await clock.advance(299);
      expect(patch).not.toHaveBeenCalled();

      await clock.advance(1);
      expect(patch).toHaveBeenCalledTimes(1);
      expect(patch).toHaveBeenCalledWith({
        'ui.theme.palette': 'whale',
        'ui.theme.mode': 'dark',
      });
    } finally {
      ctrl.dispose();
    }
  });

  test('W0203-T6 storage 同步不重复保存：另一标签页切换后本页同步但不发 PATCH', async () => {
    const clock = useFakeTime();
    const storage = memStorage();
    const patch = vi.fn().mockResolvedValue({});
    const listener = vi.fn();
    const ctrl = createThemeController({ storage, win: window, doc: document, patchSettings: patch });
    ctrl.subscribe(listener);

    try {
      storage.setItem('hx:theme-palette', 'amber');
      window.dispatchEvent(new StorageEvent('storage', { key: 'hx:theme-palette', newValue: 'amber' }));

      expect(ctrl.get().palette).toBe('amber');
      expect(document.documentElement.getAttribute('data-palette')).toBe('amber');
      expect(listener).toHaveBeenCalledTimes(1);

      await clock.advance(500);
      expect(patch).not.toHaveBeenCalled();
    } finally {
      ctrl.dispose();
    }
  });

  test('useTheme 使用返回新对象的 selector 稳定渲染不触发无限循环（K309 D02）', () => {
    const storage = memStorage();
    const ctrl = createThemeController({ storage, win: window, doc: document });
    let renders = 0;

    function Probe() {
      renders += 1;
      const summary = useTheme((s) => ({ p: s.palette, r: s.resolved }), ctrl);
      return createElement('div', null, `${summary.p}:${summary.r}`);
    }

    try {
      const r = render(createElement(Probe));
      expect(r.container.textContent).toBe('paper:light');
      const countBefore = renders;

      act(() => {
        ctrl.setPalette('whale');
      });
      expect(r.container.textContent).toBe('whale:light');
      expect(renders).toBe(countBefore + 1);
      r.unmount();
    } finally {
      ctrl.dispose();
    }
  });
});
