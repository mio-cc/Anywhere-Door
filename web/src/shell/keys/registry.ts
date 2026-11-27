import { SHORTCUTS } from '@/lib/protocol';
import { hasText, t, type TextKey } from '@/lib/i18n';
import { detectMac, formatShortcutCombo, isComposingEvent, normalizeKeyEvent } from './normalize';

export type ShortcutId = (typeof SHORTCUTS)[number]['id'];
export type ShortcutLayer = 'global' | 'canvas' | 'nest' | 'composer';
export type KeyHandler = (e: KeyboardEvent, phase: 'down' | 'up') => boolean;
export type Overlay = 'modal' | 'popover' | 'peek';

export interface ShortcutDef {
  readonly id: string;
  readonly keys: readonly string[];
  readonly layer: string;
  readonly inInput: boolean;
  readonly hold: boolean;
  readonly label: string;
  readonly group: string;
}

export interface ShortcutListItem {
  readonly id: ShortcutId;
  readonly keys: string[];
  readonly group: string;
  readonly label: string;
}

export interface ShortcutRegistryApi {
  on(id: ShortcutId, handler: KeyHandler, opts?: { readonly when?: () => boolean }): () => void;
  pushOverlay(kind: Overlay, handler: (e: KeyboardEvent) => boolean): () => void;
  label(id: ShortcutId, isMac?: boolean): string;
  list(isMac?: boolean): ShortcutListItem[];
  attach(target: Window): () => void;
  setComposerCheck(fn: (target: EventTarget | null) => boolean): void;
  dispatchEvent(e: KeyboardEvent, phase: 'down' | 'up'): boolean;
  releaseHeld(): void;
}

export const RESERVED_SHORTCUTS = new Set<string>([
  'Mod+W',
  'Mod+T',
  'Mod+N',
  'Mod+L',
  'Mod+R',
  'Mod+Shift+[',
  'Mod+Shift+]',
  'F5',
  'F11',
  'F12',
]);

export const EXPECTED_SHORTCUT_IDS: readonly string[] = [
  'palette.open',
  'center.open',
  'run.start',
  'estop.trigger',
  'ls.toggle',
  'rs.toggle',
  'rs.insp',
  'focus.toggle',
  'lib.search',
  'help.keys',
  'session.prev',
  'session.next',
  'session.new',
  'tool.select',
  'tool.pan',
  'tool.pan.hold',
  'node.quickAdd',
  'sel.delete',
  'sel.all',
  'sel.duplicate',
  'clip.copy',
  'clip.paste',
  'group',
  'ungroup',
  'undo',
  'redo',
  'view.fit',
  'view.focusSel',
  'layout.tidy',
  'node.rename',
  'sel.move',
  'sel.moveFar',
  'focus.nextNode',
  'focus.prevNode',
  'link.keyboard',
  'menu.context',
  'nest.peek',
  'nest.enter',
  'nest.up',
  'canvas.escape',
  'composer.send',
  'composer.newline',
  'composer.editLast',
  'composer.blur',
];

const REPEAT_ALLOWED_COMBOS = new Set<string>([
  'ArrowUp',
  'ArrowDown',
  'ArrowLeft',
  'ArrowRight',
  'Shift+ArrowUp',
  'Shift+ArrowDown',
  'Shift+ArrowLeft',
  'Shift+ArrowRight',
  'Mod+Z',
  'Mod+Shift+Z',
]);

function readBuiltinShortcuts(): readonly ShortcutDef[] {
  return SHORTCUTS.map((entry) => ({
    id: entry.id,
    keys: entry.keys,
    layer: entry.layer,
    inInput: entry['in_input'],
    hold: entry.hold,
    label: entry.label,
    group: entry.group,
  }));
}

// K309 D03：任何不带修饰键（无 '+'）的单键（含字母、数字、符号及 Escape/Delete/Backspace/方向键/F 键/Space）均为裸单键
function isBareSingleKey(combo: string): boolean {
  return combo.length > 0 && !combo.includes('+');
}

/**
 * 构建期与单元测试快捷键表冲突检查（W.02.04 §6.3 五项全检）。
 * 返回所有违规描述，空数组表示全部通过。
 */
export function validateShortcuts(
  defs: readonly ShortcutDef[] = readBuiltinShortcuts(),
  hasLabelKey: (key: string) => boolean = hasText,
): string[] {
  const errors: string[] = [];
  const seenLayerCombo = new Map<string, string>();
  const seenIds = new Set<string>();

  for (const def of defs) {
    if (seenIds.has(def.id)) {
      errors.push(`duplicate shortcut id: ${def.id}`);
    }
    seenIds.add(def.id);

    if (!hasLabelKey(def.label)) {
      errors.push(`missing i18n label key: ${def.label} (id=${def.id})`);
    }

    for (const combo of def.keys) {
      if (RESERVED_SHORTCUTS.has(combo)) {
        errors.push(`reserved shortcut key used: ${combo} (id=${def.id})`);
      }

      const lcKey = `${def.layer}::${combo}`;
      const prevId = seenLayerCombo.get(lcKey);
      if (prevId !== undefined && prevId !== def.id) {
        errors.push(`duplicate combo in layer ${def.layer}: ${combo} (${prevId} vs ${def.id})`);
      } else {
        seenLayerCombo.set(lcKey, def.id);
      }

      if (def.layer !== 'composer' && def.inInput && isBareSingleKey(combo)) {
        errors.push(`bare single key cannot have in_input=true outside composer: ${combo} (id=${def.id})`);
      }
    }
  }

  const expectedSet = new Set(EXPECTED_SHORTCUT_IDS);
  for (const id of expectedSet) {
    if (!seenIds.has(id)) {
      errors.push(`missing expected shortcut id: ${id}`);
    }
  }
  for (const id of seenIds) {
    if (!expectedSet.has(id)) {
      errors.push(`unexpected shortcut id not in W.02.04 §4.1: ${id}`);
    }
  }

  return errors;
}

function isEditableTarget(target: EventTarget | null): boolean {
  if (!target || typeof HTMLElement === 'undefined' || !(target instanceof HTMLElement)) {
    return false;
  }
  const tag = target.tagName;
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') {
    return true;
  }
  if (target.isContentEditable) return true;
  return target.closest('[contenteditable="true"], [role="textbox"]') !== null;
}

function defaultComposerCheck(target: EventTarget | null): boolean {
  if (!target || typeof HTMLElement === 'undefined' || !(target instanceof HTMLElement)) {
    return false;
  }
  return target.closest('[data-hx-composer="true"]') !== null;
}

interface Binding {
  readonly id: string;
  readonly handler: KeyHandler;
  readonly when?: () => boolean;
}

interface OverlayEntry {
  readonly kind: Overlay;
  readonly handler: (e: KeyboardEvent) => boolean;
}

export function createKeyRegistry(options?: {
  readonly defs?: readonly ShortcutDef[];
  readonly isMac?: () => boolean;
}): ShortcutRegistryApi {
  const defs = options?.defs ?? readBuiltinShortcuts();
  const getIsMac = options?.isMac ?? (() => detectMac());

  const defById = new Map<string, ShortcutDef>();
  const lookup = new Map<string, ShortcutDef>();
  for (const d of defs) {
    defById.set(d.id, d);
    for (const combo of d.keys) {
      lookup.set(`${d.layer}::${combo}`, d);
    }
  }

  const bindings = new Map<string, Binding[]>();
  const overlays: Record<Overlay, OverlayEntry[]> = {
    modal: [],
    popover: [],
    peek: [],
  };
  const heldBindings = new Map<string, Binding>();
  let composerCheck: (target: EventTarget | null) => boolean = defaultComposerCheck;
  let warnedUnknownId = false;

  const invokeHandler = (fn: () => boolean): boolean => {
    try {
      return fn();
    } catch (err) {
      console.error('[keys] shortcut handler threw:', err);
      // W.02.04 §8：抛异常视为已处理，不继续落到下一层
      return true;
    }
  };

  const tryLayerCombo = (
    layer: ShortcutLayer,
    combo: string,
    e: KeyboardEvent,
    phase: 'down' | 'up',
  ): boolean => {
    const def = lookup.get(`${layer}::${combo}`);
    if (!def) return false;
    if (phase === 'up' && !def.hold) return false;

    const list = bindings.get(def.id);
    if (!list || list.length === 0) return false;

    for (let i = list.length - 1; i >= 0; i -= 1) {
      const b = list[i];
      if (!b) continue;
      if (b.when && !b.when()) continue;
      const handled = invokeHandler(() => b.handler(e, phase));
      if (handled) {
        if (def.hold) {
          if (phase === 'down') {
            heldBindings.set(def.id, b);
          } else {
            heldBindings.delete(def.id);
          }
        }
        return true;
      }
    }
    return false;
  };

  const tryInputAllowedCombo = (combo: string, e: KeyboardEvent, phase: 'down' | 'up'): boolean => {
    const order: readonly ShortcutLayer[] = ['nest', 'canvas', 'global'];
    for (const layer of order) {
      const def = lookup.get(`${layer}::${combo}`);
      if (!def || !def.inInput) continue;
      if (tryLayerCombo(layer, combo, e, phase)) return true;
    }
    return false;
  };

  const releaseHeld = (): void => {
    if (heldBindings.size === 0) return;
    const entries = Array.from(heldBindings.entries());
    heldBindings.clear();
    const synthetic = new KeyboardEvent('keyup', { key: ' ', code: 'Space' });
    for (const [, b] of entries) {
      invokeHandler(() => b.handler(synthetic, 'up'));
    }
  };

  const dispatchEvent = (e: KeyboardEvent, phase: 'down' | 'up'): boolean => {
    if (isComposingEvent(e)) return false;

    const combo = normalizeKeyEvent(e, getIsMac());
    if (combo === null) return false;

    if (phase === 'down' && e.repeat && !REPEAT_ALLOWED_COMBOS.has(combo)) {
      return false;
    }

    // keyup 只处理已按住的 hold 键，不走完整层分发
    if (phase === 'up') {
      const layers: readonly ShortcutLayer[] = ['composer', 'nest', 'canvas', 'global'];
      for (const layer of layers) {
        const def = lookup.get(`${layer}::${combo}`);
        if (def && def.hold && heldBindings.has(def.id)) {
          const b = heldBindings.get(def.id);
          heldBindings.delete(def.id);
          if (b) {
            const handled = invokeHandler(() => b.handler(e, 'up'));
            if (handled) {
              e.preventDefault();
              e.stopPropagation();
            }
            return handled;
          }
        }
      }
      return false;
    }

    // ① modal：栈顶先收；未处理则拦截（仅 estop.trigger 允许穿透，W.02.04 §6.1）
    if (overlays.modal.length > 0) {
      const top = overlays.modal[overlays.modal.length - 1];
      if (top && invokeHandler(() => top.handler(e))) {
        e.preventDefault();
        e.stopPropagation();
        return true;
      }
      const estopDef = defById.get('estop.trigger');
      if (estopDef && estopDef.keys.includes(combo) && tryLayerCombo('global', combo, e, 'down')) {
        e.preventDefault();
        e.stopPropagation();
        return true;
      }
      return false;
    }

    // ② popover：栈顶先收，未处理继续往下
    if (overlays.popover.length > 0) {
      const top = overlays.popover[overlays.popover.length - 1];
      if (top && invokeHandler(() => top.handler(e))) {
        e.preventDefault();
        e.stopPropagation();
        return true;
      }
    }

    // ③ peek：子图预览浮层栈顶先收，未处理继续往下
    if (overlays.peek.length > 0) {
      const top = overlays.peek[overlays.peek.length - 1];
      if (top && invokeHandler(() => top.handler(e))) {
        e.preventDefault();
        e.stopPropagation();
        return true;
      }
    }

    const inComposer = composerCheck(e.target);
    const inEditable = isEditableTarget(e.target);

    // ④ composer：焦点在对话框里
    if (inComposer) {
      if (tryLayerCombo('composer', combo, e, 'down')) {
        e.preventDefault();
        e.stopPropagation();
        return true;
      }
      if (tryInputAllowedCombo(combo, e, 'down')) {
        e.preventDefault();
        e.stopPropagation();
        return true;
      }
      return false;
    }

    // ⑤ 其他可编辑输入框：只有 in_input=true 的快捷键生效
    if (inEditable) {
      if (tryInputAllowedCombo(combo, e, 'down')) {
        e.preventDefault();
        e.stopPropagation();
        return true;
      }
      return false;
    }

    // ⑥ nest / canvas → ⑦ global
    const normalLayers: readonly ShortcutLayer[] = ['nest', 'canvas', 'global'];
    for (const layer of normalLayers) {
      if (tryLayerCombo(layer, combo, e, 'down')) {
        e.preventDefault();
        e.stopPropagation();
        return true;
      }
    }

    return false;
  };

  return {
    on(id: ShortcutId, handler: KeyHandler, opts?: { readonly when?: () => boolean }): () => void {
      if (!defById.has(id)) {
        if (import.meta.env.DEV) {
          throw new Error(`Unknown shortcut id: ${id}`);
        }
        if (!warnedUnknownId) {
          warnedUnknownId = true;
          console.error('[keys] unknown shortcut id:', id);
        }
        return () => undefined;
      }
      const list = bindings.get(id) ?? [];
      const entry: Binding = opts?.when ? { id, handler, when: opts.when } : { id, handler };
      list.push(entry);
      bindings.set(id, list);

      return () => {
        const cur = bindings.get(id);
        if (!cur) return;
        const idx = cur.indexOf(entry);
        if (idx >= 0) cur.splice(idx, 1);
        if (heldBindings.get(id) === entry) {
          heldBindings.delete(id);
        }
      };
    },

    pushOverlay(kind: Overlay, handler: (e: KeyboardEvent) => boolean): () => void {
      const entry: OverlayEntry = { kind, handler };
      overlays[kind].push(entry);
      return () => {
        const stack = overlays[kind];
        const idx = stack.lastIndexOf(entry);
        if (idx >= 0) stack.splice(idx, 1);
      };
    },

    label(id: ShortcutId, isMac?: boolean): string {
      const def = defById.get(id);
      const first = def?.keys[0];
      if (!first) return '';
      return formatShortcutCombo(first, isMac ?? getIsMac());
    },

    list(isMac?: boolean): ShortcutListItem[] {
      const mac = isMac ?? getIsMac();
      return defs.map((d) => ({
        id: d.id as ShortcutId,
        keys: d.keys.map((k) => formatShortcutCombo(k, mac)),
        group: d.group,
        label: hasText(d.label) ? t(d.label as TextKey) : d.label,
      }));
    },

    attach(target: Window): () => void {
      const onKeyDown = (e: KeyboardEvent): void => {
        dispatchEvent(e, 'down');
      };
      const onKeyUp = (e: KeyboardEvent): void => {
        dispatchEvent(e, 'up');
      };
      const onBlur = (): void => {
        releaseHeld();
      };

      target.addEventListener('keydown', onKeyDown);
      target.addEventListener('keyup', onKeyUp);
      target.addEventListener('blur', onBlur);

      return () => {
        target.removeEventListener('keydown', onKeyDown);
        target.removeEventListener('keyup', onKeyUp);
        target.removeEventListener('blur', onBlur);
        releaseHeld();
      };
    },

    setComposerCheck(fn: (target: EventTarget | null) => boolean): void {
      composerCheck = fn;
    },

    dispatchEvent,
    releaseHeld,
  };
}

export const keys: ShortcutRegistryApi = createKeyRegistry();
