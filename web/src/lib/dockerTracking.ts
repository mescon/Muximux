import type { App } from '$lib/types';

/**
 * Clears Docker tracking on every app in `apps` that is tracked under `key`,
 * in place, and returns how many were changed.
 *
 * Detaching happens server-side (Settings -> Discovery), but the Settings
 * dialog keeps its own copy of the config. Without this, the dialog went on
 * showing the app as Docker-managed with its URL locked until it was closed
 * and reopened (#479).
 */
export function clearDockerTracking(apps: App[], key: string): number {
  if (!key) return 0;
  let changed = 0;
  for (const app of apps) {
    if (app.docker_key !== key) continue;
    app.docker_key = undefined;
    app.docker_endpoint = undefined;
    app.docker_strategy = undefined;
    changed++;
  }
  return changed;
}

const TRACKING_FIELDS = new Set(['docker_key', 'docker_endpoint', 'docker_strategy', 'docker_managed_url', 'docker_auto']);

/**
 * JSON.stringify replacer that leaves out Docker tracking fields. The server
 * owns them (a save never changes them), so a detach made while the Settings
 * dialog is open must not count as an unsaved change.
 */
export function withoutDockerTracking(key: string, value: unknown): unknown {
  return TRACKING_FIELDS.has(key) ? undefined : value;
}
