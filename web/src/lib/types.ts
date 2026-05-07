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
  allowed_groups?: string[];  // when set, user must be in at least one of these groups (case-insensitive); admins bypass
  force_icon_background?: boolean;  // show icon background even when global setting is off
  permissions?: string[];  // browser feature policy: camera, microphone, geolocation, fullscreen, etc.
  allow_notifications?: boolean;  // enable postMessage notification bridge
  gateway_domain?: string;  // populated by the backend when a gateway site references this app via app_name (Phase 4 pairing)
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
  show_home_button: boolean;
  home_icon?: AppIcon;
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

export interface GatewaySite {
  domain: string;
  backend_url: string;
  tls?: '' | 'auto' | 'custom' | 'none';
  tls_cert?: string;
  tls_key?: string;
  strip_frame_blockers?: boolean;
  streaming?: boolean;
  proxy_headers?: Record<string, string>;
  forwarded_headers?: boolean;
  app_name?: string;
}

// GatewayMutationResponse covers both the success and failure shapes
// the server returns for create / update / delete. The server emits
// the success shape on 2xx responses and the failure shape on the
// 503 divergence path; other 4xx/5xx errors flow through the generic
// error response (a `{error: "..."}` JSON body).
//
// `mismatch: true` is the divergence signal (Caddy is serving a
// candidate config but disk has the prior one) — the UI shows a
// sticky banner asking the operator to restart Muximux.
export interface GatewayMutationResponse {
  success: boolean;
  site?: GatewaySite;
  restart_required?: boolean;
  mismatch?: boolean;
  error?: string;
}

export interface GatewayValidationResponse {
  valid: boolean;
  error?: string;
}

export interface UserInfo {
  username: string;
  role: string;
  email?: string;
  display_name?: string;
  groups?: string[];  // group memberships used by per-app allowed_groups filtering
}

export interface CreateUserRequest {
  username: string;
  password: string;
  role: string;
  email?: string;
  display_name?: string;
  groups?: string[];
}

export interface UpdateUserRequest {
  role?: string;
  email?: string;
  display_name?: string;
  groups?: string[];  // omit to leave existing groups untouched; pass [] to clear
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

// DiscoveryDockerStatus mirrors discovery.StatusResult on the backend.
// The four-state UI gating ladder (CTA / disabled-unreachable /
// disabled-strategy / active) reads Configured + Reachable +
// StrategyOK in combination.
export interface DiscoveryDockerStatus {
  configured: boolean;
  reachable: boolean;
  strategy_ok: boolean;
  endpoint?: string;
  api_version?: string;
  strategy?: string;
  self_detect_method?: string;
  last_error?: string;
  refresh_divergences?: number;
  last_divergence_at?: string;
  recovered_at?: string;
  last_refresh_at?: string;
  tls_warning?: string;
}

// DiscoveryDockerConfig mirrors config.DiscoveryDockerConfig. Sent to
// PUT /api/discovery/docker/config and POST /api/discovery/docker/test.
export interface DiscoveryDockerConfig {
  enabled: boolean;
  endpoint: string;
  tls: DiscoveryTLSConfig;
  network_strategy: 'container_ip' | 'container_dns' | 'host_port' | 'host_docker_internal' | '';
  host_ip?: string;
  network_filter?: string;
  refresh_interval: string;
}

export interface DiscoveryTLSConfig {
  enabled: boolean;
  ca_cert?: string;
  client_cert?: string;
  client_key?: string;
}

// DiscoverySuggestion mirrors discovery.Suggestion on the backend.
// Returned by /api/discovery/docker/scan; consumed by the Discover
// modal which lets the operator edit fields and pick which to import.
export interface DiscoverySuggestion {
  key: string;
  stability: 'stable' | 'recreate-fragile' | 'task-fragile';
  name: string;
  icon?: string;
  group?: string;
  url: string;
  health_url?: string;
  effective_strategy: string;
  container_id: string;
  container_name?: string;
  image_ref: string;
  confidence: 'high' | 'medium' | 'low';
  requires_input?: boolean;
  notes?: string[];
  suggested_domain?: string;
}

export interface DiscoveryScanResult {
  suggestions?: DiscoverySuggestion[];
  scan_blocked?: string;
  error?: string;
}
