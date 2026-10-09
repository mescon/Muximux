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

  it('fall back to the :root values for a theme that does not define them', () => {
    const base = rootVars();
    const userTheme = { ...base, '--accent-primary': '#ff00aa', '--bg-base': '#101010' };
    expect(hex(parseColor('var(--danger-text)', userTheme)!)).toBe(hex(parseColor(EXPECTED['--danger-text'])!));
    expect(hex(parseColor('var(--accent-text)', userTheme)!)).toBe('#ff00aa');
    for (const theme of loadBundledThemes()) {
      for (const key of Object.keys(EXPECTED)) expect(theme.vars[key], `${theme.id} ${key}`).toBeDefined();
    }
  });
});
