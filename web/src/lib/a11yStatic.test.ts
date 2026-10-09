// Static accessibility guard. Scans every component for colour classes and CSS that
// bypass the theme tokens, for focus outlines removed without replacement, for form
// controls without a programmatic label and for icon-only buttons without a name.
// The guard is strict: any finding fails the test and is listed as file:line.
//
// Scope: .svelte components (class strings, <style> blocks, style="..." and style={...}
// text colours, style:color directives) and the class rules over every non-test .ts file
// under src/ (a class map in a store would otherwise pass). Out of scope, on purpose:
// - background/border colours in inline styles: the inline style= sites carry user app and
//   group colours (Navigation, AppIcon, theme previews), which are data, not theme colours;
// - gradients: only the theme preview swatches use them, built from theme values;
// - colours in .ts string values outside class names (hex defaults for user-picked app and
//   group colours, the contrast maths); only Tailwind class names are checked there.
import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { parse, type AST } from 'svelte/compiler';

const SRC = path.join(process.cwd(), 'src');

const PALETTE = 'red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose|slate|gray|zinc|neutral|stone|brand';
const PROPS = 'text|bg|border|border-t|border-b|border-l|border-r|border-x|border-y|border-s|border-e|ring|ring-offset|outline|divide|from|via|to|fill|stroke|placeholder|accent|caret|shadow|decoration';
const PALETTE_CLASS = new RegExp(String.raw`(?<![\w-])(?:[\w-]+:)*!?(?:${PROPS})-(?:${PALETTE})-\d{2,3}(?:/\d{1,3})?(?![\w-])`, 'g');
// Arbitrary colour values, e.g. text-[#fff] or bg-[rgb(1,2,3)]; counted with the palette rule.
const HEX_CLASS = new RegExp(String.raw`(?<![\w-])(?:[\w-]+:)*!?(?:${PROPS})-\[(?:#[0-9a-fA-F]{3,8}|(?:rgba?|hsla?|oklch|oklab|lab|lch|hwb)\([^\]\s]*\))\](?![\w-])`, 'g');
const WHITE_BLACK = new RegExp(String.raw`(?<![\w-])(?:[\w-]+:)*!?(?:${PROPS})-(?:white|black)(?:/\d{1,3})?(?![\w-])`, 'g');
// A whole declaration (up to ";"), so the black-tint exemption below sees all of it.
const STYLE_COLOUR = /(?<![\w-])(?:color|background(?:-color)?|border(?:-top|-right|-bottom|-left)?(?:-color)?|fill|stroke|outline(?:-color)?)\s*:[^;{}]*?(?:#[0-9a-f]{3,8}\b|\brgba?\(|\b(?:white|black)\b)[^;{}]*/gi;
// Text colour in an inline style="..." attribute.
const INLINE_COLOUR = /(?<![\w-])color\s*:[^;"]*?(?:#[0-9a-f]{3,8}\b|\brgba?\(|\b(?:white|black)\b)/gi;
const BLACK_TINT = /rgba?\(\s*0\s*,\s*0\s*,\s*0\s*[,)]/;
// (?<![\w-]) keeps border-color, outline-color, accent-color and --tw-ring-color out.
const TOKEN_AS_TEXT = /(?<![\w-])color\s*:\s*var\(--(?:status-(?:success|warning|error|info)|accent-primary)\)/g;
const ARBITRARY_TOKEN = /(?<![\w-])(?:[\w-]+:)*!?(?:text|bg|border|ring|outline|divide|fill|stroke|decoration|caret|accent)-\[var\(--(?:accent-primary|status-[\w-]+)\)\]/g;
// Counts every way of removing the focus outline: outline-none, outline-hidden and outline-0
// utilities (any variant) and outline: none / outline: 0 declarations. Task 13 zeroed the
// count by relying on the global :focus-visible rule in app.css instead.
// A focus ring utility is a second focus indicator on top of the global outline (spec S7).
const FOCUS_RING = /(?<![\w-])(?:[\w-]+:)*focus(?:-visible|-within)?:ring(?:-[\w/.[\]()-]+)?(?![\w-])/g;
const OUTLINE = /(?<![\w-])(?:[\w-]+:)*outline-(?:none|hidden|0)(?![\w-])|(?<![\w-])outline\s*:\s*(?:none|0(?:px)?)(?![\w.%-])/g;

// A hover utility identical to the element's own base utility changes nothing, so pointer
// users get no hover cue (the brand-400 -> brand-300 shifts collapsed onto one token this
// way). Matched per literal segment: text between quotes, backticks and braces, so the two
// arms of a {cond ? 'a' : 'b'} ternary are judged separately.
const NOOP_HOVER_UTIL = /^(?:(?:text|bg|border(?:-[trblxyse])?|decoration|ring|outline|fill|stroke)-.+|underline|no-underline|line-through)$/;
function noOpHovers(src: string): Array<[number, string]> {
  const out: Array<[number, string]> = [];
  for (const seg of src.matchAll(/[^"'`{}]+/g)) {
    const tokens = [...seg[0].matchAll(/\S+/g)];
    const base = new Set(tokens.map((t) => t[0]).filter((t) => !t.includes(':')));
    for (const t of tokens) {
      const m = /^hover:(.+)$/.exec(t[0]);
      if (m && !m[1].includes(':') && NOOP_HOVER_UTIL.test(m[1]) && base.has(m[1])) out.push([seg.index! + t.index!, `${m[1]} ${t[0]}`]);
    }
  }
  return out;
}

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

function walk(dir: string, ext: string): string[] {
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) return e.name === 'paraglide' || e.name === 'test' ? [] : walk(p, ext);
    return p.endsWith(ext) && !/\.test\./.test(p) && !p.endsWith('.d.ts') ? [p] : [];
  });
}
const files = walk(SRC, '.svelte');
const tsFiles = walk(SRC, '.ts');

// Text colour in style={...} expressions and colour literals in style:color directives.
function styleExprHits(src: string): Array<[number, string]> {
  const out: Array<[number, string]> = [];
  for (const a of src.matchAll(/\sstyle=\{/g)) {
    const start = a.index! + a[0].length;
    let depth = 1, i = start;
    for (; i < src.length && depth > 0; i++) {
      if (src[i] === '{') depth++;
      else if (src[i] === '}') depth--;
    }
    const expr = src.slice(start, i - 1);
    for (const m of expr.matchAll(INLINE_COLOUR)) out.push([start + m.index!, `style={}: ${m[0]}`]);
  }
  for (const m of src.matchAll(/\sstyle:color(?:=(?:"([^"]*)"|\{([^}]*)\}))?/g)) {
    const v = m[1] ?? m[2] ?? '';
    if (/#[0-9a-f]{3,8}\b|\brgba?\(|\b(?:white|black)\b/i.test(v)) out.push([m.index! + 1, m[0].trim()]);
  }
  return out;
}
const rel = (f: string) => path.relative(SRC, f);
const lineOf = (src: string, idx: number) => src.slice(0, idx).split('\n').length;

type Finding = string;
const findings: Record<string, Finding[]> = { palette: [], whiteBlack: [], styleColours: [], tokenAsText: [], arbitraryToken: [], outline: [], focusRing: [], noOpHover: [], unlabeled: [], unnamedButtons: [], nestedInteractive: [] };

// Class rules over .ts files: a class map or a class string built in a store or helper.
for (const f of tsFiles) {
  const src = fs.readFileSync(f, 'utf8');
  const add = (rule: string, idx: number, text: string) => findings[rule].push(`${rel(f)}:${lineOf(src, idx)} ${text.trim()}`);
  for (const m of src.matchAll(PALETTE_CLASS)) add('palette', m.index!, m[0]);
  for (const m of src.matchAll(HEX_CLASS)) add('palette', m.index!, m[0]);
  for (const m of src.matchAll(WHITE_BLACK)) if (!isAllowed(f, m[0], '')) add('whiteBlack', m.index!, m[0]);
  for (const m of src.matchAll(ARBITRARY_TOKEN)) add('arbitraryToken', m.index!, m[0]);
  for (const m of src.matchAll(FOCUS_RING)) add('focusRing', m.index!, m[0]);
  for (const [idx, text] of noOpHovers(src)) add('noOpHover', idx, text);
}
const parseFailures: string[] = [];

// An element a user can focus or activate. Mirrors axe's nested-interactive: such an element
// must not contain another one (the inner control is unreachable or unnamed for screen readers).
const INTERACTIVE_ROLES = /^(?:button|link|checkbox|radio|switch|tab|menuitem|menuitemcheckbox|menuitemradio|option|slider|spinbutton|textbox|combobox|searchbox)$/;
// Pure so the rule can be unit tested on snippets. `attr` returns the attribute node or undefined.
function interactiveElement(
  n: { name: string },
  attr: (name: string) => unknown,
  attrText: (a: never) => string,
): boolean {
  const text = (name: string) => attrText(attr(name) as never);
  if (n.name === 'button' || n.name === 'select' || n.name === 'textarea' || n.name === 'summary') return true;
  if (n.name === 'a') return !!attr('href');
  if (n.name === 'input') return !/^hidden$/.test(text('type'));
  const role = text('role');
  if (INTERACTIVE_ROLES.test(role)) return true;
  return false;
}

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
  for (const m of src.matchAll(FOCUS_RING)) add('focusRing', m.index!, m[0]);
  for (const [idx, text] of styleExprHits(src)) add('styleColours', idx, text);
  for (const [idx, text] of styleHits(src)) add('styleColours', idx, text);
  for (const [idx, text] of noOpHovers(src)) add('noOpHover', idx, text);
  for (const a of src.matchAll(/\sstyle="([^"]*)"/g)) {
    const offset = a.index! + a[0].indexOf(a[1]);
    for (const m of a[1].matchAll(INLINE_COLOUR)) add('styleColours', offset + m.index!, `style: ${m[0]}`);
  }
  // AST pass: unlabeled controls and unnamed icon-only buttons (the audit's static.mjs logic)
  let ast: AST.Root;
  try { ast = parse(src, { modern: true }); } catch (e) { parseFailures.push(`${rel(f)}: ${(e as Error).message}`); continue; }
  const attr = (n: WithAttrs, name: string): AST.Attribute | undefined =>
    n.attributes.find((a): a is AST.Attribute => a.type === 'Attribute' && a.name === name);
  const isInteractive = (n: AST.RegularElement): boolean => interactiveElement(n, (name) => attr(n, name), attrText);
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
      if (isInteractive(el)) {
        const outer = anc.find(isInteractive);
        if (outer) findings.nestedInteractive.push(`${rel(f)}:${lineOf(src, el.start)} <${el.name}> inside <${outer.name}> at line ${lineOf(src, outer.start)}`);
      }
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
    it('palette also flags arbitrary rgb/hsl/oklch colour values', () => {
      expect(hits(HEX_CLASS, 'bg-[rgb(1,2,3)] text-[oklch(0.5_0.1_20)] hover:border-[hsl(0_0%_50%)]')).toHaveLength(3);
      expect(hits(HEX_CLASS, 'bg-[var(--x)] w-[calc(100%-2px)] text-[14px]')).toEqual([]);
    });
    it('focusRing flags focus ring utilities with any variant, not decorative rings', () => {
      expect(hits(FOCUS_RING, 'focus:ring-2 focus-visible:ring-accent-primary sm:focus:ring focus-within:ring-[var(--x)]')).toHaveLength(4);
      expect(hits(FOCUS_RING, 'ring-2 ring-accent-primary hover:ring-1 focus:outline-offset-2 my-focus:ring-2x')).toEqual([]);
    });
    it('styleColours covers style={...} text colours and style:color literals', () => {
      const src = '<a style={x ? `color: #fff` : "color: var(--text-primary)"}></a>\n<b style:color="#000"></b><i style:color={c}></i><u style={`border-bottom: 2px solid ${c}`}></u>';
      expect(styleExprHits(src).map(([i, t]) => [lineOf(src, i), t])).toEqual([[1, 'style={}: color: #fff'], [2, 'style:color="#000"']]);
    });
    it('whiteBlack flags white and black utilities with variants, not bg-black/50 lookalikes', () => {
      expect(hits(WHITE_BLACK, 'text-white hover:bg-black/40 border-t-white focus:ring-white')).toHaveLength(4);
      expect(hits(WHITE_BLACK, 'text-whitespace bg-blackish text-white-ish white')).toEqual([]);
    });
    it('outline flags outline-none, outline-hidden, outline-0, outline: none and outline: 0, not outline-offset or lookalikes', () => {
      expect(hits(OUTLINE, 'outline-none focus:outline-none .a { outline: none; } .b { outline:none }')).toHaveLength(4);
      expect(hits(OUTLINE, 'outline-hidden focus-visible:outline-hidden outline-0 focus:outline-0 .a { outline: 0; } .b { outline:0px }')).toHaveLength(6);
      expect(hits(OUTLINE, 'outline-2 outline-offset-2 outline-offset-0 outline-nonexistent outline-01 .a { outline: 2px solid red; } .b { outline-offset: 0; } .c { outline: 0.5px solid red; } --my-outline: none;')).toEqual([]);
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
    it('noOpHover flags a hover colour or decoration identical to the base, per literal segment', () => {
      const found = (s: string) => noOpHovers(s).map(([, t]) => t);
      expect(found('class="text-accent-text hover:text-accent-text"')).toEqual(['text-accent-text hover:text-accent-text']);
      expect(found('class="underline hover:underline"')).toEqual(['underline hover:underline']);
      expect(found("class=\"{x ? 'a' : 'border-border bg-bg-surface hover:border-border'}\"")).toEqual(['border-border hover:border-border']);
      expect(found('class="bg-accent-muted text-accent-text hover:bg-accent-muted"')).toEqual(['bg-accent-muted hover:bg-accent-muted']);
      // Different values, other variants and separate ternary arms are not no-ops.
      expect(found('class="text-text-muted hover:text-text-primary hover:underline"')).toEqual([]);
      expect(found('class="underline hover:decoration-2 dark:text-a hover:text-a"')).toEqual([]);
      expect(found("class=\"{on ? 'text-text-primary' : 'text-text-secondary hover:text-text-primary'}\"")).toEqual([]);
      expect(found('class="w-4 hover:w-4 sm:hover:text-a text-a"')).toEqual([]);
      // The reported index points at the hover token.
      const src = 'a\n<a class="text-x hover:text-x">';
      expect(lineOf(src, noOpHovers(src)[0][0])).toBe(2);
    });
    it('nestedInteractive flags a control inside a button, link or role=button, not siblings or presentation wrappers', () => {
      const nested = (markup: string): string[] => {
        const out: string[] = [];
        const walkNode = (n: unknown, anc: string[]): void => {
          if (!isObj(n)) return;
          let next = anc;
          if (n.type === 'RegularElement') {
            const el = n as unknown as AST.RegularElement;
            const a = (name: string) => el.attributes.find((x) => x.type === 'Attribute' && x.name === name);
            const t = (x: unknown) => { const at = x as AST.Attribute | undefined; return at ? markup.slice(at.start, at.end).replace(/^[\w-]+=/, '').replace(/["']/g, '') : ''; };
            if (interactiveElement(el, a, t as (x: never) => string)) {
              if (anc.length) out.push(`${el.name} in ${anc[0]}`);
              next = [...anc, el.name];
            }
          }
          for (const k of Object.keys(n)) {
            if (k === 'parent' || k === 'metadata') continue;
            const v = n[k];
            if (Array.isArray(v)) v.forEach((c) => walkNode(c, next));
            else if (isObj(v)) { if (v.type) walkNode(v, next); else if (Array.isArray(v.nodes)) v.nodes.forEach((c) => walkNode(c, next)); }
          }
        };
        walkNode(parse(markup, { modern: true }).fragment, []);
        return out;
      };
      expect(nested('<div role="button" tabindex="0"><button>x</button></div>')).toEqual(['button in div']);
      expect(nested('<button><input type="checkbox" /></button>')).toEqual(['input in button']);
      expect(nested('<a href="/x"><button>x</button></a>')).toEqual(['button in a']);
      expect(nested('<div class="card"><button>pick</button><button>delete</button></div>')).toEqual([]);
      expect(nested('<div role="presentation"><button>x</button></div><a name="x"><button>y</button></a>')).toEqual([]);
      expect(nested('<button><input type="hidden" /></button>')).toEqual([]);
    });
    it('blankStyle keeps line numbers', () => {
      const css = '/* a\n b */\n.x { color: #fff; }';
      expect(blankStyle(css)).toHaveLength(css.length);
      expect(blankStyle(css).split('\n')).toHaveLength(3);
    });
  });

  it('app.css keeps the global :focus-visible outline in the focus colour', () => {
    const css = fs.readFileSync(path.join(SRC, 'app.css'), 'utf8');
    expect(css).toMatch(/(?:^|\n):focus-visible\s*\{[^}]*outline:\s*2px solid var\(--border-focus\)/);
    expect([...blankStyle(css).matchAll(OUTLINE)].map((m) => m[0])).toEqual([]);
  });

  it('scans components and non-test .ts files', () => {
    expect(files.length).toBeGreaterThan(30);
    expect(tsFiles.some((f) => f.endsWith(path.join('lib', 'themeStore.ts')))).toBe(true);
    expect(tsFiles.some((f) => /\.test\.ts$/.test(f))).toBe(false);
  });

  it('every component parses', () => {
    expect(parseFailures, parseFailures.join('\n')).toEqual([]);
  });

  for (const rule of Object.keys(findings)) {
    it(`${rule}: no findings`, () => {
      const list = findings[rule];
      expect(list, `${rule}: ${list.length} findings\n${list.join('\n')}`).toEqual([]);
    });
  }
});
