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

function parseInner(input: string, vars: Vars, mode: Mode, depth: number): RGBA | null {
  if (depth > 32) return null;
  const str = input.trim().replace(/\s*!important$/, '');
  if (!str) return null;

  let inner = fnArgs(str, 'var');
  if (inner !== null) {
    const [name, ...rest] = splitArgs(inner);
    const value = vars[name];
    if (value === undefined && rest.length === 0) return null;
    return parseInner(value ?? rest.join(','), vars, mode, depth + 1);
  }
  inner = fnArgs(str, 'light-dark');
  if (inner !== null) {
    const [light, dark] = splitArgs(inner);
    return parseInner(mode === 'dark' ? dark : light, vars, mode, depth + 1);
  }
  inner = fnArgs(str, 'color-mix');
  if (inner !== null) {
    const [space, a, b] = splitArgs(inner);
    // oklab mixing is only exact here when one side is transparent (alpha-only); app.css uses
    // oklab for Tailwind-style tints and srgb everywhere else.
    if (!/^in\s+(srgb|oklab)$/.test(space) || !a || !b) return null;
    const part = (s: string) => {
      const m = s.match(/^(.*?)\s+([\d.]+)%$/);
      return m ? { c: m[1], p: parseFloat(m[2]) / 100 } : { c: s, p: NaN };
    };
    const pa = part(a), pb = part(b);
    let p1 = pa.p, p2 = pb.p;
    if (Number.isNaN(p1) && Number.isNaN(p2)) { p1 = 0.5; p2 = 0.5; }
    else if (Number.isNaN(p1)) p1 = 1 - p2;
    else if (Number.isNaN(p2)) p2 = 1 - p1;
    const ca = parseInner(pa.c, vars, mode, depth + 1), cb = parseInner(pb.c, vars, mode, depth + 1);
    if (!ca || !cb) return null;
    const sum = p1 + p2;
    if (sum <= 0) return null;
    const w1 = p1 / sum, w2 = p2 / sum;
    const alpha = ca.a * w1 + cb.a * w2;
    const ch = (k: 'r' | 'g' | 'b') => (alpha === 0 ? 0 : (ca[k] * ca.a * w1 + cb[k] * cb.a * w2) / alpha);
    return { r: ch('r'), g: ch('g'), b: ch('b'), a: alpha * (sum < 1 ? sum : 1) };
  }

  if (str === 'transparent') return { r: 0, g: 0, b: 0, a: 0 };
  if (str === 'white') return { r: 1, g: 1, b: 1, a: 1 };
  if (str === 'black') return { r: 0, g: 0, b: 0, a: 1 };

  let m = str.match(/^#([0-9a-f]{3,8})$/i);
  if (m) {
    let h = m[1];
    if (h.length <= 4) h = [...h].map((c) => c + c).join('');
    if (h.length !== 6 && h.length !== 8) return null;
    const n = (i: number) => parseInt(h.slice(i, i + 2), 16) / 255;
    return { r: n(0), g: n(2), b: n(4), a: h.length === 8 ? n(6) : 1 };
  }
  m = str.match(/^rgba?\(\s*([\d.]+)[\s,]+([\d.]+)[\s,]+([\d.]+)(?:[\s,/]+([\d.]+%?))?\s*\)$/);
  if (m) return { r: +m[1] / 255, g: +m[2] / 255, b: +m[3] / 255, a: pct(m[4]) };
  m = str.match(/^oklch\(\s*([\d.]+)(%?)\s+([\d.]+)\s+([\d.]+)(?:\s*\/\s*([\d.]+%?))?\s*\)$/);
  if (m) {
    const L = m[2] ? parseFloat(m[1]) / 100 : parseFloat(m[1]);
    return { ...oklchToSrgbClamped(L, parseFloat(m[3]), parseFloat(m[4])), a: pct(m[5]) };
  }
  m = str.match(/^hsla?\(\s*([\d.]+)[\s,]+([\d.]+)%[\s,]+([\d.]+)%(?:[\s,/]+([\d.]+%?))?\s*\)$/);
  if (m) return { ...hslToRgb(+m[1], +m[2], +m[3]), a: pct(m[4]) };
  return null;
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

const lin = (x: number) => (x <= 0.04045 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4);

export function luminance(c: RGBA): number {
  return 0.2126 * lin(c.r) + 0.7152 * lin(c.g) + 0.0722 * lin(c.b);
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
const INK: RGBA = { r: 17 / 255, g: 17 / 255, b: 17 / 255, a: 1 };

export function pickOnColor(bg: RGBA): '#ffffff' | '#111111' {
  return contrast(WHITE, bg) >= contrast(INK, bg) ? '#ffffff' : '#111111';
}
