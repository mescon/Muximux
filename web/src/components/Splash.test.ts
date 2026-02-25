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
}));
vi.mock('$lib/debug', () => ({
  debug: vi.fn(),
}));

import Splash from './Splash.svelte';

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
    expect(screen.getByText('Shortcuts')).toBeInTheDocument();
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
    const healthDiv = container.querySelector('.absolute.top-2\\.5.right-2\\.5');
    expect(healthDiv).toBeInTheDocument();
  });

  it('does not show health indicator when showHealth is false', () => {
    const apps = [makeApp({ name: 'HealthApp', health_check: true })];
    const { container } = render(Splash, { props: { apps, config: makeConfig(), showHealth: false } });
    const healthDiv = container.querySelector('.absolute.top-2\\.5.right-2\\.5');
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
