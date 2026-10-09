<script lang="ts">
  import { iconLabel } from '$lib/iconUrl';
  import { onMount, untrack } from 'svelte';
  import { fade, fly } from 'svelte/transition';
  import { type App, type Config, type Group, type KeybindingsConfig, makeApp, makeGroup, stampUniqueIds } from '$lib/types';
  import { normaliseBase, rebaseConfig, stampOriginalNames, type MergeConflict } from '$lib/configMerge';
  import { refreshDockerTracking, withoutDockerTracking } from '$lib/dockerTracking';
  import IconBrowser from './IconBrowser.svelte';
  import AppForm from './AppForm.svelte';
  import { focusTrap } from '$lib/focusTrap';
  import AppIcon from './AppIcon.svelte';
  import KeybindingsEditor from './KeybindingsEditor.svelte';
  import AboutTab from './settings/AboutTab.svelte';
  import AppsTab from './settings/AppsTab.svelte';
  import GatewayTab from './settings/GatewayTab.svelte';
  import DiscoveryTab from './settings/DiscoveryTab.svelte';
  import DiscoverModal from './settings/DiscoverModal.svelte';
  import GeneralTab from './settings/GeneralTab.svelte';
  import SecurityTab from './settings/SecurityTab.svelte';
  import ThemeTab from './settings/ThemeTab.svelte';
  import { get } from 'svelte/store';
  import { selectedFamily, variantMode, setThemeFamily, setVariantMode } from '$lib/themeStore';
  import { isMobileViewport } from '$lib/useSwipe';
  import { exportConfig, parseImportedConfig, fetchConfig, errorText, type ImportedConfig } from '$lib/api';
  import { slugify, findSlugConflict } from '$lib/slug';
  import { toasts } from '$lib/toastStore';
  import { customBindings, getKeybindingsForConfig, initKeybindings } from '$lib/keybindingsStore';
  import { appSchema, groupSchema, extractErrors } from '$lib/schemas';
  import { popularApps, templateToApp, type PopularAppTemplate } from '$lib/popularApps';
  import * as m from '$lib/paraglide/messages.js';

  let {
    config,
    apps,
    initialTab = 'general',
    initialEditAppName,
    onclose,
    onsave,
    onauthchange,
  }: {
    config: Config;
    apps: App[];
    initialTab?: 'general' | 'apps' | 'theme' | 'keybindings' | 'security' | 'gateway' | 'discovery' | 'about';
    initialEditAppName?: string | null;
    onclose?: () => void;
    /** Saves config three-way against base; rejects on failure, which keeps the dialog open. */
    onsave?: (config: Config, base: Config) => Promise<void> | void;
    /** Called with the new auth block after the Security tab changed the auth method. */
    onauthchange?: (auth: NonNullable<Config['auth']>) => void;
  } = $props();

  // Exported: returns true if Escape was consumed by closing an inner sub-modal,
  // or while a save is in flight (the dialog must stay open until it settles).
  export function handleEscape(): boolean {
    if (saving) return true;
    if (showIconBrowser) { showIconBrowser = false; iconBrowserTarget = null; return true; }
    if (editingApp) { cancelEditApp(); return true; }
    if (editingGroup) { cancelEditGroup(); return true; }
    if (showAddApp) { showAddApp = false; return true; }
    if (showAddGroup) { showAddGroup = false; return true; }
    if (pendingImport) { pendingImport = null; showImportConfirm = false; return true; }
    return false; // No sub-modal was open; caller should close Settings
  }

  let isMobile = $state(false);

  onMount(() => {
    isMobile = isMobileViewport();
    const handleResize = () => { isMobile = isMobileViewport(); };
    window.addEventListener('resize', handleResize);

    // Deep-link straight into an app's edit modal (right-click on a nav
    // icon, #407). Resolve by name against dndGroupedApps: the edit
    // modal's save/cancel flow depends on object identity within those
    // arrays, so the App-level object cannot be used directly.
    if (initialEditAppName) {
      const match = Object.values(dndGroupedApps).flat().find(a => a.name === initialEditAppName);
      if (match) startEditApp(match);
    }

    return () => window.removeEventListener('resize', handleResize);
  });

  // Active tab
  let activeTab = $state(untrack(() => initialTab ?? 'general'));

  // Deep copy of a wire object (a parent's state proxy included, which
  // structuredClone refuses).
  function clone<T>(v: T): T {
    return JSON.parse(JSON.stringify(v)) as T;
  }

  // The one load pipeline, used at mount and for every rebase snapshot:
  // normalise through the factories so every optional field (omitempty in
  // Go) is present, preventing bind:value from adding new properties and
  // triggering false "unsaved changes"; stamp the svelte-dnd-action ids and
  // the rename identity (original_name).
  function prepare(source: Config, sourceApps: App[]): { config: Config; apps: App[] } {
    const c = clone(source);
    c.groups = (c.groups ?? []).map((g: Group) => makeGroup(g));
    const a = clone(sourceApps ?? []).map(x => makeApp(x));
    stampUniqueIds(a);
    stampUniqueIds(c.groups);
    stampOriginalNames(a, c.groups);
    return { config: c, apps: a };
  }

  const loaded = untrack(() => prepare(config, apps));

  // Local copy of config for editing.
  let localConfig = $state(loaded.config);
  let localApps = $state(loaded.apps);

  // The server config this dialog's edits are based on. Sent as the save's
  // base so the server merges three-way, and replaced on every rebase. Apps
  // come from the apps prop, which localApps started from. Apps and groups
  // go through the same factories as the payload: a default the factory
  // fills in (an absent scale becomes 1) must not read as an edit on the
  // server, or an untouched app the server removed would be saved back.
  let baseConfig = $state<Config>(untrack(() => normaliseBase(config, apps)));

  // Bumped on every rebase; GatewayTab reloads its sites when it changes.
  let configRevision = $state(0);

  // A save in flight, and the server's message when the last one failed.
  let saving = $state(false);
  let saveError = $state<string | null>(null);

  // Name collisions the last rebase kept for the user to resolve. A
  // conflict clears as soon as the user renames one of the two items.
  let rebaseConflicts = $state<MergeConflict[]>([]);

  // Icon browser state
  let showIconBrowser = $state(false);
  let iconBrowserTarget = $state<'newApp' | 'editApp' | 'newGroup' | 'editGroup' | 'homeIcon' | null>(null);

  // Track if changes have been made (declared below after snapshot variables)

  // Editing state
  let editingApp = $state<App | null>(null);
  let editingGroup = $state<Group | null>(null);
  let editAppSnapshot = $state<string | null>(null);
  let editGroupSnapshot = $state<string | null>(null);
  let editAppErrors = $state<Record<string, string>>({});
  let editGroupErrors = $state<Record<string, string>>({});
  let showAddApp = $state(false);
  let addAppStep = $state<'choose' | 'configure'>('choose');
  let addAppSearch = $state('');
  let addAppSearchLower = $derived(addAppSearch.toLowerCase());
  let showAddGroup = $state(false);

  // Discover modal state. mode tracks whether the modal was opened
  // from the Apps tab (defaults to creating apps) or the Gateway tab
  // (defaults to creating gateway sites). Closed when both falsy.
  let showDiscoverModal = $state(false);
  let discoverMode = $state<'apps' | 'gateway'>('apps');

  function openDiscoverFromApps() {
    discoverMode = 'apps';
    showDiscoverModal = true;
  }
  function openDiscoverFromGateway() {
    discoverMode = 'gateway';
    showDiscoverModal = true;
  }

  // Import/export state
  let showImportConfirm = $state(false);
  let pendingImport = $state<ImportedConfig | null>(null);

  // New app/group templates
  const newAppTemplate: App = makeApp();
  const newGroupTemplate: Group = makeGroup();

  let newApp = $state({ ...newAppTemplate });
  let newGroup = $state({ ...newGroupTemplate });

  // Icon browser: derive the current target's icon for pre-populating the browser
  let iconBrowserIcon = $derived(
    iconBrowserTarget === 'editApp' ? editingApp?.icon :
    iconBrowserTarget === 'editGroup' ? editingGroup?.icon :
    iconBrowserTarget === 'newApp' ? newApp.icon :
    iconBrowserTarget === 'newGroup' ? newGroup.icon :
    iconBrowserTarget === 'homeIcon' ? localConfig.navigation.home_icon : null
  );

  // Validation error state
  let appErrors = $state<Record<string, string>>({});
  let groupErrors = $state<Record<string, string>>({});

  // Dirty-state keys. Key-sorted JSON, lists sorted by name, without the
  // client-only dnd `id` and the Docker tracking fields (the server owns
  // them, so a detach made while this dialog is open is not an unsaved
  // change). Ordering lives in the `order` fields, so a rebase that only
  // changes key or list order never reads as an edit. config.apps is left
  // out: localApps is the apps' source of truth until the save.
  function stateKey(value: unknown): string {
    return JSON.stringify(value, (key, v) => {
      if (key === 'id' || withoutDockerTracking(key, v) === undefined) return undefined;
      if (v && typeof v === 'object' && !Array.isArray(v)) {
        const o = v as Record<string, unknown>;
        // An empty object and an absent one decode alike in Go (the
        // rebase yields theme: {} where the server sent none).
        if (Object.keys(o).length === 0 && key !== '') return undefined;
        return Object.fromEntries(Object.keys(o).sort().map(k => [k, o[k]]));
      }
      return v;
    });
  }
  function sortedByName<T extends { name: string }>(list: T[]): T[] {
    return [...list].sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0));
  }
  function configKey(c: Config): string {
    return stateKey({ ...c, apps: undefined, groups: sortedByName(c.groups ?? []) });
  }
  function appsKey(list: App[]): string {
    return stateKey(sortedByName(list));
  }

  // Snapshots of the loaded config, so hasChanges starts as false.
  let initialConfigSnapshot = $state(configKey(loaded.config));
  let initialAppsSnapshot = $state(appsKey(loaded.apps));

  // Snapshot theme so we can revert on close without save. A rebase moves
  // it to the server's theme.
  let initialFamily = $state(untrack(() => get(selectedFamily)));
  let initialVariant = $state(untrack(() => get(variantMode)));

  // Keybinding edits go straight into the global store (the editor and the
  // live shortcuts read it), so they are compared against the bindings the
  // dialog opened with, normalised like getKeybindingsForConfig.
  function keybindingsKey(kb?: KeybindingsConfig): string {
    const bindings = Object.entries(kb?.bindings ?? {}).filter(([, combos]) => combos && combos.length > 0);
    return stateKey(Object.fromEntries(bindings));
  }
  let initialKeybindings = $state(untrack(() => keybindingsKey(getKeybindingsForConfig())));
  let keybindingsDirty = $derived.by(() => {
    void $customBindings;
    return keybindingsKey(getKeybindingsForConfig()) !== initialKeybindings;
  });

  // Track if changes have been made
  let hasChanges = $derived(configKey(localConfig) !== initialConfigSnapshot ||
                  appsKey(localApps) !== initialAppsSnapshot ||
                  keybindingsDirty ||
                  $selectedFamily !== initialFamily ||
                  $variantMode !== initialVariant);

  // The Security tab applies an auth method change straight to the server
  // (PUT /api/auth/method) and mirrors it into localConfig.auth. The server
  // already has it and PUT /api/config ignores the auth block, so move it
  // into the saved snapshot; any other unsaved edits stay unsaved. The app
  // shell is told too, so reopening Settings shows the new method.
  function handleAuthMethodApplied() {
    if (!localConfig.auth) return;
    const auth = $state.snapshot(localConfig.auth);
    const saved = JSON.parse(initialConfigSnapshot) as Config;
    saved.auth = auth;
    initialConfigSnapshot = configKey(saved);
    onauthchange?.(auth);
  }

  // Mutable arrays for svelte-dnd-action (NOT reactive derivations — the library owns these)
  let dndGroups = $state<Group[]>([...untrack(() => localConfig).groups].sort((a, b) => a.order - b.order));
  let dndGroupedApps = $state<Record<string, App[]>>(buildGroupedApps());

  function buildGroupedApps(): Record<string, App[]> {
    const acc: Record<string, App[]> = {};
    for (const app of localApps) {
      const group = app.group || '';
      if (!acc[group]) acc[group] = [];
      acc[group].push(app);
    }
    Object.values(acc).forEach(arr => arr.sort((a, b) => a.order - b.order));
    return acc;
  }

  function rebuildDndArrays() {
    dndGroups = [...localConfig.groups].sort((a, b) => a.order - b.order);
    dndGroupedApps = buildGroupedApps();
  }

  // Conflicts still unresolved by the current edits, one per kind and name.
  let conflicts = $derived.by(() => {
    const out: MergeConflict[] = [];
    for (const c of rebaseConflicts) {
      const list: { name: string }[] = c.kind === 'app' ? localApps : localConfig.groups;
      if (out.some(o => o.kind === c.kind && o.name === c.name)) continue;
      if (list.filter(i => i.name === c.name).length < 2) continue;
      out.push(c);
    }
    return out;
  });

  // A server config waiting for an app/group sub-modal or a save to finish.
  // Rebasing under an open modal would replace the objects it edits, so the
  // rebase waits; the effect below applies it once nothing holds them.
  let pendingRebase = $state.raw<Config | null>(null);
  // Set once a save succeeded: the dialog is closing, later pushes are moot.
  let closed = false;
  let subModalOpen = $derived(!!editingApp || !!editingGroup || showAddApp || showAddGroup);

  /**
   * Rebases the unsaved edits onto a fresh server config (a push, a
   * reconnect, a Discover import). Applies now, or queues while an app or
   * group sub-modal is open or a save is in flight.
   */
  export function rebase(theirs: Config): void {
    if (closed) return;
    if (subModalOpen || saving) {
      pendingRebase = theirs;
      return;
    }
    applyRebase(theirs);
  }

  function applyRebase(theirsIn: Config) {
    const theirs = clone(theirsIn);
    const hadEdits = hasChanges;
    const hadKeybindingEdits = keybindingsDirty;
    const hadThemeEdits = get(selectedFamily) !== initialFamily || get(variantMode) !== initialVariant;
    const beforeConfig = configKey(localConfig);
    const beforeApps = appsKey(localApps);
    const { config: merged, apps: mergedApps, conflicts: found } = rebaseConfig({
      base: $state.snapshot(baseConfig) as Config,
      local: $state.snapshot(localConfig) as Config,
      localApps: $state.snapshot(localApps) as App[],
      theirs,
    });
    // Merged items keep original_name = the server's name, so a rename the
    // user made before this rebase still saves as a rename. Only ids are
    // re-stamped.
    stampUniqueIds(mergedApps);
    stampUniqueIds(merged.groups);
    const fresh = prepare(theirs, theirs.apps ?? []);
    baseConfig = normaliseBase(theirs, theirs.apps ?? []);
    localConfig = { ...merged, apps: clone(mergedApps) };
    localApps = mergedApps;
    initialConfigSnapshot = configKey(fresh.config);
    initialAppsSnapshot = appsKey(fresh.apps);
    rebaseConflicts = found;
    // Untouched keybindings follow the server; edited ones stay and are
    // compared against the server's from now on.
    if (!hadKeybindingEdits) initKeybindings(theirs.keybindings);
    initialKeybindings = keybindingsKey(theirs.keybindings);
    syncThemeOnRebase(theirs.theme, hadThemeEdits);
    rebuildDndArrays();
    configRevision += 1;
    if (hadEdits && (configKey(localConfig) !== beforeConfig || appsKey(localApps) !== beforeApps)) {
      toasts.info(m.settings_rebased());
    }
  }

  // An untouched theme follows the server, so a save never writes back the
  // theme this dialog opened with over one another admin saved meanwhile.
  // An edited theme stays; either way the server's theme is the one a
  // discard returns to.
  function syncThemeOnRebase(theme: Config['theme'], edited: boolean) {
    if (!theme?.family) return;
    const v = theme.variant;
    const variant = v === 'dark' || v === 'light' || v === 'system' ? v : null;
    if (!edited) {
      if (get(selectedFamily) !== theme.family) setThemeFamily(theme.family);
      if (variant && get(variantMode) !== variant) setVariantMode(variant);
    }
    initialFamily = theme.family;
    if (variant) initialVariant = variant;
  }

  // A new config prop (the shell refetched after a push or reconnect).
  // svelte-ignore state_referenced_locally
  let lastConfigProp = config;
  $effect(() => {
    const c = config;
    if (c === lastConfigProp) return;
    lastConfigProp = c;
    untrack(() => rebase(c));
  });

  $effect(() => {
    const t = pendingRebase;
    if (!t || subModalOpen || saving) return;
    untrack(() => {
      pendingRebase = null;
      if (!closed) applyRebase(t);
    });
  });

  // Sync callbacks from AppsTab DnD
  function syncGroupOrder(groups: Group[]) {
    localConfig.groups = [...groups];
  }

  function syncAppOrder(groupName: string, items: App[]) {
    if (groupName === '__delete__' || groupName === '__rebuild__') {
      // Full rebuild from dndGroupedApps
      const allApps: App[] = [];
      for (const apps of Object.values(dndGroupedApps)) {
        allApps.push(...apps);
      }
      stampUniqueIds(allApps);
      localApps = allApps;
      if (groupName === '__rebuild__') {
        localConfig.groups = [...dndGroups];
      }
      return;
    }
    // Sync a single group's apps back to localApps
    const otherApps = localApps.filter(a => (a.group || '') !== groupName && !items.find(n => n.name === a.name));
    localApps = [...otherApps, ...items];
  }

  // Detach and Re-link under Discovery change tracking on the server. Copy
  // the server's tracking onto the local apps so a detached app's URL unlocks
  // without reopening Settings.
  function handleDockerTrackingChanged() {
    void refreshDockerTracking(editingApp ? [localApps, [editingApp]] : [localApps], fetchConfig);
  }

  // The config sent to the server: without the client-only dnd `id` that
  // stampUniqueIds puts on every app and group.
  function savePayload(): Config {
    const c = $state.snapshot(localConfig) as Config;
    const strip = <T extends object>(item: T): T => {
      const { id: _id, ...rest } = item as T & { id?: unknown };
      return rest as T;
    };
    c.apps = (c.apps ?? []).map(strip);
    c.groups = (c.groups ?? []).map(strip);
    return c;
  }

  // Closes only after the save succeeded. On failure the dialog stays open
  // with every edit and shows the server's message.
  async function handleSave() {
    if (saving) return;
    saving = true;
    saveError = null;
    try {
      // Update config with local changes
      localConfig.apps = localApps;
      // Capture current theme from stores into config
      localConfig.theme = {
        family: get(selectedFamily),
        variant: get(variantMode)
      };
      // Include keybindings if changed
      if (keybindingsDirty) {
        localConfig.keybindings = getKeybindingsForConfig();
      }
      await onsave?.(savePayload(), $state.snapshot(baseConfig) as Config);
      closed = true;
      pendingRebase = null;
      onclose?.();
    } catch (e) {
      saveError = errorText(e, m.toast_failedSaveConfig());
    } finally {
      saving = false;
    }
  }

  // Discover imported apps or sites: rebase onto the server's config like
  // a push, keeping unsaved edits, then close the modal (S-52).
  async function handleDiscoverImported() {
    try {
      rebase(await fetchConfig());
    } catch {
      toasts.error(m.error_failedLoadConfig());
    } finally {
      showDiscoverModal = false;
    }
  }

  // Inline confirmation state
  let confirmClose = $state(false);

  /**
   * Every way of closing the dialog goes through here. Returns true when it
   * closed (nothing unsaved), false when it showed the discard prompt or a
   * save is still in flight. Closing reverts the previewed theme and any
   * keybinding edits.
   */
  export function requestClose(): boolean {
    if (saving) return false;
    if (hasChanges) {
      confirmClose = true;
      return false;
    }
    discardAndClose();
    return true;
  }

  function confirmCloseDiscard() {
    if (saving) return;
    confirmClose = false;
    discardAndClose();
  }

  function discardAndClose() {
    revertTheme();
    initKeybindings($state.snapshot(baseConfig.keybindings) as KeybindingsConfig | undefined);
    onclose?.();
  }

  function revertTheme() {
    setThemeFamily(initialFamily);
    setVariantMode(initialVariant);
  }

  function selectPopularApp(template: PopularAppTemplate) {
    const app = templateToApp(template, template.defaultUrl, localApps.length);
    newApp = { ...app };
    addAppStep = 'configure';
  }

  function startCustomApp() {
    newApp = { ...newAppTemplate };
    addAppStep = 'configure';
  }

  // Validates http_action-specific fields (URL scheme, method default). Returns
  // an error message if invalid, or empty string when valid. Mutates app to set
  // a sensible method default. Backend validates again on save (Task 2), this
  // is purely a UX nicety so the form catches scheme typos before round-tripping.
  function validateHttpAction(app: App): string {
    if (!app.url) {
      return 'URL is required';
    }
    let parsed: URL | null = null;
    try { parsed = new URL(app.url); } catch { /* fall through */ }
    if (!parsed || (parsed.protocol !== 'http:' && parsed.protocol !== 'https:')) {
      return 'URL must use http:// or https://';
    }
    if (!app.http_action_method) {
      app.http_action_method = 'POST';
    }
    return '';
  }

  function addApp() {
    const result = appSchema.safeParse(newApp);
    if (!result.success) {
      appErrors = extractErrors(result);
      return;
    }
    if (newApp.open_mode === 'http_action') {
      const httpActionErr = validateHttpAction(newApp);
      if (httpActionErr) {
        appErrors = { url: httpActionErr };
        return;
      }
    }
    const slugClash = findSlugConflict(newApp, localApps);
    if (slugClash) {
      appErrors = { name: m.appForm_slugConflict({ other: slugClash, slug: slugify(newApp.name ?? '') }) };
      return;
    }
    appErrors = {};
    newApp.order = localApps.length;
    const app = { ...newApp };
    // Auto-create the group if it doesn't exist yet (e.g. gallery apps with preset groups)
    if (app.group && !localConfig.groups.some(g => g.name === app.group)) {
      const groupName = app.group as string;
      const autoGroup = makeGroup({
        name: groupName,
        icon: { type: 'lucide', name: 'folder', file: '', url: '', variant: '' },
        color: '',
        order: localConfig.groups.length,
      });
      localConfig.groups = [...localConfig.groups, autoGroup];
      stampUniqueIds(localConfig.groups);
    }
    localApps = [...localApps, app];
    stampUniqueIds(localApps);
    newApp = { ...newAppTemplate };
    showAddApp = false;
    rebuildDndArrays();
  }

  function addGroup() {
    const result = groupSchema.safeParse(newGroup);
    if (!result.success) {
      groupErrors = extractErrors(result);
      return;
    }
    groupErrors = {};
    newGroup.order = localConfig.groups.length;
    const group = { ...newGroup };
    localConfig.groups = [...localConfig.groups, group];
    stampUniqueIds(localConfig.groups);
    newGroup = { ...newGroupTemplate };
    showAddGroup = false;
    rebuildDndArrays();
  }

  function startEditApp(app: App) {
    editAppSnapshot = JSON.stringify(app);
    editingApp = app;
  }

  function startEditGroup(group: Group) {
    editGroupSnapshot = JSON.stringify(group);
    editingGroup = group;
  }

  function closeEditApp() {
    if (editingApp) {
      const result = appSchema.safeParse({ name: editingApp.name, url: editingApp.url });
      if (!result.success) {
        editAppErrors = extractErrors(result);
        return;
      }
      if (editingApp.open_mode === 'http_action') {
        const httpActionErr = validateHttpAction(editingApp);
        if (httpActionErr) {
          editAppErrors = { url: httpActionErr };
          return;
        }
      }
      const slugClash = findSlugConflict(editingApp, localApps);
      if (slugClash) {
        editAppErrors = { name: m.appForm_slugConflict({ other: slugClash, slug: slugify(editingApp.name ?? '') }) };
        return;
      }
      editAppErrors = {};
      // Sync DnD app changes back to localApps before rebuilding
      const allApps: App[] = [];
      for (const apps of Object.values(dndGroupedApps)) {
        allApps.push(...apps);
      }
      stampUniqueIds(allApps);
      localApps = allApps;
    }
    editingApp = null;
    editAppSnapshot = null;
    rebuildDndArrays();
  }

  function cancelEditApp() {
    if (editingApp && editAppSnapshot) {
      const original = JSON.parse(editAppSnapshot) as App;
      for (const apps of Object.values(dndGroupedApps)) {
        const idx = apps.findIndex(a => a === editingApp);
        if (idx !== -1) { Object.assign(apps[idx], original); break; }
      }
    }
    editingApp = null;
    editAppSnapshot = null;
    editAppErrors = {};
    rebuildDndArrays();
  }

  function closeEditGroup() {
    if (editingGroup) {
      const result = groupSchema.safeParse({ name: editingGroup.name });
      if (!result.success) {
        editGroupErrors = extractErrors(result);
        return;
      }
      editGroupErrors = {};
      // Sync DnD group changes back to localConfig before rebuilding
      localConfig.groups = [...dndGroups];
      stampUniqueIds(localConfig.groups);
    }
    editingGroup = null;
    editGroupSnapshot = null;
    rebuildDndArrays();
  }

  function cancelEditGroup() {
    if (editingGroup && editGroupSnapshot) {
      const original = JSON.parse(editGroupSnapshot) as Group;
      const idx = dndGroups.findIndex(g => g === editingGroup);
      if (idx !== -1) { Object.assign(dndGroups[idx], original); }
    }
    editingGroup = null;
    editGroupSnapshot = null;
    editGroupErrors = {};
    rebuildDndArrays();
  }

  // Export config as YAML file
  function handleExport() {
    exportConfig();
    toasts.success(m.toast_configExported());
  }

  // Handle import file selection
  async function handleImportSelect(e: Event) {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;

    try {
      const content = await file.text();
      pendingImport = await parseImportedConfig(content);
      showImportConfirm = true;
    } catch (err) {
      toasts.error(err instanceof Error ? err.message : m.toast_failedParseConfig());
    }

    // Reset input so same file can be selected again
    input.value = '';
  }

  // Apply imported config
  function applyImport() {
    if (!pendingImport) return;

    localConfig = {
      ...localConfig,
      title: pendingImport.title,
      navigation: pendingImport.navigation,
      groups: pendingImport.groups,
    };
    localApps = pendingImport.apps;

    // Assign stable ids for svelte-dnd-action
    stampUniqueIds(localApps);
    stampUniqueIds(localConfig.groups);
    rebuildDndArrays();

    showImportConfirm = false;
    pendingImport = null;
    toasts.success(m.toast_configImported());
  }

  function cancelImport() {
    showImportConfirm = false;
    pendingImport = null;
  }

  function handleIconSelect(detail: { name: string; variant: string; type: string }) {
    const { name, variant, type } = detail;
    const iconData = {
      type: type as 'dashboard' | 'lucide' | 'custom' | 'url',
      name: type === 'custom' ? '' : name,
      variant,
      file: type === 'custom' ? name : '',
      url: '',
      color: '',
      background: '',
    };

    if (iconBrowserTarget === 'newApp') {
      newApp = { ...newApp, icon: iconData };
    } else if (iconBrowserTarget === 'editApp' && editingApp) {
      editingApp.icon = iconData;
    } else if (iconBrowserTarget === 'newGroup') {
      newGroup = { ...newGroup, icon: iconData };
    } else if (iconBrowserTarget === 'editGroup' && editingGroup) {
      editingGroup.icon = iconData;
    } else if (iconBrowserTarget === 'homeIcon') {
      localConfig.navigation.home_icon = iconData;
    }
    showIconBrowser = false;
    iconBrowserTarget = null;
  }

  function openIconBrowser(target: 'newApp' | 'editApp' | 'newGroup' | 'editGroup' | 'homeIcon') {
    iconBrowserTarget = target;
    showIconBrowser = true;
  }

</script>

<div class="settings">

<div
  class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 {isMobile ? 'p-0' : 'p-4'}"
  transition:fade={{ duration: 150 }}
>
  <div
    class="bg-bg-surface shadow-2xl w-full overflow-hidden border border-border flex flex-col
           {isMobile
             ? 'h-full max-h-full rounded-none'
             : 'rounded-xl max-w-4xl max-h-[90vh]'}"
    in:fly={{ y: isMobile ? 50 : 20, duration: 200 }}
    out:fade={{ duration: 100 }}
  >
    <!-- Header -->
    <div class="flex items-center justify-between p-4 border-b border-border flex-shrink-0">
      <h2 class="text-lg font-semibold text-text-primary">{m.settings_title()}</h2>
      <div class="flex items-center gap-2">
        {#if hasChanges}
          <span class="text-xs text-yellow-400">{m.settings_unsavedChanges()}</span>
        {/if}
        <button
          class="btn btn-primary btn-sm disabled:opacity-50"
          disabled={!hasChanges || saving || conflicts.length > 0}
          onclick={handleSave}
        >
          {saving ? m.settings_saving() : m.settings_saveChanges()}
        </button>
        <button
          class="btn btn-ghost btn-icon btn-sm"
          onclick={() => requestClose()}
          disabled={saving}
          aria-label={m.settings_closeSettings()}
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    </div>

    <!-- Save failure: the dialog stays open with every edit -->
    {#if saveError}
      <div class="px-4 py-2 bg-red-600/20 border-b border-red-600/40 text-sm text-red-200" role="alert">
        {m.settings_saveFailed({ error: saveError })}
      </div>
    {/if}

    <!-- Name conflicts a rebase kept: Save stays blocked until one is renamed -->
    {#if conflicts.length > 0}
      <div class="px-4 py-2 bg-red-600/20 border-b border-red-600/40 text-sm text-red-200 space-y-1" role="alert" data-testid="settings-conflicts">
        {#each conflicts as c (`${c.kind}:${c.name}`)}
          <p title={c.message}>{c.kind === 'app' ? m.settings_conflictApp({ name: c.name }) : m.settings_conflictGroup({ name: c.name })}</p>
        {/each}
      </div>
    {/if}

    <!-- Unsaved changes confirmation banner -->
    {#if confirmClose}
      <div class="flex items-center justify-between px-4 py-2 bg-yellow-600/20 border-b border-yellow-600/40">
        <span class="text-sm text-yellow-200">{m.settings_discardPrompt()}</span>
        <div class="flex gap-2">
          <button
            class="btn btn-secondary btn-sm"
            onclick={() => confirmClose = false}
          >{m.settings_keepEditing()}</button>
          <button
            class="btn btn-danger btn-sm"
            disabled={saving}
            onclick={confirmCloseDiscard}
          >{m.settings_discard()}</button>
        </div>
      </div>
    {/if}

    <!-- Tabs - scrollable on mobile -->
    <div class="flex border-b border-border flex-shrink-0 overflow-x-auto scrollbar-hide">
      {#each [
        { id: 'general', get label() { return m.settings_general(); } },
        { id: 'apps', get label() { return m.settings_appsAndGroups(); } },
        { id: 'theme', get label() { return m.settings_theme(); } },
        { id: 'keybindings', get label() { return m.settings_keybindings(); } },
        { id: 'security', get label() { return m.settings_security(); } },
        { id: 'gateway', get label() { return m.settings_gateway(); } },
        { id: 'discovery', get label() { return 'Discovery'; } },
        { id: 'about', get label() { return m.settings_about(); } }
      ] as tab (tab.id)}
        <button
          class="px-4 py-3 text-sm font-medium transition-colors border-b-2 whitespace-nowrap min-h-[48px]
                 {activeTab === tab.id
                   ? 'text-brand-400 border-brand-400'
                   : 'text-text-muted border-transparent hover:text-text-secondary hover:border-border'}"
          onclick={() => activeTab = tab.id as typeof activeTab}
        >
          {tab.label}
        </button>
      {/each}
    </div>

    <!-- Content -->
    <div class="flex-1 overflow-y-auto p-6">
      <!-- General Settings -->
      {#if activeTab === 'general'}
        <GeneralTab bind:localConfig bind:localApps onexport={handleExport} onimportselect={handleImportSelect} onopenhomeicon={() => openIconBrowser('homeIcon')} />


      <!-- Apps & Groups Settings -->
      {:else if activeTab === 'apps'}
        <AppsTab
          bind:dndGroups
          bind:dndGroupedApps
          localAppsCount={localApps.length}
          localGroupsCount={localConfig.groups.length}
          onstartEditApp={startEditApp}
          onstartEditGroup={startEditGroup}
          onshowAddApp={() => { appErrors = {}; addAppStep = 'choose'; addAppSearch = ''; showAddApp = true; }}
          onshowAddGroup={() => { groupErrors = {}; showAddGroup = true; }}
          onsyncGroupOrder={syncGroupOrder}
          onsyncAppOrder={syncAppOrder}
          ondiscoveryconfigure={() => { activeTab = 'discovery'; }}
          ondiscoveryscan={openDiscoverFromApps}
        />

      <!-- Theme Settings -->
      {:else if activeTab === 'theme'}
        <ThemeTab />

      <!-- Keybindings Settings -->
      {:else if activeTab === 'keybindings'}
        <KeybindingsEditor />

      <!-- Security Settings -->
      {:else if activeTab === 'security'}
        <SecurityTab {localConfig} hasUnsavedChanges={hasChanges} onmethodapplied={handleAuthMethodApplied} />

      <!-- Gateway sites -->
      {:else if activeTab === 'gateway'}
        <GatewayTab
          {configRevision}
          ondiscoveryconfigure={() => { activeTab = 'discovery'; }}
          ondiscoveryscan={openDiscoverFromGateway}
        />

      <!-- Discovery (Docker auto-discovery) -->
      {:else if activeTab === 'discovery'}
        <DiscoveryTab ontrackingchanged={handleDockerTrackingChanged} />

      <!-- About -->
      {:else if activeTab === 'about'}
        <AboutTab />
      {/if}
    </div>
  </div>
</div>

<!-- Discover-from-Docker modal. Lives outside the tabs so it can be
     opened from either the Apps tab or the Gateway tab without
     re-mounting. -->
<DiscoverModal
  bind:open={showDiscoverModal}
  mode={discoverMode}
  onclose={() => { showDiscoverModal = false; }}
  onimported={handleDiscoverImported}
/>

<!-- Add App Modal -->
{#if showAddApp}
  <div
    class="fixed inset-0 z-[60] flex items-center justify-center bg-black/50 p-4"
    transition:fade={{ duration: 100 }}
  >
    <div
      class="bg-bg-surface rounded-xl shadow-2xl w-full border border-border {addAppStep === 'choose' ? 'max-w-2xl' : 'max-w-lg'}"
      role="dialog"
      aria-modal="true"
      aria-labelledby="add-app-title"
      tabindex="-1"
      use:focusTrap
      in:fly={{ y: 10, duration: 150 }}
      out:fade={{ duration: 75 }}
    >
      <div class="flex items-center justify-between p-4 border-b border-border">
        <div class="flex items-center gap-2">
          {#if addAppStep === 'configure'}
            <button
              class="btn btn-ghost btn-icon"
              onclick={() => { addAppStep = 'choose'; addAppSearch = ''; }}
              aria-label={m.common_back()}
            >
              <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
              </svg>
            </button>
          {/if}
          <h3 id="add-app-title" class="text-lg font-semibold text-text-primary">{addAppStep === 'choose' ? m.settings_addApplication() : m.settings_configureApp({ appName: newApp.name || m.settings_appFallbackName() })}</h3>
        </div>
        <button
          class="btn btn-ghost btn-icon"
          onclick={() => showAddApp = false}
          aria-label={m.common_close()}
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      {#if addAppStep === 'choose'}
        <!-- Step 1: Choose from popular apps or custom -->
        <div class="p-4 max-h-[65vh] overflow-y-auto">
          <!-- Search -->
          <div class="mb-4">
            <input
              type="text"
              bind:value={addAppSearch}
              class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500 text-sm"
              placeholder={m.settings_searchApps()}
            />
          </div>

          <!-- Custom App card -->
          {#if !addAppSearch}
            <button
              class="w-full flex items-center gap-3 p-3 mb-4 rounded-lg border-2 border-dashed border-border-subtle hover:border-brand-500 hover:bg-bg-hover transition-colors text-start"
              onclick={startCustomApp}
            >
              <div class="w-10 h-10 rounded-lg bg-bg-elevated flex items-center justify-center flex-shrink-0">
                <svg class="w-5 h-5 text-text-muted" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
                </svg>
              </div>
              <div>
                <div class="text-sm font-medium text-text-primary">{m.settings_customApp()}</div>
                <div class="text-xs text-text-muted">{m.settings_customAppDesc()}</div>
              </div>
            </button>
          {/if}

          <!-- Popular apps by category -->
          {#each Object.entries(popularApps) as [category, templates] (category)}
            {@const filtered = addAppSearch ? templates.filter(t => t.name.toLowerCase().includes(addAppSearchLower) || t.description.toLowerCase().includes(addAppSearchLower)) : templates}
            {#if filtered.length > 0}
              <div class="mb-4">
                <h4 class="text-xs font-semibold text-text-muted uppercase tracking-wider mb-2">{category}</h4>
                <div class="grid grid-cols-2 gap-2">
                  {#each filtered as template (template.name)}
                    {@const alreadyAdded = localApps.some(a => a.name === template.name)}
                    <button
                      class="flex items-center gap-3 p-2.5 rounded-lg text-start transition-colors hover:bg-bg-hover {alreadyAdded ? 'bg-bg-elevated/30' : 'bg-bg-surface'}"
                      onclick={() => selectPopularApp(template)}
                      title={template.description}
                    >
                      <div class="flex-shrink-0">
                        <AppIcon icon={{ type: template.iconType || 'dashboard', name: template.icon, file: '', url: '', variant: 'svg', background: template.iconBackground }} name={template.name} color={template.color} size="sm" showBackground={localConfig.navigation.show_icon_background} />
                      </div>
                      <div class="min-w-0">
                        <div class="text-sm font-medium text-text-primary truncate flex items-center gap-1.5">
                          {template.name}
                          {#if alreadyAdded}
                            <span class="text-[10px] px-1.5 py-0.5 rounded-full bg-bg-overlay text-text-muted font-normal flex-shrink-0">{m.settings_added()}</span>
                          {/if}
                        </div>
                        <div class="text-xs text-text-disabled truncate">{template.description}</div>
                      </div>
                    </button>
                  {/each}
                </div>
              </div>
            {/if}
          {/each}

          {#if addAppSearch && Object.values(popularApps).every(templates => templates.every(t => !t.name.toLowerCase().includes(addAppSearchLower) && !t.description.toLowerCase().includes(addAppSearchLower)))}
            <div class="text-center py-6">
              <p class="text-text-muted text-sm mb-3">{m.settings_noMatchingApps()}</p>
              <button
                class="btn btn-primary btn-sm"
                onclick={startCustomApp}
              >
                {m.settings_addAsCustomApp()}
              </button>
            </div>
          {/if}
        </div>
      {:else}
        <!-- Step 2: Configure app details -->
        <div class="p-4 max-h-[60vh] overflow-y-auto">
          <AppForm
            bind:app={newApp}
            mode="create"
            groups={localConfig.groups}
            allApps={localApps}
            errors={appErrors}
            onopenicon={() => openIconBrowser('newApp')}
            ondefaultchange={(checked) => {
              if (checked) {
                localApps.forEach(a => a.default = false);
                localConfig.navigation.show_splash_on_startup = false;
              }
            }}
            onclearerror={(field) => { delete appErrors[field]; appErrors = appErrors; }}
          />
        </div>
        <div class="flex justify-end gap-2 p-4 border-t border-border">
          <button
            class="btn btn-secondary btn-sm"
            onclick={() => showAddApp = false}
          >
            {m.common_cancel()}
          </button>
          <button
            class="btn btn-primary btn-sm"
            onclick={addApp}
          >
            {m.settings_addApp()}
          </button>
        </div>
      {/if}
    </div>
  </div>
{/if}

<!-- Add Group Modal -->
{#if showAddGroup}
  <div
    class="fixed inset-0 z-[60] flex items-center justify-center bg-black/50 p-4"
    transition:fade={{ duration: 100 }}
  >
    <div
      class="bg-bg-surface rounded-xl shadow-2xl w-full max-w-md border border-border"
      in:fly={{ y: 10, duration: 150 }}
      out:fade={{ duration: 75 }}
    >
      <div class="flex items-center justify-between p-4 border-b border-border">
        <h3 class="text-lg font-semibold text-text-primary">{m.settings_addGroup()}</h3>
        <button
          class="btn btn-ghost btn-icon"
          onclick={() => showAddGroup = false}
          aria-label={m.common_close()}
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <div class="p-4 space-y-4">
        <div>
          <label for="group-name" class="block text-sm font-medium text-text-secondary mb-1">{m.settings_name()}</label>
          <input
            id="group-name"
            type="text"
            bind:value={newGroup.name}
            oninput={() => { delete groupErrors.name; groupErrors = groupErrors; }}
            class="w-full px-3 py-2 bg-bg-elevated border rounded-md text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500 {groupErrors.name ? 'border-red-500' : 'border-border-subtle'}"
            placeholder={m.settings_groupNamePlaceholder()}
          />
          {#if groupErrors.name}<p class="text-red-400 text-xs mt-1">{groupErrors.name}</p>{/if}
        </div>
        <div>
          <span class="block text-sm font-medium text-text-secondary mb-1">{m.settings_icon()}</span>
          <div class="flex items-center gap-3">
            <button type="button" class="cursor-pointer rounded hover:ring-2 hover:ring-brand-500 transition-all" onclick={() => openIconBrowser('newGroup')}>
              <AppIcon icon={newGroup.icon} name={newGroup.name || 'G'} color={newGroup.color} size="lg" />
            </button>
            <button
              class="btn btn-secondary btn-sm flex-1 text-start"
              onclick={() => openIconBrowser('newGroup')}
            >
              {iconLabel(newGroup.icon) || m.settings_chooseIcon()}
            </button>
          </div>
        </div>
        <div>
          <label for="group-color" class="block text-sm font-medium text-text-secondary mb-1">{m.settings_color()}</label>
          <div class="flex items-center gap-2">
            <input
              id="group-color"
              type="color"
              bind:value={newGroup.color}
              class="w-10 h-10 rounded cursor-pointer"
            />
            <input
              type="text"
              bind:value={newGroup.color}
              class="flex-1 px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500 text-sm"
            />
          </div>
        </div>
      </div>
      <div class="flex justify-end gap-2 p-4 border-t border-border">
        <button
          class="px-4 py-2 text-sm text-text-muted hover:text-text-primary rounded-md hover:bg-bg-hover"
          onclick={() => showAddGroup = false}
        >
          {m.common_cancel()}
        </button>
        <button
          class="btn btn-primary btn-sm"
          onclick={addGroup}
        >
          {m.settings_addGroup()}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Edit App Modal -->
{#if editingApp}
  <div
    class="fixed inset-0 z-[60] flex items-center justify-center bg-black/50 p-4"
    in:fade={{ duration: 100 }}
  >
    <div
      class="bg-bg-surface rounded-xl shadow-2xl w-full max-w-lg border border-border"
      role="dialog"
      aria-modal="true"
      aria-labelledby="edit-app-title"
      tabindex="-1"
      use:focusTrap
      in:fly={{ y: 10, duration: 150 }}
    >
      <div class="flex items-center justify-between p-4 border-b border-border">
        <h3 id="edit-app-title" class="text-lg font-semibold text-text-primary">{m.settings_editApp({ appName: editingApp.name })}</h3>
        <button
          class="btn btn-ghost btn-icon"
          onclick={cancelEditApp}
          aria-label={m.common_close()}
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <div class="p-4 max-h-[60vh] overflow-y-auto">
        <AppForm
          bind:app={editingApp}
          mode="edit"
          groups={localConfig.groups}
          allApps={localApps}
          errors={editAppErrors}
          onopenicon={() => openIconBrowser('editApp')}
          ondefaultchange={(checked) => {
            if (checked) {
              for (const apps of Object.values(dndGroupedApps)) {
                for (const a of apps) {
                  if (a !== editingApp) a.default = false;
                }
              }
              localConfig.navigation.show_splash_on_startup = false;
            }
          }}
          onclearerror={(field) => { delete editAppErrors[field]; editAppErrors = editAppErrors; }}
        />
      </div>
      <div class="flex justify-end gap-2 p-4 border-t border-border">
        <button
          class="btn btn-secondary btn-sm"
          onclick={cancelEditApp}
        >
          {m.common_cancel()}
        </button>
        <button
          class="btn btn-primary btn-sm"
          onclick={closeEditApp}
        >
          {m.common_done()}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Edit Group Modal -->
{#if editingGroup}
  <div
    class="fixed inset-0 z-[60] flex items-center justify-center bg-black/50 p-4"
    in:fade={{ duration: 100 }}
  >
    <div
      class="bg-bg-surface rounded-xl shadow-2xl w-full max-w-md border border-border"
      in:fly={{ y: 10, duration: 150 }}
    >
      <div class="flex items-center justify-between p-4 border-b border-border">
        <h3 class="text-lg font-semibold text-text-primary">{m.settings_editGroup({ groupName: editingGroup.name })}</h3>
        <button
          class="btn btn-ghost btn-icon"
          onclick={cancelEditGroup}
          aria-label={m.common_close()}
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <div class="p-4 space-y-4">
        <div>
          <label for="edit-group-name" class="block text-sm font-medium text-text-secondary mb-1">{m.settings_name()}</label>
          <input
            id="edit-group-name"
            type="text"
            bind:value={editingGroup.name}
            oninput={() => { delete editGroupErrors.name; editGroupErrors = editGroupErrors; }}
            class="w-full px-3 py-2 bg-bg-elevated border rounded-md text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500 {editGroupErrors.name ? 'border-red-500' : 'border-border-subtle'}"
          />
          {#if editGroupErrors.name}<p class="text-red-400 text-xs mt-1">{editGroupErrors.name}</p>{/if}
        </div>
        <div>
          <span class="block text-sm font-medium text-text-secondary mb-1">{m.settings_icon()}</span>
          <div class="flex items-center gap-3">
            <button type="button" class="cursor-pointer rounded hover:ring-2 hover:ring-brand-500 transition-all" onclick={() => openIconBrowser('editGroup')}>
              <AppIcon icon={editingGroup.icon} name={editingGroup.name} color={editingGroup.color} size="lg" />
            </button>
            <div class="flex-1">
              <button
                class="btn btn-secondary btn-sm w-full text-start"
                onclick={() => openIconBrowser('editGroup')}
              >
                {iconLabel(editingGroup.icon) || m.settings_chooseIcon()}
              </button>
              <p class="text-xs text-text-muted mt-1">
                {editingGroup.icon?.type === 'dashboard' ? m.settings_dashboardIcon() : editingGroup.icon?.type || m.settings_noIconSet()}
              </p>
            </div>
          </div>
          {#if editingGroup?.icon?.type === 'lucide'}
            <div class="flex items-center gap-4 mt-2">
              <label class="flex items-center gap-2 text-xs text-text-muted">
                {m.settings_iconColor()}
                <input type="color" value={editingGroup!.icon.color || '#ffffff'} oninput={(e) => editingGroup!.icon.color = (e.target as HTMLInputElement).value} class="w-8 h-8 rounded cursor-pointer" />
                {#if editingGroup!.icon.color}
                  <button class="text-text-disabled hover:text-text-secondary" onclick={() => editingGroup!.icon.color = ''} title={m.settings_resetToDefault()}>&times;</button>
                {/if}
              </label>
              <label class="flex items-center gap-2 text-xs text-text-muted">
                {m.settings_background()}
                <input type="color" value={editingGroup!.icon.background || editingGroup!.color || '#374151'} oninput={(e) => editingGroup!.icon.background = (e.target as HTMLInputElement).value} class="w-8 h-8 rounded cursor-pointer" />
                <button class="text-text-disabled hover:text-text-secondary text-xs" onclick={() => editingGroup!.icon.background = 'transparent'} title={m.settings_transparent()}>{m.settings_none()}</button>
                {#if editingGroup!.icon.background}
                  <button class="text-text-disabled hover:text-text-secondary" onclick={() => editingGroup!.icon.background = ''} title={m.settings_resetToGroupColor()}>&times;</button>
                {/if}
              </label>
            </div>
          {/if}
        </div>
        <div>
          <label for="edit-group-color" class="block text-sm font-medium text-text-secondary mb-1">{m.settings_color()}</label>
          <div class="flex items-center gap-2">
            <input
              id="edit-group-color"
              type="color"
              bind:value={editingGroup.color}
              class="w-10 h-10 rounded cursor-pointer"
            />
            <input
              type="text"
              bind:value={editingGroup.color}
              class="flex-1 px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500 text-sm"
            />
          </div>
        </div>
      </div>
      <div class="flex justify-end gap-2 p-4 border-t border-border">
        <button
          class="btn btn-secondary btn-sm"
          onclick={cancelEditGroup}
        >
          {m.common_cancel()}
        </button>
        <button
          class="btn btn-primary btn-sm"
          onclick={closeEditGroup}
        >
          {m.common_done()}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Icon Browser Modal -->
{#if showIconBrowser}
  <div
    class="fixed inset-0 z-[70] flex items-center justify-center bg-black/50 {isMobile ? 'p-0' : 'p-4'}"
    transition:fade={{ duration: 100 }}
  >
    <div
      class="bg-bg-surface shadow-2xl w-full border border-border
             {isMobile ? 'h-full max-h-full rounded-none' : 'rounded-xl max-w-3xl'}"
      in:fly={{ y: 10, duration: 150 }}
      out:fade={{ duration: 75 }}
    >
      <div class="flex items-center justify-between p-4 border-b border-border">
        <h3 class="text-lg font-semibold text-text-primary">{m.settings_selectIcon()}</h3>
        <button
          class="btn btn-ghost btn-icon"
          onclick={() => { showIconBrowser = false; iconBrowserTarget = null; }}
          aria-label={m.common_close()}
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <IconBrowser
        selectedIcon={iconBrowserIcon?.type === 'dashboard' || iconBrowserIcon?.type === 'lucide' ? iconBrowserIcon.name : ''}
        selectedVariant={iconBrowserIcon?.variant || 'svg'}
        selectedType={iconBrowserIcon?.type === 'dashboard' || iconBrowserIcon?.type === 'lucide' ? iconBrowserIcon.type : 'dashboard'}
        onselect={handleIconSelect}
        onclose={() => { showIconBrowser = false; iconBrowserTarget = null; }}
      />
    </div>
  </div>
{/if}

<!-- Import Confirmation Modal -->
{#if showImportConfirm && pendingImport}
  <div
    class="fixed inset-0 z-[70] flex items-center justify-center bg-black/50 p-4"
    in:fade={{ duration: 100 }}
  >
    <div
      class="bg-bg-surface rounded-xl shadow-2xl w-full max-w-md border border-border"
      in:fly={{ y: 10, duration: 150 }}
    >
      <div class="flex items-center justify-between p-4 border-b border-border">
        <h3 class="text-lg font-semibold text-text-primary">{m.settings_importConfig()}</h3>
        <button
          class="btn btn-ghost btn-icon"
          onclick={cancelImport}
          aria-label={m.common_close()}
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <div class="p-4 space-y-4">
        <p class="text-text-secondary">
          {m.settings_importDesc()}
        </p>
        <div class="bg-bg-hover rounded-lg p-3 text-sm">
          <div class="text-text-muted">{m.settings_importPreview()}</div>
          <div class="text-text-primary font-medium">{pendingImport.title}</div>
          <div class="text-text-muted text-xs mt-1">
            {m.settings_importSummary({ appCount: pendingImport.apps.length, groupCount: pendingImport.groups.length })}
          </div>
        </div>
        <p class="text-yellow-400 text-sm flex items-center gap-2">
          <svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
          {m.settings_importWarning()}
        </p>
      </div>
      <div class="flex justify-end gap-2 p-4 border-t border-border">
        <button
          class="btn btn-secondary btn-sm"
          onclick={cancelImport}
        >
          {m.common_cancel()}
        </button>
        <button
          class="btn btn-primary btn-sm"
          onclick={applyImport}
        >
          {m.settings_import()}
        </button>
      </div>
    </div>
  </div>
{/if}
</div>

<style>
  /* Theme-aware overrides: map Tailwind's hardcoded grays to CSS custom properties.
     This makes the Settings UI adapt to light, dark, and custom themes instead of
     being locked to dark-mode gray values. */

  /* Surface backgrounds */
  .settings :global(.bg-bg-surface) {
    background-color: var(--bg-surface) !important;
  }
  .settings :global(.bg-bg-elevated) {
    background-color: var(--bg-elevated) !important;
  }
  .settings :global([class*="bg-bg-elevated/"]) {
    background-color: var(--bg-hover) !important;
  }
  .settings :global(.bg-bg-overlay) {
    background-color: var(--bg-overlay) !important;
  }

  /* Borders */
  .settings :global(.border-border) {
    border-color: var(--border-default) !important;
  }
  .settings :global(.border-border-subtle) {
    border-color: var(--border-subtle) !important;
  }
  .settings :global(.border-border-strong) {
    border-color: var(--border-strong) !important;
  }

  /* Text */
  .settings :global(.text-text-primary) {
    color: var(--text-primary) !important;
  }
  .settings :global(.text-text-secondary) {
    color: var(--text-secondary) !important;
  }
  .settings :global(.text-text-muted) {
    color: var(--text-muted) !important;
  }
  .settings :global(.text-text-disabled) {
    color: var(--text-disabled) !important;
  }

  /* Hover backgrounds */
  .settings :global(.hover\:bg-bg-elevated:hover) {
    background-color: var(--bg-hover) !important;
  }
  .settings :global(.hover\:bg-bg-overlay:hover) {
    background-color: var(--bg-active) !important;
  }
  .settings :global(.hover\:bg-bg-active:hover) {
    background-color: var(--bg-active) !important;
  }

  /* Hover text */
  .settings :global(.hover\:text-text-primary:hover) {
    color: var(--text-primary) !important;
  }
  .settings :global(.hover\:text-text-secondary:hover) {
    color: var(--text-secondary) !important;
  }

  /* Hover borders */
  .settings :global(.hover\:border-border-subtle:hover) {
    border-color: var(--border-default) !important;
  }
  .settings :global(.hover\:border-border-strong:hover) {
    border-color: var(--border-strong) !important;
  }

  /* App status indicators (global so they survive DnD reparenting to body) */
  :global(.app-indicator) {
    display: inline-flex;
    align-items: center;
    gap: 3px;
    font-size: 0.875rem;
    line-height: 1;
    padding: 4px 8px;
    border-radius: 4px;
    background: var(--bg-elevated);
    color: var(--text-muted);
    white-space: nowrap;
    flex-shrink: 0;
  }

  /* Drop indicator for intra-group drag-and-drop */
  .settings :global(.drop-indicator) {
    height: 2px;
    background: var(--accent-primary);
    border-radius: 1px;
    margin: 0 8px;
    box-shadow: 0 0 6px var(--accent-primary);
  }


  /* Range inputs: use theme accent color */
  .settings :global(input[type="range"]) {
    accent-color: var(--accent-primary);
  }

  /* Focus rings: use theme accent instead of hardcoded brand-500 */
  .settings :global(*:focus) {
    --tw-ring-color: var(--accent-primary) !important;
  }
</style>
