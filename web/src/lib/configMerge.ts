// Client-side three-way rebase of unsaved Settings edits onto a fresh
// server config. The rules mirror internal/handlers/config_merge.go so a
// rebase here and a three-way save on the server agree on every item:
//
//   - a field mine left as it was in base takes theirs, otherwise mine;
//   - items match base by identity (original_name, else name), renames
//     claiming first; apps fall back to docker_key for server renames;
//   - a server-added item is kept, a user-deleted item stays deleted;
//   - a server-removed item is dropped unless mine edited it, in which
//     case it is kept without server-owned tracking fields;
//   - server-owned app fields always come from theirs;
//   - apps left in a group mine deleted become ungrouped, and a group
//     rename re-points apps once, after the merge.
//
// Where the server would answer a name collision with a 409, the rebase
// keeps both items and reports a MergeConflict: the save stays rejected
// until the user renames one of them, so Settings shows the message and
// blocks or explains the save.

import { type App, type Config, type Group, type KeyCombo, type KeybindingsConfig, makeApp, makeGroup } from './types';

type Obj = Record<string, unknown>;

/**
 * JSON names of the fields Go declares as pointers in the client config
 * shape. Go compares a pointer by presence: nil and a pointer to the zero
 * value differ (nil often means true, e.g. proxy_skip_tls_verify), so for
 * these keys only undefined and null read as absent and false, 0 or a
 * zero struct stay a value. Every name is unique across the shape, so the
 * set is keyed by name alone.
 */
export const POINTER_FIELDS: ReadonlySet<string> = new Set([
  'http_action_show_toast', // handlers.ClientAppConfig.HTTPActionShowToast *bool (internal/handlers/api.go)
  'health_check',           // handlers.ClientAppConfig.HealthCheck *bool (internal/handlers/api.go)
  'proxy_skip_tls_verify',  // handlers.ClientAppConfig.ProxySkipTLSVerify *bool (internal/handlers/api.go)
  'shortcut',               // handlers.ClientAppConfig.Shortcut *int (internal/handlers/api.go)
  'home_icon',              // config.NavigationConfig.HomeIcon *AppIconConfig (internal/config/config.go)
  'health',                 // handlers.ClientConfigUpdate.Health *config.HealthConfig (internal/handlers/api.go)
  'keybindings',            // handlers.ClientConfigUpdate.Keybindings *config.KeybindingsConfig (internal/handlers/api.go)
]);

function isZero(v: unknown): boolean {
  return v === undefined || v === null || v === '' || v === false || v === 0;
}

// canonical maps v to its comparison form, matching Go's typed-struct
// compare: object keys are sorted, and undefined, null, '', false, 0,
// empty arrays and empty objects all read as absent, since Go decodes an
// absent field to its zero value. A pointer value (pointer = true, see
// POINTER_FIELDS) is absent only when undefined or null; a present one
// keeps its zero value. Array elements keep their zero values (a Go slice
// keeps them too); only nested objects inside are normalised.
function canonical(v: unknown, pointer = false): unknown {
  if (pointer) {
    if (v === undefined || v === null) return undefined;
    return typeof v === 'object' ? { ptr: canonical(v) ?? {} } : { ptr: v };
  }
  if (isZero(v)) return undefined;
  if (Array.isArray(v)) {
    if (v.length === 0) return undefined;
    return v.map(e => (e !== null && typeof e === 'object' ? canonical(e) ?? {} : e ?? null));
  }
  if (typeof v === 'object') {
    const out: Obj = {};
    for (const k of Object.keys(v as Obj).sort()) {
      const c = canonical((v as Obj)[k], POINTER_FIELDS.has(k));
      if (c !== undefined) out[k] = c;
    }
    return Object.keys(out).length === 0 ? undefined : out;
  }
  return v;
}

function equalAs(a: unknown, b: unknown, pointer: boolean): boolean {
  return JSON.stringify(canonical(a, pointer)) === JSON.stringify(canonical(b, pointer));
}

/**
 * Compares a and b the way Go compares the typed values: key-sorted, with
 * absent, null, zero values and empty containers alike, except for the
 * POINTER_FIELDS keys inside objects, which compare by presence.
 */
export function deepEqual(a: unknown, b: unknown): boolean {
  return equalAs(a, b, false);
}

/** Theirs when mine still equals base, else mine. */
export function mergeField<T>(base: T, mine: T, theirs: T): T {
  return deepEqual(mine, base) ? theirs : mine;
}

/**
 * Copies theirs and overwrites every key (over the union of all three
 * sides) where mine differs from base. Keys in skip always stay theirs.
 */
export function mergeObject<T extends object>(base: T, mine: T, theirs: T, skip?: ReadonlySet<keyof T>): T {
  const b = base as Obj, m = mine as Obj;
  const out: Obj = { ...(theirs as Obj) };
  const keys = new Set([...Object.keys(b), ...Object.keys(m), ...Object.keys(theirs as Obj)]);
  for (const k of keys) {
    if (skip?.has(k as keyof T) || equalAs(m[k], b[k], POINTER_FIELDS.has(k))) continue;
    if (m[k] === undefined) delete out[k];
    else out[k] = m[k];
  }
  return out as T;
}

/** App fields a save never takes from the payload (Go serverOwnedAppFields). */
export const SERVER_OWNED_APP_FIELDS: ReadonlySet<keyof App> = new Set<keyof App>([
  'original_name', 'proxyUrl', 'gateway_domain',
  'docker_key', 'docker_endpoint', 'docker_strategy', 'docker_managed_url',
]);

/** Keys only the client adds: the svelte-dnd-action `id` stamp. */
export const CLIENT_ONLY_KEYS: ReadonlySet<string> = new Set(['id']);

const GROUP_SKIP: ReadonlySet<keyof Group> = new Set<keyof Group>(['original_name']);

function dropKeys<T>(item: T, keys: Iterable<string>): T {
  const out = { ...item } as Obj;
  for (const k of keys) delete out[k];
  return out as T;
}

// clone deep-copies a wire object (it may be a Svelte state proxy, which
// structuredClone refuses).
function clone<T>(v: T): T {
  return JSON.parse(JSON.stringify(v)) as T;
}

/** makeApp(app) without client-only keys, so defaults never read as edits. */
export function normaliseApp(app: App): App {
  return dropKeys(makeApp(clone(app)), CLIENT_ONLY_KEYS);
}

/** makeGroup(group) without client-only keys. */
export function normaliseGroup(group: Group): Group {
  return dropKeys(makeGroup(clone(group)), CLIENT_ONLY_KEYS);
}

/** Records each item's current name as its rename identity. */
export function stampOriginalNames(apps: App[], groups: Group[]): void {
  for (const a of apps) a.original_name = a.name;
  for (const g of groups) g.original_name = g.name;
}

function stripServerOwned(a: App): App {
  return dropKeys(a, [...SERVER_OWNED_APP_FIELDS, ...CLIENT_ONLY_KEYS]);
}

function stripGroupIdentity(g: Group): Group {
  return dropKeys(g, ['original_name', ...CLIENT_ONLY_KEYS]);
}

function appsEqual(a: App, b: App): boolean {
  return deepEqual(stripServerOwned(a), stripServerOwned(b));
}

function groupsEqual(a: Group, b: Group): boolean {
  return deepEqual(stripGroupIdentity(a), stripGroupIdentity(b));
}

/** A name collision the rebase resolved in favour of the user's item. */
export interface MergeConflict {
  kind: 'app' | 'group';
  name: string;
  /** The item's old name when the user's side renamed it. */
  renamedFrom?: string;
  /** Text Settings shows the user. */
  message: string;
}

function conflict(kind: 'app' | 'group', name: string, renamedFrom: string, ownDuplicate = false): MergeConflict {
  const message = ownDuplicate
    ? `two ${kind}s are named "${name}"; rename one of them before saving`
    : `both you and the server now have ${kind === 'app' ? 'an' : 'a'} ${kind} named "${name}"; rename one of them before saving`;
  const c: MergeConflict = { kind, name, message };
  if (renamedFrom) c.renamedFrom = renamedFrom;
  return c;
}

type Named = { name: string; original_name?: string };

function identity(item: Named): string {
  return item.original_name || item.name;
}

function renamedFrom(item: Named): string {
  return item.original_name && item.original_name !== item.name ? item.original_name : '';
}

// claimEach pairs each mine item with one counterpart from lookup: items
// carrying original_name claim first, the rest match by name against what
// is still unclaimed. An item whose match is taken gets undefined.
function claimEach<M extends Named, S>(mine: M[], lookup: (id: string) => S | undefined): (S | undefined)[] {
  const out: (S | undefined)[] = new Array(mine.length).fill(undefined);
  const claimed = new Set<S>();
  for (const renamesPass of [true, false]) {
    mine.forEach((m, i) => {
      if (Boolean(m.original_name) !== renamesPass) return;
      const p = lookup(identity(m));
      if (p !== undefined && !claimed.has(p)) {
        claimed.add(p);
        out[i] = p;
      }
    });
  }
  return out;
}

interface Merged<T> {
  item: T;
  base?: T;
  theirs?: T;
  fromMine: boolean;
  renamedFrom: string;
}

interface ListRules<T extends Named> {
  kind: 'app' | 'group';
  equal: (a: T, b: T) => boolean;
  strip: (item: T) => T;
  skip: ReadonlySet<keyof T>;
  // matchers builds, per merge, how a base identity finds its theirs item
  // and whether an unclaimed theirs item is one the user deleted.
  matchers: (base: T[], theirs: T[]) => {
    lookupTheirs: (id: string) => T | undefined;
    deletedByUser: (t: T) => boolean;
  };
}

interface ListResult<T> {
  merged: Merged<T>[];
  conflicts: MergeConflict[];
}

function mergeList<T extends Named>(base: T[], mine: T[], theirs: T[], rules: ListRules<T>): ListResult<T> {
  const baseByName = byName(base);
  const { lookupTheirs, deletedByUser } = rules.matchers(base, theirs);
  const bases = claimEach(mine, id => baseByName.get(id));
  const matched = claimEach(mine, lookupTheirs);
  const consumed = new Set<T>();
  const merged: Merged<T>[] = [];
  const conflicts: MergeConflict[] = [];
  mine.forEach((m, i) => {
    const b = bases[i], t = matched[i];
    const from = renamedFrom(m);
    if (t) consumed.add(t);
    if (!b && t) {
      // New in mine while the server added its match: keep both. A shared
      // name is reported by reportDuplicates; different names do not clash.
      merged.push({ item: rules.strip(m), fromMine: true, renamedFrom: from });
      merged.push({ item: { ...t, original_name: t.name }, theirs: t, fromMine: false, renamedFrom: '' });
    } else if (!b) {
      merged.push({ item: rules.strip(m), fromMine: true, renamedFrom: from });
    } else if (!t) {
      // Removed by the server: dropped unless the user edited it.
      if (!rules.equal(m, b)) merged.push({ item: rules.strip(m), base: b, fromMine: true, renamedFrom: from });
    } else {
      const item = mergeObject(b, m, t, rules.skip);
      item.original_name = t.name;
      merged.push({ item, base: b, theirs: t, fromMine: true, renamedFrom: from });
    }
  });
  for (const t of theirs) {
    if (consumed.has(t) || deletedByUser(t)) continue;
    merged.push({ item: { ...t, original_name: t.name }, theirs: t, fromMine: false, renamedFrom: '' });
  }
  reportDuplicates(rules.kind, merged, conflicts);
  return { merged, conflicts };
}

// reportDuplicates records every name that occurs twice in the merge.
// Both items stay so the user can see them and rename one; the server
// would reject the save with a 409 until then.
function reportDuplicates<T extends Named>(kind: 'app' | 'group', merged: Merged<T>[], conflicts: MergeConflict[]): void {
  const first = new Map<string, Merged<T>>();
  for (const cur of merged) {
    const prev = first.get(cur.item.name);
    if (!prev) first.set(cur.item.name, cur);
    else conflicts.push(conflict(kind, cur.item.name, cur.renamedFrom || prev.renamedFrom, cur.fromMine && prev.fromMine));
  }
}

function byName<T extends Named>(items: T[]): Map<string, T> {
  return new Map(items.map(i => [i.name, i]));
}

const APP_RULES: ListRules<App> = {
  kind: 'app',
  equal: appsEqual,
  strip: stripServerOwned,
  skip: SERVER_OWNED_APP_FIELDS,
  matchers: (base, theirs) => {
    const baseByName = byName(base);
    const theirsByName = byName(theirs);
    const theirsByKey = new Map(theirs.filter(t => t.docker_key).map(t => [t.docker_key!, t]));
    const baseKeys = new Set(base.filter(b => b.docker_key).map(b => b.docker_key!));
    return {
      // By name, else (a server rename) by the base app's docker_key.
      lookupTheirs: id => {
        const t = theirsByName.get(id);
        if (t) return t;
        const key = baseByName.get(id)?.docker_key;
        return key ? theirsByKey.get(key) : undefined;
      },
      deletedByUser: t => baseByName.has(t.name) || (!!t.docker_key && baseKeys.has(t.docker_key)),
    };
  },
};

const GROUP_RULES: ListRules<Group> = {
  kind: 'group',
  equal: groupsEqual,
  strip: stripGroupIdentity,
  skip: GROUP_SKIP,
  matchers: (base, theirs) => {
    const baseByName = byName(base);
    const theirsByName = byName(theirs);
    return {
      lookupTheirs: id => theirsByName.get(id),
      deletedByUser: t => baseByName.has(t.name),
    };
  },
};

/** Merges app lists (inputs already normalised). Name conflicts keep both apps. */
export function mergeApps(base: App[], mine: App[], theirs: App[]): App[] {
  return mergeList(base, mine, theirs, APP_RULES).merged.map(m => m.item);
}

/** Merges group lists with the app rules, matching by name only. */
export function mergeGroups(base: Group[], mine: Group[], theirs: Group[]): Group[] {
  return mergeList(base, mine, theirs, GROUP_RULES).merged.map(m => m.item);
}

/**
 * Re-points apps still carrying a renamed group's old name. When a group
 * of the old name still exists (the user added a new one under it), only
 * apps wasIn reports as having been in the old group move.
 */
export function cascadeGroupRenames(groups: Group[], apps: App[], wasIn: (i: number, group: string) => boolean = () => false): void {
  const renamed = new Map<string, string>();
  const present = new Set<string>();
  for (const g of groups) {
    present.add(g.name);
    if (g.original_name && g.original_name !== g.name) renamed.set(g.original_name, g.name);
  }
  apps.forEach((a, i) => {
    const n = renamed.get(a.group);
    if (n === undefined || (present.has(a.group) && !wasIn(i, a.group))) return;
    a.group = n;
  });
}

// ungroupDeletedGroups moves apps out of groups mine deleted: a base group
// no mine group claims by identity and no merged group still bears.
function ungroupDeletedGroups(base: Group[], mine: Group[], merged: Group[], apps: App[]): void {
  const kept = new Set([...mine.map(identity), ...merged.map(g => g.name)]);
  const deleted = new Set(base.filter(b => !kept.has(b.name)).map(b => b.name));
  for (const a of apps) {
    if (deleted.has(a.group)) a.group = '';
  }
}

function mergeOptional<T extends object>(base: T | undefined, mine: T | undefined, theirs: T | undefined): T | undefined {
  if (mine === undefined) return theirs;
  if (base === undefined || theirs === undefined) return mine;
  return mergeObject(base, mine, theirs);
}

function mergeKeybindings(base?: KeybindingsConfig, mine?: KeybindingsConfig, theirs?: KeybindingsConfig): KeybindingsConfig | undefined {
  if (mine === undefined) return theirs;
  const b = base?.bindings ?? {}, m = mine.bindings ?? {}, t = theirs?.bindings ?? {};
  const out: Record<string, KeyCombo[]> = { ...t };
  for (const action of Object.keys(m)) {
    if (!deepEqual(m[action], b[action])) out[action] = m[action];
  }
  for (const action of Object.keys(b)) {
    if (!(action in m)) delete out[action];
  }
  return { bindings: out };
}

export interface RebaseInput { base: Config; local: Config; localApps: App[]; theirs: Config }
export interface RebaseResult { config: Config; apps: App[]; conflicts: MergeConflict[] }

// The scalar fields of Go's ClientConfigUpdate. Every other top-level key
// (auth, tls, gateway, discovery, env_overrides) is not part of a config
// save, so the rebase takes the server's value for it.
const TOP_LEVEL_SCALARS = ['title', 'language', 'log_level', 'proxy_timeout', 'session_cookie_domain'] as const;

/**
 * Rebases the user's unsaved edits (local config plus localApps) from
 * base onto theirs. All lists are normalised first; result items carry
 * original_name = theirs' name and no `id` (callers re-stamp).
 */
export function rebaseConfig(input: RebaseInput): RebaseResult {
  const { base, local, localApps, theirs } = input;
  const groups = mergeList(
    (base.groups ?? []).map(normaliseGroup),
    (local.groups ?? []).map(normaliseGroup),
    (theirs.groups ?? []).map(normaliseGroup),
    GROUP_RULES,
  );
  const apps = mergeList(
    (base.apps ?? []).map(normaliseApp),
    localApps.map(normaliseApp),
    (theirs.apps ?? []).map(normaliseApp),
    APP_RULES,
  );
  const mergedGroups = groups.merged.map(m => m.item);
  const mergedApps = apps.merged.map(m => m.item);
  ungroupDeletedGroups(
    (base.groups ?? []).map(normaliseGroup),
    (local.groups ?? []).map(normaliseGroup),
    mergedGroups,
    mergedApps,
  );
  cascadeGroupRenames(mergedGroups, mergedApps, (i, group) =>
    apps.merged[i].base?.group === group || apps.merged[i].theirs?.group === group);

  const config: Config = { ...theirs };
  const fields = config as unknown as Obj;
  for (const k of TOP_LEVEL_SCALARS) {
    const v = mergeField(base[k], local[k], theirs[k]);
    if (v === undefined) delete fields[k];
    else fields[k] = v;
  }
  // Navigation and Theme are value structs in Go: merged per field, an
  // absent side reading as the zero struct. Health and Keybindings are
  // pointers: an absent payload keeps the server's (mergeHealth and
  // mergeKeybindings).
  config.navigation = mergeObject(base.navigation ?? {}, local.navigation ?? {}, theirs.navigation ?? {}) as Config['navigation'];
  config.theme = mergeObject(base.theme ?? {}, local.theme ?? {}, theirs.theme ?? {}) as Config['theme'];
  const pointers = {
    health: mergeOptional(base.health, local.health, theirs.health),
    keybindings: mergeKeybindings(base.keybindings, local.keybindings, theirs.keybindings),
  };
  for (const [k, v] of Object.entries(pointers)) {
    if (v === undefined) delete fields[k];
    else fields[k] = v;
  }
  config.groups = mergedGroups;
  config.apps = mergedApps;
  return { config, apps: mergedApps, conflicts: [...groups.conflicts, ...apps.conflicts] };
}
