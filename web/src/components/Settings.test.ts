import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import { normaliseBase } from '$lib/configMerge';

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
  mockFetchConfig,
  mockCustomBindings,
  mockInitKeybindings,
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
      get(): T {
        return value;
      },
    };
  }

  // Stands in for the keybindings store: Settings reads customBindings and
  // getKeybindingsForConfig, and discard calls initKeybindings.
  type Bindings = Record<string, { key: string; ctrl?: boolean }[]>;
  const mockCustomBindings = makeStore<Bindings>({});

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
      info: vi.fn(),
    },
    mockFetchConfig: vi.fn(),
    mockCustomBindings,
    mockInitKeybindings: vi.fn((kb?: { bindings?: Bindings }) => mockCustomBindings.set(kb?.bindings ?? {})),
    mockGetKeybindingsForConfig: vi.fn(() => {
      const b = Object.fromEntries(Object.entries(mockCustomBindings.get()).filter(([, c]) => c.length > 0));
      return { bindings: Object.keys(b).length > 0 ? b : undefined };
    }),
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

vi.mock('$lib/api', async (importOriginal) => ({
  getBase: vi.fn(() => ''),
  API_BASE: '',
  exportConfig: (...args: unknown[]) => mockExportConfig(...args),
  parseImportedConfig: (...args: unknown[]) => mockParseImportedConfig(...args),
  fetchConfig: (...args: unknown[]) => mockFetchConfig(...args),
  errorText: (await importOriginal<typeof import('$lib/api')>()).errorText,
}));

vi.mock('$lib/toastStore', () => ({
  toasts: mockToasts,
}));

vi.mock('$lib/keybindingsStore', () => ({
  customBindings: mockCustomBindings,
  getKeybindingsForConfig: (...args: unknown[]) => mockGetKeybindingsForConfig(...args),
  keybindings: { subscribe: (fn: (v: unknown[]) => void) => { fn([]); return () => {}; } },
  formatKeybinding: vi.fn(() => ''),
  formatKeyCombo: vi.fn(() => ''),
  initKeybindings: (...args: unknown[]) => mockInitKeybindings(...(args as [])),
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

// Live props of the mounted AppsTab, GatewayTab and DiscoverModal mocks.
// Bound and plain props are getters, so reads see the dialog's current state.
let appsTabProps: Record<string, unknown> = {};
let gatewayTabProps: Record<string, unknown> = {};
let discoverModalProps: Record<string, unknown> = {};
let securityTabProps: Record<string, unknown> = {};

function makeMockAppsTab(...args: unknown[]) {
  const { target, props } = resolveArgs(args);
  if (!target) return { $destroy() {} };
  appsTabProps = props;
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
  const { target } = resolveArgs(args);
  if (!target) return { $destroy() {} };
  const div = document.createElement('div');
  div.dataset.testid = 'mock-keybindings-editor';

  // The real editor writes straight into the keybindings store.
  const btn = document.createElement('button');
  btn.textContent = 'Change Keybinding';
  btn.dataset.testid = 'trigger-keybinding-change';
  btn.onclick = () => mockCustomBindings.set({ test: [{ key: 't' }] });
  div.appendChild(btn);
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

function makeMockAppForm(...args: unknown[]) {
  const { target, props } = resolveArgs(args);
  if (!target) return { $destroy() {} };
  const div = document.createElement('div');
  div.dataset.testid = 'mock-app-form';

  // Render essential form elements that the Settings tests query
  const prefix = props.mode === 'edit' ? 'edit' : 'create';
  const app = props.app as Record<string, unknown> | undefined;
  const errors = (props.errors ?? {}) as Record<string, string>;

  // Name field
  const nameLabel = document.createElement('label');
  nameLabel.htmlFor = `${prefix}-app-name`;
  nameLabel.textContent = 'Name';
  div.appendChild(nameLabel);
  const nameInput = document.createElement('input');
  nameInput.id = `${prefix}-app-name`;
  nameInput.type = 'text';
  nameInput.value = (app?.name as string) ?? '';
  nameInput.setAttribute('aria-label', 'Name');
  nameInput.oninput = () => {
    if (typeof props.onclearerror === 'function') props.onclearerror('name');
  };
  div.appendChild(nameInput);
  if (errors.name) {
    const err = document.createElement('p');
    err.textContent = errors.name;
    err.className = 'error-name';
    div.appendChild(err);
  }

  // URL field
  const urlLabel = document.createElement('label');
  urlLabel.htmlFor = `${prefix}-app-url`;
  urlLabel.textContent = 'URL';
  div.appendChild(urlLabel);
  const urlInput = document.createElement('input');
  urlInput.id = `${prefix}-app-url`;
  urlInput.type = 'url';
  urlInput.value = (app?.url as string) ?? '';
  urlInput.setAttribute('aria-label', 'URL');
  urlInput.oninput = () => {
    if (typeof props.onclearerror === 'function') props.onclearerror('url');
  };
  div.appendChild(urlInput);
  if (errors.url) {
    const err = document.createElement('p');
    err.textContent = errors.url;
    err.className = 'error-url';
    div.appendChild(err);
  }

  // Group field
  const groupLabel = document.createElement('label');
  groupLabel.htmlFor = `${prefix}-app-group`;
  groupLabel.textContent = 'Group';
  div.appendChild(groupLabel);
  const groupSelect = document.createElement('select');
  groupSelect.id = `${prefix}-app-group`;
  div.appendChild(groupSelect);

  // Display section elements
  const displayHeader = document.createElement('h4');
  displayHeader.textContent = 'Display';
  div.appendChild(displayHeader);
  for (const text of ['Enabled', 'Default app']) {
    const span = document.createElement('span');
    span.textContent = text;
    div.appendChild(span);
  }

  // Open Mode
  const modeLabel = document.createElement('label');
  modeLabel.htmlFor = `${prefix}-app-mode`;
  modeLabel.textContent = 'Open Mode';
  div.appendChild(modeLabel);
  const modeSelect = document.createElement('select');
  modeSelect.id = `${prefix}-app-mode`;
  div.appendChild(modeSelect);

  // Scale
  const scaleLabel = document.createElement('label');
  scaleLabel.htmlFor = `${prefix}-app-scale`;
  scaleLabel.textContent = 'Scale: 100%';
  div.appendChild(scaleLabel);
  const scaleInput = document.createElement('input');
  scaleInput.id = `${prefix}-app-scale`;
  scaleInput.type = 'range';
  div.appendChild(scaleInput);

  // Proxy section
  const proxyHeader = document.createElement('h4');
  proxyHeader.textContent = 'Proxy';
  div.appendChild(proxyHeader);
  const proxySpan = document.createElement('span');
  proxySpan.textContent = 'Use reverse proxy';
  div.appendChild(proxySpan);

  // Advanced section
  const advHeader = document.createElement('h4');
  advHeader.textContent = 'Advanced';
  div.appendChild(advHeader);
  for (const text of ['Health check', 'Keyboard Shortcut', 'Force icon background', 'Invert icon colors']) {
    const span = document.createElement('span');
    span.textContent = text;
    div.appendChild(span);
  }
  const minRoleLabel = document.createElement('label');
  minRoleLabel.htmlFor = `${prefix}-app-min-role`;
  minRoleLabel.textContent = 'Minimum Role';
  div.appendChild(minRoleLabel);
  const minRoleSelect = document.createElement('select');
  minRoleSelect.id = `${prefix}-app-min-role`;
  div.appendChild(minRoleSelect);

  // Icon type description
  const iconType = (app?.icon as Record<string, unknown>)?.type;
  if (iconType === 'dashboard') {
    const desc = document.createElement('p');
    desc.textContent = 'Dashboard Icon';
    div.appendChild(desc);
  }

  // Icon chooser button (triggers onopenicon callback)
  const iconBtn = document.createElement('button');
  iconBtn.className = 'btn btn-secondary btn-sm';
  const iconName = (app?.icon as Record<string, unknown>)?.name as string;
  iconBtn.textContent = iconName || 'Choose icon...';
  if (typeof props.onopenicon === 'function') {
    iconBtn.onclick = () => (props.onopenicon as () => void)();
  }
  div.appendChild(iconBtn);

  // No group option
  const noGroupOpt = document.createElement('option');
  noGroupOpt.textContent = 'No group';
  groupSelect.appendChild(noGroupOpt);

  target.appendChild(div);
  return { $destroy() { div.remove(); } };
}
vi.mock('./AppForm.svelte', () => ({ default: makeMockAppForm }));

vi.mock('./settings/AboutTab.svelte', () => ({ default: noopComponent }));
// Stands in for SecurityTab: mirrors a server-accepted auth method change into
// localConfig the way the real tab does, then reports it to the dialog.
function makeMockSecurityTab(...args: unknown[]) {
  const { target, props } = resolveArgs(args);
  if (!target) return { $destroy() {} };
  const div = document.createElement('div');
  div.dataset.testid = 'mock-security-tab';
  securityTabProps = props;
  const localConfig = props.localConfig as Config;
  const onmethodapplied = props.onmethodapplied as (() => void) | undefined;
  const add = (testid: string, onclick: () => void) => {
    const btn = document.createElement('button');
    btn.textContent = testid;
    btn.dataset.testid = testid;
    btn.onclick = onclick;
    div.appendChild(btn);
  };
  add('trigger-method-applied', () => {
    localConfig.auth = { method: 'forward_auth', trusted_proxies: ['10.0.0.0/8'] };
    onmethodapplied?.();
  });
  add('trigger-title-edit', () => { localConfig.title = 'Edited title'; });
  add('trigger-applied-without-auth', () => {
    delete localConfig.auth;
    onmethodapplied?.();
  });
  target.appendChild(div);
  return { $destroy() { div.remove(); } };
}
vi.mock('./settings/SecurityTab.svelte', () => ({ default: makeMockSecurityTab }));
vi.mock('./settings/ThemeTab.svelte', () => ({ default: noopComponent }));

function makeMockGatewayTab(...args: unknown[]) {
  const { target, props } = resolveArgs(args);
  if (!target) return { $destroy() {} };
  gatewayTabProps = props;
  return { $destroy() {} };
}
vi.mock('./settings/GatewayTab.svelte', () => ({ default: makeMockGatewayTab }));

function makeMockDiscoverModal(...args: unknown[]) {
  const { props } = resolveArgs(args);
  discoverModalProps = props;
  return { $destroy() {} };
}
vi.mock('./settings/DiscoverModal.svelte', () => ({ default: makeMockDiscoverModal }));

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
    max_open_tabs: 0,
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
  initialEditAppName?: string;
  onclose?: () => void;
  onsave?: (config: Config) => void;
  onauthchange?: (auth: Config['auth']) => void;
} = {}) {
  const config = makeConfig(overrides.config);
  const apps = overrides.apps ?? sampleApps;
  return render(Settings, {
    props: {
      config,
      apps,
      ...(overrides.initialTab ? { initialTab: overrides.initialTab } : {}),
      ...(overrides.initialEditAppName ? { initialEditAppName: overrides.initialEditAppName } : {}),
      ...(overrides.onclose ? { onclose: overrides.onclose } : {}),
      ...(overrides.onsave ? { onsave: overrides.onsave } : {}),
      ...(overrides.onauthchange ? { onauthchange: overrides.onauthchange } : {}),
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
    mockCustomBindings.set({});
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

    // #26: the modal must be an accessible, labelled dialog and move focus
    // into itself when opened.
    it('exposes the Add App modal as a labelled dialog and moves focus in', async () => {
      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-add-app'));

      const dialog = await screen.findByRole('dialog');
      expect(dialog).toHaveAttribute('aria-modal', 'true');
      // aria-labelledby points at the heading, so the accessible name is set.
      expect(dialog).toHaveAccessibleName('Add Application');
      // focusTrap moved focus into the dialog rather than leaving it on the
      // body / the trigger button outside.
      await waitFor(() => {
        expect(dialog.contains(document.activeElement)).toBe(true);
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

    it('keeps modal open when addApp validation fails', async () => {
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

      // Validation was called and failed
      expect(mockAppSchemaSafeParse).toHaveBeenCalled();
      expect(mockExtractErrors).toHaveBeenCalled();
      // Modal stays open -- configure heading still present
      expect(screen.getByText(/Configure/)).toBeInTheDocument();
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
    it('opens the edit modal directly when initialEditAppName is set (right-click deep link, #407)', async () => {
      renderSettings({ initialTab: 'apps', initialEditAppName: 'Sonarr' });
      await waitFor(() => {
        expect(screen.getByText('Edit Sonarr')).toBeInTheDocument();
      });
    });

    it('ignores an unknown initialEditAppName (no modal, no crash)', async () => {
      renderSettings({ initialTab: 'apps', initialEditAppName: 'DoesNotExist' });
      // The apps tab renders normally with no edit modal open.
      await waitFor(() => {
        expect(screen.getByTestId('trigger-edit-app')).toBeInTheDocument();
      });
      expect(screen.queryByText(/^Edit DoesNotExist$/)).not.toBeInTheDocument();
    });

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

    it('keeps edit modal open when Done clicked with invalid data', async () => {
      mockAppSchemaSafeParse.mockReturnValueOnce({
        success: false,
        error: { issues: [{ path: ['name'], message: 'Name cannot be empty' }] },
      });
      mockExtractErrors.mockReturnValueOnce({ name: 'Name cannot be empty' });

      renderSettings({ initialTab: 'apps' });
      await fireEvent.click(screen.getByTestId('trigger-edit-app'));
      await waitFor(() => { expect(screen.getByText('Edit TestApp')).toBeInTheDocument(); });

      await fireEvent.click(screen.getByText('Done'));

      // Validation was called and failed
      expect(mockAppSchemaSafeParse).toHaveBeenCalled();
      expect(mockExtractErrors).toHaveBeenCalled();
      // Modal stays open
      expect(screen.getByText('Edit TestApp')).toBeInTheDocument();
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

    it('stays clean after the Security tab applies an auth method change', async () => {
      const onauthchange = vi.fn();
      const onclose = vi.fn();
      const { container } = renderSettings({ initialTab: 'security', onauthchange, onclose });

      await fireEvent.click(screen.getByTestId('trigger-method-applied'));

      expect(onauthchange).toHaveBeenCalledWith({ method: 'forward_auth', trusted_proxies: ['10.0.0.0/8'] });
      expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument();
      expect(screen.getByText('Save Changes')).toBeDisabled();
      await fireEvent.click(container.querySelector('[aria-label="Close settings"]')!);
      expect(onclose).toHaveBeenCalledTimes(1);
    });

    it('keeps other unsaved edits after an auth method change', async () => {
      renderSettings({ initialTab: 'security' });

      await fireEvent.click(screen.getByTestId('trigger-title-edit'));
      await waitFor(() => expect(screen.getByText('Unsaved changes')).toBeInTheDocument());
      await fireEvent.click(screen.getByTestId('trigger-method-applied'));

      expect(screen.getByText('Unsaved changes')).toBeInTheDocument();
      expect(screen.getByText('Save Changes')).not.toBeDisabled();
    });

    it('ignores an applied notice when there is no auth block', async () => {
      const onauthchange = vi.fn();
      renderSettings({ initialTab: 'security', onauthchange });

      await fireEvent.click(screen.getByTestId('trigger-applied-without-auth'));

      expect(onauthchange).not.toHaveBeenCalled();
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

    it('includes keybindings in save when they were edited', async () => {
      const onsave = vi.fn();

      renderSettings({ onsave, initialTab: 'keybindings' });
      await fireEvent.click(screen.getByTestId('trigger-keybinding-change'));
      await waitFor(() => { expect(screen.getByText('Save Changes')).not.toBeDisabled(); });

      await fireEvent.click(screen.getByText('Save Changes'));

      const savedConfig = onsave.mock.calls[0][0] as Config;
      expect(savedConfig.keybindings).toEqual({ bindings: { test: [{ key: 't' }] } });
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
  // =======================================================================
  // O. Base-aware save and rebase onto server changes (#494)
  // =======================================================================
  describe('Save with base and rebase', () => {
    type Tab = 'general' | 'apps' | 'security' | 'gateway';

    // As the app shell passes them: config.apps and the apps prop agree.
    function serverConfig(apps: App[] = sampleApps, overrides: Partial<Config> = {}): Config {
      return makeConfig({ apps, ...overrides });
    }

    function renderFull(config: Config, opts: { initialTab?: Tab; onsave?: (c: Config, b: Config) => Promise<void>; onclose?: () => void } = {}) {
      return render(Settings, {
        props: {
          config,
          apps: config.apps,
          ...(opts.initialTab ? { initialTab: opts.initialTab } : {}),
          ...(opts.onsave ? { onsave: opts.onsave } : {}),
          ...(opts.onclose ? { onclose: opts.onclose } : {}),
        },
      });
    }

    function listed(group = ''): App[] {
      return ((appsTabProps.dndGroupedApps as Record<string, App[]>)[group] ?? []);
    }
    function names(group = ''): string[] {
      return listed(group).map(a => a.name);
    }

    const radarr = makeApp({ name: 'Radarr', order: 2 });

    beforeEach(() => {
      appsTabProps = {};
      gatewayTabProps = {};
      discoverModalProps = {};
    });

    it('sends base with the save', async () => {
      const config = serverConfig();
      const onsave = vi.fn().mockResolvedValue(undefined);
      const onclose = vi.fn();
      renderFull(config, { initialTab: 'security', onsave, onclose });

      await fireEvent.click(screen.getByTestId('trigger-title-edit'));
      await fireEvent.click(screen.getByText('Save Changes'));

      await waitFor(() => expect(onclose).toHaveBeenCalledTimes(1));
      expect(onsave).toHaveBeenCalledTimes(1);
      const [saved, base] = onsave.mock.calls[0] as [Config, Config];
      expect(saved.title).toBe('Edited title');
      // The server config, with apps and groups normalised like the payload.
      expect(base).toEqual(normaliseBase(config, config.apps));
      expect(base.title).toBe(config.title);
    });

    it('stays open and shows the error when the save fails', async () => {
      const onsave = vi.fn().mockRejectedValueOnce(new Error('bad')).mockResolvedValue(undefined);
      const onclose = vi.fn();
      renderFull(serverConfig(), { initialTab: 'security', onsave, onclose });

      await fireEvent.click(screen.getByTestId('trigger-title-edit'));
      await fireEvent.click(screen.getByText('Save Changes'));

      expect(await screen.findByText('Save failed: bad')).toBeInTheDocument();
      expect(onclose).not.toHaveBeenCalled();
      expect(screen.getByText('Unsaved changes')).toBeInTheDocument();
      expect(screen.getByText('Save Changes')).not.toBeDisabled();

      // The edit survived: a retry sends it and then closes.
      await fireEvent.click(screen.getByText('Save Changes'));
      await waitFor(() => expect(onclose).toHaveBeenCalledTimes(1));
      expect((onsave.mock.calls[1][0] as Config).title).toBe('Edited title');
      expect(screen.queryByText('Save failed: bad')).not.toBeInTheDocument();
    });

    it('falls back to a generic message when the save rejects with a non-error', async () => {
      const onsave = vi.fn().mockRejectedValue('nope');
      renderFull(serverConfig(), { initialTab: 'security', onsave });

      await fireEvent.click(screen.getByTestId('trigger-title-edit'));
      await fireEvent.click(screen.getByText('Save Changes'));

      expect(await screen.findByText('Save failed: Failed to save configuration')).toBeInTheDocument();
    });

    it('shows Saving and blocks Save while the save is in flight', async () => {
      let resolve!: () => void;
      const onsave = vi.fn(() => new Promise<void>(r => { resolve = r; }));
      const onclose = vi.fn();
      renderFull(serverConfig(), { initialTab: 'security', onsave, onclose });

      await fireEvent.click(screen.getByTestId('trigger-title-edit'));
      await fireEvent.click(screen.getByText('Save Changes'));

      expect(screen.getByText('Saving...')).toBeDisabled();
      resolve();
      await waitFor(() => expect(onclose).toHaveBeenCalledTimes(1));
    });

    it('rebases on a new config prop and keeps unsaved edits', async () => {
      const onsave = vi.fn().mockResolvedValue(undefined);
      const { rerender } = renderFull(serverConfig(), { initialTab: 'security', onsave });

      await fireEvent.click(screen.getByTestId('trigger-title-edit'));
      const theirs = serverConfig([...sampleApps, radarr]);
      await rerender({ config: theirs, apps: theirs.apps });

      await fireEvent.click(screen.getByText('Apps & Groups'));
      expect(names()).toEqual(['Grafana', 'Sonarr', 'Radarr']);
      expect(screen.getByText('Unsaved changes')).toBeInTheDocument();
      expect(mockToasts.info).toHaveBeenCalledWith('Settings were updated by the server; your unsaved edits were kept.');

      await fireEvent.click(screen.getByText('Save Changes'));
      await waitFor(() => expect(onsave).toHaveBeenCalledTimes(1));
      const [saved, base] = onsave.mock.calls[0] as [Config, Config];
      expect(saved.title).toBe('Edited title');
      expect(saved.apps.map(a => a.name)).toEqual(['Grafana', 'Sonarr', 'Radarr']);
      expect(base.apps.map(a => a.name)).toEqual(['Grafana', 'Sonarr', 'Radarr']);
      expect(base.title).toBe('Muximux');
    });

    it('a rebase with no server change leaves the dialog clean', async () => {
      const config = serverConfig(sampleApps, { groups: [makeGroup({ name: 'Media' })] });
      const { rerender } = renderFull(config, { initialTab: 'apps' });

      const copy = JSON.parse(JSON.stringify(config)) as Config;
      await rerender({ config: copy, apps: copy.apps });

      expect(names()).toEqual(['Grafana', 'Sonarr']);
      expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument();
      expect(screen.getByText('Save Changes')).toBeDisabled();
      expect(mockToasts.info).not.toHaveBeenCalled();
    });

    it('a server change with no local edits stays clean and silent', async () => {
      const { rerender } = renderFull(serverConfig(), { initialTab: 'apps' });

      const theirs = serverConfig([...sampleApps, radarr], { title: 'Renamed' });
      await rerender({ config: theirs, apps: theirs.apps });

      expect(names()).toEqual(['Grafana', 'Sonarr', 'Radarr']);
      expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument();
      expect(mockToasts.info).not.toHaveBeenCalled();
    });

    it('import refreshes without a phantom dirty state', async () => {
      const groups = [makeGroup({ name: 'Media', order: 0 }), makeGroup({ name: 'Tools', order: 1 })];
      const apps = [
        makeApp({ name: 'Plex', group: 'Media', order: 0 }),
        makeApp({ name: 'Loose', group: '', order: 0 }),
      ];
      const config = serverConfig(apps, { groups });
      const fresh = serverConfig([
        ...apps,
        makeApp({ name: 'Jellyfin', group: 'Media', order: 1, docker_key: 'name:jellyfin' }),
      ], { groups });
      mockFetchConfig.mockResolvedValue(fresh);
      const onclose = vi.fn();
      const { container } = renderFull(config, { initialTab: 'apps', onclose });

      (appsTabProps.ondiscoveryscan as () => void)();
      await waitFor(() => expect(discoverModalProps.open).toBe(true));
      await (discoverModalProps.onimported as () => Promise<void>)();

      await waitFor(() => expect(names('Media')).toEqual(['Plex', 'Jellyfin']));
      expect(names('')).toEqual(['Loose']);
      expect(names('Tools')).toEqual([]);
      expect(discoverModalProps.open).toBe(false);
      expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument();
      expect(screen.getByText('Save Changes')).toBeDisabled();

      await fireEvent.click(container.querySelector('[aria-label="Close settings"]')!);
      expect(screen.queryByText('You have unsaved changes. Discard?')).not.toBeInTheDocument();
      expect(onclose).toHaveBeenCalledTimes(1);
    });

    it('import keeps an unrelated unsaved edit', async () => {
      const fresh = serverConfig([...sampleApps, radarr]);
      mockFetchConfig.mockResolvedValue(fresh);
      const onsave = vi.fn().mockResolvedValue(undefined);
      renderFull(serverConfig(), { initialTab: 'security', onsave });

      await fireEvent.click(screen.getByTestId('trigger-title-edit'));
      await (discoverModalProps.onimported as () => Promise<void>)();

      await waitFor(() => expect(screen.getByText('Unsaved changes')).toBeInTheDocument());
      await fireEvent.click(screen.getByText('Save Changes'));
      await waitFor(() => expect(onsave).toHaveBeenCalledTimes(1));
      const [saved] = onsave.mock.calls[0] as [Config, Config];
      expect(saved.title).toBe('Edited title');
      expect(saved.apps.map(a => a.name)).toContain('Radarr');
    });

    it('closes the Discover modal and reports a failed refresh after an import', async () => {
      mockFetchConfig.mockRejectedValue(new Error('offline'));
      renderFull(serverConfig(), { initialTab: 'apps' });

      (appsTabProps.ondiscoveryscan as () => void)();
      await waitFor(() => expect(discoverModalProps.open).toBe(true));
      await (discoverModalProps.onimported as () => Promise<void>)();

      expect(mockToasts.error).toHaveBeenCalledWith('Failed to load configuration');
      await waitFor(() => expect(discoverModalProps.open).toBe(false));
      expect(names()).toEqual(['Grafana', 'Sonarr']);
    });

    it('queues a rebase while an app is being edited', async () => {
      const { rerender } = renderFull(serverConfig(), { initialTab: 'apps' });

      const held = listed()[0];
      (appsTabProps.onstartEditApp as (a: App) => void)(held);
      await waitFor(() => expect(screen.getByText('Edit Grafana')).toBeInTheDocument());
      held.url = 'https://changed.example';

      const theirs = serverConfig([...sampleApps, radarr]);
      await rerender({ config: theirs, apps: theirs.apps });
      expect(names()).toEqual(['Grafana', 'Sonarr']);
      expect(listed()[0]).toBe(held);

      await fireEvent.click(screen.getByText('Cancel'));

      // Cancel found the modal's object in place and restored it.
      expect(held.url).toBe('https://example.com');
      await waitFor(() => expect(names()).toEqual(['Grafana', 'Sonarr', 'Radarr']));
      expect(listed()[0].url).toBe('https://example.com');
      expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument();
    });

    it('queues a rebase while a save is in flight and applies it when the save fails', async () => {
      let reject!: (e: Error) => void;
      const onsave = vi.fn(() => new Promise<void>((_, r) => { reject = r; }));
      const { rerender } = renderFull(serverConfig(), { initialTab: 'apps', onsave });
      mockSelectedFamily.set('nord');
      await waitFor(() => expect(screen.getByText('Save Changes')).not.toBeDisabled());

      await fireEvent.click(screen.getByText('Save Changes'));
      const theirs = serverConfig([...sampleApps, radarr]);
      await rerender({ config: theirs, apps: theirs.apps });
      expect(names()).toEqual(['Grafana', 'Sonarr']);

      reject(new Error('conflict'));
      expect(await screen.findByText('Save failed: conflict')).toBeInTheDocument();
      await waitFor(() => expect(names()).toEqual(['Grafana', 'Sonarr', 'Radarr']));
    });

    it('ignores server changes once a save succeeded', async () => {
      const onsave = vi.fn().mockResolvedValue(undefined);
      const onclose = vi.fn();
      const { rerender } = renderFull(serverConfig(), { initialTab: 'apps', onsave, onclose });
      mockSelectedFamily.set('nord');
      await waitFor(() => expect(screen.getByText('Save Changes')).not.toBeDisabled());

      await fireEvent.click(screen.getByText('Save Changes'));
      await waitFor(() => expect(onclose).toHaveBeenCalledTimes(1));
      const theirs = serverConfig([...sampleApps, radarr]);
      await rerender({ config: theirs, apps: theirs.apps });

      expect(names()).toEqual(['Grafana', 'Sonarr']);
    });

    it('passes configRevision to GatewayTab', async () => {
      const { rerender } = renderFull(serverConfig(), { initialTab: 'gateway' });
      expect(gatewayTabProps.configRevision).toBe(0);

      const theirs = serverConfig([...sampleApps, radarr]);
      await rerender({ config: theirs, apps: theirs.apps });

      expect(gatewayTabProps.configRevision).toBe(1);
    });

    it('blocks the save on a name conflict until one item is renamed', async () => {
      const { rerender } = renderFull(serverConfig(), { initialTab: 'apps' });

      // The user renames Grafana to Radarr while the server adds its own Radarr.
      listed()[0].name = 'Radarr';
      const theirs = serverConfig([...sampleApps, radarr]);
      await rerender({ config: theirs, apps: theirs.apps });

      const banner = await screen.findByTestId('settings-conflicts');
      expect(banner).toHaveTextContent('Two apps are named "Radarr". Rename one of them before saving.');
      expect(screen.getByText('Save Changes')).toBeDisabled();
      expect(names()).toEqual(['Radarr', 'Sonarr', 'Radarr']);
      const ids = listed().map(a => (a as App & { id: string }).id);
      expect(new Set(ids).size).toBe(ids.length);

      listed()[2].name = 'Radarr 4K';
      await waitFor(() => expect(screen.queryByTestId('settings-conflicts')).not.toBeInTheDocument());
      expect(screen.getByText('Save Changes')).not.toBeDisabled();
    });

    it('lists one message per conflicting name', async () => {
      const { rerender } = renderFull(serverConfig(), { initialTab: 'apps' });

      listed()[0].name = 'Radarr';
      listed()[1].name = 'Radarr';
      const theirs = serverConfig([...sampleApps, radarr]);
      await rerender({ config: theirs, apps: theirs.apps });

      const banner = await screen.findByTestId('settings-conflicts');
      expect(banner.querySelectorAll('p')).toHaveLength(1);
      const ids = listed().map(a => (a as App & { id: string }).id);
      expect(ids).toHaveLength(3);
      expect(new Set(ids).size).toBe(3);
    });

    it('reports a group name conflict', async () => {
      const config = serverConfig(sampleApps, { groups: [makeGroup({ name: 'Media' })] });
      const { rerender } = renderFull(config, { initialTab: 'apps' });

      (appsTabProps.dndGroups as Group[])[0].name = 'Tools';
      const theirs = serverConfig(sampleApps, { groups: [makeGroup({ name: 'Media' }), makeGroup({ name: 'Tools', order: 1 })] });
      await rerender({ config: theirs, apps: theirs.apps });

      expect(await screen.findByTestId('settings-conflicts')).toHaveTextContent('Two groups are named "Tools". Rename one of them before saving.');
      expect(screen.getByText('Save Changes')).toBeDisabled();
    });
  });
  // =======================================================================
  // Close paths, keybinding discard, in-flight save (S-17, S-31)
  // =======================================================================
  describe('Close paths and discard', () => {
    function idsIn(list: unknown[]): unknown[] {
      return list.filter(i => Object.prototype.hasOwnProperty.call(i, 'id'));
    }

    it('keybinding edits make the dialog dirty and discard reverts them', async () => {
      const keybindings = { bindings: { search: [{ key: 'j' }] } };
      // The shell initialised the store from the server config before opening.
      mockCustomBindings.set({ search: [{ key: 'j' }] });
      const onclose = vi.fn();
      const { component } = render(Settings, {
        props: { config: makeConfig({ keybindings }), apps: sampleApps, initialTab: 'keybindings', onclose },
      });
      expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument();

      mockCustomBindings.set({ search: [{ key: 'x', ctrl: true }] });
      await waitFor(() => expect(screen.getByText('Unsaved changes')).toBeInTheDocument());

      // Undoing the edit by hand makes the dialog clean again (not sticky).
      mockCustomBindings.set({ search: [{ key: 'j' }] });
      await waitFor(() => expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument());

      mockCustomBindings.set({ search: [{ key: 'x', ctrl: true }] });
      await waitFor(() => expect(screen.getByText('Unsaved changes')).toBeInTheDocument());

      expect(component.requestClose()).toBe(false);
      await fireEvent.click(await screen.findByText('Discard'));

      expect(mockInitKeybindings).toHaveBeenCalledWith(keybindings);
      expect(mockCustomBindings.get()).toEqual(keybindings.bindings);
      expect(onclose).toHaveBeenCalledTimes(1);
    });

    it('an emptied binding list reads as no binding', async () => {
      renderSettings({ initialTab: 'keybindings' });
      mockCustomBindings.set({ search: [] });
      await Promise.resolve();
      expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument();
      expect(screen.getByText('Save Changes')).toBeDisabled();
    });

    it('requestClose returns false and shows the prompt when dirty; true and reverts theme when clean', async () => {
      const onclose = vi.fn();
      const dirty = renderSettings({ onclose });
      mockSelectedFamily.set('nord');
      await waitFor(() => expect(screen.getByText('Unsaved changes')).toBeInTheDocument());

      expect(dirty.component.requestClose()).toBe(false);
      expect(await screen.findByText('You have unsaved changes. Discard?')).toBeInTheDocument();
      expect(onclose).not.toHaveBeenCalled();
      dirty.unmount();

      // A clean dialog closes at once and still puts the theme back: the
      // preview store is reset to the family the dialog opened with.
      mockSelectedFamily.set('default');
      mockSetThemeFamily.mockClear();
      const clean = renderSettings({ onclose });
      expect(clean.component.requestClose()).toBe(true);
      expect(onclose).toHaveBeenCalledTimes(1);
      expect(mockSetThemeFamily).toHaveBeenCalledWith('default');
      expect(mockSetVariantMode).toHaveBeenCalledWith('dark');
      expect(mockInitKeybindings).toHaveBeenCalledWith(undefined);
    });

    it('tells the Security tab whether there are unsaved changes', async () => {
      securityTabProps = {};
      renderSettings({ initialTab: 'security' });
      expect(securityTabProps.hasUnsavedChanges).toBe(false);

      await fireEvent.click(screen.getByTestId('trigger-title-edit'));
      await waitFor(() => expect(securityTabProps.hasUnsavedChanges).toBe(true));
    });

    it('keeps the dialog open on every close path while a save is in flight', async () => {
      let reject!: (e: Error) => void;
      const onsave = vi.fn(() => new Promise<void>((_, r) => { reject = r; }));
      const onclose = vi.fn();
      const { component, container } = render(Settings, {
        props: { config: makeConfig(), apps: sampleApps, initialTab: 'security', onsave, onclose },
      });

      await fireEvent.click(screen.getByTestId('trigger-title-edit'));
      // Open the discard prompt first, then save from it.
      expect(component.requestClose()).toBe(false);
      await screen.findByText('Discard');
      await fireEvent.click(screen.getByText('Save Changes'));
      expect(screen.getByText('Saving...')).toBeDisabled();

      // Escape: consumed, so the shell does not close Settings.
      expect(component.handleEscape()).toBe(true);
      // Gear, keybinding, navigateHome, hashchange: all go through requestClose.
      expect(component.requestClose()).toBe(false);
      // X button and Discard are disabled and do nothing.
      const closeBtn = container.querySelector('[aria-label="Close settings"]') as HTMLButtonElement;
      expect(closeBtn).toBeDisabled();
      await fireEvent.click(closeBtn);
      const discard = screen.getByText('Discard') as HTMLButtonElement;
      expect(discard).toBeDisabled();
      await fireEvent.click(discard);
      expect(onclose).not.toHaveBeenCalled();
      expect(mockSetThemeFamily).not.toHaveBeenCalled();

      // The save fails: the dialog is still open with the edit.
      reject(new Error('bad'));
      expect(await screen.findByText('Save failed: bad')).toBeInTheDocument();
      expect(screen.getByText('Unsaved changes')).toBeInTheDocument();
      expect(onclose).not.toHaveBeenCalled();
      expect(component.handleEscape()).toBe(false);
      expect(discard).not.toBeDisabled();
    });

    it('sends apps and groups without the client-only id', async () => {
      const onsave = vi.fn().mockResolvedValue(undefined);
      appsTabProps = {};
      renderSettings({
        onsave,
        initialTab: 'apps',
        config: { groups: [makeGroup({ name: 'Media' })] },
        apps: [makeApp({ name: 'Plex', group: 'Media' }), makeApp({ name: 'Loose', order: 1 })],
      });
      // The dialog stamps ids for drag and drop.
      expect(idsIn(appsTabProps.dndGroups as Group[])).toHaveLength(1);
      expect(idsIn(Object.values(appsTabProps.dndGroupedApps as Record<string, App[]>).flat())).toHaveLength(2);

      mockSelectedFamily.set('nord');
      await waitFor(() => expect(screen.getByText('Save Changes')).not.toBeDisabled());
      await fireEvent.click(screen.getByText('Save Changes'));
      await waitFor(() => expect(onsave).toHaveBeenCalledTimes(1));

      const [saved] = onsave.mock.calls[0] as [Config, Config];
      expect(saved.apps.map(a => a.name).sort()).toEqual(['Loose', 'Plex']);
      expect(saved.groups.map(g => g.name)).toEqual(['Media']);
      expect(idsIn(saved.apps)).toEqual([]);
      expect(idsIn(saved.groups)).toEqual([]);
      expect(JSON.stringify(saved)).not.toContain('"id"');
    });

    describe('sparse server config', () => {
      // Shaped like GET /api/config: omitempty fields absent, icons partial.
      function sparseConfig(plexUrl = 'http://plex:32400'): Config {
        const apps = [
          {
            name: 'Plex', url: plexUrl,
            icon: { type: 'dashboard', name: 'plex', color: '', background: '' },
            color: '#E5A00D', group: 'Media', order: 0, enabled: true, default: false,
            open_mode: 'iframe', proxy: false, scale: 1,
          },
          {
            name: 'Sonarr', url: 'http://sonarr:8989', health_check: true,
            icon: { type: 'lucide', name: 'tv', color: '', background: '' },
            color: '#35C5F4', group: '', order: 0, enabled: true, default: true,
            open_mode: 'new_tab', proxy: true, scale: 1, min_role: 'admin',
          },
        ] as unknown as App[];
        return {
          title: 'Home',
          navigation: makeNav(),
          groups: [{ name: 'Media', icon: { type: 'dashboard', color: '', background: '' }, color: '#3498db', order: 0, expanded: true }] as unknown as Group[],
          apps,
          auth: { method: 'none' },
        } as Config;
      }

      beforeEach(() => {
        appsTabProps = {};
        discoverModalProps = {};
      });

      function renderSparse(config: Config, onclose = vi.fn()) {
        return render(Settings, { props: { config, apps: config.apps, initialTab: 'apps', onclose } });
      }
      function media(): App[] {
        return (appsTabProps.dndGroupedApps as Record<string, App[]>).Media ?? [];
      }

      it('starts clean', () => {
        renderSparse(sparseConfig());
        expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument();
        expect(screen.getByText('Save Changes')).toBeDisabled();
      });

      it('a push that changes the URL of an untouched app shows the new URL and stays clean', async () => {
        const { rerender } = renderSparse(sparseConfig());

        const theirs = sparseConfig('http://plex.lan:32400');
        await rerender({ config: theirs, apps: theirs.apps });

        await waitFor(() => expect(media()[0].url).toBe('http://plex.lan:32400'));
        expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument();
        expect(screen.getByText('Save Changes')).toBeDisabled();
        expect(mockToasts.info).not.toHaveBeenCalled();
      });

      it('an import with no edits stays clean', async () => {
        const fresh = sparseConfig();
        fresh.apps = [...fresh.apps, {
          name: 'Jellyfin', url: 'http://jellyfin:8096',
          icon: { type: 'dashboard', name: 'jellyfin', color: '', background: '' },
          color: '#00A4DC', group: 'Media', order: 1, enabled: true, default: false,
          open_mode: 'iframe', proxy: false, scale: 1, docker_key: 'name:jellyfin',
        } as unknown as App];
        mockFetchConfig.mockResolvedValue(fresh);
        const onclose = vi.fn();
        const { container } = renderSparse(sparseConfig(), onclose);

        (appsTabProps.ondiscoveryscan as () => void)();
        await waitFor(() => expect(discoverModalProps.open).toBe(true));
        await (discoverModalProps.onimported as () => Promise<void>)();

        await waitFor(() => expect(media().map(a => a.name)).toEqual(['Plex', 'Jellyfin']));
        expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument();
        expect(screen.getByText('Save Changes')).toBeDisabled();
        await fireEvent.click(container.querySelector('[aria-label="Close settings"]')!);
        expect(screen.queryByText('You have unsaved changes. Discard?')).not.toBeInTheDocument();
        expect(onclose).toHaveBeenCalledTimes(1);
      });

      // I-1 (S-05): the base goes through the same factories as the
      // payload, so the server never reads a filled-in default (an absent
      // scale becomes 1) as an edit and saves a removed app back.
      it('sends a base normalised like the payload for an untouched sparse app', async () => {
        const config = sparseConfig();
        config.apps = [...config.apps, {
          name: 'Whoami', url: 'http://10.0.0.5:8080',
          icon: { type: 'dashboard', name: 'whoami', color: '', background: '' },
          color: '#22c55e', group: '', order: 1, enabled: true, default: false,
          open_mode: 'iframe', proxy: false, docker_key: 'name:whoami',
        } as unknown as App];
        const onsave = vi.fn().mockResolvedValue(undefined);
        render(Settings, { props: { config, apps: config.apps, initialTab: 'security', onsave } });
        expect(screen.getByText('Save Changes')).toBeDisabled();

        await fireEvent.click(screen.getByTestId('trigger-title-edit'));
        await fireEvent.click(screen.getByText('Save Changes'));
        await waitFor(() => expect(onsave).toHaveBeenCalledTimes(1));

        const [saved, base] = onsave.mock.calls[0] as [Config, Config];
        const strip = (a: App) => { const { original_name: _o, ...rest } = a; return rest; };
        for (const name of ['Plex', 'Sonarr', 'Whoami']) {
          const sent = saved.apps.find(a => a.name === name)!;
          const from = base.apps.find(a => a.name === name)!;
          expect(strip(sent)).toEqual(from);
        }
        expect(base.apps.find(a => a.name === 'Whoami')!.scale).toBe(1);
        expect(saved.groups.map(g => { const { original_name: _o, ...rest } = g; return rest; })).toEqual(base.groups);
      });

      it('a push that removes an untouched sparse app drops it from the payload and the base', async () => {
        const withWhoami = (): Config => {
          const c = sparseConfig();
          c.apps = [...c.apps, {
            name: 'Whoami', url: 'http://10.0.0.5:8080',
            icon: { type: 'dashboard', name: 'whoami', color: '', background: '' },
            color: '#22c55e', group: '', order: 1, enabled: true, default: false,
            open_mode: 'iframe', proxy: false, docker_key: 'name:whoami',
          } as unknown as App];
          return c;
        };
        const onsave = vi.fn().mockResolvedValue(undefined);
        const config = withWhoami();
        const { rerender } = render(Settings, { props: { config, apps: config.apps, initialTab: 'security', onsave } });
        await fireEvent.click(screen.getByTestId('trigger-title-edit'));

        const theirs = sparseConfig();
        await rerender({ config: theirs, apps: theirs.apps });
        await fireEvent.click(screen.getByText('Save Changes'));
        await waitFor(() => expect(onsave).toHaveBeenCalledTimes(1));

        const [saved, base] = onsave.mock.calls[0] as [Config, Config];
        expect(saved.title).toBe('Edited title');
        expect(saved.apps.map(a => a.name).sort()).toEqual(['Plex', 'Sonarr']);
        expect(base.apps.map(a => a.name).sort()).toEqual(['Plex', 'Sonarr']);
        expect(base.apps.every(a => a.scale === 1 && a.force_icon_background === false)).toBe(true);
      });
    });

    describe('theme pushed while open (I-2)', () => {
      beforeEach(() => {
        mockSetThemeFamily.mockImplementation((f: string) => mockSelectedFamily.set(f));
        mockSetVariantMode.mockImplementation((v: 'dark' | 'light' | 'system') => mockVariantMode.set(v));
        mockSelectedFamily.set('nord');
        mockVariantMode.set('dark');
      });

      it('an untouched theme follows the server and is not written back stale', async () => {
        const config = makeConfig({ apps: sampleApps, theme: { family: 'nord', variant: 'dark' } });
        const onsave = vi.fn().mockResolvedValue(undefined);
        const { rerender } = render(Settings, { props: { config, apps: config.apps, initialTab: 'security', onsave } });
        // Another admin saved dracula/light; the shell refetched and passed
        // it down without touching the theme stores (Settings is open).
        const theirs = makeConfig({ apps: sampleApps, theme: { family: 'dracula', variant: 'light' } });
        await rerender({ config: theirs, apps: theirs.apps });
        expect(mockSelectedFamily.get()).toBe('dracula');
        expect(mockVariantMode.get()).toBe('light');
        // Following the server is not an unsaved edit.
        expect(screen.getByText('Save Changes')).toBeDisabled();

        await fireEvent.click(screen.getByTestId('trigger-title-edit'));
        await fireEvent.click(screen.getByText('Save Changes'));
        await waitFor(() => expect(onsave).toHaveBeenCalledTimes(1));
        const [saved, base] = onsave.mock.calls[0] as [Config, Config];
        expect(base.theme).toEqual({ family: 'dracula', variant: 'light' });
        expect(saved.theme).toEqual({ family: 'dracula', variant: 'light' });
      });

      it('an edited theme stays, and discard returns to the server theme', async () => {
        const config = makeConfig({ apps: sampleApps, theme: { family: 'nord', variant: 'dark' } });
        const onclose = vi.fn();
        const { rerender, component } = render(Settings, { props: { config, apps: config.apps, initialTab: 'security', onclose } });
        mockSelectedFamily.set('solarized');
        const theirs = makeConfig({ apps: sampleApps, theme: { family: 'dracula', variant: 'bogus' as 'dark' } });
        await rerender({ config: theirs, apps: theirs.apps });
        expect(mockSelectedFamily.get()).toBe('solarized');
        expect(mockVariantMode.get()).toBe('dark');

        expect(component.requestClose()).toBe(false);
        await fireEvent.click(await screen.findByText('Discard'));
        expect(mockSetThemeFamily).toHaveBeenLastCalledWith('dracula');
        expect(mockSetVariantMode).toHaveBeenLastCalledWith('dark');
        expect(onclose).toHaveBeenCalledTimes(1);
      });

      it('a push without a theme leaves the theme alone', async () => {
        const config = makeConfig({ apps: sampleApps, theme: { family: 'nord', variant: 'dark' } });
        const { rerender } = render(Settings, { props: { config, apps: config.apps, initialTab: 'security' } });
        const theirs = makeConfig({ apps: sampleApps, title: 'Other' });
        delete theirs.theme;
        await rerender({ config: theirs, apps: theirs.apps });
        expect(mockSetThemeFamily).not.toHaveBeenCalled();
        expect(mockSelectedFamily.get()).toBe('nord');
      });
    });

    it('a push takes untouched keybindings from the server and keeps edited ones', async () => {
      const { rerender } = render(Settings, { props: { config: makeConfig(), apps: sampleApps } });

      // Untouched: the server's new bindings replace the store, still clean.
      const pushed = makeConfig({ keybindings: { bindings: { search: [{ key: 'k' }] } } });
      await rerender({ config: pushed, apps: sampleApps });
      expect(mockCustomBindings.get()).toEqual({ search: [{ key: 'k' }] });
      expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument();

      // Edited: the edit survives the next push and is compared against it.
      mockCustomBindings.set({ search: [{ key: 'q' }] });
      await waitFor(() => expect(screen.getByText('Unsaved changes')).toBeInTheDocument());
      const again = makeConfig({ title: 'Other', keybindings: { bindings: { search: [{ key: 'k' }] } } });
      await rerender({ config: again, apps: sampleApps });
      expect(mockCustomBindings.get()).toEqual({ search: [{ key: 'q' }] });
      expect(screen.getByText('Unsaved changes')).toBeInTheDocument();

      // Setting it to the server's value leaves nothing to save.
      mockCustomBindings.set({ search: [{ key: 'k' }] });
      await waitFor(() => expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument());
    });
  });
});
