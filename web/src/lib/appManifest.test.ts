import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  DASHBOARD_MANIFEST_HREF,
  manifestHrefForApp,
  manifestHrefForRoute,
  swapManifestLink,
  syncManifestToRoute,
} from './appManifest';
import { makeApp } from './types';

const plex = makeApp({ name: 'Plex', url: 'https://plex.local' });
const sonarr = makeApp({ name: 'Sonarr!', url: 'https://sonarr.local' });
const newTab = makeApp({ name: 'Docs', url: 'https://docs.local', open_mode: 'new_tab' });

describe('per-app PWA manifest', () => {
  beforeEach(() => {
    document.head.innerHTML = '';
    vi.unstubAllGlobals();
  });

  afterEach(() => {
    document.head.innerHTML = '';
    vi.unstubAllGlobals();
  });

  describe('manifestHrefForApp', () => {
    it('builds the per-app manifest URL from the slug', () => {
      expect(manifestHrefForApp(plex)).toBe('./manifest/plex.webmanifest');
    });

    it('slugifies names the same way routes do', () => {
      expect(manifestHrefForApp(sonarr)).toBe('./manifest/sonarr.webmanifest');
    });

    it('returns null for apps that open outside the dashboard', () => {
      expect(manifestHrefForApp(newTab)).toBeNull();
    });

    it('returns null without an app or without a slug', () => {
      expect(manifestHrefForApp(null)).toBeNull();
      expect(manifestHrefForApp(undefined)).toBeNull();
      expect(manifestHrefForApp(makeApp({ name: '!!!' }))).toBeNull();
    });
  });

  describe('manifestHrefForRoute', () => {
    const apps = [plex, sonarr];

    it('maps a single-app route to its manifest', () => {
      expect(manifestHrefForRoute('plex', apps)).toBe('./manifest/plex.webmanifest');
    });

    it('falls back to the dashboard manifest for home, splits, and reserved hashes', () => {
      for (const route of ['', 'plex+sonarr', 'settings', 'logs', 'overview', 'unknown']) {
        expect(manifestHrefForRoute(route, apps)).toBe(DASHBOARD_MANIFEST_HREF);
      }
    });

    it('ignores apps that open outside the dashboard', () => {
      expect(manifestHrefForRoute('docs', [newTab])).toBe(DASHBOARD_MANIFEST_HREF);
    });
  });

  describe('swapManifestLink', () => {
    it('creates a credentialed manifest link when none exists', () => {
      expect(swapManifestLink('./manifest/plex.webmanifest')).toBe(true);
      const link = document.querySelector<HTMLLinkElement>('link[rel="manifest"]');
      expect(link?.getAttribute('href')).toBe('./manifest/plex.webmanifest');
      expect(link?.crossOrigin).toBe('use-credentials');
    });

    it('retargets the existing link and keeps credentials', () => {
      swapManifestLink(DASHBOARD_MANIFEST_HREF);
      expect(swapManifestLink('./manifest/plex.webmanifest')).toBe(true);
      const link = document.querySelector<HTMLLinkElement>('link[rel="manifest"]');
      expect(link?.getAttribute('href')).toBe('./manifest/plex.webmanifest');
      expect(link?.crossOrigin).toBe('use-credentials');
    });

    it('is a no-op when the href already matches', () => {
      swapManifestLink(DASHBOARD_MANIFEST_HREF);
      expect(swapManifestLink(DASHBOARD_MANIFEST_HREF)).toBe(false);
      expect(document.querySelectorAll('link[rel="manifest"]')).toHaveLength(1);
    });
  });

  describe('syncManifestToRoute', () => {
    const apps = [plex, sonarr];

    it('points at the per-app manifest on a single-app route', () => {
      swapManifestLink(DASHBOARD_MANIFEST_HREF);
      expect(syncManifestToRoute('plex', apps)).toBe(true);
      expect(document.querySelector<HTMLLinkElement>('link[rel="manifest"]')?.getAttribute('href'))
        .toBe('./manifest/plex.webmanifest');
    });

    it('restores the dashboard manifest when leaving the app', () => {
      swapManifestLink('./manifest/plex.webmanifest');
      expect(syncManifestToRoute('', apps)).toBe(true);
      expect(document.querySelector<HTMLLinkElement>('link[rel="manifest"]')?.getAttribute('href'))
        .toBe(DASHBOARD_MANIFEST_HREF);
    });

    it('keeps the dashboard manifest on split routes', () => {
      swapManifestLink(DASHBOARD_MANIFEST_HREF);
      expect(syncManifestToRoute('plex+sonarr', apps)).toBe(false);
      expect(document.querySelector<HTMLLinkElement>('link[rel="manifest"]')?.getAttribute('href'))
        .toBe(DASHBOARD_MANIFEST_HREF);
    });
  });
});
