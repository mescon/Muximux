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
const base = ((window as unknown as Record<string, string>).__MUXIMUX_BASE__) || '';
const API_BASE = base + '/api/auth';

// Check auth status
export async function checkAuthStatus(): Promise<void> {
  authState.update((state) => ({ ...state, loading: true, error: null }));

  try {
    const response = await fetch(`${API_BASE}/status`);
    const data = await response.json();

    authState.set({
      authenticated: data.authenticated,
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

    const data = await response.json();

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

  // Redirect to external auth provider's logout page (e.g. Authelia, Authentik)
  if (logoutUrl) {
    window.location.href = logoutUrl;
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

    const data = await response.json();
    return {
      success: data.success,
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
