import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import SecurityTab from './SecurityTab.svelte';
import type { Config } from '$lib/types';

// Mock API module
const mockListUsers = vi.fn().mockResolvedValue([]);
const mockCreateUser = vi.fn().mockResolvedValue({ success: true });
const mockUpdateUser = vi.fn().mockResolvedValue({ success: true });
const mockDeleteUserAccount = vi.fn().mockResolvedValue({ success: true });
const mockChangeAuthMethod = vi.fn().mockResolvedValue({ success: true });

vi.mock('$lib/api', () => ({
  listUsers: (...args: unknown[]) => mockListUsers(...args),
  createUser: (...args: unknown[]) => mockCreateUser(...args),
  updateUser: (...args: unknown[]) => mockUpdateUser(...args),
  deleteUserAccount: (...args: unknown[]) => mockDeleteUserAccount(...args),
  changeAuthMethod: (...args: unknown[]) => mockChangeAuthMethod(...args),
}));

// Mock authStore
const mockChangePassword = vi.fn();
const { mockIsAdmin, mockCurrentUser } = vi.hoisted(() => {
  return {
    mockIsAdmin: { value: true },
    mockCurrentUser: { value: { username: 'admin', role: 'admin' } },
  };
});

vi.mock('$lib/authStore', async () => {
  const { writable, derived: _derived } = await import('svelte/store');
  const isAdminBase = writable(mockIsAdmin.value);
  const currentUserBase = writable(mockCurrentUser.value);
  return {
    isAdmin: { subscribe: isAdminBase.subscribe },
    currentUser: { subscribe: currentUserBase.subscribe },
    changePassword: (...args: unknown[]) => mockChangePassword(...args),
  };
});

function makeConfig(overrides: Partial<Config['auth']> = {}): Config {
  return {
    title: 'Test',
    navigation: {
      position: 'top',
      width: '220px',
      auto_hide: false,
      auto_hide_delay: '0.5s',
      show_on_hover: true,
      show_labels: true,
      show_logo: true,
      show_app_colors: true,
      show_icon_background: false,
      icon_scale: 1,
      show_splash_on_startup: false,
      show_shadow: true,
      bar_style: 'grouped',
      floating_position: 'bottom-right',
      hide_sidebar_footer: false,
    },
    groups: [],
    apps: [],
    auth: {
      method: 'forward_auth',
      trusted_proxies: ['10.0.0.0/8'],
      headers: {
        user: 'Remote-User',
        email: 'Remote-Email',
        groups: 'Remote-Groups',
        name: 'Remote-Name',
      },
      ...overrides,
    },
  };
}

function makeBuiltinConfig(): Config {
  return makeConfig({ method: 'builtin' });
}

describe('SecurityTab', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListUsers.mockResolvedValue([]);
    mockCreateUser.mockResolvedValue({ success: true });
    mockChangeAuthMethod.mockResolvedValue({ success: true });
  });

  // ─── Existing tests (preserved) ───────────────────────────────────────────

  it('renders logout URL input for forward_auth config', async () => {
    const config = makeConfig({ logout_url: 'https://auth.example.com/logout' });
    render(SecurityTab, { props: { localConfig: config } });

    await waitFor(() => {
      const input = screen.getByLabelText('Logout URL') as HTMLInputElement;
      expect(input).toBeInTheDocument();
      expect(input.value).toBe('https://auth.example.com/logout');
    });
  });

  it('pre-fills empty logout URL from config', async () => {
    const config = makeConfig({ logout_url: '' });
    render(SecurityTab, { props: { localConfig: config } });

    await waitFor(() => {
      const input = screen.getByLabelText('Logout URL') as HTMLInputElement;
      expect(input).toBeInTheDocument();
      expect(input.value).toBe('');
    });
  });

  it('includes logout_url in auth method change request', async () => {
    const config = makeConfig({ logout_url: 'https://auth.example.com/logout' });
    render(SecurityTab, { props: { localConfig: config } });

    // Wait for component to mount and pre-fill fields
    await waitFor(() => {
      expect(screen.getByLabelText('Logout URL')).toBeInTheDocument();
    });

    // Change the logout URL to trigger the "fields changed" state
    const input = screen.getByLabelText('Logout URL') as HTMLInputElement;
    await fireEvent.input(input, { target: { value: 'https://new-auth.example.com/logout' } });

    // Find and click the update button
    await waitFor(() => {
      const updateBtn = screen.getByRole('button', { name: /update/i });
      expect(updateBtn).toBeInTheDocument();
    });

    const updateBtn = screen.getByRole('button', { name: /update/i });
    await fireEvent.click(updateBtn);

    await waitFor(() => {
      expect(mockChangeAuthMethod).toHaveBeenCalledWith(
        expect.objectContaining({
          method: 'forward_auth',
          logout_url: 'https://new-auth.example.com/logout',
        }),
      );
    });
  });

  it('preserves existing logout URL when switching presets', async () => {
    const config = makeConfig({ logout_url: 'https://my-custom-url.com/logout' });
    render(SecurityTab, { props: { localConfig: config } });

    await waitFor(() => {
      const input = screen.getByLabelText('Logout URL') as HTMLInputElement;
      expect(input.value).toBe('https://my-custom-url.com/logout');
    });

    // Switch to Authentik preset — should NOT overwrite custom URL
    const authentikBtns = screen.getAllByRole('button', { name: /authentik/i });
    await fireEvent.click(authentikBtns[0]);

    await waitFor(() => {
      const input = screen.getByLabelText('Logout URL') as HTMLInputElement;
      expect(input.value).toBe('https://my-custom-url.com/logout');
    });
  });

  // ─── Authentication Method Selection ──────────────────────────────────────

  describe('auth method selection', () => {
    it('renders all three auth method cards', async () => {
      const config = makeConfig({ method: 'none' });
      render(SecurityTab, { props: { localConfig: config } });

      expect(screen.getByText('Password authentication')).toBeInTheDocument();
      expect(screen.getByText('Auth proxy')).toBeInTheDocument();
      expect(screen.getByText('No authentication')).toBeInTheDocument();
    });

    it('shows "Current" badge on the active method', async () => {
      const config = makeConfig({ method: 'forward_auth' });
      render(SecurityTab, { props: { localConfig: config } });

      // Wait for mount to initialize selectedAuthMethod
      await waitFor(() => {
        const badges = screen.getAllByText('Current');
        expect(badges.length).toBeGreaterThanOrEqual(1);
      });
    });

    it('selects builtin method when password card is clicked', async () => {
      const config = makeConfig({ method: 'none' });
      render(SecurityTab, { props: { localConfig: config } });

      await waitFor(() => {
        expect(screen.getByText('Password authentication')).toBeInTheDocument();
      });

      const passwordBtn = screen.getByText('Password authentication').closest('button')!;
      await fireEvent.click(passwordBtn);

      // After clicking, the "Update Method" button should appear since method changed
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /update method/i })).toBeInTheDocument();
      });
    });

    it('selects none method when no-auth card is clicked', async () => {
      const config = makeConfig({ method: 'forward_auth' });
      render(SecurityTab, { props: { localConfig: config } });

      await waitFor(() => {
        expect(screen.getByText('No authentication')).toBeInTheDocument();
      });

      const noneBtn = screen.getByText('No authentication').closest('button')!;
      await fireEvent.click(noneBtn);

      // After clicking, the "Update Method" button should appear since method changed
      await waitFor(() => {
        expect(screen.getByRole('button', { name: /update method/i })).toBeInTheDocument();
      });
    });

    it('shows security warning when no-auth is selected', async () => {
      const config = makeConfig({ method: 'none' });
      render(SecurityTab, { props: { localConfig: config } });

      await waitFor(() => {
        expect(screen.getByText('Security warning')).toBeInTheDocument();
      });
    });

    it('does not show Update Method button when method has not changed', async () => {
      const config = makeConfig({ method: 'forward_auth' });
      render(SecurityTab, { props: { localConfig: config } });

      await waitFor(() => {
        expect(screen.getByLabelText('Logout URL')).toBeInTheDocument();
      });

      // The Update button should not be visible
      expect(screen.queryByRole('button', { name: /update method/i })).not.toBeInTheDocument();
    });
  });

  // ─── Forward Auth Config ──────────────────────────────────────────────────

  describe('forward auth configuration', () => {
    it('renders trusted proxy IPs textarea', async () => {
      const config = makeConfig();
      render(SecurityTab, { props: { localConfig: config } });

      await waitFor(() => {
        expect(screen.getByLabelText('Trusted proxy IPs')).toBeInTheDocument();
      });
    });

    it('pre-fills trusted proxies from config', async () => {
      const config = makeConfig({ trusted_proxies: ['10.0.0.0/8', '172.16.0.0/12'] });
      render(SecurityTab, { props: { localConfig: config } });

      await waitFor(() => {
        const textarea = screen.getByLabelText('Trusted proxy IPs') as HTMLTextAreaElement;
        expect(textarea.value).toBe('10.0.0.0/8\n172.16.0.0/12');
      });
    });

    it('renders proxy type selector with three options', async () => {
      const config = makeConfig();
      render(SecurityTab, { props: { localConfig: config } });

      await waitFor(() => {
        expect(screen.getByText('Proxy type')).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Authelia' })).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Authentik' })).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Custom' })).toBeInTheDocument();
      });
    });

    it('shows advanced header fields when Advanced toggle is clicked', async () => {
      const config = makeConfig();
      render(SecurityTab, { props: { localConfig: config } });

      await waitFor(() => {
        expect(screen.getByText(/advanced/i)).toBeInTheDocument();
      });

      const advancedBtn = screen.getByText(/Advanced: Header names/i).closest('button')!;
      await fireEvent.click(advancedBtn);

      await waitFor(() => {
        expect(screen.getByLabelText('User header')).toBeInTheDocument();
        expect(screen.getByLabelText('Email header')).toBeInTheDocument();
        expect(screen.getByLabelText('Groups header')).toBeInTheDocument();
        expect(screen.getByLabelText('Name header')).toBeInTheDocument();
      });
    });

    it('switches to authentik preset headers when Authentik is clicked', async () => {
      const config = makeConfig();
      render(SecurityTab, { props: { localConfig: config } });

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Authentik' })).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByRole('button', { name: 'Authentik' }));

      // Open advanced to verify headers
      const advancedBtn = screen.getByText(/Advanced: Header names/i).closest('button')!;
      await fireEvent.click(advancedBtn);

      await waitFor(() => {
        const userHeader = screen.getByLabelText('User header') as HTMLInputElement;
        expect(userHeader.value).toBe('X-authentik-username');
        const emailHeader = screen.getByLabelText('Email header') as HTMLInputElement;
        expect(emailHeader.value).toBe('X-authentik-email');
      });
    });

    it('disables Update button when forward_auth but no trusted proxies', async () => {
      const config = makeConfig({ method: 'none', trusted_proxies: [] });
      render(SecurityTab, { props: { localConfig: config } });

      // Switch to forward_auth
      const proxyBtn = screen.getByText('Auth proxy').closest('button')!;
      await fireEvent.click(proxyBtn);

      await waitFor(() => {
        const updateBtn = screen.getByRole('button', { name: /update method/i });
        expect(updateBtn).toBeDisabled();
      });
    });

    it('shows Update Method button when fa fields differ from config', async () => {
      const config = makeConfig();
      render(SecurityTab, { props: { localConfig: config } });

      await waitFor(() => {
        expect(screen.getByLabelText('Trusted proxy IPs')).toBeInTheDocument();
      });

      // Change trusted proxies text
      const textarea = screen.getByLabelText('Trusted proxy IPs') as HTMLTextAreaElement;
      await fireEvent.input(textarea, { target: { value: '192.168.0.0/16' } });

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /update method/i })).toBeInTheDocument();
      });
    });

    it('calls changeAuthMethod with forward_auth fields on update', async () => {
      const config = makeConfig();
      render(SecurityTab, { props: { localConfig: config } });

      await waitFor(() => {
        expect(screen.getByLabelText('Trusted proxy IPs')).toBeInTheDocument();
      });

      // Modify proxies to trigger faFieldsChanged
      const textarea = screen.getByLabelText('Trusted proxy IPs') as HTMLTextAreaElement;
      await fireEvent.input(textarea, { target: { value: '192.168.1.0/24' } });

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /update method/i })).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByRole('button', { name: /update method/i }));

      await waitFor(() => {
        expect(mockChangeAuthMethod).toHaveBeenCalledWith(
          expect.objectContaining({
            method: 'forward_auth',
            trusted_proxies: ['192.168.1.0/24'],
          }),
        );
      });
    });

    it('shows method error message when changeAuthMethod fails', async () => {
      mockChangeAuthMethod.mockResolvedValueOnce({ success: false, message: 'Invalid proxy range' });
      const config = makeConfig({ method: 'none' });
      render(SecurityTab, { props: { localConfig: config } });

      // Switch to builtin to trigger update
      const passwordBtn = screen.getByText('Password authentication').closest('button')!;
      await fireEvent.click(passwordBtn);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /update method/i })).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByRole('button', { name: /update method/i }));

      await waitFor(() => {
        expect(screen.getByText('Invalid proxy range')).toBeInTheDocument();
      });
    });

    it('shows method error when changeAuthMethod throws', async () => {
      mockChangeAuthMethod.mockRejectedValueOnce(new Error('Network error'));
      const config = makeConfig({ method: 'none' });
      render(SecurityTab, { props: { localConfig: config } });

      const passwordBtn = screen.getByText('Password authentication').closest('button')!;
      await fireEvent.click(passwordBtn);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /update method/i })).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByRole('button', { name: /update method/i }));

      await waitFor(() => {
        expect(screen.getByText('Network error')).toBeInTheDocument();
      });
    });
  });

  // ─── Password Change (builtin active) ─────────────────────────────────────

  describe('password change (builtin method active)', () => {
    it('renders Change Password section when builtin is active', async () => {
      render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      // onMount sets selectedAuthMethod to builtin, so "Change Password" heading should show
      await waitFor(() => {
        // Use the h4 heading specifically (there's also the button text)
        const heading = screen.getByRole('heading', { name: 'Change Password' });
        expect(heading).toBeInTheDocument();
      });
    });

    it('renders current password, new password, and confirm fields', async () => {
      render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByLabelText('Current password')).toBeInTheDocument();
        expect(screen.getByLabelText('New password')).toBeInTheDocument();
        expect(screen.getByLabelText('Confirm new password')).toBeInTheDocument();
      });
    });

    it('disables Change Password button when fields are empty', async () => {
      render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        const btn = screen.getByRole('button', { name: /change password/i });
        expect(btn).toBeDisabled();
      });
    });

    it('disables Change Password button when new password is too short', async () => {
      render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByLabelText('Current password')).toBeInTheDocument();
      });

      await fireEvent.input(screen.getByLabelText('Current password'), { target: { value: 'oldpass' } });
      await fireEvent.input(screen.getByLabelText('New password'), { target: { value: 'short' } });
      await fireEvent.input(screen.getByLabelText('Confirm new password'), { target: { value: 'short' } });

      const btn = screen.getByRole('button', { name: /change password/i });
      expect(btn).toBeDisabled();
    });

    it('disables Change Password button when passwords do not match', async () => {
      render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByLabelText('Current password')).toBeInTheDocument();
      });

      await fireEvent.input(screen.getByLabelText('Current password'), { target: { value: 'oldpass' } });
      await fireEvent.input(screen.getByLabelText('New password'), { target: { value: 'newpassword1' } });
      await fireEvent.input(screen.getByLabelText('Confirm new password'), { target: { value: 'newpassword2' } });

      const btn = screen.getByRole('button', { name: /change password/i });
      expect(btn).toBeDisabled();
    });

    it('shows mismatch warning when confirm differs from new password', async () => {
      render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByLabelText('New password')).toBeInTheDocument();
      });

      await fireEvent.input(screen.getByLabelText('New password'), { target: { value: 'newpassword1' } });
      await fireEvent.input(screen.getByLabelText('Confirm new password'), { target: { value: 'different' } });

      await waitFor(() => {
        expect(screen.getByText('Passwords do not match')).toBeInTheDocument();
      });
    });

    it('shows min-length warning when new password is too short', async () => {
      render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByLabelText('New password')).toBeInTheDocument();
      });

      await fireEvent.input(screen.getByLabelText('New password'), { target: { value: 'abc' } });

      await waitFor(() => {
        expect(screen.getByText('Password must be at least 8 characters')).toBeInTheDocument();
      });
    });

    it('calls changePassword and shows success on valid submit', async () => {
      mockChangePassword.mockResolvedValueOnce({ success: true });
      render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByLabelText('Current password')).toBeInTheDocument();
      });

      await fireEvent.input(screen.getByLabelText('Current password'), { target: { value: 'oldpassword' } });
      await fireEvent.input(screen.getByLabelText('New password'), { target: { value: 'newpassword123' } });
      await fireEvent.input(screen.getByLabelText('Confirm new password'), { target: { value: 'newpassword123' } });

      const btn = screen.getByRole('button', { name: /change password/i });
      expect(btn).not.toBeDisabled();
      await fireEvent.click(btn);

      await waitFor(() => {
        expect(mockChangePassword).toHaveBeenCalledWith('oldpassword', 'newpassword123');
        expect(screen.getByText('Password changed successfully')).toBeInTheDocument();
      });
    });

    it('shows error when changePassword fails', async () => {
      mockChangePassword.mockResolvedValueOnce({ success: false, message: 'Incorrect password' });
      render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByLabelText('Current password')).toBeInTheDocument();
      });

      await fireEvent.input(screen.getByLabelText('Current password'), { target: { value: 'wrong' } });
      await fireEvent.input(screen.getByLabelText('New password'), { target: { value: 'newpassword123' } });
      await fireEvent.input(screen.getByLabelText('Confirm new password'), { target: { value: 'newpassword123' } });

      await fireEvent.click(screen.getByRole('button', { name: /change password/i }));

      await waitFor(() => {
        expect(screen.getByText('Incorrect password')).toBeInTheDocument();
      });
    });
  });

  // ─── User Management (builtin + admin) ────────────────────────────────────

  describe('user management (builtin + admin)', () => {
    it('shows User Management heading when builtin + admin', async () => {
      render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByText('User Management')).toBeInTheDocument();
      });
    });

    it('renders Add User button', async () => {
      render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /add user/i })).toBeInTheDocument();
      });
    });

    it('loads and displays users on mount', async () => {
      mockListUsers.mockResolvedValueOnce([
        { username: 'alice', role: 'admin' },
        { username: 'bob', role: 'user', email: 'bob@example.com' },
      ]);

      render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByText('alice')).toBeInTheDocument();
        expect(screen.getByText('bob')).toBeInTheDocument();
      });
    });

    it('shows user email when provided', async () => {
      mockListUsers.mockResolvedValueOnce([
        { username: 'alice', role: 'admin', email: 'alice@test.com' },
      ]);

      render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByText('alice@test.com')).toBeInTheDocument();
      });
    });

    it('shows user avatar with first letter of username', async () => {
      mockListUsers.mockResolvedValueOnce([
        { username: 'alice', role: 'admin' },
      ]);

      render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByText('A')).toBeInTheDocument();
      });
    });

    it('shows loading text while fetching users', async () => {
      // Make listUsers hang
      mockListUsers.mockImplementation(() => new Promise(() => {}));

      render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByText('Loading users...')).toBeInTheDocument();
      });
    });

    it('shows error when loading users fails', async () => {
      mockListUsers.mockRejectedValueOnce(new Error('Server down'));

      render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByText('Server down')).toBeInTheDocument();
      });
    });

    it('shows add user form when Add User button is clicked', async () => {
      render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /add user/i })).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByRole('button', { name: /add user/i }));

      await waitFor(() => {
        expect(screen.getByLabelText('Username')).toBeInTheDocument();
        expect(screen.getByLabelText('Password')).toBeInTheDocument();
        expect(screen.getByLabelText('Role')).toBeInTheDocument();
      });
    });

    it('has role select with admin, power-user, and user options in add form', async () => {
      render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /add user/i })).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByRole('button', { name: /add user/i }));

      await waitFor(() => {
        const roleSelect = screen.getByLabelText('Role') as HTMLSelectElement;
        const options = Array.from(roleSelect.options).map(o => o.value);
        expect(options).toContain('admin');
        expect(options).toContain('power-user');
        expect(options).toContain('user');
      });
    });

    it('disables Add button when username is empty', async () => {
      render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /add user/i })).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByRole('button', { name: /add user/i }));

      await waitFor(() => {
        // The "Add" button inside the add user form
        const addBtns = screen.getAllByRole('button', { name: 'Add' });
        const addBtn = addBtns[addBtns.length - 1];
        expect(addBtn).toBeDisabled();
      });
    });

    it('disables Add button when password is too short', async () => {
      render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /add user/i })).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByRole('button', { name: /add user/i }));

      await waitFor(() => {
        expect(screen.getByLabelText('Username')).toBeInTheDocument();
      });

      await fireEvent.input(screen.getByLabelText('Username'), { target: { value: 'newuser' } });
      await fireEvent.input(screen.getByLabelText('Password'), { target: { value: 'short' } });

      const addBtns = screen.getAllByRole('button', { name: 'Add' });
      const addBtn = addBtns[addBtns.length - 1];
      expect(addBtn).toBeDisabled();
    });

    it('calls createUser and reloads user list on successful add', async () => {
      mockListUsers.mockResolvedValue([]);
      mockCreateUser.mockResolvedValueOnce({ success: true });

      render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /add user/i })).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByRole('button', { name: /add user/i }));

      await waitFor(() => {
        expect(screen.getByLabelText('Username')).toBeInTheDocument();
      });

      await fireEvent.input(screen.getByLabelText('Username'), { target: { value: 'newuser' } });
      await fireEvent.input(screen.getByLabelText('Password'), { target: { value: 'password123' } });

      const addBtns = screen.getAllByRole('button', { name: 'Add' });
      const addBtn = addBtns[addBtns.length - 1];
      await fireEvent.click(addBtn);

      await waitFor(() => {
        expect(mockCreateUser).toHaveBeenCalledWith({
          username: 'newuser',
          password: 'password123',
          role: 'user',
        });
      });
    });

    it('shows error when createUser fails with message', async () => {
      mockCreateUser.mockResolvedValueOnce({ success: false, message: 'Username taken' });

      render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /add user/i })).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByRole('button', { name: /add user/i }));

      await waitFor(() => {
        expect(screen.getByLabelText('Username')).toBeInTheDocument();
      });

      await fireEvent.input(screen.getByLabelText('Username'), { target: { value: 'admin' } });
      await fireEvent.input(screen.getByLabelText('Password'), { target: { value: 'password123' } });

      const addBtns = screen.getAllByRole('button', { name: 'Add' });
      await fireEvent.click(addBtns[addBtns.length - 1]);

      await waitFor(() => {
        expect(screen.getByText('Username taken')).toBeInTheDocument();
      });
    });

    it('shows error when createUser throws', async () => {
      mockCreateUser.mockRejectedValueOnce(new Error('Network failure'));

      render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /add user/i })).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByRole('button', { name: /add user/i }));

      await waitFor(() => {
        expect(screen.getByLabelText('Username')).toBeInTheDocument();
      });

      await fireEvent.input(screen.getByLabelText('Username'), { target: { value: 'newuser' } });
      await fireEvent.input(screen.getByLabelText('Password'), { target: { value: 'password123' } });

      const addBtns = screen.getAllByRole('button', { name: 'Add' });
      await fireEvent.click(addBtns[addBtns.length - 1]);

      await waitFor(() => {
        expect(screen.getByText('Network failure')).toBeInTheDocument();
      });
    });

    it('hides add user form when Cancel is clicked', async () => {
      render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /add user/i })).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByRole('button', { name: /add user/i }));

      await waitFor(() => {
        expect(screen.getByLabelText('Username')).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByRole('button', { name: /cancel/i }));

      await waitFor(() => {
        expect(screen.queryByLabelText('Username')).not.toBeInTheDocument();
      });
    });

    it('shows delete confirmation when delete button is clicked', async () => {
      mockListUsers.mockResolvedValueOnce([
        { username: 'bob', role: 'user' },
      ]);

      const { container } = render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByText('bob')).toBeInTheDocument();
      });

      // Click the delete icon button (trash icon)
      const deleteBtn = container.querySelector('button[title="Delete user"]') as HTMLButtonElement;
      expect(deleteBtn).toBeTruthy();
      await fireEvent.click(deleteBtn);

      // Confirmation buttons should appear
      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
        expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
      });
    });

    it('calls deleteUserAccount when delete is confirmed', async () => {
      mockListUsers.mockResolvedValueOnce([
        { username: 'bob', role: 'user' },
      ]);
      mockDeleteUserAccount.mockResolvedValueOnce({ success: true });

      const { container } = render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByText('bob')).toBeInTheDocument();
      });

      const deleteBtn = container.querySelector('button[title="Delete user"]') as HTMLButtonElement;
      await fireEvent.click(deleteBtn);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByRole('button', { name: 'Delete' }));

      await waitFor(() => {
        expect(mockDeleteUserAccount).toHaveBeenCalledWith('bob');
      });
    });

    it('cancels delete when cancel confirmation is clicked', async () => {
      mockListUsers.mockResolvedValueOnce([
        { username: 'bob', role: 'user' },
      ]);

      const { container } = render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByText('bob')).toBeInTheDocument();
      });

      const deleteBtn = container.querySelector('button[title="Delete user"]') as HTMLButtonElement;
      await fireEvent.click(deleteBtn);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));

      // Confirmation should be dismissed - delete icon should be visible again
      await waitFor(() => {
        expect(screen.queryByRole('button', { name: 'Delete' })).not.toBeInTheDocument();
      });
    });

    it('disables delete button for the current user', async () => {
      mockListUsers.mockResolvedValueOnce([
        { username: 'admin', role: 'admin' },
      ]);

      const { container } = render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByText('admin')).toBeInTheDocument();
      });

      const deleteBtn = container.querySelector("button[title=\"Can't delete yourself\"]") as HTMLButtonElement;
      expect(deleteBtn).toBeTruthy();
      expect(deleteBtn).toBeDisabled();
    });

    it('shows security error when deleteUser throws', async () => {
      mockListUsers.mockResolvedValueOnce([
        { username: 'bob', role: 'user' },
      ]);
      mockDeleteUserAccount.mockRejectedValueOnce(new Error('Delete failed'));

      const { container } = render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByText('bob')).toBeInTheDocument();
      });

      const deleteBtn = container.querySelector('button[title="Delete user"]') as HTMLButtonElement;
      await fireEvent.click(deleteBtn);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByRole('button', { name: 'Delete' }));

      await waitFor(() => {
        expect(screen.getByText('Delete failed')).toBeInTheDocument();
      });
    });

    it('shows security error when updateUserRole throws', async () => {
      mockListUsers.mockResolvedValueOnce([
        { username: 'bob', role: 'user' },
      ]);
      mockUpdateUser.mockRejectedValueOnce(new Error('Update failed'));

      const { container } = render(SecurityTab, { props: { localConfig: makeBuiltinConfig() } });

      await waitFor(() => {
        expect(screen.getByText('bob')).toBeInTheDocument();
      });

      // Find the role select for bob
      const selects = container.querySelectorAll('select');
      const bobSelect = Array.from(selects).find(s => s.value === 'user');
      expect(bobSelect).toBeTruthy();

      await fireEvent.change(bobSelect!, { target: { value: 'admin' } });

      await waitFor(() => {
        expect(screen.getByText('Update failed')).toBeInTheDocument();
      });
    });
  });

  // ─── Setup user when switching to builtin from none ───────────────────────

  describe('initial user setup (no existing users)', () => {
    it('shows setup form when builtin selected but no users exist', async () => {
      const config = makeConfig({ method: 'none' });
      mockListUsers.mockResolvedValue([]);

      render(SecurityTab, { props: { localConfig: config } });

      // Click builtin
      const passwordBtn = screen.getByText('Password authentication').closest('button')!;
      await fireEvent.click(passwordBtn);

      await waitFor(() => {
        expect(screen.getByText('Create your first user to enable password authentication.')).toBeInTheDocument();
        expect(screen.getByLabelText('Username')).toBeInTheDocument();
      });
    });

    it('shows text about existing users when builtin selected and users exist', async () => {
      const config = makeConfig({ method: 'none' });
      mockListUsers.mockResolvedValue([
        { username: 'existing', role: 'admin' },
      ]);

      render(SecurityTab, { props: { localConfig: config } });

      // Wait for users to load
      await waitFor(() => {
        expect(mockListUsers).toHaveBeenCalled();
      });

      // Click builtin
      const passwordBtn = screen.getByText('Password authentication').closest('button')!;
      await fireEvent.click(passwordBtn);

      await waitFor(() => {
        expect(screen.getByText('Switch to password authentication using existing users.')).toBeInTheDocument();
      });
    });
  });

  // ─── Success / error banner ───────────────────────────────────────────────

  describe('success messages', () => {
    it('shows success message when switching auth method succeeds', async () => {
      const config = makeConfig({ method: 'builtin' });
      mockChangeAuthMethod.mockResolvedValueOnce({ success: true });

      render(SecurityTab, { props: { localConfig: config } });

      // Switch to none
      const noneBtn = screen.getByText('No authentication').closest('button')!;
      await fireEvent.click(noneBtn);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /update method/i })).toBeInTheDocument();
      });

      await fireEvent.click(screen.getByRole('button', { name: /update method/i }));

      await waitFor(() => {
        expect(screen.getByText(/Authentication method changed to none/)).toBeInTheDocument();
      });
    });
  });
});
