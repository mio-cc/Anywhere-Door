import { describe, expect, test } from 'vitest';
import { render } from '@tests/kit';
import { Spinner } from './Spinner';

describe('W.08.05 Spinner', () => {
  test('W0805-T7 Spinner：缺省 role="status" 与隐藏文案，label=null 为纯图形', () => {
    const r = render(<Spinner size={16} />);
    const st = r.getByRole('status');
    expect(st.textContent).toBe('加载中');

    r.rerender(<Spinner size={12} label={null} />);
    expect(r.queryAllByRole('status')).toHaveLength(0);
    const spin = r.container.querySelector('.hx-spin--12');
    expect(spin?.getAttribute('aria-hidden')).toBe('true');
    expect(spin?.getAttribute('role')).toBeNull();
  });
});
