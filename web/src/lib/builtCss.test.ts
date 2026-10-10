// The generic semantic-token fallback must survive the production CSS pipeline.
// Vite's Lightning CSS minifier lowers light-dark() into var(--lightningcss-light, ...)
// helpers that only app.css's own color-scheme blocks switch, so a bundled or user
// theme file would always get the dark branch. app.css switches the fallback on
// <html data-color-scheme> instead; these tests hold that in the compiled output.
//
// The first test reproduces the build in-process (Tailwind compile, then Lightning CSS
// with Vite's default 'baseline-widely-available' targets), so it runs without a
// build. The second checks the real bundle in internal/server/dist when one exists
// that is newer than app.css (run `npm run build` first to exercise it).
import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { compile } from '@tailwindcss/node';
import { transform } from 'lightningcss';
import { APP_CSS } from '../test/themeFixtures';

const DIST_ASSETS = path.join(process.cwd(), '..', 'internal', 'server', 'dist', 'assets');
const version = (major: number, minor = 0) => (major << 16) | (minor << 8);
// vite/dist/node: ESBUILD_BASELINE_WIDELY_AVAILABLE_TARGET.
const VITE_TARGETS = { chrome: version(111), edge: version(111), firefox: version(114), safari: version(16, 4), ios_saf: version(16, 4) };

const FALLBACK_TOKENS = [
  ...['success', 'warning', 'danger', 'info'].flatMap((s) => [`--${s}-text`, `--${s}-bg`, `--${s}-border`]),
  '--danger-solid', '--danger-solid-hover', '--danger-on-solid', '--accent-text',
];

function assertNoLightDark(css: string) {
  expect(css).not.toMatch(/light-dark\(/);
  // The helpers may still be declared by color-scheme blocks, but nothing may read them.
  expect(css).not.toMatch(/var\(--lightningcss-/);
  for (const token of FALLBACK_TOKENS) {
    for (const m of css.matchAll(new RegExp(`${token}:([^;}]*)`, 'g'))) expect(m[1], token).not.toMatch(/lightningcss/);
  }
  expect(css).toMatch(/--danger-on-solid:var\(--fallback-on-solid\)/);
  expect(css).toMatch(/:root\[data-color-scheme="?light"?\]\{--fallback-ink:(?:black|#000);--fallback-on-solid:#fff(?:fff)?\}/);
}

describe('built CSS', () => {
  it('has no light-dark() or Lightning CSS helpers in the fallback tokens (in-process build)', async () => {
    const source = fs.readFileSync(APP_CSS, 'utf8');
    const compiler = await compile(source, { base: path.dirname(APP_CSS), from: APP_CSS, onDependency: () => {} });
    const out = transform({
      filename: 'app.css',
      code: Buffer.from(compiler.build([])),
      minify: true,
      targets: VITE_TARGETS,
    }).code.toString();
    assertNoLightDark(out);
  });

  // Only a bundle built from the current app.css counts: a stale one from an older
  // checkout would fail for reasons this tree has already fixed.
  const sourceTime = fs.statSync(APP_CSS).mtimeMs;
  const built = fs.existsSync(DIST_ASSETS)
    ? fs.readdirSync(DIST_ASSETS).filter((f) => /^index-.*\.css$/.test(f) && fs.statSync(path.join(DIST_ASSETS, f)).mtimeMs >= sourceTime)
    : [];
  it.skipIf(built.length === 0)('has no light-dark() or Lightning CSS helpers in internal/server/dist (when built from this app.css)', () => {
    for (const f of built) assertNoLightDark(fs.readFileSync(path.join(DIST_ASSETS, f), 'utf8'));
  });
});
