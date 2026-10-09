// The semantic status and accent tokens: :root holds the generic fallback derived
// from each theme's --status-* and --accent-primary (spec C1), the built-in themes
// override it with tuned values (Appendix D), and a theme that does not define the
// tokens inherits the fallback.
import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { parseColor, hex } from './contrast';
import { rootVars, lightSchemeVars, schemeRootVars, muximuxVars, muximuxLightVars, loadBundledThemes, APP_CSS, THEMES_DIR } from '../test/themeFixtures';
import { builtinThemes } from './themeStore';

const ink = 'var(--fallback-ink)';
const status = { success: 'success', warning: 'warning', danger: 'error', info: 'info' } as const;

const EXPECTED: Record<string, string> = {
  ...Object.fromEntries(Object.entries(status).flatMap(([k, s]) => [
    [`--${k}-text`, `color-mix(in srgb, var(--status-${s}) 40%, ${ink})`],
    [`--${k}-bg`, `color-mix(in srgb, var(--status-${s}) 12%, transparent)`],
    [`--${k}-border`, `color-mix(in srgb, var(--status-${s}) 60%, ${ink})`],
  ])),
  '--danger-solid': 'var(--danger-border)',
  '--danger-solid-hover': `color-mix(in srgb, var(--danger-solid) 90%, ${ink})`,
  '--danger-on-solid': 'var(--fallback-on-solid)',
  '--accent-text': `color-mix(in srgb, var(--accent-primary) 51%, ${ink})`,
};

describe('semantic tokens', () => {
  it('are defined in :root as the generic fallback', () => {
    const vars = rootVars();
    for (const [key, value] of Object.entries(EXPECTED)) expect(vars[key], key).toBe(value);
  });

  it('resolve to the tuned values in both built-in themes', () => {
    for (const vars of [muximuxVars(), muximuxLightVars()]) {
      for (const key of Object.keys(EXPECTED)) expect(parseColor(`var(${key})`, vars), key).not.toBeNull();
    }
    expect(hex(parseColor('var(--accent-text)', muximuxVars(), 'dark')!)).toBe('#2dd4bf');
    expect(hex(parseColor('var(--accent-text)', muximuxLightVars(), 'light')!)).toBe('#04776e');
    expect(muximuxVars()['--danger-on-solid']).toBe('#111111');
    expect(muximuxLightVars()['--danger-on-solid']).toBe('#ffffff');
  });

  it('switch the fallback ink on data-color-scheme, not light-dark()', () => {
    expect(rootVars()['--fallback-ink']).toBe('white');
    expect(rootVars()['--fallback-on-solid']).toBe('#111111');
    expect(lightSchemeVars()).toEqual({ '--fallback-ink': 'black', '--fallback-on-solid': '#ffffff' });
    // light-dark() is lowered by the build into helper variables that theme files never switch.
    const css = fs.readFileSync(APP_CSS, 'utf8').replace(/\/\*[\s\S]*?\*\//g, '');
    expect(css).not.toMatch(/light-dark\(/);
  });

  it('fall back to the :root formulas for a theme that does not define them', () => {
    const user = (mode: 'dark' | 'light') => ({ ...schemeRootVars(mode), '--accent-primary': '#ff0000', '--status-error': '#0000ff' });
    // Dark: mixed towards white. 51% of #ff0000 + 49% white = #ff7a7a.
    expect(hex(parseColor('var(--accent-text)', user('dark'))!)).toBe('#ff7d7d');
    // 40% of #0000ff + 60% white = #9999ff.
    expect(hex(parseColor('var(--danger-text)', user('dark'))!)).toBe('#9999ff');
    expect(hex(parseColor('var(--danger-on-solid)', user('dark'))!)).toBe('#111111');
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
      // (jsdom may re-serialise whitespace, so compare without spaces.)
      const squash = (v: string) => v.replace(/\s+/g, '');
      expect(squash(computed('--success-text'))).toBe(squash(EXPECTED['--success-text']));
      expect(squash(computed('--accent-text'))).toBe(squash(EXPECTED['--accent-text']));
      // Resolving the cascaded values: accent-text follows the theme's own accent,
      // mixed towards the dark-scheme ink (51% of #ff00aa + 49% white = #ff7dd4).
      const vars = Object.fromEntries(Object.keys(rootVars()).map((k) => [k, computed(k)]));
      expect(hex(parseColor('var(--accent-text)', vars)!)).toBe('#ff7dd4');
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

  it('give a light user theme without the tokens the light fallback values', () => {
    // What the browser sees for a light user theme: :root, the light scheme block that
    // themeStore turns on, then the theme's own block (no semantic tokens).
    const vars = { ...schemeRootVars('light'), '--accent-primary': '#ff0000', '--status-error': '#0000ff', '--status-success': '#00ff00' };
    // Mixed towards black: 51% of #ff0000 = #820000; 40% of #0000ff = #000066.
    expect(hex(parseColor('var(--accent-text)', vars)!)).toBe('#820000');
    expect(hex(parseColor('var(--danger-text)', vars)!)).toBe('#000066');
    expect(hex(parseColor('var(--success-border)', vars)!)).toBe('#009900');
    expect(hex(parseColor('var(--danger-solid)', vars)!)).toBe('#000099');
    expect(hex(parseColor('var(--danger-on-solid)', vars)!)).toBe('#ffffff');
    // 90% of #000099 + 10% black.
    expect(hex(parseColor('var(--danger-solid-hover)', vars)!)).toBe('#00008a');
  });

  it('mark every light bundled theme as light in the metadata themeStore reads', () => {
    const light: string[] = [];
    for (const file of fs.readdirSync(THEMES_DIR).filter((f) => f.endsWith('.css')).sort()) {
      const raw = fs.readFileSync(path.join(THEMES_DIR, file), 'utf8');
      const meta = raw.match(/@theme-is-dark:\s*(true|false)/)?.[1];
      const scheme = raw.replace(/\/\*[\s\S]*?\*\//g, '').match(/color-scheme:\s*(dark|light)/)?.[1];
      expect(meta, `${file} @theme-is-dark`).toBeDefined();
      expect(scheme, `${file} color-scheme`).toBe(meta === 'true' ? 'dark' : 'light');
      if (meta === 'false') light.push(file.replace(/\.css$/, ''));
    }
    expect(light).toEqual([
      'catppuccin-light', 'cineplex-light', 'dracula-light', 'gruvbox-light', 'muximux-light',
      'nord-light', 'rose-pine-light', 'solarized-light', 'tokyo-night-light',
    ]);
    expect(builtinThemes.find((t) => t.id === 'muximux-light')?.isDark).toBe(false);
    expect(builtinThemes.find((t) => t.id === 'muximux')?.isDark).toBe(true);
  });
});
