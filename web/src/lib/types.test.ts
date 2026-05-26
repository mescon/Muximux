import { describe, it, expect } from 'vitest';
import { getEffectiveUrl } from './types';
import type { App, DockerState, UserInfo, DiscoveryDockerConfig, DiscoveryDockerStatus } from './types';

function makeApp(overrides: Partial<App> = {}): App {
  return {
    name: 'Test',
    url: 'http://localhost:8080',
    icon: { type: 'lucide', name: 'home', file: '', url: '', variant: '' },
    group: 'Default',
    proxy: false,
    open_mode: 'iframe',
    enabled: true,
    order: 0,
    health_check: false,
    color: '',
    ...overrides,
  } as App;
}

describe('getEffectiveUrl', () => {
  it('returns app.url when no proxyUrl', () => {
    const app = makeApp({ url: 'http://sonarr:8989' });
    expect(getEffectiveUrl(app)).toBe('http://sonarr:8989');
  });

  it('returns proxyUrl with base prefix when proxyUrl is set', () => {
    const app = makeApp({ url: 'http://sonarr:8989', proxyUrl: '/proxy/sonarr/' });
    expect(getEffectiveUrl(app)).toBe('/proxy/sonarr/');
  });

  it('prepends __MUXIMUX_BASE__ when set', () => {
    (globalThis as Record<string, unknown>).__MUXIMUX_BASE__ = '/muximux';
    try {
      const app = makeApp({ proxyUrl: '/proxy/sonarr/' });
      expect(getEffectiveUrl(app)).toBe('/muximux/proxy/sonarr/');
    } finally {
      delete (globalThis as Record<string, unknown>).__MUXIMUX_BASE__;
    }
  });
});

describe('Docker lifecycle type extensions', () => {
  it('DockerState union accepts all spec statuses', () => {
    const states: DockerState['status'][] = [
      'running', 'exited', 'paused', 'restarting', 'created', 'dead', 'missing',
    ];
    expect(states.length).toBe(7);
  });

  it('DockerState union accepts all spec health values', () => {
    const healths: DockerState['health'][] = ['healthy', 'unhealthy', 'starting', 'none'];
    expect(healths.length).toBe(4);
  });

  it('UserInfo carries can_use_docker_lifecycle', () => {
    const u: UserInfo = {
      username: 'erik',
      role: 'admin',
      can_use_docker_lifecycle: true,
    };
    expect(u.can_use_docker_lifecycle).toBe(true);
  });

  it('DiscoveryDockerConfig carries lifecycle and badge fields', () => {
    const c: DiscoveryDockerConfig = {
      enabled: true,
      endpoint: 'unix:///var/run/docker.sock',
      tls: { enabled: false },
      network_strategy: 'container_ip',
      refresh_interval: '60s',
      lifecycle_enabled: true,
      lifecycle_min_role: 'admin',
      lifecycle_allowed_groups: [],
      health_badge_placement: 'overview',
    };
    expect(c.lifecycle_enabled).toBe(true);
  });

  it('DiscoveryDockerStatus carries socket_writable and lifecycle_enabled', () => {
    const s: DiscoveryDockerStatus = {
      configured: true,
      reachable: true,
      strategy_ok: true,
      socket_writable: false,
      lifecycle_enabled: false,
    };
    expect(s.socket_writable).toBe(false);
  });
});
