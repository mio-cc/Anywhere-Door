import { describe, expect, test } from 'vitest';
import { render } from '@tests/kit';
import { FieldError } from './FieldError';

describe('W.08.02 FieldError', () => {
  test('渲染 .hx-ferr 与 role="alert"', () => {
    const r = render(<FieldError id="e1" text="字段无效" />);
    const el = r.getByRole('alert');
    expect(el.id).toBe('e1');
    expect(el.classList.contains('hx-ferr')).toBe(true);
    expect(el.textContent).toBe('字段无效');
  });
});
