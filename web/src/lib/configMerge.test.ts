import { describe, it, expect } from 'vitest';
import {
  byCodePoint,
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
  POINTER_FIELDS,
  CLIENT_ONLY_KEYS,
} from './configMerge';
import { type App, type Config, type Group, makeApp, makeGroup, stampAppId, stampGroupId } from './types';

// rawApp is the wire shape the server sends for Go's testApp: every field
// without omitempty is present at its zero value, the omitempty ones (icon
// file/url/variant, force_icon_background, min_role, ...) are absent.
function rawApp(name: string, url: string, extra: Partial<App> = {}): App {
  return {
    name, url, icon: { type: 'dashboard', name: 'x', color: '', background: '' },
    color: '', group: '', order: 0, enabled: true, default: false, open_mode: '', proxy: false, scale: 0,
    ...extra,
  } as unknown as App;
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
  it('deepEqual treats absent, null, zero values, empty arrays and empty objects alike and ignores key order', () => {
    expect(deepEqual(undefined, [])).toBe(true);
    expect(deepEqual(null, {})).toBe(true);
    expect(deepEqual({ a: 1, b: [] }, { a: 1 })).toBe(true);
    expect(deepEqual({ a: 1, b: 2 }, { b: 2, a: 1 })).toBe(true);
    expect(deepEqual(['a'], ['b'])).toBe(false);
    expect(deepEqual([undefined, 1], [null, 1])).toBe(true);
    expect(deepEqual('', undefined)).toBe(true);
    expect(deepEqual(false, undefined)).toBe(true);
    expect(deepEqual(0, undefined)).toBe(true);
    expect(deepEqual({ a: { b: '' } }, {})).toBe(true);
    // Zero values inside a slice are kept, as Go keeps them.
    expect(deepEqual([0], [])).toBe(false);
    expect(deepEqual([''], [false])).toBe(false);
    expect(deepEqual([{ a: '' }], [{}])).toBe(true);
  });

  it('a real change to a zero value still counts as different', () => {
    expect(deepEqual(true, false)).toBe(false);
    expect(deepEqual('x', '')).toBe(false);
    expect(deepEqual(5, 0)).toBe(false);
    expect(deepEqual({ pinned: true }, { pinned: false })).toBe(false);
  });

  // Each pair is (base, mine) for one app field; edited is the verdict the
  // Go merge reaches comparing the typed ClientAppConfig fields named.
  it.each([
    // Pinned bool: absent and false are both the zero value.
    ['pinned absent vs false', {}, { pinned: false }, false],
    ['pinned true vs false', { pinned: true }, { pinned: false }, true],
    // Pointer fields (*bool, *int): Go compares by presence, nil != &false.
    // HealthCheck *bool (nil = enabled).
    ['health_check nil vs false', {}, { health_check: false }, true],
    ['health_check false vs nil', { health_check: false }, { health_check: undefined }, true],
    ['health_check nil vs nil', {}, {}, false],
    ['health_check false vs false', { health_check: false }, {}, false],
    ['health_check true vs false', { health_check: true }, { health_check: false }, true],
    // ProxySkipTLSVerify *bool (nil = skip).
    ['proxy_skip_tls_verify nil vs false', {}, { proxy_skip_tls_verify: false }, true],
    ['proxy_skip_tls_verify false vs nil', { proxy_skip_tls_verify: false }, { proxy_skip_tls_verify: undefined }, true],
    ['proxy_skip_tls_verify nil vs nil', {}, {}, false],
    // HTTPActionShowToast *bool (nil = show).
    ['http_action_show_toast nil vs false', {}, { http_action_show_toast: false }, true],
    ['http_action_show_toast false vs nil', { http_action_show_toast: false }, { http_action_show_toast: undefined }, true],
    ['http_action_show_toast nil vs nil', {}, {}, false],
    // Shortcut *int: nil vs a pointer to 0.
    ['shortcut nil vs 0', {}, { shortcut: 0 }, true],
    ['shortcut 0 vs nil', { shortcut: 0 }, { shortcut: undefined }, true],
    ['shortcut nil vs nil', {}, {}, false],
    // Color string: "" is the zero value.
    ['color absent vs empty', {}, { color: '' }, false],
    ['color set vs cleared', { color: '#fff' }, { color: '' }, true],
    // Order int and Scale float64: 0 is the zero value.
    ['order absent vs 0', {}, { order: 0 }, false],
    ['order 5 vs 0', { order: 5 }, { order: 0 }, true],
    // AllowedGroups []string with omitempty: nil and [] alike.
    ['allowed_groups absent vs []', {}, { allowed_groups: [] }, false],
    ['allowed_groups set vs []', { allowed_groups: ['ops'] }, { allowed_groups: [] }, true],
    // ProxyHeaders map[string]string: nil and {} alike.
    ['proxy_headers absent vs {}', {}, { proxy_headers: {} }, false],
    // Icon AppIconConfig struct: zero fields drop out.
    ['icon variant absent vs empty', {}, { icon: { type: 'dashboard', name: 'x', variant: '' } }, false],
    ['icon invert false vs true', {}, { icon: { type: 'dashboard', name: 'x', invert: true } }, true],
    // Shortcut *int: set vs cleared.
    ['shortcut 3 vs cleared', { shortcut: 3 }, { shortcut: undefined }, true],
  ] as [string, Partial<App>, Partial<App>, boolean][])('edited verdict matches Go: %s', (_label, baseExtra, mineExtra, edited) => {
    const base = rawApp('A', 'u', baseExtra);
    const mine = makeApp(rawApp('A', 'u', { ...baseExtra, ...mineExtra }));
    stampAppId(mine);
    // A server removal drops an untouched app and keeps an edited one.
    const got = rebaseConfig({ base: cfg({ apps: [base] }), local: cfg(), localApps: [mine], theirs: cfg() });
    expect(got.apps.length === 1).toBe(edited);
  });

  it('untouched pinned: false over an absent base is not an edit, health_check: false is', () => {
    const base = rawApp('A', 'u');
    const theirs = rawApp('A', 'u', { pinned: true });
    let got = rebaseConfig({ base: cfg({ apps: [base] }), local: cfg(), localApps: [makeApp(rawApp('A', 'u', { pinned: false }))], theirs: cfg({ apps: [theirs] }) });
    expect(got.apps[0].pinned).toBe(true);
    // Go: HealthCheck *bool, nil (enabled) vs &false (disabled) is an edit.
    got = rebaseConfig({
      base: cfg({ apps: [base] }), local: cfg(),
      localApps: [makeApp(rawApp('A', 'u', { health_check: false }))],
      theirs: cfg({ apps: [rawApp('A', 'u', { color: '#srv' })] }),
    });
    expect(got.apps[0]).toMatchObject({ health_check: false, color: '#srv' });
    // Unchecking back to nil over a stored false is an edit too.
    got = rebaseConfig({
      base: cfg({ apps: [rawApp('A', 'u', { proxy_skip_tls_verify: false })] }), local: cfg(),
      localApps: [makeApp(rawApp('A', 'u'))],
      theirs: cfg({ apps: [rawApp('A', 'u', { proxy_skip_tls_verify: false })] }),
    });
    expect(got.apps[0].proxy_skip_tls_verify).toBeUndefined();
  });

  it('pointer structs compare by presence: home_icon, health, keybindings', () => {
    // Go: NavigationConfig.HomeIcon *AppIconConfig, nil vs &AppIconConfig{}.
    expect(deepEqual({ home_icon: {} }, {})).toBe(false);
    expect(deepEqual({ home_icon: { type: 'dashboard', name: '' } }, { home_icon: { type: 'dashboard' } })).toBe(true);
    // Go: ClientConfigUpdate.Health / Keybindings pointers.
    expect(deepEqual({ health: { enabled: false } }, {})).toBe(false);
    expect(deepEqual({ keybindings: {} }, {})).toBe(false);
    expect(deepEqual({ keybindings: null }, {})).toBe(true);
    expect(POINTER_FIELDS.has('proxy_skip_tls_verify')).toBe(true);
    const nav = { position: 'top', width: '200px' } as Config['navigation'];
    const got = rebaseConfig({
      base: cfg({ navigation: nav }),
      local: cfg({ navigation: { ...nav, home_icon: { type: 'dashboard' } } }),
      localApps: [],
      theirs: cfg({ navigation: { ...nav, width: '300px' } }),
    });
    expect(got.config.navigation).toMatchObject({ width: '300px', home_icon: { type: 'dashboard' } });
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
      'docker_endpoint', 'docker_key', 'docker_managed_health_check', 'docker_managed_url', 'docker_strategy', 'gateway_domain', 'original_name', 'proxyUrl',
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

  it('health is a pointer (absent mine keeps theirs), theme is a value struct merged per field', () => {
    const health = { enabled: true, interval: '1m', timeout: '5s' };
    const theme = { family: 'nord', variant: 'dark' as const };
    // Go mergeHealth: nil mine keeps theirs; nil base or theirs keeps mine.
    let got = rebaseConfig({ base: cfg(), local: cfg(), localApps: [], theirs: cfg({ health, theme }) });
    expect(got.config.health).toEqual(health);
    expect(got.config.theme).toEqual(theme);
    got = rebaseConfig({ base: cfg(), local: cfg({ health }), localApps: [], theirs: cfg() });
    expect(got.config.health).toEqual(health);
    got = rebaseConfig({ base: cfg({ theme }), local: cfg(), localApps: [], theirs: cfg({ theme }) });
    expect('health' in got.config).toBe(false);
    // Go ThemeConfig is a value: an untouched theme takes theirs per field,
    // even when theirs is the zero struct.
    got = rebaseConfig({ base: cfg({ theme }), local: cfg({ theme }), localApps: [], theirs: cfg() });
    expect(got.config.theme).toEqual({});
    got = rebaseConfig({
      base: cfg({ theme }),
      local: cfg({ theme: { family: 'nord', variant: 'light' } }),
      localApps: [],
      theirs: cfg({ theme: { family: 'dracula', variant: 'dark' } }),
    });
    expect(got.config.theme).toEqual({ family: 'dracula', variant: 'light' });
  });

  it('only ClientConfigUpdate scalars merge; other top-level keys take theirs', () => {
    const got = rebaseConfig({
      base: cfg({ log_level: 'info', gateway: 'g1' }),
      local: cfg({ log_level: 'debug', gateway: 'mine', auth: { method: 'none' } }),
      localApps: [],
      theirs: cfg({ log_level: 'info', gateway: 'g2', auth: { method: 'builtin' } }),
    });
    expect(got.config.log_level).toBe('debug');
    expect(got.config.gateway).toBe('g2');
    expect(got.config.auth).toEqual({ method: 'builtin' });
    const cleared = rebaseConfig({ base: cfg({ language: 'sv' }), local: cfg(), localApps: [], theirs: cfg({ language: 'sv' }) });
    expect('language' in cleared.config).toBe(false);
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

  it('rename onto a server-added app name keeps both apps and reports a conflict', () => {
    const base = cfg({ apps: [rawApp('A', 'u')] });
    const theirs = cfg({ apps: [rawApp('A', 'u'), rawApp('B', 'srv')] });
    const got = rebaseConfig({ base, local: cfg(), localApps: [makeApp(rawApp('B', 'mine', { original_name: 'A' }))], theirs });
    expect(got.apps.map(a => [a.name, a.url, a.original_name])).toEqual([['B', 'mine', 'A'], ['B', 'srv', 'B']]);
    expect(got.conflicts).toEqual([{
      kind: 'app', name: 'B', renamedFrom: 'A',
      message: 'both you and the server now have an app named "B"; rename one of them before saving',
    }]);
  });

  it('both sides adding one name keeps both items and reports a conflict', () => {
    const got = rebaseConfig({
      base: cfg(),
      local: cfg({ groups: [group('G', { color: '#mine' })] }),
      localApps: [makeApp(rawApp('Same', 'http://mine'))],
      theirs: cfg({ groups: [group('G', { color: '#srv' })], apps: [rawApp('Same', 'http://theirs', { docker_key: 'k' })] }),
    });
    expect(got.apps.map(a => [a.url, a.docker_key, a.original_name])).toEqual([
      ['http://mine', undefined, undefined],
      ['http://theirs', 'k', 'Same'],
    ]);
    expect(got.config.groups.map(g => g.color)).toEqual(['#mine', '#srv']);
    expect(got.conflicts.map(c => [c.kind, c.name, c.renamedFrom])).toEqual([['group', 'G', undefined], ['app', 'Same', undefined]]);
    expect(got.conflicts[0].message).toBe('both you and the server now have a group named "G"; rename one of them before saving');
  });

  it('a rename whose old name the server added on its own keeps both without a conflict', () => {
    // mine {B, original A}, base without A, theirs with A: the names differ,
    // so nothing collides.
    const got = rebaseConfig({
      base: cfg(),
      local: cfg(),
      localApps: [makeApp(rawApp('B', 'u', { original_name: 'A' }))],
      theirs: cfg({ apps: [rawApp('A', 'u')] }),
    });
    expect(got.apps.map(a => a.name)).toEqual(['B', 'A']);
    expect(got.conflicts).toEqual([]);
  });

  it('two user items of one name are both kept and reported', () => {
    const got = rebaseConfig({
      base: cfg(),
      local: cfg(),
      localApps: [makeApp(rawApp('Dup', 'u')), makeApp(rawApp('Dup', 'v'))],
      theirs: cfg(),
    });
    expect(got.apps).toHaveLength(2);
    expect(got.conflicts).toEqual([{ kind: 'app', name: 'Dup', message: 'two apps are named "Dup"; rename one of them before saving' }]);
  });
});

describe('rebaseConfig with a null health', () => {
  const h = { enabled: true, interval: '1m', timeout: '5s' };
  it('reads a null side as absent instead of throwing', () => {
    const nul = null as unknown as Config['health'];
    expect(rebaseConfig({ base: cfg({ health: nul }), local: cfg({ health: h }), localApps: [], theirs: cfg({ health: nul }) }).config.health).toEqual(h);
    expect(rebaseConfig({ base: cfg({ health: h }), local: cfg({ health: nul }), localApps: [], theirs: cfg({ health: h }) }).config.health).toEqual(h);
    expect(rebaseConfig({ base: cfg(), local: cfg({ health: nul }), localApps: [], theirs: cfg({ health: nul }) }).config.health).toBeUndefined();
  });
});

describe('byCodePoint', () => {
  it('orders keys by code point, independent of locale', () => {
    expect(['b', 'a', 'B', 'a'].sort(byCodePoint)).toEqual(['B', 'a', 'a', 'b']);
  });
});

describe('server-owned health check marker', () => {
  it('treats docker_managed_health_check as server-owned', () => {
    expect(SERVER_OWNED_APP_FIELDS.has('docker_managed_health_check')).toBe(true);
  });
});
