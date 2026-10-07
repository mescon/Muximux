import type { App, Config } from '$lib/types';

const TRACKED = ['docker_key', 'docker_endpoint', 'docker_strategy'] as const;

/**
 * Copies Docker tracking from the server's apps onto the Settings dialog's
 * local copies, matched by name, in place. Returns how many apps changed.
 *
 * Tracking is server-owned and changes outside the dialog's save path
 * (Detach and Re-link under Settings -> Discovery), so the dialog asks the
 * server instead of guessing what changed. Without this it went on showing
 * a detached app as Docker-managed, with its URL locked, until it was
 * closed and reopened (#479). Apps the server does not know are left alone.
 */
export function syncDockerTracking(local: App[], server: App[]): number {
  const byName = new Map(server.map((a) => [a.name, a]));
  let changed = 0;
  for (const app of local) {
    const fresh = byName.get(app.name);
    if (!fresh) continue;
    let diff = false;
    for (const f of TRACKED) {
      if ((app[f] || undefined) !== (fresh[f] || undefined)) {
        app[f] = fresh[f] || undefined;
        diff = true;
      }
    }
    if (diff) changed++;
  }
  return changed;
}

/**
 * Fetches the server config and syncs tracking onto each list of local apps.
 * Returns false when the fetch fails; the local copies are then left as they
 * were, which is safe because a save never applies tracking fields.
 */
export async function refreshDockerTracking(lists: App[][], fetchConfig: () => Promise<Config>): Promise<boolean> {
  let server: App[];
  try {
    server = (await fetchConfig()).apps ?? [];
  } catch {
    return false;
  }
  for (const list of lists) syncDockerTracking(list, server);
  return true;
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
