import { RuleTester } from 'eslint';
import { describe, test } from 'vitest';
import rule from './no-react-in-canvas-core.js';

const tester = new RuleTester({
  languageOptions: {
    ecmaVersion: 2022,
    sourceType: 'module',
    parserOptions: { ecmaFeatures: { jsx: true } },
  },
});

describe('hx/no-react-in-canvas-core', () => {
  test('W12-T13/no-react-in-canvas-core 规则测试', () => {
    tester.run('no-react-in-canvas-core', rule, {
      valid: [
        { code: 'export function draw(x) { return x + 1; }' },
        { code: 'const user = { name: "ok" }; export const u = user.name;' },
        { code: 'function usable() { return true; } usable();' },
      ],
      invalid: [
        {
          code: 'export function f() { return useState(0); }',
          errors: [{ messageId: 'noHook' }],
        },
        {
          code: 'export function f(React) { return React.useEffect(() => {}); }',
          errors: [{ messageId: 'noHook' }],
        },
        {
          code: 'export function View() { return <div />; }',
          errors: [{ messageId: 'noJsx' }],
        },
      ],
    });
  });
});
