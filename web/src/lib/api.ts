import type { Config, App, Group, SetupRequest, SetupResponse, UserInfo, CreateUserRequest, UpdateUserRequest, ChangeAuthMethodRequest, SystemInfo, UpdateInfo, LogEntry, GatewaySite, GatewayMutationResponse, GatewayValidationResponse, DiscoveryDockerStatus, DiscoveryDockerConfig, DiscoveryScanResult, DiscoveryImportRequest, DiscoveryImportResult, DiscoveryTrackedListResult, DiscoveryRelinkProbeResult, DiscoveryRelinkConfirmRequest, DiscoveryRelinkConfirmResult, FireActionResult, DockerState } from './types';

/** Returns the configured base path (e.g. "/muximux") or "" if none. */
export function getBase(): string {
  return (globalThis as unknown as Record<string, string>).__MUXIMUX_BASE__ || '';
}

export const API_BASE = getBase() + '/api';

/**
 * ApiError carries the HTTP status alongside a concise message so callers
 * can distinguish "unauthorized" from "backend is down" from "validation
 * failed" without re-parsing a free-form string (findings.md M18).
 */
export class ApiError extends Error {
  public readonly status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
    this.name = 'ApiError';
  }
}

async function request<R>(method: string, path: string, data?: unknown): Promise<R> {
  const opts: RequestInit = { method };
  // X-Requested-With on every state-changing call. The server's CSRF
  // middleware admits POST/PUT/DELETE/PATCH only when the request
  // carries either a JSON Content-Type or this header. Cross-origin
  // pages cannot send a custom header without a CORS preflight that
  // we don't grant, which closes the browser-form CSRF surface for
  // verbs that previously slipped through (codebase review C2).
  const headers: Record<string, string> = {};
  if (method !== 'GET') {
    headers['X-Requested-With'] = 'XMLHttpRequest';
  }
  if (data !== undefined) {
    headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(data);
  }
  if (Object.keys(headers).length > 0) {
    opts.headers = headers;
  }
  const response = await fetch(`${API_BASE}${path}`, opts);
  if (!response.ok) {
    // Prefer JSON { error | message } when the server sent one. Fall
    // back to a short, HTML-stripped plaintext body. Never forward raw
    // HTML (typically a reverse-proxy 502 page) because it is useless
    // to the user and clutters toasts (findings.md M18). Status is
    // preserved on ApiError so callers can branch on 401/403 vs. 5xx
    // without string matching.
    const text = await response.text();
    const friendly = extractFriendlyErrorMessage(text);
    const message = friendly
      ? `API error: ${response.status} ${friendly}`
      : `API error: ${response.status}`;
    throw new ApiError(response.status, message);
  }
  if (response.status === 204 || method === 'DELETE') return undefined as R;
  return response.json();
}

// stripRequestIDSuffix removes the trailing "(request_id: <id>)"
// fragment that the Go respondError helper appends to 5xx and
// 401/403 bodies. Keeping the request_id out of toasts makes them
// readable; users who need the ID can still read X-Request-ID off
// the response (devtools) or copy it from the Logs tab.
//
// Implemented with simple substring search instead of a regex
// because SonarCloud S5852 flags any regex with quantifiers as
// potential ReDoS, even when the actual pattern is anchored and
// linear. Direct indexOf/lastIndexOf gives the same result with
// no ambiguity.
function stripRequestIDSuffix(s: string): string {
  const trimmed = s.trim();
  if (!trimmed.endsWith(')')) return trimmed;
  const markerIdx = trimmed.lastIndexOf('(request_id:');
  if (markerIdx === -1) return trimmed;
  return trimmed.slice(0, markerIdx).trim();
}

function extractFriendlyErrorMessage(body: string): string {
  if (!body) return '';
  try {
    const parsed = JSON.parse(body);
    if (parsed && typeof parsed === 'object') {
      const obj = parsed as Record<string, unknown>;
      if (typeof obj.error === 'string') return stripRequestIDSuffix(obj.error);
      if (typeof obj.message === 'string') return stripRequestIDSuffix(obj.message);
    }
  } catch {
    // Not JSON; fall through to plain-text handling.
  }
  const trimmed = body.trim();
  // Drop HTML-looking bodies entirely (reverse-proxy 502 pages).
  if (trimmed.startsWith('<') || /<html|<body|<!doctype/i.test(trimmed)) {
    return '';
  }
  if (trimmed.length > 200) return '';
  return stripRequestIDSuffix(trimmed);
}

async function fetchJSON<T>(path: string): Promise<T> {
  return request<T>('GET', path);
}

async function postJSON<T, R>(path: string, data: T): Promise<R> {
  return request<R>('POST', path, data);
}

async function putJSON<T, R>(path: string, data: T): Promise<R> {
  return request<R>('PUT', path, data);
}

async function deleteJSON(path: string): Promise<void> {
  return request<void>('DELETE', path);
}

/**
 * Submit the setup wizard. setupToken is the one-time proof-of-ownership
 * token printed on the server's stdout during first boot; required while
 * the instance is in the pre-setup state (findings.md C1).
 */
export async function submitSetup(data: SetupRequest, setupToken?: string): Promise<SetupResponse> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    'X-Requested-With': 'XMLHttpRequest',
  };
  if (setupToken) {
    headers['X-Setup-Token'] = setupToken;
  }
  const response = await fetch(`${API_BASE}/auth/setup`, {
    method: 'POST',
    headers,
    body: JSON.stringify(data),
  });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(`API error: ${response.status} ${text}`);
  }
  return response.json();
}

export async function listUsers(): Promise<UserInfo[]> {
  return fetchJSON<UserInfo[]>('/auth/users');
}

export async function createUser(data: CreateUserRequest): Promise<{ success: boolean; user?: UserInfo; message?: string }> {
  return postJSON<CreateUserRequest, { success: boolean; user?: UserInfo; message?: string }>('/auth/users', data);
}

export async function updateUser(username: string, data: UpdateUserRequest): Promise<{ success: boolean; user?: UserInfo; message?: string }> {
  return putJSON<UpdateUserRequest, { success: boolean; user?: UserInfo; message?: string }>(`/auth/users/${encodeURIComponent(username)}`, data);
}

export async function deleteUserAccount(username: string): Promise<void> {
  await deleteJSON(`/auth/users/${encodeURIComponent(username)}`);
}

export async function changeAuthMethod(data: ChangeAuthMethodRequest): Promise<{ success: boolean; method?: string; message?: string }> {
  return putJSON<ChangeAuthMethodRequest, { success: boolean; method?: string; message?: string }>('/auth/method', data);
}

// API key management. Status reports whether a key is configured;
// the plaintext is never exposed because only the bcrypt hash is
// stored. Generate returns the plaintext exactly once.
export async function getAPIKeyStatus(): Promise<{ configured: boolean }> {
  return fetchJSON<{ configured: boolean }>('/auth/api-key');
}

export async function generateAPIKey(): Promise<{ success: boolean; key: string; warning: string; rotated: boolean; configured: boolean; message?: string }> {
  return postJSON<undefined, { success: boolean; key: string; warning: string; rotated: boolean; configured: boolean; message?: string }>('/auth/api-key', undefined);
}

export async function deleteAPIKey(): Promise<void> {
  await deleteJSON('/auth/api-key');
}

// Gateway sites CRUD. All admin-only on the server side; the UI calls
// these only from the Gateway settings tab which is itself admin-gated.
export async function listGatewaySites(): Promise<GatewaySite[]> {
  const out = await fetchJSON<GatewaySite[] | null>('/gateway/sites');
  return out ?? [];
}

export async function createGatewaySite(site: GatewaySite): Promise<GatewayMutationResponse> {
  return postJSON<GatewaySite, GatewayMutationResponse>('/gateway/sites', site);
}

export async function updateGatewaySite(domain: string, site: GatewaySite): Promise<GatewayMutationResponse> {
  return putJSON<GatewaySite, GatewayMutationResponse>(`/gateway/sites/${encodeURIComponent(domain)}`, site);
}

export async function deleteGatewaySite(domain: string): Promise<void> {
  await deleteJSON(`/gateway/sites/${encodeURIComponent(domain)}`);
}

export async function validateGatewaySite(site: GatewaySite): Promise<GatewayValidationResponse> {
  return postJSON<GatewaySite, GatewayValidationResponse>('/gateway/validate', site);
}

export async function fetchRecentLogs(limit?: number): Promise<LogEntry[]> {
  const params = limit ? `?limit=${limit}` : '';
  return fetchJSON<LogEntry[]>(`/logs/recent${params}`);
}

export async function fetchConfig(): Promise<Config> {
  return fetchJSON<Config>('/config');
}

export async function saveConfig(config: Config): Promise<Config> {
  return putJSON<Config, Config>('/config', config);
}

export async function fetchApps(): Promise<App[]> {
  return fetchJSON<App[]>('/apps');
}

export async function fetchGroups(): Promise<Group[]> {
  return fetchJSON<Group[]>('/groups');
}

// Individual app CRUD
export async function getApp(name: string): Promise<App> {
  return fetchJSON<App>(`/app/${encodeURIComponent(name)}`);
}

export async function createApp(app: Partial<App>): Promise<App> {
  return postJSON<Partial<App>, App>('/apps', app);
}

export async function updateApp(name: string, app: Partial<App>): Promise<App> {
  return putJSON<Partial<App>, App>(`/app/${encodeURIComponent(name)}`, app);
}

export async function deleteApp(name: string): Promise<void> {
  const response = await fetch(`${API_BASE}/app/${encodeURIComponent(name)}`, {
    method: 'DELETE',
    headers: { 'X-Requested-With': 'XMLHttpRequest' },
  });
  if (!response.ok) {
    throw new Error(`API error: ${response.status}`);
  }
}

// DockerActionResult mirrors handlers.dockerActionResult on the
// backend. `error` is the short, operator-readable message from
// mapDockerError; the full daemon error stays in the audit log.
export interface DockerActionResult {
  status?: string;
  error?: string;
  latency_ms: number;
}

async function postDockerAction(name: string, action: 'start' | 'stop' | 'restart'): Promise<DockerActionResult> {
  // Path: /api/app-docker/{name}/{action} -- the hyphenated prefix
  // avoids the /api/apps/ catch-all that intercepts /api/apps/{name}
  // health routes. See dev/2026-05-22-docker-container-lifecycle-plan.md
  // Task 22 for the path-collision write-up.
  let res: Response;
  try {
    res = await fetch(`${API_BASE}/app-docker/${encodeURIComponent(name)}/${action}`, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'X-Requested-With': 'XMLHttpRequest' },
    });
  } catch (e) {
    // Network failure (daemon unreachable, connection reset): surface as
    // an error result so the caller can toast it rather than throwing an
    // unhandled rejection.
    return { error: e instanceof Error ? e.message : 'Network error', latency_ms: 0 };
  }
  // The op-result paths (success and daemon error) return JSON in this
  // shape; the gate-ladder denials (401/403/503/404/400) return a
  // text/plain body via respondError. Tolerate both, and never throw:
  // a denial must surface to the user as an error toast, not a silent
  // JSON.parse SyntaxError.
  const text = await res.text().catch(() => '');
  if (text) {
    try {
      const parsed = JSON.parse(text) as DockerActionResult;
      if (parsed.status !== undefined || parsed.error !== undefined) {
        return parsed;
      }
    } catch {
      // Not JSON -> a text/plain denial body; fall through.
    }
  }
  return { error: text.trim() || `Request failed (${res.status})`, latency_ms: 0 };
}

export async function dockerStart(name: string): Promise<DockerActionResult> {
  return postDockerAction(name, 'start');
}
export async function dockerStop(name: string): Promise<DockerActionResult> {
  return postDockerAction(name, 'stop');
}
export async function dockerRestart(name: string): Promise<DockerActionResult> {
  return postDockerAction(name, 'restart');
}

export async function getDockerState(): Promise<Map<string, DockerState>> {
  const res = await fetch(`${API_BASE}/discovery/docker-state`, { credentials: 'same-origin' });
  if (!res.ok) {
    throw new Error(`API error: ${res.status}`);
  }
  const obj = (await res.json()) as Record<string, DockerState>;
  return new Map(Object.entries(obj));
}

/**
 * fireAppAction fires the configured http_action against an app's URL via
 * the server-side relay. Returns the result on 2xx and on 502 (the server
 * encodes a network/timeout error in the body so the caller can render
 * a useful toast). Throws ApiError on auth/permission/not-found failures
 * so callers can branch on those cleanly.
 *
 * Path: POST /api/app-action/{name}
 */
export async function fireAppAction(name: string): Promise<FireActionResult> {
  const response = await fetch(`${API_BASE}/app-action/${encodeURIComponent(name)}`, {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'X-Requested-With': 'XMLHttpRequest' },
  });
  if (response.status === 502) {
    return response.json();
  }
  if (!response.ok) {
    const text = await response.text();
    const friendly = extractFriendlyErrorMessage(text);
    const message = friendly
      ? `API error: ${response.status} ${friendly}`
      : `API error: ${response.status}`;
    throw new ApiError(response.status, message);
  }
  return response.json();
}

// Individual group CRUD
export async function getGroup(name: string): Promise<Group> {
  return fetchJSON<Group>(`/group/${encodeURIComponent(name)}`);
}

export async function createGroup(group: Partial<Group>): Promise<Group> {
  return postJSON<Partial<Group>, Group>('/groups', group);
}

export async function updateGroup(name: string, group: Partial<Group>): Promise<Group> {
  return putJSON<Partial<Group>, Group>(`/group/${encodeURIComponent(name)}`, group);
}

export async function deleteGroup(name: string): Promise<void> {
  const response = await fetch(`${API_BASE}/group/${encodeURIComponent(name)}`, {
    method: 'DELETE',
    headers: { 'X-Requested-With': 'XMLHttpRequest' },
  });
  if (!response.ok) {
    throw new Error(`API error: ${response.status}`);
  }
}

/**
 * HealthResult carries a precise reason when checkHealth returns false so
 * UI code can tell "backend is down" from "fetch threw a TypeError"
 * (usually a CORS misconfig or aborted request). findings.md M19.
 */
export type HealthResult =
  | { ok: true }
  | { ok: false; reason: 'http_error' | 'network_error'; status?: number; message?: string };

export async function checkHealth(): Promise<boolean> {
  const result = await checkHealthDetailed();
  return result.ok;
}

export async function checkHealthDetailed(): Promise<HealthResult> {
  try {
    const response = await fetch(`${API_BASE}/health`);
    if (response.ok) {
      return { ok: true };
    }
    return { ok: false, reason: 'http_error', status: response.status };
  } catch (e) {
    // TypeError here is typically "Failed to fetch" — network-level
    // error (CORS, DNS, aborted). Keep the message so the operator
    // can tell a misconfig from a truly unhealthy backend.
    const message = e instanceof Error ? e.message : String(e);
    return { ok: false, reason: 'network_error', message };
  }
}

// Icon types
export interface IconInfo {
  name: string;
  variants: string[];
}

export async function listDashboardIcons(query?: string): Promise<IconInfo[]> {
  const params = query ? `?q=${encodeURIComponent(query)}` : '';
  return fetchJSON<IconInfo[]>(`/icons/dashboard${params}`);
}

export function getDashboardIconUrl(name: string, variant: string = 'svg'): string {
  return `${getBase()}/icons/dashboard/${name}.${variant}`;
}

// Lucide icons
export interface LucideIconInfo {
  name: string;
  categories?: string[];
}

export async function listLucideIcons(query?: string): Promise<LucideIconInfo[]> {
  const params = query ? `?q=${encodeURIComponent(query)}` : '';
  return fetchJSON<LucideIconInfo[]>(`/icons/lucide${params}`);
}

export function getLucideIconUrl(name: string): string {
  return `${getBase()}/icons/lucide/${name}.svg`;
}

// Custom icons
export interface CustomIconInfo {
  name: string;
  content_type: string;
  size: number;
}

export async function listCustomIcons(): Promise<CustomIconInfo[]> {
  return fetchJSON<CustomIconInfo[]>('/icons/custom');
}

export function getCustomIconUrl(name: string): string {
  return `${getBase()}/icons/custom/${name}`;
}

export async function uploadCustomIcon(file: File, name?: string): Promise<{ name: string; status: string }> {
  const formData = new FormData();
  formData.append('icon', file);
  if (name) {
    formData.append('name', name);
  }

  const response = await fetch(`${API_BASE}/icons/custom`, {
    method: 'POST',
    headers: { 'X-Requested-With': 'XMLHttpRequest' },
    body: formData,
  });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || 'Upload failed');
  }

  return response.json();
}

export async function fetchCustomIconFromUrl(url: string, name?: string): Promise<{ name: string; status: string }> {
  return postJSON<{ url: string; name?: string }, { name: string; status: string }>('/icons/custom/fetch', { url, name });
}

export async function deleteCustomIcon(name: string): Promise<void> {
  const response = await fetch(`${API_BASE}/icons/custom/${encodeURIComponent(name)}`, {
    method: 'DELETE',
    headers: { 'X-Requested-With': 'XMLHttpRequest' },
  });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || 'Delete failed');
  }
}

// App health types
export type HealthStatus = 'unknown' | 'healthy' | 'unhealthy';

export interface AppHealth {
  name: string;
  status: HealthStatus;
  response_time_ms: number;
  last_check: string;
  last_error?: string;
  uptime_percent: number;
  check_count: number;
  success_count: number;
}

export async function fetchAllAppHealth(): Promise<AppHealth[]> {
  return fetchJSON<AppHealth[]>('/apps/health');
}

export async function fetchAppHealth(appName: string): Promise<AppHealth> {
  return fetchJSON<AppHealth>(`/apps/${encodeURIComponent(appName)}/health`);
}

export async function triggerHealthCheck(appName: string): Promise<AppHealth> {
  return postJSON<Record<string, unknown>, AppHealth>(`/apps/${encodeURIComponent(appName)}/health/check`, {});
}

// Proxy types and functions
export interface ProxyStatus {
  enabled: boolean;
  running: boolean;
  tls: boolean;
  domain?: string;
  gateway?: string;
}

export async function getProxyStatus(): Promise<ProxyStatus> {
  return fetchJSON<ProxyStatus>('/proxy/status');
}

// Helper to generate a slug from app name
export function slugify(name: string): string {
  return name
    .toLowerCase()
    .replaceAll(/\s+/g, '-')
    .replaceAll(/[^a-z0-9-]/g, '');
}

/**
 * Export config as a downloadable YAML file via the backend.
 */
export function exportConfig(): void {
  globalThis.location.href = `${API_BASE}/config/export`;
}

/**
 * Parsed import result returned by the backend (same shape as Config).
 */
export interface ImportedConfig {
  title: string;
  navigation: Config['navigation'];
  groups: Group[];
  apps: App[];
}

/**
 * Send a YAML config file to the backend for parsing and validation.
 * Returns the parsed config for preview before applying.
 */
export async function parseImportedConfig(yamlContent: string): Promise<ImportedConfig> {
  const resp = await fetch(`${API_BASE}/config/import`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-yaml', 'X-Requested-With': 'XMLHttpRequest' },
    body: yamlContent,
  });
  if (!resp.ok) {
    const text = await resp.text();
    throw new Error(text || 'Failed to parse config');
  }
  return resp.json();
}

export async function fetchSystemInfo(): Promise<SystemInfo> {
  return fetchJSON<SystemInfo>('/system/info');
}

export async function checkForUpdates(): Promise<UpdateInfo> {
  return fetchJSON<UpdateInfo>('/system/updates');
}

// Discovery (Docker auto-discovery).
export async function fetchDiscoveryDockerStatus(): Promise<DiscoveryDockerStatus> {
  return fetchJSON<DiscoveryDockerStatus>('/discovery/docker/status');
}

/**
 * Returns the names of every Docker network the configured daemon
 * exposes. Used by the Discovery settings tab to power both an
 * autocomplete datalist and an inline chip list for the
 * network_filter input, so operators pick from real values instead
 * of guessing at network names. Errors (e.g. daemon unreachable)
 * surface as a 502 from the handler; callers should fall back to a
 * plain text input rather than blocking the form.
 */
export async function listDockerNetworks(): Promise<{ networks: string[] }> {
  return fetchJSON<{ networks: string[] }>('/discovery/docker/networks');
}

export async function updateDiscoveryDockerConfig(cfg: DiscoveryDockerConfig): Promise<DiscoveryDockerStatus> {
  return putJSON<DiscoveryDockerConfig, DiscoveryDockerStatus>('/discovery/docker/config', cfg);
}

export async function testDiscoveryDockerConfig(cfg: DiscoveryDockerConfig): Promise<DiscoveryDockerStatus> {
  return postJSON<DiscoveryDockerConfig, DiscoveryDockerStatus>('/discovery/docker/test', cfg);
}

export async function scanDockerContainers(): Promise<DiscoveryScanResult> {
  return fetchJSON<DiscoveryScanResult>('/discovery/docker/scan');
}

export async function importDockerSuggestions(req: DiscoveryImportRequest): Promise<DiscoveryImportResult> {
  return postJSON<DiscoveryImportRequest, DiscoveryImportResult>('/discovery/docker/import', req);
}

export async function listDockerTracked(): Promise<DiscoveryTrackedListResult> {
  return fetchJSON<DiscoveryTrackedListResult>('/discovery/docker/tracked');
}

export async function detachDockerTracked(key: string): Promise<void> {
  // Caller treats 404 as success-with-no-effect (idempotency for
  // double-clicks). The shared deleteJSON helper resolves on 2xx
  // and rejects on 4xx/5xx, so the caller catches and inspects.
  await deleteJSON(`/discovery/docker/track/${encodeURIComponent(key)}`);
}

export async function probeDockerRelink(key: string): Promise<DiscoveryRelinkProbeResult> {
  return postJSON<{ key: string }, DiscoveryRelinkProbeResult>('/discovery/docker/relink/probe', { key });
}

export async function confirmDockerRelink(req: DiscoveryRelinkConfirmRequest): Promise<DiscoveryRelinkConfirmResult> {
  return postJSON<DiscoveryRelinkConfirmRequest, DiscoveryRelinkConfirmResult>('/discovery/docker/relink/confirm', req);
}
