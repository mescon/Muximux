import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/svelte';
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

describe('DiscoveryTab live status banner branches', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockApi.listDockerTracked.mockResolvedValue({ entries: [], current_endpoint: '' });
  });

  it('shows the gray "disabled" banner when configured=false', async () => {
    mockApi.fetchDiscoveryDockerStatus.mockResolvedValue(makeStatus({ configured: false }));
    render(DiscoveryTab);
    await waitFor(() => expect(screen.getByText(/Discovery is disabled/i)).toBeInTheDocument());
  });

  it('shows the red "unreachable" banner when reachable=false', async () => {
    mockApi.fetchDiscoveryDockerStatus.mockResolvedValue(
      makeStatus({ reachable: false, last_error: 'connection refused' }),
    );
    render(DiscoveryTab);
    await waitFor(() =>
      expect(screen.getByText(/Daemon unreachable: connection refused/i)).toBeInTheDocument(),
    );
  });

  it('shows the amber strategy-mismatch banner when reachable=true but strategy_ok=false', async () => {
    mockApi.fetchDiscoveryDockerStatus.mockResolvedValue(
      makeStatus({ reachable: true, strategy_ok: false }),
    );
    render(DiscoveryTab);
    await waitFor(() =>
      expect(screen.getByText(/cannot identify Muximux's container/i)).toBeInTheDocument(),
    );
  });

  it('surfaces a tls_warning beneath the live status banner', async () => {
    // tls_warning is a soft hint (key file world-readable etc.) - it
    // is informational and lives below the main banner so the
    // operator can fix it without other gating logic firing.
    mockApi.fetchDiscoveryDockerStatus.mockResolvedValue(
      makeStatus({ tls_warning: 'docker key file is world-readable' }),
    );
    render(DiscoveryTab);
    await waitFor(() =>
      expect(screen.getByText(/docker key file is world-readable/i)).toBeInTheDocument(),
    );
  });
});

describe('DiscoveryTab form: TLS section + host_ip gating', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockApi.listDockerTracked.mockResolvedValue({ entries: [], current_endpoint: '' });
  });

  it('hides the TLS section by default (unix:// endpoint)', async () => {
    mockApi.fetchDiscoveryDockerStatus.mockResolvedValue(
      makeStatus({ endpoint: 'unix:///var/run/docker.sock' }),
    );
    render(DiscoveryTab);
    await waitFor(() => expect(mockApi.fetchDiscoveryDockerStatus).toHaveBeenCalled());
    expect(screen.queryByText(/mTLS \(for tcp:\/\/ endpoints\)/i)).not.toBeInTheDocument();
  });

  it('reveals the TLS section when the endpoint is switched to tcp://', async () => {
    mockApi.fetchDiscoveryDockerStatus.mockResolvedValue(makeStatus());
    render(DiscoveryTab);
    await waitFor(() =>
      expect((document.getElementById('dd-endpoint') as HTMLInputElement as HTMLInputElement).value).toContain(
        'unix:///',
      ),
    );

    const endpointInput = document.getElementById('dd-endpoint') as HTMLInputElement as HTMLInputElement;
    await fireEvent.input(endpointInput, { target: { value: 'tcp://docker.example.com:2376' } });

    // The TLS <details> summary appears now.
    expect(screen.getByText(/mTLS \(for tcp:\/\/ endpoints\)/i)).toBeInTheDocument();
  });

  it('reveals the Host IP field when network strategy is switched to host_port', async () => {
    mockApi.fetchDiscoveryDockerStatus.mockResolvedValue(makeStatus());
    render(DiscoveryTab);
    await waitFor(() => expect(screen.getByLabelText(/Network strategy/i)).toBeInTheDocument());

    expect(screen.queryByLabelText(/Host IP/i)).not.toBeInTheDocument();
    const strategySelect = screen.getByLabelText(/Network strategy/i) as HTMLSelectElement;
    await fireEvent.change(strategySelect, { target: { value: 'host_port' } });
    expect(screen.getByLabelText(/Host IP/i)).toBeInTheDocument();
  });
});

describe('DiscoveryTab save + test connection wiring', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockApi.listDockerTracked.mockResolvedValue({ entries: [], current_endpoint: '' });
  });

  it('POSTs the current form to updateDiscoveryDockerConfig on Save and reflects the new status', async () => {
    mockApi.fetchDiscoveryDockerStatus.mockResolvedValue(makeStatus({ configured: false }));
    // Server returns the saved-and-reloaded status.
    mockApi.updateDiscoveryDockerConfig.mockResolvedValue(
      makeStatus({ configured: true, reachable: true, strategy_ok: true }),
    );
    render(DiscoveryTab);
    await waitFor(() => expect(screen.getByText(/Discovery is disabled/i)).toBeInTheDocument());

    await fireEvent.click(screen.getByRole('button', { name: /^Save$/i }));

    await waitFor(() => expect(mockApi.updateDiscoveryDockerConfig).toHaveBeenCalled());
    // The post-save status drives the banner to "Connected".
    await waitFor(() =>
      expect(screen.getByText(/Connected to Docker API/i)).toBeInTheDocument(),
    );
  });

  it('surfaces a save error inline rather than swallowing it', async () => {
    mockApi.fetchDiscoveryDockerStatus.mockResolvedValue(makeStatus({ configured: false }));
    mockApi.updateDiscoveryDockerConfig.mockRejectedValue(new Error('refresh_interval is invalid'));
    render(DiscoveryTab);
    await waitFor(() => expect(screen.getByText(/Discovery is disabled/i)).toBeInTheDocument());

    await fireEvent.click(screen.getByRole('button', { name: /^Save$/i }));

    await waitFor(() =>
      expect(screen.getByText(/refresh_interval is invalid/i)).toBeInTheDocument(),
    );
  });

  it('renders the Test result banner with the API response', async () => {
    mockApi.fetchDiscoveryDockerStatus.mockResolvedValue(makeStatus({ configured: false }));
    mockApi.testDiscoveryDockerConfig.mockResolvedValue(
      makeStatus({ reachable: true, strategy_ok: true, configured: true }),
    );
    render(DiscoveryTab);
    await waitFor(() => expect(mockApi.fetchDiscoveryDockerStatus).toHaveBeenCalled());

    await fireEvent.click(screen.getByRole('button', { name: /^Test connection$/i }));

    await waitFor(() => expect(mockApi.testDiscoveryDockerConfig).toHaveBeenCalled());
    expect(screen.getByText(/Test result:/i)).toBeInTheDocument();
  });

  it('renders a red Test result banner when the test call rejects (network down / 5xx)', async () => {
    // Discovery starts already enabled so the form's enabled flag is
    // true. Without that, the catch arm builds a testResult with
    // configured=false, which short-circuits to the gray "Discovery
    // is disabled" badge - hiding the real error.
    mockApi.fetchDiscoveryDockerStatus.mockResolvedValue(makeStatus({ configured: true }));
    mockApi.testDiscoveryDockerConfig.mockRejectedValue(new Error('connect ETIMEDOUT'));
    render(DiscoveryTab);
    await waitFor(() => expect(mockApi.fetchDiscoveryDockerStatus).toHaveBeenCalled());

    await fireEvent.click(screen.getByRole('button', { name: /^Test connection$/i }));

    // Rejection routes through the catch arm which sets reachable=false
    // + last_error. statusBadge() renders that as
    //   "Daemon unreachable: <message>"
    // under a "Test result:" prefix.
    await waitFor(() =>
      expect(screen.getByText(/Daemon unreachable: connect ETIMEDOUT/i)).toBeInTheDocument(),
    );
  });

  it('shows the top-level error when initial load fails', async () => {
    mockApi.fetchDiscoveryDockerStatus.mockRejectedValue(new Error('404 not configured'));
    render(DiscoveryTab);
    await waitFor(() =>
      expect(screen.getByText(/404 not configured/i)).toBeInTheDocument(),
    );
  });
});
