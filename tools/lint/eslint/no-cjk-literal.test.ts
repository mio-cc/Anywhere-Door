import { RuleTester } from 'eslint';
import { describe, test } from 'vitest';
import rule from './no-cjk-literal.js';

const tester = new RuleTester({
  languageOptions: {
    ecmaVersion: 2022,
    sourceType: 'module',
    parserOptions: { ecmaFeatures: { jsx: true } },
  },
});

describe('hx/no-cjk-literal', () => {
  test('W12-T13/no-cjk-literal 规则测试', () => {
    tester.run('no-cjk-literal', rule, {
      valid: [
        { code: '// 中文注释允许\nexport const label = "run";' },
        { code: "export const yen = '\\uFFE5';" },
        { code: 'export const node = <span>{t("common.ok")}</span>;' },
      ],
      invalid: [
        {
          code: "export const a = '运行';",
          errors: [{ messageId: 'noCjk' }],
        },
        {
          code: 'export const b = `运行 ${x}`;',
          errors: [{ messageId: 'noCjk' }],
        },
        {
          code: 'export const c = <b>运行</b>;',
          errors: [{ messageId: 'noCjk' }],
        },
      ],
    });
  });
});
