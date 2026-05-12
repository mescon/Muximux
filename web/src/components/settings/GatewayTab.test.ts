import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import GatewayTab from './GatewayTab.svelte';
import type { GatewaySite } from '$lib/types';

const mockListGatewaySites = vi.fn();
const mockCreateGatewaySite = vi.fn();
const mockUpdateGatewaySite = vi.fn();
const mockDeleteGatewaySite = vi.fn();
const mockValidateGatewaySite = vi.fn();

const mockFetchApps = vi.fn();

vi.mock('$lib/api', () => ({
  listGatewaySites: (...args: unknown[]) => mockListGatewaySites(...args),
  createGatewaySite: (...args: unknown[]) => mockCreateGatewaySite(...args),
  updateGatewaySite: (...args: unknown[]) => mockUpdateGatewaySite(...args),
  deleteGatewaySite: (...args: unknown[]) => mockDeleteGatewaySite(...args),
  validateGatewaySite: (...args: unknown[]) => mockValidateGatewaySite(...args),
  fetchApps: (...args: unknown[]) => mockFetchApps(...args),
}));

vi.mock('$lib/authStore', async () => {
  const { writable } = await import('svelte/store');
  return {
    isAdmin: { subscribe: writable(true).subscribe },
  };
});

function makeSite(overrides: Partial<GatewaySite> = {}): GatewaySite {
  return {
    domain: 'sonarr.example.com',
    backend_url: 'http://sonarr:8989',
    tls: 'auto',
    ...overrides,
  };
}

describe('GatewayTab', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListGatewaySites.mockResolvedValue([]);
    mockCreateGatewaySite.mockResolvedValue({ success: true, restart_required: false });
    mockUpdateGatewaySite.mockResolvedValue({ success: true, restart_required: false });
    mockDeleteGatewaySite.mockResolvedValue(undefined);
    mockValidateGatewaySite.mockResolvedValue({ valid: true });
    mockFetchApps.mockResolvedValue([]);
  });

  it('renders the empty state when no sites are configured', async () => {
    mockListGatewaySites.mockResolvedValue([]);
    render(GatewayTab);

    await waitFor(() => {
      expect(screen.getByText(/No gateway sites yet/i)).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: /add gateway site/i })).toBeInTheDocument();
  });

  it('renders existing sites in the table with status badges', async () => {
    mockListGatewaySites.mockResolvedValue([
      makeSite({ domain: 'plex.example.com', backend_url: 'http://plex:32400', streaming: true }),
      makeSite({ domain: 'sonarr.example.com', strip_frame_blockers: true, app_name: 'Sonarr' }),
    ]);
    render(GatewayTab);

    await waitFor(() => {
      expect(screen.getByText('plex.example.com')).toBeInTheDocument();
      expect(screen.getByText('sonarr.example.com')).toBeInTheDocument();
    });
    // Streaming badge
    expect(screen.getByText('streaming')).toBeInTheDocument();
    // Embeddable badge (StripFrameBlockers)
    expect(screen.getByText('embeddable')).toBeInTheDocument();
    // App-link badge text contains the app name
    expect(screen.getByText(/app: Sonarr/i)).toBeInTheDocument();
  });

  it('opens the create modal and submits a new site', async () => {
    mockListGatewaySites.mockResolvedValue([]);
    mockCreateGatewaySite.mockResolvedValue({ success: true, restart_required: false });
    render(GatewayTab);

    await waitFor(() => expect(screen.getByText(/No gateway sites yet/i)).toBeInTheDocument());

    await fireEvent.click(screen.getByRole('button', { name: /add gateway site/i }));

    const domain = screen.getByLabelText('Domain') as HTMLInputElement;
    const backend = screen.getByLabelText('Backend URL') as HTMLInputElement;
    await fireEvent.input(domain, { target: { value: 'plex.example.com' } });
    await fireEvent.input(backend, { target: { value: 'http://plex:32400' } });

    await fireEvent.click(screen.getByRole('button', { name: /add site/i }));

    await waitFor(() => {
      expect(mockCreateGatewaySite).toHaveBeenCalledTimes(1);
    });
    const arg = mockCreateGatewaySite.mock.calls[0][0];
    expect(arg.domain).toBe('plex.example.com');
    expect(arg.backend_url).toBe('http://plex:32400');
  });

  it('refuses to submit without domain or backend URL', async () => {
    mockListGatewaySites.mockResolvedValue([]);
    render(GatewayTab);

    await waitFor(() => expect(screen.getByRole('button', { name: /add gateway site/i })).toBeInTheDocument());
    await fireEvent.click(screen.getByRole('button', { name: /add gateway site/i }));

    // Submit without filling anything
    await fireEvent.click(screen.getByRole('button', { name: /add site/i }));

    await waitFor(() => {
      expect(screen.getByText(/Domain and backend URL are required/i)).toBeInTheDocument();
    });
    expect(mockCreateGatewaySite).not.toHaveBeenCalled();
  });

  it('shows a divergence banner when the API returns mismatch=true', async () => {
    // Simulate the 503 path: server returns a structured response with
    // success=false and mismatch=true, indicating the running Caddy
    // config does not match what's on disk and a restart is needed.
    mockListGatewaySites.mockResolvedValue([]);
    mockCreateGatewaySite.mockResolvedValue({
      success: false,
      mismatch: true,
      error: 'config save failed and rollback reload failed; restart Muximux to recover',
    });
    render(GatewayTab);

    await waitFor(() => expect(screen.getByRole('button', { name: /add gateway site/i })).toBeInTheDocument());
    await fireEvent.click(screen.getByRole('button', { name: /add gateway site/i }));
    await fireEvent.input(screen.getByLabelText('Domain'), { target: { value: 'x.example.com' } });
    await fireEvent.input(screen.getByLabelText('Backend URL'), { target: { value: 'http://x:80' } });
    await fireEvent.click(screen.getByRole('button', { name: /add site/i }));

    await waitFor(() => {
      expect(screen.getByText(/Configuration mismatch/i)).toBeInTheDocument();
    });
  });

  it('shows a banner when the API reports restart_required', async () => {
    mockListGatewaySites.mockResolvedValue([]);
    mockCreateGatewaySite.mockResolvedValue({ success: true, restart_required: true });
    render(GatewayTab);

    await waitFor(() => expect(screen.getByRole('button', { name: /add gateway site/i })).toBeInTheDocument());
    await fireEvent.click(screen.getByRole('button', { name: /add gateway site/i }));
    await fireEvent.input(screen.getByLabelText('Domain'), { target: { value: 'x.example.com' } });
    await fireEvent.input(screen.getByLabelText('Backend URL'), { target: { value: 'http://x:80' } });
    await fireEvent.click(screen.getByRole('button', { name: /add site/i }));

    await waitFor(() => {
      expect(screen.getByText(/Caddy isn't running yet/i)).toBeInTheDocument();
    });
  });

  it('opens the edit modal pre-populated, then submits the update', async () => {
    mockListGatewaySites.mockResolvedValue([
      makeSite({ domain: 'sonarr.example.com', backend_url: 'http://sonarr:8989' }),
    ]);
    render(GatewayTab);

    await waitFor(() => expect(screen.getByText('sonarr.example.com')).toBeInTheDocument());

    await fireEvent.click(screen.getByRole('button', { name: /^edit$/i }));

    const backend = screen.getByLabelText('Backend URL') as HTMLInputElement;
    expect(backend.value).toBe('http://sonarr:8989');

    await fireEvent.input(backend, { target: { value: 'http://sonarr:8990' } });
    await fireEvent.click(screen.getByRole('button', { name: /save changes/i }));

    await waitFor(() => {
      expect(mockUpdateGatewaySite).toHaveBeenCalledWith('sonarr.example.com', expect.objectContaining({
        backend_url: 'http://sonarr:8990',
      }));
    });
  });

  it('requires confirmation before deleting a site', async () => {
    mockListGatewaySites.mockResolvedValue([
      makeSite({ domain: 'gone.example.com' }),
    ]);
    render(GatewayTab);

    await waitFor(() => expect(screen.getByText('gone.example.com')).toBeInTheDocument());

    // Clicking the trash icon arms the per-row confirm.
    await fireEvent.click(screen.getByRole('button', { name: /delete gone.example.com/i }));
    expect(mockDeleteGatewaySite).not.toHaveBeenCalled();

    // Clicking the explicit Delete button commits.
    await fireEvent.click(screen.getByRole('button', { name: /^delete$/i }));
    await waitFor(() => {
      expect(mockDeleteGatewaySite).toHaveBeenCalledWith('gone.example.com');
    });
  });

  it('serialises proxy_headers from the textarea on submit', async () => {
    mockListGatewaySites.mockResolvedValue([]);
    render(GatewayTab);

    await waitFor(() => expect(screen.getByRole('button', { name: /add gateway site/i })).toBeInTheDocument());
    await fireEvent.click(screen.getByRole('button', { name: /add gateway site/i }));
    await fireEvent.input(screen.getByLabelText('Domain'), { target: { value: 'sonarr.example.com' } });
    await fireEvent.input(screen.getByLabelText('Backend URL'), { target: { value: 'http://sonarr:8989' } });
    await fireEvent.input(screen.getByLabelText('Upstream headers'), {
      target: { value: 'X-Api-Key: abc-123\nAuthorization: Bearer xyz' },
    });
    await fireEvent.click(screen.getByRole('button', { name: /add site/i }));

    await waitFor(() => expect(mockCreateGatewaySite).toHaveBeenCalledTimes(1));
    const arg = mockCreateGatewaySite.mock.calls[0][0];
    expect(arg.proxy_headers).toEqual({
      'X-Api-Key': 'abc-123',
      'Authorization': 'Bearer xyz',
    });
  });

  it('reveals custom cert/key fields only when TLS=custom', async () => {
    mockListGatewaySites.mockResolvedValue([]);
    render(GatewayTab);

    await waitFor(() => expect(screen.getByRole('button', { name: /add gateway site/i })).toBeInTheDocument());
    await fireEvent.click(screen.getByRole('button', { name: /add gateway site/i }));

    expect(screen.queryByLabelText('TLS cert path')).not.toBeInTheDocument();

    const tls = screen.getByLabelText('TLS') as HTMLSelectElement;
    await fireEvent.change(tls, { target: { value: 'custom' } });

    await waitFor(() => {
      expect(screen.getByLabelText('TLS cert path')).toBeInTheDocument();
      expect(screen.getByLabelText('TLS key path')).toBeInTheDocument();
    });
  });
});

describe('GatewayTab Require Muximux login (auth gate)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListGatewaySites.mockResolvedValue([]);
    mockCreateGatewaySite.mockResolvedValue({ success: true, restart_required: false });
    mockFetchApps.mockResolvedValue([]);
  });

  it('hides the min_role + allowed_groups sub-fields until Require Muximux login is checked', async () => {
    render(GatewayTab);
    await waitFor(() => expect(screen.getByRole('button', { name: /add gateway site/i })).toBeInTheDocument());
    await fireEvent.click(screen.getByRole('button', { name: /add gateway site/i }));

    // The require-auth checkbox is in the modal but its sub-fields
    // aren't rendered yet.
    expect(screen.getByTestId('gw-require-auth')).toBeInTheDocument();
    expect(screen.queryByLabelText(/Minimum role/i)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/Allowed groups/i)).not.toBeInTheDocument();
  });

  it('reveals min_role + allowed_groups sub-fields after the checkbox is ticked', async () => {
    render(GatewayTab);
    await waitFor(() => expect(screen.getByRole('button', { name: /add gateway site/i })).toBeInTheDocument());
    await fireEvent.click(screen.getByRole('button', { name: /add gateway site/i }));

    const requireAuth = screen.getByTestId('gw-require-auth').querySelector('input[type="checkbox"]') as HTMLInputElement;
    await fireEvent.click(requireAuth);

    expect(screen.getByLabelText(/Minimum role/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/Allowed groups/i)).toBeInTheDocument();
  });

  it('serialises allowed_groups from the comma-separated field into an array on submit', async () => {
    render(GatewayTab);
    await waitFor(() => expect(screen.getByRole('button', { name: /add gateway site/i })).toBeInTheDocument());
    await fireEvent.click(screen.getByRole('button', { name: /add gateway site/i }));

    await fireEvent.input(screen.getByLabelText('Domain'), { target: { value: 'sonarr.example.com' } });
    await fireEvent.input(screen.getByLabelText('Backend URL'), { target: { value: 'http://sonarr:8989' } });
    // Toggle on, then fill role + groups.
    const requireAuth = screen.getByTestId('gw-require-auth').querySelector('input[type="checkbox"]') as HTMLInputElement;
    await fireEvent.click(requireAuth);
    const role = screen.getByLabelText(/Minimum role/i) as HTMLSelectElement;
    await fireEvent.change(role, { target: { value: 'power-user' } });
    const groups = screen.getByLabelText(/Allowed groups/i) as HTMLInputElement;
    // Mixed whitespace + trailing comma deliberately so we exercise
    // the trim + filter(Boolean) path in the serialiser.
    await fireEvent.input(groups, { target: { value: ' family ,  admins ,  ' } });

    await fireEvent.click(screen.getByRole('button', { name: /add site/i }));

    await waitFor(() => expect(mockCreateGatewaySite).toHaveBeenCalledTimes(1));
    const arg = mockCreateGatewaySite.mock.calls[0][0];
    expect(arg.require_auth).toBe(true);
    expect(arg.min_role).toBe('power-user');
    expect(arg.allowed_groups).toEqual(['family', 'admins']);
  });

  it('omits allowed_groups from the payload when the field is empty (preserves "any user" semantics)', async () => {
    // The validator on the Go side treats an empty allowed_groups as
    // "no group check"; emitting it as an empty array vs. omitting it
    // is semantically equivalent at present, but the frontend has
    // historically omitted to keep the wire payload tight and avoid
    // surprises when the back-end changes its mind. This pins it.
    render(GatewayTab);
    await waitFor(() => expect(screen.getByRole('button', { name: /add gateway site/i })).toBeInTheDocument());
    await fireEvent.click(screen.getByRole('button', { name: /add gateway site/i }));

    await fireEvent.input(screen.getByLabelText('Domain'), { target: { value: 'x.example.com' } });
    await fireEvent.input(screen.getByLabelText('Backend URL'), { target: { value: 'http://x:80' } });
    const requireAuth = screen.getByTestId('gw-require-auth').querySelector('input[type="checkbox"]') as HTMLInputElement;
    await fireEvent.click(requireAuth);
    // Don't touch the groups field at all.

    await fireEvent.click(screen.getByRole('button', { name: /add site/i }));
    await waitFor(() => expect(mockCreateGatewaySite).toHaveBeenCalledTimes(1));
    const arg = mockCreateGatewaySite.mock.calls[0][0];
    expect(arg.require_auth).toBe(true);
    expect(arg.allowed_groups).toBeUndefined();
  });

  it('hydrates the require_auth + sub-fields from an existing site on edit', async () => {
    mockListGatewaySites.mockResolvedValue([
      makeSite({
        domain: 'sonarr.example.com',
        require_auth: true,
        min_role: 'user',
        allowed_groups: ['family', 'admins'],
      }),
    ]);
    render(GatewayTab);
    await waitFor(() => expect(screen.getByText('sonarr.example.com')).toBeInTheDocument());

    await fireEvent.click(screen.getByRole('button', { name: /^edit$/i }));

    // Sub-fields are rendered because require_auth=true hydrated.
    const role = screen.getByLabelText(/Minimum role/i) as HTMLSelectElement;
    expect(role.value).toBe('user');
    const groups = screen.getByLabelText(/Allowed groups/i) as HTMLInputElement;
    expect(groups.value).toBe('family, admins');
  });
});

describe('GatewayTab "Show in navigation menu" dropdown', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockListGatewaySites.mockResolvedValue([]);
    mockCreateGatewaySite.mockResolvedValue({ success: true, restart_required: false });
    mockFetchApps.mockResolvedValue([
      // Two existing apps so the optgroup branch is exercised.
      { name: 'Existing-A', url: 'http://a:80', icon: { type: 'dashboard', name: 'a' } },
      { name: 'Existing-B', url: 'http://b:80', icon: { type: 'dashboard', name: 'b' } },
    ]);
  });

  it('renders the existing-app optgroup populated from fetchApps()', async () => {
    render(GatewayTab);
    await waitFor(() => expect(mockFetchApps).toHaveBeenCalled());
    await fireEvent.click(screen.getByRole('button', { name: /add gateway site/i }));

    // <option> elements get an accessible name == their text content.
    expect(screen.getByRole('option', { name: 'Existing-A' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Existing-B' })).toBeInTheDocument();
  });

  it('reveals the "new app name" inline input when the operator picks "create new app"', async () => {
    render(GatewayTab);
    await waitFor(() => expect(mockFetchApps).toHaveBeenCalled());
    await fireEvent.click(screen.getByRole('button', { name: /add gateway site/i }));

    expect(screen.queryByLabelText(/New app name/i)).not.toBeInTheDocument();

    const choice = screen.getByLabelText(/Show in navigation menu/i) as HTMLSelectElement;
    await fireEvent.change(choice, { target: { value: '__create__' } });

    expect(screen.getByLabelText(/New app name/i)).toBeInTheDocument();
  });

  it('links to an existing app: form payload carries app_name and no new-app field', async () => {
    render(GatewayTab);
    await waitFor(() => expect(mockFetchApps).toHaveBeenCalled());
    await fireEvent.click(screen.getByRole('button', { name: /add gateway site/i }));

    await fireEvent.input(screen.getByLabelText('Domain'), { target: { value: 'b.example.com' } });
    await fireEvent.input(screen.getByLabelText('Backend URL'), { target: { value: 'http://b:80' } });
    const choice = screen.getByLabelText(/Show in navigation menu/i) as HTMLSelectElement;
    await fireEvent.change(choice, { target: { value: 'Existing-B' } });

    await fireEvent.click(screen.getByRole('button', { name: /add site/i }));
    await waitFor(() => expect(mockCreateGatewaySite).toHaveBeenCalledTimes(1));
    const arg = mockCreateGatewaySite.mock.calls[0][0];
    expect(arg.app_name).toBe('Existing-B');
  });

  it('surfaces a fetchApps failure to the top-level error banner', async () => {
    // The deliberate-loud-failure path: see comments in GatewayTab.svelte
    // around loadApps. Silently returning [] would risk unlinking an
    // existing gateway-to-app pair on save.
    mockFetchApps.mockRejectedValue(new Error('500 internal'));
    render(GatewayTab);
    await waitFor(() =>
      expect(screen.getByText(/500 internal|Failed to load apps list/i)).toBeInTheDocument(),
    );
  });
});

describe('GatewayTab Docker-managed lock on edit', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetchApps.mockResolvedValue([]);
    mockUpdateGatewaySite.mockResolvedValue({ success: true, restart_required: false });
  });

  it('renders the Docker-managed lock notice and disables Backend URL when docker_key is set on a site', async () => {
    mockListGatewaySites.mockResolvedValue([
      makeSite({ domain: 'sonarr.example.com', docker_key: 'label:sonarr-stable' }),
    ]);
    render(GatewayTab);
    await waitFor(() => expect(screen.getByText('sonarr.example.com')).toBeInTheDocument());
    await fireEvent.click(screen.getByRole('button', { name: /^edit$/i }));

    // Lock badge + Docker key string are both rendered.
    expect(screen.getByTestId('gw-form-docker-locked')).toBeInTheDocument();
    expect(screen.getByText(/label:sonarr-stable/i)).toBeInTheDocument();
    // Backend URL field becomes readonly so the operator can't drift
    // it away from the refresh-poller's source-of-truth.
    const url = screen.getByTestId('gw-form-backend-url') as HTMLInputElement;
    expect(url.readOnly).toBe(true);
  });

  it('renders the standard hint instead of the lock when docker_key is absent', async () => {
    mockListGatewaySites.mockResolvedValue([
      makeSite({ domain: 'plain.example.com', docker_key: undefined }),
    ]);
    render(GatewayTab);
    await waitFor(() => expect(screen.getByText('plain.example.com')).toBeInTheDocument());
    await fireEvent.click(screen.getByRole('button', { name: /^edit$/i }));

    expect(screen.queryByTestId('gw-form-docker-locked')).not.toBeInTheDocument();
    expect(screen.getByText(/Where Muximux forwards requests/i)).toBeInTheDocument();
  });
});
