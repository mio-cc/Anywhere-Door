import type { UiEnv } from '@/components/ui';

export interface FakeOverlayRecord {
  readonly id: number;
  readonly kind: 'modal' | 'popover';
  readonly handler: (e: KeyboardEvent) => boolean;
  popped: boolean;
}

export interface FakeEnvController {
  readonly env: UiEnv;
  readonly history: readonly FakeOverlayRecord[];
  readonly active: readonly FakeOverlayRecord[];
  reducedMotion: boolean;
  readonly root: HTMLElement;
  dispatchKey(e: KeyboardEvent | (KeyboardEventInit & { key: string })): boolean;
  cleanup(): void;
}

/** 测试用界面环境假实现（W.08 §4.3、W.08.04 实施卡）。 */
export function fakeEnv(opts?: { reducedMotion?: boolean; portalRoot?: HTMLElement }): FakeEnvController {
  const history: FakeOverlayRecord[] = [];
  const active: FakeOverlayRecord[] = [];
  let seq = 0;
  let ownRoot = false;
  let root = opts?.portalRoot;
  if (!root) {
    root = document.createElement('div');
    root.setAttribute('data-fake-layers', 'true');
    document.body.appendChild(root);
    ownRoot = true;
  }

  const ctrl: FakeEnvController = {
    reducedMotion: Boolean(opts?.reducedMotion),
    history,
    active,
    root,
    env: {
      pushOverlay(kind, handler) {
        seq += 1;
        const rec: FakeOverlayRecord = { id: seq, kind, handler, popped: false };
        history.push(rec);
        active.push(rec);
        return () => {
          if (rec.popped) return;
          rec.popped = true;
          const idx = active.indexOf(rec);
          if (idx >= 0) active.splice(idx, 1);
        };
      },
      reducedMotion() {
        return ctrl.reducedMotion;
      },
      portalRoot() {
        if (!root.isConnected) {
          document.body.appendChild(root);
        }
        return root;
      },
    },
    dispatchKey(input) {
      const ev =
        input instanceof KeyboardEvent
          ? input
          : new KeyboardEvent('keydown', { bubbles: true, cancelable: true, ...input });
      // 模拟 W.02.04 §6.1：只把键交给栈顶（modal 优先于 popover，同层后压入者优先）
      const topModal = [...active].reverse().find((r) => r.kind === 'modal');
      const top = topModal ?? active[active.length - 1];
      if (!top) return false;
      return top.handler(ev);
    },
    cleanup() {
      active.length = 0;
      if (ownRoot && root.parentNode) {
        root.parentNode.removeChild(root);
      }
    },
  };

  return ctrl;
}
