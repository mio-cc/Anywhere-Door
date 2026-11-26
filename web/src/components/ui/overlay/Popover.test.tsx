import { describe, expect, test, vi } from 'vitest';
import { fakeEnv, render } from '@tests/kit';
import { Popover } from './Popover';
import { UiEnvProvider } from './UiEnv';

vi.mock('@floating-ui/dom', () => ({
  computePosition: vi.fn().mockResolvedValue({ x: 32, y: 48, placement: 'bottom-start' }),
  autoUpdate: vi.fn((_r: unknown, _f: unknown, u: () => void) => {
    u();
    return vi.fn();
  }),
  offset: vi.fn((n: number) => ({ name: 'offset', n })),
  flip: vi.fn((o: unknown) => ({ name: 'flip', o })),
  shift: vi.fn((o: unknown) => ({ name: 'shift', o })),
  size: vi.fn((o: unknown) => ({ name: 'size', o })),
}));

describe('W.08.04 Popover', () => {
  test('Popover 渲染 glass/padding 类与数字宽度', () => {
    const fe = fakeEnv({ reducedMotion: true });
    const anchor = document.createElement('button');
    document.body.appendChild(anchor);
    anchor.getBoundingClientRect = () => ({
      x: 0,
      y: 0,
      width: 80,
      height: 28,
      top: 0,
      left: 0,
      right: 80,
      bottom: 28,
      toJSON: () => ({}),
    });

    try {
      render(
        <UiEnvProvider value={fe.env}>
          <Popover anchor={anchor} open onClose={vi.fn()} width={240} padding="sm" glass ariaLabel="panel">
            <div>body</div>
          </Popover>
        </UiEnvProvider>,
      );
      const pop = fe.root.querySelector('.hx-pop') as HTMLElement;
      expect(pop.classList.contains('hx-pop--glass')).toBe(true);
      expect(pop.classList.contains('hx-pop--pad-sm')).toBe(true);
      expect(pop.style.width).toBe('240px');
    } finally {
      anchor.remove();
      fe.cleanup();
    }
  });
});
