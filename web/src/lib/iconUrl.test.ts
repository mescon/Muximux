import { describe, it, expect, vi } from 'vitest';
import type { AppIcon } from './types';

vi.mock('./api', () => ({ getBase: () => '/base' }));

import { resolveIconUrl, hasIcon, iconLabel } from './iconUrl';

describe('resolveIconUrl', () => {
  it('returns null for missing icon or missing identifiers', () => {
    expect(resolveIconUrl(undefined)).toBe(null);
    expect(resolveIconUrl({ type: 'dashboard' } as AppIcon)).toBe(null);
    expect(resolveIconUrl({ type: 'custom' } as AppIcon)).toBe(null);
    expect(resolveIconUrl({ type: 'lucide' } as AppIcon)).toBe(null);
    expect(resolveIconUrl({ type: 'url' } as AppIcon)).toBe(null);
  });

  it('builds dashboard icon URLs with variant (default svg)', () => {
    expect(resolveIconUrl({ type: 'dashboard', name: 'sonarr' } as AppIcon)).toBe('/base/icons/dashboard/sonarr.svg');
    expect(resolveIconUrl({ type: 'dashboard', name: 'plex', variant: 'png' } as AppIcon)).toBe('/base/icons/dashboard/plex.png');
  });

  it('builds custom and lucide URLs, passes url icons through', () => {
    expect(resolveIconUrl({ type: 'custom', file: 'my.png' } as AppIcon)).toBe('/base/icons/custom/my.png');
    expect(resolveIconUrl({ type: 'lucide', name: 'home' } as AppIcon)).toBe('/base/icons/lucide/home.svg');
    expect(resolveIconUrl({ type: 'url', url: 'https://x/y.png' } as AppIcon)).toBe('https://x/y.png');
  });
});

describe('hasIcon', () => {
  it('is true for a custom upload identified by file, even with an empty name (#437)', () => {
    expect(hasIcon({ type: 'custom', name: '', file: 'my-logo.png' })).toBe(true);
  });

  it('is true for a remote icon identified by url', () => {
    expect(hasIcon({ type: 'url', name: '', url: 'https://example.com/i.png' })).toBe(true);
  });

  it('is true for dashboard and lucide icons identified by name', () => {
    expect(hasIcon({ type: 'dashboard', name: 'plex' })).toBe(true);
    expect(hasIcon({ type: 'lucide', name: 'house' })).toBe(true);
  });

  it('is false when the identifying field for the type is missing', () => {
    expect(hasIcon({ type: 'custom', name: 'ignored' })).toBe(false);
    expect(hasIcon({ type: 'dashboard', name: '' })).toBe(false);
    expect(hasIcon({ type: 'url' })).toBe(false);
  });

  it('is false for undefined or null', () => {
    expect(hasIcon(undefined)).toBe(false);
    expect(hasIcon(null)).toBe(false);
  });
});

describe('iconLabel', () => {
  it('returns the name for dashboard and lucide icons', () => {
    expect(iconLabel({ type: 'dashboard', name: 'plex' })).toBe('plex');
    expect(iconLabel({ type: 'lucide', name: 'house' })).toBe('house');
  });

  it('returns the filename for a custom upload and the url for a remote icon', () => {
    expect(iconLabel({ type: 'custom', name: '', file: 'my-logo.png' })).toBe('my-logo.png');
    expect(iconLabel({ type: 'url', name: '', url: 'https://example.com/i.png' })).toBe('https://example.com/i.png');
  });

  it('returns null when nothing identifies the icon', () => {
    expect(iconLabel({ type: 'custom', name: '' })).toBe(null);
    expect(iconLabel(undefined)).toBe(null);
    expect(iconLabel(null)).toBe(null);
  });
});
