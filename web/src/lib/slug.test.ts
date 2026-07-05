import { describe, it, expect } from 'vitest';
import { slugify, findSlugConflict } from './slug';
import type { App } from './types';

describe('slugify', () => {
  it('converts spaces to hyphens', () => {
    expect(slugify('hello world')).toBe('hello-world');
  });

  it('removes special characters', () => {
    expect(slugify('hello@world!')).toBe('helloworld');
  });

  it('converts to lowercase', () => {
    expect(slugify('Hello World')).toBe('hello-world');
  });

  it('collapses multiple spaces into single hyphen', () => {
    expect(slugify('hello   world')).toBe('hello-world');
  });

  it('handles empty string', () => {
    expect(slugify('')).toBe('');
  });

  it('handles string with only special chars', () => {
    expect(slugify('!@#$%')).toBe('');
  });

  it('keeps numbers', () => {
    expect(slugify('app 2 test')).toBe('app-2-test');
  });

  it('keeps existing hyphens', () => {
    expect(slugify('my-app')).toBe('my-app');
  });

  // These pin the cases where the old TS implementation diverged from the Go
  // backend (config.Slugify), which broke the /proxy route vs the nav hash.
  it('treats underscores as separators (matches backend)', () => {
    expect(slugify('My_App')).toBe('my-app');
  });

  it('trims leading and trailing dashes (matches backend)', () => {
    expect(slugify('-Radarr-')).toBe('radarr');
    expect(slugify('  spaced  ')).toBe('spaced');
  });

  it('collapses runs of mixed separators to a single dash (matches backend)', () => {
    expect(slugify('a--b')).toBe('a-b');
    expect(slugify('a _ - b')).toBe('a-b');
  });
});

describe('findSlugConflict', () => {
  const app = (name: string, enabled = true) => {
    const a = { name, enabled } as unknown as App;
    (a as { id?: string }).id = name; // stampAppId sets id = name
    return a;
  };

  it('returns null when slugs are distinct', () => {
    expect(findSlugConflict(app('Radarr'), [app('Sonarr'), app('Lidarr')])).toBeNull();
  });

  it('finds a case-only / separator collision and names the other app', () => {
    expect(findSlugConflict(app('radarr'), [app('Radarr')])).toBe('Radarr');
    expect(findSlugConflict(app('My-App'), [app('My App')])).toBe('My App');
  });

  it('excludes the app itself by id (renaming does not self-flag)', () => {
    const self = app('Radarr');
    (self as { name: string }).name = 'radarr'; // renamed, id still "Radarr"
    expect(findSlugConflict(self, [self])).toBeNull();
  });

  it('ignores disabled apps (no route, no collision)', () => {
    expect(findSlugConflict(app('radarr'), [app('Radarr', false)])).toBeNull();
    expect(findSlugConflict(app('radarr', false), [app('Radarr')])).toBeNull();
  });
});
