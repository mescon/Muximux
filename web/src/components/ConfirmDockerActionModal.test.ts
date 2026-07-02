import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/svelte';
import ConfirmDockerActionModal from './ConfirmDockerActionModal.svelte';

vi.mock('$lib/paraglide/messages.js', () => ({
  docker_modal_confirm_stop: ({ name }: { name: string }) => `Stop container '${name}'?`,
  docker_modal_confirm_restart: ({ name }: { name: string }) => `Restart container '${name}'?`,
  docker_modal_body: ({ image, uptimeOrExit }: { image: string; uptimeOrExit: string }) =>
    `Image: ${image} · ${uptimeOrExit}`,
  common_cancel: () => 'Cancel',
  common_confirm: () => 'Confirm',
}));

describe('ConfirmDockerActionModal', () => {
  it('renders the stop heading + body', () => {
    const { getByText } = render(ConfirmDockerActionModal, {
      props: {
        appName: 'sonarr',
        action: 'stop',
        image: 'linuxserver/sonarr:latest',
        uptimeOrExit: 'up 3d',
      },
    });
    expect(getByText("Stop container 'sonarr'?")).toBeTruthy();
    expect(getByText('Image: linuxserver/sonarr:latest · up 3d')).toBeTruthy();
  });

  it('Esc dispatches cancel', async () => {
    const oncancel = vi.fn();
    render(ConfirmDockerActionModal, {
      props: { appName: 'sonarr', action: 'stop', image: 'x', uptimeOrExit: '', oncancel },
    });
    await fireEvent.keyDown(window, { key: 'Escape' });
    expect(oncancel).toHaveBeenCalledOnce();
  });

  it('Enter dispatches confirm', async () => {
    const onconfirm = vi.fn();
    render(ConfirmDockerActionModal, {
      props: { appName: 'sonarr', action: 'stop', image: 'x', uptimeOrExit: '', onconfirm },
    });
    await fireEvent.keyDown(window, { key: 'Enter' });
    expect(onconfirm).toHaveBeenCalledOnce();
  });

  it('clicking the Confirm button dispatches confirm', async () => {
    const onconfirm = vi.fn();
    const { getByText } = render(ConfirmDockerActionModal, {
      props: { appName: 'sonarr', action: 'restart', image: 'x', uptimeOrExit: '', onconfirm },
    });
    await fireEvent.click(getByText('Confirm'));
    expect(onconfirm).toHaveBeenCalledOnce();
  });

  it('clicking the backdrop dispatches cancel', async () => {
    const oncancel = vi.fn();
    const { container } = render(ConfirmDockerActionModal, {
      props: { appName: 'sonarr', action: 'stop', image: 'x', uptimeOrExit: '', oncancel },
    });
    const backdrop = container.querySelector('.modal-backdrop') as HTMLElement;
    await fireEvent.click(backdrop);
    expect(oncancel).toHaveBeenCalledOnce();
  });

  // #27: while the confirmed action runs the modal stays open but is
  // disabled/non-dismissable, so it cannot be re-fired or cancelled.
  it('disables both buttons and marks the dialog busy while loading', () => {
    const { getByText, container } = render(ConfirmDockerActionModal, {
      props: { appName: 'sonarr', action: 'restart', image: 'x', uptimeOrExit: '', loading: true },
    });
    expect((getByText('Confirm').closest('button') as HTMLButtonElement).disabled).toBe(true);
    expect((getByText('Cancel').closest('button') as HTMLButtonElement).disabled).toBe(true);
    expect(container.querySelector('[role="dialog"]')?.getAttribute('aria-busy')).toBe('true');
  });

  it('ignores Escape and backdrop clicks while loading', async () => {
    const oncancel = vi.fn();
    const { container } = render(ConfirmDockerActionModal, {
      props: { appName: 'sonarr', action: 'stop', image: 'x', uptimeOrExit: '', loading: true, oncancel },
    });
    await fireEvent.keyDown(window, { key: 'Escape' });
    await fireEvent.click(container.querySelector('.modal-backdrop') as HTMLElement);
    expect(oncancel).not.toHaveBeenCalled();
  });
});
