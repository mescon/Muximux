export interface App {
  name: string;
  url: string;
  proxyUrl?: string;  // Proxy path for iframe loading (when proxy enabled)
  health_url?: string;
  icon: AppIcon;
  color: string;
  group: string;
  order: number;
  enabled: boolean;
  default: boolean;
  open_mode: 'iframe' | 'new_tab' | 'new_window' | 'redirect';
  proxy: boolean;
  health_check?: boolean;  // true = enabled, undefined/false = disabled (opt-in)
  proxy_skip_tls_verify?: boolean;
  proxy_headers?: Record<string, string>;
  scale: number;
  shortcut?: number;  // 1-9 keyboard shortcut slot
  min_role?: string;  // minimum role to see this app
  force_icon_background?: boolean;  // show icon background even when global setting is off
}

export function getEffectiveUrl(app: App): string {
  if (app.proxyUrl) {
    const base = (globalThis as unknown as Record<string, string>).__MUXIMUX_BASE__ || '';
    return base + app.proxyUrl;
  }
  return app.url;
}

export interface AppIcon {
  type: 'dashboard' | 'lucide' | 'custom' | 'url';
  name?: string;
  file?: string;
  url?: string;
  variant?: string;
  color?: string;
  background?: string;
  invert?: boolean;
}

export interface Group {
  name: string;
  icon: AppIcon;
  color: string;
  order: number;
  expanded: boolean;
}

// Factory functions for consistent object construction. Use these instead of
// inline object literals to avoid field omissions and inconsistent defaults.
export function makeApp(overrides: Partial<App> = {}): App {
  const { icon: iconOverrides, ...rest } = overrides;
  return {
    name: '',
    url: '',
    icon: { type: 'dashboard', name: '', file: '', url: '', variant: '', color: '', background: '', invert: false, ...iconOverrides },
    color: '#22c55e',
    group: '',
    order: 0,
    enabled: true,
    default: false,
    open_mode: 'iframe',
    proxy: false,
    scale: 1,
    force_icon_background: false,
    min_role: '',
    ...rest,
  };
}

export function makeGroup(overrides: Partial<Group> = {}): Group {
  const { icon: iconOverrides, ...rest } = overrides;
  return {
    name: '',
    icon: { type: 'dashboard', name: '', file: '', url: '', variant: '', color: '', background: '', invert: false, ...iconOverrides },
    color: '#3498db',
    order: 0,
    expanded: true,
    ...rest,
  };
}

// svelte-dnd-action requires a stable `id` on every item. App/Group types don't
// include it (it's not persisted), so we stamp it via a cast. Use these helpers
// instead of hand-rolling the cast everywhere.
export function stampAppId(app: App) { (app as App & Record<string, unknown>).id = app.name; }
export function stampGroupId(group: Group) { (group as Group & Record<string, unknown>).id = group.name; }

export interface NavigationConfig {
  position: 'top' | 'left' | 'right' | 'bottom' | 'floating';
  width: string;
  auto_hide: boolean;
  auto_hide_delay: string;
  show_on_hover: boolean;
  show_labels: boolean;
  show_logo: boolean;
  show_app_colors: boolean;
  show_icon_background: boolean;
  icon_scale: number;
  show_splash_on_startup: boolean;
  show_shadow: boolean;
  floating_position: 'bottom-right' | 'bottom-left' | 'top-right' | 'top-left';
  bar_style: 'grouped' | 'flat';
  hide_sidebar_footer: boolean;
  max_open_tabs: number;
}

export interface HealthConfig {
  enabled: boolean;
  interval: string;
  timeout: string;
}

export interface AuthConfig {
  method: 'none' | 'builtin' | 'forward_auth' | 'oidc';
  trusted_proxies?: string[];
  headers?: Record<string, string>;
  logout_url?: string;
}

export interface TLSConfig {
  domain?: string;
  email?: string;
  cert?: string;
  key?: string;
}

export interface KeyCombo {
  key: string;
  ctrl?: boolean;
  alt?: boolean;
  shift?: boolean;
  meta?: boolean;
}

export interface KeybindingsConfig {
  bindings?: Record<string, KeyCombo[]>;
}

export interface ThemeConfig {
  family: string;
  variant: 'dark' | 'light' | 'system';
}

export interface LogEntry {
  timestamp: string;
  level: 'debug' | 'info' | 'warn' | 'error';
  message: string;
  source: string;
  attrs?: Record<string, string>;
}

export interface Config {
  title: string;
  language?: string;
  log_level?: string;
  proxy_timeout?: string;
  navigation: NavigationConfig;
  theme?: ThemeConfig;
  health?: HealthConfig;
  auth?: AuthConfig;
  tls?: TLSConfig;
  gateway?: string;
  keybindings?: KeybindingsConfig;
  groups: Group[];
  apps: App[];
}

export interface SetupRequest {
  method: 'builtin' | 'forward_auth' | 'none';
  username?: string;
  password?: string;
  trusted_proxies?: string[];
  headers?: Record<string, string>;
  logout_url?: string;
}

export interface SetupResponse {
  success: boolean;
  method: string;
  error?: string;
}

export interface UserInfo {
  username: string;
  role: string;
  email?: string;
  display_name?: string;
}

export interface CreateUserRequest {
  username: string;
  password: string;
  role: string;
  email?: string;
  display_name?: string;
}

export interface UpdateUserRequest {
  role?: string;
  email?: string;
  display_name?: string;
}

export interface ChangeAuthMethodRequest {
  method: 'builtin' | 'forward_auth' | 'none';
  trusted_proxies?: string[];
  headers?: Record<string, string>;
  logout_url?: string;
}

export interface SystemInfo {
  version: string;
  commit: string;
  build_date: string;
  go_version: string;
  os: string;
  arch: string;
  environment: 'docker' | 'native';
  uptime: string;
  uptime_seconds: number;
  started_at: string;
  data_dir: string;
  links: SystemLinks;
}

export interface SystemLinks {
  github: string;
  issues: string;
  releases: string;
  wiki: string;
}

export interface UpdateInfo {
  current_version: string;
  latest_version: string;
  update_available: boolean;
  release_url: string;
  changelog: string;
  published_at: string;
  download_urls: Record<string, string>;
}
