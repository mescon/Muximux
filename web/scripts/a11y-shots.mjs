// Screenshots of the key screens for the accessibility work, plus a pixelmatch diff
// between two runs. Not part of the test suite and adds no dependency to the repo.
//
// Setup (once):
//   mkdir -p /tmp/mxa11y && cd /tmp/mxa11y && npm init -y >/dev/null \
//     && npm i playwright pixelmatch pngjs && npx playwright install chromium
//
// Two phases, because the onboarding wizard needs a fresh instance and the rest needs a
// configured one (builtin auth, a few apps and groups, user admin / a11y-pass-word):
//   1. fresh data dir:   node a11y-shots.mjs --phase onboarding --base http://127.0.0.1:18411 \
//                          --data /tmp/mxa11y/data-fresh --out /tmp/mxa11y/after
//   2. seeded data dir:  node a11y-shots.mjs --phase login --base http://127.0.0.1:18411 --out /tmp/mxa11y/after
//                        node a11y-shots.mjs --phase main --themes <one theme> --base http://127.0.0.1:18411 \
//                          --out /tmp/mxa11y/after
//                        (main logs in and uses the theme from the data dir's config.yaml, so restart the
//                         instance with theme.family / theme.variant set for each theme)
//   3. compare:          node a11y-shots.mjs --phase diff --out /tmp/mxa11y/after --diff /tmp/mxa11y/before
// Common options: --themes muximux,muximux-light,solarized-light,gruvbox  [--axe]
//
// Animations are disabled (reduced motion plus an injected zero-duration stylesheet) so the
// two runs are comparable.
import { createRequire } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';

const require = createRequire(path.join(process.env.MXA11Y_DIR ?? '/tmp/mxa11y', 'package.json'));
const args = {};
for (let i = 2; i < process.argv.length; i++) {
  const a = process.argv[i];
  if (!a.startsWith('--')) continue;
  const next = process.argv[i + 1];
  args[a.slice(2)] = next === undefined || next.startsWith('--') ? true : next;
}
const OUT = args.out;
const PHASE = args.phase ?? 'main';
fs.mkdirSync(OUT, { recursive: true });

if (PHASE === 'diff') {
  const { PNG } = require('pngjs');
  const pm = require('pixelmatch'); // ESM-only in v6+: require() hands back the namespace
  const pixelmatch = pm.default ?? pm;
  const rows = ['| screenshot | differing px | note |', '|---|---|---|'];
  for (const f of fs.readdirSync(OUT).filter((f) => f.endsWith('.png') && !f.endsWith('.diff.png')).sort()) {
    const a = path.join(args.diff, f);
    if (!fs.existsSync(a)) { rows.push(`| ${f} | - | no before image |`); continue; }
    const img1 = PNG.sync.read(fs.readFileSync(a));
    const img2 = PNG.sync.read(fs.readFileSync(path.join(OUT, f)));
    if (img1.width !== img2.width || img1.height !== img2.height) {
      rows.push(`| ${f} | - | size differs ${img1.width}x${img1.height} vs ${img2.width}x${img2.height} |`);
      continue;
    }
    const diff = new PNG({ width: img1.width, height: img1.height });
    const n = pixelmatch(img1.data, img2.data, diff.data, img1.width, img1.height, { threshold: 0.05 });
    if (n > 0) fs.writeFileSync(path.join(OUT, f.replace('.png', '.diff.png')), PNG.sync.write(diff));
    rows.push(`| ${f} | ${n} | |`);
  }
  fs.writeFileSync(path.join(OUT, 'diff.md'), rows.join('\n') + '\n');
  console.log(rows.join('\n'));
  process.exit(0);
}

const BASE = args.base;
const THEMES = String(args.themes ?? 'muximux,muximux-light,solarized-light,gruvbox').split(',');
const USER = 'admin';
const PASS = 'a11y-pass-word';
const { chromium } = require('playwright');
const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1280, height: 900 }, reducedMotion: 'reduce' });
const page = await ctx.newPage();
const axeResults = [];

const KILL_ANIM = '*,*::before,*::after{animation:none!important;transition:none!important;caret-color:transparent!important}';
async function settle(ms = 600) {
  await page.addStyleTag({ content: KILL_ANIM }).catch(() => {});
  await page.waitForTimeout(ms);
}
async function shot(name) {
  await settle();
  await page.screenshot({ path: path.join(OUT, `${name}.png`), fullPage: true });
  if (args.axe) {
    const { default: AxeBuilder } = require('@axe-core/playwright');
    const r = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']).analyze();
    axeResults.push({ name, violations: r.violations.map((v) => ({ id: v.id, nodes: v.nodes.length })) });
  }
}
// The onboarding wizard resets the stored theme, so there the theme is forced on the document
// directly (stylesheet plus data-theme), as the shared theme CSS files are plain attribute selectors.
async function forceTheme(id) {
  if (!['muximux', 'muximux-light'].includes(id) && !(await page.$(`link[data-a11y="${id}"]`))) {
    await page.evaluate(([href, key]) => new Promise((res) => {
      const l = document.createElement('link');
      l.rel = 'stylesheet'; l.href = href; l.dataset.a11y = key; l.onload = res; l.onerror = res;
      document.head.appendChild(l);
    }), [`${BASE}/themes/${id}.css`, id]);
  }
  await page.evaluate((t) => {
    document.documentElement.dataset.theme = t;
    document.documentElement.classList.toggle('dark', !t.endsWith('light'));
  }, id);
  await page.waitForTimeout(150);
}
// Click through the DOM: with fullPage screenshots the dialog sometimes sits under its own backdrop for
// Playwright's hit test.
const click = async (re) => { await page.waitForTimeout(120); await page.getByRole('button', { name: re }).first().evaluate((el) => el.click()); };

if (PHASE === 'onboarding') {
  const token = fs.readFileSync(path.join(args.data, '.setup-token'), 'utf8').trim();
  await page.goto(BASE);
  for (const t of THEMES) {
    await page.goto(BASE);
    await page.waitForSelector('#setup-token');
    await forceTheme(t);
    await shot(`onboarding-welcome-${t}`);
    await page.fill('#setup-token', token);
    await click(/get started|let.?s get started/i);
    await page.waitForSelector('#setup-username, h2');
    await click(/password/i); // expands the builtin card
    await page.fill('#setup-username', 'admin');
    await page.fill('#setup-password', 'short');
    await page.fill('#setup-confirm', 'different');
    await forceTheme(t);
    await shot(`onboarding-security-short-password-${t}`);
    await page.fill('#setup-password', PASS);
    await page.fill('#setup-confirm', PASS);
    await forceTheme(t);
    await shot(`onboarding-security-builtin-${t}`);
    await click(/no authentication/i);
    await forceTheme(t);
    await shot(`onboarding-security-none-${t}`);
    await click(/no authentication/i);
    await click(/password/i);
    await page.fill('#setup-username', 'admin');
    await page.fill('#setup-password', PASS);
    await page.fill('#setup-confirm', PASS);
    await page.getByRole('button', { name: /continue/i }).last().click();
    await page.waitForTimeout(500);
    await forceTheme(t);
    await shot(`onboarding-apps-${t}`);
  }
} else if (PHASE === 'login') {
  // Logged out: login screen, plus a failed login for the error notice. The theme is forced on the
  // document because the stored theme comes from the config, which needs a session.
  await page.goto(`${BASE}/login`);
  await page.waitForSelector('#username');
  for (const t of THEMES) {
    await forceTheme(t);
    await shot(`login-${t}`);
    await page.fill('#username', USER);
    await page.fill('#password', 'wrong-password');
    await page.locator('form button[type=submit], form button.btn-primary').first().click();
    await page.waitForTimeout(700);
    await forceTheme(t);
    await shot(`login-error-${t}`);
  }
} else {
  // PHASE main: one theme per run. The theme comes from the seeded config (theme.family / theme.variant
  // of the data dir), which is what the app applies once logged in; stored themes are overridden by it.
  const t = THEMES[0];
  await page.goto(`${BASE}/login`);
  await page.waitForSelector('#username');
  await page.fill('#username', USER);
  await page.fill('#password', PASS);
  await page.locator('form button[type=submit], form button.btn-primary').first().click();
  await page.waitForTimeout(2500);
  {
    await page.goto(BASE);
    console.log(`theme ${t}: data-theme=${await page.evaluate(() => document.documentElement.dataset.theme)}`);
    await page.waitForTimeout(3500); // two health check rounds
    await shot(`splash-${t}`);
    // Open the default app so the navigation is shown with the health dots.
    const app = page.getByText('Whoami').first();
    await app.click().catch(() => {});
    await page.waitForTimeout(1200);
    await shot(`navigation-${t}`);

    await page.keyboard.press('l');
    await page.waitForTimeout(700);
    await shot(`logs-${t}`);
    await page.keyboard.press('Escape');

    await page.keyboard.press('s');
    await page.waitForTimeout(500);
    const tabs = [['general', /^general$/i], ['apps', /apps & groups/i], ['theme', /^theme$/i], ['keybindings', /keybindings/i],
      ['security', /^security$/i], ['gateway', /^gateway$/i], ['discovery', /^discovery$/i], ['about', /^about$/i]];
    for (const [id, re] of tabs) {
      await click(re);
      await page.waitForTimeout(500);
      await shot(`settings-${id}-${t}`);
    }
    await click(/^general$/i);
    await page.focus('#title');
    await page.keyboard.press('Tab');
    await shot(`settings-focus-${t}`);
    await page.fill('#title', 'Changed');
    await shot(`settings-unsaved-${t}`);
    await page.route('**/api/config', (route) => route.request().method() === 'PUT'
      ? route.fulfill({ status: 400, contentType: 'application/json', body: JSON.stringify({ error: 'validation failed: title is reserved' }) })
      : route.continue());
    await click(/save changes/i);
    await page.waitForTimeout(500);
    await shot(`settings-save-error-${t}`);
    await page.unroute('**/api/config');
    await page.keyboard.press('Escape');
    await page.waitForTimeout(400);
    await shot(`settings-discard-prompt-${t}`);
    await click(/keep editing/i);
    await page.fill('#title', 'Muximux'); // back to the saved value: nothing unsaved
    await page.keyboard.press('Escape');
    await page.waitForTimeout(500);
  }
}
if (args.axe) fs.writeFileSync(path.join(OUT, `axe-${PHASE}.json`), JSON.stringify(axeResults, null, 2));
await browser.close();
