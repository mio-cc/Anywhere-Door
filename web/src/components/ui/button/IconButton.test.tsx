import { describe, expect, test } from 'vitest';
import { act, fakeEnv, render } from '@tests/kit';
import { UiEnvProvider } from '../overlay/UiEnv';
import { IconButton } from './IconButton';

describe('W.08.01 IconButton', () => {
  test('W0801-T5 图标按钮：aria-label、pressed、dot 与提示框中的快捷键', () => {
    const fe = fakeEnv({ reducedMotion: true });
    try {
      const r = render(
        <UiEnvProvider value={fe.env}>
          <IconButton icon="bell" label="通知" shortcut="Ctrl+K" pressed dot="danger" />
          <IconButton icon="gear" label="设置" />
        </UiEnvProvider>,
      );

      const bell = r.getByRole('button', { name: '通知' });
      expect(bell.getAttribute('aria-pressed')).toBe('true');
      expect(bell.querySelector('.hx-iconbtn__dot--danger')).not.toBeNull();

      const gear = r.getByRole('button', { name: '设置' });
      expect(gear.getAttribute('aria-pressed')).toBeNull();

      Object.defineProperty(bell, 'matches', { configurable: true, value: () => true });
      act(() => {
        bell.focus();
      });
      expect(fe.root.querySelector('kbd')?.textContent).toBe('Ctrl+K');
    } finally {
      fe.cleanup();
    }
  });

  test('W0801-T6 空 label：开发模式抛错', () => {
    expect(() => render(<IconButton icon="bell" label="" />)).toThrow();
  });
});
