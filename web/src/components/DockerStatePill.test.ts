import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/svelte';
import DockerStatePill from './DockerStatePill.svelte';
import type { DockerState } from '$lib/types';

const base: DockerState = { status: 'running', health: 'healthy', restart_count: 0, image: 'x' };

describe('DockerStatePill', () => {
  it('renders nothing when running + healthy', () => {
    const { container } = render(DockerStatePill, { props: { state: base } });
    expect(container.textContent?.trim() ?? '').toBe('');
  });

  it('renders Stopped when status=exited', () => {
    const { getByText } = render(DockerStatePill, {
      props: { state: { ...base, status: 'exited', health: 'none' } },
    });
    expect(getByText('Stopped')).toBeTruthy();
  });

  it('renders Unhealthy when health=unhealthy', () => {
    const { getByText } = render(DockerStatePill, {
      props: { state: { ...base, status: 'running', health: 'unhealthy' } },
    });
    expect(getByText('Unhealthy')).toBeTruthy();
  });

  it('renders Paused when status=paused', () => {
    const { getByText } = render(DockerStatePill, {
      props: { state: { ...base, status: 'paused' } },
    });
    expect(getByText('Paused')).toBeTruthy();
  });

  it('renders Restarting when status=restarting', () => {
    const { getByText } = render(DockerStatePill, {
      props: { state: { ...base, status: 'restarting' } },
    });
    expect(getByText('Restarting')).toBeTruthy();
  });

  it('renders Starting when health=starting', () => {
    const { getByText } = render(DockerStatePill, {
      props: { state: { ...base, status: 'running', health: 'starting' } },
    });
    expect(getByText('Starting')).toBeTruthy();
  });
});
