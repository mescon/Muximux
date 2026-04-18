import { writable, derived, get } from 'svelte/store';
import { debug } from './debug';

// User interface
export interface User {
  username: string;
  role: string;
  email?: string;
  display_name?: string;
}

// Auth state
export interface AuthState {
  authenticated: boolean;
  user: User | null;
  loading: boolean;
  error: string | null;
  setupRequired: boolean;
  logoutUrl: string | null;
}

// Initial state
const initialState: AuthState = {
  authenticated: false,
  user: null,
  loading: true,
  error: null,
  setupRequired: false,
  logoutUrl: null,
};

// Create the store
export const authState = writable<AuthState>(initialState);

// Derived stores
export const isAuthenticated = derived(authState, ($state) => $state.authenticated);
export const currentUser = derived(authState, ($state) => $state.user);
export const isAdmin = derived(authState, ($state) => $state.user?.role === 'admin');
export const isLoading = derived(authState, ($state) => $state.loading);
export const setupRequired = derived(authState, ($state) => $state.setupRequired);

// API functions
const base = ((globalThis as unknown as Record<string, string>).__MUXIMUX_BASE__) || '';
const API_BASE = base + '/api/auth';

/**
 * parseJSONSafely runs response.json() but tolerates empty bodies and
 * non-JSON error responses (e.g. a reverse-proxy 502 HTML page). Callers
 * get `null` on failure instead of an uncaught SyntaxError that strands
 * them in the "Unexpected token '<'" loop noted in findings.md M17.
 *
 * Prefers response.json() so common test mocks that only stub .json()
 * keep working; falls back to response.text() + JSON.parse for real
 * responses that might not carry a JSON body.
 */
async function parseJSONSafely(response: Response): Promise<unknown | null> {
  try {
    return await response.json();
  } catch {
    // .json() threw (not JSON, or body-not-present mock). Try text.
  }
  try {
    if (typeof response.text === 'function') {
      const text = await response.text();
      if (!text) return null;
      return JSON.parse(text);
    }
  } catch {
    // fall through
  }
  return null;
}

// Check auth status
export async function checkAuthStatus(): Promise<void> {
  authState.update((state) => ({ ...state, loading: true, error: null }));

  try {
    const response = await fetch(`${API_BASE}/status`);
    // Only bail when we explicitly got a non-OK status. Undefined `ok`
    // (some mocks) is treated as success so the original test coverage
    // of the JSON-parse path keeps working.
    if (response.ok === false) {
      // Don't try to JSON-parse a 5xx HTML page; surface a clean
      // "backend unavailable" state instead of a SyntaxError
      // (findings.md M17).
      authState.set({
        authenticated: false,
        user: null,
        loading: false,
        error: `Auth status check failed (${response.status})`,
        setupRequired: false,
        logoutUrl: null,
      });
      return;
    }
    const data = (await parseJSONSafely(response)) as {
      authenticated?: boolean;
      user?: User;
      setup_required?: boolean;
      logout_url?: string;
    } | null;
    if (!data) {
      authState.set({
        authenticated: false,
        user: null,
        loading: false,
        error: 'Auth status response was not valid JSON',
        setupRequired: false,
        logoutUrl: null,
      });
      return;
    }

    authState.set({
      authenticated: data.authenticated ?? false,
      user: data.user || null,
      loading: false,
      error: null,
      setupRequired: data.setup_required || false,
      logoutUrl: data.logout_url || null,
    });
    debug('auth', 'status', { authenticated: data.authenticated, setup_required: data.setup_required });
  } catch (e) {
    authState.set({
      authenticated: false,
      user: null,
      loading: false,
      error: e instanceof Error ? e.message : 'Failed to check auth status',
      setupRequired: false,
      logoutUrl: null,
    });
  }
}

// Login
export async function login(username: string, password: string, rememberMe: boolean = false): Promise<{ success: boolean; message?: string }> {
  authState.update((state) => ({ ...state, loading: true, error: null }));

  try {
    const response = await fetch(`${API_BASE}/login`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ username, password, remember_me: rememberMe }),
    });

    const raw = await parseJSONSafely(response);
    const data = (raw ?? {}) as { success?: boolean; user?: User; message?: string };

    // Distinguish "backend answered with a proper JSON failure" from
    // "backend returned an HTML error page" so the user sees "service
    // unavailable" instead of a truncated parse failure
    // (findings.md M17).
    if (response.ok === false && !raw) {
      const message = `Login failed (${response.status})`;
      authState.update((state) => ({ ...state, loading: false, error: message }));
      return { success: false, message };
    }

    if (data.success) {
      authState.set({
        authenticated: true,
        user: data.user,
        loading: false,
        error: null,
        setupRequired: false,
        logoutUrl: get(authState).logoutUrl,
      });
      debug('auth', 'login success', data.user?.username);
      return { success: true };
    } else {
      authState.update((state) => ({
        ...state,
        loading: false,
        error: data.message || 'Login failed',
      }));
      debug('auth', 'login failed', data.message);
      return { success: false, message: data.message };
    }
  } catch (e) {
    const message = e instanceof Error ? e.message : 'Login failed';
    authState.update((state) => ({
      ...state,
      loading: false,
      error: message,
    }));
    return { success: false, message };
  }
}

// Logout
export async function logout(): Promise<void> {
  // Capture logout URL before clearing state
  const logoutUrl = get(authState).logoutUrl;

  try {
    await fetch(`${API_BASE}/logout`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
    });
  } catch (e) {
    console.error('Logout error:', e);
  }

  authState.set({
    authenticated: false,
    user: null,
    loading: false,
    error: null,
    setupRequired: false,
    logoutUrl: null,
  });

  // Redirect to external auth provider's logout page (e.g. Authelia,
  // Authentik) but only if the URL is http(s) (findings.md H23). Any
  // vector that influenced this field (forward-auth misconfig,
  // compromised upstream) would otherwise accept `javascript:` or
  // `data:` and fire at logout.
  if (logoutUrl && isSafeExternalURL(logoutUrl)) {
    globalThis.location.href = logoutUrl;
  } else if (logoutUrl) {
    console.warn('[auth] ignored logout_url with unsafe scheme', logoutUrl);
  }
}

function isSafeExternalURL(raw: string): boolean {
  try {
    const u = new URL(raw, globalThis.location.href);
    return u.protocol === 'http:' || u.protocol === 'https:';
  } catch {
    return false;
  }
}

// Change password
export async function changePassword(currentPassword: string, newPassword: string): Promise<{ success: boolean; message?: string }> {
  try {
    const response = await fetch(`${API_BASE}/password`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        current_password: currentPassword,
        new_password: newPassword,
      }),
    });

    const raw = await parseJSONSafely(response);
    const data = (raw ?? {}) as { success?: boolean; message?: string };
    if (response.ok === false && !raw) {
      return { success: false, message: `Password change failed (${response.status})` };
    }
    return {
      success: data.success ?? false,
      message: data.message,
    };
  } catch (e) {
    return {
      success: false,
      message: e instanceof Error ? e.message : 'Failed to change password',
    };
  }
}

// Get current user (for use outside of components)
export function getUser(): User | null {
  return get(authState).user;
}

// Check if user has a specific role
export function hasRole(role: string): boolean {
  const user = get(authState).user;
  return user?.role === role;
}
