import { describe, it, expect, vi } from 'vitest';
import type { AppIcon } from './types';

vi.mock('./api', () => ({ getBase: () => '/base' }));

import { resolveIconUrl } from './iconUrl';

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
