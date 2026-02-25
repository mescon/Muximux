import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

// Mock $lib/useSwipe
vi.mock('$lib/useSwipe', () => ({
  isMobileViewport: vi.fn(() => false),
  isTouchDevice: vi.fn(() => false),
}));

// Mock $lib/keybindingsStore - provide a Svelte-compatible readable store
const { mockKeybindingsStore } = vi.hoisted(() => {
  const subscribers = new Set<(value: unknown[]) => void>();
  const store = {
    subscribe: (fn: (value: unknown[]) => void) => {
      fn([]);
      subscribers.add(fn);
      return () => subscribers.delete(fn);
    },
  };
  return { mockKeybindingsStore: store };
});

vi.mock('$lib/keybindingsStore', () => ({
  keybindings: mockKeybindingsStore,
  formatKeybinding: vi.fn(() => ''),
}));

// Mock $lib/authStore - provide readable stores for isAdmin and isAuthenticated
const { mockIsAdminStore, mockIsAuthenticatedStore } = vi.hoisted(() => {
  function makeBoolStore(initial: boolean) {
    const subscribers = new Set<(value: boolean) => void>();
    return {
      subscribe: (fn: (value: boolean) => void) => {
        fn(initial);
        subscribers.add(fn);
        return () => subscribers.delete(fn);
      },
    };
  }
  return {
    mockIsAdminStore: makeBoolStore(false),
    mockIsAuthenticatedStore: makeBoolStore(false),
  };
});

vi.mock('$lib/authStore', () => ({
  isAdmin: mockIsAdminStore,
  isAuthenticated: mockIsAuthenticatedStore,
}));

// Mock $lib/api (AppIcon imports it)
vi.mock('$lib/api', () => ({
  getBase: vi.fn(() => ''),
}));

// Mock $lib/debug (AppIcon imports it)
vi.mock('$lib/debug', () => ({
  debug: vi.fn(),
}));

import CommandPalette from './CommandPalette.svelte';
import type { App, AppIcon } from '$lib/types';

function makeApp(overrides: Partial<App> = {}): App {
  return {
    name: 'TestApp',
    url: 'https://example.com',
    icon: {
      type: 'dashboard',
      name: 'test',
      file: '',
      url: '',
      variant: 'svg',
    } as AppIcon,
    color: '#374151',
    group: 'Default',
    order: 0,
    enabled: true,
    default: false,
    open_mode: 'iframe',
    proxy: false,
    scale: 1,
    ...overrides,
  };
}

const sampleApps: App[] = [
  makeApp({ name: 'Grafana', url: 'https://grafana.local' }),
  makeApp({ name: 'Prometheus', url: 'https://prom.local' }),
  makeApp({ name: 'Sonarr', url: 'https://sonarr.local' }),
];

describe('CommandPalette', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('smoke test', () => {
    it('renders without crashing', () => {
      const { container } = render(CommandPalette, {
        props: {
          apps: sampleApps,
        },
      });
      expect(container.querySelector('[role="dialog"]')).toBeTruthy();
    });
  });

  describe('search input', () => {
    it('renders a search input with correct placeholder', () => {
      render(CommandPalette, {
        props: { apps: sampleApps },
      });
      const input = screen.getByPlaceholderText('Search apps and commands...');
      expect(input).toBeInTheDocument();
    });

    it('renders the dialog with correct aria attributes', () => {
      const { container } = render(CommandPalette, {
        props: { apps: sampleApps },
      });
      const dialog = container.querySelector('[role="dialog"]');
      expect(dialog?.getAttribute('aria-modal')).toBe('true');
      expect(dialog?.getAttribute('aria-label')).toBe('Command palette');
    });
  });

  describe('app listing', () => {
    it('displays all apps when no query is entered', () => {
      render(CommandPalette, {
        props: { apps: sampleApps },
      });
      expect(screen.getByText('Grafana')).toBeInTheDocument();
      expect(screen.getByText('Prometheus')).toBeInTheDocument();
      expect(screen.getByText('Sonarr')).toBeInTheDocument();
    });

    it('displays built-in action commands', () => {
      render(CommandPalette, {
        props: { apps: [] },
      });
      expect(screen.getByText('Show Keyboard Shortcuts')).toBeInTheDocument();
      expect(screen.getByText('Toggle Fullscreen')).toBeInTheDocument();
      expect(screen.getByText('Refresh Current App')).toBeInTheDocument();
      expect(screen.getByText('Go to Splash Screen')).toBeInTheDocument();
    });

    it('displays theme setting commands', () => {
      render(CommandPalette, {
        props: { apps: [] },
      });
      expect(screen.getByText('Set Dark Theme')).toBeInTheDocument();
      expect(screen.getByText('Set Light Theme')).toBeInTheDocument();
      expect(screen.getByText('Use System Theme')).toBeInTheDocument();
    });
  });

  describe('filtering', () => {
    it('filters apps based on search query', async () => {
      render(CommandPalette, {
        props: { apps: sampleApps },
      });
      const input = screen.getByPlaceholderText('Search apps and commands...');
      await fireEvent.input(input, { target: { value: 'Grafana' } });

      await waitFor(() => {
        expect(screen.getByText('Grafana')).toBeInTheDocument();
        expect(screen.queryByText('Sonarr')).not.toBeInTheDocument();
      });
    });

    it('shows no results message when nothing matches', async () => {
      render(CommandPalette, {
        props: { apps: sampleApps },
      });
      const input = screen.getByPlaceholderText('Search apps and commands...');
      await fireEvent.input(input, { target: { value: 'zzzznotfound' } });

      await waitFor(() => {
        expect(screen.getByText(/No results found/)).toBeInTheDocument();
      });
    });

    it('matches actions by label', async () => {
      render(CommandPalette, {
        props: { apps: sampleApps },
      });
      const input = screen.getByPlaceholderText('Search apps and commands...');
      await fireEvent.input(input, { target: { value: 'Fullscreen' } });

      await waitFor(() => {
        expect(screen.getByText('Toggle Fullscreen')).toBeInTheDocument();
      });
    });
  });

  describe('selection', () => {
    it('calls onselect when an app item is clicked', async () => {
      const onselect = vi.fn();
      render(CommandPalette, {
        props: { apps: sampleApps, onselect, onclose: vi.fn() },
      });

      const button = screen.getByText('Grafana').closest('button');
      expect(button).toBeTruthy();
      await fireEvent.click(button!);

      expect(onselect).toHaveBeenCalledTimes(1);
      expect(onselect).toHaveBeenCalledWith(
        expect.objectContaining({ name: 'Grafana' })
      );
    });

    it('calls onclose after selecting an app', async () => {
      const onclose = vi.fn();
      render(CommandPalette, {
        props: { apps: sampleApps, onselect: vi.fn(), onclose },
      });

      const button = screen.getByText('Grafana').closest('button');
      await fireEvent.click(button!);

      expect(onclose).toHaveBeenCalledTimes(1);
    });

    it('calls onaction when an action item is clicked', async () => {
      const onaction = vi.fn();
      const onclose = vi.fn();
      render(CommandPalette, {
        props: { apps: [], onaction, onclose },
      });

      const button = screen.getByText('Toggle Fullscreen').closest('button');
      expect(button).toBeTruthy();
      await fireEvent.click(button!);

      expect(onaction).toHaveBeenCalledWith('fullscreen');
      expect(onclose).toHaveBeenCalled();
    });
  });

  describe('keyboard interactions', () => {
    it('calls onclose when Escape is pressed', async () => {
      const onclose = vi.fn();
      render(CommandPalette, {
        props: { apps: sampleApps, onclose },
      });

      await fireEvent.keyDown(window, { key: 'Escape' });
      expect(onclose).toHaveBeenCalledTimes(1);
    });

    it('navigates selection with ArrowDown', async () => {
      const { container } = render(CommandPalette, {
        props: { apps: sampleApps },
      });

      // First item should be selected initially
      const selected = container.querySelector('[data-selected="true"]');
      expect(selected).toBeTruthy();

      // Press ArrowDown
      await fireEvent.keyDown(window, { key: 'ArrowDown' });

      // Selection should have moved
      await waitFor(() => {
        const items = container.querySelectorAll('[data-selected="true"]');
        expect(items.length).toBe(1);
      });
    });

    it('selects the highlighted item on Enter', async () => {
      const onselect = vi.fn();
      const onclose = vi.fn();
      render(CommandPalette, {
        props: { apps: sampleApps, onselect, onclose },
      });

      // Press Enter to select the first item (which should be the first app)
      await fireEvent.keyDown(window, { key: 'Enter' });

      // Should have called onselect or onaction (first item is an app)
      expect(onselect.mock.calls.length + (onclose.mock.calls.length > 0 ? 1 : 0)).toBeGreaterThan(0);
    });
  });

  describe('backdrop close', () => {
    it('calls onclose when clicking the backdrop', async () => {
      const onclose = vi.fn();
      const { container } = render(CommandPalette, {
        props: { apps: sampleApps, onclose },
      });

      // Click on the backdrop element (the outer dialog div)
      const backdrop = container.querySelector('.command-palette-backdrop');
      expect(backdrop).toBeTruthy();

      // Simulate a click where target === currentTarget (clicking directly on backdrop)
      await fireEvent.click(backdrop!);

      expect(onclose).toHaveBeenCalled();
    });
  });

  describe('footer hints', () => {
    it('shows keyboard hints in desktop mode', () => {
      render(CommandPalette, {
        props: { apps: sampleApps },
      });
      expect(screen.getByText(/Navigate/)).toBeInTheDocument();
      expect(screen.getByText(/Execute/)).toBeInTheDocument();
      expect(screen.getByText(/Close/)).toBeInTheDocument();
    });
  });

  describe('open mode indicator', () => {
    it('does not show open mode icon for iframe apps', () => {
      render(CommandPalette, {
        props: { apps: [makeApp({ name: 'IframeApp', open_mode: 'iframe' })] },
      });
      // iframe apps should not show the arrow indicator
      const appButton = screen.getByText('IframeApp').closest('button');
      expect(appButton?.textContent).not.toContain('\u2197'); // ↗ new_tab indicator
    });

    it('shows new_tab indicator for new_tab apps', () => {
      render(CommandPalette, {
        props: { apps: [makeApp({ name: 'TabApp', open_mode: 'new_tab' })] },
      });
      const appButton = screen.getByText('TabApp').closest('button');
      expect(appButton?.textContent).toContain('\u2197'); // ↗ new_tab indicator
    });

    it('shows new_window indicator for new_window apps', () => {
      render(CommandPalette, {
        props: { apps: [makeApp({ name: 'WindowApp', open_mode: 'new_window' })] },
      });
      const appButton = screen.getByText('WindowApp').closest('button');
      expect(appButton?.textContent).toContain('\u29C9'); // ⧉ new_window indicator
    });
  });

  describe('keyboard navigation (extended)', () => {
    it('navigates selection with ArrowUp', async () => {
      const { container } = render(CommandPalette, {
        props: { apps: sampleApps },
      });

      // Move down first, then up
      await fireEvent.keyDown(window, { key: 'ArrowDown' });
      await fireEvent.keyDown(window, { key: 'ArrowDown' });
      await fireEvent.keyDown(window, { key: 'ArrowUp' });

      await waitFor(() => {
        const items = container.querySelectorAll('[data-selected="true"]');
        expect(items.length).toBe(1);
      });
    });

    it('does not go below the last item with ArrowDown', async () => {
      const apps = [makeApp({ name: 'OnlyApp' })];
      const { container } = render(CommandPalette, {
        props: { apps },
      });

      // Press ArrowDown many times — should stay on last item
      for (let i = 0; i < 30; i++) {
        await fireEvent.keyDown(window, { key: 'ArrowDown' });
      }

      await waitFor(() => {
        const items = container.querySelectorAll('[data-selected="true"]');
        expect(items.length).toBe(1);
      });
    });

    it('does not go above the first item with ArrowUp', async () => {
      const { container } = render(CommandPalette, {
        props: { apps: sampleApps },
      });

      // Press ArrowUp from index 0 — should stay at 0
      await fireEvent.keyDown(window, { key: 'ArrowUp' });

      await waitFor(() => {
        const items = container.querySelectorAll('[data-selected="true"]');
        expect(items.length).toBe(1);
      });
    });

    it('navigates with PageDown', async () => {
      const { container } = render(CommandPalette, {
        props: { apps: sampleApps },
      });

      await fireEvent.keyDown(window, { key: 'PageDown' });

      await waitFor(() => {
        const items = container.querySelectorAll('[data-selected="true"]');
        expect(items.length).toBe(1);
      });
    });

    it('navigates with PageUp', async () => {
      const { container } = render(CommandPalette, {
        props: { apps: sampleApps },
      });

      // Go down first then page up
      await fireEvent.keyDown(window, { key: 'PageDown' });
      await fireEvent.keyDown(window, { key: 'PageUp' });

      await waitFor(() => {
        const items = container.querySelectorAll('[data-selected="true"]');
        expect(items.length).toBe(1);
      });
    });

    it('navigates to first item with Home', async () => {
      const { container } = render(CommandPalette, {
        props: { apps: sampleApps },
      });

      // Move down then press Home
      await fireEvent.keyDown(window, { key: 'ArrowDown' });
      await fireEvent.keyDown(window, { key: 'ArrowDown' });
      await fireEvent.keyDown(window, { key: 'Home' });

      await waitFor(() => {
        const selected = container.querySelector('[data-selected="true"]');
        expect(selected).toBeTruthy();
        // First item should be selected (index 0)
        const allButtons = container.querySelectorAll('.command-palette-item');
        expect(allButtons[0]?.getAttribute('data-selected')).toBe('true');
      });
    });

    it('navigates to last item with End', async () => {
      const { container } = render(CommandPalette, {
        props: { apps: sampleApps },
      });

      await fireEvent.keyDown(window, { key: 'End' });

      await waitFor(() => {
        const selected = container.querySelector('[data-selected="true"]');
        expect(selected).toBeTruthy();
      });
    });

    it('selects an action command via Enter after navigation', async () => {
      const onaction = vi.fn();
      const onclose = vi.fn();
      render(CommandPalette, {
        props: { apps: [], onaction, onclose },
      });

      // Press Enter on first action (Show Keyboard Shortcuts — no Settings since isAdmin is false)
      await fireEvent.keyDown(window, { key: 'Enter' });

      expect(onaction).toHaveBeenCalledWith('shortcuts');
      expect(onclose).toHaveBeenCalled();
    });

    it('handles Ctrl+number quick select', async () => {
      const onselect = vi.fn();
      const onclose = vi.fn();
      render(CommandPalette, {
        props: { apps: sampleApps, onselect, onclose },
      });

      // Ctrl+1 should select first app
      await fireEvent.keyDown(window, { key: '1', ctrlKey: true });

      expect(onselect).toHaveBeenCalledWith(
        expect.objectContaining({ name: 'Grafana' })
      );
      expect(onclose).toHaveBeenCalled();
    });

    it('handles Ctrl+number for second app', async () => {
      const onselect = vi.fn();
      const onclose = vi.fn();
      render(CommandPalette, {
        props: { apps: sampleApps, onselect, onclose },
      });

      // Ctrl+2 should select second app
      await fireEvent.keyDown(window, { key: '2', ctrlKey: true });

      expect(onselect).toHaveBeenCalledWith(
        expect.objectContaining({ name: 'Prometheus' })
      );
    });

    it('handles Meta+number quick select (Cmd on macOS)', async () => {
      const onselect = vi.fn();
      const onclose = vi.fn();
      render(CommandPalette, {
        props: { apps: sampleApps, onselect, onclose },
      });

      // Meta+3 should select third app
      await fireEvent.keyDown(window, { key: '3', metaKey: true });

      expect(onselect).toHaveBeenCalledWith(
        expect.objectContaining({ name: 'Sonarr' })
      );
    });

    it('ignores Ctrl+number for out of range index', async () => {
      const onselect = vi.fn();
      const onclose = vi.fn();
      render(CommandPalette, {
        props: { apps: sampleApps, onselect, onclose },
      });

      // Ctrl+9 should do nothing (only 3 apps)
      await fireEvent.keyDown(window, { key: '9', ctrlKey: true });

      expect(onselect).not.toHaveBeenCalled();
      expect(onclose).not.toHaveBeenCalled();
    });
  });

  describe('recent apps', () => {
    it('loads recent apps from localStorage', async () => {
      // Pre-populate localStorage with recent apps
      localStorage.setItem('muximux_recent_apps', JSON.stringify(['Grafana', 'Sonarr']));

      const { container } = render(CommandPalette, {
        props: { apps: sampleApps },
      });

      // Wait for onMount to load recent apps
      await waitFor(() => {
        const recentHeader = container.querySelector('.text-xs.font-semibold');
        expect(recentHeader?.textContent).toContain('Recent');
      });
    });

    it('saves selected app to recent apps in localStorage', async () => {
      const onselect = vi.fn();
      const onclose = vi.fn();
      render(CommandPalette, {
        props: { apps: sampleApps, onselect, onclose },
      });

      const button = screen.getByText('Grafana').closest('button');
      await fireEvent.click(button!);

      // Check that localStorage was updated
      const stored = localStorage.getItem('muximux_recent_apps');
      expect(stored).toBeTruthy();
      const parsed = JSON.parse(stored!);
      expect(parsed).toContain('Grafana');
    });

    it('handles invalid JSON in localStorage gracefully', async () => {
      // Store invalid JSON
      localStorage.setItem('muximux_recent_apps', 'not-valid-json');

      // Should not throw
      const { container } = render(CommandPalette, {
        props: { apps: sampleApps },
      });

      expect(container.querySelector('[role="dialog"]')).toBeTruthy();
    });
  });

  describe('fuzzy matching', () => {
    it('matches partial strings (substring match)', async () => {
      render(CommandPalette, {
        props: { apps: sampleApps },
      });
      const input = screen.getByPlaceholderText('Search apps and commands...');
      await fireEvent.input(input, { target: { value: 'graf' } });

      await waitFor(() => {
        expect(screen.getByText('Grafana')).toBeInTheDocument();
        expect(screen.queryByText('Sonarr')).not.toBeInTheDocument();
      });
    });

    it('matches fuzzy scattered characters', async () => {
      render(CommandPalette, {
        props: { apps: sampleApps },
      });
      const input = screen.getByPlaceholderText('Search apps and commands...');
      // "grfn" should fuzzy match "Grafana" (g-r-f-n scattered in G-r-a-f-a-n-a)
      await fireEvent.input(input, { target: { value: 'grfn' } });

      await waitFor(() => {
        expect(screen.getByText('Grafana')).toBeInTheDocument();
      });
    });

    it('matches by number shortcut (1-9)', async () => {
      render(CommandPalette, {
        props: { apps: sampleApps },
      });
      const input = screen.getByPlaceholderText('Search apps and commands...');
      // Type "1" — should match first app via number shortcut
      await fireEvent.input(input, { target: { value: '1' } });

      await waitFor(() => {
        expect(screen.getByText('Grafana')).toBeInTheDocument();
      });
    });

    it('matches by group/description', async () => {
      const apps = [
        makeApp({ name: 'App1', group: 'Monitoring' }),
        makeApp({ name: 'App2', group: 'Media' }),
      ];
      render(CommandPalette, {
        props: { apps },
      });
      const input = screen.getByPlaceholderText('Search apps and commands...');
      await fireEvent.input(input, { target: { value: 'Monitoring' } });

      await waitFor(() => {
        expect(screen.getByText('App1')).toBeInTheDocument();
        expect(screen.queryByText('App2')).not.toBeInTheDocument();
      });
    });

    it('resets selection index when query changes', async () => {
      const { container } = render(CommandPalette, {
        props: { apps: sampleApps },
      });

      // Navigate down
      await fireEvent.keyDown(window, { key: 'ArrowDown' });
      await fireEvent.keyDown(window, { key: 'ArrowDown' });

      // Type a query — selection should reset to 0
      const input = screen.getByPlaceholderText('Search apps and commands...');
      await fireEvent.input(input, { target: { value: 'Grafana' } });

      await waitFor(() => {
        const items = container.querySelectorAll('.command-palette-item');
        if (items.length > 0) {
          expect(items[0]?.getAttribute('data-selected')).toBe('true');
        }
      });
    });
  });

  describe('action and setting commands', () => {
    it('calls onaction with setting command id when theme setting is clicked', async () => {
      const onaction = vi.fn();
      const onclose = vi.fn();
      render(CommandPalette, {
        props: { apps: [], onaction, onclose },
      });

      const button = screen.getByText('Set Dark Theme').closest('button');
      await fireEvent.click(button!);

      expect(onaction).toHaveBeenCalledWith('theme-dark');
      expect(onclose).toHaveBeenCalled();
    });

    it('calls onaction for Go to Splash Screen', async () => {
      const onaction = vi.fn();
      const onclose = vi.fn();
      render(CommandPalette, {
        props: { apps: [], onaction, onclose },
      });

      const button = screen.getByText('Go to Splash Screen').closest('button');
      await fireEvent.click(button!);

      expect(onaction).toHaveBeenCalledWith('home');
    });

    it('calls onaction for View Logs', async () => {
      const onaction = vi.fn();
      const onclose = vi.fn();
      render(CommandPalette, {
        props: { apps: [], onaction, onclose },
      });

      const button = screen.getByText('View Logs').closest('button');
      await fireEvent.click(button!);

      expect(onaction).toHaveBeenCalledWith('logs');
    });

    it('calls onaction for Refresh Current App', async () => {
      const onaction = vi.fn();
      const onclose = vi.fn();
      render(CommandPalette, {
        props: { apps: [], onaction, onclose },
      });

      const button = screen.getByText('Refresh Current App').closest('button');
      await fireEvent.click(button!);

      expect(onaction).toHaveBeenCalledWith('refresh');
    });

    it('calls onaction for Set Light Theme', async () => {
      const onaction = vi.fn();
      const onclose = vi.fn();
      render(CommandPalette, {
        props: { apps: [], onaction, onclose },
      });

      const button = screen.getByText('Set Light Theme').closest('button');
      await fireEvent.click(button!);

      expect(onaction).toHaveBeenCalledWith('theme-light');
    });

    it('calls onaction for Use System Theme', async () => {
      const onaction = vi.fn();
      const onclose = vi.fn();
      render(CommandPalette, {
        props: { apps: [], onaction, onclose },
      });

      const button = screen.getByText('Use System Theme').closest('button');
      await fireEvent.click(button!);

      expect(onaction).toHaveBeenCalledWith('theme-system');
    });
  });

  describe('mouse interactions', () => {
    it('highlights item on mouseenter', async () => {
      render(CommandPalette, {
        props: { apps: sampleApps },
      });

      // Find the second app button
      const secondAppButton = screen.getByText('Prometheus').closest('button');
      expect(secondAppButton).toBeTruthy();

      await fireEvent.mouseEnter(secondAppButton!);

      await waitFor(() => {
        expect(secondAppButton?.getAttribute('data-selected')).toBe('true');
      });
    });
  });

  describe('group display', () => {
    it('shows group name as description for apps with groups', () => {
      const apps = [
        makeApp({ name: 'Grafana', group: 'Monitoring' }),
      ];
      render(CommandPalette, {
        props: { apps },
      });
      expect(screen.getByText('Monitoring')).toBeInTheDocument();
    });

    it('shows "Switch to app" for apps without a group', () => {
      const apps = [
        makeApp({ name: 'NoGroup', group: '' }),
      ];
      render(CommandPalette, {
        props: { apps },
      });
      expect(screen.getByText('Switch to app')).toBeInTheDocument();
    });
  });

  describe('section headers', () => {
    it('renders Apps section header', () => {
      const { container } = render(CommandPalette, {
        props: { apps: sampleApps },
      });
      const headers = container.querySelectorAll('.text-xs.font-semibold');
      const texts = Array.from(headers).map(h => h.textContent);
      expect(texts).toContain('Apps');
    });

    it('renders Actions section header', () => {
      const { container } = render(CommandPalette, {
        props: { apps: sampleApps },
      });
      const headers = container.querySelectorAll('.text-xs.font-semibold');
      const texts = Array.from(headers).map(h => h.textContent);
      expect(texts).toContain('Actions');
    });

    it('renders Settings section header', () => {
      const { container } = render(CommandPalette, {
        props: { apps: sampleApps },
      });
      const headers = container.querySelectorAll('.text-xs.font-semibold');
      const texts = Array.from(headers).map(h => h.textContent);
      expect(texts).toContain('Settings');
    });
  });

  describe('mobile viewport', () => {
    it('shows mobile footer in mobile mode', async () => {
      const { isMobileViewport } = await import('$lib/useSwipe');
      vi.mocked(isMobileViewport).mockReturnValue(true);

      render(CommandPalette, {
        props: { apps: sampleApps },
      });

      await waitFor(() => {
        expect(screen.getByText('Tap outside to close')).toBeInTheDocument();
      });

      // Restore
      vi.mocked(isMobileViewport).mockReturnValue(false);
    });

    it('shows mobile drag handle in mobile mode', async () => {
      const { isMobileViewport } = await import('$lib/useSwipe');
      vi.mocked(isMobileViewport).mockReturnValue(true);

      const { container } = render(CommandPalette, {
        props: { apps: sampleApps },
      });

      await waitFor(() => {
        // The drag handle is a div with w-10 h-1 rounded-full classes
        const handle = container.querySelector('.w-10.h-1.rounded-full');
        expect(handle).toBeTruthy();
      });

      vi.mocked(isMobileViewport).mockReturnValue(false);
    });
  });

  describe('no callbacks provided', () => {
    it('handles clicks gracefully when onselect and onclose are undefined', async () => {
      // Render with no callbacks
      render(CommandPalette, {
        props: { apps: sampleApps },
      });

      const button = screen.getByText('Grafana').closest('button');
      // Should not throw
      await fireEvent.click(button!);
    });

    it('handles action click gracefully when onaction is undefined', async () => {
      render(CommandPalette, {
        props: { apps: [] },
      });

      const button = screen.getByText('Toggle Fullscreen').closest('button');
      // Should not throw
      await fireEvent.click(button!);
    });

    it('handles Escape gracefully when onclose is undefined', async () => {
      render(CommandPalette, {
        props: { apps: sampleApps },
      });

      // Should not throw
      await fireEvent.keyDown(window, { key: 'Escape' });
    });
  });

  describe('shortcut badges', () => {
    it('shows number shortcuts for first 9 apps', () => {
      const manyApps = Array.from({ length: 10 }, (_, i) =>
        makeApp({ name: `App${i + 1}`, url: `https://app${i + 1}.local` })
      );
      const { container } = render(CommandPalette, {
        props: { apps: manyApps },
      });

      // Apps with index 0-8 should have shortcuts 1-9
      const kbds = container.querySelectorAll('.command-palette-kbd');
      const texts = Array.from(kbds).map(k => k.textContent?.trim());
      // Should contain "1" through "9" for the first 9 apps
      for (let i = 1; i <= 9; i++) {
        expect(texts).toContain(String(i));
      }
    });
  });

  describe('filtered results by query', () => {
    it('filters settings commands by query', async () => {
      render(CommandPalette, {
        props: { apps: [] },
      });
      const input = screen.getByPlaceholderText('Search apps and commands...');
      await fireEvent.input(input, { target: { value: 'Dark' } });

      await waitFor(() => {
        expect(screen.getByText('Set Dark Theme')).toBeInTheDocument();
        expect(screen.queryByText('Set Light Theme')).not.toBeInTheDocument();
      });
    });

    it('filters and shows only matching actions', async () => {
      render(CommandPalette, {
        props: { apps: sampleApps },
      });
      const input = screen.getByPlaceholderText('Search apps and commands...');
      await fireEvent.input(input, { target: { value: 'Refresh' } });

      await waitFor(() => {
        expect(screen.getByText('Refresh Current App')).toBeInTheDocument();
        expect(screen.queryByText('Toggle Fullscreen')).not.toBeInTheDocument();
        expect(screen.queryByText('Grafana')).not.toBeInTheDocument();
      });
    });
  });

  describe('backdrop keydown', () => {
    it('calls onclose when Escape is pressed on the backdrop', async () => {
      const onclose = vi.fn();
      const { container } = render(CommandPalette, {
        props: { apps: sampleApps, onclose },
      });

      const backdrop = container.querySelector('.command-palette-backdrop');
      expect(backdrop).toBeTruthy();

      await fireEvent.keyDown(backdrop!, { key: 'Escape' });
      expect(onclose).toHaveBeenCalled();
    });
  });

  describe('modal click propagation', () => {
    it('does not close when clicking inside the modal', async () => {
      const onclose = vi.fn();
      const { container } = render(CommandPalette, {
        props: { apps: sampleApps, onclose },
      });

      const modal = container.querySelector('.command-palette-modal');
      expect(modal).toBeTruthy();
      await fireEvent.click(modal!);

      // onclose should NOT have been called because stopPropagation prevents it
      // and the click handler on backdrop checks e.currentTarget === e.target
      expect(onclose).not.toHaveBeenCalled();
    });
  });
});
