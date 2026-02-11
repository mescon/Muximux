import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { get } from 'svelte/store';
import {
  authState,
  isAuthenticated,
  currentUser,
  isAdmin,
  isLoading,
  checkAuthStatus,
  login,
  logout,
  changePassword,
  getUser,
  hasRole,
} from './authStore';

// Mock fetch globally
const mockFetch = vi.fn();
globalThis.fetch = mockFetch;

describe('authStore', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Reset store to initial state
    authState.set({
      authenticated: false,
      user: null,
      loading: true,
      error: null,
    });
  });

  describe('initial state', () => {
    it('should start with loading true', () => {
      expect(get(isLoading)).toBe(true);
    });

    it('should start with authenticated false', () => {
      expect(get(isAuthenticated)).toBe(false);
    });

    it('should start with no user', () => {
      expect(get(currentUser)).toBeNull();
    });

    it('should start with isAdmin false', () => {
      expect(get(isAdmin)).toBe(false);
    });
  });

  describe('checkAuthStatus', () => {
    it('should update state when authenticated', async () => {
      mockFetch.mockResolvedValueOnce({
        json: () =>
          Promise.resolve({
            authenticated: true,
            user: { username: 'testuser', role: 'admin' },
          }),
      });

      await checkAuthStatus();

      expect(get(isAuthenticated)).toBe(true);
      expect(get(currentUser)).toEqual({ username: 'testuser', role: 'admin' });
      expect(get(isAdmin)).toBe(true);
      expect(get(isLoading)).toBe(false);
    });

    it('should update state when not authenticated', async () => {
      mockFetch.mockResolvedValueOnce({
        json: () =>
          Promise.resolve({
            authenticated: false,
            user: null,
          }),
      });

      await checkAuthStatus();

      expect(get(isAuthenticated)).toBe(false);
      expect(get(currentUser)).toBeNull();
      expect(get(isLoading)).toBe(false);
    });

    it('should handle fetch errors', async () => {
      mockFetch.mockRejectedValueOnce(new Error('Network error'));

      await checkAuthStatus();

      expect(get(isAuthenticated)).toBe(false);
      expect(get(authState).error).toBe('Network error');
      expect(get(isLoading)).toBe(false);
    });

    it('should call correct endpoint', async () => {
      mockFetch.mockResolvedValueOnce({
        json: () => Promise.resolve({ authenticated: false }),
      });

      await checkAuthStatus();

      expect(mockFetch).toHaveBeenCalledWith('/api/auth/status');
    });
  });

  describe('login', () => {
    it('should successfully login', async () => {
      mockFetch.mockResolvedValueOnce({
        json: () =>
          Promise.resolve({
            success: true,
            user: { username: 'testuser', role: 'user' },
          }),
      });

      const result = await login('testuser', 'password123', false);

      expect(result.success).toBe(true);
      expect(get(isAuthenticated)).toBe(true);
      expect(get(currentUser)?.username).toBe('testuser');
    });

    it('should handle login failure', async () => {
      mockFetch.mockResolvedValueOnce({
        json: () =>
          Promise.resolve({
            success: false,
            message: 'Invalid credentials',
          }),
      });

      const result = await login('testuser', 'wrongpass', false);

      expect(result.success).toBe(false);
      expect(result.message).toBe('Invalid credentials');
      expect(get(isAuthenticated)).toBe(false);
      expect(get(authState).error).toBe('Invalid credentials');
    });

    it('should handle network error during login', async () => {
      mockFetch.mockRejectedValueOnce(new Error('Connection refused'));

      const result = await login('testuser', 'password', false);

      expect(result.success).toBe(false);
      expect(result.message).toBe('Connection refused');
    });

    it('should send correct payload', async () => {
      mockFetch.mockResolvedValueOnce({
        json: () => Promise.resolve({ success: true, user: {} }),
      });

      await login('myuser', 'mypass', true);

      expect(mockFetch).toHaveBeenCalledWith('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username: 'myuser',
          password: 'mypass',
          remember_me: true,
        }),
      });
    });
  });

  describe('logout', () => {
    beforeEach(async () => {
      // Set up authenticated state
      authState.set({
        authenticated: true,
        user: { username: 'testuser', role: 'user' },
        loading: false,
        error: null,
      });
    });

    it('should clear auth state on logout', async () => {
      mockFetch.mockResolvedValueOnce({});

      await logout();

      expect(get(isAuthenticated)).toBe(false);
      expect(get(currentUser)).toBeNull();
    });

    it('should call logout endpoint', async () => {
      mockFetch.mockResolvedValueOnce({});

      await logout();

      expect(mockFetch).toHaveBeenCalledWith('/api/auth/logout', {
        method: 'POST',
      });
    });

    it('should clear state even if logout request fails', async () => {
      mockFetch.mockRejectedValueOnce(new Error('Network error'));

      await logout();

      expect(get(isAuthenticated)).toBe(false);
      expect(get(currentUser)).toBeNull();
    });
  });

  describe('changePassword', () => {
    it('should return success on valid password change', async () => {
      mockFetch.mockResolvedValueOnce({
        json: () =>
          Promise.resolve({
            success: true,
            message: 'Password updated',
          }),
      });

      const result = await changePassword('oldpass', 'newpass');

      expect(result.success).toBe(true);
      expect(result.message).toBe('Password updated');
    });

    it('should return failure on invalid password change', async () => {
      mockFetch.mockResolvedValueOnce({
        json: () =>
          Promise.resolve({
            success: false,
            message: 'Current password incorrect',
          }),
      });

      const result = await changePassword('wrongpass', 'newpass');

      expect(result.success).toBe(false);
      expect(result.message).toBe('Current password incorrect');
    });

    it('should handle network error', async () => {
      mockFetch.mockRejectedValueOnce(new Error('Request failed'));

      const result = await changePassword('old', 'new');

      expect(result.success).toBe(false);
      expect(result.message).toBe('Request failed');
    });

    it('should send correct payload', async () => {
      mockFetch.mockResolvedValueOnce({
        json: () => Promise.resolve({ success: true }),
      });

      await changePassword('current', 'new123');

      expect(mockFetch).toHaveBeenCalledWith('/api/auth/password', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          current_password: 'current',
          new_password: 'new123',
        }),
      });
    });
  });

  describe('getUser', () => {
    it('should return user when authenticated', () => {
      authState.set({
        authenticated: true,
        user: { username: 'testuser', role: 'admin', email: 'test@example.com' },
        loading: false,
        error: null,
      });

      const user = getUser();

      expect(user).toEqual({
        username: 'testuser',
        role: 'admin',
        email: 'test@example.com',
      });
    });

    it('should return null when not authenticated', () => {
      authState.set({
        authenticated: false,
        user: null,
        loading: false,
        error: null,
      });

      const user = getUser();

      expect(user).toBeNull();
    });
  });

  describe('hasRole', () => {
    it('should return true when user has role', () => {
      authState.set({
        authenticated: true,
        user: { username: 'admin', role: 'admin' },
        loading: false,
        error: null,
      });

      expect(hasRole('admin')).toBe(true);
    });

    it('should return false when user has different role', () => {
      authState.set({
        authenticated: true,
        user: { username: 'user', role: 'user' },
        loading: false,
        error: null,
      });

      expect(hasRole('admin')).toBe(false);
    });

    it('should return false when not authenticated', () => {
      authState.set({
        authenticated: false,
        user: null,
        loading: false,
        error: null,
      });

      expect(hasRole('admin')).toBe(false);
    });
  });

  describe('derived stores', () => {
    it('isAdmin should be true only for admin role', () => {
      authState.set({
        authenticated: true,
        user: { username: 'normaluser', role: 'user' },
        loading: false,
        error: null,
      });

      expect(get(isAdmin)).toBe(false);

      authState.set({
        authenticated: true,
        user: { username: 'adminuser', role: 'admin' },
        loading: false,
        error: null,
      });

      expect(get(isAdmin)).toBe(true);
    });
  });
});
