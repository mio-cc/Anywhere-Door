import { describe, expect, test } from 'vitest';
import { ASSIGN, ERRORS } from './generated/messages';

describe('W.10 生成的消息文案', () => {
  test('W10-T15 生成物 ASSIGN 与 ERRORS 检查', () => {
    expect(Object.keys(ASSIGN).sort()).toEqual(['C1', 'C10', 'C2', 'C4', 'C6', 'C8']);
    expect(typeof ERRORS['internal']).toBe('string');
    expect(ERRORS['internal']?.length).toBeGreaterThan(0);
  });
});
