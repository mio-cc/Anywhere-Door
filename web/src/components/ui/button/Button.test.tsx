import { ESLint } from 'eslint';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, expectTypeOf, test, vi } from 'vitest';
import { act, fakeEnv, render, user } from '@tests/kit';
import type { GlideGroupProps } from '../glide/GlideGroup';
import type { IconProps } from '../icon/Icon';
import { UiEnvProvider } from '../overlay/UiEnv';
import { Button, type ButtonProps, type ButtonVariant } from './Button';
import type { IconButtonProps } from './IconButton';
import type { KbdProps } from './Kbd';

const here = dirname(fileURLToPath(import.meta.url));
const webDir = resolve(here, '..', '..', '..', '..');

describe('W.08.01 Button', () => {
  test('W0801-T1 props 对照', () => {
    expectTypeOf<ButtonProps['variant']>().toEqualTypeOf<ButtonVariant | undefined>();
    expectTypeOf<ButtonProps['loading']>().toEqualTypeOf<boolean | undefined>();
    expectTypeOf<ButtonProps['disabledReason']>().toEqualTypeOf<string | undefined>();
    expectTypeOf<IconButtonProps['label']>().toEqualTypeOf<string>();
    expectTypeOf<IconButtonProps['shortcut']>().toEqualTypeOf<string | undefined>();
    expectTypeOf<KbdProps['text']>().toEqualTypeOf<string>();
    expectTypeOf<IconProps['size']>().toEqualTypeOf<14 | 16 | 32 | undefined>();
    expectTypeOf<GlideGroupProps['variant']>().toEqualTypeOf<'row' | 'icon'>();
    expect(true).toBe(true);
  });

  test('W0801-T2 变体类名：五种变体无 is-primary，size=sm 加 hx-ctl--sm', () => {
    const variants: ButtonVariant[] = ['primary', 'secondary', 'ghost', 'danger', 'model'];
    for (const v of variants) {
      const r = render(
        <Button variant={v} size="sm">
          btn
        </Button>,
      );
      const el = r.getByRole('button', { name: 'btn' });
      expect(el.classList.contains(`hx-btn--${v}`)).toBe(true);
      expect(el.classList.contains(`is-${v}`)).toBe(false);
      expect(el.classList.contains('hx-ctl--sm')).toBe(true);
      r.unmount();
    }
  });

  test('W0801-T3 加载：aria-busy、锁定宽度 --_w、点击无效且含 Spinner', async () => {
    const onClick = vi.fn();
    const r = render(<Button onClick={onClick}>提交</Button>);
    const btn = r.getByRole('button', { name: '提交' });
    Object.defineProperty(btn, 'offsetWidth', { configurable: true, value: 88 });

    r.rerender(
      <Button loading onClick={onClick}>
        提交
      </Button>,
    );
    expect(btn.getAttribute('aria-busy')).toBe('true');
    expect(btn.style.getPropertyValue('--_w')).toBe('88px');
    expect(btn.querySelector('.hx-spin')).not.toBeNull();

    await user.click(btn);
    expect(onClick).not.toHaveBeenCalled();
  });

  test('W0801-T4 禁用原因：有原因可聚焦并弹出提示框，无原因原生 disabled', async () => {
    const fe = fakeEnv({ reducedMotion: true });
    const onClick = vi.fn();
    try {
      const r = render(
        <UiEnvProvider value={fe.env}>
          <Button disabled disabledReason="离线时不可用" onClick={onClick}>
            保存
          </Button>
        </UiEnvProvider>,
      );
      const btn = r.getByRole('button', { name: '保存' }) as HTMLButtonElement;
      expect(btn.disabled).toBe(false);
      expect(btn.getAttribute('aria-disabled')).toBe('true');
      await user.click(btn);
      await user.key('Enter', btn);
      expect(onClick).not.toHaveBeenCalled();

      Object.defineProperty(btn, 'matches', { configurable: true, value: () => true });
      act(() => {
        btn.focus();
      });
      expect(fe.root.textContent).toContain('离线时不可用');

      r.rerender(
        <UiEnvProvider value={fe.env}>
          <Button disabled onClick={onClick}>
            保存
          </Button>
        </UiEnvProvider>,
      );
      expect((r.getByRole('button', { name: '保存' }) as HTMLButtonElement).disabled).toBe(true);
    } finally {
      fe.cleanup();
    }
  });

  test('W0801-T13 ESLint：components/ui 与目录外导入边界检查', async () => {
    const { createConfig } = (await import(resolve(webDir, 'eslint.config.js'))) as {
      createConfig: (o?: { enableTypeChecked?: boolean }) => unknown[];
    };
    const eslint = new ESLint({
      cwd: webDir,
      overrideConfigFile: true,
      overrideConfig: createConfig({ enableTypeChecked: false }) as never,
    });
    const [r1] = await eslint.lintText(
      "import type { UiState } from '@/state/types';\nexport type X = UiState;\n",
      {
        filePath: resolve(webDir, 'src/components/ui/button/Button.tsx'),
      },
    );
    expect(r1?.messages.map((m) => m.ruleId)).toContain('no-restricted-imports');

    const [r2] = await eslint.lintText(
      "import { Button } from '@/components/ui/button/Button';\nexport const B = Button;\n",
      {
        filePath: resolve(webDir, 'src/panels/x.tsx'),
      },
    );
    expect(r2?.messages.map((m) => m.ruleId)).toContain('no-restricted-imports');
  }, 15000);
});
