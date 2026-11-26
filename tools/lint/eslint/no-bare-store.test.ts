import { RuleTester } from 'eslint';
import { describe, test } from 'vitest';
import rule from './no-bare-store.js';

const tester = new RuleTester({
  languageOptions: {
    ecmaVersion: 2022,
    sourceType: 'module',
  },
});

describe('hx/no-bare-store', () => {
  test('W12-T13/no-bare-store 规则测试', () => {
    tester.run('no-bare-store', rule, {
      valid: [
        { code: 'const title = useGraphStore((s) => s.title);' },
        { code: 'const sel = useUiStore((s) => s.tool, Object.is);' },
        { code: 'const stores = useAppStores();' },
      ],
      invalid: [
        {
          code: 'const all = useGraphStore();',
          errors: [{ messageId: 'bareStore' }],
        },
        {
          code: 'const ui = useUiStore();',
          errors: [{ messageId: 'bareStore' }],
        },
        {
          code: 'const run = useRunStore();',
          errors: [{ messageId: 'bareStore' }],
        },
      ],
    });
  });
});
