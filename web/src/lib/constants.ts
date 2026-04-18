import type { App } from './types';
import * as m from '$lib/paraglide/messages.js';

export const openModes: readonly { value: App['open_mode']; get label(): string; get description(): string }[] = [
  { value: 'iframe', get label() { return m.appForm_embedded(); }, get description() { return m.appForm_embeddedDesc(); } },
  { value: 'new_tab', get label() { return m.appForm_newTab(); }, get description() { return m.appForm_newTabDesc(); } },
  { value: 'new_window', get label() { return m.appForm_newWindow(); }, get description() { return m.appForm_newWindowDesc(); } },
  { value: 'redirect', get label() { return m.appForm_redirect(); }, get description() { return m.appForm_redirectDesc(); } },
] as const;

/**
 * Metadata for an iframe Permissions-Policy feature that Muximux can delegate.
 * Used by the Settings UI to render checkboxes, tooltips, and "read more" links.
 */
export interface IframePermissionInfo {
  /** Permissions-Policy directive name (also used in the iframe `allow` attribute). */
  id: string;
  /** One-line description shown on hover. */
  description: string;
  /** External documentation URL (typically MDN). */
  docsUrl: string;
}

/**
 * The full set of browser feature policy permissions that Muximux can delegate
 * to an embedded iframe. Kept in one place so the UI checkboxes, the "all"
 * sentinel expansion, the Permissions-Policy header on the Go side, and the
 * iframe `allow` attribute builder stay in sync.
 */
export const IFRAME_PERMISSIONS: readonly IframePermissionInfo[] = [
  { id: 'camera', description: 'Capture video from the user\'s camera', docsUrl: 'https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Permissions-Policy/camera' },
  { id: 'microphone', description: 'Capture audio from the user\'s microphone', docsUrl: 'https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Permissions-Policy/microphone' },
  { id: 'geolocation', description: 'Read the user\'s geographic location', docsUrl: 'https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Permissions-Policy/geolocation' },
  { id: 'display-capture', description: 'Capture the user\'s screen for screen-sharing', docsUrl: 'https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Permissions-Policy/display-capture' },
  { id: 'fullscreen', description: 'Enter fullscreen mode', docsUrl: 'https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Permissions-Policy/fullscreen' },
  { id: 'clipboard-read', description: 'Read from the system clipboard', docsUrl: 'https://developer.mozilla.org/en-US/docs/Web/API/Clipboard/read' },
  { id: 'clipboard-write', description: 'Write to the system clipboard', docsUrl: 'https://developer.mozilla.org/en-US/docs/Web/API/Clipboard/write' },
  { id: 'autoplay', description: 'Play audio/video without a user gesture', docsUrl: 'https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Permissions-Policy/autoplay' },
  { id: 'midi', description: 'Communicate with MIDI devices (Web MIDI)', docsUrl: 'https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Permissions-Policy/midi' },
  { id: 'payment', description: 'Use the Payment Request API for checkout flows', docsUrl: 'https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Permissions-Policy/payment' },
  { id: 'publickey-credentials-get', description: 'Sign in with existing passkeys / WebAuthn credentials', docsUrl: 'https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Permissions-Policy/publickey-credentials-get' },
  { id: 'publickey-credentials-create', description: 'Register new passkeys / WebAuthn credentials', docsUrl: 'https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Permissions-Policy/publickey-credentials-create' },
  { id: 'encrypted-media', description: 'Play DRM-protected media (e.g. Plex / Jellyfin premium content)', docsUrl: 'https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Permissions-Policy/encrypted-media' },
  { id: 'screen-wake-lock', description: 'Prevent the screen from dimming or locking', docsUrl: 'https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Permissions-Policy/screen-wake-lock' },
  { id: 'picture-in-picture', description: 'Open videos in a floating Picture-in-Picture window', docsUrl: 'https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Permissions-Policy/picture-in-picture' },
  { id: 'usb', description: 'Communicate with USB devices (WebUSB)', docsUrl: 'https://developer.mozilla.org/en-US/docs/Web/API/WebUSB_API' },
  { id: 'serial', description: 'Communicate with serial ports (Web Serial)', docsUrl: 'https://developer.mozilla.org/en-US/docs/Web/API/Web_Serial_API' },
  { id: 'hid', description: 'Talk to HID devices like gamepads and security keys (WebHID)', docsUrl: 'https://developer.mozilla.org/en-US/docs/Web/API/WebHID_API' },
] as const;

/** Just the directive names, derived from IFRAME_PERMISSIONS. */
export const ALL_IFRAME_PERMISSIONS: readonly string[] = IFRAME_PERMISSIONS.map(p => p.id);

/**
 * Resolve `permissions` from config into the effective list of delegated
 * features, honouring the `all` / `none` sentinel values.
 *
 *   permissions: [all]       -> every permission in ALL_IFRAME_PERMISSIONS
 *   permissions: [none]      -> no permissions (same as unset)
 *   permissions: [camera]    -> just camera
 *   permissions: undefined   -> no permissions
 *
 * If both `all` and `none` appear, `none` wins (fail-safe: deny by default).
 */
export function resolvePermissions(perms: readonly string[] | undefined): string[] {
  if (!perms || perms.length === 0) return [];
  if (perms.includes('none')) return [];
  if (perms.includes('all')) return [...ALL_IFRAME_PERMISSIONS];
  return perms.filter(p => (ALL_IFRAME_PERMISSIONS as readonly string[]).includes(p));
}
