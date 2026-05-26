import { describe, it, expect, beforeEach, vi } from 'vitest';
import { get } from 'svelte/store';
import { dockerStateStore, refreshDockerState, applyDockerStateChange } from './dockerStateStore';
import type { DockerState } from './types';

describe('dockerStateStore', () => {
  beforeEach(() => {
    dockerStateStore.set(new Map());
    vi.restoreAllMocks();
  });

  it('starts empty', () => {
    expect(get(dockerStateStore).size).toBe(0);
  });

  it('refreshDockerState populates from API', async () => {
    const payload: Record<string, DockerState> = {
      sonarr: { status: 'running', health: 'healthy', restart_count: 0, image: 'sonarr:latest' },
      radarr: { status: 'exited', health: 'none', restart_count: 2, image: 'radarr:latest' },
    };
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => payload,
    }));

    await refreshDockerState();
    const map = get(dockerStateStore);
    expect(map.size).toBe(2);
    expect(map.get('sonarr')?.status).toBe('running');
    expect(map.get('radarr')?.status).toBe('exited');
  });

  it('applyDockerStateChange creates a new Map (reference inequality preserved)', () => {
    const before = get(dockerStateStore);
    applyDockerStateChange('sonarr', { status: 'running', health: 'healthy', restart_count: 0, image: 'sonarr:latest' });
    const after = get(dockerStateStore);
    expect(after).not.toBe(before);
    expect(after.get('sonarr')?.status).toBe('running');
  });

  it('applyDockerStateChange replaces existing entries', () => {
    applyDockerStateChange('sonarr', { status: 'running', health: 'healthy', restart_count: 0, image: 'sonarr:latest' });
    applyDockerStateChange('sonarr', { status: 'exited', health: 'none', restart_count: 1, image: 'sonarr:latest' });
    expect(get(dockerStateStore).get('sonarr')?.status).toBe('exited');
  });
});
