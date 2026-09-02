import type { AppIcon } from './types';
import { getBase } from './api';

/**
 * Resolve an icon config to a fetchable image URL, or null when the config
 * is incomplete. Single source of truth shared by AppIcon (inline rendering)
 * and the dynamic tab favicon (#407), so the two can never disagree about
 * where an icon lives.
 */
export function resolveIconUrl(icon: AppIcon | undefined | null): string | null {
  if (!icon) return null;

  const base = getBase();
  switch (icon.type) {
    case 'dashboard': {
      if (!icon.name) return null;
      const variant = icon.variant || 'svg';
      return `${base}/icons/dashboard/${icon.name}.${variant}`;
    }
    case 'custom':
      if (!icon.file) return null;
      return `${base}/icons/custom/${icon.file}`;
    case 'url':
      return icon.url || null;
    case 'lucide':
      if (!icon.name) return null;
      return `${base}/icons/lucide/${icon.name}.svg`;
    default:
      return null;
  }
}

/**
 * True when the config names an icon that resolveIconUrl can turn into a URL.
 * Callers must not test `icon.name` for this: dashboard and lucide icons use
 * `name`, but custom uploads use `file` and remote icons use `url`, so a
 * `name` check silently rejects both of those types (#437).
 */
export function hasIcon(icon: AppIcon | undefined | null): icon is AppIcon {
  return resolveIconUrl(icon) !== null;
}
