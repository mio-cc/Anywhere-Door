import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { ESLint, type Linter } from 'eslint';
import stylelint from 'stylelint';
import { build as viteBuild } from 'vite';
import { describe, expect, test } from 'vitest';
import { createConfig } from '../../eslint.config.js';
import stylelintConfig from '../../stylelint.config.js';
import viteConfig from '../../vite.config.js';
import { collectTestIds } from './ids.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const webDir = resolve(here, '..', '..');

async function lintFixture(relPath: string, code: string): Promise<Linter.LintMessage[]> {
  const eslint = new ESLint({
    cwd: webDir,
    overrideConfigFile: true,
    overrideConfig: createConfig({ enableTypeChecked: false }) as Linter.Config[],
  });
  const [res] = await eslint.lintText(code, { filePath: resolve(webDir, relPath) });
  return res?.messages ?? [];
}

describe('W.12 lint 规则、test:ids 与构建干净', () => {
  test('W12-T13 文件夹样例 lint 检查', async () => {
    const cases: Array<{ dir: string; okFile: string; badFile: string; wantRules: string[] }> = [
      {
        dir: 'kit-import',
        okFile: 'src/ok.ts',
        badFile: 'src/bad.ts',
        wantRules: ['no-restricted-imports'],
      },
      {
        dir: 'canvas-core',
        okFile: 'src/canvas/core/ok.ts',
        badFile: 'src/canvas/core/bad.ts',
        wantRules: ['hx/no-react-in-canvas-core', 'no-restricted-imports'],
      },
      {
        dir: 'protocol-generated',
        okFile: 'src/lib/protocol/ok.ts',
        badFile: 'src/panels/bad.ts',
        wantRules: ['no-restricted-imports'],
      },
    ];

    for (const c of cases) {
      const okCode = readFileSync(resolve(webDir, 'tests', 'lint', c.dir, 'ok', c.okFile), 'utf8');
      const okMsgs = await lintFixture(c.okFile, okCode);
      expect(okMsgs, `${c.dir}/ok 不应报错`).toEqual([]);

      const badCode = readFileSync(resolve(webDir, 'tests', 'lint', c.dir, 'bad', c.badFile), 'utf8');
      const badMsgs = await lintFixture(c.badFile, badCode);
      const gotRules = badMsgs.map((m) => m.ruleId).sort();
      expect(gotRules, `${c.dir}/bad 必须恰好报出期望规则`).toEqual([...c.wantRules].sort());
    }

    // 补充验证：react-dom/client、components/ui 目录入口、protocol 子路径、snake_case 属性门禁、hx/no-bare-store 尚未启用
    const extraBad: Array<{ path: string; code: string; want: string[] }> = [
      {
        path: 'src/canvas/core/sub.ts',
        code: "import { createRoot } from 'react-dom/client';\nexport const fn = createRoot;\n",
        want: ['no-restricted-imports'],
      },
      {
        path: 'src/components/ui/bad.ts',
        code: "import type { X } from '@/state';\nexport type Y = X;\n",
        want: ['no-restricted-imports'],
      },
      {
        path: 'src/panels/proto-sub.ts',
        code: "import { fromWire } from '@/lib/protocol/mapping';\nexport const f = fromWire;\n",
        want: ['no-restricted-imports'],
      },
      {
        path: 'src/panels/snake.ts',
        code: 'export function read(x: { cost_micros: number }): number { return x.cost_micros; }\n',
        want: ['@typescript-eslint/naming-convention', 'no-restricted-syntax'],
      },
    ];
    for (const item of extraBad) {
      const msgs = await lintFixture(item.path, item.code);
      expect(msgs.map((m) => m.ruleId).sort(), item.path).toEqual([...item.want].sort());
    }

    // 开放对象字符串键访问允许通过（W.04.03 §6.4、§10 验收 5）
    const openOk = await lintFixture(
      'src/panels/open-ok.ts',
      "export function readParam(p: Record<string, unknown>): unknown { return p['max_rounds']; }\n",
    );
    expect(openOk).toEqual([]);

    // hx/no-bare-store 须由 W.03.05 启用（W.12 §6.4）
    const bareStoreMsgs = await lintFixture(
      'src/panels/bare.ts',
      'declare function useGraphStore(): unknown;\nexport function useDemo(): unknown { return useGraphStore(); }\n',
    );
    expect(bareStoreMsgs).toEqual([]);

    // Stylelint：验证复合属性值中的裸色也被拒绝，合法令牌通过（G.06.03、K302）
    const cssOk = await stylelint.lint({
      code: '.hx-btn { color: var(--hx-text-1); border: 1px solid var(--hx-line-1); z-index: var(--hx-z-float); }\n',
      config: stylelintConfig,
    });
    expect(cssOk.errored).toBe(false);

    const badCssCases = [
      '.hx-btn { border: 1px solid rgb(0 0 0); }\n',
      '.hx-btn { background: linear-gradient(rgb(0 0 0), rgb(255 255 255)); }\n',
      '.hx-btn { box-shadow: 0 0 2px rgba(0, 0, 0, 0.2); }\n',
      '@media (max-width: 900px) { .hx-btn { opacity: 1; } }\n',
    ];
    for (const css of badCssCases) {
      const res = await stylelint.lint({ code: css, config: stylelintConfig });
      expect(res.errored, `Stylelint 应拒绝：${css}`).toBe(true);
    }
  });

  test('W12-T17 test:ids 编号分组与去重检查', () => {
    const dir = mkdtempSync(join(tmpdir(), 'hx-ids-'));
    try {
      const srcDir = join(dir, 'src');
      mkdirSync(srcDir, { recursive: true });
      const call = 'te' + 'st';
      writeFileSync(
        join(srcDir, 'a.test.ts'),
        `${call}('X99-T1 a', () => {});\n${call}.each([1, 2])('X98-T1 b', () => {});\n`,
        'utf8',
      );
      const ok = collectTestIds([srcDir], dir);
      expect(Array.from(ok.byLeaf.keys()).sort()).toEqual(['X98', 'X99']);
      expect(ok.duplicates).toEqual([]);

      writeFileSync(join(srcDir, 'b.test.ts'), `${call}.each([3])('X98-T1 dup', () => {});\n`, 'utf8');
      const dup = collectTestIds([srcDir], dir);
      expect(dup.duplicates).toHaveLength(1);
      expect(dup.duplicates[0]?.key).toBe('X98-T1');
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  });

  test('W12-T18 构建干净：web/dist 不含 tests/kit 代码', async () => {
    const result = await viteBuild({
      ...viteConfig,
      root: webDir,
      logLevel: 'silent',
      build: {
        ...viteConfig.build,
        write: false,
      },
    });
    const outputs = Array.isArray(result) ? result : [result];
    const moduleIds: string[] = [];
    let hasIndexHtml = false;
    for (const out of outputs) {
      if (!('output' in out)) continue;
      for (const item of out.output) {
        if (item.fileName === 'index.html') hasIndexHtml = true;
        if (item.type === 'chunk') {
          moduleIds.push(...Object.keys(item.modules));
        }
      }
    }
    expect(hasIndexHtml).toBe(true);
    expect(moduleIds.length).toBeGreaterThan(0);
    for (const id of moduleIds) {
      const norm = id.replace(/\\/g, '/');
      expect(norm, `产包模块不应包含 tests/kit：${norm}`).not.toContain('/tests/kit/');
      expect(norm, `产包模块不应包含 tests/visual/gallery：${norm}`).not.toContain('/tests/visual/gallery/');
    }
  });
});
