// Static accessibility guard. Scans every component for colour classes and CSS that
// bypass the theme tokens, for focus outlines removed without replacement, for form
// controls without a programmatic label and for icon-only buttons without a name.
// BUDGET holds the counts still allowed while the structure PR migrates the code;
// the PR's last tasks set every budget to 0 and then remove the budgets.
import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { parse, type AST } from 'svelte/compiler';

const SRC = path.join(process.cwd(), 'src');

export const BUDGET: Record<string, number> = {
  palette: 369,
  whiteBlack: 0,
  styleColours: 25,
  tokenAsText: 31,
  arbitraryToken: 34,
  outline: 84,
  unlabeled: 36,
  unnamedButtons: 17, // corrected measurement: icon components and {@render} no longer count as a name (was 13)
};

const PALETTE = 'red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose|slate|gray|zinc|neutral|stone|brand';
const PROPS = 'text|bg|border|border-t|border-b|border-l|border-r|border-x|border-y|border-s|border-e|ring|ring-offset|outline|divide|from|via|to|fill|stroke|placeholder|accent|caret|shadow|decoration';
const PALETTE_CLASS = new RegExp(String.raw`(?<![\w-])(?:[\w-]+:)*!?(?:${PROPS})-(?:${PALETTE})-\d{2,3}(?:/\d{1,3})?(?![\w-])`, 'g');
// Arbitrary hex values, e.g. text-[#fff]; counted with the palette rule.
const HEX_CLASS = new RegExp(String.raw`(?<![\w-])(?:[\w-]+:)*!?(?:${PROPS})-\[#[0-9a-fA-F]{3,8}\](?![\w-])`, 'g');
const WHITE_BLACK = new RegExp(String.raw`(?<![\w-])(?:[\w-]+:)*!?(?:${PROPS})-(?:white|black)(?:/\d{1,3})?(?![\w-])`, 'g');
// A whole declaration (up to ";"), so the black-tint exemption below sees all of it.
const STYLE_COLOUR = /(?<![\w-])(?:color|background(?:-color)?|border(?:-top|-right|-bottom|-left)?(?:-color)?|fill|stroke|outline(?:-color)?)\s*:[^;{}]*?(?:#[0-9a-f]{3,8}\b|\brgba?\(|\b(?:white|black)\b)[^;{}]*/gi;
// Text colour in an inline style="..." attribute.
const INLINE_COLOUR = /(?<![\w-])color\s*:[^;"]*?(?:#[0-9a-f]{3,8}\b|\brgba?\(|\b(?:white|black)\b)/gi;
const BLACK_TINT = /rgba?\(\s*0\s*,\s*0\s*,\s*0\s*[,)]/;
// (?<![\w-]) keeps border-color, outline-color, accent-color and --tw-ring-color out.
const TOKEN_AS_TEXT = /(?<![\w-])color\s*:\s*var\(--(?:status-(?:success|warning|error|info)|accent-primary)\)/g;
const ARBITRARY_TOKEN = /(?<![\w-])(?:[\w-]+:)*!?(?:text|bg|border|ring|outline|divide|fill|stroke|decoration|caret|accent)-\[var\(--(?:accent-primary|status-[\w-]+)\)\]/g;
// Counts every outline-none and outline:none; Task 13 zeroes the count by replacing each
// one with a visible focus style.
const OUTLINE = /(?<![\w-])(?:[\w-]+:)*outline-none(?![\w-])|outline\s*:\s*none/g;

// Decorative uses that carry no information and are not text. `before` is matched against
// the text just before the class, so the AboutTab rule only covers the three logo svgs
// (`<svg class="w-5 h-5 text-blue-400" ...>`) and not any other use of those colours.
const ALLOW: Array<{ file: RegExp; cls: RegExp; before?: RegExp }> = [
  { file: /./, cls: /^bg-black(?:\/(?:50|60))?$/ },
  { file: /settings\/AboutTab\.svelte$/, cls: /^text-(?:blue|yellow|cyan)-400$/, before: /<svg class="w-\d h-\d $/ },
];
const isAllowed = (file: string, match: string, before: string) =>
  ALLOW.some((a) => a.file.test(file) && a.cls.test(match.replace(/^(?:[\w-]+:)*!?/, '')) && (!a.before || a.before.test(before)));

// Same-length blanking keeps every index, so lineOf() reports the real line.
const blank = (s: string) => s.replace(/[^\n]/g, ' ');
const blankStyle = (css: string) =>
  css.replace(/\/\*[\s\S]*?\*\//g, blank).replace(/var\([^)]*\)/g, (v) => 'var(' + blank(v.slice(4, -1)) + ')');

// Colour declarations inside <style> blocks as [index in src, declaration].
function styleHits(src: string): Array<[number, string]> {
  const out: Array<[number, string]> = [];
  for (const style of src.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/g)) {
    const body = blankStyle(style[1]);
    const offset = style.index! + style[0].indexOf(style[1]);
    for (const m of body.matchAll(STYLE_COLOUR)) {
      if (BLACK_TINT.test(m[0])) continue; // black tints: backdrops and shadows
      out.push([offset + m.index!, m[0]]);
    }
  }
  return out;
}

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
const parseFailures: string[] = [];

type Obj = Record<string, unknown>;
type WithAttrs = AST.RegularElement | AST.Component;
const isObj = (v: unknown): v is Obj => !!v && typeof v === 'object';

for (const f of files) {
  const src = fs.readFileSync(f, 'utf8');
  const add = (rule: string, idx: number, text: string) => findings[rule].push(`${rel(f)}:${lineOf(src, idx)} ${text.trim()}`);
  for (const m of src.matchAll(PALETTE_CLASS)) if (!isAllowed(f, m[0], src.slice(Math.max(0, m.index! - 40), m.index!))) add('palette', m.index!, m[0]);
  for (const m of src.matchAll(HEX_CLASS)) add('palette', m.index!, m[0]);
  for (const m of src.matchAll(WHITE_BLACK)) if (!isAllowed(f, m[0], src.slice(Math.max(0, m.index! - 40), m.index!))) add('whiteBlack', m.index!, m[0]);
  for (const m of src.matchAll(TOKEN_AS_TEXT)) add('tokenAsText', m.index!, m[0]);
  for (const m of src.matchAll(ARBITRARY_TOKEN)) add('arbitraryToken', m.index!, m[0]);
  for (const m of src.matchAll(OUTLINE)) add('outline', m.index!, m[0]);
  for (const [idx, text] of styleHits(src)) add('styleColours', idx, text);
  for (const a of src.matchAll(/\sstyle="([^"]*)"/g)) {
    const offset = a.index! + a[0].indexOf(a[1]);
    for (const m of a[1].matchAll(INLINE_COLOUR)) add('styleColours', offset + m.index!, `style: ${m[0]}`);
  }
  // AST pass: unlabeled controls and unnamed icon-only buttons (the audit's static.mjs logic)
  let ast: AST.Root;
  try { ast = parse(src, { modern: true }); } catch (e) { parseFailures.push(`${rel(f)}: ${(e as Error).message}`); continue; }
  const attr = (n: WithAttrs, name: string): AST.Attribute | undefined =>
    n.attributes.find((a): a is AST.Attribute => a.type === 'Attribute' && a.name === name);
  const hasSpread = (n: WithAttrs) => n.attributes.some((a) => a.type === 'SpreadAttribute');
  const attrText = (a: AST.Attribute | undefined) => (a ? src.slice(a.start, a.end).replace(/^[\w-]+=/, '').replace(/["']/g, '') : '');
  const labelFor = new Set<string>();
  const inputs: Array<{ n: AST.RegularElement; inLabel: boolean }> = [];
  // Visible name of a button: text, an expression, an sr-only span, or a component that
  // is given an aria-label or title. A bare icon component or {@render} is not a name.
  const textOf = (n: AST.RegularElement): string => {
    let t = '';
    const w = (x: unknown): void => {
      if (!isObj(x)) return;
      if (x.type === 'Text') t += String(x.data).trim();
      else if (x.type === 'ExpressionTag') t += '{x}';
      else if (x.type === 'Component') {
        const c = x as unknown as AST.Component;
        if (attr(c, 'aria-label') || attr(c, 'title') || hasSpread(c)) t += '{c}';
      } else if (x.type === 'RegularElement') {
        const el = x as unknown as AST.RegularElement;
        if (el.name === 'span' && /sr-only/.test(attrText(attr(el, 'class')))) t += '{sr}';
      }
      const frag = x.fragment as { nodes?: unknown[] } | undefined;
      frag?.nodes?.forEach(w);
      for (const k of ['consequent', 'alternate', 'body', 'fallback', 'then', 'catch', 'pending']) (x[k] as { nodes?: unknown[] } | null | undefined)?.nodes?.forEach(w);
    };
    n.fragment.nodes.forEach(w);
    return t;
  };
  const visit = (n: unknown, anc: AST.RegularElement[]): void => {
    if (!isObj(n)) return;
    let next = anc;
    if (n.type === 'RegularElement') {
      const el = n as unknown as AST.RegularElement;
      if (el.name === 'label' && attr(el, 'for')) labelFor.add(attrText(attr(el, 'for')));
      if (['input', 'select', 'textarea'].includes(el.name) && !/hidden|submit|button/.test(attrText(attr(el, 'type')))) inputs.push({ n: el, inLabel: anc.some((a) => a.name === 'label') });
      if (el.name === 'button' && !attr(el, 'aria-label') && !attr(el, 'aria-labelledby') && !hasSpread(el) && textOf(el) === '') findings.unnamedButtons.push(`${rel(f)}:${lineOf(src, el.start)} <button>${attr(el, 'title') ? ' (title only)' : ''}`);
      next = [...anc, el];
    }
    for (const k of Object.keys(n)) {
      if (k === 'parent' || k === 'metadata') continue;
      const v = n[k];
      if (Array.isArray(v)) v.forEach((c) => visit(c, next));
      else if (isObj(v)) { if (v.type) visit(v, next); else if (Array.isArray(v.nodes)) v.nodes.forEach((c) => visit(c, next)); }
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
    it('palette flags raw palette classes with variants and opacity, not tokens', () => {
      expect(hits(PALETTE_CLASS, 'text-red-500 hover:bg-blue-600/50 focus:ring-green-400 dark:text-gray-300 sm:hover:border-l-rose-500 ring-offset-slate-900')).toEqual(
        ['text-red-500', 'hover:bg-blue-600/50', 'focus:ring-green-400', 'dark:text-gray-300', 'sm:hover:border-l-rose-500', 'ring-offset-slate-900']);
      expect(hits(PALETTE_CLASS, 'text-text-primary bg-bg-surface border-border text-red text-red-5 data-red-500 my-text-red-500 text-redish-500')).toEqual([]);
      expect(hits(HEX_CLASS, 'text-[#fff] hover:bg-[#1a2b3c]')).toHaveLength(2);
      expect(hits(HEX_CLASS, 'text-[var(--x)] w-[#abc]')).toEqual([]);
    });
    it('whiteBlack flags white and black utilities with variants, not bg-black/50 lookalikes', () => {
      expect(hits(WHITE_BLACK, 'text-white hover:bg-black/40 border-t-white focus:ring-white')).toHaveLength(4);
      expect(hits(WHITE_BLACK, 'text-whitespace bg-blackish text-white-ish white')).toEqual([]);
    });
    it('outline flags outline-none and outline: none, not outline-offset or outline-none-ish', () => {
      expect(hits(OUTLINE, 'outline-none focus:outline-none .a { outline: none; } .b { outline:none }')).toHaveLength(4);
      expect(hits(OUTLINE, 'outline-2 outline-offset-2 outline-nonexistent .a { outline: 2px solid red; }')).toEqual([]);
    });
    it('inline colour flags text colour only', () => {
      expect(hits(INLINE_COLOUR, 'color: white; color:#fff; color: rgba(1, 2, 3, 0.5)')).toHaveLength(3);
      expect(hits(INLINE_COLOUR, 'background: white; border-color: #fff; color: var(--text-primary); accent-color: red')).toEqual([]);
    });
    it('arbitraryToken catches token arbitrary values with variants', () => {
      expect(hits(ARBITRARY_TOKEN, 'text-[var(--accent-primary)] focus:ring-[var(--accent-primary)] border-[var(--border-default)]')).toHaveLength(2);
      expect(hits(ARBITRARY_TOKEN, 'bg-[var(--status-error)] dark:fill-[var(--status-success)]')).toHaveLength(2);
      expect(hits(ARBITRARY_TOKEN, 'text-[var(--text-primary)] w-[var(--accent-primary)] text-[14px]')).toEqual([]);
    });
    it('allow list covers bg-black backdrops and only the AboutTab logo svgs', () => {
      expect(isAllowed('a/B.svelte', 'bg-black/50', '')).toBe(true);
      expect(isAllowed('a/B.svelte', 'bg-black/70', '')).toBe(false);
      expect(isAllowed('a/B.svelte', 'text-black', '')).toBe(false);
      expect(isAllowed('settings/AboutTab.svelte', 'text-blue-400', '<svg class="w-5 h-5 ')).toBe(true);
      expect(isAllowed('settings/AboutTab.svelte', 'text-blue-400', '<span class="')).toBe(false);
      expect(isAllowed('settings/AboutTab.svelte', 'text-red-400', '<svg class="w-5 h-5 ')).toBe(false);
      expect(isAllowed('settings/Other.svelte', 'text-blue-400', '<svg class="w-5 h-5 ')).toBe(false);
    });
    it('tokenAsText ignores border-, outline- and accent-color and --tw-ring-color', () => {
      expect(hits(TOKEN_AS_TEXT, 'border-color: var(--accent-primary); outline-color: var(--accent-primary); accent-color: var(--accent-primary); --tw-ring-color: var(--accent-primary)')).toEqual([]);
      expect(hits(TOKEN_AS_TEXT, 'a { color: var(--accent-primary); }')).toHaveLength(1);
    });
    it('styleColours exempts black tints by the whole declaration', () => {
      const black = hits(STYLE_COLOUR, 'background: rgba(0, 0, 0, 0.6);');
      expect(black).toHaveLength(1);
      expect(BLACK_TINT.test(black[0])).toBe(true);
      const other = hits(STYLE_COLOUR, 'background: rgba(55, 65, 81, 0.5);');
      expect(other).toHaveLength(1);
      expect(BLACK_TINT.test(other[0])).toBe(false);
    });
    it('styleColours reports the real line in a multi-line style block', () => {
      const src = '<div></div>\n<style>\n  /* a\n  b */\n  .a { background: rgba(0, 0, 0, 0.5); }\n  .b {\n    color: #fff;\n    border-color: var(--x, #abc);\n  }\n</style>\n';
      const found = styleHits(src).map(([i, t]) => [lineOf(src, i), t]);
      expect(found).toEqual([[7, 'color: #fff']]);
    });
    it('blankStyle keeps line numbers', () => {
      const css = '/* a\n b */\n.x { color: #fff; }';
      expect(blankStyle(css)).toHaveLength(css.length);
      expect(blankStyle(css).split('\n')).toHaveLength(3);
    });
  });

  it('every component parses', () => {
    expect(parseFailures, parseFailures.join('\n')).toEqual([]);
  });

  for (const rule of Object.keys(findings)) {
    it(`${rule}: exactly ${BUDGET[rule]} findings`, () => {
      const list = findings[rule];
      const msg = `${rule}: ${list.length} findings (budget ${BUDGET[rule]})\n${list.join('\n')}`;
      if (BUDGET[rule] === 0) expect(list, msg).toEqual([]);
      else {
        expect(list.length, msg).toBeLessThanOrEqual(BUDGET[rule]);
        expect(list.length, `lower BUDGET.${rule} to ${list.length}`).toBe(BUDGET[rule]);
      }
      console.info(`a11y guard ${rule}: ${list.length}`);
    });
  }
});
