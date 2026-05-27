import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import { writable } from 'svelte/store';
import type { Config, App } from '$lib/types';

vi.mock('$lib/healthStore', () => ({
  healthData: writable(new Map()),
}));
vi.mock('$lib/api', () => ({
  triggerHealthCheck: vi.fn(),
  getBase: vi.fn().mockReturnValue(''),
  dockerStart: vi.fn().mockResolvedValue({ status: 'running', latency_ms: 12 }),
  dockerStop: vi.fn().mockResolvedValue({ status: 'exited', latency_ms: 12 }),
  dockerRestart: vi.fn().mockResolvedValue({ status: 'running', latency_ms: 12 }),
}));
vi.mock('$lib/debug', () => ({
  debug: vi.fn(),
}));
vi.mock('$lib/toastStore', () => ({
  toasts: { error: vi.fn(), success: vi.fn(), warning: vi.fn(), info: vi.fn() },
}));

import Splash from './Splash.svelte';
import { dockerStateStore } from '$lib/dockerStateStore';
import { authState } from '$lib/authStore';

function makeConfig(overrides: Partial<Config> = {}): Config {
  return {
    title: 'Test',
    navigation: {
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
      show_shadow: true,
      bar_style: 'grouped',
      floating_position: 'bottom-right',
      hide_sidebar_footer: false,
      max_open_tabs: 0,
    },
    groups: [{ name: 'Media', order: 0, icon: { type: 'lucide', name: 'play', file: '', url: '', variant: '', }, color: '', expanded: true }],
    apps: [],
    ...overrides,
  };
}

function makeApp(overrides: Partial<App> = {}): App {
  return {
    name: 'Test App',
    url: 'http://localhost',
    icon: { type: 'lucide', name: 'home', file: '', url: '', variant: '' },
    color: '#3b82f6',
    group: 'Media',
    proxy: false,
    open_mode: 'iframe',
    enabled: true,
    default: false,
    order: 0,
    health_check: false,
    scale: 1,
    ...overrides,
  };
}

describe('Splash', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders empty state when no apps', () => {
    render(Splash, { props: { apps: [], config: makeConfig() } });
    expect(screen.getByText('No applications yet')).toBeInTheDocument();
    expect(screen.getByText(/Add your first application/)).toBeInTheDocument();
  });

  it('shows "Add Application" button in empty state when onsettings provided', () => {
    const onsettings = vi.fn();
    render(Splash, { props: { apps: [], config: makeConfig(), onsettings } });
    expect(screen.getByRole('button', { name: /Add Application/ })).toBeInTheDocument();
  });

  it('does not show "Add Application" button when onsettings not provided', () => {
    render(Splash, { props: { apps: [], config: makeConfig() } });
    expect(screen.queryByRole('button', { name: /Add Application/ })).not.toBeInTheDocument();
  });

  it('renders app cards for each app', () => {
    const apps = [
      makeApp({ name: 'Sonarr', order: 0 }),
      makeApp({ name: 'Radarr', order: 1 }),
    ];
    render(Splash, { props: { apps, config: makeConfig() } });
    expect(screen.getByText('Sonarr')).toBeInTheDocument();
    expect(screen.getByText('Radarr')).toBeInTheDocument();
  });

  it('groups apps by their group name', () => {
    const config = makeConfig({
      groups: [
        { name: 'Media', order: 0, icon: { type: 'lucide', name: 'play', file: '', url: '', variant: '' }, color: '', expanded: true },
        { name: 'Tools', order: 1, icon: { type: 'lucide', name: 'wrench', file: '', url: '', variant: '' }, color: '', expanded: true },
      ],
    });
    const apps = [
      makeApp({ name: 'Sonarr', group: 'Media', order: 0 }),
      makeApp({ name: 'Portainer', group: 'Tools', order: 0 }),
    ];
    render(Splash, { props: { apps, config } });

    // Both group headers should appear
    expect(screen.getByText('Media')).toBeInTheDocument();
    expect(screen.getByText('Tools')).toBeInTheDocument();
  });

  it('calls onselect when app card clicked', async () => {
    const onselect = vi.fn();
    const apps = [makeApp({ name: 'Sonarr' })];
    render(Splash, { props: { apps, config: makeConfig(), onselect } });

    const appButton = screen.getByText('Sonarr').closest('button');
    expect(appButton).toBeInTheDocument();
    await fireEvent.click(appButton!);

    expect(onselect).toHaveBeenCalledTimes(1);
    expect(onselect).toHaveBeenCalledWith(expect.objectContaining({ name: 'Sonarr' }));
  });

  it('shows app count in footer', () => {
    const apps = [
      makeApp({ name: 'Sonarr', order: 0 }),
      makeApp({ name: 'Radarr', order: 1 }),
    ];
    const { container } = render(Splash, { props: { apps, config: makeConfig() } });

    // Footer shows the count of active (enabled) apps
    const footer = container.querySelector('footer');
    expect(footer).toBeInTheDocument();
    expect(footer!.textContent).toContain('2');
    expect(footer!.textContent).toContain('active');
  });

  it('shows keyboard hints derived from keybindings store', () => {
    const apps = [makeApp()];
    render(Splash, { props: { apps, config: makeConfig() } });

    // The splash header contains keyboard shortcut hints from the keybindings store
    expect(screen.getByText('Search')).toBeInTheDocument();
    expect(screen.getByText('Ctrl+K')).toBeInTheDocument();
    expect(screen.getByText('All shortcuts')).toBeInTheDocument();
    expect(screen.getByText('?')).toBeInTheDocument();
  });

  it('shows open mode icon for new_tab apps', () => {
    const apps = [makeApp({ name: 'TabApp', open_mode: 'new_tab' })];
    render(Splash, { props: { apps, config: makeConfig() } });
    // new_tab shows arrow icon (↗ = \u2197)
    const appBtn = screen.getByText('TabApp').closest('button');
    expect(appBtn).toBeInTheDocument();
    expect(appBtn!.textContent).toContain('\u2197');
  });

  it('shows open mode icon for new_window apps', () => {
    const apps = [makeApp({ name: 'WinApp', open_mode: 'new_window' })];
    render(Splash, { props: { apps, config: makeConfig() } });
    const appBtn = screen.getByText('WinApp').closest('button');
    expect(appBtn).toBeInTheDocument();
    // new_window shows ⧉ icon (\u29C9)
    expect(appBtn!.textContent).toContain('\u29C9');
  });

  it('does not show open mode icon for iframe apps', () => {
    const apps = [makeApp({ name: 'IframeApp', open_mode: 'iframe' })];
    render(Splash, { props: { apps, config: makeConfig() } });
    const appBtn = screen.getByText('IframeApp').closest('button');
    expect(appBtn!.textContent).not.toContain('\u2197');
    expect(appBtn!.textContent).not.toContain('\u29C9');
  });

  it('shows Settings button in footer when onsettings is provided and apps exist', () => {
    const onsettings = vi.fn();
    const apps = [makeApp({ name: 'Sonarr' })];
    render(Splash, { props: { apps, config: makeConfig(), onsettings } });
    const footer = screen.getByText('Settings');
    expect(footer).toBeInTheDocument();
  });

  it('calls onsettings when Settings footer button is clicked', async () => {
    const onsettings = vi.fn();
    const apps = [makeApp({ name: 'Sonarr' })];
    render(Splash, { props: { apps, config: makeConfig(), onsettings } });
    const settingsBtn = screen.getByText('Settings').closest('button')!;
    await fireEvent.click(settingsBtn);
    expect(onsettings).toHaveBeenCalledTimes(1);
  });

  it('shows About button in footer when apps exist', async () => {
    const onabout = vi.fn();
    const apps = [makeApp({ name: 'Sonarr' })];
    render(Splash, { props: { apps, config: makeConfig(), onabout } });
    const aboutBtn = screen.getByText('About').closest('button')!;
    await fireEvent.click(aboutBtn);
    expect(onabout).toHaveBeenCalledTimes(1);
  });

  it('shows health indicator for apps with health_check enabled', () => {
    const apps = [makeApp({ name: 'HealthApp', health_check: true })];
    const { container } = render(Splash, { props: { apps, config: makeConfig(), showHealth: true } });
    // The health indicator wrapper should be present
    // HealthIndicator is rendered inside a div with specific classes
    const healthDiv = container.querySelector('.absolute.top-2\\.5.end-2\\.5');
    expect(healthDiv).toBeInTheDocument();
  });

  it('does not show health indicator when showHealth is false', () => {
    const apps = [makeApp({ name: 'HealthApp', health_check: true })];
    const { container } = render(Splash, { props: { apps, config: makeConfig(), showHealth: false } });
    const healthDiv = container.querySelector('.absolute.top-2\\.5.end-2\\.5');
    expect(healthDiv).not.toBeInTheDocument();
  });

  it('sorts apps within groups by order', () => {
    const apps = [
      makeApp({ name: 'Second', order: 2 }),
      makeApp({ name: 'First', order: 1 }),
    ];
    const { container } = render(Splash, { props: { apps, config: makeConfig() } });
    const buttons = container.querySelectorAll('button.app-card');
    // First should come before Second due to order sorting
    const names = Array.from(buttons).map(b => b.textContent || '');
    const firstIdx = names.findIndex(n => n.includes('First'));
    const secondIdx = names.findIndex(n => n.includes('Second'));
    expect(firstIdx).toBeLessThan(secondIdx);
  });

  it('puts Ungrouped group last', () => {
    const config = makeConfig({
      groups: [
        { name: 'Media', order: 0, icon: { type: 'lucide', name: 'play', file: '', url: '', variant: '' }, color: '', expanded: true },
      ],
    });
    const apps = [
      makeApp({ name: 'Grouped', group: 'Media', order: 0 }),
      makeApp({ name: 'Loose', group: 'Ungrouped', order: 0 }),
    ];
    const { container } = render(Splash, { props: { apps, config } });
    const sections = container.querySelectorAll('section');
    // Media section should come before Ungrouped section
    expect(sections.length).toBe(2);
  });

  it('shows group count text (singular)', () => {
    const apps = [makeApp({ name: 'Sonarr' })];
    const { container } = render(Splash, { props: { apps, config: makeConfig() } });
    const footer = container.querySelector('footer');
    expect(footer!.textContent).toContain('1');
    expect(footer!.textContent).toContain('group');
  });

  // -------------------------------------------------------------------------
  // Keyboard shortcut badge display
  // -------------------------------------------------------------------------
  describe('Keyboard shortcut badges', () => {
    it('shows badge only for apps with explicit shortcut', () => {
      const apps = [
        makeApp({ name: 'First', order: 0 }),
        makeApp({ name: 'Second', order: 1, shortcut: 3 }),
      ];
      const { container } = render(Splash, { props: { apps, config: makeConfig() } });
      const cards = container.querySelectorAll('.app-card');
      // First has no shortcut — no badge
      expect(cards[0].querySelector('.kbd')).toBeNull();
      // Second has explicit shortcut 3
      expect(cards[1].querySelector('.kbd')?.textContent?.trim()).toBe('3');
    });

    it('does not show badge for apps without shortcut', () => {
      const apps = [
        makeApp({ name: 'NoShortcut', order: 0 }),
      ];
      const { container } = render(Splash, { props: { apps, config: makeConfig() } });
      const badge = container.querySelector('.app-card .kbd');
      expect(badge).toBeNull();
    });

    it('shows correct shortcut numbers for multiple apps', () => {
      const apps = [
        makeApp({ name: 'First', order: 0, shortcut: 1 }),
        makeApp({ name: 'Second', order: 1, shortcut: 2 }),
        makeApp({ name: 'Third', order: 2 }),
      ];
      const { container } = render(Splash, { props: { apps, config: makeConfig() } });
      const cards = container.querySelectorAll('.app-card');
      expect(cards[0].querySelector('.kbd')?.textContent?.trim()).toBe('1');
      expect(cards[1].querySelector('.kbd')?.textContent?.trim()).toBe('2');
      // Third has no shortcut — no badge
      expect(cards[2].querySelector('.kbd')).toBeNull();
    });
  });
});

describe('Splash Docker integration', () => {
  beforeEach(() => {
    dockerStateStore.set(new Map());
    authState.set({ authenticated: true, user: { username: 'erik', role: 'admin', can_use_docker_lifecycle: true }, loading: false, error: null, setupRequired: false, logoutUrl: null });
  });

  it('renders the Docker status cluster (logo) for docker_key apps', () => {
    const apps = [{ name: 'sonarr', docker_key: 'name:/sonarr', enabled: true, open_mode: 'iframe' } as App];
    const { container } = render(Splash, { props: { apps, config: { groups: [], discovery: { docker: { health_badge_placement: 'overview' } } } as any } });
    // The passive status cluster (logo) shows for every docker app,
    // even before state loads and regardless of lifecycle permission.
    expect(container.querySelector('.docker-cluster svg')).not.toBeNull();
  });

  it('does not render the docker cluster for non-docker apps', () => {
    const apps = [{ name: 'plain', enabled: true, open_mode: 'iframe' } as App];
    const { container } = render(Splash, { props: { apps, config: { groups: [], discovery: { docker: { health_badge_placement: 'overview' } } } as any } });
    // No DockerLogo inside the card top-right cluster.
    const card = container.querySelector('.app-card');
    // The HealthIndicator path renders its own dot; the docker
    // cluster wrapper has class .docker-cluster.
    expect(card?.querySelector('.docker-cluster')).toBeNull();
  });

  it('does not render the action footer when can_use_docker_lifecycle is false', () => {
    authState.update((s) => ({ ...s, user: { ...s.user!, can_use_docker_lifecycle: false } }));
    dockerStateStore.set(new Map([['sonarr', { status: 'running', health: 'healthy', restart_count: 0, image: 'x' }]]));
    const apps = [{ name: 'sonarr', docker_key: 'name:/sonarr', enabled: true, open_mode: 'iframe' } as App];
    const { container } = render(Splash, { props: { apps, config: { groups: [], discovery: { docker: { health_badge_placement: 'overview' } } } as any } });
    expect(container.querySelector('.docker-control-footer')).toBeNull();
    // The passive status cluster (logo) still shows.
    expect(container.querySelector('.docker-cluster svg')).not.toBeNull();
  });

  it('renders the action footer below the card when can_use_docker_lifecycle is true', () => {
    dockerStateStore.set(new Map([['sonarr', { status: 'running', health: 'healthy', restart_count: 0, image: 'x' }]]));
    const apps = [{ name: 'sonarr', docker_key: 'name:/sonarr', enabled: true, open_mode: 'iframe' } as App];
    const { container } = render(Splash, { props: { apps, config: { groups: [], discovery: { docker: { health_badge_placement: 'overview' } } } as any } });
    const footer = container.querySelector('.docker-control-footer');
    expect(footer).not.toBeNull();
    // The footer is a sibling of the card button, not inside it -- so a tap
    // to open the app can never land on a lifecycle action.
    expect(footer!.closest('.app-card')).toBeNull();
    // Status cluster (logo) still renders too.
    expect(container.querySelector('.docker-cluster svg')).not.toBeNull();
  });

  it('applies grayscale to exited apps', () => {
    dockerStateStore.set(new Map([['sonarr', { status: 'exited', health: 'none', restart_count: 0, image: 'x' }]]));
    const apps = [{ name: 'sonarr', docker_key: 'name:/sonarr', enabled: true, open_mode: 'iframe' } as App];
    const { container } = render(Splash, { props: { apps, config: { groups: [], discovery: { docker: { health_badge_placement: 'overview' } } } as any } });
    expect(container.querySelector('.app-card.exited')).not.toBeNull();
  });

  it('renders state-appropriate action buttons for a running app (stop + restart, no start)', () => {
    dockerStateStore.set(new Map([['sonarr', { status: 'running', health: 'healthy', restart_count: 0, image: 'x' }]]));
    const apps = [{ name: 'sonarr', docker_key: 'name:/sonarr', enabled: true, open_mode: 'iframe' } as App];
    const { container } = render(Splash, { props: { apps, config: { groups: [], discovery: { docker: { health_badge_placement: 'overview' } } } as any } });
    const labels = Array.from(container.querySelectorAll('.docker-action-btn')).map((b) => b.getAttribute('aria-label'));
    expect(labels).toContain('Stop container');
    expect(labels).toContain('Restart container');
    expect(labels).not.toContain('Start container');
  });

  it('opens the confirm modal for stop (does not fire immediately)', async () => {
    const api = await import('$lib/api');
    dockerStateStore.set(new Map([['sonarr', { status: 'running', health: 'healthy', restart_count: 0, image: 'lscr.io/sonarr' }]]));
    const apps = [{ name: 'sonarr', docker_key: 'name:/sonarr', enabled: true, open_mode: 'iframe' } as App];
    const { container } = render(Splash, { props: { apps, config: { groups: [], discovery: { docker: { health_badge_placement: 'overview' } } } as any } });
    const stopBtn = container.querySelector('.docker-action-btn[aria-label="Stop container"]') as HTMLButtonElement;
    await fireEvent.click(stopBtn);
    // Modal is shown, action not yet fired.
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(api.dockerStop).not.toHaveBeenCalled();
  });

  it('fires dockerStart immediately for start (bypasses confirm modal)', async () => {
    const api = await import('$lib/api');
    dockerStateStore.set(new Map([['sonarr', { status: 'exited', health: 'none', restart_count: 0, image: 'x' }]]));
    const apps = [{ name: 'sonarr', docker_key: 'name:/sonarr', enabled: true, open_mode: 'iframe' } as App];
    const { container } = render(Splash, { props: { apps, config: { groups: [], discovery: { docker: { health_badge_placement: 'overview' } } } as any } });
    const startBtn = container.querySelector('.docker-action-btn[aria-label="Start container"]') as HTMLButtonElement;
    await fireEvent.click(startBtn);
    expect(api.dockerStart).toHaveBeenCalledWith('sonarr');
    // No confirm modal for start.
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('clicking Confirm in the stop modal actually fires dockerStop', async () => {
    const api = await import('$lib/api');
    vi.mocked(api.dockerStop).mockClear();
    dockerStateStore.set(new Map([['sonarr', { status: 'running', health: 'healthy', restart_count: 0, image: 'x' }]]));
    const apps = [{ name: 'sonarr', docker_key: 'name:/sonarr', enabled: true, open_mode: 'iframe' } as App];
    const { container } = render(Splash, { props: { apps, config: { groups: [], discovery: { docker: { health_badge_placement: 'overview' } } } as any } });
    await fireEvent.click(container.querySelector('.docker-action-btn[aria-label="Stop container"]') as HTMLButtonElement);
    // Confirm in the modal -> the destructive action must actually fire.
    await fireEvent.click(screen.getByRole('button', { name: /confirm/i }));
    expect(api.dockerStop).toHaveBeenCalledWith('sonarr');
  });

  it('shows an error toast when an action fails', async () => {
    const api = await import('$lib/api');
    const { toasts } = await import('$lib/toastStore');
    vi.mocked(toasts.error).mockClear();
    vi.mocked(api.dockerStart).mockResolvedValueOnce({ error: 'Port already in use', latency_ms: 8 });
    dockerStateStore.set(new Map([['sonarr', { status: 'exited', health: 'none', restart_count: 0, image: 'x' }]]));
    const apps = [{ name: 'sonarr', docker_key: 'name:/sonarr', enabled: true, open_mode: 'iframe' } as App];
    const { container } = render(Splash, { props: { apps, config: { groups: [], discovery: { docker: { health_badge_placement: 'overview' } } } as any } });
    // Start fires immediately (no modal); the failed result must toast.
    await fireEvent.click(container.querySelector('.docker-action-btn[aria-label="Start container"]') as HTMLButtonElement);
    await Promise.resolve();
    expect(toasts.error).toHaveBeenCalled();
  });

  it('hides all Docker chrome on the overview when health_badge_placement is off', () => {
    dockerStateStore.set(new Map([['sonarr', { status: 'running', health: 'healthy', restart_count: 0, image: 'x' }]]));
    const apps = [{ name: 'sonarr', docker_key: 'name:/sonarr', enabled: true, open_mode: 'iframe' } as App];
    const { container } = render(Splash, { props: { apps, config: { groups: [], discovery: { docker: { health_badge_placement: 'off' } } } as any } });
    expect(container.querySelector('.docker-cluster')).toBeNull();
    expect(container.querySelector('.docker-control-footer')).toBeNull();
  });

  it('shows Docker chrome on the overview when health_badge_placement is overview', () => {
    dockerStateStore.set(new Map([['sonarr', { status: 'running', health: 'healthy', restart_count: 0, image: 'x' }]]));
    const apps = [{ name: 'sonarr', docker_key: 'name:/sonarr', enabled: true, open_mode: 'iframe' } as App];
    const { container } = render(Splash, { props: { apps, config: { groups: [], discovery: { docker: { health_badge_placement: 'overview' } } } as any } });
    expect(container.querySelector('.docker-cluster')).not.toBeNull();
    expect(container.querySelector('.docker-control-footer')).not.toBeNull();
  });
});
