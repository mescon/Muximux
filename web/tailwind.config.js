/** @type {import('tailwindcss').Config} */
export default {
  content: ['./src/**/*.{html,js,svelte,ts}', './index.html'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // Customizable brand colors
        brand: {
          50: 'var(--color-brand-50, #f0fdf4)',
          100: 'var(--color-brand-100, #dcfce7)',
          200: 'var(--color-brand-200, #bbf7d0)',
          300: 'var(--color-brand-300, #86efac)',
          400: 'var(--color-brand-400, #4ade80)',
          500: 'var(--color-brand-500, #22c55e)',
          600: 'var(--color-brand-600, #16a34a)',
          700: 'var(--color-brand-700, #15803d)',
          800: 'var(--color-brand-800, #166534)',
          900: 'var(--color-brand-900, #14532d)',
          950: 'var(--color-brand-950, #052e16)',
        },
      },
    },
  },
  plugins: [],
};
