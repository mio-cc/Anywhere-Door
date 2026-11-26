import { createContext } from 'react';

export interface LayerEntry {
  readonly id: string;
  el(): HTMLElement | null;
  anchor(): HTMLElement | null;
  readonly parent: string | null;
}

const entries = new Map<string, LayerEntry>();

export const OverlayParent = createContext<string | null>(null);

export function pushLayer(e: LayerEntry): () => void {
  entries.set(e.id, e);
  let removed = false;
  return () => {
    if (removed) return;
    removed = true;
    if (entries.get(e.id) === e) {
      entries.delete(e.id);
    }
  };
}

/** 判断 node 是否在这个浮层、它的锚点或它的任意子浮层里（W.08.04 §4.1）。 */
export function inTree(id: string, node: Node): boolean {
  const entry = entries.get(id);
  if (!entry) return false;
  const root = entry.el();
  if (root?.contains(node)) return true;
  const anchor = entry.anchor();
  if (anchor?.contains(node)) return true;
  for (const child of entries.values()) {
    if (child.parent === id && inTree(child.id, node)) {
      return true;
    }
  }
  return false;
}

/** 当前打开着的浮层栈大小（测试泄漏检查用）。 */
export function layerStackSize(): number {
  return entries.size;
}
