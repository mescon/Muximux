import type { App } from './types';

/**
 * True when a click carries the universal "open in a new browser tab"
 * intent: Ctrl+Click (Cmd+Click on macOS) or middle-click. Selecting an app
 * this way opens its effective URL in a new browser tab instead of the
 * app's configured open_mode (#407).
 *
 * http_action apps are excluded -- they have no page to open, so the
 * modifier falls through to the normal fire-action path.
 */
export function wantsNewTab(e: MouseEvent | undefined, app: App): boolean {
  if (!e) return false;
  if (app.open_mode === 'http_action') return false;
  return e.ctrlKey || e.metaKey || e.button === 1;
}
