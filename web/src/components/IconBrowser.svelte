<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import {
    listDashboardIcons,
    getDashboardIconUrl,
    listLucideIcons,
    getLucideIconUrl,
    listCustomIcons,
    getCustomIconUrl,
    uploadCustomIcon,
    deleteCustomIcon,
    type IconInfo,
    type LucideIconInfo,
    type CustomIconInfo
  } from '$lib/api';
  import { toasts } from '$lib/toastStore';
  import SkeletonIconGrid from './SkeletonIconGrid.svelte';
  import ErrorState from './ErrorState.svelte';

  let {
    selectedIcon = '',
    selectedVariant = 'svg',
    selectedType = 'dashboard' as 'dashboard' | 'lucide' | 'custom',
    onselect,
    onclose,
  }: {
    selectedIcon?: string;
    selectedVariant?: string;
    selectedType?: 'dashboard' | 'lucide' | 'custom';
    onselect?: (detail: { name: string; variant: string; type: string }) => void;
    onclose?: () => void;
  } = $props();

  type IconTab = 'dashboard' | 'lucide' | 'custom';
  let activeTab = $state<IconTab>(untrack(() => selectedType));

  let searchQuery = $state('');

  // Dashboard icons
  let dashboardIcons = $state<IconInfo[]>([]);
  let filteredDashboardIcons = $state<IconInfo[]>([]);

  // Lucide icons
  let lucideIcons = $state<LucideIconInfo[]>([]);
  let filteredLucideIcons = $state<LucideIconInfo[]>([]);

  // Custom icons
  let customIcons = $state<CustomIconInfo[]>([]);
  let filteredCustomIcons = $state<CustomIconInfo[]>([]);

  let loading = $state(true);
  let error = $state<string | null>(null);
  let uploading = $state(false);
  let uploadError = $state<string | null>(null);

  // Debounce search
  let searchTimeout: ReturnType<typeof setTimeout>;

  // File input ref
  let fileInput = $state<HTMLInputElement | undefined>(undefined);

  // Infinite scroll
  const BATCH_SIZE = 100;
  let displayCount = $state(BATCH_SIZE);
  let observer: IntersectionObserver;

  function observeSentinel(node: HTMLElement) {
    observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && displayCount < allCurrentIcons.length) {
          displayCount += BATCH_SIZE;
        }
      },
      { rootMargin: '200px' }
    );
    observer.observe(node);
    return {
      destroy() {
        observer.disconnect();
      }
    };
  }

  onMount(async () => {
    await loadAllIcons();
  });

  async function loadAllIcons() {
    loading = true;
    error = null;
    try {
      const failed: string[] = [];
      const [dashboard, lucide, custom] = await Promise.all([
        listDashboardIcons().catch(() => { failed.push('Dashboard'); return [] as IconInfo[]; }),
        listLucideIcons().catch(() => { failed.push('Lucide'); return [] as LucideIconInfo[]; }),
        listCustomIcons().catch(() => { failed.push('Custom'); return [] as CustomIconInfo[]; })
      ]);
      dashboardIcons = dashboard || [];
      lucideIcons = lucide || [];
      customIcons = custom || [];
      if (failed.length > 0 && failed.length < 3) {
        toasts.warning(`Some icon sources failed to load: ${failed.join(', ')}`);
      } else if (failed.length === 3) {
        error = 'Failed to load icons from any source';
      }
      applyFilter();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load icons';
    } finally {
      loading = false;
    }
  }

  async function loadCustomIcons() {
    try {
      customIcons = (await listCustomIcons()) || [];
      applyFilter();
    } catch {
      toasts.error('Failed to reload custom icons');
    }
  }

  function applyFilter() {
    const query = searchQuery.toLowerCase().trim();
    if (query) {
      filteredDashboardIcons = dashboardIcons.filter(i => i.name.toLowerCase().includes(query));
      filteredLucideIcons = lucideIcons.filter(i =>
        i.name.toLowerCase().includes(query) ||
        (i.categories?.some(c => c.toLowerCase().includes(query)) ?? false)
      );
      filteredCustomIcons = customIcons.filter(i => i.name.toLowerCase().includes(query));
    } else {
      filteredDashboardIcons = dashboardIcons;
      filteredLucideIcons = lucideIcons;
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
  }

  function getIconUrl(name: string, type: IconTab): string {
    switch (type) {
      case 'dashboard':
        return getDashboardIconUrl(name, selectedVariant);
      case 'lucide':
        return getLucideIconUrl(name);
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

  let confirmDeleteIcon = $state<string | null>(null);

  function handleDeleteIcon(name: string) {
    confirmDeleteIcon = name;
  }

  async function confirmDeleteIconAction() {
    if (!confirmDeleteIcon) return;
    const name = confirmDeleteIcon;
    confirmDeleteIcon = null;
    try {
      await deleteCustomIcon(name);
      await loadCustomIcons();
      if (selectedIcon === name && selectedType === 'custom') {
        selectedIcon = '';
      }
    } catch (e) {
      toasts.error(e instanceof Error ? e.message : 'Delete failed');
    }
  }

  let allCurrentIcons = $derived.by(() => {
    switch (activeTab) {
      case 'dashboard': return filteredDashboardIcons;
      case 'lucide': return filteredLucideIcons;
      case 'custom': return filteredCustomIcons;
    }
  });
  let currentIcons = $derived(allCurrentIcons.slice(0, displayCount));
  let hasMore = $derived(displayCount < allCurrentIcons.length);
  let totalCount = $derived.by(() => {
    switch (activeTab) {
      case 'dashboard': return dashboardIcons.length;
      case 'lucide': return lucideIcons.length;
      case 'custom': return customIcons.length;
    }
  });

  // Reset display count when search or tab changes
  $effect(() => {
    searchQuery;
    activeTab;
    displayCount = BATCH_SIZE;
  });
</script>

<div class="flex flex-col h-full max-h-[60vh]">
  <!-- Tabs -->
  <div class="flex border-b border-gray-700">
    <button
      class="px-4 py-2 text-sm font-medium transition-colors border-b-2
             {activeTab === 'dashboard'
               ? 'text-brand-400 border-brand-400'
               : 'text-gray-400 border-transparent hover:text-gray-300'}"
      onclick={() => activeTab = 'dashboard'}
    >
      Dashboard Icons
      <span class="text-xs text-gray-500 ml-1">({dashboardIcons.length})</span>
    </button>
    <button
      class="px-4 py-2 text-sm font-medium transition-colors border-b-2
             {activeTab === 'lucide'
               ? 'text-brand-400 border-brand-400'
               : 'text-gray-400 border-transparent hover:text-gray-300'}"
      onclick={() => activeTab = 'lucide'}
    >
      Lucide
      <span class="text-xs text-gray-500 ml-1">({lucideIcons.length})</span>
    </button>
    <button
      class="px-4 py-2 text-sm font-medium transition-colors border-b-2
             {activeTab === 'custom'
               ? 'text-brand-400 border-brand-400'
               : 'text-gray-400 border-transparent hover:text-gray-300'}"
      onclick={() => activeTab = 'custom'}
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
      oninput={handleSearch}
      placeholder="Search icons..."
      class="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-md text-white
             focus:outline-none focus:ring-2 focus:ring-brand-500 text-sm"
    />
  </div>

  <!-- Variant selector (only for dashboard icons) -->
  {#if activeTab === 'dashboard'}
    <div class="px-3 py-2 border-b border-gray-700 flex gap-2">
      {#each ['svg', 'png', 'webp'] as variant (variant)}
        <button
          class="px-3 py-1 text-xs rounded-full transition-colors
                 {selectedVariant === variant
                   ? 'bg-brand-500 text-white'
                   : 'bg-gray-700 text-gray-400 hover:text-white'}"
          onclick={() => selectedVariant = variant}
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
        onchange={handleFileSelect}
        class="hidden"
      />
      <button
        class="w-full px-3 py-2 border-2 border-dashed border-gray-600 rounded-lg
               text-gray-400 hover:text-white hover:border-gray-500 transition-colors
               flex items-center justify-center gap-2"
        onclick={() => fileInput?.click()}
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
        onretry={loadAllIcons}
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
        {#each currentIcons as icon (icon.name)}
          <div class="relative group">
            <button
              class="aspect-square p-2 rounded-lg border transition-all w-full
                     {selectedIcon === icon.name && selectedType === activeTab
                       ? 'border-brand-500 bg-brand-500/10'
                       : 'border-gray-700 hover:border-gray-600 hover:bg-gray-700/50'}"
              onclick={() => selectIcon(icon.name, activeTab)}
              title={icon.name}
            >
              {#if activeTab === 'lucide'}
                <div
                  class="w-full h-full lucide-icon"
                  style="-webkit-mask-image: url({getIconUrl(icon.name, activeTab)}); mask-image: url({getIconUrl(icon.name, activeTab)});"
                  role="img"
                  aria-label={icon.name}
                ></div>
              {:else}
                <img
                  src={getIconUrl(icon.name, activeTab)}
                  alt={icon.name}
                  class="w-full h-full object-contain"
                  loading="lazy"
                />
              {/if}
            </button>
            {#if activeTab === 'custom'}
              {#if confirmDeleteIcon === icon.name}
                <!-- Inline confirmation overlay -->
                <div class="absolute inset-0 rounded-lg bg-gray-900/90 flex flex-col items-center justify-center gap-1 z-10">
                  <span class="text-[10px] text-red-400">Delete?</span>
                  <div class="flex gap-1">
                    <button
                      class="px-1.5 py-0.5 text-[10px] rounded bg-red-600 hover:bg-red-500 text-white"
                      onclick={(e: MouseEvent) => { e.stopPropagation(); confirmDeleteIconAction(); }}
                    >Yes</button>
                    <button
                      class="px-1.5 py-0.5 text-[10px] rounded bg-gray-600 hover:bg-gray-500 text-white"
                      onclick={(e: MouseEvent) => { e.stopPropagation(); confirmDeleteIcon = null; }}
                    >No</button>
                  </div>
                </div>
              {:else}
                <button
                  class="absolute -top-1 -right-1 w-5 h-5 bg-red-500 hover:bg-red-600 rounded-full
                         text-white flex items-center justify-center opacity-0 group-hover:opacity-100
                         transition-opacity text-xs"
                  onclick={(e: MouseEvent) => { e.stopPropagation(); handleDeleteIcon(icon.name); }}
                  title="Delete"
                >
                  <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              {/if}
            {/if}
          </div>
        {/each}
      </div>
      <!-- Infinite scroll sentinel -->
      <div use:observeSentinel class="h-1"></div>
      {#if hasMore}
        <div class="text-center py-3 text-xs text-gray-400">
          Loading more... ({currentIcons.length} of {allCurrentIcons.length})
        </div>
      {/if}
    {/if}
  </div>

  <!-- Footer -->
  <div class="p-3 border-t border-gray-700 flex justify-between items-center">
    <span class="text-xs text-gray-400">
      {allCurrentIcons.length}{allCurrentIcons.length !== totalCount ? ` of ${totalCount}` : ''} icons
    </span>
    <div class="flex gap-2">
      <button
        class="px-3 py-1.5 text-sm text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
        onclick={() => onclose?.()}
      >
        Cancel
      </button>
      <button
        class="px-3 py-1.5 text-sm bg-brand-600 hover:bg-brand-700 text-white rounded-md disabled:opacity-50"
        disabled={!selectedIcon}
        onclick={() => onselect?.({ name: selectedIcon, variant: selectedType === 'dashboard' ? selectedVariant : 'svg', type: selectedType })}
      >
        Select Icon
      </button>
    </div>
  </div>
</div>

<style>
  .lucide-icon {
    background-color: var(--text-primary, #fff);
    -webkit-mask-size: contain;
    mask-size: contain;
    -webkit-mask-repeat: no-repeat;
    mask-repeat: no-repeat;
    -webkit-mask-position: center;
    mask-position: center;
  }
</style>
