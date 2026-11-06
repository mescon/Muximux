/**
 * Deep-link paths in Muximux hash routes (issue #545).
 *
 *   #emby?path=%2Fweb%2Findex.html%23!%2Fitem%3Fid%3D123
 *   #emby+radarr?emby=%2Fweb%2F...&radarr=%2Fmovie%2F384978
 *
 * The part before `?` is the usual route. A single-app route takes `path=`
 * (or the app's slug as key); a split route takes one key per app slug.
 * A path opens only in a target with allow_deep_links, whatever the link
 * came from, and must pass the message API's path rule (resolveDeepLink).
 * Anything refused still switches apps and logs why.
 */

import type { App } from './types';
import { slugify } from './slug';
import { resolveDeepLink } from './navigatePath';
import { paneShowing } from './splitStore.svelte';
import { type DeepLinks, deepLinkKey, setDeepLink } from './linkNavigation';

const TAG = '[muximux:navigate]';

export interface HashRoute {
  /** The route without its query: 'emby' or 'emby+radarr'. */
  route: string;
  /** Decoded path per app slug of the route. */
  paths: Map<string, string>;
}

/**
 * Split a hash (without the leading '#') into its route and paths. Values
 * are decoded once with decodeURIComponent on the raw text: URLSearchParams
 * would turn a '+' in a path into a space.
 */
export function parseHashRoute(hash: string): HashRoute {
  const q = hash.indexOf('?');
  if (q === -1) return { route: hash, paths: new Map() };
  const route = hash.slice(0, q);
  const slugs = route.split('+', 2);
  const paths = new Map<string, string>();
  for (const pair of hash.slice(q + 1).split('&')) {
    if (!pair) continue;
    const eq = pair.indexOf('=');
    const key = eq === -1 ? pair : pair.slice(0, eq);
    const raw = eq === -1 ? '' : pair.slice(eq + 1);
    const slug = key === 'path' && slugs.length === 1 ? slugs[0] : key;
    if (!slugs.includes(slug)) {
      console.warn(`${TAG} ignoring "${key}" in the link: it names no app of #${route}.`,
        slugs.length === 1 ? 'Use path= for a single-app link.' : 'Use the app slugs as keys in a split link.');
      continue;
    }
    let value: string;
    try {
      value = decodeURIComponent(raw);
    } catch {
      console.warn(`${TAG} ignoring the path for "${slug}": it is not valid URL encoding.`);
      continue;
    }
    if (value) paths.set(slug, value);
  }
  return { route, paths };
}

/** The frame URL for a hash-route path, or null (with a warning) if refused. */
export function hashDeepLinkUrl(app: App, path: string, baseUrl: string): string | null {
  if (app.enabled === false) {
    console.warn(`${TAG} not opening the path in "${app.name}": the app is disabled.`);
    return null;
  }
  if (!app.allow_deep_links) {
    console.warn(`${TAG} not opening the path in "${app.name}": the app does not allow deep links.`,
      `Turn on "Allow deep links" for ${app.name} in Muximux.`);
    return null;
  }
  if (app.open_mode && app.open_mode !== 'iframe') {
    console.warn(`${TAG} not opening the path in "${app.name}": the app does not open in an iframe.`);
    return null;
  }
  const resolved = resolveDeepLink(app, path, baseUrl);
  if (!resolved.ok) {
    console.warn(`${TAG} not opening the path in "${app.name}": it is not valid inside the app.`,
      'Use a path relative to the app that stays within its base URL.');
    return null;
  }
  return resolved.url;
}

/**
 * Hand each accepted path to the frame showing its app. Call after the
 * route has placed the apps; an app not shown in any pane gets nothing.
 */
export function applyHashPaths(paths: Map<string, string>, apps: App[], links: DeepLinks, baseUrl: string): void {
  for (const [slug, path] of paths) {
    const app = apps.find(a => slugify(a.name) === slug);
    if (!app) continue;
    const pane = paneShowing(app.name);
    if (pane === null) continue;
    const url = hashDeepLinkUrl(app, path, baseUrl);
    if (url) setDeepLink(links, deepLinkKey(pane, app.name), url);
  }
}
