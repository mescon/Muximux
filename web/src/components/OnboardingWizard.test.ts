import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

// --- Hoisted store values and mock fns ---
const {
  mockCurrentStep,
  mockSelectedApps,
  mockSelectedNavigation,
  mockShowLabels,
  mockStepProgress,
  mockActiveStepOrder,
  mockSelectedFamily,
  mockVariantMode,
  mockSystemTheme,
  mockThemeFamilies,
  mockNextStep,
  mockPrevStep,
  mockConfigureSteps,
  mockSetThemeFamily,
  mockSetVariantMode,
  mockDetectCustomThemes,
  mockApplyPreset,
  mockBuildForwardAuthRequest,
  mockTemplateToApp,
  mockPlexTemplate,
  mockSonarrTemplate,
  mockPortainerTemplate,
} = vi.hoisted(() => {
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
      update(updater: (v: T) => T) {
        value = updater(value);
        subs.forEach(fn => fn(value));
      },
    };
  }
  return {
    mockCurrentStep: makeStore('welcome' as string),
    mockSelectedApps: makeStore([] as unknown[]),
    mockSelectedNavigation: makeStore('left' as string),
    mockShowLabels: makeStore(true),
    mockStepProgress: makeStore(0),
    mockActiveStepOrder: makeStore(['welcome', 'apps', 'navigation', 'theme', 'complete']),
    mockSelectedFamily: makeStore('default'),
    mockVariantMode: makeStore('dark' as 'dark' | 'light' | 'system'),
    mockSystemTheme: makeStore('dark' as string),
    mockThemeFamilies: makeStore([] as unknown[]),
    mockNextStep: vi.fn(),
    mockPrevStep: vi.fn(),
    mockConfigureSteps: vi.fn(),
    mockSetThemeFamily: vi.fn(),
    mockSetVariantMode: vi.fn(),
    mockDetectCustomThemes: vi.fn().mockResolvedValue(undefined),
    mockApplyPreset: vi.fn((preset: string, _logoutUrl: string) => {
      const presets: Record<string, { headers: { user: string; email: string; groups: string; name: string }; logoutUrl: string }> = {
        authelia: {
          headers: { user: 'Remote-User', email: 'Remote-Email', groups: 'Remote-Groups', name: 'Remote-Name' },
          logoutUrl: 'https://auth.example.com/logout',
        },
        authentik: {
          headers: { user: 'X-authentik-username', email: 'X-authentik-email', groups: 'X-authentik-groups', name: 'X-authentik-name' },
          logoutUrl: 'https://auth.example.com/outpost.goauthentik.io/sign_out',
        },
        custom: {
          headers: { user: 'Remote-User', email: 'Remote-Email', groups: 'Remote-Groups', name: 'Remote-Name' },
          logoutUrl: '',
        },
      };
      return presets[preset] || presets.custom;
    }),
    mockBuildForwardAuthRequest: vi.fn(
      (proxies: string, user: string, email: string, groups: string, name: string, logoutUrl: string) => ({
        trusted_proxies: proxies.split(/[,\n]/).map((s: string) => s.trim()).filter((s: string) => s.length > 0),
        headers: { user, email, groups, name },
        logout_url: logoutUrl,
      })
    ),
    mockTemplateToApp: vi.fn((template: { name: string; icon: string; color: string; group: string }, url: string, order: number) => ({
      name: template.name,
      url,
      icon: { type: 'dashboard', name: template.icon, file: '', url: '', variant: 'svg' },
      color: template.color,
      group: template.group,
      order,
      enabled: true,
      default: false,
      open_mode: 'iframe',
      proxy: false,
      scale: 1,
    })),
    mockPlexTemplate: {
      name: 'Plex',
      defaultUrl: 'http://localhost:32400/web',
      icon: 'plex',
      iconType: 'dashboard',
      color: '#E5A00D',
      iconBackground: '#2D2200',
      group: 'Media',
      description: 'Stream your media library',
    },
    mockSonarrTemplate: {
      name: 'Sonarr',
      defaultUrl: 'http://localhost:8989',
      icon: 'sonarr',
      iconType: 'dashboard',
      color: '#00CCFF',
      iconBackground: '#002233',
      group: 'Downloads',
      description: 'TV show management',
    },
    mockPortainerTemplate: {
      name: 'Portainer',
      defaultUrl: 'http://localhost:9000',
      icon: 'portainer',
      iconType: 'dashboard',
      color: '#13BEF9',
      iconBackground: '#001D2E',
      group: 'System',
      description: 'Container management',
    },
  };
});

// --- Mocks ---

vi.mock('$lib/onboardingStore', () => ({
  currentStep: mockCurrentStep,
  selectedApps: mockSelectedApps,
  selectedNavigation: mockSelectedNavigation,
  showLabels: mockShowLabels,
  nextStep: mockNextStep,
  prevStep: mockPrevStep,
  stepProgress: mockStepProgress,
  configureSteps: mockConfigureSteps,
  activeStepOrder: mockActiveStepOrder,
  resetOnboarding: vi.fn(),
  goToStep: vi.fn(),
  getStepOrder: vi.fn(() => ['welcome', 'apps', 'navigation', 'theme', 'complete']),
  getTotalSteps: vi.fn(() => 5),
}));

vi.mock('$lib/popularApps', () => ({
  popularApps: {
    'Media': [mockPlexTemplate],
    'Downloads': [mockSonarrTemplate],
    'System': [mockPortainerTemplate],
  },
  getAllGroups: vi.fn(() => ['Media', 'Downloads', 'System']),
  templateToApp: mockTemplateToApp,
  getAllPopularApps: vi.fn(() => [mockPlexTemplate, mockSonarrTemplate, mockPortainerTemplate]),
  templatesByName: vi.fn(() => new Map([
    ['Plex', mockPlexTemplate],
    ['Sonarr', mockSonarrTemplate],
    ['Portainer', mockPortainerTemplate],
  ])),
}));

vi.mock('$lib/themeStore', () => ({
  themeFamilies: mockThemeFamilies,
  selectedFamily: mockSelectedFamily,
  variantMode: mockVariantMode,
  setThemeFamily: mockSetThemeFamily,
  setVariantMode: mockSetVariantMode,
  detectCustomThemes: mockDetectCustomThemes,
  systemTheme: mockSystemTheme,
  builtinThemes: [],
  customThemes: { subscribe: (fn: (v: unknown[]) => void) => { fn([]); return () => {}; } },
  allThemes: { subscribe: (fn: (v: unknown[]) => void) => { fn([]); return () => {}; } },
  resolvedTheme: { subscribe: (fn: (v: string) => void) => { fn('dark'); return () => {}; } },
  isDarkTheme: { subscribe: (fn: (v: boolean) => void) => { fn(true); return () => {}; } },
  initTheme: vi.fn(),
  syncFromConfig: vi.fn(),
}));

vi.mock('$lib/forwardAuthPresets', () => ({
  forwardAuthPresets: {
    authelia: { user: 'Remote-User', email: 'Remote-Email', groups: 'Remote-Groups', name: 'Remote-Name', logoutUrl: 'https://auth.example.com/logout' },
    authentik: { user: 'X-authentik-username', email: 'X-authentik-email', groups: 'X-authentik-groups', name: 'X-authentik-name', logoutUrl: 'https://auth.example.com/outpost.goauthentik.io/sign_out' },
    custom: { user: 'Remote-User', email: 'Remote-Email', groups: 'Remote-Groups', name: 'Remote-Name', logoutUrl: '' },
  },
  applyPreset: mockApplyPreset,
  buildForwardAuthRequest: mockBuildForwardAuthRequest,
  detectPreset: vi.fn(() => 'custom'),
}));

vi.mock('$lib/api', () => ({
  getBase: vi.fn(() => ''),
  API_BASE: '',
}));

vi.mock('$lib/constants', () => ({
  openModes: [
    { value: 'iframe', label: 'Embedded', description: 'Show inside Muximux' },
    { value: 'new_tab', label: 'New Tab', description: 'Open in a new browser tab' },
  ],
}));

vi.mock('$lib/debug', () => ({
  debug: vi.fn(),
}));

vi.mock('$lib/authStore', () => ({
  isAdmin: { subscribe: (fn: (v: boolean) => void) => { fn(true); return () => {}; } },
  currentUser: { subscribe: (fn: (v: unknown) => void) => { fn({ username: 'admin', role: 'admin' }); return () => {}; } },
  isAuthenticated: { subscribe: (fn: (v: boolean) => void) => { fn(true); return () => {}; } },
  logout: vi.fn(),
}));

vi.mock('$lib/healthStore', () => ({
  healthData: { subscribe: (fn: (v: Map<string, unknown>) => void) => { fn(new Map()); return () => {}; } },
  refreshHealth: vi.fn(),
  startHealthPolling: vi.fn(),
  stopHealthPolling: vi.fn(),
}));

vi.mock('$lib/useSwipe', () => ({
  isMobileViewport: vi.fn(() => false),
  isTouchDevice: vi.fn(() => false),
  createEdgeSwipeHandlers: vi.fn(() => ({ onpointerdown: vi.fn(), onpointermove: vi.fn(), onpointerup: vi.fn() })),
}));

vi.mock('$lib/keybindingsStore', () => ({
  keybindings: { subscribe: (fn: (v: unknown[]) => void) => { fn([]); return () => {}; } },
  formatKeybinding: vi.fn(() => ''),
}));

vi.mock('svelte-dnd-action', () => ({
  dndzone: () => ({ update: vi.fn(), destroy: vi.fn() }),
  TRIGGERS: { DRAG_STARTED: 'dragStarted' },
  SOURCES: { POINTER: 'pointer' },
}));

// Mock sub-components that OnboardingWizard imports as no-op Svelte 5 components
function noopComponent() {
  return { $destroy: vi.fn() };
}
vi.mock('./AppIcon.svelte', () => ({ default: noopComponent }));
vi.mock('./Navigation.svelte', () => ({ default: noopComponent }));
vi.mock('./IconBrowser.svelte', () => ({ default: noopComponent }));

import OnboardingWizard from './OnboardingWizard.svelte';

function renderWizard(props: { oncomplete?: (detail: unknown) => void; needsSetup?: boolean } = {}) {
  return render(OnboardingWizard, { props });
}

describe('OnboardingWizard', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Reset stores to initial state
    mockCurrentStep.set('welcome');
    mockStepProgress.set(0);
    mockSelectedApps.set([]);
    mockSelectedNavigation.set('left');
    mockShowLabels.set(true);
    mockSelectedFamily.set('default');
    mockVariantMode.set('dark');
    mockActiveStepOrder.set(['welcome', 'apps', 'navigation', 'theme', 'complete']);
    mockThemeFamilies.set([]);
    // Reset fetch mock
    vi.restoreAllMocks();
  });

  // =======================================================================
  // 1. Step navigation
  // =======================================================================
  describe('Step navigation', () => {
    it('renders the wizard container with dialog role', () => {
      const { container } = renderWizard();
      const dialog = container.querySelector('[role="dialog"]');
      expect(dialog).toBeTruthy();
    });

    it('shows step progress indicator with step labels (no security)', () => {
      renderWizard();
      expect(screen.getByText('Welcome')).toBeInTheDocument();
      expect(screen.getByText('Apps')).toBeInTheDocument();
      expect(screen.getByText('Style')).toBeInTheDocument();
      expect(screen.getByText('Theme')).toBeInTheDocument();
      expect(screen.getByText('Done')).toBeInTheDocument();
    });

    it('shows Security step label when needsSetup is true', () => {
      mockActiveStepOrder.set(['welcome', 'security', 'apps', 'navigation', 'theme', 'complete']);
      renderWizard({ needsSetup: true });
      expect(screen.getByText('Security')).toBeInTheDocument();
    });

    it('calls configureSteps on mount', () => {
      renderWizard({ needsSetup: true });
      expect(mockConfigureSteps).toHaveBeenCalledWith(true);
    });

    it('calls configureSteps(false) when needsSetup is not set', () => {
      renderWizard();
      expect(mockConfigureSteps).toHaveBeenCalledWith(false);
    });

    it('does not show Back button on welcome step', () => {
      renderWizard();
      expect(screen.queryByText('Back')).not.toBeInTheDocument();
    });

    it('shows Back button on non-welcome steps', () => {
      mockCurrentStep.set('apps');
      mockStepProgress.set(1);
      renderWizard();
      expect(screen.getByText('Back')).toBeInTheDocument();
    });

    it('calls prevStep when Back button is clicked', async () => {
      mockCurrentStep.set('apps');
      mockStepProgress.set(1);
      renderWizard();
      await fireEvent.click(screen.getByText('Back'));
      expect(mockPrevStep).toHaveBeenCalledTimes(1);
    });

    it('shows Continue button on apps step', () => {
      mockCurrentStep.set('apps');
      mockStepProgress.set(1);
      renderWizard();
      expect(screen.getByText('Continue')).toBeInTheDocument();
    });

    it('Continue button is disabled on apps step when no apps are selected', () => {
      mockCurrentStep.set('apps');
      mockStepProgress.set(1);
      mockSelectedApps.set([]);
      renderWizard();
      const continueBtn = screen.getByText('Continue');
      expect(continueBtn).toBeDisabled();
    });

    it('shows Finish instead of Continue on theme step', () => {
      mockCurrentStep.set('theme');
      mockStepProgress.set(3);
      renderWizard();
      expect(screen.getByText('Finish')).toBeInTheDocument();
    });

    it('does not show Continue or Finish on complete step', () => {
      mockCurrentStep.set('complete');
      mockStepProgress.set(4);
      renderWizard();
      expect(screen.queryByText('Continue')).not.toBeInTheDocument();
      expect(screen.queryByText('Finish')).not.toBeInTheDocument();
    });

    it('shows Continue on navigation step and calls nextStep', async () => {
      mockCurrentStep.set('navigation');
      mockStepProgress.set(2);
      renderWizard();
      const continueBtn = screen.getByText('Continue');
      expect(continueBtn).toBeInTheDocument();
      await fireEvent.click(continueBtn);
      expect(mockNextStep).toHaveBeenCalledTimes(1);
    });
  });

  // =======================================================================
  // 2. Welcome step
  // =======================================================================
  describe('Welcome step', () => {
    it('shows the Get Started button', () => {
      renderWizard();
      expect(screen.getByText("Let's Get Started")).toBeInTheDocument();
    });

    it('calls nextStep when Get Started is clicked', async () => {
      renderWizard();
      await fireEvent.click(screen.getByText("Let's Get Started"));
      expect(mockNextStep).toHaveBeenCalledTimes(1);
    });

    it('shows Restore from Backup button', () => {
      renderWizard();
      expect(screen.getByText('Restore from Backup')).toBeInTheDocument();
    });

    it('shows welcome heading and description', () => {
      renderWizard();
      expect(screen.getByText('Welcome to Muximux')).toBeInTheDocument();
    });

    it('shows feature highlights', () => {
      renderWizard();
      expect(screen.getByText('One Dashboard')).toBeInTheDocument();
      expect(screen.getByText('Built-in Proxy')).toBeInTheDocument();
      expect(screen.getByText('Quick Access')).toBeInTheDocument();
    });

    it('shows setup-specific description when needsSetup is true', () => {
      renderWizard({ needsSetup: true });
      expect(screen.getByText(/secure and set up your applications/)).toBeInTheDocument();
    });

    it('shows standard description when needsSetup is false', () => {
      renderWizard({ needsSetup: false });
      expect(screen.getByText(/set up your applications in a few quick steps/)).toBeInTheDocument();
    });

    it('has a hidden file input for backup restore', () => {
      const { container } = renderWizard();
      const fileInput = container.querySelector('input[type="file"][accept=".yaml,.yml"]');
      expect(fileInput).toBeTruthy();
      expect(fileInput?.className).toContain('hidden');
    });

    it('shows existing configuration prompt text', () => {
      renderWizard();
      expect(screen.getByText('Have an existing configuration?')).toBeInTheDocument();
    });
  });

  // =======================================================================
  // 2b. Config restore flow
  // =======================================================================
  describe('Config restore', () => {
    it('shows restoring state during file upload', async () => {
      // Mock fetch to hang (never resolve) so we can see restoring state
      const fetchPromise = new Promise(() => {}); // never resolves
      vi.spyOn(globalThis, 'fetch').mockReturnValue(fetchPromise as Promise<Response>);

      const { container } = renderWizard();
      const fileInput = container.querySelector('input[type="file"][accept=".yaml,.yml"]') as HTMLInputElement;

      // Create a mock file
      const file = new File(['test: config'], 'config.yaml', { type: 'application/x-yaml' });
      Object.defineProperty(fileInput, 'files', { value: [file], configurable: true });

      await fireEvent.change(fileInput);

      // The button text should change to "Restoring..."
      await waitFor(() => {
        expect(screen.getByText('Restoring...')).toBeInTheDocument();
      });
    });

    it('shows error when restore response is not ok', async () => {
      vi.spyOn(globalThis, 'fetch').mockResolvedValue(
        new Response(JSON.stringify({ error: 'Invalid config format' }), { status: 400, headers: { 'Content-Type': 'application/json' } })
      );

      const { container } = renderWizard();
      const fileInput = container.querySelector('input[type="file"][accept=".yaml,.yml"]') as HTMLInputElement;

      const file = new File(['bad: config'], 'config.yaml', { type: 'application/x-yaml' });
      Object.defineProperty(fileInput, 'files', { value: [file], configurable: true });

      await fireEvent.change(fileInput);

      await waitFor(() => {
        expect(screen.getByText('Invalid config format')).toBeInTheDocument();
      });
    });

    it('shows generic error when restore response has no JSON body', async () => {
      vi.spyOn(globalThis, 'fetch').mockResolvedValue(
        new Response('Server Error', { status: 500 })
      );

      const { container } = renderWizard();
      const fileInput = container.querySelector('input[type="file"][accept=".yaml,.yml"]') as HTMLInputElement;

      const file = new File(['bad: config'], 'config.yaml', { type: 'application/x-yaml' });
      Object.defineProperty(fileInput, 'files', { value: [file], configurable: true });

      await fireEvent.change(fileInput);

      await waitFor(() => {
        expect(screen.getByText('Restore failed (500)')).toBeInTheDocument();
      });
    });

    it('shows error when fetch throws a network error', async () => {
      vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('Network error'));

      const { container } = renderWizard();
      const fileInput = container.querySelector('input[type="file"][accept=".yaml,.yml"]') as HTMLInputElement;

      const file = new File(['test: config'], 'config.yaml', { type: 'application/x-yaml' });
      Object.defineProperty(fileInput, 'files', { value: [file], configurable: true });

      await fireEvent.change(fileInput);

      await waitFor(() => {
        expect(screen.getByText('Network error')).toBeInTheDocument();
      });
    });

    it('shows fallback error message for non-Error throws', async () => {
      vi.spyOn(globalThis, 'fetch').mockRejectedValue('unknown error');

      const { container } = renderWizard();
      const fileInput = container.querySelector('input[type="file"][accept=".yaml,.yml"]') as HTMLInputElement;

      const file = new File(['test: config'], 'config.yaml', { type: 'application/x-yaml' });
      Object.defineProperty(fileInput, 'files', { value: [file], configurable: true });

      await fireEvent.change(fileInput);

      await waitFor(() => {
        expect(screen.getByText('Failed to restore config')).toBeInTheDocument();
      });
    });

    it('does nothing when no file is selected', async () => {
      const fetchSpy = vi.spyOn(globalThis, 'fetch');
      const { container } = renderWizard();
      const fileInput = container.querySelector('input[type="file"][accept=".yaml,.yml"]') as HTMLInputElement;

      Object.defineProperty(fileInput, 'files', { value: [], configurable: true });
      await fireEvent.change(fileInput);

      expect(fetchSpy).not.toHaveBeenCalled();
    });

    it('calls location.reload on successful restore', async () => {
      vi.spyOn(globalThis, 'fetch').mockResolvedValue(
        new Response(JSON.stringify({ success: true }), { status: 200, headers: { 'Content-Type': 'application/json' } })
      );

      const reloadMock = vi.fn();
      Object.defineProperty(globalThis, 'location', {
        value: { reload: reloadMock },
        writable: true,
        configurable: true,
      });

      const { container } = renderWizard();
      const fileInput = container.querySelector('input[type="file"][accept=".yaml,.yml"]') as HTMLInputElement;

      const file = new File(['test: config'], 'config.yaml', { type: 'application/x-yaml' });
      Object.defineProperty(fileInput, 'files', { value: [file], configurable: true });

      await fireEvent.change(fileInput);

      await waitFor(() => {
        expect(reloadMock).toHaveBeenCalled();
      });
    });
  });

  // =======================================================================
  // 3. Security step
  // =======================================================================
  describe('Security step', () => {
    beforeEach(() => {
      mockActiveStepOrder.set(['welcome', 'security', 'apps', 'navigation', 'theme', 'complete']);
      mockCurrentStep.set('security');
      mockStepProgress.set(1);
    });

    it('shows security step heading', () => {
      renderWizard({ needsSetup: true });
      expect(screen.getByText('Secure Your Dashboard')).toBeInTheDocument();
      expect(screen.getByText('Choose how you want to protect access to Muximux')).toBeInTheDocument();
    });

    it('shows three auth method options', () => {
      renderWizard({ needsSetup: true });
      expect(screen.getByText('Create a password')).toBeInTheDocument();
      expect(screen.getByText('I use an auth proxy')).toBeInTheDocument();
      expect(screen.getByText('No authentication')).toBeInTheDocument();
    });

    it('shows the Recommended badge on builtin password', () => {
      renderWizard({ needsSetup: true });
      expect(screen.getByText('Recommended')).toBeInTheDocument();
    });

    it('expands builtin auth form when Create a password is clicked', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('Create a password'));
      await waitFor(() => {
        expect(screen.getByLabelText('Username')).toBeInTheDocument();
        expect(screen.getByLabelText('Password')).toBeInTheDocument();
        expect(screen.getByLabelText('Confirm password')).toBeInTheDocument();
      });
    });

    it('collapses builtin form when clicking Create a password again', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('Create a password'));
      await waitFor(() => {
        expect(screen.getByLabelText('Username')).toBeInTheDocument();
      });
      // Click again to collapse
      await fireEvent.click(screen.getByText('Create a password'));
      await waitFor(() => {
        expect(screen.queryByLabelText('Username')).not.toBeInTheDocument();
      });
    });

    it('shows password validation error when password is too short', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('Create a password'));
      await waitFor(() => {
        expect(screen.getByLabelText('Password')).toBeInTheDocument();
      });
      const passwordInput = screen.getByLabelText('Password');
      await fireEvent.input(passwordInput, { target: { value: 'short' } });
      await waitFor(() => {
        expect(screen.getByText('Password must be at least 8 characters')).toBeInTheDocument();
      });
    });

    it('shows password mismatch error when passwords differ', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('Create a password'));
      await waitFor(() => {
        expect(screen.getByLabelText('Password')).toBeInTheDocument();
      });
      const passwordInput = screen.getByLabelText('Password');
      const confirmInput = screen.getByLabelText('Confirm password');
      await fireEvent.input(passwordInput, { target: { value: 'password123' } });
      await fireEvent.input(confirmInput, { target: { value: 'different' } });
      await waitFor(() => {
        expect(screen.getByText('Passwords do not match')).toBeInTheDocument();
      });
    });

    it('Continue is enabled when builtin form is valid', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('Create a password'));
      await waitFor(() => {
        expect(screen.getByLabelText('Username')).toBeInTheDocument();
      });
      const usernameInput = screen.getByLabelText('Username');
      const passwordInput = screen.getByLabelText('Password');
      const confirmInput = screen.getByLabelText('Confirm password');
      await fireEvent.input(usernameInput, { target: { value: 'admin' } });
      await fireEvent.input(passwordInput, { target: { value: 'password123' } });
      await fireEvent.input(confirmInput, { target: { value: 'password123' } });
      await waitFor(() => {
        const continueBtn = screen.getByText('Continue');
        expect(continueBtn).not.toBeDisabled();
      });
    });

    it('clicking Continue on security step with valid builtin calls nextStep', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('Create a password'));
      await waitFor(() => {
        expect(screen.getByLabelText('Password')).toBeInTheDocument();
      });
      const usernameInput = screen.getByLabelText('Username');
      const passwordInput = screen.getByLabelText('Password');
      const confirmInput = screen.getByLabelText('Confirm password');
      await fireEvent.input(usernameInput, { target: { value: 'admin' } });
      await fireEvent.input(passwordInput, { target: { value: 'password123' } });
      await fireEvent.input(confirmInput, { target: { value: 'password123' } });
      await waitFor(() => {
        expect(screen.getByText('Continue')).not.toBeDisabled();
      });
      await fireEvent.click(screen.getByText('Continue'));
      expect(mockNextStep).toHaveBeenCalledTimes(1);
    });

    it('expands forward auth form when auth proxy is clicked', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('I use an auth proxy'));
      await waitFor(() => {
        expect(screen.getByLabelText('Trusted proxy IPs')).toBeInTheDocument();
      });
    });

    it('shows forward auth preset buttons when expanded', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('I use an auth proxy'));
      await waitFor(() => {
        expect(screen.getByText('Authelia')).toBeInTheDocument();
        expect(screen.getByText('Authentik')).toBeInTheDocument();
        expect(screen.getByText('Custom')).toBeInTheDocument();
      });
    });

    it('clicking a forward auth preset calls applyPreset', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('I use an auth proxy'));
      await waitFor(() => {
        expect(screen.getByText('Authentik')).toBeInTheDocument();
      });
      await fireEvent.click(screen.getByText('Authentik'));
      expect(mockApplyPreset).toHaveBeenCalledWith('authentik', expect.any(String));
    });

    it('Continue is disabled on forward auth until trusted proxies filled', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('I use an auth proxy'));
      await waitFor(() => {
        expect(screen.getByLabelText('Trusted proxy IPs')).toBeInTheDocument();
      });
      const continueBtn = screen.getByText('Continue');
      expect(continueBtn).toBeDisabled();
    });

    it('Continue is enabled on forward auth when trusted proxies filled', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('I use an auth proxy'));
      await waitFor(() => {
        expect(screen.getByLabelText('Trusted proxy IPs')).toBeInTheDocument();
      });
      const proxiesInput = screen.getByLabelText('Trusted proxy IPs');
      await fireEvent.input(proxiesInput, { target: { value: '10.0.0.1/32' } });
      await waitFor(() => {
        const continueBtn = screen.getByText('Continue');
        expect(continueBtn).not.toBeDisabled();
      });
    });

    it('expands no-auth section with risk warning when clicked', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('No authentication'));
      await waitFor(() => {
        expect(screen.getByText('Security warning')).toBeInTheDocument();
        expect(screen.getByText(/understand the risks/)).toBeInTheDocument();
      });
    });

    it('Continue is disabled on no-auth until risk acknowledged', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('No authentication'));
      await waitFor(() => {
        expect(screen.getByText('Security warning')).toBeInTheDocument();
      });
      const continueBtn = screen.getByText('Continue');
      expect(continueBtn).toBeDisabled();
    });

    it('Continue is enabled after acknowledging risk in no-auth', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('No authentication'));
      await waitFor(() => {
        expect(screen.getByText(/understand the risks/)).toBeInTheDocument();
      });
      const checkbox = screen.getByRole('checkbox');
      await fireEvent.click(checkbox);
      await waitFor(() => {
        const continueBtn = screen.getByText('Continue');
        expect(continueBtn).not.toBeDisabled();
      });
    });

    it('Continue is disabled on security step when no auth method selected', () => {
      renderWizard({ needsSetup: true });
      const continueBtn = screen.getByText('Continue');
      expect(continueBtn).toBeDisabled();
    });

    it('description mentions auth proxy providers', () => {
      renderWizard({ needsSetup: true });
      expect(screen.getByText(/Authelia, Authentik, or another reverse proxy/)).toBeInTheDocument();
    });

    it('shows logout URL field when forward auth is expanded', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('I use an auth proxy'));
      await waitFor(() => {
        expect(screen.getByLabelText('Logout URL')).toBeInTheDocument();
      });
    });

    it('shows advanced header names toggle when forward auth is expanded', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('I use an auth proxy'));
      await waitFor(() => {
        expect(screen.getByText(/Advanced: Header names/)).toBeInTheDocument();
      });
    });

    it('shows advanced header fields when toggle is clicked', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('I use an auth proxy'));
      await waitFor(() => {
        expect(screen.getByText(/Advanced: Header names/)).toBeInTheDocument();
      });
      await fireEvent.click(screen.getByText(/Advanced: Header names/));
      await waitFor(() => {
        expect(screen.getByLabelText('User header')).toBeInTheDocument();
        expect(screen.getByLabelText('Email header')).toBeInTheDocument();
        expect(screen.getByLabelText('Groups header')).toBeInTheDocument();
        expect(screen.getByLabelText('Name header')).toBeInTheDocument();
      });
    });

    it('switching between auth methods toggles forms', async () => {
      renderWizard({ needsSetup: true });
      // First select builtin
      await fireEvent.click(screen.getByText('Create a password'));
      await waitFor(() => {
        expect(screen.getByLabelText('Username')).toBeInTheDocument();
      });
      // Switch to forward auth
      await fireEvent.click(screen.getByText('I use an auth proxy'));
      await waitFor(() => {
        expect(screen.queryByLabelText('Username')).not.toBeInTheDocument();
        expect(screen.getByLabelText('Trusted proxy IPs')).toBeInTheDocument();
      });
    });

    it('shows auth method status text in footer when builtin selected', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('Create a password'));
      await waitFor(() => {
        // Footer status text shows "Password" for builtin method
        const footerStatus = document.querySelector('.text-sm.text-text-disabled');
        expect(footerStatus?.textContent?.trim()).toBe('Password');
      });
    });

    it('shows "Auth proxy" status text for forward auth', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('I use an auth proxy'));
      await waitFor(() => {
        expect(screen.getByText('Auth proxy')).toBeInTheDocument();
      });
    });

    it('shows "No auth" status text for none', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('No authentication'));
      await waitFor(() => {
        expect(screen.getByText('No auth')).toBeInTheDocument();
      });
    });

    it('shows proxy type description text', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('I use an auth proxy'));
      await waitFor(() => {
        expect(screen.getByText('Proxy type')).toBeInTheDocument();
      });
    });

    it('shows IP address help text in forward auth', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('I use an auth proxy'));
      await waitFor(() => {
        expect(screen.getByText('IP addresses or CIDR ranges, one per line')).toBeInTheDocument();
      });
    });
  });

  // =======================================================================
  // 4. Apps step
  // =======================================================================
  describe('Apps step', () => {
    beforeEach(() => {
      mockCurrentStep.set('apps');
      mockStepProgress.set(1);
    });

    it('shows apps step heading', () => {
      renderWizard();
      expect(screen.getByText('What apps do you have?')).toBeInTheDocument();
      expect(screen.getByText("Select the services you're already running")).toBeInTheDocument();
    });

    it('shows App Catalog section', () => {
      renderWizard();
      expect(screen.getByText('App Catalog')).toBeInTheDocument();
      expect(screen.getByText('Click to add apps to your menu')).toBeInTheDocument();
    });

    it('shows custom app form fields', () => {
      renderWizard();
      expect(screen.getByPlaceholderText('App name')).toBeInTheDocument();
      expect(screen.getByPlaceholderText('http://localhost:8080')).toBeInTheDocument();
    });

    it('shows Add button for custom app (disabled when empty)', () => {
      renderWizard();
      const addBtn = screen.getByRole('button', { name: 'Add' });
      expect(addBtn).toBeInTheDocument();
      expect(addBtn).toBeDisabled();
    });

    it('shows Your Menu section', () => {
      renderWizard();
      expect(screen.getByText('Your Menu')).toBeInTheDocument();
    });

    it('shows empty state message when no apps selected', () => {
      renderWizard();
      expect(screen.getByText('Select apps from the left to get started')).toBeInTheDocument();
    });

    it('shows status text with 0 apps selected', () => {
      renderWizard();
      expect(screen.getByText('0 apps selected')).toBeInTheDocument();
    });

    it('Continue is disabled when no apps are selected', () => {
      renderWizard();
      const continueBtn = screen.getByText('Continue');
      expect(continueBtn).toBeDisabled();
    });

    it('renders popular app templates in categories', () => {
      renderWizard();
      // Check for category headers
      expect(screen.getByText('Media')).toBeInTheDocument();
      expect(screen.getByText('Downloads')).toBeInTheDocument();
      expect(screen.getByText('System')).toBeInTheDocument();
    });

    it('renders popular app names', () => {
      renderWizard();
      expect(screen.getByText('Plex')).toBeInTheDocument();
      expect(screen.getByText('Sonarr')).toBeInTheDocument();
      expect(screen.getByText('Portainer')).toBeInTheDocument();
    });

    it('renders app descriptions', () => {
      renderWizard();
      expect(screen.getByText('Stream your media library')).toBeInTheDocument();
      expect(screen.getByText('TV show management')).toBeInTheDocument();
      expect(screen.getByText('Container management')).toBeInTheDocument();
    });

    it('toggles app selection when an app card is clicked', async () => {
      renderWizard();
      // Find the Plex card and click it
      const plexCard = screen.getByRole('checkbox', { name: /Plex/i });
      expect(plexCard).toHaveAttribute('aria-checked', 'false');
      await fireEvent.click(plexCard);
      await waitFor(() => {
        expect(plexCard).toHaveAttribute('aria-checked', 'true');
      });
    });

    it('shows selected app count after selecting an app', async () => {
      renderWizard();
      const plexCard = screen.getByRole('checkbox', { name: /Plex/i });
      await fireEvent.click(plexCard);
      await waitFor(() => {
        expect(screen.getByText('1 app selected')).toBeInTheDocument();
      });
    });

    it('deselects app on second click', async () => {
      renderWizard();
      const plexCard = screen.getByRole('checkbox', { name: /Plex/i });
      await fireEvent.click(plexCard);
      await waitFor(() => {
        expect(plexCard).toHaveAttribute('aria-checked', 'true');
      });
      await fireEvent.click(plexCard);
      await waitFor(() => {
        expect(plexCard).toHaveAttribute('aria-checked', 'false');
      });
    });

    it('enables Continue when apps are selected', async () => {
      renderWizard();
      const plexCard = screen.getByRole('checkbox', { name: /Plex/i });
      await fireEvent.click(plexCard);
      await waitFor(() => {
        const continueBtn = screen.getByText('Continue');
        expect(continueBtn).not.toBeDisabled();
      });
    });

    it('enables Add button when both custom app fields have values', async () => {
      renderWizard();
      const nameInput = screen.getByPlaceholderText('App name');
      const urlInput = screen.getByPlaceholderText('http://localhost:8080');

      await fireEvent.input(nameInput, { target: { value: 'MyApp' } });
      await fireEvent.input(urlInput, { target: { value: 'http://localhost:3000' } });

      await waitFor(() => {
        const addBtn = screen.getByRole('button', { name: 'Add' });
        expect(addBtn).not.toBeDisabled();
      });
    });

    it('clears custom app form after adding', async () => {
      renderWizard();
      const nameInput = screen.getByPlaceholderText('App name') as HTMLInputElement;
      const urlInput = screen.getByPlaceholderText('http://localhost:8080') as HTMLInputElement;

      await fireEvent.input(nameInput, { target: { value: 'MyApp' } });
      await fireEvent.input(urlInput, { target: { value: 'http://localhost:3000' } });

      const addBtn = screen.getByRole('button', { name: 'Add' });
      await fireEvent.click(addBtn);

      // After adding, the Add button should be disabled again (fields cleared)
      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Add' })).toBeDisabled();
      });
    });

    it('toggles app via keyboard Enter', async () => {
      renderWizard();
      const plexCard = screen.getByRole('checkbox', { name: /Plex/i });
      await fireEvent.keyDown(plexCard, { key: 'Enter' });
      await waitFor(() => {
        expect(plexCard).toHaveAttribute('aria-checked', 'true');
      });
    });

    it('toggles app via keyboard Space', async () => {
      renderWizard();
      const plexCard = screen.getByRole('checkbox', { name: /Plex/i });
      await fireEvent.keyDown(plexCard, { key: ' ' });
      await waitFor(() => {
        expect(plexCard).toHaveAttribute('aria-checked', 'true');
      });
    });

    it('shows custom app label', () => {
      renderWizard();
      expect(screen.getByText('Custom app')).toBeInTheDocument();
    });

    it('shows Add Group button when apps are selected', async () => {
      renderWizard();
      const plexCard = screen.getByRole('checkbox', { name: /Plex/i });
      await fireEvent.click(plexCard);
      await waitFor(() => {
        expect(screen.getByText('Add Group')).toBeInTheDocument();
      });
    });
  });

  // =======================================================================
  // 5. Style step (navigation)
  // =======================================================================
  describe('Style step (navigation)', () => {
    beforeEach(() => {
      mockCurrentStep.set('navigation');
      mockStepProgress.set(2);
    });

    it('shows navigation step heading', () => {
      renderWizard();
      expect(screen.getByText('Choose Your Navigation Style')).toBeInTheDocument();
      expect(screen.getByText('Select how you want to navigate between your apps')).toBeInTheDocument();
    });

    it('shows all navigation position options', () => {
      renderWizard();
      expect(screen.getByText('Top Bar')).toBeInTheDocument();
      expect(screen.getByText('Left Sidebar')).toBeInTheDocument();
      expect(screen.getByText('Right Sidebar')).toBeInTheDocument();
      expect(screen.getByText('Bottom Bar')).toBeInTheDocument();
      expect(screen.getByText('Floating')).toBeInTheDocument();
    });

    it('shows settings controls for label and logo toggles', () => {
      renderWizard();
      expect(screen.getByText('Show Labels')).toBeInTheDocument();
      expect(screen.getByText('Show Logo')).toBeInTheDocument();
      expect(screen.getByText('App Color Accents')).toBeInTheDocument();
      expect(screen.getByText('Icon Background')).toBeInTheDocument();
    });

    it('shows Icon Size control', () => {
      renderWizard();
      expect(screen.getByText('Icon Size')).toBeInTheDocument();
      expect(screen.getByText('Scale app icons in the navigation')).toBeInTheDocument();
    });

    it('shows Start on Overview toggle', () => {
      renderWizard();
      expect(screen.getByText('Start on Overview')).toBeInTheDocument();
    });

    it('shows Auto-hide Menu toggle', () => {
      renderWizard();
      expect(screen.getByText('Auto-hide Menu')).toBeInTheDocument();
    });

    it('shows Continue button on style step', () => {
      renderWizard();
      expect(screen.getByText('Continue')).toBeInTheDocument();
    });

    it('calls nextStep when Continue is clicked on style step', async () => {
      renderWizard();
      await fireEvent.click(screen.getByText('Continue'));
      expect(mockNextStep).toHaveBeenCalledTimes(1);
    });

    it('shows floating position options when floating is selected', async () => {
      mockSelectedNavigation.set('floating');
      renderWizard();
      await waitFor(() => {
        expect(screen.getByText('Bottom Right')).toBeInTheDocument();
        expect(screen.getByText('Bottom Left')).toBeInTheDocument();
        expect(screen.getByText('Top Right')).toBeInTheDocument();
        expect(screen.getByText('Top Left')).toBeInTheDocument();
      });
    });

    it('shows bar style options when top navigation is selected', async () => {
      mockSelectedNavigation.set('top');
      renderWizard();
      await waitFor(() => {
        expect(screen.getByText('Group Dropdowns')).toBeInTheDocument();
        expect(screen.getByText('Flat List')).toBeInTheDocument();
      });
    });

    it('shows bar style options when bottom navigation is selected', async () => {
      mockSelectedNavigation.set('bottom');
      renderWizard();
      await waitFor(() => {
        expect(screen.getByText('Group Dropdowns')).toBeInTheDocument();
        expect(screen.getByText('Flat List')).toBeInTheDocument();
      });
    });

    it('shows Collapsible Footer option for left sidebar', () => {
      mockSelectedNavigation.set('left');
      renderWizard();
      expect(screen.getByText('Collapsible Footer')).toBeInTheDocument();
    });

    it('shows Collapsible Footer option for right sidebar', () => {
      mockSelectedNavigation.set('right');
      renderWizard();
      expect(screen.getByText('Collapsible Footer')).toBeInTheDocument();
    });

    it('does not show Collapsible Footer for top position', () => {
      mockSelectedNavigation.set('top');
      renderWizard();
      expect(screen.queryByText('Collapsible Footer')).not.toBeInTheDocument();
    });

    it('shows helper descriptions for settings', () => {
      renderWizard();
      expect(screen.getByText('Display app names next to icons')).toBeInTheDocument();
      expect(screen.getByText('Display the Muximux logo in the menu')).toBeInTheDocument();
      expect(screen.getByText('Highlight the active app with its color')).toBeInTheDocument();
      expect(screen.getByText('Show colored circle behind app icons')).toBeInTheDocument();
    });
  });

  // =======================================================================
  // 6. Theme step
  // =======================================================================
  describe('Theme step', () => {
    beforeEach(() => {
      mockCurrentStep.set('theme');
      mockStepProgress.set(3);
    });

    it('shows theme step heading', () => {
      renderWizard();
      expect(screen.getByText('Choose Your Theme')).toBeInTheDocument();
      expect(screen.getByText('Pick a visual style for your dashboard')).toBeInTheDocument();
    });

    it('shows variant mode selector buttons (Dark, System, Light)', () => {
      renderWizard();
      expect(screen.getByText('Dark')).toBeInTheDocument();
      expect(screen.getByText('System')).toBeInTheDocument();
      expect(screen.getByText('Light')).toBeInTheDocument();
    });

    it('calls setVariantMode when Dark is clicked', async () => {
      renderWizard();
      await fireEvent.click(screen.getByText('Dark'));
      expect(mockSetVariantMode).toHaveBeenCalledWith('dark');
    });

    it('calls setVariantMode when Light is clicked', async () => {
      renderWizard();
      await fireEvent.click(screen.getByText('Light'));
      expect(mockSetVariantMode).toHaveBeenCalledWith('light');
    });

    it('calls setVariantMode when System is clicked', async () => {
      renderWizard();
      await fireEvent.click(screen.getByText('System'));
      expect(mockSetVariantMode).toHaveBeenCalledWith('system');
    });

    it('shows informational text about theme customization', () => {
      renderWizard();
      expect(screen.getByText(/Changes apply live/)).toBeInTheDocument();
    });

    it('renders theme family grid when families are available', () => {
      mockThemeFamilies.set([
        {
          id: 'catppuccin',
          name: 'Catppuccin',
          description: 'Pastel theme',
          darkTheme: { preview: { bg: '#1e1e2e', surface: '#313244', accent: '#cba6f7' } },
          lightTheme: null,
        },
      ]);
      renderWizard();
      expect(screen.getByText('Catppuccin')).toBeInTheDocument();
      expect(screen.getByText('Pastel theme')).toBeInTheDocument();
    });

    it('calls setThemeFamily when a theme family card is clicked', async () => {
      mockThemeFamilies.set([
        {
          id: 'dracula',
          name: 'Dracula',
          description: 'Dark vampire theme',
          darkTheme: { preview: { bg: '#282a36', surface: '#44475a', accent: '#bd93f9' } },
          lightTheme: null,
        },
      ]);
      renderWizard();
      await fireEvent.click(screen.getByText('Dracula'));
      expect(mockSetThemeFamily).toHaveBeenCalledWith('dracula');
    });

    it('shows Finish button on theme step', () => {
      renderWizard();
      expect(screen.getByText('Finish')).toBeInTheDocument();
    });

    it('calls nextStep when Finish is clicked', async () => {
      renderWizard();
      await fireEvent.click(screen.getByText('Finish'));
      expect(mockNextStep).toHaveBeenCalledTimes(1);
    });

    it('renders multiple theme families', () => {
      mockThemeFamilies.set([
        {
          id: 'catppuccin',
          name: 'Catppuccin',
          description: 'Pastel theme',
          darkTheme: { preview: { bg: '#1e1e2e', surface: '#313244', accent: '#cba6f7' } },
          lightTheme: null,
        },
        {
          id: 'dracula',
          name: 'Dracula',
          description: 'Dark vampire theme',
          darkTheme: { preview: { bg: '#282a36', surface: '#44475a', accent: '#bd93f9' } },
          lightTheme: null,
        },
        {
          id: 'default',
          name: 'Default',
          description: 'Built-in theme',
          darkTheme: { preview: { bg: '#111827', surface: '#1f2937', accent: '#6366f1' } },
          lightTheme: { preview: { bg: '#ffffff', surface: '#f3f4f6', accent: '#6366f1' } },
        },
      ]);
      renderWizard();
      expect(screen.getByText('Catppuccin')).toBeInTheDocument();
      expect(screen.getByText('Dracula')).toBeInTheDocument();
      expect(screen.getByText('Default')).toBeInTheDocument();
    });

    it('shows theme family without preview swatches when no preview', () => {
      mockThemeFamilies.set([
        {
          id: 'no-preview',
          name: 'NoPreview',
          description: 'Theme without preview',
          darkTheme: null,
          lightTheme: null,
        },
      ]);
      renderWizard();
      expect(screen.getByText('NoPreview')).toBeInTheDocument();
      expect(screen.getByText('Theme without preview')).toBeInTheDocument();
    });

    it('shows theme family without description', () => {
      mockThemeFamilies.set([
        {
          id: 'minimal',
          name: 'Minimal',
          description: '',
          darkTheme: { preview: { bg: '#000', surface: '#111', accent: '#fff' } },
          lightTheme: null,
        },
      ]);
      renderWizard();
      expect(screen.getByText('Minimal')).toBeInTheDocument();
    });

    it('shows custom themes text at bottom', () => {
      renderWizard();
      expect(screen.getByText(/you can create custom themes later in Settings/)).toBeInTheDocument();
    });
  });

  // =======================================================================
  // 7. Done step (complete)
  // =======================================================================
  describe('Done step', () => {
    beforeEach(() => {
      mockCurrentStep.set('complete');
      mockStepProgress.set(4);
    });

    it('shows completion heading', () => {
      renderWizard();
      expect(screen.getByText("You're All Set!")).toBeInTheDocument();
    });

    it('shows Launch Dashboard button', () => {
      renderWizard();
      expect(screen.getByText('Launch Dashboard')).toBeInTheDocument();
    });

    it('shows setup summary with expected fields', () => {
      renderWizard();
      expect(screen.getByText('Setup Summary')).toBeInTheDocument();
      expect(screen.getByText('Applications')).toBeInTheDocument();
      expect(screen.getByText('Navigation')).toBeInTheDocument();
      expect(screen.getByText('Groups')).toBeInTheDocument();
      expect(screen.getByText('Show Labels')).toBeInTheDocument();
    });

    it('shows app count of 0 when no apps selected', () => {
      renderWizard();
      expect(screen.getByText(/0 apps?\./)).toBeInTheDocument();
    });

    it('calls oncomplete when Launch Dashboard is clicked', async () => {
      const oncomplete = vi.fn();
      renderWizard({ oncomplete });
      await fireEvent.click(screen.getByText('Launch Dashboard'));
      expect(oncomplete).toHaveBeenCalledTimes(1);
    });

    it('oncomplete is called with apps, navigation, groups, and theme', async () => {
      const oncomplete = vi.fn();
      renderWizard({ oncomplete });
      await fireEvent.click(screen.getByText('Launch Dashboard'));

      const arg = oncomplete.mock.calls[0][0] as Record<string, unknown>;
      expect(arg).toHaveProperty('apps');
      expect(arg).toHaveProperty('navigation');
      expect(arg).toHaveProperty('groups');
      expect(arg).toHaveProperty('theme');
      expect(Array.isArray(arg.apps)).toBe(true);
      expect(Array.isArray(arg.groups)).toBe(true);
    });

    it('shows Back button on complete step', () => {
      renderWizard();
      expect(screen.getByText('Back')).toBeInTheDocument();
    });

    it('shows navigation position in summary', () => {
      mockSelectedNavigation.set('left');
      renderWizard();
      expect(screen.getByText('left')).toBeInTheDocument();
    });

    it('shows Yes for show labels when enabled', () => {
      mockShowLabels.set(true);
      renderWizard();
      expect(screen.getByText('Yes')).toBeInTheDocument();
    });

    it('shows No for show labels when disabled', () => {
      mockShowLabels.set(false);
      renderWizard();
      expect(screen.getByText('No')).toBeInTheDocument();
    });

    it('shows theme name in summary', () => {
      mockThemeFamilies.set([
        { id: 'catppuccin', name: 'Catppuccin', description: '', darkTheme: null, lightTheme: null },
      ]);
      mockSelectedFamily.set('catppuccin');
      renderWizard();
      expect(screen.getByText('Catppuccin')).toBeInTheDocument();
    });

    it('shows default family name when no matching theme family', () => {
      mockSelectedFamily.set('default');
      mockThemeFamilies.set([]);
      renderWizard();
      // Falls back to $selectedFamily value
      expect(screen.getByText('default')).toBeInTheDocument();
    });

    it('navigation config has correct position value', async () => {
      mockSelectedNavigation.set('right');
      const oncomplete = vi.fn();
      renderWizard({ oncomplete });
      await fireEvent.click(screen.getByText('Launch Dashboard'));
      const arg = oncomplete.mock.calls[0][0] as Record<string, unknown>;
      const nav = arg.navigation as { position: string };
      expect(nav.position).toBe('right');
    });

    it('theme config has correct family and variant', async () => {
      mockSelectedFamily.set('dracula');
      mockVariantMode.set('light');
      const oncomplete = vi.fn();
      renderWizard({ oncomplete });
      await fireEvent.click(screen.getByText('Launch Dashboard'));
      const arg = oncomplete.mock.calls[0][0] as Record<string, unknown>;
      const theme = arg.theme as { family: string; variant: string };
      expect(theme.family).toBe('dracula');
      expect(theme.variant).toBe('light');
    });

    it('does not include setup when needsSetup is false', async () => {
      const oncomplete = vi.fn();
      renderWizard({ oncomplete, needsSetup: false });
      await fireEvent.click(screen.getByText('Launch Dashboard'));
      const arg = oncomplete.mock.calls[0][0] as Record<string, unknown>;
      expect(arg.setup).toBeUndefined();
    });

    it('renders without oncomplete callback without crashing', async () => {
      renderWizard();
      // Should not throw when clicking Launch Dashboard without callback
      await fireEvent.click(screen.getByText('Launch Dashboard'));
    });
  });

  // =======================================================================
  // 8. Theme initialization
  // =======================================================================
  describe('Theme initialization', () => {
    it('resets theme to default dark on mount', () => {
      renderWizard();
      expect(mockSetThemeFamily).toHaveBeenCalledWith('default');
      expect(mockSetVariantMode).toHaveBeenCalledWith('dark');
    });

    it('calls detectCustomThemes on mount', () => {
      renderWizard();
      expect(mockDetectCustomThemes).toHaveBeenCalledTimes(1);
    });
  });

  // =======================================================================
  // 9. Rendering edge cases
  // =======================================================================
  describe('Rendering edge cases', () => {
    it('renders without crashing with needsSetup=true', () => {
      mockActiveStepOrder.set(['welcome', 'security', 'apps', 'navigation', 'theme', 'complete']);
      const { container } = renderWizard({ needsSetup: true });
      expect(container.querySelector('[role="dialog"]')).toBeTruthy();
    });

    it('renders without crashing with needsSetup=false', () => {
      const { container } = renderWizard({ needsSetup: false });
      expect(container.querySelector('[role="dialog"]')).toBeTruthy();
    });

    it('renders without oncomplete callback', () => {
      const { container } = renderWizard();
      expect(container.querySelector('[role="dialog"]')).toBeTruthy();
    });

    it('renders apps step with popular apps templates', () => {
      mockCurrentStep.set('apps');
      mockStepProgress.set(1);
      renderWizard();
      expect(screen.getByText('What apps do you have?')).toBeInTheDocument();
      expect(screen.getByText('Plex')).toBeInTheDocument();
    });

    it('renders theme step with empty theme families', () => {
      mockCurrentStep.set('theme');
      mockStepProgress.set(3);
      mockThemeFamilies.set([]);
      renderWizard();
      expect(screen.getByText('Choose Your Theme')).toBeInTheDocument();
    });

    it('renders correctly for all step store values', () => {
      const steps = ['welcome', 'apps', 'navigation', 'theme', 'complete'];
      for (let i = 0; i < steps.length; i++) {
        mockCurrentStep.set(steps[i]);
        mockStepProgress.set(i);
        const { container, unmount } = renderWizard();
        expect(container.querySelector('[role="dialog"]')).toBeTruthy();
        unmount();
      }
    });

    it('renders security step without crashing', () => {
      mockActiveStepOrder.set(['welcome', 'security', 'apps', 'navigation', 'theme', 'complete']);
      mockCurrentStep.set('security');
      mockStepProgress.set(1);
      const { container } = renderWizard({ needsSetup: true });
      expect(container.querySelector('[role="dialog"]')).toBeTruthy();
    });
  });

  // =======================================================================
  // 10. Stepper visual state
  // =======================================================================
  describe('Stepper visual state', () => {
    it('marks current step as active in stepper', () => {
      mockStepProgress.set(0);
      const { container } = renderWizard();
      const activeCircle = container.querySelector('.stepper-circle.active');
      expect(activeCircle).toBeTruthy();
    });

    it('marks completed steps with checkmark', () => {
      mockStepProgress.set(2);
      const { container } = renderWizard();
      const completedCircles = container.querySelectorAll('.stepper-circle.completed');
      expect(completedCircles.length).toBe(2);
    });

    it('marks future steps as pending', () => {
      mockStepProgress.set(1);
      const { container } = renderWizard();
      const pendingCircles = container.querySelectorAll('.stepper-circle.pending');
      expect(pendingCircles.length).toBeGreaterThan(0);
    });

    it('shows correct number of stepper nodes for 5-step wizard', () => {
      const { container } = renderWizard();
      const nodes = container.querySelectorAll('.stepper-node');
      expect(nodes.length).toBe(5);
    });

    it('shows correct number of stepper nodes for 6-step wizard with security', () => {
      mockActiveStepOrder.set(['welcome', 'security', 'apps', 'navigation', 'theme', 'complete']);
      const { container } = renderWizard({ needsSetup: true });
      const nodes = container.querySelectorAll('.stepper-node');
      expect(nodes.length).toBe(6);
    });

    it('shows step numbers for pending steps', () => {
      mockStepProgress.set(0);
      const { container } = renderWizard();
      // The current step (0) shows "1", pending steps show their numbers
      const circleTexts = Array.from(container.querySelectorAll('.stepper-circle span'));
      expect(circleTexts.length).toBeGreaterThan(0);
    });
  });

  // =======================================================================
  // 11. Apps step - popular app interaction details
  // =======================================================================
  describe('Apps step - detailed interactions', () => {
    beforeEach(() => {
      mockCurrentStep.set('apps');
      mockStepProgress.set(1);
    });

    it('shows groups after selecting apps from different categories', async () => {
      renderWizard();

      // Select Plex (Media group)
      const plexCard = screen.getByRole('checkbox', { name: /Plex/i });
      await fireEvent.click(plexCard);

      // Select Sonarr (Downloads group)
      const sonarrCard = screen.getByRole('checkbox', { name: /Sonarr/i });
      await fireEvent.click(sonarrCard);

      await waitFor(() => {
        expect(screen.getByText('2 apps selected')).toBeInTheDocument();
      });
    });

    it('correctly increments count when selecting multiple apps', async () => {
      renderWizard();

      await fireEvent.click(screen.getByRole('checkbox', { name: /Plex/i }));
      await fireEvent.click(screen.getByRole('checkbox', { name: /Sonarr/i }));
      await fireEvent.click(screen.getByRole('checkbox', { name: /Portainer/i }));

      await waitFor(() => {
        expect(screen.getByText('3 apps selected')).toBeInTheDocument();
      });
    });

    it('shows "Drag apps between groups to organize" help text', () => {
      renderWizard();
      expect(screen.getByText('Drag apps between groups to organize')).toBeInTheDocument();
    });
  });

  // =======================================================================
  // 12. Navigation step - position-specific options
  // =======================================================================
  describe('Navigation step - position-specific rendering', () => {
    beforeEach(() => {
      mockCurrentStep.set('navigation');
      mockStepProgress.set(2);
    });

    it('does not show floating position options for left sidebar', () => {
      mockSelectedNavigation.set('left');
      renderWizard();
      expect(screen.queryByText('Bottom Right')).not.toBeInTheDocument();
    });

    it('does not show bar style options for left sidebar', () => {
      mockSelectedNavigation.set('left');
      renderWizard();
      expect(screen.queryByText('Group Dropdowns')).not.toBeInTheDocument();
    });

    it('does not show floating position options for top bar', () => {
      mockSelectedNavigation.set('top');
      renderWizard();
      expect(screen.queryByText('Bottom Right')).not.toBeInTheDocument();
    });

    it('does not show Collapsible Footer for floating position', () => {
      mockSelectedNavigation.set('floating');
      renderWizard();
      expect(screen.queryByText('Collapsible Footer')).not.toBeInTheDocument();
    });

    it('does not show Collapsible Footer for bottom bar', () => {
      mockSelectedNavigation.set('bottom');
      renderWizard();
      expect(screen.queryByText('Collapsible Footer')).not.toBeInTheDocument();
    });

    it('shows description for auto-hide', () => {
      renderWizard();
      expect(screen.getByText('Automatically collapse the menu after inactivity')).toBeInTheDocument();
    });

    it('shows Start on Overview description', () => {
      renderWizard();
      expect(screen.getByText('Show the dashboard overview when Muximux opens')).toBeInTheDocument();
    });

  });

  // =======================================================================
  // 13. Complete step with custom apps
  // =======================================================================
  describe('Complete step with custom apps in store', () => {
    it('shows correct app count with custom apps', () => {
      mockCurrentStep.set('complete');
      mockStepProgress.set(4);
      mockSelectedApps.set([
        {
          name: 'MyApp',
          url: 'http://localhost:3000',
          icon: { type: 'dashboard', name: '', file: '', url: '', variant: '' },
          color: '#22c55e',
          group: '',
          order: 0,
          enabled: true,
          default: false,
          open_mode: 'iframe',
          proxy: false,
          scale: 1,
        },
      ]);
      renderWizard();
      expect(screen.getByText(/1 app\./)).toBeInTheDocument();
    });

    it('calls oncomplete with custom apps from store', async () => {
      mockCurrentStep.set('complete');
      mockStepProgress.set(4);
      const customApp = {
        name: 'MyApp',
        url: 'http://localhost:3000',
        icon: { type: 'dashboard', name: '', file: '', url: '', variant: '' },
        color: '#22c55e',
        group: '',
        order: 0,
        enabled: true,
        default: false,
        open_mode: 'iframe',
        proxy: false,
        scale: 1,
      };
      mockSelectedApps.set([customApp]);
      const oncomplete = vi.fn();
      renderWizard({ oncomplete });
      await fireEvent.click(screen.getByText('Launch Dashboard'));
      const arg = oncomplete.mock.calls[0][0] as Record<string, unknown>;
      const apps = arg.apps as Array<{ name: string }>;
      expect(apps.some(a => a.name === 'MyApp')).toBe(true);
    });
  });

  // =======================================================================
  // 14. Footer status text
  // =======================================================================
  describe('Footer status text', () => {
    it('shows apps selected count on apps step', () => {
      mockCurrentStep.set('apps');
      mockStepProgress.set(1);
      renderWizard();
      expect(screen.getByText('0 apps selected')).toBeInTheDocument();
    });

    it('does not show status text on welcome step', () => {
      mockCurrentStep.set('welcome');
      mockStepProgress.set(0);
      renderWizard();
      expect(screen.queryByText(/apps? selected/)).not.toBeInTheDocument();
    });

    it('does not show status text on theme step', () => {
      mockCurrentStep.set('theme');
      mockStepProgress.set(3);
      renderWizard();
      expect(screen.queryByText(/apps? selected/)).not.toBeInTheDocument();
    });
  });

  // =======================================================================
  // 15. Global keydown handler
  // =======================================================================
  describe('Global keydown handler', () => {
    it('pressing Enter on welcome step calls nextStep', async () => {
      const { container } = renderWizard();
      const dialog = container.querySelector('[role="dialog"]')!;
      await fireEvent.keyDown(dialog, { key: 'Enter' });
      expect(mockNextStep).toHaveBeenCalled();
    });

    it('pressing Enter on navigation step calls nextStep', async () => {
      mockCurrentStep.set('navigation');
      mockStepProgress.set(2);
      const { container } = renderWizard();
      const dialog = container.querySelector('[role="dialog"]')!;
      await fireEvent.keyDown(dialog, { key: 'Enter' });
      expect(mockNextStep).toHaveBeenCalled();
    });

    it('pressing Enter on theme step calls nextStep', async () => {
      mockCurrentStep.set('theme');
      mockStepProgress.set(3);
      const { container } = renderWizard();
      const dialog = container.querySelector('[role="dialog"]')!;
      await fireEvent.keyDown(dialog, { key: 'Enter' });
      expect(mockNextStep).toHaveBeenCalled();
    });

    it('non-Enter key does nothing', async () => {
      const { container } = renderWizard();
      const dialog = container.querySelector('[role="dialog"]')!;
      await fireEvent.keyDown(dialog, { key: 'Escape' });
      expect(mockNextStep).not.toHaveBeenCalled();
    });

    it('pressing Enter on complete step calls oncomplete', async () => {
      mockCurrentStep.set('complete');
      mockStepProgress.set(4);
      const oncomplete = vi.fn();
      const { container } = renderWizard({ oncomplete });
      const dialog = container.querySelector('[role="dialog"]')!;
      await fireEvent.keyDown(dialog, { key: 'Enter' });
      expect(oncomplete).toHaveBeenCalled();
    });
  });

  // =======================================================================
  // 16. Navigation preview rendering
  // =======================================================================
  describe('Navigation preview rendering', () => {
    it('renders preview on navigation step with left sidebar', () => {
      mockCurrentStep.set('navigation');
      mockStepProgress.set(2);
      mockSelectedNavigation.set('left');
      const { container } = renderWizard();
      expect(container.querySelector('[role="dialog"]')).toBeTruthy();
    });

    it('renders preview on navigation step with right sidebar', () => {
      mockCurrentStep.set('navigation');
      mockStepProgress.set(2);
      mockSelectedNavigation.set('right');
      const { container } = renderWizard();
      expect(container.querySelector('[role="dialog"]')).toBeTruthy();
    });

    it('renders preview on navigation step with top bar', () => {
      mockCurrentStep.set('navigation');
      mockStepProgress.set(2);
      mockSelectedNavigation.set('top');
      const { container } = renderWizard();
      expect(container.querySelector('[role="dialog"]')).toBeTruthy();
    });

    it('renders preview on navigation step with bottom bar', () => {
      mockCurrentStep.set('navigation');
      mockStepProgress.set(2);
      mockSelectedNavigation.set('bottom');
      const { container } = renderWizard();
      expect(container.querySelector('[role="dialog"]')).toBeTruthy();
    });

    it('renders preview on navigation step with floating', () => {
      mockCurrentStep.set('navigation');
      mockStepProgress.set(2);
      mockSelectedNavigation.set('floating');
      const { container } = renderWizard();
      expect(container.querySelector('[role="dialog"]')).toBeTruthy();
    });

    it('renders preview on theme step', () => {
      mockCurrentStep.set('theme');
      mockStepProgress.set(3);
      mockSelectedNavigation.set('left');
      const { container } = renderWizard();
      expect(container.querySelector('[role="dialog"]')).toBeTruthy();
    });

    it('renders fallback preview apps when nothing selected', () => {
      mockCurrentStep.set('navigation');
      mockStepProgress.set(2);
      // With no apps selected, the preview should show fallback apps
      const { container } = renderWizard();
      expect(container.querySelector('[role="dialog"]')).toBeTruthy();
    });
  });

  // =======================================================================
  // 17. Security step - forward auth preset switching
  // =======================================================================
  describe('Security step - forward auth presets', () => {
    beforeEach(() => {
      mockActiveStepOrder.set(['welcome', 'security', 'apps', 'navigation', 'theme', 'complete']);
      mockCurrentStep.set('security');
      mockStepProgress.set(1);
    });

    it('clicking Authelia preset calls applyPreset with authelia', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('I use an auth proxy'));
      await waitFor(() => {
        expect(screen.getByText('Authelia')).toBeInTheDocument();
      });
      await fireEvent.click(screen.getByText('Authelia'));
      expect(mockApplyPreset).toHaveBeenCalledWith('authelia', expect.any(String));
    });

    it('clicking Custom preset calls applyPreset with custom', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('I use an auth proxy'));
      await waitFor(() => {
        expect(screen.getByText('Custom')).toBeInTheDocument();
      });
      await fireEvent.click(screen.getByText('Custom'));
      expect(mockApplyPreset).toHaveBeenCalledWith('custom', expect.any(String));
    });

    it('shows logout URL help text', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('I use an auth proxy'));
      await waitFor(() => {
        expect(screen.getByText(/auth provider's logout endpoint/)).toBeInTheDocument();
      });
    });

    it('shows trusted proxies help text', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('I use an auth proxy'));
      await waitFor(() => {
        expect(screen.getByText('IP addresses or CIDR ranges, one per line')).toBeInTheDocument();
      });
    });
  });

  // =======================================================================
  // 18. Security step - none auth risk flow
  // =======================================================================
  describe('Security step - none auth risk acknowledgment', () => {
    beforeEach(() => {
      mockActiveStepOrder.set(['welcome', 'security', 'apps', 'navigation', 'theme', 'complete']);
      mockCurrentStep.set('security');
      mockStepProgress.set(1);
    });

    it('shows full security warning text', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('No authentication'));
      await waitFor(() => {
        expect(screen.getByText(/anyone who can reach this port/)).toBeInTheDocument();
      });
    });

    it('collapses none form when clicking No authentication again', async () => {
      renderWizard({ needsSetup: true });
      await fireEvent.click(screen.getByText('No authentication'));
      await waitFor(() => {
        expect(screen.getByText('Security warning')).toBeInTheDocument();
      });
      await fireEvent.click(screen.getByText('No authentication'));
      await waitFor(() => {
        expect(screen.queryByText('Security warning')).not.toBeInTheDocument();
      });
    });

    it('shows no-auth description text', () => {
      renderWizard({ needsSetup: true });
      expect(screen.getByText('Anyone with network access gets full control')).toBeInTheDocument();
    });

    it('shows builtin auth description text', () => {
      renderWizard({ needsSetup: true });
      expect(screen.getByText('Set up a username and password to protect your dashboard')).toBeInTheDocument();
    });
  });

  // =======================================================================
  // 19. Apps step - Add button and custom app validation
  // =======================================================================
  describe('Apps step - custom app validation', () => {
    beforeEach(() => {
      mockCurrentStep.set('apps');
      mockStepProgress.set(1);
    });

    it('Add button disabled with only name filled', async () => {
      renderWizard();
      const nameInput = screen.getByPlaceholderText('App name');
      await fireEvent.input(nameInput, { target: { value: 'MyApp' } });
      await waitFor(() => {
        const addBtn = screen.getByRole('button', { name: 'Add' });
        expect(addBtn).toBeDisabled();
      });
    });

    it('Add button disabled with only URL filled', async () => {
      renderWizard();
      const urlInput = screen.getByPlaceholderText('http://localhost:8080');
      await fireEvent.input(urlInput, { target: { value: 'http://test.com' } });
      await waitFor(() => {
        const addBtn = screen.getByRole('button', { name: 'Add' });
        expect(addBtn).toBeDisabled();
      });
    });
  });

  // =======================================================================
  // 20. Complete step - summary values
  // =======================================================================
  describe('Complete step - summary values', () => {
    beforeEach(() => {
      mockCurrentStep.set('complete');
      mockStepProgress.set(4);
    });

    it('shows navigation position capitalized', () => {
      mockSelectedNavigation.set('top');
      renderWizard();
      expect(screen.getByText('top')).toBeInTheDocument();
    });

    it('shows right navigation position', () => {
      mockSelectedNavigation.set('right');
      renderWizard();
      expect(screen.getByText('right')).toBeInTheDocument();
    });

    it('shows Theme label in summary', () => {
      renderWizard();
      // The summary has a dt with "Theme" - use getAllByText since "Theme" appears in stepper too
      const themeElements = screen.getAllByText('Theme');
      expect(themeElements.length).toBeGreaterThanOrEqual(1);
    });
  });

  // =======================================================================
  // 21. Navigation step - Auto-hide expansion
  // =======================================================================
  describe('Navigation step - auto-hide options', () => {
    beforeEach(() => {
      mockCurrentStep.set('navigation');
      mockStepProgress.set(2);
    });

    it('shows auto-hide delay options when auto-hide checkbox is checked', async () => {
      renderWizard();
      // Find and click the Auto-hide Menu checkbox
      const autoHideLabel = screen.getByText('Auto-hide Menu');
      const autoHideCheckbox = autoHideLabel.closest('label')!.querySelector('input[type="checkbox"]')!;
      await fireEvent.click(autoHideCheckbox);
      await waitFor(() => {
        expect(screen.getByText('Hide after')).toBeInTheDocument();
      });
    });

    it('shows shadow option when auto-hide is enabled', async () => {
      renderWizard();
      const autoHideLabel = screen.getByText('Auto-hide Menu');
      const autoHideCheckbox = autoHideLabel.closest('label')!.querySelector('input[type="checkbox"]')!;
      await fireEvent.click(autoHideCheckbox);
      await waitFor(() => {
        expect(screen.getByText(/Shadow — show a drop shadow/)).toBeInTheDocument();
      });
    });

    it('shows delay select with options when auto-hide is enabled', async () => {
      renderWizard();
      const autoHideLabel = screen.getByText('Auto-hide Menu');
      const autoHideCheckbox = autoHideLabel.closest('label')!.querySelector('input[type="checkbox"]')!;
      await fireEvent.click(autoHideCheckbox);
      await waitFor(() => {
        const select = screen.getByDisplayValue('0.5s');
        expect(select).toBeInTheDocument();
      });
    });
  });

  // =======================================================================
  // 22. Enter key on apps step with selected apps
  // =======================================================================
  describe('Global keydown - apps step with selections', () => {
    it('pressing Enter on apps step with apps selected calls nextStep', async () => {
      mockCurrentStep.set('apps');
      mockStepProgress.set(1);
      const { container } = renderWizard();

      // Select an app first
      const plexCard = screen.getByRole('checkbox', { name: /Plex/i });
      await fireEvent.click(plexCard);
      await waitFor(() => {
        expect(plexCard).toHaveAttribute('aria-checked', 'true');
      });

      const dialog = container.querySelector('[role="dialog"]')!;
      await fireEvent.keyDown(dialog, { key: 'Enter' });
      expect(mockNextStep).toHaveBeenCalled();
    });

    it('pressing Enter on apps step with no apps does not call nextStep', async () => {
      mockCurrentStep.set('apps');
      mockStepProgress.set(1);
      const { container } = renderWizard();
      const dialog = container.querySelector('[role="dialog"]')!;
      await fireEvent.keyDown(dialog, { key: 'Enter' });
      expect(mockNextStep).not.toHaveBeenCalled();
    });

    it('pressing Enter on security step with valid builtin calls handleSecuritySubmit', async () => {
      mockActiveStepOrder.set(['welcome', 'security', 'apps', 'navigation', 'theme', 'complete']);
      mockCurrentStep.set('security');
      mockStepProgress.set(1);
      renderWizard({ needsSetup: true });

      // Set up valid builtin auth
      await fireEvent.click(screen.getByText('Create a password'));
      await waitFor(() => {
        expect(screen.getByLabelText('Username')).toBeInTheDocument();
      });
      await fireEvent.input(screen.getByLabelText('Username'), { target: { value: 'admin' } });
      await fireEvent.input(screen.getByLabelText('Password'), { target: { value: 'password123' } });
      await fireEvent.input(screen.getByLabelText('Confirm password'), { target: { value: 'password123' } });

      const dialog = document.querySelector('[role="dialog"]')!;
      await fireEvent.keyDown(dialog, { key: 'Enter' });
      expect(mockNextStep).toHaveBeenCalled();
    });
  });

  // =======================================================================
  // 23. Complete step with security setup data
  // =======================================================================
  describe('Complete step - security setup in oncomplete', () => {
    it('oncomplete includes setup with forward_auth when configured', async () => {
      mockActiveStepOrder.set(['welcome', 'security', 'apps', 'navigation', 'theme', 'complete']);
      mockCurrentStep.set('security');
      mockStepProgress.set(1);

      renderWizard({ needsSetup: true });

      // Configure forward auth
      await fireEvent.click(screen.getByText('I use an auth proxy'));
      await waitFor(() => {
        expect(screen.getByLabelText('Trusted proxy IPs')).toBeInTheDocument();
      });
      await fireEvent.input(screen.getByLabelText('Trusted proxy IPs'), { target: { value: '10.0.0.1/32' } });

      // The forward auth method is now set; verify Continue is enabled
      await waitFor(() => {
        expect(screen.getByText('Continue')).not.toBeDisabled();
      });
    });

    it('oncomplete includes setup with none auth when risk acknowledged', async () => {
      mockActiveStepOrder.set(['welcome', 'security', 'apps', 'navigation', 'theme', 'complete']);
      mockCurrentStep.set('security');
      mockStepProgress.set(1);

      renderWizard({ needsSetup: true });

      // Select none auth and acknowledge risk
      await fireEvent.click(screen.getByText('No authentication'));
      await waitFor(() => {
        expect(screen.getByRole('checkbox')).toBeInTheDocument();
      });
      await fireEvent.click(screen.getByRole('checkbox'));

      // Verify Continue is enabled
      await waitFor(() => {
        expect(screen.getByText('Continue')).not.toBeDisabled();
      });

      // Click Continue to proceed
      await fireEvent.click(screen.getByText('Continue'));
      expect(mockNextStep).toHaveBeenCalled();
    });
  });

  // =======================================================================
  // 24. Apps step - "Add Group" button
  // =======================================================================
  describe('Apps step - Add Group interaction', () => {
    beforeEach(() => {
      mockCurrentStep.set('apps');
      mockStepProgress.set(1);
    });

    it('clicking Add Group button adds a new group', async () => {
      renderWizard();
      // First select an app to make groups section visible
      const plexCard = screen.getByRole('checkbox', { name: /Plex/i });
      await fireEvent.click(plexCard);
      await waitFor(() => {
        expect(screen.getByText('Add Group')).toBeInTheDocument();
      });
      await fireEvent.click(screen.getByText('Add Group'));
      await waitFor(() => {
        // The new group "New Group" should appear
        const newGroupInput = screen.getByDisplayValue('New Group');
        expect(newGroupInput).toBeInTheDocument();
      });
    });
  });

  // =======================================================================
  // 25. Navigation step - Show On Hover toggle
  // =======================================================================
  describe('Navigation step - show on hover', () => {
    beforeEach(() => {
      mockCurrentStep.set('navigation');
      mockStepProgress.set(2);
    });

    it('shows Show on Hover option when auto-hide is enabled', async () => {
      renderWizard();
      const autoHideLabel = screen.getByText('Auto-hide Menu');
      const autoHideCheckbox = autoHideLabel.closest('label')!.querySelector('input[type="checkbox"]')!;
      await fireEvent.click(autoHideCheckbox);
      await waitFor(() => {
        // The show-on-hover option is a sub-option of auto-hide
        expect(screen.getByText(/Shadow/)).toBeInTheDocument();
      });
    });
  });

  // =======================================================================
  // 26. Full flow: select apps then complete
  // =======================================================================
  describe('Full flow - apps selection through to completion', () => {
    it('completes onboarding with popular apps selected', async () => {
      // Start on apps step
      mockCurrentStep.set('apps');
      mockStepProgress.set(1);
      const oncomplete = vi.fn();
      renderWizard({ oncomplete });

      // Select Plex and Sonarr
      const plexCard = screen.getByRole('checkbox', { name: /Plex/i });
      const sonarrCard = screen.getByRole('checkbox', { name: /Sonarr/i });
      await fireEvent.click(plexCard);
      await fireEvent.click(sonarrCard);

      await waitFor(() => {
        expect(screen.getByText('2 apps selected')).toBeInTheDocument();
      });

      // Switch to complete step (simulating that the user navigated through)
      mockCurrentStep.set('complete');
      mockStepProgress.set(4);

      // Wait for the complete step to render
      await waitFor(() => {
        expect(screen.getByText("You're All Set!")).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByText('Launch Dashboard'));

      expect(oncomplete).toHaveBeenCalledTimes(1);
      const arg = oncomplete.mock.calls[0][0] as Record<string, unknown>;
      expect(arg).toHaveProperty('apps');
      expect(arg).toHaveProperty('groups');
      const apps = arg.apps as Array<{ name: string }>;
      // Should include the selected popular apps
      expect(apps.length).toBeGreaterThanOrEqual(2);
    });

    it('completes with apps and shows correct summary count', async () => {
      mockCurrentStep.set('apps');
      mockStepProgress.set(1);
      renderWizard();

      // Select all 3 apps
      await fireEvent.click(screen.getByRole('checkbox', { name: /Plex/i }));
      await fireEvent.click(screen.getByRole('checkbox', { name: /Sonarr/i }));
      await fireEvent.click(screen.getByRole('checkbox', { name: /Portainer/i }));

      await waitFor(() => {
        expect(screen.getByText('3 apps selected')).toBeInTheDocument();
      });

      // Switch to complete step
      mockCurrentStep.set('complete');
      mockStepProgress.set(4);

      await waitFor(() => {
        expect(screen.getByText(/3 apps\./)).toBeInTheDocument();
      });
    });

    it('preview shows selected popular apps on navigation step', async () => {
      mockCurrentStep.set('apps');
      mockStepProgress.set(1);
      renderWizard();

      // Select Plex
      const plexCard = screen.getByRole('checkbox', { name: /Plex/i });
      await fireEvent.click(plexCard);

      // Switch to navigation step
      mockCurrentStep.set('navigation');
      mockStepProgress.set(2);

      await waitFor(() => {
        expect(screen.getByText('Choose Your Navigation Style')).toBeInTheDocument();
      });
    });
  });

  // =======================================================================
  // 27. Apps step - group visibility with selected apps
  // =======================================================================
  describe('Apps step - group management with selections', () => {
    beforeEach(() => {
      mockCurrentStep.set('apps');
      mockStepProgress.set(1);
    });

    it('shows groups section with group count when apps are selected', async () => {
      renderWizard();
      await fireEvent.click(screen.getByRole('checkbox', { name: /Plex/i }));
      await waitFor(() => {
        // Groups section header appears
        expect(screen.getByText(/Groups/)).toBeInTheDocument();
      });
    });

    it('shows remove group button for each group', async () => {
      renderWizard();
      await fireEvent.click(screen.getByRole('checkbox', { name: /Plex/i }));
      await waitFor(() => {
        const removeBtn = screen.queryAllByRole('button', { name: /Remove group/i });
        expect(removeBtn.length).toBeGreaterThan(0);
      });
    });

    it('adds another instance of a selected app via the "+" button', async () => {
      renderWizard();
      // Select Plex first
      const plexCard = screen.getByRole('checkbox', { name: /Plex/i });
      await fireEvent.click(plexCard);
      await waitFor(() => {
        expect(plexCard).toHaveAttribute('aria-checked', 'true');
      });

      // Now the "Add another Plex" button should be visible
      await waitFor(() => {
        const addAnotherBtn = screen.getByTitle('Add another Plex');
        expect(addAnotherBtn).toBeInTheDocument();
      });

      // Click it to add "Plex 2"
      await fireEvent.click(screen.getByTitle('Add another Plex'));

      await waitFor(() => {
        expect(screen.getByText('2 apps selected')).toBeInTheDocument();
      });
    });

    it('clicking Add Group twice creates unique group names', async () => {
      renderWizard();
      // Select an app first to show groups
      await fireEvent.click(screen.getByRole('checkbox', { name: /Plex/i }));
      await waitFor(() => {
        expect(screen.getByText('Add Group')).toBeInTheDocument();
      });

      // Click Add Group twice
      await fireEvent.click(screen.getByText('Add Group'));
      await waitFor(() => {
        expect(screen.getByDisplayValue('New Group')).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByText('Add Group'));
      await waitFor(() => {
        expect(screen.getByDisplayValue('New Group 2')).toBeInTheDocument();
      });
    });
  });

  // =======================================================================
  // 28. Complete step with setup for forward auth
  // =======================================================================
  describe('Complete step with forward auth setup', () => {
    it('builds setup with forward auth and calls oncomplete', async () => {
      mockActiveStepOrder.set(['welcome', 'security', 'apps', 'navigation', 'theme', 'complete']);
      mockCurrentStep.set('security');
      mockStepProgress.set(1);
      const oncomplete = vi.fn();
      renderWizard({ oncomplete, needsSetup: true });

      // Select forward auth
      await fireEvent.click(screen.getByText('I use an auth proxy'));
      await waitFor(() => {
        expect(screen.getByLabelText('Trusted proxy IPs')).toBeInTheDocument();
      });
      await fireEvent.input(screen.getByLabelText('Trusted proxy IPs'), { target: { value: '10.0.0.1/32' } });

      // Switch to complete step
      mockCurrentStep.set('complete');
      mockStepProgress.set(5);

      await waitFor(() => {
        expect(screen.getByText("You're All Set!")).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByText('Launch Dashboard'));
      expect(oncomplete).toHaveBeenCalledTimes(1);
      const arg = oncomplete.mock.calls[0][0] as Record<string, unknown>;
      expect(arg).toHaveProperty('setup');
      const setup = arg.setup as { method: string };
      expect(setup.method).toBe('forward_auth');
    });

    it('builds setup with builtin auth and calls oncomplete', async () => {
      mockActiveStepOrder.set(['welcome', 'security', 'apps', 'navigation', 'theme', 'complete']);
      mockCurrentStep.set('security');
      mockStepProgress.set(1);
      const oncomplete = vi.fn();
      renderWizard({ oncomplete, needsSetup: true });

      // Select builtin auth
      await fireEvent.click(screen.getByText('Create a password'));
      await waitFor(() => {
        expect(screen.getByLabelText('Username')).toBeInTheDocument();
      });
      await fireEvent.input(screen.getByLabelText('Username'), { target: { value: 'admin' } });
      await fireEvent.input(screen.getByLabelText('Password'), { target: { value: 'password123' } });
      await fireEvent.input(screen.getByLabelText('Confirm password'), { target: { value: 'password123' } });

      // Switch to complete step
      mockCurrentStep.set('complete');
      mockStepProgress.set(5);

      await waitFor(() => {
        expect(screen.getByText("You're All Set!")).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByText('Launch Dashboard'));
      expect(oncomplete).toHaveBeenCalledTimes(1);
      const arg = oncomplete.mock.calls[0][0] as Record<string, unknown>;
      expect(arg).toHaveProperty('setup');
      const setup = arg.setup as { method: string; username?: string; password?: string };
      expect(setup.method).toBe('builtin');
      expect(setup.username).toBe('admin');
      expect(setup.password).toBe('password123');
    });

    it('builds setup with none auth when risk acknowledged', async () => {
      mockActiveStepOrder.set(['welcome', 'security', 'apps', 'navigation', 'theme', 'complete']);
      mockCurrentStep.set('security');
      mockStepProgress.set(1);
      const oncomplete = vi.fn();
      renderWizard({ oncomplete, needsSetup: true });

      // Select none auth and acknowledge risk
      await fireEvent.click(screen.getByText('No authentication'));
      await waitFor(() => {
        expect(screen.getByRole('checkbox')).toBeInTheDocument();
      });
      await fireEvent.click(screen.getByRole('checkbox'));

      // Switch to complete step
      mockCurrentStep.set('complete');
      mockStepProgress.set(5);

      await waitFor(() => {
        expect(screen.getByText("You're All Set!")).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByText('Launch Dashboard'));
      expect(oncomplete).toHaveBeenCalledTimes(1);
      const arg = oncomplete.mock.calls[0][0] as Record<string, unknown>;
      expect(arg).toHaveProperty('setup');
      const setup = arg.setup as { method: string };
      expect(setup.method).toBe('none');
    });
  });
});
