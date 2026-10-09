import { describe, it, expect } from 'vitest';
import {
  deepEqual,
  mergeField,
  mergeObject,
  normaliseApp,
  normaliseGroup,
  stampOriginalNames,
  mergeApps,
  mergeGroups,
  cascadeGroupRenames,
  rebaseConfig,
  SERVER_OWNED_APP_FIELDS,
  CLIENT_ONLY_KEYS,
} from './configMerge';
import { type App, type Config, type Group, makeApp, makeGroup, stampAppId, stampGroupId } from './types';

// rawApp is the wire shape the server sends (Go testApp): no icon
// file/url/variant, no force_icon_background, no min_role.
function rawApp(name: string, url: string, extra: Partial<App> = {}): App {
  return { name, url, enabled: true, icon: { type: 'dashboard', name: 'x' }, ...extra } as App;
}

function app(name: string, url: string, extra: Partial<App> = {}): App {
  return normaliseApp(rawApp(name, url, extra));
}

function group(name: string, extra: Partial<Group> = {}): Group {
  return normaliseGroup({ name, ...extra } as Group);
}

function cfg(extra: Partial<Config> = {}): Config {
  return {
    title: 'T',
    navigation: { position: 'top', width: '200px' } as Config['navigation'],
    groups: [],
    apps: [],
    ...extra,
  };
}

const names = (items: { name: string }[]) => items.map(i => i.name).join(',');

describe('configMerge primitives', () => {
  it('deepEqual treats absent, null, empty arrays and empty objects alike and ignores key order', () => {
    expect(deepEqual(undefined, [])).toBe(true);
    expect(deepEqual(null, {})).toBe(true);
    expect(deepEqual({ a: 1, b: [] }, { a: 1 })).toBe(true);
    expect(deepEqual({ a: 1, b: 2 }, { b: 2, a: 1 })).toBe(true);
    expect(deepEqual(['a'], ['b'])).toBe(false);
    expect(deepEqual([undefined, 1], [null, 1])).toBe(true);
    expect(deepEqual('', undefined)).toBe(false);
  });

  it('mergeField takes theirs only when mine is unchanged', () => {
    expect(mergeField('a', 'a', 'c')).toBe('c');
    expect(mergeField('a', 'b', 'c')).toBe('b');
  });

  it('mergeObject: mine wins only where changed, skip keeps theirs, cleared keys are removed', () => {
    const base = { position: 'top', width: '220px', show_labels: true, extra: 'x', locked: 'b' };
    const mine = { position: 'left', width: '220px', show_labels: true, locked: 'm' } as typeof base;
    const theirs = { position: 'top', width: '300px', show_labels: false, extra: 'x', locked: 't' };
    const got = mergeObject(base, mine, theirs, new Set(['locked'] as const));
    expect(got).toEqual({ position: 'left', width: '300px', show_labels: false, locked: 't' });
  });

  it('exposes the server-owned and client-only key sets', () => {
    expect([...SERVER_OWNED_APP_FIELDS].sort()).toEqual([
      'docker_endpoint', 'docker_key', 'docker_managed_url', 'docker_strategy', 'gateway_domain', 'original_name', 'proxyUrl',
    ]);
    expect(CLIENT_ONLY_KEYS.has('id')).toBe(true);
  });

  it('stampOriginalNames copies each name', () => {
    const apps = [app('A', 'u')];
    const groups = [group('G')];
    stampOriginalNames(apps, groups);
    expect(apps[0].original_name).toBe('A');
    expect(groups[0].original_name).toBe('G');
  });

  it('normalise drops the id stamp and fills makeApp defaults', () => {
    const a = makeApp({ name: 'A' });
    stampAppId(a);
    const n = normaliseApp(a) as App & { id?: string };
    expect(n.id).toBeUndefined();
    expect(n.icon.variant).toBe('');
    const g = makeGroup({ name: 'G' });
    stampGroupId(g);
    expect((normaliseGroup(g) as Group & { id?: string }).id).toBeUndefined();
  });
});

describe('mergeApps (mirrors config_merge_test.go)', () => {
  it('untouched url keeps theirs', () => {
    const base = [app('Whoami', 'http://172.17.0.2')];
    const mine = [app('Whoami', 'http://172.17.0.2', { color: '#fff' })];
    const theirs = [app('Whoami', 'http://172.17.0.9', { docker_key: 'k1' })];
    const got = mergeApps(base, mine, theirs);
    expect(got).toHaveLength(1);
    expect(got[0]).toMatchObject({ url: 'http://172.17.0.9', color: '#fff', docker_key: 'k1', original_name: 'Whoami' });
  });

  it('edited url wins', () => {
    const got = mergeApps([app('W', 'http://a')], [app('W', 'http://manual')], [app('W', 'http://b')]);
    expect(got[0].url).toBe('http://manual');
  });

  it('server-added kept, user-deleted dropped', () => {
    const base = [app('Keep', 'u'), app('Gone', 'u')];
    const mine = [app('Keep', 'u')];
    const theirs = [app('Keep', 'u'), app('Gone', 'u'), app('New', 'u')];
    const got = mergeApps(base, mine, theirs);
    expect(names(got)).toBe('Keep,New');
    expect(got[1].original_name).toBe('New');
  });

  it('server-removed dropped unless edited (tracking stripped)', () => {
    const base = [app('Old', 'u'), app('Edited', 'u')];
    const mine = [app('Old', 'u'), app('Edited', 'u2', { docker_key: 'stale' })];
    const got = mergeApps(base, mine, []);
    expect(got).toHaveLength(1);
    expect(got[0]).toMatchObject({ name: 'Edited', url: 'u2' });
    expect(got[0].docker_key).toBeUndefined();
    expect(got[0].original_name).toBeUndefined();
  });

  it('empty vs missing arrays are unchanged', () => {
    const base = [app('Gone', 'u')];
    const mine = [app('Gone', 'u', { allowed_groups: [], permissions: [], proxy_headers: {} })];
    expect(mergeApps(base, mine, [])).toHaveLength(0);
    const theirs = [app('Gone', 'u', { allowed_groups: ['ops'] })];
    const got = mergeApps(base, mine, theirs);
    expect(got).toHaveLength(1);
    expect(got[0].allowed_groups).toEqual(['ops']);
  });

  it('rename with server url refresh', () => {
    const base = [app('Old', 'http://a')];
    const mine = [app('New', 'http://a', { original_name: 'Old' })];
    const theirs = [app('Old', 'http://b', { docker_key: 'k' })];
    const got = mergeApps(base, mine, theirs);
    expect(got[0]).toMatchObject({ name: 'New', url: 'http://b', original_name: 'Old', docker_key: 'k' });
  });

  it('server rename matched by docker_key', () => {
    const base = [app('whoami', 'u', { docker_key: 'k' })];
    const mine = [app('whoami', 'u', { pinned: true })];
    const theirs = [app('Whoami Pretty', 'u', { docker_key: 'k' })];
    const got = mergeApps(base, mine, theirs);
    expect(got).toHaveLength(1);
    expect(got[0]).toMatchObject({ name: 'Whoami Pretty', pinned: true, original_name: 'Whoami Pretty' });
  });

  it('delete beats server update', () => {
    expect(mergeApps([app('X', 'http://a')], [], [app('X', 'http://b')])).toHaveLength(0);
  });

  it('user-deleted app is not resurrected under its server rename', () => {
    const base = [app('whoami', 'u', { docker_key: 'k' })];
    const theirs = [app('Whoami Pretty', 'u', { docker_key: 'k' })];
    expect(mergeApps(base, [], theirs)).toHaveLength(0);
  });

  it('server-removed untouched tracked app is removed', () => {
    const base = [app('tracked', 'u', { docker_key: 'gone' })];
    const mine = [app('tracked', 'u', { docker_key: 'gone' })];
    expect(mergeApps(base, mine, [])).toHaveLength(0);
  });

  it('new app strips tracking', () => {
    const got = mergeApps([], [app('Brand', 'u', { docker_key: 'forged', proxyUrl: '/p' })], []);
    expect(got[0].docker_key).toBeUndefined();
    expect(got[0].proxyUrl).toBeUndefined();
  });

  it('rename claims before name match', () => {
    const renamed = app('Y', 'http://mine', { original_name: 'X' });
    const fresh = app('X', 'http://fresh', { docker_key: 'forged' });
    for (const mine of [[renamed, fresh], [fresh, renamed]]) {
      const base = [app('X', 'http://a')];
      const theirs = [app('X', 'http://a', { docker_key: 'k', color: '#srv' })];
      const got = mergeApps(base, mine, theirs);
      expect(got).toHaveLength(2);
      const y = got.find(a => a.name === 'Y')!;
      expect(y).toMatchObject({ original_name: 'X', docker_key: 'k', color: '#srv', url: 'http://mine' });
      const x = got.find(a => a.name === 'X')!;
      expect(x.original_name).toBeUndefined();
      expect(x.docker_key).toBeUndefined();
      expect(x.url).toBe('http://fresh');
    }
  });
});

describe('mergeGroups (mirrors config_merge_test.go)', () => {
  it('group rename keeps theirs name as original', () => {
    const base = [group('Media', { color: '#111' })];
    const mine = [group('Video', { original_name: 'Media', color: '#111' })];
    const theirs = [group('Media', { color: '#222' })];
    const got = mergeGroups(base, mine, theirs);
    expect(got).toHaveLength(1);
    expect(got[0]).toMatchObject({ name: 'Video', original_name: 'Media', color: '#222' });
  });

  it('covers every branch', () => {
    const base = [group('Keep', { color: '#1' }), group('UserDeleted'), group('ServerRemoved'), group('ServerRemovedEdited', { color: '#1' })];
    const mine = [
      group('Keep', { color: '#1', order: 5 }),
      group('ServerRemoved'),
      group('ServerRemovedEdited', { original_name: 'ServerRemovedEdited', color: '#2' }),
      group('Brand', { original_name: 'bogus-but-unknown' }),
    ];
    const theirs = [group('Keep', { color: '#9' }), group('UserDeleted'), group('ServerAdded')];
    const got = mergeGroups(base, mine, theirs);
    expect(names(got)).toBe('Keep,ServerRemovedEdited,Brand,ServerAdded');
    expect(got[0]).toMatchObject({ color: '#9', order: 5, original_name: 'Keep' });
    expect(got[1].color).toBe('#2');
    expect(got[1].original_name).toBeUndefined();
    expect(got[2].original_name).toBeUndefined();
    expect(got[3].original_name).toBe('ServerAdded');
  });

  it('rename claims before name match', () => {
    const renamed = group('Y', { original_name: 'X', color: '#1' });
    const fresh = group('X', { color: '#new' });
    for (const mine of [[renamed, fresh], [fresh, renamed]]) {
      const got = mergeGroups([group('X', { color: '#1' })], mine, [group('X', { color: '#1', expanded: false })]);
      expect(got).toHaveLength(2);
      expect(got.find(g => g.name === 'Y')).toMatchObject({ original_name: 'X', expanded: false });
      const x = got.find(g => g.name === 'X')!;
      expect(x.original_name).toBeUndefined();
      expect(x.color).toBe('#new');
      expect(x.expanded).toBe(true);
    }
  });
});

describe('cascadeGroupRenames', () => {
  it('group rename cascades', () => {
    const apps = [app('A', 'u', { group: 'Media' }), app('B', 'u', { group: 'Other' })];
    cascadeGroupRenames([group('Video', { original_name: 'Media' })], apps);
    expect(apps.map(a => a.group)).toEqual(['Video', 'Other']);
  });

  it('ignores groups that were not renamed', () => {
    const apps = [app('A', 'u', { group: 'Same' })];
    cascadeGroupRenames([group('Same', { original_name: 'Same' }), group('Plain')], apps);
    expect(apps[0].group).toBe('Same');
  });

  it('a new group reusing the old name keeps its apps', () => {
    const groups = [group('Y', { original_name: 'X' }), group('X')];
    const apps = [app('Placed', 'u', { group: 'X' }), app('Stored', 'u', { group: 'X' }), app('Moved', 'u', { group: 'Y' })];
    cascadeGroupRenames(groups, apps, (i, g) => i === 1 && g === 'X');
    expect(apps.map(a => a.group)).toEqual(['X', 'Y', 'Y']);
  });
});

describe('rebaseConfig', () => {
  it('conflict: mine wins', () => {
    const got = rebaseConfig({
      base: cfg({ title: 'A', language: 'en' }),
      local: cfg({ title: 'B', language: 'en' }),
      localApps: [],
      theirs: cfg({ title: 'C', language: 'sv' }),
    });
    expect(got.config.title).toBe('B');
    expect(got.config.language).toBe('sv');
    expect(got.conflicts).toEqual([]);
  });

  it('keybindings per action', () => {
    const combo = (key: string) => [{ key }];
    const got = rebaseConfig({
      base: cfg({ keybindings: { bindings: { search: combo('a'), logs: combo('l') } } }),
      local: cfg({ keybindings: { bindings: { search: combo('b') } } }),
      localApps: [],
      theirs: cfg({ keybindings: { bindings: { search: combo('a'), logs: combo('l'), home: combo('h') } } }),
    });
    expect(got.config.keybindings?.bindings).toEqual({ search: combo('b'), home: combo('h') });
  });

  it('keybindings: absent mine keeps theirs, absent base and theirs keep mine', () => {
    const theirsKb = { bindings: { x: [{ key: 'x' }] } };
    expect(rebaseConfig({ base: cfg(), local: cfg(), localApps: [], theirs: cfg({ keybindings: theirsKb }) }).config.keybindings).toEqual(theirsKb);
    const mineKb = { bindings: { y: [{ key: 'y' }] } };
    expect(rebaseConfig({ base: cfg(), local: cfg({ keybindings: mineKb }), localApps: [], theirs: cfg() }).config.keybindings).toEqual(mineKb);
    expect(rebaseConfig({ base: cfg(), local: cfg({ keybindings: {} }), localApps: [], theirs: cfg() }).config.keybindings).toEqual({ bindings: {} });
  });

  it('merges nested sections, health and lists', () => {
    const base = cfg({
      navigation: { position: 'top', width: '200px' } as Config['navigation'],
      theme: { family: 'default', variant: 'dark' },
      health: { enabled: true, interval: '30s', timeout: '5s' },
      groups: [group('G')],
      apps: [rawApp('X', 'u')],
    });
    const local = cfg({
      navigation: { position: 'left', width: '200px' } as Config['navigation'],
      theme: { family: 'default', variant: 'dark' },
      health: { enabled: false, interval: '30s', timeout: '5s' },
      groups: [group('G2', { original_name: 'G' })],
    });
    const theirs = cfg({
      navigation: { position: 'top', width: '300px' } as Config['navigation'],
      theme: { family: 'nord', variant: 'dark' },
      health: { enabled: true, interval: '2m', timeout: '5s' },
      groups: [group('G')],
      apps: [rawApp('X', 'u2', { group: 'G' }), rawApp('Y', 'u')],
      env_overrides: { log_level: 'MUXIMUX_LOG_LEVEL' },
    });
    const got = rebaseConfig({ base, local, localApps: [makeApp(rawApp('X', 'u'))], theirs });
    expect(got.config.navigation).toMatchObject({ position: 'left', width: '300px' });
    expect(got.config.theme?.family).toBe('nord');
    expect(got.config.health).toEqual({ enabled: false, interval: '2m', timeout: '5s' });
    expect(got.config.env_overrides).toEqual({ log_level: 'MUXIMUX_LOG_LEVEL' });
    expect(got.config.groups).toHaveLength(1);
    expect(got.config.groups[0]).toMatchObject({ name: 'G2', original_name: 'G' });
    expect(names(got.apps)).toBe('X,Y');
    expect(got.apps[0]).toMatchObject({ url: 'u2', group: 'G2' });
    expect(got.config.apps).toBe(got.apps);
  });

  it('optional sections: absent mine keeps theirs, absent base or theirs keeps mine, absent everywhere stays absent', () => {
    const health = { enabled: true, interval: '1m', timeout: '5s' };
    const theme = { family: 'nord', variant: 'dark' as const };
    let got = rebaseConfig({ base: cfg(), local: cfg(), localApps: [], theirs: cfg({ health, theme }) });
    expect(got.config.health).toEqual(health);
    expect(got.config.theme).toEqual(theme);
    got = rebaseConfig({ base: cfg(), local: cfg({ health }), localApps: [], theirs: cfg() });
    expect(got.config.health).toEqual(health);
    got = rebaseConfig({ base: cfg({ theme }), local: cfg({ theme }), localApps: [], theirs: cfg() });
    expect(got.config.theme).toEqual(theme);
    got = rebaseConfig({ base: cfg({ theme }), local: cfg(), localApps: [], theirs: cfg({ theme }) });
    expect(got.config.health).toBeUndefined();
    expect('health' in got.config).toBe(false);
  });

  it('tolerates configs without group or app lists', () => {
    const bare = { title: 'T', navigation: {} } as Config;
    const got = rebaseConfig({ base: bare, local: bare, localApps: [], theirs: bare });
    expect(got.apps).toEqual([]);
    expect(got.config.groups).toEqual([]);
  });

  it('unedited normalised app equals raw base', () => {
    const raw = rawApp('A', 'u', { color: '#fff', group: '', order: 0, default: false, open_mode: 'iframe', proxy: false, scale: 1 });
    const local = makeApp(JSON.parse(JSON.stringify(raw)));
    stampAppId(local);
    // (a) the server removed A: untouched, so it is dropped.
    let got = rebaseConfig({ base: cfg({ apps: [raw] }), local: cfg(), localApps: [local], theirs: cfg() });
    expect(got.apps).toHaveLength(0);
    // (b) the server changed A's url and icon: both come from theirs.
    const theirsA = rawApp('A', 'u2', { icon: { type: 'dashboard', name: 'x', variant: 'svg' } });
    got = rebaseConfig({ base: cfg({ apps: [raw] }), local: cfg(), localApps: [local], theirs: cfg({ apps: [theirsA] }) });
    expect(got.apps).toHaveLength(1);
    expect(got.apps[0].url).toBe('u2');
    expect(got.apps[0].icon.variant).toBe('svg');
    expect((got.apps[0] as App & { id?: string }).id).toBeUndefined();
    expect(got.apps[0].original_name).toBe('A');
  });

  it('ignores the id stamp on groups', () => {
    const raw = { name: 'G', icon: { type: 'dashboard', name: 'g' }, color: '#1', order: 0, expanded: true } as Group;
    const local = makeGroup(JSON.parse(JSON.stringify(raw)));
    stampGroupId(local);
    // Removed on the server and untouched locally: dropped, not kept as edited.
    let got = rebaseConfig({ base: cfg({ groups: [raw] }), local: cfg({ groups: [local] }), localApps: [], theirs: cfg() });
    expect(got.config.groups).toHaveLength(0);
    // Changed on the server: theirs wins.
    got = rebaseConfig({ base: cfg({ groups: [raw] }), local: cfg({ groups: [local] }), localApps: [], theirs: cfg({ groups: [{ ...raw, color: '#2' }] }) });
    expect(got.config.groups[0].color).toBe('#2');
    expect((got.config.groups[0] as Group & { id?: string }).id).toBeUndefined();
  });

  it('apps in a group the user deleted become ungrouped, including server-added ones', () => {
    const base = cfg({ groups: [group('G'), group('Keep')], apps: [rawApp('Mine', 'u', { group: 'G' })] });
    const local = cfg({ groups: [group('Keep')] });
    const theirs = cfg({
      groups: [group('G'), group('Keep')],
      apps: [rawApp('Mine', 'u', { group: 'G' }), rawApp('Added', 'u', { group: 'G' }), rawApp('Kept', 'u', { group: 'Keep' })],
    });
    const got = rebaseConfig({ base, local, localApps: [makeApp(rawApp('Mine', 'u', { group: '' }))], theirs });
    expect(names(got.config.groups)).toBe('Keep');
    expect(Object.fromEntries(got.apps.map(a => [a.name, a.group]))).toEqual({ Mine: '', Added: '', Kept: 'Keep' });
  });

  it('group rename with a new group under the old name only moves apps that were in the old group', () => {
    const base = cfg({ groups: [group('X')], apps: [rawApp('Stored', 'u', { group: 'X' })] });
    const local = cfg({ groups: [group('Y', { original_name: 'X' }), group('X')] });
    const theirs = cfg({ groups: [group('X')], apps: [rawApp('Stored', 'u', { group: 'X' }), rawApp('SrvAdded', 'u', { group: 'X' })] });
    const localApps = [makeApp(rawApp('Stored', 'u', { group: 'X' })), makeApp(rawApp('Placed', 'u', { group: 'X' }))];
    const got = rebaseConfig({ base, local, localApps, theirs });
    expect(Object.fromEntries(got.apps.map(a => [a.name, a.group]))).toEqual({ Stored: 'Y', Placed: 'X', SrvAdded: 'Y' });
  });

  it('rename onto a server-added app name keeps the user app and reports a conflict', () => {
    const base = cfg({ apps: [rawApp('A', 'u')] });
    const theirs = cfg({ apps: [rawApp('A', 'u'), rawApp('B', 'srv')] });
    const got = rebaseConfig({ base, local: cfg(), localApps: [makeApp(rawApp('B', 'mine', { original_name: 'A' }))], theirs });
    expect(got.apps).toHaveLength(1);
    expect(got.apps[0]).toMatchObject({ name: 'B', url: 'mine', original_name: 'A' });
    expect(got.conflicts).toEqual([{
      kind: 'app', name: 'B', renamedFrom: 'A',
      message: 'an app named "B" was added on the server while you renamed "A" to "B"; your app is kept and saving will replace the server\'s',
    }]);
  });

  it('both sides adding one name keeps the user item and reports a conflict', () => {
    const got = rebaseConfig({
      base: cfg(),
      local: cfg({ groups: [group('G', { color: '#mine' })] }),
      localApps: [makeApp(rawApp('Same', 'http://mine'))],
      theirs: cfg({ groups: [group('G', { color: '#srv' })], apps: [rawApp('Same', 'http://theirs', { docker_key: 'k' })] }),
    });
    expect(got.apps).toHaveLength(1);
    expect(got.apps[0].url).toBe('http://mine');
    expect(got.apps[0].docker_key).toBeUndefined();
    expect(got.config.groups).toHaveLength(1);
    expect(got.config.groups[0].color).toBe('#mine');
    expect(got.conflicts.map(c => [c.kind, c.name, c.renamedFrom])).toEqual([['group', 'G', undefined], ['app', 'Same', undefined]]);
    expect(got.conflicts[0].message).toBe('a group named "G" was added on the server while you also added one; your group is kept and saving will replace the server\'s');
  });

  it('two user items of one name are both kept and reported', () => {
    const got = rebaseConfig({
      base: cfg(),
      local: cfg(),
      localApps: [makeApp(rawApp('Dup', 'u')), makeApp(rawApp('Dup', 'v'))],
      theirs: cfg(),
    });
    expect(got.apps).toHaveLength(2);
    expect(got.conflicts).toEqual([{ kind: 'app', name: 'Dup', message: 'two apps are named "Dup"; rename one before saving' }]);
  });
});
