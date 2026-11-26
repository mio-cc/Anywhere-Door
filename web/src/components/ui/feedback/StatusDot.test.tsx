import { describe, expect, test } from 'vitest';
import { render } from '@tests/kit';
import { StatusDot } from './StatusDot';

describe('W.08.05 StatusDot', () => {
  test('W0805-T6 StatusDot：role="img"、aria-label、running 呼吸控制与空 label 抛错', () => {
    const r = render(<StatusDot state="running" label="运行中" />);
    const dot = r.getByRole('img', { name: '运行中' });
    expect(dot.classList.contains('is-breathing')).toBe(true);

    r.rerender(<StatusDot state="running" label="运行中" breathing={false} />);
    expect(r.getByRole('img', { name: '运行中' }).classList.contains('is-breathing')).toBe(false);

    expect(() => render(<StatusDot state="idle" label="" />)).toThrow();
  });
});
