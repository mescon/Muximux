import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  slugify,
  parseImportedConfig,
  exportConfig,
  fetchConfig,
  saveConfig,
  fetchApps,
  fetchGroups,
  getApp,
  createApp,
  updateApp,
  deleteApp,
  getGroup,
  createGroup,
  updateGroup,
  deleteGroup,
  checkHealth,
  listDashboardIcons,
  getDashboardIconUrl,
  listLucideIcons,
  getLucideIconUrl,
  listCustomIcons,
  getCustomIconUrl,
  uploadCustomIcon,
  deleteCustomIcon,
  fetchAllAppHealth,
  fetchAppHealth,
  triggerHealthCheck,
  getProxyStatus,
  listUsers,
  createUser,
  updateUser,
  deleteUserAccount,
  changeAuthMethod,
  getAPIKeyStatus,
  generateAPIKey,
  deleteAPIKey,
} from './api';
import type { Config, CreateUserRequest, UpdateUserRequest, ChangeAuthMethodRequest } from './types';

// --- Helpers ---
function mockFetchOk(data: unknown) {
  return vi.fn().mockResolvedValue({
    ok: true,
    status: 200,
    statusText: 'OK',
    json: () => Promise.resolve(data),
    text: () => Promise.resolve(JSON.stringify(data)),
  });
}

function mockFetchError(status: number, statusText: string, body?: string) {
  const responseBody = body ?? statusText;
  return vi.fn().mockResolvedValue({
    ok: false,
    status,
    statusText,
    json: () => Promise.reject(new Error('not json')),
    text: () => Promise.resolve(responseBody),
  });
}

function makeConfig(overrides: Partial<Config> = {}): Config {
  return {
    title: 'Test',
    navigation: {
      position: 'top',
      width: '64px',
      auto_hide: false,
      auto_hide_delay: '3s',
      show_on_hover: false,
      show_labels: true,
      show_logo: true,
      show_app_colors: true,
      show_icon_background: true,
      icon_scale: 1,
      show_splash_on_startup: true,
      show_shadow: true,
      floating_position: 'bottom-right',
      bar_style: 'grouped',
      hide_sidebar_footer: false,
      max_open_tabs: 0,
    },
    groups: [],
    apps: [],
    ...overrides,
  };
}

// --- Tests ---

describe('slugify', () => {
  it('converts spaces to hyphens', () => {
    expect(slugify('hello world')).toBe('hello-world');
  });

  it('removes special characters', () => {
    expect(slugify('hello@world!')).toBe('helloworld');
  });

  it('converts to lowercase', () => {
    expect(slugify('Hello World')).toBe('hello-world');
  });

  it('collapses multiple spaces into single hyphen', () => {
    expect(slugify('hello   world')).toBe('hello-world');
  });

  it('handles empty string', () => {
    expect(slugify('')).toBe('');
  });

  it('handles string with only special chars', () => {
    expect(slugify('!@#$%')).toBe('');
  });

  it('keeps numbers', () => {
    expect(slugify('app 2 test')).toBe('app-2-test');
  });

  it('keeps existing hyphens', () => {
    expect(slugify('my-app')).toBe('my-app');
  });
});

describe('parseImportedConfig', () => {
  let originalFetch: typeof globalThis.fetch;

  beforeEach(() => {
    originalFetch = globalThis.fetch;
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  it('sends YAML to backend and returns parsed config', async () => {
    const mockResponse = {
      title: 'Test Config',
      navigation: { position: 'top' },
      groups: [],
      apps: [{ name: 'App1', url: 'http://example.com' }],
    };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse),
    });

    const result = await parseImportedConfig('server:\n  title: Test Config\n');
    expect(result.title).toBe('Test Config');
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/config/import', {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-yaml' },
      body: 'server:\n  title: Test Config\n',
    });
  });

  it('throws on backend error', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      text: () => Promise.resolve('Invalid YAML: bad format'),
    });

    await expect(parseImportedConfig('bad yaml')).rejects.toThrow('Invalid YAML: bad format');
  });
});

describe('exportConfig', () => {
  it('redirects to the YAML export endpoint', () => {
    // exportConfig sets window.location.href — just verify it doesn't throw
    // (full navigation test requires browser env)
    expect(() => exportConfig()).not.toThrow();
  });
});

describe('fetchJSON / postJSON / putJSON wrappers', () => {
  let originalFetch: typeof globalThis.fetch;

  beforeEach(() => {
    originalFetch = globalThis.fetch;
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  describe('fetchConfig', () => {
    it('returns config on success', async () => {
      const config = makeConfig();
      globalThis.fetch = mockFetchOk(config);
      const result = await fetchConfig();
      expect(result).toEqual(config);
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/config', { method: 'GET' });
    });

    it('throws on non-OK response', async () => {
      globalThis.fetch = mockFetchError(500, 'Internal Server Error');
      await expect(fetchConfig()).rejects.toThrow('API error: 500 Internal Server Error');
    });
  });

  describe('saveConfig', () => {
    it('sends PUT with config body', async () => {
      const config = makeConfig();
      globalThis.fetch = mockFetchOk(config);
      const result = await saveConfig(config);
      expect(result).toEqual(config);
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config),
      });
    });

    it('throws on non-OK response with body text', async () => {
      globalThis.fetch = mockFetchError(400, 'Bad Request', 'Validation failed');
      await expect(saveConfig(makeConfig())).rejects.toThrow('API error: 400 Validation failed');
    });
  });

  describe('fetchApps', () => {
    it('fetches apps list', async () => {
      const apps = [{ name: 'App1', url: 'http://a.com' }];
      globalThis.fetch = mockFetchOk(apps);
      const result = await fetchApps();
      expect(result).toEqual(apps);
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/apps', { method: 'GET' });
    });
  });

  describe('fetchGroups', () => {
    it('fetches groups list', async () => {
      const groups = [{ name: 'Group1' }];
      globalThis.fetch = mockFetchOk(groups);
      const result = await fetchGroups();
      expect(result).toEqual(groups);
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/groups', { method: 'GET' });
    });
  });

  describe('getApp', () => {
    it('fetches a single app by name', async () => {
      const app = { name: 'MyApp', url: 'http://a.com' };
      globalThis.fetch = mockFetchOk(app);
      const result = await getApp('MyApp');
      expect(result).toEqual(app);
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/app/MyApp', { method: 'GET' });
    });

    it('encodes special characters in name', async () => {
      globalThis.fetch = mockFetchOk({});
      await getApp('my app/test');
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/app/my%20app%2Ftest', { method: 'GET' });
    });
  });

  describe('createApp', () => {
    it('sends POST to /apps', async () => {
      const app = { name: 'New', url: 'http://new.com' };
      globalThis.fetch = mockFetchOk(app);
      const result = await createApp(app);
      expect(result).toEqual(app);
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/apps', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(app),
      });
    });
  });

  describe('updateApp', () => {
    it('sends PUT to /app/:name', async () => {
      const app = { name: 'Updated', url: 'http://up.com' };
      globalThis.fetch = mockFetchOk(app);
      const result = await updateApp('MyApp', app);
      expect(result).toEqual(app);
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/app/MyApp', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(app),
      });
    });
  });

  describe('deleteApp', () => {
    it('sends DELETE to /app/:name', async () => {
      globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, status: 200 });
      await deleteApp('MyApp');
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/app/MyApp', { method: 'DELETE' });
    });

    it('throws on non-OK response', async () => {
      globalThis.fetch = vi.fn().mockResolvedValue({ ok: false, status: 404 });
      await expect(deleteApp('Missing')).rejects.toThrow('API error: 404');
    });
  });

  describe('getGroup', () => {
    it('fetches a single group by name', async () => {
      const group = { name: 'MyGroup' };
      globalThis.fetch = mockFetchOk(group);
      const result = await getGroup('MyGroup');
      expect(result).toEqual(group);
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/group/MyGroup', { method: 'GET' });
    });
  });

  describe('createGroup', () => {
    it('sends POST to /groups', async () => {
      const group = { name: 'NewGroup' };
      globalThis.fetch = mockFetchOk(group);
      const result = await createGroup(group);
      expect(result).toEqual(group);
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/groups', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(group),
      });
    });
  });

  describe('updateGroup', () => {
    it('sends PUT to /group/:name', async () => {
      const group = { name: 'Updated' };
      globalThis.fetch = mockFetchOk(group);
      const result = await updateGroup('MyGroup', group);
      expect(result).toEqual(group);
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/group/MyGroup', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(group),
      });
    });
  });

  describe('deleteGroup', () => {
    it('sends DELETE to /group/:name', async () => {
      globalThis.fetch = vi.fn().mockResolvedValue({ ok: true, status: 200 });
      await deleteGroup('MyGroup');
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/group/MyGroup', { method: 'DELETE' });
    });

    it('throws on non-OK response', async () => {
      globalThis.fetch = vi.fn().mockResolvedValue({ ok: false, status: 404 });
      await expect(deleteGroup('Missing')).rejects.toThrow('API error: 404');
    });
  });

  describe('checkHealth', () => {
    it('returns true when API is healthy', async () => {
      globalThis.fetch = vi.fn().mockResolvedValue({ ok: true });
      const result = await checkHealth();
      expect(result).toBe(true);
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/health');
    });

    it('returns false when API returns non-OK', async () => {
      globalThis.fetch = vi.fn().mockResolvedValue({ ok: false });
      const result = await checkHealth();
      expect(result).toBe(false);
    });

    it('returns false when fetch throws', async () => {
      globalThis.fetch = vi.fn().mockRejectedValue(new Error('Network error'));
      const result = await checkHealth();
      expect(result).toBe(false);
    });
  });

  describe('icon functions', () => {
    it('listDashboardIcons fetches without query', async () => {
      globalThis.fetch = mockFetchOk([]);
      await listDashboardIcons();
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/icons/dashboard', { method: 'GET' });
    });

    it('listDashboardIcons fetches with query', async () => {
      globalThis.fetch = mockFetchOk([]);
      await listDashboardIcons('home');
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/icons/dashboard?q=home', { method: 'GET' });
    });

    it('getDashboardIconUrl returns correct URL', () => {
      expect(getDashboardIconUrl('home')).toBe('/icons/dashboard/home.svg');
      expect(getDashboardIconUrl('home', 'png')).toBe('/icons/dashboard/home.png');
    });

    it('listLucideIcons fetches without query', async () => {
      globalThis.fetch = mockFetchOk([]);
      await listLucideIcons();
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/icons/lucide', { method: 'GET' });
    });

    it('listLucideIcons fetches with query', async () => {
      globalThis.fetch = mockFetchOk([]);
      await listLucideIcons('arrow');
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/icons/lucide?q=arrow', { method: 'GET' });
    });

    it('getLucideIconUrl returns correct URL', () => {
      expect(getLucideIconUrl('arrow-left')).toBe('/icons/lucide/arrow-left.svg');
    });

    it('listCustomIcons fetches', async () => {
      globalThis.fetch = mockFetchOk([]);
      await listCustomIcons();
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/icons/custom', { method: 'GET' });
    });

    it('getCustomIconUrl returns correct URL', () => {
      expect(getCustomIconUrl('my-icon.png')).toBe('/icons/custom/my-icon.png');
    });
  });

  describe('uploadCustomIcon', () => {
    it('uploads file with FormData', async () => {
      globalThis.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ name: 'icon.png', status: 'ok' }),
      });

      const file = new File(['content'], 'icon.png', { type: 'image/png' });
      const result = await uploadCustomIcon(file);
      expect(result).toEqual({ name: 'icon.png', status: 'ok' });

      const call = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
      expect(call[0]).toBe('/api/icons/custom');
      expect(call[1].method).toBe('POST');
      expect(call[1].body).toBeInstanceOf(FormData);
    });

    it('uploads file with custom name', async () => {
      globalThis.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ name: 'custom.png', status: 'ok' }),
      });

      const file = new File(['content'], 'icon.png', { type: 'image/png' });
      await uploadCustomIcon(file, 'custom.png');

      const call = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
      const formData = call[1].body as FormData;
      expect(formData.get('name')).toBe('custom.png');
    });

    it('throws on failure', async () => {
      globalThis.fetch = vi.fn().mockResolvedValue({
        ok: false,
        text: () => Promise.resolve('Bad file'),
      });

      const file = new File(['content'], 'icon.png');
      await expect(uploadCustomIcon(file)).rejects.toThrow('Bad file');
    });

    it('throws default message when body is empty', async () => {
      globalThis.fetch = vi.fn().mockResolvedValue({
        ok: false,
        text: () => Promise.resolve(''),
      });

      const file = new File(['content'], 'icon.png');
      await expect(uploadCustomIcon(file)).rejects.toThrow('Upload failed');
    });
  });

  describe('deleteCustomIcon', () => {
    it('sends DELETE request', async () => {
      globalThis.fetch = vi.fn().mockResolvedValue({ ok: true });
      await deleteCustomIcon('icon.png');
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/icons/custom/icon.png', { method: 'DELETE' });
    });

    it('throws on failure', async () => {
      globalThis.fetch = vi.fn().mockResolvedValue({
        ok: false,
        text: () => Promise.resolve('Not found'),
      });
      await expect(deleteCustomIcon('missing.png')).rejects.toThrow('Not found');
    });

    it('throws default message when body is empty', async () => {
      globalThis.fetch = vi.fn().mockResolvedValue({
        ok: false,
        text: () => Promise.resolve(''),
      });
      await expect(deleteCustomIcon('missing.png')).rejects.toThrow('Delete failed');
    });
  });

  describe('app health functions', () => {
    it('fetchAllAppHealth fetches /apps/health', async () => {
      globalThis.fetch = mockFetchOk([]);
      await fetchAllAppHealth();
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/apps/health', { method: 'GET' });
    });

    it('fetchAppHealth fetches single app health', async () => {
      globalThis.fetch = mockFetchOk({ name: 'app1', status: 'healthy' });
      await fetchAppHealth('app1');
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/apps/app1/health', { method: 'GET' });
    });

    it('triggerHealthCheck posts to health/check', async () => {
      globalThis.fetch = mockFetchOk({ name: 'app1', status: 'healthy' });
      await triggerHealthCheck('app1');
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/apps/app1/health/check', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({}),
      });
    });
  });

  describe('getProxyStatus', () => {
    it('fetches proxy status', async () => {
      const status = { enabled: true, running: true, tls: false };
      globalThis.fetch = mockFetchOk(status);
      const result = await getProxyStatus();
      expect(result).toEqual(status);
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/proxy/status', { method: 'GET' });
    });
  });

  describe('listUsers', () => {
    it('returns array of users on success', async () => {
      const users = [
        { username: 'admin', role: 'admin', email: 'admin@example.com' },
        { username: 'viewer', role: 'viewer' },
      ];
      globalThis.fetch = mockFetchOk(users);
      const result = await listUsers();
      expect(result).toEqual(users);
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/auth/users', { method: 'GET' });
    });

    it('throws on non-OK response', async () => {
      globalThis.fetch = mockFetchError(500, 'Internal Server Error');
      await expect(listUsers()).rejects.toThrow('API error: 500 Internal Server Error');
    });
  });

  describe('createUser', () => {
    it('sends POST with user data and returns result', async () => {
      const request: CreateUserRequest = {
        username: 'newuser',
        password: 'secret123',
        role: 'viewer',
        email: 'new@example.com',
      };
      const response = { success: true, user: { username: 'newuser', role: 'viewer', email: 'new@example.com' } };
      globalThis.fetch = mockFetchOk(response);
      const result = await createUser(request);
      expect(result).toEqual(response);
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/auth/users', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(request),
      });
    });

    it('throws on non-OK response', async () => {
      const request: CreateUserRequest = { username: 'newuser', password: 'secret', role: 'viewer' };
      globalThis.fetch = mockFetchError(400, 'Bad Request', 'Username already exists');
      await expect(createUser(request)).rejects.toThrow('API error: 400 Username already exists');
    });
  });

  describe('updateUser', () => {
    it('sends PUT with encoded username and update data', async () => {
      const data: UpdateUserRequest = { role: 'admin', display_name: 'Updated Name' };
      const response = { success: true, user: { username: 'testuser', role: 'admin', display_name: 'Updated Name' } };
      globalThis.fetch = mockFetchOk(response);
      const result = await updateUser('testuser', data);
      expect(result).toEqual(response);
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/auth/users/testuser', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      });
    });

    it('encodes special characters in username', async () => {
      const data: UpdateUserRequest = { role: 'viewer' };
      const response = { success: true };
      globalThis.fetch = mockFetchOk(response);
      await updateUser('user name/special', data);
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/auth/users/user%20name%2Fspecial', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      });
    });

    it('throws on non-OK response', async () => {
      const data: UpdateUserRequest = { role: 'admin' };
      globalThis.fetch = mockFetchError(404, 'Not Found', 'User not found');
      await expect(updateUser('missing', data)).rejects.toThrow('API error: 404 User not found');
    });
  });

  describe('deleteUserAccount', () => {
    it('sends DELETE request for user', async () => {
      globalThis.fetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        statusText: 'OK',
        text: () => Promise.resolve(''),
      });
      await deleteUserAccount('testuser');
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/auth/users/testuser', {
        method: 'DELETE',
      });
    });

    it('encodes special characters in username', async () => {
      globalThis.fetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        statusText: 'OK',
        text: () => Promise.resolve(''),
      });
      await deleteUserAccount('user name/special');
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/auth/users/user%20name%2Fspecial', {
        method: 'DELETE',
      });
    });

    it('throws on non-OK response', async () => {
      globalThis.fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 403,
        statusText: 'Forbidden',
        text: () => Promise.resolve('Cannot delete own account'),
      });
      await expect(deleteUserAccount('admin')).rejects.toThrow('API error: 403 Cannot delete own account');
    });
  });

  describe('changeAuthMethod', () => {
    it('sends PUT with method data and returns result', async () => {
      const data: ChangeAuthMethodRequest = { method: 'builtin' };
      const response = { success: true, method: 'builtin' };
      globalThis.fetch = mockFetchOk(response);
      const result = await changeAuthMethod(data);
      expect(result).toEqual(response);
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/auth/method', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      });
    });

    it('sends PUT with forward_auth config', async () => {
      const data: ChangeAuthMethodRequest = {
        method: 'forward_auth',
        trusted_proxies: ['10.0.0.1'],
        headers: { 'X-Forwarded-User': 'username' },
      };
      const response = { success: true, method: 'forward_auth' };
      globalThis.fetch = mockFetchOk(response);
      const result = await changeAuthMethod(data);
      expect(result).toEqual(response);
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/auth/method', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      });
    });

    it('throws on non-OK response', async () => {
      const data: ChangeAuthMethodRequest = { method: 'none' };
      globalThis.fetch = mockFetchError(400, 'Bad Request', 'Invalid method');
      await expect(changeAuthMethod(data)).rejects.toThrow('API error: 400 Invalid method');
    });
  });

  describe('getAPIKeyStatus', () => {
    it('GETs /auth/api-key and returns the status payload', async () => {
      globalThis.fetch = mockFetchOk({ configured: true });
      const result = await getAPIKeyStatus();
      expect(result).toEqual({ configured: true });
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/auth/api-key', { method: 'GET' });
    });

    it('throws an ApiError on a non-OK response', async () => {
      globalThis.fetch = mockFetchError(403, 'Forbidden', 'admin only');
      await expect(getAPIKeyStatus()).rejects.toThrow('API error: 403 admin only');
    });
  });

  describe('generateAPIKey', () => {
    it('POSTs /auth/api-key without a body and returns the plaintext key once', async () => {
      const response = { success: true, key: 'muximux_abc', warning: 'shown once', rotated: false, configured: true };
      globalThis.fetch = mockFetchOk(response);
      const result = await generateAPIKey();
      expect(result).toEqual(response);
      // postJSON called with `undefined` body sets no Content-Type and no body.
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/auth/api-key', { method: 'POST' });
    });

    it('throws when the server reports failure', async () => {
      globalThis.fetch = mockFetchError(500, 'Internal Server Error', 'rand failed');
      await expect(generateAPIKey()).rejects.toThrow('API error: 500 rand failed');
    });
  });

  describe('deleteAPIKey', () => {
    it('DELETEs /auth/api-key and resolves on 200', async () => {
      // The endpoint returns JSON, but the helper discards bodies on DELETE.
      globalThis.fetch = mockFetchOk({ success: true, configured: false });
      await expect(deleteAPIKey()).resolves.toBeUndefined();
      expect(globalThis.fetch).toHaveBeenCalledWith('/api/auth/api-key', { method: 'DELETE' });
    });

    it('throws on a non-OK response', async () => {
      globalThis.fetch = mockFetchError(500, 'Internal Server Error', 'disk write failed');
      await expect(deleteAPIKey()).rejects.toThrow('API error: 500 disk write failed');
    });
  });
});
