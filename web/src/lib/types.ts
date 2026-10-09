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
  pinned?: boolean;  // shown directly in the top/bottom bar next to the group dropdowns; stays in its group
  open_mode: 'iframe' | 'new_tab' | 'new_window' | 'redirect' | 'http_action';
  proxy: boolean;
  health_check?: boolean;  // true = enabled, undefined/false = disabled (opt-in)
  docker_managed_health_check?: boolean;  // health_check is driven by Docker labels
  proxy_skip_tls_verify?: boolean;
  proxy_headers?: Record<string, string>;
  // Emit X-Forwarded-* / X-Real-IP to the backend. Defaults to true when
  // unset; set false for backends fronted by a reverse proxy that rejects
  // those headers from an untrusted source (Traefik answers 400).
  forwarded_headers?: boolean;
  http_action_method?: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH';
  http_action_headers?: Record<string, string>;
  http_action_confirm?: boolean;
  http_action_show_toast?: boolean;  // default true; set false for silent fire-and-forget
  scale: number;
  shortcut?: number;  // 1-9 keyboard shortcut slot
  min_role?: string;  // minimum role to see this app
  allowed_groups?: string[];  // when set, user must be in at least one of these groups (case-insensitive); admins bypass
  force_icon_background?: boolean;  // show icon background even when global setting is off
  permissions?: string[];  // browser feature policy: camera, microphone, geolocation, fullscreen, etc.
  allow_notifications?: boolean;  // enable postMessage notification bridge
  gateway_domain?: string;  // populated by the backend when a gateway site references this app via app_name (Phase 4 pairing)
  // Docker auto-management tracking. When docker_key is set, the
  // refresh poller is the source of truth for `url` and the Apps
  // form locks the URL field with a "Detach to edit" prompt.
  docker_key?: string;
  docker_endpoint?: string;
  docker_strategy?: string;
  // Server-owned: the URL the Docker poller last wrote, so a later
  // manual edit can be told apart from a poller refresh.
  docker_managed_url?: string;
  // Transport-only rename identity: the name this app had in the base a
  // save was made against. Never stored.
  original_name?: string;
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
  // Set by the server (admins only) on a group Docker discovery created
  // and still manages from muximux.group.* labels. Read-only: the server
  // ignores it in a payload and clears it when the icon, color or order
  // is edited.
  docker_managed?: boolean;
  // Set by the server (admins only) while a managed group's order comes
  // from a muximux.group.order label; only then does a reorder release it.
  docker_order?: boolean;
  // Transport-only rename identity, as on App.original_name.
  original_name?: string;
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

/**
 * Stamps every item in a list with an id unique within that list: its name,
 * or for a second item of the same name `name` plus a NUL and a counter. Two
 * items share a name only while a Settings rebase conflict waits for the
 * user to rename one; keyed lists and svelte-dnd-action need distinct ids
 * meanwhile. The id is client-only (the server ignores it), so the suffix
 * never reaches the saved config.
 */
export function stampUniqueIds(items: (App | Group)[]): void {
  const seen = new Map<string, number>();
  for (const item of items) {
    const n = seen.get(item.name) ?? 0;
    seen.set(item.name, n + 1);
    (item as (App | Group) & Record<string, unknown>).id = n === 0 ? item.name : `${item.name}\u0000${n}`;
  }
}

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
  dynamic_tab_branding?: boolean;
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
  forward_auth_admin_groups?: string[];
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
  /**
   * Mirrors server.session_cookie_domain. Surfaced read-only to the UI so
   * the Gateway tab can pre-warn when an operator ticks require_auth on a
   * site but no cookie domain is configured — gating that subdomain
   * without a cookie scope would loop the visitor between gate and login.
   */
  session_cookie_domain?: string;
  navigation: NavigationConfig;
  theme?: ThemeConfig;
  health?: HealthConfig;
  auth?: AuthConfig;
  tls?: TLSConfig;
  gateway?: string;
  keybindings?: KeybindingsConfig;
  discovery?: DiscoveryConfig;
  // Field path -> flag or environment variable that overrides it; such
  // fields are read-only in Settings.
  env_overrides?: Record<string, string>;
  groups: Group[];
  apps: App[];
}

// ConfigSaveRequest is the PUT /api/config body. `base` is the config the
// edits were made against; with it the server merges three ways instead of
// replacing what changed on its side meanwhile.
export type ConfigSaveRequest = Config & { base?: Config };

// DiscoveryConfig mirrors config.DiscoveryConfig. Currently only the
// Docker sub-config is surfaced to the UI; the nav and overview read
// docker.health_badge_placement to decide where Docker badges render.
export interface DiscoveryConfig {
  docker?: DiscoveryDockerConfig;
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
  // Skip verification of a self-signed / untrusted HTTPS backend
  // certificate (e.g. Proxmox). Only takes effect when backend_url is
  // https; the public-facing TLS is unaffected.
  backend_skip_tls_verify?: boolean;
  proxy_headers?: Record<string, string>;
  forwarded_headers?: boolean;
  app_name?: string;
  // Docker tracking, mirroring App.docker_key. The poller updates
  // backend_url every refresh tick when set; the UI locks the
  // BackendURL field with a Detach prompt.
  docker_key?: string;
  docker_endpoint?: string;
  docker_strategy?: string;
  // Gateway auth gate. When require_auth is true, requests to
  // this site go through Muximux's session check via Caddy's
  // forward_auth before reaching the backend. min_role +
  // allowed_groups mirror App access rules.
  require_auth?: boolean;
  min_role?: string;
  allowed_groups?: string[];
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
  // Per-user lifecycle gate: true iff lifecycle_enabled + writable
  // socket + user satisfies role floor + group allowlist. Single
  // source of truth -- never recompute on the frontend.
  can_use_docker_lifecycle?: boolean;
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

export interface OIDCSettings {
  enabled: boolean;
  issuer_url: string;
  client_id: string;
  client_secret_set: boolean;
  redirect_url: string;
  scopes: string[] | null;
  username_claim: string;
  email_claim: string;
  groups_claim: string;
  display_name_claim: string;
  admin_groups: string[] | null;
  provider_logout: boolean;
  post_logout_redirect_url: string;
  logout_url: string;
  auto_redirect: boolean;
  disable_local_login: boolean;
  env_fields: Record<string, string>;
  default_callback_url: string;
  backchannel_url: string;
  current_session_is_oidc: boolean;
}

export interface OIDCTestResult {
  reachable: boolean;
  error?: string;
  issuer?: string;
  authorization: boolean;
  token: boolean;
  userinfo: boolean;
  jwks: boolean;
  end_session: boolean;
  backchannel_supported: boolean;
}

export type OIDCSettingsUpdate = Partial<Omit<OIDCSettings,
  'enabled' | 'client_secret_set' | 'env_fields' | 'default_callback_url' | 'backchannel_url' | 'current_session_is_oidc'>> & {
  client_secret?: string;
};

export interface ChangeAuthMethodRequest {
  method: 'builtin' | 'forward_auth' | 'oidc' | 'none';
  trusted_proxies?: string[];
  headers?: Record<string, string>;
  logout_url?: string;
  forward_auth_admin_groups?: string[];
  oidc?: OIDCSettingsUpdate;
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
  // Lifecycle gating surfaced for the Settings panel + the can_use
  // computation. Both fields default to false on a fresh install.
  socket_writable?: boolean;
  lifecycle_enabled?: boolean;
}

// DockerState mirrors discovery.DockerState on the backend. The value
// shape is exactly the JSON returned by /api/discovery/docker-state
// and the `state` field of docker_state_changed WebSocket events.
export interface DockerState {
  status: 'running' | 'exited' | 'paused' | 'restarting' | 'removing' | 'created' | 'dead' | 'missing';
  health: 'healthy' | 'unhealthy' | 'starting' | 'none';
  started_at?: string;
  finished_at?: string;
  exit_code?: number;
  restart_count: number;
  image: string;
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
  lifecycle_enabled?: boolean;
  lifecycle_min_role?: 'admin' | 'power-user' | 'user' | '';
  lifecycle_allowed_groups?: string[];
  health_badge_placement?: 'off' | 'overview' | 'overview_and_nav' | '';
  auto_import?: 'off' | 'add' | 'update' | 'sync';
  require_explicit_enable?: boolean;
}

// DiscoveryDockerConfigResponse is the body of GET
// /api/discovery/docker/config. env_overrides names the environment
// variable behind each field the tab must show as locked.
export interface DiscoveryDockerConfigResponse {
  config: DiscoveryDockerConfig;
  env_overrides?: Record<string, string>;
}

// DiscoveryAutoImportSkip explains why a suggestion is not eligible for
// auto-import. 'disabled' rows are filtered out by the server.
export interface DiscoveryAutoImportSkip {
  code: 'disabled' | 'not_enabled' | 'unlabeled' | 'no_port' | 'no_url' | 'invalid';
  detail?: string;
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
  backend_url?: string;
  fixed_url?: boolean;
  gateway_requested?: boolean;
  health_url?: string;
  effective_strategy: string;
  container_id: string;
  container_name?: string;
  image_ref: string;
  confidence: 'high' | 'medium' | 'low';
  requires_input?: boolean;
  notes?: string[];
  suggested_domain?: string;

  // Label-derived overrides. Carried through to the import endpoint
  // so an operator can fully configure an app from container labels
  // without editing in the modal. All optional - absent means
  // "use catalog or UI default".
  color?: string;
  order?: number;
  open_mode?: 'iframe' | 'new_tab' | 'new_window' | 'redirect' | 'http_action';
  proxy?: boolean;
  proxy_skip_tls_verify?: boolean;
  min_role?: 'user' | 'power-user' | 'admin';
  allowed_groups?: string[];
  permissions?: string[];
  allow_notifications?: boolean;
  default?: boolean;
  shortcut?: number;
  http_action_method?: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH';
  http_action_headers?: Record<string, string>;
  http_action_confirm?: boolean;
  http_action_show_toast?: boolean;
  suggested_gateway?: SuggestedGatewayConfig;
  labeled: boolean;
  label_enabled?: boolean;
  auto_import_skip?: DiscoveryAutoImportSkip;
  health_check?: boolean;
}

// SuggestedGatewayConfig mirrors discovery.SuggestedGatewayConfig.
// Populated when muximux.gateway.* labels are set alongside
// muximux.app.gateway.domain.
export interface SuggestedGatewayConfig {
  tls?: 'auto' | 'none' | 'custom';
  streaming?: boolean;
  strip_frame_blockers?: boolean;
  forwarded_headers?: boolean;
  skip_tls_verify?: boolean;
  require_auth?: boolean;
  min_role?: 'user' | 'power-user' | 'admin';
  allowed_groups?: string[];
}

export interface DiscoveryScanResult {
  suggestions?: DiscoverySuggestion[];
  scan_blocked?: string;
  error?: string;
  opted_out?: number;
}

// Discovery import. Mirrors handlers.ImportRequest / ImportResult.
export interface DiscoveryImportItem {
  key: string;
  strategy: string;
  app?: Partial<App> | null;
  gateway?: GatewaySite | null;
  // Per-row routing decision when `app` is set:
  //   'direct'  -> menu links to the discovered container URL
  //   'proxy'   -> menu links via /proxy/<slug> (sets app.proxy=true)
  //   'gateway' -> menu links to https://<gateway.domain>; app is
  //                NOT auto-managed, the gateway site is instead
  // Optional; '' or omitted = 'direct' for backward compat.
  routing?: 'direct' | 'proxy' | 'gateway';
  skip_if_exists?: boolean;
}

export interface DiscoveryImportRequest {
  items: DiscoveryImportItem[];
}

export interface DiscoveryImportItemResult {
  key: string;
  status: 'created' | 'skipped_exists' | 'validation_failed' | 'name_collision_in_batch' | 'aborted_by_batch_failure';
  error?: string;
  app_name?: string;
  domain?: string;
}

export interface DiscoveryImportResult {
  success: boolean;
  error?: string;
  items: DiscoveryImportItemResult[];
}

// DiscoveryTrackedEntry mirrors handlers.TrackedEntry. Used by the
// Currently-tracked subsection in Settings -> Discovery.
export interface DiscoveryTrackedEntry {
  kind: 'app' | 'gateway';
  name: string;
  key: string;
  strategy: string;
  endpoint: string;
  url: string;
  last_seen_at?: string;
  endpoint_matches: boolean;
  missing_since?: string;
}

export interface DiscoveryQuarantinedEntry {
  kind: 'app' | 'gateway';
  name: string;
  key: string;
  reason: string;
}

export interface DiscoveryTrackedListResult {
  entries: DiscoveryTrackedEntry[];
  quarantined?: DiscoveryQuarantinedEntry[];
  current_endpoint: string;
}

// Re-link probe: handlers.RelinkProbeResult. Either `container` is
// set (the tracked key was found on the current endpoint and the
// frontend just needs a Confirm) or `candidates` lists every running
// container (the operator must pick one). `error` is set when the
// daemon refused the list call - rendered inline in the picker.
export interface DiscoveryRelinkCandidate {
  key: string;
  name: string;
  image: string;
}

export interface DiscoveryRelinkProbeResult {
  found: boolean;
  container?: DiscoveryRelinkCandidate;
  candidates?: DiscoveryRelinkCandidate[];
}

export interface DiscoveryRelinkConfirmRequest {
  old_key: string;
  new_key: string;
  strategy?: string;
}

export interface DiscoveryRelinkConfirmResult {
  updated_apps: string[];
  updated_sites: string[];
}

/**
 * FireActionResult is the JSON payload returned by POST /api/app-action/{name}.
 * Either status or error is populated, never both. latency_ms is always set.
 */
export interface FireActionResult {
  status?: number;
  error?: string;
  latency_ms: number;
  url_host?: string;
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH' | string;
}
