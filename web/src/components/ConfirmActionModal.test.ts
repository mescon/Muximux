import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';
import ConfirmActionModal from './ConfirmActionModal.svelte';
import type { App } from '$lib/types';

function makeApp(overrides: Partial<App> = {}): App {
  return {
    name: 'Restart Sonarr',
    url: 'https://n8n.local/webhook/restart-sonarr',
    icon: { type: 'dashboard', name: 'sonarr' },
    color: '#22c55e',
    group: '',
    order: 0,
    enabled: true,
    default: false,
    open_mode: 'http_action',
    proxy: false,
    scale: 1,
    http_action_method: 'POST',
    ...overrides,
  };
}

describe('ConfirmActionModal', () => {
  it('renders method + url', () => {
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    const { getByText } = render(ConfirmActionModal, { props: { app: makeApp(), onConfirm, onCancel } });
    expect(getByText(/POST/)).toBeTruthy();
    expect(getByText(/n8n\.local/)).toBeTruthy();
  });

  it('defaults method to POST when http_action_method is unset', () => {
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    const { getByText } = render(ConfirmActionModal, {
      props: { app: makeApp({ http_action_method: undefined }), onConfirm, onCancel },
    });
    expect(getByText(/POST/)).toBeTruthy();
  });

  it('Confirm button calls onConfirm', async () => {
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    const { getByRole } = render(ConfirmActionModal, { props: { app: makeApp(), onConfirm, onCancel } });
    await fireEvent.click(getByRole('button', { name: /confirm/i }));
    expect(onConfirm).toHaveBeenCalled();
    expect(onCancel).not.toHaveBeenCalled();
  });

  it('Cancel button calls onCancel', async () => {
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    const { getByRole } = render(ConfirmActionModal, { props: { app: makeApp(), onConfirm, onCancel } });
    await fireEvent.click(getByRole('button', { name: /cancel/i }));
    expect(onCancel).toHaveBeenCalled();
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it('Escape key calls onCancel', async () => {
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    render(ConfirmActionModal, { props: { app: makeApp(), onConfirm, onCancel } });
    await fireEvent.keyDown(window, { key: 'Escape' });
    await waitFor(() => expect(onCancel).toHaveBeenCalled());
  });

  it('backdrop click calls onCancel', async () => {
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    const { getByTestId } = render(ConfirmActionModal, { props: { app: makeApp(), onConfirm, onCancel } });
    await fireEvent.click(getByTestId('confirm-modal-backdrop'));
    expect(onCancel).toHaveBeenCalled();
  });
});
