import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/svelte';
import DockerActionsPopover from './DockerActionsPopover.svelte';
import type { DockerState } from '$lib/types';

vi.mock('$lib/paraglide/messages.js', () => ({
  docker_popover_action_start: () => 'Start container',
  docker_popover_action_stop: () => 'Stop container',
  docker_popover_action_restart: () => 'Restart container',
  docker_popover_header_running: () => 'Running · healthy',
}));

const running: DockerState = { status: 'running', health: 'healthy', restart_count: 0, image: 'x' };
const exited: DockerState = { status: 'exited', health: 'none', restart_count: 0, image: 'x' };

describe('DockerActionsPopover', () => {
  it('running state shows Stop + Restart, not Start', () => {
    const { getByText, queryByText } = render(DockerActionsPopover, {
      props: { state: running, appName: 'sonarr' },
    });
    expect(getByText('Stop container')).toBeTruthy();
    expect(getByText('Restart container')).toBeTruthy();
    expect(queryByText('Start container')).toBeNull();
  });

  it('exited state shows Start only', () => {
    const { getByText, queryByText } = render(DockerActionsPopover, {
      props: { state: exited, appName: 'sonarr' },
    });
    expect(getByText('Start container')).toBeTruthy();
    expect(queryByText('Stop container')).toBeNull();
    expect(queryByText('Restart container')).toBeNull();
  });

  it('clicking Stop dispatches the action event', async () => {
    const onAction = vi.fn();
    const { getByText } = render(DockerActionsPopover, {
      props: { state: running, appName: 'sonarr', onaction: onAction },
    });
    await fireEvent.click(getByText('Stop container'));
    expect(onAction).toHaveBeenCalledWith('stop');
  });
});
