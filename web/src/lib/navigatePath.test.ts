import { describe, it, expect } from 'vitest';
import { resolveDeepLink, isFragmentOnlyChange, MAX_NAV_PATH } from './navigatePath';
import { makeApp } from './types';

const url = (u: string, extra: Record<string, unknown> = {}) => makeApp({ url: u, ...extra });

describe('resolveDeepLink', () => {
  it('resolves a path with hash on a root app', () => {
    expect(resolveDeepLink(url('http://emby:8096/'), '/web/index.html#!/item?id=123')).toEqual({
      ok: true,
      url: 'http://emby:8096/web/index.html#!/item?id=123',
    });
  });

  it('uses the directory of a file-ending app url as base', () => {
    expect(
      resolveDeepLink(url('http://emby:8096/web/index.html'), '/web/index.html#!/item?id=123'),
    ).toEqual({ ok: true, url: 'http://emby:8096/web/index.html#!/item?id=123' });
  });

  it('keeps sub-path apps inside their directory', () => {
    const app = url('http://host/sonarr/');
    expect(resolveDeepLink(app, '/series/x')).toEqual({ ok: false });
    expect(resolveDeepLink(app, 'series/x')).toEqual({ ok: true, url: 'http://host/sonarr/series/x' });
    expect(resolveDeepLink(app, '/sonarr/series/x')).toEqual({
      ok: true,
      url: 'http://host/sonarr/series/x',
    });
  });

  it('drops the base url query', () => {
    expect(resolveDeepLink(url('http://homarr/?board=main'), '/board/2')).toEqual({
      ok: true,
      url: 'http://homarr/board/2',
    });
  });

  it('rejects dot-segment escapes', () => {
    expect(resolveDeepLink(url('http://host/sonarr/'), '../radarr/')).toEqual({ ok: false });
  });

  it.each([
    'https://evil.example/',
    '//evil.example/x',
    '/\\evil.example',
    '\\evil',
    'javascript:alert(1)',
    'data:text/html,x',
  ])('rejects %s', (p) => {
    expect(resolveDeepLink(url('http://host/'), p)).toEqual({ ok: false });
  });

  it.each([
    '..%2fradarr/',
    '%2e%2e%2fradarr/',
    '..%2Fradarr/',
    'x%5c..%5cradarr',
    '%2e%2e/',
    '%E0%A4%A',
  ])('rejects encoded traversal %s', (p) => {
    expect(resolveDeepLink(url('http://host/sonarr/'), p)).toEqual({ ok: false });
    expect(
      resolveDeepLink(url('http://host/sonarr/', { proxyUrl: '/proxy/sonarr/' }), p, ''),
    ).toEqual({ ok: false });
  });

  it('accepts legitimate encoded characters', () => {
    expect(resolveDeepLink(url('http://host/sonarr/'), '/sonarr/series/a%20b')).toEqual({
      ok: true,
      url: 'http://host/sonarr/series/a%20b',
    });
  });

  it('rejects control characters', () => {
    expect(resolveDeepLink(url('http://host/'), 'a\u0007b')).toEqual({ ok: false });
    expect(resolveDeepLink(url('http://host/'), 'a\u007fb')).toEqual({ ok: false });
  });

  it('enforces the length limit', () => {
    const app = url('http://host/');
    expect(MAX_NAV_PATH).toBe(2048);
    expect(resolveDeepLink(app, 'x'.repeat(2049))).toEqual({ ok: false });
    expect(resolveDeepLink(app, '/' + 'x'.repeat(2047)).ok).toBe(true);
  });

  it('rejects a result that is not a safe app url', () => {
    const app = url('http://host/', { proxyUrl: '//evil/' });
    expect(resolveDeepLink(app, 'x', '')).toEqual({ ok: false });
  });

  it('rejects an unparseable app url', () => {
    expect(resolveDeepLink(url('http://'), 'x')).toEqual({ ok: false });
  });

  it('resolves single-slash local app urls', () => {
    expect(resolveDeepLink(url('/local/'), 'x')).toEqual({
      ok: true,
      url: `${window.location.origin}/local/x`,
    });
  });

  describe('proxied apps', () => {
    it('maps a root app onto the proxy path', () => {
      const app = url('http://emby:8096/', { proxyUrl: '/proxy/emby/' });
      expect(resolveDeepLink(app, '/web/index.html?x=1#!/item?id=1', '')).toEqual({
        ok: true,
        url: '/proxy/emby/web/index.html?x=1#!/item?id=1',
      });
    });

    it('maps a sub-path app with a base path', () => {
      const app = url('http://host/sonarr/', { proxyUrl: '/proxy/sonarr/' });
      expect(resolveDeepLink(app, 'series/x?y=2#top', '/mux')).toEqual({
        ok: true,
        url: '/mux/proxy/sonarr/series/x?y=2#top',
      });
    });

    it('defaults base to empty', () => {
      const app = url('http://emby:8096/', { proxyUrl: '/proxy/emby/' });
      expect(resolveDeepLink(app, '/a')).toEqual({ ok: true, url: '/proxy/emby/a' });
    });

    it('accepts the target path itself', () => {
      const app = url('http://emby/web/index.html', { proxyUrl: '/proxy/emby/' });
      expect(resolveDeepLink(app, '/web/index.html', '')).toEqual({ ok: true, url: '/proxy/emby/' });
    });

    it('rejects paths outside the proxy target path', () => {
      const app = url('http://emby/web/index.html', { proxyUrl: '/proxy/emby/' });
      expect(resolveDeepLink(app, '/web/other')).toEqual({ ok: false });
    });
  });
});

describe('isFragmentOnlyChange', () => {
  it('is true when only the fragment differs', () => {
    expect(isFragmentOnlyChange('http://emby/web/index.html', 'http://emby/web/index.html#!/item?id=1')).toBe(true);
    expect(isFragmentOnlyChange('http://emby/web/index.html#!/home', 'http://emby/web/index.html#!/item')).toBe(true);
  });

  it('is true for the same URL with a fragment (still a fragment navigation)', () => {
    expect(isFragmentOnlyChange('http://a/x#y', 'http://a/x#y')).toBe(true);
  });

  it('resolves relative proxy paths against the shell', () => {
    expect(isFragmentOnlyChange('/proxy/emby/web/', '/proxy/emby/web/#!/item')).toBe(true);
  });

  it('is false when the target has no fragment', () => {
    expect(isFragmentOnlyChange('http://a/x#y', 'http://a/x')).toBe(false);
    expect(isFragmentOnlyChange('http://a/x', 'http://a/x')).toBe(false);
  });

  it('is false when path or query differ', () => {
    expect(isFragmentOnlyChange('http://a/x', 'http://a/z#y')).toBe(false);
    expect(isFragmentOnlyChange('http://a/x?q=1', 'http://a/x?q=2#y')).toBe(false);
  });

  it('is false for unparseable URLs', () => {
    expect(isFragmentOnlyChange('http://[', 'http://a/#y')).toBe(false);
  });
});
