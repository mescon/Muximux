<script lang="ts">
  import { createEventDispatcher, onMount, onDestroy } from 'svelte';
  import type { App, Config, Group } from '$lib/types';
  import AppIcon from './AppIcon.svelte';
  import HealthIndicator from './HealthIndicator.svelte';
  import { currentUser, isAuthenticated, logout } from '$lib/authStore';
  import { createEdgeSwipeHandlers, isTouchDevice } from '$lib/useSwipe';

  export let apps: App[];
  export let showHealth: boolean = true;
  export let currentApp: App | null;
  export let config: Config;

  const dispatch = createEventDispatcher<{
    select: App;
    search: void;
    splash: void;
    settings: void;
    logout: void;
  }>();

  // User menu state
  let userMenuOpen = false;

  async function handleLogout() {
    await logout();
    dispatch('logout');
  }

  // Sidebar width state (for left/right layouts)
  let sidebarWidth = 220;
  let isResizing = false;
  let minWidth = 180;
  let maxWidth = 400;

  // Auto-hide state
  let isHidden = false;
  let hideTimeout: ReturnType<typeof setTimeout> | null = null;
  const collapsedStripWidth = 48; // Width/height of visible strip when collapsed (fits icon + border)

  // Calculate actual width for sidebars (for layout reflow)
  $: effectiveSidebarWidth = isHidden && config.navigation.auto_hide && !isMobile ? collapsedStripWidth : sidebarWidth;

  // Group expansion state (persisted to localStorage)
  let expandedGroups: Record<string, boolean> = {};

  // Responsive state
  let isMobile = false;
  let isTablet = false;
  let mobileMenuOpen = false;
  let hasTouchSupport = false;

  // Group apps by their group
  $: groupedApps = apps.reduce((acc, app) => {
    const group = app.group || 'Ungrouped';
    if (!acc[group]) acc[group] = [];
    acc[group].push(app);
    return acc;
  }, {} as Record<string, App[]>);

  // Sort apps within groups by order
  $: Object.keys(groupedApps).forEach(group => {
    groupedApps[group].sort((a, b) => a.order - b.order);
  });

  // Get sorted groups from config
  $: sortedGroups = [...config.groups].sort((a, b) => a.order - b.order);

  // Get group names in order, including 'Ungrouped' at the end
  $: groupNames = [
    ...sortedGroups.map(g => g.name),
    ...(groupedApps['Ungrouped'] ? ['Ungrouped'] : [])
  ].filter(name => groupedApps[name]);

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

    // Set up mouse/pointer listeners for auto-hide
    if (config.navigation.auto_hide) {
      document.addEventListener('mousemove', handleMouseMove);
    }

    return () => {
      window.removeEventListener('resize', checkResponsive);
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('pointermove', handleResizeMove);
      document.removeEventListener('pointerup', handleResizeEnd);
      document.removeEventListener('pointercancel', handleResizeEnd);
      cleanupEdgeSwipe();
    };
  });

  function checkResponsive() {
    isMobile = window.innerWidth < 640;
    isTablet = window.innerWidth >= 640 && window.innerWidth < 1024;
  }

  function toggleGroup(name: string) {
    expandedGroups[name] = !expandedGroups[name];
    expandedGroups = expandedGroups; // Trigger reactivity
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
  // Document-level: only used to detect mouse hitting the screen edge (to reveal collapsed nav)
  function handleMouseMove(e: MouseEvent) {
    if (!config.navigation.auto_hide || !isHidden) return;
    if (!config.navigation.show_on_hover) return;

    const threshold = 20;
    const pos = config.navigation.position;

    let shouldShow = false;
    if (pos === 'left' && e.clientX < threshold) shouldShow = true;
    if (pos === 'right' && e.clientX > window.innerWidth - threshold) shouldShow = true;
    if (pos === 'top' && e.clientY < threshold) shouldShow = true;
    if (pos === 'bottom' && e.clientY > window.innerHeight - threshold) shouldShow = true;

    if (shouldShow) {
      isHidden = false;
      if (hideTimeout) clearTimeout(hideTimeout);
    }
  }

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

  // CSS classes based on position
  $: positionClasses = {
    top: 'fixed top-0 left-0 right-0 z-40',
    left: 'fixed top-0 left-0 bottom-0 z-40',
    right: 'fixed top-0 right-0 bottom-0 z-40',
    bottom: 'fixed bottom-0 left-0 right-0 z-40',
    floating: 'fixed z-40'
  };

  $: hideTransform = {
    top: 'translateY(-100%)',
    left: 'translateX(-100%)',
    right: 'translateX(100%)',
    bottom: 'translateY(100%)',
    floating: 'scale(0.8) opacity(0)'
  };
</script>

<!-- Mobile hamburger menu -->
{#if isMobile && config.navigation.position !== 'bottom'}
  <button
    class="fixed top-4 left-4 z-50 p-2 bg-gray-800 rounded-lg border border-gray-700 text-white lg:hidden"
    on:click={() => mobileMenuOpen = !mobileMenuOpen}
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
    on:click={() => mobileMenuOpen = false}
  ></button>
{/if}

<!-- TOP NAVIGATION -->
{#if config.navigation.position === 'top'}
  {@const isCollapsedTop = isHidden && config.navigation.auto_hide}
  <nav
    class="border-b transition-all duration-300 relative"
    style="
      background: var(--bg-surface);
      border-color: var(--border-subtle);
      height: {isCollapsedTop ? collapsedStripWidth + 'px' : '56px'};
    "
    on:mouseenter={handleNavEnter}
    on:mouseleave={handleNavLeave}
  >
    <!-- Collapsed icon strip - show app icons when collapsed -->
    {#if isCollapsedTop}
      <!-- svelte-ignore a11y-click-events-have-key-events -->
      <div class="absolute inset-0 flex items-center justify-center gap-1 z-20 px-4 cursor-pointer" on:click={() => isHidden = false} role="button" tabindex="0">
        {#each apps as app}
          <button
            class="flex-shrink-0 transition-all duration-200 rounded"
            style="opacity: {currentApp?.name === app.name ? '1' : '0.4'};
                   {config.navigation.show_app_colors && currentApp?.name === app.name ? `border-bottom: 2px solid ${app.color || '#22c55e'}` : ''}"
            on:click|stopPropagation={() => dispatch('select', app)}
            title={app.name}
          >
            <AppIcon icon={app.icon} name={app.name} color={app.color} size="sm" showBackground={false} />
          </button>
        {/each}
      </div>
    {/if}
    <!-- Content wrapper -->
    <div
      class="flex items-center justify-between h-14 px-4 transition-opacity duration-200"
      style="opacity: {isCollapsedTop ? '0' : '1'}; pointer-events: {isCollapsedTop ? 'none' : 'auto'};"
    >
      <!-- Logo -->
      <div class="flex items-center space-x-4">
        {#if config.navigation.show_logo}
          <button
            class="text-xl font-bold text-white hover:text-brand-400 transition-colors"
            on:click={() => dispatch('splash')}
          >
            {config.title}
          </button>
        {/if}

        <!-- App tabs - horizontal scrollable -->
        <div class="flex items-center space-x-1 overflow-x-auto scrollbar-hide max-w-[calc(100vw-300px)]">
          {#each apps as app}
            <button
              class="px-3 py-2 rounded-md text-sm font-medium transition-colors whitespace-nowrap flex items-center gap-1
                     {currentApp?.name === app.name
                       ? 'bg-gray-900 text-white'
                       : 'text-gray-300 hover:bg-gray-700 hover:text-white'}"
              style={config.navigation.show_app_colors && currentApp?.name === app.name ? `border-bottom: 2px solid ${app.color || '#22c55e'}` : ''}
              on:click={() => dispatch('select', app)}
            >
              {#if !config.navigation.show_labels}
                <!-- Icon only mode -->
                <AppIcon icon={app.icon} name={app.name} color={app.color} size="sm" showBackground={config.navigation.show_icon_background} />
              {:else}
                {app.name}
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
          on:click={() => dispatch('search')}
          title="Search (Ctrl+K)"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
        </button>
        <button
          class="p-2 text-gray-400 hover:text-white rounded-md hover:bg-gray-700"
          on:click={() => dispatch('settings')}
          title="Settings"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
        </button>
        {#if $isAuthenticated && $currentUser}
          <button
            class="p-2 text-gray-400 hover:text-red-400 rounded-md hover:bg-gray-700 transition-colors"
            on:click={handleLogout}
            title="Sign out"
          >
            <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
            </svg>
          </button>
        {/if}
      </div>
    </div>
  </nav>

<!-- LEFT SIDEBAR -->
{:else if config.navigation.position === 'left'}
  {@const isCollapsed = isHidden && config.navigation.auto_hide && !isMobile}
  <aside
    class="border-r flex flex-col h-full transition-all duration-300 relative
           {isMobile ? (mobileMenuOpen ? 'translate-x-0' : '-translate-x-full') : ''}"
    style="
      background: var(--bg-surface);
      border-color: var(--border-subtle);
      width: {isMobile ? '280px' : effectiveSidebarWidth + 'px'};
    "
    on:mouseenter={handleNavEnter}
    on:mouseleave={handleNavLeave}
  >
    <!-- Content wrapper - stays visible when collapsed so icons maintain position -->
    <div
      class="flex flex-col h-full transition-all duration-200 overflow-hidden"
    >
    <!-- Header (hidden when collapsed) -->
    {#if config.navigation.show_logo}
      <div class="p-4 border-b border-gray-700 transition-opacity duration-200"
           style="opacity: {isCollapsed ? '0' : '1'}; pointer-events: {isCollapsed ? 'none' : 'auto'};">
        <button
          class="text-xl font-bold text-white hover:text-brand-400 transition-colors w-full text-left"
          on:click={() => { dispatch('splash'); mobileMenuOpen = false; }}
        >
          {config.title}
        </button>
      </div>
    {/if}

    <!-- Search button (hidden when collapsed) -->
    <div class="p-2 transition-opacity duration-200"
         style="opacity: {isCollapsed ? '0' : '1'}; pointer-events: {isCollapsed ? 'none' : 'auto'};">
      <button
        class="w-full flex items-center gap-2 px-3 py-2 text-gray-400 hover:text-white rounded-md hover:bg-gray-700 text-sm"
        on:click={() => dispatch('search')}
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
        <span>Search...</span>
        <span class="ml-auto text-xs bg-gray-700 px-1.5 py-0.5 rounded">⌘K</span>
      </button>
    </div>

    <!-- App list with groups -->
    <div class="flex-1 overflow-y-auto scrollbar-hide p-2">
      {#each groupNames as groupName}
        {@const groupConfig = getGroupConfig(groupName)}
        <div class="mb-2">
          <!-- Group header -->
          <button
            class="w-full flex items-center gap-2 px-2 py-1.5 text-xs font-semibold text-gray-400 uppercase tracking-wider hover:text-gray-300 transition-opacity duration-200"
            style="opacity: {isCollapsed ? '0' : '1'};"
            on:click={() => toggleGroup(groupName)}
          >
            <svg
              class="w-3 h-3 transition-transform {expandedGroups[groupName] ? 'rotate-90' : ''}"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
            </svg>
            {#if groupConfig?.icon?.name}
              <div class="flex-shrink-0">
                <AppIcon icon={groupConfig.icon} name={groupName} color={groupConfig.color || '#374151'} size="sm" showBackground={false} />
              </div>
            {:else if groupConfig?.color}
              <span class="w-2 h-2 rounded-full" style="background-color: {groupConfig.color}"></span>
            {/if}
            <span class="truncate">{groupName}</span>
            <span class="ml-auto text-gray-500">{groupedApps[groupName]?.length || 0}</span>
          </button>

          <!-- Apps in group -->
          {#if expandedGroups[groupName] || isCollapsed}
            <div class="mt-1 space-y-0.5">
              {#each groupedApps[groupName] || [] as app}
                <button
                  class="w-full flex items-center gap-2 px-2 py-2 rounded-md text-sm transition-colors
                         {currentApp?.name === app.name
                           ? 'bg-gray-700 text-white'
                           : 'text-gray-300 hover:bg-gray-700/50 hover:text-white'}"
                  style="border-left: 3px solid {config.navigation.show_app_colors && (currentApp?.name === app.name || isCollapsed) ? (app.color || '#22c55e') : 'transparent'};
                         {isCollapsed && currentApp?.name !== app.name ? 'opacity: 0.5;' : ''}"
                  on:click={() => { dispatch('select', app); mobileMenuOpen = false; }}
                >
                  <!-- App icon -->
                  <div class="flex-shrink-0">
                    <AppIcon icon={app.icon} name={app.name} color={app.color} size="sm" showBackground={config.navigation.show_icon_background} />
                  </div>
                  {#if config.navigation.show_labels && !isCollapsed}
                    <span class="truncate">{app.name}</span>
                  {/if}
                  {#if showHealth && !isCollapsed}
                    <span class="ml-auto">
                      <HealthIndicator appName={app.name} size="sm" />
                    </span>
                  {/if}
                  {#if app.open_mode !== 'iframe' && !isCollapsed}
                    <span class="text-xs opacity-60">{getOpenModeIcon(app.open_mode)}</span>
                  {/if}
                </button>
              {/each}
            </div>
          {/if}
        </div>
      {/each}
    </div>

    <!-- Footer (hidden when collapsed) -->
    <div class="p-2 border-t border-gray-700 space-y-1 transition-opacity duration-200"
         style="opacity: {isCollapsed ? '0' : '1'}; pointer-events: {isCollapsed ? 'none' : 'auto'};">
      <button
        class="w-full flex items-center gap-2 px-3 py-2 text-gray-400 hover:text-white rounded-md hover:bg-gray-700 text-sm"
        on:click={() => dispatch('settings')}
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
        <span>Settings</span>
      </button>

      {#if $isAuthenticated && $currentUser}
        <button
          class="w-full flex items-center gap-2 px-3 py-2 text-gray-400 hover:text-red-400 rounded-md hover:bg-gray-700 text-sm transition-colors"
          on:click={handleLogout}
          title="Sign out"
        >
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
          </svg>
          <span>Sign out</span>
        </button>
      {/if}
    </div>
    </div> <!-- End content wrapper -->

    <!-- Resize handle - touch-friendly with pointer events -->
    {#if !isMobile}
      <div
        class="absolute top-0 right-0 w-2 h-full cursor-ew-resize hover:bg-brand-500/50 active:bg-brand-500/70 transition-colors touch-none"
        on:pointerdown={handleResizeStart}
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
  {@const isCollapsedRight = isHidden && config.navigation.auto_hide && !isMobile}
  <aside
    class="border-l flex flex-col h-full transition-all duration-300 relative
           {isMobile ? (mobileMenuOpen ? 'translate-x-0' : 'translate-x-full') : ''}"
    style="
      background: var(--bg-surface);
      border-color: var(--border-subtle);
      width: {isMobile ? '280px' : effectiveSidebarWidth + 'px'};
    "
    on:mouseenter={handleNavEnter}
    on:mouseleave={handleNavLeave}
  >
    <!-- Content wrapper - stays visible when collapsed so icons maintain position -->
    <div
      class="flex flex-col h-full transition-all duration-200 overflow-hidden"
    >
    <!-- Header (hidden when collapsed) -->
    {#if config.navigation.show_logo}
      <div class="p-4 border-b border-gray-700 transition-opacity duration-200"
           style="opacity: {isCollapsedRight ? '0' : '1'}; pointer-events: {isCollapsedRight ? 'none' : 'auto'};">
        <button
          class="text-xl font-bold text-white hover:text-brand-400 transition-colors w-full text-right"
          on:click={() => { dispatch('splash'); mobileMenuOpen = false; }}
        >
          {config.title}
        </button>
      </div>
    {/if}

    <!-- Search button (hidden when collapsed) -->
    <div class="p-2 transition-opacity duration-200"
         style="opacity: {isCollapsedRight ? '0' : '1'}; pointer-events: {isCollapsedRight ? 'none' : 'auto'};">
      <button
        class="w-full flex items-center gap-2 px-3 py-2 text-gray-400 hover:text-white rounded-md hover:bg-gray-700 text-sm"
        on:click={() => dispatch('search')}
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
        <span>Search...</span>
        <span class="ml-auto text-xs bg-gray-700 px-1.5 py-0.5 rounded">⌘K</span>
      </button>
    </div>

    <!-- App list with groups -->
    <div class="flex-1 overflow-y-auto scrollbar-hide p-2">
      {#each groupNames as groupName}
        {@const groupConfig = getGroupConfig(groupName)}
        <div class="mb-2">
          <button
            class="w-full flex items-center gap-2 px-2 py-1.5 text-xs font-semibold text-gray-400 uppercase tracking-wider hover:text-gray-300 transition-opacity duration-200"
            style="opacity: {isCollapsedRight ? '0' : '1'};"
            on:click={() => toggleGroup(groupName)}
          >
            <svg
              class="w-3 h-3 transition-transform {expandedGroups[groupName] ? 'rotate-90' : ''}"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
            </svg>
            {#if groupConfig?.icon?.name}
              <div class="flex-shrink-0">
                <AppIcon icon={groupConfig.icon} name={groupName} color={groupConfig.color || '#374151'} size="sm" showBackground={false} />
              </div>
            {:else if groupConfig?.color}
              <span class="w-2 h-2 rounded-full" style="background-color: {groupConfig.color}"></span>
            {/if}
            <span class="truncate">{groupName}</span>
            <span class="ml-auto text-gray-500">{groupedApps[groupName]?.length || 0}</span>
          </button>

          {#if expandedGroups[groupName] || isCollapsedRight}
            <div class="mt-1 space-y-0.5">
              {#each groupedApps[groupName] || [] as app}
                <button
                  class="w-full flex items-center gap-2 px-2 py-2 rounded-md text-sm transition-colors
                         {currentApp?.name === app.name
                           ? 'bg-gray-700 text-white'
                           : 'text-gray-300 hover:bg-gray-700/50 hover:text-white'}"
                  style="border-left: 3px solid {config.navigation.show_app_colors && (currentApp?.name === app.name || isCollapsedRight) ? (app.color || '#22c55e') : 'transparent'};
                         {config.navigation.show_app_colors && !isCollapsedRight && currentApp?.name === app.name ? `border-right: 3px solid ${app.color || '#22c55e'};` : ''}
                         {isCollapsedRight && currentApp?.name !== app.name ? 'opacity: 0.5;' : ''}"
                  on:click={() => { dispatch('select', app); mobileMenuOpen = false; }}
                >
                  <div class="flex-shrink-0">
                    <AppIcon icon={app.icon} name={app.name} color={app.color} size="sm" showBackground={config.navigation.show_icon_background} />
                  </div>
                  {#if config.navigation.show_labels && !isCollapsedRight}
                    <span class="truncate">{app.name}</span>
                  {/if}
                  {#if showHealth && !isCollapsedRight}
                    <span class="ml-auto">
                      <HealthIndicator appName={app.name} size="sm" />
                    </span>
                  {/if}
                  {#if app.open_mode !== 'iframe' && !isCollapsedRight}
                    <span class="text-xs opacity-60">{getOpenModeIcon(app.open_mode)}</span>
                  {/if}
                </button>
              {/each}
            </div>
          {/if}
        </div>
      {/each}
    </div>

    <!-- Footer (hidden when collapsed) -->
    <div class="p-2 border-t border-gray-700 space-y-1 transition-opacity duration-200"
         style="opacity: {isCollapsedRight ? '0' : '1'}; pointer-events: {isCollapsedRight ? 'none' : 'auto'};">
      <button
        class="w-full flex items-center gap-2 px-3 py-2 text-gray-400 hover:text-white rounded-md hover:bg-gray-700 text-sm"
        on:click={() => dispatch('settings')}
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
        <span>Settings</span>
      </button>

      {#if $isAuthenticated && $currentUser}
        <button
          class="w-full flex items-center gap-2 px-3 py-2 text-gray-400 hover:text-red-400 rounded-md hover:bg-gray-700 text-sm transition-colors"
          on:click={handleLogout}
          title="Sign out"
        >
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
          </svg>
          <span>Sign out</span>
        </button>
      {/if}
    </div>
    </div> <!-- End content wrapper -->

    <!-- Resize handle (left side for right sidebar) - touch-friendly with pointer events -->
    {#if !isMobile}
      <div
        class="absolute top-0 left-0 w-2 h-full cursor-ew-resize hover:bg-brand-500/50 active:bg-brand-500/70 transition-colors touch-none"
        on:pointerdown={handleResizeStart}
        role="slider"
        aria-label="Resize sidebar"
        tabindex="0"
        aria-valuenow={sidebarWidth}
        aria-valuemin={minWidth}
        aria-valuemax={maxWidth}
      ></div>
    {/if}
  </aside>

<!-- BOTTOM BAR (Dock-style) -->
{:else if config.navigation.position === 'bottom'}
  {@const isCollapsedBottom = isHidden && config.navigation.auto_hide}
  <nav
    class="backdrop-blur border-t transition-all duration-300 relative"
    style="
      background: var(--glass-bg);
      border-color: var(--border-subtle);
      height: {isCollapsedBottom ? collapsedStripWidth + 'px' : 'auto'};
    "
    on:mouseenter={handleNavEnter}
    on:mouseleave={handleNavLeave}
  >
    <!-- Collapsed icon strip - show app icons when collapsed -->
    {#if isCollapsedBottom}
      <!-- svelte-ignore a11y-click-events-have-key-events -->
      <div class="absolute inset-0 flex items-center justify-center gap-1 z-20 px-4 cursor-pointer" on:click={() => isHidden = false} role="button" tabindex="0">
        {#each apps as app}
          <button
            class="flex-shrink-0 transition-all duration-200 rounded"
            style="opacity: {currentApp?.name === app.name ? '1' : '0.4'};"
            on:click|stopPropagation={() => dispatch('select', app)}
            title={app.name}
          >
            <AppIcon icon={app.icon} name={app.name} color={app.color} size="sm" showBackground={false} />
          </button>
        {/each}
      </div>
    {/if}
    <!-- Content wrapper -->
    <div
      class="flex items-center justify-center gap-2 p-3 overflow-x-auto scrollbar-hide transition-opacity duration-200"
      style="opacity: {isCollapsedBottom ? '0' : '1'}; pointer-events: {isCollapsedBottom ? 'none' : 'auto'};"
    >
      <!-- Home/Splash button -->
      {#if config.navigation.show_logo}
        <button
          class="p-3 rounded-xl bg-gray-700/50 hover:bg-gray-600/50 transition-all hover:scale-110 group"
          on:click={() => dispatch('splash')}
          title={config.title}
        >
          <svg class="w-6 h-6 text-gray-300 group-hover:text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" />
          </svg>
        </button>
        <div class="w-px h-10 bg-gray-700"></div>
      {/if}

      <!-- App icons -->
      {#each apps as app}
        <button
          class="relative p-2 rounded-xl transition-all hover:scale-110 group
                 {currentApp?.name === app.name ? 'bg-gray-700' : 'hover:bg-gray-700/50'}"
          on:click={() => dispatch('select', app)}
          title={app.name}
        >
          <AppIcon icon={app.icon} name={app.name} color={app.color} size="lg" showBackground={config.navigation.show_icon_background} />

          <!-- Health indicator -->
          {#if showHealth}
            <span class="absolute top-1 right-1">
              <HealthIndicator appName={app.name} size="sm" showTooltip={false} />
            </span>
          {/if}

          <!-- Active indicator dot -->
          {#if currentApp?.name === app.name}
            <span class="absolute bottom-0 left-1/2 -translate-x-1/2 w-1 h-1 rounded-full bg-white"></span>
          {/if}

          <!-- Tooltip on hover -->
          <span class="absolute bottom-full left-1/2 -translate-x-1/2 mb-2 px-2 py-1 text-xs bg-gray-900 text-white rounded opacity-0 group-hover:opacity-100 transition-opacity whitespace-nowrap pointer-events-none">
            {app.name}
            {#if app.open_mode !== 'iframe'}
              {getOpenModeIcon(app.open_mode)}
            {/if}
          </span>
        </button>
      {/each}

      <div class="w-px h-10 bg-gray-700"></div>

      <!-- Search button -->
      <button
        class="p-3 rounded-xl bg-gray-700/50 hover:bg-gray-600/50 transition-all hover:scale-110 group"
        on:click={() => dispatch('search')}
        title="Search (Ctrl+K)"
      >
        <svg class="w-6 h-6 text-gray-300 group-hover:text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
      </button>

      <!-- Settings button -->
      <button
        class="p-3 rounded-xl bg-gray-700/50 hover:bg-gray-600/50 transition-all hover:scale-110 group"
        on:click={() => dispatch('settings')}
        title="Settings"
      >
        <svg class="w-6 h-6 text-gray-300 group-hover:text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
      </button>

      {#if $isAuthenticated && $currentUser}
        <button
          class="p-3 rounded-xl bg-gray-700/50 hover:bg-red-600/30 transition-all hover:scale-110 group"
          on:click={handleLogout}
          title="Sign out"
        >
          <svg class="w-6 h-6 text-gray-300 group-hover:text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
          </svg>
        </button>
      {/if}
    </div>
  </nav>

<!-- FLOATING (Minimal) -->
{:else if config.navigation.position === 'floating'}
  {@const floatingPosition = 'bottom-6 right-6'}
  <div
    class="fixed {floatingPosition} z-40 transition-all duration-300"
    class:opacity-50={isHidden && config.navigation.auto_hide}
    class:scale-90={isHidden && config.navigation.auto_hide}
    on:mouseenter={handleNavEnter}
    on:mouseleave={handleNavLeave}
  >
    <!-- Expanded menu -->
    <div class="flex flex-col-reverse items-end gap-2 mb-2">
      {#each apps.slice(0, 6) as app}
        <button
          class="flex items-center gap-2 px-3 py-2 bg-gray-800 border border-gray-700 rounded-full shadow-lg
                 hover:bg-gray-700 transition-all hover:scale-105
                 {currentApp?.name === app.name ? 'ring-2 ring-brand-500' : ''}"
          on:click={() => dispatch('select', app)}
        >
          <AppIcon icon={app.icon} name={app.name} color={app.color} size="md" showBackground={config.navigation.show_icon_background} />
          <span class="text-sm text-white pr-1">{app.name}</span>
          {#if showHealth}
            <HealthIndicator appName={app.name} size="sm" />
          {/if}
        </button>
      {/each}

      {#if apps.length > 6}
        <button
          class="flex items-center gap-2 px-3 py-2 bg-gray-800 border border-gray-700 rounded-full shadow-lg hover:bg-gray-700 transition-all"
          on:click={() => dispatch('splash')}
        >
          <span class="text-sm text-gray-400">+{apps.length - 6} more</span>
        </button>
      {/if}
    </div>

    <!-- Main FAB buttons -->
    <div class="flex items-center gap-2">
      {#if $isAuthenticated && $currentUser}
        <button
          class="p-3 bg-gray-800 border border-gray-700 rounded-full shadow-lg hover:bg-gray-700 transition-all hover:scale-110"
          on:click={handleLogout}
          title="Sign out"
        >
          <svg class="w-5 h-5 text-gray-300 hover:text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
          </svg>
        </button>
      {/if}
      <button
        class="p-3 bg-gray-800 border border-gray-700 rounded-full shadow-lg hover:bg-gray-700 transition-all hover:scale-110"
        on:click={() => dispatch('search')}
        title="Search"
      >
        <svg class="w-5 h-5 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
      </button>
      <button
        class="p-3 bg-gray-800 border border-gray-700 rounded-full shadow-lg hover:bg-gray-700 transition-all hover:scale-110"
        on:click={() => dispatch('settings')}
        title="Settings"
      >
        <svg class="w-5 h-5 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
      </button>
      <button
        class="p-4 bg-brand-600 hover:bg-brand-700 text-white rounded-full shadow-lg transition-all hover:scale-110"
        on:click={() => dispatch('splash')}
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

  /* ═══════════════════════════════════════════════════════════════════════════
     THEME-AWARE NAVIGATION STYLES
     Override Tailwind gray classes with CSS custom properties
     ═══════════════════════════════════════════════════════════════════════════ */

  /* Navigation containers */
  nav, aside {
    background: var(--bg-surface) !important;
    border-color: var(--border-subtle) !important;
  }

  /* Glass effect for bottom bar */
  nav[class*="backdrop-blur"] {
    background: var(--glass-bg) !important;
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
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
    --tw-ring-color: var(--accent-primary) !important;
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
</style>
