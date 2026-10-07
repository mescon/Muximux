import { describe, it, expect } from 'vitest';
import { clearDockerTracking, withoutDockerTracking } from './dockerTracking';
import type { App } from './types';

const app = (name: string, key?: string): App =>
  ({ name, url: 'http://x', docker_key: key, docker_endpoint: key ? 'unix:///d.sock' : undefined, docker_strategy: key ? 'container_ip' : undefined }) as App;

describe('clearDockerTracking', () => {
  it('clears every tracking field on apps tracked under the key', () => {
    const apps = [app('Sonarr', 'label:sonarr'), app('Radarr', 'label:radarr'), app('Manual')];
    expect(clearDockerTracking(apps, 'label:sonarr')).toBe(1);
    expect(apps[0]).toMatchObject({ docker_key: undefined, docker_endpoint: undefined, docker_strategy: undefined });
    expect(apps[1].docker_key).toBe('label:radarr');
    expect(apps[2].docker_key).toBeUndefined();
  });

  it('does nothing for an unknown or empty key', () => {
    const apps = [app('Sonarr', 'label:sonarr')];
    expect(clearDockerTracking(apps, 'label:other')).toBe(0);
    expect(clearDockerTracking(apps, '')).toBe(0);
    expect(apps[0].docker_key).toBe('label:sonarr');
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
