/**
 * Navigation bridge for embedded apps.
 *
 * Lets an opted-in embedded app ask Muximux to switch to an app (itself or
 * another), optionally at a path inside that app.
 *
 * Protocol:
 *   window.parent.postMessage(
 *     { type: 'muximux:navigate', app: 'sonarr', path: '/series/foo' },
 *     '*'
 *   );
 *
 * Muximux replies to the sender with
 *   { type: 'muximux:navigate-result', ok: true }
 * or
 *   { type: 'muximux:navigate-result', ok: false, error: <NavigateError> }
 * posted with the sender's origin as targetOrigin. An optional `id` (a
 * string up to MAX_REQUEST_ID characters, or a finite number) on the
 * request is copied into the reply.
 *
 * A frame nested inside an opted-in app (a dashboard's iframe widget, say)
 * can send the same message; it counts as that app. Its origin must be the
 * origin of one of the configured apps, since any page the app frame shows
 * could otherwise nest a frame of its own and send as the app.
 *
 * A second message, muximux:frame-link, comes from Muximux itself: a link
 * without target="_top" in an app (a Homarr tile set to open in the current
 * tab) loads Muximux inside that app's frame. That copy does not start; it
 * sends { type: 'muximux:frame-link', hash, referrer, pushed } to this
 * window, which puts the app back and applies the hash (see framedLoad.ts).
 * The referrer, the page the link was on, must have a configured app's
 * origin for the same reason. No reply is sent.
 *
 * Security:
 * - The sender is identified by window handle (see bridgeSender.ts), never
 *   by the origin it claims.
 * - The sender must have allow_navigation: true in its Muximux config and
 *   its iframe must currently be visible, so a hidden background app cannot
 *   steer the UI.
 * - The target is looked up by slug or display name among enabled
 *   configured apps; only iframe-mode apps are valid targets.
 * - The path is validated by resolveDeepLink, which keeps it inside the
 *   target app; callers can never supply an arbitrary URL.
 *
 * Rate limit: at most one accepted navigation per sender app every
 * NAV_RATE_LIMIT_MS. Refused messages do not consume the slot.
 */

import type { App } from './types';
import { findNestedSenderFrame, findSenderFrame, frameOfWindow, isAppOrigin } from './bridgeSender';
import { resolveDeepLink } from './navigatePath';
import { slugify } from './slug';

export const NAV_RATE_LIMIT_MS = 500;
const MAX_APP_NAME = 200;
export const MAX_REQUEST_ID = 100;
export const MAX_FRAME_LINK_HASH = 2 * MAX_APP_NAME + 2;

// The request's optional id, echoed in the reply so a sender with several
// requests in flight can match each reply. Anything else is dropped.
function replyId(id: unknown): string | number | undefined {
  if (typeof id === 'string' && id.length <= MAX_REQUEST_ID) return id;
  if (typeof id === 'number' && Number.isFinite(id)) return id;
  return undefined;
}

export type NavigateError =
  | 'not_allowed'
  | 'not_visible'
  | 'bad_request'
  | 'unknown_app'
  | 'not_embeddable'
  | 'bad_path'
  | 'rate_limited';

export interface NavigateRequest {
  sender: App;
  senderFrame: HTMLIFrameElement;
  target: App;
  url?: string;
}

export interface FrameLinkRequest {
  sender: App;
  senderFrame: HTMLIFrameElement;
  /** The copy's location.hash: '' or starting with '#'. */
  hash: string;
  /** Whether the link's load added a history entry the shell can go back over. */
  pushed: boolean;
}

export interface NavigationBridgeOptions {
  getApps: () => App[];
  isFrameVisible: (frame: HTMLIFrameElement) => boolean;
  onNavigate: (req: NavigateRequest) => void;
  onFrameLink: (req: FrameLinkRequest) => void;
  /** Base URL for resolving proxied deep links. */
  baseUrl?: string;
  /** Clock override for tests; defaults to Date.now. */
  now?: () => number;
}

interface NavigateMessage {
  type: 'muximux:navigate';
  app?: unknown;
  path?: unknown;
}

// The origin of a copy's document.referrer; 'null' (never an app origin)
// when it is missing or not a URL.
function referrerOrigin(referrer: unknown): string {
  if (typeof referrer !== 'string' || referrer === '') return 'null';
  try {
    return new URL(referrer).origin;
  } catch {
    return 'null';
  }
}

function messageType(data: unknown): unknown {
  if (!data || typeof data !== 'object') return undefined;
  return (data as { type?: unknown }).type;
}

export function installNavigationBridge(opts: NavigationBridgeOptions): () => void {
  const now = opts.now ?? Date.now;
  const lastNavigated = new Map<string, number>();

  function reply(event: MessageEvent, payload: { ok: true } | { ok: false; error: NavigateError }): void {
    const id = replyId((event.data as { id?: unknown }).id);
    try {
      // Narrow to Window.postMessage (ports/SW have a different signature).
      (event.source as Window).postMessage(
        { type: 'muximux:navigate-result', ...payload, ...(id === undefined ? {} : { id }) },
        event.origin,
      );
    } catch {
      // Source may have gone away between dispatch and reply; nothing to do.
    }
  }

  // Warnings are always on (not gated by ?debug=true) so app developers can
  // see immediately why a navigate message did nothing.
  function refuse(event: MessageEvent, error: NavigateError, reason: string, app: string, hint: string): void {
    console.warn(`[muximux:navigate] ${reason}`, { app, hint });
    reply(event, { ok: false, error });
  }

  // One accepted navigation per sender every NAV_RATE_LIMIT_MS.
  function takeSlot(sender: App): boolean {
    const t = now();
    const last = lastNavigated.get(sender.name);
    if (last !== undefined && t - last < NAV_RATE_LIMIT_MS) return false;
    lastNavigated.set(sender.name, t);
    return true;
  }

  function handleMessage(event: MessageEvent): void {
    const type = messageType(event.data);
    if (type === 'muximux:navigate') handleNavigate(event, event.data as NavigateMessage);
    else if (type === 'muximux:frame-link') handleFrameLink(event, event.data as { hash?: unknown; referrer?: unknown; pushed?: unknown });
  }

  function handleNavigate(event: MessageEvent, data: NavigateMessage): void {
    const direct = findSenderFrame(opts.getApps(), event.origin, event.source);
    const nested = direct ? undefined : findNestedSenderFrame(opts.getApps(), event.source);
    const found = direct ?? nested;
    if (!found) {
      console.warn('[muximux:navigate] no matching app for message', {
        origin: event.origin,
        hint: 'The sending iframe must be a registered app in Muximux',
      });
      return;
    }
    const { app: sender, frame } = found;

    if (!sender.allow_navigation) {
      refuse(event, 'not_allowed', 'navigation not enabled for app', sender.name,
        'Toggle "Allow navigating Muximux" in the app\'s settings');
      return;
    }
    if (nested && !isAppOrigin(opts.getApps(), event.origin)) {
      refuse(event, 'not_allowed', 'a frame inside the app sent the message from an address that is not one of your apps',
        sender.name, `Serve the page from the address of an app in Muximux (it came from ${event.origin})`);
      return;
    }
    if (!opts.isFrameVisible(frame)) {
      refuse(event, 'not_visible', 'sender app is not the visible app', sender.name,
        'Only the app currently shown in Muximux can request navigation');
      return;
    }

    const targetName = data.app;
    const path = data.path;
    if (typeof targetName !== 'string' || targetName === '' || targetName.length > MAX_APP_NAME
      || (path !== undefined && typeof path !== 'string')) {
      refuse(event, 'bad_request', 'malformed navigate message', sender.name,
        'Send { type, app: "<app name or slug>", path?: "<string>" }');
      return;
    }

    // A disabled app is not shown in Muximux, so it is never a target.
    const apps = opts.getApps().filter(a => a.enabled !== false);
    const target = apps.find(a => slugify(a.name) === targetName)
      ?? apps.find(a => a.name === targetName);
    if (!target) {
      refuse(event, 'unknown_app', 'no app matches the requested name', sender.name,
        'Use the target app\'s name or slug exactly as configured in Muximux');
      return;
    }
    // An unset open_mode arrives as '' from the API and means iframe.
    if (target.open_mode && target.open_mode !== 'iframe') {
      refuse(event, 'not_embeddable', 'target app does not open in an iframe', sender.name,
        'Set the target app\'s open mode to iframe');
      return;
    }

    let url: string | undefined;
    if (path) {
      const resolved = resolveDeepLink(target, path, opts.baseUrl ?? '');
      if (!resolved.ok) {
        refuse(event, 'bad_path', 'path is not valid inside the target app', sender.name,
          'Use a path relative to the target app that stays within its base URL');
        return;
      }
      url = resolved.url;
    }

    if (!takeSlot(sender)) {
      refuse(event, 'rate_limited', 'navigating too fast', sender.name,
        `Wait at least ${NAV_RATE_LIMIT_MS} ms between navigate messages`);
      return;
    }

    opts.onNavigate({ sender, senderFrame: frame, target, url });
    reply(event, { ok: true });
  }

  // The copy is Muximux, so it must report this origin and be showing the
  // shell's own page; a same-origin proxied app cannot pass for it.
  function isShellCopy(frame: HTMLIFrameElement, origin: string): boolean {
    if (origin !== window.location.origin) return false;
    try {
      return frame.contentWindow?.location.pathname === window.location.pathname;
    } catch {
      return false;
    }
  }

  function handleFrameLink(event: MessageEvent, data: { hash?: unknown; referrer?: unknown; pushed?: unknown }): void {
    const found = frameOfWindow(opts.getApps(), event.source);
    if (!found || !isShellCopy(found.frame, event.origin)) return;
    const { app: sender, frame } = found;
    const warn = (reason: string, hint: string) =>
      console.warn(`[muximux:navigate] ${reason}`, { app: sender.name, hint });

    if (!sender.allow_navigation) {
      warn('a link in this app opened Muximux inside it, but navigation is not enabled for the app',
        'Toggle "Allow navigating Muximux" in the app\'s settings, or give the link target="_top"');
      return;
    }
    if (!opts.isFrameVisible(frame)) {
      warn('sender app is not the visible app', 'Only the app currently shown in Muximux can request navigation');
      return;
    }
    if (!isAppOrigin(opts.getApps(), referrerOrigin(data.referrer))) {
      warn('the link was not on the page of one of your apps, or the page sends no referrer',
        'Give the link target="_top"');
      return;
    }
    const hash = data.hash;
    // '' (a link to Muximux without a hash) only puts the app back.
    if (typeof hash !== 'string' || (hash !== '' && !hash.startsWith('#')) || hash.length > MAX_FRAME_LINK_HASH) {
      warn('malformed frame link', 'Link to Muximux\'s address followed by #<app>');
      return;
    }
    if (!takeSlot(sender)) {
      warn('navigating too fast', `Wait at least ${NAV_RATE_LIMIT_MS} ms between navigations`);
      return;
    }
    opts.onFrameLink({ sender, senderFrame: frame, hash, pushed: data.pushed === true });
  }

  window.addEventListener('message', handleMessage);
  return () => {
    window.removeEventListener('message', handleMessage);
  };
}
