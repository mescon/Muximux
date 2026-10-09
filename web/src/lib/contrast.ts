// WCAG 2.1 contrast helpers shared by the theme tests and the theme editor.
// parseColor() understands the forms used in app.css and the theme files:
// hex, rgb()/rgba(), hsl()/hsla(), oklch(), transparent, white, black,
// var() with fallback, light-dark() and color-mix(). The oklch conversion
// reduces chroma until the colour fits sRGB, which is what browsers do.

export interface RGBA { r: number; g: number; b: number; a: number }
export type Vars = Record<string, string>;
export type Mode = 'light' | 'dark';

const clamp01 = (x: number) => Math.min(1, Math.max(0, x));

function oklchToSrgb(L: number, C: number, H: number) {
  const h = (H * Math.PI) / 180;
  const a = C * Math.cos(h), b = C * Math.sin(h);
  const l_ = L + 0.3963377774 * a + 0.2158037573 * b;
  const m_ = L - 0.1055613458 * a - 0.0638541728 * b;
  const s_ = L - 0.0894841775 * a - 1.291485548 * b;
  const l = l_ ** 3, m = m_ ** 3, s = s_ ** 3;
  const lr = 4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s;
  const lg = -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s;
  const lb = -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s;
  const enc = (x: number) => (x <= 0.0031308 ? 12.92 * x : 1.055 * Math.sign(x) * Math.abs(x) ** (1 / 2.4) - 0.055);
  return { r: enc(lr), g: enc(lg), b: enc(lb) };
}

const linear = (x: number) => (x <= 0.04045 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4);

function srgbToOklab(c: { r: number; g: number; b: number }) {
  const r = linear(c.r), g = linear(c.g), b = linear(c.b);
  const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b);
  const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b);
  const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b);
  return {
    L: 0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s,
    a: 1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s,
    b: 0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s,
  };
}

// oklab -> sRGB through the oklch path (same matrices), so out-of-gamut results are
// chroma-reduced like any other oklch colour.
function oklabToSrgb(L: number, a: number, b: number) {
  const C = Math.hypot(a, b);
  const H = (Math.atan2(b, a) * 180) / Math.PI;
  return oklchToSrgbClamped(L, C, H);
}

const inGamut = (c: { r: number; g: number; b: number }) =>
  [c.r, c.g, c.b].every((v) => v >= -0.0005 && v <= 1.0005);

function oklchToSrgbClamped(L: number, C: number, H: number) {
  let c = C;
  let rgb = oklchToSrgb(L, c, H);
  while (!inGamut(rgb) && c > 0) {
    c -= 0.002;
    rgb = oklchToSrgb(L, Math.max(c, 0), H);
  }
  return { r: clamp01(rgb.r), g: clamp01(rgb.g), b: clamp01(rgb.b) };
}

function hslToRgb(h: number, s: number, l: number) {
  s /= 100; l /= 100;
  const k = (n: number) => (n + h / 30) % 12;
  const a = s * Math.min(l, 1 - l);
  const f = (n: number) => l - a * Math.max(-1, Math.min(k(n) - 3, Math.min(9 - k(n), 1)));
  return { r: f(0), g: f(8), b: f(4) };
}

// Splits a function argument list on top-level commas.
function splitArgs(s: string): string[] {
  const out: string[] = [];
  let depth = 0, cur = '';
  for (const ch of s) {
    if (ch === '(') depth++;
    if (ch === ')') depth--;
    if (ch === ',' && depth === 0) { out.push(cur.trim()); cur = ''; } else cur += ch;
  }
  if (cur.trim()) out.push(cur.trim());
  return out;
}

function fnArgs(str: string, name: string): string | null {
  return str.startsWith(name + '(') && str.endsWith(')') ? str.slice(name.length + 1, -1) : null;
}

const pct = (s: string | undefined, fallback = 1) =>
  s === undefined ? fallback : s.endsWith('%') ? parseFloat(s) / 100 : parseFloat(s);

function parseVar(inner: string, vars: Vars, mode: Mode, depth: number): RGBA | null {
  const [name, ...rest] = splitArgs(inner);
  const value = vars[name];
  if (value === undefined && rest.length === 0) return null;
  return parseInner(value ?? rest.join(','), vars, mode, depth + 1);
}

function parseLightDark(inner: string, vars: Vars, mode: Mode, depth: number): RGBA | null {
  const [light, dark] = splitArgs(inner);
  if (!light || !dark) return null;
  return parseInner(mode === 'dark' ? dark : light, vars, mode, depth + 1);
}

// One color-mix() operand: "<color> [<percentage>%]". A missing percentage is NaN.
function mixPart(s: string) {
  const m = s.match(/^(.*?)\s+([\d.]+)%$/);
  return m ? { c: m[1], p: parseFloat(m[2]) / 100 } : { c: s, p: NaN };
}

// Fills in omitted percentages: both omitted is 50/50, one omitted is the complement.
function mixWeights(p1: number, p2: number): [number, number] {
  if (Number.isNaN(p1) && Number.isNaN(p2)) return [0.5, 0.5];
  if (Number.isNaN(p1)) return [1 - p2, p2];
  if (Number.isNaN(p2)) return [p1, 1 - p1];
  return [p1, p2];
}

function mixColors(space: string, ca: RGBA, cb: RGBA, p1: number, p2: number): RGBA | null {
  const sum = p1 + p2;
  if (sum <= 0) return null;
  const w1 = p1 / sum, w2 = p2 / sum;
  const alpha = ca.a * w1 + cb.a * w2;
  const outA = alpha * Math.min(sum, 1);
  if (alpha === 0) return { r: 0, g: 0, b: 0, a: outA };
  if (space.endsWith('oklab')) {
    const la = srgbToOklab(ca), lb = srgbToOklab(cb);
    const mix = (k: 'L' | 'a' | 'b') => (la[k] * ca.a * w1 + lb[k] * cb.a * w2) / alpha;
    return { ...oklabToSrgb(mix('L'), mix('a'), mix('b')), a: outA };
  }
  const ch = (k: 'r' | 'g' | 'b') => (ca[k] * ca.a * w1 + cb[k] * cb.a * w2) / alpha;
  return { r: ch('r'), g: ch('g'), b: ch('b'), a: outA };
}

// Mixing follows CSS Color 4: premultiplied alpha, interpolated in the named space
// (srgb or oklab); percentages that sum below 100% scale the alpha.
function parseColorMix(inner: string, vars: Vars, mode: Mode, depth: number): RGBA | null {
  const [space, a, b] = splitArgs(inner);
  if (!/^in\s+(srgb|oklab)$/.test(space) || !a || !b) return null;
  const pa = mixPart(a), pb = mixPart(b);
  const [p1, p2] = mixWeights(pa.p, pb.p);
  const ca = parseInner(pa.c, vars, mode, depth + 1), cb = parseInner(pb.c, vars, mode, depth + 1);
  if (!ca || !cb) return null;
  return mixColors(space, ca, cb, p1, p2);
}

function parseHex(str: string): RGBA | null {
  const m = str.match(/^#([0-9a-f]{3,8})$/i);
  if (!m) return null;
  let h = m[1];
  if (h.length <= 4) h = [...h].map((c) => c + c).join('');
  if (h.length !== 6 && h.length !== 8) return null;
  const n = (i: number) => parseInt(h.slice(i, i + 2), 16) / 255;
  return { r: n(0), g: n(2), b: n(4), a: h.length === 8 ? n(6) : 1 };
}

function parseRgb(str: string): RGBA | null {
  const m = str.match(/^rgba?\(\s*([\d.]+)[\s,]+([\d.]+)[\s,]+([\d.]+)(?:[\s,/]+([\d.]+%?))?\s*\)$/);
  return m ? { r: +m[1] / 255, g: +m[2] / 255, b: +m[3] / 255, a: pct(m[4]) } : null;
}

function parseOklch(str: string): RGBA | null {
  const m = str.match(/^oklch\(\s*([\d.]+)(%?)\s+([\d.]+)\s+([\d.]+)(?:\s*\/\s*([\d.]+%?))?\s*\)$/);
  if (!m) return null;
  const L = m[2] ? parseFloat(m[1]) / 100 : parseFloat(m[1]);
  return { ...oklchToSrgbClamped(L, parseFloat(m[3]), parseFloat(m[4])), a: pct(m[5]) };
}

function parseHsl(str: string): RGBA | null {
  const m = str.match(/^hsla?\(\s*([\d.]+)[\s,]+([\d.]+)%[\s,]+([\d.]+)%(?:[\s,/]+([\d.]+%?))?\s*\)$/);
  return m ? { ...hslToRgb(+m[1], +m[2], +m[3]), a: pct(m[4]) } : null;
}

const KEYWORDS: Record<string, RGBA> = {
  transparent: { r: 0, g: 0, b: 0, a: 0 },
  white: { r: 1, g: 1, b: 1, a: 1 },
  black: { r: 0, g: 0, b: 0, a: 1 },
};

function parseInner(input: string, vars: Vars, mode: Mode, depth: number): RGBA | null {
  if (depth > 32) return null;
  const str = input.trim().replace(/\s*!important$/, '');
  if (!str) return null;

  const fnParsers: [string, (inner: string, vars: Vars, mode: Mode, depth: number) => RGBA | null][] = [
    ['var', parseVar], ['light-dark', parseLightDark], ['color-mix', parseColorMix],
  ];
  for (const [name, parse] of fnParsers) {
    const inner = fnArgs(str, name);
    if (inner !== null) return parse(inner, vars, mode, depth);
  }

  if (Object.hasOwn(KEYWORDS, str)) return { ...KEYWORDS[str] };
  return parseHex(str) ?? parseRgb(str) ?? parseOklch(str) ?? parseHsl(str);
}

export function parseColor(input: string | null | undefined, vars: Vars = {}, mode: Mode = 'dark'): RGBA | null {
  return input ? parseInner(input, vars, mode, 0) : null;
}

export function over(fg: RGBA, bg: RGBA): RGBA {
  const a = fg.a + bg.a * (1 - fg.a);
  if (a === 0) return { r: 0, g: 0, b: 0, a: 0 };
  const ch = (k: 'r' | 'g' | 'b') => (fg[k] * fg.a + bg[k] * bg.a * (1 - fg.a)) / a;
  return { r: ch('r'), g: ch('g'), b: ch('b'), a };
}

export function luminance(c: RGBA): number {
  return 0.2126 * linear(c.r) + 0.7152 * linear(c.g) + 0.0722 * linear(c.b);
}

export function contrast(fg: RGBA, bg: RGBA): number {
  const f = fg.a < 1 ? over(fg, bg) : fg;
  const l1 = luminance(f), l2 = luminance(bg);
  return (Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05);
}

export function hex(c: RGBA): string {
  return '#' + [c.r, c.g, c.b].map((v) => Math.round(clamp01(v) * 255).toString(16).padStart(2, '0')).join('');
}

const WHITE: RGBA = { r: 1, g: 1, b: 1, a: 1 };
const BLACK: RGBA = { r: 0, g: 0, b: 0, a: 1 };

// Text colour for a fill: whichever of pure white and pure black has the higher WCAG
// contrast. Pure black (not a softer ink) keeps every fill at 4.58:1 or better; the worst
// case is a fill near luminance 0.179, where both sides are about equal. A translucent fill
// is composited over `base` (the page background it sits on) first.
export function pickOnColor(fill: RGBA, base: RGBA = WHITE): '#ffffff' | '#000000' {
  const bg = fill.a < 1 ? over(fill, { ...base, a: 1 }) : fill;
  return contrast(WHITE, bg) >= contrast(BLACK, bg) ? '#ffffff' : '#000000';
}
