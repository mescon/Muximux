import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { normaliseBase, rebaseConfig } from './configMerge';
import type { App, Config } from './types';

// Shared with internal/handlers/config_merge_parity_test.go: the Go merge
// and this rebase must agree on it.
interface Fixture {
  base_server: Config;
  base_sent: Config;
  mine: Config;
  theirs: Config;
  expected: { title: string; apps: string[]; groups: string[] };
}

const fixture = JSON.parse(
  readFileSync(resolve(__dirname, '../../../internal/handlers/testdata/merge_parity_sparse.json'), 'utf8'),
) as Fixture;

describe('merge parity with the server (sparse server config)', () => {
  it('normaliseBase turns the server config into the base the Go test merges with', () => {
    expect(normaliseBase(fixture.base_server, fixture.base_server.apps as App[])).toEqual(fixture.base_sent);
  });

  it.each([
    ['the raw server config', 'base_server'],
    ['the normalised base', 'base_sent'],
  ] as const)('rebasing from %s drops the server-removed app and group like the Go merge', (_label, key) => {
    const { config, apps } = rebaseConfig({
      base: fixture[key],
      local: fixture.mine,
      localApps: fixture.mine.apps,
      theirs: fixture.theirs,
    });
    expect(config.title).toBe(fixture.expected.title);
    expect(apps.map(a => a.name)).toEqual(fixture.expected.apps);
    expect(config.groups.map(g => g.name)).toEqual(fixture.expected.groups);
  });
});
