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
  import { checkAuthStatus, isAuthenticated, isAdmin, login, setupRequired } from './lib/authStore';
  import { resetOnboarding } from './lib/onboardingStore';
  import { initTheme, setTheme, syncFromConfig, loadCustomThemesFromServer } from './lib/themeStore';
  import { isFullscreen, toggleFullscreen, exitFullscreen } from './lib/fullscreenStore';
  import { createSwipeHandlers, isMobileViewport, type SwipeResult } from './lib/useSwipe';
  import { findAction, initKeybindings, type KeyAction } from './lib/keybindingsStore';
  import { initDebug, debug } from './lib/debug';

  let config = $state<Config | null>(null);
  let apps = $state<App[]>([]);
  let currentApp = $state<App | null>(null);
  let showSplash = $state(true);
  let showSettings = $state(false);
  let settingsInitialTab = $state<'general' | 'apps' | 'theme' | 'keybindings' | 'security' | 'about'>('general');
  let settingsRef = $state<Settings | undefined>(undefined);
  let showShortcuts = $state(false);
  let showCommandPalette = $state(false);
  let showLogs = $state(false);
  let loading = $state(true);
  let error = $state<string | null>(null);

  // Toast position adapts to navigation position to avoid overlay
  let toastPosition = $derived.by(() => {
    const pos = config?.navigation?.position;
    if (pos === 'bottom') return 'top-right' as const;
    if (pos === 'right') return 'bottom-left' as const;
    return 'bottom-right' as const;
  });

  // Auth state
  let authRequired = $state(false);
  let authChecked = $state(false);

  // Onboarding state
  let showOnboarding = $state(false);

  // Version info (fetched after auth)
  let appVersion = $state('');

  /**
   * Resolve template variables in the dashboard title.
   * Supported: %title%, %url%, %group%, %version%
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
      .replaceAll('%version%', appVersion);

    // Clean up dangling separators around empty values
    // e.g. "Muximux -  - " → "Muximux"
    result = result.replaceAll(/\s*[—–\-|:]\s*(?=[—–\-|:\s]*$)/g, '');
    result = result.replaceAll(/\s*[—–\-|:]\s*[—–\-|:]\s*/g, ' — ');
    return result.trim() || 'Muximux';
  }

  // Dynamic favicon — renders the MuximuxLogo SVG at 32×32 onto a canvas,
  // converts to a PNG data URL, and swaps the favicon link.  Rasterising to
  // PNG avoids browser quirks with SVG data-URL favicons.
  let lastFaviconColor = '';
  const FAVICON_PATHS = 'M64.45 48C68.63 48 72.82 47.99 77.01 48.01 80.83 59.09 84.77 70.14 88.54 81.24 92.32 70.17 96.13 59.1 99.85 48c4.19-.01 8.39 0 12.58 0 .96 17.67 2.07 35.33 3.06 53-4.04 0-8.09.01-12.13-.01-.47-7.25-.89-14.51-1.29-21.76-2.41 7.26-4.92 14.5-7.36 21.76-4.1-.04-8.21.16-12.31-.14-2.47-7.49-5.04-14.95-7.71-22.37-.25 7.52-1.07 15-.33 22.52-4.08 0-8.17 0-12.26 0C63.17 83.33 64.36 65.67 64.45 48zM119.6 48c4.05 0 8.09 0 12.14.01 0 11-.02 22 0 33-.23 4.46 3.97 8.34 8.36 8.01 4.1-.11 7.54-3.94 7.43-7.99.02-11-.01-22 0-33.02 4.07 0 8.14 0 12.21.01-.07 11.48.11 22.97-.09 34.45-.51 11.15-11.73 20.11-22.71 18.4-9.3-1.1-17-9.52-17.32-18.86-.05-11.34-.01-22.67-.02-34zM165.5 48.03c4.79-.06 9.58-.02 14.37-.03 2.93 4.67 5.85 9.35 8.77 14.03 2.75-4.71 5.63-9.34 8.4-14.04 4.78.02 9.57 0 14.35.02-5.34 8.47-10.47 17.09-15.61 25.68 5.71 9.08 11.15 18.34 17.01 27.32-4.82-.04-9.64.04-14.46-.05-3.24-5.17-6.4-10.38-9.63-15.54-3.22 5.18-6.35 10.41-9.57 15.6-4.72-.04-9.45-.01-14.17-.02 5.59-9.09 11.04-18.26 16.57-27.38-5.53-8.41-10.43-17.22-16.03-25.59zM216.6 48c4.04 0 8.09 0 12.14.01-.01 29.67-.01 59.35 0 89.03.09 4.35.03 8.92-2.15 12.83-4.1 8.6-14.86 13.29-23.92 10.24-8.18-2.41-14.2-10.6-14.08-19.13.02-10.99 0-21.98.01-32.97 4.04 0 8.09-.01 12.14.01 0 10.98-.02 21.96 0 32.95-.26 4.5 4.01 8.44 8.44 8.05 4.07-.16 7.45-3.95 7.35-7.98-.02-31.01.12-62.02-.07-93.01zM133.45 108c4.18 0 8.37-.01 12.56.01 3.83 11.08 7.75 22.14 11.55 33.24 3.74-11.08 7.58-22.14 11.29-33.23 4.19-.02 8.39-.01 12.58 0 .96 17.67 2.07 35.33 3.06 53-4.05 0-8.09.01-12.13-.01-.47-7.24-.89-14.48-1.29-21.72-2.43 7.24-4.92 14.47-7.36 21.72-4.09-.02-8.19.12-12.27-.11-2.53-7.48-5.06-14.97-7.75-22.4-.25 7.52-1.08 15-.32 22.52-4.09 0-8.18 0-12.27 0C131.17 143.33 132.36 125.67 133.45 108zM234.5 108.03c4.79-.06 9.58-.02 14.37-.03 2.91 4.67 5.86 9.32 8.73 14.02 2.81-4.67 5.65-9.33 8.43-14.03 4.79.02 9.58 0 14.36.02-5.35 8.47-10.46 17.08-15.61 25.67 5.7 9.09 11.15 18.34 17.01 27.33-4.82-.04-9.64.04-14.46-.05-3.24-5.16-6.4-10.38-9.63-15.54-3.25 5.18-6.33 10.46-9.62 15.61-4.71-.08-9.41-.02-14.12-.04 5.59-9.09 11.04-18.26 16.57-27.38-5.53-8.41-10.43-17.22-16.03-25.59z';

  function updateFavicon() {
    const accentColor = getComputedStyle(document.documentElement).getPropertyValue('--accent-primary').trim();
    if (!accentColor || accentColor === lastFaviconColor) return;
    lastFaviconColor = accentColor;

    // Build SVG, render to canvas, convert to PNG data URL
    const size = 64;
    const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 341 207"><g fill="${accentColor}"><path d="${FAVICON_PATHS}"/></g></svg>`;
    const blob = new Blob([svg], { type: 'image/svg+xml;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const img = new Image();
    img.onload = () => {
      const canvas = document.createElement('canvas');
      canvas.width = size;
      canvas.height = size;
      const ctx = canvas.getContext('2d')!;
      // Center the wide logo in the square canvas
      const scale = size / Math.max(341, 207);
      const w = 341 * scale, h = 207 * scale;
      ctx.drawImage(img, (size - w) / 2, (size - h) / 2, w, h);
      URL.revokeObjectURL(url);

      const pngUrl = canvas.toDataURL('image/png');
      // Remove old and insert new link to force browser to pick it up
      const old = document.getElementById('favicon-dynamic');
      if (old) old.remove();
      const link = document.createElement('link');
      link.id = 'favicon-dynamic';
      link.rel = 'icon';
      link.type = 'image/png';
      link.href = pngUrl;
      document.head.prepend(link);
    };
    img.src = url;
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
    // Initialize debug logging (must come before other inits)
    initDebug();

    // Initialize theme system
    initTheme();

    // Sync favicon accent color with the current theme, and watch for
    // attribute changes on <html> (the theme store sets data-theme / style).
    updateFavicon();
    const faviconObserver = new MutationObserver(() => updateFavicon());
    faviconObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['style', 'data-theme', 'class'] });

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

    // Auto-login after auth method transition (credentials stored by Settings)
    const autoLogin = sessionStorage.getItem('muximux_auto_login');
    if (autoLogin) {
      sessionStorage.removeItem('muximux_auto_login');
      try {
        const { u, p } = JSON.parse(autoLogin);
        if (u && p) {
          await login(u, p, true);
        }
      } catch {
        // Fall through — user will see the login form
      }
    }

    // Try to load config (will 401 if auth required and not logged in)
    try {
      config = await fetchConfig();
      apps = config.apps;
      debug('config', 'loaded', { apps: apps.length, auth: config.auth?.method, health: config.health?.interval });
      authRequired = config.auth?.method !== 'none' && config.auth?.method !== undefined;

      // Sync theme from server config (keeps localStorage in sync across browsers)
      if (config.theme) {
        syncFromConfig(config.theme);
      }

      // Load custom themes from server (non-blocking)
      loadCustomThemesFromServer();

      // Initialize keybindings from config
      initKeybindings(config.keybindings);

      // Show onboarding when no apps are configured
      if (apps.length === 0) {
        resetOnboarding();
        showOnboarding = true;
        loading = false;
        return;
      }

      // Fetch version info (non-blocking)
      fetchSystemInfo().then(info => appVersion = info.version).catch(() => {});

      // Check if we should return to a specific section (e.g. after auth method change reload)
      const returnTo = sessionStorage.getItem('muximux_return_to');
      if (returnTo) {
        sessionStorage.removeItem('muximux_return_to');
        settingsInitialTab = returnTo as typeof settingsInitialTab;
        showSettings = true;
      }

      showDefaultApp();
      startServices();

      // Listen for config updates via WebSocket
      onWsEvent('config_updated', (payload) => {
        const newConfig = payload as Config;
        config = newConfig;
        apps = newConfig.apps;
        debug('config', 'updated via ws', { apps: newConfig.apps.length });
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

      // Load custom themes from server
      loadCustomThemesFromServer();

      // Initialize keybindings from config
      initKeybindings(config.keybindings);

      // Refresh version info
      fetchSystemInfo().then(info => appVersion = info.version).catch(() => {});

      // Check if we should return to a specific section (e.g. after auth method change)
      const returnTo = sessionStorage.getItem('muximux_return_to');
      if (returnTo) {
        sessionStorage.removeItem('muximux_return_to');
        settingsInitialTab = returnTo as typeof settingsInitialTab;
        showSettings = true;
      }

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
    authRequired = true;
    config = null;
    apps = [];
    currentApp = null;
    showSplash = true;
    showSettings = false;
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
      showLogs = false;
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
        if (get(isAdmin)) showSettings = true;
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
        if (showCommandPalette) showCommandPalette = false;
        else if (showSettings) {
          if (!settingsRef?.handleEscape()) showSettings = false;
        }
      }
      return;
    }

    // Escape is always hardcoded for closing modals
    if (event.key === 'Escape') {
      if (showCommandPalette) showCommandPalette = false;
      else if (showSettings) {
        // Let Settings close its sub-modals first; only close Settings itself if no sub-modal was open
        if (!settingsRef?.handleEscape()) showSettings = false;
      }
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
    debug('keys', 'action', action);
    switch (action) {
      case 'search':
        showCommandPalette = true;
        break;
      case 'settings':
        if ($isAdmin) showSettings = !showSettings;
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
        const slot = parseInt(action.replace('app', ''));
        // First try to find an app with this shortcut explicitly assigned
        const shortcutApp = apps.find(a => a.shortcut === slot);
        if (shortcutApp) {
          selectApp(shortcutApp);
        } else {
          // Fall back to positional index for apps without explicit shortcuts
          const appIndex = slot - 1;
          if (apps[appIndex]) {
            selectApp(apps[appIndex]);
          }
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
    class="h-full overflow-hidden"
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
        <Splash {apps} {config} onselect={(app) => selectApp(app)} onsettings={$isAdmin ? () => showSettings = true : undefined} onabout={() => { settingsInitialTab = 'about'; showSettings = true; }} />
      {:else if showLogs}
        <Logs onclose={() => { showLogs = false; showSplash = true; }} />
      {:else if currentApp}
        <AppFrame app={currentApp} />
      {:else if $isFullscreen}
        <!-- Show splash content in fullscreen if no app selected -->
        <Splash {apps} {config} onselect={(app) => selectApp(app)} onsettings={$isAdmin ? () => showSettings = true : undefined} onabout={() => { settingsInitialTab = 'about'; showSettings = true; }} />
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
      bind:this={settingsRef}
      {config}
      {apps}
      initialTab={settingsInitialTab}
      onclose={() => { showSettings = false; settingsInitialTab = 'general'; }}
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

<!-- Toast notifications (always rendered, position adapts to nav) -->
<Toaster position={toastPosition} theme="dark" richColors />

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
