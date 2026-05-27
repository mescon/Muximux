import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/svelte';
import DockerStatePill from './DockerStatePill.svelte';
import type { DockerState } from '$lib/types';

vi.mock('$lib/paraglide/messages.js', () => ({
  docker_popover_header_running: () => 'Running',
  docker_state_stopped: () => 'Stopped',
  docker_state_paused: () => 'Paused',
  docker_state_restarting: () => 'Restarting',
  docker_state_starting: () => 'Starting',
  docker_state_unhealthy: () => 'Unhealthy',
}));

const base: DockerState = { status: 'running', health: 'healthy', restart_count: 0, image: 'x' };

// The component renders a small status dot (role=img) labelled via
// aria-label/title, coloured by state -- but only when the state is
// worth a glance. A healthy, running container renders nothing.
describe('DockerStatePill (status dot)', () => {
  it('renders nothing when running + healthy (quiet by default)', () => {
    const { queryByRole } = render(DockerStatePill, { props: { state: base } });
    expect(queryByRole('img')).toBeNull();
  });

  it('renders nothing when running with no healthcheck (health=none)', () => {
    const { queryByRole } = render(DockerStatePill, {
      props: { state: { ...base, status: 'running', health: 'none' } },
    });
    expect(queryByRole('img')).toBeNull();
  });

  it('renders a red dot labelled Stopped when status=exited', () => {
    const { getByRole } = render(DockerStatePill, {
      props: { state: { ...base, status: 'exited', health: 'none' } },
    });
    const dot = getByRole('img', { name: 'Stopped' });
    expect(dot.getAttribute('style')).toContain('--status-error');
  });

  it('treats missing as a red Stopped dot', () => {
    const { getByRole } = render(DockerStatePill, {
      props: { state: { ...base, status: 'missing', health: '' } },
    });
    const dot = getByRole('img', { name: 'Stopped' });
    expect(dot.getAttribute('style')).toContain('--status-error');
  });

  it('renders an amber dot labelled Unhealthy when health=unhealthy', () => {
    const { getByRole } = render(DockerStatePill, {
      props: { state: { ...base, status: 'running', health: 'unhealthy' } },
    });
    const dot = getByRole('img', { name: 'Unhealthy' });
    expect(dot.getAttribute('style')).toContain('--status-warning');
  });

  it('renders a blue dot labelled Restarting when status=restarting', () => {
    const { getByRole } = render(DockerStatePill, {
      props: { state: { ...base, status: 'restarting' } },
    });
    const dot = getByRole('img', { name: 'Restarting' });
    expect(dot.getAttribute('style')).toContain('--status-info');
  });
});
