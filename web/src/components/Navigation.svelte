<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import type { App, Config } from '$lib/types';

  export let apps: App[];
  export let currentApp: App | null;
  export let config: Config;

  const dispatch = createEventDispatcher<{
    select: App;
    search: void;
    splash: void;
  }>();

  function getOpenModeIcon(mode: string): string {
    switch (mode) {
      case 'new_tab': return '↗';
      case 'new_window': return '⧉';
      default: return '';
    }
  }
</script>

{#if config.navigation.position === 'top'}
  <nav class="bg-gray-800 border-b border-gray-700 px-4">
    <div class="flex items-center justify-between h-14">
      <!-- Logo -->
      <div class="flex items-center space-x-4">
        {#if config.navigation.show_logo}
          <button
            class="text-xl font-bold text-white hover:text-brand-400 transition-colors"
            on:click={() => dispatch('splash')}
          >
            Muximux
          </button>
        {/if}

        <!-- App tabs -->
        <div class="flex items-center space-x-1">
          {#each apps as app}
            <button
              class="px-3 py-2 rounded-md text-sm font-medium transition-colors
                     {currentApp?.name === app.name
                       ? 'bg-gray-900 text-white'
                       : 'text-gray-300 hover:bg-gray-700 hover:text-white'}"
              style={currentApp?.name === app.name ? `border-bottom: 2px solid ${app.color || '#22c55e'}` : ''}
              on:click={() => dispatch('select', app)}
            >
              {app.name}
              {#if app.open_mode !== 'iframe'}
                <span class="ml-1 text-xs opacity-60">{getOpenModeIcon(app.open_mode)}</span>
              {/if}
            </button>
          {/each}
        </div>
      </div>

      <!-- Right side actions -->
      <div class="flex items-center space-x-2">
        <!-- Search button -->
        <button
          class="p-2 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          on:click={() => dispatch('search')}
          title="Search (Ctrl+K)"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
        </button>

        <!-- Settings button (placeholder) -->
        <button
          class="p-2 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          title="Settings"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
        </button>
      </div>
    </div>
  </nav>
{:else}
  <!-- TODO: Implement left, right, bottom, floating layouts -->
  <nav class="bg-gray-800 p-4">
    <p class="text-gray-400">Navigation position: {config.navigation.position} (not yet implemented)</p>
  </nav>
{/if}
