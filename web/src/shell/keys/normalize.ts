/** 快捷键物理键位规范化与平台显示文案（W.02.04 §5.1、§6.4）。 */

export interface KeyEventLike {
  readonly code: string;
  readonly key: string;
  readonly ctrlKey: boolean;
  readonly metaKey: boolean;
  readonly altKey: boolean;
  readonly shiftKey: boolean;
  readonly isComposing?: boolean;
  readonly keyCode?: number;
}

const CODE_TO_KEY: Readonly<Record<string, string>> = {
  BracketLeft: '[',
  BracketRight: ']',
  Comma: ',',
  Period: '.',
  Slash: '/',
  Enter: 'Enter',
  NumpadEnter: 'Enter',
  Escape: 'Escape',
  Delete: 'Delete',
  Backspace: 'Backspace',
  Space: 'Space',
  Tab: 'Tab',
  ContextMenu: 'ContextMenu',
  ArrowUp: 'ArrowUp',
  ArrowDown: 'ArrowDown',
  ArrowLeft: 'ArrowLeft',
  ArrowRight: 'ArrowRight',
};

const MODIFIER_CODES = new Set([
  'ControlLeft',
  'ControlRight',
  'MetaLeft',
  'MetaRight',
  'AltLeft',
  'AltRight',
  'ShiftLeft',
  'ShiftRight',
]);

export function detectMac(nav?: {
  readonly platform?: string;
  readonly userAgent?: string;
  readonly userAgentData?: { readonly platform?: string };
}): boolean {
  const target = nav ?? (typeof navigator !== 'undefined' ? navigator : undefined);
  if (!target) return false;
  const uaPlatform = (target as { readonly userAgentData?: { readonly platform?: string } }).userAgentData
    ?.platform;
  if (typeof uaPlatform === 'string' && uaPlatform.length > 0) {
    return /mac/i.test(uaPlatform);
  }
  if (typeof target.platform === 'string' && target.platform.length > 0) {
    return /mac|iphone|ipad|ipod/i.test(target.platform);
  }
  if (typeof target.userAgent === 'string') {
    return /mac os x|macintosh/i.test(target.userAgent);
  }
  return false;
}

export function isComposingEvent(e: KeyEventLike): boolean {
  if (e.isComposing === true) return true;
  // 兼容旧版浏览器输入法组字 keyCode 229（W.02.04 §6.2 第 1 步）
  const raw = (e as unknown as { readonly keyCode?: unknown }).keyCode;
  return raw === 229;
}

export function codeToMainKey(code: string, fallbackKey = ''): string | null {
  if (MODIFIER_CODES.has(code)) return null;
  if (code.length === 4 && code.startsWith('Key')) {
    return code.slice(3).toUpperCase();
  }
  if (code.length === 6 && code.startsWith('Digit')) {
    return code.slice(5);
  }
  if (/^F([1-9]|1[0-2])$/.test(code)) {
    return code;
  }
  const mapped = CODE_TO_KEY[code];
  if (mapped !== undefined) return mapped;

  // 合成测试事件可能未传 code，仅传 key 时做降级映射
  if (code === '' && fallbackKey.length > 0) {
    if (fallbackKey === ' ') return 'Space';
    if (fallbackKey.length === 1) return fallbackKey.toUpperCase();
    return fallbackKey;
  }
  return null;
}

/**
 * 把 KeyboardEvent 规范成固定顺序组合串：Mod / Ctrl / Alt / Shift + 主键（W.02.04 §5.1）。
 * 物理键位优先使用 `e.code`，避免 macOS 按住 Alt 产生特殊字符。
 */
export function normalizeKeyEvent(e: KeyEventLike, isMac: boolean = detectMac()): string | null {
  const main = codeToMainKey(e.code, e.key);
  if (main === null) return null;

  const parts: string[] = [];
  const hasMod = isMac ? e.metaKey : e.ctrlKey;
  const hasCtrl = isMac ? e.ctrlKey : false;

  if (hasMod) parts.push('Mod');
  if (hasCtrl) parts.push('Ctrl');
  if (e.altKey) parts.push('Alt');
  if (e.shiftKey) parts.push('Shift');
  parts.push(main);
  return parts.join('+');
}

const DISPLAY_MAIN_MAC: Readonly<Record<string, string>> = {
  ArrowUp: '↑',
  ArrowDown: '↓',
  ArrowLeft: '←',
  ArrowRight: '→',
  Enter: '↵',
  Escape: 'Esc',
  Backspace: '⌫',
  Delete: '⌦',
  Space: 'Space',
  Tab: 'Tab',
};

const DISPLAY_MAIN_OTHER: Readonly<Record<string, string>> = {
  ArrowUp: '↑',
  ArrowDown: '↓',
  ArrowLeft: '←',
  ArrowRight: '→',
  Enter: 'Enter',
  Escape: 'Esc',
  Backspace: 'Backspace',
  Delete: 'Del',
  Space: 'Space',
  Tab: 'Tab',
};

/** 将快捷键组合串格式化为用户可读按键文字（W.02.04 §6.4、U.13.04-R5）。 */
export function formatShortcutCombo(combo: string, isMac: boolean = detectMac()): string {
  const parts = combo.split('+');
  const main = parts[parts.length - 1] ?? '';
  const mods = parts.slice(0, -1);

  if (isMac) {
    let prefix = '';
    for (const m of mods) {
      if (m === 'Mod') prefix += '⌘';
      else if (m === 'Ctrl') prefix += '⌃';
      else if (m === 'Alt') prefix += '⌥';
      else if (m === 'Shift') prefix += '⇧';
    }
    const keyText = DISPLAY_MAIN_MAC[main] ?? main;
    return `${prefix}${keyText}`;
  }

  const out: string[] = [];
  for (const m of mods) {
    if (m === 'Mod' || m === 'Ctrl') out.push('Ctrl');
    else if (m === 'Alt') out.push('Alt');
    else if (m === 'Shift') out.push('Shift');
  }
  out.push(DISPLAY_MAIN_OTHER[main] ?? main);
  return out.join('+');
}
