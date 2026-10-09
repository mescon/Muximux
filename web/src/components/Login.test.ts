import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

vi.mock('$lib/authStore', () => ({
  login: vi.fn().mockResolvedValue({ success: true }),
  consumeJustLoggedOut: vi.fn().mockReturnValue(false),
}));
vi.mock('$lib/api', () => ({
  getBase: vi.fn().mockReturnValue(''),
}));

import { login, consumeJustLoggedOut } from '$lib/authStore';
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

  it('shows the error as a danger notice', async () => {
    vi.mocked(login).mockResolvedValueOnce({ success: false, message: 'Invalid credentials' });

    render(Login);
    await waitFor(() => {
      expect(screen.getByLabelText('Username')).toBeInTheDocument();
    });
    await fireEvent.input(screen.getByLabelText('Username'), { target: { value: 'admin' } });
    await fireEvent.input(screen.getByLabelText('Password'), { target: { value: 'wrong' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Sign in' }));

    await waitFor(() => {
      expect(screen.getByText('Invalid credentials').className).toContain('notice-danger');
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

  describe('SSO options', () => {
    let hrefSetter: ReturnType<typeof vi.fn>;
    const originalDescriptor = Object.getOwnPropertyDescriptor(window, 'location')!;

    function setup(search: string, status: Record<string, unknown>) {
      hrefSetter = vi.fn();
      Object.defineProperty(window, 'location', {
        value: { pathname: '/', search, set href(v: string) { hrefSetter(v); } },
        writable: true,
        configurable: true,
      });
      globalThis.fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ auth_method: 'builtin', oidc_enabled: true, ...status }),
      });
    }

    afterEach(() => {
      Object.defineProperty(window, 'location', originalDescriptor);
    });

    it('auto-redirects to the provider when enabled', async () => {
      setup('', { oidc_auto_redirect: true });
      render(Login);
      await waitFor(() => {
        expect(hrefSetter).toHaveBeenCalledWith('/api/auth/oidc/login?redirect=%2F');
      });
    });

    it.each(['?logged_out=1', '?local=1', '?error=x'])('does not auto-redirect with %s', async (search) => {
      setup(search, { oidc_auto_redirect: true });
      render(Login);
      await waitFor(() => {
        expect(screen.getByLabelText('Username')).toBeInTheDocument();
      });
      expect(hrefSetter).not.toHaveBeenCalled();
    });

    it('does not auto-redirect right after an in-app logout', async () => {
      vi.mocked(consumeJustLoggedOut).mockReturnValueOnce(true);
      setup('', { oidc_auto_redirect: true });
      render(Login);
      await waitFor(() => {
        expect(screen.getByLabelText('Username')).toBeInTheDocument();
      });
      expect(hrefSetter).not.toHaveBeenCalled();
    });

    it.each([
      ['?error=oidc_failed', 'Single sign-on failed. Please try again.'],
      ['?error=oidc_denied', 'Sign-in was cancelled at the identity provider.'],
      ['?error=oidc_state', 'The sign-in request expired or was already used. Please try again.'],
      ['?error=%3Cb%3Eprovider%20text%3C%2Fb%3E', 'Single sign-on failed. Please try again.'],
    ])('shows a translated message for %s and does not auto-redirect', async (search, text) => {
      setup(search, { oidc_auto_redirect: true });
      render(Login);
      await waitFor(() => {
        expect(screen.getByRole('alert')).toHaveTextContent(text);
      });
      expect(screen.queryByText(/provider text/)).not.toBeInTheDocument();
      expect(hrefSetter).not.toHaveBeenCalled();
    });

    it('shows the SSO error even when local login is off', async () => {
      setup('?error=oidc_denied', { local_login: false });
      render(Login);
      await waitFor(() => {
        expect(screen.getByRole('alert')).toHaveTextContent('Sign-in was cancelled at the identity provider.');
      });
    });

    it('shows no SSO error without ?error', async () => {
      setup('', {});
      render(Login);
      await waitFor(() => {
        expect(screen.getByLabelText('Username')).toBeInTheDocument();
      });
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });

    it('hides the local form when local login is off', async () => {
      setup('?local=1', { local_login: false });
      render(Login);
      await waitFor(() => {
        expect(
          screen.getByText('Sign-in with a username and password is turned off. Use single sign-on.')
        ).toBeInTheDocument();
      });
      expect(screen.queryByLabelText('Username')).not.toBeInTheDocument();
      expect(screen.queryByLabelText('Password')).not.toBeInTheDocument();
      expect(screen.queryByText('or continue with username')).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: /single sign-on|SSO/i })).toBeInTheDocument();
    });
  });
});
