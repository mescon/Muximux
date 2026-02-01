<script lang="ts">
  import { onMount } from 'svelte';
  import Navigation from './components/Navigation.svelte';
  import AppFrame from './components/AppFrame.svelte';
  import Splash from './components/Splash.svelte';
  import Search from './components/Search.svelte';
  import type { App, Config } from './lib/types';
  import { fetchConfig } from './lib/api';

  let config: Config | null = null;
  let apps: App[] = [];
  let currentApp: App | null = null;
  let showSplash = true;
  let showSearch = false;
  let loading = true;
  let error: string | null = null;

  onMount(async () => {
    try {
      config = await fetchConfig();
      apps = config.apps;

      // Find default app
      const defaultApp = apps.find(app => app.default);
      if (defaultApp) {
        currentApp = defaultApp;
        showSplash = false;
      }

      loading = false;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load configuration';
      loading = false;
    }
  });

  function selectApp(app: App) {
    if (app.open_mode === 'new_tab') {
      window.open(app.url, '_blank');
    } else if (app.open_mode === 'new_window') {
      window.open(app.url, app.name, 'width=1200,height=800');
    } else {
      currentApp = app;
      showSplash = false;
    }
  }

  function handleKeydown(event: KeyboardEvent) {
    // Global keyboard shortcuts
    if (event.key === '/' || (event.ctrlKey && event.key === 'k')) {
      event.preventDefault();
      showSearch = true;
    } else if (event.key === 'Escape') {
      showSearch = false;
    } else if (event.key === '?') {
      // TODO: Show keyboard shortcuts help
    }
  }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if loading}
  <div class="flex items-center justify-center h-full bg-gray-900">
    <div class="text-center">
      <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-brand-500 mx-auto"></div>
      <p class="mt-4 text-gray-400">Loading Muximux...</p>
    </div>
  </div>
{:else if error}
  <div class="flex items-center justify-center h-full bg-gray-900">
    <div class="text-center text-red-400">
      <svg class="w-12 h-12 mx-auto mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
      </svg>
      <p>{error}</p>
    </div>
  </div>
{:else if config}
  <div class="h-full flex flex-col bg-gray-900">
    <Navigation
      {apps}
      {currentApp}
      {config}
      on:select={(e) => selectApp(e.detail)}
      on:search={() => showSearch = true}
      on:splash={() => showSplash = true}
    />

    <main class="flex-1 overflow-hidden">
      {#if showSplash}
        <Splash {apps} {config} on:select={(e) => selectApp(e.detail)} />
      {:else if currentApp}
        <AppFrame app={currentApp} />
      {/if}
    </main>
  </div>

  {#if showSearch}
    <Search
      {apps}
      on:select={(e) => { selectApp(e.detail); showSearch = false; }}
      on:close={() => showSearch = false}
    />
  {/if}
{/if}
