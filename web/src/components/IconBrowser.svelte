<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import { listDashboardIcons, getDashboardIconUrl } from '$lib/api';
  import type { IconInfo } from '$lib/api';

  export let selectedIcon: string = '';
  export let selectedVariant: string = 'svg';

  const dispatch = createEventDispatcher<{
    select: { name: string; variant: string };
    close: void;
  }>();

  let searchQuery = '';
  let icons: IconInfo[] = [];
  let filteredIcons: IconInfo[] = [];
  let loading = true;
  let error: string | null = null;

  // Debounce search
  let searchTimeout: ReturnType<typeof setTimeout>;

  onMount(async () => {
    await loadIcons();
  });

  async function loadIcons() {
    loading = true;
    error = null;
    try {
      icons = await listDashboardIcons();
      filteredIcons = icons;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load icons';
    } finally {
      loading = false;
    }
  }

  function handleSearch() {
    if (searchTimeout) clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => {
      if (searchQuery.trim()) {
        const query = searchQuery.toLowerCase();
        filteredIcons = icons.filter(icon =>
          icon.name.toLowerCase().includes(query)
        );
      } else {
        filteredIcons = icons;
      }
    }, 200);
  }

  function selectIcon(name: string) {
    selectedIcon = name;
    dispatch('select', { name, variant: selectedVariant });
  }
</script>

<div class="flex flex-col h-full max-h-[60vh]">
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

  <!-- Variant selector -->
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

  <!-- Icon grid -->
  <div class="flex-1 overflow-y-auto p-3">
    {#if loading}
      <div class="flex items-center justify-center py-8">
        <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-brand-500"></div>
      </div>
    {:else if error}
      <div class="text-center py-8 text-red-400">
        <p>{error}</p>
        <button
          class="mt-2 text-sm text-brand-400 hover:text-brand-300"
          on:click={loadIcons}
        >
          Retry
        </button>
      </div>
    {:else if filteredIcons.length === 0}
      <div class="text-center py-8 text-gray-400">
        {#if searchQuery}
          No icons found matching "{searchQuery}"
        {:else}
          No icons available
        {/if}
      </div>
    {:else}
      <div class="grid grid-cols-6 sm:grid-cols-8 md:grid-cols-10 gap-2">
        {#each filteredIcons as icon}
          <button
            class="aspect-square p-2 rounded-lg border transition-all
                   {selectedIcon === icon.name
                     ? 'border-brand-500 bg-brand-500/10'
                     : 'border-gray-700 hover:border-gray-600 hover:bg-gray-700/50'}"
            on:click={() => selectIcon(icon.name)}
            title={icon.name}
          >
            <img
              src={getDashboardIconUrl(icon.name, selectedVariant)}
              alt={icon.name}
              class="w-full h-full object-contain"
              loading="lazy"
            />
          </button>
        {/each}
      </div>
    {/if}
  </div>

  <!-- Footer -->
  <div class="p-3 border-t border-gray-700 flex justify-between items-center">
    <span class="text-xs text-gray-400">
      {filteredIcons.length} icons available
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
        on:click={() => dispatch('select', { name: selectedIcon, variant: selectedVariant })}
      >
        Select Icon
      </button>
    </div>
  </div>
</div>
