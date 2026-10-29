import type { App } from './types';
import { isSafeAppUrl } from './appUrl';

// resolveDeepLink is the security boundary for app deep links: it decides
// whether a path requested for an embedded app stays inside that app. The
// path is resolved against the app URL and rejected unless the result keeps
// the app's origin and its base directory (the app URL path up to its last
// "/", or the whole path when its last segment has no "."), so "../", absolute URLs, "//host", backslash tricks, control
// characters, and javascript:/data: schemes can never leave the app. Proxied
// apps are then mapped onto their proxy path (the app URL path with one
// trailing "/" trimmed, as the backend proxy does), and the final URL must
// still pass isSafeAppUrl.

export const MAX_NAV_PATH = 2048;

export type DeepLinkResult = { ok: true; url: string } | { ok: false };

const REJECT: DeepLinkResult = { ok: false };
// eslint-disable-next-line no-control-regex
const CONTROL_CHARS = /[\u0000-\u001f\u007f]/;
const SCHEME = /^[a-z][a-z0-9+.-]*:/i;
const ENCODED_SEPARATOR = /%2f|%5c/i;

// A server may percent-decode the path after the URL parser has accepted it,
// so "..%2fx" would become "../x" upstream. Reject encoded separators and any
// segment that decodes to a dot segment (or fails to decode at all).
function hasUnsafeSegments(pathname: string): boolean {
  if (ENCODED_SEPARATOR.test(pathname)) return true;
  try {
    return pathname.split('/').some((seg) => {
      const d = decodeURIComponent(seg);
      return d === '.' || d === '..';
    });
  } catch {
    return true;
  }
}

export function resolveDeepLink(app: App, path: string, base: string = ''): DeepLinkResult {
  if (path.length > MAX_NAV_PATH || CONTROL_CHARS.test(path) || SCHEME.test(path)) return REJECT;
  if (path.startsWith('//') || path.startsWith('\\') || path.startsWith('/\\')) return REJECT;

  let appBase: URL;
  let u: URL;
  try {
    appBase = new URL(app.url, window.location.origin);
    // A last segment without a "." (http://host/sonarr) names a directory,
    // not a file, so treat it like http://host/sonarr/.
    const last = appBase.pathname.slice(appBase.pathname.lastIndexOf('/') + 1);
    if (last !== '' && !last.includes('.')) appBase.pathname += '/';
    u = new URL(path, appBase);
  } catch {
    return REJECT;
  }

  const baseDir = appBase.pathname.slice(0, appBase.pathname.lastIndexOf('/') + 1);
  // The directory itself without its slash (/sonarr) is the app's own page.
  const inBase = u.pathname.startsWith(baseDir) || u.pathname + '/' === baseDir;
  if (u.origin !== appBase.origin || !inBase) return REJECT;
  if (hasUnsafeSegments(u.pathname)) return REJECT;

  let result = u.href;
  if (app.proxyUrl) {
    const targetPath = appBase.pathname.replace(/\/$/, '');
    if (targetPath !== '' && u.pathname !== targetPath && !u.pathname.startsWith(targetPath + '/')) {
      return REJECT;
    }
    const rest = u.pathname.slice(targetPath.length).replace(/^\//, '');
    result = base + app.proxyUrl + rest + u.search + u.hash;
  }

  return isSafeAppUrl(result) ? { ok: true, url: result } : REJECT;
}

/**
 * Whether navigating a frame from `from` to `to` only changes the fragment.
 * Browsers then scroll or fire hashchange inside the frame but fire no load
 * event, so a loading overlay waiting for `load` would never clear.
 */
export function isFragmentOnlyChange(from: string, to: string): boolean {
  if (!to.includes('#')) return false;
  try {
    const a = new URL(from, window.location.href);
    const b = new URL(to, window.location.href);
    a.hash = '';
    b.hash = '';
    return a.href === b.href;
  } catch {
    return false;
  }
}
