import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

// The manifest is fetched from ./manifest.json relative to wherever the SPA is
// served, which may be a base_path such as /muximux/. Every URL in it must
// therefore be relative too: an absolute start_url resolves to the site root,
// lands outside the manifest's scope, and makes the app uninstallable under
// a base path (#436).
describe('web manifest', () => {
  const manifest = JSON.parse(readFileSync(resolve(__dirname, '../../public/manifest.json'), 'utf8'));

  it('uses relative start_url, scope and id', () => {
    expect(manifest.start_url).toBe('./');
    expect(manifest.scope).toBe('./');
    expect(manifest.id).toBe('./');
  });

  it('declares installable icons with relative paths', () => {
    expect(manifest.display).toBe('standalone');
    const sizes = manifest.icons.map((i: { sizes: string }) => i.sizes);
    expect(sizes).toEqual(expect.arrayContaining(['192x192', '512x512']));
    for (const icon of manifest.icons) {
      expect(icon.src.startsWith('./')).toBe(true);
    }
  });
});
