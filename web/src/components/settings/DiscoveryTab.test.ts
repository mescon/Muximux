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
  listDockerNetworks: vi.fn(),
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
    mockApi.listDockerNetworks.mockResolvedValue({ networks: [] });
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
    mockApi.listDockerNetworks.mockResolvedValue({ networks: [] });
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
    mockApi.listDockerNetworks.mockResolvedValue({ networks: [] });
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
    mockApi.listDockerNetworks.mockResolvedValue({ networks: [] });
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
    mockApi.listDockerNetworks.mockResolvedValue({ networks: [] });
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

describe('DiscoveryTab network-filter autocomplete', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockApi.fetchDiscoveryDockerStatus.mockResolvedValue(makeStatus());
    mockApi.listDockerTracked.mockResolvedValue({ entries: [], current_endpoint: '' });
  });

  it('renders a chip strip of available networks once /api/discovery/docker/networks responds', async () => {
    mockApi.listDockerNetworks.mockResolvedValue({
      networks: ['bridge', 'host', 'media', 'arr_default'],
    });
    render(DiscoveryTab);

    // Chip strip is rendered with a "Available on this daemon:" label
    // and one clickable button per network name. We pin both: the
    // label exists, and every network appears as its own role=button
    // inside the strip's data-testid scope.
    await waitFor(() =>
      expect(screen.getByText(/Available on this daemon:/i)).toBeInTheDocument(),
    );
    const strip = screen.getByTestId('dd-filter-chips');
    for (const name of ['bridge', 'host', 'media', 'arr_default']) {
      expect(strip.querySelector(`button[title="Use Docker network ${name}"]`)).toBeTruthy();
    }
  });

  it('clicking a chip fills the network_filter input with that network name', async () => {
    mockApi.listDockerNetworks.mockResolvedValue({ networks: ['media', 'monitoring'] });
    render(DiscoveryTab);
    await waitFor(() => expect(screen.getByText(/Available on this daemon:/i)).toBeInTheDocument());

    const filter = document.getElementById('dd-filter') as HTMLInputElement;
    expect(filter.value).toBe('');
    const mediaChip = screen.getByTitle('Use Docker network media');
    await fireEvent.click(mediaChip);
    expect(filter.value).toBe('media');
  });

  it('hides the chip strip when no networks are available (daemon unreachable / discovery off)', async () => {
    // listDockerNetworks rejecting is the "daemon unreachable" path;
    // returning an empty array is the "discovery off" path. Both
    // collapse to "no chips" in the UI, which is what we want -
    // there's nothing useful to show and the input falls back to a
    // plain text field.
    mockApi.listDockerNetworks.mockRejectedValue(new Error('502'));
    render(DiscoveryTab);
    // Wait for the initial load to settle.
    await waitFor(() => expect(mockApi.fetchDiscoveryDockerStatus).toHaveBeenCalled());
    // The label only renders when there is at least one network.
    expect(screen.queryByText(/Available on this daemon:/i)).not.toBeInTheDocument();
    expect(screen.queryByTestId('dd-filter-chips')).not.toBeInTheDocument();
    // Plain text input still exists.
    expect(document.getElementById('dd-filter')).toBeTruthy();
  });

  it('renders a datalist mirroring the chip names so native autocomplete also works', async () => {
    mockApi.listDockerNetworks.mockResolvedValue({ networks: ['bridge', 'media'] });
    render(DiscoveryTab);
    await waitFor(() => expect(screen.getByText(/Available on this daemon:/i)).toBeInTheDocument());

    const list = document.getElementById('dd-filter-networks');
    expect(list).toBeTruthy();
    const options = list?.querySelectorAll('option') ?? [];
    expect(options.length).toBe(2);
    expect(options[0].getAttribute('value')).toBe('bridge');
    expect(options[1].getAttribute('value')).toBe('media');
    // The input is wired to the datalist via list=.
    const input = document.getElementById('dd-filter') as HTMLInputElement;
    expect(input.getAttribute('list')).toBe('dd-filter-networks');
  });

  it('handles a malformed listDockerNetworks response (networks=undefined) without crashing', async () => {
    // The frontend nullish-coalesces `r.networks ?? []` to defend
    // against backend payloads that omit the field (older releases,
    // a broken proxy, a future schema change). Cover that arm so a
    // regression in the helper doesn't silently throw at mount.
    mockApi.listDockerNetworks.mockResolvedValue({} as { networks: string[] });
    render(DiscoveryTab);
    await waitFor(() => expect(mockApi.fetchDiscoveryDockerStatus).toHaveBeenCalled());
    // No chip strip should render, but the form must still mount.
    expect(screen.queryByTestId('dd-filter-chips')).not.toBeInTheDocument();
    expect(document.getElementById('dd-filter')).toBeTruthy();
  });

  it('the chip strip shows a "clear" affordance once a network is selected', async () => {
    mockApi.listDockerNetworks.mockResolvedValue({ networks: ['media'] });
    render(DiscoveryTab);
    await waitFor(() => expect(screen.getByText(/Available on this daemon:/i)).toBeInTheDocument());

    // No clear affordance until a value is picked.
    expect(screen.queryByText(/^clear$/i)).not.toBeInTheDocument();
    await fireEvent.click(screen.getByTitle('Use Docker network media'));

    // Now the clear button is in the strip.
    expect(screen.getByText(/^clear$/i)).toBeInTheDocument();
    await fireEvent.click(screen.getByText(/^clear$/i));
    const input = document.getElementById('dd-filter') as HTMLInputElement;
    expect(input.value).toBe('');
  });
});

describe('DiscoveryTab lifecycle subsection', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockApi.listDockerTracked.mockResolvedValue({ entries: [], current_endpoint: 'unix:///var/run/docker.sock' });
    mockApi.listDockerNetworks.mockResolvedValue({ networks: [] });
  });

  it('renders the socket-status line when status is loaded', async () => {
    mockApi.fetchDiscoveryDockerStatus.mockResolvedValue(makeStatus({ socket_writable: true, lifecycle_enabled: false }));
    render(DiscoveryTab);
    expect(await screen.findByText(/Docker socket: writable/i)).toBeInTheDocument();
  });

  it('disables the lifecycle_enabled checkbox when the socket is read-only', async () => {
    mockApi.fetchDiscoveryDockerStatus.mockResolvedValue(makeStatus({ socket_writable: false, lifecycle_enabled: false }));
    render(DiscoveryTab);
    const checkbox = (await screen.findByLabelText(/Enable container lifecycle controls/i)) as HTMLInputElement;
    expect(checkbox.disabled).toBe(true);
  });

  it('shows the min-role dropdown and allowed-groups input when lifecycle is enabled', async () => {
    mockApi.fetchDiscoveryDockerStatus.mockResolvedValue(makeStatus({ socket_writable: true, lifecycle_enabled: true }));
    render(DiscoveryTab);
    expect(await screen.findByLabelText(/Minimum role/i)).toBeInTheDocument();
    expect(await screen.findByLabelText(/Allowed groups/i)).toBeInTheDocument();
  });

  it('hides the health-badge placement nothing extra when socket unreachable but still renders subsection', async () => {
    mockApi.fetchDiscoveryDockerStatus.mockResolvedValue(makeStatus({ reachable: false, socket_writable: false }));
    render(DiscoveryTab);
    expect(await screen.findByLabelText(/Show container health badges/i)).toBeInTheDocument();
  });
});
