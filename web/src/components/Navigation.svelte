<script lang="ts">
  import { onMount } from 'svelte';
  import type { App, Config, Group } from '$lib/types';
  import AppIcon from './AppIcon.svelte';
  import HealthIndicator from './HealthIndicator.svelte';
  import { healthData } from '$lib/healthStore';
  import { currentUser, isAuthenticated, logout } from '$lib/authStore';
  import { createEdgeSwipeHandlers, isTouchDevice } from '$lib/useSwipe';
  import MuximuxLogo from './MuximuxLogo.svelte';
  import { captureKeybindings, toggleCaptureKeybindings } from '$lib/keybindingCaptureStore';

  let {
    apps,
    showHealth = true,
    currentApp,
    config,
    showSplash = false,
    onselect,
    onsearch,
    onsplash,
    onsettings,
    onlogs,
    onlogout,
  }: {
    apps: App[];
    showHealth?: boolean;
    currentApp: App | null;
    config: Config;
    showSplash?: boolean;
    onselect?: (app: App) => void;
    onsearch?: () => void;
    onsplash?: () => void;
    onsettings?: () => void;
    onlogs?: () => void;
    onlogout?: () => void;
  } = $props();

  async function handleLogout() {
    await logout();
    onlogout?.();
  }

  function isUnhealthy(appName: string): boolean {
    return $healthData.get(appName)?.status === 'unhealthy';
  }

  // Sidebar width state (for left/right layouts)
  let sidebarWidth = $state(220);
  let isResizing = $state(false);
  let minWidth = 180;
  let maxWidth = 400;

  // Auto-hide state
  let isHidden = $state(false);
  let hideTimeout: ReturnType<typeof setTimeout> | null = null;
  const collapsedStripWidth = 48; // Width of visible strip when sidebar collapsed (fits icon + border)
  const collapsedBarHeight = 6; // Height of visible strip when top/bottom bar collapsed (thin reveal strip)

  // Group expansion state (persisted to localStorage)
  let expandedGroups: Record<string, boolean> = $state({});

  // Responsive state
  let isMobile = $state(false);

  // Calculate actual width for sidebars (for layout reflow)
  // When auto_hide is on, always reserve only the collapsed strip in the layout.
  // The expanded sidebar overlays the content instead of pushing it.
  let effectiveSidebarWidth = $derived((config.navigation.auto_hide || !config.navigation.show_labels) && !isMobile ? collapsedStripWidth : sidebarWidth);
  let mobileMenuOpen = $state(false);
  let hasTouchSupport = $state(false);

  // Group apps by their group, sorted by order
  let groupedApps = $derived.by(() => {
    const acc = {} as Record<string, App[]>;
    for (const app of apps) {
      const group = app.group || 'Ungrouped';
      if (!acc[group]) acc[group] = [];
      acc[group].push(app);
    }
    for (const group of Object.keys(acc)) {
      acc[group].sort((a, b) => a.order - b.order);
    }
    return acc;
  });

  // Get sorted groups from config
  let sortedGroups = $derived([...config.groups].sort((a, b) => a.order - b.order));

  // Get group names in order, including 'Ungrouped' at the end
  let groupNames = $derived([
    ...sortedGroups.map(g => g.name),
    ...(groupedApps['Ungrouped'] ? ['Ungrouped'] : [])
  ].filter(name => groupedApps[name]));

  // Initialize expanded state from localStorage
  onMount(() => {
    const stored = localStorage.getItem('muximux_expanded_groups');
    if (stored) {
      try {
        expandedGroups = JSON.parse(stored);
      } catch {
        expandedGroups = {};
      }
    }
    // Default all groups to expanded if not set
    groupNames.forEach(name => {
      if (expandedGroups[name] === undefined) {
        expandedGroups[name] = true;
      }
    });

    // Restore sidebar width
    const storedWidth = localStorage.getItem('muximux_sidebar_width');
    if (storedWidth) {
      sidebarWidth = parseInt(storedWidth, 10);
    }

    // Set up responsive listeners
    checkResponsive();
    window.addEventListener('resize', checkResponsive);

    // Detect touch support
    hasTouchSupport = isTouchDevice();

    // Set up edge swipe for mobile sidebar
    setupEdgeSwipe();

    return () => {
      window.removeEventListener('resize', checkResponsive);
      document.removeEventListener('pointermove', handleResizeMove);
      document.removeEventListener('pointerup', handleResizeEnd);
      document.removeEventListener('pointercancel', handleResizeEnd);
      cleanupEdgeSwipe();
    };
  });

  // Edge reveal is handled by the nav element's own mouseenter/mouseleave events.
  // The collapsed strip sits at the screen edge, so hovering the edge = hovering the nav.

  function checkResponsive() {
    isMobile = window.innerWidth < 640;
  }

  function toggleGroup(name: string) {
    expandedGroups[name] = !expandedGroups[name];
    localStorage.setItem('muximux_expanded_groups', JSON.stringify(expandedGroups));
  }

  function getGroupConfig(name: string): Group | undefined {
    return config.groups.find(g => g.name === name);
  }

  function getOpenModeIcon(mode: string): string {
    switch (mode) {
      case 'new_tab': return '↗';
      case 'new_window': return '⧉';
      default: return '';
    }
  }

  // Resize handling for sidebars - using pointer events for touch support
  function handleResizeStart(e: PointerEvent) {
    isResizing = true;
    // Capture pointer for reliable tracking outside element bounds
    (e.target as HTMLElement).setPointerCapture(e.pointerId);
    document.addEventListener('pointermove', handleResizeMove);
    document.addEventListener('pointerup', handleResizeEnd);
    document.addEventListener('pointercancel', handleResizeEnd);
    e.preventDefault();
  }

  function handleResizeMove(e: PointerEvent) {
    if (!isResizing) return;

    if (config.navigation.position === 'left') {
      sidebarWidth = Math.min(maxWidth, Math.max(minWidth, e.clientX));
    } else if (config.navigation.position === 'right') {
      sidebarWidth = Math.min(maxWidth, Math.max(minWidth, window.innerWidth - e.clientX));
    }
  }

  function handleResizeEnd(e: PointerEvent) {
    isResizing = false;
    // Release pointer capture
    if (e?.target) {
      try {
        (e.target as HTMLElement).releasePointerCapture(e.pointerId);
      } catch {
        // Ignore if already released
      }
    }
    document.removeEventListener('pointermove', handleResizeMove);
    document.removeEventListener('pointerup', handleResizeEnd);
    document.removeEventListener('pointercancel', handleResizeEnd);
    localStorage.setItem('muximux_sidebar_width', sidebarWidth.toString());
  }

  function handleResizeKeydown(e: KeyboardEvent) {
    const step = e.shiftKey ? 40 : 10;
    if (e.key === 'ArrowRight' || e.key === 'ArrowUp') {
      e.preventDefault();
      sidebarWidth = Math.min(maxWidth, sidebarWidth + step);
      localStorage.setItem('muximux_sidebar_width', sidebarWidth.toString());
    } else if (e.key === 'ArrowLeft' || e.key === 'ArrowDown') {
      e.preventDefault();
      sidebarWidth = Math.max(minWidth, sidebarWidth - step);
      localStorage.setItem('muximux_sidebar_width', sidebarWidth.toString());
    } else if (e.key === 'Home') {
      e.preventDefault();
      sidebarWidth = minWidth;
      localStorage.setItem('muximux_sidebar_width', sidebarWidth.toString());
    } else if (e.key === 'End') {
      e.preventDefault();
      sidebarWidth = maxWidth;
      localStorage.setItem('muximux_sidebar_width', sidebarWidth.toString());
    }
  }

  // Edge swipe handlers for opening sidebar on mobile
  let edgeSwipeHandlers: ReturnType<typeof createEdgeSwipeHandlers> | null = null;

  function setupEdgeSwipe() {
    if (!isMobile || !hasTouchSupport) return;

    const edge = config.navigation.position === 'right' ? 'right' : 'left';
    edgeSwipeHandlers = createEdgeSwipeHandlers(
      edge,
      () => { mobileMenuOpen = true; },
      () => { mobileMenuOpen = false; },
      { edgeWidth: 25, threshold: 40 }
    );

    // Attach to document for edge detection
    document.addEventListener('pointerdown', edgeSwipeHandlers.onpointerdown);
    document.addEventListener('pointerup', edgeSwipeHandlers.onpointerup);
    document.addEventListener('pointercancel', edgeSwipeHandlers.onpointercancel);
  }

  function cleanupEdgeSwipe() {
    if (edgeSwipeHandlers) {
      document.removeEventListener('pointerdown', edgeSwipeHandlers.onpointerdown);
      document.removeEventListener('pointerup', edgeSwipeHandlers.onpointerup);
      document.removeEventListener('pointercancel', edgeSwipeHandlers.onpointercancel);
      edgeSwipeHandlers = null;
    }
  }

  // Auto-hide handling
  // Nav element enter/leave: controls hide timer based on whether mouse is inside the nav
  function handleNavEnter() {
    if (!config.navigation.auto_hide) return;
    isHidden = false;
    if (hideTimeout) clearTimeout(hideTimeout);
  }

  function handleNavLeave() {
    if (!config.navigation.auto_hide) return;
    if (hideTimeout) clearTimeout(hideTimeout);
    const delayMs = parseDelay(config.navigation.auto_hide_delay);
    hideTimeout = setTimeout(() => {
      isHidden = true;
    }, delayMs);
  }

  function parseDelay(delay: string): number {
    const match = delay.match(/^([\d.]+)(ms|s)?$/);
    if (!match) return 3000;
    const value = parseFloat(match[1]);
    const unit = match[2] || 's';
    return unit === 'ms' ? value : value * 1000;
  }

  // Icon scale for app icons (not logo, search, settings, logout)
  let iconScale = $derived(config.navigation.icon_scale || 1);

  // Hide logout when auth is 'none' — the virtual admin user shouldn't appear to be "logged in"
  let hasRealAuth = $derived(config.auth?.method !== undefined && config.auth.method !== 'none');

  // Whether shortcuts are currently active (considers both global toggle and per-app setting)
  let appDisablesShortcuts = $derived(currentApp && !showSplash && currentApp.disable_keyboard_shortcuts);
  let shortcutsActive = $derived($captureKeybindings && !appDisablesShortcuts);
  let keyboardTooltip = $derived(appDisablesShortcuts
    ? 'Keyboard shortcuts disabled for this app — keys are forwarded to the app'
    : $captureKeybindings
      ? 'Muximux is capturing keyboard shortcuts (e.g. number keys to switch apps) — click to forward all keys to the app instead'
      : 'Keyboard shortcuts paused — all keys are forwarded to the app — click to let Muximux capture shortcuts again');

</script>

<!-- Mobile hamburger menu -->
{#if isMobile && config.navigation.position !== 'bottom'}
  <button
    class="fixed top-4 left-4 z-50 p-2 bg-gray-800 rounded-lg border border-gray-700 text-white lg:hidden"
    onclick={() => mobileMenuOpen = !mobileMenuOpen}
  >
    <svg class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      {#if mobileMenuOpen}
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
      {:else}
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
      {/if}
    </svg>
  </button>
{/if}

<!-- Mobile overlay -->
{#if mobileMenuOpen}
  <button
    class="fixed inset-0 bg-black/50 z-30 lg:hidden"
    onclick={() => mobileMenuOpen = false}
    aria-label="Close menu"
  ></button>
{/if}

<!-- TOP NAVIGATION -->
{#if config.navigation.position === 'top'}
  {@const isCollapsedTop = isHidden && config.navigation.auto_hide}
  <nav
    class="relative"
    style="
      height: {config.navigation.auto_hide ? collapsedBarHeight + 'px' : '56px'};
    "
    onmouseenter={handleNavEnter}
    onmouseleave={handleNavLeave}
  >
    <!-- Inner panel - overlays content when auto_hide on, clips content as it collapses -->
    <div
      class="top-nav-panel border-b"
      style="background: var(--bg-surface); border-color: var(--border-subtle);"
      style:height="{isCollapsedTop ? collapsedBarHeight : 56}px"
      style:box-shadow={config.navigation.auto_hide && config.navigation.show_shadow ? '0 4px 24px rgba(0,0,0,0.25)' : null}
      style:position={config.navigation.auto_hide ? 'absolute' : null}
      style:top={config.navigation.auto_hide ? '0' : null}
      style:left={config.navigation.auto_hide ? '0' : null}
      style:right={config.navigation.auto_hide ? '0' : null}
      style:z-index={config.navigation.auto_hide ? '30' : null}
    >
    <div
      class="flex items-center justify-between px-4"
      style="height: 56px;"
    >
      <!-- Logo + app tabs -->
      <div class="flex items-center space-x-4">
        {#if config.navigation.show_logo}
          <button
            class="flex-shrink-0 hover:opacity-80"
            style="color: var(--accent-primary); opacity: {showSplash ? '0.6' : '1'}; transition: opacity 0.2s ease;"
            onclick={() => onsplash?.()}
            title={config.title}
          >
            <MuximuxLogo height="24" />
          </button>
        {:else}
          <button
            class="flex-shrink-0 p-1 rounded-md hover:bg-gray-700 transition-all"
            style="color: var(--accent-primary); opacity: {showSplash ? '0.6' : '1'}; transition: opacity 0.2s ease;"
            onclick={() => onsplash?.()}
            title="Overview"
          >
            <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-4 0a1 1 0 01-1-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 01-1 1h-2z" />
            </svg>
          </button>
        {/if}

        <!-- App tabs - always visible, labels hidden when collapsed -->
        <div class="flex items-center space-x-1 overflow-x-auto scrollbar-hide max-w-[calc(100vw-300px)]">
          {#each apps as app (app.name)}
            <button
              class="relative group px-2 py-2 rounded-md text-sm font-medium transition-colors whitespace-nowrap flex items-center gap-1
                     {currentApp?.name === app.name
                       ? 'bg-gray-900 text-white'
                       : 'text-gray-300 hover:bg-gray-700 hover:text-white'}
                     {showHealth && isUnhealthy(app.name) && currentApp?.name !== app.name ? 'opacity-50' : ''}"
              style={config.navigation.show_app_colors && currentApp?.name === app.name ? `border-bottom: 2px solid ${app.color || '#22c55e'}` : ''}
              onclick={() => onselect?.(app)}
            >
              <AppIcon icon={app.icon} name={app.name} color={app.color} size="sm" scale={iconScale} showBackground={config.navigation.show_icon_background} />
              {#if config.navigation.show_labels}
                <span>{app.name}</span>
              {:else}
                <span class="inline-block max-w-0 overflow-hidden opacity-0 group-hover:max-w-[120px] group-hover:opacity-100 transition-all duration-200 whitespace-nowrap">{app.name}</span>
              {/if}
              {#if showHealth}
                <HealthIndicator appName={app.name} size="sm" />
              {/if}
              {#if app.open_mode !== 'iframe'}
                <span class="text-xs opacity-60">{getOpenModeIcon(app.open_mode)}</span>
              {/if}
            </button>
          {/each}
        </div>
      </div>

      <!-- Right side actions -->
      <div class="flex items-center space-x-2">
        <button
          class="p-2 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          onclick={() => onsearch?.()}
          title="Search (Ctrl+K)"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
        </button>
        <button
          class="p-2 rounded-md hover:bg-gray-700 transition-colors"
          class:text-brand-400={shortcutsActive}
          class:text-gray-500={!shortcutsActive}
          class:opacity-50={appDisablesShortcuts}
          onclick={() => !appDisablesShortcuts && toggleCaptureKeybindings()}
          title={keyboardTooltip}
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
          </svg>
        </button>
        <button
          class="p-2 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          onclick={() => onlogs?.()}
          title="Logs"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 10h16M4 14h16M4 18h12" />
          </svg>
        </button>
        <button
          class="p-2 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          onclick={() => onsettings?.()}
          title="Settings"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.11 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
        </button>
        {#if hasRealAuth && $isAuthenticated && $currentUser}
          <button
            class="p-2 text-gray-400 hover:text-red-400 rounded-md hover:bg-gray-700 transition-colors"
            onclick={handleLogout}
            title="Sign out"
          >
            <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
            </svg>
          </button>
        {/if}
      </div>
    </div>
    </div> <!-- End top-nav-panel -->
  </nav>

<!-- LEFT SIDEBAR -->
{:else if config.navigation.position === 'left'}
  {@const labelsCollapsed = !config.navigation.show_labels && !isMobile}
  {@const isCollapsed = labelsCollapsed || (isHidden && config.navigation.auto_hide && !isMobile)}
  <aside
    class="flex-shrink-0 h-full relative
           {isMobile ? (mobileMenuOpen ? 'translate-x-0' : '-translate-x-full') : ''}"
    style="width: {isMobile ? '280px' : effectiveSidebarWidth + 'px'};"
    onmouseenter={handleNavEnter}
    onmouseleave={handleNavLeave}
  >
    <!-- Content panel - overlays content when auto-hide expands -->
    <div
      class="sidebar-panel flex flex-col h-full overflow-hidden border-r"
      style="background: var(--bg-surface); border-color: var(--border-subtle);"
      style:width="{isCollapsed ? collapsedStripWidth : sidebarWidth}px"
      style:box-shadow={config.navigation.auto_hide && config.navigation.show_shadow && !isMobile ? '4px 0 24px rgba(0,0,0,0.25)' : null}
      style:position={config.navigation.auto_hide && !isMobile ? 'absolute' : null}
      style:top={config.navigation.auto_hide && !isMobile ? '0' : null}
      style:left={config.navigation.auto_hide && !isMobile ? '0' : null}
      style:bottom={config.navigation.auto_hide && !isMobile ? '0' : null}
      style:z-index={config.navigation.auto_hide && !isMobile ? '30' : null}
    >
    <!-- Header — fixed height, logo scales via CSS transform for smooth animation -->
    {#if config.navigation.show_logo}
      <div class="border-b border-gray-700 flex items-center justify-center overflow-hidden"
           style="height: 100px;">
        <button
          class="hover:opacity-80 flex items-center justify-center"
          style="color: var(--accent-primary); transform: scale({isCollapsed ? 0.25 : 1}); opacity: {showSplash ? '0.6' : '1'}; transition: transform 0.3s ease, opacity 0.2s ease;"
          onclick={() => { onsplash?.(); mobileMenuOpen = false; }}
          title={config.title}
        >
          <MuximuxLogo height="80" />
        </button>
      </div>
    {:else}
      <div class="border-b border-gray-700 flex items-center justify-center overflow-hidden"
           style="height: {isCollapsed ? `${collapsedStripWidth}px` : '52px'};">
        <button
          class="p-2 rounded-md hover:bg-gray-700 transition-all"
          style="color: var(--accent-primary); opacity: {showSplash ? '0.6' : '1'}; transition: opacity 0.2s ease;"
          onclick={() => { onsplash?.(); mobileMenuOpen = false; }}
          title="Overview"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-4 0a1 1 0 01-1-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 01-1 1h-2z" />
          </svg>
        </button>
      </div>
    {/if}

    <!-- Search — fixed height, icon centered via container, text fades smoothly -->
    <div class="flex items-center"
         style="height: 52px; padding: 8px {isCollapsed ? '0' : '0.5rem'}; transition: padding 0.3s ease;">
      <button
        class="w-full flex items-center py-1.5 text-gray-400 hover:text-white rounded-md hover:bg-gray-700 text-sm"
        onclick={() => onsearch?.()}
        title="Search (Ctrl+K)"
      >
        <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
        </div>
        <span style="opacity: {isCollapsed ? '0' : '1'}; transition: opacity 0.15s ease; white-space: nowrap;">Search...</span>
        <span class="ml-auto mr-2 text-xs bg-gray-700 px-1.5 py-0.5 rounded" style="opacity: {isCollapsed ? '0' : '1'}; transition: opacity 0.15s ease; white-space: nowrap;">⌘K</span>
      </button>
    </div>

    <!-- App list with groups -->
    <div class="flex-1 overflow-y-auto scrollbar-hide"
         style="padding: 0.5rem {isCollapsed ? '0' : '0.5rem'}; transition: padding 0.3s ease;">
      {#each groupNames as groupName (groupName)}
        {@const groupConfig = getGroupConfig(groupName)}
        <div class="mb-2">
          <!-- Group header — icon stays visible when collapsed, text fades -->
          <button
            class="w-full flex items-center py-1.5 text-xs font-semibold text-gray-400 uppercase tracking-wider hover:text-gray-300"
            onclick={() => !isCollapsed && toggleGroup(groupName)}
            style="pointer-events: {isCollapsed ? 'none' : 'auto'};"
          >
            <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
              {#if groupConfig?.icon?.name}
                <AppIcon icon={groupConfig.icon} name={groupName} color={groupConfig.color || '#374151'} size="sm" scale={iconScale} showBackground={config.navigation.show_icon_background} />
              {:else if groupConfig?.color}
                <span class="w-2 h-2 rounded-full" style="background-color: {groupConfig.color}"></span>
              {:else}
                <svg
                  class="w-3 h-3 transition-transform {expandedGroups[groupName] ? 'rotate-90' : ''}"
                  fill="none" viewBox="0 0 24 24" stroke="currentColor"
                >
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                </svg>
              {/if}
            </div>
            <div class="flex items-center overflow-hidden flex-1 min-w-0" style="opacity: {isCollapsed ? '0' : '1'}; transition: opacity 0.15s ease;">
              {#if groupConfig?.icon?.name || groupConfig?.color}
                <svg
                  class="w-3 h-3 transition-transform {expandedGroups[groupName] ? 'rotate-90' : ''} mr-1 flex-shrink-0"
                  fill="none" viewBox="0 0 24 24" stroke="currentColor"
                >
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                </svg>
              {/if}
              <span class="truncate">{groupName}</span>
              <span class="ml-auto text-gray-500 flex-shrink-0">{groupedApps[groupName]?.length || 0}</span>
            </div>
          </button>

          <!-- Apps in group -->
          <div class="group-apps-wrapper" class:expanded={expandedGroups[groupName] || isCollapsed}>
            <div class="group-apps-inner mt-1 space-y-0.5">
              {#each groupedApps[groupName] || [] as app (app.name)}
                <button
                  class="w-full flex items-center py-1.5 rounded-md text-sm transition-colors relative
                         {currentApp?.name === app.name
                           ? 'bg-gray-700 text-white'
                           : 'text-gray-300 hover:bg-gray-700/50 hover:text-white'}"
                  style="opacity: {isCollapsed && currentApp?.name !== app.name ? 0.5 : showHealth && isUnhealthy(app.name) && currentApp?.name !== app.name ? 0.5 : 1};"
                  tabindex={expandedGroups[groupName] || isCollapsed ? 0 : -1}
                  onclick={() => { onselect?.(app); mobileMenuOpen = false; }}
                >
                  {#if config.navigation.show_app_colors && currentApp?.name === app.name}
                    <div class="absolute left-0 top-1 bottom-1 w-[3px] rounded-full" style="background: {app.color || '#22c55e'};"></div>
                  {/if}
                  <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
                    <AppIcon icon={app.icon} name={app.name} color={app.color} size="sm" scale={iconScale} showBackground={config.navigation.show_icon_background} />
                  </div>
                  {#if config.navigation.show_labels}
                    <span class="truncate" style="opacity: {isCollapsed ? '0' : '1'}; transition: opacity 0.15s ease;">{app.name}</span>
                  {/if}
                  {#if showHealth}
                    <span class="ml-auto pr-2" style="opacity: {isCollapsed ? '0' : '1'}; transition: opacity 0.15s ease;">
                      <HealthIndicator appName={app.name} size="sm" />
                    </span>
                  {/if}
                  {#if app.open_mode !== 'iframe'}
                    <span class="text-xs pr-1" style="opacity: {isCollapsed ? '0' : '0.6'}; transition: opacity 0.15s ease;">{getOpenModeIcon(app.open_mode)}</span>
                  {/if}
                </button>
              {/each}
            </div>
          </div>
        </div>
      {/each}
    </div>

    <!-- Footer — settings cog always visible, text fades smoothly -->
    <div class="border-t border-gray-700"
         style="padding: 8px {isCollapsed ? '0' : '0.5rem'}; transition: padding 0.3s ease;">
      <button
        class="w-full flex items-center py-1.5 text-gray-400 hover:text-white rounded-md hover:bg-gray-700 text-sm"
        onclick={() => { onlogs?.(); mobileMenuOpen = false; }}
        title="Logs"
      >
        <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 10h16M4 14h16M4 18h12" />
          </svg>
        </div>
        <span style="opacity: {isCollapsed ? '0' : '1'}; transition: opacity 0.15s ease; white-space: nowrap;">Logs</span>
      </button>

      <button
        class="w-full flex items-center py-1.5 text-gray-400 hover:text-white rounded-md hover:bg-gray-700 text-sm"
        onclick={() => onsettings?.()}
        title="Settings"
      >
        <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
        </div>
        <span style="opacity: {isCollapsed ? '0' : '1'}; transition: opacity 0.15s ease; white-space: nowrap;">Settings</span>
      </button>

      <button
        class="w-full flex items-center py-1.5 rounded-md hover:bg-gray-700 text-sm transition-colors"
        class:text-brand-400={shortcutsActive}
        class:text-gray-500={!shortcutsActive}
        style="opacity: {isCollapsed && !shortcutsActive ? '0.3' : appDisablesShortcuts ? '0.5' : '1'}; transition: opacity 0.15s ease; pointer-events: {appDisablesShortcuts ? 'none' : 'auto'};"
        onclick={() => toggleCaptureKeybindings()}
        title={keyboardTooltip}
      >
        <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
          </svg>
        </div>
        <span style="opacity: {isCollapsed ? '0' : '1'}; transition: opacity 0.15s ease; white-space: nowrap;">Shortcuts</span>
      </button>

      {#if hasRealAuth && $isAuthenticated && $currentUser}
        <button
          class="w-full flex items-center py-1.5 text-gray-400 hover:text-red-400 rounded-md hover:bg-gray-700 text-sm transition-colors"
          style="opacity: {isCollapsed ? '0' : '1'}; transition: opacity 0.15s ease; pointer-events: {isCollapsed ? 'none' : 'auto'};"
          tabindex={isCollapsed ? -1 : 0}
          onclick={handleLogout}
          title="Sign out"
        >
          <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
            </svg>
          </div>
          <span style="opacity: {isCollapsed ? '0' : '1'}; transition: opacity 0.15s ease; white-space: nowrap;">Sign out</span>
        </button>
      {/if}
    </div>
    </div> <!-- End content wrapper -->

    <!-- Resize handle - only when not auto-hiding and labels visible -->
    {#if !isMobile && !config.navigation.auto_hide && config.navigation.show_labels}
      <div
        class="absolute top-0 right-0 w-2 h-full cursor-ew-resize hover:bg-brand-500/50 active:bg-brand-500/70 transition-colors touch-none"
        onpointerdown={handleResizeStart}
        onkeydown={handleResizeKeydown}
        role="slider"
        aria-label="Resize sidebar"
        tabindex="0"
        aria-valuenow={sidebarWidth}
        aria-valuemin={minWidth}
        aria-valuemax={maxWidth}
      ></div>
    {/if}
  </aside>

<!-- RIGHT SIDEBAR -->
{:else if config.navigation.position === 'right'}
  {@const labelsCollapsedRight = !config.navigation.show_labels && !isMobile}
  {@const isCollapsedRight = labelsCollapsedRight || (isHidden && config.navigation.auto_hide && !isMobile)}
  <aside
    class="flex-shrink-0 h-full relative
           {isMobile ? (mobileMenuOpen ? 'translate-x-0' : 'translate-x-full') : ''}"
    style="width: {isMobile ? '280px' : effectiveSidebarWidth + 'px'};"
    onmouseenter={handleNavEnter}
    onmouseleave={handleNavLeave}
  >
    <!-- Content panel - overlays content when auto-hide expands -->
    <div
      class="sidebar-panel flex flex-col h-full overflow-hidden border-l"
      style="background: var(--bg-surface); border-color: var(--border-subtle);"
      style:width="{isCollapsedRight ? collapsedStripWidth : sidebarWidth}px"
      style:box-shadow={config.navigation.auto_hide && config.navigation.show_shadow && !isMobile ? '-4px 0 24px rgba(0,0,0,0.25)' : null}
      style:position={config.navigation.auto_hide && !isMobile ? 'absolute' : null}
      style:top={config.navigation.auto_hide && !isMobile ? '0' : null}
      style:right={config.navigation.auto_hide && !isMobile ? '0' : null}
      style:bottom={config.navigation.auto_hide && !isMobile ? '0' : null}
      style:z-index={config.navigation.auto_hide && !isMobile ? '30' : null}
    >
    <!-- Header — fixed height, logo scales via CSS transform for smooth animation -->
    {#if config.navigation.show_logo}
      <div class="border-b border-gray-700 flex items-center justify-center overflow-hidden"
           style="height: 100px;">
        <button
          class="hover:opacity-80 flex items-center justify-center"
          style="color: var(--accent-primary); transform: scale({isCollapsedRight ? 0.25 : 1}); opacity: {showSplash ? '0.6' : '1'}; transition: transform 0.3s ease, opacity 0.2s ease;"
          onclick={() => { onsplash?.(); mobileMenuOpen = false; }}
          title={config.title}
        >
          <MuximuxLogo height="80" />
        </button>
      </div>
    {:else}
      <div class="border-b border-gray-700 flex items-center justify-center overflow-hidden"
           style="height: {isCollapsedRight ? `${collapsedStripWidth}px` : '52px'};">
        <button
          class="p-2 rounded-md hover:bg-gray-700 transition-all"
          style="color: var(--accent-primary); opacity: {showSplash ? '0.6' : '1'}; transition: opacity 0.2s ease;"
          onclick={() => { onsplash?.(); mobileMenuOpen = false; }}
          title="Overview"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-4 0a1 1 0 01-1-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 01-1 1h-2z" />
          </svg>
        </button>
      </div>
    {/if}

    <!-- Search — fixed height, icon centered via container, text fades smoothly -->
    <div class="flex items-center"
         style="height: 52px; padding: 8px {isCollapsedRight ? '0' : '0.5rem'}; transition: padding 0.3s ease;">
      <button
        class="w-full flex items-center py-1.5 text-gray-400 hover:text-white rounded-md hover:bg-gray-700 text-sm"
        onclick={() => onsearch?.()}
        title="Search (Ctrl+K)"
      >
        <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
        </div>
        <span style="opacity: {isCollapsedRight ? '0' : '1'}; transition: opacity 0.15s ease; white-space: nowrap;">Search...</span>
        <span class="ml-auto mr-2 text-xs bg-gray-700 px-1.5 py-0.5 rounded" style="opacity: {isCollapsedRight ? '0' : '1'}; transition: opacity 0.15s ease; white-space: nowrap;">⌘K</span>
      </button>
    </div>

    <!-- App list with groups -->
    <div class="flex-1 overflow-y-auto scrollbar-hide"
         style="padding: 0.5rem {isCollapsedRight ? '0' : '0.5rem'}; transition: padding 0.3s ease;">
      {#each groupNames as groupName (groupName)}
        {@const groupConfig = getGroupConfig(groupName)}
        <div class="mb-2">
          <!-- Group header — icon stays visible when collapsed, text fades -->
          <button
            class="w-full flex items-center py-1.5 text-xs font-semibold text-gray-400 uppercase tracking-wider hover:text-gray-300"
            onclick={() => !isCollapsedRight && toggleGroup(groupName)}
            style="pointer-events: {isCollapsedRight ? 'none' : 'auto'};"
          >
            <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
              {#if groupConfig?.icon?.name}
                <AppIcon icon={groupConfig.icon} name={groupName} color={groupConfig.color || '#374151'} size="sm" scale={iconScale} showBackground={config.navigation.show_icon_background} />
              {:else if groupConfig?.color}
                <span class="w-2 h-2 rounded-full" style="background-color: {groupConfig.color}"></span>
              {:else}
                <svg
                  class="w-3 h-3 transition-transform {expandedGroups[groupName] ? 'rotate-90' : ''}"
                  fill="none" viewBox="0 0 24 24" stroke="currentColor"
                >
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                </svg>
              {/if}
            </div>
            <div class="flex items-center overflow-hidden flex-1 min-w-0" style="opacity: {isCollapsedRight ? '0' : '1'}; transition: opacity 0.15s ease;">
              {#if groupConfig?.icon?.name || groupConfig?.color}
                <svg
                  class="w-3 h-3 transition-transform {expandedGroups[groupName] ? 'rotate-90' : ''} mr-1 flex-shrink-0"
                  fill="none" viewBox="0 0 24 24" stroke="currentColor"
                >
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                </svg>
              {/if}
              <span class="truncate">{groupName}</span>
              <span class="ml-auto text-gray-500 flex-shrink-0">{groupedApps[groupName]?.length || 0}</span>
            </div>
          </button>

          <div class="group-apps-wrapper" class:expanded={expandedGroups[groupName] || isCollapsedRight}>
            <div class="group-apps-inner mt-1 space-y-0.5">
              {#each groupedApps[groupName] || [] as app (app.name)}
                <button
                  class="w-full flex items-center py-1.5 rounded-md text-sm transition-colors relative
                         {currentApp?.name === app.name
                           ? 'bg-gray-700 text-white'
                           : 'text-gray-300 hover:bg-gray-700/50 hover:text-white'}"
                  style="opacity: {isCollapsedRight && currentApp?.name !== app.name ? 0.5 : showHealth && isUnhealthy(app.name) && currentApp?.name !== app.name ? 0.5 : 1};"
                  tabindex={expandedGroups[groupName] || isCollapsedRight ? 0 : -1}
                  onclick={() => { onselect?.(app); mobileMenuOpen = false; }}
                >
                  {#if config.navigation.show_app_colors && currentApp?.name === app.name}
                    <div class="absolute right-0 top-1 bottom-1 w-[3px] rounded-full" style="background: {app.color || '#22c55e'};"></div>
                  {/if}
                  <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
                    <AppIcon icon={app.icon} name={app.name} color={app.color} size="sm" scale={iconScale} showBackground={config.navigation.show_icon_background} />
                  </div>
                  {#if config.navigation.show_labels}
                    <span class="truncate" style="opacity: {isCollapsedRight ? '0' : '1'}; transition: opacity 0.15s ease;">{app.name}</span>
                  {/if}
                  {#if showHealth}
                    <span class="ml-auto pr-2" style="opacity: {isCollapsedRight ? '0' : '1'}; transition: opacity 0.15s ease;">
                      <HealthIndicator appName={app.name} size="sm" />
                    </span>
                  {/if}
                  {#if app.open_mode !== 'iframe'}
                    <span class="text-xs pr-1" style="opacity: {isCollapsedRight ? '0' : '0.6'}; transition: opacity 0.15s ease;">{getOpenModeIcon(app.open_mode)}</span>
                  {/if}
                </button>
              {/each}
            </div>
          </div>
        </div>
      {/each}
    </div>

    <!-- Footer — settings cog always visible, text fades smoothly -->
    <div class="border-t border-gray-700"
         style="padding: 8px {isCollapsedRight ? '0' : '0.5rem'}; transition: padding 0.3s ease;">
      <button
        class="w-full flex items-center py-1.5 text-gray-400 hover:text-white rounded-md hover:bg-gray-700 text-sm"
        onclick={() => { onlogs?.(); mobileMenuOpen = false; }}
        title="Logs"
      >
        <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 10h16M4 14h16M4 18h12" />
          </svg>
        </div>
        <span style="opacity: {isCollapsedRight ? '0' : '1'}; transition: opacity 0.15s ease; white-space: nowrap;">Logs</span>
      </button>

      <button
        class="w-full flex items-center py-1.5 text-gray-400 hover:text-white rounded-md hover:bg-gray-700 text-sm"
        onclick={() => onsettings?.()}
        title="Settings"
      >
        <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
        </div>
        <span style="opacity: {isCollapsedRight ? '0' : '1'}; transition: opacity 0.15s ease; white-space: nowrap;">Settings</span>
      </button>

      <button
        class="w-full flex items-center py-1.5 rounded-md hover:bg-gray-700 text-sm transition-colors"
        class:text-brand-400={shortcutsActive}
        class:text-gray-500={!shortcutsActive}
        style="opacity: {isCollapsedRight && !shortcutsActive ? '0.3' : appDisablesShortcuts ? '0.5' : '1'}; transition: opacity 0.15s ease; pointer-events: {appDisablesShortcuts ? 'none' : 'auto'};"
        onclick={() => toggleCaptureKeybindings()}
        title={keyboardTooltip}
      >
        <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
          </svg>
        </div>
        <span style="opacity: {isCollapsedRight ? '0' : '1'}; transition: opacity 0.15s ease; white-space: nowrap;">Shortcuts</span>
      </button>

      {#if hasRealAuth && $isAuthenticated && $currentUser}
        <button
          class="w-full flex items-center py-1.5 text-gray-400 hover:text-red-400 rounded-md hover:bg-gray-700 text-sm transition-colors"
          style="opacity: {isCollapsedRight ? '0' : '1'}; transition: opacity 0.15s ease; pointer-events: {isCollapsedRight ? 'none' : 'auto'};"
          tabindex={isCollapsedRight ? -1 : 0}
          onclick={handleLogout}
          title="Sign out"
        >
          <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
            </svg>
          </div>
          <span style="opacity: {isCollapsedRight ? '0' : '1'}; transition: opacity 0.15s ease; white-space: nowrap;">Sign out</span>
        </button>
      {/if}
    </div>
    </div> <!-- End content wrapper -->

    <!-- Resize handle (left side for right sidebar) - only when not auto-hiding and labels visible -->
    {#if !isMobile && !config.navigation.auto_hide && config.navigation.show_labels}
      <div
        class="absolute top-0 left-0 w-2 h-full cursor-ew-resize hover:bg-brand-500/50 active:bg-brand-500/70 transition-colors touch-none"
        onpointerdown={handleResizeStart}
        onkeydown={handleResizeKeydown}
        role="slider"
        aria-label="Resize sidebar"
        tabindex="0"
        aria-valuenow={sidebarWidth}
        aria-valuemin={minWidth}
        aria-valuemax={maxWidth}
      ></div>
    {/if}
  </aside>

<!-- BOTTOM BAR -->
{:else if config.navigation.position === 'bottom'}
  {@const isCollapsedBottom = isHidden && config.navigation.auto_hide}
  <nav
    class="relative"
    style="
      height: {config.navigation.auto_hide ? collapsedBarHeight + 'px' : '56px'};
    "
    onmouseenter={handleNavEnter}
    onmouseleave={handleNavLeave}
  >
    <!-- Inner panel - overlays content when auto_hide on, clips content as it collapses -->
    <div
      class="bottom-nav-panel border-t"
      style="border-color: var(--border-subtle);"
      style:height="{isCollapsedBottom ? collapsedBarHeight : 56}px"
      style:box-shadow={config.navigation.auto_hide && config.navigation.show_shadow ? '0 -4px 24px rgba(0,0,0,0.25)' : null}
      style:position={config.navigation.auto_hide ? 'absolute' : null}
      style:bottom={config.navigation.auto_hide ? '0' : null}
      style:left={config.navigation.auto_hide ? '0' : null}
      style:right={config.navigation.auto_hide ? '0' : null}
      style:z-index={config.navigation.auto_hide ? '30' : null}
    >
    <div
      class="flex items-center justify-between px-4"
      style="height: 56px;"
    >
      <!-- Logo + app tabs -->
      <div class="flex items-center space-x-4">
        {#if config.navigation.show_logo}
          <button
            class="flex-shrink-0 hover:opacity-80"
            style="color: var(--accent-primary); opacity: {showSplash ? '0.6' : '1'}; transition: opacity 0.2s ease;"
            onclick={() => onsplash?.()}
            title={config.title}
          >
            <MuximuxLogo height="24" />
          </button>
        {:else}
          <button
            class="flex-shrink-0 p-1 rounded-md hover:bg-gray-700 transition-all"
            style="color: var(--accent-primary); opacity: {showSplash ? '0.6' : '1'}; transition: opacity 0.2s ease;"
            onclick={() => onsplash?.()}
            title="Overview"
          >
            <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-4 0a1 1 0 01-1-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 01-1 1h-2z" />
            </svg>
          </button>
        {/if}

        <!-- App tabs -->
        <div class="flex items-center space-x-1 overflow-x-auto scrollbar-hide max-w-[calc(100vw-300px)]">
          {#each apps as app (app.name)}
            <button
              class="relative group px-2 py-2 rounded-md text-sm font-medium transition-colors whitespace-nowrap flex items-center gap-1
                     {currentApp?.name === app.name
                       ? 'bg-gray-900 text-white'
                       : 'text-gray-300 hover:bg-gray-700 hover:text-white'}
                     {showHealth && isUnhealthy(app.name) && currentApp?.name !== app.name ? 'opacity-50' : ''}"
              style={config.navigation.show_app_colors && currentApp?.name === app.name ? `border-top: 2px solid ${app.color || '#22c55e'}` : ''}
              onclick={() => onselect?.(app)}
            >
              <AppIcon icon={app.icon} name={app.name} color={app.color} size="sm" scale={iconScale} showBackground={config.navigation.show_icon_background} />
              {#if config.navigation.show_labels}
                <span>{app.name}</span>
              {:else}
                <span class="inline-block max-w-0 overflow-hidden opacity-0 group-hover:max-w-[120px] group-hover:opacity-100 transition-all duration-200 whitespace-nowrap">{app.name}</span>
              {/if}
              {#if showHealth}
                <HealthIndicator appName={app.name} size="sm" />
              {/if}
              {#if app.open_mode !== 'iframe'}
                <span class="text-xs opacity-60">{getOpenModeIcon(app.open_mode)}</span>
              {/if}
            </button>
          {/each}
        </div>
      </div>

      <!-- Right side actions -->
      <div class="flex items-center space-x-2">
        <button
          class="p-2 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          onclick={() => onsearch?.()}
          title="Search (Ctrl+K)"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
        </button>
        <button
          class="p-2 rounded-md hover:bg-gray-700 transition-colors"
          class:text-brand-400={shortcutsActive}
          class:text-gray-500={!shortcutsActive}
          class:opacity-50={appDisablesShortcuts}
          onclick={() => !appDisablesShortcuts && toggleCaptureKeybindings()}
          title={keyboardTooltip}
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
          </svg>
        </button>
        <button
          class="p-2 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          onclick={() => onlogs?.()}
          title="Logs"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 10h16M4 14h16M4 18h12" />
          </svg>
        </button>
        <button
          class="p-2 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          onclick={() => onsettings?.()}
          title="Settings"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.11 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
        </button>
        {#if hasRealAuth && $isAuthenticated && $currentUser}
          <button
            class="p-2 text-gray-400 hover:text-red-400 rounded-md hover:bg-gray-700 transition-colors"
            onclick={handleLogout}
            title="Sign out"
          >
            <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
            </svg>
          </button>
        {/if}
      </div>
    </div>
    </div> <!-- End bottom-nav-panel -->
  </nav>

<!-- FLOATING (Minimal) -->
{:else if config.navigation.position === 'floating'}
  {@const floatingPosition = 'bottom-6 right-6'}
  {@const isCollapsedFloat = isHidden && config.navigation.auto_hide}
  <div
    class="fixed {floatingPosition} z-40"
    style="pointer-events: {isCollapsedFloat ? 'none' : 'auto'};"
    onmouseenter={handleNavEnter}
    onmouseleave={handleNavLeave}
    role="presentation"
  >
    <!-- App list - fades completely when collapsed -->
    <div
      class="flex flex-col-reverse items-end gap-2 mb-2"
      style="opacity: {isCollapsedFloat ? '0' : '1'}; transform: translateY({isCollapsedFloat ? '10px' : '0'}); pointer-events: {isCollapsedFloat ? 'none' : 'auto'}; transition: opacity 0.25s ease, transform 0.3s ease;"
    >
      {#each apps.slice(0, 6) as app (app.name)}
        <button
          class="flex items-center gap-2 px-3 py-2 bg-gray-800 border border-gray-700 rounded-full shadow-lg
                 hover:bg-gray-700 transition-all hover:scale-105
                 {currentApp?.name === app.name ? 'ring-2 ring-brand-500' : ''}
                 {showHealth && isUnhealthy(app.name) && currentApp?.name !== app.name ? 'opacity-50' : ''}"
          onclick={() => onselect?.(app)}
        >
          <AppIcon icon={app.icon} name={app.name} color={app.color} size="md" scale={iconScale} showBackground={config.navigation.show_icon_background} />
          <span class="text-sm text-white pr-1">{app.name}</span>
          {#if showHealth}
            <HealthIndicator appName={app.name} size="sm" />
          {/if}
        </button>
      {/each}

      {#if apps.length > 6}
        <button
          class="flex items-center gap-2 px-3 py-2 bg-gray-800 border border-gray-700 rounded-full shadow-lg hover:bg-gray-700 transition-all"
          onclick={() => onsplash?.()}
        >
          <span class="text-sm text-gray-400">+{apps.length - 6} more</span>
        </button>
      {/if}
    </div>

    <!-- Bottom row -->
    <div class="flex items-center gap-2">
      <!-- Secondary buttons - fade when collapsed -->
      <div
        class="flex items-center gap-2"
        style="opacity: {isCollapsedFloat ? '0' : '1'}; transform: scale({isCollapsedFloat ? '0.9' : '1'}); pointer-events: {isCollapsedFloat ? 'none' : 'auto'}; transition: opacity 0.2s ease, transform 0.3s ease;"
      >
        {#if hasRealAuth && $isAuthenticated && $currentUser}
          <button
            class="p-3 bg-gray-800 border border-gray-700 rounded-full shadow-lg hover:bg-gray-700 transition-all hover:scale-110"
            onclick={handleLogout}
            title="Sign out"
          >
            <svg class="w-5 h-5 text-gray-300 hover:text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
            </svg>
          </button>
        {/if}
        <button
          class="p-3 bg-gray-800 border border-gray-700 rounded-full shadow-lg hover:bg-gray-700 transition-all hover:scale-110"
          onclick={() => onsearch?.()}
          title="Search"
        >
          <svg class="w-5 h-5 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
        </button>
        <button
          class="p-3 border rounded-full shadow-lg transition-all
                 {appDisablesShortcuts ? 'opacity-50' : 'hover:scale-110'}
                 {shortcutsActive ? 'bg-brand-600/20 border-brand-500/50' : 'bg-gray-800 border-gray-700'}"
          onclick={() => !appDisablesShortcuts && toggleCaptureKeybindings()}
          title={keyboardTooltip}
        >
          <svg class="w-5 h-5 {shortcutsActive ? 'text-brand-400' : 'text-gray-500'}" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
          </svg>
        </button>
        <button
          class="p-3 bg-gray-800 border border-gray-700 rounded-full shadow-lg hover:bg-gray-700 transition-all hover:scale-110"
          onclick={() => onlogs?.()}
          title="Logs"
        >
          <svg class="w-5 h-5 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 10h16M4 14h16M4 18h12" />
          </svg>
        </button>
        <button
          class="p-3 bg-gray-800 border border-gray-700 rounded-full shadow-lg hover:bg-gray-700 transition-all hover:scale-110"
          onclick={() => onsettings?.()}
          title="Settings"
        >
          <svg class="w-5 h-5 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
        </button>
      </div>
      <!-- Main FAB - always visible and interactive -->
      <button
        class="p-4 bg-brand-600 hover:bg-brand-700 text-white rounded-full shadow-lg transition-all hover:scale-110"
        style="pointer-events: auto;"
        onclick={() => onsplash?.()}
        title={config.title}
      >
        <svg class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
        </svg>
      </button>
    </div>
  </div>
{/if}

<style>
  /* Ensure sidebar has relative positioning for resize handle */
  aside {
    position: relative;
  }

  /* Sidebar panel transition — declared in CSS (not inline) so Svelte's
     style attribute replacement never removes the transition declaration */
  .sidebar-panel {
    transition: width 0.3s ease, box-shadow 0.3s ease;
    will-change: width;
  }

  /* Top nav panel — clips content as height shrinks (like sidebar clips on width) */
  .top-nav-panel {
    transition: height 0.3s ease, box-shadow 0.3s ease;
    will-change: height;
    overflow: hidden;
  }

  /* Bottom nav panel — clips content as height shrinks + glass effect */
  .bottom-nav-panel {
    transition: height 0.3s ease, box-shadow 0.3s ease;
    will-change: height;
    overflow: hidden;
    background: var(--glass-bg) !important;
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
  }

  /* ═══════════════════════════════════════════════════════════════════════════
     THEME-AWARE NAVIGATION STYLES
     Override Tailwind gray classes with CSS custom properties
     ═══════════════════════════════════════════════════════════════════════════ */

  /* Navigation containers */
  nav, aside {
    background: var(--bg-surface) !important;
    border-color: var(--border-subtle) !important;
  }

  /* Text colors */
  nav :global(.text-white),
  aside :global(.text-white) {
    color: var(--text-primary) !important;
  }

  nav :global(.text-gray-300),
  aside :global(.text-gray-300) {
    color: var(--text-secondary) !important;
  }

  nav :global(.text-gray-400),
  aside :global(.text-gray-400) {
    color: var(--text-muted) !important;
  }

  nav :global(.text-gray-500),
  aside :global(.text-gray-500) {
    color: var(--text-disabled) !important;
  }

  /* Background colors */
  nav :global(.bg-gray-700),
  aside :global(.bg-gray-700) {
    background: var(--bg-hover) !important;
  }

  nav :global(.bg-gray-800),
  aside :global(.bg-gray-800) {
    background: var(--bg-surface) !important;
  }

  nav :global(.bg-gray-900),
  aside :global(.bg-gray-900) {
    background: var(--bg-base) !important;
  }

  /* Hover states */
  nav :global(.hover\:bg-gray-700:hover),
  aside :global(.hover\:bg-gray-700:hover) {
    background: var(--bg-hover) !important;
  }

  nav :global(.hover\:bg-gray-600\/50:hover),
  aside :global(.hover\:bg-gray-600\/50:hover) {
    background: var(--bg-active) !important;
  }

  nav :global(.hover\:text-white:hover),
  aside :global(.hover\:text-white:hover) {
    color: var(--text-primary) !important;
  }

  /* Border colors */
  nav :global(.border-gray-700),
  aside :global(.border-gray-700) {
    border-color: var(--border-subtle) !important;
  }

  /* Active/selected states */
  nav :global(.bg-gray-700\/50),
  aside :global(.bg-gray-700\/50) {
    background: var(--bg-hover) !important;
  }

  /* Floating buttons */
  :global(.bg-gray-800.border.border-gray-700) {
    background: var(--bg-surface) !important;
    border-color: var(--border-default) !important;
  }

  /* Keyboard shortcut badges */
  nav :global(.bg-gray-700.px-1\.5),
  aside :global(.bg-gray-700.px-1\.5) {
    background: var(--bg-overlay) !important;
    color: var(--text-muted) !important;
  }

  /* Brand color buttons */
  :global(.bg-brand-600) {
    background: var(--accent-primary) !important;
  }

  :global(.hover\:bg-brand-700:hover) {
    background: var(--accent-secondary) !important;
  }

  :global(.ring-brand-500) {
    outline-color: var(--accent-primary) !important;
  }

  /* Mobile overlay */
  :global(.bg-black\/50) {
    background: rgba(0, 0, 0, 0.6) !important;
    backdrop-filter: blur(4px);
    -webkit-backdrop-filter: blur(4px);
  }

  /* Resize handle */
  :global(.hover\:bg-brand-500\/50:hover) {
    background: var(--accent-muted) !important;
  }

  :global(.active\:bg-brand-500\/70:active) {
    background: var(--accent-primary) !important;
    opacity: 0.7;
  }

  /* Smooth expand/collapse for group app lists */
  .group-apps-wrapper {
    display: grid;
    grid-template-rows: 0fr;
    transition: grid-template-rows 0.25s ease;
  }
  .group-apps-wrapper.expanded {
    grid-template-rows: 1fr;
  }
  .group-apps-inner {
    overflow: hidden;
    min-height: 0;
  }
</style>
