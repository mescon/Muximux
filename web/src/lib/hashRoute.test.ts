import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { applyHashPaths, hashDeepLinkUrl, parseHashRoute } from './hashRoute';
import { makeApp } from './types';
import { splitState } from './splitStore.svelte';
import { deepLinkKey, type DeepLinks } from './linkNavigation';

let warn: ReturnType<typeof vi.spyOn>;
beforeEach(() => { warn = vi.spyOn(console, 'warn').mockImplementation(() => {}); });
afterEach(() => {
  warn.mockRestore();
  splitState.enabled = false;
  splitState.panels = [null, null];
  splitState.activePanel = 0;
});

const entries = (h: string) => [...parseHashRoute(h).paths];

describe('parseHashRoute', () => {
  it('returns a plain route unchanged', () => {
    expect(parseHashRoute('emby')).toEqual({ route: 'emby', paths: new Map() });
    expect(parseHashRoute('emby+radarr')).toEqual({ route: 'emby+radarr', paths: new Map() });
  });

  it('maps path= to the single app', () => {
    const r = parseHashRoute('emby?path=%2Fweb%2Findex.html%23!%2Fitem%3Fid%3D123');
    expect(r.route).toBe('emby');
    expect(entries('emby?path=%2Fweb%2Findex.html%23!%2Fitem%3Fid%3D123')).toEqual([['emby', '/web/index.html#!/item?id=123']]);
  });

  it('accepts the app slug as key for a single app', () => {
    expect(entries('emby?emby=%2Fweb%2F')).toEqual([['emby', '/web/']]);
  });

  it('keys split paths by app slug, including hyphenated slugs', () => {
    expect(entries('emby+uptime-kuma?uptime-kuma=%2Fdashboard%2F1&emby=%2Fweb%2F')).toEqual([
      ['uptime-kuma', '/dashboard/1'], ['emby', '/web/'],
    ]);
  });

  it('keeps a + in a path instead of turning it into a space', () => {
    expect(entries('radarr?path=%2Fmovie%2Fa+b')).toEqual([['radarr', '/movie/a+b']]);
  });

  it('accepts an unencoded path as long as it has no &', () => {
    expect(entries('emby?path=/web/index.html#!/item?id=1')).toEqual([['emby', '/web/index.html#!/item?id=1']]);
  });

  it('ignores and warns about keys that name no app of the route', () => {
    expect(entries('emby+radarr?path=%2Fx&sonarr=%2Fy')).toEqual([]);
    expect(warn).toHaveBeenCalledTimes(2);
    expect(warn.mock.calls[0][1]).toContain('app slugs');
    expect(entries('emby?radarr=%2Fx')).toEqual([]);
    expect(warn.mock.calls[2][1]).toContain('path=');
  });

  it('ignores bad encoding, empty values and empty pairs', () => {
    expect(entries('emby?path=%E0%A4%A')).toEqual([]);
    expect(warn.mock.calls[0][0]).toContain('not valid URL encoding');
    expect(entries('emby?path=')).toEqual([]);
    expect(entries('emby?path')).toEqual([]);
    expect(entries('emby?&&path=%2Fa')).toEqual([['emby', '/a']]);
  });
});

describe('hashDeepLinkUrl', () => {
  const emby = makeApp({ name: 'Emby', url: 'http://emby:8096/', allow_deep_links: true });

  it('resolves an allowed path with the message API rule', () => {
    expect(hashDeepLinkUrl(emby, '/web/index.html', '')).toBe('http://emby:8096/web/index.html');
  });

  it('refuses an app without allow_deep_links', () => {
    expect(hashDeepLinkUrl({ ...emby, allow_deep_links: false }, '/web/', '')).toBeNull();
    expect(warn.mock.calls[0][1]).toContain('Allow deep links');
  });

  it('refuses a disabled app', () => {
    expect(hashDeepLinkUrl({ ...emby, enabled: false }, '/web/', '')).toBeNull();
    expect(warn.mock.calls[0][0]).toContain('disabled');
  });

  it('refuses an app that does not open in an iframe', () => {
    expect(hashDeepLinkUrl({ ...emby, open_mode: 'new_tab' }, '/web/', '')).toBeNull();
    expect(warn.mock.calls[0][0]).toContain('iframe');
  });

  it('refuses a path that leaves the app', () => {
    for (const p of ['http://evil/', '//evil/x', 'javascript:alert(1)']) {
      expect(hashDeepLinkUrl(emby, p, '')).toBeNull();
    }
    expect(warn.mock.calls[0][0]).toContain('not valid inside the app');
  });

  it('maps a proxied app onto its proxy path', () => {
    const proxied = { ...emby, proxyUrl: '/proxy/emby/' };
    expect(hashDeepLinkUrl(proxied, '/web/a', '/base')).toBe('/base/proxy/emby/web/a');
  });
});

describe('applyHashPaths', () => {
  const emby = makeApp({ name: 'Emby', url: 'http://emby/', allow_deep_links: true });
  const radarr = makeApp({ name: 'Radarr', url: 'http://radarr/', allow_deep_links: true });
  const plain = makeApp({ name: 'Plain', url: 'http://plain/' });
  const apps = [emby, radarr, plain];

  it('sets a deep link for the app outside split view', () => {
    splitState.panels = [emby, null];
    const links: DeepLinks = {};
    applyHashPaths(new Map([['emby', '/web/']]), apps, links, '');
    expect(links[deepLinkKey(0, 'Emby')].url).toBe('http://emby/web/');
  });

  it('sets each split path on the pane showing its app', () => {
    splitState.enabled = true;
    splitState.panels = [emby, radarr];
    const links: DeepLinks = {};
    applyHashPaths(new Map([['radarr', '/movie/1'], ['emby', '/web/']]), apps, links, '');
    expect(links[deepLinkKey(1, 'Radarr')].url).toBe('http://radarr/movie/1');
    expect(links[deepLinkKey(0, 'Emby')].url).toBe('http://emby/web/');
  });

  it('skips unknown apps, apps not shown and refused paths', () => {
    splitState.panels = [plain, null];
    const links: DeepLinks = {};
    applyHashPaths(new Map([['nope', '/a'], ['radarr', '/movie/1'], ['plain', '/x']]), apps, links, '');
    expect(links).toEqual({});
    expect(warn).toHaveBeenCalledTimes(1);
  });
});
