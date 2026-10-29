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
 * posted with the sender's origin as targetOrigin.
 *
 * Security:
 * - The sender is identified by window handle (see bridgeSender.ts), never
 *   by the origin it claims.
 * - The sender must have allow_navigation: true in its Muximux config and
 *   its iframe must currently be visible, so a hidden background app cannot
 *   steer the UI.
 * - The target is looked up by slug or display name among configured apps;
 *   only iframe-mode apps are valid targets.
 * - The path is validated by resolveDeepLink, which keeps it inside the
 *   target app; callers can never supply an arbitrary URL.
 *
 * Rate limit: at most one accepted navigation per sender app every
 * NAV_RATE_LIMIT_MS. Refused messages do not consume the slot.
 */

import type { App } from './types';
import { findSenderFrame } from './bridgeSender';
import { resolveDeepLink } from './navigatePath';
import { slugify } from './slug';

export const NAV_RATE_LIMIT_MS = 500;
const MAX_APP_NAME = 200;

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

export interface NavigationBridgeOptions {
  getApps: () => App[];
  isFrameVisible: (frame: HTMLIFrameElement) => boolean;
  onNavigate: (req: NavigateRequest) => void;
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

function isNavigateMessage(data: unknown): data is NavigateMessage {
  if (!data || typeof data !== 'object') return false;
  return (data as { type?: unknown }).type === 'muximux:navigate';
}

export function installNavigationBridge(opts: NavigationBridgeOptions): () => void {
  const now = opts.now ?? Date.now;
  const lastNavigated = new Map<string, number>();

  function reply(event: MessageEvent, payload: { ok: true } | { ok: false; error: NavigateError }): void {
    if (!event.source) return;
    try {
      // Narrow to Window.postMessage (ports/SW have a different signature).
      (event.source as Window).postMessage(
        { type: 'muximux:navigate-result', ...payload },
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

  function handleMessage(event: MessageEvent): void {
    if (!isNavigateMessage(event.data)) return;
    const data = event.data;

    const found = findSenderFrame(opts.getApps(), event.origin, event.source);
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

    const apps = opts.getApps();
    const target = apps.find(a => slugify(a.name) === targetName)
      ?? apps.find(a => a.name === targetName);
    if (!target) {
      refuse(event, 'unknown_app', 'no app matches the requested name', sender.name,
        'Use the target app\'s name or slug exactly as configured in Muximux');
      return;
    }
    if (target.open_mode !== undefined && target.open_mode !== 'iframe') {
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

    const t = now();
    const last = lastNavigated.get(sender.name);
    if (last !== undefined && t - last < NAV_RATE_LIMIT_MS) {
      refuse(event, 'rate_limited', 'navigating too fast', sender.name,
        `Wait at least ${NAV_RATE_LIMIT_MS} ms between navigate messages`);
      return;
    }
    lastNavigated.set(sender.name, t);

    opts.onNavigate({ sender, senderFrame: frame, target, url });
    reply(event, { ok: true });
  }

  window.addEventListener('message', handleMessage);
  return () => {
    window.removeEventListener('message', handleMessage);
  };
}
