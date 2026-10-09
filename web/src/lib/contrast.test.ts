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
  it('interpolates opaque color-mix() in oklab, not as an sRGB blend', () => {
    // Reference values from the CSS Color 4 algorithm (premultiplied oklab):
    // mixing black and white 50/50 in oklab gives L=0.5, i.e. #636363, not srgb's #808080.
    const grey = parseColor('color-mix(in oklab, #000000 50%, #ffffff)')!;
    expect(hex(grey)).toBe('#636363');
    expect(hex(parseColor('color-mix(in srgb, #000000 50%, #ffffff)')!)).toBe('#808080');
    // A 60/40 border mix of a red and a dark surface: oklab result is lighter than sRGB's.
    const vars = { '--s': '#ef4444', '--b': '#1f2937' };
    const ok = parseColor('color-mix(in oklab, var(--s) 60%, var(--b))', vars)!;
    const rgb = parseColor('color-mix(in srgb, var(--s) 60%, var(--b))', vars)!;
    expect(ok.a).toBe(1);
    expect(luminance(ok)).toBeGreaterThan(luminance(rgb));
    // Endpoints round-trip exactly.
    expect(hex(parseColor('color-mix(in oklab, #22c55e 100%, #000000)')!)).toBe('#22c55e');
    // Percentages below 100% in total scale the alpha.
    expect(parseColor('color-mix(in oklab, #22c55e 30%, #000000 30%)')!.a).toBeCloseTo(0.6, 5);
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
  it('matches the #777 reference and reads short hex like the long form', () => {
    // #777777 on white is the classic just-below-AA pair (4.48:1).
    expect(near(contrast(parseColor('#777')!, parseColor('#fff')!), 4.48)).toBe(true);
    expect(contrast(parseColor('#777')!, parseColor('#fff')!)).toBeLessThan(4.5);
    expect(hex(parseColor('#777')!)).toBe('#777777');
    expect(parseColor('#7778')!.a).toBeCloseTo(0x88 / 255, 5);
  });
  it('resolves light-mode values from a theme map', () => {
    const vars = { '--fg': 'light-dark(#1f2937, #f3f4f6)', '--bg': 'light-dark(#ffffff, #111827)' };
    const light = contrast(parseColor('var(--fg)', vars, 'light')!, parseColor('var(--bg)', vars, 'light')!);
    const dark = contrast(parseColor('var(--fg)', vars, 'dark')!, parseColor('var(--bg)', vars, 'dark')!);
    expect(hex(parseColor('var(--bg)', vars, 'light')!)).toBe('#ffffff');
    expect(light).toBeGreaterThan(4.5);
    expect(dark).toBeGreaterThan(4.5);
    expect(light).not.toBeCloseTo(dark, 2);
  });
});

describe('pickOnColor', () => {
  const best = (fill: string) => {
    const bg = parseColor(fill)!;
    return contrast(parseColor(pickOnColor(bg))!, bg);
  };
  it('picks pure black or pure white', () => {
    expect(pickOnColor(parseColor('#2dd4bf')!)).toBe('#000000');
    expect(pickOnColor(parseColor('#1a1a1a')!)).toBe('#ffffff');
  });
  it('reaches 4.5:1 on the worst-case mid-tone accents', () => {
    // Fills near luminance 0.18, where white and black are about equal; a #111111 ink
    // would only reach about 4.35:1 here.
    for (const fill of ['#d4651f', '#3b9e5a']) {
      expect(best(fill)).toBeGreaterThanOrEqual(4.5);
    }
  });
  it('never drops below 4.58:1 across a grey ramp', () => {
    for (let v = 0; v <= 255; v++) {
      const h = v.toString(16).padStart(2, '0');
      expect(best(`#${h}${h}${h}`)).toBeGreaterThanOrEqual(4.58);
    }
  });
  it('composites a translucent fill over the base before picking', () => {
    const tint = parseColor('rgba(255, 224, 102, 0.3)')!;
    expect(pickOnColor(tint, parseColor('#000000')!)).toBe('#ffffff');
    expect(pickOnColor(tint, parseColor('#ffffff')!)).toBe('#000000');
    // Without a base the fill is judged over white.
    expect(pickOnColor(tint)).toBe('#000000');
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
