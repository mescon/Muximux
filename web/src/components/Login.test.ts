import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

vi.mock('$lib/authStore', () => ({
  login: vi.fn().mockResolvedValue({ success: true }),
}));
vi.mock('$lib/api', () => ({
  getBase: vi.fn().mockReturnValue(''),
}));

import { login } from '$lib/authStore';
import Login from './Login.svelte';

describe('Login', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Default: builtin auth, no OIDC
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ auth_method: 'builtin', oidc_enabled: false }),
    });
  });

  it('renders login form with username and password fields', async () => {
    render(Login);
    await waitFor(() => {
      expect(screen.getByLabelText('Username')).toBeInTheDocument();
      expect(screen.getByLabelText('Password')).toBeInTheDocument();
    });
  });

  it('renders "Sign in" submit button', async () => {
    render(Login);
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Sign in' })).toBeInTheDocument();
    });
  });

  it('shows error when submitting empty fields', async () => {
    render(Login);
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Sign in' })).toBeInTheDocument();
    });

    const submitButton = screen.getByRole('button', { name: 'Sign in' });
    await fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText('Username and password are required')).toBeInTheDocument();
    });
  });

  it('calls login function on form submit with credentials', async () => {
    render(Login);
    await waitFor(() => {
      expect(screen.getByLabelText('Username')).toBeInTheDocument();
    });

    const usernameInput = screen.getByLabelText('Username');
    const passwordInput = screen.getByLabelText('Password');

    await fireEvent.input(usernameInput, { target: { value: 'admin' } });
    await fireEvent.input(passwordInput, { target: { value: 'secret' } });

    const submitButton = screen.getByRole('button', { name: 'Sign in' });
    await fireEvent.click(submitButton);

    await waitFor(() => {
      expect(login).toHaveBeenCalledWith('admin', 'secret', false);
    });
  });

  it('calls onsuccess on successful login', async () => {
    const onsuccess = vi.fn();
    render(Login, { props: { onsuccess } });
    await waitFor(() => {
      expect(screen.getByLabelText('Username')).toBeInTheDocument();
    });

    await fireEvent.input(screen.getByLabelText('Username'), { target: { value: 'admin' } });
    await fireEvent.input(screen.getByLabelText('Password'), { target: { value: 'secret' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Sign in' }));

    await waitFor(() => {
      expect(onsuccess).toHaveBeenCalled();
    });
  });

  it('shows error message on failed login', async () => {
    vi.mocked(login).mockResolvedValueOnce({ success: false, message: 'Invalid credentials' });

    render(Login);
    await waitFor(() => {
      expect(screen.getByLabelText('Username')).toBeInTheDocument();
    });

    await fireEvent.input(screen.getByLabelText('Username'), { target: { value: 'admin' } });
    await fireEvent.input(screen.getByLabelText('Password'), { target: { value: 'wrong' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Sign in' }));

    await waitFor(() => {
      expect(screen.getByText('Invalid credentials')).toBeInTheDocument();
    });
  });

  it('shows OIDC button when oidc_enabled', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ auth_method: 'builtin', oidc_enabled: true }),
    });

    render(Login);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Sign in with SSO/i })).toBeInTheDocument();
    });
  });

  it('shows forward auth message when auth_method is forward_auth', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ auth_method: 'forward_auth', oidc_enabled: false }),
    });

    render(Login);

    await waitFor(() => {
      expect(screen.getByText('External Authentication')).toBeInTheDocument();
      expect(screen.getByText(/external authentication provider/i)).toBeInTheDocument();
    });
  });
});
