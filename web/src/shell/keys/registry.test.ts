import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { ESLint } from 'eslint';
import { describe, expect, test, vi } from 'vitest';
import { expectConsole } from '@tests/kit';
import { createKeyRegistry, validateShortcuts, type ShortcutDef } from './registry';

const here = dirname(fileURLToPath(import.meta.url));
const webDir = resolve(here, '..', '..', '..');

function makeKeyEvent(
  type: 'keydown' | 'keyup',
  init: KeyboardEventInit & { readonly code: string; readonly key: string },
  target?: EventTarget,
): KeyboardEvent {
  const ev = new KeyboardEvent(type, { bubbles: true, cancelable: true, ...init });
  if (target) {
    Object.defineProperty(ev, 'target', { value: target, configurable: true });
  }
  return ev;
}

describe('W.02.04 快捷键注册中心', () => {
  test('W0204-T2 冲突检查：内置 44 条全通过，且 §6.3 五项各反例均被拦下', () => {
    // 1) 真实内置表全过
    expect(validateShortcuts()).toEqual([]);

    // 2) 同层重复
    const dupLayer: ShortcutDef[] = [
      {
        id: 'a',
        keys: ['Mod+K'],
        layer: 'global',
        inInput: true,
        hold: false,
        label: 'shortcut.palette.open',
        group: '全局',
      },
      {
        id: 'b',
        keys: ['Mod+K'],
        layer: 'global',
        inInput: true,
        hold: false,
        label: 'shortcut.center.open',
        group: '全局',
      },
    ];
    expect(validateShortcuts(dupLayer).some((m) => m.includes('duplicate combo in layer global'))).toBe(true);

    // 3) 保留键 Mod+W（§10 验收 8）
    const reserved: ShortcutDef[] = [
      {
        id: 'palette.open',
        keys: ['Mod+W'],
        layer: 'global',
        inInput: true,
        hold: false,
        label: 'shortcut.palette.open',
        group: '全局',
      },
    ];
    expect(validateShortcuts(reserved).some((m) => m.includes('reserved shortcut key used: Mod+W'))).toBe(
      true,
    );

    // 4) 单键与输入框：global 层裸单键（含字母与无修饰命名键 Escape/Delete/ArrowUp/F2）不可 in_input=true（K309 D03）
    const bareInInput: ShortcutDef[] = [
      {
        id: 'focus.toggle',
        keys: ['F'],
        layer: 'global',
        inInput: true,
        hold: false,
        label: 'shortcut.focus.toggle',
        group: '全局',
      },
      {
        id: 'bad.named',
        keys: ['Escape', 'Delete', 'ArrowUp', 'F2'],
        layer: 'global',
        inInput: true,
        hold: false,
        label: 'shortcut.focus.toggle',
        group: '全局',
      },
    ];
    const bareErrs = validateShortcuts(bareInInput);
    expect(bareErrs.some((m) => m.includes(': F '))).toBe(true);
    expect(bareErrs.some((m) => m.includes(': Escape '))).toBe(true);
    expect(bareErrs.some((m) => m.includes(': Delete '))).toBe(true);
    expect(bareErrs.some((m) => m.includes(': ArrowUp '))).toBe(true);
    expect(bareErrs.some((m) => m.includes(': F2 '))).toBe(true);

    // 5) 文案缺失
    const missingLabel: ShortcutDef[] = [
      {
        id: 'palette.open',
        keys: ['Mod+K'],
        layer: 'global',
        inInput: true,
        hold: false,
        label: 'shortcut.nonexistent',
        group: '全局',
      },
    ];
    expect(validateShortcuts(missingLabel).some((m) => m.includes('missing i18n label key'))).toBe(true);

    // 6) 与 §4.1 表不一致
    const unknownId: ShortcutDef[] = [
      {
        id: 'extra.unknown',
        keys: ['Mod+J'],
        layer: 'global',
        inInput: true,
        hold: false,
        label: 'shortcut.palette.open',
        group: '全局',
      },
    ];
    expect(validateShortcuts(unknownId).some((m) => m.includes('unexpected shortcut id'))).toBe(true);
  });

  test('W0204-T3 输入框：焦点在输入框时单键不拦截，Mod+K / Mod+. / Mod+, 生效', () => {
    const reg = createKeyRegistry({ isMac: () => false });
    const input = document.createElement('textarea');
    document.body.appendChild(input);

    const onQuickAdd = vi.fn(() => true);
    const onFocus = vi.fn(() => true);
    const onLibSearch = vi.fn(() => true);
    const onFit = vi.fn(() => true);
    const onPalette = vi.fn(() => true);
    const onEstop = vi.fn(() => true);
    const onCenter = vi.fn(() => true);

    reg.on('node.quickAdd', onQuickAdd);
    reg.on('focus.toggle', onFocus);
    reg.on('lib.search', onLibSearch);
    reg.on('view.fit', onFit);
    reg.on('palette.open', onPalette);
    reg.on('estop.trigger', onEstop);
    reg.on('center.open', onCenter);

    try {
      expect(reg.dispatchEvent(makeKeyEvent('keydown', { code: 'KeyA', key: 'a' }, input), 'down')).toBe(
        false,
      );
      expect(reg.dispatchEvent(makeKeyEvent('keydown', { code: 'KeyF', key: 'f' }, input), 'down')).toBe(
        false,
      );
      expect(reg.dispatchEvent(makeKeyEvent('keydown', { code: 'Slash', key: '/' }, input), 'down')).toBe(
        false,
      );
      expect(reg.dispatchEvent(makeKeyEvent('keydown', { code: 'Digit1', key: '1' }, input), 'down')).toBe(
        false,
      );
      expect(onQuickAdd).not.toHaveBeenCalled();
      expect(onFocus).not.toHaveBeenCalled();
      expect(onLibSearch).not.toHaveBeenCalled();
      expect(onFit).not.toHaveBeenCalled();

      expect(
        reg.dispatchEvent(makeKeyEvent('keydown', { code: 'KeyK', key: 'k', ctrlKey: true }, input), 'down'),
      ).toBe(true);
      expect(
        reg.dispatchEvent(
          makeKeyEvent('keydown', { code: 'Period', key: '.', ctrlKey: true }, input),
          'down',
        ),
      ).toBe(true);
      expect(
        reg.dispatchEvent(makeKeyEvent('keydown', { code: 'Comma', key: ',', ctrlKey: true }, input), 'down'),
      ).toBe(true);
      expect(onPalette).toHaveBeenCalledTimes(1);
      expect(onEstop).toHaveBeenCalledTimes(1);
      expect(onCenter).toHaveBeenCalledTimes(1);
    } finally {
      input.remove();
    }
  });

  test('W0204-T4 组字：isComposing=true 或 keyCode=229 的 Enter 不分发', () => {
    const reg = createKeyRegistry({ isMac: () => false });
    const composer = document.createElement('div');
    composer.setAttribute('data-hx-composer', 'true');
    const ta = document.createElement('textarea');
    composer.appendChild(ta);
    document.body.appendChild(composer);

    const onSend = vi.fn(() => true);
    reg.on('composer.send', onSend);

    try {
      const ev1 = makeKeyEvent('keydown', { code: 'Enter', key: 'Enter', isComposing: true }, ta);
      expect(reg.dispatchEvent(ev1, 'down')).toBe(false);
      expect(onSend).not.toHaveBeenCalled();

      const ev2 = makeKeyEvent('keydown', { code: 'Enter', key: 'Enter' }, ta);
      Object.defineProperty(ev2, 'keyCode', { value: 229 });
      expect(reg.dispatchEvent(ev2, 'down')).toBe(false);
      expect(onSend).not.toHaveBeenCalled();

      const ev3 = makeKeyEvent('keydown', { code: 'Enter', key: 'Enter', isComposing: false }, ta);
      expect(reg.dispatchEvent(ev3, 'down')).toBe(true);
      expect(onSend).toHaveBeenCalledTimes(1);
    } finally {
      composer.remove();
    }
  });

  test('W0204-T5 分层：Esc 依次关弹层→取消选中→返回上一层；modal 拦截 [ 但允许急停穿透', () => {
    const reg = createKeyRegistry({ isMac: () => false });
    const events: string[] = [];
    let hasSel = true;

    let popPopover: (() => void) | null = null;
    popPopover = reg.pushOverlay('popover', (e) => {
      if (e.key === 'Escape') {
        events.push('close-menu');
        popPopover?.();
        return true;
      }
      return false;
    });

    reg.on('canvas.escape', () => {
      if (hasSel) {
        hasSel = false;
        events.push('clear-sel');
      } else {
        events.push('nest-up');
      }
      return true;
    });

    const onLsToggle = vi.fn(() => true);
    const onEstop = vi.fn(() => true);
    reg.on('ls.toggle', onLsToggle);
    reg.on('estop.trigger', onEstop);

    // 1) Esc 依次关菜单 → 清选中 → 返回上一层
    reg.dispatchEvent(makeKeyEvent('keydown', { code: 'Escape', key: 'Escape' }), 'down');
    reg.dispatchEvent(makeKeyEvent('keydown', { code: 'Escape', key: 'Escape' }), 'down');
    reg.dispatchEvent(makeKeyEvent('keydown', { code: 'Escape', key: 'Escape' }), 'down');
    expect(events).toEqual(['close-menu', 'clear-sel', 'nest-up']);

    // 2) modal 打开时按 [ 不触发左栏，但 Mod+. 急停仍生效
    const popModal = reg.pushOverlay('modal', () => false);
    expect(reg.dispatchEvent(makeKeyEvent('keydown', { code: 'BracketLeft', key: '[' }), 'down')).toBe(false);
    expect(onLsToggle).not.toHaveBeenCalled();

    expect(
      reg.dispatchEvent(makeKeyEvent('keydown', { code: 'Period', key: '.', ctrlKey: true }), 'down'),
    ).toBe(true);
    expect(onEstop).toHaveBeenCalledTimes(1);
    popModal();
  });

  test('W0204-T6 hold 补发：按住 Space 后窗口 blur 补发一次 up', () => {
    const reg = createKeyRegistry({ isMac: () => false });
    const phases: Array<'down' | 'up'> = [];
    reg.on('tool.pan.hold', (_e, phase) => {
      phases.push(phase);
      return true;
    });

    const detach = reg.attach(window);
    try {
      window.dispatchEvent(
        new KeyboardEvent('keydown', { code: 'Space', key: ' ', bubbles: true, cancelable: true }),
      );
      expect(phases).toEqual(['down']);

      window.dispatchEvent(new Event('blur'));
      expect(phases).toEqual(['down', 'up']);
    } finally {
      detach();
    }
  });

  test('W0204-T8 处理函数异常：抛错捕获记 console.error 且不落到下一层，ESLint 禁止外部监听 keydown', async () => {
    const reg = createKeyRegistry({ isMac: () => false });
    const lowerHandler = vi.fn(() => true);

    reg.on('canvas.escape', lowerHandler);
    const pop = reg.pushOverlay('popover', () => {
      throw new Error('boom in overlay');
    });

    expectConsole('error', /shortcut handler threw/, 1);
    const handled = reg.dispatchEvent(makeKeyEvent('keydown', { code: 'Escape', key: 'Escape' }), 'down');
    expect(handled).toBe(true);
    expect(lowerHandler).not.toHaveBeenCalled();
    pop();

    // 验证 ESLint 规则拦截 shell/keys/ 以外监听 keydown（使用非 type-checked 配置，支持虚拟文件路径）
    const { createConfig } = (await import('../../../eslint.config.js')) as {
      createConfig: (opts?: { enableTypeChecked?: boolean }) => unknown[];
    };
    const eslint = new ESLint({
      cwd: webDir,
      overrideConfigFile: true,
      overrideConfig: createConfig({ enableTypeChecked: false }) as never,
    });
    const [badRes] = await eslint.lintText("window.addEventListener('keydown', () => {});\n", {
      filePath: resolve(webDir, 'src', 'panels', 'bad.ts'),
    });
    expect(
      badRes?.messages.some((m) => m.ruleId === 'no-restricted-syntax' && m.message.includes('W.02.04')),
    ).toBe(true);

    // K309 D04：onkeydown 赋值同样被拦
    const [badAssign] = await eslint.lintText('window.onkeydown = () => {};\n', {
      filePath: resolve(webDir, 'src', 'panels', 'bad2.ts'),
    });
    expect(
      badAssign?.messages.some((m) => m.ruleId === 'no-restricted-syntax' && m.message.includes('W.02.04')),
    ).toBe(true);
  }, 15000);
});
