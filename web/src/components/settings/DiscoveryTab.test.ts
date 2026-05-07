import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';
import type { DiscoveryDockerStatus } from '$lib/types';

const mockApi = vi.hoisted(() => ({
  fetchDiscoveryDockerStatus: vi.fn(),
  updateDiscoveryDockerConfig: vi.fn(),
  testDiscoveryDockerConfig: vi.fn(),
  scanDockerContainers: vi.fn(),
  importDockerSuggestions: vi.fn(),
  listDockerTracked: vi.fn(),
  detachDockerTracked: vi.fn(),
  probeDockerRelink: vi.fn(),
  confirmDockerRelink: vi.fn(),
}));

vi.mock('$lib/api', () => mockApi);

import DiscoveryTab from './DiscoveryTab.svelte';

function makeStatus(overrides: Partial<DiscoveryDockerStatus> = {}): DiscoveryDockerStatus {
  return {
    configured: true,
    reachable: true,
    strategy_ok: true,
    endpoint: 'unix:///var/run/docker.sock',
    api_version: '1.45',
    strategy: 'container_ip',
    self_detect_method: 'cgroup-v2',
    ...overrides,
  };
}

describe('DiscoveryTab divergence banner', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockApi.fetchDiscoveryDockerStatus.mockResolvedValue(makeStatus());
    // Stub the tracked-entries call so the embedded sub-component
    // doesn't throw under render.
    mockApi.listDockerTracked.mockResolvedValue({ entries: [], current_endpoint: 'unix:///var/run/docker.sock' });
  });

  it('hides the divergence banner when refresh_divergences is zero or missing', async () => {
    mockApi.fetchDiscoveryDockerStatus.mockResolvedValue(makeStatus());
    render(DiscoveryTab);
    await waitFor(() => expect(mockApi.fetchDiscoveryDockerStatus).toHaveBeenCalled());
    expect(screen.queryByText(/Gateway divergence detected/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/Gateway recovered/i)).not.toBeInTheDocument();
  });

  it('shows the active red banner when there is a divergence and no recovery yet', async () => {
    mockApi.fetchDiscoveryDockerStatus.mockResolvedValue(
      makeStatus({
        refresh_divergences: 2,
        last_divergence_at: '2026-05-06T10:00:00Z',
        recovered_at: undefined,
      }),
    );
    render(DiscoveryTab);
    await waitFor(() => {
      expect(screen.getByText(/Gateway divergence detected/i)).toBeInTheDocument();
    });
    expect(screen.getByText(/Last divergence at 2026-05-06T10:00:00Z/i)).toBeInTheDocument();
  });

  it('shows the amber recovered banner once a clean tick has happened post-divergence', async () => {
    mockApi.fetchDiscoveryDockerStatus.mockResolvedValue(
      makeStatus({
        refresh_divergences: 1,
        last_divergence_at: '2026-05-06T10:00:00Z',
        recovered_at: '2026-05-06T10:01:30Z',
      }),
    );
    render(DiscoveryTab);
    await waitFor(() => {
      expect(screen.getByText(/Gateway recovered/i)).toBeInTheDocument();
    });
    expect(screen.getByText(/recovered at 2026-05-06T10:01:30Z/i)).toBeInTheDocument();
  });
});

describe('DiscoveryTab currently-tracked panel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockApi.fetchDiscoveryDockerStatus.mockResolvedValue(makeStatus());
  });

  it('renders the empty state when there are no tracked entries', async () => {
    mockApi.listDockerTracked.mockResolvedValue({ entries: [], current_endpoint: 'unix:///var/run/docker.sock' });
    render(DiscoveryTab);
    await waitFor(() => {
      expect(screen.getByText(/No apps or gateway sites are linked to Docker yet/i)).toBeInTheDocument();
    });
  });

  it('renders tracked rows and surfaces a Re-link button when the endpoint mismatches', async () => {
    mockApi.listDockerTracked.mockResolvedValue({
      entries: [
        {
          kind: 'app',
          name: 'sonarr',
          key: 'label:sonarr-stable',
          strategy: 'container_ip',
          endpoint: 'unix:///var/run/docker.sock',
          url: 'http://10.0.0.42:8989',
          last_seen_at: '2026-05-08T01:00:00Z',
          endpoint_matches: true,
        },
        {
          kind: 'gateway',
          name: 'stranded.example.com',
          key: 'label:stranded',
          strategy: 'container_dns',
          endpoint: 'tcp://old:2375',
          url: 'http://10.0.0.50:80',
          endpoint_matches: false,
        },
      ],
      current_endpoint: 'unix:///var/run/docker.sock',
    });
    render(DiscoveryTab);
    await waitFor(() => expect(screen.getByText('sonarr')).toBeInTheDocument());
    expect(screen.getByText('stranded.example.com')).toBeInTheDocument();
    // Endpoint mismatch -> Re-link button visible for the stranded row
    const relinkButtons = screen.queryAllByTestId('tracked-relink-btn');
    expect(relinkButtons).toHaveLength(1);
    // Both rows still get a Detach button
    expect(screen.queryAllByTestId('tracked-detach-btn')).toHaveLength(2);
  });
});
