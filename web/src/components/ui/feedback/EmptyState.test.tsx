import { describe, expect, test, vi } from 'vitest';
import { expectConsole, render } from '@tests/kit';
import { EmptyState } from './EmptyState';

describe('W.08.05 EmptyState', () => {
  test('W0805-T9 EmptyState：标题说明按钮顺序、page 尺寸、最多 2 个动作且无 img', () => {
    expectConsole('warn', /at most 2 actions/, 1);
    const r = render(
      <EmptyState
        icon="play"
        title="还没有运行过"
        desc="点击运行启动当前图"
        size="page"
        actions={[
          { label: '运行', onClick: vi.fn(), variant: 'secondary' },
          { label: '让 AI 帮我做', onClick: vi.fn(), variant: 'model' },
          { label: '多余按钮', onClick: vi.fn() },
        ]}
      />,
    );
    const root = r.container.querySelector('.hx-empty--page');
    expect(root).not.toBeNull();
    expect(root?.querySelector('.hx-empty__title')?.textContent).toBe('还没有运行过');
    expect(root?.querySelector('.hx-empty__desc')?.textContent).toBe('点击运行启动当前图');
    expect(root?.querySelectorAll('button')).toHaveLength(2);
    expect(root?.querySelector('img')).toBeNull();
  });
});
