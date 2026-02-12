import js from '@eslint/js';
import ts from 'typescript-eslint';
import svelte from 'eslint-plugin-svelte';
import globals from 'globals';

export default ts.config(
  js.configs.recommended,
  ...ts.configs.recommended,
  ...svelte.configs['flat/recommended'],
  {
    languageOptions: {
      globals: {
        ...globals.browser,
        ...globals.node,
      },
    },
    rules: {
      // Allow unused vars/args prefixed with _
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_', varsIgnorePattern: '^_' }],
      // Svelte 5 runes ($state, $derived) look like unused expressions
      '@typescript-eslint/no-unused-expressions': 'off',
      // Explicit any: warn rather than error (gradual migration)
      '@typescript-eslint/no-explicit-any': 'warn',
      // Each keys: warn for now, enforce incrementally
      'svelte/require-each-key': 'warn',
    },
  },
  {
    // Svelte-specific overrides (applied AFTER general rules)
    files: ['**/*.svelte', '**/*.svelte.ts', '**/*.svelte.js'],
    languageOptions: {
      parserOptions: {
        parser: ts.parser,
      },
    },
    rules: {
      // Svelte component exports and template-referenced variables appear unused to ESLint
      // because it can't see Svelte template usage. Downgrade to warn for .svelte files.
      '@typescript-eslint/no-unused-vars': ['warn', { argsIgnorePattern: '^_', varsIgnorePattern: '^_|^\\$\\$' }],
    },
  },
  {
    ignores: [
      'dist/',
      'node_modules/',
      '.svelte-kit/',
      'coverage/',
    ],
  },
);
