<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import {
    listDashboardIcons,
    getDashboardIconUrl,
    listBuiltinIcons,
    getBuiltinIconUrl,
    listCustomIcons,
    getCustomIconUrl,
    uploadCustomIcon,
    deleteCustomIcon,
    type IconInfo,
    type BuiltinIconInfo,
    type CustomIconInfo
  } from '$lib/api';
  import SkeletonIconGrid from './SkeletonIconGrid.svelte';
  import ErrorState from './ErrorState.svelte';

  export let selectedIcon: string = '';
  export let selectedVariant: string = 'svg';
  export let selectedType: 'dashboard' | 'builtin' | 'custom' = 'dashboard';

  const dispatch = createEventDispatcher<{
    select: { name: string; variant: string; type: string };
    close: void;
  }>();

  type IconTab = 'dashboard' | 'builtin' | 'custom';
  let activeTab: IconTab = selectedType;

  let searchQuery = '';

  // Dashboard icons
  let dashboardIcons: IconInfo[] = [];
  let filteredDashboardIcons: IconInfo[] = [];

  // Builtin icons
  let builtinIcons: BuiltinIconInfo[] = [];
  let filteredBuiltinIcons: BuiltinIconInfo[] = [];

  // Custom icons
  let customIcons: CustomIconInfo[] = [];
  let filteredCustomIcons: CustomIconInfo[] = [];

  let loading = true;
  let error: string | null = null;
  let uploading = false;
  let uploadError: string | null = null;

  // Debounce search
  let searchTimeout: ReturnType<typeof setTimeout>;

  // File input ref
  let fileInput: HTMLInputElement;

  onMount(async () => {
    await loadAllIcons();
  });

  async function loadAllIcons() {
    loading = true;
    error = null;
    try {
      const [dashboard, builtin, custom] = await Promise.all([
        listDashboardIcons().catch(() => []),
        listBuiltinIcons().catch(() => []),
        listCustomIcons().catch(() => [])
      ]);
      dashboardIcons = dashboard;
      builtinIcons = builtin;
      customIcons = custom;
      applyFilter();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load icons';
    } finally {
      loading = false;
    }
  }

  async function loadCustomIcons() {
    try {
      customIcons = await listCustomIcons();
      applyFilter();
    } catch (e) {
      // Ignore
    }
  }

  function applyFilter() {
    const query = searchQuery.toLowerCase().trim();
    if (query) {
      filteredDashboardIcons = dashboardIcons.filter(i => i.name.toLowerCase().includes(query));
      filteredBuiltinIcons = builtinIcons.filter(i => i.name.toLowerCase().includes(query));
      filteredCustomIcons = customIcons.filter(i => i.name.toLowerCase().includes(query));
    } else {
      filteredDashboardIcons = dashboardIcons;
      filteredBuiltinIcons = builtinIcons;
      filteredCustomIcons = customIcons;
    }
  }

  function handleSearch() {
    if (searchTimeout) clearTimeout(searchTimeout);
    searchTimeout = setTimeout(applyFilter, 200);
  }

  function selectIcon(name: string, type: IconTab) {
    selectedIcon = name;
    selectedType = type;
    dispatch('select', { name, variant: type === 'dashboard' ? selectedVariant : 'svg', type });
  }

  function getIconUrl(name: string, type: IconTab): string {
    switch (type) {
      case 'dashboard':
        return getDashboardIconUrl(name, selectedVariant);
      case 'builtin':
        return getBuiltinIconUrl(name);
      case 'custom':
        return getCustomIconUrl(name);
    }
  }

  async function handleFileSelect(event: Event) {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;

    uploading = true;
    uploadError = null;

    try {
      await uploadCustomIcon(file);
      await loadCustomIcons();
      input.value = ''; // Reset input
    } catch (e) {
      uploadError = e instanceof Error ? e.message : 'Upload failed';
    } finally {
      uploading = false;
    }
  }

  async function handleDeleteIcon(name: string) {
    if (!confirm(`Delete custom icon "${name}"?`)) return;

    try {
      await deleteCustomIcon(name);
      await loadCustomIcons();
      if (selectedIcon === name && selectedType === 'custom') {
        selectedIcon = '';
      }
    } catch (e) {
      alert(e instanceof Error ? e.message : 'Delete failed');
    }
  }

  $: currentIcons = activeTab === 'dashboard' ? filteredDashboardIcons :
                    activeTab === 'builtin' ? filteredBuiltinIcons :
                    filteredCustomIcons;
</script>

<div class="flex flex-col h-full max-h-[60vh]">
  <!-- Tabs -->
  <div class="flex border-b border-gray-700">
    <button
      class="px-4 py-2 text-sm font-medium transition-colors border-b-2
             {activeTab === 'dashboard'
               ? 'text-brand-400 border-brand-400'
               : 'text-gray-400 border-transparent hover:text-gray-300'}"
      on:click={() => activeTab = 'dashboard'}
    >
      Dashboard Icons
      <span class="text-xs text-gray-500 ml-1">({dashboardIcons.length})</span>
    </button>
    <button
      class="px-4 py-2 text-sm font-medium transition-colors border-b-2
             {activeTab === 'builtin'
               ? 'text-brand-400 border-brand-400'
               : 'text-gray-400 border-transparent hover:text-gray-300'}"
      on:click={() => activeTab = 'builtin'}
    >
      Builtin
      <span class="text-xs text-gray-500 ml-1">({builtinIcons.length})</span>
    </button>
    <button
      class="px-4 py-2 text-sm font-medium transition-colors border-b-2
             {activeTab === 'custom'
               ? 'text-brand-400 border-brand-400'
               : 'text-gray-400 border-transparent hover:text-gray-300'}"
      on:click={() => activeTab = 'custom'}
    >
      Custom
      <span class="text-xs text-gray-500 ml-1">({customIcons.length})</span>
    </button>
  </div>

  <!-- Search -->
  <div class="p-3 border-b border-gray-700">
    <input
      type="text"
      bind:value={searchQuery}
      on:input={handleSearch}
      placeholder="Search icons..."
      class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white
             focus:outline-none focus:ring-2 focus:ring-brand-500 text-sm"
    />
  </div>

  <!-- Variant selector (only for dashboard icons) -->
  {#if activeTab === 'dashboard'}
    <div class="px-3 py-2 border-b border-gray-700 flex gap-2">
      {#each ['svg', 'png', 'webp'] as variant}
        <button
          class="px-3 py-1 text-xs rounded-full transition-colors
                 {selectedVariant === variant
                   ? 'bg-brand-500 text-white'
                   : 'bg-gray-700 text-gray-400 hover:text-white'}"
          on:click={() => selectedVariant = variant}
        >
          {variant.toUpperCase()}
        </button>
      {/each}
    </div>
  {/if}

  <!-- Custom icon upload (only for custom tab) -->
  {#if activeTab === 'custom'}
    <div class="px-3 py-2 border-b border-gray-700">
      <input
        bind:this={fileInput}
        type="file"
        accept=".svg,.png,.jpg,.jpeg,.webp,.gif"
        on:change={handleFileSelect}
        class="hidden"
      />
      <button
        class="w-full px-3 py-2 border-2 border-dashed border-gray-600 rounded-lg
               text-gray-400 hover:text-white hover:border-gray-500 transition-colors
               flex items-center justify-center gap-2"
        on:click={() => fileInput.click()}
        disabled={uploading}
      >
        {#if uploading}
          <div class="animate-spin rounded-full h-4 w-4 border-b-2 border-white"></div>
          Uploading...
        {:else}
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
          </svg>
          Upload Custom Icon
        {/if}
      </button>
      {#if uploadError}
        <p class="text-xs text-red-400 mt-1">{uploadError}</p>
      {/if}
    </div>
  {/if}

  <!-- Icon grid -->
  <div class="flex-1 overflow-y-auto p-3">
    {#if loading}
      <SkeletonIconGrid count={40} />
    {:else if error}
      <ErrorState
        title="Failed to load icons"
        message={error}
        icon="network"
        compact
        on:retry={loadAllIcons}
      />
    {:else if currentIcons.length === 0}
      <ErrorState
        title={searchQuery ? 'No matches found' : activeTab === 'custom' ? 'No custom icons' : 'No icons available'}
        message={searchQuery ? `No icons found matching "${searchQuery}"` : activeTab === 'custom' ? 'Upload your own icons to use them here' : ''}
        icon="empty"
        showRetry={false}
        compact
      />
    {:else}
      <div class="grid grid-cols-6 sm:grid-cols-8 md:grid-cols-10 gap-2">
        {#each currentIcons as icon}
          <div class="relative group">
            <button
              class="aspect-square p-2 rounded-lg border transition-all w-full
                     {selectedIcon === icon.name && selectedType === activeTab
                       ? 'border-brand-500 bg-brand-500/10'
                       : 'border-gray-700 hover:border-gray-600 hover:bg-gray-700/50'}"
              on:click={() => selectIcon(icon.name, activeTab)}
              title={icon.name}
            >
              <img
                src={getIconUrl(icon.name, activeTab)}
                alt={icon.name}
                class="w-full h-full object-contain"
                loading="lazy"
              />
            </button>
            {#if activeTab === 'custom'}
              <button
                class="absolute -top-1 -right-1 w-5 h-5 bg-red-500 hover:bg-red-600 rounded-full
                       text-white flex items-center justify-center opacity-0 group-hover:opacity-100
                       transition-opacity text-xs"
                on:click|stopPropagation={() => handleDeleteIcon(icon.name)}
                title="Delete"
              >
                <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>

  <!-- Footer -->
  <div class="p-3 border-t border-gray-700 flex justify-between items-center">
    <span class="text-xs text-gray-400">
      {currentIcons.length} icons
    </span>
    <div class="flex gap-2">
      <button
        class="px-3 py-1.5 text-sm text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
        on:click={() => dispatch('close')}
      >
        Cancel
      </button>
      <button
        class="px-3 py-1.5 text-sm bg-brand-600 hover:bg-brand-700 text-white rounded-md disabled:opacity-50"
        disabled={!selectedIcon}
        on:click={() => dispatch('select', { name: selectedIcon, variant: selectedType === 'dashboard' ? selectedVariant : 'svg', type: selectedType })}
      >
        Select Icon
      </button>
    </div>
  </div>
</div>
