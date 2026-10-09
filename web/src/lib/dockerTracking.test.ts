import { describe, it, expect } from 'vitest';
import { vi } from 'vitest';
import { refreshDockerTracking, syncDockerTracking, withoutDockerTracking } from './dockerTracking';
import type { Config } from './types';
import type { App } from './types';

const app = (name: string, key?: string, endpoint = 'unix:///d.sock'): App =>
  ({ name, url: 'http://x', docker_key: key, docker_endpoint: key ? endpoint : undefined, docker_strategy: key ? 'container_ip' : undefined }) as App;

describe('syncDockerTracking', () => {
  it('clears tracking the server dropped (detach)', () => {
    const local = [app('Sonarr', 'label:sonarr'), app('Radarr', 'label:radarr')];
    expect(syncDockerTracking(local, [app('Sonarr'), app('Radarr', 'label:radarr')])).toBe(1);
    expect(local[0]).toMatchObject({ docker_key: undefined, docker_endpoint: undefined, docker_strategy: undefined });
    expect(local[1].docker_key).toBe('label:radarr');
  });

  it('picks up a new key after a re-link, so a later detach still matches', () => {
    const local = [app('Sonarr', 'label:sonarr-old')];
    expect(syncDockerTracking(local, [app('Sonarr', 'label:sonarr-new')])).toBe(1);
    expect(local[0].docker_key).toBe('label:sonarr-new');
  });

  it('only follows the server, so an app on another endpoint keeps its tracking', () => {
    // The server detaches by key on the current endpoint only; the same key
    // under a different endpoint stays tracked there and here.
    const local = [app('A', 'name:web', 'unix:///d.sock'), app('B', 'name:web', 'tcp://other:2375')];
    syncDockerTracking(local, [app('A'), app('B', 'name:web', 'tcp://other:2375')]);
    expect(local[0].docker_key).toBeUndefined();
    expect(local[1]).toMatchObject({ docker_key: 'name:web', docker_endpoint: 'tcp://other:2375' });
  });

  it('leaves unknown apps and unchanged apps alone', () => {
    const local = [app('Local only', 'label:x'), app('Same', 'label:same')];
    expect(syncDockerTracking(local, [app('Same', 'label:same')])).toBe(0);
    expect(local[0].docker_key).toBe('label:x');
  });

  it('treats empty strings and missing fields as the same untracked state', () => {
    const local = [{ name: 'Manual', url: 'http://x', docker_key: '' } as App];
    expect(syncDockerTracking(local, [app('Manual')])).toBe(0);
  });
});

describe('withoutDockerTracking', () => {
  it('makes tracked and detached copies of an app serialize the same', () => {
    const tracked = app('Sonarr', 'label:sonarr');
    const detached = app('Sonarr');
    expect(JSON.stringify(tracked, withoutDockerTracking)).toBe(JSON.stringify(detached, withoutDockerTracking));
    expect(JSON.stringify(tracked, withoutDockerTracking)).toContain('"name":"Sonarr"');
  });
});

describe('refreshDockerTracking', () => {
  it('syncs every list from one fetch of the server config', async () => {
    const localApps = [app('Sonarr', 'label:sonarr')];
    const editing = [app('Sonarr', 'label:sonarr')];
    const fetchConfig = vi.fn().mockResolvedValue({ apps: [app('Sonarr')] } as unknown as Config);
    expect(await refreshDockerTracking([localApps, editing], fetchConfig)).toBe(true);
    expect(fetchConfig).toHaveBeenCalledTimes(1);
    expect(localApps[0].docker_key).toBeUndefined();
    expect(editing[0].docker_key).toBeUndefined();
  });

  it('leaves the local copies untouched when the fetch fails', async () => {
    const localApps = [app('Sonarr', 'label:sonarr')];
    expect(await refreshDockerTracking([localApps], () => Promise.reject(new Error('offline')))).toBe(false);
    expect(localApps[0].docker_key).toBe('label:sonarr');
  });

  it('copes with a config that has no apps list', async () => {
    const localApps = [app('Sonarr', 'label:sonarr')];
    expect(await refreshDockerTracking([localApps], () => Promise.resolve({} as Config))).toBe(true);
    expect(localApps[0].docker_key).toBe('label:sonarr');
  });
});

describe('docker_managed_health_check', () => {
  it('strips docker_managed_health_check with the other tracking fields', () => {
    expect(withoutDockerTracking('docker_managed_health_check', true)).toBeUndefined();
  });
  it('syncs docker_managed_health_check after a server-side detach, including false', () => {
    const local = [{ name: 'Emby', docker_managed_health_check: true }] as App[];
    expect(syncDockerTracking(local, [{ name: 'Emby', docker_managed_health_check: false }] as App[])).toBe(1);
    expect(local[0].docker_managed_health_check).toBe(false);
    expect(syncDockerTracking(local, [{ name: 'Emby' }] as App[])).toBe(1);
    expect(local[0].docker_managed_health_check).toBeUndefined();
  });
});

