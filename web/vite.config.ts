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
        // Pull heavy third-party libs into stable vendor chunks so
        // they stay cached across deploys (the chunk filename hash
        // only changes when the library's own version bumps, not
        // on every application code change). Returning users skip
        // re-downloading them entirely.
        //
        // Vite 8 / rolldown requires manualChunks as a function
        // (the older object-literal form is rejected at build).
        manualChunks(id: string) {
          if (id.includes('node_modules/svelte-dnd-action')) return 'vendor-dnd';
          if (id.includes('node_modules/svelte-sonner')) return 'vendor-toast';
          if (id.includes('node_modules/marked')) return 'vendor-marked';
          if (id.includes('node_modules/zod')) return 'vendor-zod';
          return undefined;
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
