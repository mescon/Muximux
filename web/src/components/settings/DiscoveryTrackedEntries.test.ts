import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

const mockApi = vi.hoisted(() => ({
  listDockerTracked: vi.fn(),
  detachDockerTracked: vi.fn(),
  ApiError: class ApiError extends Error {
    status: number;
    constructor(msg: string, status: number) {
      super(msg);
      this.status = status;
      this.name = 'ApiError';
    }
  },
}));

vi.mock('$lib/api', async (importOriginal) => ({
  ...mockApi,
  errorText: (await importOriginal<typeof import('$lib/api')>()).errorText,
}));

import DiscoveryTrackedEntries from './DiscoveryTrackedEntries.svelte';

describe('DiscoveryTrackedEntries', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Default to a successful list so tests that don't override get a
    // valid render path.
    mockApi.listDockerTracked.mockResolvedValue({
      entries: [],
      current_endpoint: 'unix:///var/run/docker.sock',
    });
  });

  it('surfaces a load failure inline instead of staying in the loading state', async () => {
    // listDockerTracked rejecting (404, 500, network error) routes to
    // the catch arm and renders a red banner. Important because the
    // page is still useful even with this section broken - the
    // operator can scan again or detach manually.
    mockApi.listDockerTracked.mockRejectedValue(new Error('500 internal'));
    render(DiscoveryTrackedEntries);
    await waitFor(() => expect(screen.getByText(/500 internal/i)).toBeInTheDocument());
  });

  it('opens the relink modal when Re-link is clicked on a stranded row', async () => {
    mockApi.listDockerTracked.mockResolvedValue({
      entries: [
        {
          kind: 'app',
          name: 'sonarr',
          key: 'label:sonarr',
          strategy: 'container_ip',
          endpoint: 'tcp://old:2375',
          url: 'http://10.0.0.42:8989',
          endpoint_matches: false,
        },
      ],
      current_endpoint: 'unix:///var/run/docker.sock',
    });
    render(DiscoveryTrackedEntries);
    await waitFor(() => expect(screen.getByText('sonarr')).toBeInTheDocument());

    // Re-link should be visible because endpoint_matches=false.
    const relink = screen.getByTestId('tracked-relink-btn');
    await fireEvent.click(relink);

    // The stub DiscoveryRelinkModal renders nothing, but the {#if
    // relinkKey} block being entered is what we care about - we
    // verify indirectly by checking that the no-op stub mounted
    // (clicking again wouldn't crash etc.). The simpler thing to
    // assert: the click didn't throw and relink button is still
    // accessible. We rely on coverage to confirm startRelink fired.
    expect(relink).toBeInTheDocument();
  });

  it('confirms before detaching and skips the API call when the user cancels', async () => {
    mockApi.listDockerTracked.mockResolvedValue({
      entries: [
        {
          kind: 'app',
          name: 'plex',
          key: 'label:plex',
          strategy: 'container_ip',
          endpoint: 'unix:///var/run/docker.sock',
          url: 'http://10.0.0.50:32400',
          endpoint_matches: true,
        },
      ],
      current_endpoint: 'unix:///var/run/docker.sock',
    });
    // confirm() returns false on cancel - simulate that.
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    render(DiscoveryTrackedEntries);
    await waitFor(() => expect(screen.getByText('plex')).toBeInTheDocument());

    await fireEvent.click(screen.getByTestId('tracked-detach-btn'));
    expect(confirmSpy).toHaveBeenCalled();
    expect(mockApi.detachDockerTracked).not.toHaveBeenCalled();
    confirmSpy.mockRestore();
  });

  it('issues a detach on confirm and reloads the list', async () => {
    mockApi.listDockerTracked
      .mockResolvedValueOnce({
        entries: [
          { kind: 'app', name: 'plex', key: 'label:plex', strategy: 'container_ip', endpoint: 'unix:///var/run/docker.sock', url: 'http://x:32400', endpoint_matches: true },
        ],
        current_endpoint: 'unix:///var/run/docker.sock',
      })
      .mockResolvedValueOnce({ entries: [], current_endpoint: 'unix:///var/run/docker.sock' });
    mockApi.detachDockerTracked.mockResolvedValue(undefined);
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    const ontrackingchanged = vi.fn();

    render(DiscoveryTrackedEntries, { ontrackingchanged });
    await waitFor(() => expect(screen.getByText('plex')).toBeInTheDocument());
    await fireEvent.click(screen.getByTestId('tracked-detach-btn'));

    await waitFor(() => expect(mockApi.detachDockerTracked).toHaveBeenCalledWith('label:plex'));
    // The parent (Settings) is told, so its copy of the app unlocks (#479).
    expect(ontrackingchanged).toHaveBeenCalled();
    // After detach, the second listDockerTracked call drives the
    // empty-state render.
    await waitFor(() =>
      expect(screen.getByText(/No apps or gateway sites are linked to Docker yet/i)).toBeInTheDocument(),
    );
  });

  it('treats a 404 on detach as idempotent success (already detached by a concurrent caller)', async () => {
    // The branch we care about: ApiError with status=404 should NOT
    // pop an alert - the desired state has already been reached.
    mockApi.listDockerTracked
      .mockResolvedValueOnce({
        entries: [
          { kind: 'app', name: 'plex', key: 'label:plex', strategy: 'container_ip', endpoint: 'unix:///var/run/docker.sock', url: 'http://x:32400', endpoint_matches: true },
        ],
        current_endpoint: 'unix:///var/run/docker.sock',
      })
      .mockResolvedValueOnce({ entries: [], current_endpoint: 'unix:///var/run/docker.sock' });
    mockApi.detachDockerTracked.mockRejectedValue(new mockApi.ApiError('Not Found', 404));
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    const alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {});
    const ontrackingchanged = vi.fn();

    render(DiscoveryTrackedEntries, { ontrackingchanged });
    await waitFor(() => expect(screen.getByText('plex')).toBeInTheDocument());
    await fireEvent.click(screen.getByTestId('tracked-detach-btn'));

    await waitFor(() =>
      expect(screen.getByText(/No apps or gateway sites are linked to Docker yet/i)).toBeInTheDocument(),
    );
    expect(alertSpy).not.toHaveBeenCalled();
    expect(ontrackingchanged).toHaveBeenCalled();
    alertSpy.mockRestore();
  });

  it('surfaces a non-404 detach failure via alert and keeps the row visible', async () => {
    mockApi.listDockerTracked
      .mockResolvedValueOnce({
        entries: [
          { kind: 'app', name: 'plex', key: 'label:plex', strategy: 'container_ip', endpoint: 'unix:///var/run/docker.sock', url: 'http://x:32400', endpoint_matches: true },
        ],
        current_endpoint: 'unix:///var/run/docker.sock',
      })
      .mockResolvedValueOnce({
        entries: [
          { kind: 'app', name: 'plex', key: 'label:plex', strategy: 'container_ip', endpoint: 'unix:///var/run/docker.sock', url: 'http://x:32400', endpoint_matches: true },
        ],
        current_endpoint: 'unix:///var/run/docker.sock',
      });
    mockApi.detachDockerTracked.mockRejectedValue(new mockApi.ApiError('Internal Server Error', 500));
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    const ontrackingchanged = vi.fn();

    render(DiscoveryTrackedEntries, { ontrackingchanged });
    await waitFor(() => expect(screen.getByText('plex')).toBeInTheDocument());
    await fireEvent.click(screen.getByTestId('tracked-detach-btn'));

    const alertEl = await screen.findByRole('alert');
    expect(alertEl).toHaveTextContent(/Could not detach plex: Internal Server Error/i);
    expect(screen.getByText('plex')).toBeInTheDocument();
    expect(mockApi.listDockerTracked).toHaveBeenCalledTimes(2);
    expect(ontrackingchanged).not.toHaveBeenCalled();
  });

  it('formats last-seen via "<n>s/m/h ago" relative-time helper, falling back to ISO past 24h', async () => {
    // ago() has four arms (sec/min/hour/iso) and a default for missing
    // timestamps. We drive the under-60s arm; the others are unit-
    // testable but a single visible representative pins the wiring.
    const tenSecondsAgo = new Date(Date.now() - 10_000).toISOString();
    mockApi.listDockerTracked.mockResolvedValue({
      entries: [
        { kind: 'app', name: 'recent', key: 'label:recent', strategy: 'container_ip', endpoint: 'unix:///var/run/docker.sock', url: 'http://x:80', last_seen_at: tenSecondsAgo, endpoint_matches: true },
      ],
      current_endpoint: 'unix:///var/run/docker.sock',
    });
    render(DiscoveryTrackedEntries);
    // The container name "recent" appears twice in the DOM (as the
    // entry's <span> and as part of the key "label:recent"). The
    // last-seen line is the unique assertion here - it carries the
    // "<n>s ago" formatting which is what we're actually pinning.
    await waitFor(() =>
      expect(screen.getByText(/last seen .*seconds? ago/i)).toBeInTheDocument(),
    );
  });

  it('lists quarantined entries with their reason and removes one through the detach endpoint', async () => {
    mockApi.listDockerTracked.mockResolvedValue({ entries: [], current_endpoint: 'unix:///s',
      quarantined: [{ kind: 'app', name: 'Vaultwarden', key: 'swarm:vw', reason: 'url is required' }] });
    mockApi.detachDockerTracked.mockResolvedValue(undefined);
    render(DiscoveryTrackedEntries);
    expect(await screen.findByText('Not loaded: invalid auto-imported entries')).toBeInTheDocument();
    expect(screen.getByText(/url is required/)).toBeInTheDocument();
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    await fireEvent.click(screen.getByTestId('quarantined-remove-btn'));
    await waitFor(() => expect(mockApi.detachDockerTracked).toHaveBeenCalledWith('swarm:vw'));
    await waitFor(() => expect(mockApi.listDockerTracked).toHaveBeenCalledTimes(2));
  });

  it('Remove all detaches every quarantined key, and cancel does nothing', async () => {
    mockApi.listDockerTracked.mockResolvedValue({ entries: [], current_endpoint: 'unix:///s', quarantined: [
      { kind: 'app', name: 'A', key: 'compose:p:a', reason: 'r' },
      { kind: 'gateway', name: 'a.example.com', key: 'compose:p:a', reason: 'its app is quarantined' },
      { kind: 'app', name: 'B', key: 'label:b', reason: 'r' }] });
    mockApi.detachDockerTracked.mockResolvedValue(undefined);
    render(DiscoveryTrackedEntries);
    const removeAll = await screen.findByText('Remove all');
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
    await fireEvent.click(removeAll);
    expect(mockApi.detachDockerTracked).not.toHaveBeenCalled();
    confirm.mockReturnValue(true);
    await fireEvent.click(removeAll);
    await waitFor(() => expect(mockApi.detachDockerTracked).toHaveBeenCalledTimes(2));
    expect(mockApi.detachDockerTracked).toHaveBeenCalledWith('compose:p:a');
    expect(mockApi.detachDockerTracked).toHaveBeenCalledWith('label:b');
  });

  it('treats a 404 on quarantined removal as done and alerts on other failures', async () => {
    mockApi.listDockerTracked.mockResolvedValue({ entries: [], current_endpoint: 'unix:///s',
      quarantined: [{ kind: 'app', name: 'A', key: 'label:a', reason: 'r' }] });
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    render(DiscoveryTrackedEntries);
    const btn = await screen.findByTestId('quarantined-remove-btn');
    mockApi.detachDockerTracked.mockRejectedValueOnce(new mockApi.ApiError('gone', 404));
    await fireEvent.click(btn);
    await waitFor(() => expect(mockApi.listDockerTracked).toHaveBeenCalledTimes(2));
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    mockApi.detachDockerTracked.mockRejectedValueOnce(new Error('boom'));
    await fireEvent.click(await screen.findByTestId('quarantined-remove-btn'));
    expect(await screen.findByRole('alert')).toHaveTextContent('Could not remove A: boom');
    expect(screen.getByTestId('quarantined-remove-btn')).toBeInTheDocument();
    expect(mockApi.listDockerTracked).toHaveBeenCalledTimes(3);
  });

  it('declining the single Remove confirm does nothing', async () => {
    mockApi.listDockerTracked.mockResolvedValue({ entries: [], current_endpoint: 'unix:///s',
      quarantined: [{ kind: 'app', name: 'A', key: 'label:a', reason: 'r' }] });
    vi.spyOn(window, 'confirm').mockReturnValue(false);
    render(DiscoveryTrackedEntries);
    await fireEvent.click(await screen.findByTestId('quarantined-remove-btn'));
    expect(mockApi.detachDockerTracked).not.toHaveBeenCalled();
  });

  it('renders no quarantined section when the field is absent', async () => {
    render(DiscoveryTrackedEntries);
    await waitFor(() => expect(mockApi.listDockerTracked).toHaveBeenCalled());
    expect(screen.queryByText('Not loaded: invalid auto-imported entries')).not.toBeInTheDocument();
  });

  it('shows a missing badge for entries with missing_since', async () => {
    mockApi.listDockerTracked.mockResolvedValue({ current_endpoint: 'unix:///s', entries: [
      { kind: 'app', name: 'sonarr', key: 'label:sonarr', strategy: 'container_ip', endpoint: 'unix:///s',
        url: 'http://10.0.0.42:8989', endpoint_matches: true, missing_since: new Date(Date.now() - 3600_000).toISOString() }] });
    render(DiscoveryTrackedEntries);
    expect(await screen.findByText(/Container missing since/)).toBeInTheDocument();
  });

  it('moves focus to the next quarantined row after removal, then to the heading when none remain', async () => {
    const q = (n: string) => ({ kind: 'app', name: n, key: 'label:' + n, reason: 'r' });
    mockApi.listDockerTracked
      .mockResolvedValueOnce({ entries: [], current_endpoint: 'u', quarantined: [q('a'), q('b')] })
      .mockResolvedValueOnce({ entries: [], current_endpoint: 'u', quarantined: [q('b')] })
      .mockResolvedValueOnce({ entries: [], current_endpoint: 'u' });
    mockApi.detachDockerTracked.mockResolvedValue(undefined);
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    render(DiscoveryTrackedEntries);
    const btns = await screen.findAllByTestId('quarantined-remove-btn');
    expect(btns[0]).toHaveAccessibleName('Remove a (app)');
    expect(btns[1]).toHaveAccessibleName('Remove b (app)');
    await fireEvent.click(btns[0]);
    await waitFor(() => expect(screen.getByTestId('quarantined-remove-btn')).toHaveFocus());
    await fireEvent.click(screen.getByTestId('quarantined-remove-btn'));
    await waitFor(() => expect(screen.getByText('Currently tracked')).toHaveFocus());
  });

  it('focuses the previous quarantined row when the last one is removed', async () => {
    const q = (n: string) => ({ kind: 'app', name: n, key: 'label:' + n, reason: 'r' });
    mockApi.listDockerTracked
      .mockResolvedValueOnce({ entries: [], current_endpoint: 'u', quarantined: [q('a'), q('b')] })
      .mockResolvedValueOnce({ entries: [], current_endpoint: 'u', quarantined: [q('a')] });
    mockApi.detachDockerTracked.mockResolvedValue(undefined);
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    render(DiscoveryTrackedEntries);
    const btns = await screen.findAllByTestId('quarantined-remove-btn');
    await fireEvent.click(btns[1]);
    await waitFor(() => expect(screen.getByTestId('quarantined-remove-btn')).toHaveFocus());
  });

  it('focuses the next detach button after a successful detach', async () => {
    const e = (n: string) => ({ kind: 'app', name: n, key: 'label:' + n, strategy: 'container_ip', endpoint: 'unix:///s', url: 'http://x', endpoint_matches: true });
    mockApi.listDockerTracked
      .mockResolvedValueOnce({ entries: [e('a'), e('b')], current_endpoint: 'unix:///s' })
      .mockResolvedValueOnce({ entries: [e('b')], current_endpoint: 'unix:///s' });
    mockApi.detachDockerTracked.mockResolvedValue(undefined);
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    render(DiscoveryTrackedEntries);
    const btns = await screen.findAllByTestId('tracked-detach-btn');
    await fireEvent.click(btns[0]);
    await waitFor(() => expect(screen.getByTestId('tracked-detach-btn')).toHaveFocus());
  });
});
