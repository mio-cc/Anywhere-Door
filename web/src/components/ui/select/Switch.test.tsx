import { describe, expect, test, vi } from 'vitest';
import { render, user } from '@tests/kit';
import { Switch } from './Switch';

describe('W.08.03 Switch', () => {
  test('W0803-T7 Switch：点击切换、role="switch"、aria-checked 与 labelPosition=none', async () => {
    const onChange = vi.fn();
    const r = render(<Switch checked={false} onChange={onChange} label="列表视图" />);
    const sw = r.getByRole('switch', { name: '列表视图' });
    expect(sw.getAttribute('aria-checked')).toBe('false');
    await user.click(sw);
    expect(onChange).toHaveBeenCalledWith(true);

    r.rerender(<Switch checked onChange={onChange} label="减少动效" labelPosition="none" />);
    const sw2 = r.getByRole('switch', { name: '减少动效' });
    expect(sw2.getAttribute('aria-checked')).toBe('true');
    expect(r.container.querySelector('.hx-switch__label')).toBeNull();
  });
});
