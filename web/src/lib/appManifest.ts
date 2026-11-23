import { slugify } from './slug';
import type { App } from './types';

/**
 * Per-app PWA manifest (M-536): points the dashboard's single
 * `link[rel=manifest]` at `./manifest/<slug>.webmanifest` while a single app
 * route is showing, so "Install" installs that app as its own standalone
 * PWA instead of the whole dashboard. Chrome keys installs by manifest id,
 * and the server gives each app its own id, so the dashboard and each app
 * install separately.
 *
 * Relative hrefs keep installs working under a base_path (#436): absolute
 * URLs would resolve to the site root, outside the manifest's scope.
 */

export const DASHBOARD_MANIFEST_HREF = './manifest.json';

/** `./manifest/<slug>.webmanifest` for the app, or null when no single app is showing. */
export function manifestHrefForApp(app: App | null | undefined): string | null {
  if (!app) return null;
  const slug = slugify(app.name ?? '');
  if (!slug) return null;
  if (app.open_mode && app.open_mode !== 'iframe') return null;
  return `./manifest/${slug}.webmanifest`;
}

/**
 * Manifest href for a parsed hash route: the per-app manifest for a single
 * app route, the dashboard manifest for anything else (home, splits,
 * reserved hashes like #settings/#logs, empty hash).
 */
export function manifestHrefForRoute(route: string, apps: App[]): string {
  if (!route || route.includes('+')) return DASHBOARD_MANIFEST_HREF;
  const app = apps.find((a) => slugify(a.name ?? '') === route);
  return manifestHrefForApp(app) ?? DASHBOARD_MANIFEST_HREF;
}

/**
 * Point the injected `link[rel=manifest]` at `href` (no-op when already
 * there). A missing link is created the same way App.svelte's boot
 * injection does: credentialed fetch so forward-auth proxies let the
 * manifest through. Returns true when the link was created or retargeted.
 */
export function swapManifestLink(href: string): boolean {
  if (typeof document === 'undefined') return false;
  let link = document.querySelector<HTMLLinkElement>('link[rel="manifest"]');
  if (!link) {
    link = document.createElement('link');
    link.rel = 'manifest';
    link.crossOrigin = 'use-credentials';
    link.href = href;
    document.head.appendChild(link);
    return true;
  }
  if (link.getAttribute('href') === href) return false;
  link.href = href;
  link.crossOrigin = 'use-credentials';
  return true;
}

/**
 * Sync the manifest link to the current layout: per-app manifest on a
 * single-app route, dashboard manifest everywhere else. Returns true when
 * the link changed. Call after every committed route change (hashchange
 * handled, selectApp/updateHash, navigateHome) — not during transient
 * overlays (Settings/Logs keep whatever the underlying route shows).
 * Hrefs stay relative (`./…`) so installs keep working under a base_path
 * (#436): the build's `base: './'` resolves them against the SPA root.
 */
export function syncManifestToRoute(route: string, apps: App[]): boolean {
  return swapManifestLink(manifestHrefForRoute(route, apps));
}
