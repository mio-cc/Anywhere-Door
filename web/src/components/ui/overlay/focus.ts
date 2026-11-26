import type { OverlayProps } from './Overlay';

const FOCUSABLE_SELECTOR = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled]):not([type="hidden"])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',');

function isVisible(el: HTMLElement): boolean {
  if (el.hidden) return false;
  if (el.getAttribute('aria-hidden') === 'true') return false;
  if (el.style.display === 'none' || el.style.visibility === 'hidden') return false;
  let cur: HTMLElement | null = el.parentElement;
  while (cur) {
    if (cur.hidden || cur.getAttribute('aria-hidden') === 'true') return false;
    cur = cur.parentElement;
  }
  return true;
}

export function focusables(root: HTMLElement): HTMLElement[] {
  return Array.from(root.querySelectorAll(FOCUSABLE_SELECTOR)).filter(
    (node): node is HTMLElement => node instanceof HTMLElement && node.tabIndex >= 0 && isVisible(node),
  );
}

export function focusInitial(root: HTMLElement, how: 'first' | 'container'): void {
  if (how === 'container') {
    root.focus({ preventScroll: true });
    return;
  }
  const auto = root.querySelector<HTMLElement>('[data-autofocus]');
  if (auto && isVisible(auto) && !auto.hasAttribute('disabled')) {
    auto.focus({ preventScroll: true });
    return;
  }
  const list = focusables(root);
  const first = list[0];
  if (first) {
    first.focus({ preventScroll: true });
  } else {
    root.focus({ preventScroll: true });
  }
}

export function cycleTab(root: HTMLElement, e: KeyboardEvent): boolean {
  const list = focusables(root);
  e.preventDefault();
  if (list.length === 0) {
    root.focus({ preventScroll: true });
    return true;
  }
  const active = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  const idx = active ? list.indexOf(active) : -1;
  if (e.shiftKey) {
    const prev = idx <= 0 ? list[list.length - 1] : list[idx - 1];
    prev?.focus({ preventScroll: true });
  } else {
    const next = idx < 0 || idx >= list.length - 1 ? list[0] : list[idx + 1];
    next?.focus({ preventScroll: true });
  }
  return true;
}

export function resolveRestore(r: OverlayProps['restoreFocus'], opener: Element | null): HTMLElement | null {
  if (r === false) return null;
  let target: Element | null = null;
  if (typeof r === 'function') {
    target = r();
  } else if (r instanceof HTMLElement) {
    target = r;
  }
  if (target instanceof HTMLElement && target.isConnected) {
    return target;
  }
  if (opener instanceof HTMLElement && opener.isConnected) {
    return opener;
  }
  return null;
}
