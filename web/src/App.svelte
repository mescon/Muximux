<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import Navigation from './components/Navigation.svelte';
  import AppFrame from './components/AppFrame.svelte';
  import Splash from './components/Splash.svelte';
  import Search from './components/Search.svelte';
  import Settings from './components/Settings.svelte';
  import ShortcutsHelp from './components/ShortcutsHelp.svelte';
  import CommandPalette from './components/CommandPalette.svelte';
  import Login from './components/Login.svelte';
  import OnboardingWizard from './components/OnboardingWizard.svelte';
  import ToastContainer from './components/ToastContainer.svelte';
  import ErrorState from './components/ErrorState.svelte';
  import type { App, Config, NavigationConfig, Group } from './lib/types';
  import { fetchConfig, saveConfig } from './lib/api';
  import { toasts } from './lib/toastStore';
  import { startHealthPolling, stopHealthPolling } from './lib/healthStore';
  import { connect as connectWs, disconnect as disconnectWs, on as onWsEvent } from './lib/websocketStore';
  import { authState, checkAuthStatus, logout, isAuthenticated, currentUser, isAdmin } from './lib/authStore';
  import { isOnboardingComplete } from './lib/onboardingStore';
  import { initTheme, setTheme } from './lib/themeStore';
  import { isFullscreen, toggleFullscreen, exitFullscreen } from './lib/fullscreenStore';
  import { createSwipeHandlers, isMobileViewport, type SwipeResult } from './lib/useSwipe';
  import { findAction, initKeybindings, type KeyAction } from './lib/keybindingsStore';
  import type { Config as ConfigType } from './lib/types';

  let config: Config | null = null;
  let apps: App[] = [];
  let currentApp: App | null = null;
  let showSplash = true;
  let showSearch = false;
  let showSettings = false;
  let showShortcuts = false;
  let showCommandPalette = false;
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

  // Mobile swipe state
  let isMobile = false;
  let mainContentElement: HTMLElement;

  // Swipe gesture handlers for app switching on mobile
  function handleAppSwipe(result: SwipeResult) {
    if (!isMobile || !currentApp || showSplash || !result.direction) return;
    if (result.direction !== 'left' && result.direction !== 'right') return;

    const currentIndex = apps.findIndex(a => a.name === currentApp?.name);
    if (currentIndex === -1) return;

    let nextIndex: number;
    if (result.direction === 'left') {
      // Swipe left = next app
      nextIndex = (currentIndex + 1) % apps.length;
    } else {
      // Swipe right = previous app
      nextIndex = (currentIndex - 1 + apps.length) % apps.length;
    }

    selectApp(apps[nextIndex]);
  }

  const swipeHandlers = createSwipeHandlers(handleAppSwipe, undefined, {
    threshold: 60,
    maxDuration: 400,
    minVelocity: 0.25,
  });

  onMount(async () => {
    // Initialize theme system
    initTheme();

    // Check if mobile viewport
    isMobile = isMobileViewport();
    const handleResize = () => { isMobile = isMobileViewport(); };
    window.addEventListener('resize', handleResize);

    // First check auth status
    await checkAuthStatus();
    authChecked = true;

    // If not authenticated but auth is required, the API call will fail
    // Try to load config
    try {
      config = await fetchConfig();
      apps = config.apps;
      authRequired = config.auth?.method !== 'none' && config.auth?.method !== undefined && config.auth?.method !== '';

      // Initialize keybindings from config
      initKeybindings(config.keybindings);

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
    // Cleanup resize listener is handled by onMount return
  });

  async function handleLoginSuccess() {
    // Re-fetch config after login
    try {
      config = await fetchConfig();
      apps = config.apps;

      // Initialize keybindings from config
      initKeybindings(config.keybindings);

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
      toasts.success('Dashboard setup complete!');
    } catch (e) {
      console.error('Failed to save onboarding config:', e);
      toasts.error('Failed to save configuration');
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
      toasts.success('Settings saved successfully');
    } catch (e) {
      console.error('Failed to save config:', e);
      toasts.error('Failed to save configuration');
    }
  }

  // Handle command palette actions
  function handleCommandAction(actionId: string) {
    showCommandPalette = false;

    switch (actionId) {
      case 'search':
        showSearch = true;
        break;
      case 'settings':
        showSettings = true;
        break;
      case 'shortcuts':
        showShortcuts = true;
        break;
      case 'fullscreen':
        toggleFullscreen();
        break;
      case 'refresh':
        if (currentApp && !showSplash) {
          const frame = document.querySelector('iframe');
          if (frame) frame.src = frame.src;
        }
        break;
      case 'home':
        showSplash = true;
        break;
      case 'theme-dark':
        setTheme('dark');
        toasts.success('Switched to dark theme');
        break;
      case 'theme-light':
        setTheme('light');
        toasts.success('Switched to light theme');
        break;
      case 'theme-system':
        setTheme('system');
        toasts.success('Using system theme');
        break;
    }
  }

  function handleKeydown(event: KeyboardEvent) {
    // Don't trigger shortcuts when not authenticated or on login page
    if (authRequired && !$isAuthenticated) {
      return;
    }

    // Don't trigger shortcuts when typing in inputs (except Escape)
    if (event.target instanceof HTMLInputElement || event.target instanceof HTMLTextAreaElement) {
      if (event.key === 'Escape') {
        showSearch = false;
        showSettings = false;
      }
      return;
    }

    // Escape is always hardcoded for closing modals
    if (event.key === 'Escape') {
      if (showCommandPalette) showCommandPalette = false;
      else if (showSearch) showSearch = false;
      else if (showSettings) showSettings = false;
      else if (showShortcuts) showShortcuts = false;
      else if (!showSplash && currentApp) showSplash = true;
      return;
    }

    // Find the action for this key event using customizable keybindings
    const action = findAction(event);
    if (!action) return;

    // Prevent default for most actions
    event.preventDefault();

    // Execute the action
    executeAction(action);
  }

  function executeAction(action: KeyAction) {
    switch (action) {
      case 'search':
        showSearch = true;
        break;
      case 'commandPalette':
        showCommandPalette = true;
        break;
      case 'settings':
        showSettings = !showSettings;
        break;
      case 'shortcuts':
        showShortcuts = !showShortcuts;
        break;
      case 'home':
        showSplash = true;
        break;
      case 'refresh':
        if (currentApp && !showSplash) {
          const frame = document.querySelector('iframe');
          if (frame) frame.src = frame.src;
        }
        break;
      case 'fullscreen':
        toggleFullscreen();
        break;
      case 'nextApp':
        if (!showSearch && apps.length > 0) {
          const currentIndex = currentApp ? apps.findIndex(a => a.name === currentApp?.name) : -1;
          const nextIndex = (currentIndex + 1) % apps.length;
          selectApp(apps[nextIndex]);
        }
        break;
      case 'prevApp':
        if (!showSearch && apps.length > 0) {
          const currentIndex = currentApp ? apps.findIndex(a => a.name === currentApp?.name) : -1;
          const prevIndex = (currentIndex - 1 + apps.length) % apps.length;
          selectApp(apps[prevIndex]);
        }
        break;
      case 'app1':
      case 'app2':
      case 'app3':
      case 'app4':
      case 'app5':
      case 'app6':
      case 'app7':
      case 'app8':
      case 'app9':
        const appIndex = parseInt(action.replace('app', '')) - 1;
        if (apps[appIndex]) {
          selectApp(apps[appIndex]);
        }
        break;
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
    <ErrorState
      title="Failed to load dashboard"
      message={error}
      icon="network"
      on:retry={() => window.location.reload()}
    />
  </div>
{:else if config}
  <!-- Main layout container - direction changes based on nav position -->
  <div
    class="h-full bg-gray-900 dark:bg-gray-900"
    class:flex={!isFloatingLayout && !$isFullscreen}
    class:flex-row={isHorizontalLayout && navPosition === 'left' && !$isFullscreen}
    class:flex-row-reverse={isHorizontalLayout && navPosition === 'right' && !$isFullscreen}
    class:flex-col={!isHorizontalLayout && navPosition === 'top' && !$isFullscreen}
    class:flex-col-reverse={!isHorizontalLayout && navPosition === 'bottom' && !$isFullscreen}
  >
    <!-- Navigation (hidden in fullscreen mode) -->
    {#if !$isFullscreen}
      <Navigation
        {apps}
        {currentApp}
        {config}
        on:select={(e) => selectApp(e.detail)}
        on:search={() => showSearch = true}
        on:splash={() => showSplash = true}
        on:settings={() => showSettings = !showSettings}
      />
    {/if}

    <!-- Main content area - with swipe gesture support on mobile -->
    <main
      class="flex-1 overflow-hidden relative"
      bind:this={mainContentElement}
      on:pointerdown={isMobile ? swipeHandlers.onpointerdown : undefined}
      on:pointermove={isMobile ? swipeHandlers.onpointermove : undefined}
      on:pointerup={isMobile ? swipeHandlers.onpointerup : undefined}
      on:pointercancel={isMobile ? swipeHandlers.onpointercancel : undefined}
    >
      {#if showSplash && !$isFullscreen}
        <Splash {apps} {config} on:select={(e) => selectApp(e.detail)} />
      {:else if currentApp}
        <AppFrame app={currentApp} />
      {:else if $isFullscreen}
        <!-- Show splash content in fullscreen if no app selected -->
        <Splash {apps} {config} on:select={(e) => selectApp(e.detail)} />
      {/if}
    </main>

    <!-- Fullscreen exit button -->
    {#if $isFullscreen}
      <div class="fixed top-4 right-4 z-50 flex items-center gap-2">
        <button
          class="p-2 bg-gray-800/80 hover:bg-gray-700 text-white rounded-lg backdrop-blur-sm
                 border border-gray-600 shadow-lg transition-all opacity-30 hover:opacity-100"
          on:click={exitFullscreen}
          title="Exit fullscreen (F)"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    {/if}
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

  <!-- Command palette -->
  {#if showCommandPalette}
    <CommandPalette
      {apps}
      on:select={(e) => { selectApp(e.detail); showCommandPalette = false; }}
      on:action={(e) => handleCommandAction(e.detail)}
      on:close={() => showCommandPalette = false}
    />
  {/if}
{/if}

<!-- Toast notifications (always rendered) -->
<ToastContainer />
