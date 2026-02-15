<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import Navigation from './components/Navigation.svelte';
  import AppFrame from './components/AppFrame.svelte';
  import Splash from './components/Splash.svelte';
  import Settings from './components/Settings.svelte';
  import ShortcutsHelp from './components/ShortcutsHelp.svelte';
  import CommandPalette from './components/CommandPalette.svelte';
  import Login from './components/Login.svelte';
  import OnboardingWizard from './components/OnboardingWizard.svelte';
  import Logs from './components/Logs.svelte';
  import { Toaster } from 'svelte-sonner';
  import ErrorState from './components/ErrorState.svelte';
  import { getEffectiveUrl, type App, type Config, type NavigationConfig, type Group, type ThemeConfig } from './lib/types';
  import { fetchConfig, saveConfig, submitSetup, fetchSystemInfo } from './lib/api';
  import { toasts } from './lib/toastStore';
  import { startHealthPolling, stopHealthPolling } from './lib/healthStore';
  import { connect as connectWs, disconnect as disconnectWs, on as onWsEvent } from './lib/websocketStore';
  import { initLogStore } from './lib/logStore';
  import { get } from 'svelte/store';
  import { checkAuthStatus, logout, isAuthenticated, setupRequired } from './lib/authStore';
  import { resetOnboarding } from './lib/onboardingStore';
  import { initTheme, setTheme, syncFromConfig } from './lib/themeStore';
  import { isFullscreen, toggleFullscreen, exitFullscreen } from './lib/fullscreenStore';
  import { createSwipeHandlers, isMobileViewport, type SwipeResult } from './lib/useSwipe';
  import { findAction, initKeybindings, type KeyAction } from './lib/keybindingsStore';
  import { captureKeybindings, isProtectedKey, toggleCaptureKeybindings } from './lib/keybindingCaptureStore';

  let config = $state<Config | null>(null);
  let apps = $state<App[]>([]);
  let currentApp = $state<App | null>(null);
  let showSplash = $state(true);
  let showSettings = $state(false);
  let showShortcuts = $state(false);
  let showCommandPalette = $state(false);
  let showLogs = $state(false);
  let loading = $state(true);
  let error = $state<string | null>(null);

  // Auth state
  let authRequired = $state(false);
  let authChecked = $state(false);

  // Onboarding state
  let showOnboarding = $state(false);

  // Version info (fetched once for title variables)
  let appVersion = $state('');
  fetchSystemInfo().then(info => appVersion = info.version).catch(() => {});

  /**
   * Resolve template variables in the dashboard title.
   * Supported: %title%, %url%, %group%, %version%, %count%
   * When a variable resolves to empty, surrounding separators are cleaned up.
   */
  function resolveTitle(template: string, app: App | null): string {
    if (!template) return 'Muximux';
    const hasVariables = template.includes('%');
    if (!hasVariables) {
      // Legacy behavior: prepend app name if no variables are used
      return app ? `${app.name} — ${template}` : template;
    }

    let result = template
      .replaceAll('%title%', app?.name || '')
      .replaceAll('%url%', app?.url || '')
      .replaceAll('%group%', app?.group || '')
      .replaceAll('%version%', appVersion)
      .replaceAll('%count%', String(apps.length));

    // Clean up dangling separators around empty values
    // e.g. "Muximux -  - " → "Muximux"
    result = result.replaceAll(/\s*[—–\-|:]\s*(?=[—–\-|:\s]*$)/g, '');
    result = result.replaceAll(/\s*[—–\-|:]\s*[—–\-|:]\s*/g, ' — ');
    return result.trim() || 'Muximux';
  }

  // Computed layout properties
  let navPosition = $derived(config?.navigation.position || 'top');
  let isHorizontalLayout = $derived(navPosition === 'left' || navPosition === 'right');
  let isFloatingLayout = $derived(navPosition === 'floating');

  // Mobile swipe state
  let isMobile = $state(false);
  let mainContentElement = $state<HTMLElement | undefined>(undefined);

  function parseIntervalMs(intervalStr: string, fallback = 30000): number {
    const match = intervalStr.match(/^(\d+)(ms|s|m)?$/);
    if (!match) return fallback;
    const value = parseInt(match[1], 10);
    const unit = match[2] || 's';
    if (unit === 'ms') return value;
    if (unit === 'm') return value * 60 * 1000;
    return value * 1000;
  }

  function showDefaultApp() {
    if (!config || config.navigation.show_splash_on_startup) return;
    const defaultApp = apps.find(app => app.default);
    if (defaultApp) {
      currentApp = defaultApp;
      showSplash = false;
    }
  }

  function startServices() {
    if (!config) return;
    if (config.health?.enabled !== false) {
      const intervalMs = parseIntervalMs(config.health?.interval || '30s');
      startHealthPolling(intervalMs);
    }
    connectWs();
    initLogStore();
  }

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

    // If setup is needed, go straight to unified wizard
    if (get(setupRequired)) {
      showOnboarding = true;
      loading = false;
      return;
    }

    // If not authenticated but auth is required, the API call will fail
    // Try to load config
    try {
      config = await fetchConfig();
      apps = config.apps;
      authRequired = config.auth?.method !== 'none' && config.auth?.method !== undefined;

      // Sync theme from server config (keeps localStorage in sync across browsers)
      if (config.theme) {
        syncFromConfig(config.theme);
      }

      // Initialize keybindings from config
      initKeybindings(config.keybindings);

      // Show onboarding when no apps are configured
      if (apps.length === 0) {
        resetOnboarding();
        showOnboarding = true;
        loading = false;
        return;
      }

      showDefaultApp();
      startServices();

      // Listen for config updates via WebSocket
      onWsEvent('config_updated', (payload) => {
        const newConfig = payload as Config;
        config = newConfig;
        apps = newConfig.apps;
        // Sync theme if changed from another session
        if (newConfig.theme) {
          syncFromConfig(newConfig.theme);
        }
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

      // Sync theme from server config
      if (config.theme) {
        syncFromConfig(config.theme);
      }

      // Initialize keybindings from config
      initKeybindings(config.keybindings);

      showDefaultApp();
      startServices();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load configuration';
    }
  }

  async function handleLogout() {
    // logout() is already called by Navigation before this fires,
    // so just clean up services and reset client state.
    stopHealthPolling();
    disconnectWs();
    config = null;
    apps = [];
    currentApp = null;
    showSplash = true;
  }

  async function handleOnboardingComplete(detail: { apps: App[]; navigation: NavigationConfig; groups: Group[]; theme: ThemeConfig; setup?: import('./lib/types').SetupRequest }) {
    const { apps: newApps, navigation, groups, theme, setup } = detail;

    try {
      // Submit security setup first (if this was initial setup)
      if (setup) {
        const resp = await submitSetup(setup);
        if (!resp.success) {
          toasts.error(resp.error || 'Security setup failed');
          return;
        }
        // Re-check auth status and load config now that guard is down
        await checkAuthStatus();
        config = await fetchConfig();
        apps = config.apps;
        authRequired = config.auth?.method !== 'none' && config.auth?.method !== undefined;
      }

      if (!config) return;

      // Update config with onboarding selections
      const newConfig: Config = {
        ...config,
        navigation: {
          ...config.navigation,
          ...navigation
        },
        theme,
        groups,
        apps: newApps
      };

      const saved = await saveConfig(newConfig);
      config = saved;
      apps = saved.apps;

      // After onboarding, always show the overview (splash) page
      showSplash = true;
      currentApp = null;

      startServices();

      // Hide onboarding
      showOnboarding = false;
      toasts.success('Dashboard setup complete!');
    } catch (e) {
      console.error('Failed to save onboarding config:', e);
      toasts.error('Failed to save configuration');
    }
  }

  function selectApp(app: App) {
    const url = getEffectiveUrl(app);
    if (app.open_mode === 'new_tab') {
      window.open(url, '_blank');
    } else if (app.open_mode === 'new_window') {
      window.open(url, app.name);
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
      case 'logs':
        showLogs = true;
        showSplash = false;
        currentApp = null;
        break;
      case 'toggle-keybindings':
        toggleCaptureKeybindings();
        toasts.success($captureKeybindings ? 'Keyboard shortcuts enabled' : 'Keyboard shortcuts paused');
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
        showCommandPalette = false;
        showSettings = false;
      }
      return;
    }

    // Gate shortcuts: check global capture toggle and per-app disable setting
    const appDisablesShortcuts = currentApp && !showSplash && currentApp.disable_keyboard_shortcuts;
    const shouldCapture = $captureKeybindings && !appDisablesShortcuts;
    if (!shouldCapture && !isProtectedKey(event)) return;

    // Escape is always hardcoded for closing modals
    if (event.key === 'Escape') {
      if (showCommandPalette) showCommandPalette = false;
      else if (showSettings) showSettings = false;
      else if (showShortcuts) showShortcuts = false;
      else if (showLogs) { showLogs = false; showSplash = true; }
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
      case 'logs':
        showLogs = true;
        showSplash = false;
        currentApp = null;
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
        if (apps.length > 0) {
          const currentIndex = currentApp ? apps.findIndex(a => a.name === currentApp?.name) : -1;
          const nextIndex = (currentIndex + 1) % apps.length;
          selectApp(apps[nextIndex]);
        }
        break;
      case 'prevApp':
        if (apps.length > 0) {
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
      case 'app9': {
        const appIndex = parseInt(action.replace('app', '')) - 1;
        if (apps[appIndex]) {
          selectApp(apps[appIndex]);
        }
        break;
      }
    }
  }
</script>

<!-- Dynamic page title -->
<svelte:head>
  <title>{resolveTitle(config?.title || 'Muximux', currentApp)}</title>
</svelte:head>

<svelte:window onkeydown={handleKeydown} />

{#if loading || !authChecked}
  <div class="flex items-center justify-center h-full" style="background: var(--bg-base);">
    <div class="text-center">
      <div class="animate-spin rounded-full h-12 w-12 border-b-2 mx-auto" style="border-color: var(--accent-primary);"></div>
      <p class="mt-4" style="color: var(--text-muted);">Loading Muximux...</p>
    </div>
  </div>
{:else if $setupRequired || showOnboarding}
  <OnboardingWizard
    needsSetup={$setupRequired}
    oncomplete={handleOnboardingComplete}
  />
{:else if authRequired && !$isAuthenticated}
  <Login onsuccess={handleLoginSuccess} />
{:else if error}
  <div class="flex items-center justify-center h-full" style="background: var(--bg-base);">
    <ErrorState
      title="Failed to load dashboard"
      message={error}
      icon="network"
      onretry={() => window.location.reload()}
    />
  </div>
{:else if config}
  <!-- Main layout container - direction changes based on nav position -->
  <div
    class="h-full"
    style="background: var(--bg-base);"
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
        {showSplash}
        onselect={(app) => selectApp(app)}
        onsearch={() => showCommandPalette = true}
        onsplash={() => showSplash = true}
        onsettings={() => showSettings = !showSettings}
        onlogs={() => { showLogs = true; showSplash = false; currentApp = null; }}
        onlogout={handleLogout}
      />
    {/if}

    <!-- Main content area - with swipe gesture support on mobile -->
    <main
      class="flex-1 overflow-hidden relative"
      bind:this={mainContentElement}
      onpointerdown={isMobile ? swipeHandlers.onpointerdown : undefined}
      onpointermove={isMobile ? swipeHandlers.onpointermove : undefined}
      onpointerup={isMobile ? swipeHandlers.onpointerup : undefined}
      onpointercancel={isMobile ? swipeHandlers.onpointercancel : undefined}
    >
      {#if showSplash && !$isFullscreen}
        <Splash {apps} {config} onselect={(app) => selectApp(app)} onsettings={() => showSettings = true} />
      {:else if showLogs}
        <Logs onclose={() => { showLogs = false; showSplash = true; }} />
      {:else if currentApp}
        <AppFrame app={currentApp} />
      {:else if $isFullscreen}
        <!-- Show splash content in fullscreen if no app selected -->
        <Splash {apps} {config} onselect={(app) => selectApp(app)} onsettings={() => showSettings = true} />
      {/if}
    </main>

    <!-- Fullscreen exit button -->
    {#if $isFullscreen}
      <div class="fixed top-4 right-4 z-50 flex items-center gap-2">
        <button
          class="fullscreen-exit-btn p-2 rounded-lg backdrop-blur-sm shadow-lg transition-all opacity-30 hover:opacity-100"
          onclick={exitFullscreen}
          title="Exit fullscreen (F)"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    {/if}
  </div>

  <!-- Settings panel -->
  {#if showSettings}
    <Settings
      {config}
      {apps}
      onclose={() => showSettings = false}
      onsave={(newConfig) => handleSaveConfig(newConfig)}
    />
  {/if}

  <!-- Keyboard shortcuts help -->
  {#if showShortcuts}
    <ShortcutsHelp onclose={() => showShortcuts = false} />
  {/if}

  <!-- Command palette -->
  {#if showCommandPalette}
    <CommandPalette
      {apps}
      onselect={(app) => { selectApp(app); showCommandPalette = false; }}
      onaction={(actionId) => handleCommandAction(actionId)}
      onclose={() => showCommandPalette = false}
    />
  {/if}
{/if}

<!-- Toast notifications (always rendered) -->
<Toaster position="bottom-right" theme="dark" richColors />

<style>
  .fullscreen-exit-btn {
    background: var(--glass-bg);
    color: var(--text-primary);
    border: 1px solid var(--border-default);
  }

  .fullscreen-exit-btn:hover {
    background: var(--bg-hover);
  }
</style>
