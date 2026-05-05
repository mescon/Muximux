import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import GatewayTab from './GatewayTab.svelte';
import type { GatewaySite } from '$lib/types';

const mockListGatewaySites = vi.fn();
const mockCreateGatewaySite = vi.fn();
const mockUpdateGatewaySite = vi.fn();
const mockDeleteGatewaySite = vi.fn();
const mockValidateGatewaySite = vi.fn();

vi.mock('$lib/api', () => ({
  listGatewaySites: (...args: unknown[]) => mockListGatewaySites(...args),
  createGatewaySite: (...args: unknown[]) => mockCreateGatewaySite(...args),
  updateGatewaySite: (...args: unknown[]) => mockUpdateGatewaySite(...args),
  deleteGatewaySite: (...args: unknown[]) => mockDeleteGatewaySite(...args),
  validateGatewaySite: (...args: unknown[]) => mockValidateGatewaySite(...args),
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
