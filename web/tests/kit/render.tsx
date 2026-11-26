import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';

export { act };

export interface Rendered {
  container: HTMLElement;
  rerender(ui: React.ReactElement): void;
  unmount(): void;
  getByRole(role: string, o?: { name?: string | RegExp }): HTMLElement;
  queryAllByRole(role: string, o?: { name?: string | RegExp }): HTMLElement[];
  getByText(text: string | RegExp): HTMLElement;
}

const mounted = new Set<{ root: Root; container: HTMLElement; ownContainer: boolean }>();

function isAriaHidden(el: Element): boolean {
  let cur: Element | null = el;
  while (cur) {
    if (cur.getAttribute('aria-hidden') === 'true') return true;
    if (cur instanceof HTMLElement && cur.hidden) return true;
    cur = cur.parentElement;
  }
  return false;
}

function implicitRole(el: Element): string | null {
  const explicit = el.getAttribute('role');
  if (explicit) return explicit;
  const tag = el.tagName.toLowerCase();
  switch (tag) {
    case 'button':
      return 'button';
    case 'a':
      return el.hasAttribute('href') ? 'link' : null;
    case 'textarea':
      return 'textbox';
    case 'select':
      return el.hasAttribute('multiple') ? 'listbox' : 'combobox';
    case 'option':
      return 'option';
    case 'dialog':
      return 'dialog';
    case 'nav':
      return 'navigation';
    case 'main':
      return 'main';
    case 'img':
      return el.getAttribute('alt') === '' ? 'presentation' : 'img';
    case 'input': {
      const type = (el.getAttribute('type') ?? 'text').toLowerCase();
      if (type === 'hidden' || type === 'password') return null;
      if (type === 'checkbox') return 'checkbox';
      if (type === 'radio') return 'radio';
      if (type === 'range') return 'slider';
      if (type === 'number') return 'spinbutton';
      if (type === 'search') return 'searchbox';
      if (type === 'button' || type === 'submit' || type === 'reset') return 'button';
      if (type === 'text' || type === 'email' || type === 'tel' || type === 'url') return 'textbox';
      return null;
    }
    default:
      return null;
  }
}

function labelTextForInput(el: Element): string {
  const doc = el.ownerDocument;
  const parts: string[] = [];
  const id = el.getAttribute('id');
  if (id) {
    for (const lbl of Array.from(doc.querySelectorAll('label'))) {
      if (lbl.getAttribute('for') === id) {
        const txt = lbl.textContent.replace(/\s+/g, ' ').trim();
        if (txt !== '') parts.push(txt);
      }
    }
  }
  const wrapping = el.closest('label');
  if (wrapping) {
    const clone = wrapping.cloneNode(true) as HTMLElement;
    for (const ctrl of Array.from(clone.querySelectorAll('input,select,textarea,button'))) {
      ctrl.remove();
    }
    const txt = clone.textContent.replace(/\s+/g, ' ').trim();
    if (txt !== '' && !parts.includes(txt)) parts.push(txt);
  }
  return parts.join(' ');
}

// 按 WAI-ARIA Accessible Name 计算优先级：aria-labelledby > aria-label > 关联 label / alt > 内容文本 > title
function accessibleName(el: Element): string {
  const labelledBy = el.getAttribute('aria-labelledby');
  if (labelledBy) {
    const parts = labelledBy
      .split(/\s+/)
      .map((id) => el.ownerDocument.getElementById(id)?.textContent.replace(/\s+/g, ' ').trim() ?? '')
      .filter(Boolean);
    if (parts.length > 0) return parts.join(' ');
  }
  const ariaLabel = el.getAttribute('aria-label');
  if (ariaLabel !== null && ariaLabel.trim() !== '') return ariaLabel.trim();
  const tag = el.tagName.toLowerCase();
  if (tag === 'input' || tag === 'textarea' || tag === 'select') {
    const lbl = labelTextForInput(el);
    if (lbl !== '') return lbl;
  }
  if (tag === 'img') {
    const alt = el.getAttribute('alt');
    if (alt) return alt.trim();
  }
  const text = el.textContent.replace(/\s+/g, ' ').trim();
  if (text !== '') return text;
  return (el.getAttribute('title') ?? '').trim();
}

function matchText(actual: string, expected: string | RegExp): boolean {
  return typeof expected === 'string' ? actual === expected : expected.test(actual);
}

function queryAllByRoleIn(root: HTMLElement, role: string, o?: { name?: string | RegExp }): HTMLElement[] {
  const all = [root, ...Array.from(root.querySelectorAll('*'))];
  const out: HTMLElement[] = [];
  for (const el of all) {
    if (!(el instanceof HTMLElement)) continue;
    if (isAriaHidden(el)) continue;
    if (implicitRole(el) !== role) continue;
    if (o?.name !== undefined && !matchText(accessibleName(el), o.name)) continue;
    out.push(el);
  }
  return out;
}

function getByRoleIn(root: HTMLElement, role: string, o?: { name?: string | RegExp }): HTMLElement {
  const found = queryAllByRoleIn(root, role, o);
  const nameDesc =
    o?.name === undefined ? '' : ` (name=${typeof o.name === 'string' ? `"${o.name}"` : o.name.toString()})`;
  if (found.length === 0) {
    throw new Error(`找不到 role="${role}"${nameDesc} 的元素`);
  }
  if (found.length > 1) {
    throw new Error(`找到 ${String(found.length)} 个 role="${role}"${nameDesc} 的元素，预期唯一`);
  }
  return found[0]!;
}

function getByTextIn(root: HTMLElement, text: string | RegExp): HTMLElement {
  const all = Array.from(root.querySelectorAll('*')).filter(
    (n): n is HTMLElement => n instanceof HTMLElement,
  );
  const matches = all.filter((el) => {
    const normalized = el.textContent.replace(/\s+/g, ' ').trim();
    if (!matchText(normalized, text)) return false;
    return !Array.from(el.children).some((c) => matchText(c.textContent.replace(/\s+/g, ' ').trim(), text));
  });
  const desc = typeof text === 'string' ? `"${text}"` : text.toString();
  if (matches.length === 0) {
    throw new Error(`找不到文本为 ${desc} 的元素`);
  }
  if (matches.length > 1) {
    throw new Error(`找到 ${String(matches.length)} 个文本为 ${desc} 的元素，预期唯一`);
  }
  return matches[0]!;
}

export function render(ui: React.ReactElement, o?: { container?: HTMLElement }): Rendered {
  const ownContainer = o?.container === undefined;
  const container = o?.container ?? document.createElement('div');
  if (ownContainer && !container.isConnected) {
    document.body.appendChild(container);
  }
  const root = createRoot(container);
  const rec = { root, container, ownContainer };
  mounted.add(rec);

  act(() => {
    root.render(ui);
  });

  return {
    container,
    rerender(nextUi: React.ReactElement) {
      act(() => {
        root.render(nextUi);
      });
    },
    unmount() {
      if (!mounted.has(rec)) return;
      mounted.delete(rec);
      act(() => {
        root.unmount();
      });
      if (ownContainer && container.parentNode) {
        container.parentNode.removeChild(container);
      }
    },
    getByRole(role, opts) {
      return getByRoleIn(container, role, opts);
    },
    queryAllByRole(role, opts) {
      return queryAllByRoleIn(container, role, opts);
    },
    getByText(text) {
      return getByTextIn(container, text);
    },
  };
}

export function cleanupRendered(): void {
  for (const rec of Array.from(mounted)) {
    mounted.delete(rec);
    act(() => {
      rec.root.unmount();
    });
    if (rec.ownContainer && rec.container.parentNode) {
      rec.container.parentNode.removeChild(rec.container);
    }
  }
}
