import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/svelte';
import type { DiscoveryRelinkProbeResult } from '$lib/types';

const mockApi = vi.hoisted(() => ({
  probeDockerRelink: vi.fn(),
  confirmDockerRelink: vi.fn(),
  detachDockerTracked: vi.fn(),
}));
vi.mock('$lib/api', () => mockApi);

import DiscoveryRelinkModal from './DiscoveryRelinkModal.svelte';

describe('DiscoveryRelinkModal probe outcomes', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the "found" confirm UI when the probe matches a container on the current daemon', async () => {
    const result: DiscoveryRelinkProbeResult = {
      found: true,
      container: { key: 'name:sonarr', name: 'sonarr', image: 'linuxserver/sonarr' },
    };
    mockApi.probeDockerRelink.mockResolvedValue(result);

    render(DiscoveryRelinkModal, { trackingKey: 'name:sonarr-old', onClose: () => {} });

    await waitFor(() => {
      expect(screen.getByText(/Container/i)).toBeInTheDocument();
    });
    // Multiple "sonarr" matches expected (tracked key + matched
    // name + image). The unique signal is the "Re-link?" prompt
    // copy.
    expect(screen.getByText(/Re-link\?/i)).toBeInTheDocument();
    // Confirm button is rendered with the correct testid for the
    // happy path
    expect(screen.getByTestId('relink-confirm-btn')).toBeInTheDocument();
  });

  it('renders the candidate picker when no match is found', async () => {
    mockApi.probeDockerRelink.mockResolvedValue({
      found: false,
      candidates: [
        { key: 'name:radarr', name: 'radarr', image: 'linuxserver/radarr' },
        { key: 'name:prowlarr', name: 'prowlarr', image: 'linuxserver/prowlarr' },
      ],
    });

    render(DiscoveryRelinkModal, { trackingKey: 'name:absent', onClose: () => {} });

    await waitFor(() => {
      expect(screen.getByText(/No container with key/i)).toBeInTheDocument();
    });
    const linkButtons = screen.getAllByTestId('relink-pick-btn');
    expect(linkButtons).toHaveLength(2);
    expect(screen.getByTestId('relink-detach-btn')).toBeInTheDocument();
  });

  it('fires confirmDockerRelink with the right old_key when the operator confirms a found match', async () => {
    mockApi.probeDockerRelink.mockResolvedValue({
      found: true,
      container: { key: 'name:sonarr', name: 'sonarr', image: 'linuxserver/sonarr' },
    });
    mockApi.confirmDockerRelink.mockResolvedValue({
      updated_apps: ['Sonarr'],
      updated_sites: [],
    });

    let closed = false;
    render(DiscoveryRelinkModal, {
      trackingKey: 'name:sonarr-old',
      onClose: () => {
        closed = true;
      },
    });

    await waitFor(() =>
      expect(screen.getByTestId('relink-confirm-btn')).toBeInTheDocument(),
    );
    await fireEvent.click(screen.getByTestId('relink-confirm-btn'));

    await waitFor(() => expect(mockApi.confirmDockerRelink).toHaveBeenCalled());
    const payload = mockApi.confirmDockerRelink.mock.calls[0][0];
    expect(payload.old_key).toBe('name:sonarr-old');
    expect(payload.new_key).toBe('name:sonarr');
    await waitFor(() => expect(closed).toBe(true));
  });

  it('renders the daemon-error path inline (HTTP 502 -> ApiError catch)', async () => {
    // Backend now returns 502 on daemon-unreachable; the helper
    // rejects with ApiError and the modal's catch sets
    // probeError. No sidecar Error field on a 200 anymore.
    mockApi.probeDockerRelink.mockRejectedValue(
      new Error('API error: 502 cannot reach docker daemon: connection refused'),
    );

    render(DiscoveryRelinkModal, { trackingKey: 'name:x', onClose: () => {} });

    await waitFor(() => {
      expect(screen.getByText(/connection refused/i)).toBeInTheDocument();
    });
    expect(screen.queryByTestId('relink-confirm-btn')).not.toBeInTheDocument();
    expect(screen.queryByTestId('relink-pick-btn')).not.toBeInTheDocument();
  });
});
