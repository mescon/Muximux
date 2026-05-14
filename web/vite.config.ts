import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import tailwindcss from '@tailwindcss/vite';
import { paraglideVitePlugin } from '@inlang/paraglide-js';
import path from 'node:path';

export default defineConfig({
  base: './',
  plugins: [
    tailwindcss(),
    paraglideVitePlugin({
      project: './project.inlang',
      outdir: './src/lib/paraglide',
      strategy: ['localStorage', 'baseLocale'],
      emitGitIgnore: false,
    }),
    svelte(),
  ],
  resolve: {
    alias: {
      $lib: path.resolve('./src/lib'),
    },
  },
  build: {
    outDir: '../internal/server/dist',
    emptyOutDir: true,
    chunkSizeWarningLimit: 1600,
    rollupOptions: {
      output: {
        // Pull heavy third-party libs into a stable "vendor" chunk
        // so they stay cached across deploys (filename hash only
        // changes when their own versions bump, not on every
        // application code change). Returning users skip
        // re-downloading them entirely.
        manualChunks: {
          'vendor-dnd': ['svelte-dnd-action'],
          'vendor-toast': ['svelte-sonner'],
          'vendor-marked': ['marked'],
          'vendor-zod': ['zod'],
        },
      },
    },
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:3000',
        changeOrigin: true,
      },
      '/proxy': {
        target: 'http://localhost:3000',
        changeOrigin: true,
      },
      '/icons': {
        target: 'http://localhost:3000',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:3000',
        ws: true,
      },
    },
  },
});
