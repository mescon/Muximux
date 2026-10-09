// Theme contrast test. Every check the colours PR will enforce strictly runs here
// already; until that PR lands the known failures are listed in contrastBaseline.json
// and the test fails on any NEW failure or any STALE baseline entry.
// Prune fixed entries (remove-only) with: CONTRAST_WRITE_BASELINE=1 npx vitest run themeContrast
// Adding new known failures is deliberate and needs both flags:
//   CONTRAST_WRITE_BASELINE=1 CONTRAST_ALLOW_ADD=1 npx vitest run themeContrast
import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { parseColor, over, contrast, luminance, type RGBA } from './contrast';
import { loadBundledThemes, muximuxVars, muximuxLightVars, rootVars, parseVarBlock, THEMES_DIR, type ThemeFixture } from '../test/themeFixtures';

const AA = 4.5, NON_TEXT = 3;
const SIX = ['--bg-base', '--bg-surface', '--bg-elevated', '--bg-overlay', '--bg-hover', '--bg-active'];
const PARENTS = ['--bg-base', '--bg-surface', '--bg-elevated'];
const STATUS = ['success', 'warning', 'danger', 'info'] as const;
const SEMANTIC_KEYS = /^--(?:(?:success|warning|danger|info)-(?:text|bg|border)|accent-text|danger-solid|danger-solid-hover|danger-on-solid)$/;
const BASELINE = path.join(process.cwd(), 'src', 'test', 'contrastBaseline.json');

interface Failure { key: string; ratio: number; need: number }
const failures: Failure[] = [];

function resolve(t: ThemeFixture, name: string): RGBA {
  const c = parseColor(`var(${name})`, t.vars, t.mode);
  if (!c) throw new Error(`${t.id}: cannot resolve ${name} = ${t.vars[name]}`);
  return c;
}
function check(t: ThemeFixture, kind: string, fgName: string, fg: RGBA, bgName: string, bg: RGBA, need: number) {
  const r = contrast(fg, bg);
  if (r < need) failures.push({ key: `${t.id} | ${fgName} on ${bgName} | ${kind}`, ratio: r, need });
}

// All checks for one theme (used for the bundled themes and for the fallback simulation).
function runChecks(t: ThemeFixture, opts: { base: boolean; semantic: boolean; hierarchy: boolean }) {
  const bg = (n: string) => resolve(t, n);
  if (opts.base) {
    for (const fg of ['--text-primary', '--text-secondary']) for (const b of SIX) check(t, 'text', fg, resolve(t, fg), b, bg(b), AA);
    for (const b of SIX.slice(0, 4)) check(t, 'text', '--text-muted', resolve(t, '--text-muted'), b, bg(b), AA);
    check(t, 'on-primary', '--accent-on-primary', resolve(t, '--accent-on-primary'), '--accent-primary', resolve(t, '--accent-primary'), AA);
    for (const b of SIX.slice(0, 2)) check(t, 'focus-ring', '--border-focus', resolve(t, '--border-focus'), b, bg(b), NON_TEXT);
    for (const fg of ['--border-default', '--border-subtle']) for (const b of SIX.slice(0, 2)) check(t, 'neutral-border', fg, over(resolve(t, fg), bg(b)), b, bg(b), NON_TEXT);
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
  for (const t of THEMES) runChecks(t, { base: true, semantic: true, hierarchy: true });

  it('fallback tokens pass for every bundled theme without its own semantic tokens', () => {
    const fallback = Object.fromEntries(Object.entries(rootVars()).filter(([k]) => SEMANTIC_KEYS.test(k)));
    for (const t of THEMES) {
      const own = Object.fromEntries(Object.entries(t.vars).filter(([k]) => !SEMANTIC_KEYS.test(k)));
      runChecks({ ...t, id: `${t.id} (fallback)`, vars: { ...own, ...fallback } }, { base: false, semantic: true, hierarchy: false });
    }
  });

  it('app.css muximux blocks mirror public/themes/muximux.css and muximux-light.css', () => {
    const file = (name: string) => parseVarBlock(fs.readFileSync(path.join(THEMES_DIR, name), 'utf8'));
    const dark = file('muximux.css'), light = file('muximux-light.css');
    const appDark = muximuxVars(), appLight = muximuxLightVars();
    for (const [k, v] of Object.entries(dark)) expect(appDark[k], `muximux ${k}`).toBe(v);
    for (const [k, v] of Object.entries(light)) expect(appLight[k], `muximux-light ${k}`).toBe(v);
  });

  it('matches the checked-in baseline of known failures', () => {
    const byKey = new Map(failures.map((f) => [f.key, f]));
    const keys = [...byKey.keys()].sort();
    let baseline: string[] = fs.existsSync(BASELINE) ? JSON.parse(fs.readFileSync(BASELINE, 'utf8')) : [];
    const detail = (k: string) => { const f = byKey.get(k)!; return `${k} = ${f.ratio.toFixed(2)} (needs ${f.need})`; };
    let added = keys.filter((k) => !baseline.includes(k));
    if (process.env.CONTRAST_WRITE_BASELINE) {
      if (process.env.CONTRAST_ALLOW_ADD) {
        console.info(`contrast baseline: ADDING ${added.length} failing pairs:\n${added.map(detail).join('\n')}`);
        baseline = keys;
      } else {
        baseline = baseline.filter((k) => byKey.has(k)); // remove-only
      }
      fs.writeFileSync(BASELINE, JSON.stringify([...baseline].sort(), null, 2) + '\n');
      added = keys.filter((k) => !baseline.includes(k));
    }
    const stale = baseline.filter((k) => !byKey.has(k));
    expect(added.map(detail), 'new contrast failures (to accept them deliberately: CONTRAST_WRITE_BASELINE=1 CONTRAST_ALLOW_ADD=1)').toEqual([]);
    expect(stale, 'baseline entries that now pass; remove them with: CONTRAST_WRITE_BASELINE=1 npx vitest run themeContrast').toEqual([]);
    console.info(`theme contrast: ${keys.length} known failing pairs in the baseline`);
  });
});
