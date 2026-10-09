// Theme contrast test. Strict: every pair below must pass for every bundled theme and for
// the fallback tokens. A change to a theme file or to the fallback formulas in app.css must
// keep every pair green; there is no baseline of known failures.
import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { parseColor, over, contrast, luminance, type RGBA } from './contrast';
import { loadBundledThemes, muximuxVars, muximuxLightVars, schemeRootVars, parseVarBlock, THEMES_DIR, type ThemeFixture } from '../test/themeFixtures';

const AA = 4.5, NON_TEXT = 3;
const SIX = ['--bg-base', '--bg-surface', '--bg-elevated', '--bg-overlay', '--bg-hover', '--bg-active'];
const PARENTS = ['--bg-base', '--bg-surface', '--bg-elevated'];
const STATUS = ['success', 'warning', 'danger', 'info'] as const;
const SEMANTIC_KEYS = /^--(?:(?:success|warning|danger|info)-(?:text|bg|border)|accent-text|danger-solid|danger-solid-hover|danger-on-solid|border-input)$/;

interface Failure { key: string; ratio: number; need: number }
const failures: Failure[] = [];

function resolve(t: ThemeFixture, name: string): RGBA {
  const c = parseColor(`var(${name})`, t.vars, t.mode);
  if (!c) throw new Error(`${t.id}: cannot resolve ${name} = ${t.vars[name]}`);
  return c;
}
let checked = 0;
function check(t: ThemeFixture, kind: string, fgName: string, fg: RGBA, bgName: string, bg: RGBA, need: number) {
  checked++;
  const r = contrast(fg, bg);
  if (r < need) failures.push({ key: `${t.id} | ${fgName} on ${bgName} | ${kind}`, ratio: r, need });
}

// All checks for one theme (used for the bundled themes and for the fallback simulation).
function runChecks(t: ThemeFixture, opts: { base: boolean; semantic: boolean; hierarchy: boolean }) {
  const bg = (n: string) => resolve(t, n);
  if (opts.base) {
    for (const fg of ['--text-primary', '--text-secondary']) for (const b of SIX) check(t, 'text', fg, resolve(t, fg), b, bg(b), AA);
    for (const b of [...SIX.slice(0, 4), '--bg-hover']) check(t, 'text', '--text-muted', resolve(t, '--text-muted'), b, bg(b), AA);
    check(t, 'on-primary', '--accent-on-primary', resolve(t, '--accent-on-primary'), '--accent-primary', resolve(t, '--accent-primary'), AA);
    for (const b of SIX.slice(0, 2)) check(t, 'focus-ring', '--border-focus', resolve(t, '--border-focus'), b, bg(b), NON_TEXT);
  }
  if (opts.semantic) {
    // Form controls (on base, surface, elevated or overlay) draw their boundary with --border-input (WCAG 1.4.11).
    // --border-subtle/--border-default are decorative separators and are exempt.
    for (const b of SIX.slice(0, 4)) check(t, 'input-boundary', '--border-input', over(resolve(t, '--border-input'), bg(b)), b, bg(b), NON_TEXT);
  }
  if (opts.base) {
    for (const b of PARENTS) check(t, 'health-unknown', '--text-muted', resolve(t, '--text-muted'), b, bg(b), NON_TEXT);
  }
  if (opts.semantic) {
    for (const s of STATUS) {
      const text = resolve(t, `--${s}-text`), tint = resolve(t, `--${s}-bg`), border = resolve(t, `--${s}-border`);
      for (const p of PARENTS) {
        const parent = bg(p), eff = over(tint, parent);
        check(t, 'notice-text', `--${s}-text`, text, `${p}+${s}-bg`, eff, AA);
        check(t, 'inline-text', `--${s}-text`, text, p, parent, AA);
        check(t, 'notice-border', `--${s}-border`, border, p, parent, NON_TEXT);
        check(t, 'notice-border', `--${s}-border`, border, `${p}+${s}-bg`, eff, NON_TEXT);
      }
    }
    const accentText = resolve(t, '--accent-text');
    const subtleOnSurface = over(resolve(t, '--accent-subtle'), bg('--bg-surface'));
    for (const p of PARENTS) check(t, 'accent-text', '--accent-text', accentText, p, bg(p), AA);
    check(t, 'accent-text', '--accent-text', accentText, '--bg-surface+accent-subtle', subtleOnSurface, AA);
    // .badge-accent: accent text on the accent-muted tint.
    check(t, 'accent-text', '--accent-text', accentText, '--bg-surface+accent-muted', over(resolve(t, '--accent-muted'), bg('--bg-surface')), AA);
    // Tinted composites on their real parent surfaces (axe found these under 4.5:1; see composite-fix-report.md).
    // Settings dialog and Logs sit on --bg-surface, onboarding on --bg-base.
    const stack = (parent: RGBA, ...tints: RGBA[]) => tints.reduce((acc, tint) => over(tint, acc), parent);
    const accentMuted = resolve(t, '--accent-muted'), accentSubtle = resolve(t, '--accent-subtle');
    const warnBg = resolve(t, '--warning-bg'), okBg = resolve(t, '--success-bg');
    const muted = resolve(t, '--text-muted'), okText = resolve(t, '--success-text');
    // Settings app rows sit in a group body washed with ~10% of --bg-elevated over --bg-surface.
    const rowWash = over({ ...bg('--bg-elevated'), a: 0.1 }, bg('--bg-surface'));
    // .badge-accent, Apps "Default" badge, Logs source pills, discovery network chips
    check(t, 'accent-badge', '--accent-text', accentText, '--bg-surface+row-wash+accent-muted', stack(rowWash, accentMuted), AA);
    for (const p of ['--bg-base', '--bg-surface']) {
      // Selected option cards (Settings General/Security, onboarding): muted description text on accent-subtle
      check(t, 'muted-on-tint', '--text-muted', muted, `${p}+accent-subtle`, stack(bg(p), accentSubtle), AA);
      // "No authentication" card: muted text on warning-bg (the inner .notice drops its own tint, see app.css)
      check(t, 'muted-on-tint', '--text-muted', muted, `${p}+warning-bg`, stack(bg(p), warnBg), AA);
    }
    // CURRENT pill inside a selected Security card
    check(t, 'success-pill', '--success-text', okText, '--bg-surface+accent-subtle+success-bg', stack(bg('--bg-surface'), accentSubtle, okBg), AA);
    // App.svelte Toaster: the tinted status toast sits on --bg-elevated (see "toast" rules in app.css)
    for (const s of STATUS) check(t, 'toast', `--${s}-text`, resolve(t, `--${s}-text`), `--bg-elevated+${s}-bg`, stack(bg('--bg-elevated'), resolve(t, `--${s}-bg`)), AA);
    const solid = resolve(t, '--danger-solid');
    check(t, 'danger-button', '--danger-on-solid', resolve(t, '--danger-on-solid'), '--danger-solid', solid, AA);
    for (const p of PARENTS) check(t, 'danger-button', '--danger-solid', solid, p, bg(p), NON_TEXT);
    for (const p of PARENTS) {
      check(t, 'health-dot', '--success-border', resolve(t, '--success-border'), p, bg(p), NON_TEXT);
      check(t, 'health-dot', '--danger-border', resolve(t, '--danger-border'), p, bg(p), NON_TEXT);
    }
  }
  if (opts.hierarchy && t.mode === 'dark') {
    const L = ['--bg-base', '--bg-surface', '--bg-elevated', '--bg-overlay'].map((n) => luminance(bg(n)));
    for (let i = 1; i < L.length; i++) if (!(L[i] > L[i - 1])) failures.push({ key: `${t.id} | surface hierarchy | ${['base', 'surface', 'elevated', 'overlay'][i]} not lighter than the one below`, ratio: L[i] - L[i - 1], need: 0 });
  }
}

const THEMES = loadBundledThemes();

describe('bundled theme contrast', () => {
  for (const t of THEMES) {
    it(`${t.id} resolves every token and runs its checks`, () => {
      const before = checked;
      runChecks(t, { base: true, semantic: true, hierarchy: true });
      expect(checked).toBeGreaterThan(before);
    });
  }

  it('fallback tokens pass for every bundled theme without its own semantic tokens', () => {
    for (const t of THEMES) {
      // :root as the browser sees it for this theme's mode (the fallback ink flips for light themes)
      const fallback = Object.fromEntries(Object.entries(schemeRootVars(t.mode)).filter(([k]) => /^--fallback-/.test(k) || SEMANTIC_KEYS.test(k)));
      const own = Object.fromEntries(Object.entries(t.vars).filter(([k]) => !SEMANTIC_KEYS.test(k)));
      runChecks({ ...t, id: `${t.id} (fallback)`, vars: { ...own, ...fallback } }, { base: false, semantic: true, hierarchy: false });
    }
  });

  it('app.css maps the sonner toast colours to the semantic tokens', () => {
    const css = fs.readFileSync(path.join(process.cwd(), 'src', 'app.css'), 'utf8');
    for (const [toast, token] of [['success', 'success'], ['warning', 'warning'], ['info', 'info'], ['error', 'danger']]) {
      for (const part of ['bg', 'border', 'text']) {
        expect(css, `toast ${toast}-${part}`).toMatch(new RegExp(`--${toast}-${part}:\\s*var\\(--mx-${token}-${part}\\)`));
      }
    }
  });

  it('app.css muximux blocks mirror public/themes/muximux.css and muximux-light.css', () => {
    const file = (name: string) => parseVarBlock(fs.readFileSync(path.join(THEMES_DIR, name), 'utf8'));
    const dark = file('muximux.css'), light = file('muximux-light.css');
    const appDark = muximuxVars(), appLight = muximuxLightVars();
    for (const [k, v] of Object.entries(dark)) expect(appDark[k], `muximux ${k}`).toBe(v);
    for (const [k, v] of Object.entries(light)) expect(appLight[k], `muximux-light ${k}`).toBe(v);
  });

  it('a minimal user theme passes with the fallback tokens', () => {
    const css = fs.readFileSync(path.join(process.cwd(), 'src', 'test', 'fixtures', 'user-theme-minimal.css'), 'utf8');
    const own = parseVarBlock(css.slice(css.indexOf('{')));
    const t: ThemeFixture = { id: 'user-minimal', file: 'fixture', mode: 'light', vars: { ...schemeRootVars('light'), ...own } };
    runChecks(t, { base: false, semantic: true, hierarchy: false });
    // --accent-on-primary is inherited (#111111) and the fixture's accent is light, so it must pass too
    check(t, 'on-primary', '--accent-on-primary', resolve(t, '--accent-on-primary'), '--accent-primary', resolve(t, '--accent-primary'), AA);
  });

  it('has no failing pair', () => {
    const report = failures.map((f) => `${f.key} = ${f.ratio.toFixed(2)}`).sort().join('\n');
    expect(failures, `contrast failures:\n${report}`).toEqual([]);
  });
});
