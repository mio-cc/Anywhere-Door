import { act } from 'react';

export interface PointerStep {
  readonly kind: 'down' | 'move' | 'up';
  readonly x: number;
  readonly y: number;
  readonly button?: number;
  readonly pointerId?: number;
}

function isMac(): boolean {
  if (typeof navigator === 'undefined') return false;
  const nav = navigator as Navigator & { userAgentData?: { platform?: string } };
  const p = nav.userAgentData?.platform ?? navigator.platform;
  return /mac/i.test(p);
}

function parseCombo(combo: string): KeyboardEventInit {
  const clean = combo.replace(/^\{|\}$/g, '');
  const parts = clean.split('+');
  const main = parts.pop() ?? '';
  let ctrlKey = false;
  let metaKey = false;
  let altKey = false;
  let shiftKey = false;
  for (const mod of parts) {
    const m = mod.toLowerCase();
    if (m === 'mod') {
      if (isMac()) metaKey = true;
      else ctrlKey = true;
    } else if (m === 'ctrl' || m === 'control') {
      ctrlKey = true;
    } else if (m === 'meta' || m === 'cmd') {
      metaKey = true;
    } else if (m === 'alt' || m === 'option') {
      altKey = true;
    } else if (m === 'shift') {
      shiftKey = true;
    }
  }
  const code = /^[a-zA-Z]$/.test(main)
    ? `Key${main.toUpperCase()}`
    : /^[0-9]$/.test(main)
      ? `Digit${main}`
      : main;
  return {
    key: main,
    code,
    ctrlKey,
    metaKey,
    altKey,
    shiftKey,
    bubbles: true,
    cancelable: true,
  };
}

function parseTypeTokens(text: string): string[] {
  const tokens: string[] = [];
  let i = 0;
  while (i < text.length) {
    if (text[i] === '{') {
      const end = text.indexOf('}', i + 1);
      if (end > i) {
        tokens.push(text.slice(i, end + 1));
        i = end + 1;
        continue;
      }
    }
    tokens.push(text[i]!);
    i += 1;
  }
  return tokens;
}

// 通过原生原型 setter 写入 value，避免绕过 React 受控组件的 _valueTracker（P.09 K302）。
function setNativeValue(el: HTMLInputElement | HTMLTextAreaElement, nextValue: string): void {
  const proto =
    el instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
  const desc = Object.getOwnPropertyDescriptor(proto, 'value');
  if (desc && typeof desc.set === 'function') {
    // eslint-disable-next-line @typescript-eslint/unbound-method -- 显式通过 Reflect.apply 绑定 this
    Reflect.apply(desc.set, el, [nextValue]);
  } else {
    el.value = nextValue;
  }
}

function focusableElements(root: Document): HTMLElement[] {
  const sel =
    'a[href],button:not([disabled]),input:not([disabled]):not([type="hidden"]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])';
  return Array.from(root.querySelectorAll(sel)).filter((n): n is HTMLElement => n instanceof HTMLElement);
}

export const user = {
  async click(el: Element): Promise<void> {
    await act(async () => {
      el.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, cancelable: true, button: 0 }));
      el.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true, button: 0 }));
      if (el instanceof HTMLElement) {
        el.focus();
      }
      el.dispatchEvent(new PointerEvent('pointerup', { bubbles: true, cancelable: true, button: 0 }));
      el.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, cancelable: true, button: 0 }));
      el.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, button: 0 }));
      await Promise.resolve();
    });
  },

  async key(combo: string, target?: Element): Promise<void> {
    const el = target ?? document.activeElement ?? document.body;
    const init = parseCombo(combo);
    await act(async () => {
      el.dispatchEvent(new KeyboardEvent('keydown', init));
      el.dispatchEvent(new KeyboardEvent('keyup', init));
      await Promise.resolve();
    });
  },

  async type(el: Element, text: string): Promise<void> {
    const tokens = parseTypeTokens(text);
    await act(async () => {
      if (el instanceof HTMLElement) {
        el.focus();
      }
      for (const tok of tokens) {
        const isSpecial = tok.startsWith('{') && tok.endsWith('}');
        const init = parseCombo(tok);
        const downEv = new KeyboardEvent('keydown', init);
        const notPrevented = el.dispatchEvent(downEv);
        if (
          !isSpecial &&
          notPrevented &&
          (el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement)
        ) {
          setNativeValue(el, el.value + tok);
          el.dispatchEvent(new Event('input', { bubbles: true }));
        }
        el.dispatchEvent(new KeyboardEvent('keyup', init));
      }
      await Promise.resolve();
    });
  },

  async tab(o?: { shift?: boolean }): Promise<void> {
    await act(async () => {
      const target = document.activeElement ?? document.body;
      const init: KeyboardEventInit = {
        key: 'Tab',
        code: 'Tab',
        shiftKey: Boolean(o?.shift),
        bubbles: true,
        cancelable: true,
      };
      const downEv = new KeyboardEvent('keydown', init);
      const notPrevented = target.dispatchEvent(downEv);
      if (notPrevented) {
        const list = focusableElements(document);
        if (list.length > 0) {
          const cur =
            document.activeElement instanceof HTMLElement ? list.indexOf(document.activeElement) : -1;
          const delta = o?.shift ? -1 : 1;
          const nextIdx =
            cur < 0 ? (o?.shift ? list.length - 1 : 0) : (cur + delta + list.length) % list.length;
          list[nextIdx]?.focus();
        }
      }
      const after = document.activeElement ?? document.body;
      after.dispatchEvent(new KeyboardEvent('keyup', init));
      await Promise.resolve();
    });
  },

  async pointer(el: Element, seq: readonly PointerStep[]): Promise<void> {
    await act(async () => {
      for (const s of seq) {
        const evType = s.kind === 'down' ? 'pointerdown' : s.kind === 'move' ? 'pointermove' : 'pointerup';
        el.dispatchEvent(
          new PointerEvent(evType, {
            bubbles: true,
            cancelable: true,
            clientX: s.x,
            clientY: s.y,
            button: s.button ?? 0,
            pointerId: s.pointerId ?? 1,
          }),
        );
      }
      await Promise.resolve();
    });
  },
};
