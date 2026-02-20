import type { Config, App, Group, SetupRequest, SetupResponse, UserInfo, CreateUserRequest, UpdateUserRequest, ChangeAuthMethodRequest, SystemInfo, UpdateInfo, LogEntry } from './types';

/** Returns the configured base path (e.g. "/muximux") or "" if none. */
export function getBase(): string {
  return (globalThis as unknown as Record<string, string>).__MUXIMUX_BASE__ || '';
}

const API_BASE = getBase() + '/api';

async function fetchJSON<T>(path: string): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`);
  if (!response.ok) {
    throw new Error(`API error: ${response.status} ${response.statusText}`);
  }
  return response.json();
}

async function postJSON<T, R>(path: string, data: T): Promise<R> {
  const response = await fetch(`${API_BASE}${path}`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(data),
  });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(`API error: ${response.status} ${text}`);
  }
  return response.json();
}

async function putJSON<T, R>(path: string, data: T): Promise<R> {
  const response = await fetch(`${API_BASE}${path}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(data),
  });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(`API error: ${response.status} ${text}`);
  }
  return response.json();
}

async function deleteJSON(path: string): Promise<void> {
  const response = await fetch(`${API_BASE}${path}`, {
    method: 'DELETE',
    headers: {
      'Content-Type': 'application/json',
    },
  });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(`API error: ${response.status} ${text}`);
  }
}

export async function submitSetup(data: SetupRequest): Promise<SetupResponse> {
  return postJSON<SetupRequest, SetupResponse>('/auth/setup', data);
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
  });
  if (!response.ok) {
    throw new Error(`API error: ${response.status}`);
  }
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
  });
  if (!response.ok) {
    throw new Error(`API error: ${response.status}`);
  }
}

export async function checkHealth(): Promise<boolean> {
  try {
    const response = await fetch(`${API_BASE}/health`);
    return response.ok;
  } catch {
    return false;
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
    body: formData,
  });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || 'Upload failed');
  }

  return response.json();
}

export async function deleteCustomIcon(name: string): Promise<void> {
  const response = await fetch(`${API_BASE}/icons/custom/${encodeURIComponent(name)}`, {
    method: 'DELETE',
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
    headers: { 'Content-Type': 'application/x-yaml' },
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
