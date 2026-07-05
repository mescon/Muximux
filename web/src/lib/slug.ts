import type { App } from './types';

// Slug logic lives in its own side-effect-free module (not the fetch-heavy
// api.ts) so components and tests can use it without dragging in -- or
// mocking away -- the whole API client.

// slugify generates a URL slug from an app name. This MUST match the backend
// config.Slugify byte-for-byte, since the same slug keys the /proxy/<slug>/
// route, the nav deep-link hash, and the health entry. Rules: lowercase,
// keep alphanumerics, collapse any run of space/dash/underscore to a single
// dash, and trim edge dashes.
export function slugify(name: string): string {
  const out: string[] = [];
  let lastDash = true; // start true to suppress a leading dash
  for (const ch of name) {
    if (ch >= 'A' && ch <= 'Z') {
      out.push(ch.toLowerCase());
      lastDash = false;
    } else if ((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9')) {
      out.push(ch);
      lastDash = false;
    } else if (ch === ' ' || ch === '-' || ch === '_') {
      if (!lastDash) {
        out.push('-');
        lastDash = true;
      }
    }
    // anything else (punctuation, non-ASCII) is dropped, as in the backend
  }
  let slug = out.join('');
  if (slug.endsWith('-')) slug = slug.slice(0, -1);
  return slug;
}

// findSlugConflict returns the name of an existing enabled app whose slug
// matches `target`'s, or null if there is none. It is the single client-side
// implementation of the backend's config.validateUniqueAppSlugs rule, shared
// by the app form (as-you-type warning) and the settings commit (hard block).
// The target is excluded from the comparison by its stable id (stampAppId
// sets id to the original name), so renaming an app never flags it against
// itself. Only enabled apps get a /proxy route, so disabled apps never clash.
export function findSlugConflict(target: App, apps: App[]): string | null {
  if (!target.enabled) return null;
  const slug = slugify(target.name ?? '');
  if (!slug) return null;
  const targetId = (target as App & { id?: string }).id;
  const clash = apps.find((a) => {
    const aid = (a as App & { id?: string }).id;
    return aid !== targetId && a.enabled && slugify(a.name ?? '') === slug;
  });
  return clash ? clash.name : null;
}
