import { describe, expect, test, vi } from 'vitest';
import { fakeEnv, render, user } from '@tests/kit';
import { UiEnvProvider } from '../overlay/UiEnv';
import { Segmented } from './Segmented';

describe('W.08.03 Segmented', () => {
  test('W0803-T6 Segmented：→ 切换并移焦点且 stopPropagation，漫游 tabIndex、--_i 与减少动效', async () => {
    const fe = fakeEnv({ reducedMotion: true });
    const onChange = vi.fn();
    const parentKey = vi.fn();
    const items = [
      { value: 'low', label: '低' },
      { value: 'med', label: '中' },
      { value: 'high', label: '高' },
    ] as const;

    try {
      const r = render(
        <UiEnvProvider value={fe.env}>
          <div onKeyDown={parentKey}>
            <Segmented value="low" items={items} onChange={onChange} ariaLabel="思考强度" />
          </div>
        </UiEnvProvider>,
      );

      const group = r.getByRole('radiogroup', { name: '思考强度' });
      expect(group.style.getPropertyValue('--_i')).toBe('0');
      expect(group.classList.contains('has-motion')).toBe(false);
      expect(group.classList.contains('is-reduced')).toBe(true);

      const radios = r.queryAllByRole('radio');
      expect(radios.map((el) => el.tabIndex)).toEqual([0, -1, -1]);

      radios[0]!.focus();
      await user.key('ArrowRight', radios[0]);
      expect(onChange).toHaveBeenCalledWith('med');
      expect(document.activeElement).toBe(radios[1]);
      expect(parentKey).not.toHaveBeenCalled();

      r.rerender(
        <UiEnvProvider value={fe.env}>
          <div onKeyDown={parentKey}>
            <Segmented value="high" items={items} onChange={onChange} ariaLabel="思考强度" />
          </div>
        </UiEnvProvider>,
      );
      expect(r.getByRole('radiogroup', { name: '思考强度' }).style.getPropertyValue('--_i')).toBe('2');
    } finally {
      fe.cleanup();
    }
  });
});
