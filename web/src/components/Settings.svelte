<script lang="ts">
  import { onMount } from 'svelte';
  import { fade, fly } from 'svelte/transition';
  import { flip } from 'svelte/animate';
  import type { App, Config, Group } from '$lib/types';
  import IconBrowser from './IconBrowser.svelte';
  import AppIcon from './AppIcon.svelte';
  import KeybindingsEditor from './KeybindingsEditor.svelte';
  import { get } from 'svelte/store';
  import { themeMode, resolvedTheme, setTheme, allThemes, isDarkTheme, saveCustomThemeToServer, deleteCustomThemeFromServer, getCurrentThemeVariables, themeVariableGroups, sanitizeThemeId, selectedFamily, variantMode, themeFamilies, setThemeFamily, setVariantMode, type ThemeMode, type VariantMode, type ThemeFamily } from '$lib/themeStore';
  import { isMobileViewport } from '$lib/useSwipe';
  import { exportConfig, parseImportedConfig } from '$lib/api';
  import { toasts } from '$lib/toastStore';
  import { getKeybindingsForConfig } from '$lib/keybindingsStore';
  import { dndzone } from 'svelte-dnd-action';
  import { appSchema, groupSchema, extractErrors } from '$lib/schemas';

  let {
    config,
    apps,
    onclose,
    onsave,
  }: {
    config: Config;
    apps: App[];
    onclose?: () => void;
    onsave?: (config: Config) => void;
  } = $props();

  let isMobile = $state(false);

  onMount(() => {
    isMobile = isMobileViewport();
    const handleResize = () => { isMobile = isMobileViewport(); };
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  });

  // Active tab
  let activeTab = $state<'general' | 'apps' | 'theme' | 'keybindings'>('general');

  // Local copy of config for editing
  let localConfig = $state(JSON.parse(JSON.stringify(config)) as Config);
  let localApps = $state(JSON.parse(JSON.stringify(apps)) as App[]);

  // Icon browser state
  let showIconBrowser = $state(false);
  let iconBrowserTarget = $state<'newApp' | 'editApp' | 'newGroup' | 'editGroup' | null>(null);

  // Drag and drop config
  const flipDurationMs = 200;

  // Track keybindings changes
  let keybindingsChanged = $state(false);

  // Track if changes have been made
  let hasChanges = $derived(JSON.stringify(localConfig) !== initialConfigSnapshot ||
                  JSON.stringify(localApps) !== initialAppsSnapshot ||
                  keybindingsChanged ||
                  $selectedFamily !== initialFamily ||
                  $variantMode !== initialVariant);

  // Editing state
  let editingApp = $state<App | null>(null);
  let editingGroup = $state<Group | null>(null);
  let showAddApp = $state(false);
  let showAddGroup = $state(false);

  // Import/export state
  let importFileInput = $state<HTMLInputElement | undefined>(undefined);
  let showImportConfirm = $state(false);
  let pendingImport = $state<ReturnType<typeof parseImportedConfig> | null>(null);

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
    disable_keyboard_shortcuts: false
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

  // Validation error state
  let appErrors = $state<Record<string, string>>({});
  let groupErrors = $state<Record<string, string>>({});

  // Assign stable `id` fields for svelte-dnd-action (must be done once, before building dnd arrays)
  localApps.forEach(a => { (a as any).id = a.name; });
  localConfig.groups.forEach(g => { (g as any).id = g.name; });

  // Snapshot taken AFTER id fields are added, so hasChanges starts as false
  const initialConfigSnapshot = JSON.stringify(localConfig);
  const initialAppsSnapshot = JSON.stringify(localApps);

  // Snapshot theme so we can revert on close without save
  const initialFamily = get(selectedFamily);
  const initialVariant = get(variantMode);
  const initialTheme = get(themeMode) as ThemeMode;

  // Mutable arrays for svelte-dnd-action (NOT reactive derivations — the library owns these)
  let dndGroups = $state<Group[]>([...localConfig.groups].sort((a, b) => a.order - b.order));
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

  // DnD handlers for groups
  function handleGroupDndConsider(e: CustomEvent<any>) {
    dndGroups = e.detail.items;
  }
  function handleGroupDndFinalize(e: CustomEvent<any>) {
    dndGroups = e.detail.items;
    dndGroups.forEach((g, i) => { g.order = i; });
    localConfig.groups = [...dndGroups];
  }

  // DnD handlers for apps within a group
  function handleAppDndConsider(e: CustomEvent<any>, groupName: string) {
    dndGroupedApps[groupName] = e.detail.items;
  }
  function handleAppDndFinalize(e: CustomEvent<any>, groupName: string) {
    const newItems = e.detail.items as App[];
    newItems.forEach((a, i) => { a.group = groupName; a.order = i; (a as any).id = a.name; });
    dndGroupedApps[groupName] = newItems;
    // Sync back to localApps
    const otherApps = localApps.filter(a => (a.group || '') !== groupName && !newItems.find(n => n.name === a.name));
    localApps = [...otherApps, ...newItems];
  }

  function handleSave() {
    // Update config with local changes
    localConfig.apps = localApps;
    // Include keybindings if changed
    if (keybindingsChanged) {
      localConfig.keybindings = getKeybindingsForConfig();
    }
    onsave?.(localConfig);
    onclose?.();
  }

  // Inline confirmation state
  let confirmClose = $state(false);
  let confirmDeleteApp = $state<App | null>(null);
  let confirmDeleteGroup = $state<Group | null>(null);
  let confirmDeleteTheme = $state<string | null>(null);

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

  function addApp() {
    const result = appSchema.safeParse(newApp);
    if (!result.success) {
      appErrors = extractErrors(result);
      return;
    }
    appErrors = {};
    newApp.order = localApps.length;
    const app = { ...newApp } as any;
    app.id = app.name;
    localApps = [...localApps, app];
    newApp = { ...newAppTemplate };
    showAddApp = false;
    rebuildDndArrays();
  }

  function deleteApp(app: App) {
    confirmDeleteApp = app;
  }

  function confirmDeleteAppAction() {
    if (confirmDeleteApp) {
      localApps = localApps.filter(a => a.name !== confirmDeleteApp!.name);
      confirmDeleteApp = null;
      rebuildDndArrays();
    }
  }

  function addGroup() {
    const result = groupSchema.safeParse(newGroup);
    if (!result.success) {
      groupErrors = extractErrors(result);
      return;
    }
    groupErrors = {};
    newGroup.order = localConfig.groups.length;
    const group = { ...newGroup } as any;
    group.id = group.name;
    localConfig.groups = [...localConfig.groups, group];
    newGroup = { ...newGroupTemplate };
    showAddGroup = false;
    rebuildDndArrays();
  }

  function deleteGroup(group: Group) {
    confirmDeleteGroup = group;
  }

  function confirmDeleteGroupAction() {
    if (confirmDeleteGroup) {
      localConfig.groups = localConfig.groups.filter(g => g.name !== confirmDeleteGroup!.name);
      localApps = localApps.map(app =>
        app.group === confirmDeleteGroup!.name ? { ...app, group: '' } : app
      );
      localApps.forEach(a => { (a as any).id = a.name; });
      confirmDeleteGroup = null;
      rebuildDndArrays();
    }
  }

  function closeEditApp() {
    if (editingApp) {
      (editingApp as any).id = editingApp.name;
      // Sync DnD app changes back to localApps before rebuilding
      const allApps: App[] = [];
      for (const apps of Object.values(dndGroupedApps)) {
        allApps.push(...apps);
      }
      localApps = allApps;
    }
    editingApp = null;
    rebuildDndArrays();
  }

  function closeEditGroup() {
    if (editingGroup) {
      (editingGroup as any).id = editingGroup.name;
      // Sync DnD group changes back to localConfig before rebuilding
      localConfig.groups = [...dndGroups];
    }
    editingGroup = null;
    rebuildDndArrays();
  }

  // Export config to JSON file
  function handleExport() {
    const exportData = {
      ...localConfig,
      apps: localApps,
    };
    exportConfig(exportData as Config);
    toasts.success('Configuration exported');
  }

  // Handle import file selection
  function handleImportSelect(e: Event) {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = (event) => {
      try {
        const content = event.target?.result as string;
        pendingImport = parseImportedConfig(content);
        showImportConfirm = true;
      } catch (err) {
        toasts.error(err instanceof Error ? err.message : 'Failed to parse config file');
      }
    };
    reader.readAsText(file);

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
    localApps.forEach(a => { (a as any).id = a.name; });
    localConfig.groups.forEach(g => { (g as any).id = g.name; });
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
    const iconData = { type: type as 'dashboard' | 'lucide' | 'custom', name, variant, file: '', url: '' };

    if (iconBrowserTarget === 'newApp') {
      newApp.icon = iconData;
    } else if (iconBrowserTarget === 'editApp' && editingApp) {
      editingApp.icon = iconData;
    } else if (iconBrowserTarget === 'newGroup') {
      newGroup.icon = iconData;
    } else if (iconBrowserTarget === 'editGroup' && editingGroup) {
      editingGroup.icon = iconData;
    }
    showIconBrowser = false;
    iconBrowserTarget = null;
  }

  function openIconBrowser(target: 'newApp' | 'editApp' | 'newGroup' | 'editGroup') {
    iconBrowserTarget = target;
    showIconBrowser = true;
  }

  const navPositions = [
    { value: 'top', label: 'Top', description: 'Horizontal bar at the top' },
    { value: 'left', label: 'Left Sidebar', description: 'Vertical sidebar on the left' },
    { value: 'right', label: 'Right Sidebar', description: 'Vertical sidebar on the right' },
    { value: 'bottom', label: 'Bottom Dock', description: 'macOS-style dock at the bottom' },
    { value: 'floating', label: 'Floating', description: 'Minimal floating buttons' }
  ];

  const openModes = [
    { value: 'iframe', label: 'Embedded', description: 'Show inside Muximux' },
    { value: 'new_tab', label: 'New Tab', description: 'Open in a new browser tab' },
    { value: 'new_window', label: 'New Window', description: 'Open in a popup window' }
  ];

  // Theme editor state
  let showThemeEditor = $state(false);
  let themeEditorVars: Record<string, string> = $state({});
  let themeEditorDefaults: Record<string, string> = $state({});
  let saveThemeName = $state('');
  let saveThemeDescription = $state('');
  let saveThemeAuthor = $state('');
  let isSavingTheme = $state(false);

  function openThemeEditor() {
    themeEditorDefaults = getCurrentThemeVariables();
    themeEditorVars = { ...themeEditorDefaults };
    showThemeEditor = true;
  }

  // Refresh theme editor when the active theme changes while editor is open
  $effect(() => {
    $resolvedTheme; // track
    if (showThemeEditor) {
      // Clear any live preview overrides from the previous theme
      for (const name of Object.keys(themeEditorVars)) {
        document.documentElement.style.removeProperty(name);
      }
      // Re-read the new theme's variables
      // Use a microtask so the theme CSS has loaded
      queueMicrotask(() => {
        themeEditorDefaults = getCurrentThemeVariables();
        themeEditorVars = { ...themeEditorDefaults };
      });
    }
  });

  function closeThemeEditor() {
    // Revert live preview changes
    for (const name of Object.keys(themeEditorVars)) {
      document.documentElement.style.removeProperty(name);
    }
    showThemeEditor = false;
    saveThemeName = '';
  }

  function updateThemeVar(name: string, value: string) {
    themeEditorVars[name] = value;
    // Live preview
    document.documentElement.style.setProperty(name, value);
  }

  function resetThemeVar(name: string) {
    themeEditorVars[name] = themeEditorDefaults[name];
    document.documentElement.style.removeProperty(name);
  }

  function resetAllThemeVars() {
    for (const name of Object.keys(themeEditorVars)) {
      document.documentElement.style.removeProperty(name);
    }
    themeEditorVars = { ...themeEditorDefaults };
  }

  async function handleSaveTheme() {
    if (!saveThemeName.trim()) return;
    isSavingTheme = true;
    const success = await saveCustomThemeToServer(
      saveThemeName.trim(),
      $resolvedTheme,
      $isDarkTheme,
      themeEditorVars,
      saveThemeDescription.trim(),
      saveThemeAuthor.trim()
    );
    isSavingTheme = false;
    if (success) {
      // Clear inline overrides — the saved CSS file takes over
      for (const name of Object.keys(themeEditorVars)) {
        document.documentElement.style.removeProperty(name);
      }
      // Switch to the new theme (as a standalone family)
      const id = sanitizeThemeId(saveThemeName.trim());
      setThemeFamily(id);
      setVariantMode($isDarkTheme ? 'dark' : 'light');
      showThemeEditor = false;
      saveThemeName = '';
      saveThemeDescription = '';
      saveThemeAuthor = '';
      toasts.success('Theme saved');
    } else {
      toasts.error('Failed to save theme');
    }
  }

  function handleDeleteTheme(themeId: string) {
    confirmDeleteTheme = themeId;
  }

  async function confirmDeleteThemeAction() {
    if (!confirmDeleteTheme) return;
    const themeId = confirmDeleteTheme;
    confirmDeleteTheme = null;
    const success = await deleteCustomThemeFromServer(themeId);
    if (success) {
      toasts.success('Theme deleted');
    } else {
      toasts.error('Failed to delete theme');
    }
  }

  // Convert CSS color to hex (for color input compatibility)
  function cssColorToHex(color: string): string {
    if (!color) return '#000000';
    // Already hex
    if (color.startsWith('#')) return color.length === 4
      ? '#' + color[1] + color[1] + color[2] + color[2] + color[3] + color[3]
      : color;
    // Try rgb/rgba
    const match = color.match(/rgba?\((\d+),\s*(\d+),\s*(\d+)/);
    if (match) {
      const r = parseInt(match[1]).toString(16).padStart(2, '0');
      const g = parseInt(match[2]).toString(16).padStart(2, '0');
      const b = parseInt(match[3]).toString(16).padStart(2, '0');
      return `#${r}${g}${b}`;
    }
    return '#000000';
  }

  // Variable display names
  const varLabels: Record<string, string> = {
    '--bg-base': 'Base',
    '--bg-surface': 'Surface',
    '--bg-elevated': 'Elevated',
    '--text-primary': 'Primary',
    '--text-secondary': 'Secondary',
    '--text-muted': 'Muted',
    '--accent-primary': 'Primary',
    '--accent-secondary': 'Secondary',
    '--status-success': 'Success',
    '--status-warning': 'Warning',
    '--status-error': 'Error',
    '--status-info': 'Info',
  };
</script>

<div class="settings">
<div
  class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 {isMobile ? 'p-0' : 'p-4'}"
  transition:fade={{ duration: 150 }}
>
  <div
    class="bg-gray-800 shadow-2xl w-full overflow-hidden border border-gray-700 flex flex-col
           {isMobile
             ? 'h-full max-h-full rounded-none'
             : 'rounded-xl max-w-4xl max-h-[90vh]'}"
    in:fly={{ y: isMobile ? 50 : 20, duration: 200 }}
    out:fade={{ duration: 100 }}
  >
    <!-- Header -->
    <div class="flex items-center justify-between p-4 border-b border-gray-700 flex-shrink-0">
      <h2 class="text-lg font-semibold text-white">Settings</h2>
      <div class="flex items-center gap-2">
        {#if hasChanges}
          <span class="text-xs text-yellow-400">Unsaved changes</span>
        {/if}
        <button
          class="px-3 py-1.5 text-sm bg-brand-600 hover:bg-brand-700 text-white rounded-md disabled:opacity-50"
          disabled={!hasChanges}
          onclick={handleSave}
        >
          Save Changes
        </button>
        <button
          class="p-1.5 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          onclick={handleClose}
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
            class="px-3 py-1 text-xs rounded bg-gray-600 hover:bg-gray-500 text-white"
            onclick={() => confirmClose = false}
          >Keep Editing</button>
          <button
            class="px-3 py-1 text-xs rounded bg-red-600 hover:bg-red-500 text-white"
            onclick={confirmCloseDiscard}
          >Discard</button>
        </div>
      </div>
    {/if}

    <!-- Tabs - scrollable on mobile -->
    <div class="flex border-b border-gray-700 flex-shrink-0 overflow-x-auto scrollbar-hide">
      {#each [
        { id: 'general', label: 'General' },
        { id: 'apps', label: 'Apps & Groups' },
        { id: 'theme', label: 'Theme' },
        { id: 'keybindings', label: 'Keybindings' }
      ] as tab}
        <button
          class="px-4 py-3 text-sm font-medium transition-colors border-b-2 whitespace-nowrap min-h-[48px]
                 {activeTab === tab.id
                   ? 'text-brand-400 border-brand-400'
                   : 'text-gray-400 border-transparent hover:text-gray-300 hover:border-gray-600'}"
          onclick={() => activeTab = tab.id}
        >
          {tab.label}
        </button>
      {/each}
    </div>

    <!-- Content -->
    <div class="flex-1 overflow-y-auto p-6">
      <!-- General Settings -->
      {#if activeTab === 'general'}
        <div class="space-y-6">
          <!-- Dashboard Title -->
          <div>
            <label for="title" class="block text-sm font-medium text-gray-300 mb-2">
              Dashboard Title
            </label>
            <input
              id="title"
              type="text"
              bind:value={localConfig.title}
              class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white
                     focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
              placeholder="Muximux"
            />
          </div>

          <!-- Navigation Position -->
          <div>
            <label class="block text-sm font-medium text-gray-300 mb-2">
              Navigation Position
            </label>
            <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
              {#each navPositions as pos}
                <button
                  class="p-3 rounded-lg border text-left transition-colors
                         {localConfig.navigation.position === pos.value
                           ? 'border-brand-500 bg-brand-500/10 text-white'
                           : 'border-gray-600 hover:border-gray-500 text-gray-300'}"
                  onclick={() => localConfig.navigation.position = pos.value}
                >
                  <div class="font-medium">{pos.label}</div>
                  <div class="text-xs text-gray-400 mt-1">{pos.description}</div>
                </button>
              {/each}
            </div>
          </div>

          <!-- Navigation Options -->
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <label class="flex items-center gap-3 p-3 bg-gray-700/50 rounded-lg cursor-pointer">
              <input
                type="checkbox"
                bind:checked={localConfig.navigation.show_logo}
                class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500 focus:ring-offset-gray-800"
              />
              <div>
                <div class="text-sm text-white">Show Logo</div>
                <div class="text-xs text-gray-400">Display dashboard title in navigation</div>
              </div>
            </label>

            <label class="flex items-center gap-3 p-3 bg-gray-700/50 rounded-lg cursor-pointer">
              <input
                type="checkbox"
                bind:checked={localConfig.navigation.show_labels}
                class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500 focus:ring-offset-gray-800"
              />
              <div>
                <div class="text-sm text-white">Show Labels</div>
                <div class="text-xs text-gray-400">Display app names in navigation</div>
              </div>
            </label>

            <label class="flex items-center gap-3 p-3 bg-gray-700/50 rounded-lg cursor-pointer">
              <input
                type="checkbox"
                bind:checked={localConfig.navigation.show_app_colors}
                class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500 focus:ring-offset-gray-800"
              />
              <div>
                <div class="text-sm text-white">App Color Accents</div>
                <div class="text-xs text-gray-400">Show colored borders on app items</div>
              </div>
            </label>

            <label class="flex items-center gap-3 p-3 bg-gray-700/50 rounded-lg cursor-pointer">
              <input
                type="checkbox"
                bind:checked={localConfig.navigation.show_icon_background}
                class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500 focus:ring-offset-gray-800"
              />
              <div>
                <div class="text-sm text-white">Icon Background</div>
                <div class="text-xs text-gray-400">Show colored background behind app icons</div>
              </div>
            </label>

            <label class="flex items-center gap-3 p-3 bg-gray-700/50 rounded-lg cursor-pointer">
              <input
                type="checkbox"
                bind:checked={localConfig.navigation.show_splash_on_startup}
                class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500 focus:ring-offset-gray-800"
              />
              <div>
                <div class="text-sm text-white">Show Splash on Startup</div>
                <div class="text-xs text-gray-400">Show the overview screen instead of the default app on load</div>
              </div>
            </label>

            <div class="p-3 bg-gray-700/50 rounded-lg">
              <label class="flex items-center gap-3 cursor-pointer">
                <input
                  type="checkbox"
                  bind:checked={localConfig.navigation.auto_hide}
                  class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500 focus:ring-offset-gray-800"
                />
                <div class="flex-1">
                  <div class="text-sm text-white">Auto-hide Navigation</div>
                  <div class="text-xs text-gray-400">Hide navigation after inactivity</div>
                </div>
              </label>
              <!-- Hide delay dropdown - nested inside, only shown when enabled -->
              {#if localConfig.navigation.auto_hide}
                <div class="flex items-center gap-3 mt-3 pt-3 border-t border-gray-600">
                  <div class="flex-1 text-xs text-gray-400 pl-7">Hide after</div>
                  <select
                    bind:value={localConfig.navigation.auto_hide_delay}
                    class="px-2 py-1 text-xs bg-gray-600 border border-gray-500 rounded text-white focus:ring-brand-500 focus:border-brand-500"
                  >
                    <option value="0.5s">0.5s</option>
                    <option value="1s">1s</option>
                    <option value="2s">2s</option>
                    <option value="3s">3s</option>
                    <option value="5s">5s</option>
                  </select>
                </div>
                <label class="flex items-center gap-3 mt-3 pt-3 border-t border-gray-600 cursor-pointer">
                  <input
                    type="checkbox"
                    bind:checked={localConfig.navigation.show_shadow}
                    class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500 focus:ring-offset-gray-800 ml-7"
                  />
                  <div>
                    <div class="text-xs text-gray-400">Show shadow</div>
                  </div>
                </label>
              {/if}
            </div>

            <label class="flex items-center gap-3 p-3 bg-gray-700/50 rounded-lg cursor-pointer">
              <input
                type="checkbox"
                bind:checked={localConfig.navigation.show_on_hover}
                class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500 focus:ring-offset-gray-800"
              />
              <div>
                <div class="text-sm text-white">Show on Hover</div>
                <div class="text-xs text-gray-400">Show navigation when mouse is near edge</div>
              </div>
            </label>
          </div>

          <!-- Import/Export Configuration -->
          <div class="pt-4 border-t border-gray-700">
            <h3 class="text-sm font-medium text-gray-300 mb-3">Configuration</h3>
            <div class="flex flex-wrap gap-3">
              <button
                class="px-4 py-2 text-sm bg-gray-700 hover:bg-gray-600 text-white rounded-md flex items-center gap-2"
                onclick={handleExport}
              >
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
                </svg>
                Export Config
              </button>
              <button
                class="px-4 py-2 text-sm bg-gray-700 hover:bg-gray-600 text-white rounded-md flex items-center gap-2"
                onclick={() => importFileInput?.click()}
              >
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
                </svg>
                Import Config
              </button>
              <input
                bind:this={importFileInput}
                type="file"
                accept=".json"
                class="hidden"
                onchange={handleImportSelect}
              />
            </div>
            <p class="text-xs text-gray-500 mt-2">
              Export your current configuration or import a previously saved one.
            </p>
          </div>
        </div>

      <!-- Apps & Groups Settings -->
      {:else if activeTab === 'apps'}
        <div class="space-y-4">
          <!-- Action buttons -->
          <div class="flex justify-between items-center">
            <h3 class="text-sm font-medium text-gray-300">Apps & Groups</h3>
            <div class="flex gap-2">
              <button
                class="px-3 py-1.5 text-sm bg-gray-600 hover:bg-gray-500 text-white rounded-md flex items-center gap-1"
                onclick={() => { groupErrors = {}; showAddGroup = true; }}
              >
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 14v6m-3-3h6M6 10h2a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v2a2 2 0 002 2zm10 0h2a2 2 0 002-2V6a2 2 0 00-2-2h-2a2 2 0 00-2 2v2a2 2 0 002 2zM6 20h2a2 2 0 002-2v-2a2 2 0 00-2-2H6a2 2 0 00-2 2v2a2 2 0 002 2z" />
                </svg>
                Add Group
              </button>
              <button
                class="px-3 py-1.5 text-sm bg-brand-600 hover:bg-brand-700 text-white rounded-md flex items-center gap-1"
                onclick={() => { appErrors = {}; showAddApp = true; }}
              >
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
                </svg>
                Add App
              </button>
            </div>
          </div>

          <div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-gray-500">
            <span>Drag apps to reorder or move between groups. Drag group headers to reorder groups.</span>
            <span class="flex items-center gap-3 text-gray-500">
              <span class="flex items-center gap-1"><span class="app-indicator"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" /></svg></span> Proxy</span>
              <span class="flex items-center gap-1"><span class="app-indicator"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" /></svg></span> New tab</span>
              <span class="flex items-center gap-1"><span class="app-indicator"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><rect x="2" y="3" width="20" height="14" rx="2" ry="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/></svg></span> New window</span>
              <span class="flex items-center gap-1"><span class="app-indicator">50%</span> Scale</span>
              <span class="flex items-center gap-1"><span class="app-indicator">⌨</span> Keyboard</span>
            </span>
          </div>

          <!-- Groups with their apps (dnd-zone for group reordering) -->
          <div class="space-y-3" use:dndzone={{items: dndGroups, flipDurationMs, type: 'groups', dropTargetStyle: {}}} onconsider={handleGroupDndConsider} onfinalize={handleGroupDndFinalize}>
            {#each dndGroups as group (group.id)}
              {@const appsInGroup = dndGroupedApps[group.name] || []}
              <div class="rounded-lg border border-gray-700" animate:flip={{duration: flipDurationMs}}>
                <!-- Group header -->
                <div class="flex items-center gap-3 p-3 bg-gray-700/30 rounded-t-lg cursor-grab active:cursor-grabbing">
                  <!-- Drag handle -->
                  <div class="flex-shrink-0 text-gray-500 hover:text-gray-300">
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8h16M4 16h16" />
                    </svg>
                  </div>

                  <!-- Group icon -->
                  <div class="flex-shrink-0">
                    {#if group.icon?.name}
                      <AppIcon icon={group.icon} name={group.name} color={group.color || '#374151'} size="sm" showBackground={true} />
                    {:else}
                      <span class="w-6 h-6 rounded flex-shrink-0 block" style="background-color: {group.color || '#374151'}"></span>
                    {/if}
                  </div>

                  <!-- Group info -->
                  <div class="flex-1 min-w-0">
                    <span class="font-medium text-white text-sm">{group.name}</span>
                    <span class="text-xs text-gray-500 ml-2">{appsInGroup.length} apps</span>
                  </div>

                  <!-- Group actions -->
                  {#if confirmDeleteGroup?.name === group.name}
                    <div class="flex items-center gap-1">
                      <span class="text-xs text-red-400 mr-1">Delete?</span>
                      <button class="px-2 py-1 text-xs rounded bg-red-600 hover:bg-red-500 text-white"
                              onclick={confirmDeleteGroupAction}>Yes</button>
                      <button class="px-2 py-1 text-xs rounded bg-gray-600 hover:bg-gray-500 text-white"
                              onclick={() => confirmDeleteGroup = null}>No</button>
                    </div>
                  {:else}
                    <div class="flex items-center gap-1">
                      <button class="p-1 text-gray-400 hover:text-white rounded hover:bg-gray-600"
                              onclick={() => editingGroup = group} title="Edit group">
                        <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                        </svg>
                      </button>
                      <button class="p-1 text-gray-400 hover:text-red-400 rounded hover:bg-gray-600"
                              onclick={() => deleteGroup(group)} title="Delete group">
                        <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                        </svg>
                      </button>
                    </div>
                  {/if}
                </div>

                <!-- Apps in this group (dnd-zone for app reordering + cross-group) -->
                <div class="p-2 space-y-1 min-h-[36px]" use:dndzone={{items: appsInGroup, flipDurationMs, type: 'apps', dropTargetStyle: {}}} onconsider={(e) => handleAppDndConsider(e, group.name)} onfinalize={(e) => handleAppDndFinalize(e, group.name)}>
                  {#if appsInGroup.length === 0}
                    <div class="text-center py-3 text-gray-500 text-sm italic">No apps in this group</div>
                  {/if}
                  {#each appsInGroup as app (app.id)}
                    <div
                      class="flex items-center gap-3 p-2 rounded-md group/app hover:bg-gray-700/30 cursor-grab active:cursor-grabbing"
                      animate:flip={{duration: flipDurationMs}}
                    >
                      <!-- Drag handle -->
                      <div class="flex-shrink-0 text-gray-600 hover:text-gray-400">
                        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8h16M4 16h16" />
                        </svg>
                      </div>
                      <div class="flex-shrink-0">
                        <AppIcon icon={app.icon} name={app.name} color={app.color} size="md" />
                      </div>
                      <div class="flex-1 min-w-0">
                        <div class="flex items-center gap-2 flex-wrap">
                          <span class="font-medium text-white text-sm truncate">{app.name}</span>
                          {#if app.default}
                            <span class="text-xs bg-brand-500/20 text-brand-400 px-1.5 py-0.5 rounded">Default</span>
                          {/if}
                          {#if !app.enabled}
                            <span class="text-xs bg-gray-600 text-gray-400 px-1.5 py-0.5 rounded">Disabled</span>
                          {/if}
                          {#if app.proxy}
                            <span class="app-indicator" title="Proxied through server">
                              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" /></svg>
                            </span>
                          {/if}
                          {#if app.open_mode && app.open_mode !== 'iframe'}
                            <span class="app-indicator" title="Opens in {app.open_mode.replace('_', ' ')}">
                              {#if app.open_mode === 'new_tab'}
                                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" /></svg>
                              {:else if app.open_mode === 'new_window'}
                                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><rect x="2" y="3" width="20" height="14" rx="2" ry="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/></svg>
                              {:else}
                                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M17 8l4 4m0 0l-4 4m4-4H3" /></svg>
                              {/if}
                            </span>
                          {/if}
                          {#if app.scale && app.scale !== 1}
                            <span class="app-indicator" title="Scaled to {Math.round(app.scale * 100)}%">
                              {Math.round(app.scale * 100)}%
                            </span>
                          {/if}
                          {#if app.disable_keyboard_shortcuts}
                            <span class="app-indicator" title="App captures keyboard shortcuts">⌨</span>
                          {/if}
                        </div>
                        <span class="text-xs text-gray-400 truncate block">{app.url}</span>
                      </div>
                      <!-- App actions -->
                      {#if confirmDeleteApp?.name === app.name}
                        <div class="flex items-center gap-1">
                          <span class="text-xs text-red-400 mr-1">Delete?</span>
                          <button class="px-2 py-1 text-xs rounded bg-red-600 hover:bg-red-500 text-white"
                                  onclick={confirmDeleteAppAction}>Yes</button>
                          <button class="px-2 py-1 text-xs rounded bg-gray-600 hover:bg-gray-500 text-white"
                                  onclick={() => confirmDeleteApp = null}>No</button>
                        </div>
                      {:else}
                        <div class="flex items-center gap-1 opacity-0 group-hover/app:opacity-100 transition-opacity">
                          <button class="p-1 text-gray-400 hover:text-white rounded hover:bg-gray-600"
                                  onclick={() => editingApp = app} title="Edit">
                            <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                            </svg>
                          </button>
                          <button class="p-1 text-gray-400 hover:text-red-400 rounded hover:bg-gray-600"
                                  onclick={() => deleteApp(app)} title="Delete">
                            <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                            </svg>
                          </button>
                        </div>
                      {/if}
                    </div>
                  {/each}
                </div>
              </div>
            {/each}
          </div>

          <!-- Ungrouped apps -->
          {#if (dndGroupedApps[''] || []).length > 0 || localConfig.groups.length > 0}
            {@const ungroupedApps = dndGroupedApps[''] || []}
            <div class="rounded-lg border border-gray-700 border-dashed" class:hidden={ungroupedApps.length === 0 && localConfig.groups.length === 0}>
              {#if ungroupedApps.length > 0}
                <div class="p-3 bg-gray-700/20 rounded-t-lg">
                  <span class="text-sm font-medium text-gray-400">Ungrouped</span>
                  <span class="text-xs text-gray-500 ml-2">{ungroupedApps.length} apps</span>
                </div>
              {/if}
              <div class="p-2 space-y-1 min-h-[36px]" use:dndzone={{items: ungroupedApps, flipDurationMs, type: 'apps', dropTargetStyle: {}}} onconsider={(e) => handleAppDndConsider(e, '')} onfinalize={(e) => handleAppDndFinalize(e, '')}>
                {#each ungroupedApps as app (app.id)}
                  <div
                    class="flex items-center gap-3 p-2 rounded-md group/app hover:bg-gray-700/30 cursor-grab active:cursor-grabbing"
                    animate:flip={{duration: flipDurationMs}}
                  >
                    <div class="flex-shrink-0 text-gray-600 hover:text-gray-400">
                      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8h16M4 16h16" />
                      </svg>
                    </div>
                    <div class="flex-shrink-0">
                      <AppIcon icon={app.icon} name={app.name} color={app.color} size="md" />
                    </div>
                    <div class="flex-1 min-w-0">
                      <div class="flex items-center gap-2 flex-wrap">
                        <span class="font-medium text-white text-sm truncate">{app.name}</span>
                        {#if app.default}
                          <span class="text-xs bg-brand-500/20 text-brand-400 px-1.5 py-0.5 rounded">Default</span>
                        {/if}
                        {#if !app.enabled}
                          <span class="text-xs bg-gray-600 text-gray-400 px-1.5 py-0.5 rounded">Disabled</span>
                        {/if}
                        {#if app.proxy}
                          <span class="app-indicator" title="Proxied through server">
                            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" /></svg>
                          </span>
                        {/if}
                        {#if app.open_mode && app.open_mode !== 'iframe'}
                          <span class="app-indicator" title="Opens in {app.open_mode.replace('_', ' ')}">
                            {#if app.open_mode === 'new_tab'}
                              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" /></svg>
                            {:else if app.open_mode === 'new_window'}
                              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><rect x="2" y="3" width="20" height="14" rx="2" ry="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/></svg>
                            {:else}
                              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M17 8l4 4m0 0l-4 4m4-4H3" /></svg>
                            {/if}
                          </span>
                        {/if}
                        {#if app.scale && app.scale !== 1}
                          <span class="app-indicator" title="Scaled to {Math.round(app.scale * 100)}%">
                            {Math.round(app.scale * 100)}%
                          </span>
                        {/if}
                        {#if app.disable_keyboard_shortcuts}
                          <span class="app-indicator" title="App captures keyboard shortcuts">⌨</span>
                        {/if}
                      </div>
                      <span class="text-xs text-gray-400 truncate block">{app.url}</span>
                    </div>
                    {#if confirmDeleteApp?.name === app.name}
                      <div class="flex items-center gap-1">
                        <span class="text-xs text-red-400 mr-1">Delete?</span>
                        <button class="px-2 py-1 text-xs rounded bg-red-600 hover:bg-red-500 text-white"
                                onclick={confirmDeleteAppAction}>Yes</button>
                        <button class="px-2 py-1 text-xs rounded bg-gray-600 hover:bg-gray-500 text-white"
                                onclick={() => confirmDeleteApp = null}>No</button>
                      </div>
                    {:else}
                      <div class="flex items-center gap-1 opacity-0 group-hover/app:opacity-100 transition-opacity">
                        <button class="p-1 text-gray-400 hover:text-white rounded hover:bg-gray-600"
                                onclick={() => editingApp = app} title="Edit">
                          <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                          </svg>
                        </button>
                        <button class="p-1 text-gray-400 hover:text-red-400 rounded hover:bg-gray-600"
                                onclick={() => deleteApp(app)} title="Delete">
                          <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                          </svg>
                        </button>
                      </div>
                    {/if}
                  </div>
                {/each}
              </div>
            </div>
          {/if}

          {#if localApps.length === 0 && localConfig.groups.length === 0}
            <div class="text-center py-8 text-gray-400">
              No applications or groups configured. Click "Add App" to get started.
            </div>
          {/if}
        </div>

      <!-- Theme Settings -->
      {:else if activeTab === 'theme'}
        <div class="space-y-6">
          <!-- Variant Mode Selector (Dark / System / Light) -->
          <div class="p-4 rounded-lg" style="background: var(--bg-elevated); border: 1px solid var(--border-subtle);">
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-3">
                <div class="w-10 h-10 rounded-lg flex items-center justify-center"
                     style="background: linear-gradient(135deg, var(--bg-surface) 50%, var(--bg-overlay) 50%); border: 1px solid var(--border-default);">
                  <svg class="w-5 h-5" style="color: var(--text-muted);" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                  </svg>
                </div>
                <div>
                  <div class="font-medium" style="color: var(--text-primary);">Appearance</div>
                  <div class="text-xs" style="color: var(--text-muted);">Choose dark, light, or follow your system</div>
                </div>
              </div>
              <!-- Three-way segmented control -->
              <div class="flex rounded-lg overflow-hidden" style="border: 1px solid var(--border-default);">
                {#each (['dark', 'system', 'light'] as const) as mode}
                  <button
                    class="px-3 py-1.5 text-xs font-medium transition-colors"
                    style="
                      background: {$variantMode === mode ? 'var(--accent-primary)' : 'var(--bg-surface)'};
                      color: {$variantMode === mode ? 'white' : 'var(--text-secondary)'};
                    "
                    onclick={() => setVariantMode(mode)}
                  >
                    {#if mode === 'dark'}
                      Dark
                    {:else if mode === 'system'}
                      System
                    {:else}
                      Light
                    {/if}
                  </button>
                {/each}
              </div>
            </div>
          </div>

          <!-- Theme Family Grid -->
          <div>
            <label class="block text-sm font-medium mb-3" style="color: var(--text-secondary);">
              Choose Theme
            </label>
            <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
              {#each $themeFamilies as family (family.id)}
                {@const isSelected = $selectedFamily === family.id}
                {@const isCustom = family.darkTheme ? !family.darkTheme.isBuiltin : family.lightTheme ? !family.lightTheme.isBuiltin : false}
                <button
                  class="relative p-4 rounded-xl text-left transition-all group"
                  style="
                    background: var(--bg-surface);
                    border: 2px solid {isSelected ? 'var(--accent-primary)' : 'var(--border-subtle)'};
                    box-shadow: {isSelected ? 'var(--shadow-glow)' : 'none'};
                  "
                  onclick={() => setThemeFamily(family.id)}
                >
                  <!-- Selection indicator / delete button -->
                  <div class="absolute top-3 right-3 flex items-center gap-1">
                    {#if isCustom}
                      <button
                        class="w-5 h-5 rounded-full flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity"
                        style="background: var(--status-error); color: white;"
                        onclick={(e: MouseEvent) => { e.stopPropagation(); handleDeleteTheme(family.darkTheme?.id || family.lightTheme?.id || ''); }}
                        title="Delete theme"
                      >
                        <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
                          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                        </svg>
                      </button>
                    {/if}
                    {#if isSelected}
                      <div class="w-5 h-5 rounded-full flex items-center justify-center"
                           style="background: var(--accent-primary);">
                        <svg class="w-3 h-3 text-white" fill="currentColor" viewBox="0 0 20 20">
                          <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                        </svg>
                      </div>
                    {/if}
                  </div>

                  <!-- Dual Preview Swatches (dark left, light right) -->
                  <div class="flex gap-1.5 mb-3">
                    {#if family.darkTheme?.preview && family.lightTheme?.preview}
                      <!-- Dark variant swatch -->
                      <div class="w-10 h-12 rounded-lg overflow-hidden flex flex-col shadow-md"
                           style="border: 1px solid {family.darkTheme.preview.text}20;">
                        <div class="flex-1" style="background: {family.darkTheme.preview.bg};"></div>
                        <div class="h-2" style="background: {family.darkTheme.preview.accent};"></div>
                      </div>
                      <!-- Light variant swatch -->
                      <div class="w-10 h-12 rounded-lg overflow-hidden flex flex-col shadow-md"
                           style="border: 1px solid {family.lightTheme.preview.text}20;">
                        <div class="flex-1" style="background: {family.lightTheme.preview.bg};"></div>
                        <div class="h-2" style="background: {family.lightTheme.preview.accent};"></div>
                      </div>
                    {:else}
                      <!-- Single variant swatch -->
                      {@const theme = family.darkTheme || family.lightTheme}
                      {#if theme?.preview}
                        <div class="w-12 h-12 rounded-lg overflow-hidden flex flex-col shadow-md"
                             style="border: 1px solid {theme.preview.text}20;">
                          <div class="flex-1" style="background: {theme.preview.bg};"></div>
                          <div class="h-2" style="background: {theme.preview.accent};"></div>
                        </div>
                        <div class="flex flex-col gap-1">
                          <div class="w-6 h-5.5 rounded" style="background: {theme.preview.surface}; border: 1px solid {theme.preview.text}15;"></div>
                          <div class="w-6 h-5.5 rounded" style="background: {theme.preview.accent};"></div>
                        </div>
                      {:else}
                        <div class="w-12 h-12 rounded-lg flex items-center justify-center"
                             style="background: var(--bg-elevated); border: 1px solid var(--border-subtle);">
                          <svg class="w-6 h-6" style="color: var(--text-muted);" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01" />
                          </svg>
                        </div>
                      {/if}
                    {/if}
                  </div>

                  <!-- Family Name & Badge -->
                  <div class="flex items-center gap-2">
                    <span class="font-medium" style="color: var(--text-primary);">{family.name}</span>
                    {#if isCustom}
                      <span class="text-[10px] px-1.5 py-0.5 rounded flex-shrink-0"
                            style="background: var(--accent-subtle); color: var(--accent-primary);">
                        Custom
                      </span>
                    {/if}
                  </div>
                  {#if family.description}
                    <div class="text-xs mt-0.5 pr-1" style="color: var(--text-muted);">{family.description}</div>
                  {/if}

                  <!-- Delete confirmation overlay -->
                  {#if confirmDeleteTheme === (family.darkTheme?.id || family.lightTheme?.id)}
                    <div class="absolute inset-0 rounded-xl flex items-center justify-center gap-3 z-10"
                         style="background: var(--bg-overlay); backdrop-filter: blur(4px);"
                         onclick={(e: MouseEvent) => e.stopPropagation()}>
                      <span class="text-sm font-medium" style="color: var(--text-primary);">Delete?</span>
                      <button class="px-3 py-1 rounded text-sm font-medium"
                              style="background: var(--status-error); color: white;"
                              onclick={(e: MouseEvent) => { e.stopPropagation(); confirmDeleteThemeAction(); }}>Yes</button>
                      <button class="px-3 py-1 rounded text-sm font-medium"
                              style="background: var(--bg-elevated); color: var(--text-primary);"
                              onclick={(e: MouseEvent) => { e.stopPropagation(); confirmDeleteTheme = null; }}>No</button>
                    </div>
                  {/if}
                </button>
              {/each}
            </div>
          </div>

          <!-- Current Theme Info -->
          <div class="p-4 rounded-lg" style="background: var(--bg-elevated); border: 1px solid var(--border-subtle);">
            <div class="flex items-center gap-2 text-sm">
              <span style="color: var(--text-muted);">Currently using:</span>
              <span class="font-medium capitalize" style="color: var(--text-primary);">
                {$allThemes.find(t => t.id === $resolvedTheme)?.name || $resolvedTheme} theme
              </span>
              {#if $variantMode === 'system'}
                <span class="text-xs" style="color: var(--text-disabled);">(from system preference)</span>
              {/if}
            </div>
          </div>

          <!-- Theme Customization -->
          <div class="space-y-3">
            {#if !showThemeEditor}
              <button
                class="w-full p-4 rounded-lg text-left transition-all hover:border-brand-500/50 flex items-center gap-3"
                style="background: var(--bg-surface); border: 1px solid var(--border-subtle);"
                onclick={openThemeEditor}
              >
                <div class="w-8 h-8 rounded-lg flex-shrink-0 flex items-center justify-center"
                     style="background: var(--accent-subtle);">
                  <svg class="w-4 h-4" style="color: var(--accent-primary);" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01" />
                  </svg>
                </div>
                <div>
                  <div class="font-medium text-sm" style="color: var(--text-primary);">Customize Current Theme</div>
                  <p class="text-xs mt-0.5" style="color: var(--text-muted);">Tweak colors and save as a new custom theme</p>
                </div>
              </button>
            {:else}
              <!-- Theme Editor Panel -->
              <div class="rounded-lg overflow-hidden" style="border: 1px solid var(--border-default);">
                <div class="flex items-center justify-between p-3" style="background: var(--bg-elevated);">
                  <span class="text-sm font-medium" style="color: var(--text-primary);">Theme Editor</span>
                  <div class="flex items-center gap-2">
                    <button
                      class="px-2 py-1 text-xs rounded transition-colors"
                      style="background: var(--bg-hover); color: var(--text-secondary);"
                      onclick={resetAllThemeVars}
                    >Reset All</button>
                    <button
                      class="p-1 rounded transition-colors"
                      style="color: var(--text-muted);"
                      onclick={closeThemeEditor}
                    >
                      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                      </svg>
                    </button>
                  </div>
                </div>

                <div class="p-3 space-y-4" style="background: var(--bg-surface);">
                  {#each Object.entries(themeVariableGroups) as [groupName, vars]}
                    <div>
                      <div class="text-xs font-semibold uppercase tracking-wider mb-2" style="color: var(--text-muted);">{groupName}</div>
                      <div class="space-y-2">
                        {#each vars as varName}
                          {@const isColorVar = !themeEditorVars[varName]?.startsWith('rgba') && !themeEditorVars[varName]?.includes('px')}
                          <div class="flex items-center gap-2">
                            <span class="text-xs w-20 flex-shrink-0" style="color: var(--text-secondary);">{varLabels[varName] || varName.replace('--', '')}</span>
                            {#if isColorVar}
                              <input
                                type="color"
                                value={cssColorToHex(themeEditorVars[varName] || '#000000')}
                                oninput={(e) => updateThemeVar(varName, e.currentTarget.value)}
                                class="w-8 h-8 rounded cursor-pointer border-0 p-0"
                              />
                            {/if}
                            <input
                              type="text"
                              value={themeEditorVars[varName] || ''}
                              oninput={(e) => updateThemeVar(varName, e.currentTarget.value)}
                              class="flex-1 px-2 py-1 text-xs rounded font-mono"
                              style="background: var(--bg-overlay); color: var(--text-primary); border: 1px solid var(--border-subtle);"
                            />
                            {#if themeEditorVars[varName] !== themeEditorDefaults[varName]}
                              <button
                                class="p-1 rounded transition-colors flex-shrink-0"
                                style="color: var(--text-muted);"
                                onclick={() => resetThemeVar(varName)}
                                title="Reset to default"
                              >
                                <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                                </svg>
                              </button>
                            {:else}
                              <div class="w-[22px]"></div>
                            {/if}
                          </div>
                        {/each}
                      </div>
                    </div>
                  {/each}

                  <!-- Save as theme -->
                  <div class="pt-3 space-y-2" style="border-top: 1px solid var(--border-subtle);">
                    <input
                      type="text"
                      bind:value={saveThemeName}
                      placeholder="Theme name..."
                      class="w-full px-3 py-2 text-sm rounded"
                      style="background: var(--bg-overlay); color: var(--text-primary); border: 1px solid var(--border-default);"
                    />
                    <div class="flex gap-2">
                      <input
                        type="text"
                        bind:value={saveThemeDescription}
                        placeholder="Description (optional)"
                        class="flex-1 px-3 py-2 text-sm rounded"
                        style="background: var(--bg-overlay); color: var(--text-primary); border: 1px solid var(--border-default);"
                      />
                      <input
                        type="text"
                        bind:value={saveThemeAuthor}
                        placeholder="Author (optional)"
                        class="w-32 px-3 py-2 text-sm rounded"
                        style="background: var(--bg-overlay); color: var(--text-primary); border: 1px solid var(--border-default);"
                      />
                    </div>
                    <button
                      class="w-full px-4 py-2 text-sm rounded font-medium transition-colors disabled:opacity-50"
                      style="background: var(--accent-primary); color: var(--bg-base);"
                      disabled={!saveThemeName.trim() || isSavingTheme}
                      onclick={handleSaveTheme}
                    >
                      {isSavingTheme ? 'Saving...' : 'Save Theme'}
                    </button>
                    <p class="text-xs" style="color: var(--text-disabled);">
                      Saves as a CSS file on the server. Changes are live-previewed above.
                    </p>
                  </div>
                </div>
              </div>
            {/if}

          </div>
        </div>
      {/if}

      <!-- Keybindings Settings -->
      {#if activeTab === 'keybindings'}
        <KeybindingsEditor onchange={() => keybindingsChanged = true} />
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
      class="bg-gray-800 rounded-xl shadow-2xl w-full max-w-lg border border-gray-700"
      in:fly={{ y: 10, duration: 150 }}
      out:fade={{ duration: 75 }}
    >
      <div class="flex items-center justify-between p-4 border-b border-gray-700">
        <h3 class="text-lg font-semibold text-white">Add Application</h3>
        <button
          class="p-1 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          onclick={() => showAddApp = false}
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <div class="p-4 space-y-4 max-h-[60vh] overflow-y-auto">
        <div>
          <label for="app-name" class="block text-sm font-medium text-gray-300 mb-1">Name</label>
          <input
            id="app-name"
            type="text"
            bind:value={newApp.name}
            oninput={() => { delete appErrors.name; appErrors = appErrors; }}
            class="w-full px-3 py-2 bg-gray-700 border rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500 {appErrors.name ? 'border-red-500' : 'border-gray-600'}"
            placeholder="My App"
          />
          {#if appErrors.name}<p class="text-red-400 text-xs mt-1">{appErrors.name}</p>{/if}
        </div>
        <div>
          <label for="app-url" class="block text-sm font-medium text-gray-300 mb-1">URL</label>
          <input
            id="app-url"
            type="url"
            bind:value={newApp.url}
            oninput={() => { delete appErrors.url; appErrors = appErrors; }}
            class="w-full px-3 py-2 bg-gray-700 border rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500 {appErrors.url ? 'border-red-500' : 'border-gray-600'}"
            placeholder="http://localhost:8080"
          />
          {#if appErrors.url}<p class="text-red-400 text-xs mt-1">{appErrors.url}</p>{/if}
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-300 mb-1">Icon</label>
          <div class="flex items-center gap-3">
            <button type="button" class="cursor-pointer rounded hover:ring-2 hover:ring-brand-500 transition-all" onclick={() => openIconBrowser('newApp')}>
              <AppIcon icon={newApp.icon} name={newApp.name || 'App'} color={newApp.color} size="lg" />
            </button>
            <button
              class="px-3 py-2 text-sm bg-gray-700 hover:bg-gray-600 text-white rounded-md flex-1 text-left"
              onclick={() => openIconBrowser('newApp')}
            >
              {newApp.icon?.name || 'Choose icon...'}
            </button>
          </div>
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label for="app-color" class="block text-sm font-medium text-gray-300 mb-1">Color</label>
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
                class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500 text-sm"
              />
            </div>
          </div>
          <div>
            <label for="app-group" class="block text-sm font-medium text-gray-300 mb-1">Group</label>
            <select
              id="app-group"
              bind:value={newApp.group}
              class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500"
            >
              <option value="">No group</option>
              {#each localConfig.groups as group}
                <option value={group.name}>{group.name}</option>
              {/each}
            </select>
          </div>
        </div>
        <div>
          <label for="app-mode" class="block text-sm font-medium text-gray-300 mb-1">Open Mode</label>
          <select
            id="app-mode"
            bind:value={newApp.open_mode}
            class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500"
          >
            {#each openModes as mode}
              <option value={mode.value}>{mode.label} - {mode.description}</option>
            {/each}
          </select>
        </div>
        <div>
          <label for="app-scale" class="block text-sm font-medium text-gray-300 mb-1">
            Scale: {Math.round(newApp.scale * 100)}%
          </label>
          <input
            id="app-scale"
            type="range"
            min="0.25"
            max="5"
            step="0.05"
            bind:value={newApp.scale}
            class="w-full"
          />
        </div>
        <div class="space-y-2">
          <label class="flex items-center gap-3 cursor-pointer">
            <input
              type="checkbox"
              bind:checked={newApp.disable_keyboard_shortcuts}
              class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500"
            />
            <div>
              <span class="text-sm text-white">Let app use keyboard shortcuts</span>
              <p class="text-xs text-gray-400">Pauses dashboard shortcuts while this app is active</p>
            </div>
          </label>
        </div>
      </div>
      <div class="flex justify-end gap-2 p-4 border-t border-gray-700">
        <button
          class="px-4 py-2 text-sm text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          onclick={() => showAddApp = false}
        >
          Cancel
        </button>
        <button
          class="px-4 py-2 text-sm bg-brand-600 hover:bg-brand-700 text-white rounded-md"
          onclick={addApp}
        >
          Add App
        </button>
      </div>
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
      class="bg-gray-800 rounded-xl shadow-2xl w-full max-w-md border border-gray-700"
      in:fly={{ y: 10, duration: 150 }}
      out:fade={{ duration: 75 }}
    >
      <div class="flex items-center justify-between p-4 border-b border-gray-700">
        <h3 class="text-lg font-semibold text-white">Add Group</h3>
        <button
          class="p-1 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          onclick={() => showAddGroup = false}
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <div class="p-4 space-y-4">
        <div>
          <label for="group-name" class="block text-sm font-medium text-gray-300 mb-1">Name</label>
          <input
            id="group-name"
            type="text"
            bind:value={newGroup.name}
            oninput={() => { delete groupErrors.name; groupErrors = groupErrors; }}
            class="w-full px-3 py-2 bg-gray-700 border rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500 {groupErrors.name ? 'border-red-500' : 'border-gray-600'}"
            placeholder="Media"
          />
          {#if groupErrors.name}<p class="text-red-400 text-xs mt-1">{groupErrors.name}</p>{/if}
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-300 mb-1">Icon</label>
          <div class="flex items-center gap-3">
            <button type="button" class="cursor-pointer rounded hover:ring-2 hover:ring-brand-500 transition-all" onclick={() => openIconBrowser('newGroup')}>
              <AppIcon icon={newGroup.icon} name={newGroup.name || 'G'} color={newGroup.color} size="lg" />
            </button>
            <button
              class="px-3 py-2 text-sm bg-gray-700 hover:bg-gray-600 text-white rounded-md flex-1 text-left"
              onclick={() => openIconBrowser('newGroup')}
            >
              {newGroup.icon?.name || 'Choose icon...'}
            </button>
          </div>
        </div>
        <div>
          <label for="group-color" class="block text-sm font-medium text-gray-300 mb-1">Color</label>
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
              class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500 text-sm"
            />
          </div>
        </div>
      </div>
      <div class="flex justify-end gap-2 p-4 border-t border-gray-700">
        <button
          class="px-4 py-2 text-sm text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          onclick={() => showAddGroup = false}
        >
          Cancel
        </button>
        <button
          class="px-4 py-2 text-sm bg-brand-600 hover:bg-brand-700 text-white rounded-md"
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
      class="bg-gray-800 rounded-xl shadow-2xl w-full max-w-lg border border-gray-700"
      in:fly={{ y: 10, duration: 150 }}
      out:fade={{ duration: 75 }}
    >
      <div class="flex items-center justify-between p-4 border-b border-gray-700">
        <h3 class="text-lg font-semibold text-white">Edit {editingApp.name}</h3>
        <button
          class="p-1 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          onclick={closeEditApp}
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <div class="p-4 space-y-4 max-h-[60vh] overflow-y-auto">
        <div>
          <label for="edit-app-name" class="block text-sm font-medium text-gray-300 mb-1">Name</label>
          <input
            id="edit-app-name"
            type="text"
            bind:value={editingApp.name}
            class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
        </div>
        <div>
          <label for="edit-app-url" class="block text-sm font-medium text-gray-300 mb-1">URL</label>
          <input
            id="edit-app-url"
            type="url"
            bind:value={editingApp.url}
            class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-300 mb-1">Icon</label>
          <div class="flex items-center gap-3">
            <button type="button" class="cursor-pointer rounded hover:ring-2 hover:ring-brand-500 transition-all" onclick={() => openIconBrowser('editApp')}>
              <AppIcon icon={editingApp.icon} name={editingApp.name} color={editingApp.color} size="lg" />
            </button>
            <div class="flex-1">
              <button
                class="px-3 py-2 text-sm bg-gray-700 hover:bg-gray-600 text-white rounded-md w-full text-left"
                onclick={() => openIconBrowser('editApp')}
              >
                {editingApp.icon?.name || 'Choose icon...'}
              </button>
              <p class="text-xs text-gray-400 mt-1">
                {editingApp.icon?.type === 'dashboard' ? 'Dashboard Icon' : editingApp.icon?.type || 'No icon set'}
              </p>
            </div>
          </div>
        </div>
        <div>
          <label for="edit-app-group" class="block text-sm font-medium text-gray-300 mb-1">Group</label>
          <select
            id="edit-app-group"
            bind:value={editingApp.group}
            class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500"
          >
            <option value="">No group</option>
            {#each localConfig.groups as group}
              <option value={group.name}>{group.name}</option>
            {/each}
          </select>
        </div>
        <div>
          <label for="edit-app-color" class="block text-sm font-medium text-gray-300 mb-1">Color</label>
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
              class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500 text-sm"
            />
          </div>
        </div>
        <div>
          <label for="edit-app-mode" class="block text-sm font-medium text-gray-300 mb-1">Open Mode</label>
          <select
            id="edit-app-mode"
            bind:value={editingApp.open_mode}
            class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500"
          >
            {#each openModes as mode}
              <option value={mode.value}>{mode.label}</option>
            {/each}
          </select>
        </div>
        <div>
          <label for="edit-app-scale" class="block text-sm font-medium text-gray-300 mb-1">
            Scale: {Math.round(editingApp.scale * 100)}%
          </label>
          <input
            id="edit-app-scale"
            type="range"
            min="0.25"
            max="5"
            step="0.05"
            bind:value={editingApp.scale}
            class="w-full"
          />
        </div>
        <div class="space-y-2">
          <label class="flex items-center gap-3 cursor-pointer">
            <input
              type="checkbox"
              bind:checked={editingApp.enabled}
              class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500"
            />
            <span class="text-sm text-white">Enabled</span>
          </label>
          <label class="flex items-center gap-3 cursor-pointer">
            <input
              type="checkbox"
              bind:checked={editingApp.default}
              class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500"
            />
            <span class="text-sm text-white">Default app (load on start)</span>
          </label>
          <label class="flex items-center gap-3 cursor-pointer">
            <input
              type="checkbox"
              bind:checked={editingApp.proxy}
              class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500"
            />
            <span class="text-sm text-white">Use proxy (if enabled)</span>
          </label>
          <label class="flex items-center gap-3 cursor-pointer">
            <input
              type="checkbox"
              bind:checked={editingApp.disable_keyboard_shortcuts}
              class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500"
            />
            <div>
              <span class="text-sm text-white">Let app use keyboard shortcuts</span>
              <p class="text-xs text-gray-400">Pauses dashboard shortcuts while this app is active</p>
            </div>
          </label>
        </div>
      </div>
      <div class="flex justify-end gap-2 p-4 border-t border-gray-700">
        <button
          class="px-4 py-2 text-sm text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
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
      class="bg-gray-800 rounded-xl shadow-2xl w-full max-w-md border border-gray-700"
      in:fly={{ y: 10, duration: 150 }}
      out:fade={{ duration: 75 }}
    >
      <div class="flex items-center justify-between p-4 border-b border-gray-700">
        <h3 class="text-lg font-semibold text-white">Edit {editingGroup.name}</h3>
        <button
          class="p-1 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          onclick={closeEditGroup}
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <div class="p-4 space-y-4">
        <div>
          <label for="edit-group-name" class="block text-sm font-medium text-gray-300 mb-1">Name</label>
          <input
            id="edit-group-name"
            type="text"
            bind:value={editingGroup.name}
            class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-300 mb-1">Icon</label>
          <div class="flex items-center gap-3">
            <button type="button" class="cursor-pointer rounded hover:ring-2 hover:ring-brand-500 transition-all" onclick={() => openIconBrowser('editGroup')}>
              <AppIcon icon={editingGroup.icon} name={editingGroup.name} color={editingGroup.color} size="lg" />
            </button>
            <div class="flex-1">
              <button
                class="px-3 py-2 text-sm bg-gray-700 hover:bg-gray-600 text-white rounded-md w-full text-left"
                onclick={() => openIconBrowser('editGroup')}
              >
                {editingGroup.icon?.name || 'Choose icon...'}
              </button>
              <p class="text-xs text-gray-400 mt-1">
                {editingGroup.icon?.type === 'dashboard' ? 'Dashboard Icon' : editingGroup.icon?.type || 'No icon set'}
              </p>
            </div>
          </div>
        </div>
        <div>
          <label for="edit-group-color" class="block text-sm font-medium text-gray-300 mb-1">Color</label>
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
              class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500 text-sm"
            />
          </div>
        </div>
      </div>
      <div class="flex justify-end gap-2 p-4 border-t border-gray-700">
        <button
          class="px-4 py-2 text-sm text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
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
      class="bg-gray-800 rounded-xl shadow-2xl w-full max-w-3xl border border-gray-700"
      in:fly={{ y: 10, duration: 150 }}
      out:fade={{ duration: 75 }}
    >
      <div class="flex items-center justify-between p-4 border-b border-gray-700">
        <h3 class="text-lg font-semibold text-white">Select Icon</h3>
        <button
          class="p-1 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          onclick={() => { showIconBrowser = false; iconBrowserTarget = null; }}
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <IconBrowser
        selectedIcon={
          iconBrowserTarget === 'editApp' && editingApp?.icon?.type === 'dashboard' ? editingApp.icon.name :
          iconBrowserTarget === 'editGroup' && editingGroup?.icon?.type === 'dashboard' ? editingGroup.icon.name :
          ''
        }
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
      class="bg-gray-800 rounded-xl shadow-2xl w-full max-w-md border border-gray-700"
      in:fly={{ y: 10, duration: 150 }}
      out:fade={{ duration: 75 }}
    >
      <div class="flex items-center justify-between p-4 border-b border-gray-700">
        <h3 class="text-lg font-semibold text-white">Import Configuration</h3>
        <button
          class="p-1 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          onclick={cancelImport}
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <div class="p-4 space-y-4">
        <p class="text-gray-300">
          This will replace your current configuration with the imported settings.
        </p>
        <div class="bg-gray-700/50 rounded-lg p-3 text-sm">
          <div class="text-gray-400">Preview:</div>
          <div class="text-white font-medium">{pendingImport.title}</div>
          <div class="text-gray-400 text-xs mt-1">
            {pendingImport.apps.length} apps, {pendingImport.groups.length} groups
          </div>
          {#if pendingImport.exportedAt}
            <div class="text-gray-500 text-xs mt-1">
              Exported: {new Date(pendingImport.exportedAt).toLocaleDateString()}
            </div>
          {/if}
        </div>
        <p class="text-yellow-400 text-sm flex items-center gap-2">
          <svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
          Unsaved changes will be overwritten
        </p>
      </div>
      <div class="flex justify-end gap-2 p-4 border-t border-gray-700">
        <button
          class="px-4 py-2 text-sm text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          onclick={cancelImport}
        >
          Cancel
        </button>
        <button
          class="px-4 py-2 text-sm bg-brand-600 hover:bg-brand-700 text-white rounded-md"
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
  .settings :global(.bg-gray-800) {
    background-color: var(--bg-surface) !important;
  }
  .settings :global(.bg-gray-700) {
    background-color: var(--bg-elevated) !important;
  }
  .settings :global([class*="bg-gray-700/"]) {
    background-color: var(--bg-hover) !important;
  }
  .settings :global(.bg-gray-600) {
    background-color: var(--bg-overlay) !important;
  }

  /* Borders */
  .settings :global(.border-gray-700) {
    border-color: var(--border-default) !important;
  }
  .settings :global(.border-gray-600) {
    border-color: var(--border-subtle) !important;
  }
  .settings :global(.border-gray-500) {
    border-color: var(--border-strong) !important;
  }

  /* Text */
  .settings :global(.text-white) {
    color: var(--text-primary) !important;
  }
  .settings :global(.text-gray-100),
  .settings :global(.text-gray-200) {
    color: var(--text-primary) !important;
  }
  .settings :global(.text-gray-300) {
    color: var(--text-secondary) !important;
  }
  .settings :global(.text-gray-400) {
    color: var(--text-muted) !important;
  }
  .settings :global(.text-gray-500) {
    color: var(--text-disabled) !important;
  }

  /* Hover backgrounds */
  .settings :global(.hover\:bg-gray-700:hover) {
    background-color: var(--bg-hover) !important;
  }
  .settings :global(.hover\:bg-gray-600:hover) {
    background-color: var(--bg-active) !important;
  }
  .settings :global(.hover\:bg-gray-500:hover) {
    background-color: var(--bg-active) !important;
  }

  /* Hover text */
  .settings :global(.hover\:text-white:hover) {
    color: var(--text-primary) !important;
  }
  .settings :global(.hover\:text-gray-300:hover) {
    color: var(--text-secondary) !important;
  }

  /* Hover borders */
  .settings :global(.hover\:border-gray-600:hover) {
    border-color: var(--border-default) !important;
  }
  .settings :global(.hover\:border-gray-500:hover) {
    border-color: var(--border-strong) !important;
  }

  /* Focus ring offset should match the modal surface */
  .settings :global(.focus\:ring-offset-gray-800) {
    --tw-ring-offset-color: var(--bg-surface) !important;
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
</style>
