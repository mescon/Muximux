<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import type { App } from '$lib/types';

  export let apps: App[];

  const dispatch = createEventDispatcher<{
    select: App;
    close: void;
  }>();

  let query = '';
  let selectedIndex = 0;
  let inputElement: HTMLInputElement;

  $: filteredApps = apps.filter(app => {
    if (!query) return true;
    const q = query.toLowerCase();
    return (
      app.name.toLowerCase().includes(q) ||
      app.url.toLowerCase().includes(q) ||
      app.group?.toLowerCase().includes(q)
    );
  });

  $: if (selectedIndex >= filteredApps.length) {
    selectedIndex = Math.max(0, filteredApps.length - 1);
  }

  onMount(() => {
    inputElement?.focus();
  });

  function handleKeydown(event: KeyboardEvent) {
    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault();
        selectedIndex = Math.min(selectedIndex + 1, filteredApps.length - 1);
        break;
      case 'ArrowUp':
        event.preventDefault();
        selectedIndex = Math.max(selectedIndex - 1, 0);
        break;
      case 'Enter':
        event.preventDefault();
        if (filteredApps[selectedIndex]) {
          dispatch('select', filteredApps[selectedIndex]);
        }
        break;
      case 'Escape':
        dispatch('close');
        break;
    }
  }

  function getOpenModeIcon(mode: string): string {
    switch (mode) {
      case 'new_tab': return '↗';
      case 'new_window': return '⧉';
      default: return '';
    }
  }
</script>

<!-- Backdrop -->
<div
  class="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-start justify-center pt-[15vh]"
  on:click={() => dispatch('close')}
  on:keydown={handleKeydown}
>
  <!-- Search modal -->
  <div
    class="w-full max-w-xl bg-gray-800 rounded-xl shadow-2xl border border-gray-700 overflow-hidden"
    on:click|stopPropagation
  >
    <!-- Search input -->
    <div class="p-4 border-b border-gray-700">
      <div class="flex items-center space-x-3">
        <svg class="w-5 h-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
        <input
          bind:this={inputElement}
          bind:value={query}
          type="text"
          placeholder="Search apps..."
          class="flex-1 bg-transparent text-white placeholder-gray-500 outline-none text-lg"
          on:keydown={handleKeydown}
        />
        <kbd class="hidden sm:inline-block px-2 py-1 text-xs text-gray-500 bg-gray-700 rounded">esc</kbd>
      </div>
    </div>

    <!-- Results -->
    <div class="max-h-80 overflow-auto">
      {#if filteredApps.length === 0}
        <div class="p-4 text-center text-gray-500">
          No apps found
        </div>
      {:else}
        <ul class="py-2">
          {#each filteredApps as app, i}
            <li>
              <button
                class="w-full px-4 py-3 flex items-center space-x-3 text-left
                       {i === selectedIndex ? 'bg-gray-700' : 'hover:bg-gray-700/50'}"
                on:click={() => dispatch('select', app)}
                on:mouseenter={() => selectedIndex = i}
              >
                <!-- Icon placeholder -->
                <div
                  class="w-10 h-10 rounded-lg flex items-center justify-center text-white font-bold"
                  style="background-color: {app.color || '#374151'}"
                >
                  {app.name.charAt(0).toUpperCase()}
                </div>

                <div class="flex-1 min-w-0">
                  <div class="text-white font-medium truncate">
                    {app.name}
                    {#if app.open_mode !== 'iframe'}
                      <span class="ml-1 text-xs opacity-60">{getOpenModeIcon(app.open_mode)}</span>
                    {/if}
                  </div>
                  {#if app.group}
                    <div class="text-sm text-gray-500 truncate">{app.group}</div>
                  {/if}
                </div>

                {#if i < 9}
                  <kbd class="hidden sm:inline-block px-2 py-1 text-xs text-gray-500 bg-gray-700 rounded">
                    ⌘{i + 1}
                  </kbd>
                {/if}
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>

    <!-- Footer hints -->
    <div class="px-4 py-2 border-t border-gray-700 text-xs text-gray-500 flex items-center space-x-4">
      <span>↑↓ Navigate</span>
      <span>⏎ Open</span>
      <span>esc Close</span>
    </div>
  </div>
</div>
