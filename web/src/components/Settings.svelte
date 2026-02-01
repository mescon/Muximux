<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import { fade, fly } from 'svelte/transition';
  import type { App, Config, Group } from '$lib/types';
  import IconBrowser from './IconBrowser.svelte';
  import AppIcon from './AppIcon.svelte';
  import { themeMode, resolvedTheme, setTheme, type ThemeMode } from '$lib/themeStore';
  import { isMobileViewport } from '$lib/useSwipe';
  import { exportConfig, parseImportedConfig } from '$lib/api';
  import { toasts } from '$lib/toastStore';

  export let config: Config;
  export let apps: App[];

  let isMobile = false;

  onMount(() => {
    isMobile = isMobileViewport();
    const handleResize = () => { isMobile = isMobileViewport(); };
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  });

  const dispatch = createEventDispatcher<{
    close: void;
    save: Config;
  }>();

  // Active tab
  let activeTab: 'general' | 'apps' | 'groups' | 'theme' = 'general';

  // Local copy of config for editing
  let localConfig = JSON.parse(JSON.stringify(config)) as Config;
  let localApps = JSON.parse(JSON.stringify(apps)) as App[];

  // Icon browser state
  let showIconBrowser = false;
  let iconBrowserTarget: 'newApp' | 'editApp' | null = null;

  // Drag and drop state for app reordering
  let draggedAppIndex: number | null = null;
  let dragOverIndex: number | null = null;

  // Track if changes have been made
  $: hasChanges = JSON.stringify(localConfig) !== JSON.stringify(config) ||
                  JSON.stringify(localApps) !== JSON.stringify(apps);

  // Editing state
  let editingApp: App | null = null;
  let editingGroup: Group | null = null;
  let showAddApp = false;
  let showAddGroup = false;

  // Import/export state
  let importFileInput: HTMLInputElement;
  let showImportConfirm = false;
  let pendingImport: ReturnType<typeof parseImportedConfig> | null = null;

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
    scale: 1
  };

  const newGroupTemplate: Group = {
    name: '',
    icon: '',
    color: '#3498db',
    order: 0,
    expanded: true
  };

  let newApp = { ...newAppTemplate };
  let newGroup = { ...newGroupTemplate };

  function handleSave() {
    // Update config with local changes
    localConfig.apps = localApps;
    dispatch('save', localConfig);
    dispatch('close');
  }

  function handleClose() {
    if (hasChanges) {
      if (!confirm('You have unsaved changes. Are you sure you want to close?')) {
        return;
      }
    }
    dispatch('close');
  }

  function addApp() {
    newApp.order = localApps.length;
    localApps = [...localApps, { ...newApp }];
    newApp = { ...newAppTemplate };
    showAddApp = false;
  }

  function deleteApp(app: App) {
    if (confirm(`Delete "${app.name}"? This cannot be undone.`)) {
      localApps = localApps.filter(a => a.name !== app.name);
    }
  }

  function addGroup() {
    newGroup.order = localConfig.groups.length;
    localConfig.groups = [...localConfig.groups, { ...newGroup }];
    newGroup = { ...newGroupTemplate };
    showAddGroup = false;
  }

  function deleteGroup(group: Group) {
    if (confirm(`Delete group "${group.name}"? Apps in this group will become ungrouped.`)) {
      localConfig.groups = localConfig.groups.filter(g => g.name !== group.name);
      // Move apps to ungrouped
      localApps = localApps.map(app =>
        app.group === group.name ? { ...app, group: '' } : app
      );
    }
  }

  function moveApp(app: App, direction: 'up' | 'down') {
    const index = localApps.findIndex(a => a.name === app.name);
    const newApps = [...localApps];

    if (direction === 'up' && index > 0) {
      const temp = newApps[index - 1];
      newApps[index - 1] = newApps[index];
      newApps[index] = temp;
      localApps = newApps.map((a, i) => ({ ...a, order: i }));
    } else if (direction === 'down' && index < localApps.length - 1) {
      const temp = newApps[index];
      newApps[index] = newApps[index + 1];
      newApps[index + 1] = temp;
      localApps = newApps.map((a, i) => ({ ...a, order: i }));
    }
  }

  // Drag and drop handlers for app reordering
  function handleDragStart(e: DragEvent, index: number) {
    draggedAppIndex = index;
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = 'move';
      e.dataTransfer.setData('text/plain', index.toString());
    }
  }

  function handleDragOver(e: DragEvent, index: number) {
    e.preventDefault();
    if (e.dataTransfer) {
      e.dataTransfer.dropEffect = 'move';
    }
    dragOverIndex = index;
  }

  function handleDragLeave() {
    dragOverIndex = null;
  }

  function handleDrop(e: DragEvent, targetIndex: number) {
    e.preventDefault();
    if (draggedAppIndex === null || draggedAppIndex === targetIndex) {
      draggedAppIndex = null;
      dragOverIndex = null;
      return;
    }

    const newApps = [...localApps];
    const [draggedApp] = newApps.splice(draggedAppIndex, 1);
    newApps.splice(targetIndex, 0, draggedApp);

    // Update order values
    localApps = newApps.map((a, i) => ({ ...a, order: i }));
    draggedAppIndex = null;
    dragOverIndex = null;
  }

  function handleDragEnd() {
    draggedAppIndex = null;
    dragOverIndex = null;
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

    showImportConfirm = false;
    pendingImport = null;
    toasts.success('Configuration imported - save to apply changes');
  }

  function cancelImport() {
    showImportConfirm = false;
    pendingImport = null;
  }

  function handleIconSelect(event: CustomEvent<{ name: string; variant: string; type: string }>) {
    const { name, variant, type } = event.detail;
    if (iconBrowserTarget === 'newApp') {
      newApp.icon = { type: type as 'dashboard' | 'builtin' | 'custom', name, variant, file: '', url: '' };
    } else if (iconBrowserTarget === 'editApp' && editingApp) {
      editingApp.icon = { type: type as 'dashboard' | 'builtin' | 'custom', name, variant, file: '', url: '' };
      editingApp = editingApp; // Trigger reactivity
    }
    showIconBrowser = false;
    iconBrowserTarget = null;
  }

  function openIconBrowser(target: 'newApp' | 'editApp') {
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
</script>

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
          on:click={handleSave}
        >
          Save Changes
        </button>
        <button
          class="p-1.5 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          on:click={handleClose}
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    </div>

    <!-- Tabs - scrollable on mobile -->
    <div class="flex border-b border-gray-700 flex-shrink-0 overflow-x-auto scrollbar-hide">
      {#each [
        { id: 'general', label: 'General' },
        { id: 'apps', label: 'Apps' },
        { id: 'groups', label: 'Groups' },
        { id: 'theme', label: 'Theme' }
      ] as tab}
        <button
          class="px-4 py-3 text-sm font-medium transition-colors border-b-2 whitespace-nowrap min-h-[48px]
                 {activeTab === tab.id
                   ? 'text-brand-400 border-brand-400'
                   : 'text-gray-400 border-transparent hover:text-gray-300 hover:border-gray-600'}"
          on:click={() => activeTab = tab.id}
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
                  on:click={() => localConfig.navigation.position = pos.value}
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
                bind:checked={localConfig.navigation.auto_hide}
                class="w-4 h-4 rounded border-gray-600 text-brand-500 focus:ring-brand-500 focus:ring-offset-gray-800"
              />
              <div>
                <div class="text-sm text-white">Auto-hide Navigation</div>
                <div class="text-xs text-gray-400">Hide navigation after inactivity</div>
              </div>
            </label>

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
                on:click={handleExport}
              >
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
                </svg>
                Export Config
              </button>
              <button
                class="px-4 py-2 text-sm bg-gray-700 hover:bg-gray-600 text-white rounded-md flex items-center gap-2"
                on:click={() => importFileInput?.click()}
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
                on:change={handleImportSelect}
              />
            </div>
            <p class="text-xs text-gray-500 mt-2">
              Export your current configuration or import a previously saved one.
            </p>
          </div>
        </div>

      <!-- Apps Settings -->
      {:else if activeTab === 'apps'}
        <div class="space-y-4">
          <!-- Add App Button -->
          <div class="flex justify-between items-center">
            <h3 class="text-sm font-medium text-gray-300">Manage Applications</h3>
            <button
              class="px-3 py-1.5 text-sm bg-brand-600 hover:bg-brand-700 text-white rounded-md flex items-center gap-1"
              on:click={() => showAddApp = true}
            >
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
              </svg>
              Add App
            </button>
          </div>

          <!-- App List - Drag & Drop enabled -->
          <div class="space-y-2">
            {#each localApps as app, i}
              <div
                class="flex items-center gap-3 p-3 bg-gray-700/50 rounded-lg group transition-all cursor-grab active:cursor-grabbing
                       {draggedAppIndex === i ? 'opacity-50 scale-95' : ''}
                       {dragOverIndex === i && draggedAppIndex !== i ? 'border-2 border-brand-500 border-dashed' : 'border-2 border-transparent'}"
                draggable="true"
                on:dragstart={(e) => handleDragStart(e, i)}
                on:dragover={(e) => handleDragOver(e, i)}
                on:dragleave={handleDragLeave}
                on:drop={(e) => handleDrop(e, i)}
                on:dragend={handleDragEnd}
                role="listitem"
              >
                <!-- Drag handle -->
                <div class="flex-shrink-0 text-gray-500 hover:text-gray-300 cursor-grab">
                  <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8h16M4 16h16" />
                  </svg>
                </div>

                <!-- Icon -->
                <span
                  class="w-10 h-10 rounded-lg flex items-center justify-center text-lg font-bold flex-shrink-0"
                  style="background-color: {app.color || '#374151'}"
                >
                  {app.name.charAt(0).toUpperCase()}
                </span>

                <!-- Info -->
                <div class="flex-1 min-w-0">
                  <div class="flex items-center gap-2">
                    <span class="font-medium text-white truncate">{app.name}</span>
                    {#if app.default}
                      <span class="text-xs bg-brand-500/20 text-brand-400 px-1.5 py-0.5 rounded">Default</span>
                    {/if}
                    {#if !app.enabled}
                      <span class="text-xs bg-gray-600 text-gray-400 px-1.5 py-0.5 rounded">Disabled</span>
                    {/if}
                  </div>
                  <div class="text-xs text-gray-400 truncate">{app.url}</div>
                </div>

                <!-- Group badge -->
                {#if app.group}
                  <span class="text-xs bg-gray-600 text-gray-300 px-2 py-0.5 rounded hidden sm:block">
                    {app.group}
                  </span>
                {/if}

                <!-- Actions -->
                <div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                  <button
                    class="p-1.5 text-gray-400 hover:text-white rounded hover:bg-gray-600"
                    on:click={() => editingApp = app}
                    title="Edit"
                  >
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                    </svg>
                  </button>
                  <button
                    class="p-1.5 text-gray-400 hover:text-red-400 rounded hover:bg-gray-600"
                    on:click={() => deleteApp(app)}
                    title="Delete"
                  >
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                    </svg>
                  </button>
                </div>
              </div>
            {/each}

            {#if localApps.length === 0}
              <div class="text-center py-8 text-gray-400">
                No applications configured. Click "Add App" to get started.
              </div>
            {/if}
          </div>
        </div>

      <!-- Groups Settings -->
      {:else if activeTab === 'groups'}
        <div class="space-y-4">
          <!-- Add Group Button -->
          <div class="flex justify-between items-center">
            <h3 class="text-sm font-medium text-gray-300">Manage Groups</h3>
            <button
              class="px-3 py-1.5 text-sm bg-brand-600 hover:bg-brand-700 text-white rounded-md flex items-center gap-1"
              on:click={() => showAddGroup = true}
            >
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
              </svg>
              Add Group
            </button>
          </div>

          <!-- Group List -->
          <div class="space-y-2">
            {#each localConfig.groups as group}
              <div class="flex items-center gap-3 p-3 bg-gray-700/50 rounded-lg group">
                <!-- Color indicator -->
                <span
                  class="w-8 h-8 rounded-lg flex-shrink-0"
                  style="background-color: {group.color || '#374151'}"
                ></span>

                <!-- Info -->
                <div class="flex-1 min-w-0">
                  <span class="font-medium text-white">{group.name}</span>
                  <div class="text-xs text-gray-400">
                    {localApps.filter(a => a.group === group.name).length} apps
                  </div>
                </div>

                <!-- Actions -->
                <div class="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                  <button
                    class="p-1.5 text-gray-400 hover:text-white rounded hover:bg-gray-600"
                    on:click={() => editingGroup = group}
                    title="Edit"
                  >
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                    </svg>
                  </button>
                  <button
                    class="p-1.5 text-gray-400 hover:text-red-400 rounded hover:bg-gray-600"
                    on:click={() => deleteGroup(group)}
                    title="Delete"
                  >
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                    </svg>
                  </button>
                </div>
              </div>
            {/each}

            {#if localConfig.groups.length === 0}
              <div class="text-center py-8 text-gray-400">
                No groups configured. Apps will appear in a single list.
              </div>
            {/if}
          </div>
        </div>

      <!-- Theme Settings -->
      {:else if activeTab === 'theme'}
        <div class="space-y-6">
          <!-- Theme Mode Selection -->
          <div>
            <label class="block text-sm font-medium text-gray-300 mb-3">
              Appearance
            </label>
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <!-- Dark Mode -->
              <button
                class="p-4 rounded-lg border text-left transition-colors group
                       {$themeMode === 'dark'
                         ? 'border-brand-500 bg-brand-500/10'
                         : 'border-gray-600 hover:border-gray-500'}"
                on:click={() => setTheme('dark')}
              >
                <div class="flex items-center gap-3 mb-2">
                  <div class="w-10 h-10 rounded-lg bg-gray-800 border border-gray-600 flex items-center justify-center">
                    <svg class="w-5 h-5 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
                    </svg>
                  </div>
                  {#if $themeMode === 'dark'}
                    <span class="w-4 h-4 bg-brand-500 rounded-full flex-shrink-0 ml-auto flex items-center justify-center">
                      <svg class="w-2.5 h-2.5 text-white" fill="currentColor" viewBox="0 0 20 20">
                        <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                      </svg>
                    </span>
                  {/if}
                </div>
                <div class="font-medium text-white">Dark</div>
                <div class="text-xs text-gray-400 mt-1">Always use dark theme</div>
              </button>

              <!-- Light Mode -->
              <button
                class="p-4 rounded-lg border text-left transition-colors group
                       {$themeMode === 'light'
                         ? 'border-brand-500 bg-brand-500/10'
                         : 'border-gray-600 hover:border-gray-500'}"
                on:click={() => setTheme('light')}
              >
                <div class="flex items-center gap-3 mb-2">
                  <div class="w-10 h-10 rounded-lg bg-gray-100 border border-gray-300 flex items-center justify-center">
                    <svg class="w-5 h-5 text-yellow-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
                    </svg>
                  </div>
                  {#if $themeMode === 'light'}
                    <span class="w-4 h-4 bg-brand-500 rounded-full flex-shrink-0 ml-auto flex items-center justify-center">
                      <svg class="w-2.5 h-2.5 text-white" fill="currentColor" viewBox="0 0 20 20">
                        <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                      </svg>
                    </span>
                  {/if}
                </div>
                <div class="font-medium text-white">Light</div>
                <div class="text-xs text-gray-400 mt-1">Always use light theme</div>
              </button>

              <!-- System Mode -->
              <button
                class="p-4 rounded-lg border text-left transition-colors group
                       {$themeMode === 'system'
                         ? 'border-brand-500 bg-brand-500/10'
                         : 'border-gray-600 hover:border-gray-500'}"
                on:click={() => setTheme('system')}
              >
                <div class="flex items-center gap-3 mb-2">
                  <div class="w-10 h-10 rounded-lg bg-gradient-to-br from-gray-100 to-gray-800 border border-gray-500 flex items-center justify-center">
                    <svg class="w-5 h-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                    </svg>
                  </div>
                  {#if $themeMode === 'system'}
                    <span class="w-4 h-4 bg-brand-500 rounded-full flex-shrink-0 ml-auto flex items-center justify-center">
                      <svg class="w-2.5 h-2.5 text-white" fill="currentColor" viewBox="0 0 20 20">
                        <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                      </svg>
                    </span>
                  {/if}
                </div>
                <div class="font-medium text-white">System</div>
                <div class="text-xs text-gray-400 mt-1">Match device settings</div>
              </button>
            </div>
          </div>

          <!-- Current Theme Info -->
          <div class="p-4 bg-gray-700/30 rounded-lg">
            <div class="flex items-center gap-2 text-sm">
              <span class="text-gray-400">Currently using:</span>
              <span class="font-medium text-white capitalize">{$resolvedTheme} theme</span>
              {#if $themeMode === 'system'}
                <span class="text-xs text-gray-500">(from system preference)</span>
              {/if}
            </div>
          </div>

          <!-- Brand Color (future feature) -->
          <div class="opacity-50">
            <label class="block text-sm font-medium text-gray-400 mb-2">
              Accent Color
              <span class="text-xs text-gray-500 ml-2">(Coming soon)</span>
            </label>
            <div class="flex gap-2">
              {#each ['#22c55e', '#3b82f6', '#8b5cf6', '#ec4899', '#f97316', '#eab308'] as color}
                <button
                  class="w-8 h-8 rounded-full border-2 cursor-not-allowed
                         {color === '#22c55e' ? 'border-white' : 'border-transparent'}"
                  style="background-color: {color}"
                  disabled
                ></button>
              {/each}
            </div>
          </div>
        </div>
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
          on:click={() => showAddApp = false}
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <div class="p-4 space-y-4">
        <div>
          <label for="app-name" class="block text-sm font-medium text-gray-300 mb-1">Name</label>
          <input
            id="app-name"
            type="text"
            bind:value={newApp.name}
            class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500"
            placeholder="My App"
          />
        </div>
        <div>
          <label for="app-url" class="block text-sm font-medium text-gray-300 mb-1">URL</label>
          <input
            id="app-url"
            type="url"
            bind:value={newApp.url}
            class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500"
            placeholder="http://localhost:8080"
          />
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
      </div>
      <div class="flex justify-end gap-2 p-4 border-t border-gray-700">
        <button
          class="px-4 py-2 text-sm text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          on:click={() => showAddApp = false}
        >
          Cancel
        </button>
        <button
          class="px-4 py-2 text-sm bg-brand-600 hover:bg-brand-700 text-white rounded-md disabled:opacity-50"
          disabled={!newApp.name || !newApp.url}
          on:click={addApp}
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
          on:click={() => showAddGroup = false}
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
            class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-brand-500"
            placeholder="Media"
          />
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
          on:click={() => showAddGroup = false}
        >
          Cancel
        </button>
        <button
          class="px-4 py-2 text-sm bg-brand-600 hover:bg-brand-700 text-white rounded-md disabled:opacity-50"
          disabled={!newGroup.name}
          on:click={addGroup}
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
          on:click={() => editingApp = null}
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
            <AppIcon icon={editingApp.icon} name={editingApp.name} color={editingApp.color} size="lg" />
            <div class="flex-1">
              <button
                class="px-3 py-2 text-sm bg-gray-700 hover:bg-gray-600 text-white rounded-md w-full text-left"
                on:click={() => openIconBrowser('editApp')}
              >
                {editingApp.icon?.name || 'Choose icon...'}
              </button>
              <p class="text-xs text-gray-400 mt-1">
                {editingApp.icon?.type === 'dashboard' ? 'Dashboard Icon' : editingApp.icon?.type || 'No icon set'}
              </p>
            </div>
          </div>
        </div>
        <div class="grid grid-cols-2 gap-4">
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
            Scale: {editingApp.scale}x
          </label>
          <input
            id="edit-app-scale"
            type="range"
            min="0.5"
            max="2"
            step="0.1"
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
        </div>
      </div>
      <div class="flex justify-end gap-2 p-4 border-t border-gray-700">
        <button
          class="px-4 py-2 text-sm text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          on:click={() => editingApp = null}
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
          on:click={() => editingGroup = null}
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
          on:click={() => editingGroup = null}
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
          on:click={() => { showIconBrowser = false; iconBrowserTarget = null; }}
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
      <IconBrowser
        selectedIcon={iconBrowserTarget === 'editApp' && editingApp?.icon?.type === 'dashboard' ? editingApp.icon.name : ''}
        on:select={handleIconSelect}
        on:close={() => { showIconBrowser = false; iconBrowserTarget = null; }}
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
          on:click={cancelImport}
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
          on:click={cancelImport}
        >
          Cancel
        </button>
        <button
          class="px-4 py-2 text-sm bg-brand-600 hover:bg-brand-700 text-white rounded-md"
          on:click={applyImport}
        >
          Import
        </button>
      </div>
    </div>
  </div>
{/if}
