import type { Config, App, Group, SetupRequest, SetupResponse } from './types';

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

export async function submitSetup(data: SetupRequest): Promise<SetupResponse> {
  return postJSON<SetupRequest, SetupResponse>('/auth/setup', data);
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
  return `/icons/lucide/${name}.svg`;
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

// Config export/import helpers
export interface ExportedConfig {
  title: string;
  navigation: Config['navigation'];
  groups: Group[];
  apps: App[];
  exportedAt: string;
  version: string;
}

/**
 * Export config as a downloadable JSON file
 */
export function exportConfig(config: Config): void {
  const exportData: ExportedConfig = {
    title: config.title,
    navigation: config.navigation,
    groups: config.groups,
    apps: config.apps,
    exportedAt: new Date().toISOString(),
    version: '1.0',
  };

  const blob = new Blob([JSON.stringify(exportData, null, 2)], {
    type: 'application/json',
  });

  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `muximux-config-${new Date().toISOString().split('T')[0]}.json`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

/**
 * Parse and validate an imported config file
 */
export function parseImportedConfig(content: string): ExportedConfig {
  let data: unknown;
  try {
    data = JSON.parse(content);
  } catch {
    throw new Error('Invalid JSON format');
  }

  if (typeof data !== 'object' || data === null) {
    throw new Error('Config must be a JSON object');
  }

  const obj = data as Record<string, unknown>;

  // Validate required fields
  if (typeof obj.title !== 'string') {
    throw new TypeError('Missing or invalid "title" field');
  }

  if (!obj.navigation || typeof obj.navigation !== 'object') {
    throw new TypeError('Missing or invalid "navigation" field');
  }

  if (!Array.isArray(obj.groups)) {
    throw new TypeError('Missing or invalid "groups" field');
  }

  if (!Array.isArray(obj.apps)) {
    throw new TypeError('Missing or invalid "apps" field');
  }

  // Validate apps have required fields
  for (const app of obj.apps as Record<string, unknown>[]) {
    if (typeof app.name !== 'string' || !app.name) {
      throw new Error('Each app must have a "name" field');
    }
    if (typeof app.url !== 'string' || !app.url) {
      throw new Error(`App "${app.name}" must have a "url" field`);
    }
  }

  return obj as unknown as ExportedConfig;
}
