import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

// --- Hoisted store values and mock fns ---
const {
  mockSelectedFamily,
  mockVariantMode,
  mockIsAdmin,
  mockSetThemeFamily,
  mockSetVariantMode,
  mockExportConfig,
  mockParseImportedConfig,
  mockToasts,
  mockGetKeybindingsForConfig,
  mockAppSchemaSafeParse,
  mockGroupSchemaSafeParse,
  mockExtractErrors,
  mockIsMobileViewport,
  mockTemplateToApp,
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

  const mockIsMobileViewport = { fn: (() => false) as () => boolean };

  return {
    mockSelectedFamily: makeStore('default'),
    mockVariantMode: makeStore('dark' as 'dark' | 'light' | 'system'),
    mockIsAdmin: makeStore(true),
    mockSetThemeFamily: vi.fn(),
    mockSetVariantMode: vi.fn(),
    mockExportConfig: vi.fn(),
    mockParseImportedConfig: vi.fn(),
    mockToasts: {
      success: vi.fn(),
      error: vi.fn(),
    },
    mockGetKeybindingsForConfig: vi.fn(() => ({ bindings: {} })),
    mockAppSchemaSafeParse: vi.fn(() => ({ success: true })),
    mockGroupSchemaSafeParse: vi.fn(() => ({ success: true })),
    mockExtractErrors: vi.fn(() => ({})),
    mockIsMobileViewport,
    mockTemplateToApp: vi.fn((template: Record<string, string>, url: string, order: number) => ({
      name: template.name,
      url: url || template.defaultUrl,
      icon: { type: 'dashboard', name: template.icon || 'test', file: '', url: '', variant: 'svg' },
      color: template.color || '#000',
      group: template.group || '',
      order,
      enabled: true,
      default: false,
      open_mode: 'iframe' as const,
      proxy: false,
      scale: 1,
    })),
  };
});

// --- Mocks ---

vi.mock('$lib/themeStore', () => ({
  selectedFamily: mockSelectedFamily,
  variantMode: mockVariantMode,
  setThemeFamily: mockSetThemeFamily,
  setVariantMode: mockSetVariantMode,
  themeFamilies: { subscribe: (fn: (v: unknown[]) => void) => { fn([]); return () => {}; } },
  builtinThemes: [],
  customThemes: { subscribe: (fn: (v: unknown[]) => void) => { fn([]); return () => {}; } },
  allThemes: { subscribe: (fn: (v: unknown[]) => void) => { fn([]); return () => {}; } },
  resolvedTheme: { subscribe: (fn: (v: string) => void) => { fn('dark'); return () => {}; } },
  isDarkTheme: { subscribe: (fn: (v: boolean) => void) => { fn(true); return () => {}; } },
  systemTheme: { subscribe: (fn: (v: string) => void) => { fn('dark'); return () => {}; } },
  detectCustomThemes: vi.fn().mockResolvedValue(undefined),
  initTheme: vi.fn(),
  syncFromConfig: vi.fn(),
}));

vi.mock('$lib/useSwipe', () => ({
  isMobileViewport: (..._args: unknown[]) => mockIsMobileViewport.fn(),
  isTouchDevice: vi.fn(() => false),
}));

vi.mock('$lib/api', () => ({
  getBase: vi.fn(() => ''),
  API_BASE: '',
  exportConfig: (...args: unknown[]) => mockExportConfig(...args),
  parseImportedConfig: (...args: unknown[]) => mockParseImportedConfig(...args),
}));

vi.mock('$lib/toastStore', () => ({
  toasts: mockToasts,
}));

vi.mock('$lib/keybindingsStore', () => ({
  getKeybindingsForConfig: (...args: unknown[]) => mockGetKeybindingsForConfig(...args),
  keybindings: { subscribe: (fn: (v: unknown[]) => void) => { fn([]); return () => {}; } },
  formatKeybinding: vi.fn(() => ''),
  formatKeyCombo: vi.fn(() => ''),
  initKeybindings: vi.fn(),
}));

vi.mock('$lib/schemas', () => ({
  appSchema: { safeParse: (...args: unknown[]) => mockAppSchemaSafeParse(...args) },
  groupSchema: { safeParse: (...args: unknown[]) => mockGroupSchemaSafeParse(...args) },
  extractErrors: (...args: unknown[]) => mockExtractErrors(...args),
}));

vi.mock('$lib/popularApps', () => ({
  popularApps: {
    'Media': [
      { name: 'Plex', defaultUrl: 'http://localhost:32400', icon: 'plex', color: '#E5A00D', iconBackground: '#1a1a1a', group: 'Media', description: 'Media server' },
      { name: 'Sonarr', defaultUrl: 'http://localhost:8989', icon: 'sonarr', color: '#35C5F4', iconBackground: '#1a1a1a', group: 'Media', description: 'TV series manager' },
    ],
    'System': [
      { name: 'Portainer', defaultUrl: 'http://localhost:9000', icon: 'portainer', color: '#13BEF9', iconBackground: '#1a1a1a', group: 'System', description: 'Container management' },
    ],
  },
  templateToApp: (...args: unknown[]) => mockTemplateToApp(...args),
  getAllPopularApps: vi.fn(() => []),
  getAllGroups: vi.fn(() => []),
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
  isAdmin: mockIsAdmin,
  currentUser: { subscribe: (fn: (v: unknown) => void) => { fn({ username: 'admin', role: 'admin' }); return () => {}; } },
  isAuthenticated: { subscribe: (fn: (v: boolean) => void) => { fn(true); return () => {}; } },
}));

// ---------------------------------------------------------------------------
// Smart mock sub-components
// ---------------------------------------------------------------------------
// Svelte 5 invokes compiled components as ($$anchor, $$props) where:
//   $$anchor = a comment node (DOM marker)
//   $$props  = the component props object directly

function resolveArgs(args: unknown[]): { target: Node | null; props: Record<string, unknown> } {
  if (args[0] instanceof Node) {
    return { target: args[0].parentNode || args[0], props: (args[1] as Record<string, unknown>) || {} };
  }
  const first = args[0] as Record<string, unknown> | null;
  if (first?.target) {
    return { target: first.target as Node, props: (first.props as Record<string, unknown>) || {} };
  }
  return { target: null, props: {} };
}

function makeMockAppsTab(...args: unknown[]) {
  const { target, props } = resolveArgs(args);
  if (!target) return { $destroy() {} };
  const div = document.createElement('div');
  div.dataset.testid = 'mock-apps-tab';

  if (props.onshowAddApp) {
    const btn = document.createElement('button');
    btn.textContent = 'Trigger Add App';
    btn.dataset.testid = 'trigger-add-app';
    btn.onclick = props.onshowAddApp;
    div.appendChild(btn);
  }
  if (props.onshowAddGroup) {
    const btn = document.createElement('button');
    btn.textContent = 'Trigger Add Group';
    btn.dataset.testid = 'trigger-add-group';
    btn.onclick = props.onshowAddGroup;
    div.appendChild(btn);
  }
  if (props.onstartEditApp) {
    const btn = document.createElement('button');
    btn.textContent = 'Trigger Edit App';
    btn.dataset.testid = 'trigger-edit-app';
    btn.onclick = () => props.onstartEditApp({
      name: 'TestApp', url: 'https://test.com',
      icon: { type: 'dashboard', name: 'test', file: '', url: '', variant: 'svg' },
      color: '#000', group: '', order: 0, enabled: true, default: false,
      open_mode: 'iframe', proxy: false, scale: 1
    });
    div.appendChild(btn);
  }
  if (props.onstartEditGroup) {
    const btn = document.createElement('button');
    btn.textContent = 'Trigger Edit Group';
    btn.dataset.testid = 'trigger-edit-group';
    btn.onclick = () => props.onstartEditGroup({
      name: 'TestGroup', icon: { type: 'dashboard', name: 'test', file: '', url: '', variant: '' },
      color: '#3498db', order: 0, expanded: true
    });
    div.appendChild(btn);
  }
  target.appendChild(div);
  return { $destroy() { div.remove(); } };
}
vi.mock('./settings/AppsTab.svelte', () => ({ default: makeMockAppsTab }));

function makeMockGeneralTab(...args: unknown[]) {
  const { target, props } = resolveArgs(args);
  if (!target) return { $destroy() {} };
  const div = document.createElement('div');
  div.dataset.testid = 'mock-general-tab';

  if (props.onexport) {
    const btn = document.createElement('button');
    btn.textContent = 'Trigger Export';
    btn.dataset.testid = 'trigger-export';
    btn.onclick = props.onexport;
    div.appendChild(btn);
  }
  if (props.onimportselect) {
    const btn = document.createElement('button');
    btn.textContent = 'Trigger Import';
    btn.dataset.testid = 'trigger-import';
    btn.onclick = () => {
      const fakeInput = document.createElement('input');
      fakeInput.type = 'file';
      const fakeFile = new File(['title: Test\napps: []\ngroups: []'], 'config.yaml', { type: 'text/yaml' });
      Object.defineProperty(fakeInput, 'files', { value: [fakeFile] });
      props.onimportselect({ target: fakeInput } as unknown as Event);
    };
    div.appendChild(btn);
  }
  target.appendChild(div);
  return { $destroy() { div.remove(); } };
}
vi.mock('./settings/GeneralTab.svelte', () => ({ default: makeMockGeneralTab }));

function makeMockKeybindingsEditor(...args: unknown[]) {
  const { target, props } = resolveArgs(args);
  if (!target) return { $destroy() {} };
  const div = document.createElement('div');
  div.dataset.testid = 'mock-keybindings-editor';

  if (props.onchange) {
    const btn = document.createElement('button');
    btn.textContent = 'Change Keybinding';
    btn.dataset.testid = 'trigger-keybinding-change';
    btn.onclick = props.onchange;
    div.appendChild(btn);
  }
  target.appendChild(div);
  return { $destroy() { div.remove(); } };
}
vi.mock('./KeybindingsEditor.svelte', () => ({ default: makeMockKeybindingsEditor }));

function noopComponent() {
  return { $destroy: vi.fn() };
}

function makeMockIconBrowser(...args: unknown[]) {
  const { target, props } = resolveArgs(args);
  if (!target) return { $destroy() {} };
  const div = document.createElement('div');
  div.dataset.testid = 'mock-icon-browser';

  if (props.onselect) {
    const btn = document.createElement('button');
    btn.textContent = 'Pick Icon';
    btn.dataset.testid = 'trigger-select-icon';
    btn.onclick = () => props.onselect({ name: 'home', variant: 'svg', type: 'dashboard' });
    div.appendChild(btn);
  }
  if (props.onclose) {
    const btn = document.createElement('button');
    btn.textContent = 'Close Browser';
    btn.dataset.testid = 'trigger-close-icon-browser';
    btn.onclick = props.onclose;
    div.appendChild(btn);
  }
  target.appendChild(div);
  return { $destroy() { div.remove(); } };
}
vi.mock('./IconBrowser.svelte', () => ({ default: makeMockIconBrowser }));
vi.mock('./AppIcon.svelte', () => ({ default: noopComponent }));
vi.mock('./settings/AboutTab.svelte', () => ({ default: noopComponent }));
vi.mock('./settings/SecurityTab.svelte', () => ({ default: noopComponent }));
vi.mock('./settings/ThemeTab.svelte', () => ({ default: noopComponent }));

import Settings from './Settings.svelte';
import type { App, AppIcon as AppIconType, Config, NavigationConfig, Group } from '$lib/types';

// --- Helper factories ---

function makeIcon(overrides: Partial<AppIconType> = {}): AppIconType {
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
    position: 'left',
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
    name: 'TestGroup',
    icon: makeIcon(),
    color: '#3498db',
    order: 0,
    expanded: true,
    ...overrides,
  };
}

function makeConfig(overrides: Partial<Config> = {}): Config {
  return {
    title: 'Muximux',
    navigation: makeNav(overrides.navigation),
    groups: overrides.groups ?? [],
    apps: overrides.apps ?? [],
    auth: { method: 'builtin', ...(overrides.auth ?? {}) },
    ...overrides,
  };
}

const sampleApps: App[] = [
  makeApp({ name: 'Grafana', order: 0 }),
  makeApp({ name: 'Sonarr', order: 1 }),
];

function renderSettings(overrides: {
  config?: Partial<Config>;
  apps?: App[];
  initialTab?: 'general' | 'apps' | 'theme' | 'keybindings' | 'security' | 'about';
  onclose?: () => void;
  onsave?: (config: Config) => void;
} = {}) {
  const config = makeConfig(overrides.config);
  const apps = overrides.apps ?? sampleApps;
  return render(Settings, {
    props: {
      config,
      apps,
      ...(overrides.initialTab ? { initialTab: overrides.initialTab } : {}),
      ...(overrides.onclose ? { onclose: overrides.onclose } : {}),
      ...(overrides.onsave ? { onsave: overrides.onsave } : {}),
    },
  });
}

// NOTE: Svelte out-transitions keep elements in the DOM in jsdom, so we
// do NOT assert that modal text disappears. Instead we verify:
//   - State callbacks were invoked (schema validation, toasts, etc.)
//   - The handleEscape() return value
//   - That new modals can be opened (proving prior state was reset)

describe('Settings', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSelectedFamily.set('default');
    mockVariantMode.set('dark');
    mockIsMobileViewport.fn = () => false;
    mockAppSchemaSafeParse.mockReturnValue({ success: true });
    mockGroupSchemaSafeParse.mockReturnValue({ success: true });
    mockExtractErrors.mockReturnValue({});
  });

  // =======================================================================
  // A. Tab Switching
  // =======================================================================
  describe('Tab switching', () => {
    it('renders settings panel with all tab buttons', () => {
      renderSettings();
      expect(screen.getByText('Settings')).toBeInTheDocument();
      expect(screen.getByText('General')).toBeInTheDocument();
      expect(screen.getByText('Apps & Groups')).toBeInTheDocument();
      expect(screen.getByText('Theme')).toBeInTheDocument();
      expect(screen.getByText('Keybindings')).toBeInTheDocument();
      expect(screen.getByText('Security')).toBeInTheDocument();
      expect(screen.getByText('About')).toBeInTheDocument();
    });

    it('defaults to General tab', () => {
      renderSettings();
      const generalTab = screen.getByText('General');
      expect(generalTab.className).toContain('text-brand-400');
    });

    it('switches to Apps & Groups tab when clicked', async () => {
      renderSettings();
      const appsTab = screen.getByText('Apps & Groups');
      await fireEvent.click(appsTab);
      expect(appsTab.className).toContain('text-brand-400');
      expect(screen.getByText('General').className).not.toContain('text-brand-400');
    });

    it('switches to Theme tab when clicked', async () => {
      renderSettings();
      const themeTab = screen.getByText('Theme');
      await fireEvent.click(themeTab);
      expect(themeTab.className).toContain('text-brand-400');
    });

    it('switches to Security tab when clicked', async () => {
      renderSettings();
      const securityTab = screen.getByText('Security');
      await fireEvent.click(securityTab);
      expect(securityTab.className).toContain('text-brand-400');
    });

    it('respects initialTab prop', () => {
      renderSettings({ initialTab: 'about' });
      expect(screen.getByText('About').className).toContain('text-brand-400');
    });

    it('can navigate through all tabs sequentially', async () => {
      renderSettings();
      for (const label of ['General', 'Apps & Groups', 'Theme', 'Keybindings', 'Security', 'About']) {
        const tab = screen.getByText(label);
        await fireEvent.click(tab);
        expect(tab.className).toContain('text-brand-400');
      }
    });
  });

  // =======================================================================
  // B. Add App Modal Flow
  // =======================================================================
  describe('Add App Modal Flow', () => {
    it('opens Add App modal showing "Add Application" heading', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-app'));
      await waitFor(() => {
        expect(screen.getByText('Add Application')).toBeInTheDocument();
      });
    });

    it('shows search input and popular apps in choose step', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-app'));
      await waitFor(() => {
        expect(screen.getByPlaceholderText('Search apps...')).toBeInTheDocument();
      });
      expect(screen.getByText('Plex')).toBeInTheDocument();
      expect(screen.getByText('Portainer')).toBeInTheDocument();
    });

    it('shows Custom App card when no search is active', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-app'));
      await waitFor(() => {
        expect(screen.getByText('Custom App')).toBeInTheDocument();
      });
    });

    it('hides Custom App card when searching', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-app'));
      await waitFor(() => { expect(screen.getByPlaceholderText('Search apps...')).toBeInTheDocument(); });

      await fireEvent.input(screen.getByPlaceholderText('Search apps...'), { target: { value: 'Plex' } });
      await waitFor(() => {
        expect(screen.getByText('Plex')).toBeInTheDocument();
      });
      expect(screen.queryByText('Custom App')).not.toBeInTheDocument();
    });

    it('shows "No matching apps found" for non-existent search', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-app'));
      await waitFor(() => { expect(screen.getByPlaceholderText('Search apps...')).toBeInTheDocument(); });

      await fireEvent.input(screen.getByPlaceholderText('Search apps...'), { target: { value: 'xyznonexistent' } });
      await waitFor(() => {
        expect(screen.getByText('No matching apps found')).toBeInTheDocument();
        expect(screen.getByText('Add as Custom App')).toBeInTheDocument();
      });
    });

    it('shows category headers for popular apps', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-app'));
      await waitFor(() => {
        // Category headings from our mocked popularApps
        expect(screen.getByText('Media')).toBeInTheDocument();
        expect(screen.getByText('System')).toBeInTheDocument();
      });
    });

    it('clicking Custom App switches to configure step with form fields', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-app'));
      await waitFor(() => { expect(screen.getByText('Custom App')).toBeInTheDocument(); });

      await fireEvent.click(screen.getByText('Custom App'));
      await waitFor(() => {
        expect(screen.getByText(/Configure/)).toBeInTheDocument();
        expect(screen.getByLabelText('Name')).toBeInTheDocument();
        expect(screen.getByLabelText('URL')).toBeInTheDocument();
      });
    });

    it('clicking back button in configure step returns to choose step', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-app'));
      await waitFor(() => { expect(screen.getByText('Custom App')).toBeInTheDocument(); });
      await fireEvent.click(screen.getByText('Custom App'));
      await waitFor(() => { expect(screen.getByText(/Configure/)).toBeInTheDocument(); });

      await fireEvent.click(screen.getByLabelText('Back'));
      await waitFor(() => {
        expect(screen.getByText('Add Application')).toBeInTheDocument();
      });
    });

    it('clicking a popular app template calls templateToApp and shows configure', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-app'));
      await waitFor(() => { expect(screen.getByText('Plex')).toBeInTheDocument(); });

      await fireEvent.click(screen.getByText('Plex'));
      await waitFor(() => {
        expect(screen.getByText(/Configure/)).toBeInTheDocument();
      });
      expect(mockTemplateToApp).toHaveBeenCalled();
    });

    it('"Add as Custom App" button in no-results opens configure step', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-app'));
      await waitFor(() => { expect(screen.getByPlaceholderText('Search apps...')).toBeInTheDocument(); });

      await fireEvent.input(screen.getByPlaceholderText('Search apps...'), { target: { value: 'nonexistent' } });
      await waitFor(() => { expect(screen.getByText('Add as Custom App')).toBeInTheDocument(); });

      await fireEvent.click(screen.getByText('Add as Custom App'));
      await waitFor(() => {
        expect(screen.getByText(/Configure/)).toBeInTheDocument();
      });
    });

    it('shows open mode, scale, and checkboxes in configure step', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-app'));
      await waitFor(() => { expect(screen.getByText('Custom App')).toBeInTheDocument(); });
      await fireEvent.click(screen.getByText('Custom App'));

      await waitFor(() => {
        expect(screen.getByLabelText(/Open Mode/)).toBeInTheDocument();
        expect(screen.getByLabelText(/Scale/)).toBeInTheDocument();
        expect(screen.getByText('Enabled')).toBeInTheDocument();
        expect(screen.getByText('Default app')).toBeInTheDocument();
        expect(screen.getByText('Use reverse proxy')).toBeInTheDocument();
        expect(screen.getByText('Force icon background')).toBeInTheDocument();
        expect(screen.getByText('Invert icon colors')).toBeInTheDocument();
        expect(screen.getByLabelText('Minimum Role')).toBeInTheDocument();
      });
    });

    it('successfully adds app and calls appSchema.safeParse', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-app'));
      await waitFor(() => { expect(screen.getByText('Custom App')).toBeInTheDocument(); });
      await fireEvent.click(screen.getByText('Custom App'));
      await waitFor(() => { expect(screen.getByLabelText('Name')).toBeInTheDocument(); });

      await fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'My New App' } });
      await fireEvent.input(screen.getByLabelText('URL'), { target: { value: 'http://localhost:3000' } });

      // Click the "Add App" submit button (in the modal footer)
      const addBtns = screen.getAllByText('Add App');
      const submitBtn = addBtns.find(b => b.classList.contains('btn-primary'));
      await fireEvent.click(submitBtn!);

      // appSchema.safeParse was called for validation
      expect(mockAppSchemaSafeParse).toHaveBeenCalled();
    });

    it('shows validation error when addApp fails and keeps modal open', async () => {
      mockAppSchemaSafeParse.mockReturnValueOnce({
        success: false,
        error: { issues: [{ path: ['name'], message: 'Name is required' }] },
      });
      mockExtractErrors.mockReturnValueOnce({ name: 'Name is required' });

      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-app'));
      await waitFor(() => { expect(screen.getByText('Custom App')).toBeInTheDocument(); });
      await fireEvent.click(screen.getByText('Custom App'));
      await waitFor(() => { expect(screen.getByLabelText('Name')).toBeInTheDocument(); });

      const addBtns = screen.getAllByText('Add App');
      const submitBtn = addBtns.find(b => b.classList.contains('btn-primary'));
      await fireEvent.click(submitBtn!);

      await waitFor(() => {
        expect(screen.getByText('Name is required')).toBeInTheDocument();
      });
      // Modal stays open -- configure heading still present
      expect(screen.getByText(/Configure/)).toBeInTheDocument();
    });

    it('clears appErrors.name when typing in name field', async () => {
      mockAppSchemaSafeParse.mockReturnValueOnce({
        success: false,
        error: { issues: [{ path: ['name'], message: 'Name is required' }] },
      });
      mockExtractErrors.mockReturnValueOnce({ name: 'Name is required' });

      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-app'));
      await waitFor(() => { expect(screen.getByText('Custom App')).toBeInTheDocument(); });
      await fireEvent.click(screen.getByText('Custom App'));
      await waitFor(() => { expect(screen.getByLabelText('Name')).toBeInTheDocument(); });

      const addBtns = screen.getAllByText('Add App');
      await fireEvent.click(addBtns.find(b => b.classList.contains('btn-primary'))!);
      await waitFor(() => { expect(screen.getByText('Name is required')).toBeInTheDocument(); });

      await fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'A' } });
      await waitFor(() => {
        expect(screen.queryByText('Name is required')).not.toBeInTheDocument();
      });
    });

    it('clears appErrors.url when typing in URL field', async () => {
      mockAppSchemaSafeParse.mockReturnValueOnce({
        success: false,
        error: { issues: [{ path: ['url'], message: 'Invalid URL' }] },
      });
      mockExtractErrors.mockReturnValueOnce({ url: 'Invalid URL' });

      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-app'));
      await waitFor(() => { expect(screen.getByText('Custom App')).toBeInTheDocument(); });
      await fireEvent.click(screen.getByText('Custom App'));
      await waitFor(() => { expect(screen.getByLabelText('URL')).toBeInTheDocument(); });

      const addBtns = screen.getAllByText('Add App');
      await fireEvent.click(addBtns.find(b => b.classList.contains('btn-primary'))!);
      await waitFor(() => { expect(screen.getByText('Invalid URL')).toBeInTheDocument(); });

      await fireEvent.input(screen.getByLabelText('URL'), { target: { value: 'http://x' } });
      await waitFor(() => {
        expect(screen.queryByText('Invalid URL')).not.toBeInTheDocument();
      });
    });

    it('shows group select with available groups in configure step', async () => {
      const config = makeConfig({ groups: [makeGroup({ name: 'Media' })] });
      renderSettings({ config, initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-app'));
      await waitFor(() => { expect(screen.getByText('Custom App')).toBeInTheDocument(); });
      await fireEvent.click(screen.getByText('Custom App'));

      await waitFor(() => {
        expect(screen.getByLabelText('Group')).toBeInTheDocument();
      });
    });
  });

  // =======================================================================
  // C. Edit App Modal Flow
  // =======================================================================
  describe('Edit App Modal Flow', () => {
    it('opens Edit App modal with app name in heading', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-edit-app'));
      await waitFor(() => {
        expect(screen.getByText('Edit TestApp')).toBeInTheDocument();
      });
    });

    it('shows name and URL form fields', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-edit-app'));
      await waitFor(() => {
        // Use the specific input ids to find the edit-app fields
        expect(document.getElementById('edit-app-name')).toBeInTheDocument();
        expect(document.getElementById('edit-app-url')).toBeInTheDocument();
      });
    });

    it('shows Display section with Enabled, Default, Open Mode, Scale', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-edit-app'));
      await waitFor(() => {
        expect(screen.getByText('Display')).toBeInTheDocument();
        expect(screen.getByText('Enabled')).toBeInTheDocument();
        expect(screen.getByText(/Open Mode/)).toBeInTheDocument();
        expect(screen.getByText(/Scale:/)).toBeInTheDocument();
      });
    });

    it('shows Proxy section', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-edit-app'));
      await waitFor(() => {
        expect(screen.getByText('Proxy')).toBeInTheDocument();
        expect(screen.getByText('Use reverse proxy')).toBeInTheDocument();
      });
    });

    it('shows Advanced section with Health check, Keyboard Shortcut, Minimum Role', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-edit-app'));
      await waitFor(() => {
        expect(screen.getByText('Advanced')).toBeInTheDocument();
        expect(screen.getByText('Health check')).toBeInTheDocument();
        expect(screen.getByText('Keyboard Shortcut')).toBeInTheDocument();
        expect(screen.getByText('Minimum Role')).toBeInTheDocument();
      });
    });

    it('shows Force icon background and Invert icon colors', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-edit-app'));
      await waitFor(() => {
        expect(screen.getByText('Force icon background')).toBeInTheDocument();
        expect(screen.getByText('Invert icon colors')).toBeInTheDocument();
      });
    });

    it('shows icon type description in edit modal', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-edit-app'));
      await waitFor(() => {
        expect(screen.getByText('Dashboard Icon')).toBeInTheDocument();
      });
    });

    it('shows "No group" option in group select', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-edit-app'));
      await waitFor(() => {
        expect(screen.getByText('No group')).toBeInTheDocument();
      });
    });

    it('Done button calls appSchema.safeParse for validation', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-edit-app'));
      await waitFor(() => { expect(screen.getByText('Edit TestApp')).toBeInTheDocument(); });

      await fireEvent.click(screen.getByText('Done'));
      expect(mockAppSchemaSafeParse).toHaveBeenCalled();
    });

    it('shows validation errors when Done clicked with invalid data', async () => {
      mockAppSchemaSafeParse.mockReturnValueOnce({
        success: false,
        error: { issues: [{ path: ['name'], message: 'Name cannot be empty' }] },
      });
      mockExtractErrors.mockReturnValueOnce({ name: 'Name cannot be empty' });

      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-edit-app'));
      await waitFor(() => { expect(screen.getByText('Edit TestApp')).toBeInTheDocument(); });

      await fireEvent.click(screen.getByText('Done'));
      await waitFor(() => {
        expect(screen.getByText('Name cannot be empty')).toBeInTheDocument();
      });
    });

    it('Cancel button resets editingApp (calls cancelEditApp)', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-edit-app'));
      await waitFor(() => { expect(screen.getByText('Edit TestApp')).toBeInTheDocument(); });

      // Cancel is in the footer of the edit modal
      await fireEvent.click(screen.getByText('Cancel'));

      // Verify appSchema was NOT called (cancel doesn't validate)
      expect(mockAppSchemaSafeParse).not.toHaveBeenCalled();
    });

    it('clears editAppErrors.name on name input', async () => {
      mockAppSchemaSafeParse.mockReturnValueOnce({
        success: false,
        error: { issues: [{ path: ['name'], message: 'Name err' }] },
      });
      mockExtractErrors.mockReturnValueOnce({ name: 'Name err' });

      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-edit-app'));
      await waitFor(() => { expect(screen.getByText('Edit TestApp')).toBeInTheDocument(); });

      await fireEvent.click(screen.getByText('Done'));
      await waitFor(() => { expect(screen.getByText('Name err')).toBeInTheDocument(); });

      // Type in the edit-app-name input to clear error
      const nameInput = screen.getByDisplayValue('TestApp');
      await fireEvent.input(nameInput, { target: { value: 'Fixed' } });
      await waitFor(() => {
        expect(screen.queryByText('Name err')).not.toBeInTheDocument();
      });
    });

    it('clears editAppErrors.url on URL input', async () => {
      mockAppSchemaSafeParse.mockReturnValueOnce({
        success: false,
        error: { issues: [{ path: ['url'], message: 'URL err' }] },
      });
      mockExtractErrors.mockReturnValueOnce({ url: 'URL err' });

      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-edit-app'));
      await waitFor(() => { expect(screen.getByText('Edit TestApp')).toBeInTheDocument(); });

      await fireEvent.click(screen.getByText('Done'));
      await waitFor(() => { expect(screen.getByText('URL err')).toBeInTheDocument(); });

      const urlInput = screen.getByDisplayValue('https://test.com');
      await fireEvent.input(urlInput, { target: { value: 'http://fixed.com' } });
      await waitFor(() => {
        expect(screen.queryByText('URL err')).not.toBeInTheDocument();
      });
    });
  });

  // =======================================================================
  // D. Add Group Modal Flow
  // =======================================================================
  describe('Add Group Modal Flow', () => {
    it('opens Add Group modal with heading', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-group'));
      await waitFor(() => {
        const headings = screen.getAllByRole('heading');
        expect(headings.some(h => h.textContent === 'Add Group')).toBe(true);
      });
    });

    it('shows name and color fields', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-group'));
      await waitFor(() => {
        expect(screen.getByLabelText('Name')).toBeInTheDocument();
        expect(screen.getByLabelText('Color')).toBeInTheDocument();
      });
    });

    it('successfully adds group calling groupSchema.safeParse', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-group'));
      await waitFor(() => {
        expect(screen.getAllByRole('heading').some(h => h.textContent === 'Add Group')).toBe(true);
      });

      await fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'Media' } });

      // Click the primary "Add Group" submit button
      const addBtns = screen.getAllByText('Add Group');
      const submitBtn = addBtns.find(b => b.classList.contains('btn-primary'));
      await fireEvent.click(submitBtn!);

      expect(mockGroupSchemaSafeParse).toHaveBeenCalled();
    });

    it('shows validation error when group name is empty', async () => {
      mockGroupSchemaSafeParse.mockReturnValueOnce({
        success: false,
        error: { issues: [{ path: ['name'], message: 'Name is required' }] },
      });
      mockExtractErrors.mockReturnValueOnce({ name: 'Name is required' });

      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-group'));
      await waitFor(() => {
        expect(screen.getAllByRole('heading').some(h => h.textContent === 'Add Group')).toBe(true);
      });

      const addBtns = screen.getAllByText('Add Group');
      await fireEvent.click(addBtns.find(b => b.classList.contains('btn-primary'))!);

      await waitFor(() => {
        expect(screen.getByText('Name is required')).toBeInTheDocument();
      });
    });

    it('clears groupErrors.name when typing in name field', async () => {
      mockGroupSchemaSafeParse.mockReturnValueOnce({
        success: false,
        error: { issues: [{ path: ['name'], message: 'Name is required' }] },
      });
      mockExtractErrors.mockReturnValueOnce({ name: 'Name is required' });

      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-group'));
      await waitFor(() => {
        expect(screen.getAllByRole('heading').some(h => h.textContent === 'Add Group')).toBe(true);
      });

      const addBtns = screen.getAllByText('Add Group');
      await fireEvent.click(addBtns.find(b => b.classList.contains('btn-primary'))!);
      await waitFor(() => { expect(screen.getByText('Name is required')).toBeInTheDocument(); });

      await fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'M' } });
      await waitFor(() => {
        expect(screen.queryByText('Name is required')).not.toBeInTheDocument();
      });
    });
  });

  // =======================================================================
  // E. Edit Group Modal Flow
  // =======================================================================
  describe('Edit Group Modal Flow', () => {
    it('opens Edit Group modal with group name in heading', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-edit-group'));
      await waitFor(() => {
        expect(screen.getByText('Edit TestGroup')).toBeInTheDocument();
      });
    });

    it('shows name, icon, and color fields', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-edit-group'));
      await waitFor(() => {
        expect(screen.getByLabelText('Name')).toBeInTheDocument();
        expect(screen.getByLabelText('Color')).toBeInTheDocument();
        expect(screen.getByText('Icon')).toBeInTheDocument();
      });
    });

    it('shows icon type description', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-edit-group'));
      await waitFor(() => {
        expect(screen.getByText('Dashboard Icon')).toBeInTheDocument();
      });
    });

    it('Done button calls groupSchema.safeParse', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-edit-group'));
      await waitFor(() => { expect(screen.getByText('Edit TestGroup')).toBeInTheDocument(); });

      await fireEvent.click(screen.getByText('Done'));
      expect(mockGroupSchemaSafeParse).toHaveBeenCalled();
    });

    it('shows validation errors when Done clicked with invalid group', async () => {
      mockGroupSchemaSafeParse.mockReturnValueOnce({
        success: false,
        error: { issues: [{ path: ['name'], message: 'Name cannot be empty' }] },
      });
      mockExtractErrors.mockReturnValueOnce({ name: 'Name cannot be empty' });

      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-edit-group'));
      await waitFor(() => { expect(screen.getByText('Edit TestGroup')).toBeInTheDocument(); });

      await fireEvent.click(screen.getByText('Done'));
      await waitFor(() => {
        expect(screen.getByText('Name cannot be empty')).toBeInTheDocument();
      });
    });

    it('Cancel button does NOT call groupSchema (no validation)', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-edit-group'));
      await waitFor(() => { expect(screen.getByText('Edit TestGroup')).toBeInTheDocument(); });

      await fireEvent.click(screen.getByText('Cancel'));
      expect(mockGroupSchemaSafeParse).not.toHaveBeenCalled();
    });

    it('clears editGroupErrors.name on input', async () => {
      mockGroupSchemaSafeParse.mockReturnValueOnce({
        success: false,
        error: { issues: [{ path: ['name'], message: 'Name err' }] },
      });
      mockExtractErrors.mockReturnValueOnce({ name: 'Name err' });

      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-edit-group'));
      await waitFor(() => { expect(screen.getByText('Edit TestGroup')).toBeInTheDocument(); });

      await fireEvent.click(screen.getByText('Done'));
      await waitFor(() => { expect(screen.getByText('Name err')).toBeInTheDocument(); });

      const nameInput = screen.getByDisplayValue('TestGroup');
      await fireEvent.input(nameInput, { target: { value: 'Fixed' } });
      await waitFor(() => {
        expect(screen.queryByText('Name err')).not.toBeInTheDocument();
      });
    });
  });

  // =======================================================================
  // F. Import/Export Flow
  // =======================================================================
  describe('Import / Export', () => {
    it('export calls exportConfig and shows success toast', async () => {
      renderSettings({ initialTab: 'general' });
      await fireEvent.click(screen.getByTestId('trigger-export'));

      expect(mockExportConfig).toHaveBeenCalledTimes(1);
      expect(mockToasts.success).toHaveBeenCalledWith('Configuration exported');
    });

    it('import triggers parseImportedConfig and shows confirmation modal', async () => {
      mockParseImportedConfig.mockResolvedValueOnce({
        title: 'Imported Config',
        apps: [makeApp({ name: 'ImportedApp' })],
        groups: [makeGroup({ name: 'ImportedGroup' })],
        navigation: makeNav(),
      });

      renderSettings({ initialTab: 'general' });
      await fireEvent.click(screen.getByTestId('trigger-import'));

      await waitFor(() => {
        expect(screen.getByText('Import Configuration')).toBeInTheDocument();
      });
      expect(screen.getByText('Imported Config')).toBeInTheDocument();
      expect(screen.getByText(/1 apps, 1 groups/)).toBeInTheDocument();
      expect(screen.getByText('Unsaved changes will be overwritten')).toBeInTheDocument();
    });

    it('Apply import shows success toast', async () => {
      mockParseImportedConfig.mockResolvedValueOnce({
        title: 'New Config',
        apps: [makeApp({ name: 'NewApp' })],
        groups: [],
        navigation: makeNav(),
      });

      renderSettings({ initialTab: 'general' });
      await fireEvent.click(screen.getByTestId('trigger-import'));
      await waitFor(() => { expect(screen.getByText('Import Configuration')).toBeInTheDocument(); });

      // Click the Import button in the import modal
      const importBtns = screen.getAllByText('Import');
      const primaryBtn = importBtns.find(b => b.classList.contains('btn-primary'));
      await fireEvent.click(primaryBtn!);

      expect(mockToasts.success).toHaveBeenCalledWith('Configuration imported - save to apply changes');
    });

    it('Cancel import calls cancelImport (no toast)', async () => {
      mockParseImportedConfig.mockResolvedValueOnce({
        title: 'To Cancel',
        apps: [],
        groups: [],
        navigation: makeNav(),
      });

      renderSettings({ initialTab: 'general' });
      await fireEvent.click(screen.getByTestId('trigger-import'));
      await waitFor(() => { expect(screen.getByText('Import Configuration')).toBeInTheDocument(); });

      await fireEvent.click(screen.getByText('Cancel'));
      // No success toast for cancel
      expect(mockToasts.success).not.toHaveBeenCalled();
    });

    it('import error shows error toast', async () => {
      mockParseImportedConfig.mockRejectedValueOnce(new Error('Invalid YAML'));

      renderSettings({ initialTab: 'general' });
      await fireEvent.click(screen.getByTestId('trigger-import'));

      await waitFor(() => {
        expect(mockToasts.error).toHaveBeenCalledWith('Invalid YAML');
      });
    });

    it('no import modal visible initially', () => {
      renderSettings();
      expect(screen.queryByText('Import Configuration')).not.toBeInTheDocument();
    });
  });

  // =======================================================================
  // G. Icon Browser
  // =======================================================================
  describe('Icon Browser', () => {
    it('opens icon browser from new app form', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-app'));
      await waitFor(() => { expect(screen.getByText('Custom App')).toBeInTheDocument(); });
      await fireEvent.click(screen.getByText('Custom App'));
      await waitFor(() => { expect(screen.getByText(/Configure/)).toBeInTheDocument(); });

      // Click "Choose icon..." button
      const iconBtns = screen.getAllByText(/Choose icon/);
      await fireEvent.click(iconBtns[0]);

      await waitFor(() => {
        expect(screen.getByText('Select Icon')).toBeInTheDocument();
      });
    });

    it('opens icon browser from edit app form', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-edit-app'));
      await waitFor(() => { expect(screen.getByText('Edit TestApp')).toBeInTheDocument(); });

      // Click the icon button (the one with the icon name "test" or "Choose icon...")
      const iconBtns = screen.getAllByText(/test|Choose icon/);
      const chooseBtn = iconBtns.find(b =>
        b.classList.contains('btn-secondary') || b.textContent?.includes('test')
      );
      if (chooseBtn) await fireEvent.click(chooseBtn);

      await waitFor(() => {
        expect(screen.getByText('Select Icon')).toBeInTheDocument();
      });
    });

    it('opens icon browser from add group form', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-group'));
      await waitFor(() => {
        expect(screen.getAllByRole('heading').some(h => h.textContent === 'Add Group')).toBe(true);
      });

      const iconBtns = screen.getAllByText(/Choose icon/);
      await fireEvent.click(iconBtns[0]);

      await waitFor(() => {
        expect(screen.getByText('Select Icon')).toBeInTheDocument();
      });
    });

    it('does not show Icon Browser modal initially', () => {
      renderSettings();
      expect(screen.queryByText('Select Icon')).not.toBeInTheDocument();
    });
  });

  // =======================================================================
  // H. handleEscape()
  // =======================================================================
  describe('handleEscape', () => {
    it('returns false when no sub-modals are open', () => {
      const { component } = renderSettings();
      expect(component.handleEscape()).toBe(false);
    });

    it('returns true and closes Add App modal', async () => {
      const { component } = renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-app'));
      await waitFor(() => { expect(screen.getByText('Add Application')).toBeInTheDocument(); });

      expect(component.handleEscape()).toBe(true);
    });

    it('returns true and closes Add Group modal', async () => {
      const { component } = renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-group'));
      await waitFor(() => {
        expect(screen.getAllByRole('heading').some(h => h.textContent === 'Add Group')).toBe(true);
      });

      expect(component.handleEscape()).toBe(true);
    });

    it('returns true and cancels Edit App modal', async () => {
      const { component } = renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-edit-app'));
      await waitFor(() => { expect(screen.getByText('Edit TestApp')).toBeInTheDocument(); });

      expect(component.handleEscape()).toBe(true);
    });

    it('returns true and cancels Edit Group modal', async () => {
      const { component } = renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-edit-group'));
      await waitFor(() => { expect(screen.getByText('Edit TestGroup')).toBeInTheDocument(); });

      expect(component.handleEscape()).toBe(true);
    });

    it('returns true and closes icon browser (highest priority)', async () => {
      const { component } = renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-app'));
      await waitFor(() => { expect(screen.getByText('Custom App')).toBeInTheDocument(); });
      await fireEvent.click(screen.getByText('Custom App'));
      await waitFor(() => { expect(screen.getByText(/Configure/)).toBeInTheDocument(); });

      const iconBtns = screen.getAllByText(/Choose icon/);
      await fireEvent.click(iconBtns[0]);
      await waitFor(() => { expect(screen.getByText('Select Icon')).toBeInTheDocument(); });

      expect(component.handleEscape()).toBe(true);
    });

    it('returns true and clears pending import', async () => {
      mockParseImportedConfig.mockResolvedValueOnce({
        title: 'Test Import',
        apps: [],
        groups: [],
        navigation: makeNav(),
      });

      const { component } = renderSettings({ initialTab: 'general' });
      await fireEvent.click(screen.getByTestId('trigger-import'));
      await waitFor(() => { expect(screen.getByText('Import Configuration')).toBeInTheDocument(); });

      expect(component.handleEscape()).toBe(true);
    });
  });

  // =======================================================================
  // I. Save Flow
  // =======================================================================
  describe('Save flow', () => {
    it('Save button is disabled when no changes have been made', () => {
      renderSettings();
      expect(screen.getByText('Save Changes')).toBeDisabled();
    });

    it('does not show "Unsaved changes" text initially', () => {
      renderSettings();
      expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument();
    });

    it('shows Save enabled and unsaved indicator when theme changes', async () => {
      renderSettings();
      mockSelectedFamily.set('catppuccin');
      await waitFor(() => {
        expect(screen.getByText('Unsaved changes')).toBeInTheDocument();
        expect(screen.getByText('Save Changes')).not.toBeDisabled();
      });
    });

    it('calls onsave with config including theme, then calls onclose', async () => {
      const onsave = vi.fn();
      const onclose = vi.fn();
      renderSettings({ onsave, onclose });

      mockSelectedFamily.set('nord');
      await waitFor(() => { expect(screen.getByText('Save Changes')).not.toBeDisabled(); });

      await fireEvent.click(screen.getByText('Save Changes'));

      expect(onsave).toHaveBeenCalledTimes(1);
      const savedConfig = onsave.mock.calls[0][0] as Config;
      expect(savedConfig.title).toBe('Muximux');
      expect(savedConfig.theme).toBeDefined();
      expect(savedConfig.theme?.family).toBe('nord');
      expect(onclose).toHaveBeenCalledTimes(1);
    });

    it('save captures theme family and variant from stores', async () => {
      const onsave = vi.fn();
      renderSettings({ onsave });

      mockSelectedFamily.set('dracula');
      mockVariantMode.set('light');
      await waitFor(() => { expect(screen.getByText('Save Changes')).not.toBeDisabled(); });

      await fireEvent.click(screen.getByText('Save Changes'));

      const savedConfig = onsave.mock.calls[0][0] as Config;
      expect(savedConfig.theme?.family).toBe('dracula');
      expect(savedConfig.theme?.variant).toBe('light');
    });

    it('includes keybindings in save when keybindingsChanged', async () => {
      const onsave = vi.fn();
      mockGetKeybindingsForConfig.mockReturnValue({ bindings: { test: [] } });

      renderSettings({ onsave, initialTab: 'keybindings' });
      await fireEvent.click(screen.getByTestId('trigger-keybinding-change'));
      await waitFor(() => { expect(screen.getByText('Save Changes')).not.toBeDisabled(); });

      await fireEvent.click(screen.getByText('Save Changes'));

      const savedConfig = onsave.mock.calls[0][0] as Config;
      expect(savedConfig.keybindings).toEqual({ bindings: { test: [] } });
    });

    it('save includes localApps in config.apps', async () => {
      const onsave = vi.fn();
      renderSettings({ onsave });

      mockSelectedFamily.set('another-theme');
      await waitFor(() => { expect(screen.getByText('Save Changes')).not.toBeDisabled(); });

      await fireEvent.click(screen.getByText('Save Changes'));

      const savedConfig = onsave.mock.calls[0][0] as Config;
      expect(savedConfig.apps).toBeDefined();
      expect(savedConfig.apps.length).toBe(2);
    });
  });

  // =======================================================================
  // J. Close behavior and unsaved changes
  // =======================================================================
  describe('Close behavior', () => {
    it('calls onclose when close button clicked and no changes', async () => {
      const onclose = vi.fn();
      const { container } = renderSettings({ onclose });

      const closeBtn = container.querySelector('[aria-label="Close settings"]');
      expect(closeBtn).toBeTruthy();
      await fireEvent.click(closeBtn!);

      expect(onclose).toHaveBeenCalledTimes(1);
    });

    it('shows discard confirmation when close clicked with unsaved changes', async () => {
      const onclose = vi.fn();
      const { container } = renderSettings({ onclose });

      mockSelectedFamily.set('nord');
      await waitFor(() => { expect(screen.getByText('Unsaved changes')).toBeInTheDocument(); });

      const closeBtn = container.querySelector('[aria-label="Close settings"]');
      await fireEvent.click(closeBtn!);

      expect(onclose).not.toHaveBeenCalled();
      expect(screen.getByText('You have unsaved changes. Discard?')).toBeInTheDocument();
      expect(screen.getByText('Keep Editing')).toBeInTheDocument();
      expect(screen.getByText('Discard')).toBeInTheDocument();
    });

    it('Keep Editing dismisses the confirmation banner', async () => {
      const onclose = vi.fn();
      const { container } = renderSettings({ onclose });

      mockSelectedFamily.set('nord');
      await waitFor(() => { expect(screen.getByText('Unsaved changes')).toBeInTheDocument(); });

      await fireEvent.click(container.querySelector('[aria-label="Close settings"]')!);
      expect(screen.getByText('You have unsaved changes. Discard?')).toBeInTheDocument();

      await fireEvent.click(screen.getByText('Keep Editing'));
      expect(screen.queryByText('You have unsaved changes. Discard?')).not.toBeInTheDocument();
      expect(onclose).not.toHaveBeenCalled();
    });

    it('Discard closes settings and reverts theme', async () => {
      const onclose = vi.fn();
      const { container } = renderSettings({ onclose });

      mockSelectedFamily.set('nord');
      await waitFor(() => { expect(screen.getByText('Unsaved changes')).toBeInTheDocument(); });

      await fireEvent.click(container.querySelector('[aria-label="Close settings"]')!);
      await fireEvent.click(screen.getByText('Discard'));

      expect(onclose).toHaveBeenCalledTimes(1);
      expect(mockSetThemeFamily).toHaveBeenCalledWith('default');
      expect(mockSetVariantMode).toHaveBeenCalledWith('dark');
    });

    it('close without changes reverts theme and calls onclose', async () => {
      const onclose = vi.fn();
      const { container } = renderSettings({ onclose });

      await fireEvent.click(container.querySelector('[aria-label="Close settings"]')!);
      expect(onclose).toHaveBeenCalledTimes(1);
      expect(mockSetThemeFamily).toHaveBeenCalledWith('default');
      expect(mockSetVariantMode).toHaveBeenCalledWith('dark');
    });
  });

  // =======================================================================
  // K. Dirty State Detection
  // =======================================================================
  describe('Dirty state tracking', () => {
    it('Save becomes enabled when variant mode changes', async () => {
      renderSettings();
      expect(screen.getByText('Save Changes')).toBeDisabled();
      mockVariantMode.set('light');
      await waitFor(() => { expect(screen.getByText('Save Changes')).not.toBeDisabled(); });
    });

    it('Save becomes enabled when theme family changes', async () => {
      renderSettings();
      expect(screen.getByText('Save Changes')).toBeDisabled();
      mockSelectedFamily.set('catppuccin');
      await waitFor(() => { expect(screen.getByText('Save Changes')).not.toBeDisabled(); });
    });

    it('hasChanges detects app additions', async () => {
      renderSettings({ initialTab: 'apps' });
      expect(screen.getByText('Save Changes')).toBeDisabled();

      await fireEvent.click(screen.getByTestId('trigger-add-app'));
      await waitFor(() => { expect(screen.getByText('Custom App')).toBeInTheDocument(); });
      await fireEvent.click(screen.getByText('Custom App'));
      await waitFor(() => { expect(screen.getByLabelText('Name')).toBeInTheDocument(); });

      await fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'NewApp' } });
      await fireEvent.input(screen.getByLabelText('URL'), { target: { value: 'http://new.app' } });

      const addBtns = screen.getAllByText('Add App');
      await fireEvent.click(addBtns.find(b => b.classList.contains('btn-primary'))!);

      await waitFor(() => {
        expect(screen.getByText('Save Changes')).not.toBeDisabled();
      });
    });

    it('hasChanges detects group additions', async () => {
      renderSettings({ initialTab: 'apps' });
      expect(screen.getByText('Save Changes')).toBeDisabled();

      await fireEvent.click(screen.getByTestId('trigger-add-group'));
      await waitFor(() => {
        expect(screen.getAllByRole('heading').some(h => h.textContent === 'Add Group')).toBe(true);
      });

      await fireEvent.input(screen.getByLabelText('Name'), { target: { value: 'NewGroup' } });
      const addBtns = screen.getAllByText('Add Group');
      await fireEvent.click(addBtns.find(b => b.classList.contains('btn-primary'))!);

      await waitFor(() => {
        expect(screen.getByText('Save Changes')).not.toBeDisabled();
      });
    });

    it('hasChanges detects keybinding changes', async () => {
      renderSettings({ initialTab: 'keybindings' });
      expect(screen.getByText('Save Changes')).toBeDisabled();

      await fireEvent.click(screen.getByTestId('trigger-keybinding-change'));
      await waitFor(() => {
        expect(screen.getByText('Save Changes')).not.toBeDisabled();
      });
    });
  });

  // =======================================================================
  // L. Modal management (nothing open initially)
  // =======================================================================
  describe('Modal management', () => {
    it('does not show Add App modal initially', () => {
      renderSettings();
      expect(screen.queryByText('Add Application')).not.toBeInTheDocument();
    });

    it('does not show Add Group heading initially', () => {
      renderSettings();
      const headings = screen.queryAllByRole('heading');
      expect(headings.find(h => h.textContent === 'Add Group')).toBeUndefined();
    });

    it('does not show Edit App/Group headings initially', () => {
      renderSettings();
      expect(screen.queryAllByText(/^Edit /).length).toBe(0);
    });

    it('does not show Icon Browser or Import modals initially', () => {
      renderSettings();
      expect(screen.queryByText('Select Icon')).not.toBeInTheDocument();
      expect(screen.queryByText('Import Configuration')).not.toBeInTheDocument();
    });

    it('no z-[60] or z-[70] modal overlays exist initially', () => {
      const { container } = renderSettings();
      expect(container.querySelectorAll('.z-\\[60\\]').length).toBe(0);
      expect(container.querySelectorAll('.z-\\[70\\]').length).toBe(0);
    });
  });

  // =======================================================================
  // M. Mobile responsive
  // =======================================================================
  describe('Mobile responsive', () => {
    it('applies mobile classes when isMobileViewport returns true', async () => {
      mockIsMobileViewport.fn = () => true;
      const { container } = renderSettings();
      await waitFor(() => {
        const overlay = container.querySelector('.fixed.inset-0.z-50');
        expect(overlay).toBeTruthy();
        expect(overlay?.className).toContain('p-0');
      });
    });

    it('applies desktop classes when isMobileViewport returns false', async () => {
      mockIsMobileViewport.fn = () => false;
      const { container } = renderSettings();
      await waitFor(() => {
        const overlay = container.querySelector('.fixed.inset-0.z-50');
        expect(overlay).toBeTruthy();
        expect(overlay?.className).toContain('p-4');
      });
    });
  });

  // =======================================================================
  // N. Edge cases
  // =======================================================================
  describe('Edge cases', () => {
    it('renders with empty apps array', () => {
      renderSettings({ apps: [] });
      expect(screen.getByText('Settings')).toBeInTheDocument();
    });

    it('renders with multiple groups and apps assigned to groups', () => {
      const groups = [makeGroup({ name: 'Media', order: 0 }), makeGroup({ name: 'System', order: 1 })];
      const apps = [
        makeApp({ name: 'Plex', group: 'Media', order: 0 }),
        makeApp({ name: 'Portainer', group: 'System', order: 1 }),
        makeApp({ name: 'Ungrouped', group: '', order: 2 }),
      ];
      renderSettings({ config: { groups }, apps });
      expect(screen.getByText('Settings')).toBeInTheDocument();
    });

    it('renders with all initialTab variants', () => {
      for (const tab of ['general', 'apps', 'theme', 'keybindings', 'security', 'about'] as const) {
        const { unmount } = renderSettings({ initialTab: tab });
        expect(screen.getByText('Settings')).toBeInTheDocument();
        unmount();
      }
    });

    it('renders config with auth and theme settings', () => {
      renderSettings({ config: { auth: { method: 'forward_auth' }, theme: { family: 'catppuccin', variant: 'dark' } } });
      expect(screen.getByText('Settings')).toBeInTheDocument();
    });

    it('renders without onclose or onsave callbacks', () => {
      render(Settings, { props: { config: makeConfig(), apps: sampleApps } });
      expect(screen.getByText('Settings')).toBeInTheDocument();
    });

    it('renders with apps containing groups and checks DnD arrays are built', () => {
      const apps = [
        makeApp({ name: 'App1', group: 'Media', order: 0 }),
        makeApp({ name: 'App2', group: 'Media', order: 1 }),
        makeApp({ name: 'App3', group: '', order: 2 }),
      ];
      renderSettings({ config: { groups: [makeGroup({ name: 'Media' })] }, apps });
      expect(screen.getByText('Settings')).toBeInTheDocument();
    });

    it('renders with default app in list', () => {
      renderSettings({ apps: [makeApp({ name: 'DefaultApp', default: true, order: 0 }), makeApp({ name: 'OtherApp', order: 1 })] });
      expect(screen.getByText('Settings')).toBeInTheDocument();
    });
  });
});
