import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';
import type { SystemInfo, UpdateInfo } from '$lib/types';

// Hoisted mocks for api functions
const mockApi = vi.hoisted(() => ({
  fetchSystemInfo: vi.fn(),
  checkForUpdates: vi.fn(),
}));

// Mock $lib/api
vi.mock('$lib/api', () => ({
  fetchSystemInfo: mockApi.fetchSystemInfo,
  checkForUpdates: mockApi.checkForUpdates,
}));

// Mock marked
vi.mock('marked', () => ({
  marked: {
    parse: vi.fn((text: string) => `<p>${text}</p>`),
  },
}));

import AboutTab from './AboutTab.svelte';

function makeSystemInfo(overrides: Partial<SystemInfo> = {}): SystemInfo {
  return {
    version: '3.2.0',
    commit: 'abc12345def67890',
    build_date: '2026-01-15T10:00:00Z',
    go_version: 'go1.22.0',
    os: 'linux',
    arch: 'amd64',
    environment: 'docker',
    uptime: '2d 5h 30m',
    uptime_seconds: 192600,
    started_at: '2026-01-13T04:30:00Z',
    data_dir: '/data',
    links: {
      github: 'https://github.com/mescon/Muximux',
      issues: 'https://github.com/mescon/Muximux/issues',
      releases: 'https://github.com/mescon/Muximux/releases',
      wiki: 'https://github.com/mescon/Muximux/wiki',
    },
    ...overrides,
  };
}

function makeUpdateInfo(overrides: Partial<UpdateInfo> = {}): UpdateInfo {
  return {
    current_version: '3.2.0',
    latest_version: '3.3.0',
    update_available: true,
    release_url: 'https://github.com/mescon/Muximux/releases/tag/v3.3.0',
    changelog: '## What\'s New\n- Feature A\n- Bug fix B',
    published_at: '2026-02-01T12:00:00Z',
    download_urls: {
      linux_amd64: 'https://github.com/mescon/Muximux/releases/download/v3.3.0/muximux-linux-amd64',
    },
    ...overrides,
  };
}

describe('AboutTab', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows loading state initially', () => {
    // Set up promises that never resolve to keep loading state
    mockApi.fetchSystemInfo.mockReturnValue(new Promise(() => {}));
    mockApi.checkForUpdates.mockReturnValue(new Promise(() => {}));

    render(AboutTab);

    expect(screen.getByText('Loading system info...')).toBeInTheDocument();
  });

  it('shows system info after data loads', async () => {
    const sysInfo = makeSystemInfo();
    mockApi.fetchSystemInfo.mockResolvedValue(sysInfo);
    mockApi.checkForUpdates.mockResolvedValue(null);

    render(AboutTab);

    await waitFor(() => {
      expect(screen.getByText('3.2.0')).toBeInTheDocument();
    });

    expect(screen.getByText('go1.22.0')).toBeInTheDocument();
    expect(screen.getByText('2d 5h 30m')).toBeInTheDocument();
    expect(screen.getByText('/data')).toBeInTheDocument();
    expect(screen.getByText('abc12345')).toBeInTheDocument();
  });

  it('shows environment info', async () => {
    mockApi.fetchSystemInfo.mockResolvedValue(makeSystemInfo({ environment: 'docker' }));
    mockApi.checkForUpdates.mockResolvedValue(null);

    render(AboutTab);

    await waitFor(() => {
      expect(screen.getByText('docker')).toBeInTheDocument();
    });
  });

  it('shows platform info', async () => {
    mockApi.fetchSystemInfo.mockResolvedValue(makeSystemInfo({ os: 'linux', arch: 'amd64' }));
    mockApi.checkForUpdates.mockResolvedValue(null);

    render(AboutTab);

    await waitFor(() => {
      expect(screen.getByText('linux/amd64')).toBeInTheDocument();
    });
  });

  it('shows error state on fetch failure', async () => {
    mockApi.fetchSystemInfo.mockRejectedValue(new Error('Connection refused'));
    mockApi.checkForUpdates.mockRejectedValue(new Error('Connection refused'));

    render(AboutTab);

    await waitFor(() => {
      expect(screen.getByText('Connection refused')).toBeInTheDocument();
    });

    expect(screen.getByText('Try again')).toBeInTheDocument();
  });

  it('shows "up to date" when no update is available', async () => {
    mockApi.fetchSystemInfo.mockResolvedValue(makeSystemInfo());
    mockApi.checkForUpdates.mockResolvedValue(
      makeUpdateInfo({ update_available: false, latest_version: '3.2.0' })
    );

    render(AboutTab);

    await waitFor(() => {
      expect(screen.getByText("You're up to date")).toBeInTheDocument();
    });
  });

  it('shows update available banner when update exists', async () => {
    mockApi.fetchSystemInfo.mockResolvedValue(makeSystemInfo());
    mockApi.checkForUpdates.mockResolvedValue(
      makeUpdateInfo({ update_available: true, latest_version: '3.3.0' })
    );

    render(AboutTab);

    await waitFor(() => {
      expect(screen.getByText('Update Available')).toBeInTheDocument();
    });

    expect(screen.getByText('v3.3.0')).toBeInTheDocument();
  });

  it('shows GitHub link in system info', async () => {
    mockApi.fetchSystemInfo.mockResolvedValue(makeSystemInfo());
    mockApi.checkForUpdates.mockResolvedValue(null);

    render(AboutTab);

    await waitFor(() => {
      expect(screen.getByText('GitHub')).toBeInTheDocument();
    });

    expect(screen.getByText('Issues')).toBeInTheDocument();
    expect(screen.getByText('Releases')).toBeInTheDocument();
    expect(screen.getByText('Wiki')).toBeInTheDocument();
  });

  it('shows View on GitHub link when release URL is available', async () => {
    mockApi.fetchSystemInfo.mockResolvedValue(makeSystemInfo());
    mockApi.checkForUpdates.mockResolvedValue(
      makeUpdateInfo({ update_available: true, release_url: 'https://github.com/mescon/Muximux/releases/tag/v3.3.0' })
    );

    render(AboutTab);

    await waitFor(() => {
      expect(screen.getByText('View on GitHub')).toBeInTheDocument();
    });
  });

  it('gracefully handles checkForUpdates failure while showing system info', async () => {
    mockApi.fetchSystemInfo.mockResolvedValue(makeSystemInfo());
    mockApi.checkForUpdates.mockRejectedValue(new Error('Rate limited'));

    render(AboutTab);

    // System info should still show even if update check fails
    await waitFor(() => {
      expect(screen.getByText('3.2.0')).toBeInTheDocument();
    });

    expect(screen.getByText('go1.22.0')).toBeInTheDocument();
  });
});
