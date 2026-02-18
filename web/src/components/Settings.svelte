<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { fade, fly } from 'svelte/transition';
  import type { App, Config, Group } from '$lib/types';
  import { openModes } from '$lib/constants';
  import IconBrowser from './IconBrowser.svelte';
  import AppIcon from './AppIcon.svelte';
  import KeybindingsEditor from './KeybindingsEditor.svelte';
  import AboutTab from './settings/AboutTab.svelte';
  import AppsTab from './settings/AppsTab.svelte';
  import GeneralTab from './settings/GeneralTab.svelte';
  import SecurityTab from './settings/SecurityTab.svelte';
  import ThemeTab from './settings/ThemeTab.svelte';
  import { get } from 'svelte/store';
  import { selectedFamily, variantMode, setThemeFamily, setVariantMode } from '$lib/themeStore';
  import { isMobileViewport } from '$lib/useSwipe';
  import { exportConfig, parseImportedConfig, type ImportedConfig } from '$lib/api';
  import { toasts } from '$lib/toastStore';
  import { getKeybindingsForConfig } from '$lib/keybindingsStore';
  import { appSchema, groupSchema, extractErrors } from '$lib/schemas';
  import { popularApps, templateToApp, type PopularAppTemplate } from '$lib/popularApps';

  let {
    config,
    apps,
    initialTab = 'general',
    onclose,
    onsave,
  }: {
    config: Config;
    apps: App[];
    initialTab?: 'general' | 'apps' | 'theme' | 'keybindings' | 'security' | 'about';
    onclose?: () => void;
    onsave?: (config: Config) => void;
  } = $props();

  // Exported: returns true if Escape was consumed by closing an inner sub-modal.
  export function handleEscape(): boolean {
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
    return () => window.removeEventListener('resize', handleResize);
  });

  // Active tab
  let activeTab = $state(untrack(() => initialTab ?? 'general'));

  // Local copy of config for editing
  let localConfig = $state(untrack(() => JSON.parse(JSON.stringify(config)) as Config));
  let localApps = $state(untrack(() => JSON.parse(JSON.stringify(apps)) as App[]));

  // Icon browser state
  let showIconBrowser = $state(false);
  let iconBrowserTarget = $state<'newApp' | 'editApp' | 'newGroup' | 'editGroup' | null>(null);

  // Track keybindings changes
  let keybindingsChanged = $state(false);

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

  // Import/export state
  let showImportConfirm = $state(false);
  let pendingImport = $state<ImportedConfig | null>(null);

  // New app/group templates
  const newAppTemplate: App = {
    name: '',
    url: '',
    icon: { type: 'dashboard', name: '', file: '', url: '', variant: '' },
    color: '#22c55e',
    group: '',
    order: 0,
    enabled: true,
    default: false,
    open_mode: 'iframe',
    proxy: false,
    scale: 1,
  };

  const newGroupTemplate: Group = {
    name: '',
    icon: { type: 'dashboard', name: '', file: '', url: '', variant: '' },
    color: '#3498db',
    order: 0,
    expanded: true
  };

  let newApp = $state({ ...newAppTemplate });
  let newGroup = $state({ ...newGroupTemplate });

  // Icon browser: derive the current target's icon for pre-populating the browser
  let iconBrowserIcon = $derived(
    iconBrowserTarget === 'editApp' ? editingApp?.icon :
    iconBrowserTarget === 'editGroup' ? editingGroup?.icon :
    iconBrowserTarget === 'newApp' ? newApp.icon :
    iconBrowserTarget === 'newGroup' ? newGroup.icon : null
  );

  // Validation error state
  let appErrors = $state<Record<string, string>>({});
  let groupErrors = $state<Record<string, string>>({});

  // Assign stable `id` fields for svelte-dnd-action (must be done once, before building dnd arrays)
  untrack(() => localApps).forEach(a => { (a as App & Record<string, unknown>).id = a.name; });
  untrack(() => localConfig).groups.forEach(g => { (g as Group & Record<string, unknown>).id = g.name; });

  // Snapshot taken AFTER id fields are added, so hasChanges starts as false
  const initialConfigSnapshot = untrack(() => JSON.stringify(localConfig));
  const initialAppsSnapshot = untrack(() => JSON.stringify(localApps));

  // Snapshot theme so we can revert on close without save
  const initialFamily = untrack(() => get(selectedFamily));
  const initialVariant = untrack(() => get(variantMode));

  // Track if changes have been made
  let hasChanges = $derived(JSON.stringify(localConfig) !== initialConfigSnapshot ||
                  JSON.stringify(localApps) !== initialAppsSnapshot ||
                  keybindingsChanged ||
                  $selectedFamily !== initialFamily ||
                  $variantMode !== initialVariant);

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
      allApps.forEach(a => { (a as App & Record<string, unknown>).id = a.name; });
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

  function handleSave() {
    // Update config with local changes
    localConfig.apps = localApps;
    // Capture current theme from stores into config
    localConfig.theme = {
      family: get(selectedFamily),
      variant: get(variantMode)
    };
    // Include keybindings if changed
    if (keybindingsChanged) {
      localConfig.keybindings = getKeybindingsForConfig();
    }
    onsave?.(localConfig);
    onclose?.();
  }

  // Inline confirmation state
  let confirmClose = $state(false);

  function handleClose() {
    if (hasChanges) {
      confirmClose = true;
      return;
    }
    revertTheme();
    onclose?.();
  }

  function confirmCloseDiscard() {
    confirmClose = false;
    revertTheme();
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

  function addApp() {
    const result = appSchema.safeParse(newApp);
    if (!result.success) {
      appErrors = extractErrors(result);
      return;
    }
    appErrors = {};
    newApp.order = localApps.length;
    const app: App & Record<string, unknown> = { ...newApp };
    app.id = app.name;
    localApps = [...localApps, app];
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
    const group: Group & Record<string, unknown> = { ...newGroup };
    group.id = group.name;
    localConfig.groups = [...localConfig.groups, group];
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
      editAppErrors = {};
      (editingApp as App & Record<string, unknown>).id = editingApp.name;
      // Sync DnD app changes back to localApps before rebuilding
      const allApps: App[] = [];
      for (const apps of Object.values(dndGroupedApps)) {
        allApps.push(...apps);
      }
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
      (editingGroup as Group & Record<string, unknown>).id = editingGroup.name;
      // Sync DnD group changes back to localConfig before rebuilding
      localConfig.groups = [...dndGroups];
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
    toasts.success('Configuration exported');
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
      toasts.error(err instanceof Error ? err.message : 'Failed to parse config file');
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
    localApps.forEach(a => { (a as App & Record<string, unknown>).id = a.name; });
    localConfig.groups.forEach(g => { (g as Group & Record<string, unknown>).id = g.name; });
    rebuildDndArrays();

    showImportConfirm = false;
    pendingImport = null;
    toasts.success('Configuration imported - save to apply changes');
  }

  function cancelImport() {
    showImportConfirm = false;
    pendingImport = null;
  }

  function handleIconSelect(detail: { name: string; variant: string; type: string }) {
    const { name, variant, type } = detail;
    const iconData = { type: type as 'dashboard' | 'lucide' | 'custom', name, variant, file: '', url: '', color: '', background: '' };

    if (iconBrowserTarget === 'newApp') {
      newApp = { ...newApp, icon: iconData };
    } else if (iconBrowserTarget === 'editApp' && editingApp) {
      // Replace in dndGroupedApps and editingApp with the same new object
      const updated = { ...editingApp, icon: iconData };
      for (const apps of Object.values(dndGroupedApps)) {
        const idx = apps.indexOf(editingApp);
        if (idx !== -1) { apps[idx] = updated; break; }
      }
      editingApp = updated;
    } else if (iconBrowserTarget === 'newGroup') {
      newGroup = { ...newGroup, icon: iconData };
    } else if (iconBrowserTarget === 'editGroup' && editingGroup) {
      // Replace in dndGroups and editingGroup with the same new object
      const updated = { ...editingGroup, icon: iconData };
      const idx = dndGroups.indexOf(editingGroup);
      if (idx !== -1) dndGroups[idx] = updated;
      editingGroup = updated;
    }
    showIconBrowser = false;
    iconBrowserTarget = null;
  }

  function openIconBrowser(target: 'newApp' | 'editApp' | 'newGroup' | 'editGroup') {
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
      <h2 class="text-lg font-semibold text-text-primary">Settings</h2>
      <div class="flex items-center gap-2">
        {#if hasChanges}
          <span class="text-xs text-yellow-400">Unsaved changes</span>
        {/if}
        <button
          class="btn btn-primary btn-sm disabled:opacity-50"
          disabled={!hasChanges}
          onclick={handleSave}
        >
          Save Changes
        </button>
        <button
          class="btn btn-ghost btn-icon btn-sm"
          onclick={handleClose}
          aria-label="Close settings"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    </div>

    <!-- Unsaved changes confirmation banner -->
    {#if confirmClose}
      <div class="flex items-center justify-between px-4 py-2 bg-yellow-600/20 border-b border-yellow-600/40">
        <span class="text-sm text-yellow-200">You have unsaved changes. Discard?</span>
        <div class="flex gap-2">
          <button
            class="btn btn-secondary btn-sm"
            onclick={() => confirmClose = false}
          >Keep Editing</button>
          <button
            class="btn btn-danger btn-sm"
            onclick={confirmCloseDiscard}
          >Discard</button>
        </div>
      </div>
    {/if}

    <!-- Tabs - scrollable on mobile -->
    <div class="flex border-b border-border flex-shrink-0 overflow-x-auto scrollbar-hide">
      {#each [
        { id: 'general', label: 'General' },
        { id: 'apps', label: 'Apps & Groups' },
        { id: 'theme', label: 'Theme' },
        { id: 'keybindings', label: 'Keybindings' },
        { id: 'security', label: 'Security' },
        { id: 'about', label: 'About' }
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
        <GeneralTab bind:localConfig bind:localApps onexport={handleExport} onimportselect={handleImportSelect} />


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
        />

      <!-- Theme Settings -->
      {:else if activeTab === 'theme'}
        <ThemeTab />
      {/if}

      <!-- Keybindings Settings -->
      {#if activeTab === 'keybindings'}
        <KeybindingsEditor onchange={() => keybindingsChanged = true} />
      {/if}

      <!-- Security Settings -->
      {#if activeTab === 'security'}
        <SecurityTab {localConfig} />
      {/if}

      <!-- About -->
      {#if activeTab === 'about'}
        <AboutTab />
      {/if}
    </div>
  </div>
</div>

<!-- Add App Modal -->
{#if showAddApp}
  <div
    class="fixed inset-0 z-[60] flex items-center justify-center bg-black/50 p-4"
    transition:fade={{ duration: 100 }}
  >
    <div
      class="bg-bg-surface rounded-xl shadow-2xl w-full border border-border {addAppStep === 'choose' ? 'max-w-2xl' : 'max-w-lg'}"
      in:fly={{ y: 10, duration: 150 }}
      out:fade={{ duration: 75 }}
    >
      <div class="flex items-center justify-between p-4 border-b border-border">
        <div class="flex items-center gap-2">
          {#if addAppStep === 'configure'}
            <button
              class="btn btn-ghost btn-icon"
              onclick={() => { addAppStep = 'choose'; addAppSearch = ''; }}
              aria-label="Back"
            >
              <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
              </svg>
            </button>
          {/if}
          <h3 class="text-lg font-semibold text-text-primary">{addAppStep === 'choose' ? 'Add Application' : 'Configure ' + (newApp.name || 'App')}</h3>
        </div>
        <button
          class="btn btn-ghost btn-icon"
          onclick={() => showAddApp = false}
          aria-label="Close"
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
              placeholder="Search apps..."
            />
          </div>

          <!-- Custom App card -->
          {#if !addAppSearch}
            <button
              class="w-full flex items-center gap-3 p-3 mb-4 rounded-lg border-2 border-dashed border-border-subtle hover:border-brand-500 hover:bg-bg-hover transition-colors text-left"
              onclick={startCustomApp}
            >
              <div class="w-10 h-10 rounded-lg bg-bg-elevated flex items-center justify-center flex-shrink-0">
                <svg class="w-5 h-5 text-text-muted" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
                </svg>
              </div>
              <div>
                <div class="text-sm font-medium text-text-primary">Custom App</div>
                <div class="text-xs text-text-muted">Add any app with a custom URL and icon</div>
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
                      class="flex items-center gap-3 p-2.5 rounded-lg text-left transition-colors hover:bg-bg-hover {alreadyAdded ? 'bg-bg-elevated/30' : 'bg-bg-surface'}"
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
                            <span class="text-[10px] px-1.5 py-0.5 rounded-full bg-bg-overlay text-text-muted font-normal flex-shrink-0">added</span>
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
              <p class="text-text-muted text-sm mb-3">No matching apps found</p>
              <button
                class="btn btn-primary btn-sm"
                onclick={startCustomApp}
              >
                Add as Custom App
              </button>
            </div>
          {/if}
        </div>
      {:else}
        <!-- Step 2: Configure app details -->
        <div class="p-4 space-y-4 max-h-[60vh] overflow-y-auto">
          <div>
            <label for="app-name" class="block text-sm font-medium text-text-secondary mb-1">Name</label>
            <input
              id="app-name"
              type="text"
              bind:value={newApp.name}
              oninput={() => { delete appErrors.name; appErrors = appErrors; }}
              class="w-full px-3 py-2 bg-bg-elevated border rounded-md text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500 {appErrors.name ? 'border-red-500' : 'border-border-subtle'}"
              placeholder="My App"
            />
            {#if appErrors.name}<p class="text-red-400 text-xs mt-1">{appErrors.name}</p>{/if}
          </div>
          <div>
            <label for="app-url" class="block text-sm font-medium text-text-secondary mb-1">URL</label>
            <input
              id="app-url"
              type="url"
              bind:value={newApp.url}
              oninput={() => { delete appErrors.url; appErrors = appErrors; }}
              class="w-full px-3 py-2 bg-bg-elevated border rounded-md text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500 {appErrors.url ? 'border-red-500' : 'border-border-subtle'}"
              placeholder="http://localhost:8080"
            />
            {#if appErrors.url}<p class="text-red-400 text-xs mt-1">{appErrors.url}</p>{/if}
          </div>
          <div>
            <span class="block text-sm font-medium text-text-secondary mb-1">Icon</span>
            <div class="flex items-center gap-3">
              <button type="button" class="cursor-pointer rounded hover:ring-2 hover:ring-brand-500 transition-all" onclick={() => openIconBrowser('newApp')}>
                <AppIcon icon={newApp.icon} name={newApp.name || 'App'} color={newApp.color} size="lg" />
              </button>
              <button
                class="btn btn-secondary btn-sm flex-1 text-left"
                onclick={() => openIconBrowser('newApp')}
              >
                {newApp.icon?.name || 'Choose icon...'}
              </button>
            </div>
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div>
              <label for="app-color" class="block text-sm font-medium text-text-secondary mb-1">App color</label>
              <div class="flex items-center gap-2">
                <input
                  id="app-color"
                  type="color"
                  bind:value={newApp.color}
                  class="w-10 h-10 rounded cursor-pointer"
                />
                <input
                  type="text"
                  bind:value={newApp.color}
                  class="flex-1 px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500 text-sm"
                />
              </div>
            </div>
            <div>
              <label for="app-group" class="block text-sm font-medium text-text-secondary mb-1">Group</label>
              <select
                id="app-group"
                bind:value={newApp.group}
                class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500"
              >
                <option value="">No group</option>
                {#each localConfig.groups as group (group.name)}
                  <option value={group.name}>{group.name}</option>
                {/each}
              </select>
            </div>
          </div>
          <div>
            <label for="app-mode" class="block text-sm font-medium text-text-secondary mb-1">
              Open Mode
              <span class="help-trigger relative ml-1 inline-block align-middle">
                <svg class="w-3.5 h-3.5 text-text-disabled cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
                </svg>
                <span class="help-tooltip">
                  <b>Embedded</b> — loads inside Muximux in an iframe. Best for most apps.<br/>
                  <b>New Tab</b> — opens in a separate browser tab.<br/>
                  <b>New Window</b> — opens in a popup window.
                </span>
              </span>
            </label>
            <select
              id="app-mode"
              bind:value={newApp.open_mode}
              class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500"
            >
              {#each openModes as mode (mode.value)}
                <option value={mode.value}>{mode.label}</option>
              {/each}
            </select>
          </div>
          <div>
            <label for="app-scale" class="block text-sm font-medium text-text-secondary mb-1">
              Scale: {Math.round(newApp.scale * 100)}%
            </label>
            <input
              id="app-scale"
              type="range"
              min="0.5"
              max="2"
              step="0.05"
              bind:value={newApp.scale}
              class="w-full"
            />
          </div>
          <div class="space-y-2">
            <label class="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                bind:checked={newApp.enabled}
                class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
              />
              <div>
                <span class="text-sm text-text-primary">Enabled</span>
                <p class="text-xs text-text-muted">Show this app in the navigation</p>
              </div>
            </label>
            <label class="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                bind:checked={newApp.default}
                class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
              />
              <div>
                <span class="text-sm text-text-primary">Default app</span>
                <p class="text-xs text-text-muted">Automatically load this app on startup instead of the overview</p>
              </div>
            </label>
            <label class="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                bind:checked={newApp.proxy}
                class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
              />
              <div>
                <span class="text-sm text-text-primary">Use reverse proxy</span>
                <p class="text-xs text-text-muted">Route traffic through the built-in Caddy proxy to avoid CORS and mixed-content issues</p>
              </div>
            </label>
            <label class="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                bind:checked={newApp.force_icon_background}
                class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
              />
              <div>
                <span class="text-sm text-text-primary">Force icon background</span>
                <p class="text-xs text-text-muted">Show background even when global icon backgrounds are off</p>
              </div>
            </label>
            <label class="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                bind:checked={newApp.icon.invert}
                class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
              />
              <div>
                <span class="text-sm text-text-primary">Invert icon colors</span>
                <p class="text-xs text-text-muted">Flip dark icons to light and vice versa</p>
              </div>
            </label>
          </div>
          <div>
            <label for="new-app-min-role" class="block text-sm font-medium text-text-secondary mb-1">Minimum Role</label>
            <select
              id="new-app-min-role"
              bind:value={newApp.min_role}
              class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500"
            >
              <option value="">Everyone (default)</option>
              <option value="power-user">Power User</option>
              <option value="admin">Admin</option>
            </select>
            <p class="text-xs text-text-muted mt-1">Users below this role won't see this app</p>
          </div>
        </div>
        <div class="flex justify-end gap-2 p-4 border-t border-border">
          <button
            class="px-4 py-2 text-sm text-text-muted hover:text-text-primary rounded-md hover:bg-bg-hover"
            onclick={() => showAddApp = false}
          >
            Cancel
          </button>
          <button
            class="btn btn-primary btn-sm"
            onclick={addApp}
          >
            Add App
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
        <h3 class="text-lg font-semibold text-text-primary">Add Group</h3>
        <button
          class="btn btn-ghost btn-icon"
          onclick={() => showAddGroup = false}
          aria-label="Close"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <div class="p-4 space-y-4">
        <div>
          <label for="group-name" class="block text-sm font-medium text-text-secondary mb-1">Name</label>
          <input
            id="group-name"
            type="text"
            bind:value={newGroup.name}
            oninput={() => { delete groupErrors.name; groupErrors = groupErrors; }}
            class="w-full px-3 py-2 bg-bg-elevated border rounded-md text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500 {groupErrors.name ? 'border-red-500' : 'border-border-subtle'}"
            placeholder="Media"
          />
          {#if groupErrors.name}<p class="text-red-400 text-xs mt-1">{groupErrors.name}</p>{/if}
        </div>
        <div>
          <span class="block text-sm font-medium text-text-secondary mb-1">Icon</span>
          <div class="flex items-center gap-3">
            <button type="button" class="cursor-pointer rounded hover:ring-2 hover:ring-brand-500 transition-all" onclick={() => openIconBrowser('newGroup')}>
              <AppIcon icon={newGroup.icon} name={newGroup.name || 'G'} color={newGroup.color} size="lg" />
            </button>
            <button
              class="btn btn-secondary btn-sm flex-1 text-left"
              onclick={() => openIconBrowser('newGroup')}
            >
              {newGroup.icon?.name || 'Choose icon...'}
            </button>
          </div>
        </div>
        <div>
          <label for="group-color" class="block text-sm font-medium text-text-secondary mb-1">Color</label>
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
          Cancel
        </button>
        <button
          class="btn btn-primary btn-sm"
          onclick={addGroup}
        >
          Add Group
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Edit App Modal -->
{#if editingApp}
  <div
    class="fixed inset-0 z-[60] flex items-center justify-center bg-black/50 p-4"
    transition:fade={{ duration: 100 }}
  >
    <div
      class="bg-bg-surface rounded-xl shadow-2xl w-full max-w-lg border border-border"
      in:fly={{ y: 10, duration: 150 }}
      out:fade={{ duration: 75 }}
    >
      <div class="flex items-center justify-between p-4 border-b border-border">
        <h3 class="text-lg font-semibold text-text-primary">Edit {editingApp.name}</h3>
        <button
          class="btn btn-ghost btn-icon"
          onclick={cancelEditApp}
          aria-label="Close"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <div class="p-4 space-y-4 max-h-[60vh] overflow-y-auto">
        <!-- Identity -->
        <div>
          <label for="edit-app-name" class="block text-sm font-medium text-text-secondary mb-1">
            Name
            <span class="help-trigger relative ml-1 inline-block align-middle">
              <svg class="w-3.5 h-3.5 text-text-disabled cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
              </svg>
              <span class="help-tooltip">
                Displayed in the navigation bar and page title. Also used as a unique identifier — renaming an app creates a new proxy route.
              </span>
            </span>
          </label>
          <input
            id="edit-app-name"
            type="text"
            bind:value={editingApp.name}
            oninput={() => { delete editAppErrors.name; editAppErrors = editAppErrors; }}
            class="w-full px-3 py-2 bg-bg-elevated border rounded-md text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500 {editAppErrors.name ? 'border-red-500' : 'border-border-subtle'}"
          />
          {#if editAppErrors.name}<p class="text-red-400 text-xs mt-1">{editAppErrors.name}</p>{/if}
        </div>
        <div>
          <label for="edit-app-url" class="block text-sm font-medium text-text-secondary mb-1">
            URL
            <span class="help-trigger relative ml-1 inline-block align-middle">
              <svg class="w-3.5 h-3.5 text-text-disabled cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
              </svg>
              <span class="help-tooltip">
                The full address of the application. Used to load the app in an iframe, or as the link when opened in a new tab/window.
              </span>
            </span>
          </label>
          <input
            id="edit-app-url"
            type="url"
            bind:value={editingApp.url}
            oninput={() => { delete editAppErrors.url; editAppErrors = editAppErrors; }}
            class="w-full px-3 py-2 bg-bg-elevated border rounded-md text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500 {editAppErrors.url ? 'border-red-500' : 'border-border-subtle'}"
          />
          {#if editAppErrors.url}<p class="text-red-400 text-xs mt-1">{editAppErrors.url}</p>{/if}
        </div>
        <div>
          <span class="block text-sm font-medium text-text-secondary mb-1">Icon</span>
          <div class="flex items-center gap-3">
            <button type="button" class="cursor-pointer rounded hover:ring-2 hover:ring-brand-500 transition-all" onclick={() => openIconBrowser('editApp')}>
              <AppIcon icon={editingApp.icon} name={editingApp.name} color={editingApp.color} size="lg" />
            </button>
            <div class="flex-1">
              <button
                class="btn btn-secondary btn-sm w-full text-left"
                onclick={() => openIconBrowser('editApp')}
              >
                {editingApp.icon?.name || 'Choose icon...'}
              </button>
              <p class="text-xs text-text-muted mt-1">
                {editingApp.icon?.type === 'dashboard' ? 'Dashboard Icon' : editingApp.icon?.type || 'No icon set'}
              </p>
            </div>
          </div>
          <div class="flex items-center gap-4 mt-2">
            {#if editingApp.icon?.type === 'lucide'}
              <label class="flex items-center gap-2 text-xs text-text-muted">
                Icon color
                <input type="color" value={editingApp!.icon.color || '#ffffff'} oninput={(e) => editingApp!.icon.color = (e.target as HTMLInputElement).value} class="w-8 h-8 rounded cursor-pointer" />
                {#if editingApp!.icon.color}
                  <button class="text-text-disabled hover:text-text-secondary" onclick={() => editingApp!.icon.color = ''} title="Reset to theme default">&times;</button>
                {/if}
              </label>
            {/if}
            <label class="flex items-center gap-2 text-xs text-text-muted">
              Icon background
              <input type="color" value={editingApp!.icon.background || editingApp!.color || '#374151'} oninput={(e) => editingApp!.icon.background = (e.target as HTMLInputElement).value} class="w-8 h-8 rounded cursor-pointer" />
              <button class="text-text-disabled hover:text-text-secondary text-xs" onclick={() => editingApp!.icon.background = 'transparent'} title="Transparent">none</button>
              {#if editingApp!.icon.background}
                <button class="text-text-disabled hover:text-text-secondary" onclick={() => editingApp!.icon.background = ''} title="Reset to app color">&times;</button>
              {/if}
            </label>
          </div>
        </div>
        <div>
          <label for="edit-app-color" class="block text-sm font-medium text-text-secondary mb-1">
            App color
            <span class="help-trigger relative ml-1 inline-block align-middle">
              <svg class="w-3.5 h-3.5 text-text-disabled cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
              </svg>
              <span class="help-tooltip">
                The app's accent color — used for the active tab indicator and sidebar highlight when "Show App Colors" is enabled. Also used as the default icon background unless overridden above.
              </span>
            </span>
          </label>
          <div class="flex items-center gap-2">
            <input
              id="edit-app-color"
              type="color"
              bind:value={editingApp.color}
              class="w-10 h-10 rounded cursor-pointer"
            />
            <input
              type="text"
              bind:value={editingApp.color}
              class="flex-1 px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500 text-sm"
            />
          </div>
        </div>
        <div>
          <label for="edit-app-group" class="block text-sm font-medium text-text-secondary mb-1">
            Group
            <span class="help-trigger relative ml-1 inline-block align-middle">
              <svg class="w-3.5 h-3.5 text-text-disabled cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
              </svg>
              <span class="help-tooltip">
                Groups organize apps into collapsible sections in the sidebar. Apps with no group appear under "Ungrouped."
              </span>
            </span>
          </label>
          <select
            id="edit-app-group"
            bind:value={editingApp.group}
            class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500"
          >
            <option value="">No group</option>
            {#each localConfig.groups as group (group.name)}
              <option value={group.name}>{group.name}</option>
            {/each}
          </select>
        </div>

        <!-- Display -->
        <div class="border-t border-border pt-3">
          <h4 class="text-xs font-medium text-text-disabled uppercase tracking-wide mb-3">Display</h4>
          <div class="space-y-3">
            <label class="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                bind:checked={editingApp.enabled}
                class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
              />
              <div>
                <span class="text-sm text-text-primary">Enabled
                  <span class="help-trigger relative ml-1 inline-block align-middle">
                    <svg class="w-3.5 h-3.5 text-text-disabled cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
                    </svg>
                    <span class="help-tooltip">
                      Disabled apps are hidden from non-admin users and excluded from the navigation entirely.
                    </span>
                  </span>
                </span>
                <p class="text-xs text-text-muted">Show this app in the navigation</p>
              </div>
            </label>
            <label class="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                bind:checked={editingApp.default}
                class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
              />
              <div>
                <span class="text-sm text-text-primary">Default app
                  <span class="help-trigger relative ml-1 inline-block align-middle">
                    <svg class="w-3.5 h-3.5 text-text-disabled cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
                    </svg>
                    <span class="help-tooltip">
                      Only one app can be the default. Setting this will clear the default flag on any other app.
                    </span>
                  </span>
                </span>
                <p class="text-xs text-text-muted">Automatically load this app on startup instead of the overview</p>
              </div>
            </label>
            <div>
              <label for="edit-app-mode" class="block text-sm font-medium text-text-secondary mb-1">
                Open Mode
                <span class="help-trigger relative ml-1 inline-block align-middle">
                  <svg class="w-3.5 h-3.5 text-text-disabled cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
                  </svg>
                  <span class="help-tooltip">
                    <b>Embedded</b> — loads inside Muximux in an iframe. Best for most apps.<br/>
                    <b>New Tab</b> — opens in a separate browser tab.<br/>
                    <b>New Window</b> — opens in a popup window.
                  </span>
                </span>
              </label>
              <select
                id="edit-app-mode"
                bind:value={editingApp.open_mode}
                class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500"
              >
                {#each openModes as mode (mode.value)}
                  <option value={mode.value}>{mode.label}</option>
                {/each}
              </select>
            </div>
            <div>
              <label for="edit-app-scale" class="block text-sm font-medium text-text-secondary mb-1">
                Scale: {Math.round(editingApp.scale * 100)}%
                <span class="help-trigger relative ml-1 inline-block align-middle">
                  <svg class="w-3.5 h-3.5 text-text-disabled cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
                  </svg>
                  <span class="help-tooltip">
                    Zoom level for the embedded iframe. Useful for apps with small or large UIs. Only applies to iframe open mode.
                  </span>
                </span>
              </label>
              <input
                id="edit-app-scale"
                type="range"
                min="0.5"
                max="2"
                step="0.05"
                bind:value={editingApp.scale}
                class="w-full"
              />
            </div>
          </div>
        </div>

        <!-- Proxy -->
        <div class="border-t border-border pt-3">
          <h4 class="text-xs font-medium text-text-disabled uppercase tracking-wide mb-3">Proxy</h4>
          <div class="space-y-3">
            <label class="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                bind:checked={editingApp.proxy}
                class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
              />
              <div>
                <span class="text-sm text-text-primary">Use reverse proxy
                  <span class="help-trigger relative ml-1 inline-block align-middle">
                    <svg class="w-3.5 h-3.5 text-text-disabled cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
                    </svg>
                    <span class="help-tooltip">
                      Routes all traffic through the built-in Caddy reverse proxy. The app URL is rewritten to a local <code>/proxy/app-name/</code> path, avoiding CORS, mixed-content, and cookie-domain issues.
                    </span>
                  </span>
                </span>
                <p class="text-xs text-text-muted">Route traffic through the built-in proxy to avoid CORS and mixed-content issues</p>
              </div>
            </label>
            {#if editingApp.proxy}
              <div class="ml-7 space-y-3 border-l-2 border-border pl-4 min-w-0 overflow-hidden">
                <label class="flex items-center gap-3 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={editingApp.proxy_skip_tls_verify !== false}
                    onchange={(e) => { editingApp!.proxy_skip_tls_verify = (e.target as HTMLInputElement).checked ? undefined : false; }}
                    class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
                  />
                  <div>
                    <span class="text-sm text-text-primary">Skip TLS verification
                      <span class="help-trigger relative ml-1 inline-block align-middle">
                        <svg class="w-3.5 h-3.5 text-text-disabled cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                          <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
                        </svg>
                        <span class="help-tooltip">
                          Enabled by default. Disable this only if the backend has a valid, trusted TLS certificate and you want strict verification.
                        </span>
                      </span>
                    </span>
                    <p class="text-xs text-text-muted">Disable for backends with valid certificates</p>
                  </div>
                </label>
                <div>
                  <span class="block text-sm text-text-muted mb-1">Custom headers</span>
                  <p class="text-xs text-text-disabled mb-2">Sent to the backend on every proxied request (e.g. Authorization, X-Api-Key)</p>
                  {#each Object.entries(editingApp.proxy_headers ?? {}) as [key, value] (key)}
                    <div class="flex gap-2 mb-2">
                      <input type="text" value={key} placeholder="Header name"
                        class="flex-1 min-w-0 px-2 py-1 text-sm bg-bg-elevated border border-border-subtle rounded text-text-primary placeholder-text-disabled"
                        onchange={(e) => {
                          const app = editingApp!;
                          const headers = { ...(app.proxy_headers ?? {}) };
                          const oldKey = key;
                          const newKey = (e.target as HTMLInputElement).value.trim();
                          if (newKey && newKey !== oldKey) {
                            delete headers[oldKey];
                            headers[newKey] = value;
                            app.proxy_headers = headers;
                          }
                        }}
                      />
                      <input type="text" value={value} placeholder="Value"
                        class="flex-1 min-w-0 px-2 py-1 text-sm bg-bg-elevated border border-border-subtle rounded text-text-primary placeholder-text-disabled"
                        onchange={(e) => {
                          const app = editingApp!;
                          const headers = { ...(app.proxy_headers ?? {}) };
                          headers[key] = (e.target as HTMLInputElement).value;
                          app.proxy_headers = headers;
                        }}
                      />
                      <button class="px-2 py-1 text-text-muted hover:text-red-400" title="Remove header"
                        onclick={() => {
                          const app = editingApp!;
                          const headers = { ...(app.proxy_headers ?? {}) };
                          delete headers[key];
                          app.proxy_headers = Object.keys(headers).length > 0 ? headers : undefined;
                        }}
                      >
                        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
                      </button>
                    </div>
                  {/each}
                  <button class="text-xs text-brand-400 hover:text-brand-300"
                    onclick={() => {
                      const app = editingApp!;
                      app.proxy_headers = { ...(app.proxy_headers ?? {}), '': '' };
                    }}
                  >+ Add header</button>
                </div>
              </div>
            {/if}
          </div>
        </div>

        <!-- Advanced -->
        <div class="border-t border-border pt-3">
          <h4 class="text-xs font-medium text-text-disabled uppercase tracking-wide mb-3">Advanced</h4>
          <div class="space-y-3">
            <label class="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                checked={editingApp.health_check !== false}
                onchange={(e) => {
                  editingApp!.health_check = (e.target as HTMLInputElement).checked ? undefined : false;
                }}
                class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
              />
              <div>
                <span class="text-sm text-text-primary">Health check
                  <span class="help-trigger relative ml-1 inline-block align-middle">
                    <svg class="w-3.5 h-3.5 text-text-disabled cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
                    </svg>
                    <span class="help-tooltip">
                      Periodically pings the app URL (or health URL if set) and shows a status indicator in the navigation. Enabled by default.
                    </span>
                  </span>
                </span>
                <p class="text-xs text-text-muted">Monitor availability of this app</p>
              </div>
            </label>
            {#if editingApp.health_check !== false}
              <div class="ml-7 pl-4 border-l-2 border-border">
                <label for="edit-app-health-url" class="block text-sm text-text-muted mb-1">Health check URL</label>
                <input
                  id="edit-app-health-url"
                  type="url"
                  bind:value={editingApp.health_url}
                  placeholder={editingApp.url || 'Uses app URL if empty'}
                  class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary text-sm focus:outline-none focus:ring-2 focus:ring-brand-500"
                />
                <p class="text-xs text-text-disabled mt-1">Leave blank to use the app URL</p>
              </div>
            {/if}
            <div class="flex items-center gap-3">
              <div class="flex-1">
                <span class="text-sm text-text-primary">Keyboard Shortcut
                  <span class="help-trigger relative ml-1 inline-block align-middle">
                    <svg class="w-3.5 h-3.5 text-text-disabled cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
                    </svg>
                    <span class="help-tooltip">
                      Assigns a number key (1–9) to quickly switch to this app. Each number can only be assigned to one app.
                    </span>
                  </span>
                </span>
                <p class="text-xs text-text-muted">Press this number key to switch to this app</p>
              </div>
              <select
                value={editingApp.shortcut ?? ''}
                onchange={(e) => {
                  const val = (e.target as HTMLSelectElement).value;
                  editingApp!.shortcut = val ? parseInt(val) : undefined;
                }}
                class="px-2 py-1 text-sm bg-bg-elevated border border-border-subtle rounded text-text-primary focus:ring-brand-500 focus:border-brand-500"
              >
                <option value="">None</option>
                {#each [1,2,3,4,5,6,7,8,9] as n (n)}
                  {@const taken = localApps.find(a => a.shortcut === n && a.name !== editingApp?.name)}
                  <option value={n} disabled={!!taken}>{n}{taken ? ` (${taken.name})` : ''}</option>
                {/each}
              </select>
            </div>
            <label class="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                bind:checked={editingApp.force_icon_background}
                class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
              />
              <div>
                <span class="text-sm text-text-primary">Force icon background
                  <span class="help-trigger relative ml-1 inline-block align-middle">
                    <svg class="w-3.5 h-3.5 text-text-disabled cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
                    </svg>
                    <span class="help-tooltip">
                      Always show a colored background circle behind this app's icon, even when the global "Show Icon Backgrounds" setting is off.
                    </span>
                  </span>
                </span>
                <p class="text-xs text-text-muted">Show background even when global icon backgrounds are off</p>
              </div>
            </label>
            <label class="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                bind:checked={editingApp.icon.invert}
                class="w-4 h-4 rounded border-border-subtle text-brand-500 focus:ring-brand-500"
              />
              <div>
                <span class="text-sm text-text-primary">Invert icon colors
                  <span class="help-trigger relative ml-1 inline-block align-middle">
                    <svg class="w-3.5 h-3.5 text-text-disabled cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
                    </svg>
                    <span class="help-tooltip">
                      Inverts the icon's colors (black becomes white, white becomes black). Useful for dark icons that are hard to see on dark backgrounds.
                    </span>
                  </span>
                </span>
                <p class="text-xs text-text-muted">Flip dark icons to light and vice versa</p>
              </div>
            </label>
            <div>
              <label for="edit-app-min-role" class="block text-sm font-medium text-text-secondary mb-1">
                Minimum Role
                <span class="help-trigger relative ml-1 inline-block align-middle">
                  <svg class="w-3.5 h-3.5 text-text-disabled cursor-help" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <circle cx="12" cy="12" r="10" /><path d="M9.09 9a3 3 0 015.83 1c0 2-3 3-3 3" /><line x1="12" y1="17" x2="12.01" y2="17" />
                  </svg>
                  <span class="help-tooltip">
                    Restricts visibility based on user role. Users below the selected role won't see this app in the navigation or API responses.
                  </span>
                </span>
              </label>
              <select
                id="edit-app-min-role"
                bind:value={editingApp.min_role}
                class="w-full px-3 py-2 bg-bg-elevated border border-border-subtle rounded-md text-text-primary focus:outline-none focus:ring-2 focus:ring-brand-500"
              >
                <option value="">Everyone (default)</option>
                <option value="power-user">Power User</option>
                <option value="admin">Admin</option>
              </select>
              <p class="text-xs text-text-muted mt-1">Users below this role won't see this app</p>
            </div>
          </div>
        </div>
      </div>
      <div class="flex justify-end gap-2 p-4 border-t border-border">
        <button
          class="btn btn-secondary btn-sm"
          onclick={cancelEditApp}
        >
          Cancel
        </button>
        <button
          class="btn btn-primary btn-sm"
          onclick={closeEditApp}
        >
          Done
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Edit Group Modal -->
{#if editingGroup}
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
        <h3 class="text-lg font-semibold text-text-primary">Edit {editingGroup.name}</h3>
        <button
          class="btn btn-ghost btn-icon"
          onclick={cancelEditGroup}
          aria-label="Close"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <div class="p-4 space-y-4">
        <div>
          <label for="edit-group-name" class="block text-sm font-medium text-text-secondary mb-1">Name</label>
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
          <span class="block text-sm font-medium text-text-secondary mb-1">Icon</span>
          <div class="flex items-center gap-3">
            <button type="button" class="cursor-pointer rounded hover:ring-2 hover:ring-brand-500 transition-all" onclick={() => openIconBrowser('editGroup')}>
              <AppIcon icon={editingGroup.icon} name={editingGroup.name} color={editingGroup.color} size="lg" />
            </button>
            <div class="flex-1">
              <button
                class="btn btn-secondary btn-sm w-full text-left"
                onclick={() => openIconBrowser('editGroup')}
              >
                {editingGroup.icon?.name || 'Choose icon...'}
              </button>
              <p class="text-xs text-text-muted mt-1">
                {editingGroup.icon?.type === 'dashboard' ? 'Dashboard Icon' : editingGroup.icon?.type || 'No icon set'}
              </p>
            </div>
          </div>
          {#if editingGroup.icon?.type === 'lucide'}
            <div class="flex items-center gap-4 mt-2">
              <label class="flex items-center gap-2 text-xs text-text-muted">
                Icon color
                <input type="color" value={editingGroup!.icon.color || '#ffffff'} oninput={(e) => editingGroup!.icon.color = (e.target as HTMLInputElement).value} class="w-8 h-8 rounded cursor-pointer" />
                {#if editingGroup!.icon.color}
                  <button class="text-text-disabled hover:text-text-secondary" onclick={() => editingGroup!.icon.color = ''} title="Reset to theme default">&times;</button>
                {/if}
              </label>
              <label class="flex items-center gap-2 text-xs text-text-muted">
                Background
                <input type="color" value={editingGroup!.icon.background || editingGroup!.color || '#374151'} oninput={(e) => editingGroup!.icon.background = (e.target as HTMLInputElement).value} class="w-8 h-8 rounded cursor-pointer" />
                <button class="text-text-disabled hover:text-text-secondary text-xs" onclick={() => editingGroup!.icon.background = 'transparent'} title="Transparent">none</button>
                {#if editingGroup!.icon.background}
                  <button class="text-text-disabled hover:text-text-secondary" onclick={() => editingGroup!.icon.background = ''} title="Reset to group color">&times;</button>
                {/if}
              </label>
            </div>
          {/if}
        </div>
        <div>
          <label for="edit-group-color" class="block text-sm font-medium text-text-secondary mb-1">Color</label>
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
          Cancel
        </button>
        <button
          class="btn btn-primary btn-sm"
          onclick={closeEditGroup}
        >
          Done
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Icon Browser Modal -->
{#if showIconBrowser}
  <div
    class="fixed inset-0 z-[70] flex items-center justify-center bg-black/50 p-4"
    transition:fade={{ duration: 100 }}
  >
    <div
      class="bg-bg-surface rounded-xl shadow-2xl w-full max-w-3xl border border-border"
      in:fly={{ y: 10, duration: 150 }}
      out:fade={{ duration: 75 }}
    >
      <div class="flex items-center justify-between p-4 border-b border-border">
        <h3 class="text-lg font-semibold text-text-primary">Select Icon</h3>
        <button
          class="btn btn-ghost btn-icon"
          onclick={() => { showIconBrowser = false; iconBrowserTarget = null; }}
          aria-label="Close"
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
    transition:fade={{ duration: 100 }}
  >
    <div
      class="bg-bg-surface rounded-xl shadow-2xl w-full max-w-md border border-border"
      in:fly={{ y: 10, duration: 150 }}
      out:fade={{ duration: 75 }}
    >
      <div class="flex items-center justify-between p-4 border-b border-border">
        <h3 class="text-lg font-semibold text-text-primary">Import Configuration</h3>
        <button
          class="btn btn-ghost btn-icon"
          onclick={cancelImport}
          aria-label="Close"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <div class="p-4 space-y-4">
        <p class="text-text-secondary">
          This will replace your current configuration with the imported settings.
        </p>
        <div class="bg-bg-hover rounded-lg p-3 text-sm">
          <div class="text-text-muted">Preview:</div>
          <div class="text-text-primary font-medium">{pendingImport.title}</div>
          <div class="text-text-muted text-xs mt-1">
            {pendingImport.apps.length} apps, {pendingImport.groups.length} groups
          </div>
        </div>
        <p class="text-yellow-400 text-sm flex items-center gap-2">
          <svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
          Unsaved changes will be overwritten
        </p>
      </div>
      <div class="flex justify-end gap-2 p-4 border-t border-border">
        <button
          class="btn btn-secondary btn-sm"
          onclick={cancelImport}
        >
          Cancel
        </button>
        <button
          class="btn btn-primary btn-sm"
          onclick={applyImport}
        >
          Import
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

  /* Help tooltips */
  .help-tooltip {
    display: none;
    position: absolute;
    top: calc(100% + 6px);
    left: 0;
    width: 240px;
    padding: 8px 10px;
    border-radius: 8px;
    background: var(--bg-overlay, #1f2937);
    border: 1px solid var(--border-default, #374151);
    color: var(--text-secondary, #d1d5db);
    font-size: 11px;
    line-height: 1.4;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
    z-index: 70;
    pointer-events: none;
  }

  .help-trigger:hover > .help-tooltip {
    display: block;
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
