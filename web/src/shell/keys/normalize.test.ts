import { describe, expect, test } from 'vitest';
import { formatShortcutCombo, normalizeKeyEvent } from './normalize';

describe('W.02.04 normalize', () => {
  test('W0204-T1 规范化：按物理 code 规范化 Alt+N，Windows Ctrl+K 与 macOS Meta+K 均为 Mod+K', () => {
    // macOS 按 Alt+N 时 e.key 为特殊字符 '˜'，但 e.code 仍是 KeyN
    expect(
      normalizeKeyEvent(
        { code: 'KeyN', key: '˜', altKey: true, ctrlKey: false, metaKey: false, shiftKey: false },
        true,
      ),
    ).toBe('Alt+N');

    // Windows Ctrl+K → Mod+K
    expect(
      normalizeKeyEvent(
        { code: 'KeyK', key: 'k', altKey: false, ctrlKey: true, metaKey: false, shiftKey: false },
        false,
      ),
    ).toBe('Mod+K');

    // macOS Meta+K → Mod+K
    expect(
      normalizeKeyEvent(
        { code: 'KeyK', key: 'k', altKey: false, ctrlKey: false, metaKey: true, shiftKey: false },
        true,
      ),
    ).toBe('Mod+K');

    // 符号键与顺序：Mod+Shift+G、Shift+/、Alt+[
    expect(
      normalizeKeyEvent(
        { code: 'KeyG', key: 'G', altKey: false, ctrlKey: true, metaKey: false, shiftKey: true },
        false,
      ),
    ).toBe('Mod+Shift+G');
    expect(
      normalizeKeyEvent(
        { code: 'Slash', key: '?', altKey: false, ctrlKey: false, metaKey: false, shiftKey: true },
        false,
      ),
    ).toBe('Shift+/');
    expect(
      normalizeKeyEvent(
        { code: 'BracketLeft', key: '[', altKey: true, ctrlKey: false, metaKey: false, shiftKey: false },
        false,
      ),
    ).toBe('Alt+[');
  });

  test('W0204-T7 label：macOS 显示 ⌘K，Windows 显示 Ctrl+K', () => {
    expect(formatShortcutCombo('Mod+K', true)).toBe('⌘K');
    expect(formatShortcutCombo('Mod+K', false)).toBe('Ctrl+K');
    expect(formatShortcutCombo('Mod+Shift+Z', true)).toBe('⌘⇧Z');
    expect(formatShortcutCombo('Mod+Shift+Z', false)).toBe('Ctrl+Shift+Z');
    expect(formatShortcutCombo('Alt+ArrowUp', true)).toBe('⌥↑');
    expect(formatShortcutCombo('Alt+ArrowUp', false)).toBe('Alt+↑');
  });
});
