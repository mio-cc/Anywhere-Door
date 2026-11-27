export {
  codeToMainKey,
  detectMac,
  formatShortcutCombo,
  isComposingEvent,
  normalizeKeyEvent,
  type KeyEventLike,
} from './normalize';
export {
  createKeyRegistry,
  EXPECTED_SHORTCUT_IDS,
  keys,
  RESERVED_SHORTCUTS,
  validateShortcuts,
  type KeyHandler,
  type Overlay,
  type ShortcutDef,
  type ShortcutId,
  type ShortcutLayer,
  type ShortcutListItem,
  type ShortcutRegistryApi,
} from './registry';
