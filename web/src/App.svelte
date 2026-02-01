<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import Navigation from './components/Navigation.svelte';
  import AppFrame from './components/AppFrame.svelte';
  import Splash from './components/Splash.svelte';
  import Search from './components/Search.svelte';
  import Settings from './components/Settings.svelte';
  import ShortcutsHelp from './components/ShortcutsHelp.svelte';
  import Login from './components/Login.svelte';
  import OnboardingWizard from './components/OnboardingWizard.svelte';
  import type { App, Config, NavigationConfig, Group } from './lib/types';
  import { fetchConfig, saveConfig } from './lib/api';
  import { startHealthPolling, stopHealthPolling } from './lib/healthStore';
  import { connect as connectWs, disconnect as disconnectWs, on as onWsEvent } from './lib/websocketStore';
  import { authState, checkAuthStatus, logout, isAuthenticated, currentUser, isAdmin } from './lib/authStore';
  import { isOnboardingComplete } from './lib/onboardingStore';
  import type { Config as ConfigType } from './lib/types';

  let config: Config | null = null;
  let apps: App[] = [];
  let currentApp: App | null = null;
  let showSplash = true;
  let showSearch = false;
  let showSettings = false;
  let showShortcuts = false;
  let loading = true;
  let error: string | null = null;

  // Auth state
  let authRequired = false;
  let authChecked = false;

  // Onboarding state
  let showOnboarding = false;

  // Computed layout properties
  $: navPosition = config?.navigation.position || 'top';
  $: isHorizontalLayout = navPosition === 'left' || navPosition === 'right';
  $: isFloatingLayout = navPosition === 'floating';
  $: sidebarWidth = 220; // Will be managed by Navigation component

  onMount(async () => {
    // First check auth status
    await checkAuthStatus();
    authChecked = true;

    // If not authenticated but auth is required, the API call will fail
    // Try to load config
    try {
      config = await fetchConfig();
      apps = config.apps;
      authRequired = config.auth?.method !== 'none' && config.auth?.method !== undefined && config.auth?.method !== '';

      // Check if onboarding should be shown (no apps and not completed)
      if (apps.length === 0 && !isOnboardingComplete()) {
        showOnboarding = true;
        loading = false;
        return;
      }

      // Find default app
      const defaultApp = apps.find(app => app.default);
      if (defaultApp) {
        currentApp = defaultApp;
        showSplash = false;
      }

      // Start health polling if enabled
      if (config.health?.enabled !== false) {
        // Parse interval from config (e.g., "30s" -> 30000ms)
        const intervalStr = config.health?.interval || '30s';
        const match = intervalStr.match(/^(\d+)(ms|s|m)?$/);
        let intervalMs = 30000;
        if (match) {
          const value = parseInt(match[1], 10);
          const unit = match[2] || 's';
          if (unit === 'ms') intervalMs = value;
          else if (unit === 's') intervalMs = value * 1000;
          else if (unit === 'm') intervalMs = value * 60 * 1000;
        }
        startHealthPolling(intervalMs);
      }

      // Connect to WebSocket for real-time updates
      connectWs();

      // Listen for config updates via WebSocket
      onWsEvent('config_updated', (payload) => {
        const newConfig = payload as ConfigType;
        config = newConfig;
        apps = newConfig.apps;
        // Reset current app if it no longer exists
        if (currentApp && !apps.find(a => a.name === currentApp?.name)) {
          currentApp = null;
          showSplash = true;
        }
      });

      loading = false;
    } catch (e) {
      // If we get a 401, auth is required
      if (e instanceof Error && e.message.includes('401')) {
        authRequired = true;
        loading = false;
      } else {
        error = e instanceof Error ? e.message : 'Failed to load configuration';
        loading = false;
      }
    }
  });

  onDestroy(() => {
    stopHealthPolling();
    disconnectWs();
  });

  async function handleLoginSuccess() {
    // Re-fetch config after login
    try {
      config = await fetchConfig();
      apps = config.apps;

      // Find default app
      const defaultApp = apps.find(app => app.default);
      if (defaultApp) {
        currentApp = defaultApp;
        showSplash = false;
      }

      // Start health polling
      if (config.health?.enabled !== false) {
        startHealthPolling(30000);
      }

      // Connect WebSocket
      connectWs();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load configuration';
    }
  }

  async function handleLogout() {
    await logout();
    // Stop services
    stopHealthPolling();
    disconnectWs();
    // Reset state
    config = null;
    apps = [];
    currentApp = null;
    showSplash = true;
  }

  async function handleOnboardingComplete(event: CustomEvent<{ apps: App[]; navigation: NavigationConfig; groups: Group[] }>) {
    const { apps: newApps, navigation, groups } = event.detail;

    if (!config) return;

    // Update config with onboarding selections
    const newConfig: Config = {
      ...config,
      navigation: {
        ...config.navigation,
        ...navigation
      },
      groups,
      apps: newApps
    };

    try {
      const saved = await saveConfig(newConfig);
      config = saved;
      apps = saved.apps;

      // Set first app as current if available
      if (apps.length > 0) {
        const defaultApp = apps.find(a => a.default) || apps[0];
        currentApp = defaultApp;
        showSplash = false;
      }

      // Start services
      if (config.health?.enabled !== false) {
        startHealthPolling(30000);
      }
      connectWs();

      // Hide onboarding
      showOnboarding = false;
    } catch (e) {
      console.error('Failed to save onboarding config:', e);
      alert('Failed to save configuration. See console for details.');
    }
  }

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

  async function handleSaveConfig(newConfig: Config) {
    try {
      const saved = await saveConfig(newConfig);
      config = saved;
      apps = saved.apps;
      // Reset current app if it no longer exists
      if (currentApp && !apps.find(a => a.name === currentApp?.name)) {
        currentApp = null;
        showSplash = true;
      }
    } catch (e) {
      console.error('Failed to save config:', e);
      alert('Failed to save configuration. See console for details.');
    }
  }

  function handleKeydown(event: KeyboardEvent) {
    // Don't trigger shortcuts when not authenticated or on login page
    if (authRequired && !$isAuthenticated) {
      return;
    }

    // Don't trigger shortcuts when typing in inputs
    if (event.target instanceof HTMLInputElement || event.target instanceof HTMLTextAreaElement) {
      if (event.key === 'Escape') {
        showSearch = false;
        showSettings = false;
      }
      return;
    }

    // Global keyboard shortcuts
    if (event.key === '/' || (event.ctrlKey && event.key === 'k')) {
      event.preventDefault();
      showSearch = true;
    } else if (event.key === 'Escape') {
      if (showSearch) showSearch = false;
      else if (showSettings) showSettings = false;
      else if (showShortcuts) showShortcuts = false;
      else if (!showSplash && currentApp) showSplash = true;
    } else if (event.key === '?') {
      event.preventDefault();
      showShortcuts = !showShortcuts;
    } else if (event.ctrlKey && event.key === ',') {
      event.preventDefault();
      showSettings = !showSettings;
    } else if (event.key === 'r' && !event.ctrlKey && !event.metaKey) {
      // Refresh current app iframe
      if (currentApp && !showSplash) {
        const frame = document.querySelector('iframe');
        if (frame) {
          frame.src = frame.src;
        }
      }
    } else if (event.key === 'f' && !event.ctrlKey && !event.metaKey) {
      // Toggle fullscreen (could hide nav)
      // TODO: Implement fullscreen mode
    } else if (event.key >= '1' && event.key <= '9') {
      // Quick switch to app by number
      const index = parseInt(event.key) - 1;
      if (apps[index]) {
        selectApp(apps[index]);
      }
    } else if (event.key === 'Tab') {
      // Navigate between apps
      if (!showSearch && apps.length > 0) {
        event.preventDefault();
        const currentIndex = currentApp ? apps.findIndex(a => a.name === currentApp?.name) : -1;
        const nextIndex = event.shiftKey
          ? (currentIndex - 1 + apps.length) % apps.length
          : (currentIndex + 1) % apps.length;
        selectApp(apps[nextIndex]);
      }
    }
  }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if loading || !authChecked}
  <div class="flex items-center justify-center h-full bg-gray-900">
    <div class="text-center">
      <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-brand-500 mx-auto"></div>
      <p class="mt-4 text-gray-400">Loading Muximux...</p>
    </div>
  </div>
{:else if showOnboarding}
  <OnboardingWizard on:complete={handleOnboardingComplete} />
{:else if authRequired && !$isAuthenticated}
  <Login on:success={handleLoginSuccess} />
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
  <!-- Main layout container - direction changes based on nav position -->
  <div
    class="h-full bg-gray-900"
    class:flex={!isFloatingLayout}
    class:flex-row={isHorizontalLayout && navPosition === 'left'}
    class:flex-row-reverse={isHorizontalLayout && navPosition === 'right'}
    class:flex-col={!isHorizontalLayout && navPosition === 'top'}
    class:flex-col-reverse={!isHorizontalLayout && navPosition === 'bottom'}
  >
    <!-- Navigation -->
    <Navigation
      {apps}
      {currentApp}
      {config}
      on:select={(e) => selectApp(e.detail)}
      on:search={() => showSearch = true}
      on:splash={() => showSplash = true}
      on:settings={() => showSettings = !showSettings}
    />

    <!-- Main content area -->
    <main class="flex-1 overflow-hidden relative">
      {#if showSplash}
        <Splash {apps} {config} on:select={(e) => selectApp(e.detail)} />
      {:else if currentApp}
        <AppFrame app={currentApp} />
      {/if}
    </main>
  </div>

  <!-- Search modal -->
  {#if showSearch}
    <Search
      {apps}
      on:select={(e) => { selectApp(e.detail); showSearch = false; }}
      on:close={() => showSearch = false}
    />
  {/if}

  <!-- Settings panel -->
  {#if showSettings}
    <Settings
      {config}
      {apps}
      on:close={() => showSettings = false}
      on:save={(e) => handleSaveConfig(e.detail)}
    />
  {/if}

  <!-- Keyboard shortcuts help -->
  {#if showShortcuts}
    <ShortcutsHelp on:close={() => showShortcuts = false} />
  {/if}
{/if}
