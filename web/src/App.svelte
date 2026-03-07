<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { SvelteSet } from 'svelte/reactivity';
  import Navigation from './components/Navigation.svelte';
  import AppFrame from './components/AppFrame.svelte';
  import Splash from './components/Splash.svelte';
  import Login from './components/Login.svelte';
  import { Toaster } from 'svelte-sonner';
  import ErrorState from './components/ErrorState.svelte';
  import { getEffectiveUrl, type App, type Config, type NavigationConfig, type Group, type ThemeConfig } from './lib/types';
  import { fetchConfig, saveConfig, submitSetup, fetchSystemInfo, slugify } from './lib/api';
  import { toasts } from './lib/toastStore';
  import { startHealthPolling, stopHealthPolling } from './lib/healthStore';
  import { connect as connectWs, disconnect as disconnectWs, on as onWsEvent, connectionState } from './lib/websocketStore';
  import { initLogStore } from './lib/logStore';
  import { get } from 'svelte/store';
  import { checkAuthStatus, isAuthenticated, isAdmin, login, setupRequired } from './lib/authStore';
  import { resetOnboarding } from './lib/onboardingStore';
  import { initTheme, setTheme, syncFromConfig, loadCustomThemesFromServer } from './lib/themeStore';
  import { isFullscreen, toggleFullscreen, exitFullscreen } from './lib/fullscreenStore';
  import { createSwipeHandlers, isMobileViewport, type SwipeResult } from './lib/useSwipe';
  import { findAction, initKeybindings, type KeyAction } from './lib/keybindingsStore';
  import { initDebug, debug } from './lib/debug';
  import { syncFaviconsWithTheme } from './lib/favicon';
  import { syncLocaleFromConfig } from './lib/localeStore';
  import { getLocale } from '$lib/paraglide/runtime.js';
  import * as m from '$lib/paraglide/messages.js';
  import { splitState, enableSplit, disableSplit, setActivePanel, setPanelApp, updateDividerPosition, resetSplit } from './lib/splitStore.svelte';
  import SplitDivider from './components/SplitDivider.svelte';

  let config = $state<Config | null>(null);
  let apps = $state<App[]>([]);
  let currentApp = $derived(splitState.panels[splitState.activePanel]);
  let showSplash = $state(true);
  let showSettings = $state(false);
  let settingsInitialTab = $state<'general' | 'apps' | 'theme' | 'keybindings' | 'security' | 'about'>('general');
  // eslint-disable-next-line @typescript-eslint/no-explicit-any -- dynamic imports lose component typing
  type LazyComponent = any;
  let settingsRef = $state<LazyComponent>(undefined);
  let showShortcuts = $state(false);
  let showCommandPalette = $state(false);
  let showLogs = $state(false);

  // Lazy-loaded components (code splitting)
  let SettingsComponent = $state<LazyComponent>(null);
  let ShortcutsHelpComponent = $state<LazyComponent>(null);
  let CommandPaletteComponent = $state<LazyComponent>(null);
  let OnboardingWizardComponent = $state<LazyComponent>(null);
  let LogsComponent = $state<LazyComponent>(null);

  async function loadSettings() {
    if (!SettingsComponent) SettingsComponent = (await import('./components/Settings.svelte')).default;
  }
  async function loadShortcutsHelp() {
    if (!ShortcutsHelpComponent) ShortcutsHelpComponent = (await import('./components/ShortcutsHelp.svelte')).default;
  }
  async function loadCommandPalette() {
    if (!CommandPaletteComponent) CommandPaletteComponent = (await import('./components/CommandPalette.svelte')).default;
  }
  async function loadOnboardingWizard() {
    if (!OnboardingWizardComponent) OnboardingWizardComponent = (await import('./components/OnboardingWizard.svelte')).default;
  }
  async function loadLogs() {
    if (!LogsComponent) LogsComponent = (await import('./components/Logs.svelte')).default;
  }
  let loading = $state(true);
  let error = $state<string | null>(null);

  // WebSocket connection toast tracking
  let unsubWs: (() => void) | null = null;

  // Iframe caching: track which apps the user has visited so their iframes stay alive
  let visitedAppNames = new SvelteSet<string>();
  let visitedApps = $derived(apps.filter(a => visitedAppNames.has(a.name)));

  let visitedOrder: string[] = []; // LRU order — most recent at end

  function trackVisit(appName: string) {
    // Move to end of LRU list
    visitedOrder = visitedOrder.filter(n => n !== appName);
    visitedOrder.push(appName);
    visitedAppNames.add(appName);

    // Evict oldest if over limit
    const limit = config?.navigation.max_open_tabs || 0;
    if (limit > 0) {
      while (visitedOrder.length > limit) {
        const oldest = visitedOrder.shift()!;
        // Don't evict currently visible apps (in any split panel)
        if (splitState.panels[0]?.name === oldest || splitState.panels[1]?.name === oldest) {
          visitedOrder.push(oldest); // Put back
          break; // Can't evict without removing visible apps
        }
        visitedAppNames.delete(oldest);
      }
    }
  }

  // Toast position adapts to navigation position to avoid overlay
  let toastPosition = $derived.by(() => {
    if (isMobile) return 'top-right' as const;  // avoid FAB overlap
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
  function resolveTitle(template: string): string {
    if (!template) return 'Muximux';

    let appTitle: string;
    if (splitState.enabled) {
      const left = splitState.panels[0]?.name || m.common_select();
      const right = splitState.panels[1]?.name || m.common_select();
      appTitle = `${left} :: ${right}`;
    } else if (currentApp) {
      appTitle = currentApp.name;
    } else {
      appTitle = showSplash ? m.common_overview() : '';
    }

    // Replace other variables first (may be empty), then clean up separators
    // before inserting %title% (which may contain :: in split mode).
    let result = template
      .replaceAll('%url%', currentApp?.url || '')
      .replaceAll('%group%', currentApp?.group || '')
      .replaceAll('%version%', appVersion);

    // Clean up dangling separators around empty non-title values
    result = result.replaceAll(/\s*[—–\-|:]\s*(?=[—–\-|:\s]*$)/g, '');
    result = result.replaceAll(/\s*[—–\-|:]\s*[—–\-|:]\s*/g, ' — ');

    // Insert title last so :: in split titles isn't mangled by cleanup
    if (appTitle) {
      result = result.replaceAll('%title%', appTitle);
    } else {
      result = result.replaceAll('%title%', '');
      result = result.replaceAll(/^\s*[—–\-|:]\s*/g, '');
      result = result.replaceAll(/\s*[—–\-|:]\s*$/g, '');
    }
    return result.trim() || 'Muximux';
  }

  // Hash clearing is handled explicitly by navigateHome() and clearHash().
  // No reactive $effect needed — all "go home" code paths call clearHash() directly.

  // Mobile swipe state
  let isMobile = $state(false);
  let mainContentElement = $state<HTMLElement | undefined>(undefined);

  // Computed layout properties — force floating nav on mobile
  let navPosition = $derived(isMobile ? 'floating' : (config?.navigation.position || 'top'));
  let isHorizontalLayout = $derived(navPosition === 'left' || navPosition === 'right');


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
    // Hash deep-link takes priority (e.g. /#Plex)
    if (selectAppFromHash()) return;
    if (!config || config.navigation.show_splash_on_startup) return;
    const defaultApp = apps.find(app => app.default);
    if (defaultApp) {
      selectApp(defaultApp);
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

    // Sync all favicons (tab icon, apple-touch-icon, manifest, theme-color)
    // with the current theme's --accent-primary, and re-sync on theme changes.
    syncFaviconsWithTheme();
    const faviconObserver = new MutationObserver(() => syncFaviconsWithTheme());
    faviconObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['style', 'data-theme', 'class'] });

    // Register service worker for offline asset caching
    if ('serviceWorker' in navigator) {
      navigator.serviceWorker.register('./sw.js').catch(() => {});
    }

    // Check if mobile viewport
    isMobile = isMobileViewport();
    let resizeTimer: ReturnType<typeof setTimeout>;
    const handleResize = () => { clearTimeout(resizeTimer); resizeTimer = setTimeout(() => { isMobile = isMobileViewport(); }, 100); };
    window.addEventListener('resize', handleResize);

    // Auto-switch active split panel when the user clicks inside an iframe.
    document.addEventListener('focus', handleIframeFocus, true);

    // Handle browser back/forward with hash-based app routing
    window.addEventListener('hashchange', () => {
      if (location.hash) {
        selectAppFromHash();
      } else {
        // Hash cleared (e.g. navigating to /) — go home
        showSettings = false;
        showLogs = false;
        if (splitState.panels[0] || splitState.panels[1]) resetSplit();
        showSplash = true;
      }
    });

    // First check auth status
    await checkAuthStatus();
    authChecked = true;

    // If setup is needed, go straight to unified wizard
    if (get(setupRequired)) {
      await loadOnboardingWizard();
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

      // Sync locale from server config (may trigger reload if different from localStorage)
      if (config.language && config.language !== getLocale()) {
        syncLocaleFromConfig(config.language);
        return; // reload will re-run onMount
      }

      // Inject PWA manifest now that auth has passed — deferred from index.html
      // so forward-auth proxies don't redirect the manifest fetch to a login page.
      if (!document.querySelector('link[rel="manifest"]')) {
        const link = document.createElement('link');
        link.rel = 'manifest';
        link.href = './manifest.json';
        document.head.appendChild(link);
      }

      // Load custom themes from server (non-blocking)
      loadCustomThemesFromServer();

      // Initialize keybindings from config
      initKeybindings(config.keybindings);

      // Show onboarding when no apps are configured
      if (apps.length === 0) {
        resetOnboarding();
        await loadOnboardingWizard();
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
        openSettings();
      }

      showDefaultApp();
      startServices();

      // Show toast notifications on WebSocket disconnect/reconnect
      let wasConnected = false;
      unsubWs = connectionState.subscribe((state) => {
        if (state === 'connected') {
          if (wasConnected) {
            toasts.success(m.toast_connectionRestored());
          }
          wasConnected = true;
        } else if (state === 'disconnected' && wasConnected) {
          toasts.warning(m.toast_connectionLost());
        }
      });

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
        // Reset panels if their apps no longer exist
        if (splitState.panels[0] && !apps.find(a => a.name === splitState.panels[0]?.name)) {
          splitState.panels[0] = null;
        }
        if (splitState.panels[1] && !apps.find(a => a.name === splitState.panels[1]?.name)) {
          splitState.panels[1] = null;
        }
        if (!splitState.panels[0] && !splitState.panels[1]) {
          resetSplit();
          showSplash = true;
        }
        // Prune cached iframes for apps that no longer exist or are disabled
        const validNames = new Set(apps.filter(a => a.enabled).map(a => a.name));
        for (const name of visitedAppNames) {
          if (!validNames.has(name)) visitedAppNames.delete(name);
        }
      });

      loading = false;
    } catch (e) {
      // If we get a 401, auth is required
      if (e instanceof Error && e.message.includes('401')) {
        authRequired = true;
        loading = false;
      } else {
        error = e instanceof Error ? e.message : m.error_failedLoadConfig();
        loading = false;
      }
    }
  });

  onDestroy(() => {
    unsubWs?.();
    stopHealthPolling();
    disconnectWs();
    document.removeEventListener('focus', handleIframeFocus, true);
  });

  async function handleLoginSuccess() {
    // Fix URL — the backend redirects unauthenticated requests to /login,
    // but the SPA handles auth client-side so we restore the root path.
    if (location.pathname.endsWith('/login')) {
      const base = location.pathname.replace(/\/login$/, '') || '/';
      history.replaceState(null, '', base + location.hash);
    }

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
        openSettings();
      }

      showDefaultApp();
      startServices();
    } catch (e) {
      error = e instanceof Error ? e.message : m.error_failedLoadConfig();
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
    resetSplit();
    visitedAppNames.clear();
    visitedOrder = [];
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
          toasts.error(resp.error || m.toast_securitySetupFailed());
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
        language: getLocale(),
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
      resetSplit();

      startServices();

      // Hide onboarding
      showOnboarding = false;
      toasts.success(m.toast_dashboardSetupComplete());
    } catch (e) {
      console.error('Failed to save onboarding config:', e);
      toasts.error(m.toast_failedSaveConfig());
    }
  }

  function selectApp(app: App) {
    const url = getEffectiveUrl(app);
    if (app.open_mode === 'new_tab') {
      window.open(url, '_blank');
    } else if (app.open_mode === 'new_window') {
      window.open(url, app.name);
    } else {
      trackVisit(app.name);
      setPanelApp(app);
      showSplash = false;
      showLogs = false;
      updateHash();
    }
  }

  function updateHash() {
    if (splitState.enabled && splitState.panels[0] && splitState.panels[1]) {
      history.replaceState(null, '', '#' + slugify(splitState.panels[0].name) + '+' + slugify(splitState.panels[1].name));
    } else {
      const app = splitState.panels[0] || splitState.panels[1];
      if (app) {
        history.replaceState(null, '', '#' + slugify(app.name));
      }
    }
  }

  function clearHash() {
    if (location.hash) {
      history.replaceState(null, '', location.pathname + location.search);
    }
  }

  function navigateHome() {
    showSplash = true;
    showLogs = false;
    showSettings = false;
    resetSplit();
    clearHash();
  }

  async function openSettings() {
    await loadSettings();
    showSettings = true;
    history.replaceState(null, '', '#settings');
  }

  async function openLogs() {
    await loadLogs();
    showLogs = true;
    showSplash = false;
    resetSplit();
    history.replaceState(null, '', '#logs');
  }

  /** Try to select the app whose slug matches the URL hash (e.g. /#plex, /#my-cool-app, /#plex+sonarr).
   *  Also handles reserved hashes: #settings, #logs, #overview. */
  function selectAppFromHash(): boolean {
    const hash = location.hash.slice(1);
    if (!hash || !apps.length) return false;

    // Reserved hashes
    if (hash === 'settings') {
      if (get(isAdmin)) loadSettings().then(() => showSettings = true);
      return true;
    }
    if (hash === 'logs') {
      loadLogs().then(() => { showLogs = true; showSplash = false; });
      return true;
    }
    if (hash === 'overview') {
      showSplash = true;
      showLogs = false;
      clearHash();
      return true;
    }

    if (hash.includes('+')) {
      const [slug1, slug2] = hash.split('+', 2);
      const app1 = apps.find(a => slugify(a.name) === slug1);
      const app2 = apps.find(a => slugify(a.name) === slug2);
      if (app1 && app2 && !isMobile) {
        trackVisit(app1.name);
        trackVisit(app2.name);
        if (!splitState.enabled) enableSplit('horizontal');
        splitState.activePanel = 0;
        setPanelApp(app1);
        splitState.activePanel = 1;
        setPanelApp(app2);
        showSplash = false;
        showLogs = false;
        return true;
      }
      if (app1) {
        trackVisit(app1.name);
        setPanelApp(app1);
        showSplash = false;
        showLogs = false;
        updateHash();
        return true;
      }
    }

    const app = apps.find(a => slugify(a.name) === hash);
    if (app) {
      selectApp(app);
      return true;
    }
    return false;
  }

  async function handleSaveConfig(newConfig: Config) {
    try {
      const saved = await saveConfig(newConfig);
      config = saved;
      apps = saved.apps;
      // Reset panels if their apps no longer exist
      if (splitState.panels[0] && !apps.find(a => a.name === splitState.panels[0]?.name)) {
        splitState.panels[0] = null;
      }
      if (splitState.panels[1] && !apps.find(a => a.name === splitState.panels[1]?.name)) {
        splitState.panels[1] = null;
      }
      if (!splitState.panels[0] && !splitState.panels[1]) {
        resetSplit();
        showSplash = true;
      }
      // Prune cached iframes for apps that no longer exist or are disabled
      const validNames = new Set(apps.filter(a => a.enabled).map(a => a.name));
      for (const name of visitedAppNames) {
        if (!validNames.has(name)) visitedAppNames.delete(name);
      }
      toasts.success(m.toast_settingsSaved());

      // If language changed, sync locale and reload
      if (saved.language && saved.language !== getLocale()) {
        syncLocaleFromConfig(saved.language);
        return; // reload will happen
      }
    } catch (e) {
      console.error('Failed to save config:', e);
      toasts.error(m.toast_failedSaveConfig());
    }
  }

  // Iframe clicks don't bubble to the parent DOM, but the <iframe> element
  // receives a focus event (capture phase needed because focus doesn't bubble).
  function handleIframeFocus(e: FocusEvent) {
    if (!splitState.enabled) return;
    const target = e.target;
    if (!(target instanceof HTMLIFrameElement)) return;
    const panel = target.closest('[aria-label="Split panel 1"]') ? 0
      : target.closest('[aria-label="Split panel 2"]') ? 1
      : null;
    if (panel !== null) setActivePanel(panel);
  }

  function refreshActiveApp() {
    if (!currentApp || showSplash) return;
    const frame = document.querySelector<HTMLIFrameElement>(`iframe[data-app="${currentApp.name}"]`);
    if (frame) frame.src = frame.src;
  }

  // Handle command palette actions
  function handleCommandAction(actionId: string) {
    showCommandPalette = false;

    switch (actionId) {
      case 'settings':
        if (get(isAdmin)) openSettings();
        break;
      case 'shortcuts':
        loadShortcutsHelp().then(() => showShortcuts = true);
        break;
      case 'fullscreen':
        toggleFullscreen();
        break;
      case 'refresh':
        refreshActiveApp();
        break;
      case 'home':
        navigateHome();
        break;
      case 'logout':
        handleLogout();
        break;
      case 'logs':
        openLogs();
        break;
      case 'theme-dark':
        setTheme('dark');
        toasts.success(m.toast_switchedToDark());
        break;
      case 'theme-light':
        setTheme('light');
        toasts.success(m.toast_switchedToLight());
        break;
      case 'theme-system':
        setTheme('system');
        toasts.success(m.toast_usingSystemTheme());
        break;
    }
  }

  function handleKeydown(event: KeyboardEvent) {
    // Don't trigger shortcuts when not authenticated or on login page
    if (authRequired && !$isAuthenticated) {
      return;
    }

    // Don't trigger shortcuts when typing in inputs or custom dropdowns (except Escape)
    const target = event.target as HTMLElement;
    if (target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement || target.role === 'combobox') {
      if (event.key === 'Escape') {
        if (showCommandPalette) showCommandPalette = false;
        else if (showSettings) {
          if (!settingsRef?.handleEscape()) { showSettings = false; if (splitState.panels[0]) updateHash(); else clearHash(); }
        }
      }
      return;
    }

    // Escape is always hardcoded for closing modals
    if (event.key === 'Escape') {
      if (showCommandPalette) showCommandPalette = false;
      else if (showSettings) {
        // Let Settings close its sub-modals first; only close Settings itself if no sub-modal was open
        if (!settingsRef?.handleEscape()) { showSettings = false; if (splitState.panels[0]) updateHash(); else clearHash(); }
      }
      else if (showShortcuts) showShortcuts = false;
      else if (showLogs) { showLogs = false; showSplash = !splitState.panels[0]; if (splitState.panels[0]) updateHash(); else clearHash(); }
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
        loadCommandPalette().then(() => showCommandPalette = true);
        break;
      case 'settings':
        if ($isAdmin) { if (showSettings) { showSettings = false; if (splitState.panels[0]) updateHash(); else clearHash(); } else openSettings(); }
        break;
      case 'shortcuts':
        if (showShortcuts) { showShortcuts = false; } else { loadShortcutsHelp().then(() => showShortcuts = true); }
        break;
      case 'home':
        navigateHome();
        break;
      case 'logs':
        if (showLogs) { showLogs = false; showSplash = !splitState.panels[0]; if (splitState.panels[0]) updateHash(); else clearHash(); } else openLogs();
        break;
      case 'refresh':
        refreshActiveApp();
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
        const shortcutApp = apps.find(a => a.shortcut === slot);
        if (shortcutApp) {
          selectApp(shortcutApp);
        }
        break;
      }
    }
  }
</script>

<!-- Dynamic page title -->
<svelte:head>
  <title>{resolveTitle(config?.title || 'Muximux')}</title>
</svelte:head>

<svelte:window onkeydown={handleKeydown} />

{#if loading || !authChecked}
  <div class="flex items-center justify-center h-full" style="background: var(--bg-base);">
    <div class="text-center">
      <div class="animate-spin rounded-full h-12 w-12 border-b-2 mx-auto" style="border-color: var(--accent-primary);"></div>
      <p class="mt-4" style="color: var(--text-muted);">{m.common_loadingApp()}</p>
    </div>
  </div>
{:else if ($setupRequired || showOnboarding) && OnboardingWizardComponent}
  <OnboardingWizardComponent
    needsSetup={$setupRequired}
    oncomplete={handleOnboardingComplete}
  />
{:else if authRequired && !$isAuthenticated}
  <Login onsuccess={handleLoginSuccess} />
{:else if error}
  <div class="flex items-center justify-center h-full" style="background: var(--bg-base);">
    <ErrorState
      title={m.error_failedLoadDashboard()}
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
    class:flex={!$isFullscreen}
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
        onsearch={() => { loadCommandPalette().then(() => showCommandPalette = true); }}
        onsplash={() => { if (showSplash && splitState.panels[0]) { showSplash = false; } else { navigateHome(); } }}
        onsettings={() => { if (showSettings) { showSettings = false; if (splitState.panels[0]) updateHash(); else clearHash(); } else openSettings(); }}
        onlogs={() => { if (showLogs) { showLogs = false; showSplash = !splitState.panels[0]; if (splitState.panels[0]) updateHash(); else clearHash(); } else openLogs(); }}
        onlogout={handleLogout}
        splitEnabled={splitState.enabled}
        splitOrientation={splitState.orientation}
        splitActivePanel={splitState.activePanel}
        onsplithorizontal={() => { showSplash = false; showLogs = false; enableSplit('horizontal'); updateHash(); }}
        onsplitvertical={() => { showSplash = false; showLogs = false; enableSplit('vertical'); updateHash(); }}
        onsplitclose={() => { disableSplit(); updateHash(); showSplash = !splitState.panels[0]; }}
        onsplitpanel={(panel) => setActivePanel(panel)}
        onrefresh={refreshActiveApp}
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
        <Splash {apps} {config} onselect={(app) => selectApp(app)} onsettings={$isAdmin ? () => openSettings() : undefined} onabout={() => { settingsInitialTab = 'about'; openSettings(); }} />
      {:else if showLogs && LogsComponent}
        <LogsComponent onclose={() => { showLogs = false; showSplash = !splitState.panels[0]; if (location.hash === '#logs') { if (splitState.panels[0]) updateHash(); else clearHash(); } }} />
      {:else if $isFullscreen && !currentApp}
        <Splash {apps} {config} onselect={(app) => selectApp(app)} onsettings={$isAdmin ? () => openSettings() : undefined} onabout={() => { settingsInitialTab = 'about'; openSettings(); }} />
      {/if}

      {#if splitState.enabled}
        <!-- Split view: two panel slots with divider -->
        <div
          class="absolute inset-0 flex"
          class:flex-row={splitState.orientation === 'horizontal'}
          class:flex-col={splitState.orientation === 'vertical'}
          style:visibility={showSplash || showLogs ? 'hidden' : 'visible'}
        >
          <!-- Panel 0 -->
          <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
          <div
            class="relative overflow-hidden"
            style:flex="{splitState.dividerPosition} 1 0%"
            onclick={() => setActivePanel(0)}
            role="region"
            aria-label="Split panel 1"
          >
            {#each visitedApps as app (app.name)}
              <div
                class="absolute inset-0"
                style:visibility={splitState.panels[0]?.name === app.name ? 'visible' : 'hidden'}
              >
                <AppFrame {app} />
              </div>
            {/each}
            {#if !splitState.panels[0]}
              <div class="absolute inset-0 flex flex-col items-center justify-center gap-3" style="background: var(--bg-primary);">
                <svg class="w-10 h-10" style="color: var(--text-muted); opacity: 0.4;" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M13.5 6H5.25A2.25 2.25 0 003 8.25v10.5A2.25 2.25 0 005.25 21h10.5A2.25 2.25 0 0018 18.75V10.5m-4.5-4.5L21 3m0 0h-5.25M21 3v5.25" />
                </svg>
                <p class="text-sm" style="color: var(--text-muted);">{m.common_selectApp()}</p>
              </div>
            {/if}
          </div>

          <SplitDivider
            orientation={splitState.orientation}
            activePanel={splitState.activePanel}
            onresize={(pos) => updateDividerPosition(pos)}
            ondblclick={() => updateDividerPosition(0.5)}
          />

          <!-- Panel 1 -->
          <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
          <div
            class="relative overflow-hidden"
            style:flex="{1 - splitState.dividerPosition} 1 0%"
            onclick={() => setActivePanel(1)}
            role="region"
            aria-label="Split panel 2"
          >
            {#each visitedApps as app (app.name)}
              <div
                class="absolute inset-0"
                style:visibility={splitState.panels[1]?.name === app.name ? 'visible' : 'hidden'}
              >
                <AppFrame {app} />
              </div>
            {/each}
            {#if !splitState.panels[1]}
              <div class="absolute inset-0 flex flex-col items-center justify-center gap-3" style="background: var(--bg-primary);">
                <svg class="w-10 h-10" style="color: var(--text-muted); opacity: 0.4;" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M13.5 6H5.25A2.25 2.25 0 003 8.25v10.5A2.25 2.25 0 005.25 21h10.5A2.25 2.25 0 0018 18.75V10.5m-4.5-4.5L21 3m0 0h-5.25M21 3v5.25" />
                </svg>
                <p class="text-sm" style="color: var(--text-muted);">{m.common_selectApp()}</p>
              </div>
            {/if}
          </div>
        </div>
      {:else}
        <!-- Single view (original behavior) -->
        {#each visitedApps as app (app.name)}
          <div
            class="absolute inset-0"
            style:visibility={!showSplash && !showLogs && splitState.panels[0]?.name === app.name ? 'visible' : 'hidden'}
          >
            <AppFrame {app} />
          </div>
        {/each}
      {/if}
    </main>

    <!-- Fullscreen exit button -->
    {#if $isFullscreen}
      <div class="fixed top-4 end-4 z-50 flex items-center gap-2">
        <button
          class="fullscreen-exit-btn p-2 rounded-lg backdrop-blur-sm shadow-lg transition-all opacity-30 hover:opacity-100"
          onclick={exitFullscreen}
          title={m.nav_exitFullscreen()}
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    {/if}
  </div>

  <!-- Settings panel -->
  {#if showSettings && SettingsComponent}
    <SettingsComponent
      bind:this={settingsRef}
      {config}
      {apps}
      initialTab={settingsInitialTab}
      onclose={() => { showSettings = false; settingsInitialTab = 'general'; if (location.hash === '#settings') { if (splitState.panels[0]) updateHash(); else clearHash(); } }}
      onsave={(newConfig: Config) => handleSaveConfig(newConfig)}
    />
  {/if}

  <!-- Keyboard shortcuts help -->
  {#if showShortcuts && ShortcutsHelpComponent}
    <ShortcutsHelpComponent onclose={() => showShortcuts = false} />
  {/if}

  <!-- Command palette -->
  {#if showCommandPalette && CommandPaletteComponent}
    <CommandPaletteComponent
      {apps}
      onselect={(app: App) => { selectApp(app); showCommandPalette = false; }}
      onaction={(actionId: string) => handleCommandAction(actionId)}
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
