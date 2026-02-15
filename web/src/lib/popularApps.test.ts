import { describe, it, expect } from 'vitest';
import {
  popularApps,
  getAllPopularApps,
  getAllGroups,
  templateToApp,
  type PopularAppTemplate,
} from './popularApps';

describe('popularApps', () => {
  describe('popularApps data', () => {
    it('has expected categories', () => {
      expect(Object.keys(popularApps)).toContain('Media');
      expect(Object.keys(popularApps)).toContain('Downloads');
      expect(Object.keys(popularApps)).toContain('System');
      expect(Object.keys(popularApps)).toContain('Utilities');
      expect(Object.keys(popularApps)).toContain('AI');
    });

    it('each category has at least one app', () => {
      for (const [, apps] of Object.entries(popularApps)) {
        expect(apps.length).toBeGreaterThan(0);
      }
    });

    it('each app template has required fields', () => {
      for (const apps of Object.values(popularApps)) {
        for (const app of apps) {
          expect(app.name).toBeTruthy();
          expect(app.defaultUrl).toBeTruthy();
          expect(app.icon).toBeTruthy();
          expect(app.color).toBeTruthy();
          expect(app.group).toBeTruthy();
          expect(app.description).toBeTruthy();
        }
      }
    });
  });

  describe('getAllPopularApps', () => {
    it('returns a flat list of all apps', () => {
      const allApps = getAllPopularApps();
      const totalFromCategories = Object.values(popularApps).reduce(
        (sum, apps) => sum + apps.length,
        0
      );
      expect(allApps).toHaveLength(totalFromCategories);
    });

    it('includes apps from all categories', () => {
      const allApps = getAllPopularApps();
      const groups = new Set(allApps.map((a) => a.group));
      expect(groups.has('Media')).toBe(true);
      expect(groups.has('Downloads')).toBe(true);
      expect(groups.has('System')).toBe(true);
      expect(groups.has('Utilities')).toBe(true);
    });
  });

  describe('getAllGroups', () => {
    it('returns all group names', () => {
      const groups = getAllGroups();
      expect(groups).toContain('Media');
      expect(groups).toContain('Downloads');
      expect(groups).toContain('System');
      expect(groups).toContain('Utilities');
    });

    it('returns same count as popularApps keys', () => {
      const groups = getAllGroups();
      expect(groups).toHaveLength(Object.keys(popularApps).length);
    });
  });

  describe('templateToApp', () => {
    const template: PopularAppTemplate = {
      name: 'Plex',
      defaultUrl: 'http://localhost:32400/web',
      icon: 'plex',
      color: '#E5A00D',
      iconBackground: '#2D2200',
      group: 'Media',
      description: 'Stream your media library',
    };

    it('converts a template to an App with provided URL', () => {
      const app = templateToApp(template, 'http://custom:32400', 0);
      expect(app.name).toBe('Plex');
      expect(app.url).toBe('http://custom:32400');
      expect(app.color).toBe('#E5A00D');
      expect(app.group).toBe('Media');
      expect(app.order).toBe(0);
      expect(app.enabled).toBe(true);
      expect(app.open_mode).toBe('iframe');
      expect(app.proxy).toBe(false);
      expect(app.scale).toBe(1);
      expect(app.disable_keyboard_shortcuts).toBe(false);
    });

    it('uses defaultUrl when url is empty', () => {
      const app = templateToApp(template, '', 0);
      expect(app.url).toBe('http://localhost:32400/web');
    });

    it('sets first app as default', () => {
      const app = templateToApp(template, 'http://test', 0);
      expect(app.default).toBe(true);
    });

    it('sets non-first apps as not default', () => {
      const app = templateToApp(template, 'http://test', 1);
      expect(app.default).toBe(false);
    });

    it('creates correct icon structure', () => {
      const app = templateToApp(template, 'http://test', 0);
      expect(app.icon.type).toBe('dashboard');
      expect(app.icon.name).toBe('plex');
      expect(app.icon.file).toBe('');
      expect(app.icon.url).toBe('');
      expect(app.icon.variant).toBe('svg');
      expect(app.icon.background).toBe('#2D2200');
    });
  });
});
