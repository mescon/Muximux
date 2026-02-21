<script lang="ts">
  import { onMount } from 'svelte';
  import type { App, Config, Group } from '$lib/types';
  import AppIcon from './AppIcon.svelte';
  import HealthIndicator from './HealthIndicator.svelte';
  import { healthData } from '$lib/healthStore';
  import { currentUser, isAuthenticated, isAdmin, logout } from '$lib/authStore';
  import { createEdgeSwipeHandlers, isTouchDevice } from '$lib/useSwipe';
  import MuximuxLogo from './MuximuxLogo.svelte';

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
    splitEnabled = false,
    splitOrientation = 'horizontal' as 'horizontal' | 'vertical',
    splitActivePanel = 0 as 0 | 1,
    onsplithorizontal,
    onsplitvertical,
    onsplitclose,
    onsplitpanel,
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
    splitEnabled?: boolean;
    splitOrientation?: 'horizontal' | 'vertical';
    splitActivePanel?: 0 | 1;
    onsplithorizontal?: () => void;
    onsplitvertical?: () => void;
    onsplitclose?: () => void;
    onsplitpanel?: (panel: 0 | 1) => void;
  } = $props();

  async function handleLogout() {
    await logout();
    onlogout?.();
  }

  // Split icon SVG paths (Lucide Columns2 and Rows2 rects)
  const splitHPath = 'M3 3h7v18H3zM14 3h7v18h-7z';
  const splitVPath = 'M3 3h18v7H3zM3 14h18v7H3z';

  // Panel selector arrow paths based on orientation
  const panelArrow0 = $derived(splitOrientation === 'vertical' ? 'M18 15l-6-6-6 6' : 'M15 18l-6-6 6-6');
  const panelArrow1 = $derived(splitOrientation === 'vertical' ? 'M6 9l6 6 6-6' : 'M9 18l6-6-6-6');

  // Whether health indicators should be shown for a given app.
  // Health checks are opt-in: only shown when app.health_check === true.
  function shouldShowHealth(app: App): boolean {
    if (!showHealth) return false;
    return app.health_check === true;
  }

  function isUnhealthy(app: App): boolean {
    if (!shouldShowHealth(app)) return false;
    return $healthData.get(app.name)?.status === 'unhealthy';
  }

  // Draggable floating FAB state
  const FAB_MARGIN = 24;
  const FAB_SIZE = 56;
  const DRAG_THRESHOLD = 5;
  const FAB_LS_KEY = 'muximux_fab_position';

  let fabX = $state(0);
  let fabY = $state(0);
  let fabInitialized = $state(false);
  let isDraggingFab = $state(false);
  let fabDragStartX = 0;
  let fabDragStartY = 0;
  let fabDragPointerId = -1;
  let fabDragDidMove = false;

  const PANEL_GAP = 12;
  const PANEL_MARGIN = 12;

  // Compute panel position so it never overflows the viewport.
  // Panel appears on the side of the FAB with the most horizontal space.
  // Vertical anchor: top-aligned when FAB is in the top third, bottom-aligned
  // in the bottom third, centered in the middle third.
  let panelPos = $derived.by(() => {
    if (typeof window === 'undefined') return { left: 0, top: 0, bottom: 'auto' as string | number, maxHeight: 400, width: 300, slideX: '-10px', useBottom: false };

    const vw = window.innerWidth;
    const vh = window.innerHeight;
    const half = FAB_SIZE / 2;
    const pw = isMobile ? Math.min(vw - 48, 360) : 300;

    // Horizontal: pick side with more space
    const spaceRight = vw - fabX - half - PANEL_GAP;
    const spaceLeft = fabX - half - PANEL_GAP;
    let left: number;
    let slideX: string;
    if (spaceRight >= spaceLeft) {
      left = fabX + half + PANEL_GAP;
      slideX = '-10px';
    } else {
      left = fabX - half - PANEL_GAP - pw;
      slideX = '10px';
    }
    left = Math.max(PANEL_MARGIN, Math.min(vw - pw - PANEL_MARGIN, left));

    // Vertical anchor based on FAB position in viewport thirds
    const topThird = vh / 3;
    const bottomThird = vh * 2 / 3;

    if (fabY <= topThird) {
      // Top zone: panel top aligns with FAB top, grows downward
      const top = Math.max(PANEL_MARGIN, fabY - half);
      const maxHeight = Math.min(vh * 0.7, vh - top - PANEL_MARGIN);
      return { left, top, bottom: 'auto' as string | number, maxHeight, width: pw, slideX, useBottom: false };
    } else if (fabY >= bottomThird) {
      // Bottom zone: panel bottom aligns with FAB bottom, grows upward
      const bottom = Math.max(PANEL_MARGIN, vh - fabY - half);
      const maxHeight = Math.min(vh * 0.7, vh - bottom - PANEL_MARGIN);
      return { left, top: 0, bottom, maxHeight, width: pw, slideX, useBottom: true };
    } else {
      // Middle zone: panel vertically centered on FAB
      const maxHeight = Math.min(vh * 0.7, vh - PANEL_MARGIN * 2);
      let top = fabY - maxHeight / 2;
      if (top < PANEL_MARGIN) top = PANEL_MARGIN;
      if (top + maxHeight > vh - PANEL_MARGIN) top = vh - PANEL_MARGIN - maxHeight;
      return { left, top, bottom: 'auto' as string | number, maxHeight, width: pw, slideX, useBottom: false };
    }
  });

  function floatingPositionToCoords(pos: string): { x: number; y: number } {
    const vw = window.innerWidth, vh = window.innerHeight;
    const half = FAB_SIZE / 2;
    return {
      x: pos.endsWith('right') ? vw - FAB_MARGIN - half : FAB_MARGIN + half,
      y: pos.startsWith('bottom') ? vh - FAB_MARGIN - half : FAB_MARGIN + half,
    };
  }

  function clampFabPosition(x: number, y: number) {
    const half = FAB_SIZE / 2;
    return {
      x: Math.min(window.innerWidth - FAB_MARGIN - half, Math.max(FAB_MARGIN + half, x)),
      y: Math.min(window.innerHeight - FAB_MARGIN - half, Math.max(FAB_MARGIN + half, y)),
    };
  }

  function persistFabPosition() {
    localStorage.setItem(FAB_LS_KEY, JSON.stringify({ x: fabX, y: fabY }));
  }

  function handleFabResize() {
    if (!fabInitialized) return;
    const c = clampFabPosition(fabX, fabY);
    fabX = c.x; fabY = c.y;
    persistFabPosition();
  }

  function handleFabPointerDown(e: PointerEvent) {
    if (e.button !== 0) return;
    fabDragStartX = e.clientX;
    fabDragStartY = e.clientY;
    fabDragPointerId = e.pointerId;
    fabDragDidMove = false;
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    document.addEventListener('pointermove', handleFabPointerMove);
    document.addEventListener('pointerup', handleFabPointerUp);
    document.addEventListener('pointercancel', handleFabPointerUp);
  }

  function handleFabPointerMove(e: PointerEvent) {
    if (e.pointerId !== fabDragPointerId) return;
    const dx = e.clientX - fabDragStartX;
    const dy = e.clientY - fabDragStartY;

    if (!fabDragDidMove) {
      if (Math.abs(dx) < DRAG_THRESHOLD && Math.abs(dy) < DRAG_THRESHOLD) return;
      fabDragDidMove = true;
      isDraggingFab = true;
      if (panelOpen) panelOpen = false;
      handleNavEnter();
      document.body.style.cursor = 'grabbing';
      document.querySelectorAll('iframe').forEach(f => {
        (f as HTMLElement).style.pointerEvents = 'none';
      });
    }

    const c = clampFabPosition(e.clientX, e.clientY);
    fabX = c.x;
    fabY = c.y;
  }

  function handleFabPointerUp(e: PointerEvent) {
    if (e.pointerId !== fabDragPointerId) return;
    document.removeEventListener('pointermove', handleFabPointerMove);
    document.removeEventListener('pointerup', handleFabPointerUp);
    document.removeEventListener('pointercancel', handleFabPointerUp);
    fabDragPointerId = -1;

    if (fabDragDidMove) {
      isDraggingFab = false;
      persistFabPosition();
      document.body.style.cursor = '';
      document.querySelectorAll('iframe').forEach(f => {
        (f as HTMLElement).style.pointerEvents = '';
      });
    } else {
      panelOpen = !panelOpen;
      if (panelOpen) isHidden = false;
    }
  }

  // Sidebar width state (for left/right layouts)
  let sidebarWidth = $state(220);
  let isResizing = $state(false);
  let minWidth = 180;
  let maxWidth = 400;

  // Auto-hide state
  let isHidden = $state(false);
  let hideTimeout: ReturnType<typeof setTimeout> | null = null;
  // Delay overflow:visible on top/bottom bars until height transition (300ms) completes
  let barOverflowVisible = $state(false);
  let barOverflowTimer: ReturnType<typeof setTimeout> | null = null;
  const collapsedStripWidth = 48; // Width of visible strip when sidebar collapsed (fits icon + border)
  const collapsedBarHeight = 6; // Height of visible strip when top/bottom bar collapsed (thin reveal strip)

  // Footer drawer state (for collapsible sidebar footer)
  let footerDrawerOpen = $state(false);
  let footerDrawerTimer: ReturnType<typeof setTimeout> | null = null;

  // Group expansion state (persisted to localStorage)
  let expandedGroups: Record<string, boolean> = $state({});

  // Responsive state
  let isMobile = $state(false);

  // On mobile, force floating nav regardless of config
  let effectivePosition = $derived(isMobile ? 'floating' : config.navigation.position);
  let effectiveFloatingPosition = $derived(
    isMobile ? 'bottom-right' : (config.navigation.floating_position || 'bottom-right')
  );

  // Calculate actual width for sidebars (for layout reflow)
  // When auto_hide is on, always reserve only the collapsed strip in the layout.
  // The expanded sidebar overlays the content instead of pushing it.
  let effectiveSidebarWidth = $derived((config.navigation.auto_hide || !config.navigation.show_labels) && !isMobile ? collapsedStripWidth : sidebarWidth);
  let mobileMenuOpen = $state(false);
  let panelOpen = $state(false);
  let hasTouchSupport = $state(false);

  // Group dropdown state for top/bottom bars
  let openGroupDropdown = $state<string | null>(null);
  let dropdownCloseTimeout: ReturnType<typeof setTimeout> | null = null;

  function openDropdown(groupName: string) {
    if (dropdownCloseTimeout) { clearTimeout(dropdownCloseTimeout); dropdownCloseTimeout = null; }
    openGroupDropdown = groupName;
  }
  function scheduleDropdownClose() {
    dropdownCloseTimeout = setTimeout(() => { openGroupDropdown = null; }, 150);
  }
  function cancelDropdownClose() {
    if (dropdownCloseTimeout) { clearTimeout(dropdownCloseTimeout); dropdownCloseTimeout = null; }
  }

  // Hovered group in flat bar mode (highlights member apps)
  let hoveredGroup = $state<string | null>(null);

  // Convert vertical mouse wheel to horizontal scroll for flat bar mode
  function handleBarWheel(e: WheelEvent) {
    const el = e.currentTarget as HTMLElement;
    if (!el || el.scrollWidth <= el.clientWidth) return;
    e.preventDefault();
    el.scrollLeft += e.deltaY || e.deltaX;
  }

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

  // Whether we have real groups (not just one "Ungrouped" bucket)
  let hasRealGroups = $derived(groupNames.length > 1 || (groupNames.length === 1 && groupNames[0] !== 'Ungrouped'));

  // Top/bottom bar mode: grouped dropdowns vs flat scrollable list
  let useGroupDropdowns = $derived(hasRealGroups && config.navigation.bar_style !== 'flat');

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

    window.addEventListener('keydown', handlePanelKeydown);

    // Restore FAB position from localStorage or fall back to config corner
    const storedFab = localStorage.getItem(FAB_LS_KEY);
    if (storedFab) {
      try {
        const p = JSON.parse(storedFab);
        const c = clampFabPosition(p.x, p.y);
        fabX = c.x; fabY = c.y;
      } catch { /* fall through to default */ }
    }
    if (!fabX && !fabY) {
      const coords = floatingPositionToCoords(effectiveFloatingPosition);
      fabX = coords.x; fabY = coords.y;
    }
    fabInitialized = true;
    window.addEventListener('resize', handleFabResize);

    return () => {
      window.removeEventListener('resize', checkResponsive);
      window.removeEventListener('resize', handleFabResize);
      document.removeEventListener('pointermove', handleResizeMove);
      document.removeEventListener('pointerup', handleResizeEnd);
      document.removeEventListener('pointercancel', handleResizeEnd);
      document.removeEventListener('pointermove', handleFabPointerMove);
      document.removeEventListener('pointerup', handleFabPointerUp);
      document.removeEventListener('pointercancel', handleFabPointerUp);
      cleanupEdgeSwipe();
      window.removeEventListener('keydown', handlePanelKeydown);
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

  // Position a tooltip with `position: fixed` so it escapes overflow-hidden scroll containers.
  // `above` = true renders above the anchor (bottom bar), false renders below (top bar).
  function fixedTooltip(node: HTMLElement, above: boolean) {
    const anchor = node.parentElement;
    if (!anchor) return;
    const rect = anchor.getBoundingClientRect();
    node.style.left = `${rect.left + rect.width / 2}px`;
    if (above) {
      node.style.bottom = `${window.innerHeight - rect.top + 4}px`;
    } else {
      node.style.top = `${rect.bottom + 4}px`;
    }
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

    if (effectivePosition === 'left') {
      sidebarWidth = Math.min(maxWidth, Math.max(minWidth, e.clientX));
    } else if (effectivePosition === 'right') {
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
    // Don't setup sidebar edge swipe when floating is forced on mobile
    if (effectivePosition === 'floating') return;

    const edge = effectivePosition === 'right' ? 'right' : 'left';
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

  function handlePanelKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      if (openGroupDropdown) { openGroupDropdown = null; return; }
      if (panelOpen) { panelOpen = false; }
    }
  }

  // Auto-hide handling
  // Nav element enter/leave: controls hide timer based on whether mouse is inside the nav
  function handleNavEnter() {
    if (!config.navigation.auto_hide) return;
    isHidden = false;
    if (hideTimeout) clearTimeout(hideTimeout);
    // Delay overflow:visible until height transition finishes (top/bottom bars)
    if (barOverflowTimer) clearTimeout(barOverflowTimer);
    barOverflowTimer = setTimeout(() => { barOverflowVisible = true; }, 300);
  }

  function handleNavLeave() {
    if (!config.navigation.auto_hide) return;
    if (hideTimeout) clearTimeout(hideTimeout);
    // Immediately hide overflow when collapsing
    barOverflowVisible = false;
    if (barOverflowTimer) clearTimeout(barOverflowTimer);
    const delayMs = parseDelay(config.navigation.auto_hide_delay);
    hideTimeout = setTimeout(() => {
      isHidden = true;
      footerDrawerOpen = false;
      if (effectivePosition === 'floating') panelOpen = false;
    }, delayMs);
  }

  // Footer drawer hover handlers (for collapsible sidebar footer)
  function handleFooterEnter() {
    if (footerDrawerTimer) clearTimeout(footerDrawerTimer);
    footerDrawerOpen = true;
  }
  function handleFooterLeave() {
    if (footerDrawerTimer) clearTimeout(footerDrawerTimer);
    footerDrawerTimer = setTimeout(() => { footerDrawerOpen = false; }, 300);
  }
  // When the collapsed cogwheel is hovered, pre-open the drawer so it's
  // visible as soon as the sidebar expands via the aside's handleNavEnter.
  function handleCollapsedFooterEnter() {
    footerDrawerOpen = true;
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

  // Should the footer drawer be active? Only for expanded left/right sidebars on desktop.
  let useFooterDrawer = $derived(
    config.navigation.hide_sidebar_footer &&
    (effectivePosition === 'left' || effectivePosition === 'right') &&
    !isMobile
  );

  // Hide logout when auth is 'none' — the virtual admin user shouldn't appear to be "logged in"
  let hasRealAuth = $derived(config.auth?.method !== undefined && config.auth.method !== 'none');


</script>

<!-- Mobile hamburger menu -->
{#if isMobile && effectivePosition !== 'bottom' && effectivePosition !== 'floating'}
  <button
    class="fixed top-4 left-4 z-50 p-2 bg-bg-surface rounded-lg border border-border text-text-primary lg:hidden"
    onclick={() => mobileMenuOpen = !mobileMenuOpen}
    aria-label={mobileMenuOpen ? 'Close navigation menu' : 'Open navigation menu'}
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
{#if effectivePosition === 'top'}
  {@const isCollapsedTop = isHidden && config.navigation.auto_hide}
  <nav
    class="relative z-10"
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
      style:overflow={!config.navigation.auto_hide || barOverflowVisible ? 'visible' : 'hidden'}
      style:box-shadow={config.navigation.auto_hide && config.navigation.show_shadow ? '0 4px 24px rgba(0,0,0,0.25)' : null}
      style:position={config.navigation.auto_hide ? 'absolute' : null}
      style:top={config.navigation.auto_hide ? '0' : null}
      style:left={config.navigation.auto_hide ? '0' : null}
      style:right={config.navigation.auto_hide ? '0' : null}
      style:z-index={config.navigation.auto_hide ? '30' : null}
    >
    <div
      class="flex items-center gap-4 px-4"
      style="height: 56px;"
    >
      <!-- Logo — fixed -->
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
          class="flex-shrink-0 p-1 rounded-md hover:bg-bg-hover transition-all"
          style="color: var(--accent-primary); opacity: {showSplash ? '0.6' : '1'}; transition: opacity 0.2s ease;"
          onclick={() => onsplash?.()}
          title="Overview"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-4 0a1 1 0 01-1-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 01-1 1h-2z" />
          </svg>
        </button>
      {/if}

      <!-- App tabs — flex-1 so it fills between logo and actions -->
      {#if useGroupDropdowns}
        <!-- Grouped: dropdown buttons, no scroll needed (overflow visible for dropdowns) -->
        <div class="flex-1 min-w-0 flex items-center space-x-1" style="overflow: visible;">
          {#each groupNames as groupName (groupName)}
            {@const groupConfig = getGroupConfig(groupName)}
            {@const groupApps = groupedApps[groupName] || []}
            {@const hasActiveApp = groupApps.some(a => currentApp?.name === a.name)}
            <!-- svelte-ignore a11y_no_static_element_interactions -->
            <div class="relative"
              onmouseenter={() => openDropdown(groupName)}
              onmouseleave={scheduleDropdownClose}
            >
              <button
                class="px-2.5 py-2 rounded-md text-sm font-medium transition-colors whitespace-nowrap flex items-center gap-1.5
                       {hasActiveApp
                         ? 'text-text-primary'
                         : openGroupDropdown === groupName ? 'bg-bg-hover text-text-primary' : 'text-text-muted hover:bg-bg-hover hover:text-text-primary'}"
                style={hasActiveApp && config.navigation.show_app_colors ? `border-bottom: 2px solid ${groupConfig?.color || currentApp?.color || 'var(--accent-primary)'}` : ''}
                onclick={() => openGroupDropdown = openGroupDropdown === groupName ? null : groupName}
              >
                {#if groupConfig?.icon?.name}
                  <AppIcon icon={groupConfig.icon} name={groupName} color={groupConfig.color || ''} size="sm" scale={iconScale} showBackground={false} />
                {/if}
                <span>{groupName}</span>
                <svg class="w-3 h-3 opacity-50" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                </svg>
              </button>

              {#if openGroupDropdown === groupName}
                <!-- svelte-ignore a11y_no_static_element_interactions -->
                <div
                  class="absolute top-full left-0 mt-1 min-w-[200px] max-w-[280px] rounded-lg border shadow-xl overflow-hidden z-50"
                  style="background: var(--bg-surface); border-color: var(--border-subtle);"
                  onmouseenter={cancelDropdownClose}
                  onmouseleave={scheduleDropdownClose}
                >
                  <div class="py-1 max-h-[60vh] overflow-y-auto overflow-x-hidden scrollbar-styled">
                    {#each groupApps as app (app.name)}
                      <button
                        class="group-dropdown-item w-full flex items-center gap-2 px-3 py-2 text-sm transition-colors
                               {currentApp?.name === app.name
                                 ? 'text-text-primary'
                                 : 'text-text-secondary hover:text-text-primary'}
                               {isUnhealthy(app) && currentApp?.name !== app.name ? 'opacity-50' : ''}"
                        style="background: {currentApp?.name === app.name ? 'var(--bg-hover)' : 'transparent'};"
                        onclick={() => { onselect?.(app); openGroupDropdown = null; }}
                      >
                        {#if config.navigation.show_app_colors && currentApp?.name === app.name}
                          <div class="absolute left-0 top-1 bottom-1 w-[3px] rounded-full" style="background: {app.color || '#22c55e'};"></div>
                        {/if}
                        <AppIcon icon={app.icon} name={app.name} color={app.color} size="sm" scale={iconScale} showBackground={config.navigation.show_icon_background} forceBackground={app.force_icon_background} />
                        <span class="truncate">{app.name}</span>
                        {#if shouldShowHealth(app)}
                          <span class="ml-auto flex-shrink-0"><HealthIndicator appName={app.name} size="sm" /></span>
                        {/if}
                        {#if app.open_mode !== 'iframe'}
                          <span class="text-xs opacity-60 flex-shrink-0">{getOpenModeIcon(app.open_mode)}</span>
                        {/if}
                      </button>
                    {/each}
                  </div>
                </div>
              {/if}
            </div>
          {/each}
        </div>
      {:else}
        <!-- Flat: scrollable app list with group icon dividers -->
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div class="flat-bar-scroll flex-1 min-w-0 flex items-center" onwheel={handleBarWheel} onmouseleave={() => hoveredGroup = null}>
          {#each groupNames as groupName, gi (groupName)}
            {@const groupConfig = getGroupConfig(groupName)}
            {#if hasRealGroups}
              <!-- Group icon divider -->
              <!-- svelte-ignore a11y_no_static_element_interactions -->
              <div
                class="flat-group-divider relative flex-shrink-0 flex items-center px-1.5 py-2 rounded-md transition-colors cursor-default
                       {hoveredGroup === groupName ? 'bg-bg-elevated/60' : ''}"
                style="{gi > 0 ? 'margin-left: 2px;' : ''}"
                onmouseenter={() => hoveredGroup = groupName}
                title={groupName}
              >
                {#if groupConfig?.icon?.name}
                  <span style="opacity: {hoveredGroup === groupName ? '1' : '0.4'}; transition: opacity 0.15s ease;">
                    <AppIcon icon={groupConfig.icon} name={groupName} color={groupConfig.color || ''} size="sm" scale={iconScale * 0.85} showBackground={false} />
                  </span>
                {:else}
                  <div class="w-px h-5" style="background: var(--border-subtle); opacity: {hoveredGroup === groupName ? '1' : '0.5'};"></div>
                {/if}
                {#if hoveredGroup === groupName}
                  <span
                    use:fixedTooltip={false}
                    class="fixed -translate-x-1/2 whitespace-nowrap text-[10px] font-semibold uppercase tracking-wider pointer-events-none z-[100] px-1.5 py-0.5 rounded bg-bg-elevated/90 text-text-muted shadow-sm"
                  >{groupName}</span>
                {/if}
              </div>
            {/if}
            {#each groupedApps[groupName] || [] as app (app.name)}
              <button
                class="relative group flex-shrink-0 px-2 py-2 rounded-md text-sm font-medium transition-all whitespace-nowrap flex items-center gap-1
                       {currentApp?.name === app.name
                         ? 'bg-bg-base text-text-primary'
                         : 'text-text-secondary hover:bg-bg-hover hover:text-text-primary'}
                       {isUnhealthy(app) && currentApp?.name !== app.name ? 'opacity-50' : ''}"
                style="border-bottom: 2px solid {config.navigation.show_app_colors && currentApp?.name === app.name ? (app.color || '#22c55e') : 'transparent'};
                       {hoveredGroup && hoveredGroup !== app.group ? 'opacity: 0.3;' : ''}"
                onclick={() => onselect?.(app)}
                onmouseenter={() => hoveredGroup = null}
              >
                <AppIcon icon={app.icon} name={app.name} color={app.color} size="sm" scale={iconScale} showBackground={config.navigation.show_icon_background} forceBackground={app.force_icon_background} />
                {#if config.navigation.show_labels}
                  <span>{app.name}</span>
                {:else}
                  <span class="inline-block max-w-0 overflow-hidden opacity-0 group-hover:max-w-[120px] group-hover:opacity-100 transition-all duration-200 whitespace-nowrap">{app.name}</span>
                {/if}
                {#if shouldShowHealth(app)}
                  <HealthIndicator appName={app.name} size="sm" />
                {/if}
                {#if app.open_mode !== 'iframe'}
                  <span class="text-xs opacity-60">{getOpenModeIcon(app.open_mode)}</span>
                {/if}
              </button>
            {/each}
          {/each}
        </div>
      {/if}

      <!-- Right side actions -->
      <div class="flex items-center space-x-2">
        <button
          class="p-2 text-text-muted hover:text-text-primary rounded-md hover:bg-bg-hover"
          onclick={() => onsearch?.()}
          title="Search (Ctrl+K)"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
        </button>
        <button
          class="p-2 text-text-muted hover:text-text-primary rounded-md hover:bg-bg-hover"
          onclick={() => onlogs?.()}
          title="Logs"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 10h16M4 14h16M4 18h12" />
          </svg>
        </button>
        {#if !isMobile}
          {#if splitEnabled}
            <div class="flex items-center gap-0.5">
              <button class="p-1.5 rounded-lg transition-colors {splitOrientation === 'horizontal' ? 'text-[var(--accent-primary)] bg-[var(--bg-hover)]' : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'}" onclick={() => onsplithorizontal?.()} title="Horizontal split">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path d={splitHPath} /></svg>
              </button>
              <button class="p-1.5 rounded-lg transition-colors {splitOrientation === 'vertical' ? 'text-[var(--accent-primary)] bg-[var(--bg-hover)]' : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'}" onclick={() => onsplitvertical?.()} title="Vertical split">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path d={splitVPath} /></svg>
              </button>
              <div class="flex items-center">
                <button class="p-0.5 transition-colors {splitActivePanel === 0 ? 'text-[var(--accent-primary)]' : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'}" onclick={() => onsplitpanel?.(0)} title="Target panel 1"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d={panelArrow0} /></svg></button>
                                <button class="p-0.5 transition-colors {splitActivePanel === 1 ? 'text-[var(--accent-primary)]' : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'}" onclick={() => onsplitpanel?.(1)} title="Target panel 2"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d={panelArrow1} /></svg></button>
              </div>
              <button class="p-1.5 rounded-lg transition-colors text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]" onclick={() => onsplitclose?.()} title="Close split">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M18 6L6 18M6 6l12 12" /></svg>
              </button>
            </div>
          {:else}
            <button class="p-2 rounded-lg transition-colors text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]" onclick={() => onsplithorizontal?.()} title="Split view">
              <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path d={splitHPath} /></svg>
            </button>
          {/if}
        {/if}
        {#if $isAdmin}
          <button
            class="p-2 text-text-muted hover:text-text-primary rounded-md hover:bg-bg-hover"
            onclick={() => onsettings?.()}
            title="Settings"
          >
            <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.11 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
            </svg>
          </button>
        {/if}
        {#if hasRealAuth && $isAuthenticated && $currentUser}
          <button
            class="p-2 text-text-muted hover:text-red-400 rounded-md hover:bg-bg-hover transition-colors"
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
{:else if effectivePosition === 'left'}
  {@const labelsCollapsed = !config.navigation.show_labels && !isMobile}
  {@const isCollapsed = labelsCollapsed || (isHidden && config.navigation.auto_hide && !isMobile)}
  <aside
    class="flex-shrink-0 h-full relative z-10
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
      <div class="border-b border-border flex items-center justify-center overflow-hidden"
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
      <div class="border-b border-border flex items-center justify-center overflow-hidden"
           style="height: {isCollapsed ? `${collapsedStripWidth}px` : '52px'};">
        <button
          class="p-2 rounded-md hover:bg-bg-hover transition-all"
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
        class="w-full flex items-center py-1.5 text-text-muted hover:text-text-primary rounded-md hover:bg-bg-hover text-sm"
        onclick={() => onsearch?.()}
        title="Search (Ctrl+K)"
      >
        <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
        </div>
        <span style="opacity: {isCollapsed ? '0' : '1'}; transition: opacity 0.15s ease; white-space: nowrap;">Search...</span>
        <span class="ml-auto mr-2 text-xs bg-bg-elevated px-1.5 py-0.5 rounded" style="opacity: {isCollapsed ? '0' : '1'}; transition: opacity 0.15s ease; white-space: nowrap;">⌘K</span>
      </button>
    </div>

    <!-- App list (scrollable) -->
    <div class="flex-1 overflow-y-auto scrollbar-hide"
         style="padding: 0.5rem {isCollapsed ? '0' : '0.5rem'}; transition: padding 0.3s ease;">
      {#each groupNames as groupName (groupName)}
        {@const groupConfig = getGroupConfig(groupName)}
        <div class="mb-2">
          <!-- Group header — icon stays visible when collapsed, text fades -->
          <button
            class="w-full flex items-center py-1.5 text-xs font-semibold text-text-muted uppercase tracking-wider hover:text-text-secondary"
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
              <span class="ml-auto text-text-disabled flex-shrink-0">{groupedApps[groupName]?.length || 0}</span>
            </div>
          </button>

          <!-- Apps in group -->
          <div class="group-apps-wrapper" class:expanded={expandedGroups[groupName]}>
            <div class="group-apps-inner mt-1 space-y-0.5" style="padding-left: {isCollapsed ? '0' : '0.375rem'};">
              {#each groupedApps[groupName] || [] as app (app.name)}
                {@const shouldDim = (isCollapsed && currentApp?.name !== app.name) || (isUnhealthy(app) && currentApp?.name !== app.name)}
                <button
                  class="w-full flex items-center py-1.5 rounded-md text-sm transition-colors relative
                         {currentApp?.name === app.name
                           ? 'bg-bg-elevated text-text-primary'
                           : 'text-text-secondary hover:bg-bg-hover hover:text-text-primary'}"
                  tabindex={expandedGroups[groupName] ? 0 : -1}
                  onclick={() => { onselect?.(app); mobileMenuOpen = false; }}
                >
                  {#if config.navigation.show_app_colors && currentApp?.name === app.name}
                    <div class="absolute left-0 top-1 bottom-1 w-[3px] rounded-full" style="background: {app.color || '#22c55e'};"></div>
                  {/if}
                  <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px; opacity: {shouldDim ? 0.5 : 1}; transition: opacity 0.15s ease;">
                    <AppIcon icon={app.icon} name={app.name} color={app.color} size="sm" scale={iconScale} showBackground={config.navigation.show_icon_background} forceBackground={app.force_icon_background} />
                  </div>
                  {#if config.navigation.show_labels}
                    <span class="truncate" style="opacity: {isCollapsed ? '0' : shouldDim ? '0.5' : '1'}; transition: opacity 0.15s ease;">{app.name}</span>
                  {/if}
                  {#if shouldShowHealth(app)}
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

    <!-- Footer: drawer mode / collapsed cogwheel / standard -->
    {#if useFooterDrawer && !isCollapsed}
      <div class="sidebar-footer-drawer"
           style="padding: 0 0.5rem 0.25rem;"
           role="group"
           onmouseenter={handleFooterEnter}
           onmouseleave={handleFooterLeave}>
        <div class="footer-drawer-indicator">
          <svg class="w-3 h-3 transition-transform duration-200"
               class:rotate-180={footerDrawerOpen}
               fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round"
                  stroke-width="2" d="M5 15l7-7 7 7" />
          </svg>
        </div>
        <div class="footer-drawer-content" class:expanded={footerDrawerOpen}>
          <div class="footer-drawer-inner">
            <button
              class="w-full flex items-center py-1.5 text-text-muted hover:text-text-primary rounded-md hover:bg-bg-hover text-sm"
              onclick={() => { onlogs?.(); mobileMenuOpen = false; }}
              title="Logs"
            >
              <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 10h16M4 14h16M4 18h12" />
                </svg>
              </div>
              <span style="white-space: nowrap;">Logs</span>
            </button>

            {#if !isMobile}
              {#if splitEnabled}
                <div class="w-full flex items-center py-1.5 gap-1">
                  <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;"></div>
                  <button class="p-1 rounded transition-colors {splitOrientation === 'horizontal' ? 'text-[var(--accent-primary)] bg-[var(--bg-hover)]' : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'}" onclick={() => onsplithorizontal?.()} title="Horizontal split">
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path d={splitHPath} /></svg>
                  </button>
                  <button class="p-1 rounded transition-colors {splitOrientation === 'vertical' ? 'text-[var(--accent-primary)] bg-[var(--bg-hover)]' : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'}" onclick={() => onsplitvertical?.()} title="Vertical split">
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path d={splitVPath} /></svg>
                  </button>
                  <div class="flex items-center">
                    <button class="p-0.5 transition-colors {splitActivePanel === 0 ? 'text-[var(--accent-primary)]' : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'}" onclick={() => onsplitpanel?.(0)} title="Target panel 1"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d={panelArrow0} /></svg></button>
                                        <button class="p-0.5 transition-colors {splitActivePanel === 1 ? 'text-[var(--accent-primary)]' : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'}" onclick={() => onsplitpanel?.(1)} title="Target panel 2"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d={panelArrow1} /></svg></button>
                  </div>
                  <button class="p-1 rounded transition-colors text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]" onclick={() => onsplitclose?.()} title="Close split">
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M18 6L6 18M6 6l12 12" /></svg>
                  </button>
                </div>
              {:else}
                <button class="w-full flex items-center py-1.5 rounded-lg transition-colors text-sm text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]" onclick={() => onsplithorizontal?.()} title="Split view">
                  <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path d={splitHPath} /></svg>
                  </div>
                  <span style="white-space: nowrap;">Split view</span>
                </button>
              {/if}
            {/if}

            {#if hasRealAuth && $isAuthenticated && $currentUser}
              <button
                class="w-full flex items-center py-1.5 text-text-muted hover:text-red-400 rounded-md hover:bg-bg-hover text-sm transition-colors"
                onclick={handleLogout}
                title="Sign out"
              >
                <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
                  <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
                  </svg>
                </div>
                <span style="white-space: nowrap;">Sign out</span>
              </button>
            {/if}

            {#if $isAdmin}
              <button
                class="w-full flex items-center py-1.5 text-text-muted hover:text-text-primary rounded-md hover:bg-bg-hover text-sm"
                onclick={() => onsettings?.()}
                title="Settings"
              >
                <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
                  <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                  </svg>
                </div>
                <span style="white-space: nowrap;">Settings</span>
              </button>
            {/if}
          </div>
        </div>
      </div>
    {:else if useFooterDrawer && isCollapsed}
      <!-- Collapsed cogwheel — hover expands sidebar + opens drawer -->
      <div class="flex-shrink-0 flex items-center justify-center border-t py-2"
           style="border-color: var(--border-subtle);"
           role="group"
           onmouseenter={handleCollapsedFooterEnter}>
        <svg class="w-4 h-4 text-text-muted" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
      </div>
    {:else}
      <!-- Standard footer -->
      <div class="flex-shrink-0 pt-2 pb-1 border-t" style="border-color: var(--border-subtle); padding-left: {isCollapsed ? '0' : '0.5rem'}; padding-right: {isCollapsed ? '0' : '0.5rem'};">
        <button
          class="w-full flex items-center py-1.5 text-text-muted hover:text-text-primary rounded-md hover:bg-bg-hover text-sm"
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

        {#if !isMobile}
          {#if splitEnabled}
            <div class="w-full flex items-center py-1.5 gap-1" style="opacity: {isCollapsed ? '0' : '1'}; transition: opacity 0.15s ease; pointer-events: {isCollapsed ? 'none' : 'auto'};">
              <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;"></div>
              <button class="p-1 rounded transition-colors {splitOrientation === 'horizontal' ? 'text-[var(--accent-primary)] bg-[var(--bg-hover)]' : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'}" onclick={() => onsplithorizontal?.()} title="Horizontal split">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path d={splitHPath} /></svg>
              </button>
              <button class="p-1 rounded transition-colors {splitOrientation === 'vertical' ? 'text-[var(--accent-primary)] bg-[var(--bg-hover)]' : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'}" onclick={() => onsplitvertical?.()} title="Vertical split">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path d={splitVPath} /></svg>
              </button>
              <div class="flex items-center">
                <button class="p-0.5 transition-colors {splitActivePanel === 0 ? 'text-[var(--accent-primary)]' : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'}" onclick={() => onsplitpanel?.(0)} title="Target panel 1"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d={panelArrow0} /></svg></button>
                                <button class="p-0.5 transition-colors {splitActivePanel === 1 ? 'text-[var(--accent-primary)]' : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'}" onclick={() => onsplitpanel?.(1)} title="Target panel 2"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d={panelArrow1} /></svg></button>
              </div>
              <button class="p-1 rounded transition-colors text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]" onclick={() => onsplitclose?.()} title="Close split">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M18 6L6 18M6 6l12 12" /></svg>
              </button>
            </div>
          {:else}
            <button class="w-full flex items-center py-1.5 rounded-lg transition-colors text-sm text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]" style="opacity: {isCollapsed ? '0' : '1'}; transition: opacity 0.15s ease; pointer-events: {isCollapsed ? 'none' : 'auto'};" tabindex={isCollapsed ? -1 : 0} onclick={() => onsplithorizontal?.()} title="Split view">
              <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path d={splitHPath} /></svg>
              </div>
              <span style="opacity: {isCollapsed ? '0' : '1'}; transition: opacity 0.15s ease; white-space: nowrap;">Split view</span>
            </button>
          {/if}
        {/if}

        {#if hasRealAuth && $isAuthenticated && $currentUser}
          <button
            class="w-full flex items-center py-1.5 text-text-muted hover:text-red-400 rounded-md hover:bg-bg-hover text-sm transition-colors"
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

        {#if $isAdmin}
          <button
            class="w-full flex items-center py-1.5 text-text-muted hover:text-text-primary rounded-md hover:bg-bg-hover text-sm"
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
        {/if}
      </div>
    {/if}
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
{:else if effectivePosition === 'right'}
  {@const labelsCollapsedRight = !config.navigation.show_labels && !isMobile}
  {@const isCollapsedRight = labelsCollapsedRight || (isHidden && config.navigation.auto_hide && !isMobile)}
  <aside
    class="flex-shrink-0 h-full relative z-10
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
      <div class="border-b border-border flex items-center justify-center overflow-hidden"
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
      <div class="border-b border-border flex items-center justify-center overflow-hidden"
           style="height: {isCollapsedRight ? `${collapsedStripWidth}px` : '52px'};">
        <button
          class="p-2 rounded-md hover:bg-bg-hover transition-all"
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
        class="w-full flex items-center py-1.5 text-text-muted hover:text-text-primary rounded-md hover:bg-bg-hover text-sm"
        onclick={() => onsearch?.()}
        title="Search (Ctrl+K)"
      >
        <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
        </div>
        <span style="opacity: {isCollapsedRight ? '0' : '1'}; transition: opacity 0.15s ease; white-space: nowrap;">Search...</span>
        <span class="ml-auto mr-2 text-xs bg-bg-elevated px-1.5 py-0.5 rounded" style="opacity: {isCollapsedRight ? '0' : '1'}; transition: opacity 0.15s ease; white-space: nowrap;">⌘K</span>
      </button>
    </div>

    <!-- App list (scrollable) -->
    <div class="flex-1 overflow-y-auto scrollbar-hide"
         style="padding: 0.5rem {isCollapsedRight ? '0' : '0.5rem'}; transition: padding 0.3s ease;">
      {#each groupNames as groupName (groupName)}
        {@const groupConfig = getGroupConfig(groupName)}
        <div class="mb-2">
          <!-- Group header — icon stays visible when collapsed, text fades -->
          <button
            class="w-full flex items-center py-1.5 text-xs font-semibold text-text-muted uppercase tracking-wider hover:text-text-secondary"
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
              <span class="ml-auto text-text-disabled flex-shrink-0">{groupedApps[groupName]?.length || 0}</span>
            </div>
          </button>

          <div class="group-apps-wrapper" class:expanded={expandedGroups[groupName]}>
            <div class="group-apps-inner mt-1 space-y-0.5" style="padding-left: {isCollapsedRight ? '0' : '0.375rem'};">
              {#each groupedApps[groupName] || [] as app (app.name)}
                {@const shouldDim = (isCollapsedRight && currentApp?.name !== app.name) || (isUnhealthy(app) && currentApp?.name !== app.name)}
                <button
                  class="w-full flex items-center py-1.5 rounded-md text-sm transition-colors relative
                         {currentApp?.name === app.name
                           ? 'bg-bg-elevated text-text-primary'
                           : 'text-text-secondary hover:bg-bg-hover hover:text-text-primary'}"
                  tabindex={expandedGroups[groupName] ? 0 : -1}
                  onclick={() => { onselect?.(app); mobileMenuOpen = false; }}
                >
                  {#if config.navigation.show_app_colors && currentApp?.name === app.name}
                    <div class="absolute right-0 top-1 bottom-1 w-[3px] rounded-full" style="background: {app.color || '#22c55e'};"></div>
                  {/if}
                  <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px; opacity: {shouldDim ? 0.5 : 1}; transition: opacity 0.15s ease;">
                    <AppIcon icon={app.icon} name={app.name} color={app.color} size="sm" scale={iconScale} showBackground={config.navigation.show_icon_background} forceBackground={app.force_icon_background} />
                  </div>
                  {#if config.navigation.show_labels}
                    <span class="truncate" style="opacity: {isCollapsedRight ? '0' : shouldDim ? '0.5' : '1'}; transition: opacity 0.15s ease;">{app.name}</span>
                  {/if}
                  {#if shouldShowHealth(app)}
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

    <!-- Footer: drawer mode / collapsed cogwheel / standard -->
    {#if useFooterDrawer && !isCollapsedRight}
      <div class="sidebar-footer-drawer"
           style="padding: 0 0.5rem 0.25rem;"
           role="group"
           onmouseenter={handleFooterEnter}
           onmouseleave={handleFooterLeave}>
        <div class="footer-drawer-indicator">
          <svg class="w-3 h-3 transition-transform duration-200"
               class:rotate-180={footerDrawerOpen}
               fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round"
                  stroke-width="2" d="M5 15l7-7 7 7" />
          </svg>
        </div>
        <div class="footer-drawer-content" class:expanded={footerDrawerOpen}>
          <div class="footer-drawer-inner">
            <button
              class="w-full flex items-center py-1.5 text-text-muted hover:text-text-primary rounded-md hover:bg-bg-hover text-sm"
              onclick={() => { onlogs?.(); mobileMenuOpen = false; }}
              title="Logs"
            >
              <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 10h16M4 14h16M4 18h12" />
                </svg>
              </div>
              <span style="white-space: nowrap;">Logs</span>
            </button>

            {#if !isMobile}
              {#if splitEnabled}
                <div class="w-full flex items-center py-1.5 gap-1">
                  <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;"></div>
                  <button class="p-1 rounded transition-colors {splitOrientation === 'horizontal' ? 'text-[var(--accent-primary)] bg-[var(--bg-hover)]' : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'}" onclick={() => onsplithorizontal?.()} title="Horizontal split">
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path d={splitHPath} /></svg>
                  </button>
                  <button class="p-1 rounded transition-colors {splitOrientation === 'vertical' ? 'text-[var(--accent-primary)] bg-[var(--bg-hover)]' : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'}" onclick={() => onsplitvertical?.()} title="Vertical split">
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path d={splitVPath} /></svg>
                  </button>
                  <div class="flex items-center">
                    <button class="p-0.5 transition-colors {splitActivePanel === 0 ? 'text-[var(--accent-primary)]' : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'}" onclick={() => onsplitpanel?.(0)} title="Target panel 1"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d={panelArrow0} /></svg></button>
                                        <button class="p-0.5 transition-colors {splitActivePanel === 1 ? 'text-[var(--accent-primary)]' : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'}" onclick={() => onsplitpanel?.(1)} title="Target panel 2"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d={panelArrow1} /></svg></button>
                  </div>
                  <button class="p-1 rounded transition-colors text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]" onclick={() => onsplitclose?.()} title="Close split">
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M18 6L6 18M6 6l12 12" /></svg>
                  </button>
                </div>
              {:else}
                <button class="w-full flex items-center py-1.5 rounded-lg transition-colors text-sm text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]" onclick={() => onsplithorizontal?.()} title="Split view">
                  <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
                    <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path d={splitHPath} /></svg>
                  </div>
                  <span style="white-space: nowrap;">Split view</span>
                </button>
              {/if}
            {/if}

            {#if hasRealAuth && $isAuthenticated && $currentUser}
              <button
                class="w-full flex items-center py-1.5 text-text-muted hover:text-red-400 rounded-md hover:bg-bg-hover text-sm transition-colors"
                onclick={handleLogout}
                title="Sign out"
              >
                <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
                  <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
                  </svg>
                </div>
                <span style="white-space: nowrap;">Sign out</span>
              </button>
            {/if}

            {#if $isAdmin}
              <button
                class="w-full flex items-center py-1.5 text-text-muted hover:text-text-primary rounded-md hover:bg-bg-hover text-sm"
                onclick={() => onsettings?.()}
                title="Settings"
              >
                <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
                  <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                  </svg>
                </div>
                <span style="white-space: nowrap;">Settings</span>
              </button>
            {/if}
          </div>
        </div>
      </div>
    {:else if useFooterDrawer && isCollapsedRight}
      <!-- Collapsed cogwheel — hover expands sidebar + opens drawer -->
      <div class="flex-shrink-0 flex items-center justify-center border-t py-2"
           style="border-color: var(--border-subtle);"
           role="group"
           onmouseenter={handleCollapsedFooterEnter}>
        <svg class="w-4 h-4 text-text-muted" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
      </div>
    {:else}
      <!-- Standard footer -->
      <div class="flex-shrink-0 pt-2 pb-1 border-t" style="border-color: var(--border-subtle); padding-left: {isCollapsedRight ? '0' : '0.5rem'}; padding-right: {isCollapsedRight ? '0' : '0.5rem'};">
        <button
          class="w-full flex items-center py-1.5 text-text-muted hover:text-text-primary rounded-md hover:bg-bg-hover text-sm"
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

        {#if !isMobile}
          {#if splitEnabled}
            <div class="w-full flex items-center py-1.5 gap-1" style="opacity: {isCollapsedRight ? '0' : '1'}; transition: opacity 0.15s ease; pointer-events: {isCollapsedRight ? 'none' : 'auto'};">
              <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;"></div>
              <button class="p-1 rounded transition-colors {splitOrientation === 'horizontal' ? 'text-[var(--accent-primary)] bg-[var(--bg-hover)]' : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'}" onclick={() => onsplithorizontal?.()} title="Horizontal split">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path d={splitHPath} /></svg>
              </button>
              <button class="p-1 rounded transition-colors {splitOrientation === 'vertical' ? 'text-[var(--accent-primary)] bg-[var(--bg-hover)]' : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'}" onclick={() => onsplitvertical?.()} title="Vertical split">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path d={splitVPath} /></svg>
              </button>
              <div class="flex items-center">
                <button class="p-0.5 transition-colors {splitActivePanel === 0 ? 'text-[var(--accent-primary)]' : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'}" onclick={() => onsplitpanel?.(0)} title="Target panel 1"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d={panelArrow0} /></svg></button>
                                <button class="p-0.5 transition-colors {splitActivePanel === 1 ? 'text-[var(--accent-primary)]' : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'}" onclick={() => onsplitpanel?.(1)} title="Target panel 2"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d={panelArrow1} /></svg></button>
              </div>
              <button class="p-1 rounded transition-colors text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]" onclick={() => onsplitclose?.()} title="Close split">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M18 6L6 18M6 6l12 12" /></svg>
              </button>
            </div>
          {:else}
            <button class="w-full flex items-center py-1.5 rounded-lg transition-colors text-sm text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]" style="opacity: {isCollapsedRight ? '0' : '1'}; transition: opacity 0.15s ease; pointer-events: {isCollapsedRight ? 'none' : 'auto'};" tabindex={isCollapsedRight ? -1 : 0} onclick={() => onsplithorizontal?.()} title="Split view">
              <div class="flex-shrink-0 flex items-center justify-center" style="width: {collapsedStripWidth}px;">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path d={splitHPath} /></svg>
              </div>
              <span style="opacity: {isCollapsedRight ? '0' : '1'}; transition: opacity 0.15s ease; white-space: nowrap;">Split view</span>
            </button>
          {/if}
        {/if}

        {#if hasRealAuth && $isAuthenticated && $currentUser}
          <button
            class="w-full flex items-center py-1.5 text-text-muted hover:text-red-400 rounded-md hover:bg-bg-hover text-sm transition-colors"
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

        {#if $isAdmin}
          <button
            class="w-full flex items-center py-1.5 text-text-muted hover:text-text-primary rounded-md hover:bg-bg-hover text-sm"
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
        {/if}
      </div>
    {/if}
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
{:else if effectivePosition === 'bottom'}
  {@const isCollapsedBottom = isHidden && config.navigation.auto_hide}
  <nav
    class="relative z-10"
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
      style:overflow={!config.navigation.auto_hide || barOverflowVisible ? 'visible' : 'hidden'}
      style:box-shadow={config.navigation.auto_hide && config.navigation.show_shadow ? '0 -4px 24px rgba(0,0,0,0.25)' : null}
      style:position={config.navigation.auto_hide ? 'absolute' : null}
      style:bottom={config.navigation.auto_hide ? '0' : null}
      style:left={config.navigation.auto_hide ? '0' : null}
      style:right={config.navigation.auto_hide ? '0' : null}
      style:z-index={config.navigation.auto_hide ? '30' : null}
    >
    <div
      class="flex items-center gap-4 px-4"
      style="height: 56px;"
    >
      <!-- Logo — fixed -->
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
          class="flex-shrink-0 p-1 rounded-md hover:bg-bg-hover transition-all"
          style="color: var(--accent-primary); opacity: {showSplash ? '0.6' : '1'}; transition: opacity 0.2s ease;"
          onclick={() => onsplash?.()}
          title="Overview"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-4 0a1 1 0 01-1-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 01-1 1h-2z" />
          </svg>
        </button>
      {/if}

      <!-- App tabs — flex-1 so it fills between logo and actions -->
      {#if useGroupDropdowns}
        <!-- Grouped: dropdown buttons (upward), overflow visible for dropdowns -->
        <div class="flex-1 min-w-0 flex items-center space-x-1" style="overflow: visible;">
          {#each groupNames as groupName (groupName)}
            {@const groupConfig = getGroupConfig(groupName)}
            {@const groupApps = groupedApps[groupName] || []}
            {@const hasActiveApp = groupApps.some(a => currentApp?.name === a.name)}
            <!-- svelte-ignore a11y_no_static_element_interactions -->
            <div class="relative"
              onmouseenter={() => openDropdown(groupName)}
              onmouseleave={scheduleDropdownClose}
            >
              <button
                class="px-2.5 py-2 rounded-md text-sm font-medium transition-colors whitespace-nowrap flex items-center gap-1.5
                       {hasActiveApp
                         ? 'text-text-primary'
                         : openGroupDropdown === groupName ? 'bg-bg-hover text-text-primary' : 'text-text-muted hover:bg-bg-hover hover:text-text-primary'}"
                style={hasActiveApp && config.navigation.show_app_colors ? `border-top: 2px solid ${groupConfig?.color || currentApp?.color || 'var(--accent-primary)'}` : ''}
                onclick={() => openGroupDropdown = openGroupDropdown === groupName ? null : groupName}
              >
                {#if groupConfig?.icon?.name}
                  <AppIcon icon={groupConfig.icon} name={groupName} color={groupConfig.color || ''} size="sm" scale={iconScale} showBackground={false} />
                {/if}
                <span>{groupName}</span>
                <svg class="w-3 h-3 opacity-50 rotate-180" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                </svg>
              </button>

              {#if openGroupDropdown === groupName}
                <!-- svelte-ignore a11y_no_static_element_interactions -->
                <div
                  class="absolute bottom-full left-0 mb-1 min-w-[200px] max-w-[280px] rounded-lg border shadow-xl overflow-hidden z-50"
                  style="background: var(--bg-surface); border-color: var(--border-subtle);"
                  onmouseenter={cancelDropdownClose}
                  onmouseleave={scheduleDropdownClose}
                >
                  <div class="py-1 max-h-[60vh] overflow-y-auto overflow-x-hidden scrollbar-styled">
                    {#each groupApps as app (app.name)}
                      <button
                        class="group-dropdown-item w-full flex items-center gap-2 px-3 py-2 text-sm transition-colors
                               {currentApp?.name === app.name
                                 ? 'text-text-primary'
                                 : 'text-text-secondary hover:text-text-primary'}
                               {isUnhealthy(app) && currentApp?.name !== app.name ? 'opacity-50' : ''}"
                        style="background: {currentApp?.name === app.name ? 'var(--bg-hover)' : 'transparent'};"
                        onclick={() => { onselect?.(app); openGroupDropdown = null; }}
                      >
                        {#if config.navigation.show_app_colors && currentApp?.name === app.name}
                          <div class="absolute left-0 top-1 bottom-1 w-[3px] rounded-full" style="background: {app.color || '#22c55e'};"></div>
                        {/if}
                        <AppIcon icon={app.icon} name={app.name} color={app.color} size="sm" scale={iconScale} showBackground={config.navigation.show_icon_background} forceBackground={app.force_icon_background} />
                        <span class="truncate">{app.name}</span>
                        {#if shouldShowHealth(app)}
                          <span class="ml-auto flex-shrink-0"><HealthIndicator appName={app.name} size="sm" /></span>
                        {/if}
                        {#if app.open_mode !== 'iframe'}
                          <span class="text-xs opacity-60 flex-shrink-0">{getOpenModeIcon(app.open_mode)}</span>
                        {/if}
                      </button>
                    {/each}
                  </div>
                </div>
              {/if}
            </div>
          {/each}
        </div>
      {:else}
        <!-- Flat: scrollable app list with group icon dividers -->
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div class="flat-bar-scroll flex-1 min-w-0 flex items-center" onwheel={handleBarWheel} onmouseleave={() => hoveredGroup = null}>
          {#each groupNames as groupName, gi (groupName)}
            {@const groupConfig = getGroupConfig(groupName)}
            {#if hasRealGroups}
              <!-- svelte-ignore a11y_no_static_element_interactions -->
              <div
                class="flat-group-divider relative flex-shrink-0 flex items-center px-1.5 py-2 rounded-md transition-colors cursor-default
                       {hoveredGroup === groupName ? 'bg-bg-elevated/60' : ''}"
                style="{gi > 0 ? 'margin-left: 2px;' : ''}"
                onmouseenter={() => hoveredGroup = groupName}
                title={groupName}
              >
                {#if groupConfig?.icon?.name}
                  <span style="opacity: {hoveredGroup === groupName ? '1' : '0.4'}; transition: opacity 0.15s ease;">
                    <AppIcon icon={groupConfig.icon} name={groupName} color={groupConfig.color || ''} size="sm" scale={iconScale * 0.85} showBackground={false} />
                  </span>
                {:else}
                  <div class="w-px h-5" style="background: var(--border-subtle); opacity: {hoveredGroup === groupName ? '1' : '0.5'};"></div>
                {/if}
                {#if hoveredGroup === groupName}
                  <span
                    use:fixedTooltip={true}
                    class="fixed -translate-x-1/2 whitespace-nowrap text-[10px] font-semibold uppercase tracking-wider pointer-events-none z-[100] px-1.5 py-0.5 rounded bg-bg-elevated/90 text-text-muted shadow-sm"
                  >{groupName}</span>
                {/if}
              </div>
            {/if}
            {#each groupedApps[groupName] || [] as app (app.name)}
              <button
                class="relative group flex-shrink-0 px-2 py-2 rounded-md text-sm font-medium transition-all whitespace-nowrap flex items-center gap-1
                       {currentApp?.name === app.name
                         ? 'bg-bg-base text-text-primary'
                         : 'text-text-secondary hover:bg-bg-hover hover:text-text-primary'}
                       {isUnhealthy(app) && currentApp?.name !== app.name ? 'opacity-50' : ''}"
                style="border-top: 2px solid {config.navigation.show_app_colors && currentApp?.name === app.name ? (app.color || '#22c55e') : 'transparent'};
                       {hoveredGroup && hoveredGroup !== app.group ? 'opacity: 0.3;' : ''}"
                onclick={() => onselect?.(app)}
                onmouseenter={() => hoveredGroup = null}
              >
                <AppIcon icon={app.icon} name={app.name} color={app.color} size="sm" scale={iconScale} showBackground={config.navigation.show_icon_background} forceBackground={app.force_icon_background} />
                {#if config.navigation.show_labels}
                  <span>{app.name}</span>
                {:else}
                  <span class="inline-block max-w-0 overflow-hidden opacity-0 group-hover:max-w-[120px] group-hover:opacity-100 transition-all duration-200 whitespace-nowrap">{app.name}</span>
                {/if}
                {#if shouldShowHealth(app)}
                  <HealthIndicator appName={app.name} size="sm" />
                {/if}
                {#if app.open_mode !== 'iframe'}
                  <span class="text-xs opacity-60">{getOpenModeIcon(app.open_mode)}</span>
                {/if}
              </button>
            {/each}
          {/each}
        </div>
      {/if}

      <!-- Right side actions -->
      <div class="flex items-center space-x-2">
        <button
          class="p-2 text-text-muted hover:text-text-primary rounded-md hover:bg-bg-hover"
          onclick={() => onsearch?.()}
          title="Search (Ctrl+K)"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
        </button>
        <button
          class="p-2 text-text-muted hover:text-text-primary rounded-md hover:bg-bg-hover"
          onclick={() => onlogs?.()}
          title="Logs"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 10h16M4 14h16M4 18h12" />
          </svg>
        </button>
        {#if !isMobile}
          {#if splitEnabled}
            <div class="flex items-center gap-0.5">
              <button class="p-1.5 rounded-lg transition-colors {splitOrientation === 'horizontal' ? 'text-[var(--accent-primary)] bg-[var(--bg-hover)]' : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'}" onclick={() => onsplithorizontal?.()} title="Horizontal split">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path d={splitHPath} /></svg>
              </button>
              <button class="p-1.5 rounded-lg transition-colors {splitOrientation === 'vertical' ? 'text-[var(--accent-primary)] bg-[var(--bg-hover)]' : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'}" onclick={() => onsplitvertical?.()} title="Vertical split">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path d={splitVPath} /></svg>
              </button>
              <div class="flex items-center">
                <button class="p-0.5 transition-colors {splitActivePanel === 0 ? 'text-[var(--accent-primary)]' : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'}" onclick={() => onsplitpanel?.(0)} title="Target panel 1"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d={panelArrow0} /></svg></button>
                                <button class="p-0.5 transition-colors {splitActivePanel === 1 ? 'text-[var(--accent-primary)]' : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'}" onclick={() => onsplitpanel?.(1)} title="Target panel 2"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d={panelArrow1} /></svg></button>
              </div>
              <button class="p-1.5 rounded-lg transition-colors text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]" onclick={() => onsplitclose?.()} title="Close split">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M18 6L6 18M6 6l12 12" /></svg>
              </button>
            </div>
          {:else}
            <button class="p-2 rounded-lg transition-colors text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]" onclick={() => onsplithorizontal?.()} title="Split view">
              <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path d={splitHPath} /></svg>
            </button>
          {/if}
        {/if}
        {#if $isAdmin}
          <button
            class="p-2 text-text-muted hover:text-text-primary rounded-md hover:bg-bg-hover"
            onclick={() => onsettings?.()}
            title="Settings"
          >
            <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.11 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
            </svg>
          </button>
        {/if}
        {#if hasRealAuth && $isAuthenticated && $currentUser}
          <button
            class="p-2 text-text-muted hover:text-red-400 rounded-md hover:bg-bg-hover transition-colors"
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

{:else if effectivePosition === 'floating'}
  {@const isCollapsedFloat = isHidden && config.navigation.auto_hide}

  <!-- Click-outside overlay to close panel -->
  {#if panelOpen}
    <button
      class="fixed inset-0 z-30"
      onclick={() => panelOpen = false}
      aria-label="Close navigation"
    ></button>
  {/if}

  <!-- Popover panel — positioned independently from FAB -->
  {#if panelOpen}
    <div
      class="floating-panel fixed z-40 flex flex-col border rounded-2xl shadow-2xl overflow-hidden"
      style="
        left: {panelPos.left}px;
        {panelPos.useBottom ? `bottom: ${panelPos.bottom}px;` : `top: ${panelPos.top}px;`}
        width: {panelPos.width}px;
        max-height: {panelPos.maxHeight}px;
        background: var(--bg-surface);
        border-color: var(--border-subtle);
        --float-slide-x: {panelPos.slideX};
      "
      onmouseenter={handleNavEnter}
      onmouseleave={handleNavLeave}
      role="navigation"
    >
        <!-- Scrollable app list with groups -->
        <div class="flex-1 overflow-y-auto scrollbar-hide flex flex-col" style="padding: 0.5rem;">
          {#each groupNames as groupName (groupName)}
            {@const groupConfig = getGroupConfig(groupName)}
            <div class="mb-1">
              <!-- Group header -->
              <button
                class="w-full flex items-center py-1 px-1 text-xs font-semibold text-text-muted uppercase tracking-wider hover:text-text-secondary rounded"
                onclick={() => toggleGroup(groupName)}
              >
                <div class="flex-shrink-0 flex items-center justify-center w-6">
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
                <div class="flex items-center flex-1 min-w-0 ml-1">
                  {#if groupConfig?.icon?.name || groupConfig?.color}
                    <svg
                      class="w-3 h-3 transition-transform {expandedGroups[groupName] ? 'rotate-90' : ''} mr-1 flex-shrink-0"
                      fill="none" viewBox="0 0 24 24" stroke="currentColor"
                    >
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                    </svg>
                  {/if}
                  <span class="truncate">{groupName}</span>
                  <span class="ml-auto text-text-disabled flex-shrink-0">{groupedApps[groupName]?.length || 0}</span>
                </div>
              </button>

              <!-- Apps in group -->
              <div class="group-apps-wrapper" class:expanded={expandedGroups[groupName]}>
                <div class="group-apps-inner mt-0.5 space-y-0.5">
                  {#each groupedApps[groupName] || [] as app (app.name)}
                    {@const shouldDim = isUnhealthy(app) && currentApp?.name !== app.name}
                    <button
                      class="w-full flex items-center py-1.5 px-1 rounded-md text-sm transition-colors relative
                             {currentApp?.name === app.name
                               ? 'bg-bg-elevated text-text-primary'
                               : 'text-text-secondary hover:bg-bg-hover hover:text-text-primary'}"
                      tabindex={expandedGroups[groupName] ? 0 : -1}
                      onclick={() => { onselect?.(app); panelOpen = false; }}
                    >
                      {#if config.navigation.show_app_colors && currentApp?.name === app.name}
                        <div class="absolute left-0 top-1 bottom-1 w-[3px] rounded-full" style="background: {app.color || '#22c55e'};"></div>
                      {/if}
                      <div class="flex-shrink-0 flex items-center justify-center w-6 ml-1" style="opacity: {shouldDim ? 0.5 : 1}; transition: opacity 0.15s ease;">
                        <AppIcon icon={app.icon} name={app.name} color={app.color} size="sm" scale={iconScale} showBackground={config.navigation.show_icon_background} forceBackground={app.force_icon_background} />
                      </div>
                      <span class="truncate ml-2" style="opacity: {shouldDim ? 0.5 : 1}; transition: opacity 0.15s ease;">{app.name}</span>
                      {#if shouldShowHealth(app)}
                        <span class="ml-auto pr-1">
                          <HealthIndicator appName={app.name} size="sm" />
                        </span>
                      {/if}
                      {#if app.open_mode !== 'iframe'}
                        <span class="text-xs opacity-60 pr-1">{getOpenModeIcon(app.open_mode)}</span>
                      {/if}
                    </button>
                  {/each}
                </div>
              </div>
            </div>
          {/each}
        </div>

        <!-- Footer — all action buttons in one row -->
        <div class="border-t px-2 py-2 flex items-center gap-1" style="border-color: var(--border-subtle);">
          {#if config.navigation.show_logo}
            <button
              class="p-1.5 hover:opacity-80 flex items-center rounded-md hover:bg-bg-hover"
              style="color: var(--accent-primary); opacity: {showSplash ? '0.6' : '1'}; transition: opacity 0.2s ease;"
              onclick={() => { onsplash?.(); panelOpen = false; }}
              title={config.title}
            >
              <MuximuxLogo height="18" />
            </button>
          {:else}
            <button
              class="p-1.5 rounded-md hover:bg-bg-hover transition-all"
              style="color: var(--accent-primary); opacity: {showSplash ? '0.6' : '1'}; transition: opacity 0.2s ease;"
              onclick={() => { onsplash?.(); panelOpen = false; }}
              title="Overview"
            >
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-4 0a1 1 0 01-1-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 01-1 1h-2z" />
              </svg>
            </button>
          {/if}
          <button
            class="p-1.5 text-text-muted hover:text-text-primary rounded-md hover:bg-bg-hover"
            onclick={() => { onsearch?.(); panelOpen = false; }}
            title="Search (Ctrl+K)"
          >
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
          </button>
          <div class="flex-1"></div>
          <button
            class="p-1.5 text-text-muted hover:text-text-primary rounded-md hover:bg-bg-hover"
            onclick={() => { onlogs?.(); panelOpen = false; }}
            title="Logs"
          >
            <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 10h16M4 14h16M4 18h12" />
            </svg>
          </button>
          {#if !isMobile}
            {#if splitEnabled}
              <div class="flex items-center gap-0.5">
                <button class="p-1 rounded-lg transition-colors {splitOrientation === 'horizontal' ? 'text-[var(--accent-primary)] bg-[var(--bg-hover)]' : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'}" onclick={() => onsplithorizontal?.()} title="Horizontal split">
                  <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path d={splitHPath} /></svg>
                </button>
                <button class="p-1 rounded-lg transition-colors {splitOrientation === 'vertical' ? 'text-[var(--accent-primary)] bg-[var(--bg-hover)]' : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'}" onclick={() => onsplitvertical?.()} title="Vertical split">
                  <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path d={splitVPath} /></svg>
                </button>
                <div class="flex items-center">
                  <button class="p-0.5 transition-colors {splitActivePanel === 0 ? 'text-[var(--accent-primary)]' : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'}" onclick={() => onsplitpanel?.(0)} title="Target panel 1"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d={panelArrow0} /></svg></button>
                                    <button class="p-0.5 transition-colors {splitActivePanel === 1 ? 'text-[var(--accent-primary)]' : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'}" onclick={() => onsplitpanel?.(1)} title="Target panel 2"><svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path stroke-linecap="round" stroke-linejoin="round" d={panelArrow1} /></svg></button>
                </div>
                <button class="p-1 rounded-lg transition-colors text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]" onclick={() => onsplitclose?.()} title="Close split">
                  <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M18 6L6 18M6 6l12 12" /></svg>
                </button>
              </div>
            {:else}
              <button class="p-1.5 rounded-lg transition-colors text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]" onclick={() => onsplithorizontal?.()} title="Split view">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path d={splitHPath} /></svg>
              </button>
            {/if}
          {/if}
          {#if $isAdmin}
            <button
              class="p-1.5 text-text-muted hover:text-text-primary rounded-md hover:bg-bg-hover"
              onclick={() => { onsettings?.(); panelOpen = false; }}
              title="Settings"
            >
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.11 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
              </svg>
            </button>
          {/if}
          {#if hasRealAuth && $isAuthenticated && $currentUser}
            <button
              class="p-1.5 text-text-muted hover:text-red-400 rounded-md hover:bg-bg-hover transition-colors"
              onclick={() => { handleLogout(); panelOpen = false; }}
              title="Sign out"
            >
              <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
              </svg>
            </button>
          {/if}
        </div>
      </div>
  {/if}

  <!-- FAB toggle button — fixed at (fabX, fabY), draggable -->
  <div
    class="fixed z-40"
    style="
      left: {fabX}px;
      top: {fabY}px;
      transform: translate(-50%, -50%);
      {fabInitialized ? '' : 'visibility: hidden;'}
    "
    onmouseenter={handleNavEnter}
    onmouseleave={handleNavLeave}
    role="navigation"
  >
    <button
      class="p-4 bg-brand-600 hover:bg-brand-700 text-white rounded-full shadow-lg transition-all"
      class:hover:scale-110={!isDraggingFab}
      style="
        opacity: {isCollapsedFloat && !panelOpen ? 0.5 : 1};
        transition: opacity 0.3s ease;
        cursor: {isDraggingFab ? 'grabbing' : 'grab'};
        touch-action: none;
      "
      onpointerdown={handleFabPointerDown}
      ondblclick={(e) => { e.preventDefault(); const c = floatingPositionToCoords(effectiveFloatingPosition); fabX = c.x; fabY = c.y; persistFabPosition(); }}
      title={panelOpen ? 'Close navigation' : config.title}
    >
      <svg class="w-6 h-6 transition-transform duration-200" style="transform: rotate({panelOpen ? '90deg' : '0deg'});" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        {#if panelOpen}
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        {:else}
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
        {/if}
      </svg>
    </button>
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
  nav :global(.text-text-primary),
  aside :global(.text-text-primary) {
    color: var(--text-primary) !important;
  }

  nav :global(.text-text-secondary),
  aside :global(.text-text-secondary) {
    color: var(--text-secondary) !important;
  }

  nav :global(.text-text-muted),
  aside :global(.text-text-muted) {
    color: var(--text-muted) !important;
  }

  nav :global(.text-text-disabled),
  aside :global(.text-text-disabled) {
    color: var(--text-disabled) !important;
  }

  /* Background colors */
  nav :global(.bg-bg-elevated),
  aside :global(.bg-bg-elevated) {
    background: var(--bg-hover) !important;
  }

  nav :global(.bg-bg-surface),
  aside :global(.bg-bg-surface) {
    background: var(--bg-surface) !important;
  }

  nav :global(.bg-bg-base),
  aside :global(.bg-bg-base) {
    background: var(--bg-base) !important;
  }

  /* Hover states */
  nav :global(.hover\:bg-bg-elevated:hover),
  aside :global(.hover\:bg-bg-elevated:hover) {
    background: var(--bg-hover) !important;
  }

  nav :global(.hover\:bg-bg-overlay\/50:hover),
  aside :global(.hover\:bg-bg-overlay\/50:hover) {
    background: var(--bg-active) !important;
  }

  nav :global(.hover\:text-text-primary:hover),
  aside :global(.hover\:text-text-primary:hover) {
    color: var(--text-primary) !important;
  }

  /* Border colors */
  nav :global(.border-border),
  aside :global(.border-border) {
    border-color: var(--border-subtle) !important;
  }

  /* Active/selected states */
  nav :global(.bg-bg-elevated\/50),
  aside :global(.bg-bg-elevated\/50) {
    background: var(--bg-hover) !important;
  }

  /* Floating buttons */
  :global(.bg-bg-surface.border.border-border) {
    background: var(--bg-surface) !important;
    border-color: var(--border-default) !important;
  }

  /* Keyboard shortcut badges */
  nav :global(.bg-bg-elevated.px-1\.5),
  aside :global(.bg-bg-elevated.px-1\.5) {
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

  /* Collapsible footer drawer */
  .sidebar-footer-drawer {
    flex-shrink: 0;
    border-top: 1px solid var(--border-subtle);
  }

  .footer-drawer-indicator {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 24px;
    cursor: pointer;
    color: var(--text-disabled);
    transition: background-color 0.15s ease;
  }
  .footer-drawer-indicator:hover {
    background-color: var(--bg-hover);
  }

  .footer-drawer-content {
    display: grid;
    grid-template-rows: 0fr;
    transition: grid-template-rows 0.25s ease;
  }
  .footer-drawer-content.expanded {
    grid-template-rows: 1fr;
  }
  .footer-drawer-inner {
    overflow: hidden;
    min-height: 0;
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

  /* Floating popover panel */
  .floating-panel {
    animation: floatingPanelIn 0.2s ease;
  }

  @keyframes floatingPanelIn {
    from {
      opacity: 0;
      transform: translateX(var(--float-slide-x, 0)) scale(0.97);
    }
    to {
      opacity: 1;
      transform: translateX(0) scale(1);
    }
  }

  /* Floating panel theme overrides */
  .floating-panel :global(.text-text-primary) {
    color: var(--text-primary) !important;
  }
  .floating-panel :global(.text-text-secondary) {
    color: var(--text-secondary) !important;
  }
  .floating-panel :global(.text-text-muted) {
    color: var(--text-muted) !important;
  }
  .floating-panel :global(.text-text-disabled) {
    color: var(--text-disabled) !important;
  }
  .floating-panel :global(.bg-bg-elevated) {
    background: var(--bg-hover) !important;
  }
  .floating-panel :global(.hover\:bg-bg-elevated:hover) {
    background: var(--bg-hover) !important;
  }
  .floating-panel :global(.hover\:bg-bg-elevated\/50:hover) {
    background: var(--bg-hover) !important;
  }
  .floating-panel :global(.bg-bg-elevated\/50) {
    background: var(--bg-hover) !important;
  }
  .floating-panel :global(.hover\:text-text-primary:hover) {
    color: var(--text-primary) !important;
  }
  .floating-panel :global(.border-border) {
    border-color: var(--border-subtle) !important;
  }

  /* Group dropdown item hover */
  .group-dropdown-item:hover {
    background: var(--bg-hover) !important;
  }

  /* Flat bar horizontal scroll */
  .flat-bar-scroll {
    overflow-x: auto;
    overflow-y: hidden;
    scrollbar-width: thin;
    scrollbar-color: var(--bg-active) transparent;
  }
  .flat-bar-scroll::-webkit-scrollbar {
    height: 3px;
  }
  .flat-bar-scroll::-webkit-scrollbar-track {
    background: transparent;
  }
  .flat-bar-scroll::-webkit-scrollbar-thumb {
    background: var(--bg-active);
    border-radius: 3px;
  }
  .flat-bar-scroll::-webkit-scrollbar-thumb:hover {
    background: var(--text-disabled);
  }
</style>
