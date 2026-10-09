// Static accessibility guard. Scans every component for colour classes and CSS that
// bypass the theme tokens, for focus outlines removed without replacement, for form
// controls without a programmatic label and for icon-only buttons without a name.
// BUDGET holds the counts still allowed while the structure PR migrates the code;
// the PR's last tasks set every budget to 0 and then remove the budgets.
import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { parse } from 'svelte/compiler';

const SRC = path.join(process.cwd(), 'src');

export const BUDGET: Record<string, number> = {
  palette: 533,
  whiteBlack: 0,
  styleColours: 25,
  tokenAsText: 31,
  arbitraryToken: 34,
  outline: 84,
  unlabeled: 36,
  unnamedButtons: 13,
};

const PALETTE = 'red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose|slate|gray|zinc|neutral|stone|brand';
const PROPS = 'text|bg|border|border-t|border-b|ring|outline|divide|from|via|to|fill|stroke|placeholder|accent|caret|shadow|decoration';
const PALETTE_CLASS = new RegExp(String.raw`(?<![\w-])(?:[\w-]+:)*!?(?:${PROPS})-(?:${PALETTE})-\d{2,3}(?:/\d{1,3})?(?![\w-])`, 'g');
const WHITE_BLACK = new RegExp(String.raw`(?<![\w-])(?:[\w-]+:)*!?(?:${PROPS})-(?:white|black)(?:/\d{1,3})?(?![\w-])`, 'g');
// A whole declaration (up to ";"), so the black-tint exemption below sees all of it.
const STYLE_COLOUR = /(?<![\w-])(?:color|background(?:-color)?|border(?:-top|-right|-bottom|-left)?(?:-color)?|fill|stroke|outline(?:-color)?)\s*:[^;{}]*?(?:#[0-9a-f]{3,8}\b|\brgba?\(|\b(?:white|black)\b)[^;{}]*/gi;
// Text colour in an inline style="..." attribute.
const INLINE_COLOUR = /(?<![\w-])color\s*:[^;"]*?(?:#[0-9a-f]{3,8}\b|\brgba?\(|\b(?:white|black)\b)/gi;
const BLACK_TINT = /rgba?\(\s*0\s*,\s*0\s*,\s*0\s*[,)]/;
// (?<![\w-]) keeps border-color, outline-color, accent-color and --tw-ring-color out.
const TOKEN_AS_TEXT = /(?<![\w-])color\s*:\s*var\(--(?:status-(?:success|warning|error|info)|accent-primary)\)/g;
const ARBITRARY_TOKEN = /(?<![\w-])(?:[\w-]+:)*!?(?:text|bg|border|ring|outline|divide|fill|stroke|decoration|caret|accent)-\[var\(--(?:accent-primary|status-[\w-]+)\)\]/g;
const OUTLINE = /(?<![\w-])(?:[\w-]+:)*outline-none(?![\w-])|outline\s*:\s*none/g;

// Decorative uses that carry no information and are not text.
const ALLOW: Array<{ file: RegExp; cls: RegExp }> = [
  { file: /./, cls: /^bg-black(?:\/(?:50|60))?$/ },
  { file: /settings\/AboutTab\.svelte$/, cls: /^text-(?:blue|yellow|cyan)-400$/ },
];

// Same-length blanking keeps every index, so lineOf() reports the real line.
const blank = (s: string) => s.replace(/[^\n]/g, ' ');
const blankStyle = (css: string) =>
  css.replace(/\/\*[\s\S]*?\*\//g, blank).replace(/var\([^)]*\)/g, (v) => 'var(' + blank(v.slice(4, -1)) + ')');

function walk(dir: string): string[] {
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) return e.name === 'paraglide' ? [] : walk(p);
    return p.endsWith('.svelte') && !/\.test\./.test(p) ? [p] : [];
  });
}
const files = walk(SRC);
const rel = (f: string) => path.relative(SRC, f);
const lineOf = (src: string, idx: number) => src.slice(0, idx).split('\n').length;

type Finding = string;
const findings: Record<string, Finding[]> = { palette: [], whiteBlack: [], styleColours: [], tokenAsText: [], arbitraryToken: [], outline: [], unlabeled: [], unnamedButtons: [] };

for (const f of files) {
  const src = fs.readFileSync(f, 'utf8');
  const allowed = (cls: string) => ALLOW.some((a) => a.file.test(f) && a.cls.test(cls.replace(/^(?:[\w-]+:)*!?/, '')));
  const add = (rule: string, idx: number, text: string) => findings[rule].push(`${rel(f)}:${lineOf(src, idx)} ${text.trim()}`);
  for (const m of src.matchAll(PALETTE_CLASS)) if (!allowed(m[0])) add('palette', m.index!, m[0]);
  for (const m of src.matchAll(WHITE_BLACK)) if (!allowed(m[0])) add('whiteBlack', m.index!, m[0]);
  for (const m of src.matchAll(TOKEN_AS_TEXT)) add('tokenAsText', m.index!, m[0]);
  for (const m of src.matchAll(ARBITRARY_TOKEN)) add('arbitraryToken', m.index!, m[0]);
  for (const m of src.matchAll(OUTLINE)) add('outline', m.index!, m[0]);
  for (const style of src.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/g)) {
    const body = blankStyle(style[1]);
    const offset = style.index! + style[0].indexOf(style[1]);
    for (const m of body.matchAll(STYLE_COLOUR)) {
      if (BLACK_TINT.test(m[0])) continue; // black tints: backdrops and shadows
      add('styleColours', offset + m.index!, m[0]);
    }
  }
  for (const a of src.matchAll(/\sstyle="([^"]*)"/g)) {
    const offset = a.index! + a[0].indexOf(a[1]);
    for (const m of a[1].matchAll(INLINE_COLOUR)) add('styleColours', offset + m.index!, `style: ${m[0]}`);
  }
  // AST pass: unlabeled controls and unnamed icon-only buttons (the audit's static.mjs logic)
  let ast: ReturnType<typeof parse>;
  try { ast = parse(src, { modern: true }); } catch { continue; }
  const attr = (n: any, name: string) => n.attributes?.find((a: any) => a.type === 'Attribute' && a.name === name);
  const hasSpread = (n: any) => n.attributes?.some((a: any) => a.type === 'SpreadAttribute');
  const attrText = (a: any) => (a ? src.slice(a.start, a.end).replace(/^[\w-]+=/, '').replace(/["']/g, '') : '');
  const labelFor = new Set<string>();
  const inputs: Array<{ n: any; inLabel: boolean }> = [];
  const textOf = (n: any): string => {
    let t = '';
    const w = (x: any) => {
      if (!x) return;
      if (x.type === 'Text') t += x.data.trim();
      else if (x.type === 'ExpressionTag' || x.type === 'HtmlTag' || x.type === 'Component' || x.type === 'RenderTag') t += '{x}';
      if (x.type === 'RegularElement' && x.name === 'span' && /sr-only/.test(attrText(attr(x, 'class')))) t += '{sr}';
      x.fragment?.nodes?.forEach(w);
      for (const k of ['consequent', 'alternate', 'body', 'fallback', 'then', 'catch', 'pending']) x[k]?.nodes?.forEach(w);
    };
    n.fragment.nodes.forEach(w);
    return t;
  };
  const visit = (n: any, anc: any[]) => {
    if (!n || typeof n !== 'object') return;
    if (n.type === 'RegularElement') {
      if (n.name === 'label' && attr(n, 'for')) labelFor.add(attrText(attr(n, 'for')));
      if (['input', 'select', 'textarea'].includes(n.name) && !/hidden|submit|button/.test(attrText(attr(n, 'type')))) inputs.push({ n, inLabel: anc.some((a) => a.name === 'label') });
      if (n.name === 'button' && !attr(n, 'aria-label') && !attr(n, 'aria-labelledby') && !hasSpread(n) && textOf(n) === '') findings.unnamedButtons.push(`${rel(f)}:${lineOf(src, n.start)} <button>${attr(n, 'title') ? ' (title only)' : ''}`);
    }
    const next = n.type === 'RegularElement' ? [...anc, n] : anc;
    for (const k of Object.keys(n)) {
      if (k === 'parent' || k === 'metadata') continue;
      const v = n[k];
      if (Array.isArray(v)) v.forEach((c) => visit(c, next));
      else if (v && typeof v === 'object') { if (v.type) visit(v, next); else if (v.nodes) v.nodes.forEach((c: any) => visit(c, next)); }
    }
  };
  visit(ast.fragment, []);
  for (const { n, inLabel } of inputs) {
    if (inLabel || attr(n, 'aria-label') || attr(n, 'aria-labelledby') || hasSpread(n)) continue;
    const id = attrText(attr(n, 'id'));
    if (id && labelFor.has(id)) continue;
    findings.unlabeled.push(`${rel(f)}:${lineOf(src, n.start)} <${n.name}${id ? ' id=' + id : ''}>${attr(n, 'placeholder') ? ' (placeholder only)' : ''}`);
  }
}

describe('a11y static guard', () => {
  describe('rule patterns', () => {
    const hits = (re: RegExp, s: string) => [...s.matchAll(re)].map((m) => m[0]);
    it('tokenAsText ignores border-, outline- and accent-color and --tw-ring-color', () => {
      expect(hits(TOKEN_AS_TEXT, 'border-color: var(--accent-primary); outline-color: var(--accent-primary); accent-color: var(--accent-primary); --tw-ring-color: var(--accent-primary)')).toEqual([]);
      expect(hits(TOKEN_AS_TEXT, 'a { color: var(--accent-primary); }')).toHaveLength(1);
    });
    it('styleColours exempts black tints by the whole declaration', () => {
      const decl = hits(STYLE_COLOUR, 'background: rgba(0, 0, 0, 0.6);')[0];
      expect(BLACK_TINT.test(decl)).toBe(true);
      expect(BLACK_TINT.test(hits(STYLE_COLOUR, 'background: rgba(55, 65, 81, 0.5);')[0])).toBe(false);
    });
    it('blankStyle keeps line numbers', () => {
      const css = '/* a\n b */\n.x { color: #fff; }';
      expect(blankStyle(css)).toHaveLength(css.length);
      expect(blankStyle(css).split('\n')).toHaveLength(3);
    });
    it('arbitraryToken catches token arbitrary values with variants', () => {
      expect(hits(ARBITRARY_TOKEN, 'text-[var(--accent-primary)] focus:ring-[var(--accent-primary)] border-[var(--border-default)]')).toHaveLength(2);
    });
  });

  for (const rule of Object.keys(findings)) {
    it(`${rule}: at most ${BUDGET[rule]} findings`, () => {
      const list = findings[rule];
      const msg = `${rule}: ${list.length} findings (budget ${BUDGET[rule]})\n${list.join('\n')}`;
      if (BUDGET[rule] === 0) expect(list, msg).toEqual([]);
      else expect(list.length, msg).toBeLessThanOrEqual(BUDGET[rule]);
      console.info(`a11y guard ${rule}: ${list.length}`);
    });
  }
});
