import { describe, expect, test } from 'vitest';
import { expectConsole, render, user } from '@tests/kit';
import { ErrorBoundary } from './ErrorBoundary';

function Boom(p: { shouldThrow: boolean }) {
  if (p.shouldThrow) {
    throw new Error('panel crash');
  }
  return <div>正常内容</div>;
}

describe('W.08.05 ErrorBoundary', () => {
  test('W0805-T11 ErrorBoundary：捕获子错误、打印区域名、retry 与 resetKey 恢复、隔离兄弟区域', async () => {
    const state = { fail: true };
    function DynamicBoom() {
      if (state.fail) throw new Error('panel crash');
      return <div>正常内容</div>;
    }

    expectConsole('error', /ErrorBoundary.*右侧栏 · 监控/, 2);
    expectConsole('error', /The above error occurred/, 2);
    const r = render(
      <div>
        <ErrorBoundary
          name="右侧栏 · 监控"
          resetKey="s1"
          fallback={(retry) => (
            <button
              type="button"
              onClick={() => {
                state.fail = false;
                retry();
              }}
            >
              重新加载这一块
            </button>
          )}
        >
          <DynamicBoom />
        </ErrorBoundary>
        <div>兄弟区域正常</div>
      </div>,
    );

    expect(r.getByText('兄弟区域正常')).toBeTruthy();
    const retryBtn = r.getByRole('button', { name: '重新加载这一块' });
    await user.click(retryBtn);
    expect(r.getByText('正常内容')).toBeTruthy();

    // resetKey 变化自动恢复
    r.rerender(
      <ErrorBoundary name="右侧栏 · 监控" resetKey="s1" fallback={() => <div>已兜底</div>}>
        <Boom shouldThrow />
      </ErrorBoundary>,
    );
    expect(r.getByText('已兜底')).toBeTruthy();

    r.rerender(
      <ErrorBoundary name="右侧栏 · 监控" resetKey="s2" fallback={() => <div>已兜底</div>}>
        <Boom shouldThrow={false} />
      </ErrorBoundary>,
    );
    expect(r.getByText('正常内容')).toBeTruthy();
  });

  test('ErrorBoundary 捕获 throw null（独立 hasError 标志，K307）', async () => {
    const state = { fail: true };
    function ThrowNull() {
      if (state.fail) {
        // eslint-disable-next-line @typescript-eslint/only-throw-error -- K307 验证 throw null 场景
        throw null;
      }
      return <div>恢复成功</div>;
    }

    expectConsole('error', /ErrorBoundary.*空异常测试/, 1);
    expectConsole('error', /The above error occurred/, 1);
    const r = render(
      <ErrorBoundary
        name="空异常测试"
        fallback={(retry) => (
          <button
            type="button"
            onClick={() => {
              state.fail = false;
              retry();
            }}
          >
            点击重试
          </button>
        )}
      >
        <ThrowNull />
      </ErrorBoundary>,
    );

    const btn = r.getByRole('button', { name: '点击重试' });
    await user.click(btn);
    expect(r.getByText('恢复成功')).toBeTruthy();
  });
});
