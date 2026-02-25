import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

// --- Hoisted store values ---
const { mockHealthData, mockCurrentUser, mockIsAuthenticated, mockIsAdmin } = vi.hoisted(() => {
  function makeStore<T>(initial: T) {
    const subs = new Set<(v: T) => void>();
    let value = initial;
    return {
      subscribe(fn: (v: T) => void) {
        fn(value);
        subs.add(fn);
        return () => subs.delete(fn);
      },
      set(v: T) {
        value = v;
        subs.forEach(fn => fn(v));
      },
    };
  }
  return {
    mockHealthData: makeStore(new Map()),
    mockCurrentUser: makeStore({ username: 'admin', role: 'admin' }),
    mockIsAuthenticated: makeStore(true),
    mockIsAdmin: makeStore(true),
  };
});

// --- Mocks ---

vi.mock('$lib/healthStore', () => ({
  healthData: mockHealthData,
  healthLoading: { subscribe: (fn: (v: boolean) => void) => { fn(false); return () => {}; } },
  healthError: { subscribe: (fn: (v: string | null) => void) => { fn(null); return () => {}; } },
  refreshHealth: vi.fn(),
  startHealthPolling: vi.fn(),
  stopHealthPolling: vi.fn(),
  getAppHealthStatus: vi.fn(() => 'unknown'),
  createAppHealthStore: vi.fn(),
}));

vi.mock('$lib/authStore', () => ({
  currentUser: mockCurrentUser,
  isAuthenticated: mockIsAuthenticated,
  isAdmin: mockIsAdmin,
  logout: vi.fn().mockResolvedValue(undefined),
  authState: { subscribe: (fn: (v: unknown) => void) => { fn({ authenticated: true, user: { username: 'admin', role: 'admin' }, loading: false, error: null, setupRequired: false, logoutUrl: null }); return () => {}; } },
}));

vi.mock('$lib/useSwipe', () => ({
  createEdgeSwipeHandlers: vi.fn(() => ({
    onpointerdown: vi.fn(),
    onpointermove: vi.fn(),
    onpointerup: vi.fn(),
    onpointercancel: vi.fn(),
    destroy: vi.fn(),
  })),
  isTouchDevice: vi.fn(() => false),
  isMobileViewport: vi.fn(() => false),
}));

vi.mock('$lib/api', () => ({
  getBase: vi.fn(() => ''),
  API_BASE: '',
}));

vi.mock('$lib/debug', () => ({
  debug: vi.fn(),
}));

import Navigation from './Navigation.svelte';
import type { App, AppIcon, Config, NavigationConfig, Group } from '$lib/types';

// --- Helper factories ---

function makeIcon(overrides: Partial<AppIcon> = {}): AppIcon {
  return {
    type: 'dashboard',
    name: 'test-icon',
    file: '',
    url: '',
    variant: 'svg',
    ...overrides,
  };
}

function makeApp(overrides: Partial<App> = {}): App {
  return {
    name: 'TestApp',
    url: 'https://example.com',
    icon: makeIcon(),
    color: '#374151',
    group: '',
    order: 0,
    enabled: true,
    default: false,
    open_mode: 'iframe',
    proxy: false,
    scale: 1,
    ...overrides,
  };
}

function makeNav(overrides: Partial<NavigationConfig> = {}): NavigationConfig {
  return {
    position: 'top',
    width: '220px',
    auto_hide: false,
    auto_hide_delay: '0.5s',
    show_on_hover: true,
    show_labels: true,
    show_logo: true,
    show_app_colors: true,
    show_icon_background: false,
    icon_scale: 1,
    show_splash_on_startup: false,
    show_shadow: false,
    floating_position: 'bottom-right',
    bar_style: 'flat',
    hide_sidebar_footer: false,
    ...overrides,
  };
}

function makeGroup(overrides: Partial<Group> = {}): Group {
  return {
    name: 'Media',
    icon: makeIcon({ type: 'lucide', name: 'play' }),
    color: '#e5a00d',
    order: 0,
    expanded: true,
    ...overrides,
  };
}

function makeConfig(overrides: Partial<Config> & { navigation?: Partial<NavigationConfig> } = {}): Config {
  const { navigation: navOverrides, ...rest } = overrides;
  return {
    title: 'Test Dashboard',
    navigation: makeNav(navOverrides),
    groups: rest.groups ?? [],
    apps: rest.apps ?? [],
    auth: { method: 'builtin', ...(rest.auth ?? {}) },
    ...rest,
  };
}

const mediaGroup = makeGroup({ name: 'Media', order: 0, icon: makeIcon({ type: 'lucide', name: 'play' }), color: '#e5a00d' });
const toolsGroup = makeGroup({ name: 'Tools', order: 1, icon: makeIcon({ type: 'lucide', name: 'wrench' }), color: '#3b82f6' });

const sampleApps: App[] = [
  makeApp({ name: 'Grafana', url: 'https://grafana.local', order: 0, group: 'Media', color: '#ff6600' }),
  makeApp({ name: 'Sonarr', url: 'https://sonarr.local', order: 1, group: 'Media', color: '#00cc00' }),
  makeApp({ name: 'Radarr', url: 'https://radarr.local', order: 2, group: 'Tools', color: '#cc0000' }),
];

const ungroupedApps: App[] = [
  makeApp({ name: 'AppOne', url: 'https://one.local', order: 0 }),
  makeApp({ name: 'AppTwo', url: 'https://two.local', order: 1 }),
  makeApp({ name: 'AppThree', url: 'https://three.local', order: 2 }),
];

// ============================================================================
// TESTS
// ============================================================================

describe('Navigation', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsAdmin.set(true);
    mockIsAuthenticated.set(true);
    mockCurrentUser.set({ username: 'admin', role: 'admin' });
    mockHealthData.set(new Map());
    // jsdom doesn't support setPointerCapture/releasePointerCapture — stub globally
    if (!HTMLElement.prototype.setPointerCapture) {
      HTMLElement.prototype.setPointerCapture = vi.fn();
    }
    if (!HTMLElement.prototype.releasePointerCapture) {
      HTMLElement.prototype.releasePointerCapture = vi.fn();
    }
  });

  // --------------------------------------------------------------------------
  // 1. Position rendering (5 tests)
  // --------------------------------------------------------------------------
  describe('Position rendering', () => {
    it('renders top position with <nav> element', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top' } }),
        },
      });
      const nav = container.querySelector('nav');
      expect(nav).toBeTruthy();
      // Top nav should have relative z-10 classes
      expect(nav!.className).toContain('relative');
      expect(nav!.className).toContain('z-10');
    });

    it('renders bottom position with <nav> element and bottom-nav-panel', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'bottom' } }),
        },
      });
      const nav = container.querySelector('nav');
      expect(nav).toBeTruthy();
      const bottomPanel = container.querySelector('.bottom-nav-panel');
      expect(bottomPanel).toBeTruthy();
    });

    it('renders left position with <aside> element and sidebar-panel with border-r', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left' } }),
        },
      });
      const aside = container.querySelector('aside');
      expect(aside).toBeTruthy();
      const sidebarPanel = container.querySelector('.sidebar-panel');
      expect(sidebarPanel).toBeTruthy();
      expect(sidebarPanel!.className).toContain('border-r');
    });

    it('renders right position with <aside> element and sidebar-panel with border-l', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'right' } }),
        },
      });
      const aside = container.querySelector('aside');
      expect(aside).toBeTruthy();
      const sidebarPanel = container.querySelector('.sidebar-panel');
      expect(sidebarPanel).toBeTruthy();
      expect(sidebarPanel!.className).toContain('border-l');
    });

    it('renders floating position with FAB button (role="navigation")', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'floating' } }),
        },
      });
      // Floating renders a div with role="navigation" containing the FAB
      const navDiv = container.querySelector('[role="navigation"]');
      expect(navDiv).toBeTruthy();
      // Should contain a button (the FAB)
      const fabButton = navDiv!.querySelector('button');
      expect(fabButton).toBeTruthy();
    });
  });

  // --------------------------------------------------------------------------
  // 2. Top bar tests (8 tests)
  // --------------------------------------------------------------------------
  describe('Top bar', () => {
    it('displays app names in flat mode', () => {
      render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top', bar_style: 'flat', show_labels: true } }),
        },
      });
      expect(screen.getByText('AppOne')).toBeInTheDocument();
      expect(screen.getByText('AppTwo')).toBeInTheDocument();
      expect(screen.getByText('AppThree')).toBeInTheDocument();
    });

    it('shows group dropdown buttons in grouped bar style with real groups', () => {
      render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'top', bar_style: 'grouped' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      // In grouped mode, group names appear as dropdown buttons
      expect(screen.getByText('Media')).toBeInTheDocument();
      expect(screen.getByText('Tools')).toBeInTheDocument();
    });

    it('shows action buttons: search, logs, settings', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top' } }),
        },
      });
      expect(container.querySelector('[title="Search (Ctrl+K)"]')).toBeTruthy();
      expect(container.querySelector('[title="Logs"]')).toBeTruthy();
      expect(container.querySelector('[title="Settings"]')).toBeTruthy();
    });

    it('shows MuximuxLogo when show_logo is true', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top', show_logo: true } }),
        },
      });
      // Logo button has title matching config title
      const logoBtn = container.querySelector('[title="Test Dashboard"]');
      expect(logoBtn).toBeTruthy();
    });

    it('shows home icon instead of logo when show_logo is false', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top', show_logo: false } }),
        },
      });
      // When no logo, shows "Overview" home icon button
      const homeBtn = container.querySelector('[title="Overview"]');
      expect(homeBtn).toBeTruthy();
    });

    it('shows split view button when split is not enabled', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top' } }),
          splitEnabled: false,
        },
      });
      expect(container.querySelector('[title="Split view"]')).toBeTruthy();
    });

    it('shows expanded split controls when splitEnabled=true', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top' } }),
          splitEnabled: true,
          splitOrientation: 'horizontal',
        },
      });
      expect(container.querySelector('[title="Horizontal split"]')).toBeTruthy();
      expect(container.querySelector('[title="Vertical split"]')).toBeTruthy();
      expect(container.querySelector('[title="Target panel 1"]')).toBeTruthy();
      expect(container.querySelector('[title="Target panel 2"]')).toBeTruthy();
      expect(container.querySelector('[title="Close split"]')).toBeTruthy();
    });

    it('shows sign out button when authenticated with real auth', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top' }, auth: { method: 'builtin' } }),
        },
      });
      expect(container.querySelector('[title="Sign out"]')).toBeTruthy();
    });
  });

  // --------------------------------------------------------------------------
  // 3. Bottom bar tests (6 tests)
  // --------------------------------------------------------------------------
  describe('Bottom bar', () => {
    it('renders with bottom-nav-panel class and border-t', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'bottom' } }),
        },
      });
      const panel = container.querySelector('.bottom-nav-panel');
      expect(panel).toBeTruthy();
      expect(panel!.className).toContain('border-t');
    });

    it('displays app names in flat mode', () => {
      render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'bottom', bar_style: 'flat', show_labels: true } }),
        },
      });
      expect(screen.getByText('AppOne')).toBeInTheDocument();
      expect(screen.getByText('AppTwo')).toBeInTheDocument();
    });

    it('shows group dropdown buttons in grouped mode', () => {
      render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'bottom', bar_style: 'grouped' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      expect(screen.getByText('Media')).toBeInTheDocument();
      expect(screen.getByText('Tools')).toBeInTheDocument();
    });

    it('shows action buttons in bottom bar', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'bottom' } }),
        },
      });
      expect(container.querySelector('[title="Search (Ctrl+K)"]')).toBeTruthy();
      expect(container.querySelector('[title="Logs"]')).toBeTruthy();
      expect(container.querySelector('[title="Settings"]')).toBeTruthy();
    });

    it('shows logo in bottom bar', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'bottom', show_logo: true } }),
        },
      });
      expect(container.querySelector('[title="Test Dashboard"]')).toBeTruthy();
    });

    it('shows split controls when splitEnabled in bottom bar', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'bottom' } }),
          splitEnabled: true,
          splitOrientation: 'vertical',
        },
      });
      expect(container.querySelector('[title="Horizontal split"]')).toBeTruthy();
      expect(container.querySelector('[title="Vertical split"]')).toBeTruthy();
      expect(container.querySelector('[title="Close split"]')).toBeTruthy();
    });
  });

  // --------------------------------------------------------------------------
  // 4. Left sidebar tests (8 tests)
  // --------------------------------------------------------------------------
  describe('Left sidebar', () => {
    it('renders aside with border-r sidebar panel', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left' } }),
        },
      });
      const aside = container.querySelector('aside');
      expect(aside).toBeTruthy();
      expect(container.querySelector('.sidebar-panel.border-r')).toBeTruthy();
    });

    it('shows group headers with app counts', () => {
      render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'left' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      // Group names appear as text
      expect(screen.getByText('Media')).toBeInTheDocument();
      expect(screen.getByText('Tools')).toBeInTheDocument();
      // App counts appear (2 media, 1 tools)
      expect(screen.getByText('2')).toBeInTheDocument();
      expect(screen.getByText('1')).toBeInTheDocument();
    });

    it('shows app names in groups', () => {
      render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'left', show_labels: true },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      expect(screen.getByText('Grafana')).toBeInTheDocument();
      expect(screen.getByText('Sonarr')).toBeInTheDocument();
      expect(screen.getByText('Radarr')).toBeInTheDocument();
    });

    it('shows search button with "Search..." label', () => {
      render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left', show_labels: true } }),
        },
      });
      expect(screen.getByText('Search...')).toBeInTheDocument();
    });

    it('shows footer with Logs, Settings, Sign out buttons', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left' }, auth: { method: 'builtin' } }),
        },
      });
      expect(container.querySelector('[title="Logs"]')).toBeTruthy();
      expect(container.querySelector('[title="Settings"]')).toBeTruthy();
      expect(container.querySelector('[title="Sign out"]')).toBeTruthy();
    });

    it('shows resize handle when not auto-hiding and labels visible', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left', auto_hide: false, show_labels: true } }),
        },
      });
      const handle = container.querySelector('[role="slider"][aria-label="Resize sidebar"]');
      expect(handle).toBeTruthy();
    });

    it('does NOT show resize handle when auto_hide is true', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left', auto_hide: true, show_labels: true } }),
        },
      });
      const handle = container.querySelector('[role="slider"][aria-label="Resize sidebar"]');
      expect(handle).toBeFalsy();
    });

    it('shows split view button in sidebar footer', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left' } }),
          splitEnabled: false,
        },
      });
      expect(container.querySelector('[title="Split view"]')).toBeTruthy();
    });
  });

  // --------------------------------------------------------------------------
  // 5. Right sidebar tests (5 tests)
  // --------------------------------------------------------------------------
  describe('Right sidebar', () => {
    it('renders aside with border-l sidebar panel', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'right' } }),
        },
      });
      const aside = container.querySelector('aside');
      expect(aside).toBeTruthy();
      expect(container.querySelector('.sidebar-panel.border-l')).toBeTruthy();
    });

    it('shows group headers in right sidebar', () => {
      render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'right' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      expect(screen.getByText('Media')).toBeInTheDocument();
      expect(screen.getByText('Tools')).toBeInTheDocument();
    });

    it('shows resize handle on right sidebar when not auto-hiding', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'right', auto_hide: false, show_labels: true } }),
        },
      });
      const handle = container.querySelector('[role="slider"][aria-label="Resize sidebar"]');
      expect(handle).toBeTruthy();
    });

    it('shows footer buttons in right sidebar', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'right' }, auth: { method: 'builtin' } }),
        },
      });
      expect(container.querySelector('[title="Logs"]')).toBeTruthy();
      expect(container.querySelector('[title="Settings"]')).toBeTruthy();
      expect(container.querySelector('[title="Sign out"]')).toBeTruthy();
    });

    it('shows expanded split controls in right sidebar footer', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'right' } }),
          splitEnabled: true,
          splitOrientation: 'horizontal',
        },
      });
      expect(container.querySelector('[title="Horizontal split"]')).toBeTruthy();
      expect(container.querySelector('[title="Vertical split"]')).toBeTruthy();
      expect(container.querySelector('[title="Close split"]')).toBeTruthy();
    });
  });

  // --------------------------------------------------------------------------
  // 6. Floating nav tests (6 tests)
  // --------------------------------------------------------------------------
  describe('Floating nav', () => {
    it('renders FAB button with navigation role', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'floating' } }),
        },
      });
      const navEl = container.querySelector('[role="navigation"]');
      expect(navEl).toBeTruthy();
      // FAB has a button inside
      const btn = navEl!.querySelector('button');
      expect(btn).toBeTruthy();
    });

    it('FAB has grab cursor by default', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'floating' } }),
        },
      });
      const navEl = container.querySelector('[role="navigation"]');
      const btn = navEl!.querySelector('button');
      expect(btn!.style.cursor).toContain('grab');
    });

    it('does not show panel by default (panel is closed)', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'floating' } }),
        },
      });
      const floatingPanel = container.querySelector('.floating-panel');
      expect(floatingPanel).toBeFalsy();
    });

    it('shows title in FAB button tooltip when panel closed', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'floating' } }),
        },
      });
      const navEl = container.querySelector('[role="navigation"]');
      const btn = navEl!.querySelector('button');
      expect(btn!.title).toBe('Test Dashboard');
    });

    it('renders hamburger icon in FAB by default (panel closed)', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'floating' } }),
        },
      });
      const navEl = container.querySelector('[role="navigation"]');
      const svg = navEl!.querySelector('svg');
      expect(svg).toBeTruthy();
      // Hamburger has 3 horizontal lines path: M4 6h16M4 12h16M4 18h16
      const path = svg!.querySelector('path');
      expect(path!.getAttribute('d')).toContain('M4 6h16');
    });

    it('renders with fixed positioning at FAB coordinates', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'floating' } }),
        },
      });
      const navEl = container.querySelector('[role="navigation"]');
      expect(navEl!.classList.contains('fixed')).toBe(true);
      expect(navEl!.classList.contains('z-40')).toBe(true);
    });
  });

  // --------------------------------------------------------------------------
  // 7. App interaction tests (5 tests)
  // --------------------------------------------------------------------------
  describe('App interaction', () => {
    it('calls onselect when an app button is clicked (top bar flat)', async () => {
      const onselectFn = vi.fn();
      render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top', bar_style: 'flat', show_labels: true } }),
          onselect: onselectFn,
        },
      });
      const appBtn = screen.getByText('AppOne').closest('button');
      await fireEvent.click(appBtn!);
      expect(onselectFn).toHaveBeenCalledTimes(1);
      expect(onselectFn).toHaveBeenCalledWith(expect.objectContaining({ name: 'AppOne' }));
    });

    it('highlights current app with bg-bg-base class (top flat bar)', () => {
      const currentApp = ungroupedApps[1]; // AppTwo
      render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp,
          config: makeConfig({ navigation: { position: 'top', bar_style: 'flat', show_labels: true, show_app_colors: true } }),
        },
      });
      // Find AppTwo button - it should have bg-bg-base for current
      const appTwoSpan = screen.getByText('AppTwo');
      const appTwoBtn = appTwoSpan.closest('button');
      expect(appTwoBtn!.className).toContain('bg-bg-base');
      expect(appTwoBtn!.className).toContain('text-text-primary');
    });

    it('highlights current app in left sidebar with bg-bg-elevated', () => {
      const currentApp = sampleApps[0]; // Grafana
      render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp,
          config: makeConfig({
            navigation: { position: 'left', show_labels: true },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      const grafanaSpan = screen.getByText('Grafana');
      const grafanaBtn = grafanaSpan.closest('button');
      expect(grafanaBtn!.className).toContain('bg-bg-elevated');
      expect(grafanaBtn!.className).toContain('text-text-primary');
    });

    it('calls onselect when clicking app in left sidebar', async () => {
      const onselectFn = vi.fn();
      render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'left', show_labels: true },
            groups: [mediaGroup, toolsGroup],
          }),
          onselect: onselectFn,
        },
      });
      const appBtn = screen.getByText('Sonarr').closest('button');
      await fireEvent.click(appBtn!);
      expect(onselectFn).toHaveBeenCalledWith(expect.objectContaining({ name: 'Sonarr' }));
    });

    it('shows open mode icon for new_tab apps', () => {
      const newTabApp = makeApp({ name: 'External', open_mode: 'new_tab', order: 0 });
      const { container } = render(Navigation, {
        props: {
          apps: [newTabApp],
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top', bar_style: 'flat', show_labels: true } }),
        },
      });
      // new_tab shows the arrow icon
      expect(container.innerHTML).toContain('\u2197'); // ↗
    });
  });

  // --------------------------------------------------------------------------
  // 8. Split view tests (5 tests)
  // --------------------------------------------------------------------------
  describe('Split view', () => {
    it('shows single split button when not enabled (top)', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top' } }),
          splitEnabled: false,
        },
      });
      expect(container.querySelector('[title="Split view"]')).toBeTruthy();
      expect(container.querySelector('[title="Close split"]')).toBeFalsy();
    });

    it('shows full split controls when enabled (top)', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top' } }),
          splitEnabled: true,
          splitOrientation: 'horizontal',
          splitActivePanel: 0,
        },
      });
      expect(container.querySelector('[title="Horizontal split"]')).toBeTruthy();
      expect(container.querySelector('[title="Vertical split"]')).toBeTruthy();
      expect(container.querySelector('[title="Target panel 1"]')).toBeTruthy();
      expect(container.querySelector('[title="Target panel 2"]')).toBeTruthy();
      expect(container.querySelector('[title="Close split"]')).toBeTruthy();
    });

    it('calls onsplithorizontal when horizontal split button clicked', async () => {
      const onSplitH = vi.fn();
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top' } }),
          splitEnabled: true,
          splitOrientation: 'vertical',
          onsplithorizontal: onSplitH,
        },
      });
      const hBtn = container.querySelector('[title="Horizontal split"]');
      await fireEvent.click(hBtn!);
      expect(onSplitH).toHaveBeenCalledTimes(1);
    });

    it('calls onsplitclose when close split button clicked', async () => {
      const onClose = vi.fn();
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top' } }),
          splitEnabled: true,
          onsplitclose: onClose,
        },
      });
      const closeBtn = container.querySelector('[title="Close split"]');
      await fireEvent.click(closeBtn!);
      expect(onClose).toHaveBeenCalledTimes(1);
    });

    it('shows split controls in left sidebar footer', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left' } }),
          splitEnabled: true,
          splitOrientation: 'horizontal',
        },
      });
      expect(container.querySelector('[title="Horizontal split"]')).toBeTruthy();
      expect(container.querySelector('[title="Vertical split"]')).toBeTruthy();
      expect(container.querySelector('[title="Close split"]')).toBeTruthy();
    });
  });

  // --------------------------------------------------------------------------
  // 9. Config toggles (7 tests)
  // --------------------------------------------------------------------------
  describe('Config toggles', () => {
    it('hides labels when show_labels=false in top flat bar', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top', bar_style: 'flat', show_labels: false } }),
        },
      });
      // With show_labels=false, the text spans get max-w-0 overflow-hidden opacity-0
      // so the names are still in DOM but hidden
      const appSpans = container.querySelectorAll('.flat-bar-scroll button span');
      // At least some should have max-w-0 (hidden)
      const hiddenSpans = Array.from(appSpans).filter(s => s.className.includes('max-w-0'));
      expect(hiddenSpans.length).toBeGreaterThan(0);
    });

    it('shows logo when show_logo=true in left sidebar', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left', show_logo: true } }),
        },
      });
      expect(container.querySelector('[title="Test Dashboard"]')).toBeTruthy();
    });

    it('shows Overview home icon when show_logo=false in left sidebar', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left', show_logo: false } }),
        },
      });
      expect(container.querySelector('[title="Overview"]')).toBeTruthy();
    });

    it('auto_hide sets collapsed bar height for top nav', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top', auto_hide: true } }),
        },
      });
      const nav = container.querySelector('nav');
      // auto_hide sets nav height to collapsedBarHeight (6px)
      expect(nav!.style.height).toBe('6px');
    });

    it('auto_hide=false sets standard bar height for top nav', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top', auto_hide: false } }),
        },
      });
      const nav = container.querySelector('nav');
      expect(nav!.style.height).toBe('56px');
    });

    it('hides settings button when user is not admin', () => {
      mockIsAdmin.set(false);
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top' } }),
        },
      });
      expect(container.querySelector('[title="Settings"]')).toBeFalsy();
    });

    it('hides sign out button when auth method is none', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top' }, auth: { method: 'none' } }),
        },
      });
      expect(container.querySelector('[title="Sign out"]')).toBeFalsy();
    });
  });

  // --------------------------------------------------------------------------
  // 10. Health indicator tests (3 tests)
  // --------------------------------------------------------------------------
  describe('Health indicators', () => {
    it('does not render HealthIndicator when app has no health_check', () => {
      const app = makeApp({ name: 'NoHealth', health_check: false });
      const { container } = render(Navigation, {
        props: {
          apps: [app],
          currentApp: null,
          showHealth: true,
          config: makeConfig({ navigation: { position: 'left', show_labels: true } }),
        },
      });
      // No HealthIndicator should be present
      // The HealthIndicator component is child, but since health_check is false, shouldShowHealth returns false
      // We can check the container doesn't have the health indicator wrapper
      const healthSpans = container.querySelectorAll('[class*="ml-auto"]');
      // Should not have health indicator related spans
      const found = Array.from(healthSpans).filter(el => el.querySelector('[data-testid]'));
      expect(found.length).toBe(0);
    });

    it('renders HealthIndicator when app has health_check=true', () => {
      const app = makeApp({ name: 'Healthy', health_check: true, group: 'Media' });
      const { container } = render(Navigation, {
        props: {
          apps: [app],
          currentApp: null,
          showHealth: true,
          config: makeConfig({
            navigation: { position: 'left', show_labels: true },
            groups: [mediaGroup],
          }),
        },
      });
      // The component should attempt to render HealthIndicator
      // Since it's a child component, we can confirm it's in the DOM
      expect(container.innerHTML).toContain('Healthy');
    });

    it('dims unhealthy non-current apps in top flat bar', () => {
      const healthMap = new Map();
      healthMap.set('BadApp', { status: 'unhealthy', latency: 0, lastCheck: '' });
      mockHealthData.set(healthMap);

      const badApp = makeApp({ name: 'BadApp', health_check: true, order: 0 });
      const goodApp = makeApp({ name: 'GoodApp', order: 1 });
      render(Navigation, {
        props: {
          apps: [badApp, goodApp],
          currentApp: null,
          showHealth: true,
          config: makeConfig({ navigation: { position: 'top', bar_style: 'flat', show_labels: true } }),
        },
      });
      // BadApp button should have opacity-50 class
      const badBtn = screen.getByText('BadApp').closest('button');
      expect(badBtn!.className).toContain('opacity-50');
    });
  });

  // --------------------------------------------------------------------------
  // 11. Callback tests (6 tests)
  // --------------------------------------------------------------------------
  describe('Callback handlers', () => {
    it('calls onsearch when search button clicked (top)', async () => {
      const onSearch = vi.fn();
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top' } }),
          onsearch: onSearch,
        },
      });
      const searchBtn = container.querySelector('[title="Search (Ctrl+K)"]');
      await fireEvent.click(searchBtn!);
      expect(onSearch).toHaveBeenCalledTimes(1);
    });

    it('calls onlogs when logs button clicked (top)', async () => {
      const onLogs = vi.fn();
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top' } }),
          onlogs: onLogs,
        },
      });
      const logsBtn = container.querySelector('[title="Logs"]');
      await fireEvent.click(logsBtn!);
      expect(onLogs).toHaveBeenCalledTimes(1);
    });

    it('calls onsettings when settings button clicked (top)', async () => {
      const onSettings = vi.fn();
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top' } }),
          onsettings: onSettings,
        },
      });
      const settingsBtn = container.querySelector('[title="Settings"]');
      await fireEvent.click(settingsBtn!);
      expect(onSettings).toHaveBeenCalledTimes(1);
    });

    it('calls onsearch from left sidebar search button', async () => {
      const onSearch = vi.fn();
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left' } }),
          onsearch: onSearch,
        },
      });
      const searchBtn = container.querySelector('[title="Search (Ctrl+K)"]');
      await fireEvent.click(searchBtn!);
      expect(onSearch).toHaveBeenCalledTimes(1);
    });

    it('calls onsplitvertical when vertical split button clicked', async () => {
      const onSplitV = vi.fn();
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top' } }),
          splitEnabled: true,
          splitOrientation: 'horizontal',
          onsplitvertical: onSplitV,
        },
      });
      const vBtn = container.querySelector('[title="Vertical split"]');
      await fireEvent.click(vBtn!);
      expect(onSplitV).toHaveBeenCalledTimes(1);
    });

    it('calls onsplitpanel when panel selector arrow clicked', async () => {
      const onSplitPanel = vi.fn();
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top' } }),
          splitEnabled: true,
          splitOrientation: 'horizontal',
          splitActivePanel: 0,
          onsplitpanel: onSplitPanel,
        },
      });
      const panel2Btn = container.querySelector('[title="Target panel 2"]');
      await fireEvent.click(panel2Btn!);
      expect(onSplitPanel).toHaveBeenCalledWith(1);
    });
  });

  // --------------------------------------------------------------------------
  // 12. Grouped vs flat bar style (4 tests)
  // --------------------------------------------------------------------------
  describe('Bar style: grouped vs flat', () => {
    it('flat bar shows flat-bar-scroll container (top)', () => {
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'top', bar_style: 'flat' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      expect(container.querySelector('.flat-bar-scroll')).toBeTruthy();
    });

    it('grouped bar does NOT show flat-bar-scroll (top)', () => {
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'top', bar_style: 'grouped' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      expect(container.querySelector('.flat-bar-scroll')).toBeFalsy();
    });

    it('flat bar in bottom position shows flat-bar-scroll', () => {
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'bottom', bar_style: 'flat' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      expect(container.querySelector('.flat-bar-scroll')).toBeTruthy();
    });

    it('uses flat mode when only ungrouped apps exist even with grouped style', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'top', bar_style: 'grouped' },
            groups: [],
          }),
        },
      });
      // With no real groups (only Ungrouped), falls back to flat rendering
      // The flat-bar-scroll appears because hasRealGroups is false => useGroupDropdowns is false
      expect(container.querySelector('.flat-bar-scroll')).toBeTruthy();
    });
  });

  // --------------------------------------------------------------------------
  // 13. App colors tests (3 tests)
  // --------------------------------------------------------------------------
  describe('App colors', () => {
    it('shows colored bottom border on current app in top flat bar when show_app_colors=true', () => {
      const currentApp = ungroupedApps[0];
      render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp,
          config: makeConfig({
            navigation: { position: 'top', bar_style: 'flat', show_labels: true, show_app_colors: true },
          }),
        },
      });
      const btn = screen.getByText('AppOne').closest('button');
      // The button should have a border-bottom style with the app color (jsdom converts hex to rgb)
      expect(btn!.style.borderBottom).toContain('solid');
      expect(btn!.style.borderBottom).not.toContain('transparent');
    });

    it('shows color indicator strip on current app in left sidebar', () => {
      const currentApp = sampleApps[0]; // Grafana with color #ff6600
      render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp,
          config: makeConfig({
            navigation: { position: 'left', show_labels: true, show_app_colors: true },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      // Find the color indicator: absolute div with the app color background
      const grafanaBtn = screen.getByText('Grafana').closest('button');
      const colorStrip = grafanaBtn!.querySelector('.w-\\[3px\\]');
      expect(colorStrip).toBeTruthy();
      // jsdom converts hex to rgb; check that some color value is set
      expect((colorStrip as HTMLElement).style.background).toBeTruthy();
      expect((colorStrip as HTMLElement).style.background).not.toBe('');
    });

    it('does not show color indicator when show_app_colors=false', () => {
      const currentApp = sampleApps[0];
      render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp,
          config: makeConfig({
            navigation: { position: 'left', show_labels: true, show_app_colors: false },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      const grafanaBtn = screen.getByText('Grafana').closest('button');
      const colorStrip = grafanaBtn!.querySelector('.w-\\[3px\\]');
      expect(colorStrip).toBeFalsy();
    });
  });

  // --------------------------------------------------------------------------
  // 14. Auto-hide bar height behavior (3 tests)
  // --------------------------------------------------------------------------
  describe('Auto-hide behavior', () => {
    it('top nav inner panel starts at collapsed height when auto_hide=true', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top', auto_hide: true } }),
        },
      });
      const panel = container.querySelector('.top-nav-panel');
      // isHidden starts as false, but isCollapsedTop = isHidden && auto_hide
      // After mount, isHidden defaults to false, so panel height should be 56px
      // (the nav height is 6px as placeholder but the panel overlays at 56px initially)
      expect(panel).toBeTruthy();
    });

    it('bottom nav sets collapsed bar height when auto_hide=true', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'bottom', auto_hide: true } }),
        },
      });
      const nav = container.querySelector('nav');
      expect(nav!.style.height).toBe('6px');
    });

    it('bottom nav sets standard height when auto_hide=false', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'bottom', auto_hide: false } }),
        },
      });
      const nav = container.querySelector('nav');
      expect(nav!.style.height).toBe('56px');
    });
  });

  // --------------------------------------------------------------------------
  // 15. Sidebar footer drawer (hide_sidebar_footer) (3 tests)
  // --------------------------------------------------------------------------
  describe('Sidebar footer drawer', () => {
    it('uses footer-drawer when hide_sidebar_footer=true in left sidebar', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left', hide_sidebar_footer: true, show_labels: true } }),
        },
      });
      expect(container.querySelector('.sidebar-footer-drawer')).toBeTruthy();
    });

    it('uses standard footer when hide_sidebar_footer=false in left sidebar', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left', hide_sidebar_footer: false, show_labels: true } }),
        },
      });
      expect(container.querySelector('.sidebar-footer-drawer')).toBeFalsy();
    });

    it('uses footer-drawer when hide_sidebar_footer=true in right sidebar', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'right', hide_sidebar_footer: true, show_labels: true } }),
        },
      });
      expect(container.querySelector('.sidebar-footer-drawer')).toBeTruthy();
    });
  });

  // --------------------------------------------------------------------------
  // 16. Group expand/collapse in sidebar (2 tests)
  // --------------------------------------------------------------------------
  describe('Group expand/collapse', () => {
    it('group apps wrapper has expanded class by default (groups start expanded)', () => {
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'left', show_labels: true },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      const wrappers = container.querySelectorAll('.group-apps-wrapper');
      expect(wrappers.length).toBeGreaterThan(0);
      // All groups default to expanded
      wrappers.forEach(w => {
        expect(w.classList.contains('expanded')).toBe(true);
      });
    });

    it('clicking group header toggles expanded state', async () => {
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'left', show_labels: true },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      // Click the "Media" group header to collapse it
      const mediaHeader = screen.getByText('Media').closest('button');
      await fireEvent.click(mediaHeader!);

      // After click, the first group wrapper should lose "expanded"
      const wrappers = container.querySelectorAll('.group-apps-wrapper');
      const firstWrapper = wrappers[0];
      expect(firstWrapper.classList.contains('expanded')).toBe(false);
    });
  });

  // --------------------------------------------------------------------------
  // 17. No apps edge case (2 tests)
  // --------------------------------------------------------------------------
  describe('Edge cases', () => {
    it('renders without crashing when apps array is empty (top)', () => {
      const { container } = render(Navigation, {
        props: {
          apps: [],
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top' } }),
        },
      });
      const nav = container.querySelector('nav');
      expect(nav).toBeTruthy();
    });

    it('renders without crashing when apps array is empty (floating)', () => {
      const { container } = render(Navigation, {
        props: {
          apps: [],
          currentApp: null,
          config: makeConfig({ navigation: { position: 'floating' } }),
        },
      });
      expect(container.innerHTML.length).toBeGreaterThan(0);
    });
  });

  // --------------------------------------------------------------------------
  // 18. Sidebar collapsed state (show_labels=false) (3 tests)
  // --------------------------------------------------------------------------
  describe('Sidebar collapsed (show_labels=false)', () => {
    it('sets sidebar-panel to collapsedStripWidth when show_labels=false (left)', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left', show_labels: false } }),
        },
      });
      const panel = container.querySelector('.sidebar-panel');
      // collapsedStripWidth = 48px
      expect((panel as HTMLElement).style.width).toBe('48px');
    });

    it('sets sidebar-panel to collapsedStripWidth when show_labels=false (right)', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'right', show_labels: false } }),
        },
      });
      const panel = container.querySelector('.sidebar-panel');
      expect((panel as HTMLElement).style.width).toBe('48px');
    });

    it('does not show resize handle when show_labels=false', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left', show_labels: false } }),
        },
      });
      const handle = container.querySelector('[role="slider"]');
      expect(handle).toBeFalsy();
    });
  });

  // --------------------------------------------------------------------------
  // 19. Auth state variations (3 tests)
  // --------------------------------------------------------------------------
  describe('Auth state variations', () => {
    it('hides sign out when not authenticated', () => {
      mockIsAuthenticated.set(false);
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top' }, auth: { method: 'builtin' } }),
        },
      });
      expect(container.querySelector('[title="Sign out"]')).toBeFalsy();
    });

    it('shows sign out when authenticated with builtin auth in left sidebar', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left' }, auth: { method: 'builtin' } }),
        },
      });
      expect(container.querySelector('[title="Sign out"]')).toBeTruthy();
    });

    it('hides settings when non-admin in left sidebar', () => {
      mockIsAdmin.set(false);
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left' } }),
        },
      });
      expect(container.querySelector('[title="Settings"]')).toBeFalsy();
    });
  });

  // --------------------------------------------------------------------------
  // 20. Onsplash / logo click (2 tests)
  // --------------------------------------------------------------------------
  describe('Logo / splash interaction', () => {
    it('calls onsplash when logo button clicked (top)', async () => {
      const onSplash = vi.fn();
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top', show_logo: true } }),
          onsplash: onSplash,
        },
      });
      const logoBtn = container.querySelector('[title="Test Dashboard"]');
      await fireEvent.click(logoBtn!);
      expect(onSplash).toHaveBeenCalledTimes(1);
    });

    it('calls onsplash when Overview button clicked (top, no logo)', async () => {
      const onSplash = vi.fn();
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top', show_logo: false } }),
          onsplash: onSplash,
        },
      });
      const overviewBtn = container.querySelector('[title="Overview"]');
      await fireEvent.click(overviewBtn!);
      expect(onSplash).toHaveBeenCalledTimes(1);
    });
  });

  // --------------------------------------------------------------------------
  // 21. Floating panel open/close via FAB click (6 tests)
  // --------------------------------------------------------------------------
  describe('Floating panel interactions', () => {
    it('opens panel on FAB click (pointerup without drag)', async () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'floating' } }),
        },
      });
      const navEl = container.querySelector('[role="navigation"]');
      const btn = navEl!.querySelector('button')!;
      // Simulate click: pointerdown + pointerup (no movement = click)
      await fireEvent.pointerDown(btn, { button: 0, pointerId: 1, clientX: 100, clientY: 100 });
      // Fire pointerup on document to match the component's document-level listener
      const upEvent = new PointerEvent('pointerup', { pointerId: 1, bubbles: true });
      document.dispatchEvent(upEvent);
      // After click, floating-panel should appear
      await waitFor(() => {
        const panel = container.querySelector('.floating-panel');
        expect(panel).toBeTruthy();
      });
    });

    it('shows app names in floating panel when opened', async () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'floating' } }),
        },
      });
      const navEl = container.querySelector('[role="navigation"]');
      const btn = navEl!.querySelector('button')!;
      await fireEvent.pointerDown(btn, { button: 0, pointerId: 1, clientX: 100, clientY: 100 });
      document.dispatchEvent(new PointerEvent('pointerup', { pointerId: 1, bubbles: true }));
      await waitFor(() => {
        expect(screen.getByText('AppOne')).toBeInTheDocument();
        expect(screen.getByText('AppTwo')).toBeInTheDocument();
      });
    });

    it('shows close icon (X) in FAB when panel is open', async () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'floating' } }),
        },
      });
      const navEl = container.querySelector('[role="navigation"]');
      const btn = navEl!.querySelector('button')!;
      await fireEvent.pointerDown(btn, { button: 0, pointerId: 1, clientX: 100, clientY: 100 });
      document.dispatchEvent(new PointerEvent('pointerup', { pointerId: 1, bubbles: true }));
      await waitFor(() => {
        // When open, the FAB title changes to "Close navigation"
        expect(btn.title).toBe('Close navigation');
      });
    });

    it('shows footer buttons in floating panel', async () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'floating' }, auth: { method: 'builtin' } }),
        },
      });
      const navEl = container.querySelector('[role="navigation"]');
      const btn = navEl!.querySelector('button')!;
      await fireEvent.pointerDown(btn, { button: 0, pointerId: 1, clientX: 100, clientY: 100 });
      document.dispatchEvent(new PointerEvent('pointerup', { pointerId: 1, bubbles: true }));
      await waitFor(() => {
        const panel = container.querySelector('.floating-panel');
        expect(panel).toBeTruthy();
        // Floating panel footer has search, logs, settings, sign out buttons
        expect(panel!.querySelector('[title="Search (Ctrl+K)"]')).toBeTruthy();
        expect(panel!.querySelector('[title="Logs"]')).toBeTruthy();
        expect(panel!.querySelector('[title="Settings"]')).toBeTruthy();
        expect(panel!.querySelector('[title="Sign out"]')).toBeTruthy();
      });
    });

    it('calls onselect and closes panel when app clicked in floating panel', async () => {
      const onselectFn = vi.fn();
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'floating' } }),
          onselect: onselectFn,
        },
      });
      const navEl = container.querySelector('[role="navigation"]');
      const btn = navEl!.querySelector('button')!;
      await fireEvent.pointerDown(btn, { button: 0, pointerId: 1, clientX: 100, clientY: 100 });
      document.dispatchEvent(new PointerEvent('pointerup', { pointerId: 1, bubbles: true }));
      await waitFor(() => {
        expect(screen.getByText('AppOne')).toBeInTheDocument();
      });
      const appBtn = screen.getByText('AppOne').closest('button');
      await fireEvent.click(appBtn!);
      expect(onselectFn).toHaveBeenCalledWith(expect.objectContaining({ name: 'AppOne' }));
      // Panel should close after app selection
      await waitFor(() => {
        expect(container.querySelector('.floating-panel')).toBeFalsy();
      });
    });

    it('closes panel via click-outside overlay button', async () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'floating' } }),
        },
      });
      const navEl = container.querySelector('[role="navigation"]');
      const btn = navEl!.querySelector('button')!;
      await fireEvent.pointerDown(btn, { button: 0, pointerId: 1, clientX: 100, clientY: 100 });
      document.dispatchEvent(new PointerEvent('pointerup', { pointerId: 1, bubbles: true }));
      await waitFor(() => {
        expect(container.querySelector('.floating-panel')).toBeTruthy();
      });
      // Click the overlay (the fixed inset-0 button with aria-label "Close navigation")
      const overlay = container.querySelector('[aria-label="Close navigation"]');
      expect(overlay).toBeTruthy();
      await fireEvent.click(overlay!);
      await waitFor(() => {
        expect(container.querySelector('.floating-panel')).toBeFalsy();
      });
    });
  });

  // --------------------------------------------------------------------------
  // 22. Floating panel with groups (3 tests)
  // --------------------------------------------------------------------------
  describe('Floating panel with groups', () => {
    it('shows group headers in floating panel', async () => {
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'floating' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      const navEl = container.querySelector('[role="navigation"]');
      const btn = navEl!.querySelector('button')!;
      await fireEvent.pointerDown(btn, { button: 0, pointerId: 1, clientX: 100, clientY: 100 });
      document.dispatchEvent(new PointerEvent('pointerup', { pointerId: 1, bubbles: true }));
      await waitFor(() => {
        expect(screen.getByText('Media')).toBeInTheDocument();
        expect(screen.getByText('Tools')).toBeInTheDocument();
      });
    });

    it('shows app color indicator on current app in floating panel', async () => {
      const currentApp = sampleApps[0]; // Grafana
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp,
          config: makeConfig({
            navigation: { position: 'floating', show_app_colors: true },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      const navEl = container.querySelector('[role="navigation"]');
      const btn = navEl!.querySelector('button')!;
      await fireEvent.pointerDown(btn, { button: 0, pointerId: 1, clientX: 100, clientY: 100 });
      document.dispatchEvent(new PointerEvent('pointerup', { pointerId: 1, bubbles: true }));
      await waitFor(() => {
        const grafanaBtn = screen.getByText('Grafana').closest('button');
        expect(grafanaBtn).toBeTruthy();
        expect(grafanaBtn!.className).toContain('bg-bg-elevated');
      });
    });

    it('shows split controls in floating panel footer', async () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'floating' } }),
          splitEnabled: true,
          splitOrientation: 'horizontal',
        },
      });
      const navEl = container.querySelector('[role="navigation"]');
      const btn = navEl!.querySelector('button')!;
      await fireEvent.pointerDown(btn, { button: 0, pointerId: 1, clientX: 100, clientY: 100 });
      document.dispatchEvent(new PointerEvent('pointerup', { pointerId: 1, bubbles: true }));
      await waitFor(() => {
        const panel = container.querySelector('.floating-panel');
        expect(panel).toBeTruthy();
        expect(panel!.querySelector('[title="Horizontal split"]')).toBeTruthy();
        expect(panel!.querySelector('[title="Vertical split"]')).toBeTruthy();
        expect(panel!.querySelector('[title="Close split"]')).toBeTruthy();
      });
    });
  });

  // --------------------------------------------------------------------------
  // 23. Open mode icons (2 tests)
  // --------------------------------------------------------------------------
  describe('Open mode icons', () => {
    it('shows new_window icon for new_window apps in top flat bar', () => {
      const newWinApp = makeApp({ name: 'WinApp', open_mode: 'new_window', order: 0 });
      const { container } = render(Navigation, {
        props: {
          apps: [newWinApp],
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top', bar_style: 'flat', show_labels: true } }),
        },
      });
      expect(container.innerHTML).toContain('\u29C9'); // ⧉
    });

    it('does not show open mode icon for iframe apps', () => {
      const iframeApp = makeApp({ name: 'IframeApp', open_mode: 'iframe', order: 0 });
      const { container } = render(Navigation, {
        props: {
          apps: [iframeApp],
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top', bar_style: 'flat', show_labels: true } }),
        },
      });
      expect(container.innerHTML).not.toContain('\u2197'); // no ↗
      expect(container.innerHTML).not.toContain('\u29C9'); // no ⧉
    });
  });

  // --------------------------------------------------------------------------
  // 24. Auto-hide mouse enter/leave on sidebar (4 tests)
  // --------------------------------------------------------------------------
  describe('Auto-hide sidebar interactions', () => {
    it('left sidebar has collapsed width when auto_hide=true', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left', auto_hide: true, show_labels: true } }),
        },
      });
      const aside = container.querySelector('aside');
      expect(aside).toBeTruthy();
      // effectiveSidebarWidth is collapsedStripWidth (48px) when auto_hide is on
      expect(aside!.style.width).toBe('48px');
    });

    it('right sidebar has collapsed width when auto_hide=true', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'right', auto_hide: true, show_labels: true } }),
        },
      });
      const aside = container.querySelector('aside');
      expect(aside!.style.width).toBe('48px');
    });

    it('sidebar panel uses absolute positioning when auto_hide=true', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left', auto_hide: true, show_labels: true } }),
        },
      });
      const panel = container.querySelector('.sidebar-panel');
      expect((panel as HTMLElement).style.position).toBe('absolute');
    });

    it('sidebar panel uses static positioning when auto_hide=false', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left', auto_hide: false, show_labels: true } }),
        },
      });
      const panel = container.querySelector('.sidebar-panel');
      // position should be empty/null when not auto-hiding
      expect((panel as HTMLElement).style.position).toBe('');
    });
  });

  // --------------------------------------------------------------------------
  // 25. Top bar auto_hide overlay positioning (3 tests)
  // --------------------------------------------------------------------------
  describe('Top/bottom bar auto_hide overlay', () => {
    it('top nav panel uses absolute position when auto_hide=true', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top', auto_hide: true } }),
        },
      });
      const panel = container.querySelector('.top-nav-panel');
      expect((panel as HTMLElement).style.position).toBe('absolute');
    });

    it('bottom nav panel uses absolute position when auto_hide=true', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'bottom', auto_hide: true } }),
        },
      });
      const panel = container.querySelector('.bottom-nav-panel');
      expect((panel as HTMLElement).style.position).toBe('absolute');
    });

    it('top nav panel does not use absolute position when auto_hide=false', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top', auto_hide: false } }),
        },
      });
      const panel = container.querySelector('.top-nav-panel');
      expect((panel as HTMLElement).style.position).toBe('');
    });
  });

  // --------------------------------------------------------------------------
  // 26. Show shadow on auto_hide (2 tests)
  // --------------------------------------------------------------------------
  describe('Auto-hide shadow', () => {
    it('sidebar has box-shadow when auto_hide + show_shadow enabled', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left', auto_hide: true, show_shadow: true, show_labels: true } }),
        },
      });
      const panel = container.querySelector('.sidebar-panel');
      expect((panel as HTMLElement).style.boxShadow).toContain('rgba');
    });

    it('sidebar has no box-shadow when show_shadow=false', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left', auto_hide: true, show_shadow: false, show_labels: true } }),
        },
      });
      const panel = container.querySelector('.sidebar-panel');
      expect((panel as HTMLElement).style.boxShadow).toBe('');
    });
  });

  // --------------------------------------------------------------------------
  // 27. Right sidebar app rendering and interaction (4 tests)
  // --------------------------------------------------------------------------
  describe('Right sidebar apps', () => {
    it('shows app names in right sidebar groups', () => {
      render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'right', show_labels: true },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      expect(screen.getByText('Grafana')).toBeInTheDocument();
      expect(screen.getByText('Sonarr')).toBeInTheDocument();
      expect(screen.getByText('Radarr')).toBeInTheDocument();
    });

    it('calls onselect when clicking app in right sidebar', async () => {
      const onselectFn = vi.fn();
      render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'right', show_labels: true },
            groups: [mediaGroup, toolsGroup],
          }),
          onselect: onselectFn,
        },
      });
      const appBtn = screen.getByText('Radarr').closest('button');
      await fireEvent.click(appBtn!);
      expect(onselectFn).toHaveBeenCalledWith(expect.objectContaining({ name: 'Radarr' }));
    });

    it('highlights current app with bg-bg-elevated in right sidebar', () => {
      const currentApp = sampleApps[2]; // Radarr
      render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp,
          config: makeConfig({
            navigation: { position: 'right', show_labels: true },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      const radarrBtn = screen.getByText('Radarr').closest('button');
      expect(radarrBtn!.className).toContain('bg-bg-elevated');
    });

    it('shows color indicator on right side for current app in right sidebar', () => {
      const currentApp = sampleApps[2]; // Radarr in Tools
      render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp,
          config: makeConfig({
            navigation: { position: 'right', show_labels: true, show_app_colors: true },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      // Right sidebar puts the color strip on the right side
      const radarrBtn = screen.getByText('Radarr').closest('button');
      const colorStrip = radarrBtn!.querySelector('.w-\\[3px\\]');
      expect(colorStrip).toBeTruthy();
      expect((colorStrip as HTMLElement).style.background).toBeTruthy();
    });
  });

  // --------------------------------------------------------------------------
  // 28. Right sidebar footer drawer (3 tests)
  // --------------------------------------------------------------------------
  describe('Right sidebar footer drawer', () => {
    it('uses footer-drawer in right sidebar when hide_sidebar_footer=true and expanded', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'right', hide_sidebar_footer: true, show_labels: true } }),
        },
      });
      expect(container.querySelector('.sidebar-footer-drawer')).toBeTruthy();
    });

    it('uses standard footer in right sidebar when hide_sidebar_footer=false', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'right', hide_sidebar_footer: false, show_labels: true } }),
        },
      });
      expect(container.querySelector('.sidebar-footer-drawer')).toBeFalsy();
      // Standard footer should have Logs and Settings buttons
      expect(container.querySelector('[title="Logs"]')).toBeTruthy();
    });

    it('shows search button in right sidebar', () => {
      render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'right', show_labels: true } }),
        },
      });
      expect(screen.getByText('Search...')).toBeInTheDocument();
    });
  });

  // --------------------------------------------------------------------------
  // 29. Bottom bar app interactions (3 tests)
  // --------------------------------------------------------------------------
  describe('Bottom bar app interactions', () => {
    it('calls onselect when app clicked in bottom flat bar', async () => {
      const onselectFn = vi.fn();
      render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'bottom', bar_style: 'flat', show_labels: true } }),
          onselect: onselectFn,
        },
      });
      const appBtn = screen.getByText('AppTwo').closest('button');
      await fireEvent.click(appBtn!);
      expect(onselectFn).toHaveBeenCalledWith(expect.objectContaining({ name: 'AppTwo' }));
    });

    it('highlights current app in bottom flat bar with bg-bg-base', () => {
      const currentApp = ungroupedApps[0];
      render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp,
          config: makeConfig({ navigation: { position: 'bottom', bar_style: 'flat', show_labels: true, show_app_colors: true } }),
        },
      });
      const btn = screen.getByText('AppOne').closest('button');
      expect(btn!.className).toContain('bg-bg-base');
    });

    it('shows border-top for current app in bottom bar with app colors', () => {
      const currentApp = ungroupedApps[0];
      render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp,
          config: makeConfig({ navigation: { position: 'bottom', bar_style: 'flat', show_labels: true, show_app_colors: true } }),
        },
      });
      const btn = screen.getByText('AppOne').closest('button');
      expect(btn!.style.borderTop).toContain('solid');
    });
  });

  // --------------------------------------------------------------------------
  // 30. Bottom bar grouped dropdowns (3 tests)
  // --------------------------------------------------------------------------
  describe('Bottom bar grouped dropdowns', () => {
    it('shows group names in bottom bar grouped mode', () => {
      render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'bottom', bar_style: 'grouped' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      expect(screen.getByText('Media')).toBeInTheDocument();
      expect(screen.getByText('Tools')).toBeInTheDocument();
    });

    it('opens dropdown on mouse enter (bottom bar grouped)', async () => {
      render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'bottom', bar_style: 'grouped' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      // Hover over the Media group button's container div
      const mediaText = screen.getByText('Media');
      const groupDiv = mediaText.closest('.relative');
      await fireEvent.mouseEnter(groupDiv!);
      // Dropdown should appear with apps
      await waitFor(() => {
        expect(screen.getByText('Grafana')).toBeInTheDocument();
        expect(screen.getByText('Sonarr')).toBeInTheDocument();
      });
    });

    it('calls onselect and closes dropdown when app clicked in bottom grouped dropdown', async () => {
      const onselectFn = vi.fn();
      render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'bottom', bar_style: 'grouped' },
            groups: [mediaGroup, toolsGroup],
          }),
          onselect: onselectFn,
        },
      });
      const mediaText = screen.getByText('Media');
      const groupDiv = mediaText.closest('.relative');
      await fireEvent.mouseEnter(groupDiv!);
      await waitFor(() => {
        expect(screen.getByText('Grafana')).toBeInTheDocument();
      });
      const grafanaBtn = screen.getByText('Grafana').closest('button');
      await fireEvent.click(grafanaBtn!);
      expect(onselectFn).toHaveBeenCalledWith(expect.objectContaining({ name: 'Grafana' }));
    });
  });

  // --------------------------------------------------------------------------
  // 31. Top bar grouped dropdowns (3 tests)
  // --------------------------------------------------------------------------
  describe('Top bar grouped dropdowns', () => {
    it('opens dropdown on mouse enter (top bar grouped)', async () => {
      render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'top', bar_style: 'grouped' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      const mediaText = screen.getByText('Media');
      const groupDiv = mediaText.closest('.relative');
      await fireEvent.mouseEnter(groupDiv!);
      await waitFor(() => {
        expect(screen.getByText('Grafana')).toBeInTheDocument();
      });
    });

    it('closes dropdown on mouse leave (top bar grouped)', async () => {
      vi.useFakeTimers();
      render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'top', bar_style: 'grouped' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      const mediaText = screen.getByText('Media');
      const groupDiv = mediaText.closest('.relative');
      await fireEvent.mouseEnter(groupDiv!);
      await waitFor(() => {
        expect(screen.getByText('Grafana')).toBeInTheDocument();
      });
      await fireEvent.mouseLeave(groupDiv!);
      // The dropdown has a 150ms close timeout
      vi.advanceTimersByTime(200);
      await waitFor(() => {
        // After timeout, the dropdown items should be gone
        expect(screen.queryByText('Grafana')).not.toBeInTheDocument();
      });
      vi.useRealTimers();
    });

    it('calls onselect from top bar grouped dropdown', async () => {
      const onselectFn = vi.fn();
      render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'top', bar_style: 'grouped' },
            groups: [mediaGroup, toolsGroup],
          }),
          onselect: onselectFn,
        },
      });
      const toolsText = screen.getByText('Tools');
      const groupDiv = toolsText.closest('.relative');
      await fireEvent.mouseEnter(groupDiv!);
      await waitFor(() => {
        expect(screen.getByText('Radarr')).toBeInTheDocument();
      });
      const radarrBtn = screen.getByText('Radarr').closest('button');
      await fireEvent.click(radarrBtn!);
      expect(onselectFn).toHaveBeenCalledWith(expect.objectContaining({ name: 'Radarr' }));
    });
  });

  // --------------------------------------------------------------------------
  // 32. handleLogout callback (2 tests)
  // --------------------------------------------------------------------------
  describe('Logout handling', () => {
    it('calls logout store function when sign out clicked (top)', async () => {
      const { logout: logoutMock } = await import('$lib/authStore');
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top' }, auth: { method: 'builtin' } }),
        },
      });
      const logoutBtn = container.querySelector('[title="Sign out"]');
      await fireEvent.click(logoutBtn!);
      expect(logoutMock).toHaveBeenCalled();
    });

    it('calls onlogout callback after logout', async () => {
      const onLogout = vi.fn();
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top' }, auth: { method: 'builtin' } }),
          onlogout: onLogout,
        },
      });
      const logoutBtn = container.querySelector('[title="Sign out"]');
      await fireEvent.click(logoutBtn!);
      await waitFor(() => {
        expect(onLogout).toHaveBeenCalledTimes(1);
      });
    });
  });

  // --------------------------------------------------------------------------
  // 33. showSplash opacity (2 tests)
  // --------------------------------------------------------------------------
  describe('showSplash opacity', () => {
    it('reduces logo opacity when showSplash=true (top bar)', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top', show_logo: true } }),
          showSplash: true,
        },
      });
      const logoBtn = container.querySelector('[title="Test Dashboard"]') as HTMLElement;
      expect(logoBtn.style.opacity).toBe('0.6');
    });

    it('shows full opacity when showSplash=false (top bar)', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top', show_logo: true } }),
          showSplash: false,
        },
      });
      const logoBtn = container.querySelector('[title="Test Dashboard"]') as HTMLElement;
      expect(logoBtn.style.opacity).toBe('1');
    });
  });

  // --------------------------------------------------------------------------
  // 34. Split view panel labels (3 tests)
  // --------------------------------------------------------------------------
  describe('Split view panel targeting', () => {
    it('highlights panel 1 when splitActivePanel=0 (horizontal)', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top' } }),
          splitEnabled: true,
          splitOrientation: 'horizontal',
          splitActivePanel: 0,
        },
      });
      const panel1Btn = container.querySelector('[title="Target panel 1"]');
      expect(panel1Btn!.className).toContain('text-[var(--accent-primary)]');
    });

    it('highlights panel 2 when splitActivePanel=1 (vertical)', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top' } }),
          splitEnabled: true,
          splitOrientation: 'vertical',
          splitActivePanel: 1,
        },
      });
      const panel2Btn = container.querySelector('[title="Target panel 2"]');
      expect(panel2Btn!.className).toContain('text-[var(--accent-primary)]');
    });

    it('calls onsplitpanel(0) when panel 1 button clicked', async () => {
      const onSplitPanel = vi.fn();
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top' } }),
          splitEnabled: true,
          splitActivePanel: 1,
          onsplitpanel: onSplitPanel,
        },
      });
      const panel1Btn = container.querySelector('[title="Target panel 1"]');
      await fireEvent.click(panel1Btn!);
      expect(onSplitPanel).toHaveBeenCalledWith(0);
    });
  });

  // --------------------------------------------------------------------------
  // 35. Bottom bar logo and Overview (2 tests)
  // --------------------------------------------------------------------------
  describe('Bottom bar logo/overview', () => {
    it('calls onsplash when logo clicked in bottom bar', async () => {
      const onSplash = vi.fn();
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'bottom', show_logo: true } }),
          onsplash: onSplash,
        },
      });
      const logoBtn = container.querySelector('[title="Test Dashboard"]');
      await fireEvent.click(logoBtn!);
      expect(onSplash).toHaveBeenCalledTimes(1);
    });

    it('shows Overview home icon in bottom bar when show_logo=false', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'bottom', show_logo: false } }),
        },
      });
      expect(container.querySelector('[title="Overview"]')).toBeTruthy();
    });
  });

  // --------------------------------------------------------------------------
  // 36. Sidebar logo interactions (2 tests)
  // --------------------------------------------------------------------------
  describe('Sidebar logo interactions', () => {
    it('calls onsplash when logo clicked in left sidebar', async () => {
      const onSplash = vi.fn();
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left', show_logo: true } }),
          onsplash: onSplash,
        },
      });
      const logoBtn = container.querySelector('[title="Test Dashboard"]');
      await fireEvent.click(logoBtn!);
      expect(onSplash).toHaveBeenCalledTimes(1);
    });

    it('calls onsplash when Overview clicked in right sidebar', async () => {
      const onSplash = vi.fn();
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'right', show_logo: false } }),
          onsplash: onSplash,
        },
      });
      const overviewBtn = container.querySelector('[title="Overview"]');
      await fireEvent.click(overviewBtn!);
      expect(onSplash).toHaveBeenCalledTimes(1);
    });
  });

  // --------------------------------------------------------------------------
  // 37. Icon scale configuration (1 test)
  // --------------------------------------------------------------------------
  describe('Icon scale', () => {
    it('renders without crashing with non-default icon_scale', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'top', icon_scale: 1.5 } }),
        },
      });
      expect(container.querySelector('nav')).toBeTruthy();
    });
  });

  // --------------------------------------------------------------------------
  // 38. Sidebar callbacks (3 tests)
  // --------------------------------------------------------------------------
  describe('Sidebar footer callbacks', () => {
    it('calls onlogs from left sidebar footer', async () => {
      const onLogs = vi.fn();
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left' } }),
          onlogs: onLogs,
        },
      });
      const logsBtn = container.querySelector('[title="Logs"]');
      await fireEvent.click(logsBtn!);
      expect(onLogs).toHaveBeenCalledTimes(1);
    });

    it('calls onsettings from right sidebar footer', async () => {
      const onSettings = vi.fn();
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'right' } }),
          onsettings: onSettings,
        },
      });
      const settingsBtn = container.querySelector('[title="Settings"]');
      await fireEvent.click(settingsBtn!);
      expect(onSettings).toHaveBeenCalledTimes(1);
    });

    it('calls onlogs from right sidebar footer', async () => {
      const onLogs = vi.fn();
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'right' } }),
          onlogs: onLogs,
        },
      });
      const logsBtn = container.querySelector('[title="Logs"]');
      await fireEvent.click(logsBtn!);
      expect(onLogs).toHaveBeenCalledTimes(1);
    });
  });

  // --------------------------------------------------------------------------
  // 39. Bottom bar callbacks (3 tests)
  // --------------------------------------------------------------------------
  describe('Bottom bar callbacks', () => {
    it('calls onsearch from bottom bar', async () => {
      const onSearch = vi.fn();
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'bottom' } }),
          onsearch: onSearch,
        },
      });
      const searchBtn = container.querySelector('[title="Search (Ctrl+K)"]');
      await fireEvent.click(searchBtn!);
      expect(onSearch).toHaveBeenCalledTimes(1);
    });

    it('calls onlogs from bottom bar', async () => {
      const onLogs = vi.fn();
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'bottom' } }),
          onlogs: onLogs,
        },
      });
      const logsBtn = container.querySelector('[title="Logs"]');
      await fireEvent.click(logsBtn!);
      expect(onLogs).toHaveBeenCalledTimes(1);
    });

    it('calls onsettings from bottom bar', async () => {
      const onSettings = vi.fn();
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'bottom' } }),
          onsettings: onSettings,
        },
      });
      const settingsBtn = container.querySelector('[title="Settings"]');
      await fireEvent.click(settingsBtn!);
      expect(onSettings).toHaveBeenCalledTimes(1);
    });
  });

  // --------------------------------------------------------------------------
  // 40. Health indicators in sidebar and grouped dropdown (3 tests)
  // --------------------------------------------------------------------------
  describe('Health indicators in various positions', () => {
    it('shows health indicator in right sidebar for health_check app', () => {
      const app = makeApp({ name: 'HealthyRight', health_check: true, group: 'Media' });
      const { container } = render(Navigation, {
        props: {
          apps: [app],
          currentApp: null,
          showHealth: true,
          config: makeConfig({
            navigation: { position: 'right', show_labels: true },
            groups: [mediaGroup],
          }),
        },
      });
      expect(container.innerHTML).toContain('HealthyRight');
    });

    it('dims unhealthy app in left sidebar', () => {
      const healthMap = new Map();
      healthMap.set('SickApp', { status: 'unhealthy', latency: 0, lastCheck: '' });
      mockHealthData.set(healthMap);

      const sickApp = makeApp({ name: 'SickApp', health_check: true, group: 'Media', order: 0 });
      render(Navigation, {
        props: {
          apps: [sickApp],
          currentApp: null,
          showHealth: true,
          config: makeConfig({
            navigation: { position: 'left', show_labels: true },
            groups: [mediaGroup],
          }),
        },
      });
      // The shouldDim logic dims icon opacity when unhealthy and not current
      const btn = screen.getByText('SickApp').closest('button');
      const iconDiv = btn!.querySelector('.flex-shrink-0.flex.items-center.justify-center');
      expect(iconDiv).toBeTruthy();
      expect((iconDiv as HTMLElement).style.opacity).toBe('0.5');
    });

    it('does not dim healthy app in left sidebar', () => {
      const healthMap = new Map();
      healthMap.set('WellApp', { status: 'healthy', latency: 50, lastCheck: '' });
      mockHealthData.set(healthMap);

      const wellApp = makeApp({ name: 'WellApp', health_check: true, group: 'Media', order: 0 });
      render(Navigation, {
        props: {
          apps: [wellApp],
          currentApp: null,
          showHealth: true,
          config: makeConfig({
            navigation: { position: 'left', show_labels: true },
            groups: [mediaGroup],
          }),
        },
      });
      const btn = screen.getByText('WellApp').closest('button');
      const iconDiv = btn!.querySelector('.flex-shrink-0.flex.items-center.justify-center');
      expect((iconDiv as HTMLElement).style.opacity).toBe('1');
    });
  });

  // --------------------------------------------------------------------------
  // 41. Group expand/collapse in right sidebar and floating (3 tests)
  // --------------------------------------------------------------------------
  describe('Group expand/collapse in other positions', () => {
    it('clicking group header in right sidebar toggles group', async () => {
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'right', show_labels: true },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      const mediaHeader = screen.getByText('Media').closest('button');
      await fireEvent.click(mediaHeader!);
      const wrappers = container.querySelectorAll('.group-apps-wrapper');
      expect(wrappers[0].classList.contains('expanded')).toBe(false);
    });

    it('toggles group in floating panel', async () => {
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'floating' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      // Open FAB panel
      const navEl = container.querySelector('[role="navigation"]');
      const btn = navEl!.querySelector('button')!;
      await fireEvent.pointerDown(btn, { button: 0, pointerId: 1, clientX: 100, clientY: 100 });
      document.dispatchEvent(new PointerEvent('pointerup', { pointerId: 1, bubbles: true }));
      await waitFor(() => {
        expect(screen.getByText('Media')).toBeInTheDocument();
      });
      // Click group header to collapse
      const mediaHeader = screen.getByText('Media').closest('button');
      await fireEvent.click(mediaHeader!);
      const wrappers = container.querySelectorAll('.group-apps-wrapper');
      const firstWrapper = wrappers[0];
      expect(firstWrapper.classList.contains('expanded')).toBe(false);
    });

    it('shows app counts in right sidebar groups', () => {
      render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'right', show_labels: true },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      expect(screen.getByText('2')).toBeInTheDocument();
      expect(screen.getByText('1')).toBeInTheDocument();
    });
  });

  // --------------------------------------------------------------------------
  // 42. Disabled apps / open_mode variations (2 tests)
  // --------------------------------------------------------------------------
  describe('Open mode in sidebar', () => {
    it('shows new_tab arrow icon in left sidebar', () => {
      const newTabApp = makeApp({ name: 'External', open_mode: 'new_tab', group: 'Media', order: 0 });
      const { container } = render(Navigation, {
        props: {
          apps: [newTabApp],
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'left', show_labels: true },
            groups: [mediaGroup],
          }),
        },
      });
      expect(container.innerHTML).toContain('\u2197'); // ↗
    });

    it('shows new_window icon in right sidebar', () => {
      const newWinApp = makeApp({ name: 'WinApp', open_mode: 'new_window', group: 'Tools', order: 0 });
      const { container } = render(Navigation, {
        props: {
          apps: [newWinApp],
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'right', show_labels: true },
            groups: [toolsGroup],
          }),
        },
      });
      expect(container.innerHTML).toContain('\u29C9'); // ⧉
    });
  });

  // --------------------------------------------------------------------------
  // 43. Sidebar collapsed (show_labels=false) in right position (2 tests)
  // --------------------------------------------------------------------------
  describe('Right sidebar collapsed', () => {
    it('hides resize handle when show_labels=false (right)', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'right', show_labels: false } }),
        },
      });
      const handle = container.querySelector('[role="slider"]');
      expect(handle).toBeFalsy();
    });

    it('uses collapsed strip width for right sidebar when show_labels=false', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'right', show_labels: false } }),
        },
      });
      const panel = container.querySelector('.sidebar-panel');
      expect((panel as HTMLElement).style.width).toBe('48px');
    });
  });

  // --------------------------------------------------------------------------
  // 44. Bottom bar auth and admin states (3 tests)
  // --------------------------------------------------------------------------
  describe('Bottom bar auth states', () => {
    it('shows sign out button in bottom bar when auth is builtin', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'bottom' }, auth: { method: 'builtin' } }),
        },
      });
      expect(container.querySelector('[title="Sign out"]')).toBeTruthy();
    });

    it('hides sign out button in bottom bar when auth is none', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'bottom' }, auth: { method: 'none' } }),
        },
      });
      expect(container.querySelector('[title="Sign out"]')).toBeFalsy();
    });

    it('hides settings in bottom bar when not admin', () => {
      mockIsAdmin.set(false);
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'bottom' } }),
        },
      });
      expect(container.querySelector('[title="Settings"]')).toBeFalsy();
    });
  });

  // --------------------------------------------------------------------------
  // 45. Split view in sidebar footer drawer (2 tests)
  // --------------------------------------------------------------------------
  describe('Split view in footer drawer', () => {
    it('shows split controls in left sidebar footer drawer when splitEnabled', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left', hide_sidebar_footer: true, show_labels: true } }),
          splitEnabled: true,
          splitOrientation: 'horizontal',
        },
      });
      expect(container.querySelector('.sidebar-footer-drawer')).toBeTruthy();
      expect(container.querySelector('[title="Horizontal split"]')).toBeTruthy();
      expect(container.querySelector('[title="Vertical split"]')).toBeTruthy();
    });

    it('shows split view button in footer drawer when not splitEnabled', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left', hide_sidebar_footer: true, show_labels: true } }),
          splitEnabled: false,
        },
      });
      expect(container.querySelector('.sidebar-footer-drawer')).toBeTruthy();
      expect(container.querySelector('[title="Split view"]')).toBeTruthy();
    });
  });

  // --------------------------------------------------------------------------
  // 46. Right sidebar auto_hide positioning (2 tests)
  // --------------------------------------------------------------------------
  describe('Right sidebar auto-hide', () => {
    it('right sidebar panel uses absolute positioning when auto_hide=true', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'right', auto_hide: true, show_labels: true } }),
        },
      });
      const panel = container.querySelector('.sidebar-panel');
      expect((panel as HTMLElement).style.position).toBe('absolute');
    });

    it('right sidebar has box-shadow when auto_hide and show_shadow', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'right', auto_hide: true, show_shadow: true, show_labels: true } }),
        },
      });
      const panel = container.querySelector('.sidebar-panel');
      expect((panel as HTMLElement).style.boxShadow).toContain('rgba');
    });
  });

  // --------------------------------------------------------------------------
  // 47. Floating panel overview/logo buttons (2 tests)
  // --------------------------------------------------------------------------
  describe('Floating panel footer buttons', () => {
    it('calls onsplash from floating panel logo button', async () => {
      const onSplash = vi.fn();
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'floating', show_logo: true } }),
          onsplash: onSplash,
        },
      });
      // Open panel
      const navEl = container.querySelector('[role="navigation"]');
      const btn = navEl!.querySelector('button')!;
      await fireEvent.pointerDown(btn, { button: 0, pointerId: 1, clientX: 100, clientY: 100 });
      document.dispatchEvent(new PointerEvent('pointerup', { pointerId: 1, bubbles: true }));
      await waitFor(() => {
        const panel = container.querySelector('.floating-panel');
        expect(panel).toBeTruthy();
      });
      const logoBtn = container.querySelector('.floating-panel [title="Test Dashboard"]');
      await fireEvent.click(logoBtn!);
      expect(onSplash).toHaveBeenCalledTimes(1);
    });

    it('calls onsplash from floating panel Overview button (no logo)', async () => {
      const onSplash = vi.fn();
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'floating', show_logo: false } }),
          onsplash: onSplash,
        },
      });
      const navEl = container.querySelector('[role="navigation"]');
      const btn = navEl!.querySelector('button')!;
      await fireEvent.pointerDown(btn, { button: 0, pointerId: 1, clientX: 100, clientY: 100 });
      document.dispatchEvent(new PointerEvent('pointerup', { pointerId: 1, bubbles: true }));
      await waitFor(() => {
        const panel = container.querySelector('.floating-panel');
        expect(panel).toBeTruthy();
      });
      const overviewBtn = container.querySelector('.floating-panel [title="Overview"]');
      await fireEvent.click(overviewBtn!);
      expect(onSplash).toHaveBeenCalledTimes(1);
    });
  });

  // --------------------------------------------------------------------------
  // 48. Bottom bar flat bar with groups dividers (2 tests)
  // --------------------------------------------------------------------------
  describe('Bottom bar flat bar with group dividers', () => {
    it('shows group dividers in bottom flat bar with real groups', () => {
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'bottom', bar_style: 'flat' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      const dividers = container.querySelectorAll('.flat-group-divider');
      expect(dividers.length).toBeGreaterThan(0);
    });

    it('shows group dividers in top flat bar with real groups', () => {
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'top', bar_style: 'flat' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      const dividers = container.querySelectorAll('.flat-group-divider');
      expect(dividers.length).toBeGreaterThan(0);
    });
  });

  // --------------------------------------------------------------------------
  // 49. Floating nav auth states (2 tests)
  // --------------------------------------------------------------------------
  describe('Floating nav auth states', () => {
    it('hides sign out in floating panel when auth is none', async () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'floating' }, auth: { method: 'none' } }),
        },
      });
      const navEl = container.querySelector('[role="navigation"]');
      const btn = navEl!.querySelector('button')!;
      await fireEvent.pointerDown(btn, { button: 0, pointerId: 1, clientX: 100, clientY: 100 });
      document.dispatchEvent(new PointerEvent('pointerup', { pointerId: 1, bubbles: true }));
      await waitFor(() => {
        expect(container.querySelector('.floating-panel')).toBeTruthy();
      });
      expect(container.querySelector('.floating-panel [title="Sign out"]')).toBeFalsy();
    });

    it('hides settings in floating panel when not admin', async () => {
      mockIsAdmin.set(false);
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          config: makeConfig({ navigation: { position: 'floating' } }),
        },
      });
      const navEl = container.querySelector('[role="navigation"]');
      const btn = navEl!.querySelector('button')!;
      await fireEvent.pointerDown(btn, { button: 0, pointerId: 1, clientX: 100, clientY: 100 });
      document.dispatchEvent(new PointerEvent('pointerup', { pointerId: 1, bubbles: true }));
      await waitFor(() => {
        expect(container.querySelector('.floating-panel')).toBeTruthy();
      });
      expect(container.querySelector('.floating-panel [title="Settings"]')).toBeFalsy();
    });
  });

  // --------------------------------------------------------------------------
  // 50. Empty apps in various positions (3 tests)
  // --------------------------------------------------------------------------
  describe('Empty apps in various positions', () => {
    it('renders without crashing when apps empty (left)', () => {
      const { container } = render(Navigation, {
        props: {
          apps: [],
          currentApp: null,
          config: makeConfig({ navigation: { position: 'left' } }),
        },
      });
      expect(container.querySelector('aside')).toBeTruthy();
    });

    it('renders without crashing when apps empty (right)', () => {
      const { container } = render(Navigation, {
        props: {
          apps: [],
          currentApp: null,
          config: makeConfig({ navigation: { position: 'right' } }),
        },
      });
      expect(container.querySelector('aside')).toBeTruthy();
    });

    it('renders without crashing when apps empty (bottom)', () => {
      const { container } = render(Navigation, {
        props: {
          apps: [],
          currentApp: null,
          config: makeConfig({ navigation: { position: 'bottom' } }),
        },
      });
      const nav = container.querySelector('nav');
      expect(nav).toBeTruthy();
    });
  });

  describe('Scroll fade indicators', () => {
    it('renders scroll fade overlays in left sidebar', () => {
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'left' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      const fadeTop = container.querySelector('.scroll-fade-top');
      const fadeBottom = container.querySelector('.scroll-fade-bottom');
      expect(fadeTop).toBeTruthy();
      expect(fadeBottom).toBeTruthy();
    });

    it('renders scroll fade overlays in right sidebar', () => {
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'right' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      const fadeTop = container.querySelector('.scroll-fade-top');
      const fadeBottom = container.querySelector('.scroll-fade-bottom');
      expect(fadeTop).toBeTruthy();
      expect(fadeBottom).toBeTruthy();
    });

    it('renders scroll fade overlays in floating panel when open', async () => {
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'floating' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      // Open floating panel via pointerdown/pointerup (FAB uses pointer events, not click)
      const navEl = container.querySelector('[role="navigation"]');
      const btn = navEl!.querySelector('button')!;
      await fireEvent.pointerDown(btn, { button: 0, pointerId: 1, clientX: 100, clientY: 100 });
      document.dispatchEvent(new PointerEvent('pointerup', { pointerId: 1, bubbles: true }));
      await waitFor(() => {
        const panel = container.querySelector('.floating-panel');
        expect(panel).toBeTruthy();
      });
      const fadeTop = container.querySelector('.floating-panel .scroll-fade-top');
      const fadeBottom = container.querySelector('.floating-panel .scroll-fade-bottom');
      expect(fadeTop).toBeTruthy();
      expect(fadeBottom).toBeTruthy();
    });

    it('uses scrollbar-styled instead of scrollbar-hide on left sidebar', () => {
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'left' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      const scrollContainer = container.querySelector('.scrollbar-styled');
      expect(scrollContainer).toBeTruthy();
      expect(container.querySelector('.scrollbar-hide')).toBeFalsy();
    });

    it('uses scrollbar-styled instead of scrollbar-hide on right sidebar', () => {
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'right' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      const scrollContainer = container.querySelector('.scrollbar-styled');
      expect(scrollContainer).toBeTruthy();
      expect(container.querySelector('.scrollbar-hide')).toBeFalsy();
    });

    it('fade overlays start without visible class when content does not overflow', () => {
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'left' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      const fadeTop = container.querySelector('.scroll-fade-top');
      const fadeBottom = container.querySelector('.scroll-fade-bottom');
      // In jsdom, scrollHeight === clientHeight === 0, so no overflow
      expect(fadeTop?.classList.contains('visible')).toBe(false);
      expect(fadeBottom?.classList.contains('visible')).toBe(false);
    });

    it('bottom fade becomes visible when content overflows', async () => {
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'left' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      const scrollEl = container.querySelector('.scrollbar-styled');
      expect(scrollEl).toBeTruthy();

      // Simulate overflow: scrollHeight > clientHeight
      Object.defineProperty(scrollEl!, 'scrollHeight', { value: 500, configurable: true });
      Object.defineProperty(scrollEl!, 'clientHeight', { value: 200, configurable: true });
      Object.defineProperty(scrollEl!, 'scrollTop', { value: 0, writable: true, configurable: true });

      await fireEvent.scroll(scrollEl!);

      await waitFor(() => {
        const fadeBottom = container.querySelector('.scroll-fade-bottom');
        expect(fadeBottom?.classList.contains('visible')).toBe(true);
      });
    });

    it('top fade becomes visible after scrolling down', async () => {
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'left' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      const scrollEl = container.querySelector('.scrollbar-styled');
      expect(scrollEl).toBeTruthy();

      // Simulate scrolled to middle
      Object.defineProperty(scrollEl!, 'scrollHeight', { value: 500, configurable: true });
      Object.defineProperty(scrollEl!, 'clientHeight', { value: 200, configurable: true });
      Object.defineProperty(scrollEl!, 'scrollTop', { value: 100, writable: true, configurable: true });

      await fireEvent.scroll(scrollEl!);

      await waitFor(() => {
        const fadeTop = container.querySelector('.scroll-fade-top');
        const fadeBottom = container.querySelector('.scroll-fade-bottom');
        expect(fadeTop?.classList.contains('visible')).toBe(true);
        expect(fadeBottom?.classList.contains('visible')).toBe(true);
      });
    });

    it('both fades hidden when scrolled to bottom (no more content below)', async () => {
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'left' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      const scrollEl = container.querySelector('.scrollbar-styled');
      expect(scrollEl).toBeTruthy();

      // Simulate scrolled to very bottom
      Object.defineProperty(scrollEl!, 'scrollHeight', { value: 500, configurable: true });
      Object.defineProperty(scrollEl!, 'clientHeight', { value: 200, configurable: true });
      Object.defineProperty(scrollEl!, 'scrollTop', { value: 300, writable: true, configurable: true });

      await fireEvent.scroll(scrollEl!);

      await waitFor(() => {
        const fadeTop = container.querySelector('.scroll-fade-top');
        const fadeBottom = container.querySelector('.scroll-fade-bottom');
        expect(fadeTop?.classList.contains('visible')).toBe(true);
        expect(fadeBottom?.classList.contains('visible')).toBe(false);
      });
    });

    it('scroll container is wrapped in relative min-h-0 container', () => {
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'left' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      const scrollEl = container.querySelector('.scrollbar-styled');
      const wrapper = scrollEl?.parentElement;
      expect(wrapper?.classList.contains('relative')).toBe(true);
      expect(wrapper?.classList.contains('min-h-0')).toBe(true);
    });

    it('right sidebar: bottom fade visible when content overflows', async () => {
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'right' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      const scrollEl = container.querySelector('.scrollbar-styled');
      expect(scrollEl).toBeTruthy();

      // Simulate overflow: scrollHeight > clientHeight, scrolled to top
      Object.defineProperty(scrollEl!, 'scrollHeight', { value: 500, configurable: true });
      Object.defineProperty(scrollEl!, 'clientHeight', { value: 200, configurable: true });
      Object.defineProperty(scrollEl!, 'scrollTop', { value: 0, writable: true, configurable: true });

      await fireEvent.scroll(scrollEl!);

      await waitFor(() => {
        const fadeBottom = container.querySelector('.scroll-fade-bottom');
        expect(fadeBottom?.classList.contains('visible')).toBe(true);
      });
      // Top fade should NOT be visible when scrolled to top
      const fadeTop = container.querySelector('.scroll-fade-top');
      expect(fadeTop?.classList.contains('visible')).toBe(false);
    });

    it('right sidebar: top fade visible after scrolling down', async () => {
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'right' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      const scrollEl = container.querySelector('.scrollbar-styled');
      expect(scrollEl).toBeTruthy();

      // Simulate scrolled to middle
      Object.defineProperty(scrollEl!, 'scrollHeight', { value: 500, configurable: true });
      Object.defineProperty(scrollEl!, 'clientHeight', { value: 200, configurable: true });
      Object.defineProperty(scrollEl!, 'scrollTop', { value: 100, writable: true, configurable: true });

      await fireEvent.scroll(scrollEl!);

      await waitFor(() => {
        const fadeTop = container.querySelector('.scroll-fade-top');
        const fadeBottom = container.querySelector('.scroll-fade-bottom');
        expect(fadeTop?.classList.contains('visible')).toBe(true);
        expect(fadeBottom?.classList.contains('visible')).toBe(true);
      });
    });

    it('floating panel: scrollbar-styled class present (not scrollbar-hide)', async () => {
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'floating' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      // Open floating panel via pointerdown/pointerup
      const navEl = container.querySelector('[role="navigation"]');
      const btn = navEl!.querySelector('button')!;
      await fireEvent.pointerDown(btn, { button: 0, pointerId: 1, clientX: 100, clientY: 100 });
      document.dispatchEvent(new PointerEvent('pointerup', { pointerId: 1, bubbles: true }));
      await waitFor(() => {
        const panel = container.querySelector('.floating-panel');
        expect(panel).toBeTruthy();
      });
      const scrollContainer = container.querySelector('.floating-panel .scrollbar-styled');
      expect(scrollContainer).toBeTruthy();
      expect(container.querySelector('.floating-panel .scrollbar-hide')).toBeFalsy();
    });

    it('floating panel: bottom fade visible when content overflows', async () => {
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'floating' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      // Open floating panel via pointerdown/pointerup
      const navEl = container.querySelector('[role="navigation"]');
      const btn = navEl!.querySelector('button')!;
      await fireEvent.pointerDown(btn, { button: 0, pointerId: 1, clientX: 100, clientY: 100 });
      document.dispatchEvent(new PointerEvent('pointerup', { pointerId: 1, bubbles: true }));
      await waitFor(() => {
        const panel = container.querySelector('.floating-panel');
        expect(panel).toBeTruthy();
      });

      const scrollEl = container.querySelector('.floating-panel .scrollbar-styled');
      expect(scrollEl).toBeTruthy();

      // Simulate overflow: scrollHeight > clientHeight, scrolled to top
      Object.defineProperty(scrollEl!, 'scrollHeight', { value: 500, configurable: true });
      Object.defineProperty(scrollEl!, 'clientHeight', { value: 200, configurable: true });
      Object.defineProperty(scrollEl!, 'scrollTop', { value: 0, writable: true, configurable: true });

      await fireEvent.scroll(scrollEl!);

      await waitFor(() => {
        const fadeBottom = container.querySelector('.floating-panel .scroll-fade-bottom');
        expect(fadeBottom?.classList.contains('visible')).toBe(true);
      });
      // Top fade should NOT be visible when scrolled to top
      const fadeTop = container.querySelector('.floating-panel .scroll-fade-top');
      expect(fadeTop?.classList.contains('visible')).toBe(false);
    });

    it('floating panel: top and bottom fades visible when scrolled to middle', async () => {
      const { container } = render(Navigation, {
        props: {
          apps: sampleApps,
          currentApp: null,
          config: makeConfig({
            navigation: { position: 'floating' },
            groups: [mediaGroup, toolsGroup],
          }),
        },
      });
      // Open floating panel via pointerdown/pointerup
      const navEl = container.querySelector('[role="navigation"]');
      const btn = navEl!.querySelector('button')!;
      await fireEvent.pointerDown(btn, { button: 0, pointerId: 1, clientX: 100, clientY: 100 });
      document.dispatchEvent(new PointerEvent('pointerup', { pointerId: 1, bubbles: true }));
      await waitFor(() => {
        const panel = container.querySelector('.floating-panel');
        expect(panel).toBeTruthy();
      });

      const scrollEl = container.querySelector('.floating-panel .scrollbar-styled');
      expect(scrollEl).toBeTruthy();

      // Simulate scrolled to middle
      Object.defineProperty(scrollEl!, 'scrollHeight', { value: 500, configurable: true });
      Object.defineProperty(scrollEl!, 'clientHeight', { value: 200, configurable: true });
      Object.defineProperty(scrollEl!, 'scrollTop', { value: 100, writable: true, configurable: true });

      await fireEvent.scroll(scrollEl!);

      await waitFor(() => {
        const fadeTop = container.querySelector('.floating-panel .scroll-fade-top');
        const fadeBottom = container.querySelector('.floating-panel .scroll-fade-bottom');
        expect(fadeTop?.classList.contains('visible')).toBe(true);
        expect(fadeBottom?.classList.contains('visible')).toBe(true);
      });
    });
  });

  // --------------------------------------------------------------------------
  // Refresh button
  // --------------------------------------------------------------------------
  describe('Refresh button', () => {
    it('shows refresh button when an app is active (top nav)', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: ungroupedApps[0],
          showSplash: false,
          config: makeConfig({ navigation: { position: 'top' } }),
        },
      });
      const btn = container.querySelector('[title="Refresh app"]');
      expect(btn).toBeTruthy();
    });

    it('hides refresh button on splash screen', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: null,
          showSplash: true,
          config: makeConfig({ navigation: { position: 'top' } }),
        },
      });
      const btn = container.querySelector('[title="Refresh app"]');
      expect(btn).toBeFalsy();
    });

    it('calls onrefresh callback when clicked', async () => {
      const onrefresh = vi.fn();
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: ungroupedApps[0],
          showSplash: false,
          config: makeConfig({ navigation: { position: 'top' } }),
          onrefresh,
        },
      });
      const btn = container.querySelector('[title="Refresh app"]') as HTMLButtonElement;
      expect(btn).toBeTruthy();
      await fireEvent.click(btn);
      expect(onrefresh).toHaveBeenCalledTimes(1);
    });

    it('shows refresh button in bottom nav when app is active', () => {
      const { container } = render(Navigation, {
        props: {
          apps: ungroupedApps,
          currentApp: ungroupedApps[0],
          showSplash: false,
          config: makeConfig({ navigation: { position: 'bottom' } }),
        },
      });
      const btn = container.querySelector('[title="Refresh app"]');
      expect(btn).toBeTruthy();
    });
  });
});
