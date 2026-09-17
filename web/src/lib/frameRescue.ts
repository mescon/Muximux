// Recovery for a proxied app frame that has lost its /proxy/<slug> prefix.
//
// Inside a proxied iframe the interceptor script keeps the address bar free
// of the proxy prefix so the app's router sees the paths it expects. The
// prefix is therefore missing whenever the browser makes a full document
// request for the frame: a reload of the frame, a history traversal to an
// evicted entry, or a navigation the interceptor could not rewrite (a
// location.href assignment in a browser without the Navigation API). That
// request lands on the Muximux shell, which would otherwise render the
// dashboard inside its own iframe.
//
// The iframe element carries the app's proxy path in its name attribute.
// window.name belongs to the browsing context, not the document, so it
// survives every same-origin navigation inside the frame. The shell checks
// it before mounting and sends the frame back to the proxied path.

export const FRAME_NAME_PREFIX = 'muximux-frame:';

// A proxy base is "<optional base path>/proxy/<slug>": root-relative, one
// segment per path component, no scheme or authority, so the redirect can
// never leave the origin.
const PROXY_BASE = /^(?:\/[A-Za-z0-9._~-]+)*\/proxy\/[a-z0-9-]+$/;

export interface FrameLocation {
  name: string;
  pathname: string;
  search: string;
  hash: string;
  /** Muximux base path, e.g. "/muximux"; empty when served at the root. */
  base: string;
}

/**
 * Returns the proxied URL a misrouted frame should load, or null when the
 * window is not a Muximux app frame or the path is already proxied.
 */
export function frameRescueTarget(loc: FrameLocation): string | null {
  if (!loc.name.startsWith(FRAME_NAME_PREFIX)) return null;
  const proxyBase = loc.name.slice(FRAME_NAME_PREFIX.length);
  if (!PROXY_BASE.test(proxyBase)) return null;

  let path = loc.pathname;
  if (path === proxyBase || path.startsWith(proxyBase + '/')) return null;
  if (loc.base && (path === loc.base || path.startsWith(loc.base + '/'))) {
    path = path.slice(loc.base.length);
  }
  if (path === '' || path === '/') path = '/';
  if (!path.startsWith('/')) return null;

  return proxyBase + path + loc.search + loc.hash;
}

/**
 * Redirects a misrouted app frame back to its proxied path. Returns true when
 * a redirect was issued, in which case the caller should not mount the shell.
 */
export function rescueMisroutedFrame(win: Window = window): boolean {
  let name: string;
  try {
    if (win.parent === win) return false;
    name = win.name;
  } catch {
    return false;
  }
  const base = (win as unknown as Record<string, unknown>).__MUXIMUX_BASE__;
  const target = frameRescueTarget({
    name,
    pathname: win.location.pathname,
    search: win.location.search,
    hash: win.location.hash,
    base: typeof base === 'string' ? base : '',
  });
  if (!target) return false;
  win.location.replace(target);
  return true;
}
