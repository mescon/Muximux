import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import path from 'path';

export default defineConfig({
  plugins: [svelte({ hot: !process.env.VITEST })],
  resolve: {
    alias: {
      $lib: path.resolve('./src/lib'),
    },
  },
  test: {
    globals: true,
    environment: 'jsdom',
    pool: 'forks',
    include: ['src/**/*.{test,spec}.{js,ts}'],
    setupFiles: ['./src/test/setup.ts'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json', 'html'],
      include: ['src/lib/**/*.ts', 'src/components/**/*.svelte'],
      exclude: ['src/test/**', '**/*.d.ts'],
    },
    // Properly handle Svelte component tests
    alias: {
      $lib: path.resolve('./src/lib'),
    },
  },
});
