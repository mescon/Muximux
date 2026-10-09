import { describe, it, expect } from 'vitest';
import { parseColor, over, contrast, hex, luminance, pickOnColor } from './contrast';
import { loadBundledThemes, muximuxVars, rootVars } from '../test/themeFixtures';

const near = (a: number, b: number, eps = 0.01) => Math.abs(a - b) <= eps;

describe('parseColor', () => {
  it('parses hex forms', () => {
    expect(hex(parseColor('#fff')!)).toBe('#ffffff');
    expect(parseColor('#80808080')!.a).toBeCloseTo(128 / 255, 3);
    expect(hex(parseColor('#22c55e')!)).toBe('#22c55e');
  });
  it('parses rgb, rgba, hsl and keywords', () => {
    expect(hex(parseColor('rgb(34, 197, 94)')!)).toBe('#22c55e');
    expect(parseColor('rgba(0, 0, 0, 0.6)')!.a).toBe(0.6);
    expect(parseColor('rgb(0 0 0 / 50%)')!.a).toBe(0.5);
    expect(hex(parseColor('hsl(0, 100%, 50%)')!)).toBe('#ff0000');
    expect(parseColor('transparent')!.a).toBe(0);
    expect(hex(parseColor('white')!)).toBe('#ffffff');
    expect(hex(parseColor('black')!)).toBe('#000000');
  });
  it('parses oklch and clamps out-of-gamut chroma', () => {
    const c = parseColor('oklch(70.4% 0.191 22.216)')!; // Tailwind red-400
    expect(c.r).toBeGreaterThan(c.g);
    expect([c.r, c.g, c.b].every((v) => v >= 0 && v <= 1)).toBe(true);
    expect(hex(parseColor('oklch(1 0 0)')!)).toBe('#ffffff');
  });
  it('resolves var() against the map, with fallback and a cycle guard', () => {
    const vars = { '--a': 'var(--b)', '--b': '#123456', '--x': 'var(--y)', '--y': 'var(--x)' };
    expect(hex(parseColor('var(--a)', vars)!)).toBe('#123456');
    expect(hex(parseColor('var(--missing, #abcdef)', vars)!)).toBe('#abcdef');
    expect(parseColor('var(--x)', vars)).toBeNull();
  });
  it('resolves light-dark() by mode', () => {
    expect(hex(parseColor('light-dark(#000000, #ffffff)', {}, 'light')!)).toBe('#000000');
    expect(hex(parseColor('light-dark(#000000, #ffffff)', {}, 'dark')!)).toBe('#ffffff');
  });
  it('mixes color-mix() in srgb and in oklab with transparent', () => {
    const tint = parseColor('color-mix(in oklab, oklch(63.7% 0.237 25.331) 10%, transparent)')!;
    expect(tint.a).toBeCloseTo(0.1, 5);
    expect(hex({ ...tint, a: 1 })).toBe(hex(parseColor('oklch(63.7% 0.237 25.331)')!));
    const mid = parseColor('color-mix(in srgb, #000000 50%, #ffffff)')!;
    expect(hex(mid)).toBe('#808080');
    const weighted = parseColor('color-mix(in srgb, var(--s) 40%, light-dark(black, white))', { '--s': '#22c55e' }, 'light')!;
    expect(weighted.a).toBe(1);
    expect(luminance(weighted)).toBeLessThan(luminance(parseColor('#22c55e')!));
  });
  it('returns null for unknown input', () => {
    expect(parseColor('')).toBeNull();
    expect(parseColor('not-a-colour')).toBeNull();
    expect(parseColor('color-mix(in lab, red, blue)')).toBeNull();
  });
});

describe('contrast maths', () => {
  it('matches WCAG reference ratios', () => {
    expect(contrast(parseColor('#000')!, parseColor('#fff')!)).toBeCloseTo(21, 2);
    expect(near(contrast(parseColor('#767676')!, parseColor('#ffffff')!), 4.54)).toBe(true);
  });
  it('composites translucent foregrounds over the background', () => {
    const half = parseColor('rgba(255, 255, 255, 0.5)')!;
    const onBlack = over(half, parseColor('#000')!);
    expect(hex(onBlack)).toBe('#808080');
    expect(contrast(half, parseColor('#000')!)).toBeCloseTo(contrast(onBlack, parseColor('#000')!), 6);
  });
  it('picks the darker or lighter on-colour', () => {
    expect(pickOnColor(parseColor('#2dd4bf')!)).toBe('#111111');
    expect(pickOnColor(parseColor('#1a1a1a')!)).toBe('#ffffff');
  });
});

describe('theme fixtures', () => {
  it('loads 18 bundled themes with inherited defaults', () => {
    const root = rootVars();
    // Guards against the comment bug: without stripComments only --dir comes back.
    expect(root['--accent-on-primary']).toBe('#fff');
    expect(root['--bg-base']).toMatch(/^#/);
    const themes = loadBundledThemes();
    expect(themes).toHaveLength(18);
    expect(themes.map((t) => t.id)).toContain('solarized-light');
    const nord = themes.find((t) => t.id === 'nord')!;
    expect(nord.mode).toBe('dark');
    expect(nord.vars['--accent-on-primary']).toBe('#fff'); // inherited: nord.css does not set it
  });
  it('merges the app.css muximux blocks', () => {
    expect(muximuxVars()['--bg-base']).toMatch(/^#/);
  });
});
