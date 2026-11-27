import { describe, expect, test } from 'vitest';
import { bpOf } from './breakpoints';
import { computeFitInsets, shouldHideMinimap } from './layout';

describe('W.02.02 breakpoints 与 layout 纯函数', () => {
  test('W0202-T1 bpOf：精确覆盖全部 8 个边界点', () => {
    expect(bpOf(759)).toBe('xs');
    expect(bpOf(760)).toBe('sm');
    expect(bpOf(899)).toBe('sm');
    expect(bpOf(900)).toBe('md');
    expect(bpOf(1099)).toBe('md');
    expect(bpOf(1100)).toBe('lg');
    expect(bpOf(1179)).toBe('lg');
    expect(bpOf(1180)).toBe('xl');
  });

  test('W0202-T2 fitInsets：根图/子图与浮动回复收拢/展开/夹紧 80', () => {
    // 根图 + 浮动回复收拢
    expect(
      computeFitInsets({
        inSubgraph: false,
        composerHeight: 60,
        floatReplyOpen: false,
        floatReplyHeight: 120,
      }),
    ).toEqual({ top: 28, right: 28, bottom: 104, left: 64 });

    // 子图 + 浮动回复展开 50px
    expect(
      computeFitInsets({
        inSubgraph: true,
        composerHeight: 60,
        floatReplyOpen: true,
        floatReplyHeight: 50,
      }),
    ).toEqual({ top: 28, right: 28, bottom: 154, left: 304 });

    // 浮动回复高 120px → 夹紧到 80px
    expect(
      computeFitInsets({
        inSubgraph: true,
        composerHeight: 60,
        floatReplyOpen: true,
        floatReplyHeight: 120,
      }),
    ).toEqual({ top: 28, right: 28, bottom: 184, left: 304 });
  });

  test('shouldHideMinimap：小于 1100 恒隐藏，相交时隐藏，不相交时显示', () => {
    const mini = { left: 12, right: 172, top: 600, bottom: 720 };
    const farComp = { left: 300, right: 900, top: 640, bottom: 720 };
    const nearComp = { left: 150, right: 750, top: 640, bottom: 720 };

    expect(shouldHideMinimap('md', mini, farComp)).toBe(true);
    expect(shouldHideMinimap('xl', mini, farComp)).toBe(false);
    expect(shouldHideMinimap('xl', mini, nearComp)).toBe(true);
  });
});
