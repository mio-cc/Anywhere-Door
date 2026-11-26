import { describe, expect, test } from 'vitest';
import { errorText } from './errors';
import { ERRORS } from './generated/messages';

describe('W.10 错误文案', () => {
  test('W10-T11 errorText 处理 NetError、服务端 message、错误码与未知码', () => {
    expect(errorText({ kind: 'network' })).toBe('与 Anywhere Door 的连接已断开。连接恢复后再试');
    expect(errorText({ kind: 'timeout' })).toBe('服务没有及时响应。稍后重试');
    expect(errorText({ kind: 'aborted' })).toBe('已取消');

    expect(errorText({ code: 'not_found', message: '服务端已渲染好的提示' })).toBe('服务端已渲染好的提示');
    expect(errorText({ code: 'not_found', details: { resource: '会话' } })).toBe(
      '找不到这个会话，可能已经被删除。',
    );
    expect(errorText({ code: 'rate_limited', details: { retryAfterMs: 1500 } })).toBe(
      '请求太频繁，请 1500 毫秒后再试。',
    );
    expect(errorText({ code: 'some_future_error_code' })).toBe(ERRORS['internal']);
  });
});
