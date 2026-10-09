// Loads the bundled theme CSS the way the browser resolves it: every `:root`
// block of src/app.css is inherited in source order, then the theme's own
// block overrides. Used by themeContrast.test.ts; excluded from coverage.
import fs from 'node:fs';
import path from 'node:path';
import type { Mode, Vars } from '$lib/contrast';

export interface ThemeFixture { id: string; file: string; mode: Mode; vars: Vars }

const WEB = process.cwd(); // vitest runs from web/
export const APP_CSS = path.join(WEB, 'src', 'app.css');
export const THEMES_DIR = path.join(WEB, 'public', 'themes');

// Comments must go before block matching: a banner comment in front of a block
// would otherwise become part of its selector.
export function stripComments(css: string): string {
  return css.replace(/\/\*[\s\S]*?\*\//g, '');
}

export function parseVarBlock(css: string): Vars {
  const vars: Vars = {};
  for (const m of stripComments(css).matchAll(/(--[\w-]+)\s*:\s*([^;]+);/g)) vars[m[1]] = m[2].trim();
  return vars;
}

// Top-level `selector { ... }` blocks of a comment-free stylesheet (one level of braces, enough
// for app.css token blocks; @theme and @layer blocks are skipped because their selector starts with "@").
function topLevelBlocks(css: string): Array<{ selector: string; body: string }> {
  const out: Array<{ selector: string; body: string }> = [];
  const re = /(^|\n)([^@{}\n][^{}]*?)\{([^{}]*)\}/g;
  for (const m of css.matchAll(re)) out.push({ selector: m[2].trim(), body: m[3] });
  return out;
}

function appBlocks(match: (selector: string) => boolean): Vars {
  const css = stripComments(fs.readFileSync(APP_CSS, 'utf8'));
  const vars: Vars = {};
  for (const b of topLevelBlocks(css)) if (match(b.selector)) Object.assign(vars, parseVarBlock(b.body));
  return vars;
}

const selectors = (s: string) => s.split(',').map((x) => x.trim());

export function rootVars(): Vars {
  return appBlocks((s) => selectors(s).includes(':root'));
}
export function muximuxVars(): Vars {
  return { ...rootVars(), ...appBlocks((s) => selectors(s).includes('[data-theme="muximux"]')) };
}
export function muximuxLightVars(): Vars {
  return { ...rootVars(), ...appBlocks((s) => selectors(s).includes('[data-theme="muximux-light"]')) };
}

export function loadBundledThemes(): ThemeFixture[] {
  const base = rootVars();
  return fs.readdirSync(THEMES_DIR).filter((f) => f.endsWith('.css')).sort().map((file) => {
    const raw = fs.readFileSync(path.join(THEMES_DIR, file), 'utf8');
    // Metadata lives in the header comment: read it from the raw text.
    const id = raw.match(/@theme-id:\s*([\w-]+)/)?.[1] ?? file.replace(/\.css$/, '');
    const isDark = /@theme-is-dark:\s*true/.test(raw) || /color-scheme:\s*dark/.test(stripComments(raw));
    const blocks = topLevelBlocks(stripComments(raw)).filter((b) => /\[data-theme=/.test(b.selector));
    const own: Vars = {};
    for (const b of blocks) Object.assign(own, parseVarBlock(b.body));
    return { id, file, mode: isDark ? 'dark' : 'light', vars: { ...base, ...own } };
  });
}
