import { fileURLToPath, URL } from 'node:url';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

const rootDir = fileURLToPath(new URL('.', import.meta.url));
const repoRoot = fileURLToPath(new URL('..', import.meta.url));

const sharedResolve = {
  alias: {
    '@': fileURLToPath(new URL('./src', import.meta.url)),
    '@tests/kit': fileURLToPath(new URL('./tests/kit/index.ts', import.meta.url)),
    eslint: fileURLToPath(new URL('./node_modules/eslint/lib/api.js', import.meta.url)),
    vitest: fileURLToPath(new URL('./node_modules/vitest/dist/index.js', import.meta.url)),
  },
};

const NODE_DIRS = [
  'src/ops/**/*.test.{ts,tsx}',
  'src/state/**/*.test.{ts,tsx}',
  'src/lib/protocol/**/*.test.{ts,tsx}',
  'src/canvas/core/geom/**/*.test.{ts,tsx}',
  'src/canvas/core/layout/**/*.test.{ts,tsx}',
  'tests/kit/lint.test.{ts,tsx}',
  'tests/kit/time.tz.test.{ts,tsx}',
];

export default defineConfig({
  plugins: [react()],
  resolve: sharedResolve,
  server: {
    fs: {
      allow: [rootDir, repoRoot],
    },
  },
  test: {
    projects: [
      {
        extends: true,
        test: {
          name: 'jsdom',
          environment: 'jsdom',
          setupFiles: ['./tests/kit/setup.ts'],
          include: ['src/**/*.test.{ts,tsx}', 'tests/kit/**/*.test.{ts,tsx}'],
          exclude: ['node_modules', 'dist', 'tests/lint/**', ...NODE_DIRS],
        },
      },
      {
        extends: true,
        test: {
          name: 'node',
          environment: 'node',
          setupFiles: ['./tests/kit/setup.ts'],
          include: [...NODE_DIRS, '../tools/lint/eslint/**/*.test.{ts,tsx}'],
          exclude: ['node_modules', 'dist', 'tests/lint/**'],
        },
      },
    ],
  },
});
