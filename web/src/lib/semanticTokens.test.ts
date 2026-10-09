// The semantic status and accent tokens must reproduce the Tailwind 4.3.3 shades
// the UI used before they existed (spec S1, Appendix E), and a theme that does
// not define them must inherit those values from :root.
import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { parseColor, hex } from './contrast';
import { rootVars, muximuxVars, muximuxLightVars, loadBundledThemes } from '../test/themeFixtures';

const tw = fs.readFileSync(path.join(process.cwd(), 'node_modules', 'tailwindcss', 'theme.css'), 'utf8');
const shade = (name: string) => {
  const m = tw.match(new RegExp(`--color-${name}:\\s*([^;]+);`));
  if (!m) throw new Error(`no Tailwind shade ${name}`);
  return m[1].trim();
};
const mix = (name: string, pct: number) => `color-mix(in oklab, ${shade(name)} ${pct}%, transparent)`;

const EXPECTED: Record<string, string> = {
  '--success-text': shade('green-400'),
  '--success-bg': mix('green-500', 10),
  '--success-border': mix('green-500', 40),
  '--warning-text': shade('amber-300'),
  '--warning-bg': mix('amber-500', 10),
  '--warning-border': mix('amber-500', 40),
  '--danger-text': shade('red-400'),
  '--danger-bg': mix('red-500', 10),
  '--danger-border': mix('red-500', 40),
  '--info-text': shade('blue-300'),
  '--info-bg': mix('blue-500', 15),
  '--info-border': mix('blue-500', 30),
  '--danger-solid': shade('red-600'),
  '--danger-solid-hover': shade('red-500'),
  '--danger-on-solid': '#fff',
  '--accent-text': 'var(--accent-primary)',
};

describe('semantic tokens (PR 1 values)', () => {
  it('are defined in :root with the Tailwind shades used today', () => {
    const vars = rootVars();
    for (const [key, value] of Object.entries(EXPECTED)) expect(vars[key], key).toBe(value);
  });

  it('resolve to concrete colours in both built-in themes', () => {
    for (const vars of [muximuxVars(), muximuxLightVars()]) {
      for (const key of Object.keys(EXPECTED)) expect(parseColor(`var(${key})`, vars), key).not.toBeNull();
      expect(hex(parseColor('var(--accent-text)', vars)!)).toBe(hex(parseColor('var(--accent-primary)', vars)!));
    }
  });

  it('cascade: a theme override beats :root and a token the theme omits falls back', () => {
    // Real cascade in the DOM: app.css :root first, then a user theme file (same specificity,
    // later in source order), as the app loads them.
    const rootCss = `:root { ${Object.entries(rootVars()).map(([k, v]) => `${k}: ${v};`).join(' ')} }`;
    const themeCss = '[data-theme="mine"] { --accent-primary: #ff00aa; --danger-text: #123456; }';
    const styles = [rootCss, themeCss].map((css) => {
      const el = document.createElement('style');
      el.textContent = css;
      document.head.appendChild(el);
      return el;
    });
    const html = document.documentElement;
    html.dataset.theme = 'mine';
    try {
      const cs = getComputedStyle(html);
      const computed = (k: string) => cs.getPropertyValue(k).trim();
      // The override wins over :root.
      expect(computed('--danger-text')).toBe('#123456');
      expect(computed('--accent-primary')).toBe('#ff00aa');
      // Tokens the theme omits come from :root.
      // (jsdom re-serialises oklch() without the space after "%", so compare without spaces.)
      const squash = (v: string) => v.replace(/\s+/g, '');
      expect(squash(computed('--success-text'))).toBe(squash(EXPECTED['--success-text']));
      expect(computed('--accent-text')).toBe('var(--accent-primary)');
      // Resolving the cascaded values: accent-text follows the theme's own accent.
      const vars = Object.fromEntries(Object.keys(rootVars()).map((k) => [k, computed(k)]));
      expect(hex(parseColor('var(--accent-text)', vars)!)).toBe('#ff00aa');
      // Without the theme attribute, :root alone applies.
      delete html.dataset.theme;
      expect(squash(getComputedStyle(html).getPropertyValue('--danger-text'))).toBe(squash(EXPECTED['--danger-text']));
    } finally {
      delete html.dataset.theme;
      styles.forEach((el) => el.remove());
    }
  });

  it('every bundled theme resolves every semantic token', () => {
    for (const theme of loadBundledThemes()) {
      for (const key of Object.keys(EXPECTED)) expect(theme.vars[key], `${theme.id} ${key}`).toBeDefined();
    }
  });
});
