import type { Config, App, Group } from './types';

const API_BASE = '/api';

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
  return `/icons/dashboard/${name}.${variant}`;
}

// Builtin icons
export interface BuiltinIconInfo {
  name: string;
}

export async function listBuiltinIcons(): Promise<BuiltinIconInfo[]> {
  return fetchJSON<BuiltinIconInfo[]>('/icons/builtin');
}

export function getBuiltinIconUrl(name: string): string {
  return `/icons/builtin/${name}.svg`;
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
  return `/icons/custom/${name}`;
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
  return postJSON<{}, AppHealth>(`/apps/${encodeURIComponent(appName)}/health/check`, {});
}

// Proxy types and functions
export interface ProxyStatus {
  enabled: boolean;
  running: boolean;
  listen?: string;
}

export interface AppProxyInfo {
  name?: string;
  slug: string;
  proxy_url: string;
  enabled: boolean;
}

export async function getProxyStatus(): Promise<ProxyStatus> {
  return fetchJSON<ProxyStatus>('/proxy/status');
}

export async function getAppProxyUrl(slug: string): Promise<AppProxyInfo> {
  return fetchJSON<AppProxyInfo>(`/proxy/app?slug=${encodeURIComponent(slug)}`);
}

// Helper to generate a slug from app name
export function slugify(name: string): string {
  return name
    .toLowerCase()
    .replace(/\s+/g, '-')
    .replace(/[^a-z0-9-]/g, '');
}
