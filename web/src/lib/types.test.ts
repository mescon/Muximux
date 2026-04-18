import { describe, it, expect } from 'vitest';
import { getEffectiveUrl } from './types';
import type { App } from './types';

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
