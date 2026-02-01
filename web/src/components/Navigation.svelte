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
  function handleMouseMove(e: MouseEvent) {
    if (!config.navigation.auto_hide) return;

    const threshold = 20;
    const pos = config.navigation.position;

    let shouldShow = false;
    if (pos === 'left' && e.clientX < threshold) shouldShow = true;
    if (pos === 'right' && e.clientX > window.innerWidth - threshold) shouldShow = true;
    if (pos === 'top' && e.clientY < threshold) shouldShow = true;
    if (pos === 'bottom' && e.clientY > window.innerHeight - threshold) shouldShow = true;

    if (shouldShow && isHidden) {
      isHidden = false;
      if (hideTimeout) clearTimeout(hideTimeout);
    } else if (!shouldShow && !isHidden && config.navigation.auto_hide) {
      if (hideTimeout) clearTimeout(hideTimeout);
      const delayMs = parseDelay(config.navigation.auto_hide_delay);
      hideTimeout = setTimeout(() => {
        isHidden = true;
      }, delayMs);
    }
  }

  function parseDelay(delay: string): number {
    const match = delay.match(/^(\d+)(ms|s)?$/);
    if (!match) return 3000;
    const value = parseInt(match[1], 10);
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
  <nav
    class="bg-gray-800 border-b border-gray-700 transition-transform duration-300 {isHidden && config.navigation.auto_hide ? '-translate-y-full' : ''}"
    on:mouseenter={() => { if (config.navigation.auto_hide) { isHidden = false; if (hideTimeout) clearTimeout(hideTimeout); } }}
  >
    <div class="flex items-center justify-between h-14 px-4">
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
              style={currentApp?.name === app.name ? `border-bottom: 2px solid ${app.color || '#22c55e'}` : ''}
              on:click={() => dispatch('select', app)}
            >
              {#if !config.navigation.show_labels}
                <!-- Icon only mode -->
                <AppIcon icon={app.icon} name={app.name} color={app.color} size="sm" />
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
      </div>
    </div>
  </nav>

<!-- LEFT SIDEBAR -->
{:else if config.navigation.position === 'left'}
  <aside
    class="bg-gray-800 border-r border-gray-700 flex flex-col h-full transition-transform duration-300
           {isMobile ? (mobileMenuOpen ? 'translate-x-0' : '-translate-x-full') : ''}
           {isHidden && config.navigation.auto_hide && !isMobile ? '-translate-x-full' : ''}"
    style="width: {isMobile ? '280px' : sidebarWidth + 'px'}"
    on:mouseenter={() => { if (config.navigation.auto_hide) { isHidden = false; if (hideTimeout) clearTimeout(hideTimeout); } }}
  >
    <!-- Header -->
    {#if config.navigation.show_logo}
      <div class="p-4 border-b border-gray-700">
        <button
          class="text-xl font-bold text-white hover:text-brand-400 transition-colors w-full text-left"
          on:click={() => { dispatch('splash'); mobileMenuOpen = false; }}
        >
          {config.title}
        </button>
      </div>
    {/if}

    <!-- Search button -->
    <div class="p-2">
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
          <!-- Group header (collapsible) -->
          <button
            class="w-full flex items-center gap-2 px-2 py-1.5 text-xs font-semibold text-gray-400 uppercase tracking-wider hover:text-gray-300"
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
            {#if groupConfig?.color}
              <span class="w-2 h-2 rounded-full" style="background-color: {groupConfig.color}"></span>
            {/if}
            <span>{groupName}</span>
            <span class="ml-auto text-gray-500">{groupedApps[groupName]?.length || 0}</span>
          </button>

          <!-- Apps in group -->
          {#if expandedGroups[groupName]}
            <div class="mt-1 space-y-0.5">
              {#each groupedApps[groupName] || [] as app}
                <button
                  class="w-full flex items-center gap-2 px-3 py-2 rounded-md text-sm transition-colors
                         {currentApp?.name === app.name
                           ? 'bg-gray-700 text-white'
                           : 'text-gray-300 hover:bg-gray-700/50 hover:text-white'}"
                  style={currentApp?.name === app.name ? `border-left: 3px solid ${app.color || '#22c55e'}` : 'border-left: 3px solid transparent'}
                  on:click={() => { dispatch('select', app); mobileMenuOpen = false; }}
                >
                  <!-- App icon -->
                  <div class="flex-shrink-0">
                    <AppIcon icon={app.icon} name={app.name} color={app.color} size="sm" />
                  </div>
                  {#if config.navigation.show_labels}
                    <span class="truncate">{app.name}</span>
                  {/if}
                  {#if showHealth}
                    <span class="ml-auto">
                      <HealthIndicator appName={app.name} size="sm" />
                    </span>
                  {/if}
                  {#if app.open_mode !== 'iframe'}
                    <span class="text-xs opacity-60">{getOpenModeIcon(app.open_mode)}</span>
                  {/if}
                </button>
              {/each}
            </div>
          {/if}
        </div>
      {/each}
    </div>

    <!-- Footer with settings and user menu -->
    <div class="p-2 border-t border-gray-700 space-y-1">
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
        <div class="flex items-center justify-between px-3 py-2 text-sm">
          <span class="text-gray-400 truncate">{$currentUser.display_name || $currentUser.username}</span>
          <button
            class="text-gray-500 hover:text-red-400 transition-colors"
            on:click={handleLogout}
            title="Sign out"
          >
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
            </svg>
          </button>
        </div>
      {/if}
    </div>

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
  <aside
    class="bg-gray-800 border-l border-gray-700 flex flex-col h-full transition-transform duration-300
           {isMobile ? (mobileMenuOpen ? 'translate-x-0' : 'translate-x-full') : ''}
           {isHidden && config.navigation.auto_hide && !isMobile ? 'translate-x-full' : ''}"
    style="width: {isMobile ? '280px' : sidebarWidth + 'px'}"
    on:mouseenter={() => { if (config.navigation.auto_hide) { isHidden = false; if (hideTimeout) clearTimeout(hideTimeout); } }}
  >
    <!-- Header -->
    {#if config.navigation.show_logo}
      <div class="p-4 border-b border-gray-700">
        <button
          class="text-xl font-bold text-white hover:text-brand-400 transition-colors w-full text-right"
          on:click={() => { dispatch('splash'); mobileMenuOpen = false; }}
        >
          {config.title}
        </button>
      </div>
    {/if}

    <!-- Search button -->
    <div class="p-2">
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
            class="w-full flex items-center gap-2 px-2 py-1.5 text-xs font-semibold text-gray-400 uppercase tracking-wider hover:text-gray-300"
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
            {#if groupConfig?.color}
              <span class="w-2 h-2 rounded-full" style="background-color: {groupConfig.color}"></span>
            {/if}
            <span>{groupName}</span>
            <span class="ml-auto text-gray-500">{groupedApps[groupName]?.length || 0}</span>
          </button>

          {#if expandedGroups[groupName]}
            <div class="mt-1 space-y-0.5">
              {#each groupedApps[groupName] || [] as app}
                <button
                  class="w-full flex items-center gap-2 px-3 py-2 rounded-md text-sm transition-colors
                         {currentApp?.name === app.name
                           ? 'bg-gray-700 text-white'
                           : 'text-gray-300 hover:bg-gray-700/50 hover:text-white'}"
                  style={currentApp?.name === app.name ? `border-right: 3px solid ${app.color || '#22c55e'}` : 'border-right: 3px solid transparent'}
                  on:click={() => { dispatch('select', app); mobileMenuOpen = false; }}
                >
                  <div class="flex-shrink-0">
                    <AppIcon icon={app.icon} name={app.name} color={app.color} size="sm" />
                  </div>
                  {#if config.navigation.show_labels}
                    <span class="truncate">{app.name}</span>
                  {/if}
                  {#if showHealth}
                    <span class="ml-auto">
                      <HealthIndicator appName={app.name} size="sm" />
                    </span>
                  {/if}
                  {#if app.open_mode !== 'iframe'}
                    <span class="text-xs opacity-60">{getOpenModeIcon(app.open_mode)}</span>
                  {/if}
                </button>
              {/each}
            </div>
          {/if}
        </div>
      {/each}
    </div>

    <!-- Footer with settings -->
    <div class="p-2 border-t border-gray-700">
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
    </div>

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
  <nav
    class="bg-gray-800/95 backdrop-blur border-t border-gray-700 transition-transform duration-300
           {isHidden && config.navigation.auto_hide ? 'translate-y-full' : ''}"
    on:mouseenter={() => { if (config.navigation.auto_hide) { isHidden = false; if (hideTimeout) clearTimeout(hideTimeout); } }}
  >
    <div class="flex items-center justify-center gap-2 p-3 overflow-x-auto scrollbar-hide">
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
          <AppIcon icon={app.icon} name={app.name} color={app.color} size="lg" />

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
    </div>
  </nav>

<!-- FLOATING (Minimal) -->
{:else if config.navigation.position === 'floating'}
  {@const floatingPosition = 'bottom-6 right-6'}
  <div
    class="fixed {floatingPosition} z-40 transition-all duration-300"
    class:opacity-50={isHidden && config.navigation.auto_hide}
    class:scale-90={isHidden && config.navigation.auto_hide}
    on:mouseenter={() => { if (config.navigation.auto_hide) { isHidden = false; if (hideTimeout) clearTimeout(hideTimeout); } }}
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
          <AppIcon icon={app.icon} name={app.name} color={app.color} size="md" />
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
</style>
