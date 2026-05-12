import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import { writable } from 'svelte/store';

// Standard mocks (same pattern as other component tests)
vi.mock('$lib/api', () => ({ getBase: vi.fn(() => '') }));
vi.mock('$lib/debug', () => ({ debug: vi.fn() }));
vi.mock('$lib/healthStore', () => ({ healthData: writable(new Map()) }));
vi.mock('$lib/constants', () => ({
  openModes: [
    { value: 'iframe', label: 'Embedded', description: 'Show inside Muximux' },
    { value: 'new_tab', label: 'New Tab', description: 'Open in a new browser tab' },
  ],
  IFRAME_PERMISSIONS: [
    { id: 'camera', description: 'camera desc', docsUrl: 'https://mdn.example/camera' },
    { id: 'microphone', description: 'microphone desc', docsUrl: 'https://mdn.example/microphone' },
    { id: 'geolocation', description: 'geolocation desc', docsUrl: 'https://mdn.example/geolocation' },
    { id: 'fullscreen', description: 'fullscreen desc', docsUrl: 'https://mdn.example/fullscreen' },
  ] as const,
  ALL_IFRAME_PERMISSIONS: ['camera', 'microphone', 'geolocation', 'fullscreen'] as const,
  resolvePermissions: (perms: readonly string[] | undefined) => {
    if (!perms || perms.length === 0) return [];
    if (perms.includes('none')) return [];
    if (perms.includes('all')) return ['camera', 'microphone', 'geolocation', 'fullscreen'];
    return perms.filter(p => ['camera', 'microphone', 'geolocation', 'fullscreen'].includes(p));
  },
}));

function noopComponent() {
  return { $destroy: vi.fn() };
}
vi.mock('./AppIcon.svelte', () => ({ default: noopComponent }));

import AppForm from './AppForm.svelte';
import type { App, Group } from '$lib/types';

function makeApp(overrides: Partial<App> = {}): App {
  return {
    name: 'Test App',
    url: 'http://localhost:8080',
    icon: { type: 'lucide', name: 'home', file: '', url: '', variant: '' },
    color: '#22c55e',
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

const defaultGroups: Group[] = [
  { name: 'Media', icon: { type: 'lucide', name: 'play', file: '', url: '', variant: '' }, color: '', order: 0, expanded: true },
];

describe('AppForm', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // =========================================================================
  // Identity section
  // =========================================================================
  describe('Identity section', () => {
    it('renders name input with value', () => {
      render(AppForm, { props: { app: makeApp(), mode: 'create', groups: defaultGroups, allApps: [] } });
      const input = document.getElementById('create-app-name') as HTMLInputElement;
      expect(input).toBeInTheDocument();
      expect(input.value).toBe('Test App');
    });

    it('renders URL input with value', () => {
      render(AppForm, { props: { app: makeApp(), mode: 'create', groups: defaultGroups, allApps: [] } });
      const input = document.getElementById('create-app-url') as HTMLInputElement;
      expect(input).toBeInTheDocument();
      expect(input.value).toBe('http://localhost:8080');
    });

    it('uses edit prefix for IDs in edit mode', () => {
      render(AppForm, { props: { app: makeApp(), mode: 'edit', groups: defaultGroups, allApps: [] } });
      expect(document.getElementById('edit-app-name')).toBeInTheDocument();
      expect(document.getElementById('edit-app-url')).toBeInTheDocument();
    });

    it('renders icon chooser button', () => {
      render(AppForm, { props: { app: makeApp(), mode: 'create', groups: defaultGroups, allApps: [] } });
      expect(screen.getByText('home')).toBeInTheDocument(); // icon name
    });

    it('renders color picker', () => {
      render(AppForm, { props: { app: makeApp(), mode: 'create', groups: defaultGroups, allApps: [] } });
      expect(document.getElementById('create-app-color')).toBeInTheDocument();
    });

    it('renders group dropdown with options', () => {
      render(AppForm, { props: { app: makeApp(), mode: 'create', groups: defaultGroups, allApps: [] } });
      const select = document.getElementById('create-app-group') as HTMLSelectElement;
      expect(select).toBeInTheDocument();
      expect(screen.getByText('Media')).toBeInTheDocument();
      expect(screen.getByText('No group')).toBeInTheDocument();
    });

    it('shows validation errors when provided', () => {
      render(AppForm, {
        props: {
          app: makeApp(), mode: 'create', groups: defaultGroups, allApps: [],
          errors: { name: 'Name is required' },
        },
      });
      expect(screen.getByText('Name is required')).toBeInTheDocument();
    });

    it('shows URL validation error', () => {
      render(AppForm, {
        props: {
          app: makeApp(), mode: 'create', groups: defaultGroups, allApps: [],
          errors: { url: 'Invalid URL' },
        },
      });
      expect(screen.getByText('Invalid URL')).toBeInTheDocument();
    });

    it('calls onclearerror when typing in name field', async () => {
      const onclearerror = vi.fn();
      render(AppForm, {
        props: { app: makeApp(), mode: 'create', groups: defaultGroups, allApps: [], onclearerror },
      });
      const input = document.getElementById('create-app-name')!;
      await fireEvent.input(input, { target: { value: 'New' } });
      expect(onclearerror).toHaveBeenCalledWith('name');
    });

    it('calls onclearerror when typing in URL field', async () => {
      const onclearerror = vi.fn();
      render(AppForm, {
        props: { app: makeApp(), mode: 'create', groups: defaultGroups, allApps: [], onclearerror },
      });
      const input = document.getElementById('create-app-url')!;
      await fireEvent.input(input, { target: { value: 'http://x' } });
      expect(onclearerror).toHaveBeenCalledWith('url');
    });

    it('calls onopenicon when icon button clicked', async () => {
      const onopenicon = vi.fn();
      render(AppForm, {
        props: { app: makeApp(), mode: 'create', groups: defaultGroups, allApps: [], onopenicon },
      });
      // Click the "Choose icon..." button text
      await fireEvent.click(screen.getByText('home'));
      expect(onopenicon).toHaveBeenCalledTimes(1);
    });

    it('shows tooltips on fields', () => {
      const { container } = render(AppForm, {
        props: { app: makeApp(), mode: 'edit', groups: defaultGroups, allApps: [] },
      });
      const tooltips = container.querySelectorAll('.help-tooltip');
      expect(tooltips.length).toBeGreaterThanOrEqual(2); // At least name + url
    });

    it('shows icon type description', () => {
      render(AppForm, {
        props: { app: makeApp({ icon: { type: 'dashboard', name: 'test', file: '', url: '', variant: 'svg' } }), mode: 'edit', groups: defaultGroups, allApps: [] },
      });
      expect(screen.getByText('Dashboard Icon')).toBeInTheDocument();
    });

    it('shows icon color picker for lucide icons', () => {
      const { container } = render(AppForm, {
        props: { app: makeApp(), mode: 'create', groups: defaultGroups, allApps: [] },
      });
      // lucide icon should show color picker label
      expect(container.textContent).toContain('Icon color');
    });

    it('does not show icon color picker for dashboard icons', () => {
      const { container } = render(AppForm, {
        props: { app: makeApp({ icon: { type: 'dashboard', name: 'test', file: '', url: '', variant: '' } }), mode: 'create', groups: defaultGroups, allApps: [] },
      });
      expect(container.textContent).not.toContain('Icon color');
    });
  });

  // =========================================================================
  // Display section
  // =========================================================================
  describe('Display section', () => {
    it('renders Display section header', () => {
      render(AppForm, { props: { app: makeApp(), mode: 'create', groups: defaultGroups, allApps: [] } });
      expect(screen.getByText('Display')).toBeInTheDocument();
    });

    it('renders enabled checkbox', () => {
      render(AppForm, { props: { app: makeApp(), mode: 'create', groups: defaultGroups, allApps: [] } });
      expect(screen.getByText('Enabled')).toBeInTheDocument();
    });

    it('renders default app checkbox', () => {
      render(AppForm, { props: { app: makeApp(), mode: 'create', groups: defaultGroups, allApps: [] } });
      expect(screen.getByText('Default app')).toBeInTheDocument();
    });

    it('renders open mode dropdown', () => {
      render(AppForm, { props: { app: makeApp(), mode: 'create', groups: defaultGroups, allApps: [] } });
      expect(screen.getByText(/Open Mode/)).toBeInTheDocument();
    });

    it('renders scale slider', () => {
      render(AppForm, { props: { app: makeApp(), mode: 'create', groups: defaultGroups, allApps: [] } });
      expect(screen.getByText(/Scale:/)).toBeInTheDocument();
    });

    it('calls ondefaultchange when default checkbox toggled', async () => {
      const ondefaultchange = vi.fn();
      const { container } = render(AppForm, {
        props: { app: makeApp(), mode: 'create', groups: defaultGroups, allApps: [], ondefaultchange },
      });
      // Find the default checkbox (second checkbox in the Display section)
      const checkboxes = container.querySelectorAll('input[type="checkbox"]');
      // checkboxes order: enabled, default, proxy, force_icon_bg, invert
      await fireEvent.click(checkboxes[1]); // default
      expect(ondefaultchange).toHaveBeenCalledWith(true);
    });
  });

  // =========================================================================
  // Proxy section
  // =========================================================================
  describe('Proxy section', () => {
    it('renders Proxy section header', () => {
      render(AppForm, { props: { app: makeApp(), mode: 'create', groups: defaultGroups, allApps: [] } });
      expect(screen.getByText('Proxy')).toBeInTheDocument();
    });

    it('renders proxy checkbox', () => {
      render(AppForm, { props: { app: makeApp(), mode: 'create', groups: defaultGroups, allApps: [] } });
      expect(screen.getByText('Use reverse proxy')).toBeInTheDocument();
    });

    it('shows proxy sub-options when proxy enabled', () => {
      render(AppForm, { props: { app: makeApp({ proxy: true }), mode: 'edit', groups: defaultGroups, allApps: [] } });
      expect(screen.getByText(/Skip TLS/)).toBeInTheDocument();
      expect(screen.getByText(/Custom headers/)).toBeInTheDocument();
    });

    it('hides proxy sub-options when proxy disabled', () => {
      render(AppForm, { props: { app: makeApp({ proxy: false }), mode: 'edit', groups: defaultGroups, allApps: [] } });
      expect(screen.queryByText(/Skip TLS/)).not.toBeInTheDocument();
    });

    it('shows add header button when proxy enabled', () => {
      render(AppForm, { props: { app: makeApp({ proxy: true }), mode: 'edit', groups: defaultGroups, allApps: [] } });
      expect(screen.getByText('+ Add header')).toBeInTheDocument();
    });
  });

  // =========================================================================
  // Advanced section
  // =========================================================================
  describe('Advanced section', () => {
    it('renders Advanced section header', () => {
      render(AppForm, { props: { app: makeApp(), mode: 'create', groups: defaultGroups, allApps: [] } });
      expect(screen.getByText('Advanced')).toBeInTheDocument();
    });

    it('renders health check checkbox', () => {
      render(AppForm, { props: { app: makeApp(), mode: 'edit', groups: defaultGroups, allApps: [] } });
      expect(screen.getByText('Health check')).toBeInTheDocument();
    });

    it('shows health URL when health check enabled', () => {
      render(AppForm, { props: { app: makeApp({ health_check: true }), mode: 'edit', groups: defaultGroups, allApps: [] } });
      expect(document.getElementById('edit-app-health-url')).toBeInTheDocument();
    });

    it('hides health URL when health check disabled', () => {
      render(AppForm, { props: { app: makeApp({ health_check: false }), mode: 'edit', groups: defaultGroups, allApps: [] } });
      expect(document.getElementById('edit-app-health-url')).not.toBeInTheDocument();
    });

    it('renders keyboard shortcut selector', () => {
      render(AppForm, { props: { app: makeApp(), mode: 'edit', groups: defaultGroups, allApps: [] } });
      expect(screen.getByText('Keyboard Shortcut')).toBeInTheDocument();
    });

    it('shows taken shortcuts in dropdown', () => {
      const otherApp = makeApp({ name: 'Other', shortcut: 3 });
      render(AppForm, { props: { app: makeApp(), mode: 'edit', groups: defaultGroups, allApps: [otherApp] } });
      expect(screen.getByText(/3 \(Other\)/)).toBeInTheDocument();
    });

    it('renders force icon background checkbox', () => {
      render(AppForm, { props: { app: makeApp(), mode: 'edit', groups: defaultGroups, allApps: [] } });
      expect(screen.getByText('Force icon background')).toBeInTheDocument();
    });

    it('renders invert icon colors checkbox', () => {
      render(AppForm, { props: { app: makeApp(), mode: 'edit', groups: defaultGroups, allApps: [] } });
      expect(screen.getByText('Invert icon colors')).toBeInTheDocument();
    });

    it('renders minimum role dropdown', () => {
      render(AppForm, { props: { app: makeApp(), mode: 'edit', groups: defaultGroups, allApps: [] } });
      expect(screen.getByText('Minimum Role')).toBeInTheDocument();
    });

    it('shows all role options in minimum role dropdown', () => {
      render(AppForm, { props: { app: makeApp(), mode: 'create', groups: defaultGroups, allApps: [] } });
      expect(screen.getByText('Everyone (default)')).toBeInTheDocument();
      expect(screen.getByText('Power User')).toBeInTheDocument();
      expect(screen.getByText('Admin')).toBeInTheDocument();
    });
  });

  // =========================================================================
  // Sections always visible
  // =========================================================================
  describe('sections', () => {
    it('shows all sections in create mode', () => {
      render(AppForm, { props: { app: makeApp(), mode: 'create', groups: defaultGroups, allApps: [] } });
      expect(screen.getByText('Display')).toBeInTheDocument();
      expect(screen.getByText('Proxy')).toBeInTheDocument();
      expect(screen.getByText('Advanced')).toBeInTheDocument();
    });

    it('shows all sections in edit mode', () => {
      render(AppForm, { props: { app: makeApp(), mode: 'edit', groups: defaultGroups, allApps: [] } });
      expect(screen.getByText('Display')).toBeInTheDocument();
      expect(screen.getByText('Proxy')).toBeInTheDocument();
      expect(screen.getByText('Advanced')).toBeInTheDocument();
    });
  });

  // ===========================================================================
  // 3.1.0 additions: Docker-managed lock + gateway-domain notice. Both
  // hang off optional App fields and render only when present. Without
  // tests they're dead branches in the coverage report.
  // ===========================================================================
  describe('Docker-managed lock notice', () => {
    it('hides the lock notice when the app is not managed by discovery', () => {
      render(AppForm, { props: { app: makeApp(), mode: 'edit', groups: defaultGroups, allApps: [] } });
      expect(screen.queryByTestId('app-form-docker-locked')).not.toBeInTheDocument();
    });

    it('renders the lock notice and marks URL readonly when docker_key is set', () => {
      const app = makeApp({ docker_key: 'label:sonarr-stable' });
      render(AppForm, { props: { app, mode: 'edit', groups: defaultGroups, allApps: [] } });

      // Notice visible.
      expect(screen.getByTestId('app-form-docker-locked')).toBeInTheDocument();
      // The notice references the source tracking key.
      expect(screen.getByText(/label:sonarr-stable/)).toBeInTheDocument();
      // URL field is readonly while Docker-managed - prevents the
      // operator from drifting the URL out of sync with refresh ticks.
      const url = screen.getByTestId('app-form-url') as HTMLInputElement;
      expect(url.readOnly).toBe(true);
    });
  });

  describe('Gateway-domain notice', () => {
    it('hides the gateway hint when the app has no gateway_domain', () => {
      render(AppForm, { props: { app: makeApp(), mode: 'edit', groups: defaultGroups, allApps: [] } });
      expect(screen.queryByText(/Hosted by Muximux gateway/i)).not.toBeInTheDocument();
    });

    it('renders the gateway hint with the configured subdomain and suggested URL', () => {
      const app = makeApp({ gateway_domain: 'sonarr.example.com' });
      render(AppForm, { props: { app, mode: 'edit', groups: defaultGroups, allApps: [] } });

      // Both the domain name and the canonical https://<domain>
      // suggestion appear so the operator can copy the right URL in.
      // The "Set this app's URL to ..." sentence references the same
      // domain - we anchor on the unique copy.
      expect(screen.getByText(/Hosted by Muximux gateway/i)).toBeInTheDocument();
      // The domain appears in both the "Hosted by" line and the
      // "Set this app's URL to https://<domain>" suggestion.
      const occurrences = screen.getAllByText(/sonarr\.example\.com/);
      expect(occurrences.length).toBeGreaterThanOrEqual(2);
    });
  });
});
