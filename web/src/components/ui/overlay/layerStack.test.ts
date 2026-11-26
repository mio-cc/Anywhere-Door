import { describe, expect, test } from 'vitest';
import { inTree, layerStackSize, pushLayer } from './layerStack';

describe('W.08.04 layerStack', () => {
  test('inTree 覆盖自身、锚点与嵌套子浮层并在出栈后清空', () => {
    const parentEl = document.createElement('div');
    const anchorEl = document.createElement('button');
    const childEl = document.createElement('div');
    const inner = document.createElement('span');
    const outside = document.createElement('div');
    childEl.appendChild(inner);

    const popParent = pushLayer({ id: 'p1', el: () => parentEl, anchor: () => anchorEl, parent: null });
    const popChild = pushLayer({ id: 'c1', el: () => childEl, anchor: () => null, parent: 'p1' });

    expect(layerStackSize()).toBe(2);
    expect(inTree('p1', parentEl)).toBe(true);
    expect(inTree('p1', anchorEl)).toBe(true);
    expect(inTree('p1', inner)).toBe(true);
    expect(inTree('p1', outside)).toBe(false);

    popChild();
    popParent();
    expect(layerStackSize()).toBe(0);
    expect(inTree('p1', parentEl)).toBe(false);
  });
});
