import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import path from 'node:path';

export default defineConfig({
  plugins: [svelte()],
  resolve: {
    conditions: ['browser'],
    alias: {
      $lib: path.resolve('./src/lib'),
    },
  },
  test: {
    globals: true,
    environment: 'jsdom',
    pool: 'forks',
    include: ['src/**/*.{test,spec}.{js,ts}', 'src/**/*.{test,spec}.svelte.{js,ts}'],
    setupFiles: ['./src/test/setup.ts'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json', 'html', 'lcov'],
      // Coverage is scoped to the unit-testable surface: the lib (logic,
      // stores, utilities) and the reusable components. The app shell
      // (App.svelte mount/orchestration, main.ts entrypoint, src/routes) is
      // integration code verified by manual/browser testing, not unit tests,
      // and is deliberately excluded from the percentage. The 85% threshold
      // below therefore measures lib + components, not the whole frontend.
      // If App.svelte grows unit-testable logic, move it into src/lib and it
      // is covered automatically.
      include: ['src/lib/**/*.ts', 'src/components/**/*.svelte'],
      exclude: [
        'src/test/**',
        '**/*.d.ts',
        'src/lib/paraglide/**',
        'src/App.svelte',
        'src/main.ts',
        'src/routes/**',
      ],
      thresholds: {
        statements: 85,
        branches: 70,
        functions: 80,
        lines: 85,
      },
    },
    // Properly handle Svelte component tests
    alias: {
      $lib: path.resolve('./src/lib'),
    },
  },
});
