import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import SecurityTab from './SecurityTab.svelte';
import type { Config } from '$lib/types';

// Mock API module
vi.mock('$lib/api', () => ({
  listUsers: vi.fn().mockResolvedValue([]),
  createUser: vi.fn(),
  updateUser: vi.fn(),
  deleteUserAccount: vi.fn(),
  changeAuthMethod: vi.fn().mockResolvedValue({ success: true }),
}));

// Mock authStore
vi.mock('$lib/authStore', async () => {
  const { writable } = await import('svelte/store');
  const isAdminStore = writable(true);
  const currentUserStore = writable({ username: 'admin', role: 'admin' });
  return {
    isAdmin: { subscribe: isAdminStore.subscribe },
    currentUser: { subscribe: currentUserStore.subscribe },
    changePassword: vi.fn(),
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

describe('SecurityTab', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

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
    const { changeAuthMethod } = await import('$lib/api');
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
      expect(changeAuthMethod).toHaveBeenCalledWith(
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
});
