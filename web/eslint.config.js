import js from '@eslint/js';
import reactHooks from 'eslint-plugin-react-hooks';
import reactRefresh from 'eslint-plugin-react-refresh';
import globals from 'globals';
import tseslint from 'typescript-eslint';
import hxPlugin from '../tools/lint/eslint/index.js';

export function createConfig({ enableTypeChecked = true } = {}) {
  return tseslint.config(
    {
      ignores: [
        'dist/**',
        'dist-perf/**',
        '.test/**',
        '.perf/**',
        'node_modules/**',
        'src/lib/protocol/generated/**',
        'src/lib/i18n/generated/**',
        'tests/lint/**',
      ],
    },
    js.configs.recommended,
    ...(enableTypeChecked ? tseslint.configs.strictTypeChecked : tseslint.configs.strict),
    ...(enableTypeChecked
      ? [
          {
            files: ['**/*.ts', '**/*.tsx'],
            languageOptions: {
              parserOptions: {
                projectService: true,
                tsconfigRootDir: import.meta.dirname,
              },
            },
          },
        ]
      : []),
    {
      files: ['**/*.js', '**/*.mjs'],
      ...tseslint.configs.disableTypeChecked,
      languageOptions: {
        globals: {
          ...globals.node,
        },
      },
      rules: {
        ...tseslint.configs.disableTypeChecked.rules,
        '@typescript-eslint/no-unsafe-assignment': 'off',
        '@typescript-eslint/no-unsafe-call': 'off',
        '@typescript-eslint/no-unsafe-member-access': 'off',
        '@typescript-eslint/no-unsafe-return': 'off',
        '@typescript-eslint/no-unsafe-argument': 'off',
      },
    },
    {
      files: ['src/**/*.ts', 'src/**/*.tsx', 'tests/kit/**/*.ts', 'tests/kit/**/*.tsx'],
      languageOptions: {
        globals: {
          ...globals.browser,
          ...globals.es2022,
        },
      },
      plugins: {
        hx: hxPlugin,
        'react-hooks': reactHooks,
        'react-refresh': reactRefresh,
      },
      rules: {
        ...reactHooks.configs.recommended.rules,
        'react-refresh/only-export-components': ['warn', { allowConstantExport: true }],
        '@typescript-eslint/no-explicit-any': 'error',
        '@typescript-eslint/consistent-type-imports': ['error', { prefer: 'type-imports' }],
        // hx/no-bare-store 由 W.03.05 启用（W.12 §6.4、P.09 K179）
      },
    },
    // W.12 §4.2 / §6.4、W.04.03 §4、W.04.04-R1、W.08-R2：产品代码导入边界与 camelCase 属性门禁
    {
      files: ['src/**/*.ts', 'src/**/*.tsx'],
      ignores: ['src/**/*.test.ts', 'src/**/*.test.tsx', 'src/**/*.test-d.ts'],
      rules: {
        'no-restricted-imports': [
          'error',
          {
            patterns: [
              {
                group: ['@tests/kit', '@tests/kit/*', '**/tests/kit', '**/tests/kit/**'],
                message: '产品代码不能导入 web/tests/kit（W.12 §4.2）',
              },
              {
                group: ['@/lib/protocol/*', '!@/lib/protocol', '**/lib/protocol/*', '!**/lib/protocol'],
                message:
                  'lib/protocol/ 以外只允许从 @/lib/protocol 导入协议类型和常量（W.04.03 §4、W.04.04-R1）',
              },
              {
                group: ['@/components/ui/*', '!@/components/ui', '**/components/ui/*', '!**/components/ui'],
                message: 'components/ui 以外只允许从 @/components/ui 导入（W.01-R7、W.08-R2）',
              },
            ],
          },
        ],
      },
    },
    // W.04.03 §10 验收 5、G.04.02 §2–§3：lib/protocol/ 以外禁止 snake_case 属性声明与点访问（开放对象字符串键除外）
    {
      files: ['src/**/*.ts', 'src/**/*.tsx'],
      ignores: [
        'src/lib/protocol/**',
        'src/lib/i18n/zh.ts',
        'src/lib/i18n/terms.ts',
        'src/**/*.test.ts',
        'src/**/*.test.tsx',
        'src/**/*.test-d.ts',
      ],
      rules: {
        '@typescript-eslint/naming-convention': [
          'error',
          {
            selector: 'property',
            modifiers: ['requiresQuotes'],
            format: null,
          },
          {
            selector: 'property',
            format: ['camelCase', 'PascalCase', 'UPPER_CASE'],
          },
        ],
        'no-restricted-syntax': [
          'error',
          {
            selector: 'MemberExpression[computed=false][property.name=/^[a-z][a-z0-9]*_[a-z0-9_]+$/]',
            message:
              'lib/protocol/ 以外禁止使用 snake_case 属性名；开放对象请使用字符串键访问（W.04.03 §6.4、§10 验收 5）',
          },
        ],
      },
    },
    // W.04.04 §5：lib/protocol/ 内部允许导入 generated/ 与同目录模块（但仍禁止 tests/kit）
    {
      files: ['src/lib/protocol/**/*.ts', 'src/lib/protocol/**/*.tsx'],
      ignores: ['src/**/*.test.ts', 'src/**/*.test.tsx', 'src/**/*.test-d.ts'],
      rules: {
        'no-restricted-imports': [
          'error',
          {
            patterns: [
              {
                group: ['@tests/kit', '@tests/kit/*', '**/tests/kit', '**/tests/kit/**'],
                message: '产品代码不能导入 web/tests/kit（W.12 §4.2）',
              },
            ],
          },
        ],
      },
    },
    // W.01 §4、W-R1：canvas/core/** 禁止 react / react-dom（含子路径）/ zustand/react / panels / components / protocol 子路径 / tests/kit
    {
      files: ['src/canvas/core/**/*.ts', 'src/canvas/core/**/*.tsx'],
      ignores: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
      plugins: {
        hx: hxPlugin,
      },
      rules: {
        'hx/no-react-in-canvas-core': 'error',
        'no-restricted-imports': [
          'error',
          {
            paths: [
              { name: 'react', message: 'canvas/core/** 不能导入 react（W-R1）' },
              { name: 'react-dom', message: 'canvas/core/** 不能导入 react-dom（W-R1）' },
              { name: 'zustand/react', message: 'canvas/core/** 不能导入 zustand/react（W-R1）' },
            ],
            patterns: [
              {
                group: ['react/*', 'react-dom/*', 'zustand/react/*'],
                message: 'canvas/core/** 不能导入 react / react-dom / zustand/react 子路径（W-R1）',
              },
              {
                group: ['**/panels', '**/panels/**', '@/panels', '@/panels/**'],
                message: 'canvas/core/** 不能导入 panels（W.01-R6）',
              },
              {
                group: ['**/components', '**/components/**', '@/components', '@/components/**'],
                message: 'canvas/core/** 不能导入 components（W.01-R6）',
              },
              {
                group: ['@/lib/protocol/*', '!@/lib/protocol', '**/lib/protocol/*', '!**/lib/protocol'],
                message: 'canvas/core/** 只能从 @/lib/protocol 导入（W.01-R6、W.04.04-R3）',
              },
              {
                group: ['@tests/kit', '@tests/kit/*', '**/tests/kit', '**/tests/kit/**'],
                message: '产品代码不能导入 web/tests/kit（W.12 §4.2）',
              },
            ],
          },
        ],
      },
    },
    // W.01 §4、W.08-R1：components/ui/** 禁止 state / ops / lib/protocol / shell / panels / pages / canvas / routes（含目录入口）
    {
      files: ['src/components/ui/**/*.ts', 'src/components/ui/**/*.tsx'],
      ignores: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
      rules: {
        'no-restricted-imports': [
          'error',
          {
            patterns: [
              {
                group: [
                  '**/state',
                  '**/state/**',
                  '@/state',
                  '@/state/**',
                  '**/ops',
                  '**/ops/**',
                  '@/ops',
                  '@/ops/**',
                  '**/lib/protocol',
                  '**/lib/protocol/**',
                  '@/lib/protocol',
                  '@/lib/protocol/**',
                  '**/shell',
                  '**/shell/**',
                  '@/shell',
                  '@/shell/**',
                  '**/panels',
                  '**/panels/**',
                  '@/panels',
                  '@/panels/**',
                  '**/pages',
                  '**/pages/**',
                  '@/pages',
                  '@/pages/**',
                  '**/canvas',
                  '**/canvas/**',
                  '@/canvas',
                  '@/canvas/**',
                  '**/routes',
                  '**/routes/**',
                  '@/routes',
                  '@/routes/**',
                ],
                message: 'components/ui/** 只能依赖基础样式、i18n、图标与本目录（W.01-R7、W.08-R1）',
              },
              {
                group: ['@tests/kit', '@tests/kit/*', '**/tests/kit', '**/tests/kit/**'],
                message: '产品代码不能导入 web/tests/kit（W.12 §4.2）',
              },
            ],
          },
        ],
      },
    },
    // W.10 §6.5、G.06-R4：src/** 禁止中文字面量（排除 zh.ts、terms.ts、generated/**、*.test.ts(x)）
    {
      files: ['src/**/*.ts', 'src/**/*.tsx'],
      ignores: [
        'src/lib/i18n/zh.ts',
        'src/lib/i18n/terms.ts',
        'src/lib/i18n/generated/**',
        'src/**/*.test.ts',
        'src/**/*.test.tsx',
        'src/**/*.test-d.ts',
      ],
      plugins: {
        hx: hxPlugin,
      },
      rules: {
        'hx/no-cjk-literal': 'error',
      },
    },
    // 测试文件放宽个别仅针对产品代码的规则
    {
      files: [
        'src/**/*.test.ts',
        'src/**/*.test.tsx',
        'src/**/*.test-d.ts',
        'tests/kit/**/*.ts',
        'tests/kit/**/*.tsx',
      ],
      rules: {
        'react-refresh/only-export-components': 'off',
        '@typescript-eslint/no-non-null-assertion': 'off',
      },
    },
  );
}

export default createConfig();
