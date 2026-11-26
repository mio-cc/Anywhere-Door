import { beforeEach, describe, expect, test, vi } from 'vitest';

const mockComputePosition = vi.fn();
const mockAutoUpdate = vi.fn();
const mockOffset = vi.fn((n: number) => ({ name: 'offset', n }));
const mockFlip = vi.fn((o: unknown) => ({ name: 'flip', o }));
const mockShift = vi.fn((o: unknown) => ({ name: 'shift', o }));
const mockSize = vi.fn((o: unknown) => ({ name: 'size', o }));

vi.mock('@floating-ui/dom', () => ({
  computePosition: (...args: unknown[]) => mockComputePosition(...args) as Promise<unknown>,
  autoUpdate: (...args: unknown[]) => mockAutoUpdate(...args) as () => void,
  offset: (n: number) => mockOffset(n),
  flip: (o: unknown) => mockFlip(o),
  shift: (o: unknown) => mockShift(o),
  size: (o: unknown) => mockSize(o),
}));

import { place, VIEWPORT_PADDING } from './position';

describe('W.08.04 position', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockComputePosition.mockResolvedValue({ x: 120, y: 84, placement: 'bottom-start' });
    mockAutoUpdate.mockImplementation((_ref: unknown, _fl: unknown, update: () => void) => {
      update();
      return vi.fn();
    });
  });

  test('W0804-T9 Popover 定位：computePosition、虚拟元素、锚点移除与清理', async () => {
    const anchor = document.createElement('button');
    const floating = document.createElement('div');
    document.body.append(anchor, floating);
    anchor.getBoundingClientRect = () => ({
      x: 10,
      y: 20,
      width: 100,
      height: 28,
      top: 20,
      left: 10,
      right: 110,
      bottom: 48,
      toJSON: () => ({}),
    });
    floating.getBoundingClientRect = () => ({
      x: 0,
      y: 0,
      width: 200,
      height: 120,
      top: 0,
      left: 0,
      right: 200,
      bottom: 120,
      toJSON: () => ({}),
    });

    const cleanupSpy = vi.fn();
    let capturedUpdate: (() => void) | null = null;
    mockAutoUpdate.mockImplementation((_ref: unknown, _fl: unknown, update: () => void) => {
      capturedUpdate = update;
      update();
      return cleanupSpy;
    });

    const onAnchorGone = vi.fn();
    const stop = place(anchor, floating, {
      placement: 'bottom-start',
      offset: 4,
      padding: VIEWPORT_PADDING,
      onAnchorGone,
    });

    await Promise.resolve();
    expect(mockComputePosition).toHaveBeenCalledTimes(1);
    const [, , opts] = mockComputePosition.mock.calls[0] as [
      unknown,
      unknown,
      { strategy: string; placement: string; middleware: unknown[] },
    ];
    expect(opts.strategy).toBe('fixed');
    expect(opts.placement).toBe('bottom-start');
    expect(mockOffset).toHaveBeenCalledWith(4);
    expect(mockFlip).toHaveBeenCalledWith({ padding: VIEWPORT_PADDING });
    expect(mockShift).toHaveBeenCalledWith({ padding: VIEWPORT_PADDING });
    expect(floating.style.left).toBe('120px');
    expect(floating.style.top).toBe('84px');

    // 锚点被移除后再次 update 触发 onAnchorGone
    anchor.remove();
    (capturedUpdate as (() => void) | null)?.();
    expect(onAnchorGone).toHaveBeenCalledTimes(1);

    stop();
    expect(cleanupSpy).toHaveBeenCalledTimes(1);

    // DOMRect 锚点转换为虚拟元素
    const rect: DOMRect = {
      x: 50,
      y: 60,
      width: 0,
      height: 0,
      top: 60,
      left: 50,
      right: 50,
      bottom: 60,
      toJSON: () => ({}),
    };
    const stop2 = place(rect, floating, { placement: 'right-start', offset: 0, padding: VIEWPORT_PADDING });
    const [virt] = mockComputePosition.mock.calls[1] as [{ getBoundingClientRect: () => DOMRect }];
    expect(virt.getBoundingClientRect()).toBe(rect);
    stop2();
    floating.remove();
  });
});
