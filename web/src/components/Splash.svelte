<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import type { App, Config } from '$lib/types';

  export let apps: App[];
  export let config: Config;

  const dispatch = createEventDispatcher<{
    select: App;
  }>();

  // Group apps by their group
  $: groupedApps = apps.reduce((acc, app) => {
    const group = app.group || 'Ungrouped';
    if (!acc[group]) acc[group] = [];
    acc[group].push(app);
    return acc;
  }, {} as Record<string, App[]>);

  $: groups = Object.keys(groupedApps).sort((a, b) => {
    if (a === 'Ungrouped') return 1;
    if (b === 'Ungrouped') return -1;
    return a.localeCompare(b);
  });

  function getOpenModeIcon(mode: string): string {
    switch (mode) {
      case 'new_tab': return '↗';
      case 'new_window': return '⧉';
      default: return '';
    }
  }
</script>

<div class="h-full overflow-auto bg-gradient-to-br from-gray-900 via-gray-800 to-gray-900 p-8">
  <div class="max-w-6xl mx-auto">
    <!-- Header -->
    <div class="text-center mb-12">
      <h1 class="text-4xl font-bold text-white mb-2">{config.title}</h1>
      <p class="text-gray-400">Select an application to get started</p>
    </div>

    <!-- App grid by groups -->
    {#each groups as group}
      <div class="mb-8">
        {#if groups.length > 1 || group !== 'Ungrouped'}
          <h2 class="text-lg font-semibold text-gray-300 mb-4 border-b border-gray-700 pb-2">
            {group}
          </h2>
        {/if}

        <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4">
          {#each groupedApps[group] as app}
            <button
              class="group relative flex flex-col items-center p-4 rounded-xl
                     bg-gray-800/50 hover:bg-gray-700/50 border border-gray-700
                     hover:border-gray-600 transition-all duration-200
                     hover:scale-105 hover:shadow-lg"
              on:click={() => dispatch('select', app)}
            >
              <!-- Icon placeholder -->
              <div
                class="w-16 h-16 rounded-xl flex items-center justify-center mb-3
                       text-2xl font-bold text-white"
                style="background-color: {app.color || '#374151'}"
              >
                {app.name.charAt(0).toUpperCase()}
              </div>

              <!-- App name -->
              <span class="text-sm text-gray-300 group-hover:text-white text-center">
                {app.name}
                {#if app.open_mode !== 'iframe'}
                  <span class="ml-1 text-xs opacity-60">{getOpenModeIcon(app.open_mode)}</span>
                {/if}
              </span>

              <!-- Color indicator bar -->
              <div
                class="absolute bottom-0 left-0 right-0 h-1 rounded-b-xl opacity-0 group-hover:opacity-100 transition-opacity"
                style="background-color: {app.color || '#22c55e'}"
              ></div>
            </button>
          {/each}
        </div>
      </div>
    {/each}

    {#if apps.length === 0}
      <div class="text-center py-12">
        <p class="text-gray-400 mb-4">No applications configured yet.</p>
        <button class="px-4 py-2 bg-brand-600 hover:bg-brand-700 text-white rounded-md">
          Open Settings
        </button>
      </div>
    {/if}
  </div>
</div>
