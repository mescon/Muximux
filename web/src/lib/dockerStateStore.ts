import { writable, get } from 'svelte/store';
import type { DockerState } from './types';
import { debug } from './debug';

// Map keyed by app name. Build a new Map on every mutation so
// downstream consumers that memoise by reference equality (e.g.
// $derived chains in Svelte 5) re-run when the store ticks.
export const dockerStateStore = writable<Map<string, DockerState>>(new Map());

// refreshDockerState fetches the full snapshot via GET /api/discovery/docker-state.
// Called once at app mount; thereafter the WebSocket docker_state_changed
// event keeps the store in sync without re-polling.
export async function refreshDockerState(): Promise<void> {
  try {
    const res = await fetch('/api/discovery/docker-state', { credentials: 'same-origin' });
    if (!res.ok) {
      debug('docker-state', 'refresh failed', { status: res.status });
      return;
    }
    const payload = (await res.json()) as Record<string, DockerState>;
    const next = new Map<string, DockerState>();
    for (const [name, state] of Object.entries(payload)) {
      next.set(name, state);
    }
    dockerStateStore.set(next);
    debug('docker-state', 'refreshed', { apps: next.size });
  } catch (e) {
    debug('docker-state', 'refresh error', e);
  }
}

// applyDockerStateChange is the WebSocket-event handler entry point.
// Pulled out of websocketStore.ts so it can be unit-tested directly
// without spinning up a socket.
export function applyDockerStateChange(appName: string, state: DockerState): void {
  dockerStateStore.update((prev) => {
    const next = new Map(prev);
    next.set(appName, state);
    return next;
  });
}

// getDockerStateFor is a convenience accessor for components that
// only care about a single app (e.g. Splash cards). Returns
// undefined when the app is not Docker-tracked.
export function getDockerStateFor(appName: string): DockerState | undefined {
  return get(dockerStateStore).get(appName);
}
